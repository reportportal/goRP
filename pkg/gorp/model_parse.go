package gorp

import (
	"errors"
	"fmt"
	"strings"
)

// ErrUnknownValue is wrapped by the errors of [ParseLaunchMode], [ParseItemType] and [ParseStatus]
var ErrUnknownValue = errors.New("unknown value")

// ParseLaunchMode converts a string such as "debug" to a [LaunchMode].
// Case and surrounding spaces are ignored. An unknown value gives an error that wraps [ErrUnknownValue].
func ParseLaunchMode(s string) (LaunchMode, error) {
	return parseEnum("launch mode", s, LaunchModes.Default, LaunchModes.Debug)
}

// ParseItemType converts a string such as "before_suite" to a [TestItemType].
// Case and surrounding spaces are ignored. An unknown value gives an error that wraps [ErrUnknownValue].
func ParseItemType(s string) (TestItemType, error) {
	t := TestItemTypes
	return parseEnum("test item type", s,
		t.Suite, t.Story, t.Test, t.Scenario, t.Step,
		t.BeforeClass, t.BeforeGroups, t.BeforeMethod, t.BeforeSuite, t.BeforeTest,
		t.AfterClass, t.AfterGroups, t.AfterMethod, t.AfterSuite, t.AfterTest)
}

// ParseStatus converts a string such as "passed" to a [Status].
// Case and surrounding spaces are ignored. An unknown value gives an error that wraps [ErrUnknownValue].
func ParseStatus(s string) (Status, error) {
	st := Statuses
	return parseEnum("status", s,
		st.Passed, st.Failed, st.Stopped, st.Skipped, st.Interrupted, st.Canceled, st.Info, st.Warn)
}

func parseEnum[T ~string](kind, s string, values ...T) (T, error) {
	normalized := strings.ToUpper(strings.TrimSpace(s))
	names := make([]string, 0, len(values))
	for _, v := range values {
		if string(v) == normalized {
			return v, nil
		}
		names = append(names, string(v))
	}

	return "", fmt.Errorf(
		"%w: %s %q, expected one of %s",
		ErrUnknownValue,
		kind,
		s,
		strings.Join(names, ", "),
	)
}
