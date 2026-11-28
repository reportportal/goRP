package gorp

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"math"
	mathrand "math/rand/v2"
	"net/http"
	"strconv"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"resty.dev/v3"

	"github.com/reportportal/goRP/v5/pkg/openapi"
)

const (
	defaultRetryMinWait = 500 * time.Millisecond
	defaultRetryMaxWait = 10 * time.Second

	headerRetryAfter = "Retry-After"
	// maxDrainBytes is how much of a failed response is read to keep its connection reusable
	maxDrainBytes = 64 << 10
)

// RetryPolicy says how a client repeats a failed request, see [WithRetry].
type RetryPolicy struct {
	// MaxRetries is the number of attempts after the first one. Zero or less: no retries.
	MaxRetries int
	// MinWait is the pause before the first retry, 500 ms by default. Every next pause is twice
	// as long, up to MaxWait. The actual pause is random between a half and the whole of that
	// value, so clients that failed together don't come back together.
	MinWait time.Duration
	// MaxWait is the longest pause, 10 s by default. It also limits a Retry-After the server asks for.
	MaxWait time.Duration
}

// WithRetry makes the client repeat a request that failed for a reason that may pass:
//
//   - a network error: the connection was refused or reset, or timed out;
//   - HTTP 5xx, except 501 Not Implemented;
//   - HTTP 429 Too Many Requests.
//
// Other 4xx responses are never retried, the same request would fail the same way. The pause between
// attempts grows exponentially with a random part; a Retry-After header of the response is honored,
// up to [RetryPolicy.MaxWait]. Retries stop as soon as the context of the call is done. When all
// attempts fail the call returns the error of the last one.
//
// Only requests that are safe to repeat are retried:
//
//   - GET, PUT and DELETE, which includes finishing launches and test items;
//   - the start of a launch or a test item, because it carries a UUID chosen by the client and the
//     server stores a UUID only once. [ReportingClient] fills in the UUID when the request has none.
//     A raw start ([ReportingClient.StartLaunchRaw] etc.) is retried only if its body has a "uuid";
//     a start with rerun is not retried, the server chooses the UUID of a rerun itself;
//   - no other POST. In particular logs are never sent twice: the server doesn't deduplicate them.
//
// For the generated client embedded in [Client] this means that GET, PUT and DELETE operations are
// retried and POST operations are not.
func WithRetry(policy RetryPolicy) Option {
	return func(o *clientOptions) {
		o.retry = policy.normalized()
	}
}

func (p RetryPolicy) enabled() bool {
	return p.MaxRetries > 0
}

func (p RetryPolicy) normalized() RetryPolicy {
	if p.MinWait <= 0 {
		p.MinWait = defaultRetryMinWait
	}
	if p.MaxWait <= 0 {
		p.MaxWait = defaultRetryMaxWait
	}
	p.MaxWait = max(p.MaxWait, p.MinWait)

	return p
}

// retryableStatus reports whether a response with the status may be different next time.
func retryableStatus(code int) bool {
	return code == http.StatusTooManyRequests ||
		(code >= http.StatusInternalServerError && code != http.StatusNotImplemented)
}

// retryableError reports whether an error of sending a request (no response) may pass.
// Everything is, except what repeats for certain: a finished context, a certificate
// that can't be trusted, a refused token, a malformed request.
func retryableError(ctx context.Context, err error) bool {
	if err == nil || ctx.Err() != nil || errors.Is(err, context.Canceled) {
		return false
	}

	var (
		certErr      *tls.CertificateVerificationError
		authorityErr x509.UnknownAuthorityError
		hostErr      x509.HostnameError
		invalidErr   x509.CertificateInvalidError
		tokenErr     *oauth2.RetrieveError
	)
	switch {
	case errors.As(err, &certErr),
		errors.As(err, &authorityErr),
		errors.As(err, &hostErr),
		errors.As(err, &invalidErr):
		return false
	case errors.As(err, &tokenErr):
		// the token endpoint answered: the answer decides
		return tokenErr.Response != nil && retryableStatus(tokenErr.Response.StatusCode)
	}

	msg := err.Error()
	for _, permanent := range []string{"unsupported protocol scheme", "invalid header", "redirects", "no Host in request URL"} {
		if strings.Contains(msg, permanent) {
			return false
		}
	}

	return true
}

// wait returns the pause before the retry that follows the given attempt (1 is the first attempt).
// resp is the response of that attempt, nil if there was none.
func (p RetryPolicy) wait(attempt int, resp *http.Response) time.Duration {
	if resp != nil {
		if d, ok := parseRetryAfter(resp.Header.Get(headerRetryAfter), time.Now()); ok {
			return min(d, p.MaxWait)
		}
	}

	ceiling := math.Min(float64(p.MaxWait), float64(p.MinWait)*math.Exp2(float64(attempt-1)))
	half := time.Duration(ceiling / 2)
	if half <= 0 {
		return time.Duration(ceiling)
	}

	return half + mathrand.N(half+1)
}

// parseRetryAfter reads a Retry-After header: a number of seconds or an HTTP date.
func parseRetryAfter(v string, now time.Time) (time.Duration, bool) {
	v = strings.TrimSpace(v)
	if v == "" {
		return 0, false
	}
	if seconds, err := strconv.ParseInt(v, 10, 64); err == nil {
		if seconds < 0 {
			return 0, false
		}
		if seconds > int64(math.MaxInt64/time.Second) {
			return math.MaxInt64, true
		}

		return time.Duration(seconds) * time.Second, true
	}
	if at, err := http.ParseTime(v); err == nil {
		return max(at.Sub(now), 0), true
	}

	return 0, false
}

