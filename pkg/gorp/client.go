package gorp

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"golang.org/x/oauth2"

	"github.com/reportportal/goRP/v5/pkg/openapi"
)

const defaultHTTPTimeout = 30 * time.Second

type HTTPError struct {
	StatusCode int
	Response   string
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("status code error: %d\n%s", e.StatusCode, e.Response)
}

func getClient(ctx context.Context) *http.Client {
	if ctx != nil {
		if hc, ok := ctx.Value(oauth2.HTTPClient).(*http.Client); ok {
			return hc
		}
	}
	return &http.Client{
		Timeout: defaultHTTPTimeout,
	}
}

type ClientConfigOpts struct {
	URL      string `json:"host"`
	Project  string `json:"project"`
	ApiToken string `json:"api_token"`
}
type ClientOption func() *http.Client

func WithApiKeyAuth(ctx context.Context, apiKey string) ClientOption {
	return func() *http.Client {
		ctx = context.WithValue(ctx, oauth2.HTTPClient, getClient(ctx))
		ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: apiKey})
		return oauth2.NewClient(ctx, ts)
	}
}

func WithPasswordOwnerGrantAuth(
	ctx context.Context,
	oauth2Config *oauth2.Config,
	username, password string,
) ClientOption {
	return func() *http.Client {
		ctx = context.WithValue(ctx, oauth2.HTTPClient, getClient(ctx))
		ts := NewPasswordGrantTokenSource(ctx, oauth2Config, username, password)
		return oauth2.NewClient(ctx, ts)
	}
}

// Client is ReportPortal REST API Client
type Client struct {
	*launchClient
	*openapi.APIClient
}

// NewClient creates new instance of Client
// u - URL of the server
// clientOption - authentication, see [WithApiKeyAuth] and [WithPasswordOwnerGrantAuth]
// opts - optional settings, see [Option]
func NewClient(u *url.URL, clientOption ClientOption, opts ...Option) *Client {
	o := newClientOptions(opts)
	httpClient := clientOption()
	return &Client{
		launchClient: &launchClient{
			http: o.newResty(httpClient, u.String()),
		},
		APIClient: newAPIClient(u, o.retry.httpClient(httpClient), o),
	}
}

func newAPIClient(u *url.URL, httpClient *http.Client, o *clientOptions) *openapi.APIClient {
	conf := openapi.NewConfiguration()
	conf.HTTPClient = httpClient
	conf.Scheme = u.Scheme
	conf.Host = u.Host
	o.configureOpenAPI(conf)
	return openapi.NewAPIClient(conf)
}
