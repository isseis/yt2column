//go:build test

package main

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/isseis/yt2column/internal/doctestutil"
)

// The store listing and the locale catalogs it copies values from.
const storeListingPath = repositoryRoot + "/docs/store/chrome_web_store.md"

// localeMessages is the part of extension/static/_locales/<locale>/messages.json
// this guard reads.
type localeMessages map[string]struct {
	Message string `json:"message"`
}

// storeListingKeys are the catalog keys the store listing's table holds, once
// each.
var storeListingKeys = []string{"extName", "extNameDev", "extDescription"}

// TestStoreListingMatchesMessages checks that the store listing's table copies
// extension/static/_locales/{ja,en}/messages.json: the table names the three
// keys once each, each value cell is exactly one code span, and each value
// equals the message of the same key in the locale's catalog. The subtests
// drive the same check with documents that must be refused and assert the
// refusal each time, so every property of the document that the check
// promises fails for a document that breaks it.
func TestStoreListingMatchesMessages(t *testing.T) {
	catalogs := readLocaleCatalogs(t)
	checkStoreListing(t, doctestutil.ReadDoc(t, filepath.FromSlash(storeListingPath)), catalogs)

	good := storeListingRows(catalogs)
	for _, tc := range []struct {
		name string
		doc  string
		want string
	}{
		{"wrong value", storeListingTable(withRow(good, 0, func(row []string) []string { row[1] = "`wrong`"; return row })), "but messages.json has"},
		{"wrong English value", storeListingTable(withRow(good, 0, func(row []string) []string { row[2] = "`wrong`"; return row })), "but messages.json has"},
		{"missing row", storeListingTable(append(cloneRows(good[:1]), good[2:]...)), `no row for "extNameDev"`},
		{"duplicate row", storeListingTable(append(cloneRows(good), good[0])), "appears more than once"},
		{"unknown key", storeListingTable(append(cloneRows(good), []string{"`extOther`", "`a`", "`b`"})), `unknown key "extOther"`},
		{"cell is not a code span", storeListingTable(withRow(good, 0, func(row []string) []string {
			row[1] = catalogs["ja"]["extName"].Message
			return row
		})), "is not one code span"},
		{"cell is two code spans", storeListingTable(withRow(good, 0, func(row []string) []string { row[1] = "`a` `b`"; return row })), "is not one code span"},
		{"short row", storeListingTable(withRow(good, 1, func(row []string) []string { return row[:2] })), "has 2 cells, want 3"},
		{"no key table", "a document without the table\n", "no table with a キー header"},
		{"no 英語 column", "| キー | 日本語 |\n|---|---|\n| `extName` | `x` |\n", "has no 日本語 and 英語 columns"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			recorder := &failureRecorder{TB: t}
			checkStoreListing(recorder, tc.doc, catalogs)
			if !recorder.failed {
				t.Errorf("checkStoreListing accepted a document it should refuse")
				return
			}
			if !strings.Contains(recorder.reason, tc.want) {
				t.Errorf("refusal does not say %q; it says %q", tc.want, recorder.reason)
			}
		})
	}
}

// failureRecorder records that checkStoreListing refused a document, and the
// first reason it gave, so a subtest passes when the check catches the
// document's defect for the expected reason.
type failureRecorder struct {
	testing.TB
	failed bool
	reason string
}

func (r *failureRecorder) Errorf(format string, args ...any) {
	if !r.failed {
		r.reason = fmt.Sprintf(format, args...)
	}
	r.failed = true
}

func (r *failureRecorder) Fatalf(format string, args ...any) { r.Errorf(format, args...) }

