package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"syscall"
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
	err := finishContainers(context.Background(), client, "launch-1", []rpagent.ContainerResult{
		{ID: "inner", Depth: 2, Status: gorp.Statuses.Failed, End: end},
		{ID: "outer", Depth: 1, Status: gorp.Statuses.Failed, End: end.Add(time.Second)},
		{ID: "no-time", Depth: 1, Status: gorp.Statuses.Skipped},
	}, &failureStreak{})
	if err != nil {
		t.Fatalf("finishContainers: %v", err)
	}

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
	client := rpagent.NewReportingClient("http://127.0.0.1:1", "p", "k", false)
	if err := finishContainers(
		context.Background(),
		client,
		"l",
		nil,
		&failureStreak{},
	); err != nil {
		t.Errorf("nothing to finish must not fail: %v", err)
	}
}

func TestFinishContainersGivesUpAfterConsecutiveFailures(t *testing.T) {
	var requests int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.WriteHeader(http.StatusBadGateway)
	}))
	t.Cleanup(srv.Close)
	client := rpagent.NewReportingClient(srv.URL, "proj", "key", false)

	var containers []rpagent.ContainerResult
	for i := 0; i < 20; i++ {
		containers = append(
			containers,
			rpagent.ContainerResult{ID: fmt.Sprintf("c%d", i), Status: gorp.Statuses.Passed},
		)
	}
	err := finishContainers(context.Background(), client, "l", containers, &failureStreak{})
	if !errors.Is(err, errReportPortalNotResponding) {
		t.Fatalf("error = %v, want errReportPortalNotResponding", err)
	}
	if requests != maxConsecutiveFinishFailures {
		t.Errorf("sent %d requests for 20 containers, want it to stop after %d failures",
			requests, maxConsecutiveFinishFailures)
	}
}

func TestFinishContainersStopsWhenCancelled(t *testing.T) {
	var requests int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		_, _ = w.Write([]byte(`{}`))
	}))
	t.Cleanup(srv.Close)
	client := rpagent.NewReportingClient(srv.URL, "proj", "key", false)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := finishContainers(ctx, client, "l",
		[]rpagent.ContainerResult{{ID: "a", Status: gorp.Statuses.Passed}}, &failureStreak{})
	if !errors.Is(err, context.Canceled) || requests != 0 {
		t.Errorf("err = %v, requests = %d, want context.Canceled and no request", err, requests)
	}
}

func TestFailureStreakResetsOnSuccess(t *testing.T) {
	s := &failureStreak{}
	boom := errors.New("boom")
	for i := 0; i < maxConsecutiveFinishFailures-1; i++ {
		if err := s.note(boom); err != nil {
			t.Fatalf("gave up after %d failures", i+1)
		}
	}
	_ = s.note(nil) // a success starts the count again
	for i := 0; i < maxConsecutiveFinishFailures-1; i++ {
		if err := s.note(boom); err != nil {
			t.Fatalf("a success must reset the streak, gave up after %d more", i+1)
		}
	}
	if err := s.note(boom); !errors.Is(err, errReportPortalNotResponding) {
		t.Errorf("err = %v, want errReportPortalNotResponding", err)
	}
}

func TestFinalizeTimeout(t *testing.T) {
	tests := []struct {
		env  string
		want time.Duration
	}{
		{"", defaultFinalizeTimeout},
		{"90s", 90 * time.Second},
		{"5m", 5 * time.Minute},
		{"0s", defaultFinalizeTimeout},
		{"-1m", defaultFinalizeTimeout},
		{"soon", defaultFinalizeTimeout},
		{"120", defaultFinalizeTimeout}, // a bare number has no unit
	}
	for _, tt := range tests {
		t.Run(tt.env, func(t *testing.T) {
			t.Setenv(finalizeTimeoutEnv, tt.env)
			if got := finalizeTimeout(); got != tt.want {
				t.Errorf("finalizeTimeout(%q) = %v, want %v", tt.env, got, tt.want)
			}
		})
	}
}

