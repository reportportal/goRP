package rpagent

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"

	"github.com/onsi/ginkgo/v2"
)

const (
	attachmentPrefix         = "RP_ATTACHMENT#"
	attachmentErrorPrefix    = "RP_ATTACHMENT_ERROR#"
	defaultMaxAttachmentSize = 10 * 1024 * 1024 // 10 MB
)

// attachmentPayload is the JSON-encoded value stored in a report entry for an attachment.
// Using JSON avoids the #-delimiter format that breaks when file paths contain #.
type attachmentPayload struct {
	TempPath         string `json:"tmpPath"`
	OriginalFilename string `json:"origName"`
	Message          string `json:"msg"`
}

// AttachmentConfig holds configuration for attachment handling.
type AttachmentConfig struct {
	MaxFileSize int64  // Maximum file size in bytes (default: 10 MB)
	TempDir     string // Override for the temp directory (defaults to os.TempDir())
}

var (
	attachmentConfig  = AttachmentConfig{MaxFileSize: defaultMaxAttachmentSize}
	tempAttachmentDir string
)

// InitializeAttachments creates a temporary directory for agent-managed attachments.
// Called once when the agent starts; safe to call multiple times (idempotent).
func InitializeAttachments() {
	if tempAttachmentDir != "" {
		return
	}
	// The session ID is part of the name so the wrapper can remove this run's directories,
	// and only this run's, after every worker has finished. Workers cannot all do it
	// themselves: ReportAfterSuite runs on one process only.
	prefix := "rp-agent-attachments-"
	if sessionID := os.Getenv("RP_SESSION_ID"); sessionID != "" {
		prefix += sessionID + "-"
	}
	dir, err := os.MkdirTemp("", prefix+"*")
	if err != nil {
		// Reporting must never kill the test process. Without a directory, AddAttachment
		// reports an attachment error entry instead of uploading the file.
		log.Printf("WARNING: could not create the attachment temp directory: %v", err)
		return
	}
	tempAttachmentDir = dir
	LogVerboseOperation(fmt.Sprintf("attachment temp dir: %s", tempAttachmentDir))
}

// CleanupAttachments removes the temporary attachment directory.
// Call once after all tests finish.
func CleanupAttachments() {
	if tempAttachmentDir != "" {
		_ = os.RemoveAll(tempAttachmentDir)
		tempAttachmentDir = ""
	}
}

// SetAttachmentConfig overrides the default attachment configuration.
func SetAttachmentConfig(cfg AttachmentConfig) {
	if cfg.MaxFileSize > 0 {
		attachmentConfig.MaxFileSize = cfg.MaxFileSize
	}
	if cfg.TempDir != "" {
		attachmentConfig.TempDir = cfg.TempDir
	}
}

// attachmentError is the JSON payload of an attachmentErrorPrefix report entry: an
// attachment the agent could not upload, and why.
type attachmentError struct {
	File    string `json:"file"`
	Message string `json:"msg,omitempty"`
	Reason  string `json:"reason"`
}

// reportAttachmentError leaves an entry for the agent, which logs it on the spec item.
// Never: the entry only carries the problem to the agent, so keep it out of Ginkgo's output.
func reportAttachmentError(filePath, message, reason string) {
	payload, _ := json.Marshal(attachmentError{
		File:    filepath.Base(filePath),
		Message: message,
		Reason:  reason,
	})
	ginkgo.AddReportEntry(
		filePath,
		attachmentErrorPrefix+string(payload),
		ginkgo.ReportEntryVisibilityNever,
	)
}

// formatAttachmentError turns the payload of an attachmentErrorPrefix entry into the log
// text. A payload it cannot read is logged as it is, so the problem is never lost.
func formatAttachmentError(raw string) string {
	var e attachmentError
	if err := json.Unmarshal([]byte(raw), &e); err != nil || e.Reason == "" {
		return "Attachment error: " + raw
	}
	text := "Attachment not uploaded: " + e.File
	if e.Message != "" {
		text += ` ("` + e.Message + `")`
	}
	return text + ": " + e.Reason
}

