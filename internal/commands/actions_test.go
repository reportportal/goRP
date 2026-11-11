package commands

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v3"
)

func TestParseAttributes_OnlyFirstColonSeparates(t *testing.T) {
	t.Parallel()

	attrs := parseAttributes([]string{"url:http://x:80"})

	require.Len(t, attrs, 1)
	assert.Equal(t, "url", attrs[0].GetKey())
	assert.Equal(t, "http://x:80", attrs[0].GetValue())
	assert.False(t, attrs[0].GetSystem())
}

// connectionFlags are the global flags the actions read the configuration from.
func connectionFlags(host string) []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{Name: "api-key", Value: "k"},
		&cli.StringFlag{Name: "project", Value: "proj"},
		&cli.StringFlag{Name: "host", Value: host},
	}
}

func TestWithConfig(t *testing.T) {
	t.Setenv("HOME", t.TempDir()) // no ~/.gorp

	t.Run(
		"passes the resolved configuration to the step and returns its error",
		func(t *testing.T) {
			stepErr := errors.New("boom")
			var got *clientConfig
			action := withConfig(
				func(_ context.Context, cfg *clientConfig, _ *cli.Command) (string, error) {
					got = cfg
					return "ignored", stepErr
				},
			)

			err := action(t.Context(), &cli.Command{Flags: connectionFlags("http://rp.local")})

			require.ErrorIs(t, err, stepErr)
			require.NotNil(t, got)
			assert.Equal(
				t,
				clientConfig{URL: "http://rp.local", Project: "proj", ApiKey: "k"},
				*got,
			)
		},
	)

	t.Run("does not run the step when the configuration is incomplete", func(t *testing.T) {
		called := false
		action := withConfig(func(context.Context, *clientConfig, *cli.Command) (string, error) {
			called = true
			return "", nil
		})

		err := action(t.Context(), &cli.Command{Flags: connectionFlags("")})

		require.ErrorIs(t, err, errHostNotSet)
		assert.False(t, called)
	})

	t.Run("adapts steps that only return an error", func(t *testing.T) {
		stepErr := errors.New("boom")
		action := withConfig(
			adaptNoResult(func(context.Context, *clientConfig, *cli.Command) error {
				return stepErr
			}),
		)

		err := action(t.Context(), &cli.Command{Flags: connectionFlags("http://rp.local")})

		require.ErrorIs(t, err, stepErr)
	})
}

func test2jsonCmd(host, file string, extra ...cli.Flag) (*cli.Command, *bytes.Buffer) {
	var out bytes.Buffer
	flags := append(connectionFlags(host),
		&cli.StringFlag{Name: "file", Value: file},
		&cli.StringFlag{Name: "launchName", Value: "launch"},
		&cli.BoolFlag{Name: "reportEmptyPkg"},
		&cli.StringSliceFlag{Name: "attr"},
		&cli.BoolFlag{Name: "print-launch-uuid", Value: true},
	)

	return &cli.Command{Flags: append(flags, extra...), Writer: &out}, &out
}

func TestReportTest2Json_PrintLaunchUUID(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	project := "proj"
	srv := newRecordingServer(t, project, false)
	file := writeEvents(t, []string{
		`{"time":"2025-01-01T00:00:00Z","action":"run","package":"p","test":"TestA"}`,
		`{"time":"2025-01-01T00:00:01Z","action":"pass","package":"p","test":"TestA"}`,
		`{"time":"2025-01-01T00:00:02Z","action":"pass","package":"p"}`,
	})
	cmd, out := test2jsonCmd(srv.URL, file)

	require.NoError(t, reportTest2Json(t.Context(), cmd))

	assert.Equal(t, "ReportPortal Launch UUID: launch-uuid-1\n", out.String())
}

func TestReportTest2Json_EmptyInput(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	srv := newRecordingServer(t, "proj", false)
	file := writeEvents(t, nil)

	t.Run("prints nothing", func(t *testing.T) {
		cmd, out := test2jsonCmd(srv.URL, file)

		require.NoError(t, reportTest2Json(t.Context(), cmd))

		assert.Empty(t, out.String())
	})

	t.Run("quality gate check explains that there is no launch", func(t *testing.T) {
		cmd, _ := test2jsonCmd(srv.URL, file,
			&cli.BoolFlag{Name: "quality-gate-check", Value: true})

		err := reportTest2Json(t.Context(), cmd)

		var exitErr cli.ExitCoder
		require.ErrorAs(t, err, &exitErr)
		assert.Equal(t, 1, exitErr.ExitCode())
		assert.Contains(t, err.Error(), "no launch was started")
	})
}
