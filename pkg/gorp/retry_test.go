package gorp

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"

	"github.com/reportportal/goRP/v5/pkg/openapi"
)

// fastRetry keeps the tests quick: the pauses are a few milliseconds
func fastRetry(maxRetries int) Option {
	return WithRetry(
		RetryPolicy{
			MaxRetries: maxRetries,
			MinWait:    time.Millisecond,
			MaxWait:    5 * time.Millisecond,
		},
	)
}

type seenRequest struct {
	method string
	path   string
	body   string
}

// flakyServer answers with the given statuses in turn, then with 200, and records the requests.
// The status 0 stands for a broken connection: the server closes it without a response.
type flakyServer struct {
	*httptest.Server

	mu       sync.Mutex
	statuses []int
	header   http.Header
	requests []seenRequest
}

func newFlakyServer(t *testing.T, statuses ...int) *flakyServer {
	t.Helper()

	s := &flakyServer{statuses: statuses, header: http.Header{}}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)

		s.mu.Lock()
		s.requests = append(
			s.requests,
			seenRequest{method: r.Method, path: r.URL.Path, body: string(body)},
		)
		status := http.StatusOK
		if len(s.statuses) > 0 {
			status, s.statuses = s.statuses[0], s.statuses[1:]
		}
		s.mu.Unlock()

		if status == 0 {
			conn, _, err := w.(http.Hijacker).Hijack()
			if assert.NoError(t, err) {
				_ = conn.Close()
			}

			return
		}

		for k, v := range s.header {
			w.Header()[k] = v
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		if status < http.StatusMultipleChoices {
			_, _ = w.Write([]byte(`{"id":"server-id","content":[],"page":{"totalPages":1}}`))
		} else {
			_, _ = fmt.Fprintf(w, `{"errorCode":%d,"message":"failed"}`, status*10)
		}
	}))
	t.Cleanup(s.Close)

	return s
}

func (s *flakyServer) seen() []seenRequest {
	s.mu.Lock()
	defer s.mu.Unlock()

	return append([]seenRequest(nil), s.requests...)
}

func (s *flakyServer) reporting(t *testing.T, opts ...Option) *ReportingClient {
	t.Helper()

	return NewReportingClient(s.URL, "prj", WithApiKeyAuth(t.Context(), "key"), opts...)
}

func (s *flakyServer) client(t *testing.T, opts ...Option) *Client {
	t.Helper()
	u, err := url.Parse(s.URL)
	require.NoError(t, err)

	return NewClient(u, WithApiKeyAuth(t.Context(), "key"), opts...)
}

var uuidV4 = regexp.MustCompile(
	`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`,
)

func bodyUUID(t *testing.T, body string) string {
	t.Helper()
	var parsed struct {
		UUID string `json:"uuid"`
	}
	require.NoError(t, json.Unmarshal([]byte(body), &parsed), body)

	return parsed.UUID
}

func TestRetryGet(t *testing.T) {
	t.Parallel()

	srv := newFlakyServer(t, http.StatusServiceUnavailable, http.StatusBadGateway)
	_, err := srv.client(t, fastRetry(3)).
		GetLaunchesByFilterString(t.Context(), "prj", "filter.eq.name=x")

	require.NoError(t, err)
	assert.Len(t, srv.seen(), 3)
}

func TestNoRetryWithoutOption(t *testing.T) {
	t.Parallel()

	srv := newFlakyServer(t, http.StatusServiceUnavailable)
	client := srv.client(t)

	_, err := client.GetLaunchesByFilterString(t.Context(), "prj", "")
	var httpErr *HTTPError
	require.ErrorAs(t, err, &httpErr)
	assert.Equal(t, http.StatusServiceUnavailable, httpErr.StatusCode)
	assert.Len(t, srv.seen(), 1)

	// a policy without retries is the same as none
	srv = newFlakyServer(t, http.StatusServiceUnavailable)
	_, err = srv.client(t, WithRetry(RetryPolicy{})).
		GetLaunchesByFilterString(t.Context(), "prj", "")
	require.Error(t, err)
	assert.Len(t, srv.seen(), 1)
}

