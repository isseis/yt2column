//go:build test

package coveragecompare

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
)

const (
	// judgeModule is the module path of the Go judging tests.
	judgeModule = "example.com/m"
	// judgeGoFile is the Go product file of the judging tests.
	judgeGoFile = "x/x.go"
	// judgeExtFile is the extension product file of the judging tests.
	judgeExtFile = "extension/src/a.ts"
)

// goBaseText has A (3-5), B (7-9), and C (11-13); goHeadText is it with B
// deleted.
var (
	goBaseText = joinLines(
		"package x", "",
		"func A() int {", "\treturn 1", "}", "",
		"func B() int {", "\treturn 2", "}", "",
		"func C() int {", "\treturn 3", "}",
	)
	goHeadText = joinLines(
		"package x", "",
		"func A() int {", "\treturn 1", "}", "",
		"func C() int {", "\treturn 3", "}",
	)
	goDeleteB = []string{
		"diff --git a/x/x.go b/x/x.go", "--- a/x/x.go", "+++ b/x/x.go",
		"@@ -6,4 +5,0 @@", "-", "-func B() int {", "-\treturn 2", "-}",
	}
)

// goPoint is one point of a Go judging test. A zero tests count means one
// passing test; stdout and profile replace the generated outputs when set.
type goPoint struct {
	text    string   // the content of x/x.go; empty for none
	blocks  []string // profile blocks of x/x.go, without the file
	tests   int
	exit    int
	stdout  string
	profile string
	readErr error
}

// goJSONPass returns go test -json lines for n passing top-level tests.
func goJSONPass(n int) string {
	var b strings.Builder
	for i := range n {
		fmt.Fprintf(&b, `{"Action":"pass","Package":"%s/x","Test":"Test%d"}`+"\n", judgeModule, i)
	}
	return b.String()
}

// raw returns the point's raw run and source.
func (p goPoint) raw(point Point) (rawRun, source) {
	stdout := p.stdout
	if stdout == "" {
		tests := p.tests
		if tests == 0 {
			tests = 1
		}
		stdout = goJSONPass(tests)
	}
	profile := p.profile
	if profile == "" {
		var b strings.Builder
		b.WriteString("mode: set\n")
		for _, block := range p.blocks {
			fmt.Fprintf(&b, "%s/%s:%s\n", judgeModule, judgeGoFile, block)
		}
		profile = b.String()
	}
	files := map[string]string{"go.mod": "module " + judgeModule + "\n"}
	if p.text != "" {
		files[judgeGoFile] = p.text
	}
	return rawRun{ExitCode: p.exit, Stdout: []byte(stdout), Coverage: []byte(profile)},
		&fakeSource{at: point, files: files, readErr: p.readErr}
}

// goInput builds a Go judging input.
func goInput(base, head goPoint, diff []string) judgeInput {
	baseRaw, baseSrc := base.raw(PointBase)
	headRaw, headSrc := head.raw(PointHead)
	text := ""
	if len(diff) > 0 {
		text = strings.Join(diff, "\n") + "\n"
	}
	return judgeInput{Base: baseRaw, Head: headRaw, Diff: []byte(text), BaseSource: baseSrc, HeadSource: headSrc}
}

// goPassing is the BASE and HEAD of a Go run that deletes B, which did not
// run, and keeps A and C covered.
func goPassing() (goPoint, goPoint) {
	return goPoint{text: goBaseText, blocks: []string{"3.14,5.2 1 1", "7.14,9.2 1 0", "11.14,13.2 1 1"}},
		goPoint{text: goHeadText, blocks: []string{"3.14,5.2 1 1", "7.14,9.2 1 1"}}
}

// extBaseText has a (1-3), b (5-7), and c (9-11); extHeadText is it with b
// deleted.
var (
	extBaseText = joinLines(
		"export function a() {", "  return 1;", "}", "",
		"export function b() {", "  return 2;", "}", "",
		"export function c() {", "  return 3;", "}",
	)
	extHeadText = joinLines(
		"export function a() {", "  return 1;", "}", "",
		"export function c() {", "  return 3;", "}",
	)
	extDeleteB = []string{
		"diff --git a/extension/src/a.ts b/extension/src/a.ts", "--- a/extension/src/a.ts", "+++ b/extension/src/a.ts",
		"@@ -4,4 +3,0 @@", "-", "-export function b() {", "-  return 2;", "-}",
	}
)

