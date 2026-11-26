//go:build !windows

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

// fakeRP is a minimal ReportPortal: it hands out IDs and records what the agent sent.
type fakeRP struct {
	srv *httptest.Server

	mu          sync.Mutex
	items       map[string]string // id -> name
	starts      []string          // item names in start order
	finishes    []finish          // item finishes in order
	launchState string            // status sent on launch finish, "" until finished
	launchSeen  bool
	logs        int
	uploads     []string // file names of multipart attachment uploads
	logTexts    []string
	order       []string // "item-finish:<id>" / "launch-finish" in request order
	failFinish  bool     // answer launch finish with 500
	seq         int
}

type finish struct{ ID, Name, Status string }

func newFakeRP(t *testing.T) *fakeRP {
	t.Helper()
	f := &fakeRP{items: map[string]string{}}
	f.srv = httptest.NewServer(http.HandlerFunc(f.handle))
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fakeRP) handle(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	body, _ := io.ReadAll(r.Body)
	path := r.URL.Path
	w.Header().Set("Content-Type", "application/json")
	reply := func(s string) { _, _ = w.Write([]byte(s)) }
	next := func() string { f.seq++; return fmt.Sprintf("id-%d", f.seq) }

	switch {
	case r.Method == http.MethodPost && strings.HasSuffix(path, "/launch"):
		f.launchSeen = true
		reply(`{"id":"launch-1"}`)
	case r.Method == http.MethodPut && strings.HasSuffix(path, "/finish"):
		if f.failFinish {
			w.WriteHeader(http.StatusInternalServerError)
			reply(`{"message":"boom"}`)
			return
		}
		var rq struct{ Status string }
		_ = json.Unmarshal(body, &rq)
		f.launchState = rq.Status
		f.order = append(f.order, "launch-finish")
		reply(`{"id":"launch-1","number":1}`)
	case r.Method == http.MethodPost && strings.HasSuffix(path, "/log"):
		f.logs++
		f.recordLog(r, body)
		reply(`{}`)
	case r.Method == http.MethodPost && strings.Contains(path, "/item"):
		var rq struct{ Name string }
		_ = json.Unmarshal(body, &rq)
		id := next()
		f.items[id] = rq.Name
		f.starts = append(f.starts, rq.Name)
		reply(fmt.Sprintf(`{"id":%q}`, id))
	case r.Method == http.MethodPut && strings.Contains(path, "/item/"):
		var rq struct{ Status string }
		_ = json.Unmarshal(body, &rq)
		id := path[strings.LastIndex(path, "/")+1:]
		f.finishes = append(f.finishes, finish{id, f.items[id], rq.Status})
		f.order = append(f.order, "item-finish:"+f.items[id])
		reply(fmt.Sprintf(`{"message":"ok %s"}`, id))
	default:
		reply(`{}`)
	}
}

// recordLog notes the messages and attached file names of a (batch or multipart) log request.
func (f *fakeRP) recordLog(r *http.Request, body []byte) {
	_, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || params["boundary"] == "" {
		return
	}
	mr := multipart.NewReader(bytes.NewReader(body), params["boundary"])
	for {
		part, err := mr.NextPart()
		if err != nil {
			return
		}
		data, _ := io.ReadAll(part)
		if name := part.FileName(); name != "" && part.FormName() != "json_request_part" {
			f.uploads = append(f.uploads, name)
			continue
		}
		var logs []struct{ Message string }
		if json.Unmarshal(data, &logs) == nil {
			for _, l := range logs {
				f.logTexts = append(f.logTexts, l.Message)
			}
		}
	}
}

func (f *fakeRP) started(name string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, s := range f.starts {
		if s == name {
			return true
		}
	}
	return false
}

func (f *fakeRP) finishStatus(name string) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, fi := range f.finishes {
		if fi.Name == name {
			return fi.Status
		}
	}
	return ""
}

func (f *fakeRP) launch() (seen bool, status string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.launchSeen, f.launchState
}

// ginkgoBinDir builds the real ginkgo CLI once, so the tests need nothing installed.
var (
	ginkgoOnce sync.Once
	ginkgoDir  string
	ginkgoErr  error
)

func ginkgoBin(t *testing.T) string {
	t.Helper()
	ginkgoOnce.Do(func() {
		ginkgoDir, ginkgoErr = os.MkdirTemp("", "ginkgo-bin")
		if ginkgoErr != nil {
			return
		}
		out, err := exec.Command("go", "build", "-o",
			filepath.Join(ginkgoDir, "ginkgo"), "github.com/onsi/ginkgo/v2/ginkgo").CombinedOutput()
		if err != nil {
			ginkgoErr = fmt.Errorf("%v: %s", err, out)
		}
	})
	if ginkgoErr != nil {
		t.Skipf("cannot build the ginkgo CLI: %v", ginkgoErr)
	}
	return ginkgoDir
}

