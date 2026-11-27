package commands

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// recordedItem is a test item start as seen by hierarchyServer.
type recordedItem struct {
	ID       string
	ParentID string // empty for a root item
	Body     map[string]any
}

// hierarchyServer is a ReportPortal fake that gives every started item its own ID and records the
// requests, so tests can check the reported hierarchy and request bodies.
type hierarchyServer struct {
	mu          sync.Mutex
	items       []recordedItem
	finishes    map[string]map[string]any // item ID -> finish body
	launchStart map[string]any
}

func newHierarchyServer(t *testing.T, project string) (*hierarchyServer, *httptest.Server) {
	t.Helper()
	rec := &hierarchyServer{finishes: map[string]map[string]any{}}
	decode := func(r *http.Request) map[string]any {
		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		return body
	}
	reply := func(w http.ResponseWriter, v any) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(v)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v2/"+project+"/launch", func(w http.ResponseWriter, r *http.Request) {
		rec.mu.Lock()
		rec.launchStart = decode(r)
		rec.mu.Unlock()
		reply(w, map[string]string{"id": "launch-1"})
	})
	mux.HandleFunc(
		"PUT /api/v2/"+project+"/launch/launch-1/finish",
		func(w http.ResponseWriter, _ *http.Request) {
			reply(
				w,
				map[string]string{"id": "launch-1", "link": "http://rp.example/ui/#launches/all/1"},
			)
		},
	)
	mux.HandleFunc("POST /api/v2/"+project+"/item/", func(w http.ResponseWriter, r *http.Request) {
		body := decode(r)
		rec.mu.Lock()
		id := fmt.Sprintf("item-%d", len(rec.items)+1)
		rec.items = append(rec.items, recordedItem{
			ID:       id,
			ParentID: strings.TrimPrefix(r.URL.Path, "/api/v2/"+project+"/item/"),
			Body:     body,
		})
		rec.mu.Unlock()
		reply(w, map[string]string{"id": id})
	})
	mux.HandleFunc(
		"PUT /api/v2/"+project+"/item/{id}",
		func(w http.ResponseWriter, r *http.Request) {
			body := decode(r)
			rec.mu.Lock()
			rec.finishes[r.PathValue("id")] = body
			rec.mu.Unlock()
			reply(w, map[string]string{"message": "finished"})
		},
	)
	mux.HandleFunc("POST /api/v2/"+project+"/log", func(w http.ResponseWriter, _ *http.Request) {
		reply(w, map[string]any{"responses": []map[string]string{{"id": "log-1"}}})
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return rec, srv
}

// item returns the recorded start of the item with the given name.
func (rec *hierarchyServer) item(t *testing.T, name string) recordedItem {
	t.Helper()
	rec.mu.Lock()
	defer rec.mu.Unlock()
	for _, it := range rec.items {
		if it.Body["name"] == name {
			return it
		}
	}
	require.Failf(t, "item not reported", "no item named %q", name)
	return recordedItem{}
}

func writeEventLines(t *testing.T, lines ...string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "events.jsonl")
	require.NoError(t, os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o600))
	return path
}

func event(action, test string) string {
	if test == "" {
		return fmt.Sprintf(
			`{"time":"2025-01-01T00:00:00Z","action":%q,"package":"example/pkg"}`,
			action,
		)
	}
	return fmt.Sprintf(
		`{"time":"2025-01-01T00:00:00Z","action":%q,"package":"example/pkg","test":%q}`,
		action,
		test,
	)
}