func TestRetryStatuses(t *testing.T) {
	t.Parallel()

	tests := map[int]bool{
		http.StatusBadRequest:          false,
		http.StatusUnauthorized:        false,
		http.StatusForbidden:           false,
		http.StatusNotFound:            false,
		http.StatusNotAcceptable:       false,
		http.StatusConflict:            false,
		http.StatusTooManyRequests:     true,
		http.StatusInternalServerError: true,
		http.StatusNotImplemented:      false,
		http.StatusBadGateway:          true,
		http.StatusServiceUnavailable:  true,
		http.StatusGatewayTimeout:      true,
	}
	for status, retried := range tests {
		t.Run(http.StatusText(status), func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, retried, retryableStatus(status))

			// hand-written (Resty) method
			srv := newFlakyServer(t, status)
			_, err := srv.reporting(t, fastRetry(2)).
				FinishLaunch(t.Context(), "l1", &openapi.FinishExecutionRQ{})
			if retried {
				require.NoError(t, err)
				assert.Len(t, srv.seen(), 2)
			} else {
				var httpErr *HTTPError
				require.ErrorAs(t, err, &httpErr)
				assert.Equal(t, status, httpErr.StatusCode)
				assert.Len(t, srv.seen(), 1)
			}

			// generated client
			srv = newFlakyServer(t, status)
			_, _, err = srv.client(t, fastRetry(2)).
				LaunchAPI.GetProjectLaunches(t.Context(), "prj").
				Execute()
			if retried {
				require.NoError(t, err)
				assert.Len(t, srv.seen(), 2)
			} else {
				require.Error(t, err)
				assert.Len(t, srv.seen(), 1)
			}
		})
	}
}

// When every attempt fails the caller gets the error of the last response, the same as without retries,
// and each client makes MaxRetries+1 attempts: the two retry mechanisms must not multiply.
func TestRetryExhausted(t *testing.T) {
	t.Parallel()

	failing := []int{
		http.StatusServiceUnavailable,
		http.StatusServiceUnavailable,
		http.StatusBadGateway,
		http.StatusOK,
	}

	srv := newFlakyServer(t, failing...)
	_, err := srv.client(t, fastRetry(2)).GetLaunchesByFilterString(t.Context(), "prj", "")
	var httpErr *HTTPError
	require.ErrorAs(t, err, &httpErr)
	assert.Equal(t, http.StatusBadGateway, httpErr.StatusCode)
	assert.JSONEq(t, `{"errorCode":5020,"message":"failed"}`, httpErr.Response)
	assert.Len(t, srv.seen(), 3)

	srv = newFlakyServer(t, failing...)
	_, rs, err := srv.client(t, fastRetry(2)).
		LaunchAPI.GetProjectLaunches(t.Context(), "prj").
		Execute()
	var apiErr *openapi.GenericOpenAPIError
	require.ErrorAs(t, err, &apiErr)
	assert.JSONEq(t, `{"errorCode":5020,"message":"failed"}`, string(apiErr.Body()))
	assert.Equal(t, http.StatusBadGateway, rs.StatusCode)
	assert.Len(t, srv.seen(), 3)
}

func TestRetryFinishRepeatsBody(t *testing.T) {
	t.Parallel()

	srv := newFlakyServer(t, http.StatusInternalServerError, 0)
	client := srv.reporting(t, fastRetry(3))

	_, err := client.FinishTest(t.Context(), "item-1", &openapi.FinishTestItemRQ{
		LaunchUuid: "launch-1",
		Status:     Statuses.Passed.Ptr(),
		EndTime:    time.Date(2026, 11, 27, 10, 0, 0, 0, time.UTC),
	})
	require.NoError(t, err)

	seen := srv.seen()
	require.Len(t, seen, 3)
	for _, rq := range seen {
		assert.Equal(t, http.MethodPut, rq.method)
		assert.Equal(t, "/api/v2/prj/item/item-1", rq.path)
		assert.Equal(t, seen[0].body, rq.body)
	}
	assert.Contains(t, seen[0].body, `"launchUuid":"launch-1"`)
}

