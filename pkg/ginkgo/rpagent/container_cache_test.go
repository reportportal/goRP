package rpagent

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// resetContainerCache forgets the per-process L1 cache, so a test can act as a fresh
// worker process that only sees the shared files.
func resetContainerCache() {
	inMemoryContainerCache.Range(func(k, _ any) bool {
		inMemoryContainerCache.Delete(k)
		return true
	})
}

func isolatedTemp(t *testing.T) {
	t.Helper()
	t.Setenv("TMPDIR", t.TempDir())
	resetContainerCache()
	t.Cleanup(resetContainerCache)
}

func TestContainerCacheCreatesOncePerKey(t *testing.T) {
	isolatedTemp(t)
	ccm := &ContainerCacheManager{sessionID: "sess"}

	var calls atomic.Int32
	var wg sync.WaitGroup
	ids := make([]string, 16)
	for i := range ids {
		wg.Add(1)
		go func() {
			defer wg.Done()
			id, err := ccm.GetOrCreate("parent:Describe", func() string {
				calls.Add(1)
				time.Sleep(50 * time.Millisecond)
				return "container-1"
			})
			if err != nil {
				t.Errorf("GetOrCreate: %v", err)
			}
			ids[i] = id
		}()
	}
	wg.Wait()

	if calls.Load() != 1 {
		t.Fatalf("create called %d times, want exactly 1", calls.Load())
	}
	for i, id := range ids {
		if id != "container-1" {
			t.Fatalf("caller %d got %q, want container-1", i, id)
		}
	}
}

func TestContainerCacheDifferentKeysAreIndependent(t *testing.T) {
	isolatedTemp(t)
	ccm := &ContainerCacheManager{sessionID: "sess"}
	a, _ := ccm.GetOrCreate("p:A", func() string { return "id-a" })
	b, _ := ccm.GetOrCreate("p:B", func() string { return "id-b" })
	if a != "id-a" || b != "id-b" {
		t.Fatalf("got %q, %q", a, b)
	}
}

func TestContainerCacheIsSharedThroughTheFile(t *testing.T) {
	isolatedTemp(t)
	ccm := &ContainerCacheManager{sessionID: "sess"}
	if _, err := ccm.GetOrCreate("p:A", func() string { return "id-a" }); err != nil {
		t.Fatal(err)
	}

	// A different worker process has an empty L1 cache and must reuse the stored ID.
	resetContainerCache()
	id, err := ccm.GetOrCreate("p:A", func() string {
		t.Error("create must not run when another process already stored the ID")
		return "other"
	})
	if err != nil || id != "id-a" {
		t.Fatalf("got %q, %v; want id-a", id, err)
	}

	// The session ID is in the file name, so the wrapper can clean up exactly this run.
	files, _ := filepath.Glob(filepath.Join(os.TempDir(), "rp_cache_sess_*"))
	if len(files) != 1 {
		t.Fatalf("want 1 cache file named for the session, got %v", files)
	}
}

func TestContainerCacheFailedCreateReleasesLock(t *testing.T) {
	isolatedTemp(t)
	ccm := &ContainerCacheManager{sessionID: "sess"}
	if _, err := ccm.GetOrCreate("p:A", func() string { return "" }); err == nil {
		t.Fatal("expected an error when create returns no ID")
	}
	// The lock must be gone, or every later worker would wait for an ID that never comes.
	id, err := ccm.GetOrCreate("p:A", func() string { return "id-a" })
	if err != nil || id != "id-a" {
		t.Fatalf("retry after failure: got %q, %v", id, err)
	}
}