func TestWatchSignalsCancelsOnFirstAndAllOnSecond(t *testing.T) {
	sigs := make(chan os.Signal, 2)
	finalize, cancelFinalize := context.WithCancel(context.Background())
	all, cancelAll := context.WithCancel(context.Background())
	defer cancelFinalize()
	defer cancelAll()
	stop := watchSignals(sigs, cancelFinalize, cancelAll)

	sigs <- os.Interrupt
	select {
	case <-finalize.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("the first signal did not cancel finalization")
	}
	if all.Err() != nil {
		t.Fatal("the first signal must not cancel the fallback")
	}
	sigs <- os.Interrupt
	select {
	case <-all.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("the second signal did not cancel everything")
	}
	if got := stop(); got != os.Interrupt {
		t.Errorf("stop() = %v, want the first signal", got)
	}
}

func TestWatchSignalsReportsNothingWhenNoSignal(t *testing.T) {
	stop := watchSignals(make(chan os.Signal), func() {}, func() {})
	if got := stop(); got != nil {
		t.Errorf("stop() = %v, want nil", got)
	}
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

func TestFailureStreakCountsOnlyServerTroubleNotRejections(t *testing.T) {
	http := func(code int) error { return &gorp.HTTPError{StatusCode: code} }
	refused := &net.OpError{Op: "dial", Err: syscall.ECONNREFUSED}
	tests := []struct {
		name     string
		errs     []error
		wantStop bool
	}{
		{"three 500s in a row", []error{http(500), http(500), http(500)}, true},
		{"three 503s in a row", []error{http(503), http(503), http(503)}, true},
		{"three refused connections", []error{refused, refused, refused}, true},
		{
			"a wrapped 502 counts",
			[]error{fmt.Errorf("x: %w", http(502)), http(502), http(502)},
			true,
		},
		{"mixed server trouble", []error{http(500), refused, http(504)}, true},
		{
			"ten rejected items never add up",
			[]error{
				http(
					400,
				),
				http(404),
				http(409),
				http(400),
				http(404),
				http(409),
				http(400),
				http(404),
				http(409),
				http(400),
			},
			false,
		},
		{
			"rate limiting is a server answer too",
			[]error{http(429), http(429), http(429), http(429)},
			false,
		},
		{
			"a wrapped 404 does not count",
			[]error{fmt.Errorf("x: %w", http(404)), http(404), http(404), http(404)},
			false,
		},
		{
			"an answer in between ends the streak",
			[]error{http(500), http(500), http(404), http(500), http(500)},
			false,
		},
		{
			"a success in between ends the streak",
			[]error{http(500), http(500), nil, http(500), http(500)},
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &failureStreak{}
			stopped := false
			for _, err := range tt.errs {
				if gerr := s.note(err); errors.Is(gerr, errReportPortalNotResponding) {
					stopped = true
				}
			}
			if stopped != tt.wantStop {
				t.Errorf("gave up = %v, want %v", stopped, tt.wantStop)
			}
		})
	}
}

func TestFinishContainersKeepsGoingWhenItemsAreRejected(t *testing.T) {
	var requests int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.WriteHeader(http.StatusNotFound) // the item does not exist: a rejection, not an outage
	}))
	t.Cleanup(srv.Close)
	client := rpagent.NewReportingClient(srv.URL, "proj", "key", false)

	var containers []rpagent.ContainerResult
	for i := 0; i < 12; i++ {
		containers = append(
			containers,
			rpagent.ContainerResult{ID: fmt.Sprintf("c%d", i), Status: gorp.Statuses.Passed},
		)
	}
	if err := finishContainers(
		context.Background(),
		client,
		"l",
		containers,
		&failureStreak{},
	); err != nil {
		t.Errorf("rejected items must not stop finalization: %v", err)
	}
	if requests != len(containers) {
		t.Errorf(
			"sent %d requests, want one for each of the %d containers",
			requests,
			len(containers),
		)
	}
}
