package coveragecompare

import (
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// excerptLimit is the most bytes of one excerpt the output shows. A
// test-failed excerpt keeps its end, where the failure is; any other keeps its
// start.
const excerptLimit = 2048

// Exit codes. 2 is not used: it is what a Go panic exits with.
const (
	exitPass = 0
	exitFail = 1
	exitStop = 3
)

// ExitCode is the tool's exit code for an outcome: 0 pass, 1 fail, 3 stop. An
// outcome outside the three is a stop.
func ExitCode(o Outcome) int {
	switch o {
	case OutcomePass:
		return exitPass
	case OutcomeFail:
		return exitFail
	case OutcomeStop:
		return exitStop
	default:
		return exitStop
	}
}

// Write writes the result lines and the final line. Every name, path,
// excerpt, and the BASE argument is printed quoted, because they come from
// the user or from a command's output and may hold control characters. The
// measures are printed once the run has read both reports, the deleted
// functions only with a verdict, and each stop reason with its excerpt and,
// for a failed test, the command that runs it again. The final line is
// "coverage-compare: PASS", "coverage-compare: FAIL", or
// "coverage-compare: STOP <kind>"; an outcome outside the three is written as
// an internal stop.
func Write(w io.Writer, r Result) error {
	var b strings.Builder
	writeField(&b, "target", targetCommand(r.Target)+" BASE="+strconv.Quote(r.BaseArg))
	if r.BaseCommit != "" {
		writeField(&b, "base", plainOrQuoted(r.BaseCommit))
	}
	if r.HeadCommit != "" {
		head := plainOrQuoted(r.HeadCommit)
		if r.HeadTree != "" {
			head += " tree " + plainOrQuoted(r.HeadTree)
		}
		writeField(&b, "head", head)
	}
	if r.Toolchain != "" {
		writeField(&b, "toolchain", plainOrQuoted(r.Toolchain))
	}

	final := "coverage-compare: STOP " + StopInternal.String()
	switch r.Outcome {
	case OutcomePass, OutcomeFail:
		writeMeasures(&b, r)
		writeDeleted(&b, r.Deleted)
		if r.Outcome == OutcomePass {
			b.WriteString("no compared function lost coverage\n")
			final = "coverage-compare: PASS"
			break
		}
		for _, lost := range r.Lost {
			writeField(&b, "lost", fmt.Sprintf("%s %s %d/%d -> %d/%d",
				location(lost.ID), names(lost.Names),
				lost.Before.Covered, lost.Before.Total, lost.After.Covered, lost.After.Total))
		}
		final = "coverage-compare: FAIL"
	case OutcomeStop:
		if r.Stop == StopNothingMeasured || r.Stop == StopUnmatched {
			writeMeasures(&b, r)
		}
		for _, reason := range r.Reasons {
			writeReason(&b, r, reason)
		}
		final = "coverage-compare: STOP " + r.Stop.String()
	default:
		// An outcome outside the three is the tool's own failure.
	}
	b.WriteString(final + "\n")
	_, err := io.WriteString(w, b.String())
	return err
}

// writeField writes one "<label> <value>" line with the values aligned.
func writeField(b *strings.Builder, label, value string) {
	fmt.Fprintf(b, "%-9s %s\n", label, value)
}

// writeMeasures writes the tests, files, and functions of both points.
func writeMeasures(b *strings.Builder, r Result) {
	writeField(b, "tests", fmt.Sprintf("base %d head %d", r.Base.Tests, r.Head.Tests))
	writeField(b, "files", fmt.Sprintf("base %d head %d", r.Base.Files, r.Head.Files))
	writeField(b, "functions", fmt.Sprintf("base %d head %d", r.Base.Functions, r.Head.Functions))
}

// writeDeleted writes the number of deleted functions and one line for each.
func writeDeleted(b *strings.Builder, deleted []DeletedFunc) {
	writeField(b, "deleted", strconv.Itoa(len(deleted)))
	for _, d := range deleted {
		writeField(b, "deleted", location(d.ID)+" "+names(d.Names)+runStateNote(d.State))
	}
}

// runStateNote is what a deleted function's line adds for its BASE record.
func runStateNote(state RunState) string {
	switch state {
	case RanAtBase:
		return ""
	case NotRunAtBase:
		return " (not run at BASE)"
	case NoStatements:
		return " (no statements)"
	case RanUnknown:
		return " (run state unknown)"
	default:
		return " (run state unknown)"
	}
}

// writeReason writes one stop reason: a stop line with what it applies to and
// the tool's sentence, its excerpt, and the command that runs a failed test
// again.
func writeReason(b *strings.Builder, r Result, reason StopReason) {
	parts := []string{r.Stop.String()}
	if reason.Point != PointNone {
		parts = append(parts, reason.Point.String())
	}
	if reason.Input != InputNone {
		parts = append(parts, reason.Input.String())
	}
	if reason.Line > 0 {
		parts = append(parts, "line "+strconv.Itoa(reason.Line))
	}
	if reason.Func != nil {
		if reason.Func.Line == 0 {
			parts = append(parts, "file "+strconv.Quote(reason.Func.File))
		} else {
			parts = append(parts, "function "+location(*reason.Func))
		}
	}
	if reason.Outer != nil {
		parts = append(parts, "outer "+location(*reason.Outer)+" not run at HEAD")
	}
	if reason.Test != "" {
		parts = append(parts, "test "+strconv.Quote(reason.Test))
	}
	if reason.Package != "" {
		parts = append(parts, failedUnitLabel(r.Target)+" "+strconv.Quote(reason.Package))
	}
	line := strings.Join(parts, " ")
	if reason.Detail != "" {
		line += ": " + reason.Detail
	}
	writeField(b, "stop", line)
	if reason.Excerpt != "" {
		writeField(b, "excerpt", boundedQuote(reason.Excerpt, r.Stop == StopTestFailed))
	}
	if r.Stop == StopTestFailed {
		if command := rerunCommand(r, reason); command != "" {
			writeField(b, "rerun", command)
		}
	}
}

// rerunCommand is the command that runs a failed test again at its point's
// commit: go test with the test's name and package, or node --test with the
// extension test file. It is empty when the reason names neither, or when a
// name cannot be quoted for a shell.
func rerunCommand(r Result, reason StopReason) string {
	var prefix string
	var args []string
	switch r.Target {
	case TargetGo:
		switch {
		case reason.Package == "":
			return ""
		case reason.Test != "":
			prefix = "go test -tags test -run "
			args = []string{"^" + regexp.QuoteMeta(reason.Test) + "$", reason.Package}
		default:
			prefix = "go test -tags test "
			args = []string{reason.Package}
		}
	case TargetExt:
		file, ok := strings.CutPrefix(reason.Package, "extension/")
		if !ok {
			return ""
		}
		prefix = "cd extension && node --test "
		args = []string{file}
	default:
		return ""
	}
	quoted := make([]string, len(args))
	for i, arg := range args {
		q, ok := shellQuote(arg)
		if !ok {
			return ""
		}
		quoted[i] = q
	}
	command := prefix + strings.Join(quoted, " ")
	commit := ""
	switch reason.Point {
	case PointBase:
		commit = r.BaseCommit
	case PointHead:
		commit = r.HeadCommit
	case PointNone:
	}
	if commit == "" {
		return command
	}
	return "at " + plainOrQuoted(commit) + ": " + command
}

// failedUnitLabel names what a test-failed reason's Package holds.
func failedUnitLabel(t Target) string {
	switch t {
	case TargetExt:
		return "file"
	case TargetGo:
		return "package"
	default:
		return "package"
	}
}

// targetCommand is the make command a target runs as.
func targetCommand(t Target) string {
	switch t {
	case TargetGo:
		return "make coverage-compare"
	case TargetExt:
		return "make ext-coverage-compare"
	default:
		return "make <unknown target>"
	}
}

// location is a function's quoted file and its line.
func location(id FuncID) string {
	return strconv.Quote(id.File) + ":" + strconv.Itoa(id.Line)
}

// names is a function's quoted names, or <unknown> when the report gave it
// none.
func names(list []string) string {
	if len(list) == 0 {
		return "<unknown>"
	}
	quoted := make([]string, len(list))
	for i, name := range list {
		quoted[i] = strconv.Quote(name)
	}
	return strings.Join(quoted, " ")
}

// boundedQuote quotes an excerpt of at most excerptLimit bytes, cut at a rune
// boundary, and marks a cut one.
func boundedQuote(excerpt string, keepEnd bool) string {
	if len(excerpt) <= excerptLimit {
		return strconv.Quote(excerpt)
	}
	if keepEnd {
		cut := len(excerpt) - excerptLimit
		for cut < len(excerpt) && !utf8.RuneStart(excerpt[cut]) {
			cut++
		}
		return strconv.Quote(excerpt[cut:]) + " (truncated)"
	}
	cut := excerptLimit
	for cut > 0 && !utf8.RuneStart(excerpt[cut]) {
		cut--
	}
	return strconv.Quote(excerpt[:cut]) + " (truncated)"
}

// plainOrQuoted prints a commit ID or a version as it is when it holds only
// letters, digits, and ".", "+", "-", "_", and quoted otherwise, so a value
// read from a command's output never writes a control character.
func plainOrQuoted(value string) string {
	for _, c := range value {
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9':
		case c == '.', c == '+', c == '-', c == '_':
		default:
			return strconv.Quote(value)
		}
	}
	return value
}

// shellQuote quotes an argument of a rerun command for a POSIX shell: in single
// quotes, so nothing in it is expanded when the line is pasted. A value that is
// not valid UTF-8 or holds a character that is not printable cannot be quoted
// that way without writing a control character, so it reports false and the
// rerun line is left out.
func shellQuote(value string) (string, bool) {
	if !utf8.ValidString(value) {
		return "", false
	}
	for _, c := range value {
		if !unicode.IsPrint(c) {
			return "", false
		}
	}
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'", true
}
