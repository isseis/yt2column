package coveragecompare

import (
	"slices"
	"strings"

	"github.com/isseis/yt2column/internal/strictjson"
)

// goTestFailure is one failed Go test, package, or build. Test is empty when
// the failure belongs to a package rather than a test; Package then holds the
// package or import path. Output is the standard output lines go test reported
// for that failure.
type goTestFailure struct {
	Package string
	Test    string
	Output  []string
}

// goTestReport is what one go test -json stream reported.
type goTestReport struct {
	Tests    int
	Failures []goTestFailure
}

// goTestKey identifies the stream that one output line belongs to.
type goTestKey struct {
	Package string
	Test    string
}

// goTestLine is the part of one go test -json line the parser reads.
type goTestLine struct {
	Action     string
	Package    string
	Test       string
	ImportPath string
	Output     string
	// FailedBuild names the import path whose build failed, on the package
	// fail line that follows a build-fail.
	FailedBuild string
}

// goTestParser holds what one go test -json stream reported while it is read.
type goTestParser struct {
	report       *goTestReport
	outputs      map[goTestKey][]string
	buildOutputs map[string][]string
	buildImport  map[string][]string // clean package path to its failed import paths
}

// parseGoTestJSON reads the output of go test -json. Every line must be one
// JSON object; a line that is not stops as unreadable rather than being
// skipped. Top-level tests that ended in pass, fail, or skip are counted, and
// subtests are not. A fail line names a test, or its package when it has no
// test; a build-fail line names the package its import path belongs to.
func parseGoTestJSON(data []byte, point Point) (*goTestReport, error) {
	parser := &goTestParser{
		report:       &goTestReport{},
		outputs:      make(map[goTestKey][]string),
		buildOutputs: make(map[string][]string),
		buildImport:  make(map[string][]string),
	}
	for i, line := range splitLines(string(data)) {
		entry, err := parseGoTestLine(line, point, i+1)
		if err != nil {
			return nil, err
		}
		parser.record(entry)
	}
	parser.attachOutput()
	return parser.report, nil
}

// parseGoTestLine reads the members of one output line.
func parseGoTestLine(line string, point Point, lineNo int) (goTestLine, error) {
	if line == "" {
		return goTestLine{}, stopError(point, InputGoTestJSON, lineNo, "the line is empty", line)
	}
	object, err := strictjson.ParseObject([]byte(line))
	if err != nil {
		return goTestLine{}, stopError(point, InputGoTestJSON, lineNo, "the line is not one JSON object", line)
	}
	members, err := object.Collect("Action", "Package", "Test", "ImportPath", "Output", "FailedBuild")
	if err != nil {
		return goTestLine{}, stopError(point, InputGoTestJSON, lineNo, "a consumed member has an unaccepted shape", line)
	}
	var entry goTestLine
	for _, field := range []struct {
		key    string
		target *string
	}{
		{"Action", &entry.Action},
		{"Package", &entry.Package},
		{"Test", &entry.Test},
		{"ImportPath", &entry.ImportPath},
		{"Output", &entry.Output},
		{"FailedBuild", &entry.FailedBuild},
	} {
		value, err := jsonMemberString(members, field.key)
		if err != nil {
			return goTestLine{}, stopError(point, InputGoTestJSON, lineNo, field.key+" is not a string", line)
		}
		*field.target = value
	}
	return entry, nil
}

// record takes one line: it counts a finished top-level test, records a
// failure, and keeps an output line for the failure it belongs to. A build
// failure is recorded once, from its build-fail line, rather than again from
// the package fail line that follows it.
func (p *goTestParser) record(entry goTestLine) {
	stream := goTestKey{Package: entry.Package, Test: entry.Test}
	switch entry.Action {
	case "output":
		p.outputs[stream] = append(p.outputs[stream], entry.Output)
	case "build-output":
		p.buildOutputs[entry.ImportPath] = append(p.buildOutputs[entry.ImportPath], entry.Output)
	case "pass", "fail", "skip":
		if entry.Test != "" && !strings.Contains(entry.Test, "/") {
			p.report.Tests++
		}
	}

	switch entry.Action {
	case "build-fail":
		pkg := cleanImportPath(entry.ImportPath)
		if pkg == "" {
			return
		}
		if _, ok := p.buildImport[pkg]; ok {
			if !slices.Contains(p.buildImport[pkg], entry.ImportPath) {
				p.buildImport[pkg] = append(p.buildImport[pkg], entry.ImportPath)
			}
			return
		}
		p.buildImport[pkg] = []string{entry.ImportPath}
		p.report.Failures = append(p.report.Failures, goTestFailure{Package: pkg})
	case "fail":
		if entry.Test == "" {
			// The package fail line after a build-fail names the failed
			// build in FailedBuild. It can differ from Package: an external
			// test package (p_test) or a dependency that did not compile.
			if _, built := p.buildImport[cleanImportPath(entry.FailedBuild)]; built && entry.FailedBuild != "" {
				return
			}
			if _, built := p.buildImport[stream.Package]; built {
				return
			}
		}
		switch {
		case entry.Test != "":
			p.report.Failures = append(p.report.Failures, goTestFailure{Package: stream.Package, Test: entry.Test})
		case stream.Package != "":
			p.report.Failures = append(p.report.Failures, goTestFailure{Package: stream.Package})
		}
	}
}

// attachOutput gives each failure the lines go test wrote for it. A build
// failure's compiler output comes from its import path's build-output lines,
// and the package's own FAIL line from its output lines.
func (p *goTestParser) attachOutput() {
	for i := range p.report.Failures {
		failure := &p.report.Failures[i]
		if failure.Test == "" {
			for _, importPath := range p.buildImport[failure.Package] {
				failure.Output = append(failure.Output, p.buildOutputs[importPath]...)
			}
		}
		failure.Output = append(failure.Output, p.outputs[goTestKey{Package: failure.Package, Test: failure.Test}]...)
	}
}

// cleanImportPath removes the package test binary from an import path such as
// "example.com/p [example.com/p.test]".
func cleanImportPath(importPath string) string {
	path, _, _ := strings.Cut(importPath, " [")
	return path
}

// jsonMemberString returns a string member, or an empty string when the member
// is absent.
func jsonMemberString(members map[string]strictjson.Value, key string) (string, error) {
	value, ok := members[key]
	if !ok {
		return "", nil
	}
	return value.AsString()
}
