package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v3"
)

// stepsMockServer builds an httptest.Server that handles every endpoint
// used by the individual report step commands.
func stepsMockServer(t *testing.T, project string) (
	*httptest.Server,
	*atomic.Int32, // launchStarts
	*atomic.Int32, // launchFinishes
	*atomic.Int32, // itemStarts  (root + child)
	*atomic.Int32, // itemFinishes
	*atomic.Int32, // logSaves
) {
	t.Helper()

	var launchStarts, launchFinishes, itemStarts, itemFinishes, logSaves atomic.Int32

	mux := http.NewServeMux()

	mux.HandleFunc("POST /api/v2/"+project+"/launch", func(w http.ResponseWriter, _ *http.Request) {
		launchStarts.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"id": "launch-uuid-1"})
	})

	mux.HandleFunc(
		"PUT /api/v2/"+project+"/launch/launch-uuid-1/finish",
		func(w http.ResponseWriter, _ *http.Request) {
			launchFinishes.Add(1)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).
				Encode(map[string]string{"id": "launch-uuid-1", "link": "https://rp/launch/1"})
		},
	)

	mux.HandleFunc("POST /api/v2/"+project+"/item/", func(w http.ResponseWriter, r *http.Request) {
		itemStarts.Add(1)
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path != "/api/v2/"+project+"/item/" {
			_ = json.NewEncoder(w).Encode(map[string]string{"id": "child-item-1"})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"id": "root-item-1"})
	})

	mux.HandleFunc("PUT /api/v2/"+project+"/item/", func(w http.ResponseWriter, _ *http.Request) {
		itemFinishes.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"message": "finished"})
	})

	mux.HandleFunc("POST /api/v2/"+project+"/log", func(w http.ResponseWriter, _ *http.Request) {
		logSaves.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"id": "log-1"})
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv, &launchStarts, &launchFinishes, &itemStarts, &itemFinishes, &logSaves
}

func stepCfg(serverURL, project string) *clientConfig {
	return &clientConfig{URL: serverURL, Project: project, ApiKey: "tok"}
}

// stepCmd creates a *cli.Command with a captured Writer for testing.
func stepCmd(buf *bytes.Buffer, flags []cli.Flag) *cli.Command {
	cmd := &cli.Command{Writer: buf, Flags: flags}
	return cmd
}

// ---------------------------------------------------------------------------
// start-launch
// ---------------------------------------------------------------------------

func TestStartLaunchStep(t *testing.T) {
	t.Parallel()
	project := "testproj"
	srv, starts, _, _, _, _ := stepsMockServer(t, project)

	var buf bytes.Buffer
	cmd := stepCmd(&buf, []cli.Flag{
		&cli.StringFlag{Name: "name", Value: "My Launch"},
		&cli.StringFlag{Name: "mode", Value: "DEFAULT"},
		&cli.StringFlag{Name: "description"},
		&cli.StringSliceFlag{Name: "attr"},
	})

	id, err := startLaunchStep(context.Background(), stepCfg(srv.URL, project), cmd)

	require.NoError(t, err)
	assert.Equal(t, "launch-uuid-1", id)
	assert.Equal(t, int32(1), starts.Load())
	assert.Equal(t, "launch-uuid-1\n", buf.String())
}

func TestStartLaunchStep_WithAttributes(t *testing.T) {
	t.Parallel()
	project := "testproj"
	srv, starts, _, _, _, _ := stepsMockServer(t, project)

	var buf bytes.Buffer
	cmd := stepCmd(&buf, []cli.Flag{
		&cli.StringFlag{Name: "name", Value: "Attr Launch"},
		&cli.StringFlag{Name: "mode", Value: "DEBUG"},
		&cli.StringFlag{Name: "description", Value: "launch desc"},
		&cli.StringSliceFlag{Name: "attr", Value: []string{"branch:main", "ci"}},
	})

	id, err := startLaunchStep(context.Background(), stepCfg(srv.URL, project), cmd)

	require.NoError(t, err)
	assert.Equal(t, "launch-uuid-1", id)
	assert.Equal(t, int32(1), starts.Load())
	assert.Equal(t, "launch-uuid-1\n", buf.String())
}

