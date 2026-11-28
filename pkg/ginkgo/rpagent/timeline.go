package rpagent

import (
	"time"

	"github.com/onsi/ginkgo/v2/types"

	"github.com/reportportal/goRP/v5/pkg/gorp"
)

// timeWindow bounds the timestamps sent for an item and its children. ReportPortal
// rejects or misorders children that start before their parent or end after it, so
// every timestamp taken from Ginkgo's timeline is clamped into the window of the
// spec item it belongs to.
type timeWindow struct {
	start time.Time
	end   time.Time
}

// specWindow builds the window of a spec item: from when the item was started to the
// spec's real end time. A missing or earlier end time falls back to now.
func specWindow(start, end time.Time) timeWindow {
	if end.IsZero() || end.Before(start) {
		end = time.Now()
	}
	if end.Before(start) { // clock went backwards; never produce a negative duration
		end = start
	}
	return timeWindow{start: start, end: end}
}

// clamp moves t into the window. A zero t becomes the window start.
func (w timeWindow) clamp(t time.Time) time.Time {
	if t.IsZero() {
		return w.start
	}
	if !w.start.IsZero() && t.Before(w.start) {
		return w.start
	}
	if !w.end.IsZero() && t.After(w.end) {
		return w.end
	}
	return t
}

// stepPlan is a By() step together with the time window it was active in.
type stepPlan struct {
	name  string
	order int // TimelineLocation.Order of the ByStart event
	// offset is the byte position in the spec's GinkgoWriter output at the time of the By() call.
	offset int
	start  time.Time
	end    time.Time

	explicitEnd bool // end came from a By-end event, not inferred
	endOrder    int  // TimelineLocation.Order of that By-end event (0 when inferred)
}

// planBySteps derives the window of every By() step from the spec's timeline.
//
// A step starts at its By event. It ends at its matching By-end event when one exists
// (the By(text, func(){...}) form), otherwise when the next step starts, otherwise
// when the spec item ends. All times are clamped into win.
func planBySteps(events []types.SpecEvent, win timeWindow) []stepPlan {
	type started struct {
		idx  int
		plan stepPlan
	}
	var starts []started
	for i, e := range events {
		if e.SpecEventType != types.SpecEventByStart || e.Message == "" {
			continue
		}
		starts = append(starts, started{idx: i, plan: stepPlan{
			name:   e.Message,
			order:  e.TimelineLocation.Order,
			offset: e.TimelineLocation.Offset,
			start:  win.clamp(e.TimelineLocation.Time),
		}})
	}

	consumed := make(map[int]bool) // By-end events already paired with a start
	plans := make([]stepPlan, 0, len(starts))
	for n, s := range starts {
		p := s.plan
		p.end = time.Time{}
		for j := s.idx + 1; j < len(events); j++ {
			e := events[j]
			if e.SpecEventType == types.SpecEventByEnd && e.Message == p.name && !consumed[j] {
				consumed[j] = true
				p.end = win.clamp(e.TimelineLocation.Time)
				p.explicitEnd = true
				p.endOrder = e.TimelineLocation.Order
				break
			}
		}
		if p.end.IsZero() {
			if n+1 < len(starts) {
				p.end = starts[n+1].plan.start
			} else {
				p.end = win.end
			}
		}
		if p.end.Before(p.start) {
			p.end = p.start
		}
		plans = append(plans, p)
	}

	// A step nested inside a callback step cannot outlive it.
	for i := range plans {
		if plans[i].explicitEnd {
			continue
		}
		for j := range plans {
			outer := plans[j]
			if j != i && outer.explicitEnd && !outer.start.After(plans[i].start) &&
				outer.end.Before(plans[i].end) &&
				!outer.end.Before(plans[i].start) {
				plans[i].end = outer.end
			}
		}
	}
	return plans
}

// stepStatuses assigns a ReportPortal status to every By() step.
//
// A spec that fails, is skipped or is interrupted does not fail all of its steps: steps
// that were already past stay PASSED, and only the step that was running when it
// happened gets the spec's status (plus any callback By() steps enclosing it).
//
// The running step is the last step that had started when the failure was recorded
// (failureOrder is the failure's TimelineLocation.Order). Ginkgo records a failure
// inside a By(text, func(){...}) callback after that step's end event, so asking
// "does the failure fall inside the step's window?" would miss it. Without a known
// failure (hasFailure false), the last step is treated as the running one.
func stepStatuses(
	plans []stepPlan,
	spec gorp.Status,
	failureOrder int,
	hasFailure bool,
) []gorp.Status {
	statuses := make([]gorp.Status, len(plans))
	for i := range statuses {
		statuses[i] = gorp.Statuses.Passed
	}
	if spec == gorp.Statuses.Passed || len(plans) == 0 {
		return statuses
	}

	running := runningStep(plans, failureOrder, hasFailure)
	if running < 0 {
		return statuses // the failure happened before any step started
	}

	statuses[running] = spec
	for i, p := range plans {
		if i != running && p.endOrder > 0 && p.order < plans[running].order &&
			plans[running].order < p.endOrder {
			statuses[i] = spec // a callback step that encloses the running step
		}
	}
	return statuses
}

// runningStep returns the index of the step that was running when the failure with the
// given timeline order was recorded: the last step that had already started. It returns
// -1 when no step had started (the failure happened outside every step). Without a known
// failure (hasFailure false) the last step is the running one.
func runningStep(plans []stepPlan, failureOrder int, hasFailure bool) int {
	running := -1
	for i, p := range plans { // plans are in start order
		if !hasFailure || p.order <= failureOrder {
			running = i
		}
	}
	return running
}
