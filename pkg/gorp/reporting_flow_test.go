package gorp

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"

	"github.com/reportportal/goRP/v5/pkg/openapi"
)

type recordedRequest struct {
	method, path, body string
}

// recordingServer answers every request with a JSON body that satisfies all reporting response types.
func recordingServer(t *testing.T) (*httptest.Server, func() recordedRequest) {
	t.Helper()

	var (
		mu   sync.Mutex
		last recordedRequest
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		last = recordedRequest{method: r.Method, path: r.URL.Path, body: string(body)}
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"generated-id","message":"ok","number":7}`))
	}))
	t.Cleanup(srv.Close)

	return srv, func() recordedRequest {
		mu.Lock()
		defer mu.Unlock()

		return last
	}
}

func TestReportingClient_RequestShapes(t *testing.T) {
	t.Parallel()

	srv, lastRequest := recordingServer(t)
	client := NewReportingClient(srv.URL, "prj", WithApiKeyAuth(t.Context(), "key"))
	raw := json.RawMessage(`{"raw":"body"}`)
	now := time.Now()

	tests := []struct {
		name       string
		call       func(ctx context.Context) error
		wantMethod string
		wantPath   string
		wantBody   string // substring; the raw variants must forward their body untouched
	}{
		{
			name: "start launch",
			call: func(ctx context.Context) error {
				_, err := client.StartLaunch(ctx, &openapi.StartLaunchRQ{Name: "l", StartTime: now})
				return err
			},
			wantMethod: http.MethodPost, wantPath: "/api/v2/prj/launch", wantBody: `"name":"l"`,
		},
		{
			name: "start launch raw",
			call: func(ctx context.Context) error {
				_, err := client.StartLaunchRaw(ctx, raw)
				return err
			},
			wantMethod: http.MethodPost, wantPath: "/api/v2/prj/launch", wantBody: `{"raw":"body"}`,
		},
		{
			name: "finish launch",
			call: func(ctx context.Context) error {
				_, err := client.FinishLaunch(ctx, "L1", &openapi.FinishExecutionRQ{
					EndTime: now, Status: Statuses.Passed.Ptr(),
				})
				return err
			},
			wantMethod: http.MethodPut,
			wantPath:   "/api/v2/prj/launch/L1/finish",
			wantBody:   `"status":"PASSED"`,
		},
		{
			name: "finish launch raw",
			call: func(ctx context.Context) error {
				_, err := client.FinishLaunchRaw(ctx, "L1", raw)
				return err
			},
			wantMethod: http.MethodPut,
			wantPath:   "/api/v2/prj/launch/L1/finish",
			wantBody:   `{"raw":"body"}`,
		},
		{
			name: "stop launch",
			call: func(ctx context.Context) error {
				_, err := client.StopLaunch(ctx, "L1")
				return err
			},
			wantMethod: http.MethodPut,
			wantPath:   "/api/v2/prj/launch/L1/finish",
			wantBody:   `"status":"STOPPED"`,
		},
		{
			name: "start test",
			call: func(ctx context.Context) error {
				_, err := client.StartTest(
					ctx,
					&openapi.StartTestItemRQ{Name: "t", Type: "TEST", StartTime: now},
				)
				return err
			},
			wantMethod: http.MethodPost, wantPath: "/api/v2/prj/item/", wantBody: `"name":"t"`,
		},
		{
			name: "start test raw",
			call: func(ctx context.Context) error {
				_, err := client.StartTestRaw(ctx, raw)
				return err
			},
			wantMethod: http.MethodPost, wantPath: "/api/v2/prj/item/", wantBody: `{"raw":"body"}`,
		},
		{
			name: "start child test",
			call: func(ctx context.Context) error {
				_, err := client.StartChildTest(
					ctx,
					"P1",
					&openapi.StartTestItemRQ{Name: "c", Type: "STEP", StartTime: now},
				)
				return err
			},
			wantMethod: http.MethodPost, wantPath: "/api/v2/prj/item/P1", wantBody: `"name":"c"`,
		},
		{
			name: "start child test raw",
			call: func(ctx context.Context) error {
				_, err := client.StartChildTestRaw(ctx, "P1", raw)
				return err
			},
			wantMethod: http.MethodPost,
			wantPath:   "/api/v2/prj/item/P1",
			wantBody:   `{"raw":"body"}`,
		},
		{
			name: "finish test",
			call: func(ctx context.Context) error {
				_, err := client.FinishTest(ctx, "T1", &openapi.FinishTestItemRQ{
					EndTime: now, Status: Statuses.Failed.Ptr(),
				})
				return err
			},
			wantMethod: http.MethodPut,
			wantPath:   "/api/v2/prj/item/T1",
			wantBody:   `"status":"FAILED"`,
		},
		{
			name: "finish test raw",
			call: func(ctx context.Context) error {
				_, err := client.FinishTestRaw(ctx, "T1", raw)
				return err
			},
			wantMethod: http.MethodPut, wantPath: "/api/v2/prj/item/T1", wantBody: `{"raw":"body"}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// not parallel: the server records only the last request
			require.NoError(t, tt.call(t.Context()))

			got := lastRequest()
			assert.Equal(t, tt.wantMethod, got.method)
			assert.Equal(t, tt.wantPath, got.path)
			assert.Contains(t, got.body, tt.wantBody)
		})
	}
}