func TestRetryStartGetsClientUUID(t *testing.T) {
	t.Parallel()

	starts := map[string]func(c *ReportingClient) error{
		"launch": func(c *ReportingClient) error {
			rq := &openapi.StartLaunchRQ{Name: "n"}
			_, err := c.StartLaunch(t.Context(), rq)
			assert.Empty(t, rq.Uuid, "the caller's request must stay as it was")

			return err
		},
		"root item": func(c *ReportingClient) error {
			rq := &openapi.StartTestItemRQ{Name: "n", LaunchUuid: "l"}
			_, err := c.StartTest(t.Context(), rq)
			assert.Empty(t, rq.Uuid, "the caller's request must stay as it was")

			return err
		},
		"child item": func(c *ReportingClient) error {
			rq := &openapi.StartTestItemRQ{Name: "n", LaunchUuid: "l"}
			_, err := c.StartChildTest(t.Context(), "parent", rq)
			assert.Empty(t, rq.Uuid, "the caller's request must stay as it was")

			return err
		},
	}
	for name, start := range starts {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// a lost response, then an error, then success
			srv := newFlakyServer(t, 0, http.StatusBadGateway)
			require.NoError(t, start(srv.reporting(t, fastRetry(3))))

			seen := srv.seen()
			require.Len(t, seen, 3)
			id := bodyUUID(t, seen[0].body)
			assert.Regexp(t, uuidV4, id)
			for _, rq := range seen {
				assert.Equal(t, http.MethodPost, rq.method)
				assert.Equal(t, seen[0].body, rq.body, "every attempt must carry the same UUID")
			}

			// without retries nothing is added to the request
			srv = newFlakyServer(t)
			require.NoError(t, start(srv.reporting(t)))
			assert.Empty(t, bodyUUID(t, srv.seen()[0].body))
		})
	}
}

func TestRetryStartKeepsCallersUUID(t *testing.T) {
	t.Parallel()

	const id = "7b1c0d5e-3f3a-4a57-9a51-0d4a54e3c9b1"
	srv := newFlakyServer(t, http.StatusServiceUnavailable)
	client := srv.reporting(t, fastRetry(1))

	_, err := client.StartLaunch(t.Context(), &openapi.StartLaunchRQ{Name: "n", Uuid: id})
	require.NoError(t, err)
	seen := srv.seen()
	require.Len(t, seen, 2)
	assert.Equal(t, id, bodyUUID(t, seen[0].body))
	assert.Equal(t, id, bodyUUID(t, seen[1].body))
}

// What the server can't recognize as a copy is sent once
func TestRetryNotForUnsafeRequests(t *testing.T) {
	t.Parallel()

	calls := map[string]func(c *ReportingClient) error{
		"rerun": func(c *ReportingClient) error {
			_, err := c.StartLaunch(
				t.Context(),
				&openapi.StartLaunchRQ{Name: "n", Rerun: openapi.PtrBool(true)},
			)
			return err
		},
		"raw launch without uuid": func(c *ReportingClient) error {
			_, err := c.StartLaunchRaw(t.Context(), json.RawMessage(`{"name":"n"}`))
			return err
		},
		"raw launch with empty uuid": func(c *ReportingClient) error {
			_, err := c.StartLaunchRaw(t.Context(), json.RawMessage(`{"name":"n","uuid":""}`))
			return err
		},
		"raw rerun with uuid": func(c *ReportingClient) error {
			_, err := c.StartLaunchRaw(
				t.Context(),
				json.RawMessage(`{"name":"n","uuid":"u","rerun":true}`),
			)
			return err
		},
		"raw item without uuid": func(c *ReportingClient) error {
			_, err := c.StartTestRaw(t.Context(), json.RawMessage(`{"name":"n"}`))
			return err
		},
		"raw child item, uuid is not a string": func(c *ReportingClient) error {
			_, err := c.StartChildTestRaw(
				t.Context(),
				"parent",
				json.RawMessage(`{"name":"n","uuid":7}`),
			)
			return err
		},
		"log": func(c *ReportingClient) error {
			_, err := c.SaveLog(t.Context(), &openapi.SaveLogRQ{Message: openapi.PtrString("m")})
			return err
		},
		"log batch": func(c *ReportingClient) error {
			_, err := c.SaveLogs(t.Context(), &openapi.SaveLogRQ{Message: openapi.PtrString("m")})
			return err
		},
		"log with attachment": func(c *ReportingClient) error {
			_, err := c.SaveLogMultipart(
				t.Context(),
				[]*openapi.SaveLogRQ{{Message: openapi.PtrString("m")}},
				[]Multipart{
					&ReaderMultipart{
						ContentType: "text/plain",
						FileName:    "a.txt",
						Reader:      strings.NewReader("a"),
					},
				},
			)
			return err
		},
	}
	for name, call := range calls {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			srv := newFlakyServer(t, http.StatusServiceUnavailable)
			err := call(srv.reporting(t, fastRetry(3)))

			var httpErr *HTTPError
			require.ErrorAs(t, err, &httpErr)
			assert.Equal(t, http.StatusServiceUnavailable, httpErr.StatusCode)
			assert.Len(t, srv.seen(), 1)
		})
	}

	t.Run("merge", func(t *testing.T) {
		t.Parallel()

		srv := newFlakyServer(t, http.StatusServiceUnavailable)
		_, err := srv.client(t, fastRetry(3)).
			MergeLaunches(t.Context(), "prj", &openapi.MergeLaunchesRQ{})
		require.Error(t, err)
		assert.Len(t, srv.seen(), 1)
	})
}

