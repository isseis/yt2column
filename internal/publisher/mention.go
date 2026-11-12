package publisher

import (
	"strings"
	"unicode"
)

// This file holds the mention rule: the '@' units that can start a mention,
// and the invisible character inserted after them, which the neutralized
// display Markdown and the Viewer heading use.

// zeroWidthSpace is inserted after an '@' to keep it from starting a mention.
const zeroWidthSpace = "\xe2\x80\x8b"

// replaceLookAlike returns '@' for a look-alike of it (the full-width and the
// small commercial at), and r itself for any other rune.
func replaceLookAlike(r rune) rune {
	switch r {
	case '\uFF20', '\uFE6B':
		return '@'
	default:
		return r
	}
}

// mentionAts returns the rune indices of the target '@' characters: an '@'
// at the start of text or whose preceding rune is not a letter or a digit.
// What follows the '@' does not matter, so a trailing '@' and an '@' before a
// space, punctuation, or U+200B are reported too: the Viewer drops markup when
// a reader selects and copies the rendered article, so "@**channel**" pastes
// as "@channel". text is the checked string with replaceLookAlike applied
// (look-alikes replaced one for one, nothing dropped or unescaped), so an
// index is also an index of the original string. Format characters and
// backslashes are word separators here, not stripped, so "a", U+200B, "@here"
// reports the '@'. The rule is wider than Mattermost's word splitting on purpose: it looks
// only at the rune before the '@', so it also reports "foo_@bar" and "x@@y",
// and a '@' after emphasis markup.
func mentionAts(text []rune) []int {
	var ats []int
	for i, r := range text {
		if r != '@' {
			continue
		}
		if i > 0 && (unicode.IsLetter(text[i-1]) || unicode.IsNumber(text[i-1])) {
			continue
		}
		ats = append(ats, i)
	}
	return ats
}

// mentionAtsOf applies replaceLookAlike to runes and returns mentionAts of the
// result. Because replaceLookAlike replaces one rune with one rune, an index
// is also an index of runes.
func mentionAtsOf(runes []rune) []int {
	replaced := make([]rune, len(runes))
	for i, r := range runes {
		replaced[i] = replaceLookAlike(r)
	}
	return mentionAts(replaced)
}

// neutralizeAts inserts U+200B after each target '@' unit of text and reports
// whether it inserted any. An '@' unit is a raw '@' (a look-alike included,
// see replaceLookAlike) or a character reference that can stand for '@'
// (atReferenceEnd); a unit is a target when it starts text or the rune
// before it is not a letter or a digit, whatever follows it. U+200B goes
// right after a raw '@' and right after the ';' of a reference. When
// keepURLPaths is set, a raw '@' that is a URL path '/@' (urlPathTracker) is
// left as it is.
//
// Contract: removing the inserted U+200B from the result gives text byte for
// byte, each unit gets at most one U+200B, and the time is linear in the
// length of text: one forward pass, with each rune read a bounded number of
// times, so an article full of "/@", "[", "@", and "://" cannot slow down the
// status response. The bytes of text are copied as they are, so a value that
// is not valid UTF-8 is not repaired.
func neutralizeAts(text string, keepURLPaths bool) (string, bool) {
	runes := make([]rune, 0, len(text))
	offsets := make([]int, 0, len(text)+1)
	for offset, r := range text {
		runes = append(runes, r)
		offsets = append(offsets, offset)
	}
	offsets = append(offsets, len(text))

	ats := mentionAtsOf(runes)
	nextAt := 0
	insertAfter := -1 // index of the ';' of a target reference
	var url urlPathTracker
	var b strings.Builder
	inserted := false
	for i, r := range runes {
		if keepURLPaths {
			url.step(runes, i)
		}
		insert := false
		if nextAt < len(ats) && ats[nextAt] == i {
			nextAt++
			insert = !keepURLPaths || !url.keeps(runes, i)
		}
		if r == '&' && insertAfter < i {
			if end, ok := atReferenceEnd(runes, i); ok && isUnitStart(runes, i) {
				insertAfter = end
			}
		}
		if i == insertAfter {
			insert = true
		}
		if insert && !inserted {
			b.Grow(len(text) + len(zeroWidthSpace))
			b.WriteString(text[:offsets[i+1]])
			inserted = true
		} else if inserted {
			b.WriteString(text[offsets[i]:offsets[i+1]])
		}
		if insert {
			b.WriteString(zeroWidthSpace)
		}
	}
	if !inserted {
		return text, false
	}
	return b.String(), true
}

// isUnitStart reports whether the '@' unit at index i is a target: it starts
// the text, or the rune before it is not a letter or a digit. It is the rule
// mentionAts applies to a raw '@', applied to the '&' of a reference.
func isUnitStart(runes []rune, i int) bool {
	return i == 0 || !unicode.IsLetter(runes[i-1]) && !unicode.IsNumber(runes[i-1])
}

