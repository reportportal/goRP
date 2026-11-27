package rpagent

import (
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/onsi/ginkgo/v2/types"
)

func TestStripANSI(t *testing.T) {
	cases := map[string]string{
		"plain":                            "plain",
		"\x1b[31mred\x1b[0m text":          "red text",
		"\x1b[1;38;5;243mbold grey\x1b[0m": "bold grey",
		"cursor\x1b[2K\x1b[1Gline":         "cursorline",
		"":                                 "",
	}
	for in, want := range cases {
		if got := stripANSI(in); got != want {
			t.Errorf("stripANSI(%q) = %q, want %q", in, got, want)
		}
	}
}

func plansAt(offsets ...int) []stepPlan {
	plans := make([]stepPlan, len(offsets))
	for i, off := range offsets {
		plans[i] = stepPlan{name: "s", order: i + 1, offset: off, start: at(10 * (i + 1))}
	}
	return plans
}

func TestSegmentWriterOutput(t *testing.T) {
	win := timeWindow{start: at(0), end: at(100)}

	t.Run("empty output has no segments", func(t *testing.T) {
		if got := segmentWriterOutput("", plansAt(3), win); got != nil {
			t.Errorf("got %+v", got)
		}
	})

	t.Run("no steps: everything belongs to the spec item", func(t *testing.T) {
		got := segmentWriterOutput("hello\nworld\n", nil, win)
		if len(got) != 1 || got[0].step != -1 || got[0].text != "hello\nworld\n" ||
			!got[0].at.Equal(at(0)) {
			t.Errorf("got %+v", got)
		}
	})

	t.Run("offsets from a real Ginkgo run route text to the right step", func(t *testing.T) {
		// Captured from Ginkgo 2.26: By("step one") had offset 16, By("step two") offset 32.
		out := "before any step\ninside step one\ninside step two, no newline\ncolored? end\n"
		got := segmentWriterOutput(out, plansAt(16, 32), win)
		want := []outputSegment{
			{step: -1, text: "before any step\n", at: at(0)},
			{step: 0, text: "inside step one\n", at: at(10)},
			{step: 1, text: "inside step two, no newline\ncolored? end\n", at: at(20)},
		}
		if len(got) != len(want) {
			t.Fatalf("got %+v", got)
		}
		for i := range want {
			if got[i].step != want[i].step || got[i].text != want[i].text ||
				!got[i].at.Equal(want[i].at) {
				t.Errorf("segment %d = %+v, want %+v", i, got[i], want[i])
			}
		}
	})

	t.Run(
		"offsets outside the output or out of order are clamped, never panic",
		func(t *testing.T) {
			out := "0123456789"
			got := segmentWriterOutput(
				out,
				plansAt(50, 4, -3),
				win,
			) // beyond the end, going backwards, negative
			var joined string
			for _, s := range got {
				joined += s.text
			}
			if joined != out {
				t.Errorf("segments must cover the output exactly once, got %q", joined)
			}
		},
	)

	t.Run("no output is lost or duplicated for any offsets", func(t *testing.T) {
		out := "abcdefghij"
		for a := -2; a <= 12; a++ {
			for b := -2; b <= 12; b++ {
				var joined string
				for _, s := range segmentWriterOutput(out, plansAt(a, b), win) {
					joined += s.text
				}
				if joined != out {
					t.Fatalf("offsets (%d,%d): got %q", a, b, joined)
				}
			}
		}
	})
}

