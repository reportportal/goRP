//go:build !windows

package main

import (
	"os"
	"os/exec"
	"syscall"
)

// isolate puts ginkgo in its own process group, so a terminal Ctrl-C reaches it only
// through the wrapper. Otherwise it would get the signal twice (terminal + our forward),
// and Ginkgo treats a second interrupt as "abort without cleanup".
func isolate(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func signalCode(sig os.Signal) int {
	if s, ok := sig.(syscall.Signal); ok {
		return 128 + int(s)
	}
	return 1
}

// forwardSignal delivers sig to ginkgo's whole process group, exactly as a terminal
// Ctrl-C would. Signaling only the ginkgo CLI is not enough: the test processes must
// receive it too, or the running spec is never interrupted.
func forwardSignal(cmd *exec.Cmd, sig os.Signal) {
	if sig == syscall.SIGHUP {
		sig = syscall.SIGTERM // Ginkgo shuts down gracefully on SIGTERM, but not on SIGHUP
	}
	if s, ok := sig.(syscall.Signal); ok {
		if syscall.Kill(-cmd.Process.Pid, s) == nil {
			return
		}
	}
	_ = cmd.Process.Signal(sig)
}

// killGroup force-kills ginkgo and every process in its group (the second signal).
func killGroup(cmd *exec.Cmd) {
	if syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) != nil {
		_ = cmd.Process.Kill()
	}
}
