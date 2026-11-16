//go:build e2e

package e2e

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// test2json stream of a package with a passing and a failing test (Go 1.27 OutputType annotations).
func test2jsonFixture() string {
	return strings.Join([]string{
		`{"Time":"2026-01-01T10:00:00Z","Action":"start","Package":"example.com/e2e"}`,
		`{"Time":"2026-01-01T10:00:01Z","Action":"run","Package":"example.com/e2e","Test":"TestPass"}`,
		`{"Time":"2026-01-01T10:00:01.1Z","Action":"output","Package":"example.com/e2e","Test":"TestPass","Output":"=== RUN   TestPass\n"}`,
		`{"Time":"2026-01-01T10:00:02Z","Action":"pass","Package":"example.com/e2e","Test":"TestPass","Elapsed":0.01}`,
		`{"Time":"2026-01-01T10:00:02Z","Action":"run","Package":"example.com/e2e","Test":"TestFail"}`,
		`{"Time":"2026-01-01T10:00:02.1Z","Action":"output","Package":"example.com/e2e","Test":"TestFail","Output":"    e2e_test.go:10: expected 1 got 2\n","OutputType":"error"}`,
		`{"Time":"2026-01-01T10:00:02.2Z","Action":"output","Package":"example.com/e2e","Test":"TestFail","Output":"        continuation line\n","OutputType":"error-continue"}`,
		`{"Time":"2026-01-01T10:00:03Z","Action":"fail","Package":"example.com/e2e","Test":"TestFail","Elapsed":0.02}`,
		`{"Time":"2026-01-01T10:00:03Z","Action":"fail","Package":"example.com/e2e","Elapsed":0.05}`,
	}, "\n") + "\n"
}

func TestReportTest2JSON(t *testing.T) {
	e := newEnv(t)
	name := uniqueName(t)
	input := writeFile(t, "events.jsonl", test2jsonFixture())

	res := runGorp(t, "report", "test2json", "--file", input, "--launchName", name,
		"--attr", "e2e:gorp", "--attr", "smoke", "--print-launch-uuid").requireOK(t)

	// the printed line must be what `quality-gate check --stdin` understands
	assert.Regexp(t, `ReportPortal Launch UUID: [0-9a-f-]{36}`, res.stdout)

	launch := e.findLaunch(t, name, finished)
	assert.Equal(t, "FAILED", launch.GetStatus(), "a failed test must fail the launch")
	var attrs []string
	for _, a := range launch.Attributes {
		attrs = append(attrs, a.GetKey()+":"+a.Value)
	}
	assert.ElementsMatch(t, []string{"e2e:gorp", ":smoke"}, attrs)

	items := e.items(t, launch.Id, 3) // the package suite and two tests
	assert.Equal(t, "SUITE", items["example.com/e2e"].Type)
	assert.Equal(t, "PASSED", items["TestPass"].Status)
	assert.Equal(t, "FAILED", items["TestFail"].Status)
	assert.Equal(
		t,
		items["example.com/e2e"].ID,
		items["TestFail"].Parent,
		"tests belong to the package suite",
	)

	logs := e.logs(t, launch.Id, 2)
	var errorLog *logEntry
	for i := range logs {
		if strings.EqualFold(logs[i].Level, "ERROR") { // the server lower-cases levels
			errorLog = &logs[i]
		}
	}
	require.NotNil(t, errorLog, "the error output must be reported with the ERROR level: %+v", logs)
	assert.Contains(t, errorLog.Message, "expected 1 got 2")
	assert.Contains(
		t,
		errorLog.Message,
		"continuation line",
		"error-continue output is merged into the log entry",
	)
	assert.Equal(t, items["TestFail"].ID, errorLog.ItemID)
}

func TestReportTest2JSON_PassingRunPassesLaunch(t *testing.T) {
	e := newEnv(t)
	name := uniqueName(t)
	input := writeFile(t, "events.jsonl", strings.Join([]string{
		`{"Time":"2026-01-01T10:00:00Z","Action":"run","Package":"example.com/e2e","Test":"TestOK"}`,
		`{"Time":"2026-01-01T10:00:01Z","Action":"pass","Package":"example.com/e2e","Test":"TestOK"}`,
		`{"Time":"2026-01-01T10:00:01Z","Action":"pass","Package":"example.com/e2e"}`,
	}, "\n")+"\n")

	runGorp(t, "report", "test2json", "--file", input, "--launchName", name).requireOK(t)

	launch := e.findLaunch(t, name, finished)
	assert.Equal(t, "PASSED", launch.GetStatus())
}

// The atomic commands are what shell-based integrations use: each prints the ID it created.
func TestAtomicSteps(t *testing.T) {
	e := newEnv(t)
	name := uniqueName(t)
	attachment := writeFile(t, "report.txt", "attached by the e2e test\n")

	launchUUID := runGorp(
		t,
		"report",
		"start-launch",
		"--name",
		name,
		"--attr",
		"e2e:steps",
	).requireOK(t).
		firstLine()
	require.NotEmpty(t, launchUUID)

	suite := runGorp(t, "report", "start-test", "--launch-uuid", launchUUID,
		"--name", "the suite", "--type", "SUITE").requireOK(t).firstLine()
	require.NotEmpty(t, suite)

	step := runGorp(
		t,
		"report",
		"start-test",
		"--launch-uuid",
		launchUUID,
		"--parent-uuid",
		suite,
		"--name",
		"the step",
		"--type",
		"STEP",
		"--code-ref",
		"e2e/steps",
		"--attr",
		"kind:atomic",
	).requireOK(t).firstLine()
	require.NotEmpty(t, step)

	runGorp(t, "report", "log", "--launch-uuid", launchUUID, "--item-uuid", step,
		"--message", "plain message", "--level", "INFO").requireOK(t)
	runGorp(t, "report", "log", "--launch-uuid", launchUUID, "--item-uuid", step,
		"--message", "with attachment", "--level", "ERROR", "--file", attachment).requireOK(t)

	runGorp(
		t,
		"report",
		"finish-test",
		"--launch-uuid",
		launchUUID,
		"--item-uuid",
		step,
		"--status",
		"FAILED",
	).requireOK(t)
	runGorp(
		t,
		"report",
		"finish-test",
		"--launch-uuid",
		launchUUID,
		"--item-uuid",
		suite,
		"--status",
		"FAILED",
	).requireOK(t)
	runGorp(
		t,
		"report",
		"finish-launch",
		"--launch-uuid",
		launchUUID,
		"--status",
		"FAILED",
	).requireOK(t)

	launch := e.findLaunch(t, name, finished)
	assert.Equal(t, "FAILED", launch.GetStatus())

	items := e.items(t, launch.Id, 2)
	assert.Equal(t, "STEP", items["the step"].Type)
	assert.Equal(t, "FAILED", items["the step"].Status)
	assert.Equal(t, items["the suite"].ID, items["the step"].Parent)
	assert.Equal(t, "e2e/steps", items["the step"].CodeRef)

	logs := e.logs(t, launch.Id, 2)
	byMessage := map[string]*logEntry{}
	for i := range logs {
		byMessage[logs[i].Message] = &logs[i]
	}
	require.Contains(t, byMessage, "plain message")
	require.Contains(t, byMessage, "with attachment")
	assert.True(t, strings.EqualFold("ERROR", byMessage["with attachment"].Level))
	assert.NotNil(
		t,
		byMessage["with attachment"].BinaryContent,
		"the file must be attached to the log",
	)
	assert.False(t, byMessage["plain message"].hasAttachment())
	assert.Equal(t, items["the step"].ID, byMessage["plain message"].ItemID)
}
