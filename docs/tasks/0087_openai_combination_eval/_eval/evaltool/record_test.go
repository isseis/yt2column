package main

import (
	"errors"
	"strings"
	"testing"
)

// recordComputed is the computed side of the check-record tests: a
// two-key table, a one-key table, and a table without rows.
func recordComputed() []*table {
	runs := newTable("runs", 2, "config", "article", "usd")
	runs.add("C-sol", "a-1", "0.100000")
	runs.add("A-sol", "a-1", "0.200000")
	sp := newTable("spend", 1, "scope", "total_usd")
	sp.add("record", "0.300000")
	empty := newTable("excluded", 1, "run", "bound_usd")
	return []*table{runs, sp, empty}
}

// goodRecord copies the three tables, splitting runs by config, with headers
// other than the column names and notes around them, which are not compared.
const goodRecord = `# Evaluation record

A note the check does not compare.

<!-- check-record: runs config=C-sol -->
| Config | Article | Estimated cost |
|---|---|---|
| ` + "`C-sol`" + ` | a-1 | 0.100000 |

<!-- check-record: runs config=A-sol -->
| Config | Article | Estimated cost |
|---|---|---|
| A-sol | a-1 | 0.200000 |

<!-- check-record: spend -->
| Scope | Total |
|---|---|
| record | 0.300000 |

<!-- check-record: excluded -->
| Run | Bound |
|---|---|
`

// TestCheckRecord succeeds when every copied value equals the computed one,
// and fails, naming the table, on a differing value, a missing marker, and a
// duplicate marker.
func TestCheckRecord(t *testing.T) {
	check := func(text string) error {
		rec, err := parseRecord(strings.NewReader(text))
		if err != nil {
			return err
		}
		return checkRecord(rec, recordComputed())
	}
	if err := check(goodRecord); err != nil {
		t.Fatalf("matching record: %v", err)
	}
	for name, c := range map[string]struct{ record, table string }{
		"value differs": {strings.Replace(goodRecord, "0.200000", "0.200001", 1), "runs"},
		// The table without rows shows a missing or repeated marker that no
		// row comparison could.
		"missing marker":   {strings.Replace(goodRecord, "<!-- check-record: excluded -->", "", 1), "excluded"},
		"duplicate marker": {goodRecord + "\n<!-- check-record: excluded -->\n| a | b |\n|---|---|\n", "excluded"},
		"row missing":      {strings.Replace(goodRecord, "| A-sol | a-1 | 0.200000 |\n", "", 1), "runs"},
		// Every row once, but A-sol's row under the C-sol marker.
		"row outside filter": {strings.Replace(strings.Replace(goodRecord, "| A-sol | a-1 | 0.200000 |\n", "", 1),
			"| 0.100000 |\n", "| 0.100000 |\n| A-sol | a-1 | 0.200000 |\n", 1), "runs"},
		"extra marker": {goodRecord + "\n<!-- check-record: other -->\n| a |\n|---|\n| b |\n", "other"},
	} {
		err := check(c.record)
		// The message naming the table is the contract (plan step 1-1: each
		// failure names its table), so the wording is checked as well as the
		// sentinel.
		if !errors.Is(err, errRecord) || !strings.Contains(err.Error(), "table "+c.table+":") {
			t.Errorf("%s: err = %v; want errRecord naming table %s", name, err, c.table)
		}
	}
}

// TestTablesRoundTrip reads back what writeTables writes.
func TestTablesRoundTrip(t *testing.T) {
	var b strings.Builder
	if err := writeTables(&b, recordComputed()...); err != nil {
		t.Fatal(err)
	}
	got, err := readTables(strings.NewReader(b.String()))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 || got[0].Keys != 2 || len(got[0].Rows) != 2 || got[1].Rows[0][1] != "0.300000" {
		t.Errorf("round trip = %+v", got)
	}
	bad := newTable("x", 1, "a")
	bad.add("tab\there")
	if err := writeTables(&b, bad); !errors.Is(err, errTable) {
		t.Errorf("a cell with a tab: err = %v", err)
	}
}
