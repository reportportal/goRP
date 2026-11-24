package rpagent

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/onsi/ginkgo/v2/types"

	"github.com/reportportal/goRP/v5/pkg/gorp"
	"github.com/reportportal/goRP/v5/pkg/openapi"
)

var base = time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

func at(ms int) time.Time { return base.Add(time.Duration(ms) * time.Millisecond) }

func byStart(msg string, order, ms int) types.SpecEvent {
	return types.SpecEvent{
		SpecEventType:    types.SpecEventByStart,
		Message:          msg,
		TimelineLocation: types.TimelineLocation{Order: order, Time: at(ms)},
	}
}

func byEnd(msg string, order, ms int) types.SpecEvent {
	return types.SpecEvent{
		SpecEventType:    types.SpecEventByEnd,
		Message:          msg,
		TimelineLocation: types.TimelineLocation{Order: order, Time: at(ms)},
	}
}

func TestSpecWindow(t *testing.T) {
	if w := specWindow(at(0), at(100)); !w.start.Equal(at(0)) || !w.end.Equal(at(100)) {
		t.Errorf("window = %+v", w)
	}
	if w := specWindow(at(0), time.Time{}); w.end.Before(w.start) || w.end.IsZero() {
		t.Errorf("missing end must fall back to now, got %+v", w)
	}
	far := time.Now().Add(time.Hour)
	if w := specWindow(far, at(0)); w.end.Before(w.start) {
		t.Errorf("end must never be before start: %+v", w)
	}
}

func TestTimeWindowClamp(t *testing.T) {
	w := timeWindow{start: at(10), end: at(90)}
	cases := []struct {
		name string
		in   time.Time
		want time.Time
	}{
		{"inside", at(50), at(50)},
		{"before start", at(1), at(10)},
		{"after end", at(500), at(90)},
		{"zero becomes start", time.Time{}, at(10)},
	}
	for _, c := range cases {
		if got := w.clamp(c.in); !got.Equal(c.want) {
			t.Errorf("%s: clamp = %v, want %v", c.name, got, c.want)
		}
	}
	if got := (timeWindow{}).clamp(at(5)); !got.Equal(at(5)) {
		t.Errorf("an open window must not change the time")
	}
}

func TestPlanBySteps(t *testing.T) {
	win := timeWindow{start: at(0), end: at(100)}
	span := func(p stepPlan) [2]int {
		return [2]int{
			int(p.start.Sub(base) / time.Millisecond),
			int(p.end.Sub(base) / time.Millisecond),
		}
	}

	t.Run("plain steps end where the next begins; the last ends with the spec", func(t *testing.T) {
		plans := planBySteps([]types.SpecEvent{byStart("a", 1, 10), byStart("b", 2, 40)}, win)
		if len(plans) != 2 || span(plans[0]) != [2]int{10, 40} ||
			span(plans[1]) != [2]int{40, 100} {
			t.Errorf("plans = %+v", plans)
		}
	})

	t.Run("callback step ends at its own end event", func(t *testing.T) {
		plans := planBySteps([]types.SpecEvent{
			byStart(
				"plain",
				1,
				10,
			),
			byStart("cb", 2, 40),
			byEnd("cb", 4, 70),
			byStart("after", 5, 80),
		}, win)
		if span(plans[0]) != [2]int{10, 40} || span(plans[1]) != [2]int{40, 70} ||
			span(plans[2]) != [2]int{80, 100} {
			t.Errorf("plans = %+v", plans)
		}
	})

	t.Run("a plain step nested in a callback step ends with it", func(t *testing.T) {
		plans := planBySteps([]types.SpecEvent{
			byStart("outer", 1, 10), byStart("inner", 2, 20), byEnd("outer", 4, 60),
		}, win)
		if span(plans[0]) != [2]int{10, 60} || span(plans[1]) != [2]int{20, 60} {
			t.Errorf("plans = %+v", plans)
		}
	})

	t.Run("repeated step names each pair with their own end", func(t *testing.T) {
		plans := planBySteps([]types.SpecEvent{
			byStart("x", 1, 10), byEnd("x", 2, 30), byStart("x", 3, 50), byEnd("x", 4, 70),
		}, win)
		if span(plans[0]) != [2]int{10, 30} || span(plans[1]) != [2]int{50, 70} {
			t.Errorf("plans = %+v", plans)
		}
	})

	t.Run("times outside the window are clamped", func(t *testing.T) {
		plans := planBySteps(
			[]types.SpecEvent{byStart("early", 1, -50), byStart("late", 2, 400)},
			win,
		)
		if span(plans[0])[0] != 0 || span(plans[1]) != [2]int{100, 100} {
			t.Errorf("plans = %+v", plans)
		}
	})

	t.Run("empty names and no events", func(t *testing.T) {
		if got := planBySteps([]types.SpecEvent{byStart("", 1, 10)}, win); len(got) != 0 {
			t.Errorf("empty By text must be ignored: %+v", got)
		}
		if got := planBySteps(nil, win); len(got) != 0 {
			t.Errorf("no events, no steps: %+v", got)
		}
	})
}

