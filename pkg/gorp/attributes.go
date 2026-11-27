package gorp

import (
	"runtime"
	"strconv"

	"github.com/reportportal/goRP/v5/pkg/openapi"
)

// IssueTypeNotIssue is the locator of the "No Defect" issue type. A FAILED or SKIPPED item finished
// with it is not counted as "To Investigate".
const IssueTypeNotIssue = "NOT_ISSUE"

// SkippedIssueAttributeKey is the key of the system launch attribute that tells ReportPortal whether
// skipped items are treated as issues ("To Investigate") or not. The server reads it when it finishes
// items on its own, for example the open children of a finished parent.
const SkippedIssueAttributeKey = "skippedIssue"

// NotIssue returns an issue that marks a FAILED or SKIPPED item as "No Defect".
func NotIssue() *openapi.ComEpamReportportalBaseReportingIssue {
	return openapi.NewComEpamReportportalBaseReportingIssue(IssueTypeNotIssue)
}

// SystemAttributes returns the system launch attributes that describe the reporting agent and its
// environment, like the ones ReportPortal's Java agents send: agent=<name>|<version>,
// os=<GOOS>|<GOARCH> and go=<Go version>.
func SystemAttributes(agentName, agentVersion string) []openapi.ItemAttributesRQ {
	return []openapi.ItemAttributesRQ{
		systemAttribute("agent", agentName+"|"+agentVersion),
		systemAttribute("os", runtime.GOOS+"|"+runtime.GOARCH),
		systemAttribute("go", runtime.Version()),
	}
}

// SkippedIssueAttribute returns the system launch attribute that tells ReportPortal whether skipped
// items are "To Investigate" (true) or "No Defect" (false).
func SkippedIssueAttribute(skippedIssue bool) openapi.ItemAttributesRQ {
	return systemAttribute(SkippedIssueAttributeKey, strconv.FormatBool(skippedIssue))
}

func systemAttribute(key, value string) openapi.ItemAttributesRQ {
	return openapi.ItemAttributesRQ{
		Key:    openapi.PtrString(key),
		Value:  value,
		System: openapi.PtrBool(true),
	}
}
