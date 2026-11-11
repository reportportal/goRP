package rpagent

import (
	"reflect"
	"testing"

	"github.com/onsi/ginkgo/v2/types"
)

func TestExtractParametersFromEntryName(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want []*Parameter
	}{
		{
			"two params",
			`input='hello' expected='HELLO'`,
			[]*Parameter{{"input", "hello"}, {"expected", "HELLO"}},
		},
		{"double quotes", `n="5"`, []*Parameter{{"n", "5"}}},
		{"no params", "uppercase world", nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := ExtractParametersFromEntryName(c.in)
			if len(got) == 0 && len(c.want) == 0 {
				return
			}
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("got %v, want %v", got, c.want)
			}
		})
	}
}

func TestIsDescribeTableEntry(t *testing.T) {
	cases := map[string]bool{
		`input='a' expected='b'`: true,
		`n="5"`:                  true,
		"plain spec name":        false,
		"a = b":                  false, // '=' without a quote
		`quoted 'only'`:          false, // quote without '='
	}
	for text, want := range cases {
		if got := IsDescribeTableEntry(types.SpecReport{LeafNodeText: text}); got != want {
			t.Errorf("IsDescribeTableEntry(%q) = %v, want %v", text, got, want)
		}
	}
}