// extPoint is one point of an extension judging test. A zero tests count
// means one passing test; tap replaces the generated TAP when set.
type extPoint struct {
	text   string // the content of extension/src/a.ts; empty for none
	ranges []lineRange
	// fns are the FN records of a.ts as "<line>,<name>,<count>"; lcov
	// replaces the generated report when set.
	fns   []string
	lcov  string
	tests int
	exit  int
	tap   string
}

// tapReportText returns a TAP report's summary lines.
func tapReportText(tests, fail, cancelled int) string {
	return fmt.Sprintf("TAP version 13\n# tests %d\n# suites 0\n# pass %d\n# fail %d\n# cancelled %d\n", tests, tests-fail, fail, cancelled)
}

// raw returns the point's raw run and source.
func (p extPoint) raw(point Point) (rawRun, source) {
	tap := p.tap
	if tap == "" {
		tests := p.tests
		if tests == 0 {
			tests = 1
		}
		tap = tapReportText(tests, 0, 0)
	}
	lcov := p.lcov
	if lcov == "" {
		var b strings.Builder
		b.WriteString("TN:\nSF:src/a.ts\n")
		var counts []string
		for _, fn := range p.fns {
			line, rest, _ := strings.Cut(fn, ",")
			name, count, _ := strings.Cut(rest, ",")
			fmt.Fprintf(&b, "FN:%s,%s\n", line, name)
			counts = append(counts, fmt.Sprintf("FNDA:%s,%s\n", count, name))
		}
		b.WriteString(strings.Join(counts, ""))
		b.WriteString("end_of_record\n")
		lcov = b.String()
	}
	src := &fakeSource{at: point, files: map[string]string{}, ranges: map[string]fileRanges{}}
	if p.text != "" {
		src.files[judgeExtFile] = p.text
		src.ranges[judgeExtFile] = fileRanges{Functions: p.ranges}
	}
	return rawRun{ExitCode: p.exit, Coverage: []byte(lcov), TAP: []byte(tap), ExtensionDir: "/checkout/extension"}, src
}

// extInput builds an extension judging input.
func extInput(base, head extPoint, diff []string) judgeInput {
	baseRaw, baseSrc := base.raw(PointBase)
	headRaw, headSrc := head.raw(PointHead)
	text := ""
	if len(diff) > 0 {
		text = strings.Join(diff, "\n") + "\n"
	}
	return judgeInput{Base: baseRaw, Head: headRaw, Diff: []byte(text), BaseSource: baseSrc, HeadSource: headSrc}
}

// extPassing is the BASE and HEAD of an extension run that deletes b and
// keeps a and c run.
func extPassing() (extPoint, extPoint) {
	return extPoint{
			text: extBaseText, ranges: []lineRange{{1, 3}, {5, 7}, {9, 11}},
			fns: []string{"1,a,1", "5,b,1", "9,c,2"},
		},
		extPoint{
			text: extHeadText, ranges: []lineRange{{1, 3}, {5, 7}},
			fns: []string{"1,a,1", "5,c,2"},
		}
}

// requireJudged fails the test unless the result has the outcome and stop
// kind, and returns it.
func requireJudged(t *testing.T, got Result, outcome Outcome, kind StopKind) Result {
	t.Helper()
	if got.Outcome != outcome || (outcome == OutcomeStop && got.Stop != kind) {
		t.Fatalf("outcome = %v stop %v, want %v stop %v (reasons: %+v)", got.Outcome, got.Stop, outcome, kind, got.Reasons)
	}
	return got
}

// reasonPoints returns the points of the reasons.
func reasonPoints(reasons []StopReason) []Point {
	points := make([]Point, 0, len(reasons))
	for _, reason := range reasons {
		points = append(points, reason.Point)
	}
	return points
}

