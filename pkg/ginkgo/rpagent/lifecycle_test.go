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

	"github.com/onsi/ginkgo/v2/types"

	"github.com/reportportal/goRP/v5/pkg/gorp"
)

func TestAttemptFailuresAreRecordedAndTakenOnce(t *testing.T) {
	spec := types.SpecReport{LeafNodeText: "flaky-record"}
	recordAttemptFailure(spec) // no failure: ignored
	spec.Failure = types.Failure{Message: "one", TimelineLocation: types.TimelineLocation{Order: 3}}
	recordAttemptFailure(spec)
	spec.Failure.Message = "two"
	recordAttemptFailure(spec)

	got := takeAttemptFailures(spec)
	if len(got) != 2 || got[0].Message != "one" || got[1].Message != "two" {
		t.Errorf("failures = %+v", got)
	}
	if again := takeAttemptFailures(spec); len(again) != 0 {
		t.Errorf("failures must be handed out once, got %+v", again)
	}
}

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
