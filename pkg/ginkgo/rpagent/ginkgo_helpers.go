package rpagent

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/reportportal/goRP/v5/pkg/gorp"
)

// GinkgoSpecReport is a serialisable representation of a Ginkgo SpecReport.
type GinkgoSpecReport struct {
	ContainerHierarchyTexts    []string    `json:"ContainerHierarchyTexts"`
	ContainerHierarchyLabels   [][]string  `json:"ContainerHierarchyLabels"`
	ContainerHierarchyLocs     []CodeLoc   `json:"ContainerHierarchyLocs"`
	LeafNodeText               string      `json:"LeafNodeText"`
	LeafNodeLoc                CodeLoc     `json:"LeafNodeLoc"`
	FullText                   string      `json:"FullText"`
	State                      string      `json:"State"`
	CapturedStdOutErr          string      `json:"CapturedStdOutErr"`
	CapturedGinkgoWriterOutput string      `json:"CapturedGinkgoWriterOutput"`
	Labels                     []string    `json:"Labels"`
	Priority                   *int        `json:"Priority,omitempty"`
	TestReferenceId            string      `json:"TestReferenceId,omitempty"`
	SpecEvents                 []SpecEvent `json:"SpecEvents"`
	Failure                    *struct {
		Message  string `json:"Message"`
		Location struct {
			FileName   string `json:"FileName"`
			LineNumber int    `json:"LineNumber"`
		} `json:"Location"`
	} `json:"Failure"`
}

// SpecEvent is a simplified Ginkgo spec event (used for By() extraction).
type SpecEvent struct {
	SpecEventType string `json:"SpecEventType"`
	Message       string `json:"Message"`
}

// ProcessSpecWithHierarchyAndReturnIDs creates the Describe→Context→It hierarchy in ReportPortal
// without finishing any items. Items are finished later in ReportAfterEach.
func ProcessSpecWithHierarchyAndReturnIDs(
	client *gorp.ReportingClient,
	listener *TestListener,
	spec GinkgoSpecReport,
	topSuiteID string,
	ccm *ContainerCacheManager,
	parameters []*Parameter,
) SpecHierarchyIDs {
	ib := getIdentityBuilder()
	parentID := topSuiteID
	containerIDs := make([]string, 0, len(spec.ContainerHierarchyTexts))
	containerNames := make([]string, 0, len(spec.ContainerHierarchyTexts))

	for i, container := range spec.ContainerHierarchyTexts {
		currentParent := parentID
		// The cache key stays launch-specific (it embeds the parent's ID); the item's
		// reported identity is run-independent.
		identity := ib.container(spec.ContainerHierarchyTexts, spec.ContainerHierarchyLocs, i)
		id, err := ccm.GetOrCreate(fmt.Sprintf("%s:%s", currentParent, container), func() string {
			return listener.Start(StartItem{
				Name:     container,
				ParentID: currentParent,
				Type:     gorp.TestItemTypes.Test,
				Identity: identity,
			})
		})
		if err != nil {
			// Reporting must never kill the test process. Log the error and
			// fall back to the parent ID so the spec still runs.
			log.Printf(
				"WARNING: failed to get/create container %q (will use parent): %v",
				container,
				err,
			)
			id = currentParent
		}
		if id == "" {
			id = currentParent
		}
		containerIDs = append(containerIDs, id)
		containerNames = append(containerNames, container)
		parentID = id
	}

	identity := ib.spec(spec.ContainerHierarchyTexts, spec.LeafNodeText, spec.LeafNodeLoc)
	noteSpecIdentity(identity)

	// Ginkgo has not set the spec's start time yet inside ReportBeforeEach, so the item
	// starts "now" (milliseconds before the spec body). Remember it: By() steps and logs
	// are clamped to start no earlier than this, as ReportPortal requires children to
	// start after their parent.
	start := time.Now()
	template := StartItem{
		Name:       spec.LeafNodeText,
		ParentID:   parentID,
		Priority:   spec.Priority,
		Parameters: parameters,
		Type:       gorp.TestItemTypes.Step,
		RefID:      spec.TestReferenceId,
		Labels:     tagLabels(spec.Labels),
		StartTime:  start,
		Identity:   identity,
	}
	stepID := listener.Start(template)

	return SpecHierarchyIDs{
		Template:       template,
		StepID:         stepID,
		StartTime:      start,
		Identity:       identity,
		ContainerIDs:   containerIDs,
		ContainerNames: containerNames,
	}
}

// ExtractByStatementsFromSpec returns the messages from all By() events in the spec.
func ExtractByStatementsFromSpec(spec GinkgoSpecReport) []string {
	var out []string
	for _, event := range spec.SpecEvents {
		if event.SpecEventType == "By" && event.Message != "" {
			out = append(out, event.Message)
		}
	}
	return out
}

// tagLabels returns the user's Ginkgo labels, without the rp.* control labels
// (rp.priority:N, rp.ref:ID) that are already reported as their own attributes.
func tagLabels(labels []string) []string {
	var out []string
	for _, l := range labels {
		if l = strings.TrimSpace(l); l != "" && !strings.HasPrefix(l, "rp.") {
			out = append(out, l)
		}
	}
	return out
}