// runWrapper runs the wrapper in-process against the fixture suite and returns its exit
// code and what it printed. env entries are set for this run only.
func runWrapper(t *testing.T, endpoint string, env map[string]string) int {
	t.Helper()
	if testing.Short() {
		t.Skip("runs real ginkgo; skipped with -short")
	}
	bin := ginkgoBin(t)

	props := filepath.Join(t.TempDir(), "reportportal.properties")
	content := fmt.Sprintf(
		"rp.endpoint = %s\nrp.api.key = test-key\nrp.project = proj\nrp.launch = e2e\n",
		endpoint,
	)
	if err := os.WriteFile(props, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("RP_CONFIG_FILE", props)
	t.Setenv("RP_SESSION_ID", "") // run() sets it; restore afterwards
	t.Setenv("RP_STRICT", "")
	t.Setenv("FIXTURE_FAIL", "")
	t.Setenv("FIXTURE_SLEEP", "")
	t.Setenv("FIXTURE_ATTACH", "")
	for k, v := range env {
		t.Setenv(k, v)
	}

	oldArgs := os.Args
	os.Args = []string{"ginkgo-rp", "run", "./testdata/fixture"}
	t.Cleanup(func() { os.Args = oldArgs })
	return run()
}

func TestWrapperReportsPassingRun(t *testing.T) {
	rp := newFakeRP(t)
	if code := runWrapper(t, rp.srv.URL, nil); code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !rp.started("passes") || rp.finishStatus("passes") != "PASSED" {
		t.Errorf(
			"spec 'passes' not reported as PASSED: starts=%v finishes=%v",
			rp.starts,
			rp.finishes,
		)
	}
	if seen, status := rp.launch(); !seen || status != "PASSED" {
		t.Errorf("launch seen=%v finished=%q, want PASSED", seen, status)
	}
}

func TestWrapperReportsFailureAndKeepsExitCode(t *testing.T) {
	rp := newFakeRP(t)
	code := runWrapper(t, rp.srv.URL, map[string]string{"FIXTURE_FAIL": "1"})
	if code != 1 {
		t.Fatalf("exit code = %d, want ginkgo's 1", code)
	}
	if rp.finishStatus("fails") != "FAILED" || rp.finishStatus("passes") != "PASSED" {
		t.Errorf(
			"statuses: fails=%q passes=%q",
			rp.finishStatus("fails"),
			rp.finishStatus("passes"),
		)
	}
	if _, status := rp.launch(); status != "FAILED" {
		t.Errorf("launch finished %q, want FAILED", status)
	}
	if rp.logs == 0 {
		t.Error("no logs were sent, expected at least the failure message")
	}
}

func TestWrapperRunsTestsWhenReportPortalIsDown(t *testing.T) {
	rp := newFakeRP(t)
	url := rp.srv.URL
	rp.srv.Close() // nothing listens any more

	if code := runWrapper(t, url, nil); code != 0 {
		t.Errorf("exit code = %d, want 0: ginkgo passed, reporting must not change that", code)
	}
	if code := runWrapper(t, url, map[string]string{"FIXTURE_FAIL": "1"}); code != 1 {
		t.Errorf("exit code = %d, want ginkgo's 1 with RP down", code)
	}
}

func TestWrapperStrictModeFailsWhenReportPortalIsDown(t *testing.T) {
	rp := newFakeRP(t)
	url := rp.srv.URL
	rp.srv.Close()
	if code := runWrapper(t, url, map[string]string{"RP_STRICT": "true"}); code != 1 {
		t.Errorf("exit code = %d, want 1 in strict mode", code)
	}
}

func TestWrapperFinalizeFailureKeepsGinkgoExitCode(t *testing.T) {
	rp := newFakeRP(t)
	rp.failFinish = true
	if code := runWrapper(t, rp.srv.URL, nil); code != 0 {
		t.Errorf("exit code = %d, want ginkgo's 0 when only finalizing failed", code)
	}

	rp2 := newFakeRP(t)
	rp2.failFinish = true
	if code := runWrapper(t, rp2.srv.URL, map[string]string{"RP_STRICT": "true"}); code != 2 {
		t.Errorf("exit code = %d, want 2 in strict mode", code)
	}
}

func TestWrapperAbortFinishesSuitesThenLaunchAsStopped(t *testing.T) {
	rp := newFakeRP(t)
	_ = ginkgoBin(t)
	empty := t.TempDir()

	props := filepath.Join(t.TempDir(), "reportportal.properties")
	content := fmt.Sprintf(
		"rp.endpoint = %s\nrp.api.key = k\nrp.project = proj\nrp.launch = e2e\n",
		rp.srv.URL,
	)
	if err := os.WriteFile(props, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", empty)
	t.Setenv("RP_CONFIG_FILE", props)
	t.Setenv("RP_SESSION_ID", "")
	oldArgs := os.Args
	os.Args = []string{"ginkgo-rp", "run", "./testdata/fixture"}
	t.Cleanup(func() { os.Args = oldArgs })

	if code := run(); code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	seen, status := rp.launch()
	if !seen || status != "STOPPED" {
		t.Fatalf("launch seen=%v status=%q, want STOPPED", seen, status)
	}
	// RP turns a STOPPED launch into FAILED while a suite is still open, so every suite
	// must be finished before the launch is.
	last := rp.order[len(rp.order)-1]
	if last != "launch-finish" || len(rp.order) < 2 ||
		!strings.HasPrefix(rp.order[0], "item-finish:") {
		t.Errorf("request order = %v, want suites finished before the launch", rp.order)
	}
}

// sendSignalWhenSpecRuns waits until the sleeping spec has started in the fake RP, then
// signals this process n times. The wrapper catches the signal; the test binary does not die.
func sendSignalWhenSpecRuns(t *testing.T, rp *fakeRP, sig syscall.Signal, n int) {
	t.Helper()
	// A signal that lands after the wrapper stopped listening would kill the test binary,
	// so keep a catch-all handler until the sender is done.
	catch := make(chan os.Signal, 8)
	signal.Notify(catch, sig)
	done := make(chan struct{})
	t.Cleanup(func() {
		<-done
		signal.Stop(catch)
	})
	go func() {
		defer close(done)
		deadline := time.Now().Add(2 * time.Minute)
		for time.Now().Before(deadline) {
			if rp.started("sleeps") {
				time.Sleep(500 * time.Millisecond) // let the spec body begin sleeping
				for i := 0; i < n; i++ {
					_ = syscall.Kill(os.Getpid(), sig)
					time.Sleep(300 * time.Millisecond)
				}
				return
			}
			time.Sleep(100 * time.Millisecond)
		}
	}()
}

func TestWrapperSIGTERMFinalizesLaunchAndExits143(t *testing.T) {
	rp := newFakeRP(t)
	sendSignalWhenSpecRuns(t, rp, syscall.SIGTERM, 1)

	start := time.Now()
	code := runWrapper(t, rp.srv.URL, map[string]string{"FIXTURE_SLEEP": "1"})
	if code != 128+int(syscall.SIGTERM) {
		t.Fatalf("exit code = %d, want 143", code)
	}
	if time.Since(start) > 50*time.Second {
		t.Error("the sleeping spec was not interrupted")
	}
	if _, status := rp.launch(); status == "" {
		t.Error("launch was not finished after the interrupt")
	}
	if got := rp.finishStatus("sleeps"); got != "INTERRUPTED" {
		t.Errorf("interrupted spec finished as %q, want INTERRUPTED", got)
	}
}

func TestWrapperSecondSignalKillsImmediately(t *testing.T) {
	rp := newFakeRP(t)
	sendSignalWhenSpecRuns(t, rp, syscall.SIGINT, 2)

	start := time.Now()
	code := runWrapper(t, rp.srv.URL, map[string]string{"FIXTURE_SLEEP": "1"})
	if code != 128+int(syscall.SIGINT) {
		t.Fatalf("exit code = %d, want 130", code)
	}
	if time.Since(start) > 50*time.Second {
		t.Error("the second signal did not stop the run")
	}
}

func TestWrapperDeliversLogsAndAttachments(t *testing.T) {
	rp := newFakeRP(t)
	if code := runWrapper(t, rp.srv.URL, map[string]string{"FIXTURE_ATTACH": "1"}); code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	rp.mu.Lock()
	defer rp.mu.Unlock()
	if len(rp.uploads) != 1 || rp.uploads[0] != "evidence.txt" {
		t.Errorf("uploaded files = %v, want [evidence.txt]", rp.uploads)
	}
	joined := strings.Join(rp.logTexts, "\n")
	for _, want := range []string{"custom log line", "writer line"} {
		if !strings.Contains(joined, want) {
			t.Errorf("log %q did not reach ReportPortal; got %q", want, joined)
		}
	}
	if !contains(rp.starts, "writing a file") {
		t.Errorf("By() step missing from starts %v", rp.starts)
	}
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// A closing terminal sends SIGHUP; the launch must still be closed, not left In Progress.
func TestWrapperSIGHUPFinalizesLaunchAndExits129(t *testing.T) {
	rp := newFakeRP(t)
	sendSignalWhenSpecRuns(t, rp, syscall.SIGHUP, 1)

	code := runWrapper(t, rp.srv.URL, map[string]string{"FIXTURE_SLEEP": "1"})
	if code != 128+int(syscall.SIGHUP) {
		t.Fatalf("exit code = %d, want 129", code)
	}
	if _, status := rp.launch(); status == "" {
		t.Error("launch was not finished after SIGHUP")
	}
	if got := rp.finishStatus("sleeps"); got != "INTERRUPTED" {
		t.Errorf("interrupted spec finished as %q, want INTERRUPTED", got)
	}
}
