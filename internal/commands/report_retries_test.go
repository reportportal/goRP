package commands

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v3"
)

// The streams in testdata/retries are unedited output of go1.27.4 and gotestsum v1.12.3 for a module
// with the packages "flaky" (TestFlaky and TestTable/flaky fail on their first run only, TestStable,
// TestTable/ok and TestParallel pass) and "stable" (TestFine passes):
//
//	count2.jsonl     go test -json -count=2 ./...
//	count3.jsonl     go test -json -count=3 ./stable/
//	gotestsum.jsonl  gotestsum --rerun-fails --packages ./... --jsonfile gotestsum.jsonl -- -count=1
//
// gotestsum runs the package again for every failed top-level test (-run '^TestTable$/^flaky$'), so its
// stream has three runs of the package "flaky", and the rerun of TestTable has only the subtest "flaky".
const (
	retriesFlaky  = "example.com/retries/flaky"
	retriesStable = "example.com/retries/stable"
)

func retriesCmd(t *testing.T, fixture string, retries, reportEmpty bool) *cli.Command {
	t.Helper()
	cmd := reportCmd(filepath.Join("testdata", "retries", fixture), "launch", reportEmpty)
	if !retries {
		require.NoError(t, cmd.Set("retries", "false"))
	}
	return cmd
}

// runs returns the recorded starts of the items with the given codeRef (package-qualified test name),
// or of the suite with that name, in the order they were started.
func (rec *hierarchyServer) runs(name string) []recordedItem {
	rec.mu.Lock()
	defer rec.mu.Unlock()
	var runs []recordedItem
	for _, it := range rec.items {
		if it.Body["codeRef"] == name || (it.Body["type"] == "SUITE" && it.Body["name"] == name) {
			runs = append(runs, it)
		}
	}
	return runs
}

// requireRetryChain checks that the test was started n times and that every run after the first is a
// retry of the run right before it.
func requireRetryChain(t *testing.T, rec *hierarchyServer, test string, n int) []recordedItem {
	t.Helper()
	runs := rec.runs(test)
	require.Len(t, runs, n, test)
	assert.Equal(t, false, runs[0].Body["retry"], "%s: the first run is not a retry", test)
	assert.NotContains(t, runs[0].Body, "retryOf", test)
	for i := 1; i < n; i++ {
		assert.Equal(t, true, runs[i].Body["retry"], "%s run %d", test, i+1)
		assert.Equal(t, runs[i-1].ID, runs[i].Body["retryOf"],
			"%s run %d must be a retry of the latest run, not of the first", test, i+1)
		assert.Equal(
			t,
			runs[0].ParentID,
			runs[i].ParentID,
			"%s: all runs have the same parent",
			test,
		)
		assert.Equal(t, runs[0].Body["testCaseId"], runs[i].Body["testCaseId"], test)
	}
	return runs
}

func reportRetries(
	t *testing.T,
	fixture string,
	retries, reportEmpty bool,
) *hierarchyServer {
	t.Helper()
	rec, srv := newHierarchyServer(t, "proj")
	cfg := &clientConfig{URL: srv.URL, Project: "proj", ApiKey: "tok"}
	_, _, err := reportLaunch(t.Context(), cfg, retriesCmd(t, fixture, retries, reportEmpty))
	require.NoError(t, err)
	return rec
}

func TestReportLaunch_CountRunsAreRetries(t *testing.T) {
	t.Parallel()

	for _, reportEmpty := range []bool{false, true} {
		rec := reportRetries(t, "count2.jsonl", true, reportEmpty)

		for _, test := range []string{"/TestFlaky", "/TestStable", "/TestParallel"} {
			requireRetryChain(t, rec, retriesFlaky+test, 2)
		}
		requireRetryChain(t, rec, retriesStable+"/TestFine", 2)

		// a test with subtests is reused: its subtests are the retries
		table := rec.runs(retriesFlaky + "/TestTable")
		require.Len(t, table, 1, "a test with subtests is not started again")
		assert.Equal(t, []string{"FAILED", "PASSED"}, rec.finishStatuses[table[0].ID])
		for _, sub := range []string{"/TestTable/ok", "/TestTable/flaky"} {
			runs := requireRetryChain(t, rec, retriesFlaky+sub, 2)
			assert.Equal(t, table[0].ID, runs[1].ParentID)
		}

		suite := rec.runs(retriesFlaky)
		require.Len(t, suite, 1)
		assert.Equal(t, []string{"FAILED"}, rec.finishStatuses[suite[0].ID],
			"go test fails the package when one of the runs failed")
		assert.Equal(t, "FAILED", rec.launchFinish["status"])
	}
}

