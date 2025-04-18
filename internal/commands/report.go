package commands

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/urfave/cli/v3"

	gorppkg "github.com/reportportal/goRP/v5/pkg/gorp"
	"github.com/reportportal/goRP/v5/pkg/openapi"
)

const logsBatchSize = 10

var (
	reportCommand = &cli.Command{
		Name:     "report",
		Usage:    "Reports input to report portal",
		Commands: []*cli.Command{reportTest2JsonCommand},
	}

	reportTest2JsonCommand = &cli.Command{
		Name:  "test2json",
		Usage: "Input format: test2json",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "file",
				Aliases: []string{"f"},
				Usage:   "File Name",
				Sources: cli.EnvVars("FILE"),
			},
			&cli.StringFlag{
				Name:    "launchName",
				Aliases: []string{"ln"},
				Usage:   "Launch Name",
				Sources: cli.EnvVars("LAUNCH_NAME"),
				Value:   "gorp launch",
			},
			&cli.BoolFlag{
				Name:    "reportEmptyPkg",
				Aliases: []string{"ep"},
				Usage:   "Whether empty packages need to be reporter. Default is false",
				Sources: cli.EnvVars("REPORT_EMPTY_PKG"),
				Value:   false,
			},
			&cli.StringSliceFlag{
				Name:    "attr",
				Aliases: []string{"a"},
				Usage:   "Launch attribute with format 'key:value'. Omitting a ':' separator will tag the launch with the value.",
			},
		},
		Action: reportTest2json,
	}
)

func reportTest2json(ctx context.Context, cmd *cli.Command) error {
	rpClient, _, err := buildClient(cmd)
	if err != nil {
		return err
	}
	input := make(chan *testEvent)

	// run in separate goroutine
	launchNameArg := cmd.String("launchName")
	reportEmptyPkgArg := cmd.Bool("reportEmptyPkg")
	attrArgs := cmd.StringSlice("attr")
	rep := newReporter(rpClient, launchNameArg, input, reportEmptyPkgArg, attrArgs...)

	errChan := make(chan error)
	wg := &sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := rep.receive(); err != nil {
			errChan <- err
			return
		}
	}()
	// wait for report to complete
	defer wg.Wait()

	defer close(input)

	var reader io.Reader
	if fileName := cmd.String("file"); fileName != "" {
		f, fErr := os.Open(filepath.Clean(fileName))
		if fErr != nil {
			return fErr
		}
		defer func() {
			if cErr := f.Close(); cErr != nil {
				slog.Error(cErr.Error())
			}
		}()
		reader = f
	} else {
		reader = os.Stdin
	}

	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		data := scanner.Text()

		var ev testEvent
		if err := json.Unmarshal([]byte(data), &ev); err != nil {
			slog.Error(err.Error())
			return err
		}
		select {
		case err := <-errChan:
			slog.Error("input processing interrupted", "error", err)
			return err
		case input <- &ev:
		}
	}
	return nil
}

type testEvent struct {
	Time    time.Time `json:"time"` // encodes as an RFC3339-format string
	Action  string    `json:"action"`
	Package string    `json:"package"`
	Test    string    `json:"test"`
	Elapsed float64   `json:"elapsed"` // seconds
	Output  string    `json:"output"`
}

type reporter struct {
	input            <-chan *testEvent
	client           *gorppkg.Client
	launchName       string
	launchID         string
	launchOnce       sync.Once
	launchAttributes []openapi.ItemAttributesRQ
	tests            map[string]string
	suites           map[string]string
	logs             []*openapi.SaveLogRQ
	logsBatchSize    int
	waitQueue        sync.WaitGroup
	reportEmpty      bool
}

func newReporter(
	client *gorppkg.Client,
	launchName string,
	input <-chan *testEvent,
	reportEmpty bool,
	launchAttrArgs ...string,
) *reporter {
	launchAttributes := make([]openapi.ItemAttributesRQ, len(launchAttrArgs))
	for idx, attr := range launchAttrArgs {
		// Separate the key:value pair. If `:` is not present, the entire string is considered the value and an empty key is used
		attribute := openapi.ItemAttributesRQ{
			System: openapi.PtrBool(false),
		}
		if key, value, ok := strings.Cut(attr, ":"); ok {
			attribute.SetKey(key)
			attribute.SetValue(value)
		} else {
			attribute.SetValue(attr)
		}
		launchAttributes[idx] = attribute
	}

	return &reporter{
		input:            input,
		launchName:       launchName,
		launchAttributes: launchAttributes,
		client:           client,
		launchOnce:       sync.Once{},
		tests:            map[string]string{},
		suites:           map[string]string{},
		logs:             []*openapi.SaveLogRQ{},
		logsBatchSize:    logsBatchSize,
		reportEmpty:      reportEmpty,
	}
}

func (r *reporter) reportEvent(ev *testEvent) error {
	var err error
	switch ev.Action {
	case "start":
		if r.reportEmpty {
			_, err = r.startSuite(ev)
		}
	case "run":
		err = r.startTest(ev)
	case "output":
		r.log(ev)
	case "pass":
		err = r.finish(ev, gorppkg.Statuses.Passed)
	case "fail":
		err = r.finish(ev, gorppkg.Statuses.Failed)
	}
	return err
}

