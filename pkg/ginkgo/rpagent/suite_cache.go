package rpagent

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const suiteCachePattern = "rp_suites_%s.json" // %s = sessionID

// SuiteCache maps absolute suite directory paths to their ReportPortal suite IDs.
// It is written by the wrapper process and read by each Ginkgo worker.
type SuiteCache struct {
	Suites map[string]string `json:"suites"`
}

// SaveSuiteCache writes the suite cache to the OS temp directory.
func SaveSuiteCache(sessionID string, cache *SuiteCache) error {
	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal suite cache: %w", err)
	}
	path := filepath.Join(os.TempDir(), fmt.Sprintf(suiteCachePattern, sessionID))
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("failed to write suite cache to %s: %w", path, err)
	}
	return nil
}

// LoadSuiteCache reads the suite cache from the OS temp directory.
func LoadSuiteCache(sessionID string) (*SuiteCache, error) {
	path := filepath.Join(os.TempDir(), fmt.Sprintf(suiteCachePattern, sessionID))
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read suite cache from %s: %w", path, err)
	}
	var cache SuiteCache
	if err := json.Unmarshal(data, &cache); err != nil {
		return nil, fmt.Errorf("failed to unmarshal suite cache: %w", err)
	}
	if cache.Suites == nil {
		cache.Suites = make(map[string]string)
	}
	return &cache, nil
}

// GetSuiteIDForDirectory looks up the suite ID for the given directory.
func GetSuiteIDForDirectory(sessionID, directory string) (string, error) {
	cache, err := LoadSuiteCache(sessionID)
	if err != nil {
		return "", err
	}
	absDir, err := filepath.Abs(directory)
	if err != nil {
		return "", fmt.Errorf("failed to resolve absolute path for %s: %w", directory, err)
	}
	id, ok := cache.Suites[absDir]
	if !ok {
		return "", fmt.Errorf("no suite ID found for directory: %s", absDir)
	}
	return id, nil
}
