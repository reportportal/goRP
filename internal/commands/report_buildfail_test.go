package commands

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The streams in testdata/buildfail are the unedited stdout of `go test -json` (go1.27.4) for a module
// with a package whose test file does not compile (brokentest), a package that does not compile and is
// imported by two others (dep, usea, useb), a package that passes (ok) and one that fails `go vet`
// (vetfail):
//
//	mixed.jsonl       go test -json ./ok/... ./brokentest/... ./usea/... ./useb/...
//	brokentest.jsonl  go test -json ./brokentest/...
//	vet.jsonl         go test -json ./vetfail/...
func buildFailFixture(name string) string {
	return filepath.Join("testdata", "buildfail", name)
}

// suiteErrorLogs returns the ERROR logs of the suite, as "LEVEL message". (With --reportEmptyPkg the
// suite also has the "FAIL <package> [build failed]" line of the test output as an INFO log; without
// it that line is a log of the launch, since the suite does not exist yet when it arrives.)
func (rec *hierarchyServer) suiteErrorLogs(t *testing.T, suite string) []string {
	t.Helper()
	id := rec.item(t, suite).ID
	rec.mu.Lock()
	defer rec.mu.Unlock()
	var logs []string
	for _, l := range rec.logs {
		if l["itemUuid"] == id && l["level"] == "ERROR" {
			logs = append(logs, l["level"].(string)+" "+l["message"].(string))
		}
	}
	return logs
}

func TestReportLaunch_BuildFailures(t *testing.T) {
	t.Parallel()

	const (
		pkg            = "example.com/buildfail/"
		brokenTestLogs = "ERROR # example.com/buildfail/brokentest [example.com/buildfail/brokentest.test]\n" +
			"brokentest/add_test.go:6:18: undefined: three\n" +
			"brokentest/add_test.go:9:6: declared and not used: unused\n"
		depLogs = "ERROR # example.com/buildfail/dep\ndep/dep.go:3:27: undefined: undefinedThing\n"
	)
	tests := []struct {
		fixture     string
		reportEmpty bool
		// wantFailed are the packages that failed to build, with the log of their suite
		wantFailed map[string]string
		wantItems  int
	}{
		{
			fixture: "mixed.jsonl",
			wantFailed: map[string]string{
				pkg + "brokentest": brokenTestLogs,
				// one broken dependency fails every package that imports it
				pkg + "usea": depLogs,
				pkg + "useb": depLogs,
			},
			wantItems: 5, // three failed suites, the suite of the passing package and its test
		},
		{
			fixture:     "mixed.jsonl",
			reportEmpty: true, // the suites exist already when the packages fail
			wantFailed: map[string]string{
				pkg + "brokentest": brokenTestLogs,
				pkg + "usea":       depLogs,
				pkg + "useb":       depLogs,
			},
			wantItems: 5,
		},
		{
			// the build events are the first lines of the stream and have no time
			fixture:    "brokentest.jsonl",
			wantFailed: map[string]string{pkg + "brokentest": brokenTestLogs},
			wantItems:  1,
		},
		{
			fixture: "vet.jsonl",
			wantFailed: map[string]string{
				pkg + "vetfail": "ERROR # example.com/buildfail/vetfail\n# [example.com/buildfail/vetfail]\n" +
					"vetfail/v_test.go:6:10: (*testing.common).Logf format %d has arg \"not a number\" " +
					"of wrong type string\n",
			},
			wantItems: 1,
		},
	}
	for _, tc := range tests {
		t.Run(tc.fixture, func(t *testing.T) {
			t.Parallel()

			rec, srv := newHierarchyServer(t, "proj")
			cfg := &clientConfig{URL: srv.URL, Project: "proj", ApiKey: "tok"}

			_, _, err := reportLaunch(
				t.Context(), cfg, reportCmd(buildFailFixture(tc.fixture), "launch", tc.reportEmpty),
			)
			require.NoError(t, err)

			for suite, wantLog := range tc.wantFailed {
				item := rec.item(t, suite)
				assert.Equal(t, "SUITE", item.Body["type"])
				assert.Empty(t, item.ParentID)
				assert.Equal(t, "FAILED", rec.finishes[item.ID]["status"], suite)
				assert.Equal(t, []string{wantLog}, rec.suiteErrorLogs(t, suite), suite)
			}
			assert.Len(t, rec.items, tc.wantItems)
			assert.Equal(t, "FAILED", rec.launchFinish["status"])

			// build events have no time: they must not become the start or the end of the launch
			for name, body := range map[string]map[string]any{
				"launch start": rec.launchStart, "launch finish": rec.launchFinish,
			} {
				field := "startTime"
				if name == "launch finish" {
					field = "endTime"
				}
				at, pErr := time.Parse(time.RFC3339Nano, body[field].(string))
				require.NoError(t, pErr)
				assert.Equal(t, 2026, at.Year(), "%s time must come from a test event", name)
			}
		})
	}
}

// Before Go 1.24 the build output went to stderr: the stream only has the failed package.
func TestReportLaunch_BuildFailureWithoutBuildOutput(t *testing.T) {
	t.Parallel()

	rec, srv := newHierarchyServer(t, "proj")
	file := writeEventLines(
		t,
		`{"Time":"2025-01-01T00:00:00Z","Action":"start","Package":"example/pkg"}`,
		`{"Time":"2025-01-01T00:00:00Z","Action":"output","Package":"example/pkg","Output":"FAIL\texample/pkg [build failed]\n"}`,
		`{"Time":"2025-01-01T00:00:00Z","Action":"fail","Package":"example/pkg","Elapsed":0,"FailedBuild":"example/pkg [example/pkg.test]"}`,
	)
	cfg := &clientConfig{URL: srv.URL, Project: "proj", ApiKey: "tok"}

	_, _, err := reportLaunch(t.Context(), cfg, reportCmd(file, "launch", false))
	require.NoError(t, err)

	suite := rec.item(t, "example/pkg")
	assert.Equal(t, "FAILED", rec.finishes[suite.ID]["status"])
	assert.Empty(t, rec.suiteErrorLogs(t, "example/pkg"))
}
