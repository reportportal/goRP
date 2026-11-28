package gorp

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/reportportal/goRP/v5/pkg/openapi"
)

// logBatchRequest is one multipart log request as seen by logServer.
type logBatchRequest struct {
	Messages []string
	Files    []string
}

// logServer is a ReportPortal fake for POST /api/v2/prj/log. It answers every log with an ID, except
// those whose message starts with "bad", which it rejects the way ReportPortal does: inside a 201.
type logServer struct {
	*httptest.Server

	mu       sync.Mutex
	requests []logBatchRequest
	// handle, if set, runs in every request before the answer
	handle func(w http.ResponseWriter) (done bool)
}

func newLogServer(t *testing.T) *logServer {
	t.Helper()
	ls := &logServer{}
	ls.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !assert.Equal(t, "/api/v2/prj/log", r.URL.Path) {
			return
		}
		var logs []map[string]any
		if !assert.NoError(t, json.Unmarshal([]byte(r.FormValue("json_request_part")), &logs)) {
			return
		}
		rq := logBatchRequest{}
		responses := make([]map[string]string, 0, len(logs))
		for i, l := range logs {
			msg, _ := l["message"].(string)
			rq.Messages = append(rq.Messages, msg)
			if strings.HasPrefix(msg, "bad") {
				// ReportPortal puts the stack trace into "message" and the message into "stackTrace"
				responses = append(responses, map[string]string{
					"message":    "com.epam.ReportPortalException: " + msg + " rejected\n\tat com.epam.Some.method(Some.java:1)",
					"stackTrace": "ReportPortalException: " + msg + " rejected",
				})
				continue
			}
			responses = append(responses, map[string]string{"id": fmt.Sprintf("log-%s-%d", msg, i)})
		}
		if r.MultipartForm != nil {
			for _, f := range r.MultipartForm.File["file"] {
				rq.Files = append(rq.Files, f.Filename)
			}
		}
		ls.mu.Lock()
		ls.requests = append(ls.requests, rq)
		handle := ls.handle
		ls.mu.Unlock()
		if handle != nil && handle(w) {
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{"responses": responses})
	}))
	t.Cleanup(ls.Close)
	return ls
}

func (ls *logServer) client(t *testing.T) *ReportingClient {
	t.Helper()
	return NewReportingClient(ls.URL, "prj", WithApiKeyAuth(t.Context(), "key"))
}

// received returns the requests so far; batchSizes their numbers of logs.
func (ls *logServer) received() []logBatchRequest {
	ls.mu.Lock()
	defer ls.mu.Unlock()
	return append([]logBatchRequest(nil), ls.requests...)
}

func (ls *logServer) batchSizes() []int {
	sizes := []int{}
	for _, rq := range ls.received() {
		sizes = append(sizes, len(rq.Messages))
	}
	return sizes
}

func (ls *logServer) logCount() int {
	n := 0
	for _, size := range ls.batchSizes() {
		n += size
	}
	return n
}

func testLog(message string) *openapi.SaveLogRQ {
	return &openapi.SaveLogRQ{
		LaunchUuid: "launch",
		Level:      openapi.PtrString(LogLevelInfo),
		Message:    openapi.PtrString(message),
		Time:       time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}
}

// noTimer disables the flush timer, so that only the limits under test end a batch.
const noTimer = -1

func TestSaveLogBatch_ReturnsEveryEntry(t *testing.T) {
	t.Parallel()

	srv := newLogServer(t)
	entries, err := srv.client(t).SaveLogBatch(t.Context(),
		[]*openapi.SaveLogRQ{testLog("one"), testLog("bad two"), testLog("three")}, nil)

	require.NoError(t, err, "a batch with rejected logs is still a 201")
	require.Len(t, entries, 3)
	assert.Equal(t, "log-one-0", entries[0].ID)
	assert.False(t, entries[0].Failed())
	assert.Empty(t, entries[0].Reason())
	assert.True(t, entries[1].Failed())
	assert.Equal(t, "ReportPortalException: bad two rejected", entries[1].Reason(),
		"the reason is the one-line message, whichever field the server put it in")
	assert.Contains(t, entries[1].Message, "\tat com.epam")
	assert.Equal(t, "log-three-2", entries[2].ID)
}

