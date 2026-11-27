package rpagent

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/reportportal/goRP/v5/pkg/gorp"
	"github.com/reportportal/goRP/v5/pkg/openapi"
)

// With rp.debug on, every exchange with ReportPortal is written to rp-agent.log, bodies
// included and the API key never.
func TestDebugClientLogsExchangesWithoutTheAPIKey(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module x\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Chdir(root)
	if err := InitializeRPLogger(); err != nil {
		t.Fatal(err)
	}

	srv := newTestServer(
		t,
		func(w http.ResponseWriter, r *http.Request) { writeJSON(w, `{"id":"abc"}`) },
	)
	client := NewReportingClient(srv.URL+"/api/", "proj", "SECRET-API-KEY", true)
	if _, err := client.StartLaunch(
		context.Background(), &openapi.StartLaunchRQ{Name: "n", StartTime: time.Now()},
	); err != nil {
		t.Fatalf("start launch: %v", err)
	}
	CloseRPLogger()

	b, err := os.ReadFile(filepath.Join(root, "rp-agent.log"))
	if err != nil {
		t.Fatal(err)
	}
	logged := string(b)
	if !strings.Contains(logged, "HTTP POST") || !strings.Contains(logged, `"id": "abc"`) {
		t.Errorf("the exchange was not logged:\n%s", logged)
	}
	if strings.Contains(logged, "SECRET-API-KEY") {
		t.Errorf("the API key leaked into rp-agent.log:\n%s", logged)
	}
}

func TestDebugClientLogsFailedExchanges(t *testing.T) {
	root := t.TempDir()
	_ = os.WriteFile(filepath.Join(root, "go.mod"), []byte("module x\n"), 0o600)
	t.Chdir(root)
	if err := InitializeRPLogger(); err != nil {
		t.Fatal(err)
	}
	client := NewReportingClient("http://127.0.0.1:1", "proj", "k", true) // nothing listens there
	_, err := client.FinishLaunch(
		context.Background(),
		"L",
		&openapi.FinishExecutionRQ{EndTime: time.Now()},
	)
	CloseRPLogger()
	if err == nil {
		t.Fatal("expected a connection error")
	}
	b, _ := os.ReadFile(filepath.Join(root, "rp-agent.log"))
	if !strings.Contains(string(b), "HTTP PUT") {
		t.Errorf("the failed exchange was not logged:\n%s", b)
	}
}

func TestLockRetryConfigFromEnvironment(t *testing.T) {
	t.Setenv("RP_LOCK_RETRY_COUNT", "")
	t.Setenv("RP_LOCK_RETRY_SLEEP_MS", "")
	if n, d := getLockRetryConfig(); n != 50 || d != 200*time.Millisecond {
		t.Errorf("defaults = %d, %v", n, d)
	}
	t.Setenv("RP_LOCK_RETRY_COUNT", "7")
	t.Setenv("RP_LOCK_RETRY_SLEEP_MS", "30")
	if n, d := getLockRetryConfig(); n != 7 || d != 30*time.Millisecond {
		t.Errorf("configured = %d, %v", n, d)
	}
	t.Setenv("RP_LOCK_RETRY_COUNT", "0")
	t.Setenv("RP_LOCK_RETRY_SLEEP_MS", "abc")
	if n, d := getLockRetryConfig(); n != 50 || d != 200*time.Millisecond {
		t.Errorf("unusable values must fall back to the defaults, got %d, %v", n, d)
	}
}

