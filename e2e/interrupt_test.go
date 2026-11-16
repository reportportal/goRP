//go:build e2e

package e2e

import (
	"io"
	"runtime"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Ctrl-C while the test run is still being piped in must neither hang on the blocked stdin nor leave
// the launch IN_PROGRESS.
func TestReportTest2JSON_InterruptFinishesLaunch(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("sending SIGINT to a child process is not supported on Windows")
	}
	e := newEnv(t)
	name := uniqueName(t)

	cmd, stdout, stderr := command(t.Context(), "report", "test2json", "--launchName", name)
	stdin, err := cmd.StdinPipe()
	require.NoError(t, err)
	require.NoError(t, cmd.Start())
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	t.Cleanup(func() { _ = cmd.Process.Kill() })

	// one event starts the launch and a test; stdin then stays open and idle, like a long test run
	_, err = io.WriteString(
		stdin,
		`{"Time":"2026-01-01T10:00:00Z","Action":"run","Package":"example.com/e2e","Test":"TestSlow"}`+"\n",
	)
	require.NoError(t, err)
	running := e.findLaunch(t, name, nil)
	require.Equal(t, "IN_PROGRESS", running.GetStatus())

	require.NoError(t, cmd.Process.Signal(syscall.SIGINT))

	select {
	case waitErr := <-done:
		assert.Error(t, waitErr, "an interrupted report is not a success")
	case <-time.After(30 * time.Second):
		t.Fatalf("gorp did not exit after SIGINT\nstdout: %s\nstderr: %s", stdout, stderr)
	}

	launch := e.findLaunch(t, name, finished)
	assert.Equal(t, "INTERRUPTED", launch.GetStatus(), "stderr: %s", stderr)
}
