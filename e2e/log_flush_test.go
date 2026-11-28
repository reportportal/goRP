//go:build e2e

package e2e

import (
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The output of a test that is still running must reach ReportPortal while it runs: one line is far
// from a full batch, so only the flush timer can send it.
func TestReportTest2JSON_LogsOfRunningTestAreSent(t *testing.T) {
	e := newEnv(t)
	name := uniqueName(t)

	cmd, stdout, stderr := command(t.Context(), "report", "test2json", "--launchName", name)
	stdin, err := cmd.StdinPipe()
	require.NoError(t, err)
	require.NoError(t, cmd.Start())
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	t.Cleanup(func() { _ = cmd.Process.Kill() })

	// a test starts and prints one line; then nothing happens, as in a long-running test
	_, err = io.WriteString(
		stdin,
		`{"Time":"2026-01-01T10:00:00Z","Action":"run","Package":"example.com/e2e","Test":"TestSlow"}`+"\n"+
			`{"Time":"2026-01-01T10:00:01Z","Action":"output","Package":"example.com/e2e","Test":"TestSlow","Output":"still working\n"}`+"\n",
	)
	require.NoError(t, err)

	running := e.findLaunch(t, name, nil)
	logs := e.logs(t, running.Id, 1)
	assert.Equal(t, "still working\n", logs[0].Message)
	select {
	case waitErr := <-done:
		t.Fatalf(
			"gorp exited while its input is still open: %v\nstdout: %s\nstderr: %s",
			waitErr,
			stdout,
			stderr,
		)
	default:
	}
	page, err := e.client.GetLaunchesByFilterString(
		t.Context(),
		e.cfg.Project,
		"filter.eq.name="+name,
	)
	require.NoError(t, err)
	require.Len(t, page.Content, 1)
	assert.Equal(
		t,
		"IN_PROGRESS",
		page.Content[0].GetStatus(),
		"the log arrived while the launch is running",
	)

	// the test run ends normally
	_, err = io.WriteString(
		stdin,
		`{"Time":"2026-01-01T10:00:30Z","Action":"pass","Package":"example.com/e2e","Test":"TestSlow"}`+"\n"+
			`{"Time":"2026-01-01T10:00:30Z","Action":"pass","Package":"example.com/e2e"}`+"\n",
	)
	require.NoError(t, err)
	require.NoError(t, stdin.Close())
	select {
	case waitErr := <-done:
		require.NoError(t, waitErr, "stderr: %s", stderr)
	case <-time.After(30 * time.Second):
		t.Fatalf("gorp did not exit\nstdout: %s\nstderr: %s", stdout, stderr)
	}
	assert.Equal(t, "PASSED", e.findLaunch(t, name, finished).GetStatus())
	assert.Len(t, e.logs(t, running.Id, 1), 1, "the log is not sent a second time at the end")
}
