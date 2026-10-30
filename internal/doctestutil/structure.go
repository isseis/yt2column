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
func ParseStructure(doc string) (Structure, error) {
	var structure Structure
	inBlock, textBefore, openLine := false, false, 0
	for i, line := range ScanDocLines(slices.Collect(strings.Lines(doc))) {
		lineNo := i + 1
		if line.Fence {
			if err := checkFence(line.Text, lineNo, inBlock); err != nil {
				return Structure{}, err
			}
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
		if level := HeadingLevel(line.Text); level > 0 {
			structure.Headings = append(structure.Headings, HeadingPos{Line: lineNo, Level: level})
			textBefore = false
			continue
		}
		if githubHeadingLevel(line.Text) > 0 || listHeading.MatchString(line.Text) {
			return Structure{}, fmt.Errorf("line %d: %w: a heading HeadingLevel does not count", lineNo, ErrUnsupportedHeading)
		}
		if isSetextUnderline(trimmed) {
			if textBefore {
				return Structure{}, fmt.Errorf("line %d: %w: Setext underline", lineNo, ErrUnsupportedHeading)
			}
			textBefore = false
			continue
		}
		textBefore = startsParagraph(line.Text)
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
	rest := strings.TrimLeft(line, " ")
	if len(line)-len(rest) > maxHeadingIndent {
		return 0
	}
	n := 0
	for n < len(rest) && rest[n] == '#' {
		n++
	}
	if n == 0 || n > 6 {
		return 0
	}
	if n == len(rest) || rest[n] == ' ' || rest[n] == '\t' {
		return n
	}
	return 0
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
	if strings.TrimSpace(line) == "" || strings.HasPrefix(line, "    ") || strings.HasPrefix(line, "\t") {
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
	for i < len(line) && i < 9 && line[i] >= '0' && line[i] <= '9' {
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
