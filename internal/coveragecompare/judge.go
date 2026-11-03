package coveragecompare

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
)

// target is everything that differs between the Go and the extension
// comparison. The diff, the line mapping, the classification, the stop
// priority, and the output are shared and live outside it.
type target interface {
	// readResults reads one point's test results: the test count and the
	// failures.
	readResults(raw rawRun, point Point) (testResults, error)
	// readReport reads one point's coverage report.
	readReport(raw rawRun, src source) (report, error)
	// isProduct reports whether a file at a point is product code.
	isProduct(src source, file string) (bool, error)
	// table builds one point's function table from the source and the
	// parsed report; files is the set of product files to read.
	table(ctx context.Context, src source, rep report, files []string) (funcTable, error)
	// compare compares the matched functions of the two tables.
	compare(base, head funcTable, pairs []matchedPair) ([]LostFunc, []StopReason)
}

// rawRun is what one point's test run produced: its exit code, its standard
// output, and the content of the report files it wrote.
type rawRun struct {
	ExitCode int
	Stdout   []byte // go test -json output; node writes nothing there
	Coverage []byte // the Go profile, or the lcov report
	TAP      []byte // the extension's TAP report
	// ExtensionDir is the measured checkout's extension directory, which a
	// TAP failure's location is made relative to.
	ExtensionDir string
}

// testResults is one point's test results. Failed is true when the results
// themselves report a failure; Failures names what failed, as test-failed
// reasons.
type testResults struct {
	Tests    int
	Failed   bool
	Failures []StopReason
}

// failureTailLines is how many lines of the first failure's output a
// test-failed reason keeps.
const failureTailLines = 20

// judgeInput is everything the judging reads: the raw outputs of both test
// runs, the diff from BASE to HEAD, and the two points' sources.
type judgeInput struct {
	Base       rawRun
	Head       rawRun
	Diff       []byte
	BaseSource source
	HeadSource source
}

// judge decides a run from its raw outputs, in the stop priority of the
// requirements: test-failed, unreadable, nothing-measured, unmatched, then a
// failure, and a pass when none applies. A test failure at BASE is reported
// before one at HEAD, and either before any input that cannot be read, because
// a failed run's report is not used at all. The unmatched reasons found while
// the tables were built are held until nothing-measured has been checked. The
// returned Result holds the verdict, the measures, the deleted functions, the
// lost functions, and the reasons; the caller adds what the run itself knows.
func judge(ctx context.Context, t target, in judgeInput) Result {
	baseResults, baseErr := t.readResults(in.Base, PointBase)
	headResults, headErr := t.readResults(in.Head, PointHead)
	measured := Result{Base: Measure{Tests: baseResults.Tests}, Head: Measure{Tests: headResults.Tests}}
	if reasons := testFailure(in.Base, baseResults, baseErr, PointBase); len(reasons) > 0 {
		return stopWith(measured, StopTestFailed, reasons)
	}
	if reasons := testFailure(in.Head, headResults, headErr, PointHead); len(reasons) > 0 {
		return stopWith(measured, StopTestFailed, reasons)
	}
	for _, err := range []error{baseErr, headErr} {
		if err != nil {
			return stopFromError(measured, err)
		}
	}

	baseTable, headTable, classes, err := readTables(ctx, t, in)
	if err != nil {
		return stopFromError(measured, err)
	}
	measured.Base.Files, measured.Base.Functions = baseTable.measure.Files, baseTable.measure.Functions
	measured.Head.Files, measured.Head.Functions = headTable.measure.Files, headTable.measure.Functions

	if reasons := nothingMeasured(measured); len(reasons) > 0 {
		return stopWith(measured, StopNothingMeasured, reasons)
	}
	lost, compareReasons := t.compare(baseTable, headTable, classes.pairs)
	var unmatched []StopReason
	unmatched = append(unmatched, baseTable.unmatched...)
	unmatched = append(unmatched, headTable.unmatched...)
	unmatched = append(unmatched, classes.unmatched...)
	unmatched = append(unmatched, compareReasons...)
	if len(unmatched) > 0 {
		return stopWith(measured, StopUnmatched, unmatched)
	}

	for _, entry := range classes.deleted {
		measured.Deleted = append(measured.Deleted, DeletedFunc{ID: entry.ID, Names: entry.Names, State: runState(entry)})
	}
	if len(lost) > 0 {
		measured.Outcome = OutcomeFail
		measured.Lost = lost
		return measured
	}
	measured.Outcome = OutcomePass
	return measured
}

