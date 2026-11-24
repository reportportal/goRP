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
//
// Every key can be overridden by an environment variable (RP_ENDPOINT, RP_PROJECT,
// RP_API_KEY, RP_LAUNCH, RP_DEBUG, RP_DESCRIPTION, RP_ATTRIBUTES, RP_MODE); the
// environment wins over the file.
func LoadConfig(filename string) (*Config, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to open config file %s: %w", filename, err)
	}
	defer func() { _ = file.Close() }()

	config := &Config{}
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
			if debug, err := strconv.ParseBool(value); err == nil {
				config.Debug = debug
			}
		case "rp.description":
			config.Description = value
		case "rp.attributes":
			config.Attributes = parseAttributes(value)
		case "rp.mode":
			config.Mode = value
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
	return finalizeConfig(&Config{})
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
		if debug, err := strconv.ParseBool(v); err == nil {
			config.Debug = debug
		}
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