func TestJudge(t *testing.T) {
	ctx := context.Background()
	goT := goTarget{}

	t.Run("a pass reports the measures and the deleted function", func(t *testing.T) {
		base, head := goPassing()
		got := requireJudged(t, judge(ctx, goT, goInput(base, head, goDeleteB)), OutcomePass, 0)
		if want := (Measure{Tests: 1, Files: 1, Functions: 3}); got.Base != want {
			t.Errorf("base measure = %+v, want %+v", got.Base, want)
		}
		if want := (Measure{Tests: 1, Files: 1, Functions: 2}); got.Head != want {
			t.Errorf("head measure = %+v, want %+v", got.Head, want)
		}
		want := []DeletedFunc{{ID: FuncID{judgeGoFile, 7}, Names: []string{"B"}, State: NotRunAtBase}}
		if !slices.EqualFunc(got.Deleted, want, func(a, b DeletedFunc) bool {
			return a.ID == b.ID && slices.Equal(a.Names, b.Names) && a.State == b.State
		}) {
			t.Errorf("deleted = %+v, want %+v", got.Deleted, want)
		}
	})

	t.Run("no test at a point is nothing-measured", func(t *testing.T) {
		for _, point := range []Point{PointBase, PointHead} {
			base, head := goPassing()
			empty := `{"Action":"pass","Package":"example.com/m/x"}` + "\n"
			if point == PointBase {
				base.stdout = empty
			} else {
				head.stdout = empty
			}
			got := requireJudged(t, judge(ctx, goT, goInput(base, head, goDeleteB)), OutcomeStop, StopNothingMeasured)
			if points := reasonPoints(got.Reasons); !slices.Equal(points, []Point{point}) {
				t.Errorf("%v: reason points = %v, want only %v", point, points, point)
			}
		}
	})

	t.Run("no product function at a point is nothing-measured, before unmatched", func(t *testing.T) {
		for _, point := range []Point{PointBase, PointHead} {
			base, head := goPassing()
			// An empty profile leaves the other point's functions without a
			// partner record, which alone would be unmatched.
			if point == PointBase {
				base.profile = "mode: set\n"
			} else {
				head.profile = "mode: set\n"
			}
			got := requireJudged(t, judge(ctx, goT, goInput(base, head, goDeleteB)), OutcomeStop, StopNothingMeasured)
			if points := reasonPoints(got.Reasons); !slices.Equal(points, []Point{point}) {
				t.Errorf("%v: reason points = %v, want only %v", point, points, point)
			}
		}
	})

	t.Run("a HEAD test failure comes before an unreadable BASE profile", func(t *testing.T) {
		base, head := goPassing()
		base.profile = "not a profile\n"
		head.stdout = `{"Action":"fail","Package":"example.com/m/x","Test":"TestBroken"}` + "\n"
		got := requireJudged(t, judge(ctx, goT, goInput(base, head, goDeleteB)), OutcomeStop, StopTestFailed)
		if len(got.Reasons) != 1 || got.Reasons[0].Point != PointHead || got.Reasons[0].Test != "TestBroken" {
			t.Errorf("reasons = %+v, want TestBroken at HEAD", got.Reasons)
		}
	})

	t.Run("a BASE test failure is reported before a HEAD one", func(t *testing.T) {
		base, head := goPassing()
		base.exit, head.exit = 1, 1
		got := requireJudged(t, judge(ctx, goT, goInput(base, head, goDeleteB)), OutcomeStop, StopTestFailed)
		if points := reasonPoints(got.Reasons); !slices.Equal(points, []Point{PointBase}) {
			t.Errorf("reason points = %v, want only base", points)
		}
	})

	t.Run("a non-zero exit code alone is test-failed", func(t *testing.T) {
		base, head := goPassing()
		head.exit = 1
		got := requireJudged(t, judge(ctx, goT, goInput(base, head, goDeleteB)), OutcomeStop, StopTestFailed)
		if points := reasonPoints(got.Reasons); !slices.Equal(points, []Point{PointHead}) {
			t.Errorf("reason points = %v, want only head", points)
		}
	})

	t.Run("a non-zero exit code with unreadable results is test-failed", func(t *testing.T) {
		base, head := goPassing()
		base.exit, base.stdout = 2, "panic: not json\n"
		requireJudged(t, judge(ctx, goT, goInput(base, head, goDeleteB)), OutcomeStop, StopTestFailed)
	})

	t.Run("unreadable results with exit code 0 are unreadable", func(t *testing.T) {
		base, head := goPassing()
		head.stdout = "not json\n"
		got := requireJudged(t, judge(ctx, goT, goInput(base, head, goDeleteB)), OutcomeStop, StopUnreadable)
		if got.Reasons[0].Input != InputGoTestJSON {
			t.Errorf("reason input = %v, want go test -json", got.Reasons[0].Input)
		}
	})

	t.Run("unreadable comes before nothing-measured", func(t *testing.T) {
		base, head := goPassing()
		base.profile = "mode: set\nbroken\n"
		head.profile = "mode: set\n"
		requireJudged(t, judge(ctx, goT, goInput(base, head, goDeleteB)), OutcomeStop, StopUnreadable)
	})

	t.Run("unmatched comes before a lost function", func(t *testing.T) {
		base, head := goPassing()
		// C lost its coverage, and a new function D only HEAD has.
		head.text = goHeadText + "\nfunc D() {}\n"
		head.blocks = []string{"3.14,5.2 1 1", "7.14,9.2 1 0"}
		diff := append(slices.Clone(goDeleteB), "@@ -13,0 +10,2 @@", "+", "+func D() {}")
		got := requireJudged(t, judge(ctx, goT, goInput(base, head, diff)), OutcomeStop, StopUnmatched)
		if len(got.Lost) != 0 {
			t.Errorf("lost = %+v, want none on a stop", got.Lost)
		}
	})

	t.Run("two Go declarations on one line are unmatched", func(t *testing.T) {
		text := joinLines("package x", "", "func A() int { return 1 }; func B() int { return 2 }")
		base := goPoint{text: text, blocks: []string{"3.14,3.26 1 1", "3.42,3.54 1 1"}}
		head := goPoint{text: text, blocks: []string{"3.14,3.26 1 1", "3.42,3.54 1 1"}}
		got := requireJudged(t, judge(ctx, goT, goInput(base, head, nil)), OutcomeStop, StopUnmatched)
		if points := reasonPoints(got.Reasons); !slices.Equal(points, []Point{PointBase, PointHead}) {
			t.Errorf("reason points = %v, want one from each point's table", points)
		}
		for _, reason := range got.Reasons {
			if reason.Func == nil || *reason.Func != (FuncID{judgeGoFile, 3}) {
				t.Errorf("reason = %+v, want x/x.go:3", reason)
			}
		}
	})

	t.Run("a source that cannot be read is internal", func(t *testing.T) {
		base, head := goPassing()
		head.readErr = errors.New("disk failure")
		got := requireJudged(t, judge(ctx, goT, goInput(base, head, goDeleteB)), OutcomeStop, StopInternal)
		if len(got.Reasons) != 1 || got.Reasons[0].Excerpt != "disk failure" {
			t.Errorf("reasons = %+v, want the read failure as the excerpt", got.Reasons)
		}
	})

	t.Run("an error that is not a StopError is internal", func(t *testing.T) {
		base, head := extPassing()
		target := extTarget{ranges: fakeRanges{err: context.Canceled}}
		got := requireJudged(t, judge(ctx, target, extInput(base, head, extDeleteB)), OutcomeStop, StopInternal)
		if len(got.Reasons) != 1 || got.Reasons[0].Excerpt != context.Canceled.Error() {
			t.Errorf("reasons = %+v, want the error as the excerpt", got.Reasons)
		}
	})
}

