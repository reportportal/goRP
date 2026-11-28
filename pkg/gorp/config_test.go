package gorp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/reportportal/goRP/v5/pkg/openapi"
)

func attr(key, value string) openapi.ItemAttributesRQ {
	a := openapi.ItemAttributesRQ{Value: value}
	if key != "" {
		a.Key = openapi.PtrString(key)
	}

	return a
}

const sampleProperties = `# ReportPortal agent settings, as for a Java agent
rp.endpoint = https://rp.example.com/
rp.api.key=key_from_file
rp.project : my_project
rp.launch   Nightly regression
rp.description = Line one \
                 and line two
rp.attributes = os:linux;smoke;browser:chrome
rp.mode = debug
rp.rerun = TRUE
rp.rerun.of = 7b1c0d5e-3f3a-4a57-9a51-0d4a54e3c9b1
rp.skipped.issue = false
rp.batch.size.logs = 20
! not for the agent
other.setting = x
`

func TestParseConfigFile(t *testing.T) {
	t.Parallel()

	cfg, err := ParseConfig(strings.NewReader(sampleProperties), nil)
	require.NoError(t, err)

	assert.Equal(t, "https://rp.example.com/", cfg.Endpoint)
	assert.Equal(t, "key_from_file", cfg.APIKey)
	assert.Equal(t, "my_project", cfg.Project)
	assert.Equal(t, "Nightly regression", cfg.Launch)
	assert.Equal(t, "Line one and line two", cfg.Description)
	assert.Equal(
		t,
		[]openapi.ItemAttributesRQ{
			attr("os", "linux"),
			attr("", "smoke"),
			attr("browser", "chrome"),
		},
		cfg.Attributes,
	)
	assert.Equal(t, LaunchModes.Debug, cfg.Mode)
	assert.True(t, cfg.Rerun)
	assert.Equal(t, "7b1c0d5e-3f3a-4a57-9a51-0d4a54e3c9b1", cfg.RerunOf)
	assert.False(t, cfg.SkippedIssue)
	assert.True(t, cfg.Enabled)
	assert.True(t, cfg.LaunchUUIDCreationSkip)
	assert.Empty(t, cfg.LaunchUUID)

	// settings without a field stay available, the ones of other tools are left out
	assert.Equal(t, "20", cfg.Properties["rp.batch.size.logs"])
	assert.NotContains(t, cfg.Properties, "other.setting")
}

func TestParseConfigDefaults(t *testing.T) {
	t.Parallel()

	cfg, err := ParseConfig(nil, nil)
	require.NoError(t, err)

	assert.Equal(t, &Config{
		Mode:                   LaunchModes.Default,
		SkippedIssue:           true,
		Enabled:                true,
		LaunchUUIDCreationSkip: true,
		Properties:             map[string]string{},
	}, cfg)
}

func TestParseConfigEnvOverridesFile(t *testing.T) {
	t.Parallel()

	cfg, err := ParseConfig(strings.NewReader(sampleProperties), []string{
		"PATH=/usr/bin",
		"RP_ENDPOINT=https://other.example.com",
		"RP_API_KEY= key_from_env ",
		"RP_ATTRIBUTES=ci:true",
		"RP_MODE=DEFAULT",
		"RP_RERUN=false",
		"RP_RERUN_OF=",
		"RP_SKIPPED_ISSUE=true",
		"RP_ENABLE=false",
		"RP_LAUNCH_UUID=launch-uuid",
		"RP_LAUNCH_UUID_CREATION_SKIP=false",
		"RP_HTTP_PROXY=http://proxy:3128",
		"RPX_PROJECT=not_ours",
		"GORP_PROJECT=not_ours_either",
		"malformed",
	})
	require.NoError(t, err)

	assert.Equal(t, "https://other.example.com", cfg.Endpoint)
	assert.Equal(t, "key_from_env", cfg.APIKey, "the API key is trimmed")
	assert.Equal(t, "my_project", cfg.Project, "not overridden: stays as in the file")
	assert.Equal(t, []openapi.ItemAttributesRQ{attr("ci", "true")}, cfg.Attributes)
	assert.Equal(t, LaunchModes.Default, cfg.Mode)
	assert.False(t, cfg.Rerun)
	assert.Empty(t, cfg.RerunOf, "an empty variable overrides too")
	assert.True(t, cfg.SkippedIssue)
	assert.False(t, cfg.Enabled)
	assert.Equal(t, "launch-uuid", cfg.LaunchUUID)
	assert.False(t, cfg.LaunchUUIDCreationSkip)
	assert.Equal(t, "http://proxy:3128", cfg.Properties["rp.http.proxy"])
}