func TestSaveLogMultipart_FirstEntryOfBatch(t *testing.T) {
	t.Parallel()

	srv := newLogServer(t)
	client := srv.client(t)

	rs, err := client.SaveLogMultipart(
		t.Context(),
		[]*openapi.SaveLogRQ{testLog("one"), testLog("two")},
		nil,
	)
	require.NoError(t, err)
	assert.Equal(t, "log-one-0", rs.GetId())

	rs, err = client.SaveLogMultipart(t.Context(), []*openapi.SaveLogRQ{testLog("bad one")}, nil)
	require.NoError(t, err, "as before: a rejected log is not an error of SaveLogMultipart")
	assert.Nil(t, rs.Id)
}

func TestSaveLogBatch_SingleIDResponse(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"log-1"}`))
	}))
	defer server.Close()
	client := NewReportingClient(server.URL, "prj", WithApiKeyAuth(t.Context(), "key"))

	entries, err := client.SaveLogBatch(t.Context(), []*openapi.SaveLogRQ{testLog("one")}, nil)

	require.NoError(t, err)
	assert.Equal(t, []LogBatchEntry{{ID: "log-1"}}, entries)
}

func TestLogBatcher_FlushesByCount(t *testing.T) {
	t.Parallel()

	srv := newLogServer(t)
	// one request at a time keeps the order of the batches
	b := NewLogBatcher(
		t.Context(),
		srv.client(t),
		LogBatcherOptions{FlushInterval: noTimer, MaxInFlight: 1},
	)
	for i := range 25 {
		require.NoError(t, b.Add(testLog(fmt.Sprintf("m%d", i))))
	}
	require.Eventually(
		t,
		func() bool { return len(srv.received()) == 2 },
		5*time.Second,
		5*time.Millisecond,
		"complete batches are sent without Flush or Close",
	)
	require.NoError(t, b.Close())

	assert.Equal(
		t,
		[]int{10, 10, 5},
		srv.batchSizes(),
		"the default is %d logs",
		DefaultLogBatchEntries,
	)
}

func TestLogBatcher_FlushesBySize(t *testing.T) {
	t.Parallel()

	srv := newLogServer(t)
	size := logSize(testLog("m0"), nil)
	b := NewLogBatcher(t.Context(), srv.client(t), LogBatcherOptions{
		FlushInterval: noTimer, MaxInFlight: 1, MaxEntries: 100,
		MaxBytes: 3*size + size/2, // three logs fit, the fourth does not
	})
	for i := range 7 {
		require.NoError(t, b.Add(testLog(fmt.Sprintf("m%d", i))))
	}
	// a log that is larger than the limit is sent on its own
	require.NoError(t, b.Add(testLog(strings.Repeat("x", 10*size))))
	require.NoError(t, b.Add(testLog("m8")))
	require.NoError(t, b.Close())

	assert.Equal(t, []int{3, 3, 1, 1, 1}, srv.batchSizes())
}

func TestLogBatcher_FlushesByAttachments(t *testing.T) {
	t.Parallel()

	srv := newLogServer(t)
	b := NewLogBatcher(t.Context(), srv.client(t), LogBatcherOptions{
		FlushInterval: noTimer, MaxInFlight: 1, MaxEntries: 100, MaxAttachments: 2,
	})
	for i := range 5 {
		name := fmt.Sprintf("f%d.txt", i)
		log := testLog(name)
		log.File = &openapi.File{Name: openapi.PtrString(name)}
		require.NoError(t, b.Add(log, &ReaderMultipart{
			FileName: name, ContentType: "text/plain", Reader: strings.NewReader("content"),
		}))
		require.NoError(t, b.Add(testLog("plain"))) // logs without attachment do not count
	}
	require.NoError(t, b.Close())

	var files [][]string
	for _, rq := range srv.received() {
		files = append(files, rq.Files)
	}
	assert.Equal(t, [][]string{{"f0.txt", "f1.txt"}, {"f2.txt", "f3.txt"}, {"f4.txt"}}, files,
		"a request never has more than MaxAttachments files, and a file travels with its log")
	assert.Equal(t, 10, srv.logCount())
	assert.Equal(t, 49, LogBatcherOptions{}.withDefaults().MaxAttachments)
}

func TestLogBatcher_FlushesByTimer(t *testing.T) {
	t.Parallel()

	srv := newLogServer(t)
	b := NewLogBatcher(
		t.Context(),
		srv.client(t),
		LogBatcherOptions{FlushInterval: 20 * time.Millisecond},
	)
	require.NoError(t, b.Add(testLog("log of a long-running test")))

	require.Eventually(
		t,
		func() bool { return srv.logCount() == 1 },
		5*time.Second,
		5*time.Millisecond,
		"an incomplete batch is sent when the interval has passed, without Flush or Close",
	)
	require.NoError(t, b.Close())
	assert.Equal(t, 1, srv.logCount(), "nothing is sent twice")
}

func TestLogBatcher_Flush(t *testing.T) {
	t.Parallel()

	srv := newLogServer(t)
	b := NewLogBatcher(t.Context(), srv.client(t), LogBatcherOptions{FlushInterval: noTimer})
	b.Flush() // nothing to send
	require.NoError(t, b.Add(testLog("one")))
	b.Flush()

	require.Eventually(
		t,
		func() bool { return srv.logCount() == 1 },
		5*time.Second,
		5*time.Millisecond,
	)
	require.NoError(t, b.Close())
	assert.Len(t, srv.received(), 1, "an empty batch is never sent")
}

func TestLogBatcher_LimitsRequestsInFlight(t *testing.T) {
	t.Parallel()

	srv := newLogServer(t)
	var inFlight, peak atomic.Int32
	srv.handle = func(http.ResponseWriter) bool {
		n := inFlight.Add(1)
		for {
			if p := peak.Load(); n <= p || peak.CompareAndSwap(p, n) {
				break
			}
		}
		time.Sleep(20 * time.Millisecond)
		inFlight.Add(-1)
		return false
	}
	b := NewLogBatcher(t.Context(), srv.client(t), LogBatcherOptions{
		FlushInterval: noTimer, MaxEntries: 1, MaxInFlight: 2,
	})
	for i := range 8 {
		require.NoError(t, b.Add(testLog(fmt.Sprintf("m%d", i))))
	}
	require.NoError(t, b.Close())

	assert.Equal(t, 8, srv.logCount())
	assert.EqualValues(t, 2, peak.Load(), "Add blocks while MaxInFlight requests are under way")
}

func TestLogBatcher_CloseReturnsRejectedLogs(t *testing.T) {
	t.Parallel()

	srv := newLogServer(t)
	b := NewLogBatcher(t.Context(), srv.client(t), LogBatcherOptions{FlushInterval: noTimer})
	require.NoError(t, b.Add(testLog("good")))
	require.NoError(t, b.Add(testLog("bad one")))
	require.NoError(t, b.Add(testLog("bad two")))

	err := b.Close()

	var batchErr *LogBatchError
	require.ErrorAs(t, err, &batchErr)
	assert.Equal(t, 3, batchErr.Total)
	require.Len(t, batchErr.Failed, 2)
	assert.EqualError(t, err, "2 of 3 logs were not saved: ReportPortalException: bad one rejected")
	assert.Same(t, err, b.Close(), "Close returns the same result again")
	assert.ErrorIs(t, b.Add(testLog("late")), ErrLogBatcherClosed)
	assert.Equal(
		t,
		3,
		srv.logCount(),
		"nothing is sent after Close, and a failed batch is not sent again",
	)
}

func TestLogBatcher_CloseReturnsRequestError(t *testing.T) {
	t.Parallel()

	srv := newLogServer(t)
	srv.handle = func(w http.ResponseWriter) bool {
		http.Error(w, `{"errorCode":5000,"message":"boom"}`, http.StatusInternalServerError)
		return true
	}
	b := NewLogBatcher(t.Context(), srv.client(t), LogBatcherOptions{FlushInterval: noTimer})
	require.NoError(t, b.Add(testLog("one")))

	err := b.Close()

	var httpErr *HTTPError
	require.ErrorAs(t, err, &httpErr)
	assert.Equal(t, http.StatusInternalServerError, httpErr.StatusCode)
	assert.ErrorContains(t, err, "sending a batch of 1 logs")
}

// Add from several goroutines while the timer flushes: every log arrives exactly once (run with -race).
func TestLogBatcher_ConcurrentAddAndTimer(t *testing.T) {
	t.Parallel()

	srv := newLogServer(t)
	b := NewLogBatcher(t.Context(), srv.client(t), LogBatcherOptions{
		MaxEntries: 7, FlushInterval: time.Millisecond,
	})
	const goroutines, perGoroutine = 8, 50
	var wg sync.WaitGroup
	for g := range goroutines {
		wg.Go(func() {
			for i := range perGoroutine {
				assert.NoError(t, b.Add(testLog(fmt.Sprintf("g%d-m%d", g, i))))
			}
		})
	}
	wg.Wait()
	require.NoError(t, b.Close())

	seen := map[string]int{}
	for _, rq := range srv.received() {
		assert.LessOrEqual(t, len(rq.Messages), 7)
		for _, msg := range rq.Messages {
			seen[msg]++
		}
	}
	assert.Len(t, seen, goroutines*perGoroutine)
	for msg, n := range seen {
		assert.Equal(t, 1, n, msg)
	}
}
