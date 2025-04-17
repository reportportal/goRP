package gorp

import (
	"fmt"
	"net/url"

	"resty.dev/v3"

	"github.com/reportportal/goRP/v5/pkg/openapi"
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
	*launchClient
	*openapi.APIClient
}

// NewClient creates new instance of Client
// host - server hostname
// project - name of the project
// apiKey - User Token (see user profile page)
func NewClient(host *url.URL, project, apiKey string) *Client {
	http := resty.New().
		SetBaseURL(host.String()).
		SetAuthToken(apiKey).
		AddResponseMiddleware(defaultHTTPErrorHandler)

	return &Client{
		ReportingClient: &ReportingClient{
			project: project,
			http:    http,
		},
		launchClient: &launchClient{
			http: http,
		},
		APIClient: newAPIClient(host, project),
	}
}

func newAPIClient(u *url.URL, uuid string) *openapi.APIClient {
	conf := openapi.NewConfiguration()
	conf.Host = u.Host
	conf.Scheme = u.Scheme
	conf.AddDefaultHeader("Authorization", "Bearer "+uuid)
	return openapi.NewAPIClient(conf)
}
