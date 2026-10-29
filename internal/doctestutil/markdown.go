//go:build test

package doctestutil

import (
	"slices"
	"strings"
	"unicode"
)

// AnchorFromHeading returns the anchor GitHub derives from a heading line:
// lowercase, with markup punctuation removed, each ASCII space turned into a
// hyphen, and letters, numbers, marks, connector punctuation, and hyphens kept.
// Any other character, a non-ASCII space included, is dropped.
func AnchorFromHeading(heading string) string {
	text := strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(heading), "#"))
	var b strings.Builder
	for _, r := range strings.ToLower(text) {
		switch {
		case r == '-' || unicode.IsLetter(r) || unicode.IsNumber(r) || unicode.IsMark(r) || unicode.In(r, unicode.Pc):
			b.WriteRune(r)
		case r == ' ':
			b.WriteRune('-')
		}
	}
	return b.String()
}

// HeadingsOutsideBlocks returns the heading lines of doc outside fenced code
// blocks, in order.
func HeadingsOutsideBlocks(doc string) []string {
	var headings []string
	for _, line := range ScanDocLines(slices.Collect(strings.Lines(doc))) {
		if !line.Block && HeadingLevel(line.Text) > 0 {
			headings = append(headings, line.Text)
		}
	}
	return headings
}

// SectionOutsideBlocks is SectionByHeading with fenced code blocks skipped when
// looking for headings: a '#' or '##' line inside a block neither ends the
// section nor starts one.
func SectionOutsideBlocks(doc, heading string) string {
	scanned := ScanDocLines(slices.Collect(strings.Lines(doc)))
	want := strings.TrimSpace(heading)
	for i, line := range scanned {
		if line.Block || strings.TrimSpace(line.Text) != want {
			continue
		}
		level := HeadingLevel(line.Text)
		var b strings.Builder
		for _, rest := range scanned[i+1:] {
			if l := HeadingLevel(rest.Text); !rest.Block && l > 0 && l <= level {
				break
			}
			b.WriteString(rest.Text)
			b.WriteString("\n")
		}
		return b.String()
	}
	return ""
}

// HeadingLevel reports the level of a Markdown heading line: the number of
// leading '#' characters when the line starts with one or more '#' followed by
// a space or tab. It returns 0 for any other line, so a line such as
// "#109 note" (no whitespace after '#') is not a heading.
func HeadingLevel(line string) int {
	line = strings.TrimRight(line, "\n")
	n := 0
	for n < len(line) && line[n] == '#' {
		n++
	}
	if n == 0 || n == len(line) {
		return 0
	}
	switch line[n] {
	case ' ', '\t':
		return n
	}
	return 0
}

// SectionByHeading returns the section of doc that starts at the line equal to
// heading, up to the next heading of the same or a higher level. The matching
// heading line is not included; it returns the empty string when no line
// matches. The caller passes the whole heading line (for example
// "## Heading"), so the helper works for any level.
func SectionByHeading(doc, heading string) string {
	want := strings.TrimSpace(heading)
	level := 0
	inSection := false
	var b strings.Builder
	for line := range strings.Lines(doc) {
		if l := HeadingLevel(line); l > 0 {
			if !inSection {
				if strings.TrimSpace(line) == want {
					inSection = true
					level = l
				}
				continue
			}
			if l <= level {
				break
			}
		}
		if inSection {
			b.WriteString(line)
		}
	}
	return b.String()
}

// DocLine is one line of a Markdown document as the code-fence scan classifies
// it: the text without its newline, whether the line is a fence, and whether
// the line is part of a fenced code block. An opening fence, the lines it
// encloses, and its closing fence are all part of the block.
type DocLine struct {
	Text  string
	Fence bool
	Block bool
}

// ScanDocLines classifies lines, starting outside a code block. Passing only
// the lines after a heading therefore scans them from their first line: a
// fence opened earlier in the document does not change the result. An opening
// fence may carry an info string; a closing fence is backticks alone, so a
// line such as "```not-closed" inside a block stays part of the block.
func ScanDocLines(lines []string) []DocLine {
	scanned := make([]DocLine, 0, len(lines))
	inBlock := false
	for _, line := range lines {
		text := strings.TrimSuffix(line, "\n")
		trimmed := strings.TrimSpace(text)
		fence := false
		switch {
		case !inBlock && strings.HasPrefix(trimmed, "```"):
			fence, inBlock = true, true
		case inBlock && isClosingFence(trimmed):
			fence, inBlock = true, false
		}
		scanned = append(scanned, DocLine{Text: text, Fence: fence, Block: fence || inBlock})
	}
	return scanned
}

// isClosingFence reports whether line is a closing code fence: three or more
// backticks and nothing else.
func isClosingFence(line string) bool {
	return len(line) >= 3 && strings.Trim(line, "`") == ""
}

// FencedBlock is one fenced code block: the info string after its opening
// fence's backticks, its content lines without the fence lines, and whether a
// closing fence was found.
type FencedBlock struct {
	Info   string
	Lines  []string
	Closed bool
}

// FencedBlocks returns the fenced code blocks of doc, in order. A block with no
// closing fence runs to the end of doc and is returned with Closed false.
func FencedBlocks(doc string) []FencedBlock {
	var blocks []FencedBlock
	var current *FencedBlock
	for _, line := range ScanDocLines(slices.Collect(strings.Lines(doc))) {
		switch {
		case line.Fence && current == nil:
			current = &FencedBlock{Info: strings.TrimPrefix(strings.TrimSpace(line.Text), "```")}
		case line.Fence:
			current.Closed = true
			blocks = append(blocks, *current)
			current = nil
		case current != nil:
			current.Lines = append(current.Lines, line.Text)
		}
	}
	if current != nil {
		blocks = append(blocks, *current)
	}
	return blocks
}
