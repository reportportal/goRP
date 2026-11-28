package rpagent

import (
	"encoding/json"
	"errors"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParseAttachmentPayloadRoundTripWithHashInPath(t *testing.T) {
	// A '#' in the path or message used to break the old delimiter format.
	raw, _ := json.Marshal(attachmentPayload{
		TempPath: "/tmp/a#b/c.png", OriginalFilename: "c.png", Message: "shot #1",
	})
	tmp, name, msg, err := ParseAttachmentPayload(attachmentPrefix + string(raw))
	if err != nil || tmp != "/tmp/a#b/c.png" || name != "c.png" || msg != "shot #1" {
		t.Errorf("got %q %q %q %v", tmp, name, msg, err)
	}
	if _, _, _, err := ParseAttachmentPayload("something else"); err == nil {
		t.Error("a value without the prefix must be rejected")
	}
	if _, _, _, err := ParseAttachmentPayload(attachmentPrefix + "{not json"); err == nil {
		t.Error("a corrupt payload must be rejected")
	}
}

func TestCopyToTempLocationKeepsExtensionAndContent(t *testing.T) {
	oldDir, oldCfg := tempAttachmentDir, attachmentConfig
	tempAttachmentDir, attachmentConfig = "", AttachmentConfig{
		MaxFileSize: defaultMaxAttachmentSize,
	}
	t.Cleanup(func() { CleanupAttachments(); tempAttachmentDir, attachmentConfig = oldDir, oldCfg })

	src := filepath.Join(t.TempDir(), "report.txt")
	if err := os.WriteFile(src, []byte("hello"), 0o600); err != nil {
		t.Fatal(err)
	}
	dst, err := copyToTempLocation(src) // initializes the temp dir on first use
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Ext(dst) != ".txt" {
		t.Errorf("copy %q lost the extension", dst)
	}
	if b, _ := os.ReadFile(dst); string(b) != "hello" {
		t.Errorf("copy content = %q", b)
	}

	if _, err := copyToTempLocation(filepath.Join(t.TempDir(), "missing.txt")); err == nil {
		t.Error("a missing source must fail, not produce an empty copy")
	}

	dir := tempAttachmentDir
	CleanupAttachments()
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Errorf("CleanupAttachments left %s behind", dir)
	}
}

func TestSetAttachmentConfig(t *testing.T) {
	old := attachmentConfig
	t.Cleanup(func() { attachmentConfig = old })
	SetAttachmentConfig(AttachmentConfig{MaxFileSize: 5, TempDir: "/x"})
	if attachmentConfig.MaxFileSize != 5 || attachmentConfig.TempDir != "/x" {
		t.Errorf("config = %+v", attachmentConfig)
	}
	SetAttachmentConfig(AttachmentConfig{}) // zero values must not wipe the settings
	if attachmentConfig.MaxFileSize != 5 {
		t.Errorf("an empty config overwrote the limit: %+v", attachmentConfig)
	}
}

func TestSendAttachmentLogUploadsFileAndRemovesTempCopy(t *testing.T) {
	var contentType string
	var body strings.Builder
	tl := newTestListener(t, func(w http.ResponseWriter, r *http.Request) {
		contentType = r.Header.Get("Content-Type")
		_ = r.ParseMultipartForm(1 << 20)
		for _, files := range r.MultipartForm.File {
			for _, f := range files {
				body.WriteString(f.Filename)
			}
		}
		writeJSON(w, `{}`)
	})
	path := filepath.Join(t.TempDir(), "shot.png")
	if err := os.WriteFile(path, []byte("\x89PNG\r\n\x1a\nxxxx"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := tl.SendAttachmentLog(
		"item",
		"INFO",
		"shot",
		path,
		"shot.png",
		time.Time{},
	); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(contentType, "multipart/") || body.String() != "shot.png" {
		t.Errorf(
			"content-type=%q files=%q, want a multipart upload of shot.png",
			contentType,
			body.String(),
		)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("the temp copy must be removed after a successful upload")
	}
}

func TestSendAttachmentLogFallsBackToErrorLog(t *testing.T) {
	var msgs []string
	tl := newTestListener(t, func(w http.ResponseWriter, r *http.Request) {
		for _, l := range decodeLogRQs(t, r) {
			if l.Message != nil {
				msgs = append(msgs, *l.Message)
			}
		}
		writeJSON(w, `{}`)
	})
	// The file is gone: the user must still see a log explaining it.
	if err := tl.SendAttachmentLog(
		"item",
		"INFO",
		"shot",
		"/nonexistent/x.png",
		"x.png",
		time.Time{},
	); err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 1 || !strings.Contains(msgs[0], "failed to open attachment x.png") {
		t.Errorf("logs = %q", msgs)
	}
}

func TestSendAttachmentLogUploadRejectedKeepsMessage(t *testing.T) {
	var msgs []string
	tl := newTestListener(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/") {
			ct := r.Header.Get("Content-Type")
			_ = ct
			// reject uploads that carry a file, accept plain logs
			if err := r.ParseMultipartForm(1 << 20); err == nil && len(r.MultipartForm.File) > 0 {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
		}
		for _, l := range decodeLogRQs(t, r) {
			msgs = append(msgs, *l.Message)
		}
		writeJSON(w, `{}`)
	})
	path := filepath.Join(t.TempDir(), "big.bin")
	_ = os.WriteFile(path, []byte("data"), 0o600)
	if err := tl.SendAttachmentLog(
		"item",
		"INFO",
		"my note",
		path,
		"big.bin",
		time.Time{},
	); err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 1 || !strings.Contains(msgs[0], "my note") ||
		!strings.Contains(msgs[0], "could not be uploaded") {
		t.Errorf("fallback logs = %q", msgs)
	}
}

func TestFormatAttachmentError(t *testing.T) {
	tests := []struct {
		name, raw, want string
	}{
		{
			"with a message",
			`{"file":"big.txt","msg":"a big file","reason":"the file is 10 bytes, the limit is 5 bytes"}`,
			`Attachment not uploaded: big.txt ("a big file"): the file is 10 bytes, the limit is 5 bytes`,
		},
		{
			"without a message",
			`{"file":"a.png","reason":"cannot access the file: no such file or directory"}`,
			`Attachment not uploaded: a.png: cannot access the file: no such file or directory`,
		},
		{
			"a '#' in the message is kept as typed",
			`{"file":"a.png","msg":"shot #1","reason":"r"}`,
			`Attachment not uploaded: a.png ("shot #1"): r`,
		},
		// Anything the agent cannot read is logged as it is, so the problem is never lost.
		{"not JSON", `broken`, `Attachment error: broken`},
		{"no reason", `{"file":"a.png"}`, `Attachment error: {"file":"a.png"}`},
		{"the old payload shape", `{"Msg":"x#y"}`, `Attachment error: {"Msg":"x#y"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := formatAttachmentError(tt.raw); got != tt.want {
				t.Errorf("formatAttachmentError(%s)\n got  %q\n want %q", tt.raw, got, tt.want)
			}
		})
	}
}

func TestBareErrorDropsTheRepeatedPath(t *testing.T) {
	pe := &fs.PathError{
		Op:   "stat",
		Path: "/a/b/missing.txt",
		Err:  errors.New("no such file or directory"),
	}
	if got := bareError(pe); got != "no such file or directory" {
		t.Errorf("bareError(PathError) = %q", got)
	}
	if got := bareError(errors.Join(errors.New("outer"), pe)); got == "" {
		t.Error("a wrapped error must still produce text")
	}
	if got := bareError(errors.New("plain")); got != "plain" {
		t.Errorf("bareError(plain) = %q", got)
	}
}
