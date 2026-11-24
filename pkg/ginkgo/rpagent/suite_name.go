package rpagent

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// FindSuite locates the Ginkgo suite bootstrap in dir: any *_test.go file that
// calls RunSpecs (the same rule Ginkgo uses, so the file need not be named
// *_suite_test.go). It returns the file and the suite name taken from the
// RunSpecs description argument, resolved from a string literal or a
// package-level string constant. If the name cannot be resolved statically it
// falls back to the directory name. ok is false when no RunSpecs call exists.
func FindSuite(dir string) (file, name string, ok bool) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", "", false
	}
	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), "_test.go") {
			files = append(files, filepath.Join(dir, e.Name()))
		}
	}
	sort.Strings(files)

	fset := token.NewFileSet()
	consts := map[string]string{}
	type hit struct {
		file string
		arg  ast.Expr
	}
	var hits []hit
	for _, f := range files {
		parsed, perr := parser.ParseFile(fset, f, nil, parser.SkipObjectResolution)
		if perr != nil {
			continue
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
	if len(hits) == 0 {
		return "", "", false
	}
	h := hits[0]
	return h.file, resolveString(h.arg, consts, filepath.Base(dir)), true
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
