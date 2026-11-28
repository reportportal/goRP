package rpagent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/reportportal/goRP/v5/pkg/gorp"
	"github.com/reportportal/goRP/v5/pkg/openapi"
)

func writeJSON(w http.ResponseWriter, body string) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(body))
}

// decodeLogRQs reads the logs of a POST .../log request: a single JSON log, or the
// multipart batch the agent sends per spec.
func decodeLogRQs(t *testing.T, r *http.Request) []openapi.SaveLogRQ {
	t.Helper()
	if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/") {
		if err := r.ParseMultipartForm(32 << 20); err != nil {
			t.Errorf("parse batch: %v", err)
			return nil
		}
		var batch []openapi.SaveLogRQ
		for _, v := range r.MultipartForm.Value["json_request_part"] {
			if err := json.Unmarshal([]byte(v), &batch); err != nil {
				t.Errorf("decode batch: %v", err)
			}
		}
		return batch
	}
	var one openapi.SaveLogRQ
	_ = json.NewDecoder(r.Body).Decode(&one)
	return []openapi.SaveLogRQ{one}
}

func newTestListener(t *testing.T, h http.HandlerFunc) *TestListener {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return NewTestListener(
		NewReportingClient(srv.URL, "proj", "secret-key", false),
		"launch-1",
		"My Launch",
	)
}

func TestStartTestSendsExpectedRequest(t *testing.T) {
	var gotPath, gotAuth string
	var got openapi.StartTestItemRQ
	tl := newTestListener(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotAuth = strings.TrimSuffix(r.URL.Path, "/"), r.Header.Get("Authorization")
		_ = json.NewDecoder(r.Body).Decode(&got)
		writeJSON(w, `{"id":"item-1"}`)
	})

	prio := 0
	id := tl.Start(StartItem{
		Name:       "adds",
		Priority:   &prio,
		Parameters: []*Parameter{{Key: "a", Value: "1"}},
		Type:       gorp.TestItemTypes.Step,
		RefID:      "JIRA-1",
		Identity:   ItemIdentity{TestCaseID: "pkg > adds", CodeRef: "a_test.go:10"},
	})

	if id != "item-1" {
		t.Fatalf("id = %q, want item-1", id)
	}
	if gotPath != "/api/v2/proj/item" {
		t.Errorf("path = %q", gotPath)
	}
	if gotAuth != "Bearer secret-key" {
		t.Errorf("Authorization = %q", gotAuth)
	}
	if got.LaunchUuid != "launch-1" || got.Name != "adds" || got.Type != "STEP" || got.Uuid == "" {
		t.Errorf("unexpected body: %+v", got)
	}
	attrs := map[string]string{}
	for _, a := range got.Attributes {
		attrs[*a.Key] = a.Value
	}
	if attrs["priority"] != "P0" || attrs["testReferenceId"] != "JIRA-1" {
		t.Errorf("attributes = %v", attrs)
	}
	if got.TestCaseId == nil || *got.TestCaseId != "pkg > adds" ||
		got.CodeRef == nil || *got.CodeRef != "a_test.go:10" ||
		got.UniqueId == nil || *got.UniqueId == "" {
		t.Errorf("identity fields = %+v", got)
	}
	if got.HasStats == nil || !*got.HasStats {
		t.Errorf("hasStats must default to true")
	}
	if len(got.Parameters) != 1 || got.Parameters[0].Key != "a" || *got.Parameters[0].Value != "1" {
		t.Errorf("parameters = %+v", got.Parameters)
	}
}

func TestStartTestUnderParentUsesChildEndpoint(t *testing.T) {
	var gotPath string
	tl := newTestListener(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		writeJSON(w, `{"id":"child-1"}`)
	})
	if id := tl.Start(
		StartItem{Name: "child", ParentID: "parent-1", Type: gorp.TestItemTypes.Step},
	); id != "child-1" {
		t.Fatalf("id = %q", id)
	}
	if gotPath != "/api/v2/proj/item/parent-1" {
		t.Errorf("path = %q", gotPath)
	}
}

func TestStartTestRetriesServerErrorWithSameUUID(t *testing.T) {
	var calls int32
	var uuids []string
	tl := newTestListener(t, func(w http.ResponseWriter, r *http.Request) {
		var rq openapi.StartTestItemRQ
		_ = json.NewDecoder(r.Body).Decode(&rq)
		uuids = append(uuids, rq.Uuid)
		if atomic.AddInt32(&calls, 1) == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		writeJSON(w, `{"id":"item-2"}`)
	})

	if id := tl.Start(StartItem{Name: "flaky", Type: gorp.TestItemTypes.Step}); id != "item-2" {
		t.Fatalf("id = %q, want item-2", id)
	}
	if len(uuids) != 2 || uuids[0] != uuids[1] {
		t.Errorf("retry must reuse the client UUID, got %v", uuids)
	}
}

