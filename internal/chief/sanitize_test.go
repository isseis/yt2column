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

func TestCapHead(t *testing.T) {
	atCap := strings.Repeat("a", headMaxBytes)
	if got := CapHead(atCap); got.Truncated || got.Text != atCap {
		t.Errorf("a text of exactly %d bytes was cut", headMaxBytes)
	}
	over := strings.Repeat("a", headMaxBytes-1) + "☐" + "tail"
	got := CapHead(over)
	if !got.Truncated || got.Text != strings.Repeat("a", headMaxBytes-1) {
		t.Errorf("CapHead(over) kept %d bytes, truncated %v; want the head before the split character", len(got.Text), got.Truncated)
	}
}
