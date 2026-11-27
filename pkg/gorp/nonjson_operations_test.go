package gorp

import (
	"net/http"
	"net/url"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v3"
)

func (o operationTemplate) String() string {
	return o.method + " /" + strings.Join(o.segments, "/")
}

// specOperations reads the specification the generated client was built from and splits its operations
// into those that answer a successful request with JSON and those that don't.
func specOperations(t *testing.T) (jsonOps, nonJSONOps []string) {
	t.Helper()

	type operation struct {
		Responses map[string]struct {
			Content map[string]struct {
				Schema struct {
					Type   string `yaml:"type"`
					Format string `yaml:"format"`
				} `yaml:"schema"`
			} `yaml:"content"`
		} `yaml:"responses"`
	}
	var spec struct {
		Paths map[string]map[string]yaml.Node `yaml:"paths"`
	}
	raw, err := os.ReadFile("../openapi/api/openapi.yaml")
	require.NoError(t, err)
	require.NoError(t, yaml.Unmarshal(raw, &spec))

	methods := []string{"get", "put", "post", "delete", "options", "head", "patch", "trace"}
	for path, item := range spec.Paths {
		for method, node := range item {
			if !slices.Contains(methods, method) {
				continue
			}
			var op operation
			require.NoError(t, node.Decode(&op))

			returnsJSON := false
			for status, response := range op.Responses {
				content, ok := response.Content["application/json"]
				// a binary string is a file download that the specification labels as JSON
				if strings.HasPrefix(status, "2") && ok && content.Schema.Format != "binary" {
					returnsJSON = true
				}
			}
			name := strings.ToUpper(method) + " " + path
			if returnsJSON {
				jsonOps = append(jsonOps, name)
			} else {
				nonJSONOps = append(nonJSONOps, name)
			}
		}
	}
	slices.Sort(jsonOps)
	slices.Sort(nonJSONOps)

	return jsonOps, nonJSONOps
}

// nonJSONOperations is written by hand next to a generated client, so it is recomputed here from
// the specification of that client. After a regeneration that adds or changes such operations
// the failure message has the list to paste.
func TestNonJSONOperationsMatchSpec(t *testing.T) {
	t.Parallel()

	jsonOps, nonJSONOps := specOperations(t)
	require.NotEmpty(t, jsonOps)

	listed := make([]string, 0, len(nonJSONOperations))
	for _, operation := range nonJSONOperations {
		listed = append(listed, operation.String())
	}

	assert.Equalf(t, nonJSONOps, listed,
		"nonJSONOperations differs from pkg/openapi/api/openapi.yaml, it must be:\n\t\"%s\",",
		strings.Join(nonJSONOps, "\",\n\t\""))
}

// A request is matched to an operation by its path alone, and a path parameter of one operation
// can stand where another one has a fixed segment. A JSON operation whose path also fits a template
// of nonJSONOperations loses the HTML check (it then fails to decode the page, as all of them did before).
// The test keeps that set known: today it is empty.
func TestNonJSONOperationsDoNotShadowJSONOperations(t *testing.T) {
	t.Parallel()

	jsonOps, _ := specOperations(t)

	var shadowed []string
	for _, jsonOp := range jsonOps {
		method, path, _ := strings.Cut(jsonOp, " ")
		for _, operation := range nonJSONOperations {
			if operation.matches(method, pathSegments("/api"+path)) {
				shadowed = append(shadowed, jsonOp+" by "+operation.String())
			}
		}
	}

	assert.Empty(t, shadowed)
}

func TestIsNonJSONOperation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		method string
		url    string
		want   bool
	}{
		{http.MethodGet, "https://rp.example.com/api/v1/prj/launch/5/report", true},
		{http.MethodGet, "https://rp.example.com/api/v1/prj/launch/5/report?view=html", true},
		{http.MethodGet, "https://rp.example.com/api/v1/prj/launch/5/report/", true},
		{http.MethodGet, "https://rp.example.com/behind/a/proxy/api/v1/prj/launch/5/report", true},
		{http.MethodGet, "https://rp.example.com/api/v1/data/prj/42", true},
		{http.MethodGet, "https://rp.example.com/api/v1/plugin/public/jira/file/a%2Fb.html", true},
		{http.MethodGet, "https://rp.example.com/api/v1/project/prj/tms/attachment/7", true},
		{
			http.MethodGet,
			"https://rp.example.com/api/v1/project/prj/tms/attachment/7/thumbnail",
			true,
		},
		{http.MethodGet, "https://rp.example.com/api/v1/project/export", true},
		{http.MethodDelete, "https://rp.example.com/api/organizations/1", true},
		{http.MethodPost, "https://rp.example.com/api/v1/prj/launch/5/report", false},
		{http.MethodGet, "https://rp.example.com/api/v1/prj/launch/5", false},
		{http.MethodGet, "https://rp.example.com/api/v1/prj/launch", false},
		{http.MethodGet, "https://rp.example.com/api/v1/prj/launch/a%2Fb/5/report", false},
		{http.MethodGet, "https://rp.example.com/api/organizations/1", false},
		{http.MethodGet, "https://rp.example.com/ui/", false},
		{http.MethodGet, "https://rp.example.com", false},
	}

	for _, tt := range tests {
		t.Run(tt.method+" "+tt.url, func(t *testing.T) {
			t.Parallel()

			u, err := url.Parse(tt.url)
			require.NoError(t, err)

			assert.Equal(t, tt.want, isNonJSONOperation(&http.Request{Method: tt.method, URL: u}))
		})
	}
}
