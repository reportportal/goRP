package rpagent

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/reportportal/goRP/v5/pkg/gorp"
)

const httpTimeout = 30 * time.Second

// NewReportingClient builds a goRP ReportingClient that authenticates with an API key.
// endpoint may carry a trailing slash or "/api" suffix; both are stripped.
// When debug is true, request/response bodies are written to rp-agent.log.
func NewReportingClient(endpoint, project, apiKey string, debug bool) *gorp.ReportingClient {
	host := strings.TrimSuffix(strings.TrimRight(endpoint, "/"), "/api")
	auth := gorp.WithApiKeyAuth(context.Background(), apiKey)

	option := func() *http.Client {
		c := auth()
		if c.Timeout == 0 {
			c.Timeout = httpTimeout
		}
		if debug && currentLogger() != nil {
			c.Transport = &loggingTransport{next: c.Transport}
		}
		return c
	}
	return gorp.NewReportingClient(host, project, option)
}

// loggingTransport records HTTP exchanges in rp-agent.log. Headers are never logged,
// so the API key cannot leak. Multipart bodies (attachments) are skipped.
type loggingTransport struct {
	next http.RoundTripper
}

func (t *loggingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	start := time.Now()

	var reqBody string
	if req.GetBody != nil && !strings.HasPrefix(req.Header.Get("Content-Type"), "multipart/") {
		if rc, err := req.GetBody(); err == nil {
			b, _ := io.ReadAll(rc)
			_ = rc.Close()
			reqBody = string(b)
		}
	}

	resp, err := t.next.RoundTrip(req)
	if err != nil {
		logHTTP(req.Method, req.URL.String(), err.Error(), time.Since(start), reqBody, "")
		return nil, err
	}

	body, readErr := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if readErr != nil {
		return nil, readErr
	}
	resp.Body = io.NopCloser(bytes.NewReader(body))

	logHTTP(req.Method, req.URL.String(), resp.Status, time.Since(start), reqBody, string(body))
	return resp, nil
}

// goRP defines DEBUG, INFO and ERROR; these two cover the remaining agent log levels.
const (
	logLevelWarn  = "WARN"
	logLevelFatal = "FATAL"
)
