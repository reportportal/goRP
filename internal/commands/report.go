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
	"time"

	"github.com/urfave/cli/v3"
	"golang.org/x/sync/errgroup"

	gorppkg "github.com/reportportal/goRP/v5/pkg/gorp"
	"github.com/reportportal/goRP/v5/pkg/openapi"
)

const (
	logsBatchSize = 10
	// maxLogBatchesInFlight limits concurrent log upload requests.
	maxLogBatchesInFlight = 4

	// scannerMaxLine bounds a single test2json line; output events can be larger than bufio's 64 KB default.
	scannerMaxLine = 10 * 1024 * 1024
)

// test2json action values as defined by the Go testing/cmd/test2json package.
const (
	testActionStart  = "start"
	testActionRun    = "run"
	testActionOutput = "output"
	testActionPass   = "pass"
	testActionFail   = "fail"
)

// test2json OutputType values (Go 1.27+); see cmd/test2json documentation.
const (
	testOutputTypeFrame         = "frame"
	testOutputTypeError         = "error"
	testOutputTypeErrorContinue = "error-continue"
)

var (
	reportCommand = &cli.Command{
		Name:  "report",
		Usage: "Reports input to report portal",
		Commands: []*cli.Command{
			reportTest2JsonCommand,
			startLaunchCommand,
			startTestCommand,
			reportLogCommand,
			finishTestCommand,
			finishLaunchCommand,
		},
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
				Usage:   "Whether empty packages need to be reported. Default is false",
				Sources: cli.EnvVars("REPORT_EMPTY_PKG"),
				Value:   false,
			},
			&cli.StringSliceFlag{
				Name:    "attr",
				Aliases: []string{"a"},
				Usage:   "Launch attribute with format 'key:value'. Omitting a ':' separator will tag the launch with the value.",
			},
			&cli.BoolFlag{
				Name:  "print-launch-uuid",
				Usage: "Print launch UUID to console",
				Value: false,
			},
			&cli.BoolFlag{
				Name:    "quality-gate-check",
				Aliases: []string{"qgc"},
				Usage:   "Check quality gate status. Exits with exit code 10 if quality gate check fails.",
				Sources: cli.EnvVars("QUALITY_GATE_CHECK"),
				Value:   false,
			},
			argQualityGateTimeout,
			argQualityGateCheckInterval,
		},
		Action: reportTest2Json,
	}
)

func reportTest2Json(ctx context.Context, cmd *cli.Command) error {
	cfg, err := getConfig(cmd)
	if err != nil {
		return err
	}

	// start reporting of the launch
	launchID, err := reportLaunch(ctx, cfg, cmd)
	if err != nil {
		return err
	}
	// check if we need to print launch UUID
	if cmd.Bool("print-launch-uuid") && launchID != "" {
		_, _ = fmt.Fprintf(cmd.Writer, "ReportPortal Launch UUID: %s\n", launchID)
	}
	// check if we need to check quality gate
	if cmd.Bool("quality-gate-check") {
		if launchID == "" {
			return cli.Exit(
				"quality gate check skipped: no launch was started, the input has no test events",
				1,
			)
		}
		qgErr := checkQualityGateInternal(ctx, launchID, cfg, cmd)
		if qgErr != nil {
			return qgErr
		}
	}

	return nil
}

func reportLaunch(ctx context.Context, cfg *clientConfig, cmd *cli.Command) (string, error) {
	rpClient := buildReportingClient(ctx, cfg)

	input := make(chan *testEvent)

	launchNameArg := cmd.String("launchName")
	reportEmptyPkgArg := cmd.Bool("reportEmptyPkg")
	attrArgs := cmd.StringSlice("attr")
	rep := newReporter(ctx, input, rpClient, launchNameArg, reportEmptyPkgArg, attrArgs...)

	// start reporting in a separate goroutine.
	// recvErr is written before done is closed, so it is safe to read once done is closed.
	var recvErr error
	done := make(chan struct{})
	go func() {
		defer close(done)
		recvErr = rep.receive()
	}()

	readErr := feedEventsFrom(ctx, cmd, input, done)
	close(input)
	// wait for the reporter to flush logs and finish the launch
	<-done

	// reporter errors take precedence: they are the reason the input might have been abandoned
	if recvErr != nil {
		slog.Error("input processing interrupted", "error", recvErr)
		return "", recvErr
	}
	if readErr != nil {
		return "", readErr
	}
	return rep.launchUUID, nil
}

