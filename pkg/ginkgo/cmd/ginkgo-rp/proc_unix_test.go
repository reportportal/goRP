//go:build !windows

package main

import (
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// procState is the state letter of a process as ps reports it ("T" means stopped).
func procState(t *testing.T, pid int) string {
	t.Helper()
	out, err := exec.Command("ps", "-o", "stat=", "-p", strconv.Itoa(pid)).Output()
	if err != nil {
		return "gone"
	}
	return strings.TrimSpace(string(out))
}

func waitForState(t *testing.T, pid int, want func(string) bool, what string) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if want(procState(t, pid)) {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("%s: process state is %q", what, procState(t, pid))
}

// ginkgo runs in its own process group, so the terminal's Ctrl-Z (SIGTSTP) reaches only the
// wrapper. Job control must still work: the stop is passed on to ginkgo's group, the wrapper
// stops itself, and the continue of fg/bg is passed on too.
func TestWatchJobControlPassesStopAndContinueToGinkgosGroup(t *testing.T) {
	cmd := exec.Command("sleep", "120") // stands in for ginkgo
	isolate(cmd)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		_ = cmd.Wait()
	})

	stopped := make(chan struct{}, 4)
	old := stopSelf
	stopSelf = func() { stopped <- struct{}{} } // do not stop the test binary
	t.Cleanup(func() { stopSelf = old })

	stop := watchJobControl(cmd)
	defer stop()

	isStopped := func(s string) bool { return strings.HasPrefix(s, "T") }
	isRunning := func(s string) bool { return s != "gone" && !strings.HasPrefix(s, "T") }
	waitForState(t, cmd.Process.Pid, isRunning, "before Ctrl-Z")

	_ = syscall.Kill(os.Getpid(), syscall.SIGTSTP) // the terminal's Ctrl-Z
	select {
	case <-stopped:
	case <-time.After(10 * time.Second):
		t.Fatal("the wrapper did not stop itself on SIGTSTP")
	}
	waitForState(t, cmd.Process.Pid, isStopped, "after Ctrl-Z: ginkgo's group must be stopped")

	_ = syscall.Kill(os.Getpid(), syscall.SIGCONT) // fg or bg
	waitForState(t, cmd.Process.Pid, isRunning, "after fg/bg: ginkgo's group must run again")

	// And it can be suspended again.
	_ = syscall.Kill(os.Getpid(), syscall.SIGTSTP)
	select {
	case <-stopped:
	case <-time.After(10 * time.Second):
		t.Fatal("the second Ctrl-Z was not handled")
	}
	waitForState(t, cmd.Process.Pid, isStopped, "after the second Ctrl-Z")
}
