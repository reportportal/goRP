package rpagent

import (
	"crypto/sha256"
	"encoding/hex"
	"unicode/utf16"
	"unicode/utf8"
)

// ReportPortal limits, measured against a live 5.x server: an item name over 1,024
// characters is rejected with HTTP 400 (the item is never created), and a testCaseId over
// 1,024 characters is silently cut, so two long IDs sharing a prefix would collide.
const (
	maxItemNameLen   = 1024
	maxTestCaseIDLen = 1024
	nameEllipsis     = "..."
	idHashLen        = 16
)

// boundedName shortens an item name to what ReportPortal accepts. The server counts
// UTF-16 code units, so that is what is measured here.
func boundedName(s string) string {
	if utf16Len(s) <= maxItemNameLen {
		return s
	}
	budget := maxItemNameLen - len(nameEllipsis)
	var n int
	for i, r := range s {
		w := 1
		if r >= 0x10000 {
			w = 2
		}
		if n+w > budget {
			return s[:i] + nameEllipsis
		}
		n += w
	}
	return s
}

// boundedID shortens a testCaseId to what ReportPortal stores, keeping it unique and
// deterministic: the cut prefix is followed by a hash of the whole ID, so two long IDs
// that differ only after the cut stay different.
func boundedID(s string) string {
	if len(s) <= maxTestCaseIDLen {
		return s
	}
	sum := sha256.Sum256([]byte(s))
	suffix := "~" + hex.EncodeToString(sum[:])[:idHashLen]
	cut := maxTestCaseIDLen - len(suffix)
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return s[:cut] + suffix
}

func utf16Len(s string) int {
	n := 0
	for _, r := range s {
		n += len(utf16.Encode([]rune{r}))
	}
	return n
}
