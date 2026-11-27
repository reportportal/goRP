package gorp

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseLaunchMode(t *testing.T) {
	t.Parallel()

	for in, want := range map[string]LaunchMode{
		"DEFAULT": LaunchModes.Default,
		"debug":   LaunchModes.Debug,
		" Debug ": LaunchModes.Debug,
	} {
		got, err := ParseLaunchMode(in)
		require.NoError(t, err, in)
		assert.Equal(t, want, got, in)
	}

	for _, in := range []string{"", "DEBUGGING", "de bug"} {
		got, err := ParseLaunchMode(in)
		require.ErrorIs(t, err, ErrUnknownValue, in)
		assert.Empty(t, got)
	}
	_, err := ParseLaunchMode("x")
	assert.EqualError(t, err, `unknown value: launch mode "x", expected one of DEFAULT, DEBUG`)
}

func TestParseItemType(t *testing.T) {
	t.Parallel()

	for in, want := range map[string]TestItemType{
		"suite":         TestItemTypes.Suite,
		"Story":         TestItemTypes.Story,
		"TEST":          TestItemTypes.Test,
		"scenario":      TestItemTypes.Scenario,
		"step":          TestItemTypes.Step,
		"before_class":  TestItemTypes.BeforeClass,
		"before_groups": TestItemTypes.BeforeGroups,
		"before_method": TestItemTypes.BeforeMethod,
		"before_suite":  TestItemTypes.BeforeSuite,
		"before_test":   TestItemTypes.BeforeTest,
		"after_class":   TestItemTypes.AfterClass,
		"after_groups":  TestItemTypes.AfterGroups,
		"after_method":  TestItemTypes.AfterMethod,
		"after_suite":   TestItemTypes.AfterSuite,
		"\tAFTER_TEST":  TestItemTypes.AfterTest,
	} {
		got, err := ParseItemType(in)
		require.NoError(t, err, in)
		assert.Equal(t, want, got, in)
	}

	for _, in := range []string{"", "before-suite", "BEFORESUITE", "launch"} {
		_, err := ParseItemType(in)
		require.ErrorIs(t, err, ErrUnknownValue, in)
	}
}

func TestParseStatus(t *testing.T) {
	t.Parallel()

	for in, want := range map[string]Status{
		"passed":      Statuses.Passed,
		"Failed":      Statuses.Failed,
		"STOPPED":     Statuses.Stopped,
		"skipped":     Statuses.Skipped,
		"interrupted": Statuses.Interrupted,
		"cancelled":   Statuses.Canceled, //nolint:misspell // the server's spelling
		"info":        Statuses.Info,
		"warn":        Statuses.Warn,
	} {
		got, err := ParseStatus(in)
		require.NoError(t, err, in)
		assert.Equal(t, want, got, in)
	}

	// "canceled" is not what the server calls it, IN_PROGRESS is not for clients to set
	for _, in := range []string{"", "canceled", "IN_PROGRESS", "ok"} {
		_, err := ParseStatus(in)
		require.ErrorIs(t, err, ErrUnknownValue, in)
	}
}
