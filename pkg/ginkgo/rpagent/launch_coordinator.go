package rpagent

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/reportportal/goRP/v5/pkg/gorp"
	"github.com/reportportal/goRP/v5/pkg/openapi"
)

// Temp-file name patterns used for cross-process coordination.
const (
	StateFilePattern         = "rp_agent_state_%s.json"
	ReportIndexPattern       = "rp_report_index_%s.json"
	FinalStatusFilePattern   = "rp_status_%s.json"
	ProcessReportFilePattern = "rp_status_%s_%d.json"
	ContainerCachePattern    = "rp_cache_%s_%x" // session ID, then the SHA-256 of the cache key
)

// inMemoryContainerCache is a per-process L1 cache to avoid redundant file reads.
var inMemoryContainerCache sync.Map

// AgentState is written by the wrapper process and read by every Ginkgo worker.
type AgentState struct {
	Endpoint   string `json:"endpoint"`
	Project    string `json:"project"`
	LaunchName string `json:"launchName"`
	LaunchID   string `json:"launchID"`
	SessionID  string `json:"sessionID"`
	// APIKey is intentionally omitted from the state file.
	// Workers re-read it from reportportal.properties via LoadRPConfig().
	// Storing it in the state file would leave a plaintext credential in $TMPDIR.
}

// SharedSpecStatus is appended (NDJSON) to a per-process file after each spec completes.
type SharedSpecStatus struct {
	SuiteName       string `json:"suiteName"`
	SuiteDirectory  string `json:"suiteDirectory"`
	SpecName        string `json:"specName"`
	Status          string `json:"status"`
	Priority        int    `json:"priority"`
	TestReferenceId string `json:"testReferenceId,omitempty"`
	FailureMessage  string `json:"failureMessage,omitempty"`
	FailureLocation string `json:"failureLocation,omitempty"`
	// ContainerIDs are the ReportPortal IDs of the spec's Describe/Context items, outermost
	// first. The wrapper uses them to finish those items once every worker is done.
	ContainerIDs []string `json:"containerIDs,omitempty"`
	// EndTime is when the spec really ended.
	EndTime time.Time `json:"endTime,omitempty"`
	// Fixture marks a failed BeforeSuite/AfterSuite rather than a spec: it fails its suite
	// and the launch, but is not a test, so it stays out of the test counts.
	Fixture bool `json:"fixture,omitempty"`
}

// LaunchCoordinator manages temp files used to coordinate the wrapper and Ginkgo workers.
type LaunchCoordinator struct {
	reportIndexFile string
	finalStatusFile string
	stateFile       string
}

// NewLaunchCoordinator creates a coordinator for the given session ID. It returns an
// error rather than exiting: reporting must never kill the test process.
func NewLaunchCoordinator(sessionID string) (*LaunchCoordinator, error) {
	if sessionID == "" {
		return nil, errors.New("a session ID is required to coordinate a launch")
	}
	return &LaunchCoordinator{
		reportIndexFile: filepath.Join(os.TempDir(), fmt.Sprintf(ReportIndexPattern, sessionID)),
		finalStatusFile: filepath.Join(
			os.TempDir(),
			fmt.Sprintf(FinalStatusFilePattern, sessionID),
		),
		stateFile: filepath.Join(os.TempDir(), fmt.Sprintf(StateFilePattern, sessionID)),
	}, nil
}

// LoadAgentState reads the state file written by the wrapper process.
// The API key is NOT in the state file; callers that need it must call LoadRPConfig().
func (lc *LaunchCoordinator) LoadAgentState() (*AgentState, error) {
	data, err := os.ReadFile(lc.stateFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read agent state file %s: %w", lc.stateFile, err)
	}
	var state AgentState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("failed to parse agent state file: %w", err)
	}
	if state.LaunchID == "" || state.SessionID == "" {
		return nil, fmt.Errorf("agent state file is incomplete (missing launchID or sessionID)")
	}
	return &state, nil
}

// SetAggregatedResults writes all collected spec statuses to the final status file.
// Called by the wrapper after all workers finish.
func (lc *LaunchCoordinator) SetAggregatedResults(statuses []SharedSpecStatus) error {
	data, err := json.MarshalIndent(statuses, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal aggregated stats: %w", err)
	}
	if err := os.WriteFile(lc.finalStatusFile, data, 0o600); err != nil {
		return fmt.Errorf("failed to write aggregated stats: %w", err)
	}
	LogVerboseOperation(
		fmt.Sprintf("wrote %d spec results to %s", len(statuses), lc.finalStatusFile),
	)
	return nil
}

// isFailureState reports whether a Ginkgo state string is a failure.
// Covers all five Ginkgo failure states: failed, panicked, interrupted, aborted, timedout.
func isFailureState(state string) bool {
	switch strings.ToLower(state) {
	case "failed", "panicked", "interrupted", "aborted", "timedout":
		return true
	}
	return false
}