func TestStartTestDoesNotRetryClientError(t *testing.T) {
	var calls int32
	tl := newTestListener(t, func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusBadRequest)
	})
	if id := tl.Start(StartItem{Name: "bad", Type: gorp.TestItemTypes.Step}); id != "" {
		t.Fatalf("id = %q, want empty", id)
	}
	if calls != 1 {
		t.Errorf("calls = %d, want 1 (4xx must not be retried)", calls)
	}
}

func TestSendLogAndFinishTest(t *testing.T) {
	var logBody openapi.SaveLogRQ
	var finishBody openapi.FinishTestItemRQ
	var paths []string
	tl := newTestListener(t, func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.Method+" "+r.URL.Path)
		switch r.Method + " " + r.URL.Path {
		case "POST /api/v2/proj/log":
			if logs := decodeLogRQs(t, r); len(logs) == 1 {
				logBody = logs[0]
			}
		case "PUT /api/v2/proj/item/item-1":
			_ = json.NewDecoder(r.Body).Decode(&finishBody)
		}
		writeJSON(w, `{}`)
	})

	logAt := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	endAt := logAt.Add(time.Second)
	if err := tl.SendLog("item-1", gorp.LogLevelInfo, "hello", logAt); err != nil {
		t.Fatal(err)
	}
	tl.FinishTest("t", gorp.Statuses.Failed, "item-1", endAt)

	if !logBody.Time.Equal(logAt) {
		t.Errorf("log time = %v, want the emit time %v", logBody.Time, logAt)
	}
	if !finishBody.EndTime.Equal(endAt) {
		t.Errorf("end time = %v, want %v", finishBody.EndTime, endAt)
	}

	if *logBody.ItemUuid != "item-1" || *logBody.Message != "hello" || *logBody.Level != "INFO" ||
		logBody.LaunchUuid != "launch-1" {
		t.Errorf("log body = %+v", logBody)
	}
	if *finishBody.Status != "FAILED" || finishBody.LaunchUuid != "launch-1" {
		t.Errorf("finish body = %+v", finishBody)
	}
}

func TestMapGinkgoStateToStatus(t *testing.T) {
	cases := map[string]gorp.Status{
		"passed":      gorp.Statuses.Passed,
		"failed":      gorp.Statuses.Failed,
		"panicked":    gorp.Statuses.Failed,
		"interrupted": gorp.Statuses.Interrupted,
		"aborted":     gorp.Statuses.Interrupted,
		"timedout":    gorp.Statuses.Interrupted,
		"skipped":     gorp.Statuses.Skipped,
		"pending":     gorp.Statuses.Skipped,
		"unknown":     gorp.Statuses.Skipped,
	}
	for in, want := range cases {
		if got := MapGinkgoStateToStatus(in); got != want {
			t.Errorf("MapGinkgoStateToStatus(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestStartIdentityIsIndependentOfParentAndRun(t *testing.T) {
	var bodies []openapi.StartTestItemRQ
	tl := newTestListener(t, func(w http.ResponseWriter, r *http.Request) {
		var rq openapi.StartTestItemRQ
		_ = json.NewDecoder(r.Body).Decode(&rq)
		bodies = append(bodies, rq)
		writeJSON(w, `{"id":"x"}`)
	})
	identity := ItemIdentity{TestCaseID: "pkg > Calc > adds", CodeRef: "calc_test.go:14"}

	// The same spec in two launches: different parent item IDs, same identity.
	tl.Start(
		StartItem{
			Name:     "adds",
			ParentID: "parent-run-1",
			Type:     gorp.TestItemTypes.Step,
			Identity: identity,
		},
	)
	tl.Start(
		StartItem{
			Name:     "adds",
			ParentID: "parent-run-2",
			Type:     gorp.TestItemTypes.Step,
			Identity: identity,
		},
	)

	if len(bodies) != 2 {
		t.Fatalf("got %d requests", len(bodies))
	}
	a, b := bodies[0], bodies[1]
	if *a.TestCaseId != *b.TestCaseId || *a.UniqueId != *b.UniqueId || *a.CodeRef != *b.CodeRef {
		t.Errorf("identity changed between runs: %+v vs %+v", a, b)
	}
	if a.Uuid == b.Uuid {
		t.Errorf("the item's own uuid must be unique per start")
	}
	if strings.Contains(*a.TestCaseId, "parent-run") {
		t.Errorf("testCaseId must not contain the parent ID: %s", *a.TestCaseId)
	}
}

func TestStartWithoutIdentityOmitsIdentityFields(t *testing.T) {
	var got openapi.StartTestItemRQ
	tl := newTestListener(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&got)
		writeJSON(w, `{"id":"x"}`)
	})
	tl.Start(StartItem{Name: "n", Type: gorp.TestItemTypes.Step, NoStats: true})
	if got.TestCaseId != nil || got.UniqueId != nil || got.CodeRef != nil {
		t.Errorf("identity fields must be omitted: %+v", got)
	}
	if got.HasStats == nil || *got.HasStats {
		t.Errorf("NoStats must send hasStats=false")
	}
}

func TestZeroTimesMeanNow(t *testing.T) {
	var start openapi.StartTestItemRQ
	var logBody openapi.SaveLogRQ
	tl := newTestListener(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/log") {
			if logs := decodeLogRQs(t, r); len(logs) == 1 {
				logBody = logs[0]
			}
		} else {
			_ = json.NewDecoder(r.Body).Decode(&start)
		}
		writeJSON(w, `{"id":"x"}`)
	})
	before := time.Now().Add(-time.Second)
	tl.Start(StartItem{Name: "n", Type: gorp.TestItemTypes.Step})
	_ = tl.SendLog("x", gorp.LogLevelInfo, "m", time.Time{})
	if start.StartTime.Before(before) || logBody.Time.Before(before) {
		t.Errorf("zero times must default to now: start=%v log=%v", start.StartTime, logBody.Time)
	}
}