// atReferenceEnd reports whether runes[i:] starts with a character reference
// that can stand for '@', and returns the index of its ';'. A numeric
// reference ("&#" and decimal digits, or "&#x" or "&#X" and hexadecimal
// digits, then ';') counts when its value is '@' or a look-alike of it,
// whatever the number of digits. A named reference ('&', ASCII letters and
// digits, then ';') always counts: Mattermost decodes references before it
// looks for mentions, and its table of names may change between versions, so
// the name is not looked up. The scan stops at the first rune that cannot
// continue the reference, which is never '&', so the scans of two references
// never overlap.
func atReferenceEnd(runes []rune, i int) (int, bool) {
	j := i + 1
	if j < len(runes) && runes[j] == '#' {
		j++
		base := 10
		if j < len(runes) && (runes[j] == 'x' || runes[j] == 'X') {
			base = 16
			j++
		}
		start := j
		value := 0
		for j < len(runes) {
			d, ok := digitValue(runes[j], base)
			if !ok {
				break
			}
			// Saturate above the largest rune, so a long run of digits
			// cannot overflow into the value of '@'.
			if value <= unicode.MaxRune {
				value = value*base + d
			}
			j++
		}
		if j == start || j >= len(runes) || runes[j] != ';' {
			return 0, false
		}
		return j, replaceLookAlike(rune(min(value, unicode.MaxRune+1))) == '@'
	}
	start := j
	for j < len(runes) && isASCIIAlnum(runes[j]) {
		j++
	}
	if j == start || j >= len(runes) || runes[j] != ';' {
		return 0, false
	}
	return j, true
}

// digitValue returns the value of r as a digit in base 10 or 16; a
// hexadecimal digit may be either case.
func digitValue(r rune, base int) (int, bool) {
	const digits = "0123456789abcdef"
	d := strings.IndexRune(digits[:base], unicode.ToLower(r))
	return d, d >= 0
}

func isASCIIAlnum(r rune) bool {
	return 'a' <= r && r <= 'z' || 'A' <= r && r <= 'Z' || '0' <= r && r <= '9'
}

// urlPathTracker follows, rune by rune, whether a URL that can hold a URL
// path '/@' is open at the current rune. A URL path '/@' is a raw '@' right
// after '/' (condition 1) inside a URL whose start, lowercase "http://" or
// "https://", is on the same line (condition 2) and comes after a line start,
// whitespace, a non-ASCII rune, or '(', or after a '<' that comes after one
// of those; whose host is one or more letters, digits, '.', and '-', starting
// with a letter or a digit, followed by '/' (condition 3); with no
// whitespace, '<', '>', '?', '#', or '`' from the start up to the '@'
// (condition 4); and with no '[' anywhere in the text before the start
// (condition 5). Each condition is narrower than Mattermost's autolink, so
// an '@' this keeps is one Mattermost puts inside a link and does not read as
// a mention.
type urlPathTracker struct {
	// open reports whether some URL start that meets conditions 2, 3, and 5
	// has had no rune that breaks condition 4 since.
	open bool
	// sawBracket reports whether a '[' came before the current rune.
	sawBracket bool
}

// step updates the tracker for the rune at index i, before that rune's '@'
// (if it is one) is judged. Reading the host of a URL start goes forward from
// it, but a host holds no ':', so the hosts read from two starts never
// overlap and the pass stays linear.
func (t *urlPathTracker) step(runes []rune, i int) {
	r := runes[i]
	if breaksURLPath(r) {
		t.open = false
	}
	if r == 'h' && !t.sawBracket && urlStartsAt(runes, i) {
		t.open = true
	}
	if r == '[' {
		t.sawBracket = true
	}
}

// keeps reports whether the raw '@' at index i is a URL path '/@'.
func (t *urlPathTracker) keeps(runes []rune, i int) bool {
	return t.open && i > 0 && runes[i-1] == '/'
}

// breaksURLPath reports whether r may not appear between a URL start and its
// '/@' (condition 4).
func breaksURLPath(r rune) bool {
	switch r {
	case '<', '>', '?', '#', '`':
		return true
	default:
		return unicode.IsSpace(r)
	}
}

// urlStartsAt reports whether a URL start that meets conditions 2 and 3
// begins at index i.
func urlStartsAt(runes []rune, i int) bool {
	j, ok := afterScheme(runes, i)
	if !ok || !urlStartPrecededOK(runes, i) {
		return false
	}
	host := j
	for j < len(runes) && (unicode.IsLetter(runes[j]) || unicode.IsNumber(runes[j]) || runes[j] == '.' || runes[j] == '-') {
		j++
	}
	if j == host || !unicode.IsLetter(runes[host]) && !unicode.IsNumber(runes[host]) {
		return false
	}
	return j < len(runes) && runes[j] == '/'
}

// afterScheme returns the index after "http://" or "https://" at index i.
func afterScheme(runes []rune, i int) (int, bool) {
	for _, scheme := range []string{"https://", "http://"} {
		if hasRunePrefix(runes[i:], scheme) {
			return i + len(scheme), true
		}
	}
	return 0, false
}

func hasRunePrefix(runes []rune, prefix string) bool {
	if len(runes) < len(prefix) {
		return false
	}
	for k := range len(prefix) {
		if runes[k] != rune(prefix[k]) {
			return false
		}
	}
	return true
}

// urlStartPrecededOK reports whether the rune before index i lets a URL start
// there (condition 2): a line start, whitespace, a non-ASCII rune, or '(',
// or a '<' that one of those precedes. The Viewer draws "<https://...>"
// without its '<', so a copy of the selected text puts the rune before the
// '<' right before the scheme.
func urlStartPrecededOK(runes []rune, i int) bool {
	ok := func(k int) bool {
		return k < 0 || unicode.IsSpace(runes[k]) || runes[k] > unicode.MaxASCII || runes[k] == '('
	}
	before := i - 1
	if ok(before) {
		return true
	}
	return runes[before] == '<' && ok(before-1)
}