// Stats holds pass/fail/skip counts.
type Stats struct {
	Total   int
	Passed  int
	Failed  int
	Skipped int
}

// PriorityStats holds Stats broken down by P0–P4 priority.
type PriorityStats [5]Stats

// ExecutionStats is the aggregated result of a launch, used for the console summary.
type ExecutionStats struct {
	OverallStatus  Stats
	PriorityStatus PriorityStats
}

// GetStats returns the Stats for a priority level (0–4), or nil when out of range.
func (ps *PriorityStats) GetStats(priority int) *Stats {
	if priority < 0 || priority >= len(ps) {
		return nil
	}
	return &ps[priority]
}

// GetAggregatedStatus reads and aggregates the final status file into SDK execution stats.
func (lc *LaunchCoordinator) GetAggregatedStatus() (*ExecutionStats, error) {
	data, err := os.ReadFile(lc.finalStatusFile)
	if err != nil {
		return nil, fmt.Errorf("could not read final status file %q: %w", lc.finalStatusFile, err)
	}
	var statuses []SharedSpecStatus
	if err := json.Unmarshal(data, &statuses); err != nil {
		return nil, fmt.Errorf("could not parse final status file: %w", err)
	}

	result := &ExecutionStats{}
	for _, spec := range statuses {
		if spec.Fixture {
			continue
		}
		result.OverallStatus.Total++
		switch {
		case strings.ToLower(spec.Status) == "passed":
			result.OverallStatus.Passed++
		case isFailureState(spec.Status):
			result.OverallStatus.Failed++
		default:
			result.OverallStatus.Skipped++
		}

		if spec.Priority >= 0 && spec.Priority <= 4 {
			ps := result.PriorityStatus.GetStats(spec.Priority)
			if ps != nil {
				ps.Total++
				switch {
				case strings.ToLower(spec.Status) == "passed":
					ps.Passed++
				case isFailureState(spec.Status):
					ps.Failed++
				default:
					ps.Skipped++
				}
			}
		}
	}
	return result, nil
}

// FinalizeLaunch reads aggregated results and closes the launch on ReportPortal.
// forceFailed marks the launch as FAILED regardless of test results (e.g. suite construction failure).
func FinalizeLaunch(forceFailed bool) error {
	sessionID := os.Getenv("RP_SESSION_ID")
	if sessionID == "" {
		return fmt.Errorf("RP_SESSION_ID is not set")
	}

	lc, err := NewLaunchCoordinator(sessionID)
	if err != nil {
		return err
	}
	state, err := lc.LoadAgentState()
	if err != nil {
		return fmt.Errorf("could not load agent state: %w", err)
	}
	if state.LaunchID == "" {
		return fmt.Errorf("no launch ID in agent state")
	}

	aggregated, err := lc.GetAggregatedStatus()
	if err != nil {
		return fmt.Errorf("failed to get aggregated status: %w", err)
	}

	// API key is not in the state file; read from config.
	config, err := LoadRPConfig()
	if err != nil {
		return fmt.Errorf("could not load reportportal.properties for API key: %w", err)
	}

	client := NewReportingClient(state.Endpoint, state.Project, config.APIKey, config.Debug)

	overallStatus := gorp.Statuses.Passed
	if aggregated.OverallStatus.Failed > 0 || forceFailed {
		overallStatus = gorp.Statuses.Failed
	}

	rs, err := client.FinishLaunch(context.Background(), state.LaunchID, &openapi.FinishExecutionRQ{
		EndTime: time.Now(),
		Status:  statusPtr(overallStatus),
	})
	if err != nil {
		return fmt.Errorf("failed to finish launch: %w", err)
	}

	fmt.Printf(
		"\nSUCCESS: launch %s finalized (%d total tests)\n",
		state.LaunchID,
		aggregated.OverallStatus.Total,
	)

	// Print the direct UI link only when rp.debug is enabled.
	if config.Debug && rs != nil && rs.Number != nil && *rs.Number > 0 {
		endpoint := strings.TrimRight(state.Endpoint, "/")
		fmt.Printf("  → %s/ui/#%s/launches/all/%d\n", endpoint, state.Project, *rs.Number)
	}
	return nil
}

// getLockRetryConfig returns configurable retry settings for file-lock operations.
func getLockRetryConfig() (int, time.Duration) {
	count := 50
	if v := os.Getenv("RP_LOCK_RETRY_COUNT"); v != "" {
		if i, err := strconv.Atoi(v); err == nil && i > 0 {
			count = i
		}
	}
	sleep := 200 * time.Millisecond
	if v := os.Getenv("RP_LOCK_RETRY_SLEEP_MS"); v != "" {
		if i, err := strconv.Atoi(v); err == nil && i > 0 {
			sleep = time.Duration(i) * time.Millisecond
		}
	}
	return count, sleep
}

