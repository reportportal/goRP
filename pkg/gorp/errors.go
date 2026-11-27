package gorp

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"mime"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"golang.org/x/oauth2"
	"resty.dev/v3"

	"github.com/reportportal/goRP/v5/pkg/openapi"
)

// Sentinel errors matched by [HTTPError.Is], for use with [errors.Is]:
//
//	if errors.Is(err, gorp.ErrNotFound) { ... }
var (
	// ErrUnauthorized matches a 401 response: missing, wrong or expired API key
	ErrUnauthorized = errors.New("unauthorized")
	// ErrForbidden matches a 403 response
	ErrForbidden = errors.New("forbidden")
	// ErrNotFound matches a 404 response
	ErrNotFound = errors.New("not found")
	// ErrNotAPIResponse matches a successful response that carries an HTML page instead of JSON.
	// The host or path most probably points to the ReportPortal UI, an SSO page or another server.
	// Only the Resty-based clients report it; the generated client fails to decode such a response.
	ErrNotAPIResponse = errors.New("not a ReportPortal API response")
)

// maxErrorBodyLen limits the part of a response body that goes into the error text
// when the body has no message to extract. [HTTPError.Response] always keeps the whole body.
const maxErrorBodyLen = 1024

const (
	hintNotAPI     = "expected JSON but got an HTML page, the host or path is probably not the ReportPortal API"
	hintNoEndpoint = "no such endpoint, check the host and the path"
)

// HTTPError is the error both clients return for a failed API call: the Resty-based [Client] and
// [ReportingClient] methods and every operation of the embedded generated [openapi.APIClient].
// A failed token request of [WithPasswordOwnerGrantAuth] is reported the same way, with the token
// endpoint as the request. Retrieve it with [errors.As].
//
// Only StatusCode and Response are always set. The other fields depend on what the server sent:
// ReportPortal's own error body fills ErrorCode and Message, an authentication failure fills
// OAuthError and Message.
type HTTPError struct {
	// StatusCode is the HTTP status code
	StatusCode int
	// Response is the response body as received
	Response string

	// Status is the HTTP status line, e.g. "404 Not Found"
	Status string
	// Method and URL identify the request
	Method string
	URL    string
	// ErrorCode is the ReportPortal error code (errorCode of the error body), e.g. 4041. Zero if there is none
	ErrorCode int64
	// Message is the server's explanation: the message of the ReportPortal error body or the OAuth error_description
	Message string
	// OAuthError is the OAuth error of an authentication failure, e.g. "invalid_token"
	OAuthError string

	// hint replaces the body in the error text when the body itself explains nothing
	hint   string
	notAPI bool
	cause  error
}

// Error returns a single line: request, HTTP status and what the server said about the failure.
// A body without a recognizable message is appended as is, cut to a reasonable length.
func (e *HTTPError) Error() string {
	var sb strings.Builder
	if e.Method != "" || e.URL != "" {
		sb.WriteString(strings.TrimSpace(e.Method + " " + stripQuery(e.URL)))
		sb.WriteString(": ")
	}
	if e.Status != "" {
		sb.WriteString(e.Status)
	} else {
		sb.WriteString(statusLine(e.StatusCode))
	}

	detail := e.Message
	if e.OAuthError != "" {
		detail = strings.TrimSuffix(e.OAuthError+": "+detail, ": ")
	}
	if detail == "" {
		detail = e.hint
	}
	if detail == "" {
		detail = trimBody(e.Response)
	}
	if detail != "" {
		sb.WriteString(": ")
		sb.WriteString(detail)
	}
	if e.ErrorCode != 0 {
		sb.WriteString(" (error code ")
		sb.WriteString(strconv.FormatInt(e.ErrorCode, 10))
		sb.WriteString(")")
	}

	return sb.String()
}

// Unwrap returns the [*openapi.GenericOpenAPIError] with the same status and body when the error
// comes from the generated client, the [*oauth2.RetrieveError] when a token request failed,
// nil otherwise.
func (e *HTTPError) Unwrap() error {
	return e.cause
}

