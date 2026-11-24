package rpagent

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/onsi/ginkgo/v2/types"

	"github.com/reportportal/goRP/v5/pkg/openapi"
)

func fixtureReport(
	nodeType types.NodeType,
	state types.SpecState,
	line int,
	msg string,
) types.SpecReport {
	return types.SpecReport{
		LeafNodeType:     nodeType,
		State:            state,
		LeafNodeLocation: types.CodeLocation{FileName: "/work/suite_test.go", LineNumber: line},
		StartTime:        at(0),
		EndTime:          at(25),
		Failure: types.Failure{
			Message:  msg,
			Location: types.CodeLocation{FileName: "/work/suite_test.go", LineNumber: line + 2},
		},
	}
}

func TestSuiteFixtureFailures(t *testing.T) {
	failedBefore := fixtureReport(
		types.NodeTypeBeforeSuite,
		types.SpecStateFailed,
		43,
		"database is not reachable",
	)
	failedAfter := fixtureReport(
		types.NodeTypeAfterSuite,
		types.SpecStateFailed,
		50,
		"could not drop schema",
	)

	t.Run("a failed BeforeSuite is reported", func(t *testing.T) {
		got := suiteFixtureFailures(types.SpecReports{failedBefore})
		if len(got) != 1 || got[0].name != "[BeforeSuite]" || got[0].teardown ||
			got[0].message != "database is not reachable" {
			t.Fatalf("got %+v", got)
		}
		if got[0].node.LineNumber != 43 || got[0].failedAt.LineNumber != 45 {
			t.Errorf("locations = node %+v, failure %+v", got[0].node, got[0].failedAt)
		}
	})

	t.Run("a failed AfterSuite is a teardown fixture", func(t *testing.T) {
		got := suiteFixtureFailures(types.SpecReports{failedAfter})
		if len(got) != 1 || got[0].name != "[AfterSuite]" || !got[0].teardown {
			t.Fatalf("got %+v", got)
		}
	})

	t.Run("the same failure from every parallel process is reported once", func(t *testing.T) {
		got := suiteFixtureFailures(types.SpecReports{failedBefore, failedBefore, failedBefore})
		if len(got) != 1 {
			t.Errorf("want 1, got %d", len(got))
		}
	})

	t.Run(
		"for a SynchronizedBeforeSuite the first, real failure wins over the generic one",
		func(t *testing.T) {
			real := fixtureReport(
				types.NodeTypeSynchronizedBeforeSuite,
				types.SpecStateFailed,
				43,
				"database is not reachable",
			)
			generic := fixtureReport(
				types.NodeTypeSynchronizedBeforeSuite,
				types.SpecStateFailed,
				43,
				"\x1b[1m\x1b[38;5;9mSynchronizedBeforeSuite failed on Ginkgo parallel process #1\x1b[0m\n",
			)
			got := suiteFixtureFailures(types.SpecReports{real, generic})
			if len(got) != 1 || got[0].message != "database is not reachable" ||
				got[0].name != "[SynchronizedBeforeSuite]" {
				t.Errorf("got %+v", got)
			}
		},
	)

	t.Run("passing fixtures and failing specs are ignored", func(t *testing.T) {
		spec := fixtureReport(types.NodeTypeIt, types.SpecStateFailed, 60, "an assertion")
		passed := fixtureReport(types.NodeTypeBeforeSuite, types.SpecStatePassed, 43, "")
		if got := suiteFixtureFailures(types.SpecReports{spec, passed}); len(got) != 0 {
			t.Errorf("got %+v", got)
		}
	})

	t.Run("both a setup and a teardown failure are reported, in order", func(t *testing.T) {
		got := suiteFixtureFailures(types.SpecReports{failedBefore, failedAfter})
		if len(got) != 2 || got[0].name != "[BeforeSuite]" || got[1].name != "[AfterSuite]" {
			t.Errorf("got %+v", got)
		}
	})

	t.Run("interrupted and timed-out fixtures count, ANSI codes are stripped", func(t *testing.T) {
		timedOut := fixtureReport(
			types.NodeTypeAfterSuite,
			types.SpecStateTimedout,
			50,
			"\x1b[31mtook too long\x1b[0m",
		)
		got := suiteFixtureFailures(types.SpecReports{timedOut})
		if len(got) != 1 || got[0].state != "timedout" || got[0].message != "took too long" {
			t.Errorf("got %+v", got)
		}
	})
}

type recordedRequest struct {
	method, path string
	body         []byte
}

