package rpagent

import (
	"errors"
	"go/ast"
	"go/build"
	"go/parser"
	"go/token"
	"path/filepath"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"
)

// FindSuite locates the Ginkgo suite in dir the way Ginkgo does: a package is a suite
// when one of the test files that the Go build would compile (honoring build tags such
// as the sanity tag) imports Ginkgo. It returns the file that calls RunSpecs (or, failing
// that, the first Ginkgo-importing test file) and the suite name taken from the RunSpecs
// description argument, resolved from a string literal or a package-level string constant.
// If the name cannot be resolved statically it falls back to the directory name. ok is
// false when the directory is not a Ginkgo suite for the given tags.
func FindSuite(dir string, tags []string) (file, name string, ok bool) {
	var files []string
	for _, f := range buildableTestFiles(dir, tags) {
		files = append(files, filepath.Join(dir, f))
	}
	sort.Strings(files)

	fset := token.NewFileSet()
	consts := map[string]string{}
	type hit struct {
		file string
		arg  ast.Expr
	}
	var hits []hit
	firstGinkgoFile := ""
	for _, f := range files {
		parsed, perr := parser.ParseFile(fset, f, nil, parser.SkipObjectResolution)
		if perr != nil {
			continue
		}
		if importsGinkgo(parsed) && firstGinkgoFile == "" {
			firstGinkgoFile = f
		}
		collectStringConsts(parsed, consts)
		ast.Inspect(parsed, func(n ast.Node) bool {
			call, isCall := n.(*ast.CallExpr)
			if !isCall || len(call.Args) < 2 || !isRunSpecs(call.Fun) {
				return true
			}
			hits = append(hits, hit{f, call.Args[1]})
			return true
		})
	}
	if firstGinkgoFile == "" {
		return "", "", false
	}
	if len(hits) == 0 {
		return firstGinkgoFile, filepath.Base(dir), true
	}
	h := hits[0]
	return h.file, resolveString(h.arg, consts, filepath.Base(dir)), true
}

// buildableTestFiles lists the *_test.go files in dir that the Go build compiles for the
// given tags (file-name suffixes, //go:build lines and GOOS/GOARCH are all honored).
func buildableTestFiles(dir string, tags []string) []string {
	ctx := build.Default
	ctx.BuildTags = append(append([]string(nil), ctx.BuildTags...), tags...)
	pkg, err := ctx.ImportDir(dir, 0)
	if pkg == nil {
		return nil
	}
	var nogo *build.NoGoError
	if err != nil && errors.As(err, &nogo) {
		return nil
	}
	// Other errors (such as two package names) still leave the file lists filled in.
	return append(append([]string(nil), pkg.TestGoFiles...), pkg.XTestGoFiles...)
}

// importsGinkgo reports whether the file imports Ginkgo v1 or v2, as Ginkgo's own suite
// detection does.
func importsGinkgo(f *ast.File) bool {
	for _, imp := range f.Imports {
		path, err := strconv.Unquote(imp.Path.Value)
		if err == nil && strings.HasPrefix(path, "github.com/onsi/ginkgo") {
			return true
		}
	}
	return false
}

// BuildTagsOfThisBinary returns the -tags the running test binary was built with, so a
// worker resolves its suite with the same file set the wrapper used.
func BuildTagsOfThisBinary() []string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return nil
	}
	for _, s := range info.Settings {
		if s.Key == "-tags" && s.Value != "" {
			return strings.Split(s.Value, ",")
		}
	}
	return nil
}

func isRunSpecs(fn ast.Expr) bool {
	switch f := fn.(type) {
	case *ast.Ident:
		return f.Name == "RunSpecs"
	case *ast.SelectorExpr:
		return f.Sel.Name == "RunSpecs"
	}
	return false
}

func collectStringConsts(f *ast.File, out map[string]string) {
	for _, d := range f.Decls {
		gd, isGen := d.(*ast.GenDecl)
		if !isGen || gd.Tok != token.CONST {
			continue
		}
		for _, s := range gd.Specs {
			vs := s.(*ast.ValueSpec)
			for i, id := range vs.Names {
				if i < len(vs.Values) {
					if v, vok := stringLit(vs.Values[i]); vok {
						out[id.Name] = v
					}
				}
			}
		}
	}
}

func stringLit(e ast.Expr) (string, bool) {
	lit, isLit := e.(*ast.BasicLit)
	if !isLit || lit.Kind != token.STRING {
		return "", false
	}
	v, err := strconv.Unquote(lit.Value)
	return v, err == nil
}

func resolveString(e ast.Expr, consts map[string]string, fallback string) string {
	if v, ok := stringLit(e); ok && strings.TrimSpace(v) != "" {
		return strings.TrimSpace(v)
	}
	if id, ok := e.(*ast.Ident); ok {
		if v, found := consts[id.Name]; found && strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return fallback
}
