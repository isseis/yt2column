//go:build test

package writer

import (
	"strings"
	"testing"
)

// TestBodyCharLimits pins the relationship the shorten step relies on: the
// target stays below the limit so the review step's paraphrasing has room.
func TestBodyCharLimits(t *testing.T) {
	if targetBodyChars >= maxBodyChars {
		t.Errorf("targetBodyChars = %d, want less than maxBodyChars = %d", targetBodyChars, maxBodyChars)
	}
}

func TestBodyChars(t *testing.T) {
	cases := []struct {
		name string
		body string
		want int
	}{
		{"empty", "", 0},
		{"at the limit", strings.Repeat("a", maxBodyChars), maxBodyChars},
		{"one over the limit", strings.Repeat("a", maxBodyChars+1), maxBodyChars + 1},
		// Counting the line feed would make the body 4,001 characters; the
		// body length rule does not count line feeds, so it is exactly at the
		// limit.
		{"line feed not counted", strings.Repeat("a", maxBodyChars) + "\n", maxBodyChars},
		{"many line feeds not counted", strings.Repeat("a", maxBodyChars) + strings.Repeat("\n", 12), maxBodyChars},
		{"markdown markers and spaces count", "## x\n** y **", 11},
		{"multi-byte characters count as one each", "\u672c\u6587", 2},
		{"surrogate pair counts as one character", "\U0001F600", 1},
		{"surrogate pair with a line feed", "\U0001F600\n\U0001F600", 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := bodyChars(tc.body); got != tc.want {
				t.Errorf("bodyChars(%q) = %d, want %d", tc.body, got, tc.want)
			}
		})
	}
}
