package main

import (
	"io"
	"os"
	"os/exec"
	"strings"
	"testing"
)

// captureStdoutStderr runs fn with stdout and stderr redirected, and returns what it printed.
func captureStdoutStderr(t *testing.T, fn func()) (stdout, stderr string) {
	t.Helper()
	oldOut, oldErr := os.Stdout, os.Stderr
	outR, outW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	errR, errW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout, os.Stderr = outW, errW
	outC, errC := make(chan string), make(chan string)
	go func() { b, _ := io.ReadAll(outR); outC <- string(b) }()
	go func() { b, _ := io.ReadAll(errR); errC <- string(b) }()
	func() {
		defer func() {
			os.Stdout, os.Stderr = oldOut, oldErr
			_ = outW.Close()
			_ = errW.Close()
		}()
		fn()
	}()
	return <-outC, <-errC
}

// runWithArgs runs the wrapper in-process with the given command line.
func runWithArgs(t *testing.T, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	old := os.Args
	os.Args = append([]string{"ginkgo-rp"}, args...)
	t.Cleanup(func() { os.Args = old })
	stdout, stderr = captureStdoutStderr(t, func() { code = run() })
	return code, stdout, stderr
}

func TestHelpVersionAndRefusedCommands(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantCode   int
		wantStdout string
		wantStderr string
	}{
		{"help", []string{"help"}, 0, "ginkgo-rp — ReportPortal Agent for Ginkgo", ""},
		{"--help", []string{"--help"}, 0, "ginkgo-rp — ReportPortal Agent for Ginkgo", ""},
		{"-h", []string{"-h"}, 0, "ginkgo-rp — ReportPortal Agent for Ginkgo", ""},
		{
			"-h after a command",
			[]string{"run", "-h"},
			0,
			"ginkgo-rp — ReportPortal Agent for Ginkgo",
			"",
		},
		{"help priority", []string{"help", "priority"}, 0, "Test Priorities", ""},
		{"help reference", []string{"help", "reference"}, 0, "Test Reference IDs", ""},
		{"help attachment", []string{"help", "attachment"}, 0, "File Attachments", ""},
		{"help debug", []string{"help", "debug"}, 0, "Debug Logging", ""},
		{"help parallel", []string{"help", "parallel"}, 0, "Parallel Execution", ""},
		{
			"help parameterized",
			[]string{"help", "parameterized"},
			0,
			"Parameterized / Table-Driven Tests",
			"",
		},
		{
			"help config",
			[]string{"help", "config"},
			0,
			"Configuration — reportportal.properties",
			"",
		},
		{"help unknown topic", []string{"help", "nonsense"}, 0, "Unknown help topic: nonsense", ""},
		{"version", []string{"version"}, 0, "Module: github.com/reportportal/goRP/pkg/ginkgo", ""},
		{"--version", []string{"--version"}, 0, "ginkgo-rp version", ""},
		{
			"a command ginkgo-rp does not wrap",
			[]string{"bootstrap"},
			1,
			"",
			`does not support the "bootstrap" subcommand`,
		},
		{"a typo of help", []string{"hlep"}, 1, "", "Did you mean: ginkgo-rp help"},
		{
			"a typo of --version",
			[]string{"--ver" + "soin"},
			1,
			"",
			"Did you mean: ginkgo-rp --version",
		},
		{
			"a typo of version",
			[]string{"ver" + "ison"},
			1,
			"",
			"Did you mean: ginkgo-rp version",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, stdout, stderr := runWithArgs(t, tt.args...)
			if code != tt.wantCode {
				t.Errorf("exit code = %d, want %d", code, tt.wantCode)
			}
			if !strings.Contains(stdout, tt.wantStdout) {
				t.Errorf("stdout lacks %q:\n%s", tt.wantStdout, stdout)
			}
			if !strings.Contains(stderr, tt.wantStderr) {
				t.Errorf("stderr lacks %q:\n%s", tt.wantStderr, stderr)
			}
		})
	}
}

func TestEveryHelpTopicIsListedAndNonEmpty(t *testing.T) {
	_, listed, _ := runWithArgs(t, "help", "nonsense")
	for _, topic := range []string{"priority", "reference", "attachment", "debug", "parallel", "parameterized", "config"} {
		if !strings.Contains(listed, topic) {
			t.Errorf("topic %q is not in the list of available topics", topic)
		}
		if _, out, _ := runWithArgs(t, "help", topic); len(strings.TrimSpace(out)) < 100 {
			t.Errorf("help topic %q printed almost nothing: %q", topic, out)
		}
	}
}

func TestShowVersionPrintsBuildInformationWhenSet(t *testing.T) {
	oldTime, oldCommit := BuildTime, GitCommit
	BuildTime, GitCommit = "2026-01-02T03:04:05Z", "abc1234"
	t.Cleanup(func() { BuildTime, GitCommit = oldTime, oldCommit })
	out, _ := captureStdoutStderr(t, showVersion)
	for _, want := range []string{"Built: 2026-01-02T03:04:05Z", "Commit: abc1234", "Features:"} {
		if !strings.Contains(out, want) {
			t.Errorf("version output lacks %q:\n%s", want, out)
		}
	}
}

func TestGetGitVersion(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	// Inside a checkout it names a tag or a commit; outside one it is empty. Either way no panic,
	// and no surrounding whitespace.
	if v := getGitVersion(); v != strings.TrimSpace(v) {
		t.Errorf("version %q has surrounding whitespace", v)
	}
}
