package gorp

// Log levels the server knows by name, from the least to the most severe
const (
	LogLevelTrace = "TRACE"
	LogLevelDebug = "DEBUG"
	LogLevelInfo  = "INFO"
	LogLevelWarn  = "WARN"
	LogLevelError = "ERROR"
	LogLevelFatal = "FATAL"
)

// PAYLOADS
type (
	PageDetails struct {
		PageNumber int
		PageSize   int
		SortBy     string
	}
)
