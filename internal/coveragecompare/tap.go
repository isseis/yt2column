package coveragecompare

import (
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

// tapLocationRE matches a TAP diagnostic location: an absolute path, a line,
// and a column.
var tapLocationRE = regexp.MustCompile(`^(.*):([0-9]+):([0-9]+)$`)

// TAP summary kinds.
const (
	tapTests = iota
	tapFail
	tapCancelled
)

// tapSummaryPrefixes are the summary lines the parser requires, in kind order.
var tapSummaryPrefixes = [...]string{"# tests ", "# fail ", "# cancelled "}

// tapFailure is one "not ok" line. File is the test file from the first
// failure's diagnostic, relative to the repository root, or empty when the
// diagnostic names none. Diagnostic holds that block for the first failure, so
// the output can show what failed without node's standard output.
type tapFailure struct {
	Name       string
	File       string
	Diagnostic []string
}

// tapReport is what one TAP stream reported.
type tapReport struct {
	Tests     int
	Fail      int
	Cancelled int
	Failures  []tapFailure
}

// tapParser holds the state of one TAP parse.
type tapParser struct {
	report          *tapReport
	point           Point
	extensionDir    string
	testsLine       int
	failLine        int
	cancelledLine   int
	firstDiagnostic []string
	inDiagnostic    bool
	seenFailure     bool
}

// parseTAP reads the TAP report node writes. Each of "# tests", "# fail", and
// "# cancelled" must appear exactly once at the first column, with a
// non-negative integer that fits int. A "not ok" line names a failure at any
// indentation. extensionDir is the extension directory of the measured
// checkout; a diagnostic location under it becomes a path relative to the
// repository root.
func parseTAP(data []byte, extensionDir string, point Point) (*tapReport, error) {
	parser := &tapParser{
		report:       &tapReport{},
		point:        point,
		extensionDir: extensionDir,
	}
	for i, line := range splitLines(string(data)) {
		if err := parser.line(line, i+1); err != nil {
			return nil, err
		}
	}

	switch {
	case parser.testsLine == 0:
		return nil, stopError(point, InputTAP, 0, "no # tests line appears", "")
	case parser.failLine == 0:
		return nil, stopError(point, InputTAP, 0, "no # fail line appears", "")
	case parser.cancelledLine == 0:
		return nil, stopError(point, InputTAP, 0, "no # cancelled line appears", "")
	}
	if len(parser.report.Failures) > 0 {
		parser.report.Failures[0].File = tapDiagnosticFile(parser.firstDiagnostic, extensionDir)
		parser.report.Failures[0].Diagnostic = parser.firstDiagnostic
	}
	return parser.report, nil
}

// line takes one line of the report.
func (p *tapParser) line(line string, lineNo int) error {
	if p.inDiagnostic {
		// The block opens with "---" on the line after "not ok"; without
		// it, the failure has no block, and later indented lines (such as
		// another subtest's "not ok") are read as usual.
		opens := len(p.firstDiagnostic) > 0 || strings.TrimSpace(line) == "---"
		if opens && line != "" && (line[0] == ' ' || line[0] == '\t') {
			p.firstDiagnostic = append(p.firstDiagnostic, line)
			if strings.TrimSpace(line) == "..." {
				p.inDiagnostic = false
			}
			return nil
		}
		p.inDiagnostic = false
	}

	kind, value, isSummary, wellFormed := tapSummary(line)
	if isSummary {
		return p.setSummary(kind, value, wellFormed, line, lineNo)
	}
	trimmed := strings.TrimLeft(line, " \t")
	if strings.HasPrefix(trimmed, "not ok ") {
		p.addFailure(trimmed)
	}
	return nil
}

// setSummary records one summary line.
func (p *tapParser) setSummary(kind, value int, wellFormed bool, line string, lineNo int) error {
	var seen *int
	var name string
	switch kind {
	case tapTests:
		seen, name = &p.testsLine, "# tests"
	case tapFail:
		seen, name = &p.failLine, "# fail"
	case tapCancelled:
		seen, name = &p.cancelledLine, "# cancelled"
	default:
		return nil
	}
	if *seen != 0 {
		return stopError(p.point, InputTAP, lineNo, "a second "+name+" line appears", line)
	}
	if !wellFormed {
		return stopError(p.point, InputTAP, lineNo, "the "+name+" count is not a non-negative integer that fits int", line)
	}
	*seen = lineNo
	switch kind {
	case tapTests:
		p.report.Tests = value
	case tapFail:
		p.report.Fail = value
	case tapCancelled:
		p.report.Cancelled = value
	}
	return nil
}

// addFailure records one "not ok" line and starts the diagnostic block of the
// first one.
func (p *tapParser) addFailure(trimmed string) {
	p.report.Failures = append(p.report.Failures, tapFailure{Name: tapFailureName(trimmed)})
	if !p.seenFailure {
		p.seenFailure = true
		p.inDiagnostic = true
	}
}

// tapSummary recognizes a summary line and returns its kind and value.
func tapSummary(line string) (kind, value int, isSummary, wellFormed bool) {
	for i, prefix := range tapSummaryPrefixes {
		if !strings.HasPrefix(line, prefix) {
			continue
		}
		parsed, ok := parseNonNegativeInt(strings.TrimSpace(strings.TrimPrefix(line, prefix)))
		return i, parsed, true, ok
	}
	return 0, 0, false, false
}

// tapFailureName reads the name after "not ok <number> - ". A line without that
// form has no name.
func tapFailureName(trimmed string) string {
	rest := strings.TrimPrefix(trimmed, "not ok ")
	if _, name, found := strings.Cut(rest, " - "); found {
		return name
	}
	return ""
}

// tapDiagnosticFile reads the first location line of a failure's diagnostic
// block and turns its absolute path into a path relative to the repository
// root.
func tapDiagnosticFile(lines []string, extensionDir string) string {
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		location, ok := strings.CutPrefix(trimmed, "location:")
		if !ok {
			continue
		}
		location = strings.TrimSpace(location)
		location = strings.Trim(location, `'"`)
		match := tapLocationRE.FindStringSubmatch(location)
		if match == nil {
			continue
		}
		return tapFileUnderExtension(match[1], extensionDir)
	}
	return ""
}

