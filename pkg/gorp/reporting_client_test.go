package gorp

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/reportportal/goRP/v5/pkg/openapi"
)

func TestSaveLog(t *testing.T) {
	t.Parallel()

	// Setup test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v2/prj/log", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)

		response := `{"id": "log123"}`
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(response))
	}))
	defer server.Close()

	client := NewReportingClient(server.URL, "prj", WithApiKeyAuth(t.Context(), "uuid"))
	log := &openapi.SaveLogRQ{
		ItemUuid: openapi.PtrString("item123"),
		Level:    openapi.PtrString(LogLevelInfo),
		Message:  openapi.PtrString("Test log message"),
		Time:     time.Now(),
	}

	result, err := client.SaveLog(log)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "log123", *result.Id)
}

func TestSaveLogs(t *testing.T) {
	t.Parallel()

	// Setup test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v2/prj/log", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)

		// Check content type includes multipart
		contentType := r.Header.Get("Content-Type")
		assert.Contains(t, contentType, "multipart/form-data")

		response := `{"id": "batch123"}`
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(response))
	}))
	defer server.Close()

	client := NewReportingClient(server.URL, "prj", WithApiKeyAuth(t.Context(), "uuid"))
	logs := []*openapi.SaveLogRQ{
		{
			ItemUuid: openapi.PtrString("item123"),
			Level:    openapi.PtrString(LogLevelInfo),
			Message:  openapi.PtrString("Test log message 1"),
			Time:     time.Now(),
		},
		{
			ItemUuid: openapi.PtrString("item456"),
			Level:    openapi.PtrString(LogLevelError),
			Message:  openapi.PtrString("Test log message 2"),
			Time:     time.Now(),
		},
	}

	result, err := client.SaveLogs(logs...)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "batch123", *result.Id)
}

func TestSaveLogMultipart(t *testing.T) {
	t.Parallel()

	// Create a temporary file for testing
	tmpFile, err := os.CreateTemp("", "test-*.txt")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name()) //nolint: errcheck
	_, err = tmpFile.WriteString("test content")
	require.NoError(t, err)
	require.NoError(t, tmpFile.Close())

	// Setup test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v2/prj/log", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)

		// Check that it's a multipart request
		err := r.ParseMultipartForm(10 << 20) // 10MB max
		assert.NoError(t, err)

		// Verify both parts exist
		_, ok := r.MultipartForm.File["file"]
		assert.True(t, ok, "File part should exist")
		_, ok = r.MultipartForm.Value["json_request_part"]
		assert.True(t, ok, "JSON part should exist")

		response := `{"id": "multipart123"}`
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(response))
	}))
	defer server.Close()

	client := NewReportingClient(server.URL, "prj", WithApiKeyAuth(t.Context(), "uuid"))

	// Open the file again for reading
	f, err := os.Open(tmpFile.Name())
	require.NoError(t, err)
	defer f.Close() //nolint: errcheck

	logs := []*openapi.SaveLogRQ{
		{
			ItemUuid: openapi.PtrString("item123"),
			Level:    openapi.PtrString(LogLevelError),
			Message:  openapi.PtrString("Log with attachment"),
			Time:     time.Now(),
			File: &openapi.File{
				Name: openapi.PtrString(f.Name()),
			},
		},
	}

	files := []Multipart{
		&FileMultipart{File: f},
	}

	result, err := client.SaveLogMultipart(logs, files)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "multipart123", *result.Id)
}

// Test MultipartField error handling
func TestSaveLogMultipartErrors(t *testing.T) {
	t.Parallel()

	client := NewReportingClient("http://localhost", "prj", WithApiKeyAuth(t.Context(), "uuid"))

	// Test with empty filename
	logs := []*openapi.SaveLogRQ{{
		ItemUuid: openapi.PtrString("item123"),
		Level:    openapi.PtrString(LogLevelError),
		Message:  openapi.PtrString("Error log"),
	}}

	emptyFilename := &mockMultipart{
		fileName:    "",
		contentType: "text/plain",
		reader:      bytes.NewBufferString("content"),
		err:         nil,
	}

	_, err := client.SaveLogMultipart(logs, []Multipart{emptyFilename})
	assert.Error(t, err)
	assert.Equal(t, err, errMultipartFilename)

	// Test with load error
	loadError := &mockMultipart{
		fileName:    "file.txt",
		contentType: "text/plain",
		reader:      nil,
		err:         fmt.Errorf("cannot load file"),
	}

	_, err = client.SaveLogMultipart(logs, []Multipart{loadError})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unable to read multipart")
}

// Mock implementation of Multipart interface for testing
type mockMultipart struct {
	fileName    string
	contentType string
	reader      io.Reader
	err         error
}

func (m *mockMultipart) Load() (string, string, io.Reader, error) {
	return m.fileName, m.contentType, m.reader, m.err
}