func TestParseConfigEnvNames(t *testing.T) {
	t.Parallel()

	// any case, dots or underscores: the Java agents accept all of these
	for _, name := range []string{"RP_PROJECT", "rp_project", "rp.project", "Rp_Project"} {
		cfg, err := ParseConfig(nil, []string{name + "=p"})
		require.NoError(t, err)
		assert.Equal(t, "p", cfg.Project, name)
	}

	// two spellings of one property: the choice must not depend on the order of the environment
	for _, environ := range [][]string{
		{"RP_PROJECT=upper", "rp.project=lower"},
		{"rp.project=lower", "RP_PROJECT=upper"},
	} {
		cfg, err := ParseConfig(nil, environ)
		require.NoError(t, err)
		assert.Equal(t, "upper", cfg.Project)
	}
}

func TestParseConfigAPIKeyFallsBackToUUID(t *testing.T) {
	t.Parallel()

	cfg, err := ParseConfig(strings.NewReader("rp.uuid = old_name\n"), nil)
	require.NoError(t, err)
	assert.Equal(t, "old_name", cfg.APIKey)

	cfg, err = ParseConfig(strings.NewReader("rp.uuid = old_name\nrp.api.key = new_name\n"), nil)
	require.NoError(t, err)
	assert.Equal(t, "new_name", cfg.APIKey)

	cfg, err = ParseConfig(
		strings.NewReader("rp.api.key = from_file\n"),
		[]string{"RP_UUID=from_env"},
	)
	require.NoError(t, err)
	assert.Equal(t, "from_file", cfg.APIKey, "rp.api.key wins over rp.uuid wherever they come from")
}

func TestParseConfigValues(t *testing.T) {
	t.Parallel()

	for value, want := range map[string]bool{
		"true": true, "TRUE": true, "True": true, " true ": true,
		"false": false, "yes": false, "1": false, "on": false, "": false,
	} {
		cfg, err := ParseConfig(
			nil,
			[]string{"RP_RERUN=" + value, "RP_ENABLE=" + value, "RP_SKIPPED_ISSUE=" + value},
		)
		require.NoError(t, err)
		assert.Equal(t, want, cfg.Rerun, "%q", value)
		assert.Equal(t, want, cfg.Enabled, "%q", value)
		assert.Equal(t, want, cfg.SkippedIssue, "%q", value)
	}

	for value, want := range map[string]LaunchMode{
		"debug": LaunchModes.Debug, "DEBUG": LaunchModes.Debug, "Default": LaunchModes.Default,
		"": LaunchModes.Default, "production": LaunchModes.Default,
	} {
		cfg, err := ParseConfig(nil, []string{"RP_MODE=" + value})
		require.NoError(t, err)
		assert.Equal(t, want, cfg.Mode, "%q", value)
	}
}

func TestParseAttributes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in   string
		want []openapi.ItemAttributesRQ
	}{
		{in: "", want: nil},
		{in: "   ", want: nil},
		{in: ";;", want: nil},
		{in: "smoke", want: []openapi.ItemAttributesRQ{attr("", "smoke")}},
		{in: "os:linux", want: []openapi.ItemAttributesRQ{attr("os", "linux")}},
		{
			in:   " os : linux ; smoke ;",
			want: []openapi.ItemAttributesRQ{attr("os", "linux"), attr("", "smoke")},
		},
		// as in Java: an empty key means no key, a trailing separator leaves the value alone
		{in: ":linux", want: []openapi.ItemAttributesRQ{attr("", "linux")}},
		{in: "os:", want: []openapi.ItemAttributesRQ{attr("", "os")}},
		// more than one separator: ignored, the others stay
		{in: "a:b:c;ok", want: []openapi.ItemAttributesRQ{attr("", "ok")}},
		{in: "url:http://x;ok", want: []openapi.ItemAttributesRQ{attr("", "ok")}},
		// no value: ignored
		{in: "os: ;:;ok", want: []openapi.ItemAttributesRQ{attr("", "ok")}},
		// repeated: once
		{
			in:   "a:b;a:b;b;b;a:c",
			want: []openapi.ItemAttributesRQ{attr("a", "b"), attr("", "b"), attr("a", "c")},
		},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, ParseAttributes(tt.in), "%q", tt.in)
	}
}

