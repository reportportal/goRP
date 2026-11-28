package gorp

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"

	"github.com/reportportal/goRP/v5/pkg/openapi"
)

func TestNewPasswordGrantTokenSource(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	config := &oauth2.Config{}
	username := "testuser"
	password := "testpass"

	tokenSource := NewPasswordGrantTokenSource(ctx, config, username, password)

	// Verify the returned type is a ReuseTokenSource (not passwordGrantTokenSource directly)
	assert.NotNil(t, tokenSource, "Expected non-nil token source")

	// Test that it implements oauth2.TokenSource interface
	var _ oauth2.TokenSource = tokenSource //nolint:staticcheck
}

func TestPasswordGrantTokenSource_Token_Success(t *testing.T) {
	t.Parallel()

	// Create a test server that returns a valid token response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "application/x-www-form-urlencoded", r.Header.Get("Content-Type"))

		// Parse form to verify credentials are sent
		err := r.ParseForm()
		require.NoError(t, err)
		assert.Equal(t, "password", r.Form.Get("grant_type"))
		assert.Equal(t, "testuser", r.Form.Get("username"))
		assert.Equal(t, "testpass", r.Form.Get("password"))

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(
			[]byte(`{"access_token":"new-token","token_type":"Bearer","expires_in":3600}`),
		)
	}))
	defer server.Close()

	ctx := context.Background()
	config := &oauth2.Config{
		Endpoint: oauth2.Endpoint{
			TokenURL: server.URL,
		},
	}

	tokenSource := NewPasswordGrantTokenSource(ctx, config, "testuser", "testpass")

	token, err := tokenSource.Token()
	require.NoError(t, err, "Expected no error")
	assert.Equal(t, "new-token", token.AccessToken, "Expected correct access token")
	assert.Equal(t, "Bearer", token.TokenType, "Expected Bearer token type")
	assert.True(t, token.Expiry.After(time.Now()), "Token should not be expired")
}

func TestPasswordGrantTokenSource_Token_ReusesCachedToken(t *testing.T) {
	t.Parallel()

	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(
			[]byte(`{"access_token":"cached-token","token_type":"Bearer","expires_in":3600}`),
		)
	}))
	defer server.Close()

	ctx := context.Background()
	config := &oauth2.Config{
		Endpoint: oauth2.Endpoint{
			TokenURL: server.URL,
		},
	}

	tokenSource := NewPasswordGrantTokenSource(ctx, config, "testuser", "testpass")

	// First call should fetch token
	token1, err := tokenSource.Token()
	require.NoError(t, err, "Expected no error on first call")
	assert.Equal(t, "cached-token", token1.AccessToken, "Expected correct access token")

	// Second call should reuse cached token
	token2, err := tokenSource.Token()
	require.NoError(t, err, "Expected no error on second call")
	assert.Equal(t, "cached-token", token2.AccessToken, "Expected same access token")

	// Should have only made one request due to caching
	assert.Equal(t, 1, requestCount, "Expected only one request due to token reuse")
}

func TestPasswordGrantTokenSource_Token_RefreshesExpiredToken(t *testing.T) {
	t.Parallel()

	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		var response string
		if requestCount == 1 {
			// First request returns token that expires quickly
			response = `{"access_token":"short-lived-token","token_type":"Bearer","expires_in":1}`
		} else {
			// Second request returns new token
			response = `{"access_token":"refreshed-token","token_type":"Bearer","expires_in":3600}`
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(response))
	}))
	defer server.Close()

	ctx := context.Background()
	config := &oauth2.Config{
		Endpoint: oauth2.Endpoint{
			TokenURL: server.URL,
		},
	}

	tokenSource := NewPasswordGrantTokenSource(ctx, config, "testuser", "testpass")

	// First call gets short-lived token
	token1, err := tokenSource.Token()
	require.NoError(t, err, "Expected no error on first call")
	assert.Equal(t, "short-lived-token", token1.AccessToken, "Expected short-lived token")

	// Wait for token to expire
	time.Sleep(2 * time.Second)

	// Second call should fetch new token
	token2, err := tokenSource.Token()
	require.NoError(t, err, "Expected no error on second call")
	assert.Equal(t, "refreshed-token", token2.AccessToken, "Expected refreshed token")

	// Should have made two requests
	assert.Equal(t, 2, requestCount, "Expected two requests due to token refresh")
}