func TestJudgeScenarios(t *testing.T) {
	type scenario struct {
		name        string
		target      target
		in          judgeInput
		wantOutcome Outcome
		wantStop    StopKind
		wantDeleted []FuncID // checked on a pass
		wantInput   Input    // checked on an unreadable stop
		wantFunc    *FuncID  // a function some unmatched reason must name
	}
	extT := extTarget{ranges: fakeRanges{}}
	var scenarios []scenario
	add := func(s scenario) { scenarios = append(scenarios, s) }

	// AC-33: inputs that must never pass.
	{
		base, head := extPassing()
		head.tap = tapReportText(0, 0, 0)
		add(scenario{name: "a report with no test", target: extT, in: extInput(base, head, extDeleteB), wantStop: StopNothingMeasured})
	}
	{
		base, head := extPassing()
		base.lcov = "SF:test/a.test.ts\nFN:1,t\nFNDA:1,t\nend_of_record\n"
		in := extInput(base, head, extDeleteB)
		in.BaseSource.(*fakeSource).files["extension/test/a.test.ts"] = ""
		add(scenario{name: "a report with no product function", target: extT, in: in, wantStop: StopNothingMeasured})
	}
	{
		base, head := extPassing()
		base.tap = tapReportText(1, 1, 0) + "not ok 1 - fails\n"
		add(scenario{name: "a run with a failed test", target: extT, in: extInput(base, head, extDeleteB), wantStop: StopTestFailed})
	}
	for name, lcov := range map[string]string{
		"a section with more FN than FNDA records": "SF:src/a.ts\nFN:1,a\nFN:5,c\nFNDA:1,a\nend_of_record\n",
		"a section whose FN and FNDA names differ": "SF:src/a.ts\nFN:1,a\nFNDA:1,other\nend_of_record\n",
		"a section that is not closed":             "SF:src/a.ts\nFN:1,a\nFNDA:1,a\n",
		"a report with the same SF section twice":  "SF:src/a.ts\nend_of_record\nSF:src/a.ts\nend_of_record\n",
	} {
		base, head := extPassing()
		head.lcov = lcov
		add(scenario{name: name, target: extT, in: extInput(base, head, extDeleteB), wantStop: StopUnreadable, wantInput: InputLCOV})
	}
	for name, profile := range map[string]string{
		"a malformed Go profile line":          "mode: set\n" + judgeModule + "/x/x.go:3.14,5.2 1\n",
		"a Go profile with no mode line":       judgeModule + "/x/x.go:3.14,5.2 1 1\n",
		"a Go profile whose mode line repeats": "mode: set\nmode: set\n",
	} {
		base, head := goPassing()
		base.profile = profile
		add(scenario{name: name, target: goTarget{}, in: goInput(base, head, goDeleteB), wantStop: StopUnreadable, wantInput: InputGoProfile})
	}
	{
		base, head := goPassing()
		base.blocks = []string{"3.14,5.2 1 1", "7.14,9.2 1 0", "11.14,12.10 3 1", "12.10,13.2 1 1"}
		head.blocks = []string{"3.14,5.2 1 1", "7.14,8.10 3 0", "8.10,9.2 1 1"}
		add(scenario{name: "a Go function whose coverage drops", target: goTarget{}, in: goInput(base, head, goDeleteB), wantOutcome: OutcomeFail})
	}
	{
		base, head := goPassing()
		base.blocks = []string{"3.14,5.2 1 1", "7.14,9.2 1 0", "11.14,12.10 1000 1", "12.10,13.2 1 0"}
		head.blocks = []string{"3.14,5.2 1 1", "7.14,8.10 999 1", "8.10,9.2 2 0"}
		add(scenario{name: "a Go drop that rounding would hide", target: goTarget{}, in: goInput(base, head, goDeleteB), wantOutcome: OutcomeFail})
	}
	{
		base, head := extPassing()
		head.fns = []string{"1,a,1", "5,c,0"}
		add(scenario{name: "an extension function whose run count drops to zero", target: extT, in: extInput(base, head, extDeleteB), wantOutcome: OutcomeFail})
	}
	{
		base, head := extPassing()
		head.lcov = "SF:src/other.ts\nFN:1,o\nFNDA:1,o\nend_of_record\n"
		in := extInput(base, head, extDeleteB)
		in.HeadSource.(*fakeSource).files["extension/src/other.ts"] = ""
		in.HeadSource.(*fakeSource).ranges["extension/src/other.ts"] = fileRanges{Functions: []lineRange{{1, 1}}}
		in.BaseSource.(*fakeSource).files["extension/src/other.ts"] = ""
		in.BaseSource.(*fakeSource).ranges["extension/src/other.ts"] = fileRanges{Functions: []lineRange{{1, 1}}}
		add(scenario{name: "a file that disappears from the HEAD report", target: extT, in: in, wantOutcome: OutcomeFail})
	}
	sameLine := func(baseRan, headRan string, headFns ...string) (extPoint, extPoint) {
		base, head := extPassing()
		base.ranges = append(base.ranges, lineRange{9, 10})
		base.fns = []string{"1,a,1", "5,b,1", "9,c,1", "9,inner," + baseRan}
		head.ranges = append(head.ranges, lineRange{5, 6})
		head.fns = append([]string{"1,a,1", "5,c,1", "5,inner," + headRan}, headFns...)
		return base, head
	}
	{
		base, head := sameLine("1", "0")
		add(scenario{name: "fewer run functions on one line", target: extT, in: extInput(base, head, extDeleteB), wantOutcome: OutcomeFail})
	}
	{
		base, head := sameLine("0", "1")
		add(scenario{name: "more run functions on one line", target: extT, in: extInput(base, head, extDeleteB), wantStop: StopUnmatched, wantFunc: &FuncID{judgeExtFile, 9}})
	}
	{
		base, head := sameLine("1", "1", "5,third,1")
		add(scenario{name: "a different number of functions on one line", target: extT, in: extInput(base, head, extDeleteB), wantStop: StopUnmatched, wantFunc: &FuncID{judgeExtFile, 9}})
	}
	{
		base, head := goPassing()
		head.text = strings.Replace(goHeadText, "return 3", "return 4", 1)
		diff := append(slices.Clone(goDeleteB), "@@ -12,1 +8,1 @@", "-\treturn 3", "+\treturn 4")
		add(scenario{name: "a remaining function with a rewritten line", target: goTarget{}, in: goInput(base, head, diff), wantStop: StopUnmatched, wantFunc: &FuncID{judgeGoFile, 11}})
	}
	{
		base, head := goPassing()
		head.text = strings.Replace(goHeadText, "return 3", "_ = 0\n\treturn 3", 1)
		head.blocks = []string{"3.14,5.2 1 1", "7.14,10.2 2 1"}
		diff := append(slices.Clone(goDeleteB), "@@ -11,0 +8,1 @@", "+\t_ = 0")
		add(scenario{name: "a remaining function with an added line", target: goTarget{}, in: goInput(base, head, diff), wantStop: StopUnmatched, wantFunc: &FuncID{judgeGoFile, 11}})
	}
	{
		base, head := goPassing()
		head.text = goHeadText + "\nfunc D() {}\n"
		diff := append(slices.Clone(goDeleteB), "@@ -13,0 +10,2 @@", "+", "+func D() {}")
		add(scenario{name: "a function only HEAD has", target: goTarget{}, in: goInput(base, head, diff), wantStop: StopUnmatched, wantFunc: &FuncID{judgeGoFile, 11}})
	}

	// AC-34: inputs that pass and list the deleted functions.
	{
		base, head := goPassing()
		add(scenario{
			name: "deleting a function in the middle of a file", target: goTarget{}, in: goInput(base, head, goDeleteB),
			wantOutcome: OutcomePass, wantDeleted: []FuncID{{judgeGoFile, 7}},
		})
	}
	{
		base, head := extPassing()
		base.lcov += "SF:src/gone.ts\nFN:1,g\nFN:5,h\nFNDA:1,g\nFNDA:0,h\nend_of_record\n"
		in := extInput(base, head, append(slices.Clone(extDeleteB),
			"diff --git a/extension/src/gone.ts b/extension/src/gone.ts", "deleted file mode 100644",
			"--- a/extension/src/gone.ts", "+++ /dev/null", "@@ -1,1 +0,0 @@", "-x",
		))
		in.BaseSource.(*fakeSource).files["extension/src/gone.ts"] = "x\n"
		in.BaseSource.(*fakeSource).ranges["extension/src/gone.ts"] = fileRanges{Functions: []lineRange{{1, 3}, {5, 7}}}
		add(scenario{
			name: "deleting a whole file", target: extT, in: in,
			wantOutcome: OutcomePass, wantDeleted: []FuncID{{judgeExtFile, 5}, {"extension/src/gone.ts", 1}, {"extension/src/gone.ts", 5}},
		})
	}
	{
		base, head := sameLine("0", "0")
		add(scenario{
			name: "a line with a function that did not run at BASE and still does not", target: extT, in: extInput(base, head, extDeleteB),
			wantOutcome: OutcomePass, wantDeleted: []FuncID{{judgeExtFile, 5}},
		})
	}

	for _, s := range scenarios {
		t.Run(s.name, func(t *testing.T) {
			got := requireJudged(t, judge(context.Background(), s.target, s.in), s.wantOutcome, s.wantStop)
			if s.wantInput != InputNone && got.Reasons[0].Input != s.wantInput {
				t.Errorf("reason input = %v, want %v", got.Reasons[0].Input, s.wantInput)
			}
			if s.wantFunc != nil && !slices.ContainsFunc(got.Reasons, func(r StopReason) bool {
				return r.Func != nil && *r.Func == *s.wantFunc
			}) {
				t.Errorf("reasons = %+v, want one naming %v", got.Reasons, *s.wantFunc)
			}
			if s.wantOutcome != OutcomePass {
				return
			}
			var deleted []FuncID
			for _, d := range got.Deleted {
				deleted = append(deleted, d.ID)
			}
			if !slices.Equal(deleted, s.wantDeleted) {
				t.Errorf("deleted = %v, want %v", deleted, s.wantDeleted)
			}
		})
	}
}