// recordRequests points the package globals at a mock server and records every request.
func recordRequests(t *testing.T, suiteID string) func() []recordedRequest {
	t.Helper()
	var mu sync.Mutex
	var reqs []recordedRequest
	var seq int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		reqs = append(reqs, recordedRequest{r.Method, r.URL.Path, body})
		seq++
		id := "fx-" + string(rune('0'+seq))
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPost && !strings.HasSuffix(r.URL.Path, "/log") {
			_, _ = w.Write([]byte(`{"id":"` + id + `"}`))
			return
		}
		_, _ = w.Write([]byte(`{}`))
	}))
	t.Cleanup(srv.Close)

	t.Setenv("TMPDIR", t.TempDir())
	t.Setenv("RP_SESSION_ID", "fx")
	oldClient, oldListener, oldSuite, oldState := globalClient, globalListener, globalSuiteID, agentState
	tl := NewTestListener(NewReportingClient(srv.URL, "proj", "key", false), "launch-1", "L")
	globalClient, globalListener, globalSuiteID = tl.client, tl, suiteID
	agentState = &AgentState{SessionID: "fx", LaunchID: "launch-1"}
	t.Cleanup(func() {
		globalClient, globalListener, globalSuiteID, agentState = oldClient, oldListener, oldSuite, oldState
	})

	return func() []recordedRequest {
		mu.Lock()
		defer mu.Unlock()
		return append([]recordedRequest(nil), reqs...)
	}
}

func TestReportSuiteFixtureFailuresCreatesItemLogAndStatus(t *testing.T) {
	requests := recordRequests(t, "suite-1")

	wd, _ := os.Getwd()
	before := fixtureReport(
		types.NodeTypeBeforeSuite,
		types.SpecStateFailed,
		43,
		"database is not reachable\n\x1b[31mExpected\x1b[0m",
	)
	before.LeafNodeLocation.FileName = filepath.Join(wd, "suite_test.go")
	before.Failure.Location.FileName = filepath.Join(wd, "suite_test.go")
	after := fixtureReport(
		types.NodeTypeAfterSuite,
		types.SpecStateFailed,
		50,
		"could not drop schema",
	)
	after.LeafNodeLocation.FileName = filepath.Join(wd, "suite_test.go")
	after.Failure.Location.FileName = filepath.Join(wd, "suite_test.go")

	reportSuiteFixtureFailures(types.Report{
		SpecReports: types.SpecReports{before, before, after}, // `before` twice: two processes
		PreRunStats: types.PreRunStats{TotalSpecs: 2, SpecsThatWillRun: 2},
	})

	var starts []openapi.StartTestItemRQ
	var startPaths []string
	var logs []openapi.SaveLogRQ
	finishes := map[string]openapi.FinishTestItemRQ{}
	for _, r := range requests() {
		switch {
		case r.method == http.MethodPost && strings.HasSuffix(r.path, "/log"):
			var l openapi.SaveLogRQ
			_ = json.Unmarshal(r.body, &l)
			logs = append(logs, l)
		case r.method == http.MethodPost:
			var s openapi.StartTestItemRQ
			_ = json.Unmarshal(r.body, &s)
			starts = append(starts, s)
			startPaths = append(startPaths, strings.TrimSuffix(r.path, "/"))
		case r.method == http.MethodPut:
			var f openapi.FinishTestItemRQ
			_ = json.Unmarshal(r.body, &f)
			finishes[r.path[strings.LastIndex(r.path, "/")+1:]] = f
		}
	}

	if len(starts) != 2 {
		t.Fatalf("want one item per distinct fixture failure (2), got %d", len(starts))
	}
	if starts[0].Type != "BEFORE_SUITE" || starts[0].Name != "[BeforeSuite]" ||
		startPaths[0] != "/api/v2/proj/item/suite-1" {
		t.Errorf("setup item = %+v at %s", starts[0], startPaths[0])
	}
	if starts[1].Type != "AFTER_SUITE" || starts[1].Name != "[AfterSuite]" {
		t.Errorf("teardown item = %+v", starts[1])
	}
	if starts[0].TestCaseId == nil ||
		!strings.HasSuffix(*starts[0].TestCaseId, " > [BeforeSuite]") ||
		starts[0].CodeRef == nil ||
		*starts[0].CodeRef != "rpagent/suite_test.go:43" {
		t.Errorf("identity = %+v", starts[0])
	}
	if !starts[0].StartTime.Equal(at(0)) {
		t.Errorf("start time = %v", starts[0].StartTime)
	}

	if len(logs) != 2 || *logs[0].ItemUuid != "fx-1" || *logs[0].Level != "ERROR" ||
		!logs[0].Time.Equal(at(25)) {
		t.Fatalf("logs = %+v", logs)
	}
	msg := *logs[0].Message
	if !strings.HasPrefix(msg, "[BeforeSuite] FAILED") ||
		!strings.Contains(msg, "suite_test.go:45") ||
		!strings.Contains(msg, "database is not reachable") ||
		strings.Contains(msg, "\x1b") {
		t.Errorf("message = %q", msg)
	}
	if !strings.HasSuffix(msg, "2 specs were not run because [BeforeSuite] failed.") {
		t.Errorf("a failed BeforeSuite must say how many specs did not run: %q", msg)
	}
	if strings.Contains(*logs[1].Message, "not run") {
		t.Errorf("an AfterSuite failure leaves no spec unrun: %q", *logs[1].Message)
	}
	if f := finishes["fx-1"]; f.Status == nil || *f.Status != "FAILED" || !f.EndTime.Equal(at(25)) {
		t.Errorf("setup finish = %+v", f)
	}

	// The wrapper learns about the failure from the status file, so it can fail the suite and launch.
	data, err := os.ReadFile(
		filepath.Join(os.TempDir(), "rp_status_fx_"+strconv.Itoa(os.Getpid())+".json"),
	)
	if err != nil {
		t.Fatalf("no status record was written: %v", err)
	}
	var records []SharedSpecStatus
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		var rec SharedSpecStatus
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			t.Fatal(err)
		}
		records = append(records, rec)
	}
	if len(records) != 2 || records[0].SpecName != "[BeforeSuite]" ||
		records[0].Status != "failed" ||
		!records[0].Fixture ||
		!strings.Contains(records[0].FailureMessage, "database is not reachable") ||
		!records[0].EndTime.Equal(at(25)) ||
		records[1].SpecName != "[AfterSuite]" {
		t.Errorf("records = %+v", records)
	}
}

