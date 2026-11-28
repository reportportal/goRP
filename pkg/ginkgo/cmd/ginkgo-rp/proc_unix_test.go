//go:build !windows

package main

import (
	"bufio"
	"errors"
	"fmt"
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

func TestCanBeResumed(t *testing.T) {
	oldPid, oldPpid, oldSid := getpid, getppid, getsid
	t.Cleanup(func() { getpid, getppid, getsid = oldPid, oldPpid, oldSid })

	// situation: the wrapper's pid, its parent's pid, and the session of each pid.
	tests := []struct {
		name       string
		pid, ppid  int
		sessions   map[int]int // pid -> session; 0 stands for the wrapper itself
		sidErr     bool
		wantResume bool
	}{
		{"started from a job-control shell", 500, 400, map[int]int{0: 300, 400: 300}, false, true},
		{
			"the container's init (docker run -it ... ginkgo-rp)",
			1,
			0,
			map[int]int{0: 1},
			false,
			false,
		},
		{
			"its parent is init (bash -c as the container's init)",
			500,
			1,
			map[int]int{0: 1, 1: 1},
			false,
			false,
		},
		{
			"its parent is in another session (ssh -t, a CI terminal)",
			500,
			400,
			map[int]int{0: 500, 400: 300},
			false,
			false,
		},
		{"the session cannot be read", 500, 400, map[int]int{}, true, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			getpid = func() int { return tt.pid }
			getppid = func() int { return tt.ppid }
			getsid = func(pid int) (int, error) {
				if sid, ok := tt.sessions[pid]; ok && !tt.sidErr {
					return sid, nil
				}
				return 0, errors.New("no such process")
			}
			if got := canBeResumed(); got != tt.wantResume {
				t.Errorf("canBeResumed() = %v, want %v", got, tt.wantResume)
			}
		})
	}
}

// TestJobControlHelperProcess is not a test: the tests below run it as a separate process.
// It plays the wrapper, with the real stopSelf, and a long-running child as ginkgo.
func TestJobControlHelperProcess(t *testing.T) {
	if os.Getenv("GINKGO_RP_JOBCONTROL_HELPER") != "1" {
		t.Skip("helper process for the job-control tests")
	}
	cmd := exec.Command("sleep", "120")
	isolate(cmd)
	if err := cmd.Start(); err != nil {
		fmt.Println("error:", err)
		os.Exit(2)
	}
	stop := watchJobControl(cmd)
	defer stop()
	fmt.Printf("ready sleep=%d\n", cmd.Process.Pid)
	select {} // until the test kills this process
}

// startJobControlHelper starts the helper process, in the test's own session or a new one
// (as under ssh -t or a CI terminal, where the wrapper's parent is in another session).
func startJobControlHelper(t *testing.T, newSession bool) (helper, child int) {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^TestJobControlHelperProcess$")
	cmd.Env = append(os.Environ(), "GINKGO_RP_JOBCONTROL_HELPER=1")
	if newSession {
		cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	}
	out, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = syscall.Kill(cmd.Process.Pid, syscall.SIGKILL)
		if child > 0 {
			_ = syscall.Kill(-child, syscall.SIGKILL)
		}
		_ = cmd.Wait()
	})
	lines := make(chan string, 1)
	go func() {
		sc := bufio.NewScanner(out)
		for sc.Scan() {
			if strings.HasPrefix(sc.Text(), "ready ") {
				lines <- sc.Text()
				return
			}
		}
		close(lines)
	}()
	select {
	case line, ok := <-lines:
		if !ok {
			t.Fatal("the helper process exited before it was ready")
		}
		if _, err := fmt.Sscanf(line, "ready sleep=%d", &child); err != nil {
			t.Fatalf("unexpected helper output %q: %v", line, err)
		}
	case <-time.After(30 * time.Second):
		t.Fatal("the helper process was not ready in time")
	}
	return cmd.Process.Pid, child
}

// With a job-control shell above it, the real wrapper logic must really stop itself and its
// child group on Ctrl-Z (SIGTSTP), and run again on SIGCONT.
func TestJobControlReallyStopsAndResumes(t *testing.T) {
	helper, child := startJobControlHelper(t, false)
	isStopped := func(s string) bool { return strings.HasPrefix(s, "T") }
	isRunning := func(s string) bool { return s != "gone" && !strings.HasPrefix(s, "T") }
	waitForState(t, helper, isRunning, "helper before Ctrl-Z")

	_ = syscall.Kill(helper, syscall.SIGTSTP) // what the terminal sends the wrapper
	waitForState(t, helper, isStopped, "after Ctrl-Z the wrapper itself must be stopped")
	waitForState(t, child, isStopped, "after Ctrl-Z ginkgo's group must be stopped")

	_ = syscall.Kill(helper, syscall.SIGCONT) // fg or bg
	waitForState(t, helper, isRunning, "after fg/bg the wrapper must run again")
	waitForState(t, child, isRunning, "after fg/bg ginkgo's group must run again")
}

// With nothing able to resume it (here: the parent is in another session, as under ssh -t or
// a CI terminal), Ctrl-Z must change nothing. Stopping would freeze the run for good.
func TestJobControlDoesNothingWhereNothingCanResume(t *testing.T) {
	helper, child := startJobControlHelper(t, true)
	isRunning := func(s string) bool { return s != "gone" && !strings.HasPrefix(s, "T") }
	waitForState(t, helper, isRunning, "helper before Ctrl-Z")

	_ = syscall.Kill(helper, syscall.SIGTSTP)
	time.Sleep(2 * time.Second) // long enough for a wrong stop to show
	if s := procState(t, helper); !isRunning(s) {
		t.Errorf("the wrapper stopped (state %q) although nothing could resume it", s)
	}
	if s := procState(t, child); !isRunning(s) {
		t.Errorf("ginkgo's group stopped (state %q) although nothing could resume it", s)
	}
}

// After ginkgo has exited, Ctrl-Z must still stop the wrapper (the run is being finalized): the
// signal to ginkgo's group finds nobody, and the wrapper itself still stops.
func TestJobControlStillStopsTheWrapperAfterGinkgoExited(t *testing.T) {
	cmd := exec.Command("true")
	isolate(cmd)
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}
	stopped := make(chan struct{}, 1)
	old := stopSelf
	stopSelf = func() { stopped <- struct{}{} }
	t.Cleanup(func() { stopSelf = old })

	stop := watchJobControl(cmd)
	defer stop()
	_ = syscall.Kill(os.Getpid(), syscall.SIGTSTP)
	select {
	case <-stopped:
	case <-time.After(10 * time.Second):
		t.Fatal("Ctrl-Z was ignored after ginkgo had exited")
	}
}
