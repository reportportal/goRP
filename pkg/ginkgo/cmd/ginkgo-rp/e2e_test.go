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
	startRQs    []startRec          // every item start, in order
	itemLogs    map[string][]string // item id -> messages logged on it
	order       []string            // "item-finish:<id>" / "launch-finish" in request order
	failFinish  bool                // answer launch finish with 500
	omitLink    bool                // answer launch finish without a link (an old server)
	hangFinish  bool                // never answer a finish request (item or launch)
	hung        chan struct{}       // closed at the first hung finish request
	hungOnce    sync.Once
	hangStart   time.Time    // when the first finish request arrived
	stopTries   int          // launch finish requests carrying status STOPPED
	launchAttrs []launchAttr // attributes of the launch start request
	seq         int
}

type finish struct{ ID, Name, Status, Issue string }

type launchAttr struct {
	Key, Value string
	System     bool
}

type startRec struct {
	ID, Name, RetryOf string
	Retry             bool
	Attributes        map[string]string // key -> value of the item's attributes
}

func newFakeRP(t *testing.T) *fakeRP {
	t.Helper()
	f := &fakeRP{
		items: map[string]string{}, itemLogs: map[string][]string{}, hung: make(chan struct{}),
	}
	f.srv = httptest.NewServer(http.HandlerFunc(f.handle))
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fakeRP) handle(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	body, _ := io.ReadAll(r.Body)
	path := r.URL.Path
	if f.hangFinish && f.isFinalizationRequest(r.Method, path) {
		var rq struct{ Status string }
		_ = json.Unmarshal(body, &rq)
		if strings.HasSuffix(path, "/finish") && rq.Status == "STOPPED" {
			f.stopTries++
		}
		f.hungOnce.Do(func() {
			f.hangStart = time.Now()
			close(f.hung)
		})
		f.mu.Unlock() // do not block the other requests while this one hangs
		<-r.Context().Done()
		f.mu.Lock()
		return
	}
	w.Header().Set("Content-Type", "application/json")
	reply := func(s string) { _, _ = w.Write([]byte(s)) }
	next := func() string { f.seq++; return fmt.Sprintf("id-%d", f.seq) }

	switch {
	case r.Method == http.MethodPost && strings.HasSuffix(path, "/launch"):
		var rq struct{ Attributes []launchAttr }
		_ = json.Unmarshal(body, &rq)
		f.launchAttrs = rq.Attributes
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
		if f.omitLink {
			reply(`{"id":"launch-1","number":1}`)
		} else {
			reply(
				fmt.Sprintf(
					`{"id":"launch-1","number":1,"link":"http://%s/ui/#proj/launches/all/launch-1"}`,
					r.Host,
				),
			)
		}
	case r.Method == http.MethodPost && strings.HasSuffix(path, "/log"):
		f.logs++
		f.recordLog(r, body)
		reply(`{}`)
	case r.Method == http.MethodPost && strings.Contains(path, "/item"):
		var rq struct {
			Name       string
			Retry      bool
			RetryOf    string
			Attributes []struct{ Key, Value string }
		}
		_ = json.Unmarshal(body, &rq)
		id := next()
		f.items[id] = rq.Name
		f.starts = append(f.starts, rq.Name)
		attrs := map[string]string{}
		for _, a := range rq.Attributes {
			attrs[a.Key] = a.Value
		}
		f.startRQs = append(
			f.startRQs,
			startRec{
				ID: id, Name: rq.Name, Retry: rq.Retry, RetryOf: rq.RetryOf, Attributes: attrs,
			},
		)
		reply(fmt.Sprintf(`{"id":%q}`, id))
	case r.Method == http.MethodPut && strings.Contains(path, "/item/"):
		var rq struct {
			Status string
			Issue  struct{ IssueType string }
		}
		_ = json.Unmarshal(body, &rq)
		id := path[strings.LastIndex(path, "/")+1:]
		f.finishes = append(f.finishes, finish{id, f.items[id], rq.Status, rq.Issue.IssueType})
		f.order = append(f.order, "item-finish:"+f.items[id])
		reply(fmt.Sprintf(`{"message":"ok %s"}`, id))
	default:
		reply(`{}`)
	}
}

