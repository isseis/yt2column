//go:build test

package doctestutil

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"unicode"
)

// ErrUnclosedFence reports a fenced code block with no closing fence.
var ErrUnclosedFence = errors.New("unclosed fenced code block")

// ErrUnsupportedFence reports a fence the README does not use: indented,
// opened with tildes, longer than three backticks, or with a backtick in its
// info string.
var ErrUnsupportedFence = errors.New("unsupported code fence")

// ErrUnsupportedHeading reports a Setext heading, or a line of seven or more
// '#' outside a code block.
var ErrUnsupportedHeading = errors.New("unsupported heading")

// HeadingPos is one ATX heading outside a fenced code block.
type HeadingPos struct {
	Line  int // 1-based line number
	Level int // 1-6
}

// Structure is the shape that README.md and README.en.md must share. Two
// Structures match when their heading levels, in order, and their code block
// counts are equal; the line numbers are only for the failure message.
type Structure struct {
	Headings   []HeadingPos
	CodeBlocks int
}

// maxFenceBackticks is the longest opening fence the README may use. A longer
// one can hide a closing fence from the scan: GitHub closes a block only with a
// run at least as long as the opening one.
const maxFenceBackticks = 3

// maxHeadingIndent is the most leading spaces GitHub allows on an ATX heading;
// four or more start an indented code block instead.
const maxHeadingIndent = 3

// maxHeadingLevel is the deepest ATX heading: seven or more '#' are not one.
const maxHeadingLevel = 6

// codeBlockIndent is the number of leading spaces that start an indented code
// block, and the width a tab counts as where indentation is measured.
const codeBlockIndent = 4

// maxOrderedMarkerDigits is the most digits an ordered list marker may have.
const maxOrderedMarkerDigits = 9

// listHeading matches a list item whose content begins with an ATX heading:
// one or more bullet or ordered markers, then the heading. GitHub renders it
// as a heading inside the item, which HeadingLevel does not see.
var listHeading = regexp.MustCompile(`^[ \t]*(?:(?:[-*+]|[0-9]{1,9}[.)])[ \t]+)+#{1,6}(?:[ \t]|$)`)

// ParseStructure extracts the Structure of doc, or returns an error wrapping
// one of ErrUnclosedFence, ErrUnsupportedFence, and ErrUnsupportedHeading.
//
// The scan is ScanDocLines, unchanged. ParseStructure rejects the forms where
// that scan and GitHub's Markdown can read the document differently, instead of
// counting them under one reading: the README uses none of them, and a silent
// choice could call two versions equal when GitHub renders them differently.
// Two of those forms depend on the surrounding lines rather than on the line
// alone: the content indentation of the list item a heading is in, and the
// paragraph an indented line continues instead of interrupting.
func ParseStructure(doc string) (Structure, error) {
	var structure Structure
	inBlock, textBefore, openLine := false, false, 0
	var listIndents []int
	for i, line := range ScanDocLines(slices.Collect(strings.Lines(doc))) {
		lineNo := i + 1
		if line.Fence {
			if err := checkFence(line.Text, lineNo, inBlock); err != nil {
				return Structure{}, err
			}
			// A fence outside a list item ends the item, so a heading indented
			// below the fence does not belong to it.
			listIndents = updateListIndents(listIndents, line.Text)
			if inBlock {
				inBlock = false
			} else {
				inBlock, openLine = true, lineNo
				structure.CodeBlocks++
			}
			textBefore = false
			continue
		}
		if line.Block {
			textBefore = false
			continue
		}
		trimmed := strings.TrimSpace(line.Text)
		if strings.HasPrefix(trimmed, "~~~") {
			return Structure{}, fmt.Errorf("line %d: %w: tilde fence", lineNo, ErrUnsupportedFence)
		}
		if strings.HasPrefix(line.Text, "#######") {
			return Structure{}, fmt.Errorf("line %d: %w: seven or more '#'", lineNo, ErrUnsupportedHeading)
		}
		// A list item ends at a line indented less than its content, and stays
		// open across a blank line. GitHub reads a heading indented into an open
		// item as the item's heading, not as an indented code block.
		listIndents = updateListIndents(listIndents, line.Text)
		if level := HeadingLevel(line.Text); level > 0 {
			structure.Headings = append(structure.Headings, HeadingPos{Line: lineNo, Level: level})
			textBefore = false
			continue
		}
		if githubHeadingLevel(line.Text) > 0 || listHeading.MatchString(line.Text) || headingInListItem(line.Text, innermostIndent(listIndents)) > 0 {
			return Structure{}, fmt.Errorf("line %d: %w: a heading HeadingLevel does not count", lineNo, ErrUnsupportedHeading)
		}
		if isSetextUnderline(trimmed) {
			if textBefore {
				return Structure{}, fmt.Errorf("line %d: %w: Setext underline", lineNo, ErrUnsupportedHeading)
			}
			textBefore = false
			continue
		}
		textBefore = startsParagraph(line.Text) || (textBefore && isParagraphContinuation(line.Text))
	}
	if inBlock {
		return Structure{}, fmt.Errorf("line %d: %w", openLine, ErrUnclosedFence)
	}
	return structure, nil
}

