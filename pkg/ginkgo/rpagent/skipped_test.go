package rpagent

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/onsi/ginkgo/v2/types"

	"github.com/reportportal/goRP/v5/pkg/gorp"
)

func TestSkipIsNotAnIssue(t *testing.T) {
	skip := func(state types.SpecState, msg string) types.SpecReport {
		return types.SpecReport{State: state, Failure: types.Failure{Message: msg}}
	}
	tests := []struct {
		name         string
		spec         types.SpecReport
		skippedIssue bool
		want         bool
	}{
		{
			"user Skip, rp.skipped.issue=true",
			skip(types.SpecStateSkipped, "not on this OS"),
			true,
			false,
		},
		{
			"user Skip, rp.skipped.issue=false",
			skip(types.SpecStateSkipped, "not on this OS"),
			false,
			true,
		},
		{"pending, rp.skipped.issue=true", skip(types.SpecStatePending, ""), true, true},
		{"pending, rp.skipped.issue=false", skip(types.SpecStatePending, ""), false, true},
		{"filtered out (no message)", skip(types.SpecStateSkipped, ""), true, true},
		{
			"earlier spec in the ordered container failed",
			skip(
				types.SpecStateSkipped,
				"Spec skipped because an earlier spec in an ordered container failed",
			),
			true,
			true,
		},
		{
			"BeforeAll failed",
			skip(types.SpecStateSkipped, "Spec skipped because a BeforeAll node failed"),
			true, true,
		},
		{
			"Skip() called in BeforeAll",
			skip(types.SpecStateSkipped, "Spec skipped because Skip() was called in BeforeAll"),
			true, true,
		},
		// Only skips and pending specs are ever NOT_ISSUE: a failure is a defect.
		{"failed", skip(types.SpecStateFailed, "boom"), false, false},
		{"passed", skip(types.SpecStatePassed, ""), false, false},
		{"interrupted", skip(types.SpecStateInterrupted, "Interrupted by User"), false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := skipIsNotAnIssue(tt.spec, tt.skippedIssue); got != tt.want {
				t.Errorf("skipIsNotAnIssue(%v %q, %v) = %v, want %v",
					tt.spec.State, tt.spec.Failure.Message, tt.skippedIssue, got, tt.want)
			}
		})
	}
}

func TestFinishTestSendsIssueOnlyWhenAsked(t *testing.T) {
	var got []map[string]any
	tl := newTestListener(t, func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		got = append(got, body)
		writeJSON(w, `{}`)
	})
	tl.FinishTest("plain", gorp.Statuses.Skipped, "id-1", time.Time{})
	tl.FinishTestNotIssue("quiet", gorp.Statuses.Skipped, "id-2", time.Time{})

	if len(got) != 2 {
		t.Fatalf("requests = %d, want 2", len(got))
	}
	if _, has := got[0]["issue"]; has {
		t.Errorf("a plain finish must not carry an issue: %v", got[0])
	}
	issue, _ := got[1]["issue"].(map[string]any)
	if issue["issueType"] != "NOT_ISSUE" || got[1]["status"] != "SKIPPED" {
		t.Errorf("NOT_ISSUE finish = %v", got[1])
	}
}