func TestReportLaunch_ThirdRunIsRetryOfSecond(t *testing.T) {
	t.Parallel()

	rec := reportRetries(t, "count3.jsonl", true, false)

	requireRetryChain(t, rec, retriesStable+"/TestFine", 3)
	assert.Equal(t, "PASSED", rec.launchFinish["status"])
}

func TestReportLaunch_GotestsumRerunsAreRetries(t *testing.T) {
	t.Parallel()

	for _, reportEmpty := range []bool{false, true} {
		rec := reportRetries(t, "gotestsum.jsonl", true, reportEmpty)

		requireRetryChain(t, rec, retriesFlaky+"/TestFlaky", 2)
		requireRetryChain(t, rec, retriesFlaky+"/TestTable/flaky", 2)
		// what did not run again is reported once and stays as it is
		for _, test := range []string{"/TestStable", "/TestParallel", "/TestTable/ok"} {
			requireRetryChain(t, rec, retriesFlaky+test, 1)
		}

		table := rec.runs(retriesFlaky + "/TestTable")
		require.Len(t, table, 1)
		assert.Equal(t, []string{"FAILED", "PASSED"}, rec.finishStatuses[table[0].ID])

		// the package ran three times: the first run failed, the rerun of TestFlaky passed while
		// TestTable was still failed, the rerun of TestTable passed
		suite := rec.runs(retriesFlaky)
		require.Len(t, suite, 1, "the suite of a package that runs again is reused")
		assert.Equal(t, []string{"FAILED", "FAILED", "PASSED"}, rec.finishStatuses[suite[0].ID])

		assert.Equal(t, "PASSED", rec.launchFinish["status"],
			"every failure was turned into a success by a rerun, as for gotestsum itself")
	}
}

func TestReportLaunch_RetriesDisabled(t *testing.T) {
	t.Parallel()

	rec := reportRetries(t, "count2.jsonl", false, false)
	for _, test := range []string{"/TestFlaky", "/TestTable", "/TestTable/flaky"} {
		runs := rec.runs(retriesFlaky + test)
		require.Len(t, runs, 2, test)
		for _, run := range runs {
			assert.Equal(t, false, run.Body["retry"], test)
			assert.NotContains(t, run.Body, "retryOf", test)
		}
	}

	rec = reportRetries(t, "gotestsum.jsonl", false, false)
	assert.Equal(t, "FAILED", rec.launchFinish["status"],
		"without retries the failed runs are counted items, so the launch is failed")
}

// An import path can be the prefix of another one. The failed test of a nested package must not fail
// the suite of the package above it.
func TestReportLaunch_NestedPackageDoesNotFailItsParentPackage(t *testing.T) {
	t.Parallel()

	line := func(action, pkg, test string) string {
		if test == "" {
			return fmt.Sprintf(
				`{"Time":"2025-01-01T00:00:00Z","Action":%q,"Package":%q}`,
				action,
				pkg,
			)
		}
		return fmt.Sprintf(
			`{"Time":"2025-01-01T00:00:00Z","Action":%q,"Package":%q,"Test":%q}`,
			action,
			pkg,
			test,
		)
	}
	rec, srv := newHierarchyServer(t, "proj")
	file := writeEventLines(
		t,
		line(
			"run",
			"example/a/b",
			"TestY",
		),
		line("fail", "example/a/b", "TestY"),
		line("fail", "example/a/b", ""),
		line(
			"run",
			"example/a",
			"TestX",
		),
		line("pass", "example/a", "TestX"),
		line("pass", "example/a", ""),
		// a parent test is not failed by a test whose name merely starts like it either
		line("run", "example/c", "TestZ"),
		line("run", "example/c", "TestZ/sub"),
		line("pass", "example/c", "TestZ/sub"),
		line("pass", "example/c", "TestZ"),
		line("run", "example/c", "TestZoo"),
		line("fail", "example/c", "TestZoo"),
		line("run", "example/c", "TestZ"),
		line("pass", "example/c", "TestZ"),
		line("fail", "example/c", ""),
	)
	cfg := &clientConfig{URL: srv.URL, Project: "proj", ApiKey: "tok"}
	_, _, err := reportLaunch(t.Context(), cfg, reportCmd(file, "launch", false))
	require.NoError(t, err)

	assert.Equal(t, []string{"PASSED"}, rec.finishStatuses[rec.item(t, "example/a").ID],
		"example/a/b/TestY is not a test of example/a")
	assert.Equal(t, []string{"FAILED"}, rec.finishStatuses[rec.item(t, "example/a/b").ID])
	assert.Equal(t, []string{"PASSED", "PASSED"}, rec.finishStatuses[rec.item(t, "TestZ").ID],
		"TestZoo is not a subtest of TestZ")
	assert.Equal(t, "FAILED", rec.launchFinish["status"])
}
