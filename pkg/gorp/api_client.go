package gorp

import (
	"resty.dev/v3"
)

const baseUrlLaunch = "/api/v1/{project}/launch"

// APIClient is ReportPortal REST API Client
type APIClient struct {
	*launchClient
	*filterClient
}

// NewAPIClient creates new instance of Client
// host - server hostname
// project - name of the project
// apiKey - User Token (see user profile page)
func NewAPIClient(host, project, apiKey string) *APIClient {
	http := resty.New().
		SetBaseURL(host).
		SetAuthToken(apiKey).
		AddResponseMiddleware(defaultHTTPErrorHandler)

	return newAPIClient(http, project)
}

func newAPIClient(http *resty.Client, project string) *APIClient {
	return &APIClient{
		launchClient: &launchClient{
			project: project,
			http:    http,
		},
		filterClient: &filterClient{
			project: project,
			http:    http,
		},
	}
}