func TestPasswordGrantTokenSource_Token_Error(t *testing.T) {
	t.Parallel()

	// Create a test server that returns an error response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write(
			[]byte(`{"error":"invalid_grant","error_description":"Invalid credentials"}`),
		)
	}))
	defer server.Close()

	ctx := context.Background()
	config := &oauth2.Config{
		Endpoint: oauth2.Endpoint{
			TokenURL: server.URL,
		},
	}

	tokenSource := NewPasswordGrantTokenSource(ctx, config, "testuser", "wrongpass")

	token, err := tokenSource.Token()
	require.Error(t, err, "Expected an error")
	assert.Nil(t, token, "Expected nil token on error")
	assert.Contains(
		t,
		err.Error(),
		"failed to fetch token",
		"Expected error to contain 'failed to fetch token'",
	)
}

func TestPasswordGrantTokenSource_Token_NetworkError(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// Use an invalid URL to simulate network error
	config := &oauth2.Config{
		Endpoint: oauth2.Endpoint{
			TokenURL: "http://invalid-host:9999/token",
		},
	}

	tokenSource := NewPasswordGrantTokenSource(ctx, config, "testuser", "testpass")

	token, err := tokenSource.Token()
	require.Error(t, err, "Expected an error")
	assert.Nil(t, token, "Expected nil token on error")
	assert.Contains(
		t,
		err.Error(),
		"failed to fetch token",
		"Expected error to contain 'failed to fetch token'",
	)
}

