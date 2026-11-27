//go:build e2e

package e2e

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/reportportal/goRP/v5/pkg/openapi"
)

// The fixtures are real streams, see internal/commands/report_retries_test.go: in the package "flaky"
// TestFlaky and TestTable/flaky fail on their first run and pass on the second.
func retriesFixture(name string) string {
	return filepath.Join("..", "internal", "commands", "testdata", "retries", name)
}

func withExecutions(l *openapi.LaunchViewModel) bool {
	return finished(l) && l.GetStatistics().Executions != nil
}

func retryStatuses(item *testItem) []string {
	statuses := []string{}
	for _, r := range item.Retries {
		statuses = append(statuses, r.Status)
	}

	return statuses
}

// `go test -count=2`: the second run of a test is a retry of the first, so ReportPortal counts every
// test case once, with the result of its last run.
func TestReportTest2JSON_CountRunsAreRetries(t *testing.T) {
	e := newEnv(t)
	name := uniqueName(t)

	runGorp(
		t,
		"report",
		"test2json",
		"--file",
		retriesFixture("count2.jsonl"),
		"--launchName",
		name,
	).
		requireOK(t)

	launch := e.findLaunch(t, name, withExecutions)
	executions := *launch.GetStatistics().Executions
	assert.Equal(
		t,
		int32(6),
		executions["total"],
		"six test cases ran twice each: TestFlaky, TestStable, TestTable/ok, TestTable/flaky, TestParallel, TestFine",
	)
	assert.Equal(t, int32(6), executions["passed"], "the last run of every test passed")
	assert.Zero(t, executions["failed"])
	assert.True(t, launch.GetHasRetries())
	assert.Equal(t, "FAILED", launch.GetStatus(),
		"go test fails a package when one of the runs failed, and so does the launch")

	// two suites, TestFlaky, TestStable, TestTable with two subtests, TestParallel, TestFine;
	// the first runs are not items of their own anymore
	items := e.items(t, launch.Id, 9)
	assert.Equal(t, "PASSED", items["TestFlaky"].Status)
	assert.Equal(t, []string{"FAILED"}, retryStatuses(items["TestFlaky"]))
	assert.Equal(t, []string{"PASSED"}, retryStatuses(items["TestStable"]))
	assert.Equal(t, []string{"FAILED"}, retryStatuses(items["flaky"]))
	assert.Equal(t, []string{"PASSED"}, retryStatuses(items["ok"]))
	assert.Equal(t, []string{"PASSED"}, retryStatuses(items["TestFine"]))

	// a test with subtests is reused, not retried: its subtests are the retries
	table := items["TestTable"]
	assert.Empty(t, table.Retries)
	assert.Equal(t, "PASSED", table.Status, "finished again with the status of its last run")
	assert.Equal(t, table.ID, items["flaky"].Parent)
	assert.Equal(t, table.ID, items["ok"].Parent)
	assert.Equal(t, "FAILED", items["example.com/retries/flaky"].Status)
}

// `gotestsum --rerun-fails` runs the package again for the failed tests only. The reruns are retries,
// the tests that did not run again stay counted, and a run that gotestsum made pass is a passed launch.
func TestReportTest2JSON_GotestsumRerunsAreRetries(t *testing.T) {
	e := newEnv(t)
	name := uniqueName(t)

	runGorp(
		t,
		"report",
		"test2json",
		"--file",
		retriesFixture("gotestsum.jsonl"),
		"--launchName",
		name,
	).
		requireOK(t)

	launch := e.findLaunch(t, name, withExecutions)
	executions := *launch.GetStatistics().Executions
	assert.Equal(t, int32(6), executions["total"])
	assert.Equal(t, int32(6), executions["passed"])
	assert.Zero(t, executions["failed"])
	assert.True(t, launch.GetHasRetries())
	assert.Equal(t, "PASSED", launch.GetStatus())

	items := e.items(t, launch.Id, 9)
	assert.Equal(t, []string{"FAILED"}, retryStatuses(items["TestFlaky"]))
	assert.Equal(t, []string{"FAILED"}, retryStatuses(items["flaky"]))
	for _, once := range []string{"TestStable", "TestParallel", "ok", "TestFine"} {
		require.NotNil(t, items[once], once)
		assert.Empty(t, items[once].Retries, once)
		assert.Equal(t, "PASSED", items[once].Status, once)
	}
	assert.Equal(t, "PASSED", items["TestTable"].Status)
	assert.Equal(t, "PASSED", items["example.com/retries/flaky"].Status,
		"the suite is finished again by every run of the package")
}

// --retries=false keeps every run as a test item of its own.
func TestReportTest2JSON_RetriesDisabled(t *testing.T) {
	e := newEnv(t)
	name := uniqueName(t)

	runGorp(
		t,
		"report",
		"test2json",
		"--file",
		retriesFixture("count2.jsonl"),
		"--launchName",
		name,
		"--retries=false",
	).requireOK(t)

	launch := e.findLaunch(t, name, withExecutions)
	executions := *launch.GetStatistics().Executions
	assert.Equal(t, int32(12), executions["total"], "six test cases, two runs each")
	assert.Equal(t, int32(10), executions["passed"])
	assert.Equal(t, int32(2), executions["failed"])
	assert.False(t, launch.GetHasRetries())
	assert.Equal(t, "FAILED", launch.GetStatus())
}