// isFinalizationRequest reports whether a request is one the wrapper sends while finalizing:
// finishing the launch, a container or a suite. The workers' spec finishes are not.
// The caller holds f.mu.
func (f *fakeRP) isFinalizationRequest(method, path string) bool {
	if method != http.MethodPut {
		return false
	}
	if strings.HasSuffix(path, "/finish") {
		return true
	}
	switch f.items[path[strings.LastIndex(path, "/")+1:]] {
	case "fixture", "Fixture Suite":
		return true
	}
	return false
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
		var logs []struct{ Message, ItemUuid string }
		if json.Unmarshal(data, &logs) == nil {
			for _, l := range logs {
				f.logTexts = append(f.logTexts, l.Message)
				f.itemLogs[l.ItemUuid] = append(f.itemLogs[l.ItemUuid], l.Message)
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
	t.Setenv("FIXTURE_ATTACH_ERRORS", "")
	t.Setenv("FIXTURE_PRIORITY", "")
	t.Setenv("FIXTURE_SKIPS", "")
	t.Setenv("FIXTURE_FLAKY_STDOUT", "")
	t.Setenv("RP_SKIPPED_ISSUE", "")
	t.Setenv("FIXTURE_FLAKY_AFTEREACH", "")
	t.Setenv("FIXTURE_CLEANUP_FAIL", "")
	t.Setenv("FIXTURE_REPEAT", "")
	t.Setenv("FIXTURE_REPEAT_FAIL", "")
	for k, v := range env {
		t.Setenv(k, v)
	}

	oldArgs := os.Args
	os.Args = append([]string{"ginkgo-rp", "run"}, extraWrapperArgs...)
	os.Args = append(os.Args, wrapperTarget)
	t.Cleanup(func() { os.Args = oldArgs })
	return run()
}

// wrapperTarget is the package runWrapper runs.
var wrapperTarget = "./testdata/fixture"

// extraWrapperArgs are put between "run" and the fixture path by runWrapper.
var extraWrapperArgs []string

// captureConsole redirects stdout and stderr (the wrapper's and its ginkgo child's) to a
// file until the test ends. The returned function reads what was printed so far.
func captureConsole(t *testing.T) func() string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "console")
	if err != nil {
		t.Fatal(err)
	}
	oldOut, oldErr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = f, f
	t.Cleanup(func() {
		os.Stdout, os.Stderr = oldOut, oldErr
		_ = f.Close()
	})
	return func() string {
		b, _ := os.ReadFile(f.Name())
		return string(b)
	}
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

// A failure in an AfterEach of attempt 1 is not in the It, so only Ginkgo's
// AdditionalFailures holds it. The retry chain must still carry that message.
func TestWrapperReportsFlakeAttemptsAsRetriesWithCleanupFailure(t *testing.T) {
	rp := newFakeRP(t)
	if code := runWrapper(
		t,
		rp.srv.URL,
		map[string]string{"FIXTURE_FLAKY_AFTEREACH": "1"},
	); code != 0 {
		t.Fatalf("exit code = %d, want 0: the spec passed on attempt 2", code)
	}
	rp.mu.Lock()
	defer rp.mu.Unlock()

	var attempts []startRec
	for _, s := range rp.startRQs {
		if s.Name == "flaky after each" {
			attempts = append(attempts, s)
		}
	}
	if len(attempts) != 2 {
		t.Fatalf(
			"started %d items for the flaky spec, want 2 attempts (%v)",
			len(attempts),
			rp.starts,
		)
	}
	if attempts[0].Retry || attempts[0].RetryOf != "" {
		t.Errorf("attempt 1 must be a plain item, got %+v", attempts[0])
	}
	if !attempts[1].Retry || attempts[1].RetryOf != attempts[0].ID {
		t.Errorf("attempt 2 = %+v, want retry=true retryOf=%s", attempts[1], attempts[0].ID)
	}

	statuses := map[string]string{}
	for _, fi := range rp.finishes {
		statuses[fi.ID] = fi.Status
	}
	if statuses[attempts[0].ID] != "FAILED" || statuses[attempts[1].ID] != "PASSED" {
		t.Errorf("statuses = %v, want attempt 1 FAILED and attempt 2 PASSED", statuses)
	}

	first := strings.Join(rp.itemLogs[attempts[0].ID], "\n")
	if !strings.Contains(first, "cleanup failed on attempt 1") {
		t.Errorf("attempt 1 lost its AfterEach failure; logs: %q", first)
	}
	if strings.Contains(first, "were not captured") || strings.Contains(first, "no details") {
		t.Errorf("attempt 1 shows the no-details fallback: %q", first)
	}
	if strings.Contains(strings.Join(rp.itemLogs[attempts[1].ID], "\n"), "cleanup failed") {
		t.Error("attempt 2 must not carry attempt 1's failure")
	}
}

// Ginkgo also keeps a second failure (a cleanup that fails after the It failed) only in
// AdditionalFailures. It must reach ReportPortal, next to the primary failure.
func TestWrapperLogsCleanupFailureAfterSpecFailure(t *testing.T) {
	rp := newFakeRP(t)
	if code := runWrapper(
		t,
		rp.srv.URL,
		map[string]string{"FIXTURE_CLEANUP_FAIL": "1"},
	); code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	rp.mu.Lock()
	defer rp.mu.Unlock()
	var item string
	for _, s := range rp.startRQs {
		if s.Name == "fails then cleanup fails" {
			item = s.ID
		}
	}
	if item == "" {
		t.Fatalf("spec not started: %v", rp.starts)
	}
	logs := strings.Join(rp.itemLogs[item], "\n")
	for _, want := range []string{"primary failure", "cleanup failure after the spec failed"} {
		if !strings.Contains(logs, want) {
			t.Errorf("log %q missing from the spec item; got %q", want, logs)
		}
	}
}

// MustPassRepeatedly is not a retry: nothing failed, so ReportPortal must see one plain
// item (a retry would mark the spec as flaky), with a log marking each repetition.
func TestWrapperKeepsMustPassRepeatedlyAsOneItem(t *testing.T) {
	rp := newFakeRP(t)
	if code := runWrapper(t, rp.srv.URL, map[string]string{"FIXTURE_REPEAT": "1"}); code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	rp.mu.Lock()
	defer rp.mu.Unlock()
	var items []startRec
	steps := 0
	for _, s := range rp.startRQs {
		switch s.Name {
		case "repeats":
			items = append(items, s)
		case "repetition body":
			steps++
		}
		if s.Retry || s.RetryOf != "" {
			t.Errorf("item %q was started as a retry: %+v", s.Name, s)
		}
	}
	if len(items) != 1 {
		t.Fatalf(
			"started %d items for the repeated spec, want exactly 1 (%v)",
			len(items),
			rp.starts,
		)
	}
	if steps != 3 {
		t.Errorf("By() steps = %d, want one per repetition (3)", steps)
	}
	logs := strings.Join(rp.itemLogs[items[0].ID], "\n")
	for _, want := range []string{"Repetition 1 of 3", "Repetition 2 of 3", "Repetition 3 of 3"} {
		if !strings.Contains(logs, want) {
			t.Errorf("log %q missing: %q", want, logs)
		}
	}
	if got := rp.finishStatusByID(items[0].ID); got != "PASSED" {
		t.Errorf("repeated spec finished %q, want PASSED", got)
	}
}

func TestWrapperFailedRepetitionFailsTheItem(t *testing.T) {
	rp := newFakeRP(t)
	if code := runWrapper(t, rp.srv.URL, map[string]string{"FIXTURE_REPEAT_FAIL": "1"}); code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	rp.mu.Lock()
	defer rp.mu.Unlock()
	var items []startRec
	for _, s := range rp.startRQs {
		if s.Name == "repeat fails" {
			items = append(items, s)
		}
	}
	if len(items) != 1 || items[0].Retry {
		t.Fatalf("want one plain item, got %+v", items)
	}
	logs := strings.Join(rp.itemLogs[items[0].ID], "\n")
	if !strings.Contains(logs, "failed on repetition 2") {
		t.Errorf("the failing repetition's message is missing: %q", logs)
	}
	if !strings.Contains(logs, "Repetition 2 of 3") || strings.Contains(logs, "Repetition 3 of 3") {
		t.Errorf("Ginkgo stops at the failing repetition, so only 1 and 2 are logged: %q", logs)
	}
	if got := rp.finishStatusByID(items[0].ID); got != "FAILED" {
		t.Errorf("item finished %q, want FAILED", got)
	}
}

// finishStatusByID needs rp.mu held by the caller.
func (f *fakeRP) finishStatusByID(id string) string {
	for _, fi := range f.finishes {
		if fi.ID == id {
			return fi.Status
		}
	}
	return ""
}

// The agent passes attachments and logs to itself through report entries. They are
// plumbing: Ginkgo must not print them (here with -v, which prints every entry).
func TestWrapperKeepsAgentReportEntriesOutOfTheConsole(t *testing.T) {
	rp := newFakeRP(t)
	extraWrapperArgs = []string{"-v"}
	t.Cleanup(func() { extraWrapperArgs = nil })
	console := captureConsole(t)

	code := runWrapper(t, rp.srv.URL, map[string]string{
		"FIXTURE_ATTACH":        "1",
		"FIXTURE_ATTACH_ERRORS": "1",
	})
	out := console()
	if code != 0 {
		t.Fatalf("exit code = %d, want 0\n%s", code, out)
	}
	if !strings.Contains(out, "writer line") {
		t.Fatalf("the fixture's own output is missing, so -v did not take effect:\n%s", out)
	}
	for _, plumbing := range []string{"RP_ATTACHMENT", "RP_LOG#"} {
		if strings.Contains(out, plumbing) {
			t.Errorf("console shows agent plumbing %q", plumbing)
		}
	}

	// Hiding the entries must not hide their effect on ReportPortal.
	rp.mu.Lock()
	defer rp.mu.Unlock()
	if len(rp.uploads) != 1 || rp.uploads[0] != "evidence.txt" {
		t.Errorf("uploaded files = %v, want [evidence.txt]", rp.uploads)
	}
	logs := strings.Join(rp.logTexts, "\n")
	for _, want := range []string{
		`Attachment not uploaded: missing.txt ("no such file"): cannot access the file: no such file or directory`,
		`Attachment not uploaded: big.txt ("too big"): the file is 10 bytes, the limit is 3 bytes`,
		`Attachment not uploaded: small.txt ("cannot copy"): cannot copy the file: `,
	} {
		if !strings.Contains(logs, want) {
			t.Errorf("attachment error log missing: %q\n%s", want, logs)
		}
	}
	if strings.Contains(logs, `{"`) || strings.Contains(logs, "Attachment error") {
		t.Errorf("an attachment error is logged as a raw payload:\n%s", logs)
	}
	if !strings.Contains(logs, "custom log line") {
		t.Errorf("rpagent.Log no longer reaches ReportPortal: %q", logs)
	}
}

// Every item used to get "priority: P1" even when nobody set one, which made the attribute
// useless for filtering. Only specs with a rp.priority label may carry it.
func TestWrapperSendsPriorityOnlyForLabeledSpecs(t *testing.T) {
	rp := newFakeRP(t)
	if code := runWrapper(t, rp.srv.URL, map[string]string{"FIXTURE_PRIORITY": "1"}); code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	rp.mu.Lock()
	defer rp.mu.Unlock()

	want := map[string]string{ // spec -> priority attribute ("" = must have none)
		"passes":                  "", // no label anywhere
		"explicit default":        "P1",
		"explicit critical":       "P0",
		"inherits the container":  "P3",
		"overrides the container": "P0",
	}
	seen := map[string]bool{}
	for _, s := range rp.startRQs {
		w, ok := want[s.Name]
		if !ok {
			continue
		}
		seen[s.Name] = true
		got, has := s.Attributes["priority"]
		if w == "" && has {
			t.Errorf("%q has no priority label but was sent priority=%s", s.Name, got)
		}
		if w != "" && got != w {
			t.Errorf("%q priority = %q, want %q", s.Name, got, w)
		}
	}
	for name := range want {
		if !seen[name] {
			t.Errorf("spec %q was never started: %v", name, rp.starts)
		}
	}
	// Containers and suites never carry a priority.
	for _, s := range rp.startRQs {
		if s.Name == "fixture" || s.Name == "priority container" || s.Name == "Fixture Suite" {
			if _, has := s.Attributes["priority"]; has {
				t.Errorf("container %q was sent a priority", s.Name)
			}
		}
	}
}

// shortenStopTimeout makes the last-resort stopLaunch give up quickly, so a hanging fake
// server does not make the test wait for the production 30 seconds.
func shortenStopTimeout(t *testing.T, d time.Duration) {
	t.Helper()
	old := stopLaunchTimeout
	stopLaunchTimeout = d
	t.Cleanup(func() { stopLaunchTimeout = old })
}

// signalWhenFinalizing sends sig to this process n times once the fake RP has received (and
// is hanging on) its first finish request, so the wrapper is inside finalization.
func signalWhenFinalizing(t *testing.T, rp *fakeRP, sig syscall.Signal, n int) {
	t.Helper()
	catch := make(chan os.Signal, 8) // a late signal must not kill the test binary
	signal.Notify(catch, sig)
	done := make(chan struct{})
	t.Cleanup(func() {
		<-done
		signal.Stop(catch)
	})
	go func() {
		defer close(done)
		select {
		case <-rp.hung:
		case <-time.After(2 * time.Minute):
			return
		}
		time.Sleep(500 * time.Millisecond)
		for i := 0; i < n; i++ {
			_ = syscall.Kill(os.Getpid(), sig)
			time.Sleep(300 * time.Millisecond)
		}
	}()
}

// sinceFinalizationStarted is how long the wrapper kept going after its first finish request.
func (f *fakeRP) sinceFinalizationStarted() time.Duration {
	f.mu.Lock()
	defer f.mu.Unlock()
	return time.Since(f.hangStart)
}

func (f *fakeRP) stopAttempts() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.stopTries
}

