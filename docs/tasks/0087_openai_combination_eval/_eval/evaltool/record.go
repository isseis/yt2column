package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"regexp"
	"slices"
	"strings"
)

// errRecord reports an evaluation record whose copied tables do not equal the
// computed ones. Its message names the table.
var errRecord = errors.New("record does not match the computed tables")

// markerRE is the marker the record places right above a copied table:
// "<!-- check-record: <name> -->", or with a filter on one key column,
// "<!-- check-record: <name> <column>=<value> -->".
var markerRE = regexp.MustCompile(`^<!-- check-record: ([a-z0-9-]+)(?: ([a-z0-9_]+)=(\S+))? -->$`)

// recordTable is one copied table of the record.
type recordTable struct {
	Name, FilterCol, FilterVal string
	Line                       int
	Rows                       [][]string
}

// parseRecord reads every marked table of the record. A marker must be
// followed by a Markdown table: a header line, a separator line, and the rows
// up to the first line that is not a table line. Only the cells are kept; the
// header, written in Japanese, is not compared.
func parseRecord(r io.Reader) ([]recordTable, error) {
	var out []recordTable
	var cur *recordTable
	state := 0 // 0: outside, 1: header expected, 2: separator expected, 3: rows
	lineNo := 0
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1<<20) //nolint:mnd // a record line is short; 1 MiB bounds a broken one
	for scanner.Scan() {
		lineNo++
		line := scanner.Text()
		isRow := strings.HasPrefix(strings.TrimSpace(line), "|")
		switch state {
		case 0:
			if m := markerRE.FindStringSubmatch(line); m != nil {
				cur = &recordTable{Name: m[1], FilterCol: m[2], FilterVal: m[3], Line: lineNo}
				state = 1
			} else if strings.HasPrefix(strings.TrimSpace(line), "<!--") && strings.Contains(line, "check-record") {
				return nil, fmt.Errorf("%w: line %d: malformed marker", errRecord, lineNo)
			}
		case 1, 2: //nolint:mnd // the header and the separator lines
			if !isRow {
				return nil, fmt.Errorf("%w: line %d: marker %s is not followed by a table", errRecord, cur.Line, cur.Name)
			}
			state++
		default:
			if isRow {
				cur.Rows = append(cur.Rows, recordCells(line))
				continue
			}
			out = append(out, *cur)
			cur, state = nil, 0
			if m := markerRE.FindStringSubmatch(line); m != nil {
				cur = &recordTable{Name: m[1], FilterCol: m[2], FilterVal: m[3], Line: lineNo}
				state = 1
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	switch {
	case state == 3: //nolint:mnd // a table that runs to the end of the file
		out = append(out, *cur)
	case state != 0:
		return nil, fmt.Errorf("%w: marker %s at the end of the record has no table", errRecord, cur.Name)
	}
	return out, nil
}

// recordCells returns a record row's cells with one pair of surrounding
// backquotes removed, so an identifier may be written as code.
func recordCells(line string) []string {
	cells := splitMarkdownRow(line)
	for i, c := range cells {
		if len(c) >= 2 && strings.HasPrefix(c, "`") && strings.HasSuffix(c, "`") {
			cells[i] = c[1 : len(c)-1]
		}
	}
	return cells
}

// checkRecord compares the record's copied tables with the computed ones: the
// marker names must be the computed table names, a name may be repeated only
// with distinct filters, and across a name's markers every computed row must
// appear exactly once with equal cells.
func checkRecord(rec []recordTable, computed []*table) error {
	seen := map[string]bool{}
	byName := map[string][]recordTable{}
	for _, rt := range rec {
		id := rt.Name + " " + rt.FilterCol + "=" + rt.FilterVal
		if seen[id] {
			return fmt.Errorf("%w: table %s: duplicate marker (line %d)", errRecord, rt.Name, rt.Line)
		}
		seen[id] = true
		byName[rt.Name] = append(byName[rt.Name], rt)
	}
	for _, name := range sortedKeys(byName) {
		if _, ok := findTable(computed, name); !ok {
			return fmt.Errorf("%w: table %s: marker for a table no tool computes", errRecord, name)
		}
	}
	for _, t := range computed {
		rts, ok := byName[t.Name]
		if !ok {
			return fmt.Errorf("%w: table %s: no marker in the record", errRecord, t.Name)
		}
		if err := compareTable(t, rts); err != nil {
			return err
		}
	}
	return nil
}

// compareTable compares one computed table with its copies in the record.
func compareTable(t *table, rts []recordTable) error {
	want := map[string][]string{}
	for _, row := range t.Rows {
		key := strings.Join(row[:t.Keys], "\t")
		if _, dup := want[key]; dup {
			return fmt.Errorf("%w: table %s: computed row %q appears twice", errRecord, t.Name, key)
		}
		want[key] = row
	}
	got := map[string]bool{}
	for _, rt := range rts {
		col := -1
		if rt.FilterCol != "" {
			col = slices.Index(t.Columns[:t.Keys], rt.FilterCol)
			if col < 0 {
				return fmt.Errorf("%w: table %s: filter column %s is not a key column", errRecord, t.Name, rt.FilterCol)
			}
		}
		for _, row := range rt.Rows {
			if len(row) != len(t.Columns) {
				return fmt.Errorf("%w: table %s: a row has %d cells, want %d (line %d)", errRecord, t.Name, len(row), len(t.Columns), rt.Line)
			}
			key := strings.Join(row[:t.Keys], "\t")
			if col >= 0 && row[col] != rt.FilterVal {
				return fmt.Errorf("%w: table %s: row %q is outside the marker's filter", errRecord, t.Name, key)
			}
			w, ok := want[key]
			if !ok {
				return fmt.Errorf("%w: table %s: row %q is not computed", errRecord, t.Name, key)
			}
			if got[key] {
				return fmt.Errorf("%w: table %s: row %q appears twice", errRecord, t.Name, key)
			}
			got[key] = true
			if !slices.Equal(row, w) {
				return fmt.Errorf("%w: table %s: row %q differs", errRecord, t.Name, key)
			}
		}
	}
	for key := range want {
		if !got[key] {
			return fmt.Errorf("%w: table %s: row %q is missing", errRecord, t.Name, key)
		}
	}
	return nil
}

// runCheckRecord implements "check-record": it reads the record and the table
// files the other subcommands wrote, compares them, and prints the SHA-256 of
// every input.
func runCheckRecord(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("check-record", flag.ContinueOnError)
	recordPath := fs.String("record", "", "04_evaluation.md")
	if err := fs.Parse(args); err != nil {
		return err
	}
	inputs := append([]string{*recordPath}, fs.Args()...)
	var computed []*table
	for _, p := range fs.Args() {
		ts, err := readParsed(p, readTables)
		if err != nil {
			return err
		}
		for _, t := range ts {
			if _, dup := findTable(computed, t.Name); dup {
				return fmt.Errorf("%w: table %s: computed twice", errRecord, t.Name)
			}
			computed = append(computed, t)
		}
	}
	rec, err := readParsed(*recordPath, parseRecord)
	if err != nil {
		return err
	}
	for _, p := range inputs {
		data, err := os.ReadFile(p) //nolint:gosec // paths come from the operator's arguments
		if err != nil {
			return err
		}
		sum := sha256.Sum256(data)
		if _, err := fmt.Fprintf(stdout, "sha256 %s %s\n", hex.EncodeToString(sum[:]), p); err != nil {
			return err
		}
	}
	if err := checkRecord(rec, computed); err != nil {
		return err
	}
	_, err = fmt.Fprintf(stdout, "check-record: %d tables match\n", len(computed))
	return err
}