// feedEventsFrom feeds the reporter from the --file argument, or from stdin if it is not set.
func feedEventsFrom(
	ctx context.Context,
	cmd *cli.Command,
	input chan<- *testEvent,
	done <-chan struct{},
) error {
	if fileName := cmd.String("file"); fileName != "" {
		f, err := os.Open(filepath.Clean(fileName))
		if err != nil {
			return err
		}
		defer func() {
			if cErr := f.Close(); cErr != nil {
				slog.Error(cErr.Error())
			}
		}()
		return feedEvents(ctx, f, input, done)
	}

	return feedEvents(ctx, os.Stdin, input, done)
}

// feedEvents reads test2json events from r and sends them to the reporter.
// It stops early when the reporter has exited (done is closed) or ctx is cancelled.
func feedEvents(
	ctx context.Context,
	r io.Reader,
	input chan<- *testEvent,
	done <-chan struct{},
) error {
	events, scanErr := scanEvents(ctx, r)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-done:
			return nil
		case ev, ok := <-events:
			if !ok {
				return <-scanErr
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-done:
				return nil
			case input <- ev: // send event to reporter
			}
		}
	}
}

// scanEvents decodes test2json lines from r in a separate goroutine, so that a blocked read
// (stdin without data) doesn't prevent the caller from reacting to cancellation.
// The events channel is closed when r is exhausted or ctx is cancelled; the outcome is then
// available from the error channel. A goroutine blocked in Read is abandoned until r is closed
// or the process exits.
func scanEvents(ctx context.Context, r io.Reader) (<-chan *testEvent, <-chan error) {
	events := make(chan *testEvent)
	result := make(chan error, 1)

	go func() {
		var err error
		defer func() {
			result <- err
			close(events)
		}()

		scanner := bufio.NewScanner(r)
		scanner.Buffer(make([]byte, 0, bufio.MaxScanTokenSize), scannerMaxLine)
		for scanner.Scan() {
			var ev testEvent
			if jsonErr := json.Unmarshal(scanner.Bytes(), &ev); jsonErr != nil {
				// build tools may interleave plain text with the JSON stream
				slog.Warn("skipping non-JSON input line", "error", jsonErr)
				continue
			}
			select {
			case events <- &ev:
			case <-ctx.Done():
				return
			}
		}
		if scanErr := scanner.Err(); scanErr != nil {
			err = fmt.Errorf("reading input: %w", scanErr)
		}
	}()

	return events, result
}

// testEvent represents a golang test event string
// The Action field is one of a fixed set of action descriptions:
//
//	start  - the test binary is about to be executed
//	run    - the test has started running
//	pause  - the test has been paused
//	cont   - the test has continued running
//	pass   - the test passed
//	bench  - the benchmark printed log output but did not fail
//	fail   - the test or benchmark failed
//	output - the test printed output
//	skip   - the test was skipped or the package contained no tests
//
// OutputType is set on action == "output" in Go 1.27+ (blank, "frame", "error", "error-continue").
type testEvent struct {
	Time        time.Time `json:"time"` // encodes as an RFC3339-format string
	Action      string    `json:"action"`
	Package     string    `json:"package"`
	Test        string    `json:"test"`
	Elapsed     float64   `json:"elapsed"` // seconds
	Output      string    `json:"output"`
	OutputType  string    `json:"OutputType,omitempty"`
	FailedBuild string    `json:"FailedBuild,omitempty"`
}

type reporter struct {
	ctx              context.Context
	input            <-chan *testEvent
	client           *gorppkg.ReportingClient
	launchName       string
	launchUUID       string
	failed           bool
	launchAttributes []openapi.ItemAttributesRQ
	tests            map[string]string
	suites           map[string]string
	logs             []*openapi.SaveLogRQ
	logsBatchSize    int
	errGroup         errgroup.Group
	reportEmpty      bool
}