// readTables reads both reports and the diff, and builds and classifies the
// two function tables over the same set of candidate files.
func readTables(ctx context.Context, t target, in judgeInput) (funcTable, funcTable, classification, error) {
	baseReport, err := t.readReport(in.Base, in.BaseSource)
	if err != nil {
		return funcTable{}, funcTable{}, classification{}, err
	}
	headReport, err := t.readReport(in.Head, in.HeadSource)
	if err != nil {
		return funcTable{}, funcTable{}, classification{}, err
	}
	patch, err := parseDiff(in.Diff, PointNone)
	if err != nil {
		return funcTable{}, funcTable{}, classification{}, err
	}
	candidates := candidateFiles(baseReport, headReport, patch)
	var tables []funcTable
	for _, side := range []struct {
		src source
		rep report
	}{{in.BaseSource, baseReport}, {in.HeadSource, headReport}} {
		files, err := productFiles(t.isProduct, side.src, side.rep, candidates)
		if err != nil {
			return funcTable{}, funcTable{}, classification{}, err
		}
		table, err := t.table(ctx, side.src, side.rep, files)
		if err != nil {
			return funcTable{}, funcTable{}, classification{}, err
		}
		tables = append(tables, table)
	}
	classes, err := classify(tables[0], tables[1], patch, in.BaseSource)
	if err != nil {
		return funcTable{}, funcTable{}, classification{}, err
	}
	return tables[0], tables[1], classes, nil
}

// testFailure returns the test-failed reasons of one point, or none when its
// run passed. A non-zero exit code is a failure whatever the results say; a
// result that reports a failure is one even with exit code 0. Results that
// cannot be read name no test, and with exit code 0 they are left to the
// unreadable check.
func testFailure(raw rawRun, results testResults, readErr error, point Point) []StopReason {
	switch {
	case raw.ExitCode == 0 && (readErr != nil || !results.Failed):
		return nil
	case readErr == nil && len(results.Failures) > 0:
		return results.Failures
	case raw.ExitCode != 0:
		detail := fmt.Sprintf("the test run exited with status %d and named no failed test", raw.ExitCode)
		if readErr != nil {
			detail = fmt.Sprintf("the test run exited with status %d, and its results cannot be read", raw.ExitCode)
		}
		return []StopReason{{Point: point, Detail: detail}}
	default:
		return []StopReason{{Point: point, Detail: "the test results report a failure but name no failed test"}}
	}
}

// nothingMeasured returns a reason for each point that ran no test or whose
// report names no product function.
func nothingMeasured(r Result) []StopReason {
	var reasons []StopReason
	for _, side := range []struct {
		point   Point
		measure Measure
	}{{PointBase, r.Base}, {PointHead, r.Head}} {
		if side.measure.Tests == 0 {
			reasons = append(reasons, StopReason{Point: side.point, Detail: "no test ran"})
		}
		if side.measure.Functions == 0 {
			reasons = append(reasons, StopReason{Point: side.point, Detail: "the report names no product function"})
		}
	}
	return reasons
}

// stopWith returns the measured result stopped with the reasons.
func stopWith(r Result, kind StopKind, reasons []StopReason) Result {
	r.Outcome = OutcomeStop
	r.Stop = kind
	r.Reasons = reasons
	return r
}

// stopFromError returns the measured result stopped by err. A StopError keeps
// its kind and reasons; any other error is the tool's own failure, as is a
// StopError of kind internal, whose underlying error becomes the reason's
// excerpt when it has no reason of its own.
func stopFromError(r Result, err error) Result {
	stop, ok := errors.AsType[*StopError](err)
	if !ok || stop == nil {
		return stopWith(r, StopInternal, []StopReason{{Detail: "the judging failed", Excerpt: err.Error()}})
	}
	reasons := stop.Reasons
	if stop.Kind == StopInternal && len(reasons) == 0 {
		excerpt := ""
		if stop.Err != nil {
			excerpt = stop.Err.Error()
		}
		reasons = []StopReason{{Detail: "the judging failed", Excerpt: excerpt}}
	}
	return stopWith(r, stop.Kind, reasons)
}

// goTarget is the Go comparison.
type goTarget struct{}

