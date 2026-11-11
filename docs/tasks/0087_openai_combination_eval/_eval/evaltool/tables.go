package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// table is one named output table: its column names, how many leading columns
// form a row's key, and its rows. check-record matches the evaluation record's
// copy of each table by the name, the key cells, and the value cells.
type table struct {
	Name    string
	Keys    int
	Columns []string
	Rows    [][]string
}

// errTable reports a table file or table value that does not match its form.
var errTable = errors.New("table does not match its form")

// add appends one row; it must have one cell per column.
func (t *table) add(cells ...string) {
	if len(cells) != len(t.Columns) {
		panic(fmt.Sprintf("table %s: row has %d cells, want %d", t.Name, len(cells), len(t.Columns)))
	}
	t.Rows = append(t.Rows, cells)
}

// newTable returns an empty table.
func newTable(name string, keys int, columns ...string) *table {
	return &table{Name: name, Keys: keys, Columns: columns}
}

// The lines that frame a table in the text form.
const (
	tableOpen  = "table "
	tableClose = "end"
)

// writeTables writes tables in the text form: "table <name> keys=<n>", a
// tab-separated header line, one tab-separated line per row, and "end". A cell
// may not hold a tab or a line break.
func writeTables(w io.Writer, tables ...*table) error {
	for _, t := range tables {
		if _, err := fmt.Fprintf(w, "%s%s keys=%d\n", tableOpen, t.Name, t.Keys); err != nil {
			return err
		}
		for _, row := range append([][]string{t.Columns}, t.Rows...) {
			for _, c := range row {
				if strings.ContainsAny(c, "\t\r\n") {
					return fmt.Errorf("%w: table %s: a cell holds a tab or line break", errTable, t.Name)
				}
			}
			if _, err := fmt.Fprintln(w, strings.Join(row, "\t")); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintln(w, tableClose); err != nil {
			return err
		}
	}
	return nil
}

// readTables reads the text form writeTables writes. A table name may appear
// once.
func readTables(r io.Reader) ([]*table, error) {
	var out []*table
	var cur *table
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1<<20) //nolint:mnd // a row is short; 1 MiB bounds a broken one
	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case cur == nil:
			name, keys, err := parseTableOpen(line)
			if err != nil {
				return nil, err
			}
			for _, t := range out {
				if t.Name == name {
					return nil, fmt.Errorf("%w: table %s appears twice", errTable, name)
				}
			}
			cur = &table{Name: name, Keys: keys}
		case cur.Columns == nil:
			cur.Columns = strings.Split(line, "\t")
			if cur.Keys > len(cur.Columns) {
				return nil, fmt.Errorf("%w: table %s has more keys than columns", errTable, cur.Name)
			}
		case line == tableClose:
			out = append(out, cur)
			cur = nil
		default:
			cells := strings.Split(line, "\t")
			if len(cells) != len(cur.Columns) {
				return nil, fmt.Errorf("%w: table %s: row has %d cells, want %d", errTable, cur.Name, len(cells), len(cur.Columns))
			}
			cur.Rows = append(cur.Rows, cells)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if cur != nil {
		return nil, fmt.Errorf("%w: table %s has no end line", errTable, cur.Name)
	}
	return out, nil
}

// parseTableOpen reads "table <name> keys=<n>".
func parseTableOpen(line string) (string, int, error) {
	rest, ok := strings.CutPrefix(line, tableOpen)
	if !ok {
		return "", 0, fmt.Errorf("%w: expected a table line, got %q", errTable, line)
	}
	name, keysField, ok := strings.Cut(rest, " ")
	keysText, ok2 := strings.CutPrefix(keysField, "keys=")
	if !ok || !ok2 || name == "" {
		return "", 0, fmt.Errorf("%w: table line %q", errTable, line)
	}
	keys, err := strconv.Atoi(keysText)
	if err != nil || keys < 1 {
		return "", 0, fmt.Errorf("%w: table line %q", errTable, line)
	}
	return name, keys, nil
}

// findTable returns the table with the name.
func findTable(tables []*table, name string) (*table, bool) {
	for _, t := range tables {
		if t.Name == name {
			return t, true
		}
	}
	return nil, false
}

// yesNo spells a boolean cell.
func yesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

// The key widths of the tables: config and article; config, article, and
// attempt (or criterion); and those with the stage.
const (
	keysArticle  = 2
	keysRun      = 3
	keysRunStage = 4
)

// none is the cell of a value that does not apply or does not exist.
const none = "-"
