package rpagent

import (
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

const ginkgoImport = `import . "github.com/onsi/ginkgo/v2"` + "\n"

func TestFindSuite(t *testing.T) {
	cases := []struct {
		name     string
		files    map[string]string
		tags     []string
		wantName string
		wantFile string
		wantOK   bool
	}{
		{
			"literal in suite file",
			map[string]string{"a_suite_test.go": "package a\n" + ginkgoImport +
				`func TestA(t *testing.T) { RunSpecs(t, "My Suite") }`},
			nil,
			"My Suite",
			"a_suite_test.go",
			true,
		},
		{
			"any test file name",
			map[string]string{
				"calc_test.go": "package a\n" + `import "github.com/onsi/ginkgo/v2"` + "\n" +
					`func TestA(t *testing.T) { ginkgo.RunSpecs(t, "Calc") }`,
			},
			nil,
			"Calc",
			"calc_test.go",
			true,
		},
		{"package const", map[string]string{"x_test.go": "package a\n" + ginkgoImport +
			"const suiteName = \"From Const\"\nfunc TestA(t *testing.T) { RunSpecs(t, suiteName) }"}, nil, "From Const", "x_test.go", true},
		{
			"const in sibling file",
			map[string]string{
				"a_test.go": "package a\n" + ginkgoImport + `func TestA(t *testing.T) { RunSpecs(t, title) }`,
				"b_test.go": "package a\nconst title = \"Sibling\"",
			},
			nil,
			"Sibling",
			"a_test.go",
			true,
		},
		{
			"unresolvable falls back to dir name",
			map[string]string{"x_test.go": "package a\n" + ginkgoImport +
				`func TestA(t *testing.T) { RunSpecs(t, os.Getenv("N")) }`},
			nil,
			"DIRNAME",
			"x_test.go",
			true,
		},
		{
			"ginkgo import without RunSpecs is still a suite, as in Ginkgo",
			map[string]string{"x_test.go": "package a\n" + ginkgoImport +
				`var _ = Describe("x", func() {})`},
			nil,
			"DIRNAME",
			"x_test.go",
			true,
		},
		{
			"RunSpecs call without a Ginkgo import is not a suite",
			map[string]string{"x_test.go": "package a\n" +
				`func TestA(t *testing.T) { RunSpecs(t, "S") }`},
			nil,
			"",
			"",
			false,
		},
		{
			"plain go test is not a suite",
			map[string]string{"x_test.go": "package a\nfunc TestA(t *testing.T) {}"},
			nil,
			"",
			"",
			false,
		},
		{
			"unparseable file ignored",
			map[string]string{"bad_test.go": `not go`, "ok_test.go": "package a\n" + ginkgoImport +
				`func TestA(t *testing.T) { RunSpecs(t, "Ok") }`},
			nil,
			"Ok",
			"ok_test.go",
			true,
		},
		{
			"build tag not given: the tagged suite does not exist",
			map[string]string{"s_test.go": "//go:build sanity\n\npackage a\n" +
				ginkgoImport + `func TestA(t *testing.T) { RunSpecs(t, "Tagged") }`},
			nil,
			"",
			"",
			false,
		},
		{
			"build tag given: the tagged suite is found",
			map[string]string{"s_test.go": "//go:build sanity\n\npackage a\n" +
				ginkgoImport + `func TestA(t *testing.T) { RunSpecs(t, "Tagged") }`},
			[]string{"sanity"},
			"Tagged",
			"s_test.go",
			true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "pkgdir")
			if err := os.Mkdir(dir, 0o700); err != nil {
				t.Fatal(err)
			}
			for n, b := range tc.files {
				writeFile(t, dir, n, b)
			}
			file, name, ok := FindSuite(dir, tc.tags)
			want := tc.wantName
			if want == "DIRNAME" {
				want = "pkgdir"
			}
			if ok != tc.wantOK || name != want || (ok && filepath.Base(file) != tc.wantFile) {
				t.Fatalf(
					"got (%q,%q,%v), want (%q,%q,%v)",
					filepath.Base(file),
					name,
					ok,
					tc.wantFile,
					want,
					tc.wantOK,
				)
			}
		})
	}
}
