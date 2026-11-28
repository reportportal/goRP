package rpagent

import (
	"testing"

	"github.com/onsi/ginkgo/v2/types"
)

func TestExtractPriorityTellsAnExplicitLabelFromTheDefault(t *testing.T) {
	spec := func(container []string, leaf ...string) types.SpecReport {
		s := types.SpecReport{LeafNodeLabels: leaf}
		if container != nil {
			s.ContainerHierarchyLabels = [][]string{container}
		}
		return s
	}
	tests := []struct {
		name      string
		spec      types.SpecReport
		wantLevel int
		wantSet   bool
	}{
		{"no labels", spec(nil), DefaultPriority, false},
		{"unrelated labels only", spec([]string{"smoke"}, "slow"), DefaultPriority, false},
		{"leaf label", spec(nil, Priority(0)), 0, true},
		{"the default level, set on purpose", spec(nil, Priority(1)), 1, true},
		{"container label only", spec([]string{Priority(3)}), 3, true},
		{"leaf wins over container", spec([]string{Priority(3)}, Priority(0)), 0, true},
		{"level out of range is no label", spec(nil, "rp.priority:9"), DefaultPriority, false},
		{"level not a number is no label", spec(nil, "rp.priority:high"), DefaultPriority, false},
		{
			"an invalid leaf label falls back to the container",
			spec([]string{Priority(2)}, "rp.priority:9"),
			2,
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			level, set := ExtractPriorityFromReportEntry(tt.spec)
			if level != tt.wantLevel || set != tt.wantSet {
				t.Errorf("got (%d, %v), want (%d, %v)", level, set, tt.wantLevel, tt.wantSet)
			}
		})
	}
}

func TestPriorityIfAndSpecConversionOmitTheUnlabeled(t *testing.T) {
	if priorityIf(DefaultPriority, false) != nil {
		t.Error("an unlabeled spec must have no priority")
	}
	if p := priorityIf(2, true); p == nil || *p != 2 {
		t.Errorf("priorityIf(2, true) = %v", p)
	}

	unlabeled := convertToGinkgoSpec(types.SpecReport{LeafNodeText: "x"})
	if unlabeled.Priority != nil {
		t.Errorf("unlabeled spec converts with priority %d, want none", *unlabeled.Priority)
	}
	labeled := convertToGinkgoSpec(
		types.SpecReport{LeafNodeText: "x", LeafNodeLabels: []string{Priority(4)}},
	)
	if labeled.Priority == nil || *labeled.Priority != 4 {
		t.Errorf("labeled spec priority = %v, want 4", labeled.Priority)
	}
}
