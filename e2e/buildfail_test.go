//go:build e2e

package e2e

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A package that does not compile runs no test, so its failure exists only as build output: the suite
// must be reported as FAILED with that output. The stream is real `go test -json` output, whose build
// events come first and have no time.
func TestReportTest2JSON_BuildFailure(t *testing.T) {
	e := newEnv(t)
	name := uniqueName(t)
	input := filepath.Join(
		"..",
		"internal",
		"commands",
		"testdata",
		"buildfail",
		"brokentest.jsonl",
	)

	runGorp(t, "report", "test2json", "--file", input, "--launchName", name).requireOK(t)

	launch := e.findLaunch(t, name, finished)
	assert.Equal(t, "FAILED", launch.GetStatus())
	assert.Equal(
		t,
		2026,
		launch.StartTime.Year(),
		"build events have no time and must not start the launch",
	)

	items := e.items(t, launch.Id, 1)
	suite := items["example.com/buildfail/brokentest"]
	require.NotNil(t, suite)
	assert.Equal(t, "SUITE", suite.Type)
	assert.Equal(t, "FAILED", suite.Status)

	var buildLog *logEntry
	logs := e.logs(t, launch.Id, 1)
	for i := range logs {
		if logs[i].ItemID == suite.ID && strings.EqualFold(logs[i].Level, "ERROR") {
			buildLog = &logs[i]
		}
	}
	require.NotNil(t, buildLog, "the build output must be an ERROR log of the suite: %+v", logs)
	assert.Contains(t, buildLog.Message, "brokentest/add_test.go:6:18: undefined: three")
	assert.Contains(t, buildLog.Message, "declared and not used: unused")
}
