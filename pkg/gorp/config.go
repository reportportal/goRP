package gorp

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"slices"
	"strconv"
	"strings"

	"github.com/reportportal/goRP/v5/pkg/openapi"
)

// Names of the settings [LoadConfig] understands, the same as in ReportPortal's Java agents.
const (
	PropertyEndpoint               = "rp.endpoint"
	PropertyProject                = "rp.project"
	PropertyAPIKey                 = "rp.api.key"
	PropertyUUID                   = "rp.uuid" // the old name of rp.api.key
	PropertyLaunch                 = "rp.launch"
	PropertyLaunchUUID             = "rp.launch.uuid"
	PropertyLaunchUUIDCreationSkip = "rp.launch.uuid.creation.skip"
	PropertyDescription            = "rp.description"
	PropertyAttributes             = "rp.attributes"
	PropertyMode                   = "rp.mode"
	PropertyRerun                  = "rp.rerun"
	PropertyRerunOf                = "rp.rerun.of"
	PropertySkippedIssue           = "rp.skipped.issue"
	PropertyEnable                 = "rp.enable"
)

const (
	// DefaultPropertiesFile is the file [LoadConfig] reads from the working directory when no path is given
	DefaultPropertiesFile = "reportportal.properties"

	// propertyPropertiesPath is set by the RP_PROPERTIES_PATH environment variable
	propertyPropertiesPath  = "rp.properties.path"
	propertyPrefix          = "rp."
	attributesSeparator     = ";"
	attributeValueSeparator = ":"
)

// Config is the settings of a ReportPortal agent, as the Java agents define them: `rp.*` properties
// of a reportportal.properties file, overridden by `RP_*` environment variables.
// A reporter written in Go reads the configuration its users already have for other agents.
type Config struct {
	// Endpoint is the URL of the server (rp.endpoint)
	Endpoint string
	// Project is the name of the project (rp.project)
	Project string
	// APIKey is the API key of the user (rp.api.key, or rp.uuid if the former is not set)
	APIKey string
	// Launch is the name of the launch (rp.launch)
	Launch string
	// LaunchUUID is the UUID of the launch (rp.launch.uuid): of an existing one to report into,
	// or the one to create the launch with, see LaunchUUIDCreationSkip
	LaunchUUID string
	// LaunchUUIDCreationSkip says that the launch with LaunchUUID exists already and must not be
	// started or finished (rp.launch.uuid.creation.skip). True by default.
	LaunchUUIDCreationSkip bool
	// Description is the description of the launch (rp.description)
	Description string
	// Attributes are the attributes of the launch (rp.attributes): "key:value;value;key2:value2"
	Attributes []openapi.ItemAttributesRQ
	// Mode is the mode of the launch (rp.mode). DEFAULT unless the value is "debug" in any case.
	Mode LaunchMode
	// Rerun says that the launch is a rerun of an earlier one (rp.rerun)
	Rerun bool
	// RerunOf is the UUID of the launch to rerun (rp.rerun.of)
	RerunOf string
	// SkippedIssue says whether skipped tests are to be investigated (rp.skipped.issue). True by default;
	// when false, a skipped test is to be reported with the "not an issue" type.
	SkippedIssue bool
	// Enabled says whether to report at all (rp.enable). True by default.
	Enabled bool

	// Properties has every `rp.*` setting as loaded, including the ones without a field above
	// (rp.batch.size.logs, rp.http.proxy, …), for an agent that supports more of them.
	// The API key is among them: don't log the map.
	Properties map[string]string
}

// LoadConfig reads the configuration from a properties file and the environment of the process.
//
// The file is the one at path. If path is empty it is the one the RP_PROPERTIES_PATH environment
// variable points to, and without the variable it is reportportal.properties in the working directory,
// which is allowed to be absent. A file named explicitly must exist.
//
// Environment variables override the file. A variable sets the property that its name gives in lower
// case with dots for underscores: RP_API_KEY sets rp.api.key, RP_RERUN_OF sets rp.rerun.of.
//
// Nothing is required: check the fields you need. Values are read the way the Java agents read them,
// see [ParseConfig].
func LoadConfig(path string) (*Config, error) {
	environ := os.Environ()

	optional := false
	if path == "" {
		path = envProperties(environ)[propertyPropertiesPath]
	}
	if path == "" {
		path, optional = DefaultPropertiesFile, true
	}

	file, err := os.Open(path)
	switch {
	case err == nil:
		defer func() { _ = file.Close() }()

		cfg, parseErr := ParseConfig(file, environ)
		if parseErr != nil {
			return nil, fmt.Errorf("%s: %w", path, parseErr)
		}

		return cfg, nil
	case optional && errors.Is(err, fs.ErrNotExist):
		return ParseConfig(nil, environ)
	default:
		return nil, fmt.Errorf("unable to read ReportPortal properties: %w", err)
	}
}

