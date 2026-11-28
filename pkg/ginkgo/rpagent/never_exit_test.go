package rpagent

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// terminatingCalls lists the calls in the non-test Go files of dir that end or abort the
// process: log.Fatal*, log.Panic*, os.Exit and panic.
func terminatingCalls(dir string) ([]string, error) {
	forbidden := map[string]map[string]bool{
		"log": {
			"Fatal":   true,
			"Fatalf":  true,
			"Fatalln": true,
			"Panic":   true,
			"Panicf":  true,
			"Panicln": true,
		},
		"os": {"Exit": true},
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	fset := token.NewFileSet()
	var found []string
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, filepath.Join(dir, name), nil, 0)
		if err != nil {
			return nil, err
		}
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			switch fn := call.Fun.(type) {
			case *ast.Ident:
				if fn.Name == "panic" {
					found = append(found, fset.Position(call.Pos()).String()+": panic")
				}
			case *ast.SelectorExpr:
				if pkg, ok := fn.X.(*ast.Ident); ok && forbidden[pkg.Name][fn.Sel.Name] {
					found = append(
						found,
						fset.Position(call.Pos()).String()+": "+pkg.Name+"."+fn.Sel.Name,
					)
				}
			}
			return true
		})
	}
	return found, nil
}

// Reporting must never kill the test process, so library code may not terminate it. This
// scans the package source so a regression fails here instead of ending a user's test run.
func TestLibraryNeverTerminatesTheProcess(t *testing.T) {
	found, err := terminatingCalls(".")
	if err != nil {
		t.Fatal(err)
	}
	if len(found) > 0 {
		t.Errorf("library code must not end the process:\n  %s", strings.Join(found, "\n  "))
	}
}

func TestTerminatingCallScannerFindsViolations(t *testing.T) {
	dir := t.TempDir()
	bad := "package x\n\nimport (\n\t\"log\"\n\t\"os\"\n)\n\n" +
		"func a() { log.Fatalf(\"x\") }\nfunc b() { os.Exit(1) }\nfunc c() { panic(\"x\") }\nfunc d() { log.Printf(\"fine\") }\n" +
		"// log.Fatal in a comment is fine\nvar s = \"os.Exit in a string is fine\"\n"
	if err := os.WriteFile(filepath.Join(dir, "bad.go"), []byte(bad), 0o600); err != nil {
		t.Fatal(err)
	}
	// Test files are exempt.
	if err := os.WriteFile(
		filepath.Join(dir, "ok_test.go"),
		[]byte("package x\n\nfunc e() { panic(\"in a test\") }\n"),
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	found, err := terminatingCalls(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(found) != 3 {
		t.Fatalf("want exactly log.Fatalf, os.Exit and panic, got %v", found)
	}
	for _, want := range []string{"log.Fatalf", "os.Exit", "panic"} {
		hit := false
		for _, f := range found {
			hit = hit || strings.HasSuffix(f, want)
		}
		if !hit {
			t.Errorf("%s was not reported: %v", want, found)
		}
	}
}

func TestNewLaunchCoordinatorRejectsAnEmptySessionID(t *testing.T) {
	// This used to call log.Fatal, which would have ended this very test run.
	lc, err := NewLaunchCoordinator("")
	if err == nil || lc != nil {
		t.Fatalf("an empty session ID must be an error, got %v, %v", lc, err)
	}
	if lc, err := NewLaunchCoordinator("s1"); err != nil || lc == nil {
		t.Fatalf("a session ID must work, got %v, %v", lc, err)
	}
}

func TestAttachmentsSurviveAnUnusableTempDir(t *testing.T) {
	old := tempAttachmentDir
	t.Cleanup(func() { tempAttachmentDir = old })
	tempAttachmentDir = ""
	t.Setenv("TMPDIR", filepath.Join(t.TempDir(), "does", "not", "exist"))

	// This used to call log.Fatalf and end the whole test process.
	InitializeAttachments()
	if tempAttachmentDir != "" {
		t.Fatalf("no attachment directory could be created, but one is set: %q", tempAttachmentDir)
	}

	src := filepath.Join(t.TempDir(), "report.txt")
	if err := os.WriteFile(src, []byte("data"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := copyToTempLocation(src); err == nil {
		t.Errorf(
			"without a directory the attachment must fail with an error, not be written somewhere else",
		)
	}
}
