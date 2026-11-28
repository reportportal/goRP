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
		&cli.BoolFlag{Name: "has-stats", Value: true},
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
		&cli.BoolFlag{Name: "has-stats", Value: true},
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
	assert.Equal(
		t,
		"launch-uuid-1\nhttps://rp/launch/1\n",
		buf.String(),
		"UUID first, then the launch link",
	)
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
		&cli.BoolFlag{Name: "has-stats", Value: true},
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

// ---------------------------------------------------------------------------
// Real flag parsing: command line → request body
// ---------------------------------------------------------------------------

type recordedRequest struct {
	Method string
	Path   string
	Body   map[string]any
}

// runReportCommand runs `gorp report <args>` with the real flag definitions against a server that records
// the request. The tests using it are not parallel: flag instances belong to the package-level commands.
func runReportCommand(
	t *testing.T,
	response string,
	args ...string,
) (stdout string, rqs []recordedRequest, err error) {
	t.Helper()
	t.Setenv("HOME", t.TempDir()) // no ~/.gorp
	for _, env := range []string{"LAUNCH_UUID", "PARENT_UUID", "ITEM_UUID"} {
		t.Setenv(env, "") // the flags read them
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rq := recordedRequest{Method: r.Method, Path: r.URL.Path}
		assert.NoError(t, json.NewDecoder(r.Body).Decode(&rq.Body))
		rqs = append(rqs, rq)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(response))
	}))
	defer srv.Close()

	// Copies, so that a command does not keep the writer it inherited from the root of an earlier run.
	var steps []*cli.Command
	for _, step := range []*cli.Command{startLaunchCommand, startTestCommand, finishTestCommand, finishLaunchCommand} {
		steps = append(steps, &cli.Command{Name: step.Name, Flags: step.Flags, Action: step.Action})
	}

	var buf bytes.Buffer
	root := &cli.Command{
		Name:     "gorp",
		Writer:   &buf,
		Flags:    connectionFlags(srv.URL),
		Commands: []*cli.Command{{Name: "report", Commands: steps}},
	}
	err = root.Run(t.Context(), append([]string{"gorp", "report"}, args...))

	return buf.String(), rqs, err
}

