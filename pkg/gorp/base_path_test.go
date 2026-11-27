package gorp

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/reportportal/goRP/v5/pkg/openapi"
)

// hostPathCases lists host suffixes and the path prefix every request must carry.
var hostPathCases = []struct {
	name       string
	hostSuffix string
	prefix     string
}{
	{name: "no path", hostSuffix: "", prefix: ""},
	{name: "root path", hostSuffix: "/", prefix: ""},
	{name: "prefix", hostSuffix: "/reportportal", prefix: "/reportportal"},
	{name: "prefix with trailing slash", hostSuffix: "/reportportal/", prefix: "/reportportal"},
	{name: "prefix with trailing slashes", hostSuffix: "/reportportal//", prefix: "/reportportal"},
	{name: "nested prefix", hostSuffix: "/a/b/", prefix: "/a/b"},
	{name: "escaped prefix", hostSuffix: "/report%20portal/", prefix: "/report%20portal"},
}

// newPrefixedServer serves only under prefix+"/api/" (anything else is 404) and records the escaped request paths.
func newPrefixedServer(t *testing.T, prefix string) (*httptest.Server, func() []string) {
	t.Helper()

	var mu sync.Mutex
	var requests []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requests = append(requests, r.Method+" "+r.URL.EscapedPath())
		mu.Unlock()

		if !strings.HasPrefix(r.URL.EscapedPath(), prefix+"/api/") {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{}`))
	}))
	t.Cleanup(server.Close)

	return server, func() []string {
		mu.Lock()
		defer mu.Unlock()
		return append([]string(nil), requests...)
	}
}

// TestClientKeepsHostPathPrefix checks that both halves of Client (the generated OpenAPI client and the Resty launch
// helpers) keep the path of the host URL, e.g. when ReportPortal is served under https://srv/reportportal.
func TestClientKeepsHostPathPrefix(t *testing.T) {
	t.Parallel()

	for _, tc := range hostPathCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			server, requests := newPrefixedServer(t, tc.prefix)
			u, err := url.Parse(server.URL + tc.hostSuffix)
			require.NoError(t, err)
			client := NewClient(u, WithApiKeyAuth(t.Context(), "uuid"))

			// Only the requested paths are checked: the canned "{}" body does not decode into every response model.
			// Generated client: what `gorp launch list` and `gorp quality-gate check` call.
			client.LaunchAPI.GetProjectLaunches(t.Context(), "prj").Execute()
			client.LaunchAPI.GetLaunch(t.Context(), "42", "prj").Execute()
			// Resty helper of the same client.
			client.GetLaunchesByFilterString(t.Context(), "prj", "")

			assert.Equal(t, []string{
				"GET " + tc.prefix + "/api/v1/prj/launch",
				"GET " + tc.prefix + "/api/v1/prj/launch/42",
				"GET " + tc.prefix + "/api/v1/prj/launch",
			}, requests())
		})
	}
}

// TestReportingClientKeepsHostPathPrefix checks that ReportingClient resolves the same URLs as Client for the same host.
func TestReportingClientKeepsHostPathPrefix(t *testing.T) {
	t.Parallel()

	for _, tc := range hostPathCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			server, requests := newPrefixedServer(t, tc.prefix)
			client := NewReportingClient(
				server.URL+tc.hostSuffix,
				"prj",
				WithApiKeyAuth(t.Context(), "uuid"),
			)

			_, err := client.StartLaunch(t.Context(), &openapi.StartLaunchRQ{})
			require.NoError(t, err)

			assert.Equal(t, []string{"POST " + tc.prefix + "/api/v2/prj/launch"}, requests())
		})
	}
}
