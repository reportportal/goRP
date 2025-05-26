package gorp

// Client constants
const (
	LogLevelDebug = "DEBUG"
	LogLevelInfo  = "INFO"
	LogLevelError = "ERROR"
)

// PAYLOADS
type (
	PageDetails struct {
		PageNumber int
		PageSize   int
		SortBy     string
	}
)
