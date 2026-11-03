//go:build test

package coveragecompare

import (
	"reflect"
	"testing"
)

// goRow is a recorded row of x.go, lines 3-5, with statement coverage.
func goRow(covered, total int) funcEntry {
	return funcEntry{
		ID: FuncID{File: "x.go", Line: 3}, End: 5, Names: []string{"f"}, Recorded: true,
		Cov: Coverage{Unit: UnitStatements, Covered: covered, Total: total},
	}
}

// extRow is a recorded extension row whose line has total functions, ran of
// which ran.
func extRow(file string, line, end, ran, total int, names ...string) funcEntry {
	return funcEntry{
		ID: FuncID{File: file, Line: line}, End: end, Names: names, Recorded: true,
		Cov: Coverage{Unit: UnitFunctions, Covered: ran, Total: total},
	}
}

func TestCompareGo(t *testing.T) {
	unrecorded := goRow(0, 0)
	unrecorded.Recorded = false
	tests := []struct {
		name          string
		base, head    funcEntry
		wantLost      bool
		wantUnmatched bool
	}{
		{name: "a lower fraction is lost", base: goRow(3, 4), head: goRow(2, 4), wantLost: true},
		{name: "a drop that rounds to the same percentage is lost", base: goRow(1000, 1001), head: goRow(999, 1001), wantLost: true},
		{name: "the same fraction is kept", base: goRow(3, 4), head: goRow(3, 4)},
		{name: "a higher fraction is kept", base: goRow(3, 4), head: goRow(4, 4)},
		{name: "a HEAD partner with no statements has no HEAD record", base: goRow(1, 2), head: goRow(0, 0), wantUnmatched: true},
		{name: "a HEAD partner whose file is not in the HEAD profile cannot be compared", base: goRow(1, 2), head: unrecorded, wantUnmatched: true},
		{name: "a function that did not run at BASE is not compared", base: goRow(0, 2), head: unrecorded},
		{name: "a function with no statements is not compared", base: goRow(0, 0), head: unrecorded},
		{name: "a function the BASE profile does not record is not compared", base: unrecorded, head: goRow(0, 2)},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			lost, reasons := compareGo([]matchedPair{{base: tc.base, head: tc.head}})
			if (len(lost) > 0) != tc.wantLost {
				t.Errorf("lost = %+v, want lost %v", lost, tc.wantLost)
			}
			if tc.wantLost && len(lost) == 1 {
				want := LostFunc{ID: tc.base.ID, Names: tc.base.Names, Before: tc.base.Cov, After: tc.head.Cov}
				if !reflect.DeepEqual(lost[0], want) {
					t.Errorf("lost = %+v, want %+v", lost[0], want)
				}
			}
			if (len(reasons) > 0) != tc.wantUnmatched {
				t.Errorf("reasons = %+v, want unmatched %v", reasons, tc.wantUnmatched)
			}
		})
	}
}

