package gorp

import (
	"strings"
	"testing"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/reportportal/goRP/v5/pkg/openapi"
)

// javaLength is String.length() of Java, computed independently of the code under test
func javaLength(s string) int {
	return len(utf16.Encode([]rune(s)))
}

func TestTruncate(t *testing.T) {
	t.Parallel()

	const emoji = "😀" // U+1F600: 4 bytes in UTF-8, a surrogate pair (2 units) in UTF-16

	tests := []struct {
		name  string
		in    string
		limit int
		want  string
	}{
		{name: "empty", in: "", limit: 10, want: ""},
		{name: "shorter than the limit", in: "abc", limit: 10, want: "abc"},
		{name: "exactly the limit", in: "abcdefghij", limit: 10, want: "abcdefghij"},
		{name: "one over the limit", in: "abcdefghijk", limit: 10, want: "abcdefg..."},
		{name: "zero limit", in: "abc", limit: 0, want: ""},
		{name: "negative limit", in: "abc", limit: -1, want: ""},
		{
			name:  "limit of the mark size has no room for the mark",
			in:    "abcdef",
			limit: 3,
			want:  "abc",
		},
		{name: "limit one above the mark size", in: "abcdef", limit: 4, want: "a..."},
		// 3 bytes each in UTF-8 but one UTF-16 unit: byte length must not trigger a cut
		{
			name:  "BMP characters count as one",
			in:    strings.Repeat("я", 10),
			limit: 10,
			want:  strings.Repeat("я", 10),
		},
		{
			name:  "BMP characters are cut by units",
			in:    strings.Repeat("語", 11),
			limit: 10,
			want:  strings.Repeat("語", 7) + "...",
		},
		// 5 emoji = 10 units: fits
		{
			name:  "emoji count as two",
			in:    strings.Repeat(emoji, 5),
			limit: 10,
			want:  strings.Repeat(emoji, 5),
		},
		// 6 emoji = 12 units, budget 7 units: 3 emoji (6 units), the 4th would be split
		{
			name:  "a surrogate pair is not split",
			in:    strings.Repeat(emoji, 6),
			limit: 10,
			want:  strings.Repeat(emoji, 3) + "...",
		},
		{
			name:  "odd budget with mixed text",
			in:    "ab" + strings.Repeat(emoji, 5),
			limit: 10,
			want:  "ab" + emoji + emoji + "...",
		},
		{
			name:  "no room for the mark and a pair at the cut",
			in:    "a" + emoji + "b",
			limit: 2,
			want:  "a",
		},
		{
			name:  "invalid UTF-8 counts as one unit per byte",
			in:    "ab\xff\xfecdef",
			limit: 7,
			want:  "ab\xff\xfe...",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := Truncate(tt.in, tt.limit)
			assert.Equal(t, tt.want, got)
			assert.LessOrEqual(t, javaLength(got), max(tt.limit, 0))
			if utf8.ValidString(tt.in) {
				assert.True(t, utf8.ValidString(got), "the cut must not split a character")
			}
		})
	}
}

func TestTruncateAtServerLimits(t *testing.T) {
	t.Parallel()

	for _, limit := range []int{MaxLaunchNameLength, MaxItemNameLength, MaxDescriptionLength, MaxAttributeLength} {
		for _, filler := range []string{"a", "я", "😀", "a😀"} {
			long := strings.Repeat(filler, limit+1)
			got := Truncate(long, limit)
			assert.LessOrEqual(t, javaLength(got), limit)
			// at most one unit is lost to an unsplittable pair
			assert.GreaterOrEqual(t, javaLength(got), limit-1)
			assert.True(t, strings.HasSuffix(got, "..."))
			assert.True(t, utf8.ValidString(got))
			assert.True(t, strings.HasPrefix(long, strings.TrimSuffix(got, "...")))

			fits := strings.Repeat("a", limit)
			assert.Equal(t, fits, Truncate(fits, limit))
		}
	}
}