func newReporter(
	ctx context.Context,
	input <-chan *testEvent,
	client *gorppkg.ReportingClient,
	launchName string,
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

	r := &reporter{
		ctx:              ctx,
		input:            input,
		launchName:       launchName,
		launchAttributes: launchAttributes,
		client:           client,
		tests:            map[string]string{},
		suites:           map[string]string{},
		logs:             []*openapi.SaveLogRQ{},
		logsBatchSize:    logsBatchSize,
		reportEmpty:      reportEmpty,
	}
	r.errGroup.SetLimit(maxLogBatchesInFlight)

	return r
}

func (r *reporter) reportEvent(ev *testEvent) error {
	var err error
	switch ev.Action {
	case testActionStart:
		if r.reportEmpty {
			_, err = r.startSuite(ev)
		}
	case testActionRun:
		err = r.startTest(ev)
	case testActionOutput:
		r.log(ev)
	case testActionPass:
		err = r.finish(ev, gorppkg.Statuses.Passed)
	case testActionFail:
		err = r.finish(ev, gorppkg.Statuses.Failed)
	}
	return err
}

func (r *reporter) receive() error {
	prevEventTime := time.Now()
	for ev := range r.input {
		// start launch once
		// when first event comes
		if r.launchUUID == "" {
			if err := r.startLaunch(ev.Time); err != nil {
				slog.Error(err.Error())
				return err
			}
		}

		// report event to ReportPortal
		if err := r.reportEvent(ev); err != nil {
			r.interrupt(prevEventTime)
			return err
		}

		// remember last's event time
		// for RP's finishLaunch
		prevEventTime = ev.Time
	}

	// the input ended because we were cancelled, not because the test run is complete
	if r.ctx.Err() != nil {
		r.interrupt(prevEventTime)
		return r.ctx.Err()
	}

	// make sure we flush all logs that are left
	r.flushLogs(true)
	// wait for all log batch goroutines and surface any failures
	if err := r.errGroup.Wait(); err != nil {
		r.interrupt(prevEventTime)
		return fmt.Errorf("reporting logs: %w", err)
	}

	// finish launch if started
	if r.launchUUID != "" {
		status := gorppkg.Statuses.Passed
		if r.failed {
			status = gorppkg.Statuses.Failed
		}
		if err := r.finishLaunch(r.ctx, status, prevEventTime); err != nil {
			return err
		}
	}
	return nil
}

// interrupt makes a best-effort attempt to deliver buffered logs and to finish the launch as
// INTERRUPTED, so it does not stay IN_PROGRESS forever after a mid-stream failure.
func (r *reporter) interrupt(endTime time.Time) {
	if r.launchUUID == "" {
		return
	}
	// the original context may be the reason we are here
	ctx := context.WithoutCancel(r.ctx)

	r.flushLogs(true)
	if err := r.errGroup.Wait(); err != nil {
		slog.Warn("unable to report remaining logs", "error", err)
	}
	if err := r.finishLaunch(ctx, gorppkg.Statuses.Interrupted, endTime); err != nil {
		slog.Warn("unable to finish interrupted launch", "error", err)
	}
}

