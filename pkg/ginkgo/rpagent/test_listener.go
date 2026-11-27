package rpagent

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/google/uuid"

	"github.com/reportportal/goRP/v5/pkg/gorp"
	"github.com/reportportal/goRP/v5/pkg/openapi"
)

// isRetriableError reports whether an error is a transient failure worth retrying:
// HTTP 5xx and 429, timeouts, connection refused/reset, DNS failures, and a connection
// that closed mid-response. Everything else (4xx, cancellation, bad requests) is final.
func isRetriableError(err error) bool {
	if err == nil {
		return false
	}
	var httpErr *gorp.HTTPError
	if errors.As(err, &httpErr) {
		return httpErr.StatusCode >= 500 || httpErr.StatusCode == 429
	}
	if errors.Is(err, context.Canceled) {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) ||
		errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) ||
		errors.Is(err, syscall.ECONNREFUSED) || errors.Is(err, syscall.ECONNRESET) ||
		errors.Is(err, syscall.ECONNABORTED) || errors.Is(err, syscall.EPIPE) {
		return true
	}
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return true
	}
	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}

// retry runs f up to attempts times with exponential backoff + jitter.
// It stops early on non-retriable errors.
func retry(attempts int, baseDelay time.Duration, f func() error) error {
	var err error
	for i := 0; i < attempts; i++ {
		if err = f(); err == nil {
			return nil
		}
		if !isRetriableError(err) {
			return err
		}
		if i == attempts-1 {
			break
		}
		backoff := baseDelay * time.Duration(1<<uint(i))
		jitter := time.Duration(rand.Int63n(int64(backoff)/2 + 1)) //nolint:gosec
		log.Printf("retrying in %v (attempt %d/%d): %v", backoff+jitter, i+2, attempts, err)
		time.Sleep(backoff + jitter)
	}
	return fmt.Errorf("after %d attempts, last error: %w", attempts, err)
}

// orNow returns t, or the current time when t is the zero value.
func orNow(t time.Time) time.Time {
	if t.IsZero() {
		return time.Now()
	}
	return t
}

func boolPtr(b bool) *bool            { return &b }
func strPtr(s string) *string         { return &s }
func statusPtr(s gorp.Status) *string { return strPtr(string(s)) }

// toParameterResources converts agent parameters to the generated API type.
func toParameterResources(params []*Parameter) []openapi.ParameterResource {
	if len(params) == 0 {
		return nil
	}
	out := make([]openapi.ParameterResource, 0, len(params))
	for _, p := range params {
		out = append(out, openapi.ParameterResource{Key: p.Key, Value: strPtr(p.Value)})
	}
	return out
}

// TestListener wraps the ReportPortal client with retry logic and convenience methods.
type TestListener struct {
	client     *gorp.ReportingClient
	launchUuid string
	launchName string

	mu      sync.Mutex
	pending []*openapi.SaveLogRQ // queued by QueueLog, sent by FlushLogs
}

// NewTestListener creates a TestListener.
func NewTestListener(client *gorp.ReportingClient, launchUuid, launchName string) *TestListener {
	return &TestListener{client: client, launchUuid: launchUuid, launchName: launchName}
}

// FinishSuite finishes a suite item.
func (tl *TestListener) FinishSuite(name string, status gorp.Status, id string) {
	if id == "" {
		log.Printf("no suite ID for %q, skipping finish", name)
		return
	}
	_, err := tl.client.FinishTest(context.Background(), id, &openapi.FinishTestItemRQ{
		EndTime:    time.Now(),
		Status:     statusPtr(status),
		LaunchUuid: tl.launchUuid,
	})
	if err != nil {
		log.Printf("failed to finish suite %q: %v", name, err)
	}
}

// StartItem describes a test item to start in ReportPortal.
type StartItem struct {
	Name       string
	ParentID   string // empty for a top-level item
	Priority   *int
	Parameters []*Parameter
	Type       gorp.TestItemType
	RefID      string // external test reference, sent as an attribute
	// Labels are Ginkgo labels, sent as bare-tag attributes (rp.* control labels excluded).
	Labels []string
	// NoStats excludes the item from launch statistics. Set it for By() child steps.
	NoStats bool
	// StartTime is when the item actually started. Zero means now.
	StartTime time.Time
	// Identity holds the run-independent testCaseId, codeRef and uniqueId source.
	Identity ItemIdentity
	// RetryOf is the ID of the previous attempt of the same spec. Setting it makes this
	// item a ReportPortal retry: the earlier attempts are folded under the last one and
	// only the last counts in the launch statistics.
	RetryOf string
}

