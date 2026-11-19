//go:build test

package chief

import (
	"slices"
	"strings"
	"testing"
)

// sanitizeCases are Sanitize's fixed expectations, which TestSanitizeUnits
// also joins its units against.
var sanitizeCases = map[string]struct {
	in, want string
}{
	"plain text, newline, and tab are kept": {"a\tb\nc", "a\tb\nc"},
	"non-ASCII text is kept":                {"caf\u00e9 \u2610 \u276f \u2714", "caf\u00e9 \u2610 \u276f \u2714"},
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

func TestSanitize(t *testing.T) {
	for name, c := range sanitizeCases {
		t.Run(name, func(t *testing.T) {
			if got := Sanitize(c.in); got != c.want {
				t.Errorf("Sanitize(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

// TestSanitizeUnits checks the unit boundaries and kinds against fixed
// expectations: joining the units back cannot find an error the units and
// Sanitize share, so the units are checked on their own. The joined subtests
// compare the units' texts with Sanitize's fixed expectations.
func TestSanitizeUnits(t *testing.T) {
	text := func(s string) displayUnit { return displayUnit{text: s, kind: unitText} }
	cases := map[string]struct {
		in   string
		want []displayUnit
	}{
		"empty":                                {"", []displayUnit{}},
		"ASCII characters":                     {"ab", []displayUnit{text("a"), text("b")}},
		"non-ASCII character":                  {"\u00e9\u3042", []displayUnit{text("\u00e9"), text("\u3042")}},
		"four-byte character":                  {"\U0001f6d1", []displayUnit{text("\U0001f6d1")}},
		"combining mark is its own":            {"e\u0301", []displayUnit{text("e"), text("\u0301")}},
		"newline and tab carry a kind":         {"a\nb\tc", []displayUnit{text("a"), {text: "\n", kind: unitNewline}, text("b"), {text: "\t", kind: unitTab}, text("c")}},
		"ESC is one escape unit":               {"\x1b[", []displayUnit{text(`\x1b`), text("[")}},
		"C1 control":                           {"\u009b", []displayUnit{text(`\u009b`)}},
		"format character":                     {"\u200b\u202e", []displayUnit{text(`\u200b`), text(`\u202e`)}},
		"tag-block code point":                 {"\U000e0041", []displayUnit{text(`\U000e0041`)}},
		"invalid UTF-8 bytes":                  {"\xff\xfe", []displayUnit{text(`\xff`), text(`\xfe`)}},
		"doubled backslash is one unit":        {`\x1b`, []displayUnit{text(`\\`), text("x"), text("1"), text("b")}},
		"CR is an escape, not a newline":       {"\r\n", []displayUnit{text(`\x0d`), {text: "\n", kind: unitNewline}}},
		"separators are escapes, not newlines": {"\u2028\u2029", []displayUnit{text(`\u2028`), text(`\u2029`)}},
		"DEL":                                  {"\x7f", []displayUnit{text(`\x7f`)}},
		"truncated multibyte sequence":         {"\xe3\x81", []displayUnit{text(`\xe3`), text(`\x81`)}},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			if got := sanitizeUnits(c.in); !slices.Equal(got, c.want) {
				t.Errorf("sanitizeUnits(%q) = %+v, want %+v", c.in, got, c.want)
			}
		})
	}
	for name, c := range sanitizeCases {
		t.Run("joined/"+name, func(t *testing.T) {
			var b strings.Builder
			for _, u := range sanitizeUnits(c.in) {
				b.WriteString(u.text)
			}
			if b.String() != c.want {
				t.Errorf("joined units of %q = %q, want %q", c.in, b.String(), c.want)
			}
		})
	}
}

func TestCapHead(t *testing.T) {
	const capBytes = 8
	cases := map[string]struct {
		in, want  string
		truncated bool
	}{
		"empty":                              {"", "", false},
		"one under the cap":                  {"abcdefg", "abcdefg", false},
		"exactly the cap":                    {"abcdefgh", "abcdefgh", false},
		"one over the cap":                   {"abcdefghi", "abcdefgh", true},
		"character ending at the cap":        {"abcde\u3042i", "abcde\u3042", true},
		"character across the cap is left":   {"abcdefg\u3042", "abcdefg", true},
		"four-byte character across the cap": {"abcdef\U0001f6d1", "abcdef", true},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			got := CapHead(c.in, capBytes)
			if got.Text != c.want || got.Truncated != c.truncated {
				t.Errorf("CapHead(%q, %d) = %+v, want {Text:%q Truncated:%v}", c.in, capBytes, got, c.want, c.truncated)
			}
		})
	}
	if got := CapHead("a", 0); got != (Capped{Truncated: true}) {
		t.Errorf("CapHead(%q, 0) = %+v, want an empty truncated head", "a", got)
	}
}