// configureResty turns on the retries of Resty for the hand-written methods. Resty itself retries
// only methods that are idempotent by definition; a request opts in with allowRetry.
func (p RetryPolicy) configureResty(c *resty.Client) {
	if !p.enabled() {
		return
	}

	c.SetRetryCount(p.MaxRetries).
		// the conditions and the pauses of Resty are replaced with the ones of the policy,
		// so that both clients behave the same
		SetRetryDefaultConditions(false).
		AddRetryConditions(func(rs *resty.Response, err error) bool {
			if rs.RawResponse == nil {
				return retryableError(rs.Request.Context(), err)
			}

			return retryableStatus(rs.StatusCode())
		}).
		// Resty takes a Retry-After of a 429 or 503 response as is, however long, before it asks the
		// delay strategy. The hook runs before that and brings the header down to the policy's limit.
		AddRetryHooks(func(rs *resty.Response, _ error) {
			if rs.RawResponse == nil {
				return
			}
			if d, ok := parseRetryAfter(
				rs.Header().Get(headerRetryAfter),
				time.Now(),
			); ok &&
				d > p.MaxWait {
				rs.Header().
					Set(headerRetryAfter, strconv.FormatInt(int64(math.Ceil(p.MaxWait.Seconds())), 10))
			}
		}).
		SetRetryDelayStrategy(func(rs *resty.Response, _ error) (time.Duration, error) {
			return p.wait(rs.Request.Attempt, rs.RawResponse), nil
		})
}

// retryTransport retries requests of the generated client, which has no retries of its own.
type retryTransport struct {
	base   http.RoundTripper
	policy RetryPolicy
}

// httpClient returns a client that retries. The given client is not modified: the Resty-based
// methods share it and retry on their own, wrapping it would multiply the attempts.
func (p RetryPolicy) httpClient(c *http.Client) *http.Client {
	if !p.enabled() {
		return c
	}

	base := c.Transport
	if base == nil {
		base = http.DefaultTransport
	}
	retrying := *c
	retrying.Transport = &retryTransport{base: base, policy: p}

	return &retrying
}

func (t *retryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if !replayable(req) {
		return t.base.RoundTrip(req)
	}

	ctx := req.Context()
	for attempt := 1; ; attempt++ {
		attemptReq := req
		if attempt > 1 && req.GetBody != nil {
			body, err := req.GetBody()
			if err != nil {
				return nil, err
			}
			attemptReq = req.Clone(ctx)
			attemptReq.Body = body
		}

		resp, err := t.base.RoundTrip(attemptReq)
		if attempt > t.policy.MaxRetries {
			return resp, err
		}
		if err != nil {
			if !retryableError(ctx, err) {
				return nil, err
			}
		} else if !retryableStatus(resp.StatusCode) {
			return resp, nil
		}

		pause := t.policy.wait(attempt, resp)
		if resp != nil {
			_, _ = io.CopyN(io.Discard, resp.Body, maxDrainBytes)
			_ = resp.Body.Close()
		}
		if sleepErr := sleep(ctx, pause); sleepErr != nil {
			if err != nil {
				return nil, errors.Join(sleepErr, err)
			}

			return nil, sleepErr
		}
	}
}

// replayable reports whether the transport may send the request again: the method is idempotent
// and the body, if any, can be read once more.
func replayable(req *http.Request) bool {
	switch req.Method {
	case http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodPut, http.MethodDelete:
		return req.Body == nil || req.Body == http.NoBody || req.GetBody != nil
	default:
		return false
	}
}

func sleep(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// retryableStart prepares the body of a request that starts a launch or a test item and reports
// whether the request may be repeated. It may when the server can tell the copy from a new entity,
// i.e. when the body carries a UUID. A typed request without one gets it here, in a copy.
func (p RetryPolicy) retryableStart(body any) (any, bool) {
	if !p.enabled() {
		return body, false
	}

	switch rq := body.(type) {
	case *openapi.StartLaunchRQ:
		if rq == nil || rq.GetRerun() {
			return body, false
		}
		if rq.Uuid == "" {
			withUUID := *rq
			withUUID.Uuid = newUUID()

			return &withUUID, true
		}

		return body, true
	case *openapi.StartTestItemRQ:
		if rq == nil {
			return body, false
		}
		if rq.Uuid == "" {
			withUUID := *rq
			withUUID.Uuid = newUUID()

			return &withUUID, true
		}

		return body, true
	case json.RawMessage:
		var start struct {
			UUID  string `json:"uuid"`
			Rerun bool   `json:"rerun"`
		}

		return body, json.Unmarshal(rq, &start) == nil && start.UUID != "" && !start.Rerun
	default:
		return body, false
	}
}

// newUUID returns a random (version 4) UUID.
func newUUID() string {
	var b [16]byte
	// crypto/rand.Read never fails and never returns less than asked
	_, _ = rand.Read(b[:])
	b[6] = b[6]&0x0f | 0x40 // version 4
	b[8] = b[8]&0x3f | 0x80 // RFC 9562 variant

	var out [36]byte
	hex.Encode(out[0:8], b[0:4])
	out[8] = '-'
	hex.Encode(out[9:13], b[4:6])
	out[13] = '-'
	hex.Encode(out[14:18], b[6:8])
	out[18] = '-'
	hex.Encode(out[19:23], b[8:10])
	out[23] = '-'
	hex.Encode(out[24:36], b[10:16])

	return string(out[:])
}
