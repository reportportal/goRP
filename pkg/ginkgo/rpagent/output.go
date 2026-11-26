package rpagent

import (
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/onsi/ginkgo/v2/types"

	"github.com/reportportal/goRP/v5/pkg/gorp"
)

const (
	// maxLogChunkBytes caps a single log entry; longer output is split at line boundaries.
	maxLogChunkBytes = 32 << 10
	// maxCapturedOutputBytes caps how much captured output is sent per spec and stream.
	// A chatty spec cannot flood ReportPortal or hold unbounded memory in the queue.
	maxCapturedOutputBytes = 1 << 20
)

// ansiEscape matches ANSI CSI sequences (colors, cursor movement). Libraries that log
// through GinkgoWriter often color their output; ReportPortal would show the raw codes.
var ansiEscape = regexp.MustCompile("\x1b\\[[0-9;?]*[ -/]*[@-~]")

func stripANSI(s string) string { return ansiEscape.ReplaceAllString(s, "") }

// outputSegment is a piece of captured output and the item it belongs to.
type outputSegment struct {
	step int // index into the step plans, or -1 for the spec item itself
	text string
	at   time.Time
}

// segmentWriterOutput attributes GinkgoWriter output to the item that was running when
// it was written. Ginkgo records each By() event with the byte offset it had in the
// writer stream, so the text between two By() events belongs to the earlier step, and
// text before the first By() belongs to the spec item. A segment is timestamped with
// the start of the step it belongs to: Ginkgo keeps no per-write timestamps.
func segmentWriterOutput(output string, plans []stepPlan, win timeWindow) []outputSegment {
	if output == "" {
		return nil
	}
	clamp := func(v, lo, hi int) int { return max(lo, min(v, hi)) }

	end := len(output)
	if len(plans) > 0 {
		end = clamp(plans[0].offset, 0, len(output))
	}
	segments := []outputSegment{{step: -1, text: output[:end], at: win.start}}

	pos := end
	for i, p := range plans {
		from := clamp(p.offset, pos, len(output))
		to := len(output)
		if i+1 < len(plans) {
			to = clamp(plans[i+1].offset, from, len(output))
		}
		segments = append(segments, outputSegment{step: i, text: output[from:to], at: p.start})
		pos = to
	}
	return segments
}

// splitLogText splits text into chunks of at most maxChunk bytes, breaking at line
// boundaries (and inside a single over-long line only when it cannot be avoided).
func splitLogText(text string, maxChunk int) []string {
	var chunks []string
	var cur strings.Builder
	flush := func() {
		if cur.Len() > 0 {
			chunks = append(chunks, strings.TrimRight(cur.String(), "\n"))
			cur.Reset()
		}
	}
	for _, line := range strings.SplitAfter(text, "\n") {
		for len(line) > maxChunk {
			flush()
			cut := maxChunk
			for cut > 0 && !utf8.RuneStart(line[cut]) {
				cut--
			}
			if cut == 0 {
				cut = maxChunk
			}
			chunks = append(chunks, line[:cut])
			line = line[cut:]
		}
		if cur.Len()+len(line) > maxChunk {
			flush()
		}
		cur.WriteString(line)
	}
	flush()
	return chunks
}

// limitBytes returns text cut to at most budget bytes, preferring a line boundary.
func limitBytes(text string, budget int) string {
	if budget <= 0 {
		return ""
	}
	if len(text) <= budget {
		return text
	}
	cut := text[:budget]
	if i := strings.LastIndex(cut, "\n"); i > 0 {
		cut = cut[:i]
	}
	return strings.ToValidUTF8(cut, "")
}

// processCapturedOutput sends what the spec wrote to GinkgoWriter, and what Ginkgo
// captured from stdout/stderr, as log entries on the item that was running.
//
// Ginkgo only captures stdout/stderr when it intercepts output, which it does in
// parallel runs; in a serial run that stream is empty.
func processCapturedOutput(
	spec types.SpecReport,
	stepID string,
	steps []byStep,
	plans []stepPlan,
	win timeWindow,
) {
	targetFor := func(step int) string {
		if step >= 0 && step < len(steps) && steps[step].id != "" {
			return steps[step].id
		}
		return stepID
	}

	sendOutput(
		segmentWriterOutput(spec.CapturedGinkgoWriterOutput, plans, win),
		targetFor, stepID, gorp.LogLevelDebug, "GinkgoWriter", win,
	)
	if strings.TrimSpace(spec.CapturedStdOutErr) != "" {
		sendOutput(
			[]outputSegment{{step: -1, text: spec.CapturedStdOutErr, at: win.end}},
			targetFor, stepID, gorp.LogLevelInfo, "stdout/stderr", win,
		)
	}
}

// sendOutput cleans, caps and sends output segments. noteTarget receives a warning when
// the cap was hit.
func sendOutput(
	segments []outputSegment,
	targetFor func(int) string,
	noteTarget, level, stream string,
	win timeWindow,
) {
	budget := maxCapturedOutputBytes
	omitted := 0
	for _, seg := range segments {
		text := strings.TrimRight(stripANSI(strings.ToValidUTF8(seg.text, "")), "\r\n\t ")
		if strings.TrimSpace(text) == "" {
			continue
		}
		target := targetFor(seg.step)
		if target == "" {
			continue
		}
		kept := limitBytes(text, budget)
		omitted += len(text) - len(kept)
		budget -= len(kept)

		for _, chunk := range splitLogText(kept, maxLogChunkBytes) {
			globalListener.QueueLog(target, level, chunk, win.clamp(seg.at))
		}
	}
	if omitted > 0 && noteTarget != "" {
		note := "[" + stream + " output truncated: " + strconv.Itoa(omitted) + " bytes omitted]"
		globalListener.QueueLog(noteTarget, logLevelWarn, note, win.end)
	}
}