func TestRetryRawStartWithUUID(t *testing.T) {
	t.Parallel()

	srv := newFlakyServer(t, http.StatusServiceUnavailable)
	const body = `{"name":"n","uuid":"7b1c0d5e-3f3a-4a57-9a51-0d4a54e3c9b1"}`

	_, err := srv.reporting(t, fastRetry(3)).StartTestRaw(t.Context(), json.RawMessage(body))
	require.NoError(t, err)
	seen := srv.seen()
	require.Len(t, seen, 2)
	assert.JSONEq(t, body, seen[0].body)
	assert.Equal(t, seen[0].body, seen[1].body)
}

func TestRetryNetworkError(t *testing.T) {
	t.Parallel()

	// hand-written method
	srv := newFlakyServer(t, 0, 0)
	_, err := srv.reporting(t, fastRetry(2)).
		FinishLaunch(t.Context(), "l1", &openapi.FinishExecutionRQ{})
	require.NoError(t, err)
	assert.Len(t, srv.seen(), 3)

	// generated client
	srv = newFlakyServer(t, 0, 0)
	_, _, err = srv.client(t, fastRetry(2)).
		LaunchAPI.GetProjectLaunches(t.Context(), "prj").
		Execute()
	require.NoError(t, err)
	assert.Len(t, srv.seen(), 3)

	// nobody listens: every attempt fails, the error is the network one
	srv = newFlakyServer(t)
	srv.Close()
	_, err = srv.reporting(t, fastRetry(2)).
		FinishLaunch(t.Context(), "l1", &openapi.FinishExecutionRQ{})
	require.Error(t, err)
	var httpErr *HTTPError
	assert.NotErrorAs(t, err, &httpErr)
}

func TestRetryStopsWhenContextIsDone(t *testing.T) {
	t.Parallel()

	slow := WithRetry(RetryPolicy{MaxRetries: 5, MinWait: time.Minute, MaxWait: time.Minute})
	calls := map[string]func(ctx context.Context, srv *flakyServer) error{
		"resty": func(ctx context.Context, srv *flakyServer) error {
			_, err := srv.reporting(t, slow).FinishLaunch(ctx, "l1", &openapi.FinishExecutionRQ{})
			return err
		},
		"generated": func(ctx context.Context, srv *flakyServer) error {
			_, _, err := srv.client(t, slow).LaunchAPI.GetProjectLaunches(ctx, "prj").Execute()
			return err
		},
	}
	for name, call := range calls {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			srv := newFlakyServer(t, http.StatusServiceUnavailable, http.StatusServiceUnavailable)
			ctx, cancel := context.WithCancel(t.Context())
			time.AfterFunc(100*time.Millisecond, cancel)

			started := time.Now()
			err := call(ctx, srv)

			require.ErrorIs(t, err, context.Canceled)
			assert.Less(
				t,
				time.Since(started),
				10*time.Second,
				"the pause must end with the context",
			)
			assert.Len(t, srv.seen(), 1)
		})
	}
}

func TestRetryAfterHeader(t *testing.T) {
	t.Parallel()

	calls := map[string]func(srv *flakyServer, opt Option) error{
		"resty": func(srv *flakyServer, opt Option) error {
			_, err := srv.reporting(t, opt).
				FinishLaunch(t.Context(), "l1", &openapi.FinishExecutionRQ{})
			return err
		},
		"generated": func(srv *flakyServer, opt Option) error {
			_, _, err := srv.client(t, opt).
				LaunchAPI.GetProjectLaunches(t.Context(), "prj").
				Execute()
			return err
		},
	}
	// 503 is one of the two statuses Resty reads the header for itself, 500 is left to the policy
	for _, status := range []int{http.StatusServiceUnavailable, http.StatusInternalServerError} {
		for name, call := range calls {
			t.Run(fmt.Sprintf("%s %d honored", name, status), func(t *testing.T) {
				t.Parallel()

				srv := newFlakyServer(t, status)
				srv.header.Set("Retry-After", "1")
				started := time.Now()
				err := call(
					srv,
					WithRetry(
						RetryPolicy{MaxRetries: 1, MinWait: time.Millisecond, MaxWait: time.Minute},
					),
				)

				require.NoError(t, err)
				assert.GreaterOrEqual(t, time.Since(started), time.Second)
				assert.Len(t, srv.seen(), 2)
			})
			t.Run(fmt.Sprintf("%s %d limited by MaxWait", name, status), func(t *testing.T) {
				t.Parallel()

				srv := newFlakyServer(t, status)
				srv.header.Set("Retry-After", "3600")
				started := time.Now()
				err := call(
					srv,
					WithRetry(
						RetryPolicy{MaxRetries: 1, MinWait: time.Millisecond, MaxWait: time.Second},
					),
				)

				require.NoError(t, err)
				assert.Less(t, time.Since(started), 30*time.Second)
				assert.Len(t, srv.seen(), 2)
			})
		}
	}
}