// checkFence rejects a fence line that the shared scan and GitHub could read
// differently. For an opening fence it is an indented fence, a run of four or
// more backticks, or an info string with a backtick. For a closing fence it is
// an indented fence: a run of four or more backticks closes a three-backtick
// block the same way in both readings, so it is accepted.
func checkFence(text string, lineNo int, closing bool) error {
	if strings.TrimLeftFunc(text, unicode.IsSpace) != text {
		return fmt.Errorf("line %d: %w: indented fence", lineNo, ErrUnsupportedFence)
	}
	if closing {
		return nil
	}
	info := strings.TrimLeft(text, "`")
	if run := len(text) - len(info); run > maxFenceBackticks {
		return fmt.Errorf("line %d: %w: %d backticks", lineNo, ErrUnsupportedFence, run)
	}
	if strings.Contains(info, "`") {
		return fmt.Errorf("line %d: %w: backtick in the info string", lineNo, ErrUnsupportedFence)
	}
	return nil
}

// githubHeadingLevel reports the level GitHub gives an ATX heading line, or 0.
// Unlike HeadingLevel it accepts up to three leading spaces and a line of
// hashes with no text; both are headings GitHub renders and HeadingLevel does
// not count, so ParseStructure rejects them instead of counting them as text.
func githubHeadingLevel(line string) int {
	return atxHeadingLevel(lineIndent(line), strings.TrimLeft(line, " \t"))
}

// headingInListItem reports the level GitHub gives an ATX heading on line when
// the line is in a list item whose content starts at column contentIndent, or 0.
// GitHub strips the item's indentation before reading the line, so a heading
// below four or more spaces is the item's heading, not an indented code block.
func headingInListItem(line string, contentIndent int) int {
	if contentIndent <= 0 {
		return 0
	}
	indent := lineIndent(line)
	if indent < contentIndent {
		return 0
	}
	return atxHeadingLevel(indent-contentIndent, strings.TrimLeft(line, " \t"))
}

// atxHeadingLevel reports the level of an ATX heading whose indentation is
// indent columns and whose remaining text is rest, or 0 for anything else.
func atxHeadingLevel(indent int, rest string) int {
	if indent > maxHeadingIndent {
		return 0
	}
	n := 0
	for n < len(rest) && rest[n] == '#' {
		n++
	}
	if n == 0 || n > maxHeadingLevel {
		return 0
	}
	if n == len(rest) || rest[n] == ' ' || rest[n] == '\t' {
		return n
	}
	return 0
}

// lineIndent returns the column width of line's leading spaces and tabs, a tab
// counting as codeBlockIndent columns.
func lineIndent(line string) int {
	indent := 0
	for _, r := range line {
		switch r {
		case ' ':
			indent++
		case '\t':
			indent += codeBlockIndent
		default:
			return indent
		}
	}
	return indent
}

