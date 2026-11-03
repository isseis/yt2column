//go:build test

package coveragecompare

import (
	"fmt"
	"slices"
	"strings"
	"testing"
)

// tableOf builds a function table from rows.
func tableOf(point Point, entries ...funcEntry) funcTable {
	table := newFuncTable(point)
	table.entries = append(table.entries, entries...)
	table.sortEntries()
	return table
}

// row is a function table row with a range and no record.
func row(file string, first, last int) funcEntry {
	return funcEntry{ID: FuncID{File: file, Line: first}, End: last}
}

// joinLines joins source lines into a file's text.
func joinLines(lines ...string) string {
	return strings.Join(lines, "\n") + "\n"
}

// threeFuncs is a file with A (3-4), B (6-7), and C (9-10).
var threeFuncs = joinLines(
	"package x",
	"",
	"func A() {",
	"}",
	"",
	"func B() {",
	"}",
	"",
	"func C() {",
	"}",
)

// describeClassification renders a classification for comparison.
func describeClassification(c classification) (deleted, pairs, unmatched []string) {
	for _, entry := range c.deleted {
		deleted = append(deleted, fmt.Sprintf("%s:%d", entry.ID.File, entry.ID.Line))
	}
	for _, pair := range c.pairs {
		pairs = append(pairs, fmt.Sprintf("%s:%d>%d", pair.base.ID.File, pair.base.ID.Line, pair.head.ID.Line))
	}
	for _, reason := range c.unmatched {
		unmatched = append(unmatched, fmt.Sprintf("%v %s:%d", reason.Point, reason.Func.File, reason.Func.Line))
	}
	return deleted, pairs, unmatched
}

