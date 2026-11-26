package rpagent

import (
	"testing"
	"time"

	"github.com/onsi/ginkgo/v2/types"
)

func loc(order, offset int, at time.Time) types.TimelineLocation {
	return types.TimelineLocation{Order: order, Offset: offset, Time: at}
}

func TestSplitAttemptsSingleRunIsUnchanged(t *testing.T) {
	spec := types.SpecReport{State: types.SpecStatePassed}
	if got := splitAttempts(spec, nil); len(got) != 1 {
		t.Fatalf("want 1 attempt, got %d", len(got))
	}
}

func TestSplitAttemptsCutsEventsOutputAndFailures(t *testing.T) {
	t0 := time.Now()
	at := func(s int) time.Time { return t0.Add(time.Duration(s) * time.Second) }
	spec := types.SpecReport{
		State:                      types.SpecStatePassed,
		StartTime:                  t0,
		EndTime:                    at(30),
		CapturedGinkgoWriterOutput: "AAAA" + "BBBB" + "CCCC",
		SpecEvents: []types.SpecEvent{
			{
				SpecEventType:    types.SpecEventByStart,
				Message:          "step1",
				TimelineLocation: loc(1, 0, at(1)),
			},
			{SpecEventType: types.SpecEventSpecRetry, TimelineLocation: loc(5, 4, at(10))},
			{
				SpecEventType:    types.SpecEventByStart,
				Message:          "step2",
				TimelineLocation: loc(6, 4, at(11)),
			},
			{SpecEventType: types.SpecEventSpecRetry, TimelineLocation: loc(9, 8, at(20))},
			{
				SpecEventType:    types.SpecEventByStart,
				Message:          "step3",
				TimelineLocation: loc(10, 8, at(21)),
			},
		},
	}
	failures := []types.Failure{
		{Message: "first", TimelineLocation: loc(3, 0, at(5))},
		{Message: "second", TimelineLocation: loc(8, 0, at(15))},
	}

	got := splitAttempts(spec, failures)
	if len(got) != 3 {
		t.Fatalf("want 3 attempts, got %d", len(got))
	}
	wantOut := []string{"AAAA", "BBBB", "CCCC"}
	wantMsg := []string{"first", "second", ""}
	wantState := []types.SpecState{
		types.SpecStateFailed,
		types.SpecStateFailed,
		types.SpecStatePassed,
	}
	wantStep := []string{"step1", "step2", "step3"}
	for i, a := range got {
		if a.CapturedGinkgoWriterOutput != wantOut[i] {
			t.Errorf(
				"attempt %d output = %q, want %q",
				i+1,
				a.CapturedGinkgoWriterOutput,
				wantOut[i],
			)
		}
		if a.Failure.Message != wantMsg[i] || a.State != wantState[i] {
			t.Errorf(
				"attempt %d = %q/%v, want %q/%v",
				i+1,
				a.Failure.Message,
				a.State,
				wantMsg[i],
				wantState[i],
			)
		}
		if len(a.SpecEvents) != 1 || a.SpecEvents[0].Message != wantStep[i] {
			t.Fatalf("attempt %d events = %+v, want only %s", i+1, a.SpecEvents, wantStep[i])
		}
		if off := a.SpecEvents[0].TimelineLocation.Offset; off != 0 {
			t.Errorf("attempt %d step offset = %d, want rebased to 0", i+1, off)
		}
	}
	if !got[1].StartTime.Equal(at(10)) || !got[1].EndTime.Equal(at(20)) {
		t.Errorf("attempt 2 window = %v..%v", got[1].StartTime, got[1].EndTime)
	}
}

func TestSplitAttemptsRetryWithoutCapturedFailureStillFailed(t *testing.T) {
	spec := types.SpecReport{
		State: types.SpecStatePassed,
		SpecEvents: []types.SpecEvent{
			{SpecEventType: types.SpecEventSpecRetry, TimelineLocation: loc(5, 0, time.Now())},
		},
	}
	got := splitAttempts(spec, nil)
	if got[0].State != types.SpecStateFailed || got[0].Failure.Message == "" {
		t.Errorf(
			"first attempt = %v %q, want failed with a message",
			got[0].State,
			got[0].Failure.Message,
		)
	}
}

func TestSplitAttemptsRepeatsStayPassed(t *testing.T) {
	spec := types.SpecReport{
		State: types.SpecStatePassed,
		SpecEvents: []types.SpecEvent{
			{SpecEventType: types.SpecEventSpecRepeat, TimelineLocation: loc(5, 0, time.Now())},
		},
	}
	if got := splitAttempts(spec, nil); got[0].State != types.SpecStatePassed {
		t.Errorf("repeat attempt state = %v, want passed", got[0].State)
	}
}