// listItemContent returns the column where the content of the innermost list
// item that line opens starts, and whether line opens one. Continuation lines
// indented to that column belong to the item, so a heading there is the item's
// heading even when its indentation would start an indented code block at the
// top level.
func listItemContent(line string) (int, bool) {
	i := 0
	for i < len(line) && (line[i] == ' ' || line[i] == '\t') {
		i++
	}
	content := lineIndent(line[:i])
	found := false
	for {
		width, ok := listMarkerWidth(line[i:])
		if !ok {
			break
		}
		found = true
		content += width
		i += width
	}
	return content, found
}

// innermostIndent returns the content column of the innermost open list item,
// or 0 when no list item is open.
func innermostIndent(indents []int) int {
	if len(indents) == 0 {
		return 0
	}
	return indents[len(indents)-1]
}

// updateListIndents returns the open list items after line: the items a line
// indented less than their content ends are dropped, and the item line opens, if
// any, is pushed. A blank line keeps the open items, so a loose item stays open
// across it.
func updateListIndents(indents []int, line string) []int {
	if strings.TrimSpace(line) != "" {
		indent := lineIndent(line)
		for len(indents) > 0 && indent < indents[len(indents)-1] {
			indents = indents[:len(indents)-1]
		}
	}
	content, ok := listItemContent(line)
	if !ok || isThematicBreak(line) {
		return indents
	}
	for len(indents) > 0 && indents[len(indents)-1] >= content {
		indents = indents[:len(indents)-1]
	}
	return append(indents, content)
}

// listMarkerWidth returns the number of bytes from the start of s to the end of
// a bullet or ordered list marker and the spaces after it, and whether s starts
// with one.
func listMarkerWidth(s string) (int, bool) {
	i := 0
	if i < len(s) && (s[i] == '-' || s[i] == '*' || s[i] == '+') {
		i++
	} else {
		digits := 0
		for i < len(s) && digits < maxOrderedMarkerDigits && s[i] >= '0' && s[i] <= '9' {
			i, digits = i+1, digits+1
		}
		if digits == 0 || i == len(s) || (s[i] != '.' && s[i] != ')') {
			return 0, false
		}
		i++
	}
	start := i
	for i < len(s) && s[i] == ' ' {
		i++
	}
	if i == start {
		return 0, false
	}
	return i, true
}

// isParagraphContinuation reports whether line continues an open paragraph
// although it would start a block of its own: an indented code block cannot
// interrupt a paragraph, so GitHub reads the line as paragraph text and a
// following '=' or '-' line underlines the whole paragraph.
func isParagraphContinuation(line string) bool {
	return strings.TrimSpace(line) != "" && lineIndent(line) >= codeBlockIndent
}

// isSetextUnderline reports whether line is a non-empty run of '=' or '-',
// which GitHub reads as a Setext heading underline under paragraph text.
func isSetextUnderline(line string) bool {
	return line != "" && (strings.Trim(line, "=") == "" || strings.Trim(line, "-") == "")
}

// startsParagraph reports whether line begins paragraph text, which a following
// run of '=' or '-' underlines as a Setext heading. A line that opens a list
// item, a blockquote, a table, an indented code block, an HTML block, or a
// thematic break starts another block instead.
func startsParagraph(line string) bool {
	if strings.TrimSpace(line) == "" || lineIndent(line) >= codeBlockIndent {
		return false
	}
	if isThematicBreak(line) || isTableDelimiterRow(line) {
		return false
	}
	rest := strings.TrimLeft(line, " ")
	switch rest[0] {
	case '>', '<':
		return false
	case '-', '*', '+':
		return len(rest) == 1 || (rest[1] != ' ' && rest[1] != '\t')
	}
	return !orderedListMarker(rest)
}

// isTableDelimiterRow reports whether line is a table's delimiter row: cells of
// dashes with optional colons, separated by pipes. A pipe-leading line that is
// not a delimiter row is paragraph text, so a following '---' underlines it.
func isTableDelimiterRow(line string) bool {
	if !strings.Contains(line, "|") || !strings.Contains(line, "-") {
		return false
	}
	for _, r := range line {
		if r != '|' && r != '-' && r != ':' && r != ' ' && r != '\t' {
			return false
		}
	}
	return true
}