// Is reports whether the error matches one of the sentinel errors of this package.
func (e *HTTPError) Is(target error) bool {
	switch target {
	case ErrUnauthorized:
		return e.StatusCode == http.StatusUnauthorized
	case ErrForbidden:
		return e.StatusCode == http.StatusForbidden
	case ErrNotFound:
		return e.StatusCode == http.StatusNotFound
	case ErrNotAPIResponse:
		return e.notAPI
	default:
		return false
	}
}

// OpenAPIErrorMiddleware is the [openapi.ResponseMiddlewareFunction] that [NewClient] installs on
// the generated client. It turns every failed response into an [*HTTPError], whether the API
// specification declares the status for the operation or not.
//
// Set it as ResponseMiddleware of the [openapi.Configuration] of a client you build yourself.
// The returned error unwraps to [*openapi.GenericOpenAPIError].
func OpenAPIErrorMiddleware(resp *http.Response, body []byte) error {
	if resp == nil {
		return nil
	}
	// The generated code treats everything from 300 as a failure. A successful response is left to it,
	// even an HTML one: operations that download files and reports (GetFile, GetLaunchReport, …) are
	// indistinguishable here from those that expect JSON, and the latter fail on HTML anyway.
	if resp.StatusCode < http.StatusMultipleChoices {
		return nil
	}

	httpErr, errorRS := newHTTPError(resp, body)
	var model any
	if errorRS != nil {
		model = *errorRS
	}
	httpErr.cause = openapi.NewGenericOpenAPIError(httpErr.Status, body, model)

	return httpErr
}

func defaultHTTPErrorHandler(_ *resty.Client, rs *resty.Response) error {
	notAPI := rs.IsStatusSuccess() && isHTML(rs.Header())
	if !rs.IsStatusFailure() && !notAPI {
		return nil
	}
	if rs.RawResponse == nil {
		return &HTTPError{
			StatusCode: rs.StatusCode(),
			Status:     rs.Status(),
			Response:   string(rs.Bytes()),
		}
	}
	if notAPI {
		return newNotAPIError(rs.RawResponse, rs.Bytes())
	}
	httpErr, _ := newHTTPError(rs.RawResponse, rs.Bytes())

	return httpErr
}

// newTokenError reports a failed token request in the shape of a failed API call.
// The original error stays reachable through [HTTPError.Unwrap].
func newTokenError(retrieveErr *oauth2.RetrieveError) error {
	if retrieveErr.Response == nil {
		return retrieveErr
	}
	httpErr, _ := newHTTPError(retrieveErr.Response, retrieveErr.Body)
	// The oauth2 package also reads bodies that are not JSON (form-encoded)
	if httpErr.OAuthError == "" && httpErr.Message == "" {
		httpErr.OAuthError = retrieveErr.ErrorCode
		httpErr.Message = retrieveErr.ErrorDescription
	}
	httpErr.cause = retrieveErr

	return httpErr
}

// newNotAPIError is the error for a successful response that has an HTML page instead of JSON.
func newNotAPIError(resp *http.Response, body []byte) *HTTPError {
	httpErr := baseHTTPError(resp, body)
	httpErr.notAPI = true
	httpErr.hint = hintNotAPI

	return httpErr
}

// errorBody is a union of the error bodies a ReportPortal installation answers with:
//   - ReportPortal's ErrorRS: {"errorCode":4041,"message":"…"}
//   - authentication failure: {"error":"invalid_token","error_description":"…"}
//   - Spring Boot default, for unknown paths and methods:
//     {"timestamp":"…","status":404,"error":"Not Found","path":"…"}
type errorBody struct {
	ErrorCode        json.RawMessage `json:"errorCode"`
	Message          string          `json:"message"`
	Error            string          `json:"error"`
	ErrorDescription string          `json:"error_description"`
	Status           json.RawMessage `json:"status"`
	Path             string          `json:"path"`
	Timestamp        json.RawMessage `json:"timestamp"`
}

