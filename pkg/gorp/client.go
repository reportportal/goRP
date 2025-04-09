package gorp

import (
	"fmt"

	"resty.dev/v3"
)

type HTTPError struct {
	StatusCode int
	Response   string
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("status code error: %d\n%s", e.StatusCode, e.Response)
}

// Client is ReportPortal REST API Client
type Client struct {
	*ReportingClient
	*APIClient
}

// NewClient creates new instance of Client
// host - server hostname
// project - name of the project
// apiKey - User Token (see user profile page)
func NewClient(host, project, apiKey string) *Client {
	http := resty.New().
		SetBaseURL(host).
		SetAuthToken(apiKey).
		AddResponseMiddleware(defaultHTTPErrorHandler)

	return &Client{
		ReportingClient: &ReportingClient{
			project: project,
			http:    http,
		},
		APIClient: newAPIClient(http, project),
	}
}