// isThematicBreak reports whether line is three or more '-', '*', or '_'
// characters, spaces allowed between them: a thematic break, not paragraph
// text.
func isThematicBreak(line string) bool {
	for _, c := range []string{"-", "*", "_"} {
		if len(strings.Trim(line, c+" ")) == 0 && strings.Count(line, c) >= 3 {
			return true
		}
	}
	return false
}

// orderedListMarker reports whether line starts with an ordered list marker:
// one to nine digits, '.' or ')', and a space or tab, or the end of the line.
func orderedListMarker(line string) bool {
	i := 0
	for i < len(line) && i < maxOrderedMarkerDigits && line[i] >= '0' && line[i] <= '9' {
		i++
	}
	if i == 0 || i == len(line) || (line[i] != '.' && line[i] != ')') {
		return false
	}
	return i+1 == len(line) || line[i+1] == ' ' || line[i+1] == '\t'
}

// StructureDiffKind says which part of two Structures differs.
type StructureDiffKind int

// The differences CompareStructures reports.
const (
	// StructureSame reports two Structures that match.
	StructureSame StructureDiffKind = iota
	// StructureHeadingsDiffer reports headings that differ in level or count.
	StructureHeadingsDiffer
	// StructureCodeBlocksDiffer reports a different number of code blocks.
	StructureCodeBlocksDiffer
	// StructureBothDiffer reports both a heading difference and a different
	// number of code blocks.
	StructureBothDiffer
)

// StructureDiff is the first difference between two Structures. Index is the
// 0-based position of the first heading that differs, Left and Right are that
// heading in each Structure, and a zero HeadingPos means that Structure has no
// heading at Index. Index is -1 when the headings match. LeftCodeBlocks and
// RightCodeBlocks are each Structure's count.
type StructureDiff struct {
	Kind            StructureDiffKind
	Index           int
	Left            HeadingPos
	Right           HeadingPos
	LeftCodeBlocks  int
	RightCodeBlocks int
}

// CompareStructures returns the difference between left and right: the first
// heading whose level differs (or the end of the shorter list when one list is
// a prefix of the other), the code block counts, or both. When both differ,
// both are reported.
func CompareStructures(left, right Structure) StructureDiff {
	diff := StructureDiff{
		Index:           -1,
		LeftCodeBlocks:  left.CodeBlocks,
		RightCodeBlocks: right.CodeBlocks,
	}
	headings := false
	for i := 0; i < len(left.Headings) || i < len(right.Headings); i++ {
		var l, r HeadingPos
		if i < len(left.Headings) {
			l = left.Headings[i]
		}
		if i < len(right.Headings) {
			r = right.Headings[i]
		}
		if l.Level != r.Level {
			diff.Index, diff.Left, diff.Right = i, l, r
			headings = true
			break
		}
	}
	blocks := left.CodeBlocks != right.CodeBlocks
	switch {
	case headings && blocks:
		diff.Kind = StructureBothDiffer
	case headings:
		diff.Kind = StructureHeadingsDiffer
	case blocks:
		diff.Kind = StructureCodeBlocksDiffer
	default:
		diff.Kind = StructureSame
	}
	return diff
}

// String describes the difference for a test failure message: which heading
// came first with both versions' positions, and both code block counts when
// they differ.
func (d StructureDiff) String() string {
	codeBlocks := fmt.Sprintf("code blocks differ: left %d, right %d", d.LeftCodeBlocks, d.RightCodeBlocks)
	switch d.Kind {
	case StructureSame:
		return "structures match"
	case StructureHeadingsDiffer:
		return "headings differ at " + d.headingDifference()
	case StructureCodeBlocksDiffer:
		return codeBlocks
	case StructureBothDiffer:
		return "headings differ at " + d.headingDifference() + "; " + codeBlocks
	}
	return fmt.Sprintf("StructureDiffKind(%d)", int(d.Kind))
}

// headingDifference names the first differing heading and both positions.
func (d StructureDiff) headingDifference() string {
	position := func(p HeadingPos) string {
		if p == (HeadingPos{}) {
			return "missing"
		}
		return fmt.Sprintf("line %d level %d", p.Line, p.Level)
	}
	return fmt.Sprintf("heading %d: left %s, right %s", d.Index+1, position(d.Left), position(d.Right))
}
