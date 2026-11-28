package commands

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"time"

	gorppkg "github.com/reportportal/goRP/v5/pkg/gorp"
	"github.com/reportportal/goRP/v5/pkg/openapi"
)

// The log batching of the reporter, set explicitly so that the load on the server does not change with
// the defaults of the library.
const (
	logsBatchSize = 10
	// maxLogBatchesInFlight limits concurrent log upload requests.
	maxLogBatchesInFlight = 4
	// pendingLogMaxAge is how long a log may wait for output that continues it. Continuation lines are
	// printed together with their first line, so this only matters when the test run goes silent.
	pendingLogMaxAge = time.Second
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

// Build actions of `go test -json` (Go 1.24+): the output of a failed build and its end, see `go help
// buildjson`. These events come from the go command, not from test2json: they carry an ImportPath
// instead of a Package and have no Time.
const (
	buildActionOutput = "build-output"
	buildActionFail   = "build-fail"
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
//
// Go 1.24+ also puts the build events of the go command into the stream (see buildActionOutput). They
// set ImportPath, Action and Output only. The "fail" event of a package that could not be built names
// the ImportPath of the build that failed in FailedBuild.
type testEvent struct {
	Time        time.Time `json:"time"` // encodes as an RFC3339-format string
	Action      string    `json:"action"`
	Package     string    `json:"package"`
	Test        string    `json:"test"`
	Elapsed     float64   `json:"elapsed"` // seconds
	Output      string    `json:"output"`
	OutputType  string    `json:"OutputType,omitempty"`
	FailedBuild string    `json:"FailedBuild,omitempty"`
	ImportPath  string    `json:"ImportPath,omitempty"`
}

// isBuildEvent reports whether the event is a build event of the go command. Such an event has no
// time, so it can neither start the launch nor move the time the launch is finished with.
func (ev *testEvent) isBuildEvent() bool {
	return ev.Action == buildActionOutput || ev.Action == buildActionFail
}

// launchStatusInProgress is the status of a launch that is not finished.
const launchStatusInProgress = "IN_PROGRESS"

// errEventsBeforeLaunch reports test events that are older than the launch they should be reported into.
var errEventsBeforeLaunch = errors.New(
	"the test run is older than the launch: ReportPortal silently drops items that start before " +
		"their launch. Start the launch before the tests run",
)

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
	paused map[string]bool
	// parents maps a started test to its parent test ("" when its parent is the package suite)
	parents map[string]string
	suites  map[string]string
	// logs sends the logs in batches: by size and, for long-running tests, by time
	logs *gorppkg.LogBatcher
	// pendingLog is the latest log while output that continues it may still follow (see shouldMergeOutput).
	// The batcher owns what it was given and may send it at any time, so such a log is held back here.
	pendingLog  *openapi.SaveLogRQ
	reportEmpty bool
	// skippedIssue keeps skipped tests "To Investigate" (true, ReportPortal's default) or marks them "No Defect"
	skippedIssue bool
	// launchLink is the UI link of the finished launch, as returned by ReportPortal
	launchLink  string
	description string
	mode        gorppkg.LaunchMode
	// rerun reports into an existing launch: the latest one with the same name, or rerunOf if set
	rerun   bool
	rerunOf string
	// joined is the existing launch the results are reported into (--launch-uuid). It belongs to
	// somebody else: the reporter neither starts nor finishes it. Nil when the reporter owns the launch.
	joined *gorppkg.LaunchInfo
	// started is set once the first event has been seen
	started bool
	// parameters are the parameters of the started tests: those of their own name ("size=64") and
	// those of the tests they are subtests of
	parameters map[string][]testParameter
	// retries reports a test that runs again as a retry of its previous run, see startTest
	retries bool
	// containers are the tests that had subtests in one of their runs
	containers map[string]bool
	// failedTests and failedPackages tell whether the latest run of a test or a package failed
	failedTests    map[testKey]bool
	failedPackages map[string]bool
	// buildOutput is the compiler (or vet) output of the builds, by the ImportPath of the build. It is
	// reported when a package fails because of that build; several packages can fail because of one.
	buildOutput map[string][]string
}

// reporterOptions are the optional settings of the test2json reporter.
type reporterOptions struct {
	reportEmpty  bool
	skippedIssue bool
	agentVersion string
	attributes   []string
	description  string
	// mode is the launch mode; empty means DEFAULT
	mode gorppkg.LaunchMode
	// rerun reports into the latest launch with the same name; rerunOf names the launch and implies rerun
	rerun   bool
	rerunOf string
	// joined is the existing launch to report into; nil to start (and finish) a launch
	joined *gorppkg.LaunchInfo
	// retries reports repeated runs of a test as retries
	retries bool
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
		parents:          map[string]string{},
		suites:           map[string]string{},
		buildOutput:      map[string][]string{},
		retries:          opts.retries,
		containers:       map[string]bool{},
		parameters:       map[string][]testParameter{},
		failedTests:      map[testKey]bool{},
		failedPackages:   map[string]bool{},
		logs: gorppkg.NewLogBatcher(ctx, client, gorppkg.LogBatcherOptions{
			MaxEntries:  logsBatchSize,
			MaxInFlight: maxLogBatchesInFlight,
		}),
		reportEmpty:  opts.reportEmpty,
		skippedIssue: opts.skippedIssue,
		description:  opts.description,
		mode:         opts.mode,
		rerun:        opts.rerun || opts.rerunOf != "",
		rerunOf:      opts.rerunOf,
	}
	if r.mode == "" {
		r.mode = gorppkg.LaunchModes.Default
	}
	if opts.joined != nil {
		r.joined = opts.joined
		r.launchUUID = opts.joined.UUID
	}
	return r
}

