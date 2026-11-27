//go:build e2e

package e2e

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/reportportal/goRP/v5/pkg/gorp"
	"github.com/reportportal/goRP/v5/pkg/openapi"
)

func (e *env) reportingClient(t *testing.T) *gorp.ReportingClient {
	t.Helper()

	return gorp.NewReportingClient(strings.TrimRight(e.cfg.Host, "/"), e.cfg.Project,
		gorp.WithApiKeyAuth(t.Context(), e.cfg.ApiKey))
}

// One multipart request can carry 49 attachments: the server's limit is 50 parts, and one part is the
// JSON with the logs. The batcher must split a larger set, or the server answers the request with 500.
func TestLogBatcher_SplitsAttachmentsAndReportsRejectedLogs(t *testing.T) {
	e := newEnv(t)
	name := uniqueName(t)
	client := e.reportingClient(t)
	start := time.Now()

	launch, err := client.StartLaunch(
		t.Context(),
		&openapi.StartLaunchRQ{Name: name, StartTime: start},
	)
	require.NoError(t, err)
	launchUUID := launch.GetId()
	running := e.findLaunch(t, name, nil)
	item, err := client.StartTest(t.Context(), &openapi.StartTestItemRQ{
		Name:       "attachments",
		StartTime:  start,
		LaunchUuid: launchUUID,
		Type:       string(gorp.TestItemTypes.Step),
	})
	require.NoError(t, err)

	newLog := func(i int, file string) *openapi.SaveLogRQ {
		log := &openapi.SaveLogRQ{
			LaunchUuid: launchUUID,
			ItemUuid:   item.Id,
			Level:      openapi.PtrString(gorp.LogLevelInfo),
			Message:    openapi.PtrString(fmt.Sprintf("log %d", i)),
			Time:       start.Add(time.Duration(i) * time.Millisecond),
		}
		if file != "" {
			log.File = &openapi.File{Name: openapi.PtrString(file)}
		}

		return log
	}

	// more logs than fit into one batch by count would hide the attachment limit: allow them all
	const attachments = 60
	batcher := gorp.NewLogBatcher(t.Context(), client, gorp.LogBatcherOptions{MaxEntries: 1000})
	for i := range attachments {
		file := fmt.Sprintf("file-%d.txt", i)
		require.NoError(t, batcher.Add(newLog(i, file), &gorp.ReaderMultipart{
			FileName:    file,
			ContentType: "text/plain",
			Reader:      strings.NewReader("content of " + file),
		}))
	}
	require.NoError(t, batcher.Close(), "60 attachments must go out as two requests of at most 49")

	logs := e.logs(t, running.Id, attachments)
	withAttachment := 0
	for _, l := range logs {
		if l.hasAttachment() {
			withAttachment++
		}
	}
	assert.Equal(t, attachments, withAttachment)

	// 50 attachments in one request are refused as a whole: the limit is real on this server
	_, err = client.SaveLogBatch(t.Context(), nil, fiftyFiles())
	var httpErr *gorp.HTTPError
	if errors.As(err, &httpErr) {
		assert.Equal(t, 500, httpErr.StatusCode)
	} else {
		// not a failure of gorp: this deployment allows more parts than the default
		t.Logf("the server accepted 50 attachments in one request (err: %v)", err)
	}

	// a log the server rejects comes back inside a 201, as an entry without ID
	entries, err := client.SaveLogBatch(t.Context(), []*openapi.SaveLogRQ{
		newLog(100, ""), newLog(101, "not-sent.txt"),
	}, nil)
	require.NoError(t, err)
	require.Len(t, entries, 2)
	assert.False(t, entries[0].Failed())
	assert.True(t, entries[1].Failed())
	assert.Contains(
		t,
		entries[1].Reason(),
		"There is no request part or file with name not-sent.txt",
	)

	rejecting := gorp.NewLogBatcher(t.Context(), client, gorp.LogBatcherOptions{})
	require.NoError(t, rejecting.Add(newLog(102, "not-sent-either.txt")))
	var batchErr *gorp.LogBatchError
	require.ErrorAs(t, rejecting.Close(), &batchErr)
	assert.Len(t, batchErr.Failed, 1)

	_, err = client.FinishTest(t.Context(), item.GetId(), &openapi.FinishTestItemRQ{
		EndTime: time.Now(), Status: gorp.Statuses.Passed.Ptr(), LaunchUuid: launchUUID,
	})
	require.NoError(t, err)
	_, err = client.FinishLaunch(
		t.Context(),
		launchUUID,
		&openapi.FinishExecutionRQ{EndTime: time.Now()},
	)
	require.NoError(t, err)
}

func fiftyFiles() []gorp.Multipart {
	files := make([]gorp.Multipart, 0, 50)
	for i := range 50 {
		files = append(files, &gorp.ReaderMultipart{
			FileName: fmt.Sprintf(
				"f%d.txt",
				i,
			),
			ContentType: "text/plain",
			Reader:      strings.NewReader("x"),
		})
	}

	return files
}
