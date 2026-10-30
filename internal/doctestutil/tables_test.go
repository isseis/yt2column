//go:build test

package doctestutil

import (
	"slices"
	"testing"
)

// TestMarkdownTables checks the table scan: a separator row is dropped, a blank
// or other line ends a table, and two tables of one document come back in
// order.
func TestMarkdownTables(t *testing.T) {
	const doc = "intro\n" +
		"| Flag | Meaning |\n" +
		"|---|---|\n" +
		"| `--out` | a file |\n" +
		"\n" +
		"between\n" +
		"| Name | Value |\n" +
		"| --- | --- |\n" +
		"| `A` | 1 |\n"
	want := [][][]string{
		{{"Flag", "Meaning"}, {"`--out`", "a file"}},
		{{"Name", "Value"}, {"`A`", "1"}},
	}
	if got := MarkdownTables(doc); !equalTables(got, want) {
		t.Errorf("MarkdownTables(doc) = %v, want %v", got, want)
	}
}

// equalTables compares two lists of tables cell by cell.
func equalTables(a, b [][][]string) bool {
	return slices.EqualFunc(a, b, func(x, y [][]string) bool {
		return slices.EqualFunc(x, y, slices.Equal)
	})
}

// TestFindTableAndTableRow checks the lookups: a header cell matches ignoring
// case and backticks, the first matching table and the first matching row come
// back, and a miss returns nil rather than an empty slice.
func TestFindTableAndTableRow(t *testing.T) {
	const doc = "| Variable | When unset |\n" +
		"|---|---|\n" +
		"| `A` | `1` |\n" +
		"| `B` | `2` |\n" +
		"| `B` | `9` |\n" +
		"\n" +
		"| `Other` |\n" +
		"|---|\n" +
		"| x |\n" +
		"\n" +
		"| Variable | When unset |\n" +
		"|---|---|\n" +
		"| `C` | `3` |\n"
	table := FindTable(doc, "variable")
	if table == nil {
		t.Fatal("FindTable(doc, \"variable\") = nil, want the table headed Variable")
	}
	// The first Variable table has four rows; the second one, whose only row
	// is C, has two.
	if len(table) != 4 {
		t.Errorf("FindTable kept %d rows, want the first Variable table's four", len(table))
	}
	if row := TableRow(table, "B"); !slices.Equal(row, []string{"`B`", "`2`"}) {
		t.Errorf("TableRow(table, \"B\") = %q, want the first of the two B rows", row)
	}
	if row := TableRow(table, "C"); row != nil {
		t.Errorf("TableRow(table, \"C\") = %q, want nil: C is only in the later table", row)
	}
	if other := FindTable(doc, "Other"); len(other) != 2 {
		t.Errorf("FindTable(doc, \"Other\") = %v, want the later table, whose header cell carries backticks", other)
	}
	if other := FindTable(doc, "Absent"); other != nil {
		t.Errorf("FindTable(doc, \"Absent\") = %v, want nil", other)
	}
}

// TestStripBackticks checks that every backtick at the ends of a cell is
// removed, as a code span of two backticks around a cell holding one requires,
// and that a backtick inside the cell stays.
func TestStripBackticks(t *testing.T) {
	for _, tc := range []struct{ cell, want string }{
		{"`--out`", "--out"},
		{"--out", "--out"},
		{"a`b", "a`b"},
		{"``a`b``", "a`b"},
	} {
		if got := StripBackticks(tc.cell); got != tc.want {
			t.Errorf("StripBackticks(%q) = %q, want %q", tc.cell, got, tc.want)
		}
	}
}

// TestColumnIndex checks the header lookup: a cell matches ignoring case and
// backticks, the first matching cell wins, and a miss is -1.
func TestColumnIndex(t *testing.T) {
	header := []string{"`reason`", "Status Code", "status code"}
	for _, tc := range []struct {
		name string
		want int
	}{
		{"reason", 0},
		{"status code", 1},
		{"extension shows", -1},
	} {
		if got := ColumnIndex(header, tc.name); got != tc.want {
			t.Errorf("ColumnIndex(header, %q) = %d, want %d", tc.name, got, tc.want)
		}
	}
}