// waitForFileWithRetry polls until a file exists and is non-empty.
func waitForFileWithRetry(path string, maxRetries int, sleep time.Duration) (string, error) {
	for i := 0; i < maxRetries; i++ {
		if data, err := os.ReadFile(path); err == nil && len(data) > 0 {
			return string(data), nil
		}
		time.Sleep(sleep)
	}
	return "", fmt.Errorf("timeout waiting for cache file content: %s", path)
}

// ContainerCacheManager manages the shared Describe/Context container cache
// using a two-tier strategy: L1 in-memory (sync.Map) + L2 file-per-key.
type ContainerCacheManager struct {
	sessionID string
}

// NewContainerCacheManager creates a ContainerCacheManager for the current suite.
func (lc *LaunchCoordinator) NewContainerCacheManager() *ContainerCacheManager {
	return &ContainerCacheManager{sessionID: os.Getenv("RP_SESSION_ID")}
}

// GetOrCreate returns the cached container ID for key, creating it via createFunc if absent.
//
// Cross-process correctness:
//   - A shared lock file (cacheFile+".lock") is claimed with O_EXCL so exactly one process
//     calls createFunc per key. The lock file path is the same for all processes (no PID),
//     so competing workers do actually contend on the same file.
//   - The winning process writes the final cache file and removes the lock.
//   - Losing processes wait for the final file using getLockRetryConfig().
func (ccm *ContainerCacheManager) GetOrCreate(
	key string,
	createFunc func() string,
) (string, error) {
	// L1: in-process cache
	if val, ok := inMemoryContainerCache.Load(key); ok {
		return val.(string), nil
	}

	// L2: one file per key. The session ID is part of the name so the wrapper can remove
	// exactly this run's files, and no other run's, once all workers have finished.
	hash := sha256.Sum256([]byte(key))
	cacheFilePath := filepath.Join(
		os.TempDir(),
		fmt.Sprintf(ContainerCachePattern, ccm.sessionID, hash),
	)

	if data, err := os.ReadFile(cacheFilePath); err == nil && len(data) > 0 {
		id := string(data)
		inMemoryContainerCache.Store(key, id)
		return id, nil
	}

	// Race to create: use a shared lock file (same path for all processes, no PID).
	lockPath := cacheFilePath + ".lock"
	lf, err := os.OpenFile(lockPath, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		if os.IsExist(err) {
			// Another process won the lock — wait for the final cache file.
			retryCount, retrySleep := getLockRetryConfig()
			id, waitErr := waitForFileWithRetry(cacheFilePath, retryCount, retrySleep)
			if waitErr != nil {
				return "", waitErr
			}
			if id == "" {
				return "", fmt.Errorf(
					"empty container ID received from cache file: %s",
					cacheFilePath,
				)
			}
			inMemoryContainerCache.Store(key, id)
			return id, nil
		}
		// Unexpected error — check if another process already completed the rename.
		if data, rerr := os.ReadFile(cacheFilePath); rerr == nil && len(data) > 0 {
			id := string(data)
			inMemoryContainerCache.Store(key, id)
			return id, nil
		}
		return "", fmt.Errorf("failed to create lock file: %w", err)
	}
	// We hold the lock.
	_ = lf.Close()
	defer func() { _ = os.Remove(lockPath) }()

	// Write to a tmp file first, then atomically rename so waiting processes
	// never see a partial write.
	tmpPath := cacheFilePath + ".tmp"
	f, ferr := os.OpenFile(tmpPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o600)
	if ferr != nil {
		return "", fmt.Errorf("failed to create tmp cache file: %w", ferr)
	}

	id := createFunc()
	if id == "" {
		_ = f.Close()
		_ = os.Remove(tmpPath)
		return "", fmt.Errorf("createFunc returned empty ID for key %q", key)
	}
	if _, werr := f.WriteString(id); werr != nil {
		_ = f.Close()
		_ = os.Remove(tmpPath)
		return "", fmt.Errorf("failed to write ID to cache file: %w", werr)
	}
	if serr := f.Sync(); serr != nil {
		_ = f.Close()
		_ = os.Remove(tmpPath)
		return "", fmt.Errorf("failed to sync cache file: %w", serr)
	}
	_ = f.Close()

	if rerr := os.Rename(tmpPath, cacheFilePath); rerr != nil {
		_ = os.Remove(tmpPath)
		return "", fmt.Errorf("failed to rename cache file: %w", rerr)
	}

	inMemoryContainerCache.Store(key, id)
	return id, nil
}
