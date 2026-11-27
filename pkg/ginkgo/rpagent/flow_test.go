package rpagent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/onsi/ginkgo/v2/types"

	"github.com/reportportal/goRP/v5/pkg/gorp"
	"github.com/reportportal/goRP/v5/pkg/openapi"
)

// flowServer is a fake ReportPortal that records what the agent sent while it reported specs.
type flowServer struct {
	mu       sync.Mutex
	seq      int
	starts   []flowStart
	finishes map[string]flowFinish
	logs     map[string][]sentLog // item id -> logs
	uploads  int                  // multipart file uploads
}

type flowStart struct {
	ID, Name, RetryOf string
	Retry             bool
	Attributes        map[string]string
}

type flowFinish struct{ Status, Issue string }

// newFlowListener points the package globals at a fake server for one test.
func newFlowListener(t *testing.T) *flowServer {
	t.Helper()
	fs := &flowServer{finishes: map[string]flowFinish{}, logs: map[string][]sentLog{}}
	tl := newTestListener(t, func(w http.ResponseWriter, r *http.Request) {
		fs.mu.Lock()
		defer fs.mu.Unlock()
		switch {
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/log"):
			for _, l := range decodeLogRQs(t, r) {
				fs.logs[*l.ItemUuid] = append(
					fs.logs[*l.ItemUuid],
					sentLog{item: *l.ItemUuid, level: *l.Level, msg: *l.Message, at: l.Time},
				)
			}
			if r.MultipartForm != nil {
				for _, files := range r.MultipartForm.File {
					fs.uploads += len(files)
				}
			}
			writeJSON(w, `{}`)
		case r.Method == http.MethodPost:
			var rq openapi.StartTestItemRQ
			_ = json.NewDecoder(r.Body).Decode(&rq)
			fs.seq++
			id := fmt.Sprintf("item-%d", fs.seq)
			attrs := map[string]string{}
			for _, a := range rq.Attributes {
				if a.Key != nil {
					attrs[*a.Key] = a.Value
				}
			}
			s := flowStart{ID: id, Name: rq.Name, Attributes: attrs}
			if rq.Retry != nil {
				s.Retry = *rq.Retry
			}
			if rq.RetryOf != nil {
				s.RetryOf = *rq.RetryOf
			}
			fs.starts = append(fs.starts, s)
			writeJSON(w, `{"id":"`+id+`"}`)
		case r.Method == http.MethodPut:
			var rq openapi.FinishTestItemRQ
			_ = json.NewDecoder(r.Body).Decode(&rq)
			f := flowFinish{}
			if rq.Status != nil {
				f.Status = *rq.Status
			}
			if rq.Issue != nil {
				f.Issue = rq.Issue.IssueType
			}
			fs.finishes[r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:]] = f
			writeJSON(w, `{}`)
		default:
			writeJSON(w, `{}`)
		}
	})
	oldClient, oldListener := globalClient, globalListener
	globalClient, globalListener = tl.client, tl
	t.Cleanup(func() { globalClient, globalListener = oldClient, oldListener })
	return fs
}

func (fs *flowServer) messages(item string) string {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	var out []string
	for _, l := range fs.logs[item] {
		out = append(out, l.level+": "+l.msg)
	}
	return strings.Join(out, "\n")
}

func (fs *flowServer) startNamed(name string) []flowStart {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	var out []flowStart
	for _, s := range fs.starts {
		if s.Name == name {
			out = append(out, s)
		}
	}
	return out
}

// storeActive registers the spec item the way ReportBeforeEach would have.
func storeActive(t *testing.T, spec types.SpecReport) {
	t.Helper()
	template := StartItem{
		Name:     spec.LeafNodeText,
		ParentID: "container-1",
		Type:     gorp.TestItemTypes.Step,
	}
	activeHierarchies.Store(
		spec.FullText(),
		SpecHierarchyIDs{StepID: "spec-item", StartTime: at(0), Template: template},
	)
	t.Cleanup(func() { activeHierarchies.Delete(spec.FullText()) })
}

