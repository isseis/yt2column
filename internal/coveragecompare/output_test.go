//go:build test

package coveragecompare

import (
	"errors"
	"strings"
	"testing"
)

const (
	testBaseCommit = "0123456789abcdef0123456789abcdef01234567"
	testHeadCommit = "89abcdef0123456789abcdef0123456789abcdef"
	testHeadTree   = "fedcba9876543210fedcba9876543210fedcba98"
)

// written returns what Write writes for r.
func written(t *testing.T, r Result) string {
	t.Helper()
	var b strings.Builder
	if err := Write(&b, r); err != nil {
		t.Fatalf("Write: %v", err)
	}
	return b.String()
}

// resolved returns a result whose commits and toolchain are known.
func resolved(target Target) Result {
	return Result{
		Target: target, BaseArg: "HEAD~1",
		BaseCommit: testBaseCommit, HeadCommit: testHeadCommit, HeadTree: testHeadTree,
		Toolchain: "go1.27.1",
		Base:      Measure{Tests: 1219, Files: 140, Functions: 1630},
		Head:      Measure{Tests: 1218, Files: 140, Functions: 1627},
	}
}

func TestWrite(t *testing.T) {
	header := `target    make coverage-compare BASE="HEAD~1"
base      ` + testBaseCommit + `
head      ` + testHeadCommit + ` tree ` + testHeadTree + `
toolchain go1.27.1
`
	measures := `tests     base 1219 head 1218
files     base 140 head 140
functions base 1630 head 1627
`
	tests := []struct {
		name   string
		result func() Result
		want   string
	}{
		{
			name: "a pass lists every item and each deleted function's BASE record",
			result: func() Result {
				r := resolved(TargetGo)
				r.Outcome = OutcomePass
				r.Deleted = []DeletedFunc{
					{ID: FuncID{"internal/x/y.go", 12}, Names: []string{"helper"}, State: NotRunAtBase},
					{ID: FuncID{"internal/x/y.go", 20}, Names: []string{"(*T).empty"}, State: NoStatements},
					{ID: FuncID{"internal/x/y.go", 30}, Names: []string{"ran"}, State: RanAtBase},
					{ID: FuncID{"internal/x/y.go", 40}, Names: []string{"odd"}, State: RunState(99)},
				}
				return r
			},
			want: header + measures + `deleted   4
deleted   "internal/x/y.go":12 "helper" (not run at BASE)
deleted   "internal/x/y.go":20 "(*T).empty" (no statements)
deleted   "internal/x/y.go":30 "ran"
deleted   "internal/x/y.go":40 "odd" (run state unknown)
no compared function lost coverage
coverage-compare: PASS
`,
		},
		{
			name: "a failure lists the lost line groups and no claim of kept coverage",
			result: func() Result {
				r := resolved(TargetExt)
				r.Toolchain = "v24.21.0"
				r.Outcome = OutcomeFail
				r.Deleted = []DeletedFunc{{ID: FuncID{"extension/src/a.ts", 5}, State: NotRunAtBase}}
				r.Lost = []LostFunc{{
					ID:     FuncID{"extension/src/a.ts", 9},
					Names:  []string{"anonymous_1", "inner"},
					Before: Coverage{Unit: UnitFunctions, Covered: 2, Total: 2},
					After:  Coverage{Unit: UnitFunctions, Covered: 1, Total: 2},
				}}
				return r
			},
			want: strings.Replace(strings.Replace(header, "make coverage-compare", "make ext-coverage-compare", 1), "go1.27.1", "v24.21.0", 1) +
				measures + `deleted   1
deleted   "extension/src/a.ts":5 <unknown> (not run at BASE)
lost      "extension/src/a.ts":9 "anonymous_1" "inner" 2/2 -> 1/2
coverage-compare: FAIL
`,
		},
		{
			name: "a Go test failure names the point, test, package, output end, and rerun command",
			result: func() Result {
				r := resolved(TargetGo)
				r.Outcome, r.Stop = OutcomeStop, StopTestFailed
				r.Reasons = []StopReason{
					{Point: PointHead, Input: InputGoTestJSON, Test: "TestA", Package: "example.com/m/x", Detail: "a test failed", Excerpt: "--- FAIL: TestA\nwant 1"},
					{Point: PointHead, Input: InputGoTestJSON, Package: "example.com/m/y", Detail: "a package failed to build or run"},
				}
				return r
			},
			want: header + `stop      test-failed head go test -json test "TestA" package "example.com/m/x": a test failed
excerpt   "--- FAIL: TestA\nwant 1"
rerun     at ` + testHeadCommit + `: go test -tags test -run "^TestA$" "example.com/m/x"
stop      test-failed head go test -json package "example.com/m/y": a package failed to build or run
rerun     at ` + testHeadCommit + `: go test -tags test "example.com/m/y"
coverage-compare: STOP test-failed
`,
		},
		{
			name: "an extension test failure gives the test file's rerun command",
			result: func() Result {
				r := Result{Target: TargetExt, BaseArg: "main", BaseCommit: testBaseCommit, Outcome: OutcomeStop, Stop: StopTestFailed}
				r.Reasons = []StopReason{
					{Point: PointBase, Input: InputTAP, Test: "fails", Package: "extension/test/a.test.ts", Detail: "a test failed"},
					{Point: PointBase, Detail: "the test run exited with status 1 and named no failed test"},
				}
				return r
			},
			want: `target    make ext-coverage-compare BASE="main"
base      ` + testBaseCommit + `
stop      test-failed base tap test "fails" file "extension/test/a.test.ts": a test failed
rerun     at ` + testBaseCommit + `: cd extension && node --test "test/a.test.ts"
stop      test-failed base: the test run exited with status 1 and named no failed test
coverage-compare: STOP test-failed
`,
		},
		{
			name: "an unreadable input names the point, input, line, and excerpt",
			result: func() Result {
				r := resolved(TargetGo)
				r.Outcome, r.Stop = OutcomeStop, StopUnreadable
				r.Reasons = []StopReason{
					{Point: PointBase, Input: InputGoProfile, Line: 7, Detail: "the line is not a coverage block", Excerpt: "garbage"},
					{Point: PointHead, Input: InputSource, Func: &FuncID{File: "go.mod"}, Detail: "go.mod has no single readable module path"},
				}
				return r
			},
			want: header + `stop      unreadable base go profile line 7: the line is not a coverage block
excerpt   "garbage"
stop      unreadable head source file "go.mod": go.mod has no single readable module path
coverage-compare: STOP unreadable
`,
		},
		{
			name: "an unmatched stop gives the measures, the function, and the enclosing function",
			result: func() Result {
				r := resolved(TargetGo)
				r.Outcome, r.Stop = OutcomeStop, StopUnmatched
				r.Reasons = []StopReason{{
					Point: PointBase, Func: &FuncID{"extension/src/a.ts", 9}, Outer: &FuncID{"extension/src/a.ts", 1},
					Detail: "the line's FN records are missing from the HEAD lcov",
				}}
				return r
			},
			want: header + measures + `stop      unmatched base function "extension/src/a.ts":9 outer "extension/src/a.ts":1 not run at HEAD: the line's FN records are missing from the HEAD lcov
coverage-compare: STOP unmatched
`,
		},
		{
			name: "an internal stop names the operation and the error",
			result: func() Result {
				return Result{
					Target: TargetGo, BaseArg: "x", Outcome: OutcomeStop, Stop: StopInternal,
					Reasons: []StopReason{{Detail: "the judging failed", Excerpt: "disk failure"}},
				}
			},
			want: `target    make coverage-compare BASE="x"
stop      internal: the judging failed
excerpt   "disk failure"
coverage-compare: STOP internal
`,
		},
		{
			name: "values from the user and from command output are quoted",
			result: func() Result {
				return Result{
					Target: TargetGo, BaseArg: "a\x1b[31m\"b", BaseCommit: "bad\ncommit", Toolchain: "go1\t2",
					Outcome: OutcomeStop, Stop: StopUnmatched,
					Reasons: []StopReason{{Point: PointHead, Func: &FuncID{"x\ry.go", 3}, Detail: "a HEAD function matches no remaining BASE function"}},
				}
			},
			want: `target    make coverage-compare BASE="a\x1b[31m\"b"
base      "bad\ncommit"
toolchain "go1\t2"
tests     base 0 head 0
files     base 0 head 0
functions base 0 head 0
stop      unmatched head function "x\ry.go":3: a HEAD function matches no remaining BASE function
coverage-compare: STOP unmatched
`,
		},
		{
			name: "an outcome outside the three is an internal stop",
			result: func() Result {
				return Result{Target: Target(9), BaseArg: "x", Outcome: Outcome(9), Stop: StopUnmatched}
			},
			want: `target    make <unknown target> BASE="x"
coverage-compare: STOP internal
`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := written(t, tc.result()); got != tc.want {
				t.Errorf("Write =\n%s\nwant\n%s", got, tc.want)
			}
		})
	}

	t.Run("the final line names each stop kind", func(t *testing.T) {
		for kind, word := range map[StopKind]string{
			StopUsage: "usage", StopNotAncestor: "not-ancestor", StopDirty: "dirty", StopToolchain: "toolchain",
			StopTestFailed: "test-failed", StopUnreadable: "unreadable", StopNothingMeasured: "nothing-measured",
			StopUnmatched: "unmatched", StopInternal: "internal", StopKind(99): "internal",
		} {
			got := written(t, Result{Target: TargetGo, Outcome: OutcomeStop, Stop: kind})
			if !strings.HasSuffix(got, "\ncoverage-compare: STOP "+word+"\n") {
				t.Errorf("stop %d: output ends %q, want the STOP %s line", kind, got, word)
			}
			if strings.Contains(got, "no compared function lost coverage") {
				t.Errorf("stop %d: output claims kept coverage", kind)
			}
		}
	})

	t.Run("a write failure is returned", func(t *testing.T) {
		failure := errors.New("closed")
		if err := Write(failingWriter{failure}, Result{}); !errors.Is(err, failure) {
			t.Errorf("Write error = %v, want the writer's failure", err)
		}
	})
}