// A ReportPortal that hangs while the launch is finished used to keep the wrapper waiting
// one client timeout (30 s) per request. Finalization now has an overall deadline.
func TestWrapperGivesUpFinalizingAfterTheDeadline(t *testing.T) {
	rp := newFakeRP(t)
	rp.hangFinish = true
	shortenStopTimeout(t, 3*time.Second)
	console := captureConsole(t)

	code := runWrapper(t, rp.srv.URL, map[string]string{finalizeTimeoutEnv: "2s"})
	out := console()
	if code != 0 {
		t.Fatalf("exit code = %d, want ginkgo's 0\n%s", code, out)
	}
	if took := rp.sinceFinalizationStarted(); took > 15*time.Second {
		t.Errorf("finalization went on for %v, want about 2s + the 3s fallback", took)
	}
	if !strings.Contains(out, "finalizing took longer than 2s") {
		t.Errorf("no deadline warning:\n%s", out)
	}
	if rp.stopAttempts() == 0 {
		t.Error("the launch was never asked to stop after the deadline")
	}
}

func TestWrapperSignalDuringFinalizationStopsTheLaunch(t *testing.T) {
	rp := newFakeRP(t)
	rp.hangFinish = true
	shortenStopTimeout(t, 3*time.Second)
	signalWhenFinalizing(t, rp, syscall.SIGINT, 1)

	code := runWrapper(t, rp.srv.URL, map[string]string{finalizeTimeoutEnv: "5m"})
	if code != 128+int(syscall.SIGINT) {
		t.Fatalf("exit code = %d, want 130", code)
	}
	if took := rp.sinceFinalizationStarted(); took > 15*time.Second {
		t.Errorf("Ctrl-C during finalization was swallowed: it took %v with a 5m deadline", took)
	}
	if rp.stopAttempts() == 0 {
		t.Error("the launch was not stopped after Ctrl-C")
	}
}

