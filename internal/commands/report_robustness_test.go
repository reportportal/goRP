package commands

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v3"

	"github.com/reportportal/goRP/v5/pkg/openapi"
)

// recordingServer mimics the reporting endpoints and records what it receives.
type recordingServer struct {
	*httptest.Server

	mu            sync.Mutex
	logCalls      int
	logBodies     []string
	launchFinish  []string // request bodies of launch finish calls
	failLogSaving bool
	onItemStart   func() // optional hook; the handler then blocks until the client gives up
}

func newRecordingServer(t *testing.T, project string, failLogSaving bool) *recordingServer {
	t.Helper()

	rs := &recordingServer{failLogSaving: failLogSaving}
	reply := func(w http.ResponseWriter, body map[string]string) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(body)
	}
	readBody := func(r *http.Request) string {
		b, _ := io.ReadAll(r.Body)
		return string(b)
	}

	prefix := "/api/v2/" + project
	mux := http.NewServeMux()
	mux.HandleFunc("POST "+prefix+"/launch", func(w http.ResponseWriter, _ *http.Request) {
		reply(w, map[string]string{"id": "launch-uuid-1"})
	})
	mux.HandleFunc("PUT "+prefix+"/launch/", func(w http.ResponseWriter, r *http.Request) {
		rs.mu.Lock()
		rs.launchFinish = append(rs.launchFinish, readBody(r))
		rs.mu.Unlock()
		reply(w, map[string]string{"id": "launch-uuid-1", "message": "finished"})
	})
	mux.HandleFunc("POST "+prefix+"/item/", func(w http.ResponseWriter, r *http.Request) {
		if rs.onItemStart != nil {
			_, _ = io.Copy(
				io.Discard,
				r.Body,
			) // the server notices a disconnect only after the body is read
			rs.onItemStart()
			select {
			case <-r.Context().Done():
			case <-time.After(5 * time.Second):
			}
			return
		}
		if r.URL.Path == prefix+"/item/" {
			reply(w, map[string]string{"id": "suite-1"})
			return
		}
		reply(w, map[string]string{"id": "test-1"})
	})
	mux.HandleFunc("PUT "+prefix+"/item/", func(w http.ResponseWriter, _ *http.Request) {
		reply(w, map[string]string{"message": "finished"})
	})
	mux.HandleFunc("POST "+prefix+"/log", func(w http.ResponseWriter, r *http.Request) {
		rs.mu.Lock()
		rs.logCalls++
		rs.logBodies = append(rs.logBodies, readBody(r))
		rs.mu.Unlock()
		if rs.failLogSaving {
			http.Error(w, "boom", http.StatusInternalServerError)
			return
		}
		reply(w, map[string]string{"id": "log-1"})
	})

	rs.Server = httptest.NewServer(mux)
	t.Cleanup(rs.Close)

	return rs
}

func writeEvents(t *testing.T, lines []string) string {
	t.Helper()

	f, err := os.CreateTemp(t.TempDir(), "test2json-*.jsonl")
	require.NoError(t, err)
	_, err = f.WriteString(strings.Join(lines, "\n") + "\n")
	require.NoError(t, err)
	require.NoError(t, f.Close())

	return f.Name()
}

func TestReportLaunch_LogFailureDoesNotHangAndInterruptsLaunch(t *testing.T) {
	t.Parallel()

	project := "testproj"
	srv := newRecordingServer(t, project, true)

	// the last log batch is flushed after the input is exhausted, i.e. after the scan loop has ended
	file := writeEvents(t, []string{
		`{"time":"2025-01-01T00:00:00Z","action":"run","package":"p","test":"TestA"}`,
		`{"time":"2025-01-01T00:00:01Z","action":"output","package":"p","test":"TestA","output":"hello\n"}`,
		`{"time":"2025-01-01T00:00:02Z","action":"pass","package":"p","test":"TestA"}`,
		`{"time":"2025-01-01T00:00:03Z","action":"pass","package":"p"}`,
	})

	cfg := &clientConfig{URL: srv.URL, Project: project, ApiKey: "tok"}
	cmd := reportCmd(file, "launch", false)

	errCh := make(chan error, 1)
	go func() {
		_, err := reportLaunch(t.Context(), cfg, cmd)
		errCh <- err
	}()

	select {
	case err := <-errCh:
		require.Error(t, err)
		assert.Contains(t, err.Error(), "reporting logs")
	case <-time.After(10 * time.Second):
		t.Fatal("reportLaunch hung after a late reporter error")
	}

	srv.mu.Lock()
	defer srv.mu.Unlock()
	require.Len(t, srv.launchFinish, 1, "launch must be finished even though reporting failed")
	assert.Contains(t, srv.launchFinish[0], `"status":"INTERRUPTED"`)
}