func TestFinishSpecHierarchyReportsEachFlakeAttemptAsALinkedRetry(t *testing.T) {
	fs := newFlowListener(t)
	secondary := types.AdditionalFailure{
		State: types.SpecStateFailed,
		Failure: types.Failure{
			Message:          "AfterEach also broke",
			TimelineLocation: types.TimelineLocation{Order: 3, Time: at(30)},
		},
	}
	spec := types.SpecReport{
		LeafNodeText:     "flaky",
		State:            types.SpecStatePassed,
		StartTime:        at(0),
		EndTime:          at(100),
		NumAttempts:      2,
		MaxFlakeAttempts: 2,
		SpecEvents: []types.SpecEvent{
			byStart("first try", 1, 10),
			{
				SpecEventType:    types.SpecEventSpecRetry,
				TimelineLocation: types.TimelineLocation{Order: 5, Time: at(50)},
			},
			byStart("second try", 6, 60),
		},
		AdditionalFailures: []types.AdditionalFailure{
			{
				State: types.SpecStateFailed,
				Failure: types.Failure{
					Message:          "Failure recorded during attempt 1:\nfirst attempt failed",
					Location:         types.CodeLocation{FileName: "x_test.go", LineNumber: 7},
					TimelineLocation: types.TimelineLocation{Order: 2, Time: at(20)},
				},
			},
			secondary,
		},
		ReportEntries: types.ReportEntries{
			{
				Name:             "RP_LOG#rp_log#INFO#log in attempt 1",
				Time:             at(15),
				TimelineLocation: types.TimelineLocation{Order: 1, Time: at(15)},
			},
			{
				Name:             "RP_LOG#rp_log#ERROR#log in attempt 2",
				Time:             at(70),
				TimelineLocation: types.TimelineLocation{Order: 7, Time: at(70)},
			},
		},
	}
	storeActive(t, spec)
	finishSpecHierarchy(spec)

	// Attempt 1 reuses the item ReportBeforeEach started, so the only start of the spec's
	// name is the retry of it.
	starts := fs.startNamed("flaky")
	if len(starts) != 1 || !starts[0].Retry || starts[0].RetryOf != "spec-item" {
		t.Fatalf("starts of the flaky spec = %+v, want one retry of spec-item", starts)
	}
	retry := starts[0]
	if fs.finishes["spec-item"].Status != "FAILED" || fs.finishes[retry.ID].Status != "PASSED" {
		t.Errorf("statuses = %+v, want attempt 1 FAILED and attempt 2 PASSED", fs.finishes)
	}

	first := fs.messages("spec-item") + "\n" + fs.messages(fs.byStepID(t, "first try"))
	if !strings.Contains(first, "Attempt 1 of 2") ||
		!strings.Contains(first, "first attempt failed") ||
		!strings.Contains(first, "ADDITIONAL FAILURE") ||
		!strings.Contains(first, "AfterEach also broke") ||
		!strings.Contains(first, "log in attempt 1") {
		t.Errorf("attempt 1 logs:\n%s", first)
	}
	if strings.Contains(first, "Failure recorded during attempt") {
		t.Errorf("the 'recorded during attempt' prefix must be stripped:\n%s", first)
	}
	second := fs.messages(retry.ID) + "\n" + fs.messages(fs.byStepID(t, "second try"))
	if !strings.Contains(second, "Attempt 2 of 2") ||
		!strings.Contains(second, "log in attempt 2") ||
		strings.Contains(second, "first attempt failed") {
		t.Errorf("attempt 2 logs:\n%s", second)
	}
}

func (fs *flowServer) byStepID(t *testing.T, name string) string {
	t.Helper()
	fs.mu.Lock()
	defer fs.mu.Unlock()
	for _, s := range fs.starts {
		if s.Name == name {
			return s.ID
		}
	}
	t.Fatalf("no item named %q", name)
	return ""
}