func (r *reporter) reportEvent(ev *testEvent) error {
	// output that continues a log follows it directly
	if ev.Action != testActionOutput {
		r.releasePendingLog()
	}
	var err error
	switch ev.Action {
	case testActionStart:
		if r.reportEmpty {
			err = r.startPackage(ev)
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
	// stops the flush timer of the batcher on every way out; closing twice is fine
	defer func() { _ = r.logs.Close() }()

	held := time.NewTicker(pendingLogMaxAge)
	defer held.Stop()

	prevEventTime := time.Now()
	for ev, ok := r.nextEvent(held.C); ok; ev, ok = r.nextEvent(held.C) {
		// build events have no time and no package: they are kept until a package fails because of them
		if ev.isBuildEvent() {
			if ev.Action == buildActionOutput {
				r.buildOutput[ev.ImportPath] = append(r.buildOutput[ev.ImportPath], ev.Output)
			}
			continue
		}

		// start launch once
		// when first event comes
		if !r.started {
			if err := r.begin(ev.Time); err != nil {
				slog.Error(err.Error())
				return err
			}
			r.started = true
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

	// send the logs that are left, wait for all log batches and surface any failures
	if err := r.finishLogs(); err != nil {
		r.interrupt(prevEventTime)
		return fmt.Errorf("reporting logs: %w", err)
	}

	// finish launch if started; a joined launch is finished by whoever started it
	if r.started && r.joined == nil {
		status := gorppkg.Statuses.Passed
		if r.launchFailed() {
			status = gorppkg.Statuses.Failed
		}
		if err := r.finishLaunch(r.ctx, status, prevEventTime); err != nil {
			return err
		}
	}
	return nil
}

// nextEvent waits for the next event; ok is false when the input has ended. While the input is silent
// (a test that runs long, or hangs), the log that is held back for its continuation is released on
// every tick of release, so that the last line a test printed does not stay here.
func (r *reporter) nextEvent(release <-chan time.Time) (*testEvent, bool) {
	for {
		select {
		case ev, ok := <-r.input:
			return ev, ok
		case <-release:
			r.releasePendingLog()
		}
	}
}

// interrupt makes a best-effort attempt to deliver buffered logs and to finish the launch as
// INTERRUPTED, so it does not stay IN_PROGRESS forever after a mid-stream failure.
func (r *reporter) interrupt(endTime time.Time) {
	if !r.started {
		return
	}
	// the original context may be the reason we are here
	ctx := context.WithoutCancel(r.ctx)

	if err := r.finishLogs(); err != nil {
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
	// a joined launch is not ours to finish: other reporters may still be writing to it
	if r.joined != nil {
		return
	}
	if err := r.finishLaunch(ctx, gorppkg.Statuses.Interrupted, endTime); err != nil {
		slog.Warn("unable to finish interrupted launch", "error", err)
	}
}

// begin starts the launch when the first event arrives, or checks that the launch to join can take it.
func (r *reporter) begin(startTime time.Time) error {
	if r.joined == nil {
		return r.startLaunch(startTime)
	}
	// ReportPortal accepts an item that starts before its launch and then drops it without telling
	// the client, so this is the only place where the user can learn about it.
	if startTime.Before(r.joined.StartTime) {
		return fmt.Errorf(
			"%w: the first event is at %s, launch %s starts at %s",
			errEventsBeforeLaunch,
			startTime.Format(time.RFC3339Nano),
			r.joined.UUID,
			r.joined.StartTime.Format(time.RFC3339Nano),
		)
	}
	return nil
}

// launchFailed decides the status of the launch. A failure that a later run of the same test or
// package has turned into a success (gotestsum --rerun-fails) does not fail the launch, as it does
// not fail the test run. Without retries every run is reported on its own, and any failure counts.
func (r *reporter) launchFailed() bool {
	if !r.retries {
		return r.failed
	}
	for _, failed := range r.failedTests {
		if failed {
			return true
		}
	}
	for _, failed := range r.failedPackages {
		if failed {
			return true
		}
	}
	return false
}

// testKey names a test by its package and its name in the package. The two are kept apart: an import
// path can be the prefix of another ("m/a" and "m/a/b"), so the tests of a package cannot be told from
// the joined name.
type testKey struct {
	pkg, test string
}

// hasFailedTest reports whether the latest run of a test of the package failed. With a parent, only
// its subtests count.
func (r *reporter) hasFailedTest(pkg, parent string) bool {
	for key, failed := range r.failedTests {
		if failed && key.pkg == pkg && (parent == "" || strings.HasPrefix(key.test, parent+"/")) {
			return true
		}
	}
	return false
}

// reopen makes a finished suite or test current again: a package or a test with subtests that runs
// once more is not reported as a new item, its new results go under the item it has already (see
// startTest). The item is finished again when the run ends.
func (r *reporter) reopen(id string) {
	if !slices.Contains(r.openItems, id) {
		r.openItems = append(r.openItems, id)
	}
}

// startPackage handles the "start" event of a package when empty packages are reported.
func (r *reporter) startPackage(ev *testEvent) error {
	// gotestsum --rerun-fails runs the package again for every test it reruns
	if id, found := r.suites[ev.Package]; found && r.retries {
		r.reopen(id)
		return nil
	}
	_, err := r.startSuite(ev)
	return err
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
//
// A test can run more than once in one stream: `go test -count=N` runs every test N times, and
// `gotestsum --rerun-fails` runs the failed tests again and appends those runs. With retries enabled a
// repeated run is reported as a ReportPortal retry, so the test case is still counted once, with the
// result of its last run, and the earlier runs are kept as its retries:
//
//   - A test without subtests is started again with retry=true and retryOf = the UUID of its previous
//     run. retryOf must name the latest run: the server ignores a retryOf that points to an item which
//     is a retry itself, and the new run then becomes one more counted item.
//   - A test that had subtests is not started again. It is a container that is not counted itself;
//     its item is reused, so that the subtests which run again find their previous runs under the
//     same parent and become retries, and the subtests which do not run again (gotestsum reruns only
//     the failed ones) stay counted. The same goes for the suite of the package.
func (r *reporter) startTest(ev *testEvent) error {
	testID := r.getTestName(ev)
	parentTest, name := r.parentOf(ev)
	parentID := r.tests[parentTest]
	if parentTest == "" {
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
		} else if r.retries {
			// the package runs again
			r.reopen(suiteID)
		}
		parentID = suiteID
	} else {
		r.containers[parentTest] = true
	}
	// A subtest named "key=value" is a case of a table test: the pair is reported as a parameter. A
	// subtest runs with the parameters of its parents too. The test case ID stays the full name.
	parameters := slices.Concat(r.parameters[parentTest], subtestParameters(name))
	if parentTest == "" {
		parameters = nil // the name of a top-level test is a function name
	}
	r.parameters[testID] = parameters
	rq := &openapi.StartTestItemRQ{
		Name:       name,
		StartTime:  ev.Time,
		LaunchUuid: r.launchUUID,
		HasStats:   openapi.PtrBool(true),
		UniqueId:   openapi.PtrString(testID),
		CodeRef:    openapi.PtrString(testID),
		TestCaseId: openapi.PtrString(testID),
		Type:       string(gorppkg.TestItemTypes.Step),
		Retry:      openapi.PtrBool(false),
		Parameters: parameters,
	}
	// a previous run that is still open is not a previous run: the name is not unique in the stream
	if previous, seen := r.tests[testID]; seen && r.retries &&
		!slices.Contains(r.openItems, previous) {
		if r.containers[testID] {
			r.reopen(previous)
			return nil
		}
		rq.Retry = openapi.PtrBool(true)
		rq.RetryOf = openapi.PtrString(previous)
	}
	rs, err := r.client.StartChildTest(r.ctx, parentID, rq)
	if err != nil {
		return err
	}
	id, err := entryID(rs)
	if err != nil {
		return err
	}
	r.tests[testID] = id
	r.parents[testID] = parentTest
	r.openItems = append(r.openItems, id)
	return nil
}

// parentOf finds the running test that ev is a subtest of and returns its name (package-qualified, as
// in r.tests) together with the name of ev relative to it ("TestA/b/c" under "TestA/b" is "c"). For a
// top-level test, or when no running parent is found, the parent is empty and the name is the full
// test name.
//
// Subtest names may contain "/" themselves, so every running test whose name is a prefix of ev's is a
// candidate. A test that already finished or is paused in t.Parallel() cannot be the parent:
// t.Run("c/d") after a finished t.Run("c") is a sibling of "c". Neither can a test that has a running
// subtest: it is blocked in t.Run, or its body has returned and its parallel subtests run. Of the
// remaining candidates the longest wins.
//
// test2json cannot always tell more: with parallel siblings "a" and "a/b" both running, "a/b/c" is
// either t.Run("b/c") of "a" or t.Run("c") of "a/b", and the stream is the same for both. The
// longest prefix picks "a/b". When that is wrong, the test is shown under its sibling, but every
// test case is still counted exactly once; any other choice would count one test case too many.
func (r *reporter) parentOf(ev *testEvent) (string, string) {
	var candidates []int // indexes of "/" in ev.Test that end the name of a candidate, longest first
	for i := strings.LastIndex(ev.Test, "/"); i > 0; i = strings.LastIndex(ev.Test[:i], "/") {
		if r.isRunning(ev.Package + "/" + ev.Test[:i]) {
			candidates = append(candidates, i)
		}
	}
	if len(candidates) == 0 {
		return "", ev.Test
	}
	parent := candidates[0]
	for _, i := range candidates {
		if !r.hasRunningSubtest(ev.Package + "/" + ev.Test[:i]) {
			parent = i
			break
		}
	}
	// when every candidate has a running subtest (t.Run called from a goroutine), the longest one stays
	return ev.Package + "/" + ev.Test[:parent], ev.Test[parent+1:]
}

// isRunning reports whether the test is started, not finished and not paused in t.Parallel().
func (r *reporter) isRunning(test string) bool {
	id, ok := r.tests[test]
	return ok && !r.paused[test] && slices.Contains(r.openItems, id)
}

// hasRunningSubtest reports whether one of the test's direct subtests is running.
func (r *reporter) hasRunningSubtest(test string) bool {
	for child, parent := range r.parents {
		if parent == test && r.isRunning(child) {
			return true
		}
	}
	return false
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
	if shouldMergeOutput(ev) && r.pendingLog != nil {
		lastLog := r.pendingLog
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
	r.releasePendingLog()
	if canBeContinued(ev) {
		r.pendingLog = rq
		return
	}
	r.addLog(rq)
}

// canBeContinued reports whether the output that follows ev may belong to the same log entry: the
// lines of one t.Error call (Go 1.27+), or any line of a stream without OutputType, where a
// tab-indented line continues the previous one.
func canBeContinued(ev *testEvent) bool {
	switch ev.OutputType {
	case testOutputTypeError, testOutputTypeErrorContinue, "":
		return true
	default:
		return false
	}
}

// releasePendingLog hands the log that was held back for its continuation over to the batcher.
func (r *reporter) releasePendingLog() {
	if r.pendingLog != nil {
		r.addLog(r.pendingLog)
		r.pendingLog = nil
	}
}

// addLog queues a log. The batcher sends it in the background; a failure surfaces in finishLogs.
func (r *reporter) addLog(rq *openapi.SaveLogRQ) {
	if err := r.logs.Add(rq); err != nil {
		slog.Error("unable to report log", "error", err)
	}
}

// finishLogs sends the logs that are left and waits for all log batches. It returns the first
// failure: a failed request, or logs the server answered but did not accept.
func (r *reporter) finishLogs() error {
	r.releasePendingLog()
	return r.logs.Close()
}

// logBuildFailure reports the output of the failed build as one ERROR log of the package's suite. The
// build events have no time, so the log gets the time of the package's "fail" event.
func (r *reporter) logBuildFailure(ev *testEvent, suiteID string) {
	output := r.buildOutput[ev.FailedBuild]
	if len(output) == 0 {
		// the stream has no build output: `go test -json` before Go 1.24 printed it to stderr
		slog.Warn(
			"no build output for the failed package",
			"package",
			ev.Package,
			"build",
			ev.FailedBuild,
		)
		return
	}
	r.addLog(&openapi.SaveLogRQ{
		LaunchUuid: r.launchUUID,
		ItemUuid:   openapi.PtrString(suiteID),
		Level:      openapi.PtrString(gorppkg.LogLevelError),
		Time:       ev.Time,
		Message:    openapi.PtrString(strings.Join(output, "")),
	})
}

// logItemUUID returns the test the event belongs to, or the package's suite for package-level output.
// An empty result attaches the log to the launch itself.
func (r *reporter) logItemUUID(ev *testEvent) string {
	if ev.Test == "" {
		return r.suites[ev.Package]
	}
	return r.tests[r.getTestName(ev)]
}

func (r *reporter) getTestName(ev *testEvent) string {
	return fmt.Sprintf("%s/%s", ev.Package, ev.Test)
}

func (r *reporter) startLaunch(startTime time.Time) error {
	rq := &openapi.StartLaunchRQ{
		Name:       r.launchName,
		StartTime:  startTime,
		Attributes: r.launchAttributes,
		Mode:       openapi.PtrString(string(r.mode)),
	}
	if r.description != "" {
		rq.Description = openapi.PtrString(r.description)
	}
	if r.rerun {
		rq.Rerun = openapi.PtrBool(true)
		if r.rerunOf != "" {
			rq.RerunOf = openapi.PtrString(r.rerunOf)
		}
	}
	launch, err := r.client.StartLaunch(r.ctx, rq)
	if err != nil {
		return err
	}
	// The id of the response is used for everything that follows, never rerunOf: for a rerun the
	// server answers with the UUID of the launch that is being rerun (ReportPortal 5.13.5+).
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
	r.failedTests[testKey{ev.Package, ev.Test}] = status == gorppkg.Statuses.Failed
	status = r.containerStatus(status, ev)

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

// containerStatus is the status to finish a suite or a test with. A package or a test with subtests
// that runs again runs only a part of its tests (gotestsum --rerun-fails) and can pass although a test
// of an earlier run, which did not run again, has failed. Its item holds the tests of all runs, so it
// stays FAILED then.
func (r *reporter) containerStatus(status gorppkg.Status, ev *testEvent) gorppkg.Status {
	if r.retries && status == gorppkg.Statuses.Passed && r.hasFailedTest(ev.Package, ev.Test) {
		return gorppkg.Statuses.Failed
	}
	return status
}

func (r *reporter) finishSuite(ev *testEvent, status gorppkg.Status) error {
	r.failedPackages[ev.Package] = status == gorppkg.Statuses.Failed
	status = r.containerStatus(status, ev)
	suiteID, found := r.suites[ev.Package]
	if ev.FailedBuild != "" {
		// The package was not built, so no test has run and, unless empty packages are reported, there
		// is no suite yet. The failure must not get lost: the suite is reported with the build output.
		if !found {
			var err error
			if suiteID, err = r.startSuite(ev); err != nil {
				return err
			}
		}
		r.logBuildFailure(ev, suiteID)

		return r.finishItem(r.ctx, suiteID, ev.Time, status)
	}
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
