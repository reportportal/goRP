package rpagent

import (
	"fmt"
	"math"
	"strings"

	"github.com/onsi/ginkgo/v2/types"
)

// attemptPrefix starts the message of the AdditionalFailures entry that Ginkgo adds, for
// every failed attempt except the last, when it retries a FlakeAttempts spec. The entry
// holds that attempt's State and full Failure.
const attemptPrefix = "Failure recorded during attempt "

// splitAttempts cuts a spec that Ginkgo retried (FlakeAttempts, --flake-attempts) into one
// report per attempt, so each can be reported as its own ReportPortal retry item. A spec
// that ran once yields itself.
//
// Only retries split a spec. A MustPassRepeatedly repetition is not a retry: in
// ReportPortal a retry means an earlier attempt failed, and here nothing did. The spec
// stays one item (see repeatTimes for how repetitions are marked).
//
// For a retried attempt the State and Failure are the ones Ginkgo recorded for it in
// SpecReport.AdditionalFailures. That covers a failure in an AfterEach or a DeferCleanup
// too, which only Ginkgo sees. The last attempt carries the spec's own State and Failure.
// Every other additional failure (for example a cleanup node that failed after the
// attempt's primary failure) stays on the attempt in whose window it happened, in
// AdditionalFailures, to be logged there.
func splitAttempts(spec types.SpecReport) []types.SpecReport {
	var bounds []types.SpecEvent
	for _, e := range spec.SpecEvents {
		if e.SpecEventType == types.SpecEventSpecRetry {
			bounds = append(bounds, e)
		}
	}
	if len(bounds) == 0 {
		spec.AdditionalFailures = secondaryFailures(
			spec.AdditionalFailures,
			math.MinInt,
			math.MaxInt,
		)
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
		v.AdditionalFailures = secondaryFailures(spec.AdditionalFailures, lo, hi)
		if k < len(bounds) { // an attempt that failed and was retried
			v.CapturedStdOutErr = "" // Ginkgo gives no offsets: stdout/stderr stays on the last attempt
			v.State, v.Failure = attemptFailure(spec.AdditionalFailures, k+1)
		}
		out = append(out, v)
		lo, loOffset, loTime = hi, hiOffset, hiTime
	}
	return out
}

// attemptFailure returns the State and Failure Ginkgo recorded for the given (1-based)
// attempt, without the "Failure recorded during attempt N:" prefix.
func attemptFailure(
	additional []types.AdditionalFailure,
	attempt int,
) (types.SpecState, types.Failure) {
	prefix := fmt.Sprintf("%s%d:\n", attemptPrefix, attempt)
	for _, af := range additional {
		if strings.HasPrefix(af.Failure.Message, prefix) {
			f := af.Failure
			f.Message = strings.TrimPrefix(f.Message, prefix)
			return af.State, f
		}
	}
	// Ginkgo always records one; this only guards against a future Ginkgo that does not.
	return types.SpecStateFailed, types.Failure{
		Message: "attempt failed (Ginkgo recorded no details)",
	}
}

// secondaryFailures returns the additional failures inside the (lo, hi) timeline window
// that are not an attempt's primary failure.
func secondaryFailures(all []types.AdditionalFailure, lo, hi int) []types.AdditionalFailure {
	var out []types.AdditionalFailure
	for _, af := range all {
		if strings.HasPrefix(af.Failure.Message, attemptPrefix) {
			continue
		}
		if o := af.Failure.TimelineLocation.Order; o > lo && o < hi {
			out = append(out, af)
		}
	}
	return out
}

// repeatTimes returns when each repetition of a MustPassRepeatedly spec began, the first
// at the spec's start. It is empty for a spec that is not repeated.
func repeatTimes(spec types.SpecReport) []types.TimelineLocation {
	if spec.MaxMustPassRepeatedly <= 1 {
		return nil
	}
	times := []types.TimelineLocation{{Time: spec.StartTime}}
	for _, e := range spec.SpecEvents {
		if e.SpecEventType == types.SpecEventSpecRepeat {
			times = append(times, e.TimelineLocation)
		}
	}
	return times
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
