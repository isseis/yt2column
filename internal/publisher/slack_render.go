package publisher

import (
	"strings"
	"unicode/utf8"

	"github.com/isseis/yt2column/internal/writer"
)

// zeroWidthSpace is inserted after an '@' to keep it from starting a mention.
const zeroWidthSpace = "\xe2\x80\x8b"

// slackPost is the full text of a post.
type slackPost struct {
	text string
	// keepTogetherFrom is the rune index in text from which the post must not
	// be cut: the last line of the body and the footer stay in one message.
	// It lies in [0, rune count of text]; the rune count means no such region.
	keepTogetherFrom int
}

// neutralizeMentions returns value with U+200B inserted after each '@' that
// mentionAts reports for the V3-replaced value, except an '@' that directly
// follows '/': it may be part of a URL, which U+200B would break, and
// rejectMention refuses it instead. A look-alike such as U+FF20 is treated as
// '@' and keeps its own rune, followed by U+200B. A value with no such '@' is
// returned unchanged. value must be valid UTF-8.
func neutralizeMentions(value string) string {
	return insertAfterAts(value, func(int) bool { return true })
}

// neutralizeProse does the same for a body, but only for the '@' characters
// whose byte offset in body lies inside one of prose, the ranges outside
// code. mentionAts reports rune indices of the V3-replaced body; each is
// converted to the byte offset of the same rune in body before it is compared
// with prose, because a look-alike such as U+FF20 takes three bytes in body
// and one after replacement. body must be valid UTF-8. With ProseRanges
// reporting false there is no range to pass: the caller leaves the body as it
// is, and rejectMention refuses its targets.
func neutralizeProse(body string, prose []writer.ByteRange) string {
	return insertAfterAts(body, func(offset int) bool { return inRanges(prose, offset) })
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

// inRanges reports whether offset lies in one of ranges.
func inRanges(ranges []writer.ByteRange, offset int) bool {
	for _, r := range ranges {
		if r.Start <= offset && offset < r.End {
			return true
		}
	}
	return false
}
