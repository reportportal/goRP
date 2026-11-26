package rpagent

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func clearRPEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{
		"RP_ENDPOINT", "RP_PROJECT", "RP_API_KEY", "RP_LAUNCH", "RP_DEBUG",
		"RP_DESCRIPTION", "RP_ATTRIBUTES", "RP_MODE",
	} {
		t.Setenv(k, "")
	}
}

func TestConfigEnvOnly(t *testing.T) {
	clearRPEnv(t)
	if _, err := ConfigFromEnv(); err == nil {
		t.Fatal("expected error with empty env")
	}
	t.Setenv("RP_ENDPOINT", "http://rp")
	t.Setenv("RP_PROJECT", "p")
	t.Setenv("RP_API_KEY", "k")
	t.Setenv("RP_ATTRIBUTES", "build:42; smoke ;;env:ci")
	t.Setenv("RP_MODE", "debug")
	t.Setenv("RP_DESCRIPTION", "nightly")
	c, err := ConfigFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	want := []LaunchAttribute{{"build", "42"}, {"", "smoke"}, {"env", "ci"}}
	if !reflect.DeepEqual(c.Attributes, want) || c.Mode != "DEBUG" ||
		c.Description != "nightly" || c.LaunchName != "GINKGO-RP-LAUNCH" {
		t.Fatalf("unexpected config: %+v", c)
	}
}

func TestConfigEnvOverridesFile(t *testing.T) {
	clearRPEnv(t)
	f := filepath.Join(t.TempDir(), "reportportal.properties")
	body := "rp.endpoint=http://file\nrp.project=fp\nrp.api.key=fk\nrp.launch=fromfile\nrp.attributes=a:b\n"
	if err := os.WriteFile(f, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("RP_LAUNCH", "fromenv")
	c, err := LoadConfig(f)
	if err != nil {
		t.Fatal(err)
	}
	if c.LaunchName != "fromenv" || c.Endpoint != "http://file" || len(c.Attributes) != 1 {
		t.Fatalf("unexpected config: %+v", c)
	}
}

func TestConfigRejectsBadMode(t *testing.T) {
	clearRPEnv(t)
	t.Setenv("RP_ENDPOINT", "e")
	t.Setenv("RP_PROJECT", "p")
	t.Setenv("RP_API_KEY", "k")
	t.Setenv("RP_MODE", "weird")
	if _, err := ConfigFromEnv(); err == nil {
		t.Fatal("expected error for bad mode")
	}
}

func TestTagLabels(t *testing.T) {
	got := tagLabels([]string{"smoke", "rp.priority:1", "rp.ref:X-1", " api ", ""})
	if !reflect.DeepEqual(got, []string{"smoke", "api"}) {
		t.Fatalf("got %v", got)
	}
}

func TestConfigFileEnvWinsOverSearch(t *testing.T) {
	clearRPEnv(t)
	// The properties file lives in a tree the working directory is not under.
	cfgDir := t.TempDir()
	f := filepath.Join(cfgDir, "reportportal.properties")
	body := "rp.endpoint=http://fromfile\nrp.project=p\nrp.api.key=k\n"
	if err := os.WriteFile(f, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Chdir(t.TempDir())

	if _, err := LoadRPConfig(); err == nil {
		t.Fatal("no file upwards and no env: want an error")
	}
	t.Setenv(ConfigFileEnv, f)
	c, err := LoadRPConfig()
	if err != nil || c.Endpoint != "http://fromfile" {
		t.Fatalf("with %s set: got %+v, %v", ConfigFileEnv, c, err)
	}
}

func TestConfigFileWalksUpAndHonoursDepth(t *testing.T) {
	clearRPEnv(t)
	root := t.TempDir()
	body := "rp.endpoint=e\nrp.project=p\nrp.api.key=k\n"
	if err := os.WriteFile(
		filepath.Join(root, "reportportal.properties"),
		[]byte(body),
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	deep := filepath.Join(root, "a", "b", "c")
	if err := os.MkdirAll(deep, 0o700); err != nil {
		t.Fatal(err)
	}
	t.Chdir(deep)
	if _, err := LoadRPConfig(); err != nil {
		t.Fatalf("file three levels up should be found: %v", err)
	}
	t.Setenv("RP_PROPERTIES_DEPTH", "2")
	if _, err := LoadRPConfig(); err == nil {
		t.Fatal("depth 2 must not reach a file three levels up")
	}
}

func TestConfigFileEnvPointingNowhereFallsBack(t *testing.T) {
	clearRPEnv(t)
	t.Setenv(ConfigFileEnv, filepath.Join(t.TempDir(), "missing.properties"))
	t.Chdir(t.TempDir())
	if _, err := LoadRPConfig(); err == nil {
		t.Fatal("a missing file and no env settings must be an error, not a panic")
	}
}