func TestReportStepFlags(t *testing.T) {
	t.Run(
		"start-launch --rerun-of implies rerun and prints the UUID from the response",
		func(t *testing.T) {
			out, rqs, err := runReportCommand(t, `{"id":"original-launch"}`,
				"start-launch", "--name", "L", "--mode", "debug", "--rerun-of", "original-launch")

			require.NoError(t, err)
			require.Len(t, rqs, 1)
			assert.Equal(t, "/api/v2/proj/launch", rqs[0].Path)
			assert.Equal(t, true, rqs[0].Body["rerun"])
			assert.Equal(t, "original-launch", rqs[0].Body["rerunOf"])
			assert.Equal(t, "DEBUG", rqs[0].Body["mode"], "mode is sent in upper case")
			assert.Equal(t, "original-launch\n", out)
		},
	)

	t.Run("start-launch --rerun reruns by name", func(t *testing.T) {
		_, rqs, err := runReportCommand(t, `{"id":"x"}`, "start-launch", "--name", "L", "--rerun")

		require.NoError(t, err)
		require.Len(t, rqs, 1)
		assert.Equal(t, true, rqs[0].Body["rerun"])
		assert.NotContains(t, rqs[0].Body, "rerunOf")
	})

	t.Run("start-launch without rerun flags sends neither field", func(t *testing.T) {
		_, rqs, err := runReportCommand(t, `{"id":"x"}`, "start-launch", "--name", "L")

		require.NoError(t, err)
		require.Len(t, rqs, 1)
		assert.NotContains(t, rqs[0].Body, "rerun")
		assert.NotContains(t, rqs[0].Body, "rerunOf")
		assert.Equal(t, "DEFAULT", rqs[0].Body["mode"])
	})

	t.Run("start-launch rejects an unknown mode before any request", func(t *testing.T) {
		_, rqs, err := runReportCommand(
			t,
			`{"id":"x"}`,
			"start-launch",
			"--name",
			"L",
			"--mode",
			"DEBAG",
		)

		require.EqualError(t, err, `invalid --mode "DEBAG": must be one of DEFAULT, DEBUG`)
		assert.Empty(t, rqs)
	})

	t.Run("start-test sends test case id, parameters, retry and hasStats", func(t *testing.T) {
		_, rqs, err := runReportCommand(
			t,
			`{"id":"item"}`,
			"start-test",
			"--launch-uuid",
			"l1",
			"--parent-uuid",
			"p1",
			"--name",
			"T",
			"--type",
			"step",
			"--test-case-id",
			"pkg.T",
			"--param",
			"browser=chrome",
			"--param",
			"url=http://x?a=b",
			"--param",
			"empty=",
			"--retry-of",
			"prev-attempt",
			"--has-stats=false",
		)

		require.NoError(t, err)
		require.Len(t, rqs, 1)
		body := rqs[0].Body
		assert.Equal(t, "/api/v2/proj/item/p1", rqs[0].Path)
		assert.Equal(t, "STEP", body["type"], "type is sent in upper case")
		assert.Equal(t, "pkg.T", body["testCaseId"])
		assert.Equal(t, []any{
			map[string]any{"key": "browser", "value": "chrome"},
			map[string]any{"key": "url", "value": "http://x?a=b"},
			map[string]any{"key": "empty", "value": ""},
		}, body["parameters"])
		assert.Equal(t, true, body["retry"])
		assert.Equal(t, "prev-attempt", body["retryOf"])
		assert.Equal(t, false, body["hasStats"])
	})

	t.Run("start-test splits a --param value on commas, like --attr", func(t *testing.T) {
		_, rqs, err := runReportCommand(t, `{"id":"item"}`,
			"start-test", "--launch-uuid", "l1", "--name", "T", "--param", "a=1,b=2")

		require.NoError(t, err)
		require.Len(t, rqs, 1)
		assert.Equal(t, []any{
			map[string]any{"key": "a", "value": "1"},
			map[string]any{"key": "b", "value": "2"},
		}, rqs[0].Body["parameters"])
	})

	t.Run("start-test rejects '--has-stats false': the value needs '='", func(t *testing.T) {
		_, rqs, err := runReportCommand(t, `{"id":"item"}`,
			"start-test", "--launch-uuid", "l1", "--name", "T", "--has-stats", "false")

		require.EqualError(
			t,
			err,
			`unexpected argument "false": a boolean flag takes its value with '=', e.g. --has-stats=false`,
		)
		assert.Empty(t, rqs)
	})

	t.Run("start-test defaults are unchanged", func(t *testing.T) {
		_, rqs, err := runReportCommand(
			t,
			`{"id":"item"}`,
			"start-test",
			"--launch-uuid",
			"l1",
			"--name",
			"T",
		)

		require.NoError(t, err)
		require.Len(t, rqs, 1)
		body := rqs[0].Body
		assert.Equal(t, "TEST", body["type"])
		assert.Equal(t, true, body["hasStats"])
		assert.Equal(t, false, body["retry"])
		for _, field := range []string{"retryOf", "testCaseId", "parameters"} {
			assert.NotContains(t, body, field)
		}
	})

	for name, tc := range map[string]struct {
		args    []string
		wantErr string
	}{
		"unknown type": {
			args: []string{"--type", "CASE"},
			wantErr: `invalid --type "CASE": must be one of SUITE, STORY, TEST, SCENARIO, STEP, BEFORE_CLASS, BEFORE_GROUPS, ` +
				`BEFORE_METHOD, BEFORE_SUITE, BEFORE_TEST, AFTER_CLASS, AFTER_GROUPS, AFTER_METHOD, AFTER_SUITE, AFTER_TEST`,
		},
		"parameter without '='": {
			args:    []string{"--param", "chrome"},
			wantErr: `invalid --param "chrome": expected 'key=value'`,
		},
		"parameter without a key": {
			args:    []string{"--param", "=chrome"},
			wantErr: `invalid --param "=chrome": expected 'key=value'`,
		},
		"parameter value with a comma": {
			args:    []string{"--param", "list=a,b"},
			wantErr: `invalid --param "b": expected 'key=value'`,
		},
		"retry of a root item": {
			args:    []string{"--retry-of", "prev"},
			wantErr: "--retry-of requires --parent-uuid: a root item can't be a retry",
		},
	} {
		t.Run("start-test rejects "+name+" before any request", func(t *testing.T) {
			_, rqs, err := runReportCommand(t, `{"id":"item"}`,
				append([]string{"start-test", "--launch-uuid", "l1", "--name", "T"}, tc.args...)...)

			require.EqualError(t, err, tc.wantErr)
			assert.Empty(t, rqs)
		})
	}

	t.Run("finish-test sends issue type, description and attributes", func(t *testing.T) {
		out, rqs, err := runReportCommand(
			t,
			`{"message":"ok"}`,
			"finish-test",
			"--launch-uuid",
			"l1",
			"--item-uuid",
			"i1",
			"--status",
			"FAILED",
			"--issue-type",
			"pb001",
			"--description",
			"why",
			"--attr",
			"owner:qa",
			"--attr",
			"flaky",
		)

		require.NoError(t, err)
		require.Len(t, rqs, 1)
		body := rqs[0].Body
		assert.Equal(t, "/api/v2/proj/item/i1", rqs[0].Path)
		assert.Equal(t, map[string]any{"issueType": "pb001"}, body["issue"])
		assert.Equal(t, "why", body["description"])
		assert.Equal(t, []any{
			map[string]any{"key": "owner", "value": "qa", "system": false},
			map[string]any{"value": "flaky", "system": false},
		}, body["attributes"])
		assert.Equal(t, "i1\n", out)
	})

	t.Run("finish-test without the new flags sends none of the fields", func(t *testing.T) {
		_, rqs, err := runReportCommand(t, `{"message":"ok"}`,
			"finish-test", "--launch-uuid", "l1", "--item-uuid", "i1", "--status", "PASSED")

		require.NoError(t, err)
		require.Len(t, rqs, 1)
		for _, field := range []string{"issue", "description", "attributes"} {
			assert.NotContains(t, rqs[0].Body, field)
		}
	})

	t.Run("finish-launch sends description and attributes and prints the link", func(t *testing.T) {
		out, rqs, err := runReportCommand(
			t,
			`{"id":"l1","link":"https://rp/ui/#proj/launches/all/l1"}`,
			"finish-launch",
			"--launch-uuid",
			"l1",
			"--status",
			"PASSED",
			"--description",
			"done",
			"--attr",
			"build:42",
		)

		require.NoError(t, err)
		require.Len(t, rqs, 1)
		assert.Equal(t, "/api/v2/proj/launch/l1/finish", rqs[0].Path)
		assert.Equal(t, "done", rqs[0].Body["description"])
		assert.Equal(
			t,
			[]any{map[string]any{"key": "build", "value": "42", "system": false}},
			rqs[0].Body["attributes"],
		)
		assert.Equal(t, "l1\nhttps://rp/ui/#proj/launches/all/l1\n", out)
	})

	t.Run("finish-launch prints only the UUID when the server returns no link", func(t *testing.T) {
		out, _, err := runReportCommand(t, `{"id":"l1"}`, "finish-launch", "--launch-uuid", "l1")

		require.NoError(t, err)
		assert.Equal(t, "l1\n", out)
	})
}
