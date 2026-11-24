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

func TestFindSuite(t *testing.T) {
	cases := []struct {
		name     string
		files    map[string]string
		wantName string
		wantFile string
		wantOK   bool
	}{
		{"literal in suite file", map[string]string{"a_suite_test.go": `package a
func TestA(t *testing.T) { RunSpecs(t, "My Suite") }`}, "My Suite", "a_suite_test.go", true},
		{"any test file name", map[string]string{"calc_test.go": `package a
func TestA(t *testing.T) { ginkgo.RunSpecs(t, "Calc") }`}, "Calc", "calc_test.go", true},
		{"package const", map[string]string{"x_test.go": `package a
const suiteName = "From Const"
func TestA(t *testing.T) { RunSpecs(t, suiteName) }`}, "From Const", "x_test.go", true},
		{"const in sibling file", map[string]string{
			"a_test.go": `package a
func TestA(t *testing.T) { RunSpecs(t, title) }`,
			"b_test.go": `package a
const title = "Sibling"`,
		}, "Sibling", "a_test.go", true},
		{"unresolvable falls back to dir name", map[string]string{"x_test.go": `package a
func TestA(t *testing.T) { RunSpecs(t, os.Getenv("N")) }`}, "DIRNAME", "x_test.go", true},
		{"no RunSpecs", map[string]string{"x_test.go": `package a
func TestA(t *testing.T) {}`}, "", "", false},
		{
			"unparseable file ignored",
			map[string]string{"bad_test.go": `not go`, "ok_test.go": `package a
func TestA(t *testing.T) { RunSpecs(t, "Ok") }`},
			"Ok",
			"ok_test.go",
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
			file, name, ok := FindSuite(dir)
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