// TestContainerCacheAcrossProcesses starts real OS processes that race for the same key:
// exactly one may create the item, and all must end up with its ID.
func TestContainerCacheAcrossProcesses(t *testing.T) {
	if testing.Short() {
		t.Skip("spawns processes")
	}
	shared := t.TempDir()
	createLog := filepath.Join(shared, "creates.log")

	const workers = 6
	var wg sync.WaitGroup
	out := make([]string, workers)
	for i := range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cmd := exec.Command(os.Args[0], "-test.run=^TestContainerCacheHelperProcess$")
			cmd.Env = append(os.Environ(),
				"RP_CACHE_HELPER=1",
				"RP_SESSION_ID=xproc",
				"TMPDIR="+shared,
				"RP_CACHE_CREATE_LOG="+createLog,
				"RP_LOCK_RETRY_COUNT=200",
				"RP_LOCK_RETRY_SLEEP_MS=25",
			)
			b, err := cmd.CombinedOutput()
			if err != nil {
				t.Errorf("worker %d: %v\n%s", i, err, b)
			}
			for _, line := range strings.Split(string(b), "\n") {
				if id, ok := strings.CutPrefix(line, "CACHEID="); ok {
					out[i] = id
				}
			}
		}()
	}
	wg.Wait()

	data, _ := os.ReadFile(createLog)
	if n := len(strings.Fields(string(data))); n != 1 {
		t.Fatalf("container was created %d times across processes, want 1", n)
	}
	for i, id := range out {
		if id == "" || id != out[0] {
			t.Fatalf("worker %d got %q, worker 0 got %q; all must agree", i, id, out[0])
		}
	}
}

// TestContainerCacheHelperProcess is the body of one worker in the test above.
// It does nothing in a normal test run.
func TestContainerCacheHelperProcess(t *testing.T) {
	if os.Getenv("RP_CACHE_HELPER") != "1" {
		t.Skip("helper process")
	}
	ccm := &ContainerCacheManager{sessionID: os.Getenv("RP_SESSION_ID")}
	id, err := ccm.GetOrCreate("parent:Shared Describe", func() string {
		f, ferr := os.OpenFile(
			os.Getenv("RP_CACHE_CREATE_LOG"),
			os.O_APPEND|os.O_CREATE|os.O_WRONLY,
			0o600,
		)
		if ferr == nil {
			_, _ = fmt.Fprintf(f, "%d\n", os.Getpid())
			_ = f.Close()
		}
		time.Sleep(300 * time.Millisecond) // widen the race window
		return fmt.Sprintf("container-by-%d", os.Getpid())
	})
	if err != nil {
		t.Fatal(err)
	}
	fmt.Printf("CACHEID=%s\n", id)
}

// ── hierarchy building ──────────────────────────────────────────────────────────────

type startedItem struct{ name, parent string }

// recordingRP is a minimal ReportPortal that records item starts in order.
func recordingRP(t *testing.T) (*TestListener, func() []startedItem) {
	t.Helper()
	var mu sync.Mutex
	var started []startedItem
	var n atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/item") {
			var rq struct{ Name string }
			_ = json.NewDecoder(r.Body).Decode(&rq)
			parent := ""
			if rest := strings.TrimPrefix(
				r.URL.Path,
				"/api/v2/proj/item",
			); strings.Trim(
				rest,
				"/",
			) != "" {
				parent = strings.Trim(rest, "/")
			}
			mu.Lock()
			started = append(started, startedItem{rq.Name, parent})
			mu.Unlock()
			writeJSON(w, fmt.Sprintf(`{"id":"id-%d"}`, n.Add(1)))
			return
		}
		writeJSON(w, `{}`)
	}))
	t.Cleanup(srv.Close)
	tl := NewTestListener(NewReportingClient(srv.URL, "proj", "k", false), "launch-1", "L")
	return tl, func() []startedItem {
		mu.Lock()
		defer mu.Unlock()
		return append([]startedItem(nil), started...)
	}
}

func specIn(containers []string, leaf string) GinkgoSpecReport {
	locs := make([]CodeLoc, len(containers))
	for i := range locs {
		locs[i] = CodeLoc{FileName: "/p/a_test.go", LineNumber: 10 + i}
	}
	return GinkgoSpecReport{
		ContainerHierarchyTexts: containers,
		ContainerHierarchyLocs:  locs,
		LeafNodeText:            leaf,
		LeafNodeLoc:             CodeLoc{FileName: "/p/a_test.go", LineNumber: 99},
	}
}

