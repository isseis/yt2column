package publisher

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/isseis/yt2column/internal/writer"
)

// frontMatterDelimiter opens and closes the YAML front matter.
const frontMatterDelimiter = "---"

const (
	// minRuleRun is how many repeated characters make a thematic break or a
	// code fence.
	minRuleRun = 3
	// maxListMarkerDigits is the longest digit run of an ordered list marker.
	maxListMarkerDigits = 9
)

// costUnknown is the estimated_cost_usd value when the cost is not known.
const costUnknown = "unknown"

// renderFileArticle returns the content of the --out file for a: YAML front
// matter, the title heading, the channel name line when there is a channel
// name, then the body.
//
// Contract: every front matter value is a double-quoted scalar written with
// strconv.Quote, and a checked value (valid UTF-8, no control character, see
// Article.CheckPublishable) never adds a line to the front matter or changes
// the order of its keys. The file starts with the delimiter, with no BOM or
// blank line before it. The channel name line goes through
// escapeLeadingMarkdown, so the rendered line equals the channel name.
func renderFileArticle(a writer.Article) string {
	var b strings.Builder
	b.WriteString(frontMatterDelimiter + "\n")
	if a.ChannelName != "" {
		writeFrontMatterValue(&b, "channel", a.ChannelName)
	}
	writeFrontMatterValue(&b, "source", a.SourceURL)
	writeFrontMatterValue(&b, "model", a.Model)
	writeFrontMatterValue(&b, "model_version", modelVersionOrNone(a.ModelVersion))
	if a.Shorten != nil {
		writeFrontMatterValue(&b, "shorten_model", a.Shorten.Model)
		writeFrontMatterValue(&b, "shorten_model_version", modelVersionOrNone(a.Shorten.ModelVersion))
	}
	if a.Review != nil {
		writeFrontMatterValue(&b, "review_model", a.Review.Model.Model)
		writeFrontMatterValue(&b, "review_model_version", modelVersionOrNone(a.Review.Model.ModelVersion))
	}
	cost := costUnknown
	if amount, known := articleCost(a.RunUsage); known {
		cost = amount.Format(articleCostDecimals)
	}
	writeFrontMatterValue(&b, "estimated_cost_usd", cost)
	b.WriteString(frontMatterDelimiter + "\n\n")

	fmt.Fprintf(&b, "# %s\n\n", a.Title)
	if a.ChannelName != "" {
		fmt.Fprintf(&b, "%s\n\n", escapeLeadingMarkdown(a.ChannelName))
	}
	b.WriteString(a.Body)
	return b.String()
}

// writeFrontMatterValue writes one front matter line. strconv.Quote of a valid
// UTF-8 string without control characters is a YAML double-quoted scalar.
func writeFrontMatterValue(b *strings.Builder, key, value string) {
	b.WriteString(key + ": " + strconv.Quote(value) + "\n")
}

// escapeLeadingMarkdown returns line such that a Markdown renderer cannot read
// it as a block-level construct (heading, list, quote, code block, rule), and
// shows the same text as line at block level.
//
// Contract: at most one backslash is inserted, and only at the start of the
// content (or at the delimiter of an ordered-list marker, where escaping the
// digit would show a backslash). Leading spaces and tabs are dropped: a
// paragraph does not show them, but four of them would make an indented code
// block. A line that no rule matches is returned without them but otherwise
// unchanged. Inline syntax (emphasis, links, entities) is not handled; it is
// the same residual risk as the title heading.
func escapeLeadingMarkdown(line string) string {
	line = strings.TrimLeft(line, " \t")
	if line == "" {
		return line
	}
	if n := orderedListDelimiter(line); n > 0 {
		return line[:n] + `\` + line[n:]
	}
	if startsBlock(line) {
		return `\` + line
	}
	return line
}

// startsBlock reports whether a renderer reads line, which has no leading
// space, as a block-level construct or an escape that needs a backslash in
// front. An ordered list marker is handled by orderedListDelimiter.
func startsBlock(line string) bool {
	switch line[0] {
	case '\\':
		// A backslash before ASCII punctuation would swallow the next
		// character and any escape added in front of it.
		return true
	case '>':
		return true
	case '<':
		// An HTML block starts with <tag, </tag, <!--, <?, or <!X.
		return len(line) > 1 && (isASCIILetter(rune(line[1])) || strings.IndexByte("/!?", line[1]) >= 0)
	case '#':
		return isATXHeading(line)
	case '-', '+', '*', '_':
		return isListMarker(line) || isThematicBreak(line)
	case '`', '~':
		return leadingRun(line, line[0]) >= minRuleRun
	case '[':
		// A link reference definition: [label]: destination.
		return strings.Contains(line, "]:")
	}
	return false
}

// orderedListDelimiter returns the index of the '.' or ')' that ends an
// ordered list marker at the start of line (up to nine digits, then the
// delimiter, then a space, a tab, or the end of the line), or 0 when line does
// not start with one.
func orderedListDelimiter(line string) int {
	n := leadingDigits(line)
	if n == 0 || n > maxListMarkerDigits || n >= len(line) {
		return 0
	}
	if (line[n] == '.' || line[n] == ')') && endsOrSpaceAt(line, n+1) {
		return n
	}
	return 0
}

// isATXHeading reports whether line starts with one to six '#' followed by a
// space, a tab, or the end of the line.
func isATXHeading(line string) bool {
	n := leadingRun(line, '#')
	return n <= 6 && endsOrSpaceAt(line, n)
}

// isListMarker reports whether line starts with a bullet marker followed by a
// space, a tab, or the end of the line.
func isListMarker(line string) bool {
	return line[0] != '_' && endsOrSpaceAt(line, 1)
}

// isThematicBreak reports whether line holds three or more of its first
// character, optionally separated by spaces or tabs, and nothing else.
func isThematicBreak(line string) bool {
	count := 0
	for i := range len(line) {
		switch line[i] {
		case line[0]:
			count++
		case ' ', '\t':
		default:
			return false
		}
	}
	return count >= minRuleRun
}

// endsOrSpaceAt reports whether i is the end of line or line[i] is a space or
// a tab.
func endsOrSpaceAt(line string, i int) bool {
	return i >= len(line) || line[i] == ' ' || line[i] == '\t'
}

// leadingRun returns how many times c repeats at the start of line.
func leadingRun(line string, c byte) int {
	n := 0
	for n < len(line) && line[n] == c {
		n++
	}
	return n
}

// leadingDigits returns how many ASCII digits start line.
func leadingDigits(line string) int {
	n := 0
	for n < len(line) && line[n] >= '0' && line[n] <= '9' {
		n++
	}
	return n
}
