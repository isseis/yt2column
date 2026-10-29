//go:build test

package doctestutil

import "strings"

// MarkdownTables returns every table of doc as rows of cells. Each table is a
// slice of rows, and each row a slice of trimmed cells. A separator row is
// dropped, and a blank or non-table line ends a table.
func MarkdownTables(doc string) [][][]string {
	var tables [][][]string
	var rows [][]string
	flush := func() {
		if len(rows) > 0 {
			tables = append(tables, rows)
			rows = nil
		}
	}
	for line := range strings.Lines(doc) {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "|") && strings.HasSuffix(trimmed, "|") {
			cells := splitTableRow(trimmed)
			if isTableSeparator(cells) {
				continue
			}
			rows = append(rows, cells)
			continue
		}
		flush()
	}
	flush()
	return tables
}

// splitTableRow splits one Markdown table line into trimmed cells. Cells must
// not contain an escaped pipe, which these documents do not use.
func splitTableRow(line string) []string {
	line = strings.TrimSuffix(strings.TrimPrefix(line, "|"), "|")
	parts := strings.Split(line, "|")
	cells := make([]string, 0, len(parts))
	for _, part := range parts {
		cells = append(cells, strings.TrimSpace(part))
	}
	return cells
}

// isTableSeparator reports whether cells are the "---" row under a header.
func isTableSeparator(cells []string) bool {
	dash := false
	for _, cell := range cells {
		if strings.Contains(cell, "-") {
			dash = true
		}
		if strings.Trim(cell, "-: ") != "" {
			return false
		}
	}
	return dash
}

// FindTable returns the first table of doc with a header cell equal to header,
// ignoring case and backticks, or nil when there is none.
func FindTable(doc, header string) [][]string {
	for _, table := range MarkdownTables(doc) {
		if len(table) == 0 {
			continue
		}
		for _, cell := range table[0] {
			if strings.EqualFold(StripBackticks(cell), header) {
				return table
			}
		}
	}
	return nil
}

// ColumnIndex returns the index of the header cell equal to name, ignoring
// case and backticks as FindTable does, or -1 when there is none.
func ColumnIndex(header []string, name string) int {
	for i, cell := range header {
		if strings.EqualFold(StripBackticks(cell), name) {
			return i
		}
	}
	return -1
}

// TableRow returns the first body row whose first cell equals first, ignoring
// backticks, or nil when there is none.
func TableRow(table [][]string, first string) []string {
	for _, row := range table[1:] {
		if len(row) > 0 && StripBackticks(row[0]) == first {
			return row
		}
	}
	return nil
}

// StripBackticks removes the code-span backticks around a table cell.
func StripBackticks(s string) string {
	return strings.Trim(s, "`")
}
