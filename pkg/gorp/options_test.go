package gorp

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/reportportal/goRP/v5/pkg/openapi"
)

// seenResponses records what a response middleware was called with
type seenResponses struct {
	mu     sync.Mutex
	calls  []string
	bodies []string
}

func (s *seenResponses) middleware(name string) ResponseMiddleware {
	return func(resp *http.Response, body []byte) error {
		s.mu.Lock()
		defer s.mu.Unlock()
		s.calls = append(
			s.calls,
			name+" "+resp.Request.Method+" "+resp.Request.URL.Path+" "+resp.Status,
		)
		s.bodies = append(s.bodies, string(body))

		return nil
	}
}

func jsonServer(t *testing.T, status int, body string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)

	return srv
}

func TestResponseMiddlewareReportingClient(t *testing.T) {
	t.Parallel()

	srv := jsonServer(t, http.StatusCreated, `{"id":"launch-1"}`)
	var seen seenResponses
	client := NewReportingClient(srv.URL, "prj", WithApiKeyAuth(t.Context(), "key"),
		WithResponseMiddleware(seen.middleware("first")),
		nil, // a nil option is ignored
		WithResponseMiddleware(nil),
		WithResponseMiddleware(seen.middleware("second")),
	)

	rs, err := client.StartLaunch(t.Context(), &openapi.StartLaunchRQ{Name: "n"})
	require.NoError(t, err)
	// the middlewares read the body and the client still decodes it
	assert.Equal(t, "launch-1", rs.GetId())
	assert.Equal(t, []string{
		"first POST /api/v2/prj/launch 201 Created",
		"second POST /api/v2/prj/launch 201 Created",
	}, seen.calls)
	assert.Equal(t, []string{`{"id":"launch-1"}`, `{"id":"launch-1"}`}, seen.bodies)
}

func TestResponseMiddlewareKeepsErrorHandling(t *testing.T) {
	t.Parallel()

	const body = `{"errorCode":4001,"message":"Incorrect Request"}`
	srv := jsonServer(t, http.StatusBadRequest, body)
	u, err := url.Parse(srv.URL)
	require.NoError(t, err)

	var seen seenResponses
	client := NewClient(
		u,
		WithApiKeyAuth(t.Context(), "key"),
		WithResponseMiddleware(seen.middleware("mw")),
	)

	// Resty-based method: the middleware sees the failed response, the client still reports its own error
	_, err = client.GetLaunchesByFilterString(t.Context(), "prj", "")
	var httpErr *HTTPError
	require.ErrorAs(t, err, &httpErr)
	assert.Equal(t, http.StatusBadRequest, httpErr.StatusCode)
	assert.Equal(t, body, httpErr.Response)

	// generated client: same middleware, the generated error stays
	_, _, err = client.LaunchAPI.GetProjectLaunches(t.Context(), "prj").Execute()
	var apiErr *openapi.GenericOpenAPIError
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, body, string(apiErr.Body()))

	assert.Equal(t, []string{
		"mw GET /api/v1/prj/launch 400 Bad Request",
		"mw GET /api/v1/prj/launch 400 Bad Request",
	}, seen.calls)
	assert.Equal(t, []string{body, body}, seen.bodies)
}

func TestResponseMiddlewareErrorStopsChain(t *testing.T) {
	t.Parallel()

	srv := jsonServer(t, http.StatusNotFound, `{"errorCode":4041,"message":"not found"}`)
	u, err := url.Parse(srv.URL)
	require.NoError(t, err)

	errMine := errors.New("mine")
	var seen seenResponses
	client := NewClient(u, WithApiKeyAuth(t.Context(), "key"),
		WithResponseMiddleware(func(*http.Response, []byte) error { return errMine }),
		WithResponseMiddleware(seen.middleware("never")),
	)

	_, err = client.GetLaunchesByFilterString(t.Context(), "prj", "")
	require.ErrorIs(t, err, errMine)
	var httpErr *HTTPError
	assert.NotErrorAs(t, err, &httpErr, "the middleware's error replaces the client's own")

	_, _, err = client.LaunchAPI.GetProjectLaunches(t.Context(), "prj").Execute()
	require.ErrorIs(t, err, errMine)

	assert.Empty(t, seen.calls)
}

func TestResponseMiddlewareCanFailSuccessfulResponse(t *testing.T) {
	t.Parallel()

	srv := jsonServer(t, http.StatusOK, `{"id":"x"}`)
	errMine := errors.New("mine")
	client := NewReportingClient(srv.URL, "prj", WithApiKeyAuth(t.Context(), "key"),
		WithResponseMiddleware(func(*http.Response, []byte) error { return errMine }))

	_, err := client.StartLaunch(t.Context(), &openapi.StartLaunchRQ{Name: "n"})
	assert.ErrorIs(t, err, errMine)
}

// Constructors must keep compiling for callers that pass no options.
func TestConstructorsWithoutOptions(t *testing.T) {
	t.Parallel()

	srv := jsonServer(t, http.StatusOK, `{"id":"x"}`)
	u, err := url.Parse(srv.URL)
	require.NoError(t, err)

	assert.NotNil(t, NewClient(u, WithApiKeyAuth(t.Context(), "key")))
	rs, err := NewReportingClient(srv.URL, "prj", WithApiKeyAuth(t.Context(), "key")).
		StartLaunch(t.Context(), &openapi.StartLaunchRQ{Name: "n"})
	require.NoError(t, err)
	assert.Equal(t, "x", rs.GetId())
}

// The generated client has one response middleware: the user's ones go in front of it, they don't replace it.
func TestConfigureOpenAPIKeepsExistingMiddleware(t *testing.T) {
	t.Parallel()

	errOwn := errors.New("own")
	var calls []string
	newConf := func() *openapi.Configuration {
		conf := openapi.NewConfiguration()
		conf.ResponseMiddleware = func(*http.Response, []byte) error {
			calls = append(calls, "own")

			return errOwn
		}

		return conf
	}

	conf := newConf()
	newClientOptions(nil).configureOpenAPI(conf)
	require.ErrorIs(t, conf.ResponseMiddleware(nil, nil), errOwn, "no options: left as is")

	calls = nil
	conf = newConf()
	newClientOptions([]Option{WithResponseMiddleware(func(*http.Response, []byte) error {
		calls = append(calls, "user")

		return nil
	})}).configureOpenAPI(conf)
	require.ErrorIs(t, conf.ResponseMiddleware(nil, nil), errOwn)
	assert.Equal(t, []string{"user", "own"}, calls)
}
