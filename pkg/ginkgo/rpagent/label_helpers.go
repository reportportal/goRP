package rpagent

import (
	"fmt"
	"strings"

	"github.com/onsi/ginkgo/v2"
)

// Label prefix constants used to encode metadata in Ginkgo labels.
const (
	LabelPriorityPrefix = "rp.priority:"
	LabelRefIDPrefix    = "rp.ref:"
)

// Priority returns a Ginkgo label string for setting test priority (P0–P4).
//
// Usage:
//
//	It("should login", Label(rpagent.Priority(0)), func() { ... })
func Priority(level int) string {
	if level < 0 || level > 4 {
		level = DefaultPriority
	}
	return fmt.Sprintf("%s%d", LabelPriorityPrefix, level)
}

// TestReference returns a Ginkgo label string for linking a test to an external reference ID.
//
// Usage:
//
//	It("should login", Label(rpagent.TestReference("LOGIN-TC-001")), func() { ... })
//
// An empty or invalid refID returns a no-op placeholder label so callers do not
// accidentally pass an empty string to Label(), which Ginkgo rejects.
func TestReference(refID string) string {
	if refID == "" {
		return "rp.ref:unset"
	}
	// Sanitise characters Ginkgo rejects in labels (&|!,()/). Replace with _.
	sanitised := strings.Map(func(r rune) rune {
		switch r {
		case '&', '|', '!', ',', '(', ')', '/':
			return '_'
		}
		return r
	}, refID)
	return fmt.Sprintf("%s%s", LabelRefIDPrefix, sanitised)
}

// Log sends a structured log entry to ReportPortal for the currently running test.
// level must be one of: "DEBUG", "INFO", "WARN", "ERROR", "FATAL".
//
// Usage:
//
//	rpagent.Log("INFO", "Starting token validation")
//	rpagent.Log("ERROR", "Unexpected response: "+body)
func Log(level, message string) {
	ginkgo.AddReportEntry(fmt.Sprintf("RP_LOG#%s#%s#%s", "rp_log", level, message))
}