func TestReportSuiteFixtureFailuresDoesNothingWhenThereAreNone(t *testing.T) {
	requests := recordRequests(t, "suite-1")
	passed := fixtureReport(types.NodeTypeBeforeSuite, types.SpecStatePassed, 43, "")
	reportSuiteFixtureFailures(types.Report{SpecReports: types.SpecReports{passed}})
	if got := requests(); len(got) != 0 {
		t.Errorf("a passing suite must send nothing, got %d requests", len(got))
	}

	requests = recordRequests(t, "") // no suite item: the agent is not set up
	reportSuiteFixtureFailures(
		types.Report{
			SpecReports: types.SpecReports{
				fixtureReport(types.NodeTypeAfterSuite, types.SpecStateFailed, 50, "x"),
			},
		},
	)
	if got := requests(); len(got) != 0 {
		t.Errorf("without a suite item nothing can be reported, got %d requests", len(got))
	}
}

func TestAggregatedStatusIgnoresFixtures(t *testing.T) {
	t.Setenv("TMPDIR", t.TempDir())
	lc, err := NewLaunchCoordinator("agg")
	if err != nil {
		t.Fatal(err)
	}
	err = lc.SetAggregatedResults([]SharedSpecStatus{
		{SpecName: "a", Status: "passed", Priority: 1},
		{SpecName: "b", Status: "passed", Priority: 1},
		{SpecName: "[AfterSuite]", Status: "failed", Priority: 1, Fixture: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	got, err := lc.GetAggregatedStatus()
	if err != nil {
		t.Fatal(err)
	}
	if got.OverallStatus.Total != 2 || got.OverallStatus.Failed != 0 ||
		got.OverallStatus.Passed != 2 {
		t.Errorf("a failed fixture is not a test and must not be counted: %+v", got.OverallStatus)
	}
}

func TestSpecsNotRun(t *testing.T) {
	spec := fixtureReport(types.NodeTypeIt, types.SpecStatePassed, 60, "")
	fixture := fixtureReport(types.NodeTypeBeforeSuite, types.SpecStateFailed, 43, "x")
	cases := []struct {
		name    string
		reports types.SpecReports
		willRun int
		want    int
	}{
		{
			"BeforeSuite failed: only fixtures were reported",
			types.SpecReports{fixture, fixture},
			2,
			2,
		},
		{"every spec ran", types.SpecReports{spec, spec, fixture}, 2, 0},
		{"some specs ran", types.SpecReports{spec, fixture}, 3, 2},
		{"never negative", types.SpecReports{spec, spec, spec}, 1, 0},
		{"no specs at all", nil, 0, 0},
	}
	for _, c := range cases {
		got := specsNotRun(
			types.Report{
				SpecReports: c.reports,
				PreRunStats: types.PreRunStats{SpecsThatWillRun: c.willRun},
			},
		)
		if got != c.want {
			t.Errorf("%s: got %d, want %d", c.name, got, c.want)
		}
	}
}

func TestNotRunNote(t *testing.T) {
	setup := fixtureFailure{name: "[BeforeSuite]"}
	teardown := fixtureFailure{name: "[AfterSuite]", teardown: true}
	if got := notRunNote(setup, 1); got != "\n\n1 spec was not run because [BeforeSuite] failed." {
		t.Errorf("singular: %q", got)
	}
	if got := notRunNote(
		setup,
		3,
	); got != "\n\n3 specs were not run because [BeforeSuite] failed." {
		t.Errorf("plural: %q", got)
	}
	if notRunNote(setup, 0) != "" || notRunNote(teardown, 5) != "" {
		t.Errorf("no note without unrun specs, and never for a teardown fixture")
	}
}
