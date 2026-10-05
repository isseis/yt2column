package writer

import (
	"fmt"
	"strings"
)

// The checks in this file judge the body part of the generated text against
// CommonMark 0.31.2 without being a CommonMark parser. Wherever the judgment
// is simpler than CommonMark, it rejects: a body accepted here has no raw
// HTML and no unclosed code fence in CommonMark either. Every function runs
// in time linear in its input; none searches the rest of a line once per
// character or per backtick run.

const (
	// minFenceLength is the shortest run of '`' or '~' that opens a fence.
	minFenceLength = 3
	// maxClosingFenceIndent is the most spaces a closing fence may be
	// indented by.
	maxClosingFenceIndent = 3
	// fenceLikePrefix lists what is stripped from the start of a line to
	// find a fence that is indented or nested in a block quote or list.
	fenceLikePrefix = " \t>-+*0123456789.)"
	// minSchemeLength and maxSchemeLength bound the scheme of a URI
	// autolink.
	minSchemeLength = 2
	maxSchemeLength = 32
)

// bodyLine is a line of the body part outside code fences.
type bodyLine struct {
	text string
	num  int // 1-based line number within the body part
}

// fence is an open code fence: its character and run length.
type fence struct {
	char byte
	n    int
}

// span is the byte range line[start:end] of a line.
type span struct{ start, end int }

// checkBodyMarkdown rejects a body part that contains raw HTML or ends inside
// an unclosed code fence. A returned error names the reason and the line
// number within the body part, never the text, which is untrusted.
func checkBodyMarkdown(body string) error {
	if hasBareCR(body) {
		return errBareCR
	}
	prose, err := proseLines(splitLines(body))
	if err != nil {
		return err
	}
	exempt := codeSpansExempt(prose)
	for _, l := range prose {
		var spans []span
		if exempt {
			spans, _ = codeSpans(l.text)
		}
		if err := checkLineHTML(l, spans); err != nil {
			return err
		}
	}
	return nil
}

// hasBareCR reports whether s has a '\r' that is neither followed by '\n'
// nor the last byte of s. CommonMark ends a line at a lone '\r', but not every
// renderer does, and one that does not can leave a fence open over the source
// block. A '\r' that ends s is followed by the separator's '\n' in the
// article.
func hasBareCR(s string) bool {
	for i := range len(s) - 1 {
		if s[i] == '\r' && s[i+1] != '\n' {
			return true
		}
	}
	return false
}

// splitLines splits s at "\n", "\r\n", and "\r", the line endings of
// CommonMark.
func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '\n':
			lines = append(lines, s[start:i])
			start = i + 1
		case '\r':
			lines = append(lines, s[start:i])
			if i+1 < len(s) && s[i+1] == '\n' {
				i++
			}
			start = i + 1
		}
	}
	return append(lines, s[start:])
}

// proseLines tracks code fences from the first line and returns the lines
// outside fences, without the opening and closing fence lines. It rejects a
// line outside a fence that looks like a fence but does not open one at the
// top level (an indented fence, or one in a block quote or list), whose
// closing CommonMark decides by nesting this check does not track, and a
// body that ends inside a fence.
func proseLines(lines []string) ([]bodyLine, error) {
	var prose []bodyLine
	var open fence // n == 0 outside a fence
	openedAt := 0
	for i, line := range lines {
		num := i + 1
		if open.n > 0 {
			if isClosingFence(line, open) {
				open = fence{}
			}
			continue
		}
		if f, ok := openingFence(line); ok {
			open, openedAt = f, num
			continue
		}
		if looksLikeFence(line) {
			return nil, fmt.Errorf("%w at line %d", errFenceLikeLine, num)
		}
		prose = append(prose, bodyLine{text: line, num: num})
	}
	if open.n > 0 {
		return nil, fmt.Errorf("%w opened at line %d", errUnclosedFence, openedAt)
	}
	return prose, nil
}

// openingFence reports whether line opens a code fence: with no indentation,
// a run of at least minFenceLength '`' or '~', and, after a '`' run, an info
// string without '`'.
func openingFence(line string) (fence, bool) {
	if line == "" || (line[0] != '`' && line[0] != '~') {
		return fence{}, false
	}
	n := runLength(line, 0)
	if n < minFenceLength {
		return fence{}, false
	}
	if line[0] == '`' && strings.IndexByte(line[n:], '`') >= 0 {
		return fence{}, false
	}
	return fence{char: line[0], n: n}, true
}

// isClosingFence reports whether line closes f, as CommonMark defines it: up
// to maxClosingFenceIndent spaces, at least f.n of f.char, then only spaces
// and tabs.
func isClosingFence(line string, f fence) bool {
	rest := strings.TrimLeft(line, " ")
	if len(line)-len(rest) > maxClosingFenceIndent || rest == "" || rest[0] != f.char {
		return false
	}
	n := runLength(rest, 0)
	return n >= f.n && strings.Trim(rest[n:], " \t") == ""
}

// looksLikeFence reports whether line starts with "```" or "~~~" once
// leading spaces, tabs, block-quote markers, and list markers are removed.
func looksLikeFence(line string) bool {
	rest := strings.TrimLeft(line, fenceLikePrefix)
	return strings.HasPrefix(rest, "```") || strings.HasPrefix(rest, "~~~")
}

// runLength returns the length of the run of s[i] starting at i.
func runLength(s string, i int) int {
	j := i
	for j < len(s) && s[j] == s[i] {
		j++
	}
	return j - i
}

