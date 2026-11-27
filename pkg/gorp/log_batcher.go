package gorp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/reportportal/goRP/v5/pkg/openapi"
)

// Defaults of LogBatcherOptions.
const (
	// DefaultLogBatchEntries is the number of logs sent in one request.
	DefaultLogBatchEntries = 10
	// DefaultLogBatchBytes limits the size of one request, far below the 128 MB a server accepts.
	DefaultLogBatchBytes = 16 << 20
	// DefaultLogBatchAttachments is the number of attachments in one request. A multipart request to
	// ReportPortal may have 50 parts (the default of the server's Tomcat, server.tomcat.max-part-count);
	// one of them is the JSON part with the logs. A request with 50 attachments is answered with a 500.
	DefaultLogBatchAttachments = 49
	// DefaultLogFlushInterval is how long a log waits for its batch to fill up before it is sent anyway.
	DefaultLogFlushInterval = 5 * time.Second
	// DefaultLogBatchesInFlight is the number of batch requests sent at the same time.
	DefaultLogBatchesInFlight = 4
)

// ErrLogBatcherClosed is returned by LogBatcher.Add after Close.
var ErrLogBatcherClosed = errors.New("log batcher is closed")

// LogBatchEntry is the server's answer for one log of a batch, in the order the logs were sent.
//
// A batch request is answered with 201 even when the server did not accept some of its logs: such an
// entry has no ID, and Message and StackTrace say why.
type LogBatchEntry struct {
	// ID is the UUID of the created log; empty when the log was not accepted
	ID string `json:"id,omitempty"`
	// Message and StackTrace describe the failure. ReportPortal up to at least 6.0 fills them the
	// other way round: Message has the stack trace and StackTrace the one-line message. Use Reason.
	Message    string `json:"message,omitempty"`
	StackTrace string `json:"stackTrace,omitempty"`
}

// Failed reports whether the server did not accept the log.
func (e LogBatchEntry) Failed() bool {
	return e.ID == ""
}

// Reason is the one-line description of the failure; empty when the log was accepted.
func (e LogBatchEntry) Reason() string {
	if !e.Failed() {
		return ""
	}
	// whichever field holds the short message: see LogBatchEntry.Message
	reason := e.StackTrace
	if reason == "" || (e.Message != "" && len(e.Message) < len(reason)) {
		reason = e.Message
	}
	reason, _, _ = strings.Cut(strings.TrimSpace(reason), "\n")
	if reason == "" {
		return "no log ID in the response"
	}
	return reason
}

// LogBatchError reports the logs of a batch which the server answered, but did not accept.
type LogBatchError struct {
	// Failed are the answers for the logs that were not accepted
	Failed []LogBatchEntry
	// Total is the number of logs in the batch
	Total int
}

func (e *LogBatchError) Error() string {
	reason := ""
	if len(e.Failed) > 0 {
		reason = ": " + e.Failed[0].Reason()
	}
	return fmt.Sprintf("%d of %d logs were not saved%s", len(e.Failed), e.Total, reason)
}

// LogBatcherOptions configure a LogBatcher. The zero value of a field selects its default.
type LogBatcherOptions struct {
	// MaxEntries is the number of logs that makes a batch complete (DefaultLogBatchEntries)
	MaxEntries int
	// MaxBytes is the size of the logs and their attachments that makes a batch complete
	// (DefaultLogBatchBytes). The size of an attachment is known only for a FileMultipart.
	MaxBytes int
	// MaxAttachments is the number of attachments one batch may have (DefaultLogBatchAttachments).
	// Raise it only if the server's limit of multipart parts was raised.
	MaxAttachments int
	// FlushInterval is how often an incomplete batch is sent (DefaultLogFlushInterval), so that the
	// logs of a long-running test show up while it runs. A negative value disables the timer.
	FlushInterval time.Duration
	// MaxInFlight is the number of requests sent at the same time (DefaultLogBatchesInFlight).
	// Add blocks while that many are under way.
	MaxInFlight int
}

func (o LogBatcherOptions) withDefaults() LogBatcherOptions {
	if o.MaxEntries <= 0 {
		o.MaxEntries = DefaultLogBatchEntries
	}
	if o.MaxBytes <= 0 {
		o.MaxBytes = DefaultLogBatchBytes
	}
	if o.MaxAttachments <= 0 {
		o.MaxAttachments = DefaultLogBatchAttachments
	}
	if o.FlushInterval == 0 {
		o.FlushInterval = DefaultLogFlushInterval
	}
	if o.MaxInFlight <= 0 {
		o.MaxInFlight = DefaultLogBatchesInFlight
	}
	return o
}

// LogBatcher collects logs and sends them to ReportPortal in batches. A batch is sent when it has
// MaxEntries logs, when the next log would take it over MaxBytes or MaxAttachments, when FlushInterval
// has passed, and on Flush and Close.
//
// Requests are sent in the background, up to MaxInFlight at a time; their errors, including logs the
// server answered but did not accept (LogBatchError), are returned by Close. A failed batch is not
// sent again: ReportPortal has no key to recognize a log it has seen already, so a repeated batch
// could store its logs twice.
//
// Add and Flush may be called from several goroutines. Close must not run concurrently with Add.
type LogBatcher struct {
	ctx    context.Context //nolint:containedctx // the context of the requests sent in the background
	client *ReportingClient
	opts   LogBatcherOptions

	mu          sync.Mutex
	logs        []*openapi.SaveLogRQ
	files       []Multipart
	bytes       int
	closed      bool
	requests    errgroup.Group
	stop        chan struct{}
	stopped     chan struct{}
	closeOnce   sync.Once
	closeResult error
}

