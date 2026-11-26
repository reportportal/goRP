package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/reportportal/goRP/pkg/ginkgo/rpagent"
)

// DiscoveredSuite represents a Ginkgo test suite found in the filesystem.
type DiscoveredSuite struct {
	Directory    string // Absolute, symlink-resolved path to the suite directory
	RelativePath string // Relative to the working directory
	Name         string // From RunSpecs(t, "Suite Name")
	FilePath     string // The *_test.go file that calls RunSpecs
}

// resolveDir returns the absolute, symlink-resolved path of an existing
// directory. Symlinks are resolved because the worker looks its suite up by
// its own resolved working directory.
func resolveDir(userPath string) (string, error) {
	if userPath == "" {
		return "", errors.New("empty path provided")
	}
	abs, err := filepath.Abs(userPath)
	if err != nil {
		return "", fmt.Errorf("invalid path: %w", err)
	}
	info, err := os.Stat(abs)
	if err != nil {
		return "", fmt.Errorf("cannot access path: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("path is not a directory: %s", abs)
	}
	if resolved, rerr := filepath.EvalSymlinks(abs); rerr == nil {
		abs = resolved
	}
	return abs, nil
}

// discoverOpts carries the parts of the command line that decide which packages Ginkgo
// would run, so discovery lists the same suites and not packages Ginkgo will skip.
type discoverOpts struct {
	Tags         []string // --tags, which decide which test files are compiled
	SkipPackages []string // --skip-package filters, matched as Ginkgo matches them
}

// ginkgoPath renders dir the way Ginkgo names a suite path ("./calculator"), because
// --skip-package filters are matched against that form.
func ginkgoPath(cwd, dir string) string {
	rel, err := filepath.Rel(cwd, dir)
	if err != nil {
		return dir
	}
	if !strings.HasPrefix(rel, ".") {
		rel = "." + string(filepath.Separator) + rel
	}
	return rel
}

// discoverSuites finds the Ginkgo suites under t. A suite is a package with a test file
// that imports Ginkgo and that is compiled for the given tags, as in Ginkgo itself.
func discoverSuites(t target, opts discoverOpts) ([]DiscoveredSuite, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get working directory: %w", err)
	}
	root, err := resolveDir(t.Dir)
	if err != nil {
		return nil, fmt.Errorf("invalid target path: %w", err)
	}
	if resolvedCwd, rerr := filepath.EvalSymlinks(cwd); rerr == nil {
		cwd = resolvedCwd
	}

	skipped := func(dir string) bool {
		p := ginkgoPath(cwd, dir)
		for _, f := range opts.SkipPackages {
			if f != "" && strings.Contains(p, f) {
				return true
			}
		}
		return false
	}
	suiteAt := func(dir string) (DiscoveredSuite, bool) {
		if skipped(dir) {
			return DiscoveredSuite{}, false
		}
		file, name, ok := rpagent.FindSuite(dir, opts.Tags)
		if !ok {
			return DiscoveredSuite{}, false
		}
		rel, rerr := filepath.Rel(cwd, dir)
		if rerr != nil {
			rel = dir
		}
		return DiscoveredSuite{Directory: dir, RelativePath: rel, Name: name, FilePath: file}, true
	}

	if !t.Recursive {
		s, ok := suiteAt(root)
		if !ok {
			return nil, fmt.Errorf(
				"no Ginkgo suite found in %s (needs a test file importing Ginkgo, "+
					"compiled for the given --tags, in a package not excluded by --skip-package)",
				root,
			)
		}
		return []DiscoveredSuite{s}, nil
	}

	var suites []DiscoveredSuite
	err = filepath.WalkDir(root, func(path string, d os.DirEntry, werr error) error {
		if werr != nil {
			return werr
		}
		if !d.IsDir() {
			return nil
		}
		base := d.Name()
		// Mirror Ginkgo's own directory skip rules (never skip the root itself).
		if path != root &&
			(base == "vendor" || base == "testdata" || strings.HasPrefix(base, ".") || strings.HasPrefix(base, "_")) {
			return filepath.SkipDir
		}
		if s, ok := suiteAt(path); ok {
			suites = append(suites, s)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to walk directory tree: %w", err)
	}
	return suites, nil
}