// ParseConfig builds the configuration from the content of a properties file and a list of "NAME=value"
// environment variables, either of which may be nil. It is [LoadConfig] without access to the file
// system and the environment.
//
// The file has the format of Java's .properties files, in UTF-8. Values are read like the Java agents do:
//
//   - a boolean is true if it is "true" in any case, anything else is false;
//   - rp.mode that is not a known mode means DEFAULT;
//   - rp.attributes is a list separated by ";", each item being "key:value" or just "value";
//     an item with more than one ":" is ignored;
//   - rp.endpoint, rp.project and the API key are trimmed.
//
// An attribute without a value is ignored, because the server would reject the launch.
func ParseConfig(properties io.Reader, environ []string) (*Config, error) {
	props := map[string]string{}
	if properties != nil {
		fromFile, err := parseProperties(properties)
		if err != nil {
			return nil, err
		}
		for k, v := range fromFile {
			if strings.HasPrefix(k, propertyPrefix) {
				props[k] = v
			}
		}
	}
	for k, v := range envProperties(environ) {
		props[k] = v
	}

	apiKey, ok := props[PropertyAPIKey]
	if !ok {
		apiKey = props[PropertyUUID]
	}
	mode, err := ParseLaunchMode(props[PropertyMode])
	if err != nil {
		mode = LaunchModes.Default
	}

	return &Config{
		Endpoint:               strings.TrimSpace(props[PropertyEndpoint]),
		Project:                strings.TrimSpace(props[PropertyProject]),
		APIKey:                 strings.TrimSpace(apiKey),
		Launch:                 props[PropertyLaunch],
		LaunchUUID:             props[PropertyLaunchUUID],
		LaunchUUIDCreationSkip: propertyBool(props, PropertyLaunchUUIDCreationSkip, true),
		Description:            props[PropertyDescription],
		Attributes:             ParseAttributes(props[PropertyAttributes]),
		Mode:                   mode,
		Rerun:                  propertyBool(props, PropertyRerun, false),
		RerunOf:                props[PropertyRerunOf],
		SkippedIssue:           propertyBool(props, PropertySkippedIssue, true),
		Enabled:                propertyBool(props, PropertyEnable, true),
		Properties:             props,
	}, nil
}

// ParseAttributes reads attributes in the format of rp.attributes: "key:value;value;key2:value2".
// Spaces around keys and values are dropped. An item with more than one ":" or without a value
// is ignored, and so is a repeated attribute.
func ParseAttributes(s string) []openapi.ItemAttributesRQ {
	var attributes []openapi.ItemAttributesRQ
	seen := map[[2]string]bool{}
	for item := range strings.SplitSeq(strings.TrimSpace(s), attributesSeparator) {
		if strings.TrimSpace(item) == "" {
			continue
		}
		// Java's String.split drops trailing empty strings: "key:" is the value "key"
		parts := strings.Split(item, attributeValueSeparator)
		for len(parts) > 0 && parts[len(parts)-1] == "" {
			parts = parts[:len(parts)-1]
		}

		var attr openapi.ItemAttributesRQ
		switch len(parts) {
		case 1:
			attr.Value = strings.TrimSpace(parts[0])
		case 2:
			if key := strings.TrimSpace(parts[0]); key != "" {
				attr.Key = openapi.PtrString(key)
			}
			attr.Value = strings.TrimSpace(parts[1])
		default:
			continue
		}

		id := [2]string{attr.GetKey(), attr.Value}
		if attr.Value == "" || seen[id] {
			continue
		}
		seen[id] = true
		attributes = append(attributes, attr)
	}

	return attributes
}

func propertyBool(props map[string]string, name string, def bool) bool {
	v, ok := props[name]
	if !ok {
		return def
	}

	return strings.EqualFold(strings.TrimSpace(v), "true")
}