func TestReportLaunch_LargeAndMalformedLines(t *testing.T) {
	t.Parallel()

	project := "testproj"
	srv := newRecordingServer(t, project, false)

	huge := strings.Repeat("x", 200*1024) // larger than bufio.Scanner's default 64 KB limit
	file := writeEvents(t, []string{
		`this is build noise and not JSON`,
		`{"time":"2025-01-01T00:00:00Z","action":"run","package":"p","test":"TestA"}`,
		`{"time":"2025-01-01T00:00:01Z","action":"output","package":"p","test":"TestA","output":"` + huge + `"}`,
		`{"time":"2025-01-01T00:00:02Z","action":"pass","package":"p","test":"TestA"}`,
		`{"time":"2025-01-01T00:00:03Z","action":"pass","package":"p"}`,
	})

	cfg := &clientConfig{URL: srv.URL, Project: project, ApiKey: "tok"}
	launchID, err := reportLaunch(t.Context(), cfg, reportCmd(file, "launch", false))

	require.NoError(t, err)
	assert.Equal(t, "launch-uuid-1", launchID)
	srv.mu.Lock()
	defer srv.mu.Unlock()
	assert.Equal(t, 1, srv.logCalls)
	assert.Contains(t, srv.logBodies[0], huge)
}

func TestReportLaunch_NoEmptyLogBatch(t *testing.T) {
	t.Parallel()

	project := "testproj"
	srv := newRecordingServer(t, project, false)

	file := writeEvents(t, []string{
		`{"time":"2025-01-01T00:00:00Z","action":"run","package":"p","test":"TestA"}`,
		`{"time":"2025-01-01T00:00:01Z","action":"pass","package":"p","test":"TestA"}`,
		`{"time":"2025-01-01T00:00:02Z","action":"pass","package":"p"}`,
	})

	cfg := &clientConfig{URL: srv.URL, Project: project, ApiKey: "tok"}
	_, err := reportLaunch(t.Context(), cfg, reportCmd(file, "launch", false))

	require.NoError(t, err)
	srv.mu.Lock()
	defer srv.mu.Unlock()
	assert.Zero(t, srv.logCalls, "no logs were produced, so no log request must be sent")
}

