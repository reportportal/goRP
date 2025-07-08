package gorp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
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

	// Create a context that will be cancelled
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
