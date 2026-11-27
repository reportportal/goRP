package gorp

import (
	"net/http"

	"resty.dev/v3"

	"github.com/reportportal/goRP/v5/pkg/openapi"
)

// Option configures a [Client] or a [ReportingClient]. Pass options to [NewClient] and
// [NewReportingClient] after the authentication [ClientOption].
type Option func(*clientOptions)

// ResponseMiddleware is called for every HTTP response a client receives, successful or not,
// with the response and its body. The body argument is the place to read the body from:
// resp.Body may be consumed already. A returned error becomes the error of the API call.
//
// It has the shape of [openapi.ResponseMiddlewareFunction], so one function serves the Resty-based
// methods and the generated client.
type ResponseMiddleware func(resp *http.Response, body []byte) error

type clientOptions struct {
	responseMiddlewares []ResponseMiddleware
}

func newClientOptions(opts []Option) *clientOptions {
	o := &clientOptions{}
	for _, opt := range opts {
		if opt != nil {
			opt(o)
		}
	}

	return o
}

// WithResponseMiddleware adds a [ResponseMiddleware]. Middlewares are chained, not replaced: they run
// in the order they were added and before the client's own error handling, which stays in place.
// The first error stops the chain, so a middleware that returns an error also decides what the caller
// gets instead of the client's own error for a failed response.
//
// Typical uses are logging, metrics and mapping particular responses to errors of your own.
// A response that never arrived (network error, canceled context) doesn't reach the middlewares.
func WithResponseMiddleware(m ResponseMiddleware) Option {
	return func(o *clientOptions) {
		if m != nil {
			o.responseMiddlewares = append(o.responseMiddlewares, m)
		}
	}
}

// runResponseMiddlewares runs the user's middlewares and stops at the first error.
func (o *clientOptions) runResponseMiddlewares(resp *http.Response, body []byte) error {
	for _, m := range o.responseMiddlewares {
		if err := m(resp, body); err != nil {
			return err
		}
	}

	return nil
}

// newResty builds the Resty client all hand-written methods use.
func (o *clientOptions) newResty(httpClient *http.Client, baseURL string) *resty.Client {
	c := resty.NewWithClient(httpClient).SetBaseURL(baseURL)
	if len(o.responseMiddlewares) == 0 {
		return c.AddResponseMiddleware(defaultHTTPErrorHandler)
	}

	// Resty hands the body of a successfully parsed response over to the result object;
	// buffering keeps it readable for the middlewares as well.
	c.SetResponseBodyUnlimitedReads(true)

	// Resty runs every response middleware even after one of them failed, so the user's
	// middlewares and the error handler are one middleware: the first error wins.
	return c.AddResponseMiddleware(func(c *resty.Client, rs *resty.Response) error {
		if rs.RawResponse != nil {
			if err := o.runResponseMiddlewares(rs.RawResponse, rs.Bytes()); err != nil {
				return err
			}
		}

		return defaultHTTPErrorHandler(c, rs)
	})
}

// configureOpenAPI applies the options to the configuration of the generated client.
// The generated client has a single response middleware, so the user's middlewares are put
// in front of the one the configuration already has.
func (o *clientOptions) configureOpenAPI(conf *openapi.Configuration) {
	if len(o.responseMiddlewares) == 0 {
		return
	}

	own := conf.ResponseMiddleware
	conf.ResponseMiddleware = func(resp *http.Response, body []byte) error {
		if err := o.runResponseMiddlewares(resp, body); err != nil {
			return err
		}
		if own != nil {
			return own(resp, body)
		}

		return nil
	}
}