func TestStartUsesGivenStartTime(t *testing.T) {
	var got openapi.StartTestItemRQ
	tl := newTestListener(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&got)
		writeJSON(w, `{"id":"x"}`)
	})
	at := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	tl.Start(StartItem{Name: "n", Type: gorp.TestItemTypes.Step, StartTime: at})
	if !got.StartTime.Equal(at) {
		t.Errorf("start time = %v, want %v", got.StartTime, at)
	}
}

type timeoutErr struct{}

func (timeoutErr) Error() string   { return "i/o timeout" }
func (timeoutErr) Timeout() bool   { return true }
func (timeoutErr) Temporary() bool { return false }

func TestIsRetriableError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"500", &gorp.HTTPError{StatusCode: 500}, true},
		{"429", &gorp.HTTPError{StatusCode: 429}, true},
		{"404", &gorp.HTTPError{StatusCode: 404}, false},
		{"400", &gorp.HTTPError{StatusCode: 400}, false},
		{"wrapped 503", fmt.Errorf("x: %w", &gorp.HTTPError{StatusCode: 503}), true},
		{"EOF", io.EOF, true},
		{"unexpected EOF", fmt.Errorf("read: %w", io.ErrUnexpectedEOF), true},
		{"refused", &net.OpError{Op: "dial", Err: syscall.ECONNREFUSED}, true},
		{"reset", fmt.Errorf("x: %w", syscall.ECONNRESET), true},
		{"dns", &net.DNSError{Err: "no such host", Name: "rp"}, true},
		{"net timeout", timeoutErr{}, true},
		{"deadline", context.DeadlineExceeded, true},
		{"canceled", context.Canceled, false},
		// Only typed errors count: a message that merely contains "timeout" must not.
		{"plain text", errors.New("timeout while thinking"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isRetriableError(tt.err); got != tt.want {
				t.Errorf("isRetriableError(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

func TestStartRetryOfSendsRetryFlags(t *testing.T) {
	var got map[string]any
	tl := newTestListener(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&got)
		writeJSON(w, `{"id":"x"}`)
	})
	tl.Start(StartItem{Name: "s", Type: gorp.TestItemTypes.Step, RetryOf: "prev-id"})
	if got["retry"] != true || got["retryOf"] != "prev-id" {
		t.Errorf("retry fields = %v / %v, want true / prev-id", got["retry"], got["retryOf"])
	}
}

func TestFlushLogsBatchesAndSplitsBySize(t *testing.T) {
	var requests, total int
	ids := map[string]bool{}
	tl := newTestListener(t, func(w http.ResponseWriter, r *http.Request) {
		requests++
		for _, l := range decodeLogRQs(t, r) {
			total++
			ids[*l.Uuid] = true
		}
		writeJSON(w, `{}`)
	})
	for i := 0; i < maxBatchLogs+5; i++ {
		tl.QueueLog("item", "INFO", "m", time.Time{})
	}
	if err := tl.FlushLogs(); err != nil {
		t.Fatal(err)
	}
	if requests != 2 || total != maxBatchLogs+5 || len(ids) != total {
		t.Errorf("requests=%d logs=%d unique=%d, want 2 requests, %d unique logs",
			requests, total, len(ids), maxBatchLogs+5)
	}
	if err := tl.FlushLogs(); err != nil || requests != 2 {
		t.Errorf("flushing an empty queue must send nothing (requests=%d, err=%v)", requests, err)
	}
}
