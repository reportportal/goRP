package rpagent

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/onsi/ginkgo/v2/types"
)

// Priority level constants.
const (
	P0 = 0 // Critical
	P1 = 1 // High (default)
	P2 = 2 // Medium
	P3 = 3 // Low
	P4 = 4 // Lowest

	DefaultPriority = P1
)

// ExtractPriorityFromReportEntry reads the rp.priority label from a SpecReport.
// Returns DefaultPriority if no priority label is set.
//
// Leaf-node labels take precedence over container labels so that an It() can
// override a Describe(). We scan LeafNodeLabels first; if nothing is found
// there we fall back to the full Labels() set (which includes container labels).
func ExtractPriorityFromReportEntry(spec types.SpecReport) int {
	// Try leaf node labels first.
	for _, label := range spec.LeafNodeLabels {
		if strings.HasPrefix(label, LabelPriorityPrefix) {
			if level, err := strconv.Atoi(
				strings.TrimPrefix(label, LabelPriorityPrefix),
			); err == nil {
				if level >= 0 && level <= 4 {
					return level
				}
			}
		}
	}

	// Fall back to all labels (includes container labels).
	found := -1
	count := 0
	for _, label := range spec.Labels() {
		if strings.HasPrefix(label, LabelPriorityPrefix) {
			count++
			if count > 1 {
				fmt.Fprintf(
					os.Stderr,
					"WARNING: multiple rp.priority labels on test %q — using first\n",
					spec.LeafNodeText,
				)
			}
			if found == -1 {
				if level, err := strconv.Atoi(
					strings.TrimPrefix(label, LabelPriorityPrefix),
				); err == nil {
					if level >= 0 && level <= 4 {
						found = level
					}
				}
			}
		}
	}
	if found != -1 {
		return found
	}
	return DefaultPriority
}

// GetPriorityLabel returns the human-readable label for a priority level (e.g. "P0").
func GetPriorityLabel(priority int) string {
	if priority >= 0 && priority <= 4 {
		return fmt.Sprintf("P%d", priority)
	}
	return fmt.Sprintf("P%d", DefaultPriority)
}
