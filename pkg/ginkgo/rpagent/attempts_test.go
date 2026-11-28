package rpagent

import (
	"fmt"
	"testing"
	"time"

	"github.com/onsi/ginkgo/v2/types"
)

func loc(order, offset int, at time.Time) types.TimelineLocation {
	return types.TimelineLocation{Order: order, Offset: offset, Time: at}
}

func recorded(attempt int, msg string, order int) types.AdditionalFailure {
	return types.AdditionalFailure{
		State: types.SpecStateFailed,
		Failure: types.Failure{
			Message:          fmt.Sprintf("%s%d:\n%s", attemptPrefix, attempt, msg),
			TimelineLocation: types.TimelineLocation{Order: order},
		},
	}
}

func TestSplitAttemptsSingleRunIsUnchanged(t *testing.T) {
	spec := types.SpecReport{State: types.SpecStatePassed}
	if got := splitAttempts(spec); len(got) != 1 {
		t.Fatalf("want 1 attempt, got %d", len(got))
	}
}

func TestSplitAttemptsUsesGinkgosRecordedFailures(t *testing.T) {
	t0 := time.Now()
	at := func(s int) time.Time { return t0.Add(time.Duration(s) * time.Second) }
	cleanup := types.AdditionalFailure{
		State: types.SpecStatePanicked,
		Failure: types.Failure{
			Message:          "cleanup broke",
			TimelineLocation: types.TimelineLocation{Order: 4},
		},
	}
	lateCleanup := types.AdditionalFailure{
		State: types.SpecStateFailed,
		Failure: types.Failure{
			Message:          "late cleanup",
			TimelineLocation: types.TimelineLocation{Order: 12},
		},
	}
	spec := types.SpecReport{
		State: types.SpecStateFailed,
		Failure: types.Failure{
			Message:          "final",
			TimelineLocation: loc(11, 0, at(25)),
		},
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
		// Ginkgo appends the failed attempts' failures, then any cleanup failures.
		AdditionalFailures: []types.AdditionalFailure{
			recorded(1, "first", 3), cleanup, recorded(2, "second", 8), lateCleanup,
		},
	}

	got := splitAttempts(spec)
	if len(got) != 3 {
		t.Fatalf("want 3 attempts, got %d", len(got))
	}
	wantOut := []string{"AAAA", "BBBB", "CCCC"}
	wantMsg := []string{"first", "second", "final"}
	wantState := []types.SpecState{
		types.SpecStateFailed,
		types.SpecStateFailed,
		types.SpecStateFailed,
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
	// The cleanup failure stays with the attempt it happened in; the "recorded during
	// attempt" entries are never logged as extras.
	if len(got[0].AdditionalFailures) != 1 ||
		got[0].AdditionalFailures[0].Failure.Message != "cleanup broke" {
		t.Errorf("attempt 1 extras = %+v", got[0].AdditionalFailures)
	}
	if len(got[1].AdditionalFailures) != 0 {
		t.Errorf("attempt 2 extras = %+v, want none", got[1].AdditionalFailures)
	}
	if len(got[2].AdditionalFailures) != 1 ||
		got[2].AdditionalFailures[0].Failure.Message != "late cleanup" {
		t.Errorf("final attempt extras = %+v", got[2].AdditionalFailures)
	}
	if !got[1].StartTime.Equal(at(10)) || !got[1].EndTime.Equal(at(20)) {
		t.Errorf("attempt 2 window = %v..%v", got[1].StartTime, got[1].EndTime)
	}
}

func TestSplitAttemptsFailureOfNonFinalAttemptKeepsItsState(t *testing.T) {
	spec := types.SpecReport{
		State: types.SpecStatePassed,
		SpecEvents: []types.SpecEvent{
			{SpecEventType: types.SpecEventSpecRetry, TimelineLocation: loc(5, 0, time.Now())},
		},
	}
	panicked := recorded(1, "boom", 2)
	panicked.State = types.SpecStatePanicked
	spec.AdditionalFailures = []types.AdditionalFailure{panicked}
	got := splitAttempts(spec)
	if got[0].State != types.SpecStatePanicked || got[0].Failure.Message != "boom" {
		t.Errorf("first attempt = %v %q, want panicked boom", got[0].State, got[0].Failure.Message)
	}
	if got[1].State != types.SpecStatePassed {
		t.Errorf("final attempt = %v, want passed", got[1].State)
	}
}

func TestSplitAttemptsNeverSplitsOnRepetitions(t *testing.T) {
	t0 := time.Now()
	spec := types.SpecReport{
		State:                 types.SpecStatePassed,
		StartTime:             t0,
		MaxMustPassRepeatedly: 3,
		SpecEvents: []types.SpecEvent{
			{
				SpecEventType:    types.SpecEventSpecRepeat,
				TimelineLocation: loc(5, 0, t0.Add(time.Second)),
			},
			{
				SpecEventType:    types.SpecEventSpecRepeat,
				TimelineLocation: loc(9, 0, t0.Add(2*time.Second)),
			},
		},
	}
	if got := splitAttempts(spec); len(got) != 1 {
		t.Fatalf("a MustPassRepeatedly spec must stay one item, got %d", len(got))
	}
	times := repeatTimes(spec)
	if len(times) != 3 || !times[0].Time.Equal(t0) || !times[2].Time.Equal(t0.Add(2*time.Second)) {
		t.Errorf("repeat times = %+v, want 3 starting at the spec start", times)
	}
	if got := repeatTimes(types.SpecReport{MaxMustPassRepeatedly: 1}); got != nil {
		t.Errorf("a spec run once has no repetitions, got %+v", got)
	}
}

func TestSecondaryFailuresOfASingleRunAreKept(t *testing.T) {
	spec := types.SpecReport{
		State: types.SpecStateFailed,
		AdditionalFailures: []types.AdditionalFailure{
			{
				State: types.SpecStateFailed,
				Failure: types.Failure{
					Message:          "cleanup",
					TimelineLocation: types.TimelineLocation{Order: 7},
				},
			},
		},
	}
	got := splitAttempts(spec)
	if len(got) != 1 || len(got[0].AdditionalFailures) != 1 {
		t.Fatalf("got %+v", got)
	}
}