func TestBoundedID(t *testing.T) {
	t.Parallel()

	short := "pkg/path.TestName"
	assert.Equal(t, short, BoundedID(short, MaxTestCaseIDLength))
	exact := strings.Repeat("x", MaxTestCaseIDLength)
	assert.Equal(t, exact, BoundedID(exact, MaxTestCaseIDLength))
	assert.Empty(t, BoundedID("abc", 0))

	common := strings.Repeat("x", 2000)
	a := BoundedID(common+"a", MaxTestCaseIDLength)
	b := BoundedID(common+"b", MaxTestCaseIDLength)
	assert.Len(t, a, MaxTestCaseIDLength)
	assert.NotEqual(t, a, b, "IDs that differ after the cut must stay different")
	assert.Equal(t, a, BoundedID(common+"a", MaxTestCaseIDLength), "the result must be stable")
	assert.Equal(t, strings.Repeat("x", 1007), a[:1007])
	assert.Regexp(t, `^~[0-9a-f]{16}$`, a[1007:])
}

// knownHashOf30A is the beginning of `printf %s aaaaaaaaaaaaaaaaaaaaaaaaaaaaaa | shasum -a 256`
const knownHashOf30A = "3a54fc0cbc0b0ef4"

// The result is a contract: a change would break the history of every test with a long ID.
func TestBoundedIDKnownValue(t *testing.T) {
	t.Parallel()

	got := BoundedID(strings.Repeat("a", 30), 20)
	require.Len(t, got, 20)
	assert.Equal(t, "aaa~"+knownHashOf30A, got)
}

func TestBoundedIDCountsUTF16(t *testing.T) {
	t.Parallel()

	// 600 emoji are 2400 bytes but 1200 units: over the limit of 1024
	id := strings.Repeat("😀", 600)
	got := BoundedID(id, MaxUniqueIDLength)
	// budget 1024-17 = 1007 units: 503 emoji (1006 units), the next one would be split
	assert.Equal(t, strings.Repeat("😀", 503), got[:strings.Index(got, "~")])
	assert.Equal(t, 1006+17, javaLength(got))
	assert.True(t, utf8.ValidString(got))

	// 1000 three-byte characters are 3000 bytes but 1000 units: within the limit, untouched
	bmp := strings.Repeat("語", 1000)
	assert.Equal(t, bmp, BoundedID(bmp, MaxUniqueIDLength))
}

func TestBoundedIDTinyLimit(t *testing.T) {
	t.Parallel()

	id := strings.Repeat("a", 30)
	assert.Equal(t, knownHashOf30A[:5], BoundedID(id, 5))
	assert.Equal(t, knownHashOf30A, BoundedID(id, 16))
	assert.Equal(t, knownHashOf30A, BoundedID(id, 17))
	assert.Equal(t, "a~"+knownHashOf30A, BoundedID(id, 18))
}

func TestTruncateAttributes(t *testing.T) {
	t.Parallel()

	assert.Nil(t, TruncateAttributes(nil))
	assert.Empty(t, TruncateAttributes([]openapi.ItemAttributesRQ{}))

	longKey := strings.Repeat("k", 200)
	longValue := strings.Repeat("😀", 100) // 200 units
	in := []openapi.ItemAttributesRQ{
		{Key: openapi.PtrString("os"), Value: "linux"},
		{Value: "no-key", System: openapi.PtrBool(true)},
		{Key: openapi.PtrString(longKey), Value: longValue},
	}
	got := TruncateAttributes(in)

	require.Len(t, got, 3)
	assert.Equal(t, in[0], got[0])
	assert.Equal(t, in[1], got[1])
	assert.Equal(t, strings.Repeat("k", 125)+"...", *got[2].Key)
	// budget 125 units: 62 emoji (124 units)
	assert.Equal(t, strings.Repeat("😀", 62)+"...", got[2].Value)
	// the input stays as it was
	assert.Equal(t, longKey, *in[2].Key)
	assert.Equal(t, longValue, in[2].Value)

	many := make([]openapi.ItemAttributesRQ, MaxAttributes+10)
	for i := range many {
		many[i].Value = strings.Repeat("v", i+1)
	}
	got = TruncateAttributes(many)
	require.Len(t, got, MaxAttributes)
	assert.Equal(t, "v", got[0].Value)
	assert.Equal(t, strings.Repeat("v", 125)+"...", got[MaxAttributes-1].Value)
}
