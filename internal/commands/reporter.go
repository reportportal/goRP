package commands

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"golang.org/x/sync/errgroup"

	gorppkg "github.com/reportportal/goRP/v5/pkg/gorp"
	"github.com/reportportal/goRP/v5/pkg/openapi"
)

const (
	logsBatchSize = 10
	// maxLogBatchesInFlight limits concurrent log upload requests.
	maxLogBatchesInFlight = 4
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
	r := &reporter{
		ctx:              ctx,
		input:            input,
		launchName:       launchName,
		launchAttributes: parseAttributes(launchAttrArgs),
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

	// the input ended because we were canceled, not because the test run is complete
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