func TestRetryTransportReplaysBody(t *testing.T) {
	t.Parallel()

	srv := newFlakyServer(t, http.StatusServiceUnavailable, 0)
	policy := RetryPolicy{MaxRetries: 3, MinWait: time.Millisecond}.normalized()
	client := policy.httpClient(&http.Client{})

	rq, err := http.NewRequestWithContext(
		t.Context(),
		http.MethodPut,
		srv.URL+"/x",
		bytes.NewBufferString(`{"a":1}`),
	)
	require.NoError(t, err)
	rs, err := client.Do(rq)
	require.NoError(t, err)
	defer rs.Body.Close()

	assert.Equal(t, http.StatusOK, rs.StatusCode)
	seen := srv.seen()
	require.Len(t, seen, 3)
	for _, s := range seen {
		assert.Equal(t, `{"a":1}`, s.body)
	}
}

func TestRetryTransportSendsOnceWhatItCannotReplay(t *testing.T) {
	t.Parallel()

	policy := RetryPolicy{MaxRetries: 3, MinWait: time.Millisecond}.normalized()
	requests := map[string]func(url string) (*http.Request, error){
		"POST": func(url string) (*http.Request, error) {
			return http.NewRequestWithContext(
				t.Context(),
				http.MethodPost,
				url,
				strings.NewReader("{}"),
			)
		},
		"PATCH": func(url string) (*http.Request, error) {
			return http.NewRequestWithContext(
				t.Context(),
				http.MethodPatch,
				url,
				strings.NewReader("{}"),
			)
		},
		"PUT with a body that can be read once": func(url string) (*http.Request, error) {
			return http.NewRequestWithContext(
				t.Context(),
				http.MethodPut,
				url,
				io.NopCloser(strings.NewReader("{}")),
			)
		},
	}
	for name, newRequest := range requests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			srv := newFlakyServer(t, http.StatusServiceUnavailable)
			rq, err := newRequest(srv.URL)
			require.NoError(t, err)
			rs, err := policy.httpClient(&http.Client{}).Do(rq)
			require.NoError(t, err)
			defer rs.Body.Close()

			assert.Equal(t, http.StatusServiceUnavailable, rs.StatusCode)
			assert.Len(t, srv.seen(), 1)
		})
	}
}

// The clients share one http.Client: the retrying transport must go to a copy for the generated client
func TestRetryHTTPClientIsCopied(t *testing.T) {
	t.Parallel()

	original := &http.Client{Timeout: time.Second}
	policy := RetryPolicy{MaxRetries: 1}.normalized()

	retrying := policy.httpClient(original)
	assert.NotSame(t, original, retrying)
	assert.Nil(t, original.Transport)
	assert.Equal(t, time.Second, retrying.Timeout)
	assert.IsType(t, &retryTransport{}, retrying.Transport)

	assert.Same(t, original, RetryPolicy{}.httpClient(original))
}

