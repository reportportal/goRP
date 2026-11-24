package main

import (
	"os"
	"path/filepath"
	"testing"
)

func mkSuite(t *testing.T, root, rel, file string) {
	t.Helper()
	dir := filepath.Join(root, rel)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	body := "package x\nfunc TestX(t *testing.T) { RunSpecs(t, \"S-" + rel + "\") }\n"
	if err := os.WriteFile(filepath.Join(dir, file), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestDiscoverSuites(t *testing.T) {
	root := t.TempDir()
	mkSuite(t, root, "a", "a_suite_test.go")
	mkSuite(t, root, "b/c", "whatever_test.go") // not named *_suite_test.go
	mkSuite(t, root, "vendor/v", "v_suite_test.go")
	mkSuite(t, root, "testdata/t", "t_suite_test.go")
	mkSuite(t, root, ".hidden", "h_suite_test.go")

	got, err := discoverSuites(root, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("want 2 suites (a, b/c), got %+v", got)
	}

	// Target outside the cwd is allowed (Ginkgo allows it too).
	one, err := discoverSuites(filepath.Join(root, "a"), false)
	if err != nil || len(one) != 1 || one[0].Name != "S-a" {
		t.Fatalf("non-recursive: %+v, %v", one, err)
	}

	// Non-recursive on a dir without a suite fails loudly.
	if _, err := discoverSuites(filepath.Join(root, "b"), false); err == nil {
		t.Fatal("expected error for dir without suite")
	}

	// Symlinked target resolves to the real directory (worker looks up by resolved cwd).
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(filepath.Join(root, "a"), link); err != nil {
		t.Fatal(err)
	}
	l, err := discoverSuites(link, false)
	real, _ := filepath.EvalSymlinks(filepath.Join(root, "a"))
	if err != nil || l[0].Directory != real {
		t.Fatalf("symlink: %+v, %v", l, err)
	}
}
