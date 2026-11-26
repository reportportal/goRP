package rpagent

import (
	"math"
	"sync"

	"github.com/onsi/ginkgo/v2/types"
)

// attemptFailures remembers, per spec, the failure of every attempt that failed. Ginkgo
// keeps only the last attempt's failure in the final report, so an earlier attempt's
// message would be lost; it is captured while the attempt is still running.
var attemptFailures sync.Map // spec full text -> []types.Failure

// recordAttemptFailure is called from a JustAfterEach node after every attempt.
func recordAttemptFailure(spec types.SpecReport) {
	if spec.Failure.Message == "" && spec.Failure.TimelineLocation.Order == 0 {
		return
	}
	key := spec.FullText()
	prev, _ := attemptFailures.Load(key)
	list, _ := prev.([]types.Failure)
	attemptFailures.Store(key, append(list, spec.Failure))
}

func takeAttemptFailures(spec types.SpecReport) []types.Failure {
	v, ok := attemptFailures.LoadAndDelete(spec.FullText())
	if !ok {
		return nil
	}
	list, _ := v.([]types.Failure)
	return list
}

// splitAttempts cuts a spec that Ginkgo ran several times (FlakeAttempts, MustPassRepeatedly)
// into one report per attempt, so each can be reported as its own ReportPortal item.
// A spec that ran once yields itself. All but the last attempt are failed attempts for
// a retry; the last carries the spec's real final state.
//
// failures are the failures captured per attempt (see recordAttemptFailure).
func splitAttempts(spec types.SpecReport, failures []types.Failure) []types.SpecReport {
	var bounds []types.SpecEvent
	for _, e := range spec.SpecEvents {
		if e.SpecEventType == types.SpecEventSpecRetry ||
			e.SpecEventType == types.SpecEventSpecRepeat {
			bounds = append(bounds, e)
		}
	}
	if len(bounds) == 0 {
		return []types.SpecReport{spec}
	}

	out := make([]types.SpecReport, 0, len(bounds)+1)
	lo := math.MinInt
	loOffset := 0
	loTime := spec.StartTime
	for k := 0; k <= len(bounds); k++ {
		hi, hiOffset, hiTime := math.MaxInt, len(spec.CapturedGinkgoWriterOutput), spec.EndTime
		if k < len(bounds) {
			b := bounds[k].TimelineLocation
			hi, hiOffset, hiTime = b.Order, b.Offset, b.Time
		}
		v := spec
		v.StartTime, v.EndTime = loTime, hiTime
		v.SpecEvents = nil
		for _, e := range spec.SpecEvents {
			o := e.TimelineLocation.Order
			if o <= lo || o >= hi {
				continue
			}
			e.TimelineLocation.Offset -= loOffset // offsets are into the whole GinkgoWriter stream
			v.SpecEvents = append(v.SpecEvents, e)
		}
		v.ReportEntries = nil
		for _, e := range spec.ReportEntries {
			if o := e.TimelineLocation.Order; o > lo && o < hi {
				v.ReportEntries = append(v.ReportEntries, e)
			}
		}
		v.CapturedGinkgoWriterOutput = sliceString(
			spec.CapturedGinkgoWriterOutput,
			loOffset,
			hiOffset,
		)
		if k < len(bounds) { // not the last attempt: it failed (or, for a repeat, passed)
			v.CapturedStdOutErr = ""
			v.Failure = types.Failure{}
			v.State = types.SpecStatePassed
			for _, f := range failures {
				if o := f.TimelineLocation.Order; o > lo && o < hi {
					v.Failure, v.State = f, types.SpecStateFailed
				}
			}
			if bounds[k].SpecEventType == types.SpecEventSpecRetry &&
				v.State != types.SpecStateFailed {
				// A retry means the attempt failed even if its message was not captured.
				v.State = types.SpecStateFailed
				v.Failure = types.Failure{Message: "attempt failed (details were not captured)"}
			}
		}
		out = append(out, v)
		lo, loOffset, loTime = hi, hiOffset, hiTime
	}
	return out
}

func sliceString(s string, from, to int) string {
	if from < 0 {
		from = 0
	}
	if to > len(s) {
		to = len(s)
	}
	if from >= to {
		return ""
	}
	return s[from:to]
}