func TestRetryWait(t *testing.T) {
	t.Parallel()

	policy := RetryPolicy{
		MaxRetries: 10,
		MinWait:    100 * time.Millisecond,
		MaxWait:    time.Second,
	}.normalized()
	ceilings := []time.Duration{
		100 * time.Millisecond,
		200 * time.Millisecond,
		400 * time.Millisecond,
		800 * time.Millisecond,
		time.Second,
		time.Second,
		time.Second,
	}
	for i, ceiling := range ceilings {
		seen := map[time.Duration]bool{}
		for range 200 {
			d := policy.wait(i+1, nil)
			require.GreaterOrEqual(t, d, ceiling/2, "attempt %d", i+1)
			require.LessOrEqual(t, d, ceiling, "attempt %d", i+1)
			seen[d] = true
		}
		assert.Greater(t, len(seen), 100, "the pause must be random")
	}
	// far attempts don't overflow
	assert.LessOrEqual(t, policy.wait(5000, nil), time.Second)

	withHeader := func(v string) *http.Response {
		return &http.Response{Header: http.Header{"Retry-After": []string{v}}}
	}
	assert.Equal(
		t,
		300*time.Millisecond,
		RetryPolicy{
			MinWait: time.Millisecond,
			MaxWait: 300 * time.Millisecond,
		}.normalized().
			wait(1, withHeader("2")),
	)
	assert.Equal(
		t,
		2*time.Second,
		RetryPolicy{MaxWait: time.Minute}.normalized().wait(1, withHeader("2")),
	)
	assert.Equal(t, time.Duration(0), policy.wait(1, withHeader("0")))
	// not a valid header: the usual pause
	assert.GreaterOrEqual(t, policy.wait(1, withHeader("soon")), 50*time.Millisecond)

	defaults := RetryPolicy{MaxRetries: 1}.normalized()
	assert.Equal(t, 500*time.Millisecond, defaults.MinWait)
	assert.Equal(t, 10*time.Second, defaults.MaxWait)
	assert.Equal(
		t,
		time.Minute,
		RetryPolicy{MinWait: time.Minute, MaxWait: time.Second}.normalized().MaxWait,
	)
}

func TestParseRetryAfter(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 11, 27, 10, 0, 0, 0, time.UTC)
	tests := []struct {
		in   string
		want time.Duration
		ok   bool
	}{
		{in: "", ok: false},
		{in: "abc", ok: false},
		{in: "-1", ok: false},
		{in: "0", want: 0, ok: true},
		{in: " 120 ", want: 2 * time.Minute, ok: true},
		{in: "99999999999999999", want: time.Duration(1<<63 - 1), ok: true},
		{in: "Fri, 27 Nov 2026 10:00:30 GMT", want: 30 * time.Second, ok: true},
		{in: "Fri, 27 Nov 2026 09:00:00 GMT", want: 0, ok: true},
	}
	for _, tt := range tests {
		got, ok := parseRetryAfter(tt.in, now)
		assert.Equal(t, tt.ok, ok, tt.in)
		assert.Equal(t, tt.want, got, tt.in)
	}
}

func TestRetryableError(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	done, cancel := context.WithCancel(ctx)
	cancel()

	assert.False(t, retryableError(ctx, nil))
	assert.True(t, retryableError(ctx, io.ErrUnexpectedEOF))
	assert.True(
		t,
		retryableError(
			ctx,
			&url.Error{Op: "Get", URL: "http://x", Err: errors.New("connection refused")},
		),
	)
	// the timeout of http.Client, the caller's context is still alive
	assert.True(
		t,
		retryableError(ctx, &url.Error{Op: "Get", URL: "http://x", Err: context.DeadlineExceeded}),
	)

	assert.False(t, retryableError(done, io.ErrUnexpectedEOF))
	assert.False(t, retryableError(ctx, fmt.Errorf("wrapped: %w", context.Canceled)))
	assert.False(
		t,
		retryableError(
			ctx,
			&url.Error{Err: &tls.CertificateVerificationError{Err: errors.New("bad")}},
		),
	)
	assert.False(t, retryableError(ctx, &url.Error{Err: x509.UnknownAuthorityError{}}))
	assert.False(
		t,
		retryableError(
			ctx,
			&url.Error{Err: x509.HostnameError{Certificate: &x509.Certificate{}, Host: "h"}},
		),
	)
	assert.False(
		t,
		retryableError(ctx, &url.Error{Err: errors.New(`unsupported protocol scheme "ftp"`)}),
	)
	assert.False(t, retryableError(ctx, &url.Error{Err: errors.New("stopped after 10 redirects")}))

	// the token endpoint refused the credentials / is down
	assert.False(
		t,
		retryableError(
			ctx,
			&oauth2.RetrieveError{Response: &http.Response{StatusCode: http.StatusBadRequest}},
		),
	)
	assert.True(
		t,
		retryableError(
			ctx,
			&oauth2.RetrieveError{Response: &http.Response{StatusCode: http.StatusBadGateway}},
		),
	)
}

func TestNewUUID(t *testing.T) {
	t.Parallel()

	seen := map[string]bool{}
	for range 1000 {
		id := newUUID()
		require.Regexp(t, uuidV4, id)
		require.False(t, seen[id])
		seen[id] = true
	}
}
