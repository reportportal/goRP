package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/reportportal/goRP/pkg/ginkgo/rpagent"

	"github.com/reportportal/goRP/v5/pkg/gorp"
	"github.com/reportportal/goRP/v5/pkg/openapi"
)

func TestBuildGinkgoArgs(t *testing.T) {
	skip := []string{"./rpagent"}
	cases := []struct {
		name string
		in   []string
		skip []string
		want []string
	}{
		{"no skip needed leaves args alone", []string{"-r", "./..."}, nil, []string{"-r", "./..."}},
		{"no args", nil, skip, []string{"--skip-package=./rpagent"}},
		{
			"flags only",
			[]string{"-r", "-p"},
			skip,
			[]string{"-r", "-p", "--skip-package=./rpagent"},
		},
		{
			"flags after paths move before them",
			[]string{"-r", "calculator/", "rp-features/", "--keep-going"},
			skip,
			[]string{
				"-r",
				"--keep-going",
				"--skip-package=./rpagent",
				"calculator/",
				"rp-features/",
			},
		},
		{
			"value flag stays paired",
			[]string{"--procs", "4", "./pkg/auth"},
			skip,
			[]string{"--procs", "4", "--skip-package=./rpagent", "./pkg/auth"},
		},
		{
			"merge into existing",
			[]string{"--skip-package=foo"},
			skip,
			[]string{"--skip-package=foo,./rpagent"},
		},
		{
			"merge space form",
			[]string{"--skip-package", "foo"},
			skip,
			[]string{"--skip-package=foo,./rpagent"},
		},
		{
			"already present is untouched",
			[]string{"--skip-package=foo,./rpagent", "./..."},
			skip,
			[]string{"--skip-package=foo,./rpagent", "./..."},
		},
		{
			// A user package merely containing the word must not be excluded.
			"user's own rpagent-named package is not skipped",
			[]string{"-r", "./myrpagent-tests"},
			nil,
			[]string{"-r", "./myrpagent-tests"},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := buildGinkgoArgs(c.in, c.skip); !reflect.DeepEqual(got, c.want) {
				t.Errorf("got %v, want %v", got, c.want)
			}
		})
	}
}

func TestAgentSkipPackages(t *testing.T) {
	// Ordinary project: nothing is skipped.
	plain := t.TempDir()
	if got := agentSkipPackages(plain); got != nil {
		t.Fatalf("plain project: got %v, want nil", got)
	}
	// A user project that merely has a package named rpagent: still nothing.
	if err := os.MkdirAll(filepath.Join(plain, "rpagent"), 0o700); err != nil {
		t.Fatal(err)
	}
	if got := agentSkipPackages(plain); got != nil {
		t.Fatalf("user rpagent dir: got %v, want nil", got)
	}
	// The agent's own module root.
	mod := t.TempDir()
	body := "module github.com/reportportal/goRP/pkg/ginkgo\n\ngo 1.25\n"
	if err := os.WriteFile(filepath.Join(mod, "go.mod"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := agentSkipPackages(mod); !reflect.DeepEqual(got, []string{"./rpagent"}) {
		t.Fatalf("module root: got %v", got)
	}
	// The goRP repo root, where the agent lives under pkg/ginkgo.
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "pkg", "ginkgo", "rpagent"), 0o700); err != nil {
		t.Fatal(err)
	}
	if got := agentSkipPackages(root); !reflect.DeepEqual(got, []string{"./pkg/ginkgo/rpagent"}) {
		t.Fatalf("repo root: got %v", got)
	}
}

func TestIsPackagePath(t *testing.T) {
	cases := map[string]bool{
		"./...": true, "...": true, "./pkg/auth": true, "../x": true, "calculator/": true,
		"--keep-going": false, "-r": false, "calculator": false,
	}
	for in, want := range cases {
		if got := isPackagePath(in); got != want {
			t.Errorf("isPackagePath(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestDetectTypo(t *testing.T) {
	if ok, hint := detectTypo("--h"); !ok || hint == "" {
		t.Errorf("--h should be flagged with a suggestion")
	}
	if ok, _ := detectTypo("--focus"); ok {
		t.Errorf("--focus is a real ginkgo flag and must pass through")
	}
}

func TestFinishContainers(t *testing.T) {
	type finish struct {
		id     string
		status string
		end    time.Time
		launch string
	}
	var mu sync.Mutex
	var finishes []finish
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var rq openapi.FinishTestItemRQ
		_ = json.NewDecoder(r.Body).Decode(&rq)
		mu.Lock()
		finishes = append(
			finishes,
			finish{
				r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:],
				*rq.Status,
				rq.EndTime,
				rq.LaunchUuid,
			},
		)
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	}))
	t.Cleanup(srv.Close)
	client := rpagent.NewReportingClient(srv.URL, "proj", "key", false)

	end := time.Date(2026, 1, 1, 12, 0, 5, 0, time.UTC)
	finishContainers(client, "launch-1", []rpagent.ContainerResult{
		{ID: "inner", Depth: 2, Status: gorp.Statuses.Failed, End: end},
		{ID: "outer", Depth: 1, Status: gorp.Statuses.Failed, End: end.Add(time.Second)},
		{ID: "no-time", Depth: 1, Status: gorp.Statuses.Skipped},
	})

	if len(finishes) != 3 {
		t.Fatalf("got %d finish calls: %+v", len(finishes), finishes)
	}
	if finishes[0].id != "inner" || finishes[1].id != "outer" {
		t.Errorf(
			"a child must finish before its parent, order = %s, %s",
			finishes[0].id,
			finishes[1].id,
		)
	}
	if finishes[0].status != "FAILED" || !finishes[0].end.Equal(end) ||
		finishes[0].launch != "launch-1" {
		t.Errorf("inner = %+v", finishes[0])
	}
	if !finishes[1].end.Equal(end.Add(time.Second)) {
		t.Errorf("outer must end with its last spec, got %v", finishes[1].end)
	}
	if finishes[2].status != "SKIPPED" || finishes[2].end.IsZero() {
		t.Errorf("a container with no recorded time must still get an end time: %+v", finishes[2])
	}
}

