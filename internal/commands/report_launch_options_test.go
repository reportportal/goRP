package commands

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v3"
)

// launchOptionsCmd is reportCmd plus the launch options of `report test2json`.
func launchOptionsCmd(t *testing.T, file string, set map[string]string) *cli.Command {
	t.Helper()
	cmd := reportCmd(file, "launch", false)
	cmd.Flags = append(cmd.Flags,
		&cli.StringFlag{Name: "description"},
		&cli.StringFlag{Name: "mode", Value: "DEFAULT"},
		&cli.BoolFlag{Name: "rerun"},
		&cli.StringFlag{Name: "rerun-of"},
	)
	for name, value := range set {
		require.NoError(t, cmd.Set(name, value))
	}
	return cmd
}

func TestReportLaunch_LaunchOptions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		set  map[string]string
		want map[string]any // expected fields of the launch start request; nil means absent
	}{
		{
			name: "defaults",
			want: map[string]any{
				"mode":        "DEFAULT",
				"description": nil,
				"rerun":       nil,
				"rerunOf":     nil,
			},
		},
		{
			name: "description and debug mode",
			set:  map[string]string{"description": "nightly run", "mode": "debug"},
			want: map[string]any{"mode": "DEBUG", "description": "nightly run", "rerun": nil},
		},
		{
			name: "rerun of the latest launch with the same name",
			set:  map[string]string{"rerun": "true"},
			want: map[string]any{"rerun": true, "rerunOf": nil},
		},
		{
			name: "rerun-of implies rerun",
			set:  map[string]string{"rerun-of": "original-launch"},
			want: map[string]any{"rerun": true, "rerunOf": "original-launch"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			rec, srv := newHierarchyServer(t, "proj")
			file := writeEventLines(
				t,
				event("run", "TestOne"),
				event("pass", "TestOne"),
				event("pass", ""),
			)
			cfg := &clientConfig{URL: srv.URL, Project: "proj", ApiKey: "tok"}

			launchID, _, err := reportLaunch(t.Context(), cfg, launchOptionsCmd(t, file, tc.set))
			require.NoError(t, err)

			for field, want := range tc.want {
				if want == nil {
					assert.NotContains(t, rec.launchStart, field)
					continue
				}
				assert.Equal(t, want, rec.launchStart[field], field)
			}
			// the launch is whatever the server answered with, also when --rerun-of names another UUID
			assert.Equal(t, "launch-1", launchID)
			assert.Equal(t, "launch-1", rec.item(t, "TestOne").Body["launchUuid"])
		})
	}
}

func TestReportLaunch_InvalidModeFailsBeforeReporting(t *testing.T) {
	t.Parallel()

	rec, srv := newHierarchyServer(t, "proj")
	file := writeEventLines(t, event("run", "TestOne"), event("pass", "TestOne"), event("pass", ""))
	cfg := &clientConfig{URL: srv.URL, Project: "proj", ApiKey: "tok"}

	_, _, err := reportLaunch(
		t.Context(),
		cfg,
		launchOptionsCmd(t, file, map[string]string{"mode": "DEBG"}),
	)

	require.ErrorContains(t, err, `invalid --mode "DEBG"`)
	var exitErr cli.ExitCoder
	require.True(t, errors.As(err, &exitErr))
	assert.Equal(t, 1, exitErr.ExitCode())
	assert.Nil(t, rec.launchStart, "nothing is sent for an invalid mode")
}
