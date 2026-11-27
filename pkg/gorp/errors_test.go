package gorp

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/reportportal/goRP/v5/pkg/openapi"
)

const htmlPage = "<!DOCTYPE html>\n<html><head><title>ReportPortal</title></head>\n<body><div id=\"app\"></div></body></html>"

// errorResponse is a canned server response.
type errorResponse struct {
	status int
	header map[string]string
	body   string
}

func (r errorResponse) serve(t *testing.T) *httptest.Server {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		for k, v := range r.header {
			w.Header().Set(k, v)
		}
		w.WriteHeader(r.status)
		w.Write([]byte(r.body))
	}))
	t.Cleanup(srv.Close)

	return srv
}

// The bodies are what ReportPortal 6 and the proxies in front of it send for failed requests.
// Each one is checked on the generated client and on a Resty-based client: both must return
// the same HTTPError.
func TestHTTPErrorForEveryResponseShape(t *testing.T) {
	t.Parallel()

	jsonType := map[string]string{"Content-Type": "application/json"}

	tests := []struct {
		name string
		rs   errorResponse
		// wantText is the error text after "<METHOD> <URL>: "
		wantText  string
		wantCode  int64
		wantMsg   string
		wantOAuth string
		wantIs    error
	}{
		{
			name: "ReportPortal ErrorRS with a numeric code",
			rs: errorResponse{
				status: http.StatusNotFound,
				header: jsonType,
				body:   `{"errorCode":4041,"message":"Launch '1' not found. Did you use correct Launch ID?"}`,
			},
			wantText: "404 Not Found: Launch '1' not found. Did you use correct Launch ID? (error code 4041)",
			wantCode: 4041,
			wantMsg:  "Launch '1' not found. Did you use correct Launch ID?",
			wantIs:   ErrNotFound,
		},
		{
			name: "ErrorRS on a status the spec declares as oneOf(string, ErrorRS)",
			rs: errorResponse{
				status: http.StatusUnauthorized,
				header: jsonType,
				body:   `{"errorCode":4011,"message":"You do not have enough permissions."}`,
			},
			wantText: "401 Unauthorized: You do not have enough permissions. (error code 4011)",
			wantCode: 4011,
			wantMsg:  "You do not have enough permissions.",
			wantIs:   ErrUnauthorized,
		},
		{
			name: "ErrorRS with the code as a string",
			rs: errorResponse{
				status: http.StatusForbidden,
				header: jsonType,
				body:   `{"errorCode":"4003","message":"You do not have enough permissions."}`,
			},
			wantText: "403 Forbidden: You do not have enough permissions. (error code 4003)",
			wantCode: 4003,
			wantMsg:  "You do not have enough permissions.",
			wantIs:   ErrForbidden,
		},
		{
			name: "OAuth body of a rejected API key",
			rs: errorResponse{
				status: http.StatusUnauthorized,
				header: map[string]string{"Content-Type": "application/json;charset=ISO-8859-1"},
				body:   "{\n  \"error_description\" : \"Invalid access token\",\n  \"error\" : \"invalid_token\"\n}",
			},
			wantText:  "401 Unauthorized: invalid_token: Invalid access token",
			wantMsg:   "Invalid access token",
			wantOAuth: "invalid_token",
			wantIs:    ErrUnauthorized,
		},
		{
			name: "Spring default body for an unknown path",
			rs: errorResponse{
				status: http.StatusNotFound,
				header: jsonType,
				body:   `{"timestamp":"2026-11-27T14:02:11.123+00:00","status":404,"error":"Not Found","path":"/api/v1/prj/nosuch"}`,
			},
			wantText: "404 Not Found: no such endpoint, check the host and the path",
			wantIs:   ErrNotFound,
		},
		{
			name: "Spring default body for a wrong method",
			rs: errorResponse{
				status: http.StatusMethodNotAllowed,
				header: jsonType,
				body:   `{"timestamp":"2026-11-27T14:02:11.123+00:00","status":405,"error":"Method Not Allowed","path":"/api/v2/prj/launch"}`,
			},
			wantText: `405 Method Not Allowed: {"timestamp":"2026-11-27T14:02:11.123+00:00","status":405,` +
				`"error":"Method Not Allowed","path":"/api/v2/prj/launch"}`,
		},
		{
			name: "empty body with a WWW-Authenticate challenge",
			rs: errorResponse{
				status: http.StatusUnauthorized,
				header: map[string]string{
					"WWW-Authenticate": `Bearer error="invalid_token", error_description="An error occurred: Malformed token"`,
				},
			},
			wantText:  "401 Unauthorized: invalid_token: An error occurred: Malformed token",
			wantMsg:   "An error occurred: Malformed token",
			wantOAuth: "invalid_token",
			wantIs:    ErrUnauthorized,
		},
		{
			name:     "empty body, nothing else",
			rs:       errorResponse{status: http.StatusForbidden},
			wantText: "403 Forbidden",
			wantIs:   ErrForbidden,
		},
		{
			name: "HTML page of a proxy on a status the spec declares as JSON",
			rs: errorResponse{
				status: http.StatusForbidden,
				header: map[string]string{"Content-Type": "text/html"},
				body:   "<html>\n<head><title>403 Forbidden</title></head>\n<body>\n<center>blocked</center>\n</body>\n</html>\n",
			},
			wantText: "403 Forbidden: <html> <head><title>403 Forbidden</title></head> <body> <center>blocked</center> </body> </html>",
			wantIs:   ErrForbidden,
		},
		{
			name: "plain text on a status the spec doesn't declare",
			rs: errorResponse{
				status: http.StatusBadGateway,
				header: map[string]string{"Content-Type": "text/plain"},
				body:   "upstream connect error\n",
			},
			wantText: "502 Bad Gateway: upstream connect error",
		},
		{
			name: "JSON that is not an object",
			rs: errorResponse{
				status: http.StatusInternalServerError,
				header: jsonType,
				body:   `"integration answered with garbage"`,
			},
			wantText: `500 Internal Server Error: "integration answered with garbage"`,
		},
	}

	clients := []struct {
		name      string
		generated bool
		request   string
		call      func(t *testing.T, host string) error
	}{
		{
			name:      "generated client",
			generated: true,
			request:   "GET %s/api/v1/prj/launch/1: ",
			call: func(t *testing.T, host string) error {
				t.Helper()

				u, err := url.Parse(host)
				require.NoError(t, err)
				_, _, err = NewClient(u, WithApiKeyAuth(t.Context(), "key")).LaunchAPI.
					GetLaunch(t.Context(), "1", "prj").
					Execute()

				return err
			},
		},
		{
			name:    "reporting client",
			request: "POST %s/api/v2/prj/launch: ",
			call: func(t *testing.T, host string) error {
				t.Helper()

				_, err := NewReportingClient(host, "prj", WithApiKeyAuth(t.Context(), "key")).
					StartLaunch(t.Context(), &openapi.StartLaunchRQ{Name: "l"})

				return err
			},
		},
	}

	for _, tt := range tests {
		for _, cl := range clients {
			t.Run(tt.name+"/"+cl.name, func(t *testing.T) {
				t.Parallel()

				srv := tt.rs.serve(t)

				err := cl.call(t, srv.URL)
				require.Error(t, err)

				var httpErr *HTTPError
				require.ErrorAs(t, err, &httpErr)
				assert.Equal(
					t,
					strings.ReplaceAll(cl.request, "%s", srv.URL)+tt.wantText,
					err.Error(),
				)
				assert.NotContains(t, err.Error(), "\n")
				assert.Equal(t, tt.rs.status, httpErr.StatusCode)
				assert.Equal(t, tt.rs.body, httpErr.Response)
				assert.Equal(t, tt.wantCode, httpErr.ErrorCode)
				assert.Equal(t, tt.wantMsg, httpErr.Message)
				assert.Equal(t, tt.wantOAuth, httpErr.OAuthError)

				for _, sentinel := range []error{ErrUnauthorized, ErrForbidden, ErrNotFound, ErrNotAPIResponse} {
					assert.Equalf(
						t,
						sentinel == tt.wantIs,
						errors.Is(err, sentinel),
						"errors.Is(err, %q)",
						sentinel,
					)
				}

				var apiErr *openapi.GenericOpenAPIError
				if !cl.generated {
					assert.NotErrorAs(t, err, &apiErr)

					return
				}

				// code written against the generated client keeps working
				require.ErrorAs(t, err, &apiErr)
				assert.Equal(t, tt.rs.body, string(apiErr.Body()))
				assert.Equal(t, httpErr.Status, apiErr.Error())
				if tt.wantCode != 0 && !strings.Contains(tt.rs.body, `"errorCode":"`) {
					errorRS, ok := apiErr.Model().(openapi.ComEpamReportportalBaseInfrastructureRulesExceptionErrorRS)
					require.Truef(t, ok, "unexpected error model %T", apiErr.Model())
					assert.Equal(t, tt.wantCode, errorRS.GetErrorCode())
					assert.Equal(t, tt.wantMsg, errorRS.GetMessage())
				} else {
					assert.Nil(t, apiErr.Model())
				}
			})
		}
	}
}