func TestWrapperSecondSignalDuringFinalizationGivesUpAtOnce(t *testing.T) {
	rp := newFakeRP(t)
	rp.hangFinish = true
	shortenStopTimeout(t, 60*time.Second) // only a second signal can end this quickly
	signalWhenFinalizing(t, rp, syscall.SIGINT, 2)

	code := runWrapper(t, rp.srv.URL, map[string]string{finalizeTimeoutEnv: "5m"})
	if code != 128+int(syscall.SIGINT) {
		t.Fatalf("exit code = %d, want 130", code)
	}
	if took := rp.sinceFinalizationStarted(); took > 15*time.Second {
		t.Errorf("the second Ctrl-C did not cut the fallback short: %v", took)
	}
}

// finishOf returns what the fake RP was told when the named item finished.
// The caller holds rp.mu.
func (f *fakeRP) finishOf(name string) (finish, bool) {
	for _, fi := range f.finishes {
		if fi.Name == name {
			return fi, true
		}
	}
	return finish{}, false
}

// ReportPortal files every skipped item without an issue under To Investigate. A skip
// nobody has to look at (pending, filtered out, a consequence of another spec) must be
// NOT_ISSUE; only a spec that called Skip("reason") may stay To Investigate.
func TestWrapperMarksSkipsThatNeedNoInvestigationAsNotIssue(t *testing.T) {
	for _, tc := range []struct {
		name         string
		env          string // RP_SKIPPED_ISSUE ("" = default)
		userSkipWant string // issue type expected for the spec that called Skip(); "" = none sent
		attrWant     string
	}{
		{"default (true)", "", "", "true"},
		{"explicitly true", "true", "", "true"},
		{"false", "false", "NOT_ISSUE", "false"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rp := newFakeRP(t)
			extraWrapperArgs = []string{"--skip=filtered out"}
			t.Cleanup(func() { extraWrapperArgs = nil })

			code := runWrapper(t, rp.srv.URL, map[string]string{
				"FIXTURE_SKIPS":    "1",
				"RP_SKIPPED_ISSUE": tc.env,
			})
			if code != 1 {
				t.Fatalf("exit code = %d, want 1 (the ordered group has a failing spec)", code)
			}
			rp.mu.Lock()
			defer rp.mu.Unlock()

			want := map[string]struct{ status, issue string }{
				"user skip":       {"SKIPPED", tc.userSkipWant},
				"pending spec":    {"SKIPPED", "NOT_ISSUE"},
				"filtered out":    {"SKIPPED", "NOT_ISSUE"},
				"cascade skipped": {"SKIPPED", "NOT_ISSUE"},
				"breaks":          {"FAILED", ""}, // a failure is always To Investigate
			}
			for name, w := range want {
				fi, ok := rp.finishOf(name)
				if !ok {
					t.Errorf("spec %q never finished: %v", name, rp.starts)
					continue
				}
				if fi.Status != w.status || fi.Issue != w.issue {
					t.Errorf("%q finished %s/issue=%q, want %s/issue=%q",
						name, fi.Status, fi.Issue, w.status, w.issue)
				}
			}

			var attr *launchAttr
			for i := range rp.launchAttrs {
				if rp.launchAttrs[i].Key == "skippedIssue" {
					attr = &rp.launchAttrs[i]
				}
			}
			if attr == nil || attr.Value != tc.attrWant || !attr.System {
				t.Errorf(
					"launch attribute skippedIssue = %+v, want a system attribute with value %s",
					attr,
					tc.attrWant,
				)
			}
		})
	}
}