// failingWriter fails every write.
type failingWriter struct{ err error }

func (w failingWriter) Write([]byte) (int, error) { return 0, w.err }

func TestBoundedQuote(t *testing.T) {
	exact := strings.Repeat("a", excerptLimit)
	if got := boundedQuote(exact, false); got != `"`+exact+`"` {
		t.Errorf("an excerpt at the limit was changed: %d bytes", len(got))
	}
	over := "S" + strings.Repeat("a", excerptLimit-1) + "E"
	if got, want := boundedQuote(over, false), `"`+over[:excerptLimit]+`" (truncated)`; got != want {
		t.Errorf("start-kept excerpt = %.20q..., want the first %d bytes", got, excerptLimit)
	}
	if got, want := boundedQuote(over, true), `"`+over[1:]+`" (truncated)`; got != want {
		t.Errorf("end-kept excerpt = %.20q..., want the last %d bytes", got, excerptLimit)
	}
	// A three-byte rune that straddles the limit is dropped whole.
	straddle := strings.Repeat("a", excerptLimit-1) + "あ" + "z"
	if got := boundedQuote(straddle, false); got != `"`+strings.Repeat("a", excerptLimit-1)+`" (truncated)` {
		t.Errorf("start-kept excerpt cut inside a rune: %.20q...", got[len(got)-20:])
	}
	straddleEnd := "z" + "あ" + strings.Repeat("a", excerptLimit-1)
	if got := boundedQuote(straddleEnd, true); got != `"`+strings.Repeat("a", excerptLimit-1)+`" (truncated)` {
		t.Errorf("end-kept excerpt cut inside a rune: %.20q...", got)
	}

	t.Run("a test-failed excerpt keeps its end in the output", func(t *testing.T) {
		r := Result{Target: TargetGo, Outcome: OutcomeStop, Stop: StopTestFailed, Reasons: []StopReason{{Excerpt: over}}}
		if got := written(t, r); !strings.Contains(got, `excerpt   "`+over[1:]+`" (truncated)`) {
			t.Error("the test-failed excerpt does not keep its end")
		}
	})
}

func TestExitCode(t *testing.T) {
	for outcome, want := range map[Outcome]int{OutcomePass: 0, OutcomeFail: 1, OutcomeStop: 3, Outcome(9): 3} {
		if got := ExitCode(outcome); got != want {
			t.Errorf("ExitCode(%d) = %d, want %d", outcome, got, want)
		}
	}
}