// tapFileUnderExtension returns a repository-relative path for a file under the
// measured checkout's extension directory, or an empty string when the path is
// not under it. Both sides are compared as written and with symbolic links
// resolved, because node reports the real path (on macOS, /private/var/... for
// a checkout under /var/...).
func tapFileUnderExtension(path, extensionDir string) string {
	if path == "" || extensionDir == "" {
		return ""
	}
	directories := []string{filepath.Clean(extensionDir)}
	if resolved, err := filepath.EvalSymlinks(extensionDir); err == nil {
		directories = appendUnique(directories, resolved)
	}
	files := []string{filepath.Clean(path)}
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		files = appendUnique(files, resolved)
	} else if directory, err := filepath.EvalSymlinks(filepath.Dir(path)); err == nil {
		files = appendUnique(files, filepath.Join(directory, filepath.Base(path)))
	}
	for _, directory := range directories {
		for _, file := range files {
			if rel, ok := relativeWithin(file, directory); ok {
				return "extension/" + rel
			}
		}
	}
	return ""
}

// appendUnique appends value unless list already holds it.
func appendUnique(list []string, value string) []string {
	if slices.Contains(list, value) {
		return list
	}
	return append(list, value)
}

// relativeWithin returns path relative to dir when path is strictly inside dir.
func relativeWithin(path, dir string) (string, bool) {
	rel, err := filepath.Rel(dir, path)
	if err != nil {
		return "", false
	}
	rel = filepath.ToSlash(rel)
	if rel == "." || rel == ".." || strings.HasPrefix(rel, "../") || strings.HasPrefix(rel, "/") {
		return "", false
	}
	return rel, true
}