// checkStoreListing fails t unless doc's table copies the catalogs.
func checkStoreListing(t testing.TB, doc string, catalogs map[string]localeMessages) {
	t.Helper()
	table := doctestutil.FindTable(doc, "キー")
	if table == nil {
		t.Fatalf("the document has no table with a キー header")
		return
	}
	header := table[0]
	jaCol := doctestutil.ColumnIndex(header, "日本語")
	enCol := doctestutil.ColumnIndex(header, "英語")
	if jaCol < 0 || enCol < 0 {
		t.Fatalf("the table header has no 日本語 and 英語 columns: %v", header)
		return
	}
	want := make(map[string]bool, len(storeListingKeys))
	for _, key := range storeListingKeys {
		want[key] = false
	}
	for _, row := range table[1:] {
		if len(row) != len(header) {
			t.Errorf("row %v has %d cells, want %d", row, len(row), len(header))
			continue
		}
		key := doctestutil.StripBackticks(row[0])
		seen, known := want[key]
		switch {
		case !known:
			t.Errorf("unknown key %q", key)
			continue
		case seen:
			t.Errorf("key %q appears more than once", key)
			continue
		}
		want[key] = true

		for _, column := range []struct {
			locale string
			index  int
		}{{"ja", jaCol}, {"en", enCol}} {
			got, ok := codeSpanValue(row[column.index])
			if !ok {
				t.Errorf("cell %q for %s is not one code span", row[column.index], key)
				continue
			}
			entry, ok := catalogs[column.locale][key]
			if !ok {
				t.Errorf("%s messages.json has no %q key", column.locale, key)
				continue
			}
			if got != entry.Message {
				t.Errorf("%s for %s is %q, but messages.json has %q", column.locale, key, got, entry.Message)
			}
		}
	}
	for _, key := range storeListingKeys {
		if !want[key] {
			t.Errorf("no row for %q", key)
		}
	}
}

// readLocaleCatalogs reads the two catalogs the store listing copies.
func readLocaleCatalogs(t testing.TB) map[string]localeMessages {
	t.Helper()
	catalogs := make(map[string]localeMessages, 2)
	for _, locale := range []string{"ja", "en"} {
		path := filepath.Join(repositoryRoot, "extension", "static", "_locales", locale, "messages.json")
		var messages localeMessages
		if err := json.Unmarshal([]byte(doctestutil.ReadDoc(t, path)), &messages); err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		catalogs[locale] = messages
	}
	return catalogs
}

// storeListingRows returns the table's rows for catalogs, in the order the
// store listing lists them.
func storeListingRows(catalogs map[string]localeMessages) [][]string {
	rows := make([][]string, 0, len(storeListingKeys))
	for _, key := range storeListingKeys {
		rows = append(rows, []string{"`" + key + "`", "`" + catalogs["ja"][key].Message + "`", "`" + catalogs["en"][key].Message + "`"})
	}
	return rows
}

// storeListingTable renders a document whose only table is the guarded one.
func storeListingTable(rows [][]string) string {
	var b strings.Builder
	b.WriteString("| キー | 日本語 | 英語 |\n|---|---|---|\n")
	for _, row := range rows {
		b.WriteString("| ")
		b.WriteString(strings.Join(row, " | "))
		b.WriteString(" |\n")
	}
	return b.String()
}

// withRow returns a copy of rows with change applied to rows[index].
func withRow(rows [][]string, index int, change func([]string) []string) [][]string {
	out := cloneRows(rows)
	out[index] = change(out[index])
	return out
}

// cloneRows copies rows so a change to one case does not reach another.
func cloneRows(rows [][]string) [][]string {
	out := make([][]string, len(rows))
	for i, row := range rows {
		out[i] = append([]string(nil), row...)
	}
	return out
}

// codeSpanValue returns the text of a table cell that is exactly one code span
// and reports whether the cell has that form.
func codeSpanValue(cell string) (string, bool) {
	if len(cell) < 3 || cell[0] != '`' || cell[len(cell)-1] != '`' {
		return "", false
	}
	inner := cell[1 : len(cell)-1]
	if strings.Contains(inner, "`") {
		return "", false
	}
	return inner, true
}
