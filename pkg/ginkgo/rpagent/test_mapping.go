package rpagent

import (
	"fmt"
	"os"
	"strings"

	"github.com/onsi/ginkgo/v2/types"
)

// ExtractTestReferenceIdFromReportEntry reads the rp.ref label from a SpecReport.
// Returns an empty string if no reference label is set.
// If multiple ref labels are found, the first is used and a warning is printed.
func ExtractTestReferenceIdFromReportEntry(spec types.SpecReport) string {
	found := ""
	count := 0

	for _, label := range spec.Labels() {
		if strings.HasPrefix(label, LabelRefIDPrefix) {
			count++
			if count > 1 {
				fmt.Fprintf(
					os.Stderr,
					"WARNING: multiple rp.ref labels on test %q — using first\n",
					spec.LeafNodeText,
				)
			}
			if found == "" {
				found = strings.TrimSpace(strings.TrimPrefix(label, LabelRefIDPrefix))
			}
		}
	}

	return found
}