// ---------------------------------------------------------------------------
// start-test (root)
// ---------------------------------------------------------------------------

func TestStartTestStep_Root(t *testing.T) {
	t.Parallel()
	project := "testproj"
	srv, _, _, itemStarts, _, _ := stepsMockServer(t, project)

	var buf bytes.Buffer
	cmd := stepCmd(&buf, []cli.Flag{
		&cli.StringFlag{Name: "launch-uuid", Value: "launch-uuid-1"},
		&cli.StringFlag{Name: "name", Value: "my/package"},
		&cli.StringFlag{Name: "type", Value: "SUITE"},
		&cli.StringFlag{Name: "parent-uuid"},
		&cli.StringFlag{Name: "description"},
		&cli.StringFlag{Name: "code-ref"},
		&cli.StringSliceFlag{Name: "attr"},
	})

	id, err := startTestStep(context.Background(), stepCfg(srv.URL, project), cmd)

	require.NoError(t, err)
	assert.Equal(t, "root-item-1", id)
	assert.Equal(t, int32(1), itemStarts.Load())
	assert.Equal(t, "root-item-1\n", buf.String())
}

// ---------------------------------------------------------------------------
// start-test (child)
// ---------------------------------------------------------------------------

func TestStartTestStep_Child(t *testing.T) {
	t.Parallel()
	project := "testproj"
	srv, _, _, itemStarts, _, _ := stepsMockServer(t, project)

	var buf bytes.Buffer
	cmd := stepCmd(&buf, []cli.Flag{
		&cli.StringFlag{Name: "launch-uuid", Value: "launch-uuid-1"},
		&cli.StringFlag{Name: "name", Value: "TestFoo"},
		&cli.StringFlag{Name: "type", Value: "TEST"},
		&cli.StringFlag{Name: "parent-uuid", Value: "root-item-1"},
		&cli.StringFlag{Name: "description"},
		&cli.StringFlag{Name: "code-ref", Value: "my/package/TestFoo"},
		&cli.StringSliceFlag{Name: "attr"},
	})

	id, err := startTestStep(context.Background(), stepCfg(srv.URL, project), cmd)

	require.NoError(t, err)
	assert.Equal(t, "child-item-1", id)
	assert.Equal(t, int32(1), itemStarts.Load())
	assert.Equal(t, "child-item-1\n", buf.String())
}

// ---------------------------------------------------------------------------
// log (message only)
// ---------------------------------------------------------------------------

func TestReportLogStep_Message(t *testing.T) {
	t.Parallel()
	project := "testproj"
	srv, _, _, _, _, logSaves := stepsMockServer(t, project)

	var buf bytes.Buffer
	cmd := stepCmd(&buf, []cli.Flag{
		&cli.StringFlag{Name: "launch-uuid", Value: "launch-uuid-1"},
		&cli.StringFlag{Name: "item-uuid", Value: "root-item-1"},
		&cli.StringFlag{Name: "message", Value: "hello world"},
		&cli.StringFlag{Name: "level", Value: "INFO"},
		&cli.StringFlag{Name: "file"},
	})

	id, err := reportLogStep(context.Background(), stepCfg(srv.URL, project), cmd)

	require.NoError(t, err)
	assert.Equal(t, "log-1", id)
	assert.Equal(t, int32(1), logSaves.Load())
	assert.Equal(t, "log-1\n", buf.String())
}

// ---------------------------------------------------------------------------
// log (with file attachment)
// ---------------------------------------------------------------------------

