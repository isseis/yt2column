package codexreview

import (
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"
)

// messageForm is what a final message says, judged by its grammar alone. The
// zero value, formUnreadable, is the reading that assumes least.
type messageForm int

const (
	formUnreadable messageForm = iota
	formFindings
	formNoFindings
)

// rawFinding is one finding line and its body as the final message wrote
// them, before its path is checked against the worktree.
type rawFinding struct {
	path  string
	start int
	end   int
	title string
	body  string
}

var (
	// findingLine is "- [P<n>] <title> <sep> <absolute path>:<start>-<end>". The
	// <sep> is an em dash (U+2014) between spaces; the title is greedy, so
	// the location is what follows the last separator.
	findingLine = regexp.MustCompile(`^- \[P[0-9]+\] (.+) \x{2014} (/.+):([1-9][0-9]{0,8})-([1-9][0-9]{0,8})$`)
	// priorityMark is a priority marker anywhere in a line.
	priorityMark = regexp.MustCompile(`\[P[0-9]+\]`)
	// numberedItem is a numbered list marker at the start of a line.
	numberedItem = regexp.MustCompile(`^[0-9]+\.`)
)

// The two heading lines codex writes before its findings: the first for two or
// more findings, the second for one.
const (
	headingMany = "Full review comments:"
	headingOne  = "Review comment:"
)

// bodyIndent starts every line of a finding's body.
const bodyIndent = "  "

// parseFinalMessage matches text against the two accepted forms of a final
// message.
//
// Contract: the result is formFindings with at least one finding only when a
// summary paragraph is followed by exactly one heading line and every later
// line is a finding line, an indented body line of the finding before it, or
// blank; formNoFindings only when text is a single paragraph with no heading,
// list marker, priority marker, or line ending in a colon; formUnreadable,
// with no findings, otherwise. Nothing is repaired: a line that fits no part
// makes the whole message unreadable, and findings read before it are dropped.
func parseFinalMessage(text string) (messageForm, []rawFinding) {
	if !utf8.ValidString(text) {
		return formUnreadable, nil
	}
	lines := strings.Split(text, "\n")
	heading := -1
	for i, line := range lines {
		if line == headingMany || line == headingOne {
			heading = i
			break
		}
	}
	if heading < 0 {
		if isNoFindingsParagraph(lines) {
			return formNoFindings, nil
		}
		return formUnreadable, nil
	}
	summary := lines[:heading]
	if countParagraphs(summary) == 0 {
		return formUnreadable, nil
	}
	if slices.ContainsFunc(summary, findingLine.MatchString) {
		return formUnreadable, nil
	}
	findings, ok := parseFindings(lines[heading+1:])
	if !ok || len(findings) == 0 {
		return formUnreadable, nil
	}
	return formFindings, findings
}

// parseFindings reads the lines after the heading: finding lines, each
// followed by its indented body, and blank lines. It reports false on any
// other line, and on a body line before the first finding line.
func parseFindings(lines []string) ([]rawFinding, bool) {
	var findings []rawFinding
	var current *rawFinding
	var body []string
	flush := func() {
		if current == nil {
			return
		}
		for len(body) > 0 && body[len(body)-1] == "" {
			body = body[:len(body)-1]
		}
		current.body = strings.Join(body, "\n")
		findings = append(findings, *current)
		body = nil
	}
	for _, line := range lines {
		switch {
		case isBlank(line):
			if current != nil {
				body = append(body, "")
			}
		case strings.HasPrefix(line, bodyIndent):
			if current == nil {
				return nil, false
			}
			body = append(body, strings.TrimPrefix(line, bodyIndent))
		default:
			finding, ok := parseFindingLine(line)
			if !ok {
				return nil, false
			}
			flush()
			current = &finding
		}
	}
	flush()
	return findings, true
}

// parseFindingLine reads one finding line; its start line must not be after
// its end line, and its title must not be blank.
func parseFindingLine(line string) (rawFinding, bool) {
	m := findingLine.FindStringSubmatch(line)
	if m == nil || isBlank(m[1]) {
		return rawFinding{}, false
	}
	// The grammar caps both numbers at nine digits, so they fit an int.
	start, startErr := strconv.Atoi(m[3])
	end, endErr := strconv.Atoi(m[4])
	if startErr != nil || endErr != nil || start > end {
		return rawFinding{}, false
	}
	return rawFinding{path: m[2], start: start, end: end, title: m[1]}, true
}

// isNoFindingsParagraph reports whether lines hold exactly one paragraph and
// none of its lines looks like part of a list of findings: a list marker at
// the start, a priority marker anywhere, or a colon at the end.
func isNoFindingsParagraph(lines []string) bool {
	if countParagraphs(lines) != 1 {
		return false
	}
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.ContainsAny(trimmed[:1], "-*+") || numberedItem.MatchString(trimmed) ||
			priorityMark.MatchString(trimmed) || strings.HasSuffix(trimmed, ":") {
			return false
		}
	}
	return true
}

// countParagraphs counts the runs of non-blank lines.
func countParagraphs(lines []string) int {
	n := 0
	inParagraph := false
	for _, line := range lines {
		blank := isBlank(line)
		if !blank && !inParagraph {
			n++
		}
		inParagraph = !blank
	}
	return n
}

// isBlank reports whether line holds nothing but white space.
func isBlank(line string) bool { return strings.TrimSpace(line) == "" }
