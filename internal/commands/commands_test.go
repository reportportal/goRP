package commands

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v3"
)

// writeConfigFile writes a clientConfig as JSON to HOME/.gorp in the given home directory.
func writeConfigFile(t *testing.T, homeDir string, cfg clientConfig) {
	t.Helper()
	f, err := os.Create(filepath.Join(homeDir, ".gorp"))
	require.NoError(t, err)
	defer f.Close()
	require.NoError(t, json.NewEncoder(f).Encode(cfg))
}

// emptyCmd returns a *cli.Command that has no flags set — all string/bool reads return zero values.
func emptyCmd() *cli.Command {
	return &cli.Command{}
}

func TestGetConfig_FromFile(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	writeConfigFile(t, tmpDir, clientConfig{
		ApiKey:  "file-api-key",
		Project: "file-project",
		URL:     "http://localhost:8080",
	})

	cfg, err := getConfig(emptyCmd())

	require.NoError(t, err)
	assert.Equal(t, "file-api-key", cfg.ApiKey)
	assert.Equal(t, "file-project", cfg.Project)
	assert.Equal(t, "http://localhost:8080", cfg.URL)
}

func TestGetConfig_CLIOverridesFile(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	writeConfigFile(t, tmpDir, clientConfig{
		ApiKey:  "file-key",
		Project: "file-project",
		URL:     "http://original:8080",
	})

	// Define flags with override values as defaults (simulates parsed CLI values).
	cmd := &cli.Command{}
	cmd.Flags = []cli.Flag{
		&cli.StringFlag{Name: "api-key", Value: "override-key"},
		&cli.StringFlag{Name: "project", Value: "override-project"},
		&cli.StringFlag{Name: "host", Value: "http://override:9090"},
	}

	cfg, err := getConfig(cmd)

	require.NoError(t, err)
	assert.Equal(t, "override-key", cfg.ApiKey)
	assert.Equal(t, "override-project", cfg.Project)
	assert.Equal(t, "http://override:9090", cfg.URL)
}

func TestGetConfig_NoFile_NoFlags_FailsValidation(t *testing.T) {
	// Point HOME at an empty temp dir so no config file exists.
	t.Setenv("HOME", t.TempDir())

	_, err := getConfig(emptyCmd())

	assert.ErrorIs(t, err, errApiKeyNotSet)
}

func TestGetConfig_CorruptFile(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, ".gorp"), []byte("not json"), 0o600))

	_, err := getConfig(emptyCmd())

	assert.Error(t, err)
}

func TestValidateConfig_MissingApiKey(t *testing.T) {
	t.Parallel()
	err := validateConfig(&clientConfig{Project: "p", URL: "http://h"})
	assert.ErrorIs(t, err, errApiKeyNotSet)
}

func TestValidateConfig_MissingProject(t *testing.T) {
	t.Parallel()
	err := validateConfig(&clientConfig{ApiKey: "k", URL: "http://h"})
	assert.ErrorIs(t, err, errProjectNotSet)
}

func TestValidateConfig_MissingHost(t *testing.T) {
	t.Parallel()
	err := validateConfig(&clientConfig{ApiKey: "k", Project: "p"})
	assert.ErrorIs(t, err, errHostNotSet)
}

func TestValidateConfig_AllPresent(t *testing.T) {
	t.Parallel()
	err := validateConfig(&clientConfig{ApiKey: "k", Project: "p", URL: "http://h"})
	assert.NoError(t, err)
}

func TestBuildClientFromConfig_ValidURL(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	cfg := &clientConfig{
		URL:     srv.URL,
		Project: "myproject",
		ApiKey:  "mykey",
	}

	client, returnedCfg, err := buildClientFromConfig(context.Background(), cfg)

	require.NoError(t, err)
	assert.NotNil(t, client)
	assert.Equal(t, cfg, returnedCfg)
}

func TestBuildClientFromConfig_InvalidURL(t *testing.T) {
	t.Parallel()

	cfg := &clientConfig{
		URL:     "://bad-url",
		Project: "p",
		ApiKey:  "k",
	}

	_, _, err := buildClientFromConfig(context.Background(), cfg)

	assert.Error(t, err)
}