func TestReportLogStep_WithFile(t *testing.T) {
	t.Parallel()
	project := "testproj"
	srv, _, _, _, _, logSaves := stepsMockServer(t, project)

	tmpFile, err := os.CreateTemp("", "test-attachment-*.txt")
	require.NoError(t, err)
	_, err = tmpFile.WriteString("file content")
	require.NoError(t, err)
	require.NoError(t, tmpFile.Close())
	t.Cleanup(func() { os.Remove(tmpFile.Name()) })

	var buf bytes.Buffer
	cmd := stepCmd(&buf, []cli.Flag{
		&cli.StringFlag{Name: "launch-uuid", Value: "launch-uuid-1"},
		&cli.StringFlag{Name: "item-uuid", Value: "root-item-1"},
		&cli.StringFlag{Name: "message", Value: "screenshot attached"},
		&cli.StringFlag{Name: "level", Value: "ERROR"},
		&cli.StringFlag{Name: "file", Value: tmpFile.Name()},
	})

	id, err := reportLogStep(context.Background(), stepCfg(srv.URL, project), cmd)

	require.NoError(t, err)
	assert.Equal(t, "log-1", id)
	assert.Equal(t, int32(1), logSaves.Load())
	assert.Equal(t, "log-1\n", buf.String())
}

func TestReportLogStep_WithFile_NotFound(t *testing.T) {
	t.Parallel()
	project := "testproj"
	srv, _, _, _, _, _ := stepsMockServer(t, project)

	var buf bytes.Buffer
	cmd := stepCmd(&buf, []cli.Flag{
		&cli.StringFlag{Name: "launch-uuid", Value: "launch-uuid-1"},
		&cli.StringFlag{Name: "item-uuid"},
		&cli.StringFlag{Name: "message", Value: "msg"},
		&cli.StringFlag{Name: "level", Value: "INFO"},
		&cli.StringFlag{Name: "file", Value: "/nonexistent/path.txt"},
	})

	_, err := reportLogStep(context.Background(), stepCfg(srv.URL, project), cmd)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unable to open file")
}

// ---------------------------------------------------------------------------
// finish-test
// ---------------------------------------------------------------------------

func TestFinishTestStep(t *testing.T) {
	t.Parallel()
	project := "testproj"
	srv, _, _, _, itemFinishes, _ := stepsMockServer(t, project)

	var buf bytes.Buffer
	cmd := stepCmd(&buf, []cli.Flag{
		&cli.StringFlag{Name: "launch-uuid", Value: "launch-uuid-1"},
		&cli.StringFlag{Name: "item-uuid", Value: "root-item-1"},
		&cli.StringFlag{Name: "status", Value: "PASSED"},
	})

	err := finishTestStep(context.Background(), stepCfg(srv.URL, project), cmd)

	require.NoError(t, err)
	assert.Equal(t, int32(1), itemFinishes.Load())
	assert.Equal(t, "root-item-1\n", buf.String())
}

// ---------------------------------------------------------------------------
// finish-launch
// ---------------------------------------------------------------------------

func TestFinishLaunchStep(t *testing.T) {
	t.Parallel()
	project := "testproj"
	srv, _, launchFinishes, _, _, _ := stepsMockServer(t, project)

	var buf bytes.Buffer
	cmd := stepCmd(&buf, []cli.Flag{
		&cli.StringFlag{Name: "launch-uuid", Value: "launch-uuid-1"},
		&cli.StringFlag{Name: "status", Value: "PASSED"},
	})

	err := finishLaunchStep(context.Background(), stepCfg(srv.URL, project), cmd)

	require.NoError(t, err)
	assert.Equal(t, int32(1), launchFinishes.Load())
	assert.Equal(t, "launch-uuid-1\n", buf.String())
}

// ---------------------------------------------------------------------------
// Full lifecycle: start-launch → start-test → log → finish-test → finish-launch
// ---------------------------------------------------------------------------

