package rpagent

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/reportportal/goRP/v5/pkg/gorp"
	"github.com/reportportal/goRP/v5/pkg/openapi"
)

func TestSuiteCacheRoundTripAndLookup(t *testing.T) {
	sid := fmt.Sprintf("test-%d", time.Now().UnixNano())
	path := filepath.Join(os.TempDir(), fmt.Sprintf(suiteCachePattern, sid))
	t.Cleanup(func() { _ = os.Remove(path) })

	if _, err := LoadSuiteCache(sid); err == nil {
		t.Error("loading a missing cache must fail")
	}
	dir := t.TempDir()
	if err := SaveSuiteCache(
		sid,
		&SuiteCache{Suites: map[string]string{dir: "suite-7"}},
	); err != nil {
		t.Fatal(err)
	}
	if id, err := GetSuiteIDForDirectory(sid, dir); err != nil || id != "suite-7" {
		t.Errorf("id=%q err=%v", id, err)
	}
	if _, err := GetSuiteIDForDirectory(sid, filepath.Join(dir, "other")); err == nil {
		t.Error("an unregistered directory must be an error")
	}
}

func TestLoadAgentState(t *testing.T) {
	sid := fmt.Sprintf("state-%d", time.Now().UnixNano())
	lc, err := NewLaunchCoordinator(sid)
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(os.TempDir(), fmt.Sprintf(StateFilePattern, sid))
	t.Cleanup(func() { _ = os.Remove(file) })

	if _, err := lc.LoadAgentState(); err == nil {
		t.Error("a missing state file must fail")
	}
	_ = os.WriteFile(file, []byte(`{"launchID":"","sessionID":"x"}`), 0o600)
	if _, err := lc.LoadAgentState(); err == nil {
		t.Error("a state without a launch ID must be rejected")
	}
	_ = os.WriteFile(
		file,
		[]byte(`{"endpoint":"e","project":"p","launchID":"L","sessionID":"`+sid+`"}`),
		0o600,
	)
	st, err := lc.LoadAgentState()
	if err != nil || st.LaunchID != "L" || st.Project != "p" {
		t.Errorf("state=%+v err=%v", st, err)
	}
	if _, err := NewLaunchCoordinator(""); err == nil {
		t.Error("an empty session ID must be rejected")
	}
}

func TestFinishSuiteAndLaunchLog(t *testing.T) {
	var mu sync.Mutex
	var requests []string
	tl := newTestListener(t, func(w http.ResponseWriter, r *http.Request) {
		var b map[string]any
		_ = json.NewDecoder(r.Body).Decode(&b)
		mu.Lock()
		requests = append(requests, fmt.Sprintf("%s %s %v", r.Method, r.URL.Path, b["status"]))
		mu.Unlock()
		writeJSON(w, `{}`)
	})
	tl.FinishSuite("s", gorp.Statuses.Failed, "")    // no ID: nothing is sent
	tl.FinishSuite("s", gorp.Statuses.Failed, "sid") // sent
	if err := tl.SendLaunchLog("INFO", "hello"); err != nil {
		t.Fatal(err)
	}
	if len(requests) != 2 || !strings.HasSuffix(requests[0], "/item/sid FAILED") ||
		!strings.Contains(requests[1], "/log") {
		t.Errorf("requests = %q", requests)
	}
}

func TestFinishOpenHierarchiesMarksLeftoversInterrupted(t *testing.T) {
	var mu sync.Mutex
	statuses := map[string]string{}
	tl := newTestListener(t, func(w http.ResponseWriter, r *http.Request) {
		var b struct{ Status string }
		_ = json.NewDecoder(r.Body).Decode(&b)
		mu.Lock()
		statuses[r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:]] = b.Status
		mu.Unlock()
		writeJSON(w, `{}`)
	})
	oldListener := globalListener
	globalListener = tl
	t.Cleanup(func() { globalListener = oldListener })

	activeHierarchies.Store("left over", SpecHierarchyIDs{
		StepID: "spec", ChildStepIDs: []string{"by"}, ChildStepNames: []string{"by step"},
	})
	finishOpenHierarchies()

	if statuses["spec"] != "INTERRUPTED" || statuses["by"] != "INTERRUPTED" {
		t.Errorf("statuses = %v, want both INTERRUPTED", statuses)
	}
	if _, still := activeHierarchies.Load("left over"); still {
		t.Error("the hierarchy must be removed once finished")
	}
}

func TestLaunchLink(t *testing.T) {
	link := func(s string) *string { return &s }
	tests := []struct {
		name string
		rs   *openapi.FinishLaunchRS
		want string
	}{
		{"no response", nil, ""},
		{"no link", &openapi.FinishLaunchRS{}, ""},
		{"empty link", &openapi.FinishLaunchRS{Link: link("")}, ""},
		{"blank link", &openapi.FinishLaunchRS{Link: link("  \n")}, ""},
		{
			"link",
			&openapi.FinishLaunchRS{Link: link("https://rp/ui/#p/launches/all/abc")},
			"https://rp/ui/#p/launches/all/abc",
		},
		// A launch number alone must not turn into a made-up URL.
		{
			"number only",
			&openapi.FinishLaunchRS{Number: func() *int64 { n := int64(7); return &n }()},
			"",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := launchLink(tt.rs); got != tt.want {
				t.Errorf("launchLink = %q, want %q", got, tt.want)
			}
		})
	}
}