func TestFinishContainersWithNothingToFinish(t *testing.T) {
	// Must not panic or call out when there are no containers (top-level specs only).
	finishContainers(rpagent.NewReportingClient("http://127.0.0.1:1", "p", "k", false), "l", nil)
}

func TestSplitFixtureResults(t *testing.T) {
	all := []rpagent.SharedSpecStatus{
		{SpecName: "a", Status: "passed"},
		{SpecName: "[BeforeSuite]", Status: "failed", Fixture: true},
		{SpecName: "b", Status: "failed"},
	}
	specs, fixtures := splitFixtureResults(all)
	if len(specs) != 2 || specs[0].SpecName != "a" || specs[1].SpecName != "b" {
		t.Errorf("specs = %+v", specs)
	}
	if len(fixtures) != 1 || fixtures[0].SpecName != "[BeforeSuite]" {
		t.Errorf("fixtures = %+v", fixtures)
	}
}

func TestOverallResult(t *testing.T) {
	cases := []struct {
		name                                        string
		hasSuiteFailures                            bool
		specsFailed, passed, skipped, fixtureFailed int
		want                                        string
		wantForce                                   bool
	}{
		{"all passed", false, 0, 5, 0, 0, "ALL TESTS PASSED", false},
		{"a spec failed", true, 1, 4, 0, 0, "TESTS FAILED", false},
		{
			"specs failed and a fixture failed: still a test failure",
			true,
			1,
			4,
			0,
			1,
			"TESTS FAILED",
			false,
		},
		{"only skipped", false, 0, 0, 3, 0, "ALL TESTS SKIPPED", false},
		{
			"BeforeSuite failed, nothing ran",
			true,
			0,
			0,
			0,
			1,
			"SUITE SETUP FAILED (no tests executed)",
			true,
		},
		{
			"AfterSuite failed after passing specs: tests did run",
			true,
			0,
			2,
			0,
			1,
			"SUITE SETUP OR TEARDOWN FAILED (2 tests ran)",
			true,
		},
		{
			"nothing reported but ginkgo failed (compile error)",
			true,
			0,
			0,
			0,
			0,
			"SUITE SETUP FAILED (no tests executed)",
			true,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			lines, force := overallResult(
				c.hasSuiteFailures,
				c.specsFailed,
				c.passed,
				c.skipped,
				c.fixtureFailed,
			)
			if len(lines) == 0 || !strings.Contains(lines[0], c.want) || force != c.wantForce {
				t.Errorf("lines=%q force=%v; want %q force=%v", lines, force, c.want, c.wantForce)
			}
		})
	}
}

// The agent names its temp files and directories after the session; cleanup must remove all
// of one session's, and none of another's, even when one session ID is a prefix of the other.
func TestCleanupRemovesExactlyOneSessionsFiles(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TMPDIR", tmp)
	const a, b = "sess-A", "sess-AB" // a is a prefix of b

	touch := func(name string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(tmp, name), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	for _, s := range []string{a, b} {
		touch("rp_agent_state_" + s + ".json")
		touch("rp_report_index_" + s + ".json")
		touch("rp_status_" + s + ".json")
		touch("rp_status_" + s + "_4242.json")
		touch("rp_done_" + s + "_4242.flag")
		touch("rp_suites_" + s + ".json")
		touch("rp_cache_" + s + "_deadbeef.lock")
	}

	// The real agent code creates the cache files and the attachment directory.
	t.Setenv("RP_SESSION_ID", a)
	lcA, err := rpagent.NewLaunchCoordinator(a)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := lcA.NewContainerCacheManager().
		GetOrCreate(t.Name()+":a", func() string { return "item-a" }); err != nil {
		t.Fatal(err)
	}
	rpagent.InitializeAttachments()
	t.Cleanup(rpagent.CleanupAttachments)

	t.Setenv("RP_SESSION_ID", b)
	lcB, err := rpagent.NewLaunchCoordinator(b)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := lcB.NewContainerCacheManager().
		GetOrCreate(t.Name()+":b", func() string { return "item-b" }); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(tmp, "rp-agent-attachments-"+b+"-123456"), 0o700); err != nil {
		t.Fatal(err)
	}

	list := func() (mine, others []string) {
		entries, err := os.ReadDir(tmp)
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range entries {
			if strings.Contains(e.Name(), b) {
				others = append(others, e.Name())
			} else {
				mine = append(mine, e.Name())
			}
		}
		return mine, others
	}
	mine, others := list()
	if len(mine) < 9 || len(others) < 9 {
		t.Fatalf(
			"the test must create files for both sessions, got %d and %d: %v",
			len(mine),
			len(others),
			mine,
		)
	}

	cleanup(a)

	mine, afterOthers := list()
	if len(mine) != 0 {
		t.Errorf("files of the cleaned session were left behind: %v", mine)
	}
	if len(afterOthers) != len(others) {
		t.Errorf("another session's files were removed: before %v, after %v", others, afterOthers)
	}
}