var wwwAuthenticateParam = regexp.MustCompile(`\b(error|error_description)="([^"]*)"`)

// baseHTTPError fills in what does not depend on the body: the status and the request.
func baseHTTPError(resp *http.Response, body []byte) *HTTPError {
	httpErr := &HTTPError{
		StatusCode: resp.StatusCode,
		Status:     resp.Status,
		Response:   string(body),
	}
	if httpErr.Status == "" {
		httpErr.Status = statusLine(resp.StatusCode)
	}
	if resp.Request != nil {
		httpErr.Method = resp.Request.Method
		if resp.Request.URL != nil {
			httpErr.URL = resp.Request.URL.Redacted()
		}
	}

	return httpErr
}

// newHTTPError builds the error for a response of any shape. It never fails: what can't be parsed
// stays available in Response. The second result is the ReportPortal error model, if the body is one.
func newHTTPError(
	resp *http.Response,
	body []byte,
) (*HTTPError, *openapi.ComEpamReportportalBaseInfrastructureRulesExceptionErrorRS) {
	httpErr := baseHTTPError(resp, body)

	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		httpErr.fillFromChallenge(resp.Header.Get("WWW-Authenticate"))

		return httpErr, nil
	}

	var parsed errorBody
	if trimmed[0] != '{' || json.Unmarshal(trimmed, &parsed) != nil {
		return httpErr, nil
	}

	switch {
	case parsed.ErrorCode != nil || (parsed.Message != "" && parsed.Status == nil && parsed.Error == ""):
		httpErr.Message = parsed.Message
		httpErr.ErrorCode, _ = strconv.ParseInt(strings.Trim(string(parsed.ErrorCode), `"`), 10, 64)

		var errorRS openapi.ComEpamReportportalBaseInfrastructureRulesExceptionErrorRS
		if json.Unmarshal(trimmed, &errorRS) == nil {
			return httpErr, &errorRS
		}
	case parsed.ErrorDescription != "" || (parsed.Error != "" && parsed.Status == nil && parsed.Path == ""):
		httpErr.OAuthError = parsed.Error
		httpErr.Message = parsed.ErrorDescription
	case parsed.Status != nil && (parsed.Path != "" || parsed.Timestamp != nil):
		// Besides an optional message the body only repeats the status and the request path
		httpErr.Message = parsed.Message
		if resp.StatusCode == http.StatusNotFound {
			httpErr.hint = hintNoEndpoint
		}
	}

	return httpErr, nil
}

// fillFromChallenge takes the reason from a WWW-Authenticate header, the only place that has it
// when a 401 comes without a body.
func (e *HTTPError) fillFromChallenge(challenge string) {
	if challenge == "" {
		return
	}
	for _, m := range wwwAuthenticateParam.FindAllStringSubmatch(challenge, -1) {
		if m[1] == "error" {
			e.OAuthError = m[2]
		} else {
			e.Message = m[2]
		}
	}
	if e.OAuthError == "" && e.Message == "" {
		e.hint = "WWW-Authenticate: " + challenge
	}
}

func statusLine(code int) string {
	return strings.TrimSpace(strconv.Itoa(code) + " " + http.StatusText(code))
}

func isHTML(h http.Header) bool {
	mediaType, _, err := mime.ParseMediaType(h.Get("Content-Type"))

	return err == nil && (mediaType == "text/html" || mediaType == "application/xhtml+xml")
}

func stripQuery(u string) string {
	if i := strings.IndexAny(u, "?#"); i >= 0 {
		return u[:i]
	}

	return u
}

// trimBody makes a body fit for a one-line message: whitespace collapsed, length limited.
func trimBody(body string) string {
	s := strings.Join(strings.Fields(body), " ")
	if len(s) <= maxErrorBodyLen {
		return s
	}
	cut := maxErrorBodyLen
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}

	return fmt.Sprintf("%s… (%d bytes in total)", s[:cut], len(body))
}