// NewLogBatcher creates a LogBatcher that sends its batches with client. The requests use ctx, so
// canceling it fails the batches that are not sent yet. Close must be called to send the last batch
// and to stop the flush timer.
func NewLogBatcher(
	ctx context.Context,
	client *ReportingClient,
	opts LogBatcherOptions,
) *LogBatcher {
	b := &LogBatcher{
		ctx:     ctx,
		client:  client,
		opts:    opts.withDefaults(),
		stop:    make(chan struct{}),
		stopped: make(chan struct{}),
	}
	b.requests.SetLimit(b.opts.MaxInFlight)
	if b.opts.FlushInterval > 0 {
		go b.flushPeriodically()
	} else {
		close(b.stopped)
	}
	return b
}

// flushPeriodically sends incomplete batches every FlushInterval until Close.
func (b *LogBatcher) flushPeriodically() {
	defer close(b.stopped)
	ticker := time.NewTicker(b.opts.FlushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-b.stop:
			return
		case <-ticker.C:
			b.Flush()
		}
	}
}

// Add puts a log and its attachments into the current batch and sends the batch if it is complete.
// The name of an attachment must be the File.Name of the log, see ReportingClient.SaveLogBatch.
// The batcher owns the log from here on: it must not be changed after Add.
//
// Add blocks while MaxInFlight requests are under way. It returns ErrLogBatcherClosed after Close;
// errors of the requests are returned by Close.
func (b *LogBatcher) Add(log *openapi.SaveLogRQ, files ...Multipart) error {
	size := logSize(log, files)

	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return ErrLogBatcherClosed
	}
	// the log does not fit into the current batch: send that first
	var before, after *logBatch
	if len(b.logs) > 0 &&
		(b.bytes+size > b.opts.MaxBytes || len(b.files)+len(files) > b.opts.MaxAttachments) {
		before = b.take()
	}
	b.logs = append(b.logs, log)
	b.files = append(b.files, files...)
	b.bytes += size
	if len(b.logs) >= b.opts.MaxEntries {
		after = b.take()
	}
	b.mu.Unlock()

	// outside of the lock: sending blocks while MaxInFlight requests are under way
	b.send(before)
	b.send(after)
	return nil
}

// Flush sends the current batch, however small it is. It does not wait for the request.
func (b *LogBatcher) Flush() {
	b.mu.Lock()
	batch := b.take()
	b.mu.Unlock()

	b.send(batch)
}

// Close sends the last batch, waits for all requests and returns their errors: the first failed
// request, or a LogBatchError for logs the server did not accept. Further calls return the same.
func (b *LogBatcher) Close() error {
	b.closeOnce.Do(func() {
		b.mu.Lock()
		b.closed = true
		b.mu.Unlock()

		// the timer must not start a request while the requests are waited for
		close(b.stop)
		<-b.stopped

		b.Flush()
		b.closeResult = b.requests.Wait()
	})
	return b.closeResult
}

// logBatch is the content of one request.
type logBatch struct {
	logs  []*openapi.SaveLogRQ
	files []Multipart
}

// take empties the current batch and returns it, or nil if there is nothing to send. The caller
// holds b.mu.
func (b *LogBatcher) take() *logBatch {
	if len(b.logs) == 0 {
		return nil
	}
	batch := &logBatch{logs: b.logs, files: b.files}
	b.logs, b.files, b.bytes = nil, nil, 0
	return batch
}

// send starts the request of a batch; it blocks while MaxInFlight requests are under way.
func (b *LogBatcher) send(batch *logBatch) {
	if batch == nil {
		return
	}
	b.requests.Go(func() error {
		entries, err := b.client.SaveLogBatch(b.ctx, batch.logs, batch.files)
		if err != nil {
			return fmt.Errorf("sending a batch of %d logs: %w", len(batch.logs), err)
		}
		return checkLogBatch(entries, len(batch.logs))
	})
}

// checkLogBatch turns the answer for a batch into an error if a log was not accepted. Only entries
// that are there and have no ID count: a response with fewer entries than the batch is not judged.
func checkLogBatch(entries []LogBatchEntry, total int) error {
	var failed []LogBatchEntry
	for _, entry := range entries {
		if entry.Failed() {
			failed = append(failed, entry)
		}
	}
	if len(failed) == 0 {
		return nil
	}
	return &LogBatchError{Failed: failed, Total: total}
}

// logSize estimates what a log adds to the size of a request: its JSON and its attachments, as far
// as their size is known without reading them.
func logSize(log *openapi.SaveLogRQ, files []Multipart) int {
	size := 0
	if encoded, err := json.Marshal(log); err == nil {
		size = len(encoded)
	}
	for _, file := range files {
		if fm, ok := file.(*FileMultipart); ok && fm.File != nil {
			if info, err := os.Stat(fm.Name()); err == nil {
				size += int(info.Size())
			}
		}
	}
	return size
}