// TestFinishSpecHierarchyUsesGinkgoTimeline drives the real finishing path with a
// synthetic Ginkgo report and checks every timestamp that reaches ReportPortal.
func TestFinishSpecHierarchyUsesGinkgoTimeline(t *testing.T) {
	var mu sync.Mutex
	stepStarts := map[string]time.Time{} // step name -> start
	finishes := map[string]time.Time{}   // item id -> end
	statuses := map[string]string{}      // item id -> final status
	var logs []openapi.SaveLogRQ
	var stepSeq int
	tl := newTestListener(t, func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		switch {
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/log"):
			var rq openapi.SaveLogRQ
			_ = json.NewDecoder(r.Body).Decode(&rq)
			logs = append(logs, rq)
			writeJSON(w, `{}`)
		case r.Method == http.MethodPost:
			var rq openapi.StartTestItemRQ
			_ = json.NewDecoder(r.Body).Decode(&rq)
			stepStarts[rq.Name] = rq.StartTime
			stepSeq++
			writeJSON(w, `{"id":"step-`+string(rune('0'+stepSeq))+`"}`)
		case r.Method == http.MethodPut:
			var rq openapi.FinishTestItemRQ
			_ = json.NewDecoder(r.Body).Decode(&rq)
			id := r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:]
			finishes[id] = rq.EndTime
			if rq.Status != nil {
				statuses[id] = *rq.Status
			}
			writeJSON(w, `{}`)
		}
	})

	oldClient, oldListener := globalClient, globalListener
	globalClient, globalListener = tl.client, tl
	t.Cleanup(func() { globalClient, globalListener = oldClient, oldListener })

	spec := types.SpecReport{
		ContainerHierarchyTexts: []string{"C"},
		LeafNodeText:            "spec",
		State:                   types.SpecStateFailed,
		StartTime:               at(0),
		EndTime:                 at(100),
		Failure: types.Failure{
			Message:          "boom",
			Location:         types.CodeLocation{FileName: "x_test.go", LineNumber: 3},
			TimelineLocation: types.TimelineLocation{Order: 5, Time: at(80)},
		},
		SpecEvents: []types.SpecEvent{
			byStart("one", 2, 10), byStart("two", 4, 40), byEnd("two", 6, 70),
		},
		ReportEntries: types.ReportEntries{
			{
				Name:             "RP_LOG#rp_log#INFO#in-one",
				Time:             at(20),
				TimelineLocation: types.TimelineLocation{Order: 3, Time: at(20)},
			},
			{
				Name:             "RP_LOG#rp_log#WARN#in-two",
				Time:             at(50),
				TimelineLocation: types.TimelineLocation{Order: 5, Time: at(50)},
			},
			{
				Name:             "RP_LOG#rp_log#INFO#too-late",
				Time:             at(900),
				TimelineLocation: types.TimelineLocation{Order: 7, Time: at(900)},
			},
		},
	}
	ids := SpecHierarchyIDs{StepID: "spec-item", StartTime: at(0)}
	activeHierarchies.Store(spec.FullText(), ids)
	t.Cleanup(func() { activeHierarchies.Delete(spec.FullText()) })

	finishSpecHierarchy(spec)

	if !stepStarts["one"].Equal(at(10)) || !stepStarts["two"].Equal(at(40)) {
		t.Errorf("step starts = %v", stepStarts)
	}
	if !finishes["step-1"].Equal(at(40)) || !finishes["step-2"].Equal(at(70)) {
		t.Errorf("step ends = %v (want one:+40ms, two:+70ms)", finishes)
	}
	if !finishes["spec-item"].Equal(at(100)) {
		t.Errorf("spec item must finish at spec.EndTime (+100ms), got %v", finishes["spec-item"])
	}

	// The failure (order 5) happened while step "two" (order 4) was running.
	if statuses["step-1"] != "PASSED" || statuses["step-2"] != "FAILED" ||
		statuses["spec-item"] != "FAILED" {
		t.Errorf("statuses = %v, want one:PASSED two:FAILED spec:FAILED", statuses)
	}

	byMsg := map[string]openapi.SaveLogRQ{}
	for _, l := range logs {
		byMsg[*l.Message] = l
	}
	if l := byMsg["in-one"]; !l.Time.Equal(at(20)) || *l.ItemUuid != "step-1" {
		t.Errorf("in-one: %+v", l)
	}
	if l := byMsg["in-two"]; !l.Time.Equal(at(50)) || *l.ItemUuid != "step-2" {
		t.Errorf("in-two: %+v", l)
	}
	if l := byMsg["too-late"]; !l.Time.Equal(at(100)) {
		t.Errorf("a log after the spec ended must be clamped to the end, got %v", l.Time)
	}
	found := false
	for _, l := range logs {
		if strings.HasPrefix(*l.Message, "TEST FAILED") {
			found = true
			// Step "two" (order 4) was running at the failure (order 5). Its window ends at +70ms,
			// and the failure was recorded at +80ms, so the time is clamped into the step.
			if !l.Time.Equal(at(70)) || *l.ItemUuid != "step-2" {
				t.Errorf(
					"failure log must be on the running step, clamped to its window (+70ms): %+v",
					l,
				)
			}
		}
	}
	if !found {
		t.Errorf("no failure log was sent")
	}
	_ = gorp.Statuses.Passed
}

