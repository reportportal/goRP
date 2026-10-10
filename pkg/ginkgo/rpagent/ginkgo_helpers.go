package rpagent

import (
	"fmt"
	"log"

	"github.com/reportportal/goRP/v5/pkg/gorp"
)

// GinkgoSpecReport is a serialisable representation of a Ginkgo SpecReport.
type GinkgoSpecReport struct {
	ContainerHierarchyTexts    []string    `json:"ContainerHierarchyTexts"`
	ContainerHierarchyLabels   [][]string  `json:"ContainerHierarchyLabels"`
	LeafNodeText               string      `json:"LeafNodeText"`
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
	parentID := topSuiteID
	containerIDs := make([]string, 0, len(spec.ContainerHierarchyTexts))
	containerNames := make([]string, 0, len(spec.ContainerHierarchyTexts))

	for _, container := range spec.ContainerHierarchyTexts {
		currentParent := parentID
		id, err := ccm.GetOrCreate(fmt.Sprintf("%s:%s", currentParent, container), func() string {
			return listener.StartTest(container, currentParent, nil, nil, gorp.TestItemTypes.Test)
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

	stepID := listener.StartTestWithReferenceId(
		spec.LeafNodeText, parentID, spec.Priority, parameters,
		gorp.TestItemTypes.Step, spec.TestReferenceId,
	)

	return SpecHierarchyIDs{
		StepID:         stepID,
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
