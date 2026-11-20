package rpagent

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/reportportal/goRP/v5/pkg/gorp"
	"github.com/reportportal/goRP/v5/pkg/openapi"
)

// isRetriableError reports whether an error warrants a retry.
// Retries on: network errors, HTTP 5xx, HTTP 429.
// Does NOT retry on: 4xx client errors (except 429).
func isRetriableError(err error) bool {
	if err == nil {
		return false
	}
	var httpErr *gorp.HTTPError
	if errors.As(err, &httpErr) {
		return httpErr.StatusCode >= 500 || httpErr.StatusCode == 429
	}
	s := err.Error()
	return strings.Contains(s, "connection refused") ||
		strings.Contains(s, "connection reset") ||
		strings.Contains(s, "timeout") ||
		strings.Contains(s, "EOF") ||
		strings.Contains(s, "no such host") ||
		strings.Contains(s, "network is unreachable")
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

// StartTest starts a test item (no retry-ID variant).
func (tl *TestListener) StartTest(
	name, parentUUID string,
	priority *int,
	parameters []*Parameter,
	itemType gorp.TestItemType,
) string {
	return tl.startItem(name, parentUUID, priority, parameters, itemType, "")
}

// StartTestWithReferenceId starts a test item with an external reference ID.
func (tl *TestListener) StartTestWithReferenceId(
	name, parentUUID string,
	priority *int,
	parameters []*Parameter,
	itemType gorp.TestItemType,
	refID string,
) string {
	return tl.startItem(name, parentUUID, priority, parameters, itemType, refID)
}

// StartTestWithStats starts a test item with an explicit hasStats flag.
// Pass false for By() child steps so RP does not count them in launch totals.
func (tl *TestListener) StartTestWithStats(
	name, parentUUID string,
	priority *int,
	parameters []*Parameter,
	itemType gorp.TestItemType,
	hasStats bool,
) string {
	return tl.startItemWithStats(name, parentUUID, priority, parameters, itemType, "", hasStats)
}

func (tl *TestListener) startItem(
	name, parentUUID string,
	priority *int,
	parameters []*Parameter,
	itemType gorp.TestItemType,
	refID string,
) string {
	return tl.startItemWithStats(name, parentUUID, priority, parameters, itemType, refID, true)
}

func (tl *TestListener) startItemWithStats(
	name, parentUUID string,
	priority *int,
	parameters []*Parameter,
	itemType gorp.TestItemType,
	refID string,
	hasStats bool,
) string {
	// Generate a stable client UUID for this start request so that retries are
	// idempotent: if the server created the item but the response was lost, a
	// retry with the same UUID is deduplicated by RP instead of creating a
	// duplicate item.
	clientUUID := uuid.New()

	// Build a stable identity key from the full item path (parentUUID + name)
	// so that distinct items with the same leaf name don't share a testCaseId.
	testCaseKey := name
	if parentUUID != "" {
		testCaseKey = parentUUID + "/" + name
	}

	rq := &openapi.StartTestItemRQ{
		Uuid:       clientUUID.String(),
		Name:       name,
		StartTime:  time.Now(),
		LaunchUuid: tl.launchUuid,
		HasStats:   boolPtr(hasStats),
		UniqueId:   strPtr(clientUUID.String()),
		CodeRef:    strPtr(testCaseKey),
		TestCaseId: strPtr(testCaseKey),
		Type:       string(itemType),
		Parameters: toParameterResources(parameters),
	}

	if priority != nil {
		// RP UI reads priority from attributes.
		rq.Attributes = append(rq.Attributes, openapi.ItemAttributesRQ{
			Key:   strPtr("priority"),
			Value: fmt.Sprintf("P%d", *priority),
		})
	}

	if refID != "" {
		// RP UI reads testReferenceId from attributes.
		rq.Attributes = append(rq.Attributes, openapi.ItemAttributesRQ{
			Key:   strPtr("testReferenceId"),
			Value: refID,
		})
	}

	if itemType == gorp.TestItemTypes.Suite {
		rq.CodeRef = nil
		if refID == "" {
			rq.TestCaseId = nil
		}
	}

	var rs *openapi.EntryCreatedAsyncRS
	err := retry(3, 200*time.Millisecond, func() error {
		var e error
		if parentUUID != "" {
			rs, e = tl.client.StartChildTest(context.Background(), parentUUID, rq)
		} else {
			rs, e = tl.client.StartTest(context.Background(), rq)
		}
		return e
	})
	if err != nil {
		log.Printf("WARNING: failed to start test %q after retries: %v", name, err)
		return ""
	}
	if rs == nil || rs.Id == nil {
		log.Printf("WARNING: start test %q returned no item ID", name)
		return ""
	}
	return *rs.Id
}

// FinishTestWithPriority finishes a test item.
func (tl *TestListener) FinishTestWithPriority(name string, status gorp.Status, id string, _ *int) {
	if id == "" {
		log.Printf("no test ID for %q, skipping finish", name)
		return
	}
	err := retry(3, 200*time.Millisecond, func() error {
		_, e := tl.client.FinishTest(context.Background(), id, &openapi.FinishTestItemRQ{
			EndTime:    time.Now(),
			Status:     statusPtr(status),
			LaunchUuid: tl.launchUuid,
		})
		return e
	})
	if err != nil {
		log.Printf("WARNING: failed to finish test %q after retries: %v", name, err)
	}
}

// SendLog sends a log entry to the given test item.
func (tl *TestListener) SendLog(itemID, level, message string) error {
	return retry(3, 200*time.Millisecond, func() error {
		_, e := tl.client.SaveLog(context.Background(), &openapi.SaveLogRQ{
			ItemUuid:   strPtr(itemID),
			LaunchUuid: tl.launchUuid,
			Level:      strPtr(level),
			Time:       time.Now(),
			Message:    strPtr(message),
		})
		return e
	})
}

// SendAttachmentLog streams a file to ReportPortal as a multipart log attachment.
func (tl *TestListener) SendAttachmentLog(
	itemID, level, message, filePath, originalFilename string,
) error {
	file, err := os.Open(filePath)
	if err != nil {
		errMsg := fmt.Sprintf(
			"failed to open attachment %s: %v — original message: %s",
			originalFilename,
			err,
			message,
		)
		log.Printf("ERROR: %s", errMsg)
		return tl.SendLog(itemID, gorp.LogLevelError, errMsg)
	}
	defer func() { _ = file.Close() }()

	fileInfo, err := file.Stat()
	if err != nil {
		return tl.SendLog(
			itemID,
			gorp.LogLevelError,
			fmt.Sprintf("failed to stat attachment %s: %v", originalFilename, err),
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
		Time:       time.Now(),
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
