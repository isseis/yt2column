package publisher

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// This file holds the mention rule the destinations share: the '@' characters
// that can start a mention, and the invisible character inserted after them. It
// depends on no destination, so a renderer for one destination does not carry
// what only another needs.

// zeroWidthSpace is inserted after an '@' to keep it from starting a mention.
const zeroWidthSpace = "\xe2\x80\x8b"

func replaceLookAlike(r rune) rune {
	switch r {
	case '\uFF20', '\uFE6B':
		return '@'
	case '\uFF1C', '\uFE64':
		return '<'
	case '\uFF01', '\uFE57':
		return '!'
	default:
		return r
	}
}

// mentionAts returns the rune indices of the '@' characters that may start a
// mention: a name character follows the '@', and the rune before it is not a
// letter or a digit. text is the V3-replaced form of the checked string (look-
// alikes replaced one for one, nothing dropped or unescaped), so an index is
// also an index of the original string. Format characters and backslashes
// are word separators here, not stripped, so "a", U+200B, "@here" reports the
// '@' and "@", U+200B, "name" does not. The rule is wider than Mattermost's
// word splitting on purpose: it looks only at the rune before the '@', so it
// also reports "foo_@bar" and "x@@y", and a '@' after emphasis markup.
func mentionAts(text []rune) []int {
	var ats []int
	for i := 0; i+1 < len(text); i++ {
		if text[i] != '@' || !isMentionNameChar(text[i+1]) {
			continue
		}
		if i > 0 && (unicode.IsLetter(text[i-1]) || unicode.IsNumber(text[i-1])) {
			continue
		}
		ats = append(ats, i)
	}
	return ats
}

// mentionAtsOf applies the V3 replacement to runes and returns mentionAts of
// the result. Because V3 replaces one rune with one rune, an index is also an
// index of runes.
func mentionAtsOf(runes []rune) []int {
	replaced := make([]rune, len(runes))
	for i, r := range runes {
		replaced[i] = replaceLookAlike(r)
	}
	return mentionAts(replaced)
}

// isMentionNameChar reports whether r can follow '@' in a mention. '.' and '-'
// are included although Mattermost user names cannot start with them, to err
// on the side of reporting.
func isMentionNameChar(r rune) bool {
	return r == '_' || r == '.' || r == '-' || unicode.IsLetter(r) || unicode.IsNumber(r)
}

// insertAfterAts inserts U+200B after each target '@' of value that does not
// follow '/' and whose byte offset in value satisfies inScope.
func insertAfterAts(value string, inScope func(offset int) bool) string {
	runes := []rune(value)
	ats := mentionAtsOf(runes)
	if len(ats) == 0 {
		return value
	}
	var b strings.Builder
	b.Grow(len(value) + len(ats)*len(zeroWidthSpace))
	next, offset := 0, 0
	for i, r := range runes {
		b.WriteRune(r)
		if next < len(ats) && ats[next] == i {
			next++
			if !followsSlash(runes, i) && inScope(offset) {
				b.WriteString(zeroWidthSpace)
			}
		}
		offset += utf8.RuneLen(r)
	}
	return b.String()
}

// followsSlash reports whether the rune before index i is '/'.
func followsSlash(runes []rune, i int) bool {
	return i > 0 && runes[i-1] == '/'
}