func TestCompareExt(t *testing.T) {
	const file = "extension/src/a.ts"
	const missing = "extension/src/gone.ts"
	tests := []struct {
		name          string
		base, head    funcEntry
		headRows      []funcEntry // other HEAD rows, for the enclosing function
		headInReport  bool
		wantAfter     *Coverage // the lost record's after value
		wantUnmatched bool
		wantOuter     *FuncID
	}{
		{
			name: "a function whose run count drops to zero is lost",
			base: extRow(file, 5, 7, 1, 1, "f"), head: extRow(file, 5, 7, 0, 1, "f"), headInReport: true,
			wantAfter: &Coverage{Unit: UnitFunctions, Covered: 0, Total: 1},
		},
		{
			name: "fewer run functions on a line is lost",
			base: extRow(file, 5, 7, 2, 2, "a", "b"), head: extRow(file, 5, 7, 1, 2, "a", "b"), headInReport: true,
			wantAfter: &Coverage{Unit: UnitFunctions, Covered: 1, Total: 2},
		},
		{
			name: "a line with a function that did not run at BASE and does not run at HEAD is kept",
			base: extRow(file, 5, 7, 1, 2, "anonymous_1", "anonymous_2"), head: extRow(file, 5, 7, 1, 2, "anonymous_8", "anonymous_9"), headInReport: true,
		},
		{
			name: "more run functions on a line cannot be compared",
			base: extRow(file, 5, 7, 1, 2), head: extRow(file, 5, 7, 2, 2), headInReport: true,
			wantUnmatched: true,
		},
		{
			name: "a different number of functions on a line cannot be compared",
			base: extRow(file, 5, 7, 1, 2), head: extRow(file, 5, 7, 1, 3), headInReport: true,
			headRows:      []funcEntry{extRow(file, 1, 10, 0, 1)},
			wantUnmatched: true,
		},
		{
			name: "fewer functions on a line names the enclosing function that did not run",
			base: extRow(file, 5, 7, 1, 2), head: extRow(file, 5, 7, 0, 1), headInReport: true,
			headRows:      []funcEntry{extRow(file, 1, 10, 0, 1)},
			wantUnmatched: true, wantOuter: &FuncID{file, 1},
		},
		{
			name: "a line missing from the HEAD lcov names the enclosing function that did not run",
			base: extRow(file, 5, 7, 1, 1), head: row(file, 5, 7), headInReport: true,
			headRows:      []funcEntry{extRow(file, 1, 10, 0, 1), row(file, 3, 9)},
			wantUnmatched: true, wantOuter: &FuncID{file, 1},
		},
		{
			name: "a missing line inside a function that ran gives no enclosing function",
			base: extRow(file, 5, 7, 1, 1), head: row(file, 5, 7), headInReport: true,
			headRows:      []funcEntry{extRow(file, 1, 10, 1, 1)},
			wantUnmatched: true,
		},
		{
			name: "a file the HEAD lcov does not name ran nothing at HEAD",
			base: extRow(missing, 5, 7, 1, 2), head: row(missing, 5, 7),
			wantAfter: &Coverage{Unit: UnitFunctions, Covered: 0, Total: 2},
		},
		{
			name: "a file the HEAD lcov does not name keeps a line that ran nothing at BASE",
			base: extRow(missing, 5, 7, 0, 2), head: row(missing, 5, 7),
		},
		{
			name: "a line the BASE lcov does not record is not compared",
			base: row(file, 5, 7), head: extRow(file, 5, 7, 0, 1), headInReport: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			head := tableOf(PointHead, append([]funcEntry{tc.head}, tc.headRows...)...)
			if tc.headInReport {
				head.inReport[file] = struct{}{}
			}
			lost, reasons := compareExt(head, []matchedPair{{base: tc.base, head: tc.head}})
			switch {
			case tc.wantAfter == nil && len(lost) > 0:
				t.Errorf("lost = %+v, want none", lost)
			case tc.wantAfter != nil && len(lost) != 1:
				t.Errorf("lost = %+v, want one", lost)
			case tc.wantAfter != nil:
				want := LostFunc{ID: tc.base.ID, Names: tc.base.Names, Before: tc.base.Cov, After: *tc.wantAfter}
				if !reflect.DeepEqual(lost[0], want) {
					t.Errorf("lost = %+v, want %+v", lost[0], want)
				}
			}
			if (len(reasons) > 0) != tc.wantUnmatched {
				t.Fatalf("reasons = %+v, want unmatched %v", reasons, tc.wantUnmatched)
			}
			if tc.wantUnmatched && !reflect.DeepEqual(reasons[0].Outer, tc.wantOuter) {
				t.Errorf("outer = %v, want %v", reasons[0].Outer, tc.wantOuter)
			}
		})
	}
}

func TestRunState(t *testing.T) {
	unrecorded := goRow(0, 0)
	unrecorded.Recorded = false
	tests := []struct {
		name  string
		entry funcEntry
		want  RunState
	}{
		{"a Go function that ran", goRow(1, 2), RanAtBase},
		{"a Go function that did not run", goRow(0, 2), NotRunAtBase},
		{"a Go function with no statements", goRow(0, 0), NoStatements},
		{"a function the report does not record", unrecorded, NotRunAtBase},
		{"an extension line that ran", extRow("a.ts", 1, 2, 1, 2), RanAtBase},
		{"an extension line that did not run", extRow("a.ts", 1, 2, 0, 2), NotRunAtBase},
		{"a record with no unit", funcEntry{Recorded: true}, RanUnknown},
	}
	for _, tc := range tests {
		if got := runState(tc.entry); got != tc.want {
			t.Errorf("%s: runState = %v, want %v", tc.name, got, tc.want)
		}
	}
}
