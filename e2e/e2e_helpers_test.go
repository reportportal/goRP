//go:build e2e

// Package e2e contains end-to-end tests that run the real gorp binary against a real ReportPortal.
//
// They are not part of the regular test run: every file carries the `e2e` build tag, so
// `go test ./...` (and CI) don't see them. Run them with `task e2e`, or:
//
//	go test -tags e2e -count=1 -v ./e2e/...
//
// Credentials come from `.gorp` in the repository root (git-ignored; same JSON as ~/.gorp), or from the
// file named by GORP_CONFIG, overridden by GORP_HOST, GORP_PROJECT and GORP_API_KEY. ~/.gorp is not
// used. Every test creates launches with a unique name and deletes them afterwards.
// Use a sandbox project: the tests write real data.
package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/reportportal/goRP/v5/pkg/gorp"
	"github.com/reportportal/goRP/v5/pkg/openapi"
)

// Minimal views of the items and logs the tests assert on. The generated client can't be used for
// these endpoints: the item listing needs a provider selected through a free-form params map and
// its response is not decodable.
type (
	testItem struct {
		ID         int64  `json:"id"`
		Name       string `json:"name"`
		Type       string `json:"type"`
		Status     string `json:"status"`
		CodeRef    string `json:"codeRef"`
		Parent     int64  `json:"parent"`
		TestCaseID string `json:"testCaseId"`
		Parameters []struct {
			Key   string `json:"key"`
			Value string `json:"value"`
		} `json:"parameters"`
		// Retries are the earlier runs of the item; they are not listed as items themselves
		Retries []struct {
			ID     int64  `json:"id"`
			Status string `json:"status"`
		} `json:"retries"`
	}
	logEntry struct {
		Message       string          `json:"message"`
		Level         string          `json:"level"`
		ItemID        int64           `json:"itemId"`
		BinaryContent json.RawMessage `json:"binaryContent"`
	}
)

func (l logEntry) hasAttachment() bool {
	return len(l.BinaryContent) > 0 && string(l.BinaryContent) != "null"
}

// get decodes the JSON response of a GET request to the project API.
func (e *env) get(ctx context.Context, path string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		strings.TrimRight(e.cfg.Host, "/")+"/api/v1/"+e.cfg.Project+"/"+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+e.cfg.ApiKey)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GET %s: %s: %s", path, resp.Status, body)
	}

	return json.Unmarshal(body, out)
}

const (
	// launchNamePrefix marks every launch the tests create.
	launchNamePrefix  = "gorp-e2e-"
	eventuallyTimeout = 90 * time.Second
	eventuallyTick    = 2 * time.Second
)

var (
	gorpBinary string
	runID      = time.Now().Format("20060102-150405")
	nameSeq    atomic.Int64
)

type rpConfig struct {
	Host    string `json:"host"`
	Project string `json:"project"`
	ApiKey  string `json:"api_key"`
}

func TestMain(m *testing.M) {
	os.Exit(run(m))
}

func run(m *testing.M) int {
	dir, err := os.MkdirTemp("", "gorp-e2e-")
	if err != nil {
		fmt.Fprintln(os.Stderr, "e2e:", err)
		return 1
	}
	defer os.RemoveAll(dir)

	name := "gorp"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	gorpBinary = filepath.Join(dir, name)
	out, err := exec.Command("go", "build", "-o", gorpBinary, "github.com/reportportal/goRP/v5").
		CombinedOutput()
	if err != nil {
		fmt.Fprintf(os.Stderr, "e2e: building gorp failed: %v\n%s", err, out)
		return 1
	}

	sweepLeftovers()

	return m.Run()
}

// configPath is the gorp configuration file the tests use: GORP_CONFIG, or `.gorp` in the repository
// root. ~/.gorp is deliberately not used, so the tests never run with the everyday credentials.
func configPath() string {
	if path := os.Getenv("GORP_CONFIG"); path != "" {
		abs, err := filepath.Abs(path)
		if err != nil {
			return path
		}

		return abs
	}
	dir, err := os.Getwd()
	if err != nil {
		return ".gorp"
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return filepath.Join(dir, ".gorp")
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ".gorp"
		}
		dir = parent
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)

	return err == nil
}

