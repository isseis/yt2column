package coveragecompare

import (
	"cmp"
	"slices"
	"strings"
)

// lcovFunction is one function record: its first line, the name the report
// gave it, and how many times the tests called it.
type lcovFunction struct {
	Line  int
	Name  string
	Count int
}

// lcovRunCount is one FNDA record, paired with its FN record by position.
type lcovRunCount struct {
	Name  string
	Count int
}

// lcovFile is one SF section of an lcov report. Path is relative to the
// repository root, so it starts with "extension/".
type lcovFile struct {
	Path      string
	Functions []lcovFunction
}

// lcovReport is the report of one measured point.
type lcovReport struct {
	Files []lcovFile
}

// lcovSkippedPrefixes are the record kinds the requirements allow the parser to
// read past. Their values take no part in the comparison.
var lcovSkippedPrefixes = []string{
	"TN:",
	"FNF:",
	"FNH:",
	"DA:",
	"LF:",
	"LH:",
	"BRDA:",
	"BRF:",
	"BRH:",
}

// lcovParser holds the state of one lcov parse.
type lcovParser struct {
	report       *lcovReport
	point        Point
	seen         map[string]bool
	current      *lcovFile
	functions    []lcovFunction
	counts       []lcovRunCount
	sectionStart int
}

// parseLCOV reads the lcov report node writes. It accepts a sequence of SF
// sections, each made of FN and FNDA records, the skipped records above, and
// end_of_record. A TN record may stand before a section; every other skipped
// record must be inside one. A record it does not accept, a section left open,
// a record outside a section, a repeated SF, a mismatched FN/FNDA pair, and a
// number that is not a non-negative integer that fits int all stop as
// unreadable.
func parseLCOV(data []byte, point Point) (*lcovReport, error) {
	parser := &lcovParser{report: &lcovReport{}, point: point, seen: make(map[string]bool)}
	for i, line := range splitLines(string(data)) {
		if err := parser.line(line, i+1); err != nil {
			return nil, err
		}
	}
	if parser.current != nil {
		return nil, stopError(point, InputLCOV, parser.sectionStart, "the SF section is not closed by end_of_record", parser.current.Path)
	}
	slices.SortFunc(parser.report.Files, func(a, b lcovFile) int {
		return strings.Compare(a.Path, b.Path)
	})
	return parser.report, nil
}

// line takes one line of the report.
func (p *lcovParser) line(line string, lineNo int) error {
	switch {
	case strings.HasPrefix(line, "SF:"):
		return p.startSection(line, lineNo)
	case strings.HasPrefix(line, "FN:"):
		return p.addFunction(line, lineNo)
	case strings.HasPrefix(line, "FNDA:"):
		return p.addCount(line, lineNo)
	case line == "end_of_record":
		return p.endSection(line, lineNo)
	default:
		if !isLCOVSkipped(line) {
			return stopError(p.point, InputLCOV, lineNo, "the line is not an accepted lcov record", line)
		}
		if p.current == nil && !strings.HasPrefix(line, "TN:") {
			return stopError(p.point, InputLCOV, lineNo, "a record appears outside a section", line)
		}
		return nil
	}
}

// startSection begins one SF section.
func (p *lcovParser) startSection(line string, lineNo int) error {
	if p.current != nil {
		return stopError(p.point, InputLCOV, lineNo, "a section starts before the previous section ends", line)
	}
	path, ok := lcovSourcePath(strings.TrimPrefix(line, "SF:"))
	if !ok {
		return stopError(p.point, InputLCOV, lineNo, "the SF path is empty, absolute, or holds an empty, \".\" or \"..\" element", line)
	}
	if p.seen[path] {
		return stopError(p.point, InputLCOV, lineNo, "the same SF section appears twice", line)
	}
	p.seen[path] = true
	p.current = &lcovFile{Path: path}
	p.functions = nil
	p.counts = nil
	p.sectionStart = lineNo
	return nil
}

// addFunction records one FN record.
func (p *lcovParser) addFunction(line string, lineNo int) error {
	if p.current == nil {
		return stopError(p.point, InputLCOV, lineNo, "an FN record appears outside a section", line)
	}
	start, name, ok := parseLCOVNumberAndName(strings.TrimPrefix(line, "FN:"))
	if !ok {
		return stopError(p.point, InputLCOV, lineNo, "the FN record is malformed", line)
	}
	p.functions = append(p.functions, lcovFunction{Line: start, Name: name})
	return nil
}

// addCount records one FNDA record.
func (p *lcovParser) addCount(line string, lineNo int) error {
	if p.current == nil {
		return stopError(p.point, InputLCOV, lineNo, "an FNDA record appears outside a section", line)
	}
	count, name, ok := parseLCOVNumberAndName(strings.TrimPrefix(line, "FNDA:"))
	if !ok {
		return stopError(p.point, InputLCOV, lineNo, "the FNDA record is malformed", line)
	}
	p.counts = append(p.counts, lcovRunCount{Name: name, Count: count})
	return nil
}

// endSection pairs the section's FN and FNDA records and stores the section.
func (p *lcovParser) endSection(line string, lineNo int) error {
	if p.current == nil {
		return stopError(p.point, InputLCOV, lineNo, "end_of_record appears outside a section", line)
	}
	if len(p.functions) != len(p.counts) {
		return stopError(p.point, InputLCOV, lineNo, "the section has a different number of FN and FNDA records", line)
	}
	for i := range p.functions {
		if p.functions[i].Name != p.counts[i].Name {
			return stopError(p.point, InputLCOV, lineNo, "an FN record and its FNDA record name different functions", line)
		}
		p.functions[i].Count = p.counts[i].Count
	}
	slices.SortFunc(p.functions, func(a, b lcovFunction) int {
		return cmp.Or(cmp.Compare(a.Line, b.Line), strings.Compare(a.Name, b.Name))
	})
	p.current.Functions = p.functions
	p.report.Files = append(p.report.Files, *p.current)
	p.current = nil
	return nil
}

// parseLCOVNumberAndName reads either "<line>,<name>" (FN) or
// "<count>,<name>" (FNDA). The first comma separates the number from the name.
func parseLCOVNumberAndName(record string) (int, string, bool) {
	number, name, found := strings.Cut(record, ",")
	if !found {
		return 0, "", false
	}
	value, ok := parseNonNegativeInt(number)
	if !ok {
		return 0, "", false
	}
	return value, name, true
}

// lcovSourcePath turns the SF path, which is relative to extension/, into a
// path relative to the repository root. An empty or absolute path, and a path
// with an empty, "." or ".." element, are rejected rather than cleaned, so a
// path such as ./src/a.ts cannot escape the product check or the duplicate
// section check.
func lcovSourcePath(raw string) (string, bool) {
	if !cleanRelPath(raw) {
		return "", false
	}
	return "extension/" + raw, true
}

// isLCOVSkipped reports whether a line is a record kind the requirements allow
// the parser to read past.
func isLCOVSkipped(line string) bool {
	for _, prefix := range lcovSkippedPrefixes {
		if strings.HasPrefix(line, prefix) {
			return true
		}
	}
	return false
}

// isExtProduct reports whether a repository-relative path is extension product
// code: a file under extension/src/ or extension/scripts/. The comparison is
// per path element, so extension/srcx/ is not product code.
func isExtProduct(path string) bool {
	rest, ok := strings.CutPrefix(path, "extension/")
	if !ok {
		return false
	}
	return strings.HasPrefix(rest, "src/") || strings.HasPrefix(rest, "scripts/")
}
