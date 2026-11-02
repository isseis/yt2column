// Package coveragecompare measures two commits of this repository and compares
// the coverage of the product code function by function, so that a change which
// only deletes tests can show it left every function covered.
//
// Only Run starts external commands. The input parsers and the judging
// functions are pure, so the tests can give them examples directly.
package coveragecompare

// Target selects which test suite and report the run measures.
type Target int

// Target values.
const (
	TargetGo  Target = iota + 1 // make coverage-compare
	TargetExt                   // make ext-coverage-compare
)

// Outcome is the verdict of one run.
type Outcome int

// Outcome values.
const (
	OutcomeStop Outcome = iota // zero value: a run that did not reach a verdict
	OutcomePass
	OutcomeFail
)

// StopKind is why a run stopped.
type StopKind int

// StopKind values.
const (
	StopInternal StopKind = iota // zero value: the tool's own failure
	StopUsage
	StopNotAncestor
	StopDirty
	StopToolchain
	StopTestFailed
	StopUnreadable
	StopNothingMeasured
	StopUnmatched
)

// String returns the fixed word the result lines use for the stop kind.
func (k StopKind) String() string {
	switch k {
	case StopUsage:
		return "usage"
	case StopNotAncestor:
		return "not-ancestor"
	case StopDirty:
		return "dirty"
	case StopToolchain:
		return "toolchain"
	case StopTestFailed:
		return "test-failed"
	case StopUnreadable:
		return "unreadable"
	case StopNothingMeasured:
		return "nothing-measured"
	case StopUnmatched:
		return "unmatched"
	case StopInternal:
		return "internal"
	default:
		return "internal"
	}
}

// Options is the whole input of Run.
type Options struct {
	Target   Target
	Base     string   // the BASE argument as given
	RepoRoot string   // the repository root (the current directory of make)
	Environ  []string // the process environment, KEY=VALUE
}

// FuncID identifies a function by its file, relative to the repository root,
// and its first line.
type FuncID struct {
	File string
	Line int
}

// Measure is what one point (BASE or HEAD) measured.
type Measure struct {
	Tests     int // executed tests
	Files     int // product files that appear in the point's report
	Functions int // product functions that appear in the point's report
}

// Result is everything the output is written from.
type Result struct {
	Outcome    Outcome
	Stop       StopKind // meaningful only when Outcome is OutcomeStop
	Target     Target
	BaseArg    string
	BaseCommit string
	HeadCommit string
	HeadTree   string
	Toolchain  string // "go1.27.1" or "v24.21.0"
	Base, Head Measure
	Deleted    []DeletedFunc
	Lost       []LostFunc   // OutcomeFail: functions whose coverage went down
	Reasons    []StopReason // OutcomeStop: what the stop applies to
}

// RunState is what a deleted function's record at BASE says.
type RunState int

// RunState values.
const (
	RanUnknown   RunState = iota // zero value: not printed as "ran"
	RanAtBase                    // statements covered, or a run count >= 1
	NotRunAtBase                 // in the report with nothing run, or absent from it
	NoStatements                 // a Go function with no statements
)

// DeletedFunc is one deleted function.
type DeletedFunc struct {
	ID    FuncID
	Names []string // one name for Go; the FN names of the line for the extension
	State RunState
}

// Unit says what a Coverage counts.
type Unit int

// Unit values.
const (
	UnitStatements Unit = iota + 1 // Go: covered statements of a function
	UnitFunctions                  // extension: functions on a line that ran
)

// Coverage is Covered out of Total, in Unit.
type Coverage struct {
	Unit    Unit
	Covered int
	Total   int
}

// LostFunc is one compared Go function, or one line group of extension
// functions, whose coverage went down.
type LostFunc struct {
	ID     FuncID
	Names  []string // one name for Go; every name on the line for the extension
	Before Coverage
	After  Coverage
}

// Point is BASE, HEAD, or neither (a precondition before any test ran).
type Point int

// Point values.
const (
	PointNone Point = iota
	PointBase
	PointHead
)

// String returns the fixed word the stop lines use for the point.
func (p Point) String() string {
	switch p {
	case PointBase:
		return "base"
	case PointHead:
		return "head"
	case PointNone:
		return noneWord
	default:
		return noneWord
	}
}

// Input names the input a stop reason refers to.
type Input int

// Input values.
const (
	InputNone Input = iota
	InputGoProfile
	InputGoTestJSON
	InputLCOV
	InputTAP
	InputDiff
	InputRanges
	InputSource
)

// String returns the fixed word the stop lines use for the input.
func (i Input) String() string {
	switch i {
	case InputGoProfile:
		return "go profile"
	case InputGoTestJSON:
		return "go test -json"
	case InputLCOV:
		return "lcov"
	case InputTAP:
		return "tap"
	case InputDiff:
		return "diff"
	case InputRanges:
		return "ranges"
	case InputSource:
		return "source"
	case InputNone:
		return noneWord
	default:
		return noneWord
	}
}

// StopReason names what a stop applies to. Detail is the tool's own English
// sentence; Excerpt holds text copied from a command's output or input,
// bounded in length and printed quoted (section 3.9).
type StopReason struct {
	Point   Point
	Input   Input
	Line    int     // the input's line number, 0 when none
	Func    *FuncID // the function the reason applies to
	Outer   *FuncID // the enclosing function not run at HEAD (section 3.8)
	Test    string  // a failed test's name
	Package string  // a failed Go package, or an extension test file
	Detail  string
	Excerpt string
}

// splitLines splits text into lines without keeping a final empty line. Every
// other empty line is kept, so a parser can reject it as malformed.
func splitLines(text string) []string {
	lines := splitNewlines(text)
	if n := len(lines); n > 0 && lines[n-1] == "" {
		lines = lines[:n-1]
	}
	return lines
}

// splitNewlines is splitLines without the trailing-empty-line removal.
func splitNewlines(text string) []string {
	var lines []string
	start := 0
	for i := range len(text) {
		if text[i] != '\n' {
			continue
		}
		lines = append(lines, text[start:i])
		start = i + 1
	}
	lines = append(lines, text[start:])
	return lines
}

// parseNonNegativeInt parses a non-negative decimal integer that fits int. A
// sign, a fraction, an empty string, or an out-of-range value is rejected.
func parseNonNegativeInt(s string) (int, bool) {
	if s == "" {
		return 0, false
	}
	for i := range len(s) {
		if s[i] < '0' || s[i] > '9' {
			return 0, false
		}
	}
	n := 0
	for i := range len(s) {
		digit := int(s[i] - '0')
		if n > (maxInt-digit)/decimalBase {
			return 0, false
		}
		n = n*decimalBase + digit
	}
	return n, true
}

// noneWord is the word the point and input names use when they name nothing.
const noneWord = "none"

// maxInt is the largest value of int.
const maxInt = int(^uint(0) >> 1)

// decimalBase is the base of the decimal integers the parsers accept.
const decimalBase = 10
