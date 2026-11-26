//go:build test

package main

import (
	"encoding/json"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/isseis/yt2column/internal/doctestutil"
)

// The store listing and the locale catalogs it copies values from.
const storeListingPath = repositoryRoot + "/docs/store/chrome_web_store.md"

// storeListingKeys are the catalog keys the store listing's table holds, once
// each.
var storeListingKeys = []string{"extName", "extNameDev", "extDescription"}

// localeMessages is the part of extension/static/_locales/<locale>/messages.json
// this guard reads.
type localeMessages map[string]struct {
	Message string `json:"message"`
}

// TestStoreListingMatchesMessages checks that the store listing's table copies
// extension/static/_locales/{ja,en}/messages.json: the table names the three
// keys once each, each value cell is exactly one code span, and each value
// equals the message of the same key in the locale's catalog. A table that
// leaves a key out, repeats it, or adds one fails rather than being repaired,
// and so does a cell that is not one code span.
func TestStoreListingMatchesMessages(t *testing.T) {
	doc := doctestutil.ReadDoc(t, filepath.FromSlash(storeListingPath))
	table := doctestutil.FindTable(doc, "キー")
	if table == nil {
		t.Fatalf("%s has no table with a キー header", storeListingPath)
	}
	header := table[0]
	jaCol := doctestutil.ColumnIndex(header, "日本語")
	enCol := doctestutil.ColumnIndex(header, "英語")
	if jaCol < 0 || enCol < 0 {
		t.Fatalf("%s table header has no 日本語 and 英語 columns: %v", storeListingPath, header)
	}

	catalogs := make(map[string]localeMessages, 2)
	for _, locale := range []string{"ja", "en"} {
		path := filepath.Join(repositoryRoot, "extension", "static", "_locales", locale, "messages.json")
		var messages localeMessages
		if err := json.Unmarshal([]byte(doctestutil.ReadDoc(t, path)), &messages); err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		catalogs[locale] = messages
	}

	seen := make(map[string]bool, len(storeListingKeys))
	for _, row := range table[1:] {
		if len(row) != len(header) {
			t.Errorf("%s row %v has %d cells, want %d", storeListingPath, row, len(row), len(header))
			continue
		}
		key := doctestutil.StripBackticks(row[0])
		if !slices.Contains(storeListingKeys, key) {
			t.Errorf("%s has the unknown key %q", storeListingPath, key)
			continue
		}
		if seen[key] {
			t.Errorf("%s has the key %q more than once", storeListingPath, key)
			continue
		}
		seen[key] = true

		for _, column := range []struct {
			locale string
			index  int
		}{{"ja", jaCol}, {"en", enCol}} {
			got, ok := codeSpanValue(row[column.index])
			if !ok {
				t.Errorf("%s cell %q for %s is not one code span", storeListingPath, row[column.index], key)
				continue
			}
			want := catalogs[column.locale][key].Message
			if got != want {
				t.Errorf("%s %s for %s is %q, but messages.json has %q", storeListingPath, column.locale, key, got, want)
			}
		}
	}
	for _, key := range storeListingKeys {
		if !seen[key] {
			t.Errorf("%s has no row for %q", storeListingPath, key)
		}
	}
}

// codeSpanValue returns the text of a table cell that is exactly one code span
// and reports whether the cell has that form.
func codeSpanValue(cell string) (string, bool) {
	cell = strings.TrimSpace(cell)
	if len(cell) < 3 || cell[0] != '`' || cell[len(cell)-1] != '`' {
		return "", false
	}
	inner := cell[1 : len(cell)-1]
	if strings.Contains(inner, "`") {
		return "", false
	}
	return inner, true
}

// TestCodeSpanValue checks the cell form the store listing's table requires:
// one code span and nothing else, so a cell with no backticks or with more
// than one span is refused.
func TestCodeSpanValue(t *testing.T) {
	for _, tc := range []struct {
		cell  string
		want  string
		valid bool
	}{
		{"`a`", "a", true},
		{" `a` ", "a", true},
		{"`a` `b`", "", false},
		{"a", "", false},
		{"``", "", false},
		{"`a`b`", "", false},
	} {
		got, valid := codeSpanValue(tc.cell)
		if got != tc.want || valid != tc.valid {
			t.Errorf("codeSpanValue(%q) = %q, %v, want %q, %v", tc.cell, got, valid, tc.want, tc.valid)
		}
	}
}
