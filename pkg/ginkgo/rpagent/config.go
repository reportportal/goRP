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
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading config file: %w", err)
	}

	if config.Endpoint == "" {
		return nil, fmt.Errorf("rp.endpoint is required")
	}
	if config.Project == "" {
		return nil, fmt.Errorf("rp.project is required")
	}
	if config.APIKey == "" {
		return nil, fmt.Errorf("rp.api.key is required")
	}
	if config.LaunchName == "" {
		config.LaunchName = "GINKGO-RP-LAUNCH"
	}

	return config, nil
}
