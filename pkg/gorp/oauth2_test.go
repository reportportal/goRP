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

	// Verify the returned type
	pgts, ok := tokenSource.(*passwordGrantTokenSource)
	require.True(t, ok, "Expected passwordGrantTokenSource type")

	// Verify fields are set correctly
	assert.Equal(t, ctx, pgts.ctx, "Context not set correctly")
	assert.Equal(t, config, pgts.config, "Config not set correctly")
	assert.Equal(t, username, pgts.user, "Username not set correctly")
	assert.Equal(t, password, pgts.pass, "Password not set correctly")
	assert.Nil(t, pgts.token, "Token should be nil initially")
}

func TestPasswordGrantTokenSource_Token_ValidToken(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// Create a valid token (expires in 1 hour)
	validToken := &oauth2.Token{
		AccessToken: "valid-token",
		TokenType:   "Bearer",
		Expiry:      time.Now().Add(1 * time.Hour),
	}

	pgts := &passwordGrantTokenSource{
		ctx:    ctx,
		config: &oauth2.Config{},
		token:  validToken,
		user:   "testuser",
		pass:   "testpass",
	}

	token, err := pgts.Token()
	require.NoError(t, err, "Expected no error")
	assert.Equal(t, validToken, token, "Expected to return the existing valid token")
}

func TestPasswordGrantTokenSource_Token_ExpiredToken(t *testing.T) {
	t.Parallel()

	// Create a test server that returns a valid token response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "application/x-www-form-urlencoded", r.Header.Get("Content-Type"))

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(
			[]byte(`{"access_token":"new-token","token_type":"Bearer","expires_in":3600}`),
		)
	}))
	defer server.Close()

	ctx := context.Background()

	// Create an expired token
	expiredToken := &oauth2.Token{
		AccessToken: "expired-token",
		TokenType:   "Bearer",
		Expiry:      time.Now().Add(-1 * time.Hour),
	}

	config := &oauth2.Config{
		Endpoint: oauth2.Endpoint{
			TokenURL: server.URL,
		},
	}

	pgts := &passwordGrantTokenSource{
		ctx:    ctx,
		config: config,
		token:  expiredToken,
		user:   "testuser",
		pass:   "testpass",
	}

	token, err := pgts.Token()
	require.NoError(t, err, "Expected no error")
	assert.Equal(t, "new-token", token.AccessToken, "Expected to return the new token")
	assert.Equal(t, "Bearer", token.TokenType, "Expected Bearer token type")
	assert.True(t, token.Expiry.After(time.Now()), "Token should not be expired")

	// Verify the token was updated in the struct
	assert.Equal(t, "new-token", pgts.token.AccessToken, "Token should be updated in the struct")
}

func TestPasswordGrantTokenSource_Token_NoToken(t *testing.T) {
	t.Parallel()

	// Create a test server that returns a valid token response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)

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

	pgts := &passwordGrantTokenSource{
		ctx:    ctx,
		config: config,
		token:  nil, // No existing token
		user:   "testuser",
		pass:   "testpass",
	}

	token, err := pgts.Token()
	require.NoError(t, err, "Expected no error")
	assert.Equal(t, "new-token", token.AccessToken, "Expected to return the new token")
	assert.Equal(t, "Bearer", token.TokenType, "Expected Bearer token type")

	// Verify the token was set in the struct
	assert.Equal(t, "new-token", pgts.token.AccessToken, "Token should be set in the struct")
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

	pgts := &passwordGrantTokenSource{
		ctx:    ctx,
		config: config,
		token:  nil,
		user:   "testuser",
		pass:   "wrongpass",
	}

	token, err := pgts.Token()
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

	pgts := &passwordGrantTokenSource{
		ctx:    ctx,
		config: config,
		token:  nil,
		user:   "testuser",
		pass:   "testpass",
	}

	token, err := pgts.Token()
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

	pgts := &passwordGrantTokenSource{
		ctx:    ctx,
		config: config,
		token:  nil,
		user:   "testuser",
		pass:   "testpass",
	}

	token, err := pgts.Token()
	require.Error(t, err, "Expected an error")
	assert.Nil(t, token, "Expected nil token on error")
	assert.Contains(
		t,
		err.Error(),
		"failed to fetch token",
		"Expected error to contain 'failed to fetch token'",
	)
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

	pgts := &passwordGrantTokenSource{
		ctx:    ctx,
		config: config,
		token:  nil,
		user:   "testuser",
		pass:   "testpass",
	}

	token, err := pgts.Token()
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

	// Create a test server
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
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

	pgts := &passwordGrantTokenSource{
		ctx:    ctx,
		config: config,
		token:  nil,
		user:   "testuser",
		pass:   "testpass",
	}

	// Run multiple goroutines to test concurrent access
	const numGoroutines = 10
	results := make(chan error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			_, err := pgts.Token()
			results <- err
		}()
	}

	// Collect results
	for i := 0; i < numGoroutines; i++ {
		err := <-results
		assert.NoError(t, err, "Expected no error from concurrent access")
	}

	// Note: Without proper synchronization, this test might show race conditions
	// The actual implementation should handle concurrent access properly
}

func TestPasswordGrantTokenSource_Token_EdgeCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		tokenExpiry time.Time
		expectFetch bool
	}{
		{
			name:        "token expires exactly now",
			tokenExpiry: time.Now(),
			expectFetch: true,
		},
		{
			name:        "token expires in 20 second",
			tokenExpiry: time.Now().Add(20 * time.Second),
			expectFetch: false,
		},
		{
			name:        "token expires in 5 seconds", // default expirty delta is 10 sec
			tokenExpiry: time.Now().Add(5 * time.Second),
			expectFetch: true,
		},
		{
			name:        "token expired 1 second ago",
			tokenExpiry: time.Now().Add(-1 * time.Second),
			expectFetch: true,
		},
		{
			name:        "zero time (never expires)",
			tokenExpiry: time.Time{},
			expectFetch: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fetchCalled := false
			server := httptest.NewServer(
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					fetchCalled = true
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write(
						[]byte(
							`{"access_token":"new-token","token_type":"Bearer","expires_in":3600}`,
						),
					)
				}),
			)
			defer server.Close()

			ctx := context.Background()
			config := &oauth2.Config{
				Endpoint: oauth2.Endpoint{
					TokenURL: server.URL,
				},
			}

			existingToken := &oauth2.Token{
				AccessToken: "existing-token",
				TokenType:   "Bearer",
				Expiry:      tt.tokenExpiry,
			}

			pgts := &passwordGrantTokenSource{
				ctx:    ctx,
				config: config,
				token:  existingToken,
				user:   "testuser",
				pass:   "testpass",
			}

			token, err := pgts.Token()
			require.NoError(t, err, "Expected no error")
			assert.NotNil(t, token, "Expected non-nil token")
			assert.Equal(t, tt.expectFetch, fetchCalled, "Fetch expectation mismatch")
		})
	}
}