// Start starts a test item and returns its ReportPortal ID, or "" on failure.
func (tl *TestListener) Start(item StartItem) string {
	// The item's own UUID is generated once per start request and reused across
	// retries: if the server created the item but the response was lost, the retry
	// is deduplicated by RP instead of creating a duplicate. It is intentionally
	// random per launch; test matching across launches uses Identity instead.
	clientUUID := uuid.New()

	rq := &openapi.StartTestItemRQ{
		Uuid:       clientUUID.String(),
		Name:       boundedName(item.Name),
		StartTime:  orNow(item.StartTime),
		LaunchUuid: tl.launchUuid,
		HasStats:   boolPtr(!item.NoStats),
		Type:       string(item.Type),
		Parameters: toParameterResources(item.Parameters),
	}
	if item.RetryOf != "" {
		rq.Retry = boolPtr(true)
		rq.RetryOf = strPtr(item.RetryOf)
	}
	if item.Identity.TestCaseID != "" {
		rq.TestCaseId = strPtr(boundedID(item.Identity.TestCaseID))
		rq.UniqueId = strPtr(item.Identity.uniqueID())
	}
	if item.Identity.CodeRef != "" {
		rq.CodeRef = strPtr(item.Identity.CodeRef)
	}

	if item.Priority != nil {
		// RP UI reads priority from attributes.
		rq.Attributes = append(rq.Attributes, openapi.ItemAttributesRQ{
			Key:   strPtr("priority"),
			Value: fmt.Sprintf("P%d", *item.Priority),
		})
	}

	if item.RefID != "" {
		// RP UI reads testReferenceId from attributes.
		rq.Attributes = append(rq.Attributes, openapi.ItemAttributesRQ{
			Key:   strPtr("testReferenceId"),
			Value: item.RefID,
		})
	}

	for _, l := range item.Labels {
		rq.Attributes = append(rq.Attributes, openapi.ItemAttributesRQ{Value: l})
	}

	var rs *openapi.EntryCreatedAsyncRS
	err := retry(3, 200*time.Millisecond, func() error {
		var e error
		if item.ParentID != "" {
			rs, e = tl.client.StartChildTest(context.Background(), item.ParentID, rq)
		} else {
			rs, e = tl.client.StartTest(context.Background(), rq)
		}
		return e
	})
	if err != nil {
		log.Printf("WARNING: failed to start test %q after retries: %v", item.Name, err)
		return ""
	}
	if rs == nil || rs.Id == nil {
		log.Printf("WARNING: start test %q returned no item ID", item.Name)
		return ""
	}
	return *rs.Id
}

// notIssueType is the issue type that tells ReportPortal a skipped or failed item is not a
// defect, so it is not filed under To Investigate.
const notIssueType = "NOT_ISSUE"

// FinishTest finishes a test item. end is when the item actually finished; zero means now.
func (tl *TestListener) FinishTest(name string, status gorp.Status, id string, end time.Time) {
	tl.finishItem(name, status, id, end, false)
}

// FinishTestNotIssue is FinishTest that also marks the item NOT_ISSUE. Without that,
// ReportPortal files every skipped item under To Investigate.
func (tl *TestListener) FinishTestNotIssue(
	name string,
	status gorp.Status,
	id string,
	end time.Time,
) {
	tl.finishItem(name, status, id, end, true)
}

func (tl *TestListener) finishItem(
	name string,
	status gorp.Status,
	id string,
	end time.Time,
	notIssue bool,
) {
	if id == "" {
		log.Printf("no test ID for %q, skipping finish", name)
		return
	}
	end = orNow(end)
	rq := &openapi.FinishTestItemRQ{
		EndTime:    end,
		Status:     statusPtr(status),
		LaunchUuid: tl.launchUuid,
	}
	if notIssue {
		rq.Issue = &openapi.Issue{IssueType: notIssueType}
	}
	err := retry(3, 200*time.Millisecond, func() error {
		_, e := tl.client.FinishTest(context.Background(), id, rq)
		return e
	})
	if err != nil {
		log.Printf("WARNING: failed to finish test %q after retries: %v", name, err)
	}
}

// SendLog sends a log entry to the given test item. at is when the entry was
// really emitted; zero means now.
func (tl *TestListener) SendLog(itemID, level, message string, at time.Time) error {
	at = orNow(at)
	return retry(3, 200*time.Millisecond, func() error {
		_, e := tl.client.SaveLog(context.Background(), &openapi.SaveLogRQ{
			ItemUuid:   strPtr(itemID),
			LaunchUuid: tl.launchUuid,
			Level:      strPtr(level),
			Time:       at,
			Message:    strPtr(message),
		})
		return e
	})
}

// maxBatchLogs and maxBatchBytes bound one batch request.
const (
	maxBatchLogs  = 200
	maxBatchBytes = 2 << 20
)