// envProperties picks the `rp.*` settings out of "NAME=value" environment variables. The name of
// a property is the name of the variable in lower case with dots for underscores. Of two variables
// that give the same property (RP_PROJECT and rp.project) the first in the sorted order wins.
func envProperties(environ []string) map[string]string {
	sorted := slices.Sorted(slices.Values(environ))
	props := map[string]string{}
	for _, kv := range sorted {
		name, value, ok := strings.Cut(kv, "=")
		if !ok {
			continue
		}
		name = strings.ReplaceAll(strings.ToLower(name), "_", ".")
		if !strings.HasPrefix(name, propertyPrefix) {
			continue
		}
		if _, dup := props[name]; !dup {
			props[name] = value
		}
	}

	return props
}

// parseProperties reads the format of java.util.Properties.load: `key=value`, `key:value` or
// `key value` lines, comments starting with # or !, a backslash at the end of a line to continue
// it on the next one, and the escapes \t \n \r \f \uXXXX and \<any other character>.
func parseProperties(r io.Reader) (map[string]string, error) {
	props := map[string]string{}
	scanner := bufio.NewScanner(r)
	scanner.Buffer(nil, 1<<20)
	for line := 0; scanner.Scan(); {
		line++
		text := strings.TrimLeft(scanner.Text(), " \t\f")
		if line == 1 {
			text = strings.TrimPrefix(text, "\xef\xbb\xbf")
		}
		if text == "" || text[0] == '#' || text[0] == '!' {
			continue
		}
		first := line
		for endsWithContinuation(text) {
			text = text[:len(text)-1]
			if !scanner.Scan() {
				break
			}
			line++
			text += strings.TrimLeft(scanner.Text(), " \t\f")
		}

		key, value, err := splitProperty(text)
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", first, err)
		}
		props[key] = value
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("unable to read properties: %w", err)
	}

	return props, nil
}

// endsWithContinuation reports whether the line ends with an odd number of backslashes
func endsWithContinuation(s string) bool {
	n := 0
	for n < len(s) && s[len(s)-1-n] == '\\' {
		n++
	}

	return n%2 == 1
}

// splitProperty cuts a logical line into the key and the value and resolves the escapes.
// The key ends at the first unescaped '=', ':' or white space.
func splitProperty(line string) (key, value string, err error) {
	end := len(line)
	for i := 0; i < len(line); i++ {
		if line[i] == '\\' {
			i++
			continue
		}
		if strings.IndexByte("=: \t\f", line[i]) >= 0 {
			end = i
			break
		}
	}

	rest := strings.TrimLeft(line[end:], " \t\f")
	if rest != "" && (rest[0] == '=' || rest[0] == ':') {
		rest = strings.TrimLeft(rest[1:], " \t\f")
	}

	if key, err = unescapeProperty(line[:end]); err != nil {
		return "", "", err
	}
	if value, err = unescapeProperty(rest); err != nil {
		return "", "", err
	}

	return key, value, nil
}

var errMalformedUnicodeEscape = errors.New(`malformed \uXXXX escape`)

func unescapeProperty(s string) (string, error) {
	if !strings.Contains(s, `\`) {
		return s, nil
	}

	var sb strings.Builder
	var surrogate rune
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c != '\\' || i == len(s)-1 {
			sb.WriteByte(c)
			surrogate = 0
			continue
		}
		i++
		if s[i] != 'u' {
			surrogate = 0
		}
		switch s[i] {
		case 't':
			sb.WriteByte('\t')
		case 'n':
			sb.WriteByte('\n')
		case 'r':
			sb.WriteByte('\r')
		case 'f':
			sb.WriteByte('\f')
		case 'u':
			const digits = 4
			if i+digits >= len(s) {
				return "", errMalformedUnicodeEscape
			}
			code, err := strconv.ParseUint(s[i+1:i+1+digits], 16, 16)
			if err != nil {
				return "", errMalformedUnicodeEscape
			}
			i += digits
			surrogate = writeUTF16Unit(&sb, surrogate, rune(code))
		default:
			sb.WriteByte(s[i])
		}
	}

	return sb.String(), nil
}

// writeUTF16Unit writes a UTF-16 code unit given as \uXXXX. The first half of a surrogate pair is
// returned to be passed with the second half; a half without its pair is dropped.
func writeUTF16Unit(sb *strings.Builder, high, unit rune) rune {
	const (
		highStart, lowStart, lowEnd = 0xD800, 0xDC00, 0xDFFF
		supplementaryStart          = 0x10000
		halfShift                   = 10
	)
	switch {
	case unit >= highStart && unit < lowStart:
		return unit
	case unit >= lowStart && unit <= lowEnd:
		if high != 0 {
			sb.WriteRune(supplementaryStart + (high-highStart)<<halfShift + (unit - lowStart))
		}
	default:
		sb.WriteRune(unit)
	}

	return 0
}