func TestClassify(t *testing.T) {
	const x = "x.go"
	tests := []struct {
		name          string
		baseText      map[string]string
		base          []funcEntry
		head          []funcEntry
		diff          []string
		wantDeleted   []string
		wantPairs     []string
		wantUnmatched []string
	}{
		{
			name:     "deleting a function in the middle shifts the later function's lines",
			baseText: map[string]string{x: threeFuncs},
			base:     []funcEntry{row(x, 3, 4), row(x, 6, 7), row(x, 9, 10)},
			head:     []funcEntry{row(x, 3, 4), row(x, 6, 7)},
			diff: []string{
				"diff --git a/x.go b/x.go", "--- a/x.go", "+++ b/x.go",
				"@@ -5,3 +4,0 @@", "-", "-func B() {", "-}",
			},
			wantDeleted: []string{"x.go:6"},
			wantPairs:   []string{"x.go:3>3", "x.go:9>6"},
		},
		{
			name:     "a deletion that takes the previous function's closing brace is moved to an accepted position",
			baseText: map[string]string{x: threeFuncs},
			base:     []funcEntry{row(x, 3, 4), row(x, 6, 7), row(x, 9, 10)},
			head:     []funcEntry{row(x, 3, 4), row(x, 6, 7)},
			diff: []string{
				"diff --git a/x.go b/x.go", "--- a/x.go", "+++ b/x.go",
				"@@ -4,3 +3,0 @@", "-}", "-", "-func B() {",
			},
			wantDeleted: []string{"x.go:6"},
			wantPairs:   []string{"x.go:3>3", "x.go:9>6"},
		},
		{
			name: "deleting a whole file deletes every function in it",
			base: []funcEntry{row(x, 3, 4), row(x, 6, 7)},
			diff: []string{
				"diff --git a/x.go b/x.go", "deleted file mode 100644", "--- a/x.go", "+++ /dev/null",
				"@@ -1,2 +0,0 @@", "-package x", "-",
			},
			wantDeleted: []string{"x.go:3", "x.go:6"},
		},
		{
			name:     "a rewritten line inside the range is unmatched",
			baseText: map[string]string{x: threeFuncs},
			base:     []funcEntry{row(x, 3, 4)},
			head:     []funcEntry{row(x, 3, 4)},
			diff: []string{
				"diff --git a/x.go b/x.go", "--- a/x.go", "+++ b/x.go",
				"@@ -4,1 +4,1 @@", "-}", "+} // changed",
			},
			wantUnmatched: []string{"base x.go:3", "head x.go:3"},
		},
		{
			name: "deleting part of the range is unmatched",
			baseText: map[string]string{x: joinLines(
				"package x", "", "func D() {", "\ta()", "\tb()", "}",
			)},
			base: []funcEntry{row(x, 3, 6)},
			head: []funcEntry{row(x, 3, 5)},
			diff: []string{
				"diff --git a/x.go b/x.go", "--- a/x.go", "+++ b/x.go",
				"@@ -4,1 +3,0 @@", "-\ta()",
			},
			wantUnmatched: []string{"base x.go:3", "head x.go:3"},
		},
		{
			name: "an addition inside the range is unmatched",
			base: []funcEntry{row(x, 3, 6)},
			head: []funcEntry{row(x, 3, 7)},
			diff: []string{
				"diff --git a/x.go b/x.go", "--- a/x.go", "+++ b/x.go",
				"@@ -4,0 +5,1 @@", "+\tc()",
			},
			wantUnmatched: []string{"base x.go:3", "head x.go:3"},
		},
		{
			name: "an addition right before the first line or after the last line is outside the range",
			base: []funcEntry{row(x, 3, 6)},
			head: []funcEntry{row(x, 4, 7)},
			diff: []string{
				"diff --git a/x.go b/x.go", "--- a/x.go", "+++ b/x.go",
				"@@ -2,0 +3,1 @@", "+// a comment",
				"@@ -6,0 +8,1 @@", "+",
			},
			wantPairs: []string{"x.go:3>4"},
		},
		{
			name:          "a function only HEAD has is unmatched",
			base:          []funcEntry{row(x, 3, 4)},
			head:          []funcEntry{row(x, 3, 4), row(x, 6, 7)},
			wantPairs:     []string{"x.go:3>3"},
			wantUnmatched: []string{"head x.go:6"},
		},
		{
			name: "a renamed file is a deletion and a HEAD-only file",
			base: []funcEntry{row("old.go", 3, 4)},
			head: []funcEntry{row("new.go", 3, 4)},
			diff: []string{
				"diff --git a/old.go b/old.go", "deleted file mode 100644", "--- a/old.go", "+++ /dev/null",
				"@@ -1,1 +0,0 @@", "-package x",
				"diff --git a/new.go b/new.go", "new file mode 100644", "--- /dev/null", "+++ b/new.go",
				"@@ -0,0 +1,1 @@", "+package x",
			},
			wantDeleted:   []string{"old.go:3"},
			wantUnmatched: []string{"head new.go:3"},
		},
		{
			name: "an unrecorded function of a new file no test loads is HEAD-only",
			base: []funcEntry{row(x, 3, 4)},
			head: []funcEntry{row(x, 3, 4), row("fresh.go", 3, 5)},
			diff: []string{
				"diff --git a/fresh.go b/fresh.go", "new file mode 100644", "--- /dev/null", "+++ b/fresh.go",
				"@@ -0,0 +1,1 @@", "+package x",
			},
			wantPairs:     []string{"x.go:3>3"},
			wantUnmatched: []string{"head fresh.go:3"},
		},
		{
			name: "changes outside every range (an import, a package variable, a comment before a function) are ignored",
			baseText: map[string]string{x: joinLines(
				"package x", "", `import "os"`, "", "var v = 1", "", "// F does f.", "func F() {", "}",
			)},
			base: []funcEntry{row(x, 8, 9)},
			head: []funcEntry{row(x, 8, 9)},
			diff: []string{
				"diff --git a/x.go b/x.go", "--- a/x.go", "+++ b/x.go",
				"@@ -3,1 +3,1 @@", `-import "os"`, `+import "io"`,
				"@@ -5,1 +5,1 @@", "-var v = 1", "+var v = 2",
				"@@ -7,1 +7,1 @@", "-// F does f.", "+// F does f, again.",
			},
			wantPairs: []string{"x.go:8>8"},
		},
		{
			name:      "a function the BASE report does not record still pairs with its HEAD partner",
			base:      []funcEntry{row(x, 3, 4)},
			head:      []funcEntry{{ID: FuncID{x, 3}, End: 4, Recorded: true, Cov: Coverage{Unit: UnitFunctions, Covered: 1, Total: 1}}},
			wantPairs: []string{"x.go:3>3"},
		},
		{
			name: "identical functions side by side leave the deleted one undecided",
			baseText: map[string]string{x: joinLines(
				"const x = 1", "f(() => {", "  g();", "});", "f(() => {", "  g();", "});", "h();",
			)},
			base: []funcEntry{row(x, 2, 4), row(x, 5, 7)},
			head: []funcEntry{row(x, 2, 4)},
			diff: []string{
				"diff --git a/x.go b/x.go", "--- a/x.go", "+++ b/x.go",
				"@@ -5,3 +4,0 @@", "-f(() => {", "-  g();", "-});",
			},
			wantDeleted:   []string{"x.go:5"},
			wantPairs:     []string{"x.go:2>2"},
			wantUnmatched: []string{"base x.go:2"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			src := &fakeSource{at: PointBase, files: tc.baseText}
			got, err := classify(tableOf(PointBase, tc.base...), tableOf(PointHead, tc.head...), parseTestDiff(t, tc.diff...), src)
			if err != nil {
				t.Fatalf("classify: %v", err)
			}
			deleted, pairs, unmatched := describeClassification(got)
			if !slices.Equal(deleted, tc.wantDeleted) {
				t.Errorf("deleted = %v, want %v", deleted, tc.wantDeleted)
			}
			if !slices.Equal(pairs, tc.wantPairs) {
				t.Errorf("pairs = %v, want %v", pairs, tc.wantPairs)
			}
			if !slices.Equal(unmatched, tc.wantUnmatched) {
				t.Errorf("unmatched = %v, want %v", unmatched, tc.wantUnmatched)
			}
		})
	}

	t.Run("a removed line that does not match the BASE source stops as unreadable", func(t *testing.T) {
		src := &fakeSource{at: PointBase, files: map[string]string{x: threeFuncs}}
		patch := parseTestDiff(t,
			"diff --git a/x.go b/x.go", "--- a/x.go", "+++ b/x.go",
			"@@ -6,1 +5,0 @@", "-func Other() {",
		)
		_, err := classify(tableOf(PointBase, row(x, 3, 4)), tableOf(PointHead), patch, src)
		requireStop(t, err, PointNone, InputDiff)
	})
}