func TestReportingClient_ResponseIsDecoded(t *testing.T) {
	t.Parallel()

	srv, _ := recordingServer(t)
	client := NewReportingClient(srv.URL, "prj", WithApiKeyAuth(t.Context(), "key"))

	rs, err := client.StartTest(
		t.Context(),
		&openapi.StartTestItemRQ{Name: "t", Type: "TEST", StartTime: time.Now()},
	)

	require.NoError(t, err)
	assert.Equal(t, "generated-id", rs.GetId())
}

func TestReportingClient_HTTPErrorCarriesStatusAndBody(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "launch not found", http.StatusNotFound)
	}))
	t.Cleanup(srv.Close)
	client := NewReportingClient(srv.URL, "prj", WithApiKeyAuth(t.Context(), "key"))

	_, err := client.StopLaunch(t.Context(), "missing")

	var httpErr *HTTPError
	require.ErrorAs(t, err, &httpErr)
	assert.Equal(t, http.StatusNotFound, httpErr.StatusCode)
	assert.Contains(t, err.Error(), "404")
	assert.Contains(t, err.Error(), "launch not found")
}

func TestWithPasswordOwnerGrantAuth(t *testing.T) {
	t.Parallel()

	var (
		mu         sync.Mutex
		tokenForms []url.Values
	)
	oauthSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		mu.Lock()
		tokenForms = append(tokenForms, r.PostForm)
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(
			[]byte(`{"access_token":"issued-token","token_type":"Bearer","expires_in":3600}`),
		)
	}))
	t.Cleanup(oauthSrv.Close)

	var gotAuth []string
	var authMu sync.Mutex
	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authMu.Lock()
		gotAuth = append(gotAuth, r.Header.Get("Authorization"))
		authMu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"x"}`))
	}))
	t.Cleanup(apiSrv.Close)

	cfg := &oauth2.Config{
		ClientID:     "ui",
		ClientSecret: "uiman",
		Endpoint:     oauth2.Endpoint{TokenURL: oauthSrv.URL + "/uat/sso/oauth/token"},
	}
	client := NewReportingClient(
		apiSrv.URL,
		"prj",
		WithPasswordOwnerGrantAuth(t.Context(), cfg, "user", "secret"),
	)

	for range 2 {
		_, err := client.StartLaunch(
			t.Context(),
			&openapi.StartLaunchRQ{Name: "l", StartTime: time.Now()},
		)
		require.NoError(t, err)
	}

	authMu.Lock()
	defer authMu.Unlock()
	assert.Equal(t, []string{"Bearer issued-token", "Bearer issued-token"}, gotAuth)
	mu.Lock()
	defer mu.Unlock()
	require.Len(t, tokenForms, 1, "the token must be reused while it is valid")
	assert.Equal(t, "password", tokenForms[0].Get("grant_type"))
	assert.Equal(t, "user", tokenForms[0].Get("username"))
	assert.Equal(t, "secret", tokenForms[0].Get("password"))
}

func TestReaderMultipart_Load(t *testing.T) {
	t.Parallel()

	t.Run("returns its metadata and content", func(t *testing.T) {
		t.Parallel()
		m := &ReaderMultipart{
			FileName:    "a.txt",
			ContentType: "text/plain",
			Reader:      strings.NewReader("data"),
		}

		name, contentType, reader, err := m.Load()

		require.NoError(t, err)
		assert.Equal(t, "a.txt", name)
		assert.Equal(t, "text/plain", contentType)
		content, _ := io.ReadAll(reader)
		assert.Equal(t, "data", string(content))
	})

	t.Run("requires a file name", func(t *testing.T) {
		t.Parallel()
		m := &ReaderMultipart{Reader: strings.NewReader("data")}

		_, _, _, err := m.Load()

		require.Error(t, err)
		assert.True(t, errors.Is(err, errMultipartFilename))
	})
}

func TestGetLaunchesByFilterName(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/prj/filter":
			assert.Equal(t, "saved", r.URL.Query().Get("filter.eq.name"))
			_, _ = w.Write([]byte(`{"content":[{"id":1,"name":"saved","owner":"me","type":"Launch",
				"conditions":[{"filteringField":"name","condition":"cnt","value":"smoke"}],
				"orders":[{"sortingColumn":"startTime","isAsc":false}]}],
				"page":{"number":1,"size":1,"totalElements":1,"totalPages":1}}`))
		case "/api/v1/prj/launch":
			assert.Equal(t, "smoke", r.URL.Query().Get("filter.cnt.name"))
			assert.Equal(t, "startTime,DESC", r.URL.Query().Get("page.sort"))
			_, _ = w.Write([]byte(`{"content":[{"id":5,"uuid":"u","name":"smoke run","number":3,
				"startTime":"2025-01-01T00:00:00Z"}],"page":{"number":1,"totalPages":1}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	u, _ := url.Parse(srv.URL)
	client := NewClient(u, WithApiKeyAuth(t.Context(), "key"))

	launches, err := client.GetLaunchesByFilterName(t.Context(), "prj", "saved")

	require.NoError(t, err)
	require.Len(t, launches.Content, 1)
	assert.Equal(t, int64(5), launches.Content[0].Id)
}