func TestFinishSpecHierarchyKeepsRepetitionsInOneItem(t *testing.T) {
	fs := newFlowListener(t)
	spec := types.SpecReport{
		LeafNodeText:          "repeated",
		State:                 types.SpecStatePassed,
		StartTime:             at(0),
		EndTime:               at(100),
		NumAttempts:           3,
		MaxMustPassRepeatedly: 3,
		SpecEvents: []types.SpecEvent{
			{
				SpecEventType:    types.SpecEventSpecRepeat,
				TimelineLocation: types.TimelineLocation{Order: 3, Time: at(30)},
			},
			{
				SpecEventType:    types.SpecEventSpecRepeat,
				TimelineLocation: types.TimelineLocation{Order: 6, Time: at(60)},
			},
		},
	}
	storeActive(t, spec)
	finishSpecHierarchy(spec)

	for _, s := range fs.starts {
		if s.Retry {
			t.Errorf("a repetition was started as a retry: %+v", s)
		}
	}
	msgs := fs.messages("spec-item")
	for k := 1; k <= 3; k++ {
		if !strings.Contains(msgs, fmt.Sprintf("Repetition %d of 3", k)) {
			t.Errorf("missing repetition %d:\n%s", k, msgs)
		}
	}
	if fs.finishes["spec-item"].Status != "PASSED" {
		t.Errorf("finish = %+v", fs.finishes["spec-item"])
	}
}

func TestRPLogEntriesKeepTheirLevelAndMalformedOnesAreSkipped(t *testing.T) {
	fs := newFlowListener(t)
	entries := types.ReportEntries{}
	for i, e := range []string{
		"RP_LOG#rp_log#DEBUG#d", "RP_LOG#rp_log#INFO#i", "RP_LOG#rp_log#WARN#w", "RP_LOG#rp_log#WARNING#w2",
		"RP_LOG#rp_log#ERROR#e", "RP_LOG#rp_log#FATAL#f", "RP_LOG#rp_log#whatever#unknown level",
		"RP_LOG#too-short", "not an agent entry",
	} {
		entries = append(
			entries,
			types.ReportEntry{
				Name:             e,
				Time:             at(10 + i),
				TimelineLocation: types.TimelineLocation{Order: i + 1, Time: at(10 + i)},
			},
		)
	}
	spec := types.SpecReport{
		LeafNodeText:  "levels",
		State:         types.SpecStatePassed,
		StartTime:     at(0),
		EndTime:       at(100),
		ReportEntries: entries,
	}
	storeActive(t, spec)
	finishSpecHierarchy(spec)

	msgs := fs.messages("spec-item")
	for _, want := range []string{
		"DEBUG: d", "INFO: i", "WARN: w", "WARN: w2", "ERROR: e", "FATAL: f", "INFO: unknown level",
	} {
		if !strings.Contains(msgs, want) {
			t.Errorf("log %q missing:\n%s", want, msgs)
		}
	}
	if strings.Contains(msgs, "too-short") || strings.Contains(msgs, "not an agent entry") {
		t.Errorf("a malformed or foreign entry was sent:\n%s", msgs)
	}
}

func TestAttachmentEntriesAreUploadedOrExplained(t *testing.T) {
	fs := newFlowListener(t)
	dir := t.TempDir()
	staged := filepath.Join(dir, "staged.txt")
	if err := os.WriteFile(staged, []byte("payload"), 0o600); err != nil {
		t.Fatal(err)
	}
	good, _ := json.Marshal(
		attachmentPayload{
			TempPath:         staged,
			OriginalFilename: "evidence.txt",
			Message:          "the evidence",
		},
	)
	bad, _ := json.Marshal(
		attachmentError{
			File:    "big.txt",
			Message: "too big",
			Reason:  "the file is 10 bytes, the limit is 3 bytes",
		},
	)
	entry := func(name, value string, order int) types.ReportEntry {
		return types.ReportEntry{
			Name:             name,
			Value:            types.WrapEntryValue(value),
			Time:             at(10 + order),
			TimelineLocation: types.TimelineLocation{Order: order, Time: at(10 + order)},
		}
	}
	spec := types.SpecReport{
		LeafNodeText: "attaches", State: types.SpecStatePassed, StartTime: at(0), EndTime: at(100),
		ReportEntries: types.ReportEntries{
			entry("evidence.txt", attachmentPrefix+string(good), 1),
			entry("broken", attachmentPrefix+"{not json", 2),
			entry("big.txt", attachmentErrorPrefix+string(bad), 3),
			entry("odd", attachmentErrorPrefix+"unreadable payload", 4),
			entry("unrelated", "just a value", 5),
		},
	}
	storeActive(t, spec)
	finishSpecHierarchy(spec)

	if fs.uploads != 1 {
		t.Errorf("uploads = %d, want exactly the one valid attachment", fs.uploads)
	}
	if _, err := os.Stat(staged); !os.IsNotExist(err) {
		t.Error("the staged copy must be removed after a successful upload")
	}
	msgs := fs.messages("spec-item")
	if !strings.Contains(
		msgs,
		`Attachment not uploaded: big.txt ("too big"): the file is 10 bytes, the limit is 3 bytes`,
	) {
		t.Errorf("the readable error log is missing:\n%s", msgs)
	}
	if !strings.Contains(msgs, "Attachment error: unreadable payload") {
		t.Errorf("an unreadable error payload must be logged as it is:\n%s", msgs)
	}
}

