package commands

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSubtestParameters(t *testing.T) {
	t.Parallel()

	tests := map[string][]string{ // subtest name -> "key=value" of the expected parameters
		"n=1":                    {"n=1"},
		"n=1#01":                 {"n=1"},
		"work-dur=1.5s":          {"work-dur=1.5s"},
		"Elem=*int32":            {"Elem=*int32"},
		"size=64/align=8":        {"size=64", "align=8"},
		"tls=true,http2=false":   {"tls=true", "http2=false"},
		"a=1/b=2,c=3":            {"a=1", "b=2", "c=3"},
		"name=two_words":         {"name=two_words"},
		"list=a,b,c":             {"list=a,b,c"},
		"path=a/b":               {"path=a/b"},
		"expr=a=b":               nil,
		"url=http://x/y?q=1":     nil,
		"empty=":                 nil,
		"=novalue":               nil,
		"plain":                  nil,
		"with_n=1_inside/plain":  nil,
		"case_1:_n=1":            nil,
		"fails_when_n=1":         nil, // "fails when n=1": spaces arrive as underscores
		"max_size=10":            nil, // not told apart from a sentence
		"x=5_works":              {"x=5_works"},
		"1st=x":                  nil,
		"":                       nil,
		"n=1/":                   {"n=1"},
		"SetZero/string/size=16": nil,
	}
	for name, want := range tests {
		var got []string
		for _, p := range subtestParameters(name) {
			got = append(got, p.Key+"="+p.GetValue())
		}
		assert.Equal(t, want, got, "subtest %q", name)
	}
}

// testdata/params/params.jsonl is unedited `go test -json` output (go1.27.4) of a test whose subtests
// are named after the conventions above; it shows what the testing package does to such names.
func TestReportLaunch_SubtestParameters(t *testing.T) {
	t.Parallel()

	rec, srv := newHierarchyServer(t, "proj")
	cfg := &clientConfig{URL: srv.URL, Project: "proj", ApiKey: "tok"}
	file := filepath.Join("testdata", "params", "params.jsonl")
	_, _, err := reportLaunch(t.Context(), cfg, reportCmd(file, "launch", false))
	require.NoError(t, err)

	const test = "example.com/params/p/TestParams"
	want := map[string][]string{ // code reference -> parameters
		test:                          nil,
		test + "/n=1":                 {"n=1"},
		test + "/n=1#01":              {"n=1"},
		test + "/n=2":                 {"n=2"},
		test + "/tableSize=8/elems=4": {"tableSize=8", "elems=4"},
		test + "/cache=ram":           {"cache=ram"},
		// subtests run with the parameters of their parents
		test + "/cache=ram/pages=16":             {"cache=ram", "pages=16"},
		test + "/cache=ram/pages=16/plain":       {"cache=ram", "pages=16"},
		test + "/sameSubject=true,sameKey=false": {"sameSubject=true", "sameKey=false"},
		test + "/name=two_words":                 {"name=two_words"},
		test + "/list=a,b,c":                     {"list=a,b,c"},
		test + "/expr=a=b":                       nil,
		test + "/empty=":                         nil,
		test + "/=novalue":                       nil,
		test + "/not_a_parameter":                nil,
		test + "/url=http://x/y?q=1":             nil,
	}
	for codeRef, wantParams := range want {
		runs := rec.runs(codeRef)
		require.Len(t, runs, 1, codeRef)
		body := runs[0].Body
		assert.Equal(
			t,
			codeRef,
			body["testCaseId"],
			"the test case ID is the full name, with or without parameters",
		)

		var got []string
		params, _ := body["parameters"].([]any)
		for _, p := range params {
			param, ok := p.(map[string]any)
			require.True(t, ok)
			got = append(got, param["key"].(string)+"="+param["value"].(string))
		}
		assert.Equal(t, wantParams, got, codeRef)
	}
	assert.Len(t, rec.items, len(want)+1, "the tests and the suite")
}