// codeSpansExempt reports whether a '<' inside a code span can be left
// uncounted. In CommonMark, a construct that starts first can take in a
// backtick meant to delimit a code span; this holds only when no line can
// contain such a construct and every backtick run pairs on its own line:
//   - no line has '\', '[', or ']' (escapes, and links and images, whose
//     destination or title can span lines),
//   - no line has a backtick between a '<' and the next '>' (an autolink,
//     of either kind, that contains a backtick),
//   - every backtick run pairs within its line (codeSpans).
//
// Raw HTML, the remaining such construct, is rejected wherever it is found.
func codeSpansExempt(lines []bodyLine) bool {
	for _, l := range lines {
		if strings.ContainsAny(l.text, `\[]`) || backtickInAngle(l.text) {
			return false
		}
		if _, ok := codeSpans(l.text); !ok {
			return false
		}
	}
	return true
}

// backtickInAngle reports whether a backtick follows a '<' before the next
// '>' on line. Every '<' counts, escaped or not; the exemption that uses this
// also requires that no line has a backslash, so none is escaped.
func backtickInAngle(line string) bool {
	open, tick := false, false
	for i := range len(line) {
		switch line[i] {
		case '<':
			if !open {
				open, tick = true, false
			}
		case '`':
			tick = tick || open
		case '>':
			if open && tick {
				return true
			}
			open = false
		}
	}
	return false
}

// codeSpans pairs the backtick runs of line from the left: a run not yet
// paired opens a code span that the first later run of the same length
// closes, and reading resumes after the closing run. It returns the inside
// of each span, in order, and ok is false when a run has no closing run on
// the line. The next run of each length is found in one backward pass, so no
// run is searched for twice.
func codeSpans(line string) (spans []span, ok bool) {
	runs := backtickRuns(line)
	next := make([]int, len(runs))
	nearest := make(map[int]int) // run length to the index of the nearest later run
	for i := len(runs) - 1; i >= 0; i-- {
		n := runs[i].end - runs[i].start
		j, found := nearest[n]
		if !found {
			j = -1
		}
		next[i] = j
		nearest[n] = i
	}
	for i := 0; i < len(runs); {
		j := next[i]
		if j < 0 {
			return nil, false
		}
		spans = append(spans, span{start: runs[i].end, end: runs[j].start})
		i = j + 1
	}
	return spans, true
}

// backtickRuns returns the maximal runs of backticks in line, in order.
func backtickRuns(line string) []span {
	var runs []span
	for i := 0; i < len(line); {
		if line[i] != '`' {
			i++
			continue
		}
		n := runLength(line, i)
		runs = append(runs, span{start: i, end: i + n})
		i += n
	}
	return runs
}

// checkLineHTML rejects line when it has a '<' followed by an ASCII letter,
// '/', '!', or '?', the start of every kind of CommonMark raw HTML, inline or
// block. A '<' is not counted when it is escaped by an odd number of
// backslashes, starts a URI autolink without a backtick, or lies inside one
// of spans, the code spans of the line (nil when code spans are not exempt).
func checkLineHTML(l bodyLine, spans []span) error {
	line := l.text
	for i := 0; i < len(line); i++ {
		for len(spans) > 0 && spans[0].end <= i {
			spans = spans[1:]
		}
		if line[i] != '<' || (len(spans) > 0 && spans[0].start <= i) || isEscaped(line, i) {
			continue
		}
		if n := autolinkLength(line[i:]); n > 0 {
			i += n - 1
			continue
		}
		if i+1 < len(line) && startsRawHTML(line[i+1]) {
			return fmt.Errorf("%w at line %d", errRawHTML, l.num)
		}
	}
	return nil
}

// isEscaped reports whether line[i] follows an odd number of backslashes.
func isEscaped(line string, i int) bool {
	n := 0
	for j := i - 1; j >= 0 && line[j] == '\\'; j-- {
		n++
	}
	return n%2 == 1
}

// autolinkLength returns the length of the URI autolink at the start of s,
// or 0 when there is none. The grammar is CommonMark's: '<', a scheme of
// minSchemeLength to maxSchemeLength ASCII letters, digits, '+', '.', and '-'
// starting with a letter, ':', characters other than ASCII controls, space,
// '<', and '>', then '>'. A backtick is also refused, because CommonMark
// does not let it delimit a code span inside an autolink.
func autolinkLength(s string) int {
	if len(s) < 2 || s[0] != '<' || !isASCIILetter(s[1]) {
		return 0
	}
	i := 2
	for i < len(s) && isSchemeChar(s[i]) {
		i++
	}
	if n := i - 1; n < minSchemeLength || n > maxSchemeLength || i == len(s) || s[i] != ':' {
		return 0
	}
	for i++; i < len(s); i++ {
		switch c := s[i]; {
		case c == '>':
			return i + 1
		case c <= ' ' || c == 0x7f || c == '<' || c == '`':
			return 0
		}
	}
	return 0
}

// startsRawHTML reports whether c, following '<', can start raw HTML.
func startsRawHTML(c byte) bool {
	return isASCIILetter(c) || c == '/' || c == '!' || c == '?'
}

func isASCIILetter(c byte) bool {
	return ('a' <= c && c <= 'z') || ('A' <= c && c <= 'Z')
}

func isSchemeChar(c byte) bool {
	return isASCIILetter(c) || ('0' <= c && c <= '9') || c == '+' || c == '.' || c == '-'
}