func TestClassifyRange(t *testing.T) {
	// The range is lines 10 to 20.
	tests := []struct {
		name  string
		hunks []baseHunk
		want  rangeClass
	}{
		{"no hunk", nil, rangeRemains},
		{"a deletion that covers the range", []baseHunk{{Start: 9, Removed: 13}}, rangeDeleted},
		{"a deletion of the range exactly", []baseHunk{{Start: 10, Removed: 11}}, rangeDeleted},
		{"a deletion of the last line only", []baseHunk{{Start: 20, Removed: 1}}, rangePartlyDeleted},
		{"a deletion that starts before the range and ends inside it", []baseHunk{{Start: 8, Removed: 3}}, rangePartlyDeleted},
		{"a rewrite of the first line", []baseHunk{{Start: 10, Removed: 1, Added: 1}}, rangeRewritten},
		{"a rewrite that covers the range", []baseHunk{{Start: 9, Removed: 13, Added: 1}}, rangeRewritten},
		{"a rewrite before a partial deletion", []baseHunk{{Start: 12, Removed: 1, Added: 1}, {Start: 15, Removed: 1}}, rangeRewritten},
		{"a partial deletion before an addition", []baseHunk{{Start: 12, Removed: 1}, {Start: 15, Added: 1}}, rangePartlyDeleted},
		{"an addition after the first line", []baseHunk{{Start: 10, Added: 1}}, rangeAddedInside},
		{"an addition after the line before the last", []baseHunk{{Start: 19, Added: 1}}, rangeAddedInside},
		{"an addition right before the first line", []baseHunk{{Start: 9, Added: 1}}, rangeRemains},
		{"an addition after the last line", []baseHunk{{Start: 20, Added: 1}}, rangeRemains},
		{"a rewrite right before the first line", []baseHunk{{Start: 9, Removed: 1, Added: 2}}, rangeRemains},
		{"a deletion right after the last line", []baseHunk{{Start: 21, Removed: 2}}, rangeRemains},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := (lineMapping{Hunks: tc.hunks}).classifyRange(10, 20); got != tc.want {
				t.Errorf("classifyRange = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestPlaceDeletions(t *testing.T) {
	const x = "x.go"
	deleteB := func(start int) lineMapping {
		return lineMapping{Hunks: []baseHunk{{Start: start, Removed: 3}}}
	}
	tests := []struct {
		name      string
		mapping   lineMapping
		entries   []funcEntry
		wantStart int
		wantStop  bool
	}{
		{
			name:      "git's accepted position is kept",
			mapping:   deleteB(5),
			entries:   []funcEntry{row(x, 3, 4), row(x, 6, 7), row(x, 9, 10)},
			wantStart: 5,
		},
		{
			name:      "an unaccepted position moves to the earliest accepted one",
			mapping:   deleteB(4),
			entries:   []funcEntry{row(x, 3, 4), row(x, 6, 7), row(x, 9, 10)},
			wantStart: 5,
		},
		{
			name:      "with no accepted position the hunk stays",
			mapping:   lineMapping{Hunks: []baseHunk{{Start: 4, Removed: 1}}},
			entries:   []funcEntry{row(x, 3, 10)},
			wantStart: 4,
		},
		{
			name: "the block does not move next to another hunk",
			// An addition after line 7 keeps the block at 4-6 from moving
			// to 5-7, the only position that would be accepted.
			mapping: lineMapping{Hunks: []baseHunk{
				{Start: 4, Removed: 3},
				{Start: 7, Added: 1},
			}},
			entries:   []funcEntry{row(x, 3, 4), row(x, 6, 7), row(x, 9, 10)},
			wantStart: 4,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			src := &fakeSource{at: PointBase, files: map[string]string{x: threeFuncs}}
			mapping := tc.mapping
			reasons, err := placeDeletions(src, x, &mapping, tc.entries)
			if err != nil {
				t.Fatalf("placeDeletions: %v", err)
			}
			if (len(reasons) > 0) != tc.wantStop {
				t.Errorf("reasons = %+v, want stop %v", reasons, tc.wantStop)
			}
			if got := mapping.Hunks[0].Start; got != tc.wantStart {
				t.Errorf("hunk start = %d, want %d", got, tc.wantStart)
			}
		})
	}

	t.Run("the window covers every position that deletes the same text", func(t *testing.T) {
		lines := splitLines(threeFuncs)
		first, last := deleteB(5).slideWindow(0, lines)
		if first != 4 || last != 6 {
			t.Errorf("window = %d..%d, want 4..6", first, last)
		}
	})
}