func TestSplitLogText(t *testing.T) {
	t.Run("short text is one chunk without the trailing newline", func(t *testing.T) {
		if got := splitLogText("a\nb\n", 100); len(got) != 1 || got[0] != "a\nb" {
			t.Errorf("got %q", got)
		}
	})
	t.Run("splits at line boundaries", func(t *testing.T) {
		got := splitLogText("aaaa\nbbbb\ncccc\n", 10)
		if len(got) != 2 || got[0] != "aaaa\nbbbb" || got[1] != "cccc" {
			t.Errorf("got %q", got)
		}
	})
	t.Run("an over-long line is hard split on rune boundaries", func(t *testing.T) {
		line := strings.Repeat("é", 11) // 22 bytes
		got := splitLogText(line, 5)
		if strings.Join(got, "") != line {
			t.Errorf("content changed: %q", got)
		}
		for _, c := range got {
			if len(c) > 5 || !utf8.ValidString(c) {
				t.Errorf("bad chunk %q", c)
			}
		}
	})
	t.Run("nothing in, nothing out", func(t *testing.T) {
		if got := splitLogText("", 10); len(got) != 0 {
			t.Errorf("got %q", got)
		}
	})
}

func TestLimitBytes(t *testing.T) {
	if got := limitBytes("abc", 10); got != "abc" {
		t.Errorf("under budget must be unchanged, got %q", got)
	}
	if got := limitBytes("aaaa\nbbbb\ncccc", 11); got != "aaaa\nbbbb" {
		t.Errorf("must cut at a line boundary, got %q", got)
	}
	if got := limitBytes("anything", 0); got != "" {
		t.Errorf("no budget, got %q", got)
	}
	if got := limitBytes(strings.Repeat("é", 10), 5); !utf8.ValidString(got) {
		t.Errorf("must not cut a rune: %q", got)
	}
}

type sentLog struct {
	item, level, msg string
	at               time.Time
}

// outputHarness points the package globals at a mock server and records every log.
func outputHarness(t *testing.T) func() []sentLog {
	t.Helper()
	var mu sync.Mutex
	var logs []sentLog
	var seq int
	tl := newTestListener(t, func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		switch {
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/log"):
			for _, rq := range decodeLogRQs(t, r) {
				logs = append(logs, sentLog{*rq.ItemUuid, *rq.Level, *rq.Message, rq.Time})
			}
			writeJSON(w, `{}`)
		case r.Method == http.MethodPost:
			seq++
			writeJSON(w, `{"id":"step-`+string(rune('0'+seq))+`"}`)
		default:
			writeJSON(w, `{}`)
		}
	})
	oldClient, oldListener := globalClient, globalListener
	globalClient, globalListener = tl.client, tl
	t.Cleanup(func() { globalClient, globalListener = oldClient, oldListener })
	return func() []sentLog {
		mu.Lock()
		defer mu.Unlock()
		return append([]sentLog(nil), logs...)
	}
}

func byStartAtOffset(msg string, order, ms, offset int) types.SpecEvent {
	e := byStart(msg, order, ms)
	e.TimelineLocation.Offset = offset
	return e
}

func finishWithOutput(t *testing.T, spec types.SpecReport) {
	t.Helper()
	activeHierarchies.Store(
		spec.FullText(),
		SpecHierarchyIDs{StepID: "spec-item", StartTime: at(0)},
	)
	t.Cleanup(func() { activeHierarchies.Delete(spec.FullText()) })
	finishSpecHierarchy(spec)
}

func TestCapturedOutputIsRoutedCleanedAndTimed(t *testing.T) {
	logsOf := outputHarness(t)

	// "before\n" is 7 bytes, so step "one" starts at offset 7; its text is 20 bytes
	// (including ANSI codes), so step "two" starts at offset 27.
	finishWithOutput(t, types.SpecReport{
		ContainerHierarchyTexts:    []string{"C"},
		LeafNodeText:               "writes",
		State:                      types.SpecStatePassed,
		StartTime:                  at(0),
		EndTime:                    at(100),
		CapturedGinkgoWriterOutput: "before\n\x1b[31minside one\x1b[0m\ninside two\n",
		CapturedStdOutErr:          "stdout line\n",
		SpecEvents: []types.SpecEvent{
			byStartAtOffset("one", 2, 10, 7),
			byStartAtOffset("two", 4, 40, 27),
		},
	})

	got := map[string]sentLog{}
	for _, l := range logsOf() {
		got[l.msg] = l
	}
	check := func(msg, item, level string, when time.Time) {
		t.Helper()
		l, ok := got[msg]
		if !ok {
			t.Errorf("no log %q was sent (got %v)", msg, logsOf())
			return
		}
		if l.item != item || l.level != level || !l.at.Equal(when) {
			t.Errorf(
				"%q: item=%s level=%s at=%v; want item=%s level=%s at=%v",
				msg,
				l.item,
				l.level,
				l.at,
				item,
				level,
				when,
			)
		}
	}
	check("before", "spec-item", "DEBUG", at(0))
	check("inside one", "step-1", "DEBUG", at(10)) // ANSI codes stripped
	check("inside two", "step-2", "DEBUG", at(40))
	check("stdout line", "spec-item", "INFO", at(100))
}

