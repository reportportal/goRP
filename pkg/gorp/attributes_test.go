package gorp

import (
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSystemAttributes(t *testing.T) {
	t.Parallel()

	attrs := SystemAttributes("gorp", "1.2.3")

	require.Len(t, attrs, 3)
	got := map[string]string{}
	for _, a := range attrs {
		require.NotNil(t, a.Key)
		require.NotNil(t, a.System)
		assert.True(t, *a.System, "%s must be a system attribute", *a.Key)
		got[*a.Key] = a.Value
	}
	assert.Equal(t, map[string]string{
		"agent": "gorp|1.2.3",
		"os":    runtime.GOOS + "|" + runtime.GOARCH,
		"go":    runtime.Version(),
	}, got)
}

func TestSkippedIssueAttribute(t *testing.T) {
	t.Parallel()

	for _, v := range []bool{true, false} {
		a := SkippedIssueAttribute(v)
		require.NotNil(t, a.Key)
		assert.Equal(t, "skippedIssue", *a.Key)
		assert.Equal(t, map[bool]string{true: "true", false: "false"}[v], a.Value)
		require.NotNil(t, a.System)
		assert.True(t, *a.System)
	}
}

func TestNotIssue(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "NOT_ISSUE", NotIssue().IssueType)
}