func TestGetLaunchesByFilterName_UnknownFilter(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(
			[]byte(`{"content":[],"page":{"number":1,"size":0,"totalElements":0,"totalPages":0}}`),
		)
	}))
	t.Cleanup(srv.Close)
	u, _ := url.Parse(srv.URL)
	client := NewClient(u, WithApiKeyAuth(t.Context(), "key"))

	_, err := client.GetLaunchesByFilterName(t.Context(), "prj", "nope")

	require.ErrorContains(t, err, "no filter nope found")
}

func TestSaveLogMultipart_BatchResponse(t *testing.T) {
	t.Parallel()

	// with attachments the server answers with a batch instead of a single entry
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"responses":[{"id":"batch-log-id"}]}`))
	}))
	t.Cleanup(srv.Close)
	client := NewReportingClient(srv.URL, "prj", WithApiKeyAuth(t.Context(), "key"))

	rs, err := client.SaveLogMultipart(
		t.Context(),
		[]*openapi.SaveLogRQ{
			{
				Message: openapi.PtrString("m"),
				Time:    time.Now(),
				File:    &openapi.File{Name: openapi.PtrString("a.txt")},
			},
		},
		[]Multipart{
			&ReaderMultipart{
				FileName:    "a.txt",
				ContentType: "text/plain",
				Reader:      strings.NewReader("x"),
			},
		},
	)

	require.NoError(t, err)
	assert.Equal(t, "batch-log-id", rs.GetId())
}

func TestMergeLaunches_ServerSendsEpochTimestamps(t *testing.T) {
	t.Parallel()

	var gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v2/prj/launch/merge", r.URL.Path)
		body, _ := io.ReadAll(r.Body)
		gotBody = string(body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(
			[]byte(
				`{"id":42,"uuid":"u-42","name":"merged","number":1,"startTime":1794824541063,"endTime":1794824541885}`,
			),
		)
	}))
	t.Cleanup(srv.Close)
	u, _ := url.Parse(srv.URL)
	client := NewClient(u, WithApiKeyAuth(t.Context(), "key"))

	merged, err := client.MergeLaunches(t.Context(), "prj", &openapi.MergeLaunchesRQ{
		Name: "merged", MergeType: string(MergeTypes.Basic), Launches: []int64{1, 2},
	})

	require.NoError(t, err)
	assert.Equal(t, MergedLaunch{ID: 42, UUID: "u-42", Name: "merged"}, *merged)
	assert.Contains(t, gotBody, `"launches":[1,2]`)
}
