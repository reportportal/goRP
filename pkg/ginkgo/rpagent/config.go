package rpagent

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config holds ReportPortal agent configuration.
type Config struct {
	Endpoint   string
	Project    string
	APIKey     string
	LaunchName string
	Debug      bool

	// Launch metadata (all optional).
	Description string
	Attributes  []LaunchAttribute // from rp.attributes: "key:value;tag;key2:value2"
	Mode        string            // "DEFAULT" (shown on Launches) or "DEBUG"

	// SkippedIssue says whether a spec the user skipped (Skip("reason")) is a defect to look
	// at: true leaves it for ReportPortal to file under To Investigate, false marks it NOT_ISSUE.
	// Pending specs and specs skipped without the user's doing never are issues.
	SkippedIssue bool
}

// LaunchAttribute is one launch attribute. Key is empty for a bare tag.
type LaunchAttribute struct {
	Key   string
	Value string
}

// LoadConfig reads configuration from a reportportal.properties file.
//
// Supported keys:
//
//	rp.endpoint   — ReportPortal server URL (required)
//	rp.project    — project name (required)
//	rp.api.key    — API key (required)
//	rp.launch     — launch name (optional, defaults to "GINKGO-RP-LAUNCH")
//	rp.debug      — enable HTTP debug logging: true/false (optional)
//	rp.description — launch description (optional)
//	rp.attributes  — launch attributes, "key:value;tag;key2:value2" (optional)
//	rp.mode        — DEFAULT or DEBUG (optional, defaults to DEFAULT)
//	rp.skipped.issue — true (default): a spec the user skipped is To Investigate; false: it is not an issue
//
// Every key can be overridden by an environment variable (RP_ENDPOINT, RP_PROJECT,
// RP_API_KEY, RP_LAUNCH, RP_DEBUG, RP_DESCRIPTION, RP_ATTRIBUTES, RP_MODE, RP_SKIPPED_ISSUE); the
// environment wins over the file.
func LoadConfig(filename string) (*Config, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to open config file %s: %w", filename, err)
	}
	defer func() { _ = file.Close() }()

	config := &Config{SkippedIssue: true}
	scanner := bufio.NewScanner(file)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())

		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid format at line %d: %s", lineNum, line)
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		switch key {
		case "rp.endpoint":
			config.Endpoint = value
		case "rp.project":
			config.Project = value
		case "rp.api.key":
			config.APIKey = value
		case "rp.launch":
			config.LaunchName = value
		case "rp.debug":
			parseBool("rp.debug", value, &config.Debug)
		case "rp.description":
			config.Description = value
		case "rp.attributes":
			config.Attributes = parseAttributes(value)
		case "rp.mode":
			config.Mode = value
		case "rp.skipped.issue":
			parseBool("rp.skipped.issue", value, &config.SkippedIssue)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading config file: %w", err)
	}

	return finalizeConfig(config)
}

// ConfigFromEnv builds a configuration from environment variables alone, for CI
// where no properties file exists.
func ConfigFromEnv() (*Config, error) {
	return finalizeConfig(&Config{SkippedIssue: true})
}

// finalizeConfig applies environment overrides, then validates and normalises.
func finalizeConfig(config *Config) (*Config, error) {
	envOverride(&config.Endpoint, "RP_ENDPOINT")
	envOverride(&config.Project, "RP_PROJECT")
	envOverride(&config.APIKey, "RP_API_KEY")
	envOverride(&config.LaunchName, "RP_LAUNCH")
	envOverride(&config.Description, "RP_DESCRIPTION")
	envOverride(&config.Mode, "RP_MODE")
	if v := os.Getenv("RP_DEBUG"); v != "" {
		parseBool("RP_DEBUG", v, &config.Debug)
	}
	if v := os.Getenv("RP_SKIPPED_ISSUE"); v != "" {
		parseBool("RP_SKIPPED_ISSUE", v, &config.SkippedIssue)
	}
	if v := os.Getenv("RP_ATTRIBUTES"); v != "" {
		config.Attributes = parseAttributes(v)
	}

	if config.Endpoint == "" {
		return nil, fmt.Errorf("rp.endpoint (or RP_ENDPOINT) is required")
	}
	if config.Project == "" {
		return nil, fmt.Errorf("rp.project (or RP_PROJECT) is required")
	}
	if config.APIKey == "" {
		return nil, fmt.Errorf("rp.api.key (or RP_API_KEY) is required")
	}
	if config.LaunchName == "" {
		config.LaunchName = "GINKGO-RP-LAUNCH"
	}
	switch strings.ToUpper(config.Mode) {
	case "", "DEFAULT":
		config.Mode = "DEFAULT"
	case "DEBUG":
		config.Mode = "DEBUG"
	default:
		return nil, fmt.Errorf("rp.mode (or RP_MODE) must be DEFAULT or DEBUG, got %q", config.Mode)
	}
	return config, nil
}

// parseBool reads a true/false setting into dst. An unreadable value is ignored with a warning,
// so a typo does not silently change what the user asked for.
func parseBool(name, value string, dst *bool) {
	b, err := strconv.ParseBool(value)
	if err != nil {
		fmt.Fprintf(
			os.Stderr,
			"WARNING: ignoring %s=%q (want true or false); keeping %t\n",
			name,
			value,
			*dst,
		)
		return
	}
	*dst = b
}

func envOverride(dst *string, name string) {
	if v := os.Getenv(name); v != "" {
		*dst = v
	}
}

// parseAttributes parses "key:value;tag;key2:value2". Empty parts are dropped; a part
// without a colon is a bare tag.
func parseAttributes(s string) []LaunchAttribute {
	var out []LaunchAttribute
	for _, part := range strings.Split(s, ";") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if k, v, ok := strings.Cut(part, ":"); ok && strings.TrimSpace(k) != "" {
			out = append(
				out,
				LaunchAttribute{Key: strings.TrimSpace(k), Value: strings.TrimSpace(v)},
			)
		} else {
			out = append(
				out,
				LaunchAttribute{Value: strings.TrimSpace(strings.TrimPrefix(part, ":"))},
			)
		}
	}
	return out
}
