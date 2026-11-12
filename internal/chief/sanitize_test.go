//go:build test

package chief

import (
	"strings"
	"testing"
)

func TestSanitize(t *testing.T) {
	cases := map[string]struct {
		in, want string
	}{
		"plain text, newline, and tab are kept": {"a\tb\nc", "a\tb\nc"},
		"non-ASCII text is kept":                {"café ☐ ❯ ✔", "café ☐ ❯ ✔"},
		"escape sequence loses its ESC":         {"\x1b[2Jx", `\x1b[2Jx`},
		"other C0 control":                      {"a\rb\x00", `a\x0db\x00`},
		"DEL":                                   {"\x7f", `\x7f`},
		"C1 control":                            {"\u009b", `\u009b`},
		"format character (Cf)":                 {"a\u202eb", `a\u202eb`},
		"line and paragraph separators":         {"\u2028\u2029", `\u2028\u2029`},
		"unassigned tag-block code point":       {"\U000e0000", `\U000e0000`},
		"invalid UTF-8 byte":                    {"a\xffb", `a\xffb`},
		"backslash is doubled":                  {`a\b`, `a\\b`},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			if got := Sanitize(c.in); got != c.want {
				t.Errorf("Sanitize(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

func TestCapScreen(t *testing.T) {
	lines := func(n int) string {
		var b strings.Builder
		for i := range n {
			if i > 0 {
				b.WriteByte('\n')
			}
			b.WriteString("l")
		}
		return b.String()
	}
	cases := map[string]struct {
		in            string
		wantLines     int
		wantTruncated bool
	}{
		"at the line cap":                  {lines(screenMaxLines), screenMaxLines, false},
		"at the line cap with trailing LF": {lines(screenMaxLines) + "\n", screenMaxLines, false},
		"one line over the cap":            {lines(screenMaxLines + 1), screenMaxLines, true},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			got := CapScreen(c.in)
			if got.Truncated != c.wantTruncated {
				t.Errorf("Truncated = %v, want %v", got.Truncated, c.wantTruncated)
			}
			if n := strings.Count(strings.TrimSuffix(got.Text, "\n"), "\n") + 1; n != c.wantLines {
				t.Errorf("kept %d lines, want %d", n, c.wantLines)
			}
		})
	}

	t.Run("keeps the newest line", func(t *testing.T) {
		got := CapScreen(lines(screenMaxLines) + "\nnewest")
		if !strings.HasSuffix(got.Text, "\nnewest") || !got.Truncated {
			t.Errorf("CapScreen dropped the bottom or did not mark the cut: %v", got.Truncated)
		}
	})

	t.Run("byte cap", func(t *testing.T) {
		atCap := strings.Repeat("a", screenMaxBytes)
		if got := CapScreen(atCap); got.Truncated || got.Text != atCap {
			t.Errorf("a screen of exactly %d bytes was cut", screenMaxBytes)
		}
		// One multibyte character over the cap: the cut drops the head side
		// and never leaves a partial UTF-8 sequence.
		over := "☐" + strings.Repeat("a", screenMaxBytes-1)
		got := CapScreen(over)
		if !got.Truncated || got.Text != strings.Repeat("a", screenMaxBytes-1) {
			t.Errorf("CapScreen(over) = %d bytes, truncated %v", len(got.Text), got.Truncated)
		}
	})
}

func TestCapDescription(t *testing.T) {
	atCap := strings.Repeat("a", descriptionMaxBytes)
	if got := CapDescription(atCap); got.Truncated || got.Text != atCap {
		t.Errorf("a description of exactly %d bytes was cut", descriptionMaxBytes)
	}
	over := strings.Repeat("a", descriptionMaxBytes-1) + "☐" + "tail"
	got := CapDescription(over)
	if !got.Truncated || got.Text != strings.Repeat("a", descriptionMaxBytes-1) {
		t.Errorf("CapDescription(over) kept %d bytes, truncated %v; want the head before the split character", len(got.Text), got.Truncated)
	}
}