// QueueLog adds a log to the pending batch. Nothing is sent until FlushLogs.
func (tl *TestListener) QueueLog(itemID, level, message string, at time.Time) {
	tl.mu.Lock()
	defer tl.mu.Unlock()
	tl.pending = append(tl.pending, &openapi.SaveLogRQ{
		// A fixed UUID per log lets ReportPortal drop a duplicate if a retried batch
		// was in fact already stored.
		Uuid:       strPtr(uuid.NewString()),
		ItemUuid:   strPtr(itemID),
		LaunchUuid: tl.launchUuid,
		Level:      strPtr(level),
		Time:       orNow(at),
		Message:    strPtr(message),
	})
}

// FlushLogs sends the queued logs, in as few batch requests as the size limits allow.
// Logs that could not be sent are dropped after the retries, and the error returned.
func (tl *TestListener) FlushLogs() error {
	tl.mu.Lock()
	logs := tl.pending
	tl.pending = nil
	tl.mu.Unlock()

	var firstErr error
	for len(logs) > 0 {
		n, size := 0, 0
		for n < len(logs) && n < maxBatchLogs {
			size += len(*logs[n].Message)
			if n > 0 && size > maxBatchBytes {
				break
			}
			n++
		}
		batch := logs[:n]
		logs = logs[n:]
		err := retry(3, 200*time.Millisecond, func() error {
			_, e := tl.client.SaveLogs(context.Background(), batch...)
			return e
		})
		if err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// SendAttachmentLog streams a file to ReportPortal as a multipart log attachment.
func (tl *TestListener) SendAttachmentLog(
	itemID, level, message, filePath, originalFilename string,
	at time.Time,
) error {
	at = orNow(at)
	file, err := os.Open(filePath)
	if err != nil {
		errMsg := fmt.Sprintf(
			"failed to open attachment %s: %v — original message: %s",
			originalFilename,
			err,
			message,
		)
		log.Printf("ERROR: %s", errMsg)
		return tl.SendLog(itemID, gorp.LogLevelError, errMsg, at)
	}
	defer func() { _ = file.Close() }()

	fileInfo, err := file.Stat()
	if err != nil {
		return tl.SendLog(
			itemID,
			gorp.LogLevelError,
			fmt.Sprintf("failed to stat attachment %s: %v", originalFilename, err),
			at,
		)
	}

	// Detect content type from first 512 bytes without loading the whole file.
	buf := make([]byte, 512)
	n, _ := file.Read(buf)
	contentType := http.DetectContentType(buf[:n])
	if _, err := file.Seek(0, 0); err != nil {
		return tl.SendLog(
			itemID,
			gorp.LogLevelError,
			fmt.Sprintf("failed to seek attachment %s: %v", originalFilename, err),
			at,
		)
	}

	LogVerboseOperation(
		fmt.Sprintf(
			"sending attachment: %s (%s, %d bytes)",
			originalFilename,
			contentType,
			fileInfo.Size(),
		),
	)

	logRQ := &openapi.SaveLogRQ{
		ItemUuid:   strPtr(itemID),
		LaunchUuid: tl.launchUuid,
		Level:      strPtr(level),
		Time:       at,
		Message:    strPtr(message),
		File:       &openapi.File{Name: strPtr(originalFilename)},
	}
	multipart := &gorp.ReaderMultipart{
		FileName:    originalFilename,
		ContentType: contentType,
		Reader:      file,
	}

	err = retry(3, 200*time.Millisecond, func() error {
		if _, se := file.Seek(0, 0); se != nil {
			return se
		}
		_, e := tl.client.SaveLogMultipart(
			context.Background(),
			[]*openapi.SaveLogRQ{logRQ},
			[]gorp.Multipart{multipart},
		)
		return e
	})
	if err != nil {
		log.Printf("ERROR: multipart upload failed after retries: %v", err)
		return tl.SendLog(
			itemID,
			level,
			fmt.Sprintf("%s\n[attachment %s could not be uploaded]", message, originalFilename),
			at,
		)
	}

	_ = os.Remove(filePath)
	return nil
}

// SendLaunchLog sends a log entry at the launch level (not tied to a test item).
func (tl *TestListener) SendLaunchLog(level, message string) error {
	_, err := tl.client.SaveLog(context.Background(), &openapi.SaveLogRQ{
		LaunchUuid: tl.launchUuid,
		Level:      strPtr(level),
		Time:       time.Now(),
		Message:    strPtr(message),
	})
	return err
}

// MapGinkgoStateToStatus converts a Ginkgo spec state string to a ReportPortal Status.
// All five Ginkgo failure states map to Failed or Interrupted:
//
//	failed, panicked → Failed
//	interrupted, aborted, timedout → Interrupted (test was running, not a logic failure)
func MapGinkgoStateToStatus(state string) gorp.Status {
	switch strings.ToLower(state) {
	case "passed":
		return gorp.Statuses.Passed
	case "failed", "panicked":
		return gorp.Statuses.Failed
	case "interrupted", "aborted", "timedout":
		return gorp.Statuses.Interrupted
	case "skipped", "pending":
		return gorp.Statuses.Skipped
	default:
		return gorp.Statuses.Skipped
	}
}
