package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// The flags below are the ones the maintainer reported as being mistaken for package
// paths. They take a value, and the value must never be treated as a path.
func TestSplitArgsKeepsFlagValuesOutOfPaths(t *testing.T) {
	cases := []struct {
		name      string
		in        []string
		wantFlags []string
		wantPaths []string
	}{
		{
			"compilers",
			[]string{"--compilers", "2", "./a"},
			[]string{"--compilers", "2"},
			[]string{"./a"},
		},
		{
			"ldflags",
			[]string{"--ldflags", "-X main.v=1", "./a"},
			[]string{"--ldflags", "-X main.v=1"},
			[]string{"./a"},
		},
		{
			"coverpkg",
			[]string{"--coverpkg", "./...", "./a"},
			[]string{"--coverpkg", "./..."},
			[]string{"./a"},
		},
		{
			"single-dash tags",
			[]string{"-tags", "sanity", "./a"},
			[]string{"-tags", "sanity"},
			[]string{"./a"},
		},
		{
			"label-filter",
			[]string{"-label-filter", "smoke && !slow", "./a"},
			[]string{"-label-filter", "smoke && !slow"},
			[]string{"./a"},
		},
		{"inline value", []string{"--procs=4", "./a"}, []string{"--procs=4"}, []string{"./a"}},
		{
			"bool flags",
			[]string{"-r", "--keep-going", "./a"},
			[]string{"-r", "--keep-going"},
			[]string{"./a"},
		},
		{
			"flags typed after the path",
			[]string{"./a", "--keep-going", "./b"},
			[]string{"--keep-going"},
			[]string{"./a", "./b"},
		},
		{"no paths", []string{"-r"}, []string{"-r"}, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := splitArgs(c.in)
			if !reflect.DeepEqual(got.Flags, c.wantFlags) ||
				!reflect.DeepEqual(got.Positional, c.wantPaths) {
				t.Fatalf(
					"flags %q paths %q, want flags %q paths %q",
					got.Flags,
					got.Positional,
					c.wantFlags,
					c.wantPaths,
				)
			}
		})
	}
}

func TestSplitArgsReadsTagsSkipPackageAndRecursion(t *testing.T) {
	got := splitArgs([]string{"-r", "--tags", "sanity,x", "--skip-package=foo,bar", "./a"})
	if got.Tags != "sanity,x" || got.SkipPackage != "foo,bar" || !got.Recursive {
		t.Fatalf("got %+v", got)
	}
}

func TestSplitArgsLeavesTestBinaryArgsAlone(t *testing.T) {
	got := splitArgs([]string{"./a", "--", "-ginkgo.v", "extra"})
	if !reflect.DeepEqual(got.Positional, []string{"./a"}) ||
		!reflect.DeepEqual(got.Tail, []string{"--", "-ginkgo.v", "extra"}) {
		t.Fatalf("got %+v", got)
	}
}

func TestSplitArgsSubcommand(t *testing.T) {
	got := splitArgs([]string{"run", "-r", "./a"})
	if got.Subcommand != "run" || !reflect.DeepEqual(got.Positional, []string{"./a"}) {
		t.Fatalf("got %+v", got)
	}
}

// A flag this build does not know makes Ginkgo's parser fail; the fallback must still
// find the real paths and must not fail the run.
func TestSplitArgsFallsBackOnUnknownFlags(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	if err := os.Mkdir("pkg", 0o700); err != nil {
		t.Fatal(err)
	}
	got := splitArgs(
		[]string{"--some-future-flag", "value-not-a-dir", "-tags=sanity", "pkg", "./..."},
	)
	if !reflect.DeepEqual(got.Positional, []string{"pkg", "./..."}) {
		t.Fatalf("paths = %q", got.Positional)
	}
	if got.Tags != "sanity" {
		t.Fatalf("tags = %q", got.Tags)
	}
}

func TestTargets(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.MkdirAll(filepath.Join("a", "b"), 0o700); err != nil {
		t.Fatal(err)
	}

	// No path means the current directory.
	if got := splitArgs([]string{"-r"}).targets(); !reflect.DeepEqual(got, []target{{".", true}}) {
		t.Fatalf("no path: %+v", got)
	}
	// ./... means "." recursively; a plain directory is not recursive without -r.
	if got := splitArgs(
		[]string{"./...", "a"},
	).targets(); !reflect.DeepEqual(
		got,
		[]target{{".", true}, {"a", false}},
	) {
		t.Fatalf("patterns: %+v", got)
	}
	if got := splitArgs(
		[]string{"./a/..."},
	).targets(); !reflect.DeepEqual(
		got,
		[]target{{"./a", true}},
	) {
		t.Fatalf("subtree pattern: %+v", got)
	}
	// A path that is not a directory is skipped, never fatal, and does not turn into ".".
	if got := splitArgs([]string{"runn"}).targets(); len(got) != 0 {
		t.Fatalf("typo path: %+v", got)
	}
}
