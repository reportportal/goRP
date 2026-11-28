package commands

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v3"

	gorppkg "github.com/reportportal/goRP/v5/pkg/gorp"
)

const (
	joinedUUID = "11111111-2222-3333-4444-555555555555"
	// the events of event() are stamped 2025-01-01T00:00:00Z
	launchBeforeEvents = `"2024-12-31T23:00:00Z"`
	launchAfterEvents  = `"2025-06-01T00:00:00Z"`
)

// joinServer is a ReportPortal fake for --launch-uuid: it knows the launch joinedUUID (after notFound
// lookups that answer 404) and counts the requests a joining reporter must never send.
type joinServer struct {
	*recordingServer

	lookups      atomic.Int32
	launchStarts atomic.Int32
}

func newJoinServer(
	t *testing.T,
	project, startTime string,
	notFound int32,
	failLogSaving bool,
) *joinServer {
	t.Helper()

	js := &joinServer{recordingServer: newRecordingServer(t, project, failLogSaving)}
	reporting := js.Config.Handler
	mux := http.NewServeMux()
	mux.HandleFunc(
		"GET /api/v1/"+project+"/launch/uuid/{uuid}",
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			if js.lookups.Add(1) <= notFound || r.PathValue("uuid") != joinedUUID {
				w.WriteHeader(http.StatusNotFound)
				_, _ = fmt.Fprint(w, `{"errorCode":4041,"message":"Launch not found"}`)
				return
			}
			_, _ = fmt.Fprintf(
				w,
				`{"id":7,"uuid":%q,"name":"shared","status":"IN_PROGRESS","startTime":%s}`,
				joinedUUID,
				startTime,
			)
		},
	)
	mux.HandleFunc("POST /api/v2/"+project+"/launch", func(w http.ResponseWriter, r *http.Request) {
		js.launchStarts.Add(1)
		reporting.ServeHTTP(w, r)
	})
	mux.Handle("/", reporting)
	js.Config.Handler = mux

	return js
}

// joinCmd is test2jsonCmd with --launch-uuid set.
func joinCmd(t *testing.T, host, file, uuid string, extra ...cli.Flag) *cli.Command {
	t.Helper()
	extra = append(extra, &cli.StringFlag{Name: "launch-uuid"})
	cmd, _ := test2jsonCmd(host, file, extra...)
	require.NoError(t, cmd.Set("launch-uuid", uuid))
	return cmd
}

func TestTest2JSON_JoinsExistingLaunch(t *testing.T) {
	t.Setenv("HOME", t.TempDir()) // no ~/.gorp

	srv := newJoinServer(t, "proj", launchBeforeEvents, 0, false)
	file := writeEventLines(
		t,
		event(
			"run",
			"TestOne",
		),
		event("output", "TestOne"),
		event("fail", "TestOne"),
		event("fail", ""),
	)
	cmd := joinCmd(t, srv.URL, file, joinedUUID)

	require.NoError(t, reportTest2Json(t.Context(), cmd))

	out, ok := cmd.Writer.(fmt.Stringer)
	require.True(t, ok)
	assert.Equal(t, "ReportPortal Launch UUID: "+joinedUUID+"\n", out.String(),
		"the joined launch is printed, without a link: it is not finished yet")
	assert.Zero(t, srv.launchStarts.Load(), "the launch exists: it must not be started")
	srv.mu.Lock()
	defer srv.mu.Unlock()
	assert.Empty(
		t,
		srv.launchFinish,
		"the launch belongs to somebody else: it must not be finished",
	)
	require.Len(t, srv.itemFinishes, 2)
	for _, finish := range srv.itemFinishes {
		assert.Contains(t, finish, `"launchUuid":"`+joinedUUID+`"`)
	}
}

func TestReportLaunch_JoinDoesNotFinishLaunchOnInterrupt(t *testing.T) {
	t.Parallel()

	srv := newJoinServer(t, "proj", launchBeforeEvents, 0, true) // saving logs fails
	file := writeEvents(t, []string{
		`{"time":"2025-01-01T00:00:00Z","action":"run","package":"p","test":"TestA"}`,
		`{"time":"2025-01-01T00:00:01Z","action":"output","package":"p","test":"TestA","output":"x\n"}`,
	})
	cmd := reportCmd(file, "launch", false)
	cmd.Flags = append(cmd.Flags, &cli.StringFlag{Name: "launch-uuid", Value: joinedUUID})

	cfg := &clientConfig{URL: srv.URL, Project: "proj", ApiKey: "tok"}
	_, _, err := reportLaunch(t.Context(), cfg, cmd)
	require.Error(t, err)

	srv.mu.Lock()
	defer srv.mu.Unlock()
	require.Len(t, srv.itemFinishes, 2, "the items of this reporter are still finished")
	for _, finish := range srv.itemFinishes {
		assert.Contains(t, finish, `"status":"INTERRUPTED"`)
	}
	assert.Empty(t, srv.launchFinish, "an interrupted shard must not finish the shared launch")
}

