package rpagent

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// RPLogger writes agent diagnostic logs to a file.
type RPLogger struct {
	logFile *os.File
	mu      sync.Mutex
}

var (
	rpLogger   *RPLogger
	rpLoggerMu sync.Mutex // guards reads and writes of rpLogger
)

// InitializeRPLogger creates (or appends to) rp-agent.log in the project root.
func InitializeRPLogger() error {
	root, err := findProjectRoot()
	if err != nil {
		root, _ = os.Getwd()
	}

	logPath := filepath.Join(root, "rp-agent.log")
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return fmt.Errorf("failed to create rp-agent.log: %w", err)
	}

	l := &RPLogger{logFile: f}
	l.write(
		"INFO",
		"RP_LOGGER",
		fmt.Sprintf("ReportPortal Agent started — %s", time.Now().Format("2006-01-02 15:04:05")),
	)
	l.write("INFO", "RP_LOGGER", fmt.Sprintf("log file: %s", logPath))

	rpLoggerMu.Lock()
	rpLogger = l
	rpLoggerMu.Unlock()
	return nil
}

// CloseRPLogger flushes and closes the log file.
func CloseRPLogger() {
	rpLoggerMu.Lock()
	l := rpLogger
	rpLogger = nil
	rpLoggerMu.Unlock()

	if l != nil && l.logFile != nil {
		l.write(
			"INFO",
			"RP_LOGGER",
			fmt.Sprintf(
				"ReportPortal Agent stopped — %s",
				time.Now().Format("2006-01-02 15:04:05"),
			),
		)
		_ = l.logFile.Close()
	}
}

func (l *RPLogger) write(level, component, message string) {
	if l == nil || l.logFile == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	entry := fmt.Sprintf("[%s] %s [%s] %s\n",
		time.Now().Format("2006-01-02 15:04:05.000"), level, component, message)
	_, _ = l.logFile.WriteString(entry)
	if level == "ERROR" || level == "FATAL" {
		_ = l.logFile.Sync()
	}
}

// currentLogger returns the current logger under the mutex (may be nil).
func currentLogger() *RPLogger {
	rpLoggerMu.Lock()
	l := rpLogger
	rpLoggerMu.Unlock()
	return l
}

// LogVerboseOperation logs a debug-level message to the agent log file.
func LogVerboseOperation(message string) {
	if l := currentLogger(); l != nil {
		l.write("DEBUG", "AGENT", message)
	}
}

// LogInfo logs an info-level message.
func LogInfo(component, message string) {
	if l := currentLogger(); l != nil {
		l.write("INFO", component, message)
	}
}

// LogError logs an error-level message.
func LogError(component, message string) {
	if l := currentLogger(); l != nil {
		l.write("ERROR", component, message)
	}
}

func logHTTP(method, url, status string, duration time.Duration, reqBody, respBody string) {
	l := currentLogger()
	if l == nil {
		return
	}
	entry := fmt.Sprintf(
		"HTTP %s %s -> %s (%v)\nRequest:  %s\nResponse: %s",
		method, url, status, duration,
		formatJSON(reqBody), formatJSON(respBody),
	)
	l.write("DEBUG", "HTTP", maskAuth(entry))
}

func formatJSON(body string) string {
	if body == "" {
		return "<empty>"
	}
	var v interface{}
	if err := json.Unmarshal([]byte(body), &v); err == nil {
		if b, err := json.MarshalIndent(v, "", "  "); err == nil {
			return string(b)
		}
	}
	return body
}

func maskAuth(s string) string {
	// Redact bearer tokens in logged output
	if idx := strings.Index(s, "Authorization: Bearer "); idx != -1 {
		end := strings.Index(s[idx+22:], "\n")
		if end == -1 {
			return s[:idx+22] + "[REDACTED]"
		}
		return s[:idx+22] + "[REDACTED]" + s[idx+22+end:]
	}
	return s
}

func findProjectRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", fmt.Errorf("project root (go.mod) not found")
}
