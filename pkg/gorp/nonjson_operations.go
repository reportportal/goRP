package gorp

import (
	"net/http"
	"strings"
)

// nonJSONOperations lists the operations of the generated client that answer a successful request
// with something other than JSON: files, reports, or nothing at all. An HTML page is a legitimate
// response to some of them (an HTML launch report, an HTML attachment), so [OpenAPIErrorMiddleware]
// does not take it for a wrong host there.
//
// The list is derived from pkg/openapi/api/openapi.yaml: an operation is here when none of its 2xx
// responses has application/json content, or that content is a binary string (a file download).
// TestNonJSONOperationsMatchSpec recomputes it from the specification and prints the new list
// when the two differ, so update it together with the generated client.
var nonJSONOperations = parseOperationTemplates(
	"DELETE /groups/{group_id}",
	"DELETE /groups/{group_id}/projects/{project_id}",
	"DELETE /groups/{group_id}/users/{user_id}",
	"DELETE /organizations/{org_id}",
	"DELETE /organizations/{org_id}/integrations",
	"DELETE /organizations/{org_id}/integrations/{integration_id}",
	"DELETE /organizations/{org_id}/projects/{project_id}",
	"DELETE /organizations/{org_id}/users/{user_id}",
	"DELETE /projects/{project_key}/groups/{group_id}",
	"DELETE /projects/{project_key}/log-types/{log_type_id}",
	"DELETE /users/{user_id}/avatar",
	"DELETE /v1/project/{projectKey}/launch/manual/{launchId}",
	"DELETE /v1/project/{projectKey}/launch/manual/{launchId}/test-case/execution/{executionId}",
	"DELETE /v1/project/{projectKey}/launch/manual/{launchId}/test-case/execution/{executionId}/comment",
	"DELETE /v1/project/{projectKey}/tms/dataset/{datasetId}",
	"DELETE /v1/project/{projectKey}/tms/folder/{folderId}",
	"DELETE /v1/project/{projectKey}/tms/milestone/{id}",
	"DELETE /v1/project/{projectKey}/tms/milestone/{id}/test-plan/{testPlanId}",
	"DELETE /v1/project/{projectKey}/tms/productversion/{productVersionId}",
	"DELETE /v1/project/{projectKey}/tms/sync/jobs/{jobId}",
	"DELETE /v1/project/{projectKey}/tms/test-case/batch",
	"DELETE /v1/project/{projectKey}/tms/test-case/{testCaseId}",
	"DELETE /v1/project/{projectKey}/tms/test-case/{testCaseId}/tags",
	"DELETE /v1/project/{projectKey}/tms/test-plan/{id}",
	"GET /users/{user_id}/avatar",
	"GET /v1/data/{projectKey}/{dataId}",
	"GET /v1/plugin/public/{pluginName}/file/{name}",
	"GET /v1/project/export",
	"GET /v1/project/{projectKey}/tms/attachment/{attachmentId}",
	"GET /v1/project/{projectKey}/tms/attachment/{attachmentId}/thumbnail",
	"GET /v1/project/{projectKey}/tms/folder/{folderId}/export/{fileType}",
	"GET /v1/project/{projectKey}/tms/sync/jobs",
	"GET /v1/project/{projectKey}/tms/test-case/export",
	"GET /v1/public/integration/grafana/session-check",
	"GET /v1/users/export",
	"GET /v1/{projectKey}/launch/{launchId}/report",
	"PATCH /v1/project/{projectKey}/tms/test-case/attributes/batch",
	"POST /users/{user_id}/avatar",
	"POST /v1/project/{projectKey}/launch/manual/{launchId}/test-case",
	"POST /v1/project/{projectKey}/tms/milestone/{id}/test-plan/{testPlanId}",
	"POST /v1/project/{projectKey}/tms/sync/jobs",
)

// operationTemplate is an operation of the API specification: HTTP method and path template
type operationTemplate struct {
	method   string
	segments []string
}

func parseOperationTemplates(operations ...string) []operationTemplate {
	templates := make([]operationTemplate, 0, len(operations))
	for _, operation := range operations {
		method, path, _ := strings.Cut(operation, " ")
		templates = append(
			templates,
			operationTemplate{method: method, segments: pathSegments(path)},
		)
	}

	return templates
}

func pathSegments(path string) []string {
	return strings.Split(strings.Trim(path, "/"), "/")
}

func isPathParameter(segment string) bool {
	return strings.HasPrefix(segment, "{")
}

// matches reports whether a request with the method and the path segments is a call of the operation.
// The template is compared with the end of the path: what precedes it is the base path of the server
// ("/api" by default) and whatever prefix the installation is served under.
func (o operationTemplate) matches(method string, segments []string) bool {
	if o.method != method || len(o.segments) > len(segments) {
		return false
	}
	tail := segments[len(segments)-len(o.segments):]
	for i, segment := range o.segments {
		if !isPathParameter(segment) && segment != tail[i] {
			return false
		}
	}

	return true
}

func isNonJSONOperation(req *http.Request) bool {
	if req.URL == nil {
		return false
	}
	// The escaped form keeps a path parameter with an encoded slash in one segment
	segments := pathSegments(req.URL.EscapedPath())
	for _, operation := range nonJSONOperations {
		if operation.matches(req.Method, segments) {
			return true
		}
	}

	return false
}
