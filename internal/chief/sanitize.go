package chief

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Read caps for text other agents and outside services write. A screen keeps
// its last lines (the newest output and the input line are at the bottom); a
// PR description keeps its head.
const (
	screenMaxLines       = 200
	screenMaxBytes       = 64 << 10
	descriptionMaxBytes  = 64 << 10
	visibleEscapeUnicode = 0x10000
)

// Capped is text cut to a read cap. Truncated is true when part of the input
// was dropped, so the reader can say the text is incomplete.
type Capped struct {
	Text      string
	Truncated bool
}

// CapScreen keeps the last screenMaxLines lines of a pane screen and at most
// screenMaxBytes bytes of them, dropping the head side. The cut never splits
// a UTF-8 sequence. A trailing newline does not count as a line.
func CapScreen(screen string) Capped {
	out := Capped{Text: screen}
	body := strings.TrimSuffix(out.Text, "\n")
	if strings.Count(body, "\n")+1 > screenMaxLines {
		cut := len(body)
		for range screenMaxLines {
			cut = strings.LastIndexByte(body[:cut], '\n')
		}
		out.Text = out.Text[cut+1:]
		out.Truncated = true
	}
	if len(out.Text) > screenMaxBytes {
		start := len(out.Text) - screenMaxBytes
		for start < len(out.Text) && !utf8.RuneStart(out.Text[start]) {
			start++
		}
		out.Text = out.Text[start:]
		out.Truncated = true
	}
	return out
}

// CapDescription keeps the first descriptionMaxBytes bytes of a PR
// description, dropping the tail side. The cut never splits a UTF-8 sequence.
func CapDescription(description string) Capped {
	if len(description) <= descriptionMaxBytes {
		return Capped{Text: description}
	}
	end := descriptionMaxBytes
	for end > 0 && !utf8.RuneStart(description[end]) {
		end--
	}
	return Capped{Text: description[:end], Truncated: true}
}

// Sanitize returns text for showing to the human with every character that
// could rewrite the terminal or hide text replaced by a visible escape: C0
// controls other than newline and tab, DEL, C1 controls, format characters
// (Unicode category Cf, which covers bidirectional controls and zero-width
// characters), the tag block U+E0000..U+E007F, the line and paragraph
// separators, and bytes that are not valid UTF-8. A terminal escape sequence
// loses its leading ESC, so it no longer acts. Check values are built from the
// text as read, never from this output.
func Sanitize(text string) string {
	var b strings.Builder
	b.Grow(len(text))
	for i := 0; i < len(text); {
		r, size := utf8.DecodeRuneInString(text[i:])
		switch {
		case r == utf8.RuneError && size == 1:
			fmt.Fprintf(&b, `\x%02x`, text[i])
		case hiddenRune(r):
			writeEscape(&b, r)
		default:
			b.WriteString(text[i : i+size])
		}
		i += size
	}
	return b.String()
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

// writeEscape writes r in the shortest visible escape form that holds it.
func writeEscape(b *strings.Builder, r rune) {
	switch {
	case r < c1First:
		fmt.Fprintf(b, `\x%02x`, r)
	case r < visibleEscapeUnicode:
		fmt.Fprintf(b, `\u%04x`, r)
	default:
		fmt.Fprintf(b, `\U%08x`, r)
	}
}
