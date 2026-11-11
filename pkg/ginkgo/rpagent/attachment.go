package rpagent

import (
	"encoding/json"
	"fmt"
	"io"
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
	dir, err := os.MkdirTemp("", "rp-agent-attachments-*")
	if err != nil {
		log.Fatalf("CRITICAL: failed to create temp directory for attachments: %v", err)
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
		errPayload, _ := json.Marshal(
			struct{ Msg string }{Msg: fmt.Sprintf("%s#failed to access file: %v", message, err)},
		)
		ginkgo.AddReportEntry(filePath, fmt.Sprintf("%s%s", attachmentErrorPrefix, errPayload))
		return
	}

	if fileInfo.Size() > attachmentConfig.MaxFileSize {
		log.Printf(
			"WARNING: attachment %s (%d bytes) exceeds %d byte limit",
			filePath,
			fileInfo.Size(),
			attachmentConfig.MaxFileSize,
		)
		errPayload, _ := json.Marshal(struct{ Msg string }{
			Msg: fmt.Sprintf(
				"%s#file size (%d bytes) exceeds limit (%d bytes)",
				message,
				fileInfo.Size(),
				attachmentConfig.MaxFileSize,
			),
		})
		ginkgo.AddReportEntry(filePath, fmt.Sprintf("%s%s", attachmentErrorPrefix, errPayload))
		return
	}

	tempFile, err := copyToTempLocation(filePath)
	if err != nil {
		log.Printf("WARNING: failed to copy attachment %s to temp location: %v", filePath, err)
		errPayload, _ := json.Marshal(
			struct{ Msg string }{Msg: fmt.Sprintf("%s#failed to copy file: %v", message, err)},
		)
		ginkgo.AddReportEntry(filePath, fmt.Sprintf("%s%s", attachmentErrorPrefix, errPayload))
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
	ginkgo.AddReportEntry(filePath, fmt.Sprintf("%s%s", attachmentPrefix, payload))
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
