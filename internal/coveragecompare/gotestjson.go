package coveragecompare

import (
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
}

// parseGoTestJSON reads the output of go test -json. Every line must be one
// JSON object; a line that is not stops as unreadable rather than being
// skipped. Top-level tests that ended in pass, fail, or skip are counted, and
// subtests are not. A fail line names a test, or its package when it has no
// test; a build-fail line names its import path.
func parseGoTestJSON(data []byte, point Point) (*goTestReport, error) {
	report := &goTestReport{}
	outputs := make(map[goTestKey][]string)
	for i, line := range splitLines(string(data)) {
		entry, err := parseGoTestLine(line, point, i+1)
		if err != nil {
			return nil, err
		}
		recordGoTestLine(report, outputs, entry)
	}

	for i := range report.Failures {
		failure := &report.Failures[i]
		failure.Output = outputs[goTestKey{Package: failure.Package, Test: failure.Test}]
	}
	return report, nil
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
	members, err := object.Collect("Action", "Package", "Test", "ImportPath", "Output")
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
	} {
		value, err := jsonMemberString(members, field.key)
		if err != nil {
			return goTestLine{}, stopError(point, InputGoTestJSON, lineNo, field.key+" is not a string", line)
		}
		*field.target = value
	}
	return entry, nil
}

// recordGoTestLine counts a finished top-level test, records a failure, and
// keeps an output line for the failure it belongs to.
func recordGoTestLine(report *goTestReport, outputs map[goTestKey][]string, entry goTestLine) {
	stream := goTestKey{Package: entry.Package, Test: entry.Test}
	if stream.Package == "" {
		stream.Package = entry.ImportPath
	}
	if entry.Action == "output" {
		outputs[stream] = append(outputs[stream], entry.Output)
	}
	switch entry.Action {
	case "pass", "fail", "skip":
		if entry.Test != "" && !strings.Contains(entry.Test, "/") {
			report.Tests++
		}
	}
	switch entry.Action {
	case "fail":
		switch {
		case entry.Test != "":
			report.Failures = append(report.Failures, goTestFailure{Package: stream.Package, Test: entry.Test})
		case stream.Package != "":
			report.Failures = append(report.Failures, goTestFailure{Package: stream.Package})
		}
	case "build-fail":
		if stream.Package != "" {
			report.Failures = append(report.Failures, goTestFailure{Package: stream.Package})
		}
	}
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
