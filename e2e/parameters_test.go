//go:build e2e

package e2e

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Subtests named "key=value" get parameters; the test case ID stays the full test name. The stream is
// real `go test -json` output, see internal/commands/report_parameters_test.go.
func TestReportTest2JSON_SubtestParameters(t *testing.T) {
	e := newEnv(t)
	name := uniqueName(t)
	input := filepath.Join("..", "internal", "commands", "testdata", "params", "params.jsonl")

	runGorp(t, "report", "test2json", "--file", input, "--launchName", name).requireOK(t)

	launch := e.findLaunch(t, name, finished)
	items := e.items(t, launch.Id, 17) // the suite, TestParams and its 15 subtests
	params := func(item string) []string {
		require.NotNil(t, items[item], item)
		got := []string{}
		for _, p := range items[item].Parameters {
			got = append(got, p.Key+"="+p.Value)
		}

		return got
	}

	assert.Equal(t, []string{"n=2"}, params("n=2"))
	assert.Equal(
		t,
		[]string{"n=1"},
		params("n=1#01"),
		"the suffix of a repeated name is not a value",
	)
	// the server does not keep the order of the parameters
	assert.ElementsMatch(t, []string{"tableSize=8", "elems=4"}, params("tableSize=8/elems=4"))
	assert.ElementsMatch(
		t,
		[]string{"cache=ram", "pages=16"},
		params("pages=16"),
		"with the parent's parameter",
	)
	assert.ElementsMatch(t, []string{"cache=ram", "pages=16"}, params("plain"))
	assert.ElementsMatch(
		t,
		[]string{"sameSubject=true", "sameKey=false"},
		params("sameSubject=true,sameKey=false"),
	)
	assert.Empty(t, params("expr=a=b"))
	assert.Empty(t, params("not_a_parameter"))
	assert.Empty(t, params("TestParams"))

	const test = "example.com/params/p/TestParams"
	assert.Equal(t, test+"/n=2", items["n=2"].TestCaseID)
	assert.Equal(t, test+"/cache=ram/pages=16", items["pages=16"].TestCaseID)
}