func TestStartSpecHierarchySendsPriorityRefAndLabels(t *testing.T) {
	t.Setenv("TMPDIR", t.TempDir())
	t.Setenv("RP_SESSION_ID", "start-hierarchy-test")
	fs := newFlowListener(t)

	lc, err := NewLaunchCoordinator("start-hierarchy-test")
	if err != nil {
		t.Fatal(err)
	}
	oldCoord, oldLaunch, oldSuite := coordinator, globalLaunchID, globalSuiteID
	coordinator, globalLaunchID, globalSuiteID = lc, "launch-1", "suite-1"
	t.Cleanup(func() { coordinator, globalLaunchID, globalSuiteID = oldCoord, oldLaunch, oldSuite })

	level := 2
	spec := types.SpecReport{
		ContainerHierarchyTexts: []string{"Checkout"},
		LeafNodeText:            "pays",
		LeafNodeLabels:          []string{"smoke"},
	}
	ids := startSpecHierarchy(spec, []*Parameter{{Key: "card", Value: "visa"}}, &level, "PAY-1")
	if ids.StepID == "" || len(ids.ContainerIDs) != 1 {
		t.Fatalf("ids = %+v, want a spec item under one container", ids)
	}
	started := fs.startNamed("pays")
	if len(started) != 1 || started[0].Attributes["priority"] != "P2" ||
		started[0].Attributes["testReferenceId"] != "PAY-1" {
		t.Errorf("spec start = %+v, want priority P2 and the reference", started)
	}

	// Without the agent's state nothing is started: reporting must never break the run.
	coordinator = nil
	if got := startSpecHierarchy(spec, nil, nil, ""); got.StepID != "" {
		t.Errorf("an uninitialized agent started an item: %+v", got)
	}
}

