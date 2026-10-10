package rpagent

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/reportportal/goRP/v5/pkg/gorp"
	"github.com/reportportal/goRP/v5/pkg/openapi"
)

func writeJSON(w http.ResponseWriter, body string) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(body))
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
	id := tl.StartTestWithReferenceId(
		"adds",
		"",
		&prio,
		[]*Parameter{{Key: "a", Value: "1"}},
		gorp.TestItemTypes.Step,
		"JIRA-1",
	)

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
	if id := tl.StartTest("child", "parent-1", nil, nil, gorp.TestItemTypes.Step); id != "child-1" {
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

	if id := tl.StartTest("flaky", "", nil, nil, gorp.TestItemTypes.Step); id != "item-2" {
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
	if id := tl.StartTest("bad", "", nil, nil, gorp.TestItemTypes.Step); id != "" {
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
			_ = json.NewDecoder(r.Body).Decode(&logBody)
		case "PUT /api/v2/proj/item/item-1":
			_ = json.NewDecoder(r.Body).Decode(&finishBody)
		}
		writeJSON(w, `{}`)
	})

	if err := tl.SendLog("item-1", gorp.LogLevelInfo, "hello"); err != nil {
		t.Fatal(err)
	}
	tl.FinishTestWithPriority("t", gorp.Statuses.Failed, "item-1", nil)

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