func TestCapturedOutputIsCapped(t *testing.T) {
	logsOf := outputHarness(t)

	line := strings.Repeat("0123456789abcdef", 4) + "\n" // 65 bytes
	huge := strings.Repeat(line, 20000)                  // ~1.3 MB, over the 1 MiB cap
	finishWithOutput(t, types.SpecReport{
		LeafNodeText:               "chatty",
		State:                      types.SpecStatePassed,
		StartTime:                  at(0),
		EndTime:                    at(100),
		CapturedGinkgoWriterOutput: huge,
	})

	var sent int
	var warned bool
	for _, l := range logsOf() {
		if l.level == "DEBUG" {
			sent += len(l.msg)
			if len(l.msg) > maxLogChunkBytes {
				t.Fatalf(
					"chunk of %d bytes exceeds the %d byte limit",
					len(l.msg),
					maxLogChunkBytes,
				)
			}
		}
		if l.level == "WARN" && strings.Contains(l.msg, "truncated") {
			warned = true
		}
	}
	if sent > maxCapturedOutputBytes || sent < maxCapturedOutputBytes*9/10 {
		t.Errorf(
			"sent %d bytes of output, want just under the %d cap",
			sent,
			maxCapturedOutputBytes,
		)
	}
	if !warned {
		t.Errorf("hitting the cap must be reported with a warning log")
	}
}

func TestNoCapturedOutputSendsNothing(t *testing.T) {
	logsOf := outputHarness(t)
	finishWithOutput(t, types.SpecReport{
		LeafNodeText:               "quiet",
		State:                      types.SpecStatePassed,
		StartTime:                  at(0),
		EndTime:                    at(100),
		CapturedGinkgoWriterOutput: "  \n\n",
		CapturedStdOutErr:          "\n",
	})
	if logs := logsOf(); len(logs) != 0 {
		t.Errorf("whitespace-only output must not create logs: %+v", logs)
	}
}

// Ginkgo captures stdout/stderr for the whole spec and cannot say which attempt wrote what,
// so for a spec that ran more than once the log says it covers all of them. A spec that ran
// once needs no such note.
func TestStdoutOfARepeatedSpecIsLabeled(t *testing.T) {
	for _, tc := range []struct {
		name        string
		numAttempts int
		wantLabel   string // "" = the log must be the bare output
	}{
		{"ran once", 1, ""},
		{"attempts not recorded", 0, ""},
		{"three attempts", 3, "[stdout/stderr of all 3 attempts]\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			logsOf := outputHarness(t)
			finishWithOutput(t, types.SpecReport{
				LeafNodeText:      "prints " + tc.name,
				State:             types.SpecStatePassed,
				StartTime:         at(0),
				EndTime:           at(100),
				NumAttempts:       tc.numAttempts,
				CapturedStdOutErr: "hello from stdout\n",
			})
			logs := logsOf()
			if len(logs) != 1 || logs[0].level != "INFO" {
				t.Fatalf("logs = %+v, want one INFO log", logs)
			}
			if want := tc.wantLabel + "hello from stdout"; logs[0].msg != want {
				t.Errorf("log = %q, want %q", logs[0].msg, want)
			}
		})
	}
}