func TestReadResults(t *testing.T) {
	t.Run("go keeps the last lines of the first failure's output", func(t *testing.T) {
		var b strings.Builder
		for i := 1; i <= failureTailLines+1; i++ {
			fmt.Fprintf(&b, `{"Action":"output","Package":"p","Test":"TestA","Output":"line %d\n"}`+"\n", i)
		}
		b.WriteString(`{"Action":"fail","Package":"p","Test":"TestA"}` + "\n")
		b.WriteString(`{"Action":"output","Package":"p","Test":"TestB","Output":"other\n"}` + "\n")
		b.WriteString(`{"Action":"fail","Package":"p","Test":"TestB"}` + "\n")
		results, err := goTarget{}.readResults(rawRun{Stdout: []byte(b.String())}, PointHead)
		if err != nil {
			t.Fatalf("readResults: %v", err)
		}
		if !results.Failed || len(results.Failures) != 2 {
			t.Fatalf("results = %+v, want two failures", results)
		}
		first := results.Failures[0]
		var want []string
		for i := 2; i <= failureTailLines+1; i++ {
			want = append(want, fmt.Sprintf("line %d", i))
		}
		if first.Test != "TestA" || first.Package != "p" || first.Point != PointHead || first.Excerpt != strings.Join(want, "\n") {
			t.Errorf("first failure = %+v, want TestA in p with lines 2 to %d", first, failureTailLines+1)
		}
		if results.Failures[1].Excerpt != "" {
			t.Errorf("second failure excerpt = %q, want none", results.Failures[1].Excerpt)
		}
	})

	t.Run("the extension keeps the first failure's diagnostic and test file", func(t *testing.T) {
		tap := tapReportText(2, 2, 0) +
			"not ok 1 - first\n  ---\n  error: 'boom'\n  location: '/checkout/extension/test/a.test.ts:3:1'\n  ...\n" +
			"not ok 2 - second\n"
		results, err := extTarget{}.readResults(rawRun{TAP: []byte(tap), ExtensionDir: testExtensionDir}, PointBase)
		if err != nil {
			t.Fatalf("readResults: %v", err)
		}
		if !results.Failed || results.Tests != 2 || len(results.Failures) != 2 {
			t.Fatalf("results = %+v, want two failures of two tests", results)
		}
		first := results.Failures[0]
		wantExcerpt := "  ---\n  error: 'boom'\n  location: '/checkout/extension/test/a.test.ts:3:1'\n  ..."
		if first.Test != "first" || first.Package != "extension/test/a.test.ts" || first.Excerpt != wantExcerpt {
			t.Errorf("first failure = %+v", first)
		}
		if second := results.Failures[1]; second.Test != "second" || second.Excerpt != "" {
			t.Errorf("second failure = %+v", second)
		}
	})

	t.Run("a cancelled test fails the extension run", func(t *testing.T) {
		results, err := extTarget{}.readResults(rawRun{TAP: []byte(tapReportText(1, 0, 1))}, PointBase)
		if err != nil || !results.Failed {
			t.Errorf("results = %+v, %v; want a failure", results, err)
		}
	})
}