func TestReportLaunch_TestsAreStepsAndSubtestsNest(t *testing.T) {
	t.Parallel()

	rec, srv := newHierarchyServer(t, "proj")
	file := writeEventLines(t,
		event("run", "TestTable"),
		event("run", "TestTable/a"),
		event("run", "TestTable/a/deep"),
		event("pass", "TestTable/a/deep"),
		event("pass", "TestTable/a"),
		event("run", "TestTable/b"),
		event("fail", "TestTable/b"),
		event("fail", "TestTable"),
		event("run", "TestPlain"),
		event("pass", "TestPlain"),
		event("fail", ""),
	)

	cfg := &clientConfig{URL: srv.URL, Project: "proj", ApiKey: "tok"}
	_, link, err := reportLaunch(t.Context(), cfg, reportCmd(file, "launch", false))
	require.NoError(t, err)
	assert.Equal(
		t,
		"http://rp.example/ui/#launches/all/1",
		link,
		"the server's launch link is returned",
	)

	suite := rec.item(t, "example/pkg")
	assert.Equal(t, "SUITE", suite.Body["type"])

	table := rec.item(t, "TestTable")
	assert.Equal(t, suite.ID, table.ParentID, "a top-level test is a child of the package suite")

	a := rec.item(t, "a")
	assert.Equal(t, table.ID, a.ParentID, "a subtest is a child of its parent test")
	assert.Equal(
		t,
		"example/pkg/TestTable/a",
		a.Body["testCaseId"],
		"the identity keeps the full test path",
	)

	deep := rec.item(t, "deep")
	assert.Equal(t, a.ID, deep.ParentID, "a nested subtest is a child of its own parent")

	b := rec.item(t, "b")
	assert.Equal(t, table.ID, b.ParentID)

	plain := rec.item(t, "TestPlain")
	assert.Equal(t, suite.ID, plain.ParentID)

	// ReportPortal counts executions only for STEP items, so every test must be a STEP.
	for _, it := range []recordedItem{table, a, deep, b, plain} {
		assert.Equal(t, "STEP", it.Body["type"], "item %v", it.Body["name"])
		assert.Equal(t, true, it.Body["hasStats"], "item %v", it.Body["name"])
	}
}

func TestReportLaunch_SkippedTest(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name         string
		skippedIssue bool
		wantIssue    any
	}{
		{name: "skipped tests stay To Investigate by default", skippedIssue: true, wantIssue: nil},
		{
			name:         "skipped tests are No Defect with --skipped-issue=false",
			skippedIssue: false,
			wantIssue:    map[string]any{"issueType": "NOT_ISSUE"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			rec, srv := newHierarchyServer(t, "proj")
			file := writeEventLines(t,
				event("run", "TestSkip"),
				event("skip", "TestSkip"),
				event("pass", ""),
			)
			cmd := reportCmd(file, "launch", false)
			require.NoError(t, cmd.Set("skipped-issue", fmt.Sprint(tc.skippedIssue)))

			cfg := &clientConfig{URL: srv.URL, Project: "proj", ApiKey: "tok"}
			_, _, err := reportLaunch(t.Context(), cfg, cmd)
			require.NoError(t, err)

			skipped := rec.item(t, "TestSkip")
			rec.mu.Lock()
			finish := rec.finishes[skipped.ID]
			rec.mu.Unlock()
			require.NotNil(t, finish, "the skipped test must be finished by the reporter")
			assert.Equal(t, "SKIPPED", finish["status"])
			assert.Equal(t, tc.wantIssue, finish["issue"])

			assert.Contains(t, rec.launchStart["attributes"], map[string]any{
				"key": "skippedIssue", "value": fmt.Sprint(tc.skippedIssue), "system": true,
			})
		})
	}
}

func TestReportLaunch_SystemAttributes(t *testing.T) {
	t.Parallel()

	rec, srv := newHierarchyServer(t, "proj")
	file := writeEventLines(t, event("run", "TestOne"), event("pass", "TestOne"), event("pass", ""))
	cmd := reportCmd(file, "launch", false)
	require.NoError(t, cmd.Set("attr", "build:42"))

	cfg := &clientConfig{URL: srv.URL, Project: "proj", ApiKey: "tok"}
	_, _, err := reportLaunch(t.Context(), cfg, cmd)
	require.NoError(t, err)

	attrs, ok := rec.launchStart["attributes"].([]any)
	require.True(t, ok)
	byKey := map[string]map[string]any{}
	for _, a := range attrs {
		m, isMap := a.(map[string]any)
		require.True(t, isMap)
		key, _ := m["key"].(string)
		byKey[key] = m
	}
	assert.Equal(t, "42", byKey["build"]["value"], "user attributes are kept")
	assert.NotEqual(t, true, byKey["build"]["system"], "user attributes are not system attributes")
	for _, key := range []string{"agent", "os", "go"} {
		require.Contains(t, byKey, key)
		assert.Equal(t, true, byKey[key]["system"], "%s is a system attribute", key)
	}
	assert.True(t, strings.HasPrefix(byKey["agent"]["value"].(string), "gorp|"))
}

