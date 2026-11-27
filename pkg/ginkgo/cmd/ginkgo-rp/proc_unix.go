//go:build !windows

package main

import (
	"os"
	"os/exec"
	"os/signal"
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

// stopSelf stops the wrapper, so the shell shows the job as Stopped. It uses SIGSTOP, which
// cannot be caught or ignored: a Go program that has caught SIGTSTP does not get the default
// "stop" action back by re-sending it to itself. A variable so tests can observe the call
// instead of stopping the test binary.
var stopSelf = func() {
	_ = syscall.Kill(syscall.Getpid(), syscall.SIGSTOP)
}

// watchJobControl keeps the terminal's job control working while ginkgo runs. Ginkgo is in
// its own process group (see isolate), so the terminal's Ctrl-Z reaches only the wrapper:
// without this, the wrapper would stop while ginkgo and the tests carried on in the
// background, and `fg`/`bg` would resume nothing. Instead, Ctrl-Z is passed on to ginkgo's
// group, the wrapper stops itself, and the SIGCONT of `fg`/`bg` is passed on as well.
// The returned function stops watching.
func watchJobControl(cmd *exec.Cmd) (stop func()) {
	ch := make(chan os.Signal, 4)
	signal.Notify(ch, syscall.SIGTSTP, syscall.SIGCONT)
	quit := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			select {
			case <-quit:
				return
			case sig := <-ch:
				switch sig {
				case syscall.SIGTSTP:
					_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGTSTP)
					stopSelf()
				case syscall.SIGCONT:
					_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGCONT)
				}
			}
		}
	}()
	return func() {
		close(quit)
		<-done
		signal.Stop(ch)
	}
}
