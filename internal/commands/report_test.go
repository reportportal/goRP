package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

	// Assert: either the ticker loop detected ctx.Done(), or the HTTP call itself was cancelled.
	// Both outcomes are valid representations of the quality gate check timing out.
	assert.Error(t, err)
	errMsg := err.Error()
	isTimeout := strings.Contains(errMsg, "timeout waiting for quality gate status") ||
		strings.Contains(errMsg, "context deadline exceeded")
	assert.True(t, isTimeout, "expected a timeout error, got: %s", errMsg)
	assert.GreaterOrEqual(t, callCount, 1, "Expected at least 1 API call")
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

// ---------------------------------------------------------------------------
// test2json reportLaunch pipeline
// ---------------------------------------------------------------------------

// reportingMockServer builds an httptest.Server that handles every endpoint
// the ReportingClient calls during a standard test2json report run.
// It returns the server and atomic counters for the calls it saw.
func reportingMockServer(t *testing.T, project string) (
	*httptest.Server,
	*atomic.Int32, // launchStarts
	*atomic.Int32, // launchFinishes
	*atomic.Int32, // testItemStarts  (suites + tests)
	*atomic.Int32, // testItemFinishes
) {
	t.Helper()

	var launchStarts, launchFinishes, itemStarts, itemFinishes atomic.Int32

	mux := http.NewServeMux()

	// POST /api/v2/{project}/launch  →  start launch
	mux.HandleFunc("POST /api/v2/"+project+"/launch", func(w http.ResponseWriter, _ *http.Request) {
		launchStarts.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"id": "launch-uuid-1"})
	})

	// PUT /api/v2/{project}/launch/{id}/finish  →  finish launch
	mux.HandleFunc(
		"PUT /api/v2/"+project+"/launch/launch-uuid-1/finish",
		func(w http.ResponseWriter, _ *http.Request) {
			launchFinishes.Add(1)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).
				Encode(map[string]string{"id": "launch-uuid-1", "message": "finished"})
		},
	)

	// POST /api/v2/{project}/item/  (trailing slash)  →  start suite
	mux.HandleFunc("POST /api/v2/"+project+"/item/", func(w http.ResponseWriter, r *http.Request) {
		// StartChildTest hits /item/{id}, StartTest hits /item/ — differentiate by suffix.
		if r.URL.Path != "/api/v2/"+project+"/item/" {
			// child test (suite id in path)
			itemStarts.Add(1)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]string{"id": "test-item-1"})
			return
		}
		// suite start
		itemStarts.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"id": "suite-1"})
	})

	// PUT /api/v2/{project}/item/{id}  →  finish test item
	mux.HandleFunc("PUT /api/v2/"+project+"/item/", func(w http.ResponseWriter, _ *http.Request) {
		itemFinishes.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"message": "finished"})
	})

	// POST /api/v2/{project}/log  →  save logs
	mux.HandleFunc("POST /api/v2/"+project+"/log", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"id": "log-1"})
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv, &launchStarts, &launchFinishes, &itemStarts, &itemFinishes
}

// reportCmd builds a *cli.Command with the flags reportLaunch reads.
func reportCmd(filePath, launchName string, reportEmpty bool) *cli.Command {
	cmd := &cli.Command{}
	cmd.Flags = []cli.Flag{
		&cli.StringFlag{Name: "file", Value: filePath},
		&cli.StringFlag{Name: "launchName", Value: launchName},
		&cli.BoolFlag{Name: "reportEmptyPkg", Value: reportEmpty},
		&cli.StringSliceFlag{Name: "attr"},
	}
	return cmd
}

