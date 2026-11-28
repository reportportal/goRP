package rpagent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAgentLogWritesToProjectRootAndClosesCleanly(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module x\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	sub := filepath.Join(root, "a", "b")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(sub)

	if err := InitializeRPLogger(); err != nil {
		t.Fatal(err)
	}
	LogVerboseOperation("verbose line")
	LogInfo("COMP", "info line")
	LogError("COMP", "error line")
	logHTTP("POST", "http://rp/api", "200 OK", time.Second, `{"a":1}`, "")
	CloseRPLogger()
	CloseRPLogger() // closing twice must be harmless
	LogInfo("COMP", "after close is dropped")

	b, err := os.ReadFile(filepath.Join(root, "rp-agent.log")) // found go.mod two levels up
	if err != nil {
		t.Fatal(err)
	}
	got := string(b)
	for _, want := range []string{"verbose line", "info line", "error line", "HTTP POST http://rp/api", "<empty>", "Agent stopped"} {
		if !strings.Contains(got, want) {
			t.Errorf("rp-agent.log is missing %q", want)
		}
	}
	if strings.Contains(got, "after close") {
		t.Error("nothing may be written after CloseRPLogger")
	}
}

func TestMaskAuthRedactsBearerToken(t *testing.T) {
	in := "x\nAuthorization: Bearer secret-token\nnext"
	out := maskAuth(in)
	if strings.Contains(out, "secret-token") || !strings.Contains(out, "[REDACTED]\nnext") {
		t.Errorf("maskAuth = %q", out)
	}
	if got := maskAuth("Authorization: Bearer tail"); strings.Contains(got, "tail") {
		t.Errorf("token at end of text not redacted: %q", got)
	}
	if got := maskAuth("nothing secret"); got != "nothing secret" {
		t.Errorf("unrelated text changed: %q", got)
	}
}

func TestFormatJSON(t *testing.T) {
	if formatJSON("") != "<empty>" {
		t.Error("empty body")
	}
	if got := formatJSON(`{"a":1}`); !strings.Contains(got, "\n") {
		t.Errorf("JSON should be indented, got %q", got)
	}
	if formatJSON("not json") != "not json" {
		t.Error("non-JSON must pass through")
	}
}