func TestReportLaunch_JoinRejectsEventsOlderThanLaunch(t *testing.T) {
	t.Parallel()

	srv := newJoinServer(t, "proj", launchAfterEvents, 0, false)
	file := writeEventLines(t, event("run", "TestOne"), event("pass", "TestOne"), event("pass", ""))
	cmd := reportCmd(file, "launch", false)
	cmd.Flags = append(cmd.Flags, &cli.StringFlag{Name: "launch-uuid", Value: joinedUUID})

	cfg := &clientConfig{URL: srv.URL, Project: "proj", ApiKey: "tok"}
	_, _, err := reportLaunch(t.Context(), cfg, cmd)

	require.ErrorIs(t, err, errEventsBeforeLaunch)
	srv.mu.Lock()
	defer srv.mu.Unlock()
	assert.Empty(t, srv.itemFinishes, "nothing is reported")
	assert.Empty(t, srv.launchFinish)
}

func TestTest2JSON_JoinEmptyInputIsNotAnError(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	srv := newJoinServer(t, "proj", launchBeforeEvents, 0, false)
	cmd := joinCmd(t, srv.URL, writeEvents(t, nil), joinedUUID)

	require.NoError(t, reportTest2Json(t.Context(), cmd))
	assert.Zero(t, srv.launchStarts.Load())
}

func TestTest2JSON_JoinFlagConflicts(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	tests := map[string]struct {
		flag cli.Flag
		want string
	}{
		"quality gate": {
			&cli.BoolFlag{Name: "quality-gate-check", Value: true},
			"--quality-gate-check",
		},
		"rerun":    {&cli.BoolFlag{Name: "rerun", Value: true}, "--rerun"},
		"rerun-of": {&cli.StringFlag{Name: "rerun-of", Value: "other"}, "--rerun"},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			srv := newJoinServer(t, "proj", launchBeforeEvents, 0, false)
			file := writeEventLines(t, event("run", "TestOne"), event("pass", "TestOne"))
			cmd := joinCmd(t, srv.URL, file, joinedUUID, tc.flag)

			err := reportTest2Json(t.Context(), cmd)

			require.ErrorContains(t, err, tc.want)
			assert.Zero(t, srv.lookups.Load(), "the conflict is found before anything is sent")
		})
	}
}

func TestFindLaunchToJoin(t *testing.T) {
	t.Parallel()

	client := func(srv *joinServer) *gorppkg.ReportingClient {
		return buildReportingClient(
			t.Context(),
			&clientConfig{URL: srv.URL, Project: "proj", ApiKey: "tok"},
		)
	}

	t.Run("no uuid, no lookup", func(t *testing.T) {
		t.Parallel()
		srv := newJoinServer(t, "proj", launchBeforeEvents, 0, false)

		launch, err := findLaunchToJoin(t.Context(), client(srv), "", time.Second, time.Millisecond)

		require.NoError(t, err)
		assert.Nil(t, launch)
		assert.Zero(t, srv.lookups.Load())
	})

	t.Run("a launch that was just started is waited for", func(t *testing.T) {
		t.Parallel()
		srv := newJoinServer(t, "proj", "1735686000000", 2, false) // epoch milliseconds

		launch, err := findLaunchToJoin(
			t.Context(), client(srv), joinedUUID, 5*time.Second, 5*time.Millisecond,
		)

		require.NoError(t, err)
		assert.Equal(t, joinedUUID, launch.UUID)
		assert.Equal(t, time.Date(2024, 12, 31, 23, 0, 0, 0, time.UTC), launch.StartTime)
		assert.EqualValues(t, 3, srv.lookups.Load())
	})

	t.Run("an unknown launch fails after the wait", func(t *testing.T) {
		t.Parallel()
		srv := newJoinServer(t, "proj", launchBeforeEvents, 0, false)

		_, err := findLaunchToJoin(
			t.Context(), client(srv), "unknown", 50*time.Millisecond, 5*time.Millisecond,
		)

		require.ErrorContains(t, err, "launch unknown not found")
		var httpErr *gorppkg.HTTPError
		require.ErrorAs(t, err, &httpErr)
		assert.Equal(t, http.StatusNotFound, httpErr.StatusCode)
		assert.Greater(t, srv.lookups.Load(), int32(1), "404 is retried")
	})

	t.Run("other errors are not retried", func(t *testing.T) {
		t.Parallel()
		var calls atomic.Int32
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			calls.Add(1)
			http.Error(w, `{"error":"invalid_token"}`, http.StatusUnauthorized)
		}))
		t.Cleanup(srv.Close)
		rp := buildReportingClient(
			context.Background(),
			&clientConfig{URL: srv.URL, Project: "proj", ApiKey: "tok"},
		)

		_, err := findLaunchToJoin(t.Context(), rp, joinedUUID, 5*time.Second, 5*time.Millisecond)

		require.ErrorContains(t, err, "looking up launch")
		assert.EqualValues(t, 1, calls.Load())
	})
}