func TestHierarchyNestsContainersThenSpec(t *testing.T) {
	isolatedTemp(t)
	tl, started := recordingRP(t)
	ccm := &ContainerCacheManager{sessionID: "h1"}

	ids := ProcessSpecWithHierarchyAndReturnIDs(
		nil,
		tl,
		specIn([]string{"Calc", "Add"}, "adds"),
		"suite-1",
		ccm,
		nil,
	)

	got := started()
	want := []startedItem{{"Calc", "suite-1"}, {"Add", "id-1"}, {"adds", "id-2"}}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("started %v, want %v", got, want)
	}
	if ids.StepID != "id-3" || len(ids.ContainerIDs) != 2 ||
		ids.ContainerIDs[0] != "id-1" || ids.ContainerIDs[1] != "id-2" {
		t.Fatalf("unexpected IDs: %+v", ids)
	}
	if ids.Identity.TestCaseID == "" || ids.StartTime.IsZero() {
		t.Fatalf("spec identity and start time must be set: %+v", ids)
	}
}

func TestHierarchySharesContainersBetweenSpecs(t *testing.T) {
	isolatedTemp(t)
	tl, started := recordingRP(t)
	ccm := &ContainerCacheManager{sessionID: "h2"}

	a := ProcessSpecWithHierarchyAndReturnIDs(
		nil,
		tl,
		specIn([]string{"Calc"}, "adds"),
		"suite-1",
		ccm,
		nil,
	)
	b := ProcessSpecWithHierarchyAndReturnIDs(
		nil,
		tl,
		specIn([]string{"Calc"}, "subtracts"),
		"suite-1",
		ccm,
		nil,
	)
	c := ProcessSpecWithHierarchyAndReturnIDs(
		nil,
		tl,
		specIn([]string{"Other"}, "adds"),
		"suite-1",
		ccm,
		nil,
	)

	if a.ContainerIDs[0] != b.ContainerIDs[0] {
		t.Fatalf(
			"specs under one Describe must share its item: %v vs %v",
			a.ContainerIDs,
			b.ContainerIDs,
		)
	}
	if a.ContainerIDs[0] == c.ContainerIDs[0] {
		t.Fatal("a different Describe must get its own item")
	}
	containers := 0
	for _, it := range started() {
		if it.name == "Calc" || it.name == "Other" {
			containers++
		}
	}
	if containers != 2 {
		t.Fatalf("created %d container items, want 2 (Calc once, Other once)", containers)
	}
}

func TestHierarchyWithoutContainersAttachesToSuite(t *testing.T) {
	isolatedTemp(t)
	tl, started := recordingRP(t)
	ccm := &ContainerCacheManager{sessionID: "h3"}

	ProcessSpecWithHierarchyAndReturnIDs(
		nil,
		tl,
		specIn(nil, "top-level spec"),
		"suite-1",
		ccm,
		nil,
	)

	if got := started(); len(got) != 1 || got[0].parent != "suite-1" {
		t.Fatalf("got %v, want the spec started directly under suite-1", got)
	}
}

// Container start failures must degrade to the parent, never panic or kill the run.
func TestHierarchySurvivesContainerStartFailure(t *testing.T) {
	isolatedTemp(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest) // not retried
	}))
	t.Cleanup(srv.Close)
	tl := NewTestListener(NewReportingClient(srv.URL, "proj", "k", false), "launch-1", "L")
	ccm := &ContainerCacheManager{sessionID: "h4"}

	ids := ProcessSpecWithHierarchyAndReturnIDs(
		nil,
		tl,
		specIn([]string{"Calc"}, "adds"),
		"suite-1",
		ccm,
		nil,
	)

	if ids.StepID != "" {
		t.Fatalf("StepID = %q, want empty when the server rejects every start", ids.StepID)
	}
}