func TestParseProperties(t *testing.T) {
	t.Parallel()

	const file = "\ufeff# comment\n" +
		"  ! another comment\n" +
		"\n" +
		"a=1\n" +
		"b : 2\n" +
		"c 3\n" +
		"d\t=\t4  \n" +
		"   e   =   spaces before are dropped, after are kept  \n" +
		"f=\n" +
		"g\n" +
		"h = a=b:c\n" +
		`i\ j\=k\:l = escaped key` + "\n" +
		`m = tab\there, newline\nhere, backslash\\, plain \q` + "\n" +
		`n = caf\u00e9 \uD83D\uDE00 \u00E9` + "\n" +
		"o = one, \\\n" +
		"      two, \\\n" +
		"  three\n" +
		`p = ends with a backslash\\` + "\n" +
		"q = last line continues\\"

	props, err := parseProperties(strings.NewReader(file))
	require.NoError(t, err)

	assert.Equal(t, map[string]string{
		"a":       "1",
		"b":       "2",
		"c":       "3",
		"d":       "4  ",
		"e":       "spaces before are dropped, after are kept  ",
		"f":       "",
		"g":       "",
		"h":       "a=b:c",
		"i j=k:l": "escaped key",
		"m":       "tab\there, newline\nhere, backslash\\, plain q",
		"n":       "café 😀 é",
		"o":       "one, two, three",
		"p":       `ends with a backslash\`,
		"q":       "last line continues",
	}, props)

	// the last definition of a key wins
	props, err = parseProperties(strings.NewReader("a=1\na=2\n"))
	require.NoError(t, err)
	assert.Equal(t, "2", props["a"])

	// Windows line ends
	props, err = parseProperties(strings.NewReader("a=1\r\nb=2\r\n"))
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"a": "1", "b": "2"}, props)
}

func TestParsePropertiesErrors(t *testing.T) {
	t.Parallel()

	for _, file := range []string{"a=1\nrp.launch = bad \\u12 escape\n", "a=1\nrp.launch = \\uZZZZ\n"} {
		_, err := ParseConfig(strings.NewReader(file), nil)
		require.ErrorIs(t, err, errMalformedUnicodeEscape)
		assert.ErrorContains(t, err, "line 2")
	}
}

// Not parallel: it changes the environment and the working directory.
func TestLoadConfig(t *testing.T) {
	dir := t.TempDir()
	explicit := filepath.Join(dir, "custom.properties")
	require.NoError(
		t,
		os.WriteFile(explicit, []byte("rp.project = explicit\nrp.launch = from file\n"), 0o600),
	)
	viaEnv := filepath.Join(dir, "env.properties")
	require.NoError(t, os.WriteFile(viaEnv, []byte("rp.project = via_env\n"), 0o600))

	// the settings of the machine that runs the tests must not leak in
	for _, kv := range os.Environ() {
		name, _, _ := strings.Cut(kv, "=")
		if strings.HasPrefix(strings.ToLower(name), "rp_") ||
			strings.HasPrefix(strings.ToLower(name), "rp.") {
			t.Setenv(name, "")
			require.NoError(t, os.Unsetenv(name))
		}
	}
	t.Chdir(dir)

	// nothing configured: the default file may be absent
	cfg, err := LoadConfig("")
	require.NoError(t, err)
	assert.Empty(t, cfg.Project)
	assert.True(t, cfg.Enabled)

	// the default file in the working directory
	require.NoError(
		t,
		os.WriteFile(
			filepath.Join(dir, DefaultPropertiesFile),
			[]byte("rp.project = default_file\n"),
			0o600,
		),
	)
	cfg, err = LoadConfig("")
	require.NoError(t, err)
	assert.Equal(t, "default_file", cfg.Project)

	// an explicit path, and the environment over it
	t.Setenv("RP_LAUNCH", "from env")
	cfg, err = LoadConfig(explicit)
	require.NoError(t, err)
	assert.Equal(t, "explicit", cfg.Project)
	assert.Equal(t, "from env", cfg.Launch)

	// RP_PROPERTIES_PATH instead of the default file; an explicit path still wins
	t.Setenv("RP_PROPERTIES_PATH", viaEnv)
	cfg, err = LoadConfig("")
	require.NoError(t, err)
	assert.Equal(t, "via_env", cfg.Project)
	cfg, err = LoadConfig(explicit)
	require.NoError(t, err)
	assert.Equal(t, "explicit", cfg.Project)

	// a file that was asked for must exist
	_, err = LoadConfig(filepath.Join(dir, "missing.properties"))
	require.ErrorIs(t, err, os.ErrNotExist)
	t.Setenv("RP_PROPERTIES_PATH", filepath.Join(dir, "missing.properties"))
	_, err = LoadConfig("")
	require.ErrorIs(t, err, os.ErrNotExist)

	// a broken file is reported with its path
	broken := filepath.Join(dir, "broken.properties")
	require.NoError(t, os.WriteFile(broken, []byte("rp.launch = \\uXYZ1\n"), 0o600))
	_, err = LoadConfig(broken)
	require.ErrorIs(t, err, errMalformedUnicodeEscape)
	assert.ErrorContains(t, err, "broken.properties: line 1")
}