// resolveConfig resolves the connection settings the way the CLI does: the configuration file
// (see configPath), overridden by GORP_HOST, GORP_PROJECT and GORP_API_KEY.
func resolveConfig() (rpConfig, error) {
	var cfg rpConfig
	path := configPath()
	if data, err := os.ReadFile(path); err == nil {
		if err := json.Unmarshal(data, &cfg); err != nil {
			return cfg, fmt.Errorf("%s is not valid JSON: %w", path, err)
		}
	}
	for env, dst := range map[string]*string{
		"GORP_HOST": &cfg.Host, "GORP_PROJECT": &cfg.Project, "GORP_API_KEY": &cfg.ApiKey,
	} {
		if v := os.Getenv(env); v != "" {
			*dst = v
		}
	}
	if cfg.Host == "" || cfg.Project == "" || cfg.ApiKey == "" {
		return cfg, fmt.Errorf(
			"no ReportPortal credentials: create %s (same JSON as ~/.gorp: host, project, api_key), "+
				"point GORP_CONFIG at another file, or set GORP_HOST, GORP_PROJECT and GORP_API_KEY",
			path,
		)
	}

	return cfg, nil
}

func loadConfig(t *testing.T) rpConfig {
	t.Helper()

	cfg, err := resolveConfig()
	if err != nil {
		t.Skip(err)
	}

	return cfg
}

// deleteLaunch deletes a launch. A launch that is still running can't be deleted, so it is finished
// first. A launch that doesn't exist (anymore) is not an error.
func deleteLaunch(ctx context.Context, client *gorp.Client, cfg rpConfig, id int64) error {
	for attempt := 0; attempt < 10; attempt++ {
		_, resp, err := client.LaunchAPI.DeleteLaunch(ctx, id, cfg.Project).Execute()
		switch {
		case err == nil:
			return nil
		case resp != nil && resp.StatusCode == http.StatusNotFound:
			return nil
		case resp != nil && resp.StatusCode == http.StatusNotAcceptable && attempt == 0:
			launch, _, gErr := client.LaunchAPI.GetLaunch(ctx, strconv.FormatInt(id, 10), cfg.Project).
				Execute()
			if gErr != nil {
				return gErr
			}
			u, _ := url.Parse(cfg.Host)
			stopper := gorp.NewReportingClient(strings.TrimRight(u.String(), "/"), cfg.Project,
				gorp.WithApiKeyAuth(ctx, cfg.ApiKey))
			if _, sErr := stopper.StopLaunch(ctx, launch.Uuid); sErr != nil {
				return fmt.Errorf("finishing launch %d: %w", id, sErr)
			}
		case resp != nil && resp.StatusCode == http.StatusNotAcceptable:
			time.Sleep(time.Second) // finishing is asynchronous
		default:
			return err
		}
	}

	return fmt.Errorf("launch %d could not be deleted", id)
}

// sweepLeftovers deletes launches of earlier runs that crashed before they could clean up.
func sweepLeftovers() {
	cfg, err := resolveConfig()
	if err != nil {
		return // the tests skip themselves
	}
	u, err := url.Parse(cfg.Host)
	if err != nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	client := gorp.NewClient(u, gorp.WithApiKeyAuth(ctx, cfg.ApiKey))
	launches, err := client.GetAllLaunchesByFilterString(
		ctx,
		cfg.Project,
		"filter.cnt.name="+launchNamePrefix,
	)
	if err != nil {
		fmt.Fprintln(os.Stderr, "e2e: could not look for leftovers:", err)
		return
	}
	for _, l := range launches {
		if err := deleteLaunch(ctx, client, cfg, l.Id); err != nil {
			fmt.Fprintf(os.Stderr, "e2e: could not delete leftover launch %d: %v\n", l.Id, err)
		}
	}
}

// env is a connected test environment.
type env struct {
	cfg      rpConfig
	client   *gorp.Client
	cleanups sync.Map // ids of launches with a registered deletion
}

func newEnv(t *testing.T) *env {
	t.Helper()

	cfg := loadConfig(t)
	u, err := url.Parse(cfg.Host)
	require.NoError(t, err)

	return &env{cfg: cfg, client: gorp.NewClient(u, gorp.WithApiKeyAuth(t.Context(), cfg.ApiKey))}
}

// uniqueName returns a launch name that doesn't collide with other tests or earlier runs.
func uniqueName(t *testing.T) string {
	t.Helper()

	return fmt.Sprintf(
		"gorp-e2e-%s-%d-%s",
		runID,
		nameSeq.Add(1),
		strings.ReplaceAll(t.Name(), "/", "_"),
	)
}

type result struct {
	stdout, stderr string
	exitCode       int
}

func (r result) String() string {
	return fmt.Sprintf("exit=%d\nstdout:\n%s\nstderr:\n%s", r.exitCode, r.stdout, r.stderr)
}