func TestReportLaunch_SingleTest(t *testing.T) {
	t.Parallel()

	project := "testproj"
	srv, launchStarts, launchFinishes, itemStarts, itemFinishes := reportingMockServer(t, project)

	// Build a minimal test2json input: one test, passing.
	lines := []string{
		`{"time":"2025-01-01T00:00:00Z","action":"run","package":"example/pkg","test":"TestOne"}`,
		`{"time":"2025-01-01T00:00:01Z","action":"output","package":"example/pkg","test":"TestOne","output":"=== RUN TestOne\n"}`,
		`{"time":"2025-01-01T00:00:02Z","action":"pass","package":"example/pkg","test":"TestOne","elapsed":0.1}`,
		`{"time":"2025-01-01T00:00:03Z","action":"pass","package":"example/pkg","elapsed":0.2}`,
	}
	f, err := os.CreateTemp("", "test2json-*.jsonl")
	require.NoError(t, err)
	t.Cleanup(func() { os.Remove(f.Name()) })
	_, err = f.WriteString(strings.Join(lines, "\n") + "\n")
	require.NoError(t, err)
	require.NoError(t, f.Close())

	cfg := &clientConfig{URL: srv.URL, Project: project, ApiKey: "tok"}
	cmd := reportCmd(f.Name(), "my-launch", false)

	launchID, err := reportLaunch(context.Background(), cfg, cmd)

	require.NoError(t, err)
	assert.Equal(t, "launch-uuid-1", launchID)
	assert.Equal(t, int32(1), launchStarts.Load(), "launch should start once")
	assert.Equal(t, int32(1), launchFinishes.Load(), "launch should finish once")
	// suite + test = 2 item starts; test finish + suite finish = 2 item finishes
	assert.Equal(t, int32(2), itemStarts.Load(), "suite and test should each start")
	assert.Equal(t, int32(2), itemFinishes.Load(), "suite and test should each finish")
}

func TestReportLaunch_MultipleTests(t *testing.T) {
	t.Parallel()

	project := "testproj"
	srv, launchStarts, launchFinishes, itemStarts, itemFinishes := reportingMockServer(t, project)

	lines := []string{
		`{"time":"2025-01-01T00:00:00Z","action":"run","package":"mypkg","test":"TestA"}`,
		`{"time":"2025-01-01T00:00:01Z","action":"pass","package":"mypkg","test":"TestA","elapsed":0.1}`,
		`{"time":"2025-01-01T00:00:02Z","action":"run","package":"mypkg","test":"TestB"}`,
		`{"time":"2025-01-01T00:00:03Z","action":"fail","package":"mypkg","test":"TestB","elapsed":0.2}`,
		`{"time":"2025-01-01T00:00:04Z","action":"fail","package":"mypkg","elapsed":0.3}`,
	}
	f, err := os.CreateTemp("", "test2json-*.jsonl")
	require.NoError(t, err)
	t.Cleanup(func() { os.Remove(f.Name()) })
	_, err = f.WriteString(strings.Join(lines, "\n") + "\n")
	require.NoError(t, err)
	require.NoError(t, f.Close())

	cfg := &clientConfig{URL: srv.URL, Project: project, ApiKey: "tok"}
	cmd := reportCmd(f.Name(), "multi-launch", false)

	launchID, err := reportLaunch(context.Background(), cfg, cmd)

	require.NoError(t, err)
	assert.Equal(t, "launch-uuid-1", launchID)
	assert.Equal(t, int32(1), launchStarts.Load())
	assert.Equal(t, int32(1), launchFinishes.Load())
	// 1 suite + 2 tests = 3 item starts; 2 test finishes + 1 suite finish = 3 item finishes
	assert.Equal(t, int32(3), itemStarts.Load())
	assert.Equal(t, int32(3), itemFinishes.Load())
}

func TestReportLaunch_EmptyInput(t *testing.T) {
	t.Parallel()

	project := "testproj"
	srv, launchStarts, launchFinishes, _, _ := reportingMockServer(t, project)

	f, err := os.CreateTemp("", "empty-*.jsonl")
	require.NoError(t, err)
	t.Cleanup(func() { os.Remove(f.Name()) })
	require.NoError(t, f.Close())

	cfg := &clientConfig{URL: srv.URL, Project: project, ApiKey: "tok"}
	cmd := reportCmd(f.Name(), "empty-launch", false)

	launchID, err := reportLaunch(context.Background(), cfg, cmd)

	// No events — no launch was started, launchUUID stays empty.
	require.NoError(t, err)
	assert.Empty(t, launchID)
	assert.Equal(t, int32(0), launchStarts.Load(), "empty input should not start a launch")
	assert.Equal(t, int32(0), launchFinishes.Load())
}
