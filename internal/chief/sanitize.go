package chief

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Read caps for text other agents and outside services write. A screen over
// screenMaxLines lines or screenMaxBytes bytes of text is refused whole
// (parseANSIScreen); a PR description and a report read from a transcript
// keep their head (CapHead).
const (
	screenMaxLines       = 200
	screenMaxBytes       = 64 << 10
	headMaxBytes         = 64 << 10
	visibleEscapeUnicode = 0x10000
)

// Capped is text cut to a read cap. Truncated is true when part of the input
// was dropped, so the reader can say the text is incomplete.
type Capped struct {
	Text      string
	Truncated bool
}

// CapHead keeps the first maxBytes bytes of text written by another agent or
// service, such as a PR description or a pane's last report read from its
// transcript (headMaxBytes), dropping the tail side. The cut never splits a
// UTF-8 sequence, so the head may be a few bytes shorter than maxBytes.
// maxBytes must not be negative.
func CapHead(text string, maxBytes int) Capped {
	if len(text) <= maxBytes {
		return Capped{Text: text}
	}
	end := maxBytes
	for end > 0 && !utf8.RuneStart(text[end]) {
		end--
	}
	return Capped{Text: text[:end], Truncated: true}
}

// Sanitize returns text for showing to the human with every character that
// could rewrite the terminal or hide text replaced by a visible escape: C0
// controls other than newline and tab, DEL, C1 controls, format characters
// (Unicode category Cf, which covers bidirectional controls and zero-width
// characters), the tag block U+E0000..U+E007F, the line and paragraph
// separators, and bytes that are not valid UTF-8. A backslash is doubled, so
// an escape cannot be told apart from text that spells one. A terminal escape sequence
// loses its leading ESC, so it no longer acts. Check values are built from the
// text as read, never from this output.
func Sanitize(text string) string {
	var b strings.Builder
	b.Grow(len(text))
	for _, u := range sanitizeUnits(text) {
		b.WriteString(u.text)
	}
	return b.String()
}

// unitKind declares what a display unit is, so the status table chooses how
// to show a unit by this value and never by reading its text. The zero value
// is no kind at all: a unit that carries it was not made by sanitizeUnits.
type unitKind int

const (
	// unitText is a character Sanitize keeps, or one escape it writes
	// (the doubled backslash included).
	unitText unitKind = iota + 1
	// unitNewline is a newline, which Sanitize keeps.
	unitNewline
	// unitTab is a tab, which Sanitize keeps.
	unitTab
)

// displayUnit is one unit of Sanitize's output: one escape, or one character
// Sanitize keeps. The kind field declares a newline and a tab, so the status table maps
// them to a space by the kind. Only sanitizeUnits makes one.
type displayUnit struct {
	text string
	kind unitKind
}

// sanitizeUnits splits text by Sanitize's rule into display units, in order:
// each escape (an invalid byte, a hidden character, the doubled backslash) is
// one unit, and every other character is one unit, a newline and a tab
// included, each with its kind. Sanitize(text) is the units' texts joined, so
// the rule is written once.
func sanitizeUnits(text string) []displayUnit {
	units := make([]displayUnit, 0, utf8.RuneCountInString(text))
	for i := 0; i < len(text); {
		r, size := utf8.DecodeRuneInString(text[i:])
		var u displayUnit
		switch {
		case r == utf8.RuneError && size == 1:
			u = displayUnit{text: fmt.Sprintf(`\x%02x`, text[i]), kind: unitText}
		case r == '\\':
			u = displayUnit{text: `\\`, kind: unitText}
		case r == '\n':
			u = displayUnit{text: "\n", kind: unitNewline}
		case r == '\t':
			u = displayUnit{text: "\t", kind: unitTab}
		case hiddenRune(r):
			u = displayUnit{text: escapeOf(r), kind: unitText}
		default:
			u = displayUnit{text: text[i : i+size], kind: unitText}
		}
		units = append(units, u)
		i += size
	}
	return units
}

// Code points Sanitize replaces besides the format characters.
const (
	c0End              = 0x20
	del                = 0x7f
	c1First            = 0x80
	c1Last             = 0x9f
	lineSeparator      = 0x2028
	paragraphSeparator = 0x2029
	tagFirst           = 0xe0000
	tagLast            = 0xe007f
)

// hiddenRune reports whether r is a character Sanitize replaces.
func hiddenRune(r rune) bool {
	switch {
	case r == '\n' || r == '\t':
		return false
	case r < c0End || r == del:
		return true
	case r >= c1First && r <= c1Last:
		return true
	case r == lineSeparator || r == paragraphSeparator:
		return true
	case r >= tagFirst && r <= tagLast:
		return true
	default:
		return unicode.Is(unicode.Cf, r)
	}
}

// escapeOf returns r in the shortest visible escape form that holds it.
func escapeOf(r rune) string {
	switch {
	case r < c1First:
		return fmt.Sprintf(`\x%02x`, r)
	case r < visibleEscapeUnicode:
		return fmt.Sprintf(`\u%04x`, r)
	default:
		return fmt.Sprintf(`\U%08x`, r)
	}
}
