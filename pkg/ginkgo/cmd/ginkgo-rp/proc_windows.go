//go:build windows

package main

import (
	"os"
	"os/exec"
)

func isolate(*exec.Cmd) {}

func signalCode(os.Signal) int { return 1 }

// Windows cannot deliver Ctrl-C to a child process, so stop it outright.
func forwardSignal(cmd *exec.Cmd, _ os.Signal) { _ = cmd.Process.Kill() }

func killGroup(cmd *exec.Cmd) { _ = cmd.Process.Kill() }