// bareError is err without the path an *fs.PathError repeats: the file is named already.
func bareError(err error) string {
	var pe *fs.PathError
	if errors.As(err, &pe) {
		return pe.Err.Error()
	}
	return err.Error()
}

// AddAttachment queues a file to be uploaded as an attachment to the current test.
// Must be called inside an It block.
//
// Usage:
//
//	rpagent.AddAttachment("/path/to/screenshot.png", "Login page screenshot")
func AddAttachment(filePath, message string) {
	if filePath == "" {
		log.Printf("WARNING: empty file path provided for attachment, skipping")
		return
	}

	fileInfo, err := os.Stat(filePath)
	if err != nil {
		log.Printf("WARNING: cannot access attachment file %s: %v", filePath, err)
		reportAttachmentError(filePath, message, "cannot access the file: "+bareError(err))
		return
	}

	if fileInfo.Size() > attachmentConfig.MaxFileSize {
		log.Printf(
			"WARNING: attachment %s (%d bytes) exceeds %d byte limit",
			filePath,
			fileInfo.Size(),
			attachmentConfig.MaxFileSize,
		)
		reportAttachmentError(filePath, message, fmt.Sprintf(
			"the file is %d bytes, the limit is %d bytes",
			fileInfo.Size(),
			attachmentConfig.MaxFileSize,
		))
		return
	}

	tempFile, err := copyToTempLocation(filePath)
	if err != nil {
		log.Printf("WARNING: failed to copy attachment %s to temp location: %v", filePath, err)
		reportAttachmentError(filePath, message, "cannot copy the file: "+bareError(err))
		return
	}

	// Encode the IPC payload as JSON so paths and messages containing # don't
	// corrupt the parsing in processAttachmentsWithByRouting.
	payload, err := json.Marshal(attachmentPayload{
		TempPath:         tempFile,
		OriginalFilename: filepath.Base(filePath),
		Message:          message,
	})
	if err != nil {
		log.Printf("WARNING: failed to encode attachment payload: %v", err)
		return
	}
	// Never: the entry only carries the file to the agent, so keep it out of Ginkgo's output.
	ginkgo.AddReportEntry(
		filePath,
		fmt.Sprintf("%s%s", attachmentPrefix, payload),
		ginkgo.ReportEntryVisibilityNever,
	)
	LogVerboseOperation(
		fmt.Sprintf("queued attachment: %s (%d bytes)", filepath.Base(filePath), fileInfo.Size()),
	)
}

// ParseAttachmentPayload decodes the JSON payload from a report entry value.
// Returns the temp path, original filename, and message, or an error.
func ParseAttachmentPayload(value string) (tempPath, origName, message string, err error) {
	if !hasPrefix(value, attachmentPrefix) {
		return "", "", "", fmt.Errorf("not an attachment payload")
	}
	raw := value[len(attachmentPrefix):]
	var p attachmentPayload
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
		return "", "", "", fmt.Errorf("failed to parse attachment payload: %w", err)
	}
	return p.TempPath, p.OriginalFilename, p.Message, nil
}

func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

func copyToTempLocation(originalPath string) (string, error) {
	if tempAttachmentDir == "" {
		InitializeAttachments()
	}
	tempDir := tempAttachmentDir
	if attachmentConfig.TempDir != "" {
		tempDir = attachmentConfig.TempDir
	}
	if tempDir == "" {
		return "", errors.New("the attachment temp directory is unavailable")
	}

	ext := filepath.Ext(originalPath)
	tmpFile, err := os.CreateTemp(tempDir, "rp-attachment-*"+ext)
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	defer func() { _ = tmpFile.Close() }()

	src, err := os.Open(originalPath)
	if err != nil {
		_ = os.Remove(tmpFile.Name())
		return "", fmt.Errorf("failed to open source file: %w", err)
	}
	defer func() { _ = src.Close() }()

	if _, err := io.Copy(tmpFile, src); err != nil {
		_ = os.Remove(tmpFile.Name())
		return "", fmt.Errorf("failed to copy file content: %w", err)
	}
	return tmpFile.Name(), nil
}
