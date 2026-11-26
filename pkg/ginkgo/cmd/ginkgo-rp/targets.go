package main

import (
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/onsi/ginkgo/v2/types"
)

// parsedArgs is the command line split the way Ginkgo splits it.
type parsedArgs struct {
	Subcommand  string   // "run" when given explicitly
	Flags       []string // flag tokens including their values, in order
	Positional  []string // package paths exactly as typed
	Tail        []string // "--" and everything after it, which belongs to the test binary
	Tags        string
	SkipPackage string
	Recursive   bool
}

// splitArgs separates flags from package paths using Ginkgo's own flag definitions, so a
// flag that takes a value (--tags sanity, --compilers 2, -label-filter x) is never mistaken
// for a package path and the list cannot drift out of date. If an installed Ginkgo knows
// flags this build does not, parsing fails and a path-existence heuristic takes over.
func splitArgs(args []string) parsedArgs {
	var out parsedArgs
	if i := slices.Index(args, "--"); i >= 0 {
		out.Tail = args[i:]
		args = args[:i]
	}
	if len(args) > 0 && args[0] == "run" {
		out.Subcommand = "run"
		args = args[1:]
	}

	var (
		suite    types.SuiteConfig
		reporter types.ReporterConfig
		cli      types.CLIConfig
		goFlags  types.GoFlagsConfig
	)
	fs, err := types.BuildRunCommandFlagSet(&suite, &reporter, &cli, &goFlags)
	if err != nil {
		return heuristicSplit(out, args)
	}
	rest := args
	for len(rest) > 0 {
		pos, perr := fs.Parse(rest)
		if perr != nil {
			return heuristicSplit(out, args)
		}
		out.Flags = append(out.Flags, rest[:len(rest)-len(pos)]...)
		if len(pos) == 0 {
			break
		}
		out.Positional = append(out.Positional, pos[0])
		rest = pos[1:]
	}
	out.Tags, out.SkipPackage, out.Recursive = goFlags.Tags, cli.SkipPackage, cli.Recurse
	return out
}

// heuristicSplit is the fallback for flags this build does not know: a token is a package
// path only if it looks like one (an existing directory or a ./... pattern); anything else
// is taken to be the value of the preceding flag. It never fails the run.
func heuristicSplit(out parsedArgs, args []string) parsedArgs {
	out.Flags, out.Positional = nil, nil
	for _, a := range args {
		switch {
		case strings.HasPrefix(a, "-"):
			out.Flags = append(out.Flags, a)
		case looksLikePackagePath(a):
			out.Positional = append(out.Positional, a)
		default:
			out.Flags = append(out.Flags, a)
		}
	}
	out.Tags = flagValue(args, "tags")
	out.SkipPackage = flagValue(args, "skip-package")
	out.Recursive = slices.Contains(args, "-r") || slices.Contains(args, "--r") ||
		slices.Contains(args, "-recursive") || slices.Contains(args, "--recursive")
	return out
}

func looksLikePackagePath(a string) bool {
	if a == "..." || strings.HasSuffix(a, "/...") {
		return true
	}
	fi, err := os.Stat(a)
	return err == nil && fi.IsDir()
}

// flagValue reads -name=v, --name=v or "-name v" from args.
func flagValue(args []string, name string) string {
	for i, a := range args {
		trimmed := strings.TrimLeft(a, "-")
		if trimmed == a || a == "" {
			continue
		}
		if key, val, ok := strings.Cut(trimmed, "="); ok && key == name {
			return val
		}
		if trimmed == name && i+1 < len(args) {
			return args[i+1]
		}
	}
	return ""
}

// target is one place to look for suites.
type target struct {
	Dir       string
	Recursive bool
}

// targets turns the typed package paths into directories to scan. A path that is not a
// directory is reported and skipped; it is never fatal, because Ginkgo itself is the one
// that decides whether the run is valid.
func (p parsedArgs) targets() []target {
	var out []target
	for _, raw := range p.Positional {
		dir, rec := raw, false
		if raw == "..." || strings.HasSuffix(raw, "/...") {
			dir, rec = strings.TrimSuffix(strings.TrimSuffix(raw, "..."), "/"), true
			if dir == "" {
				dir = "."
			}
		}
		fi, err := os.Stat(dir)
		if err != nil || !fi.IsDir() {
			fmt.Fprintf(
				os.Stderr,
				"WARNING: %q is not a directory; ignoring it for ReportPortal suite discovery\n",
				raw,
			)
			continue
		}
		out = append(out, target{Dir: dir, Recursive: rec || p.Recursive})
	}
	if len(out) == 0 && len(p.Positional) == 0 {
		out = append(out, target{Dir: ".", Recursive: p.Recursive})
	}
	return out
}