func TestFinalizeLaunchClosesTheLaunchAndPrintsTheServersLink(t *testing.T) {
	t.Setenv("TMPDIR", t.TempDir())
	const session = "finalize-test"
	t.Setenv("RP_SESSION_ID", session)

	var status string
	var finishes int
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut && strings.HasSuffix(r.URL.Path, "/finish") {
			var rq openapi.FinishExecutionRQ
			_ = json.NewDecoder(r.Body).Decode(&rq)
			finishes++
			if rq.Status != nil {
				status = *rq.Status
			}
			writeJSON(w, `{"id":"L1","link":"http://rp/ui/#p/launches/all/L1"}`)
			return
		}
		writeJSON(w, `{}`)
	})
	t.Setenv("RP_ENDPOINT", srv.URL)
	t.Setenv("RP_PROJECT", "p")
	t.Setenv("RP_API_KEY", "k")
	t.Setenv("RP_CONFIG_FILE", filepath.Join(t.TempDir(), "none.properties"))

	if err := FinalizeLaunch(context.Background(), false); err == nil {
		t.Error("finalizing without an agent state file must fail")
	}

	lc, _ := NewLaunchCoordinator(session)
	state, _ := json.Marshal(
		AgentState{Endpoint: srv.URL, Project: "p", LaunchID: "L1", SessionID: session},
	)
	if err := os.WriteFile(lc.stateFile, state, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := FinalizeLaunch(context.Background(), false); err == nil {
		t.Error("finalizing without aggregated results must fail")
	}

	passed := []SharedSpecStatus{
		{SpecName: "a", Status: "passed"},
		{SpecName: "b", Status: "passed", Priority: 2},
	}
	if err := lc.SetAggregatedResults(passed); err != nil {
		t.Fatal(err)
	}
	out := captureStdout(t, func() {
		if err := FinalizeLaunch(context.Background(), false); err != nil {
			t.Fatal(err)
		}
	})
	if status != "PASSED" || !strings.Contains(out, "launch L1 finalized (2 total tests)") ||
		!strings.Contains(out, "→ http://rp/ui/#p/launches/all/L1") {
		t.Errorf("status=%q output=%q", status, out)
	}

	captureStdout(t, func() { _ = FinalizeLaunch(context.Background(), true) })
	if status != "FAILED" {
		t.Errorf("forceFailed must finish the launch FAILED, got %q", status)
	}
	failed := append(
		passed,
		SharedSpecStatus{SpecName: "c", Status: "failed"},
		SharedSpecStatus{SpecName: "d", Status: "skipped"},
	)
	_ = lc.SetAggregatedResults(failed)
	captureStdout(t, func() { _ = FinalizeLaunch(context.Background(), false) })
	if status != "FAILED" {
		t.Errorf("a failed spec must finish the launch FAILED, got %q", status)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	before := finishes
	if err := FinalizeLaunch(ctx, false); err == nil || finishes != before {
		t.Errorf(
			"a canceled context must stop finalization (err=%v, finishes %d -> %d)",
			err,
			before,
			finishes,
		)
	}

	t.Setenv("RP_SESSION_ID", "")
	if err := FinalizeLaunch(context.Background(), false); err == nil {
		t.Error("finalizing without a session must fail")
	}
}

func TestAggregatedStatusCountsSpecsByPriorityAndSkipsFixtures(t *testing.T) {
	t.Setenv("TMPDIR", t.TempDir())
	lc, _ := NewLaunchCoordinator("aggregate-test")
	if _, err := lc.GetAggregatedStatus(); err == nil {
		t.Error("a missing results file must be an error")
	}
	_ = lc.SetAggregatedResults([]SharedSpecStatus{
		{Status: "passed", Priority: 0},
		{Status: "failed", Priority: 0},
		{Status: "panicked", Priority: 1},
		{Status: "skipped", Priority: 4},
		{Status: "pending", Priority: 9},
		{Status: "failed", Fixture: true},
	})
	got, err := lc.GetAggregatedStatus()
	if err != nil {
		t.Fatal(err)
	}
	if got.OverallStatus != (Stats{Total: 5, Passed: 1, Failed: 2, Skipped: 2}) {
		t.Errorf("overall = %+v, want the failed fixture left out", got.OverallStatus)
	}
	if p0 := got.PriorityStatus.GetStats(0); p0.Total != 2 || p0.Passed != 1 || p0.Failed != 1 {
		t.Errorf("P0 = %+v", p0)
	}
	if got.PriorityStatus.GetStats(5) != nil || got.PriorityStatus.GetStats(-1) != nil {
		t.Error("an out-of-range priority has no stats")
	}
	_ = os.WriteFile(lc.finalStatusFile, []byte("{not json"), 0o600)
	if _, err := lc.GetAggregatedStatus(); err == nil {
		t.Error("a corrupt results file must be an error")
	}
}

func newTestServer(t *testing.T, h http.HandlerFunc) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return srv
}

// captureStdout returns what fn printed to stdout.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	done := make(chan string)
	go func() { b, _ := io.ReadAll(r); done <- string(b) }()
	func() {
		defer func() { os.Stdout = old; _ = w.Close() }()
		fn()
	}()
	return <-done
}