func plan(name string, order, endOrder int) stepPlan {
	return stepPlan{name: name, order: order, endOrder: endOrder, explicitEnd: endOrder > 0}
}

func TestStepStatuses(t *testing.T) {
	passed, failed := gorp.Statuses.Passed, gorp.Statuses.Failed
	three := []stepPlan{plan("a", 5, 0), plan("b", 7, 0), plan("c", 9, 0)}

	cases := []struct {
		name         string
		plans        []stepPlan
		spec         gorp.Status
		failureOrder int
		hasFailure   bool
		want         []gorp.Status
	}{
		{
			"passed spec: every step passes",
			three,
			passed,
			0,
			false,
			[]gorp.Status{passed, passed, passed},
		},
		{
			"failure in the middle step",
			three,
			failed,
			8,
			true,
			[]gorp.Status{passed, failed, passed},
		},
		{
			"failure in the last step",
			three,
			failed,
			20,
			true,
			[]gorp.Status{passed, passed, failed},
		},
		{
			"failure in the first step",
			three,
			failed,
			6,
			true,
			[]gorp.Status{failed, passed, passed},
		},
		{"failure before any step", three, failed, 3, true, []gorp.Status{passed, passed, passed}},
		{
			"no failure info: the last step is the running one",
			three,
			failed,
			0,
			false,
			[]gorp.Status{passed, passed, failed},
		},
		{
			"skip inside a step", three[:2], gorp.Statuses.Skipped, 8, true,
			[]gorp.Status{passed, gorp.Statuses.Skipped},
		},
		{
			"interrupted while a step runs", three, gorp.Statuses.Interrupted, 8, true,
			[]gorp.Status{passed, gorp.Statuses.Interrupted, passed},
		},
		{
			// Ginkgo records a failure inside By(text, func(){...}) AFTER the step's end event
			// (end order 9, failure order 10); the callback step must still be the failed one.
			"failure inside a callback step is recorded after its end event",
			[]stepPlan{plan("plain", 5, 0), plan("cb", 7, 9)},
			failed, 10, true,
			[]gorp.Status{passed, failed},
		},
		{
			"a callback step that encloses the failing step fails too",
			[]stepPlan{plan("outer", 5, 12), plan("inner", 7, 0)},
			failed, 9, true,
			[]gorp.Status{failed, failed},
		},
		{
			"a finished callback step stays passed when a later step fails",
			[]stepPlan{plan("cb", 5, 8), plan("after", 9, 0)},
			failed, 10, true,
			[]gorp.Status{passed, failed},
		},
		{"no steps", nil, failed, 8, true, []gorp.Status{}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := stepStatuses(c.plans, c.spec, c.failureOrder, c.hasFailure)
			if len(got) != len(c.want) {
				t.Fatalf("got %v, want %v", got, c.want)
			}
			for i := range got {
				if got[i] != c.want[i] {
					t.Errorf(
						"step %d (%s): got %s, want %s (all: %v)",
						i,
						c.plans[i].name,
						got[i],
						c.want[i],
						got,
					)
				}
			}
		})
	}
}