// ReportPortal's own link is printed, always: it used to be printed only with rp.debug,
// and built by hand from a launch number that 6.x no longer returns.
func TestWrapperPrintsTheLaunchLinkReportPortalGave(t *testing.T) {
	rp := newFakeRP(t)
	console := captureConsole(t)
	if code := runWrapper(t, rp.srv.URL, nil); code != 0 { // rp.debug is off
		t.Fatalf("exit code = %d, want 0", code)
	}
	want := "  → " + rp.srv.URL + "/ui/#proj/launches/all/launch-1"
	if out := console(); !strings.Contains(out, want+"\n") {
		t.Errorf("the launch link %q was not printed:\n%s", want, out)
	}
}

// With no link in the answer nothing is printed: a URL built by hand is wrong on some
// ReportPortal versions, so there is no fallback, not even with rp.debug on.
func TestWrapperBuildsNoLaunchLinkOfItsOwn(t *testing.T) {
	rp := newFakeRP(t)
	rp.omitLink = true
	console := captureConsole(t)
	if code := runWrapper(t, rp.srv.URL, map[string]string{"RP_DEBUG": "true"}); code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	out := console()
	if !strings.Contains(out, "finalized") {
		t.Fatalf("the launch was not finalized:\n%s", out)
	}
	if strings.Contains(out, "→") || strings.Contains(out, "/launches/all/") {
		t.Errorf("a launch link was printed although the server gave none:\n%s", out)
	}
}

