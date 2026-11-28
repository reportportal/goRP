//go:build !windows

package main

import (
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	"golang.org/x/sys/unix"
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

// Hooks, so tests can make the check below answer for any situation.
var (
	getpid  = os.Getpid
	getppid = os.Getppid
	getsid  = unix.Getsid
)

// canBeResumed reports whether something above the wrapper can resume it after it has
// stopped: a shell with job control that will send SIGCONT on `fg` or `bg`. Where nothing
// can, stopping would freeze the run for good. That is the case when the wrapper is process 1
// (`docker run -it … ginkgo-rp`), when its parent is process 1 (a non-interactive shell as
// the container's init), and when its parent is in another session (`ssh -t host ginkgo-rp`,
// a CI pseudo-terminal): its process group is then orphaned. Without this code the kernel
// discards Ctrl-Z for an orphaned group, so the right answer there is to do nothing.
func canBeResumed() bool {
	pid, ppid := getpid(), getppid()
	if pid == 1 || ppid == 1 {
		return false
	}
	mine, err := getsid(0)
	if err != nil {
		return false
	}
	parents, err := getsid(ppid)
	return err == nil && mine == parents
}

// watchJobControl keeps the terminal's job control working from ginkgo's start until the
// wrapper ends (after ginkgo has exited, the signal to its group finds nobody, and the wrapper
// still stops itself, so Ctrl-Z works during finalization too). Ginkgo is in
// its own process group (see isolate), so the terminal's Ctrl-Z reaches only the wrapper:
// without this, the wrapper would stop while ginkgo and the tests carried on in the
// background, and `fg`/`bg` would resume nothing. Instead, Ctrl-Z is passed on to ginkgo's
// group, the wrapper stops itself, and the SIGCONT of `fg`/`bg` is passed on as well.
// Where the wrapper cannot be resumed (see canBeResumed) Ctrl-Z is left alone.
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
					if !canBeResumed() {
						continue
					}
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