// A wrong host or a missing /api prefix lands on the UI, which answers any path with 200 and its index.html.
func TestRestyClientsRejectHTMLSuccess(t *testing.T) {
	t.Parallel()

	srv := errorResponse{
		status: http.StatusOK,
		header: map[string]string{"Content-Type": "text/html; charset=utf-8"},
		body:   htmlPage,
	}.serve(t)
	u, err := url.Parse(srv.URL)
	require.NoError(t, err)

	calls := map[string]func() error{
		"ReportingClient.StartLaunch": func() error {
			_, cErr := NewReportingClient(srv.URL, "prj", WithApiKeyAuth(t.Context(), "key")).
				StartLaunch(t.Context(), &openapi.StartLaunchRQ{Name: "l"})

			return cErr
		},
		"Client.GetLaunchesByFilterString": func() error {
			_, cErr := NewClient(u, WithApiKeyAuth(t.Context(), "key")).
				GetLaunchesByFilterString(t.Context(), "prj", "page.size=1")

			return cErr
		},
	}

	for name, call := range calls {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			err := call()

			var httpErr *HTTPError
			require.ErrorAs(t, err, &httpErr)
			require.ErrorIs(t, err, ErrNotAPIResponse)
			assert.Equal(t, http.StatusOK, httpErr.StatusCode)
			assert.Equal(t, htmlPage, httpErr.Response)
			assert.Contains(t, err.Error(), "200 OK: expected JSON but got an HTML page")
			assert.NotContains(t, err.Error(), "<html>")
		})
	}
}

