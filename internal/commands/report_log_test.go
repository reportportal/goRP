package commands

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	gorppkg "github.com/reportportal/goRP/v5/pkg/gorp"
	"github.com/reportportal/goRP/v5/pkg/openapi"
)

func newTestReporter() *reporter {
	return &reporter{
		ctx:           context.Background(),
		launchUUID:    "launch-1",
		tests:         map[string]string{"pkg/TestOne": "item-1"},
		logs:          []*openapi.SaveLogRQ{},
		logsBatchSize: 100, // avoid async flush in unit tests
	}
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
	assert.False(t, shouldMergeOutput(&testEvent{OutputType: testOutputTypeFrame, Output: "\tframing"}))
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
			rep := newTestReporter()
			for _, ev := range tt.events {
				rep.log(ev)
			}
			require.Len(t, rep.logs, tt.wantCount)
			if tt.wantCount == 0 {
				return
			}
			if tt.wantLevel != "" {
				assert.Equal(t, tt.wantLevel, *rep.logs[len(rep.logs)-1].Level)
			}
			if tt.wantMsg != "" {
				assert.Equal(t, tt.wantMsg, *rep.logs[len(rep.logs)-1].Message)
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