// Ginkgo captures stdout/stderr per spec, not per attempt, and only when running in
// parallel. The earlier attempts therefore get none, and the last attempt's log holds the
// output of all of them: it must say so. (GinkgoWriter output is split exactly.)
func TestWrapperLabelsStdoutThatCoversAllAttempts(t *testing.T) {
	rp := newFakeRP(t)
	extraWrapperArgs = []string{"-p"}
	t.Cleanup(func() { extraWrapperArgs = nil })

	if code := runWrapper(
		t,
		rp.srv.URL,
		map[string]string{"FIXTURE_FLAKY_STDOUT": "1"},
	); code != 0 {
		t.Fatalf("exit code = %d, want 0 (the spec passes on attempt 3)", code)
	}
	rp.mu.Lock()
	defer rp.mu.Unlock()

	var attempts []startRec
	for _, s := range rp.startRQs {
		if s.Name == "flaky with stdout" {
			attempts = append(attempts, s)
		}
	}
	if len(attempts) != 3 {
		t.Fatalf("started %d attempts, want 3 (%v)", len(attempts), rp.starts)
	}
	// GinkgoWriter output is exact: each attempt has its own line and no other.
	for i, a := range attempts {
		logs := strings.Join(rp.itemLogs[a.ID], "\n")
		for n := 1; n <= 3; n++ {
			has := strings.Contains(logs, fmt.Sprintf("writer says attempt %d", n))
			if has != (n == i+1) {
				t.Errorf(
					"attempt %d: GinkgoWriter line for attempt %d present = %v:\n%s",
					i+1,
					n,
					has,
					logs,
				)
			}
		}
	}
	final := strings.Join(rp.itemLogs[attempts[2].ID], "\n")
	for _, a := range attempts[:2] {
		if got := strings.Join(rp.itemLogs[a.ID], "\n"); strings.Contains(got, "stdout says") {
			t.Errorf("an earlier attempt shows stdout it cannot own: %q", got)
		}
	}
	for n := 1; n <= 3; n++ {
		if want := fmt.Sprintf("stdout says attempt %d", n); !strings.Contains(final, want) {
			t.Errorf("the final item lost %q:\n%s", want, final)
		}
	}
	if !strings.Contains(final, "of all 3 attempts") {
		t.Errorf("the stdout log does not say it covers all 3 attempts:\n%s", final)
	}
}