func TestPlanByStepsRecordsEndOrderOnlyForExplicitEnds(t *testing.T) {
	win := timeWindow{start: at(0), end: at(100)}
	plans := planBySteps(
		[]types.SpecEvent{byStart("plain", 5, 10), byStart("cb", 7, 40), byEnd("cb", 9, 70)},
		win,
	)
	if plans[0].endOrder != 0 || plans[0].explicitEnd {
		t.Errorf("plain step: %+v", plans[0])
	}
	if plans[1].endOrder != 9 || !plans[1].explicitEnd {
		t.Errorf("callback step: %+v", plans[1])
	}
}

func TestRunningStep(t *testing.T) {
	plans := []stepPlan{{order: 5}, {order: 7}, {order: 9}}
	cases := []struct {
		name       string
		order      int
		hasFailure bool
		want       int
	}{
		{"between steps: the last one started", 8, true, 1},
		{"after the last step started", 20, true, 2},
		{"exactly at a step's start", 7, true, 1},
		{"before any step", 3, true, -1},
		{"no failure info: the last step", 0, false, 2},
	}
	for _, c := range cases {
		if got := runningStep(plans, c.order, c.hasFailure); got != c.want {
			t.Errorf("%s: got %d, want %d", c.name, got, c.want)
		}
	}
	if got := runningStep(nil, 5, true); got != -1 {
		t.Errorf("no steps must give -1, got %d", got)
	}
}