func TestPasswordGrantTokenSource_Token_InvalidResponse(t *testing.T) {
	t.Parallel()

	// Create a test server that returns invalid JSON
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"invalid":"json"}`)) // Missing required fields
	}))
	defer server.Close()

	ctx := context.Background()
	config := &oauth2.Config{
		Endpoint: oauth2.Endpoint{
			TokenURL: server.URL,
		},
	}

	tokenSource := NewPasswordGrantTokenSource(ctx, config, "testuser", "testpass")

	token, err := tokenSource.Token()
	require.Error(t, err, "Expected an error")
	assert.Nil(t, token, "Expected nil token on error")
}

func TestPasswordGrantTokenSource_Token_ContextCancellation(t *testing.T) {
	t.Parallel()

	// Create a test server with delay
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(
			[]byte(`{"access_token":"new-token","token_type":"Bearer","expires_in":3600}`),
		)
	}))
	defer server.Close()

	// Create a context that will be canceled
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	config := &oauth2.Config{
		Endpoint: oauth2.Endpoint{
			TokenURL: server.URL,
		},
	}

	tokenSource := NewPasswordGrantTokenSource(ctx, config, "testuser", "testpass")

	token, err := tokenSource.Token()
	require.Error(t, err, "Expected an error")
	assert.Nil(t, token, "Expected nil token on error")
	assert.Contains(
		t,
		err.Error(),
		"failed to fetch token",
		"Expected error to contain 'failed to fetch token'",
	)
}

func TestPasswordGrantTokenSource_Token_ConcurrentAccess(t *testing.T) {
	t.Parallel()

	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(
			[]byte(`{"access_token":"concurrent-token","token_type":"Bearer","expires_in":3600}`),
		)
	}))
	defer server.Close()

	ctx := context.Background()
	config := &oauth2.Config{
		Endpoint: oauth2.Endpoint{
			TokenURL: server.URL,
		},
	}

	tokenSource := NewPasswordGrantTokenSource(ctx, config, "testuser", "testpass")

	// Run multiple goroutines to test concurrent access
	const numGoroutines = 10
	results := make(chan error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			_, err := tokenSource.Token()
			results <- err
		}()
	}

	// Collect results
	for i := 0; i < numGoroutines; i++ {
		err := <-results
		assert.NoError(t, err, "Expected no error from concurrent access")
	}

	// ReuseTokenSource should handle concurrent access properly
	// Should only make one request due to proper synchronization
	assert.Equal(t, 1, requestCount, "Expected only one request due to proper synchronization")
}

func TestPasswordGrantTokenSource_DirectAccess(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(
			[]byte(`{"access_token":"direct-token","token_type":"Bearer","expires_in":3600}`),
		)
	}))
	defer server.Close()

	ctx := context.Background()
	config := &oauth2.Config{
		Endpoint: oauth2.Endpoint{
			TokenURL: server.URL,
		},
	}

	// Create passwordGrantTokenSource directly for testing
	pgts := &passwordGrantTokenSource{
		ctx:    ctx,
		config: config,
		user:   "testuser",
		pass:   "testpass",
	}

	token, err := pgts.Token()
	require.NoError(t, err, "Expected no error")
	assert.Equal(t, "direct-token", token.AccessToken, "Expected correct access token")
	assert.Equal(t, "Bearer", token.TokenType, "Expected Bearer token type")
}

func TestPasswordGrantTokenSource_EmptyCredentials(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Parse form to verify empty credentials
		err := r.ParseForm()
		require.NoError(t, err)
		assert.Equal(t, "", r.Form.Get("username"))
		assert.Equal(t, "", r.Form.Get("password"))

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(
			[]byte(`{"access_token":"empty-creds-token","token_type":"Bearer","expires_in":3600}`),
		)
	}))
	defer server.Close()

	ctx := context.Background()
	config := &oauth2.Config{
		Endpoint: oauth2.Endpoint{
			TokenURL: server.URL,
		},
	}

	tokenSource := NewPasswordGrantTokenSource(ctx, config, "", "")

	token, err := tokenSource.Token()
	require.NoError(t, err, "Expected no error")
	assert.Equal(
		t,
		"empty-creds-token",
		token.AccessToken,
		"Expected token even with empty credentials",
	)
}

// A rejected token request of the password grant never reaches the API, so neither client's
// response handling sees it. It must still come out of both clients as an HTTPError, and stay
// reachable as the oauth2 package's own error.
func TestPasswordGrantTokenFailureIsHTTPError(t *testing.T) {
	t.Parallel()

	const tokenPath = "/uat/sso/oauth/token"
	tests := []struct {
		name        string
		status      int
		contentType string
		body        string
		wantOAuth   string
		wantMessage string
		wantCode    int64
		wantText    string
	}{
		{
			name:        "ReportPortal error, as ReportPortal 6 answers a wrong password",
			status:      http.StatusBadRequest,
			contentType: "application/json",
			body:        `{"errorCode":4003,"message":"You do not have enough permissions. Bad credentials"}`,
			wantMessage: "You do not have enough permissions. Bad credentials",
			wantCode:    4003,
			wantText:    ": 400 Bad Request: You do not have enough permissions. Bad credentials (error code 4003)",
		},
		{
			name:        "JSON error",
			status:      http.StatusBadRequest,
			contentType: "application/json",
			body:        `{"error":"invalid_grant","error_description":"Bad credentials"}`,
			wantOAuth:   "invalid_grant",
			wantMessage: "Bad credentials",
			wantText:    ": 400 Bad Request: invalid_grant: Bad credentials",
		},
		{
			name:        "form-encoded error",
			status:      http.StatusUnauthorized,
			contentType: "application/x-www-form-urlencoded",
			body:        "error=invalid_client&error_description=Unknown+client",
			wantOAuth:   "invalid_client",
			wantMessage: "Unknown client",
			wantText:    ": 401 Unauthorized: invalid_client: Unknown client",
		},
		{
			name:        "error with status 200",
			status:      http.StatusOK,
			contentType: "application/json",
			body:        `{"error":"invalid_grant","error_description":"Bad credentials"}`,
			wantOAuth:   "invalid_grant",
			wantMessage: "Bad credentials",
			wantText:    ": 200 OK: invalid_grant: Bad credentials",
		},
		{
			name:        "body that is not an OAuth error",
			status:      http.StatusBadGateway,
			contentType: "text/html",
			body:        "<html><body>Bad Gateway</body></html>",
			wantText:    ": 502 Bad Gateway: <html><body>Bad Gateway</body></html>",
		},
		{
			// the oauth2 package takes "error" of this body for an OAuth error code
			name:        "wrong token URL answered by Spring's default handler",
			status:      http.StatusNotFound,
			contentType: "application/json",
			body:        `{"timestamp":"2026-01-01T00:00:00.000+00:00","status":404,"error":"Not Found","path":"/uat/sso/oauth/token"}`,
			wantText:    ": 404 Not Found: no such endpoint, check the host and the path",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			srv := httptest.NewServer(
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.URL.Path != tokenPath {
						t.Errorf(
							"a request without a token reached the API: %s %s",
							r.Method,
							r.URL.Path,
						)
					}
					w.Header().Set("Content-Type", tt.contentType)
					w.WriteHeader(tt.status)
					w.Write([]byte(tt.body))
				}),
			)
			t.Cleanup(srv.Close)
			u, err := url.Parse(srv.URL)
			require.NoError(t, err)

			option := func() ClientOption {
				config := &oauth2.Config{
					ClientID: "ui",
					Endpoint: oauth2.Endpoint{TokenURL: srv.URL + tokenPath},
				}

				return WithPasswordOwnerGrantAuth(t.Context(), config, "user", "wrong")
			}
			calls := map[string]func() error{
				"generated client": func() error {
					_, _, cErr := NewClient(
						u,
						option(),
					).LaunchAPI.GetLaunch(t.Context(), "1", "prj").
						Execute()

					return cErr
				},
				"Resty Client": func() error {
					_, cErr := NewClient(
						u,
						option(),
					).GetLaunchesByFilterString(t.Context(), "prj", "page.size=1")

					return cErr
				},
				"Resty ReportingClient": func() error {
					_, cErr := NewReportingClient(srv.URL, "prj", option()).
						StartLaunch(t.Context(), &openapi.StartLaunchRQ{Name: "l"})

					return cErr
				},
			}

			for name, call := range calls {
				t.Run(name, func(t *testing.T) {
					t.Parallel()

					err := call()

					var httpErr *HTTPError
					require.ErrorAs(t, err, &httpErr)
					assert.Equal(t, tt.status, httpErr.StatusCode)
					assert.Equal(t, http.MethodPost, httpErr.Method)
					assert.Equal(t, srv.URL+tokenPath, httpErr.URL)
					assert.Equal(t, tt.body, httpErr.Response)
					assert.Equal(t, tt.wantOAuth, httpErr.OAuthError)
					assert.Equal(t, tt.wantMessage, httpErr.Message)
					assert.Equal(t, tt.wantCode, httpErr.ErrorCode)
					assert.Contains(
						t,
						err.Error(),
						"failed to fetch token: POST "+srv.URL+tokenPath+tt.wantText,
					)
					assert.NotErrorIs(t, err, ErrNotAPIResponse)
					assert.Equal(
						t,
						tt.status == http.StatusUnauthorized,
						errors.Is(err, ErrUnauthorized),
					)

					var retrieveErr *oauth2.RetrieveError
					require.ErrorAs(t, err, &retrieveErr)
					assert.Equal(t, tt.status, retrieveErr.Response.StatusCode)
					assert.Equal(t, tt.body, string(retrieveErr.Body))
					assert.Contains(t, retrieveErr.Error(), "oauth2: ")
					assert.Same(t, retrieveErr, errors.Unwrap(httpErr))
				})
			}
		})
	}
}
