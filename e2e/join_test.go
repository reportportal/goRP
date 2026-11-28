//go:build e2e

package e2e

import (
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/reportportal/goRP/v5/pkg/openapi"
)

// shardEvents is the test2json stream of one package with one test. The events are stamped from
// start on: ReportPortal silently drops items that are older than their launch.
func shardEvents(start time.Time, pkg, test, action string) string {
	at := func(d time.Duration) string { return start.Add(d).UTC().Format(time.RFC3339Nano) }

	return strings.Join([]string{
		fmt.Sprintf(`{"Time":%q,"Action":"run","Package":%q,"Test":%q}`, at(0), pkg, test),
		fmt.Sprintf(
			`{"Time":%q,"Action":"output","Package":%q,"Test":%q,"Output":"output of %s\n"}`,
			at(time.Millisecond),
			pkg,
			test,
			test,
		),
		fmt.Sprintf(
			`{"Time":%q,"Action":%q,"Package":%q,"Test":%q}`,
			at(2*time.Millisecond),
			action,
			pkg,
			test,
		),
		fmt.Sprintf(`{"Time":%q,"Action":%q,"Package":%q}`, at(3*time.Millisecond), action, pkg),
	}, "\n") + "\n"
}

// Sharded CI: one job starts the launch, every shard reports its part with --launch-uuid, the last job
// finishes the launch. No shard may start or finish the launch.
func TestReportTest2JSON_TwoShardsIntoOneLaunch(t *testing.T) {
	e := newEnv(t)
	name := uniqueName(t)

	launchUUID := runGorp(t, "report", "start-launch", "--name", name).requireOK(t).firstLine()
	require.NotEmpty(t, launchUUID)
	running := e.findLaunch(t, name, nil)

	start := time.Now()
	shards := []string{
		writeFile(
			t,
			"shard1.jsonl",
			shardEvents(start, "example.com/shard1", "TestInShard1", "pass"),
		),
		writeFile(
			t,
			"shard2.jsonl",
			shardEvents(start, "example.com/shard2", "TestInShard2", "fail"),
		),
	}
	results := make([]result, len(shards))
	var wg sync.WaitGroup
	for i, file := range shards {
		wg.Go(func() {
			results[i] = runGorp(t, "report", "test2json", "--file", file,
				"--launch-uuid", launchUUID, "--print-launch-uuid")
		})
	}
	wg.Wait()
	for _, res := range results {
		res.requireOK(t)
		assert.Equal(t, "ReportPortal Launch UUID: "+launchUUID, strings.TrimSpace(res.stdout),
			"the joined launch is printed; there is no link before the launch is finished")
	}

	items := e.finishedItems(t, running.Id, 4) // two suites, two tests: all in the one launch
	assert.Equal(t, "PASSED", items["TestInShard1"].Status)
	assert.Equal(t, "FAILED", items["TestInShard2"].Status)
	assert.Equal(t, items["example.com/shard1"].ID, items["TestInShard1"].Parent)
	assert.Equal(t, items["example.com/shard2"].ID, items["TestInShard2"].Parent)

	page, err := e.client.GetLaunchesByFilterString(
		t.Context(),
		e.cfg.Project,
		"filter.eq.name="+name,
	)
	require.NoError(t, err)
	require.Len(t, page.Content, 1, "the shards must not start launches of their own")
	assert.Equal(
		t,
		"IN_PROGRESS",
		page.Content[0].GetStatus(),
		"the shards must not finish the launch",
	)

	runGorp(t, "report", "finish-launch", "--launch-uuid", launchUUID).requireOK(t)

	launch := e.findLaunch(t, name, func(l *openapi.LaunchViewModel) bool {
		return finished(l) && l.GetStatistics().Executions != nil
	})
	assert.Equal(t, running.Id, launch.Id)
	assert.Equal(
		t,
		"FAILED",
		launch.GetStatus(),
		"the server derives the status from the items of both shards",
	)
	executions := *launch.GetStatistics().Executions
	assert.Equal(t, int32(2), executions["total"])
	assert.Equal(t, int32(1), executions["passed"])
	assert.Equal(t, int32(1), executions["failed"])
}

// The reporting API accepts items for an unknown launch and drops them later, so the UUID is checked
// before anything is reported.
func TestReportTest2JSON_UnknownLaunchUUID(t *testing.T) {
	newEnv(t)
	input := writeFile(t, "events.jsonl",
		shardEvents(time.Now(), "example.com/e2e", "TestNeverReported", "pass"))

	res := runGorp(t, "report", "test2json", "--file", input,
		"--launch-uuid", "00000000-0000-0000-0000-000000000000")

	assert.NotZero(t, res.exitCode, res.String())
	assert.Contains(t, res.stderr, "not found")
}
