package gorp

import (
	"crypto/sha256"
	"encoding/hex"

	"github.com/reportportal/goRP/v5/pkg/openapi"
)

// Limits of the reporting API. The server measures strings the way Java does, in UTF-16 code units:
// a character outside the Basic Multilingual Plane (most emoji) counts as two. The functions of this
// file count the same way.
//
// What the server does with a longer value differs per field, see each constant.
const (
	// MaxLaunchNameLength is the longest launch name. A longer one fails the request with HTTP 400.
	MaxLaunchNameLength = 256
	// MaxItemNameLength is the longest test item name. A longer one fails the request with HTTP 400.
	MaxItemNameLength = 1024
	// MaxDescriptionLength is what the server keeps of a launch or test item description: it trims
	// the description and silently drops the rest. The description given when a launch finishes is
	// appended to the one from the start, and the limit applies to both together.
	//
	// This is the limit of the ReportPortal 6 server. The Java client still lets item descriptions
	// of up to 65536 through, which suggests that older servers kept more.
	MaxDescriptionLength = 2048
	// MaxUniqueIDLength is the longest uniqueId of a test item. A longer one fails the request with HTTP 400.
	MaxUniqueIDLength = 1024
	// MaxTestCaseIDLength is the longest testCaseId the server stores. A longer one is not an error:
	// the server keeps its first 1011 characters and appends a 32-bit hash of the rest in brackets.
	// Shorten it with [BoundedID] to decide yourself what is stored.
	MaxTestCaseIDLength = 1024
	// MaxAttributes is the largest number of attributes of a launch or a test item.
	// More fail the request with HTTP 400.
	MaxAttributes = 256
	// MaxAttributeLength is the longest attribute key or value ReportPortal agents send. It is a convention
	// of the clients (the default of the Java client). The server itself cuts longer keys and values to 512
	// UTF-16 units without an error, and may split a surrogate pair there.
	MaxAttributeLength = 128
)

const (
	// truncationMark ends a truncated string, the same one as the Java client uses
	truncationMark = "..."
	// idHashLength is the number of hex digits of the hash in a bounded ID
	idHashLength = 16
	// idHashSeparator is between the kept part of an ID and its hash
	idHashSeparator = "~"
)

// Truncate shortens s to at most limit UTF-16 code units, the unit of the server's limits, and marks
// the cut with "...". A string within the limit is returned as is. The cut never splits a character.
//
// A limit too small for the mark gives the beginning of s without it; a limit of zero or less gives "".
//
//	name = gorp.Truncate(name, gorp.MaxItemNameLength)
//
// Don't use it for identifiers: two long IDs with the same beginning become equal. See [BoundedID].
func Truncate(s string, limit int) string {
	if limit <= 0 {
		return ""
	}
	// every character takes at least one byte in UTF-8 and at most two units in UTF-16
	if len(s) <= limit || utf16Len(s) <= limit {
		return s
	}
	if limit <= len(truncationMark) {
		return utf16Prefix(s, limit)
	}

	return utf16Prefix(s, limit-len(truncationMark)) + truncationMark
}

// BoundedID shortens an identifier (testCaseId, uniqueId) to at most limit UTF-16 code units and keeps it
// unique and stable: the kept beginning is followed by "~" and a hash of the whole ID, so two long IDs
// that differ only after the cut stay different, and the same ID always gives the same result.
// An ID within the limit is returned as is.
//
//	rq.UniqueId = openapi.PtrString(gorp.BoundedID(id, gorp.MaxUniqueIDLength))
//
// A limit that leaves no room for the hash (17 units) gives the beginning of the hash alone.
func BoundedID(id string, limit int) string {
	if limit <= 0 {
		return ""
	}
	if len(id) <= limit || utf16Len(id) <= limit {
		return id
	}

	sum := sha256.Sum256([]byte(id))
	hash := hex.EncodeToString(sum[:])[:idHashLength]
	suffix := idHashSeparator + hash
	if limit <= len(suffix) {
		return hash[:min(limit, len(hash))]
	}

	return utf16Prefix(id, limit-len(suffix)) + suffix
}

// TruncateAttributes returns attributes that fit the limits: at most [MaxAttributes] of them, in the
// given order, with keys and values of at most [MaxAttributeLength]. The input is not modified.
func TruncateAttributes(attributes []openapi.ItemAttributesRQ) []openapi.ItemAttributesRQ {
	if attributes == nil {
		return nil
	}

	res := make([]openapi.ItemAttributesRQ, 0, min(len(attributes), MaxAttributes))
	for _, attr := range attributes[:min(len(attributes), MaxAttributes)] {
		if attr.Key != nil {
			attr.Key = openapi.PtrString(Truncate(*attr.Key, MaxAttributeLength))
		}
		attr.Value = Truncate(attr.Value, MaxAttributeLength)
		res = append(res, attr)
	}

	return res
}

// utf16Len is the length of s in UTF-16 code units, i.e. what String.length() gives in Java.
// Bytes that aren't valid UTF-8 count as one unit each: encoding/json sends each as U+FFFD.
func utf16Len(s string) int {
	n := 0
	for _, r := range s {
		n += utf16Width(r)
	}

	return n
}

// utf16Prefix returns the longest prefix of s that takes at most limit UTF-16 code units
// and ends at a character boundary.
func utf16Prefix(s string, limit int) string {
	n := 0
	for i, r := range s {
		n += utf16Width(r)
		if n > limit {
			return s[:i]
		}
	}

	return s
}

func utf16Width(r rune) int {
	const firstSupplementary = 0x10000
	if r >= firstSupplementary {
		return 2 //nolint:mnd // a surrogate pair
	}

	return 1
}