// readResults reads the go test -json output. Each failed test, package, or
// build is a reason, and the first one keeps the end of its output.
func (goTarget) readResults(raw rawRun, point Point) (testResults, error) {
	parsed, err := parseGoTestJSON(raw.Stdout, point)
	if err != nil {
		return testResults{}, err
	}
	results := testResults{Tests: parsed.Tests, Failed: len(parsed.Failures) > 0}
	for i, failure := range parsed.Failures {
		reason := StopReason{Point: point, Input: InputGoTestJSON, Test: failure.Test, Package: failure.Package, Detail: "a test failed"}
		if failure.Test == "" {
			reason.Detail = "a package failed to build or run"
		}
		if i == 0 {
			reason.Excerpt = tailLines(splitLines(strings.Join(failure.Output, "")), failureTailLines)
		}
		results.Failures = append(results.Failures, reason)
	}
	return results, nil
}

// readReport reads the Go profile, with the module path from the point's
// go.mod.
func (goTarget) readReport(raw rawRun, src source) (report, error) {
	goMod, err := readPresent(src, "go.mod")
	if err != nil {
		return report{}, err
	}
	modulePath, err := goModulePath(goMod, src.point())
	if err != nil {
		return report{}, err
	}
	profile, err := parseGoProfile(raw.Coverage, modulePath, src.point())
	if err != nil {
		return report{}, err
	}
	var files []string
	for _, block := range profile.Blocks {
		files = append(files, block.File)
	}
	slices.Sort(files)
	return report{files: slices.Compact(files), profile: profile}, nil
}

// isProduct reads the file's build constraint.
func (goTarget) isProduct(src source, file string) (bool, error) {
	return isGoSourceProduct(src, file)
}

// table builds a Go function table.
func (goTarget) table(_ context.Context, src source, rep report, files []string) (funcTable, error) {
	if rep.profile == nil {
		return funcTable{}, &StopError{Kind: StopInternal, Reasons: []StopReason{{Point: src.point(), Detail: "the Go table was given no profile"}}}
	}
	return buildGoTable(src, rep.profile, files)
}

// compare compares the matched Go functions.
func (goTarget) compare(_, _ funcTable, pairs []matchedPair) ([]LostFunc, []StopReason) {
	return compareGo(pairs)
}

// extTarget is the extension comparison. ranges is the range helper.
type extTarget struct {
	ranges rangeReader
}

// readResults reads the TAP report. Each "not ok" line is a reason; the first
// keeps the end of its diagnostic block and names its test file.
func (extTarget) readResults(raw rawRun, point Point) (testResults, error) {
	parsed, err := parseTAP(raw.TAP, raw.ExtensionDir, point)
	if err != nil {
		return testResults{}, err
	}
	results := testResults{Tests: parsed.Tests, Failed: parsed.Fail > 0 || parsed.Cancelled > 0}
	for i, failure := range parsed.Failures {
		reason := StopReason{Point: point, Input: InputTAP, Test: failure.Name, Package: failure.File, Detail: "a test failed"}
		if i == 0 {
			reason.Excerpt = tailLines(failure.Diagnostic, failureTailLines)
		}
		results.Failures = append(results.Failures, reason)
	}
	return results, nil
}

// readReport reads the lcov report.
func (extTarget) readReport(raw rawRun, src source) (report, error) {
	lcov, err := parseLCOV(raw.Coverage, src.point())
	if err != nil {
		return report{}, err
	}
	files := make([]string, 0, len(lcov.Files))
	for _, file := range lcov.Files {
		files = append(files, file.Path)
	}
	return report{files: files, lcov: lcov}, nil
}

// isProduct decides by the path.
func (extTarget) isProduct(src source, file string) (bool, error) {
	return isExtSourceProduct(src, file)
}

// table builds an extension function table.
func (t extTarget) table(ctx context.Context, src source, rep report, files []string) (funcTable, error) {
	if rep.lcov == nil {
		return funcTable{}, &StopError{Kind: StopInternal, Reasons: []StopReason{{Point: src.point(), Detail: "the extension table was given no lcov report"}}}
	}
	return buildExtTable(ctx, src, t.ranges, rep.lcov, files)
}

// compare compares the matched extension line groups.
func (extTarget) compare(_, head funcTable, pairs []matchedPair) ([]LostFunc, []StopReason) {
	return compareExt(head, pairs)
}

// tailLines joins the last n lines.
func tailLines(lines []string, n int) string {
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}