func TestReportLaunch_LaunchStatusFollowsResults(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		action     string
		wantStatus string
	}{
		{name: "all passed", action: "pass", wantStatus: `"status":"PASSED"`},
		{name: "test failed", action: "fail", wantStatus: `"status":"FAILED"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			project := "testproj"
			srv := newRecordingServer(t, project, false)
			file := writeEvents(t, []string{
				`{"time":"2025-01-01T00:00:00Z","action":"run","package":"p","test":"TestA"}`,
				`{"time":"2025-01-01T00:00:01Z","action":"` + tt.action + `","package":"p","test":"TestA"}`,
				`{"time":"2025-01-01T00:00:02Z","action":"` + tt.action + `","package":"p"}`,
			})

			cfg := &clientConfig{URL: srv.URL, Project: project, ApiKey: "tok"}
			_, err := reportLaunch(t.Context(), cfg, reportCmd(file, "launch", false))

			require.NoError(t, err)
			srv.mu.Lock()
			defer srv.mu.Unlock()
			require.Len(t, srv.launchFinish, 1)
			assert.Contains(t, srv.launchFinish[0], tt.wantStatus)
		})
	}
}

func TestReportLaunch_PackageWithoutTestsIsSkipped(t *testing.T) {
	t.Parallel()

	project := "testproj"
	srv := newRecordingServer(t, project, false)
	// a package with no tests (e.g. `-run` matched nothing) only produces package-level events
	file := writeEvents(t, []string{
		`{"time":"2025-01-01T00:00:00Z","action":"start","package":"p"}`,
		`{"time":"2025-01-01T00:00:01Z","action":"output","package":"p","output":"ok  \tp\t0.1s [no tests to run]\n"}`,
		`{"time":"2025-01-01T00:00:02Z","action":"pass","package":"p"}`,
	})

	cfg := &clientConfig{URL: srv.URL, Project: project, ApiKey: "tok"}
	_, err := reportLaunch(t.Context(), cfg, reportCmd(file, "launch", false))

	require.NoError(t, err)
	srv.mu.Lock()
	defer srv.mu.Unlock()
	require.Len(t, srv.launchFinish, 1)
	// there is no suite for the package, so the output must not reference an empty item
	require.Equal(t, 1, srv.logCalls)
	assert.NotContains(t, srv.logBodies[0], "itemUuid")
}

func TestReporterLog_ItemUUID(t *testing.T) {
	t.Parallel()

	rep := newTestReporter()
	rep.suites = map[string]string{"p": "suite-1"}
	rep.tests["p/TestA"] = "test-1"

	rep.log(&testEvent{Action: testActionOutput, Package: "p", Test: "TestA", Output: "in test\n"})
	rep.log(&testEvent{Action: testActionOutput, Package: "p", Output: "package level\n"})
	rep.log(&testEvent{Action: testActionOutput, Package: "other", Output: "no suite\n"})

	require.Len(t, rep.logs, 3)
	assert.Equal(t, "test-1", rep.logs[0].GetItemUuid())
	assert.Equal(t, "suite-1", rep.logs[1].GetItemUuid())
	assert.Nil(t, rep.logs[2].ItemUuid, "log without an item must not carry an empty item uuid")
}

func TestEntryID(t *testing.T) {
	t.Parallel()

	_, err := entryID(&openapi.EntryCreatedAsyncRS{})
	require.ErrorIs(t, err, errNoEntryID)

	_, err = entryID(nil)
	require.ErrorIs(t, err, errNoEntryID)

	id := "abc"
	got, err := entryID(&openapi.EntryCreatedAsyncRS{Id: &id})
	require.NoError(t, err)
	assert.Equal(t, "abc", got)
}

func TestLaunchUUIDOutputMatchesQualityGateRegex(t *testing.T) {
	t.Parallel()

	uuid := "0b1f1f7e-2c3d-4e5f-8a9b-0c1d2e3f4a5b"
	line := "ReportPortal Launch UUID: " + uuid

	found := uuidRegex.FindStringSubmatch(line)
	require.Len(t, found, 2)
	assert.Equal(t, uuid, found[1])
}

func TestCheckQualityGate_PermanentErrorFailsFast(t *testing.T) {
	t.Parallel()

	_, cfg := setupMockServer(
		t,
		"testproject",
		func(_ string, w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "not found", http.StatusNotFound)
		},
	)
	cmd := createTestCommand(time.Minute, 10*time.Millisecond)

	start := time.Now()
	err := checkQualityGateInternal(t.Context(), "launch123", cfg, cmd)

	require.Error(t, err)
	assert.Less(
		t,
		time.Since(start),
		10*time.Second,
		"a 404 will never resolve, it must not be retried until timeout",
	)
	var exitErr cli.ExitCoder
	require.ErrorAs(t, err, &exitErr)
	assert.Equal(t, 10, exitErr.ExitCode())
}

func TestCheckQualityGate_TransientErrorIsRetried(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32
	_, cfg := setupMockServer(
		t,
		"testproject",
		func(_ string, w http.ResponseWriter, _ *http.Request) {
			if calls.Add(1) < 3 {
				http.Error(w, "unavailable", http.StatusServiceUnavailable)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(getDefaultLaunchResponse("launch123", "PASSED"))
		},
	)
	cmd := createTestCommand(10*time.Second, 10*time.Millisecond)

	err := checkQualityGateInternal(t.Context(), "launch123", cfg, cmd)

	require.NoError(t, err)
	assert.Equal(t, int32(3), calls.Load())
}

func TestFeedEvents_CancelUnblocksStalledInput(t *testing.T) {
	t.Parallel()

	pr, pw := io.Pipe() // never written to: a Read blocks, like a terminal or an idle pipe on stdin
	t.Cleanup(func() { _ = pw.Close() })

	ctx, cancel := context.WithCancel(t.Context())
	errCh := make(chan error, 1)
	go func() {
		errCh <- feedEvents(ctx, pr, make(chan *testEvent), make(chan struct{}))
	}()

	cancel()

	select {
	case err := <-errCh:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(5 * time.Second):
		t.Fatal("feedEvents is stuck in a blocked read after the context was cancelled")
	}
}

func TestFeedEvents_DeliversEventsAndStopsWhenReporterIsDone(t *testing.T) {
	t.Parallel()

	input := make(chan *testEvent, 2)
	done := make(chan struct{})
	r := strings.NewReader(
		`{"time":"2025-01-01T00:00:00Z","action":"run","package":"p","test":"A"}` + "\n" +
			"not json\n" +
			`{"time":"2025-01-01T00:00:01Z","action":"pass","package":"p","test":"A"}` + "\n",
	)

	require.NoError(t, feedEvents(t.Context(), r, input, done))

	require.Len(t, input, 2)
	assert.Equal(t, "run", (<-input).Action)
	assert.Equal(t, "pass", (<-input).Action)

	// nobody receives from an unbuffered channel: closing done must release the sender
	close(done)
	require.NoError(t, feedEvents(t.Context(), strings.NewReader(
		`{"action":"run","package":"p","test":"A"}`+"\n",
	), make(chan *testEvent), done))
}

func TestFindLaunchUUID(t *testing.T) {
	t.Parallel()

	uuid := "0b1f1f7e-2c3d-4e5f-8a9b-0c1d2e3f4a5b"

	t.Run("found", func(t *testing.T) {
		t.Parallel()
		got, err := findLaunchUUID(
			t.Context(),
			strings.NewReader("noise\nReportPortal Launch UUID: "+uuid+"\n"),
		)
		require.NoError(t, err)
		assert.Equal(t, uuid, got)
	})

	t.Run("not found", func(t *testing.T) {
		t.Parallel()
		got, err := findLaunchUUID(t.Context(), strings.NewReader("noise\n"))
		require.NoError(t, err)
		assert.Empty(t, got)
	})

	t.Run("cancel unblocks stalled stdin", func(t *testing.T) {
		t.Parallel()
		pr, pw := io.Pipe()
		t.Cleanup(func() { _ = pw.Close() })

		ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
		defer cancel()

		_, err := findLaunchUUID(ctx, pr)
		require.ErrorIs(t, err, context.DeadlineExceeded)
	})
}

func TestReportLaunch_CancelledContextInterruptsLaunch(t *testing.T) {
	t.Parallel()

	project := "testproj"
	srv := newRecordingServer(t, project, false)
	file := writeEvents(t, []string{
		`{"time":"2025-01-01T00:00:00Z","action":"run","package":"p","test":"TestA"}`,
	})

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	// cancel while the first item is being started: the client aborts the request and the
	// handler returns only once it has seen that, so the outcome does not depend on timing
	srv.onItemStart = cancel

	cfg := &clientConfig{URL: srv.URL, Project: project, ApiKey: "tok"}
	_, err := reportLaunch(ctx, cfg, reportCmd(file, "launch", false))
	require.Error(t, err)

	srv.mu.Lock()
	defer srv.mu.Unlock()
	require.Len(t, srv.launchFinish, 1, "a cancelled run must not leave the launch IN_PROGRESS")
	assert.Contains(t, srv.launchFinish[0], `"status":"INTERRUPTED"`)
}