// The generated client has operations that download files and HTML reports, so a successful HTML
// response is not an error for the middleware. Operations that expect JSON fail to decode it.
func TestOpenAPIErrorMiddlewareLeavesSuccessAlone(t *testing.T) {
	t.Parallel()

	srv := errorResponse{
		status: http.StatusOK,
		header: map[string]string{"Content-Type": "text/html"},
		body:   htmlPage,
	}.serve(t)
	u, err := url.Parse(srv.URL)
	require.NoError(t, err)
	client := NewClient(u, WithApiKeyAuth(t.Context(), "key"))

	resp, err := client.LaunchAPI.GetLaunchReport(t.Context(), 1, "prj").Execute()
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	_, _, err = client.LaunchAPI.GetLaunch(t.Context(), "1", "prj").Execute()
	require.Error(t, err)

	var httpErr *HTTPError
	assert.NotErrorAs(t, err, &httpErr)
}

func TestHTTPErrorText(t *testing.T) {
	t.Parallel()

	t.Run("fields set by hand, as before the type knew the request", func(t *testing.T) {
		t.Parallel()

		err := &HTTPError{StatusCode: http.StatusNotFound, Response: "launch not found\n"}

		assert.Equal(t, "404 Not Found: launch not found", err.Error())
		require.NoError(t, err.Unwrap())
	})

	t.Run("the query string stays out of the text", func(t *testing.T) {
		t.Parallel()

		err := &HTTPError{
			StatusCode: http.StatusBadRequest,
			Status:     "400 Bad Request",
			Method:     http.MethodGet,
			URL:        "https://rp.example.com/api/v1/prj/launch?filter.eq.name=secret&page.sort=x",
			Message:    "Sorting parameter x is not defined",
			ErrorCode:  40012,
		}

		assert.Equal(
			t,
			"GET https://rp.example.com/api/v1/prj/launch: 400 Bad Request: Sorting parameter x is not defined (error code 40012)",
			err.Error(),
		)
	})

	t.Run("a long body is cut, the field keeps all of it", func(t *testing.T) {
		t.Parallel()

		body := strings.Repeat("é", 2*maxErrorBodyLen)
		err := &HTTPError{StatusCode: http.StatusBadGateway, Response: body}

		text := err.Error()
		assert.Less(t, len(text), maxErrorBodyLen+100)
		assert.True(t, strings.HasPrefix(text, "502 Bad Gateway: ééé"))
		assert.True(t, strings.HasSuffix(text, "é… (4096 bytes in total)"), text)
		assert.Equal(t, body, err.Response)
	})

	t.Run("a challenge without OAuth parameters", func(t *testing.T) {
		t.Parallel()

		httpErr, _ := newHTTPError(&http.Response{
			StatusCode: http.StatusUnauthorized,
			Header:     http.Header{"Www-Authenticate": {`Basic realm="rp"`}},
		}, nil)

		assert.Equal(t, `401 Unauthorized: WWW-Authenticate: Basic realm="rp"`, httpErr.Error())
	})
}
