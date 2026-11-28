package rpagent

import (
	"encoding/json"
	"testing"

	"github.com/onsi/ginkgo/v2/types"

	"github.com/reportportal/goRP/v5/pkg/gorp"
)

func spec(status string, endMs int, containerIDs ...string) SharedSpecStatus {
	return SharedSpecStatus{Status: status, ContainerIDs: containerIDs, EndTime: at(endMs)}
}

func byID(results []ContainerResult) map[string]ContainerResult {
	m := make(map[string]ContainerResult, len(results))
	for _, r := range results {
		m[r.ID] = r
	}
	return m
}

func TestAggregateContainersStatus(t *testing.T) {
	passed, failed, skipped := gorp.Statuses.Passed, gorp.Statuses.Failed, gorp.Statuses.Skipped
	cases := []struct {
		name  string
		specs []SharedSpecStatus
		want  map[string]gorp.Status
	}{
		{
			"all passed",
			[]SharedSpecStatus{spec("passed", 10, "c1"), spec("passed", 20, "c1")},
			map[string]gorp.Status{"c1": passed},
		},
		{
			"one failure fails the container",
			[]SharedSpecStatus{spec("passed", 10, "c1"), spec("failed", 20, "c1")},
			map[string]gorp.Status{"c1": failed},
		},
		{
			"panic, interrupt, abort and timeout all count as failure",
			[]SharedSpecStatus{
				spec(
					"panicked",
					1,
					"a",
				),
				spec("interrupted", 1, "b"),
				spec("aborted", 1, "c"),
				spec("timedout", 1, "d"),
			},
			map[string]gorp.Status{"a": failed, "b": failed, "c": failed, "d": failed},
		},
		{
			"only skipped specs: skipped",
			[]SharedSpecStatus{spec("skipped", 10, "c1"), spec("pending", 20, "c1")},
			map[string]gorp.Status{"c1": skipped},
		},
		{
			"passed and skipped: passed",
			[]SharedSpecStatus{spec("passed", 10, "c1"), spec("skipped", 20, "c1")},
			map[string]gorp.Status{"c1": passed},
		},
		{"a failure fails every container up the chain", []SharedSpecStatus{
			spec("failed", 10, "outer", "inner"), spec("passed", 20, "outer", "other"),
		}, map[string]gorp.Status{"outer": failed, "inner": failed, "other": passed}},
		{"a skipped-only inner container stays skipped inside a passing outer", []SharedSpecStatus{
			spec("passed", 10, "outer", "ok"), spec("skipped", 20, "outer", "skip"),
		}, map[string]gorp.Status{"outer": passed, "ok": passed, "skip": skipped}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := byID(AggregateContainers(c.specs, nil))
			if len(got) != len(c.want) {
				t.Fatalf("got %d containers, want %d: %+v", len(got), len(c.want), got)
			}
			for id, want := range c.want {
				if got[id].Status != want {
					t.Errorf("%s: status %s, want %s", id, got[id].Status, want)
				}
			}
		})
	}
}

func TestAggregateContainersEndTimeIsLatestDescendant(t *testing.T) {
	got := byID(AggregateContainers([]SharedSpecStatus{
		spec(
			"passed",
			30,
			"outer",
			"a",
		),
		spec("passed", 90, "outer", "b"),
		spec("passed", 50, "outer", "a"),
	}, nil))
	if !got["outer"].End.Equal(at(90)) || !got["a"].End.Equal(at(50)) ||
		!got["b"].End.Equal(at(90)) {
		t.Errorf("ends = outer:%v a:%v b:%v", got["outer"].End, got["a"].End, got["b"].End)
	}
	for id, r := range got {
		if r.End.Before(at(0)) {
			t.Errorf("%s has no end time", id)
		}
	}
}

func TestAggregateContainersAreOrderedDeepestFirst(t *testing.T) {
	results := AggregateContainers([]SharedSpecStatus{
		spec("passed", 10, "top", "mid", "leaf"),
		spec("passed", 10, "top2"),
	}, nil)
	depths := make([]int, len(results))
	for i, r := range results {
		depths[i] = r.Depth
	}
	want := []int{3, 2, 1, 1}
	if len(depths) != len(want) {
		t.Fatalf("depths = %v", depths)
	}
	for i := range want {
		if depths[i] != want[i] {
			t.Errorf("depths = %v, want %v (a child must finish before its parent)", depths, want)
			break
		}
	}
}

