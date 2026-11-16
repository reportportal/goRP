//go:build e2e

package e2e

import (
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func reportTinyLaunch(t *testing.T, name string) {
	t.Helper()

	input := writeFile(t, "events.jsonl", strings.Join([]string{
		`{"Time":"2026-01-01T10:00:00Z","Action":"run","Package":"example.com/e2e","Test":"TestOne"}`,
		`{"Time":"2026-01-01T10:00:01Z","Action":"pass","Package":"example.com/e2e","Test":"TestOne"}`,
		`{"Time":"2026-01-01T10:00:01Z","Action":"pass","Package":"example.com/e2e"}`,
	}, "\n")+"\n")
	runGorp(t, "report", "test2json", "--file", input, "--launchName", name).requireOK(t)
}

func TestLaunchListAndMerge(t *testing.T) {
	e := newEnv(t)
	first, second := uniqueName(t)+"-a", uniqueName(t)+"-b"
	reportTinyLaunch(t, first)
	reportTinyLaunch(t, second)
	l1 := e.findLaunch(t, first, finished)
	l2 := e.findLaunch(t, second, finished)

	list := runGorp(t, "launch", "list", "--filter", "filter.eq.name="+first).requireOK(t)
	assert.Contains(t, list.stdout, first)
	assert.NotContains(t, list.stdout, second)

	merged := uniqueName(t) + "-merged"
	res := runGorp(t, "launch", "merge", "--name", merged, "--type", "BASIC",
		"--ids", strconv.FormatInt(l1.Id, 10), "--ids", strconv.FormatInt(l2.Id, 10)).requireOK(t)
	require.NotEmpty(t, res.firstLine(), "merge prints the id of the new launch")

	mergedLaunch := e.findLaunch(t, merged, finished)
	assert.Equal(t, strconv.FormatInt(mergedLaunch.Id, 10), res.firstLine())
}

func TestLaunchMerge_RejectsAmbiguousInput(t *testing.T) {
	newEnv(t)

	res := runGorp(
		t,
		"launch",
		"merge",
		"--name",
		"x",
		"--ids",
		"1",
		"--filter",
		"filter.eq.name=y",
	)

	assert.NotZero(t, res.exitCode, res.String())
	assert.Contains(t, res.stderr+res.stdout, "cannot be set along with")
}

func TestQualityGate_UnknownLaunchFailsFast(t *testing.T) {
	newEnv(t)

	start := time.Now()
	res := runGorp(
		t,
		"quality-gate",
		"check",
		"--launch-uuid",
		"00000000-0000-4000-8000-000000000000",
		"--quality-gate-timeout",
		"2m",
	)

	assert.Equal(t, 10, res.exitCode, res.String())
	assert.Less(
		t,
		time.Since(start),
		30*time.Second,
		"a missing launch will never resolve, it must not be polled until timeout",
	)
}

func TestQualityGate_PipelineWithPrintedUUID(t *testing.T) {
	e := newEnv(t)
	name := uniqueName(t)
	reportTinyLaunch(t, name)
	e.findLaunch(t, name, finished)

	// documented pipeline: test2json --print-launch-uuid | quality-gate check --stdin
	// The project has no quality gate configured (or it is not resolved), so only the plumbing is
	// asserted: the UUID is found and a real check is made, which ends with exit 0 or 10, never 1.
	input := writeFile(
		t,
		"events.jsonl",
		`{"Time":"2026-01-01T10:00:00Z","Action":"run","Package":"p","Test":"T"}`+"\n"+
			`{"Time":"2026-01-01T10:00:01Z","Action":"pass","Package":"p","Test":"T"}`+"\n"+
			`{"Time":"2026-01-01T10:00:01Z","Action":"pass","Package":"p"}`+"\n",
	)
	pipeline := uniqueName(t) + "-pipeline"
	printed := runGorp(t, "report", "test2json", "--file", input, "--launchName", pipeline,
		"--print-launch-uuid").requireOK(t)
	e.findLaunch(t, pipeline, finished)

	cmd, stdout, stderr := command(
		t.Context(),
		"quality-gate",
		"check",
		"--stdin",
		"--quality-gate-timeout",
		"10s",
		"--quality-gate-check-interval",
		"2s",
	)
	cmd.Stdin = strings.NewReader(printed.stdout)
	err := cmd.Run()

	assert.Contains(t, []int{0, 10}, exitCode(err), "stdout: %s\nstderr: %s", stdout, stderr)
}