func TestAppendSpecStatusWritesOneJSONLinePerSpec(t *testing.T) {
	t.Setenv("TMPDIR", t.TempDir())
	t.Setenv("RP_SESSION_ID", "append-test")
	oldState := agentState
	agentState = &AgentState{SessionID: "append-test"}
	t.Cleanup(func() { agentState = oldState })

	appendSpecStatus(SharedSpecStatus{SpecName: "one", Status: "passed"})
	appendSpecStatus(SharedSpecStatus{SpecName: "two", Status: "failed", FailureMessage: "boom"})

	file := filepath.Join(
		os.TempDir(),
		fmt.Sprintf(ProcessReportFilePattern, "append-test", os.Getpid()),
	)
	b, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(b)), "\n")
	if len(lines) != 2 {
		t.Fatalf("status lines = %d, want 2:\n%s", len(lines), b)
	}
	var second SharedSpecStatus
	if err := json.Unmarshal(
		[]byte(lines[1]),
		&second,
	); err != nil || second.SpecName != "two" ||
		second.FailureMessage != "boom" {
		t.Errorf("second line = %+v, %v", second, err)
	}

	// Without the agent's state nothing is written (a plain Ginkgo run).
	agentState = nil
	appendSpecStatus(SharedSpecStatus{SpecName: "three"})
	if b2, _ := os.ReadFile(file); strings.Contains(string(b2), "three") {
		t.Error("a spec was recorded although the agent is not active")
	}
}

func TestSuiteCacheErrorsAreReported(t *testing.T) {
	t.Setenv("TMPDIR", t.TempDir())
	if _, err := GetSuiteIDForDirectory("no-such-session", t.TempDir()); err == nil {
		t.Error("a missing suite cache must be an error")
	}
	if err := SaveSuiteCache("s", &SuiteCache{Suites: map[string]string{"/a": "1"}}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(os.TempDir(), fmt.Sprintf(suiteCachePattern, "s"))
	_ = os.WriteFile(path, []byte("{not json"), 0o600)
	if _, err := LoadSuiteCache("s"); err == nil {
		t.Error("a corrupt suite cache must be an error")
	}
	_ = os.WriteFile(path, []byte(`{}`), 0o600) // no "suites" key
	if c, err := LoadSuiteCache("s"); err != nil || c.Suites == nil {
		t.Errorf("an empty cache must load with a usable map: %+v, %v", c, err)
	}
	t.Setenv("TMPDIR", filepath.Join(t.TempDir(), "gone"))
	if err := SaveSuiteCache("s", &SuiteCache{}); err == nil {
		t.Error("saving into a missing directory must fail, not panic")
	}
	lc, _ := NewLaunchCoordinator("x")
	if err := lc.SetAggregatedResults(nil); err == nil {
		t.Error("writing the results into a missing directory must fail")
	}
}

func TestFindProjectRootWalksUpAndFailsWithoutGoMod(t *testing.T) {
	root := t.TempDir()
	deep := filepath.Join(root, "a", "b")
	_ = os.MkdirAll(deep, 0o750)
	_ = os.WriteFile(filepath.Join(root, "go.mod"), []byte("module x\n"), 0o600)
	t.Chdir(deep)
	if got, err := findProjectRoot(); err != nil || filepathBase(got) != filepathBase(root) {
		t.Errorf("findProjectRoot = %q, %v, want %q", got, err, root)
	}
}

func filepathBase(p string) string { return filepath.Base(filepath.Clean(p)) }

func TestFinishItemGivesUpAfterRetriesWithoutPanicking(t *testing.T) {
	var calls int
	tl := newTestListener(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusServiceUnavailable)
	})
	start := time.Now()
	tl.FinishTest("x", gorp.Statuses.Passed, "id", time.Time{})
	if calls != 3 {
		t.Errorf("finish attempts = %d, want 3", calls)
	}
	if time.Since(start) > 30*time.Second {
		t.Error("retrying took far too long")
	}
	tl.FinishSuite("suite", gorp.Statuses.Failed, "id") // also gives up quietly
}

func TestSliceStringBounds(t *testing.T) {
	for _, tt := range []struct {
		s        string
		from, to int
		want     string
	}{
		{"abcdef", 1, 3, "bc"},
		{"abcdef", -5, 2, "ab"},
		{"abcdef", 2, 99, "cdef"},
		{"abcdef", 4, 2, ""},
		{"abcdef", 3, 3, ""},
	} {
		if got := sliceString(tt.s, tt.from, tt.to); got != tt.want {
			t.Errorf("sliceString(%q, %d, %d) = %q, want %q", tt.s, tt.from, tt.to, got, tt.want)
		}
	}
}
