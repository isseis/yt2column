//go:build test

package publisher

import (
	"testing"

	"github.com/isseis/yt2column/internal/writer"
)

func TestNeutralizeMentions(t *testing.T) {
	const z = zeroWidthSpace
	cases := []struct {
		name  string
		value string
		want  string
	}{
		{"expected value written with a literal U+200B", "hi @name", "hi @\u200bname"},
		{"mention", "hi @name", "hi @" + z + "name"},
		{"two mentions", "@a @b", "@" + z + "a @" + z + "b"},
		{"full-width at", "＠everyone", "＠" + z + "everyone"},
		{"small at", "﹫here", "﹫" + z + "here"},
		{"after underscore", "foo_@bar", "foo_@" + z + "bar"},
		{"unchanged when no at is a target", "Tech@Work a @ b @!", "Tech@Work a @ b @!"},
		{"unchanged after a slash", "youtube.com/@name", "youtube.com/@name"},
		{"slash exception only for that at", "/@a @b", "/@a @" + z + "b"},
		{"empty", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := neutralizeMentions(tc.value)
			if got != tc.want {
				t.Errorf("neutralizeMentions(%q) = %q, want %q", tc.value, got, tc.want)
			}
			if again := neutralizeMentions(got); again != got {
				t.Errorf("neutralizeMentions is not idempotent: %q then %q", got, again)
			}
		})
	}
}

func TestNeutralizeProse(t *testing.T) {
	const z = zeroWidthSpace
	cases := []struct {
		name string
		body string
		want string
	}{
		{"prose only", "see @name now", "see @" + z + "name now"},
		{"inside a code span", "see `@name` now", "see `@name` now"},
		{"inside and outside a code span", "`@a` and @b", "`@a` and @" + z + "b"},
		{"inside a fence", "x\n```\n@a\n```\n@b", "x\n```\n@a\n```\n@" + z + "b"},
		// A full-width at takes three bytes, so a rune index differs from the
		// byte offset after it. Comparing the rune index with the ranges would
		// move the code span's edge by two bytes per full-width at.
		{"full-width at before a code span", "＠＠ `＠a` ＠b", "＠＠ `＠a` ＠" + z + "b"},
		{"full-width at in a code span", "`＠a` ＠b ＠c", "`＠a` ＠" + z + "b ＠" + z + "c"},
		{"no target", "Tech@Work plain", "Tech@Work plain"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			prose, ok := writer.ProseRanges(tc.body)
			if !ok {
				t.Fatalf("ProseRanges(%q) could not judge the body", tc.body)
			}
			if got := neutralizeProse(tc.body, prose); got != tc.want {
				t.Errorf("neutralizeProse(%q) = %q, want %q", tc.body, got, tc.want)
			}
		})
	}
}

func TestNeutralizeProseGivenRanges(t *testing.T) {
	const body = "@a @b"
	cases := []struct {
		name  string
		prose []writer.ByteRange
		want  string
	}{
		{"no ranges means all code", nil, body},
		{"range over the first at only", []writer.ByteRange{{Start: 0, End: 2}}, "@" + zeroWidthSpace + "a @b"},
		{"range ending at the at", []writer.ByteRange{{Start: 0, End: 3}}, "@" + zeroWidthSpace + "a @b"},
		{"range covering both", []writer.ByteRange{{Start: 0, End: 5}}, "@" + zeroWidthSpace + "a @" + zeroWidthSpace + "b"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := neutralizeProse(body, tc.prose); got != tc.want {
				t.Errorf("neutralizeProse(%q, %v) = %q, want %q", body, tc.prose, got, tc.want)
			}
		})
	}
}
