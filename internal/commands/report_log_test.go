package commands

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	gorppkg "github.com/reportportal/goRP/v5/pkg/gorp"
)

// newTestReporter is a reporter with a started launch and test; the logs it sends are recorded by the
// returned server.
func newTestReporter(t *testing.T) (*reporter, *hierarchyServer) {
	t.Helper()
	rec, srv := newHierarchyServer(t, "proj")
	cfg := &clientConfig{URL: srv.URL, Project: "proj", ApiKey: "tok"}
	rep := newReporter(context.Background(), nil, buildReportingClient(t.Context(), cfg), "launch",
		reporterOptions{})
	t.Cleanup(func() { _ = rep.logs.Close() })
	rep.launchUUID = "launch-1"
	rep.tests["pkg/TestOne"] = "item-1"
	return rep, rec
}

// sentLogs finishes the logs of the reporter and returns what the server got.
func sentLogs(t *testing.T, rep *reporter, rec *hierarchyServer) []map[string]any {
	t.Helper()
	require.NoError(t, rep.finishLogs())
	rec.mu.Lock()
	defer rec.mu.Unlock()
	return rec.logs
}

func outputEvent(output, outputType string) *testEvent {
	return &testEvent{
		Time:       time.Now(),
		Action:     testActionOutput,
		Package:    "pkg",
		Test:       "TestOne",
		Output:     output,
		OutputType: outputType,
	}
}

func TestLogLevelForOutputType(t *testing.T) {
	t.Parallel()

	assert.Equal(t, gorppkg.LogLevelInfo, logLevelForOutputType(""))
	assert.Equal(t, gorppkg.LogLevelInfo, logLevelForOutputType(testOutputTypeFrame))
	assert.Equal(t, gorppkg.LogLevelError, logLevelForOutputType(testOutputTypeError))
	assert.Equal(t, gorppkg.LogLevelError, logLevelForOutputType(testOutputTypeErrorContinue))
	assert.Equal(t, gorppkg.LogLevelInfo, logLevelForOutputType("future-value"))
}

func TestShouldMergeOutput(t *testing.T) {
	t.Parallel()

	assert.True(t, shouldMergeOutput(&testEvent{OutputType: testOutputTypeErrorContinue}))
	assert.True(t, shouldMergeOutput(&testEvent{Output: "\tcontinuation"}))
	assert.False(
		t,
		shouldMergeOutput(&testEvent{OutputType: testOutputTypeFrame, Output: "\tframing"}),
	)
	assert.False(t, shouldMergeOutput(&testEvent{OutputType: testOutputTypeError, Output: "err"}))
	assert.False(t, shouldMergeOutput(&testEvent{Output: "plain line"}))
}

func TestReporterLog_OutputTypes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		events    []*testEvent
		wantCount int
		wantLevel string
		wantMsg   string
	}{
		{
			name:      "regular output",
			events:    []*testEvent{outputEvent("hello\n", "")},
			wantCount: 1,
			wantLevel: gorppkg.LogLevelInfo,
			wantMsg:   "hello\n",
		},
		{
			name:      "frame output",
			events:    []*testEvent{outputEvent("=== RUN TestOne\n", testOutputTypeFrame)},
			wantCount: 1,
			wantLevel: gorppkg.LogLevelInfo,
			wantMsg:   "=== RUN TestOne\n",
		},
		{
			name:      "error output",
			events:    []*testEvent{outputEvent("assertion failed\n", testOutputTypeError)},
			wantCount: 1,
			wantLevel: gorppkg.LogLevelError,
			wantMsg:   "assertion failed\n",
		},
		{
			name: "error-continue merges with error",
			events: []*testEvent{
				outputEvent("assertion failed\n", testOutputTypeError),
				outputEvent("stack trace line\n", testOutputTypeErrorContinue),
			},
			wantCount: 1,
			wantLevel: gorppkg.LogLevelError,
			wantMsg:   "assertion failed\n\nstack trace line\n",
		},
		{
			name:      "error-continue without pending batch",
			events:    []*testEvent{outputEvent("orphan line\n", testOutputTypeErrorContinue)},
			wantCount: 1,
			wantLevel: gorppkg.LogLevelError,
			wantMsg:   "orphan line\n",
		},
		{
			name: "legacy tab merge",
			events: []*testEvent{
				outputEvent("first line\n", ""),
				outputEvent("\tcontinuation\n", ""),
			},
			wantCount: 1,
			wantLevel: gorppkg.LogLevelError,
			wantMsg:   "first line\n\n\tcontinuation\n",
		},
		{
			name: "frame does not use tab merge",
			events: []*testEvent{
				outputEvent("first line\n", ""),
				outputEvent("\tframing\n", testOutputTypeFrame),
			},
			wantCount: 2,
		},
		{
			name:      "empty output skipped",
			events:    []*testEvent{outputEvent("", testOutputTypeError)},
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			rep, rec := newTestReporter(t)
			for _, ev := range tt.events {
				rep.log(ev)
			}
			logs := sentLogs(t, rep, rec)
			require.Len(t, logs, tt.wantCount)
			if tt.wantCount == 0 {
				return
			}
			if tt.wantLevel != "" {
				assert.Equal(t, tt.wantLevel, logs[len(logs)-1]["level"])
			}
			if tt.wantMsg != "" {
				assert.Equal(t, tt.wantMsg, logs[len(logs)-1]["message"])
			}
		})
	}
}

