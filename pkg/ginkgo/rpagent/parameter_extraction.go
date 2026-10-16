package rpagent

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/onsi/ginkgo/v2/types"
)

// Parameter is a key/value pair attached to a test item, extracted from a DescribeTable entry name.
type Parameter struct {
	Key   string
	Value string
}

// ExtractParametersFromEntryName extracts key=value parameters from a DescribeTable entry name.
// Supports formats: key='value' and key="value"
//
// Example entry name: "input='hello' operation='upper' expected='HELLO'"
func ExtractParametersFromEntryName(entryName string) []*Parameter {
	re := regexp.MustCompile(`(\w+)=['"]([^'"]*?)['"]`)
	matches := re.FindAllStringSubmatch(entryName, -1)

	params := make([]*Parameter, 0, len(matches))
	for _, match := range matches {
		if len(match) == 3 {
			params = append(params, &Parameter{Key: match[1], Value: match[2]})
		}
	}
	return params
}

// IsDescribeTableEntry reports whether the spec came from a DescribeTable.
func IsDescribeTableEntry(spec types.SpecReport) bool {
	return strings.Contains(spec.LeafNodeText, "=") &&
		(strings.Contains(spec.LeafNodeText, "'") || strings.Contains(spec.LeafNodeText, "\""))
}

// GetDescribeTableName extracts the DescribeTable name from the container hierarchy.
func GetDescribeTableName(spec types.SpecReport) string {
	for _, container := range spec.ContainerHierarchyTexts {
		if !strings.Contains(container, "=") {
			return container
		}
	}
	if len(spec.ContainerHierarchyTexts) > 0 {
		return spec.ContainerHierarchyTexts[len(spec.ContainerHierarchyTexts)-1]
	}
	return "Unknown DescribeTable"
}

// LogParameterExtraction prints extracted parameters for debugging.
func LogParameterExtraction(spec types.SpecReport, parameters []*Parameter) {
	if len(parameters) > 0 {
		fmt.Printf("Extracted %d parameter(s) from: %s\n", len(parameters), spec.LeafNodeText)
		for _, p := range parameters {
			fmt.Printf("  - %s: %s\n", p.Key, p.Value)
		}
	}
}
