//go:build test

package main

import (
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"
)

// readmeEnPath is the path from cmd/yt2column to the English README, which
// mktrans.md keeps in sync with README.md.
const readmeEnPath = "../../README.en.md"

// syncMarkerLine is the sync marker mktrans.md writes on the first line of
// README.en.md. The repository's object format is SHA-1, so the ID is 40
// lowercase hexadecimal characters; a change of format changes this test.
var syncMarkerLine = regexp.MustCompile(`^<!-- translated-from: README\.md ([0-9a-f]{40}) -->$`)

var (
	errNoSyncMarker     = errors.New("the first line is not a sync marker")
	errSecondSyncMarker = errors.New("a sync marker appears after the first line")
	errStaleSyncMarker  = errors.New("the sync marker names another README.md")
)

// parseSyncMarker returns the blob ID the sync marker on the first line of
// en names. It rejects a first line that is not exactly a marker and any later
// line that holds one, inside a code block or not.
func parseSyncMarker(en string) (string, error) {
	first, rest, _ := strings.Cut(en, "\n")
	m := syncMarkerLine.FindStringSubmatch(first)
	if m == nil {
		return "", errNoSyncMarker
	}
	if strings.Contains(rest, "<!-- translated-from:") {
		return "", errSecondSyncMarker
	}
	return m[1], nil
}

// gitBlobID returns the ID git gives a blob holding content
// (git hash-object). .gitattributes checks both READMEs out with LF line
// endings whatever core.autocrlf says, so the bytes in the work tree are the
// bytes git stores.
func gitBlobID(content []byte) string {
	h := sha1.New()
	fmt.Fprintf(h, "blob %d\x00", len(content))
	h.Write(content)
	return hex.EncodeToString(h.Sum(nil))
}

// checkReadmeSync reports whether en's sync marker names the blob of readme,
// that is, whether README.en.md was last translated from this README.md.
func checkReadmeSync(readme []byte, en string) error {
	id, err := parseSyncMarker(en)
	if err != nil {
		return err
	}
	if want := gitBlobID(readme); id != want {
		return fmt.Errorf("%w: marker %s, README.md %s", errStaleSyncMarker, id, want)
	}
	return nil
}

// TestReadmeSyncMarker checks that README.en.md carries the README.md beside
// it: its first line is the one sync marker, naming the blob of README.md.
// It runs in make test, in CI's doc-guard job for every pull request, and in
// the pre-commit hook for the two READMEs, so a README.md change cannot be
// committed or merged without the README.en.md sync, whoever makes it. While
// README.en.md does not exist (before the README becomes bilingual) it skips;
// once it exists, the README structure test fails when it is missing.
func TestReadmeSyncMarker(t *testing.T) {
	readme := []byte(readDoc(t, readmePath))
	en, err := os.ReadFile(readmeEnPath)
	if errors.Is(err, os.ErrNotExist) {
		t.Skip("README.en.md does not exist; nothing to check")
	}
	if err != nil {
		t.Fatalf("read README.en.md: %v", err)
	}
	if err := checkReadmeSync(readme, string(en)); err != nil {
		t.Fatalf("README.en.md is not in sync with README.md (%v); run /mktrans README.md and commit README.en.md together with README.md", err)
	}
}

// TestCheckReadmeSync covers the marker forms and the comparison with the
// blob of README.md.
func TestCheckReadmeSync(t *testing.T) {
	readme := []byte("# 見出し\n\n本文\n")
	id := gitBlobID(readme)
	body := "\n# Heading\n\nText\n"
	tests := []struct {
		name string
		en   string
		want error
	}{
		{"in_sync", "<!-- translated-from: README.md " + id + " -->" + body, nil},
		{"stale", "<!-- translated-from: README.md " + strings.Repeat("0", 40) + " -->" + body, errStaleSyncMarker},
		{"no_marker", "# Heading\n", errNoSyncMarker},
		{"short_id", "<!-- translated-from: README.md " + id[:39] + " -->" + body, errNoSyncMarker},
		{"long_id", "<!-- translated-from: README.md " + id + "0 -->" + body, errNoSyncMarker},
		{"uppercase_hex", "<!-- translated-from: README.md " + strings.ToUpper(id) + " -->" + body, errNoSyncMarker},
		{"no_readme_word", "<!-- translated-from: " + id + " -->" + body, errNoSyncMarker},
		{"extra_space", "<!-- translated-from:  README.md " + id + " -->" + body, errNoSyncMarker},
		{"trailing_space", "<!-- translated-from: README.md " + id + " --> " + body, errNoSyncMarker},
		{"carriage_return", "<!-- translated-from: README.md " + id + " -->\r" + body, errNoSyncMarker},
		{"marker_on_line_2", "\n<!-- translated-from: README.md " + id + " -->" + body, errNoSyncMarker},
		{"prose_names_the_marker_word", "<!-- translated-from: README.md " + id + " -->" + body + "The `translated-from:` comment.\n", nil},
		{
			"second_marker_in_code_block",
			"<!-- translated-from: README.md " + id + " -->" + body + "```\n<!-- translated-from: README.md " + id + " -->\n```\n",
			errSecondSyncMarker,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkReadmeSync(readme, tt.en)
			if tt.want == nil {
				if err != nil {
					t.Fatalf("checkReadmeSync() = %v, want nil", err)
				}
				return
			}
			if !errors.Is(err, tt.want) {
				t.Fatalf("checkReadmeSync() = %v, want %v", err, tt.want)
			}
		})
	}
}

// TestGitBlobIDMatchesGit pins gitBlobID to the IDs git hash-object prints
// for known contents.
func TestGitBlobIDMatchesGit(t *testing.T) {
	for _, tt := range []struct{ content, want string }{
		{"", "e69de29bb2d1d6434b8b29ae775ad8c2e48c5391"},
		{"hello\n", "ce013625030ba8dba906f756967f9e9ca394464a"},
	} {
		if got := gitBlobID([]byte(tt.content)); got != tt.want {
			t.Errorf("gitBlobID(%q) = %s, want %s", tt.content, got, tt.want)
		}
	}
}