// The README describes this edge case: a package whose Ginkgo files are all excluded by
// build constraints is still a suite to Ginkgo (which then cannot compile it), but it is not
// registered in ReportPortal, and the wrapper says so instead of creating an empty launch.
func TestWrapperDoesNotRegisterASuiteExcludedByTags(t *testing.T) {
	rp := newFakeRP(t)
	wrapperTarget = "./testdata/tagged"
	t.Cleanup(func() { wrapperTarget = "./testdata/fixture" })
	console := captureConsole(t)

	code := runWrapper(t, rp.srv.URL, nil) // no --tags special
	out := console()
	if code == 0 {
		t.Errorf("exit code = 0, want Ginkgo's failure: it cannot compile the package\n%s", out)
	}
	if !strings.Contains(out, "build constraints exclude all Go files") {
		t.Errorf("Ginkgo did not report the package as uncompilable:\n%s", out)
	}
	if seen, _ := rp.launch(); seen {
		t.Error("a launch was created although no suite could be registered")
	}
	if !strings.Contains(out, "no Ginkgo suite found") ||
		!strings.Contains(out, "ReportPortal reporting is OFF") {
		t.Errorf("the wrapper did not say why nothing is reported:\n%s", out)
	}
}

// With the tag the same package is a normal suite.
func TestWrapperRegistersTheTaggedSuiteWhenTheTagIsGiven(t *testing.T) {
	rp := newFakeRP(t)
	wrapperTarget = "./testdata/tagged"
	extraWrapperArgs = []string{"--tags", "special"}
	t.Cleanup(func() { wrapperTarget = "./testdata/fixture"; extraWrapperArgs = nil })

	if code := runWrapper(t, rp.srv.URL, nil); code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !rp.started("Tagged Suite") || rp.finishStatus("runs only with -tags special") != "PASSED" {
		t.Errorf("the tagged suite was not reported: starts=%v finishes=%v", rp.starts, rp.finishes)
	}
}