func TestAggregateContainersIgnoresFallbackIDs(t *testing.T) {
	// When creating a container failed, the agent fell back to its parent's ID, so a chain
	// can contain the suite ID or repeat the previous container.
	suiteIDs := map[string]bool{"suite-1": true}
	results := AggregateContainers([]SharedSpecStatus{
		spec(
			"failed",
			10,
			"suite-1",
			"c1",
		), // outer container failed to create: fell back to the suite
		spec("passed", 20, "c1", "c1", "c2", ""), // repeated and empty entries
	}, suiteIDs)
	got := byID(results)
	if _, ok := got["suite-1"]; ok {
		t.Errorf("the suite is not a container: %+v", results)
	}
	if len(got) != 2 || got["c1"].Depth != 1 || got["c2"].Depth != 2 {
		t.Errorf("results = %+v", results)
	}
	if got["c1"].Status != gorp.Statuses.Failed {
		t.Errorf("c1 holds a failed spec, got %s", got["c1"].Status)
	}
}

func TestAggregateContainersWithoutContainers(t *testing.T) {
	if got := AggregateContainers([]SharedSpecStatus{{Status: "passed"}}, nil); len(got) != 0 {
		t.Errorf("a top-level spec has no containers: %+v", got)
	}
	if got := AggregateContainers(nil, nil); len(got) != 0 {
		t.Errorf("no specs, no containers: %+v", got)
	}
}

func TestLatestEnd(t *testing.T) {
	if got := LatestEnd(
		[]SharedSpecStatus{spec("passed", 30), spec("passed", 70), spec("passed", 50)},
	); !got.Equal(
		at(70),
	) {
		t.Errorf("LatestEnd = %v", got)
	}
	if got := LatestEnd([]SharedSpecStatus{{Status: "passed"}}); !got.IsZero() {
		t.Errorf("no times means zero, got %v", got)
	}
}

func TestSharedSpecStatusIsBackwardCompatible(t *testing.T) {
	var old SharedSpecStatus
	if err := json.Unmarshal(
		[]byte(`{"suiteName":"s","specName":"x","status":"passed","priority":1}`),
		&old,
	); err != nil {
		t.Fatal(err)
	}
	if len(old.ContainerIDs) != 0 || !old.EndTime.IsZero() {
		t.Errorf("a record without the new fields must still load: %+v", old)
	}

	data, err := json.Marshal(
		SharedSpecStatus{Status: "passed", ContainerIDs: []string{"a", "b"}, EndTime: at(5)},
	)
	if err != nil {
		t.Fatal(err)
	}
	var back SharedSpecStatus
	if err := json.Unmarshal(data, &back); err != nil {
		t.Fatal(err)
	}
	if len(back.ContainerIDs) != 2 || !back.EndTime.Equal(at(5)) {
		t.Errorf("round trip lost data: %+v", back)
	}
}

func TestFinishSpecHierarchyReturnsContainerIDs(t *testing.T) {
	_ = outputHarness(t)
	s := types.SpecReport{
		ContainerHierarchyTexts: []string{"Outer", "Inner"},
		LeafNodeText:            "leaf",
		State:                   types.SpecStatePassed,
		StartTime:               at(0),
		EndTime:                 at(100),
	}
	activeHierarchies.Store(s.FullText(), SpecHierarchyIDs{
		StepID: "spec-item", StartTime: at(0), ContainerIDs: []string{"c-outer", "c-inner"},
	})
	t.Cleanup(func() { activeHierarchies.Delete(s.FullText()) })

	ids := finishSpecHierarchy(s)

	if len(ids.ContainerIDs) != 2 || ids.ContainerIDs[0] != "c-outer" ||
		ids.ContainerIDs[1] != "c-inner" {
		t.Errorf("container IDs must be handed back so they can be recorded: %+v", ids)
	}
	if got := finishSpecHierarchy(s); got.StepID != "" || len(got.ContainerIDs) != 0 {
		t.Errorf("an unknown spec must return empty IDs, got %+v", got)
	}
}