func (r *reporter) receive() error {
	prevEventTime := time.Now()
	for ev := range r.input {
		var err error
		startTime := ev.Time

		// start launch once
		// when first event comes
		r.launchOnce.Do(func() {
			if err = r.startLaunch(startTime); err != nil {
				slog.Error(err.Error())
			}
		})
		if err != nil {
			return err
		}

		// report event to ReportPortal
		err = r.reportEvent(ev)
		if err != nil {
			return err
		}

		// remember last's event time
		// for RP's finishLaunch
		prevEventTime = ev.Time
	}

	// make sure we flush all logs that are left
	r.flushLogs(true)
	// wait for requests to get sent
	r.waitQueue.Wait()

	// finish launch of started
	if r.launchID != "" {
		if err := r.finishLaunch(gorppkg.Statuses.Passed, prevEventTime); err != nil {
			return err
		}
	}
	return nil
}

func (r *reporter) startSuite(ev *testEvent) (string, error) {
	rs, err := r.client.StartTest(&openapi.StartTestItemRQ{
		Name:       ev.Package,
		StartTime:  ev.Time,
		LaunchUuid: r.launchID,
		HasStats:   openapi.PtrBool(false),
		Type:       string(gorppkg.TestItemTypes.Suite),
		Retry:      openapi.PtrBool(false),
	})
	if err != nil {
		return "", err
	}
	r.suites[ev.Package] = *rs.Id
	return *rs.Id, nil
}

func (r *reporter) startTest(ev *testEvent) error {
	testID := r.getTestName(ev)
	suiteID, found := r.suites[ev.Package]
	if !found {
		if r.reportEmpty {
			return fmt.Errorf("unable to find suiteID for package: %s", ev.Package)
		}
		var err error
		suiteID, err = r.startSuite(ev)
		if err != nil {
			return err
		}
	}
	rs, err := r.client.StartChildTest(suiteID, &openapi.StartTestItemRQ{
		Name:       ev.Test,
		StartTime:  ev.Time,
		LaunchUuid: r.launchID,
		HasStats:   openapi.PtrBool(true),
		UniqueId:   openapi.PtrString(testID),
		CodeRef:    openapi.PtrString(testID),
		TestCaseId: openapi.PtrString(testID),
		Type:       string(gorppkg.TestItemTypes.Test),
		Retry:      openapi.PtrBool(false),
	})
	if err != nil {
		return err
	}
	r.tests[testID] = *rs.Id
	return nil
}

func (r *reporter) log(ev *testEvent) {
	if ev.Output == "" {
		return
	}
	testName := r.getTestName(ev)
	testUuid := r.tests[testName]

	// if output starts from tab
	if strings.HasPrefix(strings.TrimLeft(ev.Output, " "), "\t") && len(r.logs) > 0 {
		lastLog := r.logs[len(r.logs)-1]
		lastLog.Message = openapi.PtrString(*lastLog.Message + "\n" + ev.Output)
		lastLog.Level = openapi.PtrString(gorppkg.LogLevelError)
		return
	}

	rq := &openapi.SaveLogRQ{
		ItemUuid:   openapi.PtrString(testUuid),
		LaunchUuid: r.launchID,
		Level:      openapi.PtrString(gorppkg.LogLevelInfo),
		Time:       ev.Time,
		Message:    openapi.PtrString(ev.Output),
	}
	r.logs = append(r.logs, rq)
	r.flushLogs(false)
}

func (r *reporter) flushLogs(force bool) {
	if force || (len(r.logs) >= r.logsBatchSize) {
		batch := r.logs
		r.waitQueue.Add(1)
		go func(logs []*openapi.SaveLogRQ) {
			defer r.waitQueue.Done()

			if _, err := r.client.SaveLogs(logs...); err != nil {
				slog.Error("unable to report logs", "error", err, "batch_length", len(logs))
			}
		}(batch)
		r.logs = []*openapi.SaveLogRQ{}
	}
}

func (r *reporter) getTestName(ev *testEvent) string {
	return fmt.Sprintf("%s/%s", ev.Package, ev.Test)
}

func (r *reporter) startLaunch(startTime time.Time) error {
	var launch *openapi.EntryCreatedAsyncRS
	launch, err := r.client.StartLaunch(&openapi.StartLaunchRQ{
		Name:       r.launchName,
		StartTime:  startTime,
		Attributes: r.launchAttributes,
		Mode:       openapi.PtrString(string(gorppkg.LaunchModes.Default)),
	})
	if err != nil {
		return err
	}
	r.launchID = *launch.Id
	return err
}

func (r *reporter) finishLaunch(status gorppkg.Status, endTime time.Time) error {
	_, err := r.client.FinishLaunch(r.launchID, &openapi.FinishExecutionRQ{
		Status:  status.Ptr(),
		EndTime: endTime,
	})
	return err
}

func (r *reporter) finishTest(ev *testEvent, status gorppkg.Status) error {
	testName := r.getTestName(ev)
	testID := r.tests[testName]

	_, err := r.client.FinishTest(testID, &openapi.FinishTestItemRQ{
		EndTime:    ev.Time,
		Status:     status.Ptr(),
		LaunchUuid: r.launchID,
	})
	return err
}

func (r *reporter) finish(ev *testEvent, status gorppkg.Status) error {
	var err error
	if ev.Test == "" {
		err = r.finishSuite(ev, status)
	} else {
		err = r.finishTest(ev, status)
	}
	return err
}

func (r *reporter) finishSuite(ev *testEvent, status gorppkg.Status) error {
	suiteID, found := r.suites[ev.Package]
	if !found {
		return fmt.Errorf("unable to find suiteID for package: %s", ev.Package)
	}

	_, err := r.client.FinishTest(suiteID, &openapi.FinishTestItemRQ{
		EndTime:    ev.Time,
		Status:     status.Ptr(),
		LaunchUuid: r.launchID,
	})
	return err
}