func (r *reporter) startSuite(ev *testEvent) (string, error) {
	rs, err := r.client.StartTest(r.ctx, &openapi.StartTestItemRQ{
		Name:       ev.Package,
		StartTime:  ev.Time,
		LaunchUuid: r.launchUUID,
		HasStats:   openapi.PtrBool(true),
		Type:       string(gorppkg.TestItemTypes.Suite),
		Retry:      openapi.PtrBool(false),
	})
	if err != nil {
		return "", err
	}
	id, err := entryID(rs)
	if err != nil {
		return "", err
	}
	r.suites[ev.Package] = id
	return id, nil
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
	rs, err := r.client.StartChildTest(r.ctx, suiteID, &openapi.StartTestItemRQ{
		Name:       ev.Test,
		StartTime:  ev.Time,
		LaunchUuid: r.launchUUID,
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
	id, err := entryID(rs)
	if err != nil {
		return err
	}
	r.tests[testID] = id
	return nil
}

func logLevelForOutputType(outputType string) string {
	switch outputType {
	case testOutputTypeError, testOutputTypeErrorContinue:
		return gorppkg.LogLevelError
	default:
		return gorppkg.LogLevelInfo
	}
}

func shouldMergeOutput(ev *testEvent) bool {
	if ev.OutputType == testOutputTypeErrorContinue {
		return true
	}
	// Legacy fallback for Go < 1.27 streams without OutputType.
	if ev.OutputType == "" {
		return strings.HasPrefix(strings.TrimLeft(ev.Output, " "), "\t")
	}
	return false
}

func (r *reporter) log(ev *testEvent) {
	if ev.Output == "" {
		return
	}
	if shouldMergeOutput(ev) && len(r.logs) > 0 {
		lastLog := r.logs[len(r.logs)-1]
		lastLog.Message = openapi.PtrString(lastLog.GetMessage() + "\n" + ev.Output)
		lastLog.Level = openapi.PtrString(gorppkg.LogLevelError)
		return
	}

	rq := &openapi.SaveLogRQ{
		LaunchUuid: r.launchUUID,
		Level:      openapi.PtrString(logLevelForOutputType(ev.OutputType)),
		Time:       ev.Time,
		Message:    openapi.PtrString(ev.Output),
	}
	if itemUUID := r.logItemUUID(ev); itemUUID != "" {
		rq.ItemUuid = openapi.PtrString(itemUUID)
	}
	r.logs = append(r.logs, rq)
	r.flushLogs(false)
}

// logItemUUID returns the test the event belongs to, or the package's suite for package-level output.
// An empty result attaches the log to the launch itself.
func (r *reporter) logItemUUID(ev *testEvent) string {
	if ev.Test == "" {
		return r.suites[ev.Package]
	}
	return r.tests[r.getTestName(ev)]
}

func (r *reporter) flushLogs(force bool) {
	if len(r.logs) == 0 {
		return
	}
	if force || (len(r.logs) >= r.logsBatchSize) {
		batch := r.logs
		r.errGroup.Go(func() error {
			if _, err := r.client.SaveLogs(r.ctx, batch...); err != nil {
				slog.Error("unable to report logs", "error", err, "batch_length", len(batch))
				return err
			}
			return nil
		})
		r.logs = []*openapi.SaveLogRQ{}
	}
}

func (r *reporter) getTestName(ev *testEvent) string {
	return fmt.Sprintf("%s/%s", ev.Package, ev.Test)
}

func (r *reporter) startLaunch(startTime time.Time) error {
	launch, err := r.client.StartLaunch(r.ctx, &openapi.StartLaunchRQ{
		Name:       r.launchName,
		StartTime:  startTime,
		Attributes: r.launchAttributes,
		Mode:       openapi.PtrString(string(gorppkg.LaunchModes.Default)),
	})
	if err != nil {
		return err
	}
	id, err := entryID(launch)
	if err != nil {
		return err
	}
	r.launchUUID = id
	return nil
}

func (r *reporter) finishLaunch(
	ctx context.Context,
	status gorppkg.Status,
	endTime time.Time,
) error {
	_, err := r.client.FinishLaunch(ctx, r.launchUUID, &openapi.FinishExecutionRQ{
		Status:  status.Ptr(),
		EndTime: endTime,
	})
	return err
}

func (r *reporter) finishTest(ev *testEvent, status gorppkg.Status) error {
	testName := r.getTestName(ev)
	testID := r.tests[testName]

	_, err := r.client.FinishTest(r.ctx, testID, &openapi.FinishTestItemRQ{
		EndTime:    ev.Time,
		Status:     status.Ptr(),
		LaunchUuid: r.launchUUID,
	})
	return err
}

func (r *reporter) finish(ev *testEvent, status gorppkg.Status) error {
	if status == gorppkg.Statuses.Failed {
		r.failed = true
	}
	if ev.Test == "" {
		return r.finishSuite(ev, status)
	}
	return r.finishTest(ev, status)
}

func (r *reporter) finishSuite(ev *testEvent, status gorppkg.Status) error {
	suiteID, found := r.suites[ev.Package]
	if !found {
		if !r.reportEmpty {
			// the package had no tests, so no suite was started and there is nothing to finish
			return nil
		}
		return fmt.Errorf("unable to find suiteID for package: %s", ev.Package)
	}

	_, err := r.client.FinishTest(r.ctx, suiteID, &openapi.FinishTestItemRQ{
		EndTime:    ev.Time,
		Status:     status.Ptr(),
		LaunchUuid: r.launchUUID,
	})
	return err
}