func TestTestEvent_UnmarshalOutputType(t *testing.T) {
	t.Parallel()

	raw := `{"time":"2025-01-01T00:00:00Z","action":"output","package":"pkg","test":"TestOne","output":"err\n","OutputType":"error"}`
	var ev testEvent
	require.NoError(t, json.Unmarshal([]byte(raw), &ev))
	assert.Equal(t, testOutputTypeError, ev.OutputType)
	assert.Equal(t, "err\n", ev.Output)
}

// A log that waits for its continuation is not kept back while the test run is silent.
func TestReporter_PendingLogIsReleasedWhileInputIsSilent(t *testing.T) {
	t.Parallel()

	rep, rec := newTestReporter(t)
	input := make(chan *testEvent)
	rep.input = input
	rep.log(outputEvent("the last line before the test hangs\n", testOutputTypeError))
	require.NotNil(t, rep.pendingLog, "an error line may be continued, so it is held back")

	ticks := make(chan time.Time)
	done := make(chan bool)
	go func() {
		_, ok := rep.nextEvent(ticks)
		done <- ok
	}()
	ticks <- time.Now() // received by nextEvent, which releases the log before it waits again
	close(input)
	require.False(t, <-done)

	assert.Nil(t, rep.pendingLog)
	logs := sentLogs(t, rep, rec)
	require.Len(t, logs, 1)
	assert.Equal(t, "the last line before the test hangs\n", logs[0]["message"])
}

func TestReporterLog_OnlyContinuableOutputIsHeldBack(t *testing.T) {
	t.Parallel()

	rep, _ := newTestReporter(t)
	rep.log(outputEvent("=== RUN TestOne\n", testOutputTypeFrame))
	assert.Nil(t, rep.pendingLog, "framing is never continued: it goes to the batcher at once")
	rep.log(outputEvent("legacy line\n", ""))
	assert.NotNil(
		t,
		rep.pendingLog,
		"without OutputType any line may get a tab-indented continuation",
	)
	rep.releasePendingLog()
	rep.log(outputEvent("assertion failed\n", testOutputTypeError))
	require.NotNil(t, rep.pendingLog)
	require.NoError(
		t,
		rep.reportEvent(&testEvent{Action: testActionPause, Package: "pkg", Test: "TestOne"}),
	)
	assert.Nil(t, rep.pendingLog, "an event that is not output ends the log")
}

// ReportPortal answers a batch with 201 also when it rejects its logs. That is a failure of the report,
// like a failed request.
func TestReportLaunch_RejectedLogsFailTheReport(t *testing.T) {
	t.Parallel()

	rec, srv := newHierarchyServer(t, "proj")
	rec.rejectLogs = true
	file := writeEventLines(
		t,
		event("run", "TestOne"),
		`{"time":"2025-01-01T00:00:00Z","action":"output","package":"example/pkg","test":"TestOne","output":"x\n"}`,
		event("pass", "TestOne"),
		event("pass", ""),
	)
	cfg := &clientConfig{URL: srv.URL, Project: "proj", ApiKey: "tok"}

	_, _, err := reportLaunch(t.Context(), cfg, reportCmd(file, "launch", false))

	var batchErr *gorppkg.LogBatchError
	require.ErrorAs(t, err, &batchErr)
	require.ErrorContains(
		t,
		err,
		"reporting logs: 1 of 1 logs were not saved: ReportPortalException: rejected",
	)
	assert.Equal(t, "INTERRUPTED", rec.launchFinish["status"])
}
