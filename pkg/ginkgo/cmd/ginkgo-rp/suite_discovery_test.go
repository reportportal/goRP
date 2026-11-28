package main

import (
	"os"
	"path/filepath"
	"testing"
)

func mkSuiteFile(t *testing.T, root, rel, file, header string) {
	t.Helper()
	dir := filepath.Join(root, rel)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	body := header + "package x\nimport . \"github.com/onsi/ginkgo/v2\"\n" +
		"func TestX(t *testing.T) { RunSpecs(t, \"S-" + rel + "\") }\n"
	if err := os.WriteFile(filepath.Join(dir, file), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func mkSuite(t *testing.T, root, rel, file string) { mkSuiteFile(t, root, rel, file, "") }

func TestDiscoverSuites(t *testing.T) {
	root := t.TempDir()
	mkSuite(t, root, "a", "a_suite_test.go")
	mkSuite(t, root, "b/c", "whatever_test.go") // not named *_suite_test.go
	mkSuite(t, root, "vendor/v", "v_suite_test.go")
	mkSuite(t, root, "testdata/t", "t_suite_test.go")
	mkSuite(t, root, ".hidden", "h_suite_test.go")
	mkSuite(t, root, "_private", "p_suite_test.go")
	// A plain Go test package is not a Ginkgo suite.
	if err := os.MkdirAll(filepath.Join(root, "plain"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "plain", "p_test.go"),
		[]byte("package p\nimport \"testing\"\nfunc TestP(t *testing.T) {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := discoverSuites(target{Dir: root, Recursive: true}, discoverOpts{})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("want 2 suites (a, b/c), got %+v", got)
	}

	// Target outside the cwd is allowed (Ginkgo allows it too).
	one, err := discoverSuites(target{Dir: filepath.Join(root, "a")}, discoverOpts{})
	if err != nil || len(one) != 1 || one[0].Name != "S-a" {
		t.Fatalf("non-recursive: %+v, %v", one, err)
	}

	// Non-recursive on a dir without a suite fails loudly.
	if _, err := discoverSuites(target{Dir: filepath.Join(root, "b")}, discoverOpts{}); err == nil {
		t.Fatal("expected error for dir without suite")
	}

	// Symlinked target resolves to the real directory (worker looks up by resolved cwd).
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(filepath.Join(root, "a"), link); err != nil {
		t.Fatal(err)
	}
	l, err := discoverSuites(target{Dir: link}, discoverOpts{})
	real, _ := filepath.EvalSymlinks(filepath.Join(root, "a"))
	if err != nil || l[0].Directory != real {
		t.Fatalf("symlink: %+v, %v", l, err)
	}
}

// A suite behind a build tag only exists for a run that passes that tag, exactly as for
// Ginkgo. Without this, tagged examples showed up as empty suites that never compiled.
func TestDiscoverSuitesHonoursTags(t *testing.T) {
	root := t.TempDir()
	mkSuiteFile(t, root, "tagged", "t_test.go", "//go:build sanity\n\n")
	mkSuite(t, root, "always", "a_test.go")

	got, err := discoverSuites(target{Dir: root, Recursive: true}, discoverOpts{})
	if err != nil || len(got) != 1 || got[0].Name != "S-always" {
		t.Fatalf("without the tag: %+v, %v", got, err)
	}
	got, err = discoverSuites(
		target{Dir: root, Recursive: true},
		discoverOpts{Tags: []string{"sanity"}},
	)
	if err != nil || len(got) != 2 {
		t.Fatalf("with the tag: %+v, %v", got, err)
	}
}

// --skip-package is matched against Ginkgo's "./relative/path" form, so a skipped package
// is not announced as an empty suite.
func TestDiscoverSuitesHonoursSkipPackage(t *testing.T) {
	root := t.TempDir()
	mkSuite(t, root, "keep", "k_test.go")
	mkSuite(t, root, "slow", "s_test.go")
	t.Chdir(root)

	got, err := discoverSuites(
		target{Dir: ".", Recursive: true},
		discoverOpts{SkipPackages: []string{"slow"}},
	)
	if err != nil || len(got) != 1 || got[0].Name != "S-keep" {
		t.Fatalf("got %+v, %v", got, err)
	}
	// An empty filter (from a missing flag) must not skip everything.
	got, err = discoverSuites(
		target{Dir: ".", Recursive: true},
		discoverOpts{SkipPackages: []string{""}},
	)
	if err != nil || len(got) != 2 {
		t.Fatalf("empty filter: %+v, %v", got, err)
	}
}