func TestFullReportingLifecycle(t *testing.T) {
	t.Parallel()
	project := "testproj"
	srv, launchStarts, launchFinishes, itemStarts, itemFinishes, logSaves := stepsMockServer(
		t,
		project,
	)
	cfg := stepCfg(srv.URL, project)
	ctx := context.Background()

	var buf bytes.Buffer

	launchCmd := stepCmd(&buf, []cli.Flag{
		&cli.StringFlag{Name: "name", Value: "Lifecycle Launch"},
		&cli.StringFlag{Name: "mode", Value: "DEFAULT"},
		&cli.StringFlag{Name: "description"},
		&cli.StringSliceFlag{Name: "attr"},
	})
	launchUUID, err := startLaunchStep(ctx, cfg, launchCmd)
	require.NoError(t, err)
	assert.Equal(t, "launch-uuid-1", launchUUID)

	buf.Reset()
	testCmd := stepCmd(&buf, []cli.Flag{
		&cli.StringFlag{Name: "launch-uuid", Value: launchUUID},
		&cli.StringFlag{Name: "name", Value: "TestBar"},
		&cli.StringFlag{Name: "type", Value: "TEST"},
		&cli.StringFlag{Name: "parent-uuid"},
		&cli.StringFlag{Name: "description"},
		&cli.StringFlag{Name: "code-ref"},
		&cli.StringSliceFlag{Name: "attr"},
	})
	testUUID, err := startTestStep(ctx, cfg, testCmd)
	require.NoError(t, err)
	assert.NotEmpty(t, testUUID)

	buf.Reset()
	logCmd := stepCmd(&buf, []cli.Flag{
		&cli.StringFlag{Name: "launch-uuid", Value: launchUUID},
		&cli.StringFlag{Name: "item-uuid", Value: testUUID},
		&cli.StringFlag{Name: "message", Value: "=== RUN TestBar"},
		&cli.StringFlag{Name: "level", Value: "INFO"},
		&cli.StringFlag{Name: "file"},
	})
	logID, err := reportLogStep(ctx, cfg, logCmd)
	require.NoError(t, err)
	assert.Equal(t, "log-1", logID)

	buf.Reset()
	finTestCmd := stepCmd(&buf, []cli.Flag{
		&cli.StringFlag{Name: "launch-uuid", Value: launchUUID},
		&cli.StringFlag{Name: "item-uuid", Value: testUUID},
		&cli.StringFlag{Name: "status", Value: "PASSED"},
	})
	require.NoError(t, finishTestStep(ctx, cfg, finTestCmd))

	buf.Reset()
	finLaunchCmd := stepCmd(&buf, []cli.Flag{
		&cli.StringFlag{Name: "launch-uuid", Value: launchUUID},
		&cli.StringFlag{Name: "status", Value: "PASSED"},
	})
	require.NoError(t, finishLaunchStep(ctx, cfg, finLaunchCmd))

	assert.Equal(t, int32(1), launchStarts.Load())
	assert.Equal(t, int32(1), launchFinishes.Load())
	assert.Equal(t, int32(1), itemStarts.Load())
	assert.Equal(t, int32(1), itemFinishes.Load())
	assert.Equal(t, int32(1), logSaves.Load())
}

// ---------------------------------------------------------------------------
// parseAttributes
// ---------------------------------------------------------------------------

func TestParseAttributes(t *testing.T) {
	t.Parallel()

	attrs := parseAttributes([]string{"branch:main", "ci", "env:prod"})

	require.Len(t, attrs, 3)

	assert.Equal(t, "branch", *attrs[0].Key)
	assert.Equal(t, "main", attrs[0].Value)

	assert.Nil(t, attrs[1].Key)
	assert.Equal(t, "ci", attrs[1].Value)

	assert.Equal(t, "env", *attrs[2].Key)
	assert.Equal(t, "prod", attrs[2].Value)
}

func TestParseAttributes_Empty(t *testing.T) {
	t.Parallel()
	assert.Nil(t, parseAttributes(nil))
	assert.Nil(t, parseAttributes([]string{}))
}
