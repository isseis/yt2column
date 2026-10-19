package publisher

import (
	"strings"

	"github.com/isseis/yt2column/internal/writer"
)

// zeroWidthSpace is inserted after an '@' to keep it from starting a mention.
const zeroWidthSpace = "\xe2\x80\x8b"

// neutralizeMentions returns value with U+200B inserted after each '@' that
// mentionAts reports for the V3-replaced value, except an '@' that directly
// follows '/': it may be part of a URL, which U+200B would break, and
// rejectMention refuses it instead. A look-alike such as U+FF20 is treated as
// '@' and keeps its own rune, followed by U+200B. A value with no such '@' is
// returned unchanged.
func neutralizeMentions(value string) string {
	runes := []rune(value)
	ats := mentionAtsOf(runes)
	if len(ats) == 0 {
		return value
	}
	var b strings.Builder
	b.Grow(len(value) + len(ats)*len(zeroWidthSpace))
	next := 0
	for i, r := range runes {
		b.WriteRune(r)
		if next < len(ats) && ats[next] == i {
			next++
			if !followsSlash(runes, i) {
				b.WriteString(zeroWidthSpace)
			}
		}
	}
	return b.String()
}

// neutralizeProse does the same for a body, but only for the '@' characters
// whose byte offset in body lies inside one of prose, the ranges outside
// code. mentionAts reports rune indices of the V3-replaced body; each is
// converted to the byte offset of the same rune in body before it is compared
// with prose, because a look-alike such as U+FF20 takes three bytes in body
// and one after replacement.
func neutralizeProse(body string, prose []writer.ByteRange) string {
	runes := []rune(body)
	ats := mentionAtsOf(runes)
	if len(ats) == 0 {
		return body
	}
	var b strings.Builder
	b.Grow(len(body) + len(ats)*len(zeroWidthSpace))
	next, offset := 0, 0
	for i, r := range runes {
		b.WriteRune(r)
		if next < len(ats) && ats[next] == i {
			next++
			if !followsSlash(runes, i) && inRanges(prose, offset) {
				b.WriteString(zeroWidthSpace)
			}
		}
		offset += len(string(r))
	}
	return b.String()
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

// followsSlash reports whether the rune before index i is '/'.
func followsSlash(runes []rune, i int) bool {
	return i > 0 && runes[i-1] == '/'
}

// inRanges reports whether offset lies in one of ranges.
func inRanges(ranges []writer.ByteRange, offset int) bool {
	for _, r := range ranges {
		if r.Start <= offset && offset < r.End {
			return true
		}
	}
	return false
}