func TestTest2JSON_PrintsLaunchLink(t *testing.T) {
	t.Setenv("HOME", t.TempDir()) // no ~/.gorp

	_, srv := newHierarchyServer(t, "proj")
	file := writeEventLines(t, event("run", "TestOne"), event("pass", "TestOne"), event("pass", ""))
	cmd, out := test2jsonCmd(srv.URL, file)

	require.NoError(t, reportTest2Json(t.Context(), cmd))
	assert.Contains(t, out.String(), "ReportPortal Launch: http://rp.example/ui/#launches/all/1")
}

func TestReportLaunch_SubtestParentMustBeRunning(t *testing.T) {
	t.Parallel()

	rec, srv := newHierarchyServer(t, "proj")
	file := writeEventLines(t,
		event("run", "TestX"),
		// t.Run("c") finishes, then a sibling t.Run("c/d") runs: "TestX/c/d" is not a child of "c"
		event("run", "TestX/c"),
		event("pass", "TestX/c"),
		event("run", "TestX/c/d"),
		event("pass", "TestX/c/d"),
		// t.Run("a") calls t.Parallel() and pauses, then a sibling t.Run("a/b") runs
		event("run", "TestX/a"),
		event("pause", "TestX/a"),
		event("run", "TestX/a/b"),
		event("pass", "TestX/a/b"),
		event("cont", "TestX/a"),
		// a real subtest of the resumed "a"
		event("run", "TestX/a/real"),
		event("pass", "TestX/a/real"),
		event("pass", "TestX/a"),
		event("pass", "TestX"),
		event("pass", ""),
	)

	cfg := &clientConfig{URL: srv.URL, Project: "proj", ApiKey: "tok"}
	_, _, err := reportLaunch(t.Context(), cfg, reportCmd(file, "launch", false))
	require.NoError(t, err)

	x := rec.item(t, "TestX")
	assert.Equal(
		t,
		x.ID,
		rec.item(t, "c/d").ParentID,
		"a sibling named like a finished test is not its child",
	)
	assert.Equal(
		t,
		x.ID,
		rec.item(t, "a/b").ParentID,
		"a sibling named like a paused test is not its child",
	)
	assert.Equal(
		t,
		rec.item(t, "a").ID,
		rec.item(t, "real").ParentID,
		"a resumed test can have subtests",
	)
}

func TestReportLaunch_PackageWithoutTestFilesWithReportEmpty(t *testing.T) {
	t.Parallel()

	rec, srv := newHierarchyServer(t, "proj")
	// before Go 1.20 a package without test files produced a lone "skip" with no "start"
	file := writeEventLines(
		t,
		event("skip", ""),
		`{"time":"2025-01-01T00:00:00Z","action":"start","package":"example/other"}`,
		`{"time":"2025-01-01T00:00:00Z","action":"run","package":"example/other","test":"TestOne"}`,
		`{"time":"2025-01-01T00:00:00Z","action":"pass","package":"example/other","test":"TestOne"}`,
		`{"time":"2025-01-01T00:00:00Z","action":"pass","package":"example/other"}`,
	)

	cfg := &clientConfig{URL: srv.URL, Project: "proj", ApiKey: "tok"}
	_, _, err := reportLaunch(t.Context(), cfg, reportCmd(file, "launch", true))
	require.NoError(t, err, "a package-level skip without a suite must not interrupt the launch")
	assert.Equal(
		t,
		"SUITE",
		rec.item(t, "example/other").Body["type"],
		"the other package is still reported",
	)
}
