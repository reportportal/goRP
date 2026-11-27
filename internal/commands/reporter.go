package commands

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
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
	testActionSkip   = "skip"
	testActionPause  = "pause"
	testActionCont   = "cont"
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
	ctx        context.Context
	input      <-chan *testEvent
	client     *gorppkg.ReportingClient
	launchName string
	launchUUID string
	failed     bool
	// openItems are the started, not yet finished items (suites and tests) in start order
	openItems        []string
	launchAttributes []openapi.ItemAttributesRQ
	tests            map[string]string
	// paused are the tests that called t.Parallel() and wait for their turn; they cannot have subtests
	// started while paused, so a test named like them ("a/b" next to a paused "a") is not their child
	paused        map[string]bool
	suites        map[string]string
	logs          []*openapi.SaveLogRQ
	logsBatchSize int
	errGroup      errgroup.Group
	reportEmpty   bool
	// skippedIssue keeps skipped tests "To Investigate" (true, ReportPortal's default) or marks them "No Defect"
	skippedIssue bool
	// launchLink is the UI link of the finished launch, as returned by ReportPortal
	launchLink string
}

// reporterOptions are the optional settings of the test2json reporter.
type reporterOptions struct {
	reportEmpty  bool
	skippedIssue bool
	agentVersion string
	attributes   []string
}

func newReporter(
	ctx context.Context,
	input <-chan *testEvent,
	client *gorppkg.ReportingClient,
	launchName string,
	opts reporterOptions,
) *reporter {
	attributes := parseAttributes(opts.attributes)
	attributes = append(attributes, gorppkg.SystemAttributes(agentName, opts.agentVersion)...)
	attributes = append(attributes, gorppkg.SkippedIssueAttribute(opts.skippedIssue))
	r := &reporter{
		ctx:              ctx,
		input:            input,
		launchName:       launchName,
		launchAttributes: attributes,
		client:           client,
		tests:            map[string]string{},
		paused:           map[string]bool{},
		suites:           map[string]string{},
		logs:             []*openapi.SaveLogRQ{},
		logsBatchSize:    logsBatchSize,
		reportEmpty:      opts.reportEmpty,
		skippedIssue:     opts.skippedIssue,
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
	case testActionSkip:
		err = r.finish(ev, gorppkg.Statuses.Skipped)
	case testActionPause:
		r.paused[r.getTestName(ev)] = true
	case testActionCont:
		delete(r.paused, r.getTestName(ev))
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
	// ReportPortal calculates the status of a launch that still has running items on its own (FAILED),
	// so finish them first, innermost first, to keep the INTERRUPTED status of the launch.
	for len(r.openItems) > 0 {
		id := r.openItems[len(r.openItems)-1]
		if err := r.finishItem(ctx, id, endTime, gorppkg.Statuses.Interrupted); err != nil {
			slog.Warn("unable to finish interrupted item", "item", id, "error", err)
		}
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
	r.openItems = append(r.openItems, id)
	return id, nil
}

// startTest starts a test as a STEP: ReportPortal counts executions (total, passed, failed, skipped)
// only for STEP items without children, and only those get "To Investigate". A subtest is started
// as a child of its parent test, so the parent stops being counted once it has subtests and every
// test case is counted exactly once.
func (r *reporter) startTest(ev *testEvent) error {
	testID := r.getTestName(ev)
	parentID, name := r.parentOf(ev)
	if parentID == "" {
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
		parentID = suiteID
	}
	rs, err := r.client.StartChildTest(r.ctx, parentID, &openapi.StartTestItemRQ{
		Name:       name,
		StartTime:  ev.Time,
		LaunchUuid: r.launchUUID,
		HasStats:   openapi.PtrBool(true),
		UniqueId:   openapi.PtrString(testID),
		CodeRef:    openapi.PtrString(testID),
		TestCaseId: openapi.PtrString(testID),
		Type:       string(gorppkg.TestItemTypes.Step),
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
	r.openItems = append(r.openItems, id)
	return nil
}

// parentOf finds the running test that ev is a subtest of and returns its item ID together with the
// name of ev relative to it ("TestA/b/c" under "TestA/b" is "c"). Subtest names may contain "/"
// themselves, so the longest prefix that names a running test wins. A test that already finished or
// is paused in t.Parallel() cannot be the parent: t.Run("c/d") after a finished t.Run("c") is a
// sibling of "c". For a top-level test, or when no running parent is found, the ID is empty and the
// name is the full test name.
func (r *reporter) parentOf(ev *testEvent) (string, string) {
	for i := strings.LastIndex(ev.Test, "/"); i > 0; i = strings.LastIndex(ev.Test[:i], "/") {
		name := ev.Package + "/" + ev.Test[:i]
		id, ok := r.tests[name]
		if ok && !r.paused[name] && slices.Contains(r.openItems, id) {
			return id, ev.Test[i+1:]
		}
	}
	return "", ev.Test
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
	rs, err := r.client.FinishLaunch(ctx, r.launchUUID, &openapi.FinishExecutionRQ{
		Status:  status.Ptr(),
		EndTime: endTime,
	})
	if err == nil && rs != nil {
		r.launchLink = rs.GetLink()
	}
	return err
}

func (r *reporter) finishTest(ev *testEvent, status gorppkg.Status) error {
	testName := r.getTestName(ev)
	testID := r.tests[testName]

	rq := &openapi.FinishTestItemRQ{
		EndTime:    ev.Time,
		Status:     status.Ptr(),
		LaunchUuid: r.launchUUID,
	}
	if status == gorppkg.Statuses.Skipped && !r.skippedIssue {
		rq.Issue = gorppkg.NotIssue()
	}
	return r.finishItemRQ(r.ctx, testID, rq)
}

// finishItem finishes a suite or a test. The item is not open anymore even if the request fails:
// it must not be retried as interrupted.
func (r *reporter) finishItem(
	ctx context.Context,
	id string,
	endTime time.Time,
	status gorppkg.Status,
) error {
	return r.finishItemRQ(ctx, id, &openapi.FinishTestItemRQ{
		EndTime:    endTime,
		Status:     status.Ptr(),
		LaunchUuid: r.launchUUID,
	})
}

func (r *reporter) finishItemRQ(
	ctx context.Context,
	id string,
	rq *openapi.FinishTestItemRQ,
) error {
	r.openItems = slices.DeleteFunc(r.openItems, func(open string) bool { return open == id })

	_, err := r.client.FinishTest(ctx, id, rq)

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
		// the package had no tests (or, for "skip", no test files), so no suite was started and
		// there is nothing to finish
		if !r.reportEmpty || status == gorppkg.Statuses.Skipped {
			return nil
		}
		return fmt.Errorf("unable to find suiteID for package: %s", ev.Package)
	}

	return r.finishItem(r.ctx, suiteID, ev.Time, status)
}