func TestFailureMessageGoesToTheRunningStep(t *testing.T) {
	failure := func(order, ms int) types.Failure {
		return types.Failure{
			Message:          "boom",
			Location:         types.CodeLocation{FileName: "x_test.go", LineNumber: 3},
			TimelineLocation: types.TimelineLocation{Order: order, Time: at(ms)},
		}
	}
	cases := []struct {
		name       string
		state      types.SpecState
		events     []types.SpecEvent
		failure    types.Failure
		wantItem   string
		wantAt     time.Time
		wantPrefix string
		wantLevel  string
	}{
		{
			"failure inside the second plain step",
			types.SpecStateFailed,
			[]types.SpecEvent{byStart("one", 2, 10), byStart("two", 4, 40)},
			failure(5, 60), "step-2", at(60), "TEST FAILED", "ERROR",
		},
		{
			// Ginkgo records this failure after the callback step's own end event.
			"failure inside a callback step, recorded after its end event",
			types.SpecStateFailed,
			[]types.SpecEvent{byStart("cb", 2, 10), byEnd("cb", 4, 30)},
			failure(5, 30), "step-1", at(30), "TEST FAILED", "ERROR",
		},
		{
			"failure before any step started stays on the spec item",
			types.SpecStateFailed,
			[]types.SpecEvent{byStart("late", 6, 50)},
			failure(3, 20), "spec-item", at(20), "TEST FAILED", "ERROR",
		},
		{
			"a spec without steps keeps the message on the spec item",
			types.SpecStateFailed, nil,
			failure(3, 20), "spec-item", at(20), "TEST FAILED", "ERROR",
		},
		{
			"a skip is an INFO entry on the step that was running, not an error",
			types.SpecStateSkipped,
			[]types.SpecEvent{byStart("one", 2, 10), byStart("two", 4, 40)},
			failure(5, 45), "step-2", at(45), "SKIPPED", "INFO",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			logsOf := outputHarness(t)
			finishWithOutput(t, types.SpecReport{
				LeafNodeText: "spec",
				State:        c.state,
				StartTime:    at(0),
				EndTime:      at(100),
				SpecEvents:   c.events,
				Failure:      c.failure,
			})
			var found []sentLog
			for _, l := range logsOf() {
				if strings.HasPrefix(l.msg, "TEST FAILED") || strings.HasPrefix(l.msg, "SKIPPED") {
					found = append(found, l)
				}
			}
			if len(found) != 1 {
				t.Fatalf("want exactly one failure/skip message, got %d: %+v", len(found), found)
			}
			if found[0].item != c.wantItem || !found[0].at.Equal(c.wantAt) ||
				found[0].level != c.wantLevel || !strings.HasPrefix(found[0].msg, c.wantPrefix) {
				t.Errorf("item=%s at=%v level=%s msg=%.12q; want item=%s at=%v level=%s prefix=%s",
					found[0].item, found[0].at, found[0].level, found[0].msg,
					c.wantItem, c.wantAt, c.wantLevel, c.wantPrefix)
			}
		})
	}
}

func TestFailureLog(t *testing.T) {
	spec := types.SpecReport{Failure: types.Failure{
		Message:  "currency XYZ is not supported yet",
		Location: types.CodeLocation{FileName: "tables_test.go", LineNumber: 37},
	}}

	level, msg := failureLog(spec, gorp.Statuses.Failed)
	if level != "ERROR" ||
		msg != "TEST FAILED\n\nLocation: tables_test.go:37\n\nFailure:\ncurrency XYZ is not supported yet" {
		t.Errorf("failed spec: level=%s msg=%q", level, msg)
	}

	level, msg = failureLog(spec, gorp.Statuses.Skipped)
	if level != "INFO" ||
		msg != "SKIPPED\n\nLocation: tables_test.go:37\n\nReason:\ncurrency XYZ is not supported yet" {
		t.Errorf("skipped spec: level=%s msg=%q", level, msg)
	}
	if strings.Contains(msg, "FAILED") {
		t.Errorf("a skip must never be described as a failure: %q", msg)
	}

	// Interrupted/aborted/timed-out specs are not skips: they keep the error entry.
	for _, st := range []gorp.Status{gorp.Statuses.Failed, gorp.Statuses.Interrupted} {
		if level, _ := failureLog(spec, st); level != "ERROR" {
			t.Errorf("status %s must stay an ERROR entry, got %s", st, level)
		}
	}
}
