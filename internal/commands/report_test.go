package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/urfave/cli/v3"
)

// setupMockServer creates a test server that simulates the ReportPortal API
func setupMockServer(
	t *testing.T,
	project string,
	handler func(projectName string, w http.ResponseWriter, r *http.Request),
) (*httptest.Server, *clientConfig) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handler(project, w, r)
	}))
	t.Cleanup(func() {
		server.Close()
	})

	cfg := &clientConfig{
		URL:     server.URL,
		Project: project,
		ApiKey:  "test-api-key",
	}

	return server, cfg
}

// createTestCommand creates a CLI command with necessary flags for quality gate testing
func createTestCommand(timeout, checkInterval time.Duration) *cli.Command {
	cmd := &cli.Command{}
	cmd.Flags = []cli.Flag{
		&cli.DurationFlag{Name: "quality-gate-timeout", Value: timeout},
		&cli.DurationFlag{Name: "quality-gate-check-interval", Value: checkInterval},
	}
	return cmd
}

func TestCheckQualityGate_HappyPath(t *testing.T) {
	// Setup mock server that returns PASSED quality gate status
	launchUUID := "launch123"
	project := "testproject"

	_, cfg := setupMockServer(
		t,
		project,
		func(projectName string, w http.ResponseWriter, r *http.Request) {
			expectedPath := fmt.Sprintf("/api/v1/%s/launch/%s", projectName, launchUUID)
			if r.URL.Path != expectedPath {
				http.Error(w, "Not found", http.StatusNotFound)
				return
			}

			response := getDefaultLaunchResponse(launchUUID, "PASSED")
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(response)
		},
	)

	// Create test command with required flags
	cmd := createTestCommand(10*time.Second, 100*time.Millisecond)

	// Act
	err := checkQualityGateInternal(context.Background(), launchUUID, cfg, cmd)

	// Assert
	assert.NoError(t, err)
}

func TestCheckQualityGate_MultipleAttempts(t *testing.T) {
	// Setup mock server that returns IN_PROGRESS twice then PASSED
	launchUUID := "launch123"
	project := "testproject"
	callCount := 0

	_, cfg := setupMockServer(
		t,
		project,
		func(projectName string, w http.ResponseWriter, r *http.Request) {
			expectedPath := fmt.Sprintf("/api/v1/%s/launch/%s", projectName, launchUUID)
			if r.URL.Path != expectedPath {
				http.Error(w, "Not found", http.StatusNotFound)
				return
			}

			callCount++

			// First two calls return IN PROGRESS, third returns PASSED
			status := "IN PROGRESS"
			if callCount >= 3 {
				status = "PASSED"
			}

			response := getDefaultLaunchResponse(launchUUID, status)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(response)
		},
	)

	// Create test command with required flags - using longer timeout to ensure test has enough time
	cmd := createTestCommand(2*time.Second, 50*time.Millisecond)

	// Act
	err := checkQualityGateInternal(context.Background(), launchUUID, cfg, cmd)

	// Assert
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, callCount, 3, "Expected at least 3 API calls")
}

func TestCheckQualityGate_Timeout(t *testing.T) {
	// Setup mock server that always returns IN_PROGRESS status
	launchUUID := "launch123"
	project := "testproject"
	callCount := 0

	_, cfg := setupMockServer(
		t,
		project,
		func(projectName string, w http.ResponseWriter, r *http.Request) {
			expectedPath := fmt.Sprintf("/api/v1/%s/launch/%s", projectName, launchUUID)
			if r.URL.Path != expectedPath {
				http.Error(w, "Not found", http.StatusNotFound)
				return
			}

			callCount++

			// Always return IN PROGRESS
			response := getDefaultLaunchResponse(launchUUID, "IN PROGRESS")
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(response)
		},
	)

	// Create test command with very short timeout
	cmd := createTestCommand(300*time.Millisecond, 50*time.Millisecond)

	// Act
	err := checkQualityGateInternal(context.Background(), launchUUID, cfg, cmd)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "timeout waiting for quality gate status")
	assert.GreaterOrEqual(t, callCount, 2, "Expected at least 2 API calls")
}

func TestCheckQualityGate_FailedStatus(t *testing.T) {
	// Setup mock server that returns FAILED quality gate status
	launchUUID := "launch123"
	project := "testproject"

	_, cfg := setupMockServer(
		t,
		project,
		func(projectName string, w http.ResponseWriter, r *http.Request) {
			expectedPath := fmt.Sprintf("/api/v1/%s/launch/%s", projectName, launchUUID)
			if r.URL.Path != expectedPath {
				http.Error(w, "Not found", http.StatusNotFound)
				return
			}

			response := getDefaultLaunchResponse(launchUUID, "FAILED")
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(response)
		},
	)

	// Create test command with required flags
	cmd := createTestCommand(10*time.Second, 100*time.Millisecond)

	// Act
	err := checkQualityGateInternal(context.Background(), launchUUID, cfg, cmd)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "status: FAILED")
}

func TestCheckQualityGate_MissingQualityGate(t *testing.T) {
	// Setup mock server that returns a launch with no quality gate metadata
	launchUUID := "launch123"
	project := "testproject"

	_, cfg := setupMockServer(
		t,
		project,
		func(projectName string, w http.ResponseWriter, r *http.Request) {
			expectedPath := fmt.Sprintf("/api/v1/%s/launch/%s", projectName, launchUUID)
			if r.URL.Path != expectedPath {
				http.Error(w, "Not found", http.StatusNotFound)
				return
			}

			response := getDefaultLaunchResponse(launchUUID, "")
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(response)
		},
	)

	// Create test command with required flags
	cmd := createTestCommand(10*time.Second, 100*time.Millisecond)

	// Act
	err := checkQualityGateInternal(context.Background(), launchUUID, cfg, cmd)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "quality gate metadata not found")
}

// getDefaultLaunchResponse creates a standard launch response with the specified quality gate status
func getDefaultLaunchResponse(launchUUID, qualityGateStatus string) map[string]interface{} {
	response := map[string]interface{}{
		"id":        100500,
		"uuid":      launchUUID,
		"name":      "Test Launch",
		"number":    1,
		"startTime": time.Now().Format(time.RFC3339),
		"status":    "FINISHED",
		"metadata":  map[string]interface{}{},
	}

	// Only add quality gate if status is provided
	if qualityGateStatus != "" {
		response["metadata"] = map[string]interface{}{
			"qualityGate": map[string]interface{}{
				"id":     123,
				"status": qualityGateStatus,
			},
		}
	}

	return response
}
