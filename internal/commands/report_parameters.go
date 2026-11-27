package commands

import (
	"regexp"
	"strings"

	"github.com/reportportal/goRP/v5/pkg/openapi"
)

// testParameter is the generated model of a test item parameter.
type testParameter = openapi.ComEpamReportportalBaseReportingParameterResource

var (
	// parameterKey is the key of a "key=value" subtest name: letters and digits, with "." and "-"
	// allowed inside ("keySize=128", "work-dur=1s", "http.version=2"). There is no "_" on purpose: the
	// testing package turns the spaces of a subtest name into underscores, so "fails when n=1" arrives
	// as "fails_when_n=1", which is a sentence and not a pair.
	parameterKey = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9.-]*$`)
	// duplicateSuffix is what the testing package appends to a subtest name that is used twice ("n=1#01").
	duplicateSuffix = regexp.MustCompile(`#\d{2,}$`)
)

// subtestParameters derives test item parameters from the name of a subtest, relative to its parent.
//
// Go has no parameterized tests, but a convention for naming the cases of a table: "key=value", as in
// the standard library's own tests and in the benchmark format ("BenchmarkX/size=64/align=8"). Several
// pairs in one name are separated by "/" or ",". Only names that are nothing but such pairs give
// parameters:
//
//	"n=1"                    n=1
//	"n=1#01"                 n=1 (the suffix that makes a repeated name unique is dropped)
//	"size=64/align=8"        size=64, align=8
//	"tls=true,http2=false"   tls=true, http2=false
//	"list=a,b,c"             list="a,b,c" (the rest is not a pair, so it belongs to the value)
//	"path=a/b"               path="a/b"
//	"expr=a=b", "k=", "=v"   none: ambiguous or incomplete
//	"fails_when_n=1"         none: a sentence, see parameterKey
func subtestParameters(name string) []testParameter {
	name = duplicateSuffix.ReplaceAllString(name, "")

	pieces := strings.FieldsFunc(name, func(r rune) bool { return r == '/' || r == ',' })
	params := make([]testParameter, 0, len(pieces))
	for _, piece := range pieces {
		param, ok := parseParameter(piece)
		if !ok {
			params = nil
			break
		}
		params = append(params, param)
	}
	if len(params) > 0 {
		return params
	}
	if param, ok := parseParameter(name); ok {
		return []testParameter{param}
	}
	return nil
}

// parseParameter parses "key=value" with a key as in parameterKey and a non-empty value without "=".
func parseParameter(s string) (testParameter, bool) {
	key, value, found := strings.Cut(s, "=")
	if !found || value == "" || strings.Contains(value, "=") || !parameterKey.MatchString(key) {
		return testParameter{}, false
	}
	return testParameter{Key: key, Value: openapi.PtrString(value)}, true
}
