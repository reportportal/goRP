package rpagent

import (
	"strings"
	"testing"
)

func TestBoundedName(t *testing.T) {
	short := strings.Repeat("a", maxItemNameLen)
	if boundedName(short) != short {
		t.Fatal("a name at the limit must be unchanged")
	}
	got := boundedName(strings.Repeat("a", maxItemNameLen+1))
	if utf16Len(got) > maxItemNameLen || !strings.HasSuffix(got, nameEllipsis) {
		t.Fatalf("bad cut: %d units", utf16Len(got))
	}
	// Characters outside the BMP count as two units for the server.
	emoji := boundedName(strings.Repeat("\U0001F600", 800))
	if utf16Len(emoji) > maxItemNameLen {
		t.Fatalf("emoji name is %d units, limit %d", utf16Len(emoji), maxItemNameLen)
	}
	if !strings.HasPrefix(emoji, "\U0001F600") || strings.ContainsRune(emoji, 0xFFFD) {
		t.Fatal("must not split a character")
	}
}

func TestBoundedIDStaysUniqueAndDeterministic(t *testing.T) {
	if boundedID("short") != "short" {
		t.Fatal("short IDs are unchanged")
	}
	prefix := strings.Repeat("p", maxTestCaseIDLen+50)
	a, b := boundedID(prefix+"A"), boundedID(prefix+"B")
	if len(a) > maxTestCaseIDLen || len(b) > maxTestCaseIDLen {
		t.Fatalf("too long: %d, %d", len(a), len(b))
	}
	if a == b {
		t.Fatal("IDs differing only after the cut must stay different")
	}
	if a != boundedID(prefix+"A") {
		t.Fatal("must be deterministic")
	}
}