// command prepares the gorp binary; the connection settings come from the default configuration.
func command(ctx context.Context, args ...string) (*exec.Cmd, *bytes.Buffer, *bytes.Buffer) {
	cmd := exec.CommandContext(ctx, gorpBinary, args...)
	// the binary must read the same file as the test helpers
	cmd.Env = os.Environ()
	if path := configPath(); fileExists(path) {
		cmd.Env = append(cmd.Env, "GORP_CONFIG="+path)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr

	return cmd, &stdout, &stderr
}

func exitCode(err error) int {
	if err == nil {
		return 0
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode()
	}

	return -1
}

// gorp runs the binary to completion.
func runGorp(t *testing.T, args ...string) result {
	t.Helper()

	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Minute)
	defer cancel()
	cmd, stdout, stderr := command(ctx, args...)
	err := cmd.Run()

	return result{stdout: stdout.String(), stderr: stderr.String(), exitCode: exitCode(err)}
}

func (r result) requireOK(t *testing.T) result {
	t.Helper()
	require.Zero(t, r.exitCode, r.String())

	return r
}

// firstLine returns the trimmed first line of stdout (steps print the created entity ID there).
func (r result) firstLine() string {
	line, _, _ := strings.Cut(strings.TrimSpace(r.stdout), "\n")

	return strings.TrimSpace(line)
}

// writeFile creates a file in the test's temp directory.
func writeFile(t *testing.T, name, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), name)
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))

	return path
}

// findLaunch waits until the launch with the given name is visible in ReportPortal (reporting is
// asynchronous) and registers its deletion. The optional condition lets callers wait for a state.
func (e *env) findLaunch(
	t *testing.T,
	name string,
	condition func(*openapi.LaunchViewModel) bool,
) *openapi.LaunchViewModel {
	t.Helper()

	var found *openapi.LaunchViewModel
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		page, err := e.client.GetLaunchesByFilterString(
			t.Context(), e.cfg.Project, "filter.eq.name="+url.QueryEscape(name),
		)
		if !assert.NoError(c, err) || !assert.NotEmpty(c, page.Content) {
			return
		}
		l := page.Content[0]
		if condition != nil && !condition(&l) {
			c.Errorf("launch %q is %s, condition not met yet", name, l.GetStatus())
			return
		}
		found = &l
	}, eventuallyTimeout, eventuallyTick, "launch %q did not reach the expected state", name)

	e.deleteLaunchOnCleanup(t, found.Id)

	return found
}

// finished is a findLaunch condition.
func finished(l *openapi.LaunchViewModel) bool {
	return l.GetStatus() != "IN_PROGRESS" && l.GetStatus() != ""
}

func (e *env) deleteLaunchOnCleanup(t *testing.T, id int64) {
	t.Helper()

	if _, registered := e.cleanups.LoadOrStore(id, true); registered {
		return
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		if err := deleteLaunch(ctx, e.client, e.cfg, id); err != nil {
			t.Logf("cleanup: could not delete launch %d: %v", id, err)
		}
	})
}

// items returns the test items of a launch by name, waiting until at least want of them exist.
func (e *env) items(t *testing.T, launchID int64, want int) map[string]*testItem {
	t.Helper()

	byName := map[string]*testItem{}
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		clear(byName)
		var page struct {
			Content []testItem `json:"content"`
		}
		if !assert.NoError(
			c,
			e.get(
				t.Context(),
				"item?page.size=100&filter.eq.launchId="+strconv.FormatInt(launchID, 10),
				&page,
			),
		) {
			return
		}
		for i := range page.Content {
			byName[page.Content[i].Name] = &page.Content[i]
		}
		assert.Len(c, byName, want)
	}, eventuallyTimeout, eventuallyTick, "expected %d test items in launch %d", want, launchID)

	return byName
}

// logs returns the logs of a launch, waiting until at least want of them exist.
func (e *env) logs(t *testing.T, launchID int64, want int) []logEntry {
	t.Helper()

	var logs []logEntry
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		var page struct {
			Content []logEntry `json:"content"`
		}
		if !assert.NoError(
			c,
			e.get(
				t.Context(),
				"log?page.size=100&filter.eq.launchId="+strconv.FormatInt(launchID, 10),
				&page,
			),
		) {
			return
		}
		logs = page.Content
		assert.GreaterOrEqual(c, len(logs), want)
	}, eventuallyTimeout, eventuallyTick, "expected at least %d logs in launch %d", want, launchID)

	return logs
}
