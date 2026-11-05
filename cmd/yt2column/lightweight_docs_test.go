//go:build test

package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/isseis/yt2column/internal/doctestutil"
)

// Paths from cmd/yt2column to the lightweight process's documents.
const (
	requirementsProcessPath = "../../docs/dev/conventions/requirements_process.md"
	lightweightSpecPath     = "../../.claude/commands/_lib/lightweight-spec.md"
)

// Names of the _context.md rows that hold the lightweight process's markers.
const (
	specMarkerRow   = "Lightweight spec marker"
	revertMarkerRow = "Lightweight revert marker"
)

// contextCodeSpan returns the value of the _context.md row named name: the
// code span its value cell starts with, before the cell's description. It
// fails t when the cell does not start with a code span that is not empty.
func contextCodeSpan(t *testing.T, name string) string {
	t.Helper()
	cell := contextRow(t, name)[1]
	span, ok := firstCodeSpan(cell)
	if !ok || span == "" || !strings.HasPrefix(cell, "`"+span+"`") {
		t.Fatalf("the _context.md %q row's value %q does not start with a code span", name, cell)
	}
	return span
}

// lightweightConditions is what the guard tests read of the conditions table
// of requirements_process.md: the paths of its exclude and exclude-absent
// rows, in order.
type lightweightConditions struct {
	excludes, absents []string
}

// readLightweightConditions reads the conditions table: the one fenced text
// block directly after the one heading doctestutil.LightweightConditionsHeading
// outside code blocks. It fails t when the heading or the block is missing or
// repeated, when a row is repeated or of no known kind or an exclude row is
// not one path, and when there is no exclude row, so a table the script could not read is
// never read here as an empty one.
func readLightweightConditions(t *testing.T, doc string) lightweightConditions {
	t.Helper()
	heading := doctestutil.LightweightConditionsHeading
	n := 0
	for _, h := range doctestutil.HeadingsOutsideBlocks(doc) {
		if strings.TrimSpace(h) == heading {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("requirements_process.md has %d %q headings outside code blocks, want 1", n, heading)
	}
	section := doctestutil.SectionOutsideBlocks(doc, heading)
	if first := strings.TrimSpace(section); !strings.HasPrefix(first, "```text\n") {
		t.Fatalf("%q is not followed directly by a text code block", heading)
	}
	blocks := doctestutil.FencedBlocks(section)
	if len(blocks) != 1 || !blocks[0].Closed {
		t.Fatalf("%q has %d code blocks (the first closed: %t), want one closed block",
			heading, len(blocks), len(blocks) > 0 && blocks[0].Closed)
	}
	var table lightweightConditions
	seen := map[string]struct{}{}
	for _, line := range blocks[0].Lines {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			t.Fatalf("the conditions table has a blank row")
		}
		key := strings.Join(fields, " ")
		if _, dup := seen[key]; dup {
			t.Fatalf("the conditions table repeats the row %q", key)
		}
		seen[key] = struct{}{}
		switch fields[0] {
		case "limit", "tool":
		case "exclude", "exclude-absent":
			if len(fields) != 2 {
				t.Fatalf("the conditions table row %q is not one path", strings.TrimSpace(line))
			}
			if fields[0] == "exclude" {
				table.excludes = append(table.excludes, fields[1])
			} else {
				table.absents = append(table.absents, fields[1])
			}
		default:
			t.Fatalf("the conditions table row %q is of no known kind", strings.TrimSpace(line))
		}
	}
	if len(table.excludes) == 0 {
		t.Fatal("the conditions table has no exclude row")
	}
	return table
}

// errNoEntry reports that a path of the conditions table names no entry of
// the repository, compared case-sensitively.
var errNoEntry = errors.New("no such entry")

// repoEntry reports whether rel, a path from the repository root as the
// conditions table writes it (a trailing "/" for a directory), names an entry
// of the work tree, and whether that entry is a directory. Each element is
// compared with the names its parent directory lists, so the match is
// case-sensitive even on a file system that is not, such as APFS, where
// os.Stat would find "makefile" for "Makefile". It reads the work tree, not
// git's index: a local untracked file counts as an entry, so a stray go.work
// fails an exclude-absent row here, and an exclude path that exists only
// locally passes here but fails in CI's clean checkout.
func repoEntry(rel string) (isDir bool, err error) {
	clean := strings.TrimSuffix(rel, "/")
	if clean == "" || strings.HasPrefix(clean, "/") {
		return false, fmt.Errorf("%q is not a path from the repository root", rel)
	}
	dir := repositoryRoot
	elems := strings.Split(clean, "/")
	for i, elem := range elems {
		if elem == "" || elem == "." || elem == ".." {
			return false, fmt.Errorf("%q has the element %q", rel, elem)
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			return false, err
		}
		var found os.DirEntry
		for _, e := range entries {
			if e.Name() == elem {
				found = e
				break
			}
		}
		if found == nil {
			return false, errNoEntry
		}
		if i < len(elems)-1 && !found.IsDir() {
			return false, fmt.Errorf("%q: %q is not a directory", rel, elem)
		}
		isDir = found.IsDir()
		dir = filepath.Join(dir, elem)
	}
	return isDir, nil
}

// TestLightweightExcludedPaths checks the paths of the conditions table
// against the repository: every exclude path names an entry, a directory
// exactly when it ends in "/", and every exclude-absent path names none. A
// misspelled exclude path would match no change, so the script would let a
// change to the path it meant pass.
func TestLightweightExcludedPaths(t *testing.T) {
	table := readLightweightConditions(t, doctestutil.ReadDoc(t, requirementsProcessPath))
	for _, path := range table.excludes {
		isDir, err := repoEntry(path)
		switch {
		case err != nil:
			t.Errorf("exclude %s: %v", path, err)
		case isDir != strings.HasSuffix(path, "/"):
			t.Errorf("exclude %s: the entry is a directory: %t, but the path ends in \"/\": %t",
				path, isDir, strings.HasSuffix(path, "/"))
		}
	}
	for _, path := range table.absents {
		if _, err := repoEntry(path); !errors.Is(err, errNoEntry) {
			t.Errorf("exclude-absent %s: want no such entry, got error %v", path, err)
		}
	}
}

// lightweightSpecReadCalls are the gh api calls lightweight-spec.md may make:
// the listing, the user's login, and the read of the one chosen comment's
// body, which is the spec the session works from.
var lightweightSpecReadCalls = []string{
	"gh api --paginate ",
	"gh api user --jq .login",
	"gh api repos/<owner>/<repo>/issues/comments/<id> --jq .body",
}

// lightweightListingKeys are the keys each kind of object the marker listing
// prints may hold. None of them carries a body.
var lightweightListingKeys = map[string][]string{
	"spec":   {"kind", "id", "url", "created_at", "updated_at"},
	"revert": {"kind", "id", "url", "created_at", "updated_at"},
}

// lightweightListingFixtures is the fake response for the marker listing. Its
// items separate each condition of the filter: the author's login, the
// author's type, each marker as the whole first line with and without a CR,
// a marker followed by more text, whitespace, or a second CR, or preceded by
// a space, a marker after a blank first line or on a later line, and empty
// and null bodies. Each listed object must hold every key of its kind, since
// the spec comment is chosen by created_at and id.
func lightweightListingFixtures(spec, revert string) map[string]listingFixture {
	const user, bot = "User", "Bot"
	return map[string]listingFixture{
		"issues/<issue>/comments": {
			source:   map[string]string{"id": "id", "url": "html_url", "created_at": "created_at", "updated_at": "updated_at"},
			keys:     lightweightListingKeys,
			required: lightweightListingKeys,
			items: []listingItem{
				{user, fakeUser, spec + "\n" + marked("The spec."), "spec", "", notMention},
				{user, fakeUser, spec + "\r\n" + marked("Edited on GitHub.") + "\r\n", "spec", "", notMention},
				{user, fakeUser, spec, "spec", "", notMention},
				{user, fakeUser, revert + "\n" + marked("Returned."), "revert", "", notMention},
				{user, fakeUser, revert + "\r\n" + marked("Returned."), "revert", "", notMention},
				// Another account's marker comments, a Bot named like the
				// user included, are never listed.
				{user, "other-user", spec + "\n" + marked("Not the user's."), "", "", notMention},
				{user, "other-user", revert + "\n" + marked("Not the user's."), "", "", notMention},
				{bot, fakeUser, spec + "\n" + marked("A Bot named like the user."), "", "", notMention},
				{bot, fakeUser, revert + "\n" + marked("A Bot named like the user."), "", "", notMention},
				{"Organization", fakeUser, spec + "\n" + marked("Neither a User nor a Bot."), "", "", notMention},
				// The marker must be the whole first line.
				{user, fakeUser, spec + marked("More text.") + "\n" + marked("Body."), "", "", notMention},
				{user, fakeUser, revert + " " + marked("More text."), "", "", notMention},
				{user, fakeUser, " " + spec + "\n" + marked("Body."), "", "", notMention},
				{user, fakeUser, spec + " \n" + marked("Trailing space."), "", "", notMention},
				{user, fakeUser, spec + "\t\n" + marked("Trailing tab."), "", "", notMention},
				{user, fakeUser, spec + "\r\r\n" + marked("Two CRs."), "", "", notMention},
				{user, fakeUser, "\n" + spec + "\n" + marked("Blank first line."), "", "", notMention},
				{user, fakeUser, marked("First line.") + "\n" + spec, "", "", notMention},
				{user, fakeUser, marked("A plain comment."), "", "", notMention},
				{user, fakeUser, "", "", "", notMention},
				{user, fakeUser, nil, "", "", notMention},
			},
		},
	}
}

// TestLightweightSpecListing runs the marker listing of lightweight-spec.md
// on its fake response: it lists the user's own marker comments only, of the
// right kind, with their id, URL, and times, and prints no body. Every gh api
// call of the document must be one of the known reads. checkListings runs on
// the whole document, so its count makes the listing the document's only one,
// and the listing must sit in its own section.
func TestLightweightSpecListing(t *testing.T) {
	doc := doctestutil.ReadDoc(t, lightweightSpecPath)
	section := requireOutsideBlocksSection(t, "_lib/lightweight-spec.md", doc, doctestutil.LightweightListingHeading)
	if !paginatedListing.MatchString(section) {
		t.Errorf("_lib/lightweight-spec.md %q holds no listing", doctestutil.LightweightListingHeading)
	}
	spec, revert := contextCodeSpan(t, specMarkerRow), contextCodeSpan(t, revertMarkerRow)
	checkListings(t, "_lib/lightweight-spec.md", doc, lightweightListingFixtures(spec, revert), lightweightSpecReadCalls,
		"<spec-marker>", spec, "<revert-marker>", revert)
}

// TestLightweightSpecTemplateMarker checks that the first line of the spec
// comment template is the spec marker _context.md defines, so a comment
// written from the template is one the listing finds.
func TestLightweightSpecTemplateMarker(t *testing.T) {
	doc := doctestutil.ReadDoc(t, lightweightSpecPath)
	section := requireOutsideBlocksSection(t, "_lib/lightweight-spec.md", doc, doctestutil.LightweightSpecFormatHeading)
	var templates []doctestutil.FencedBlock
	for _, block := range doctestutil.FencedBlocks(section) {
		if block.Info == "markdown" {
			templates = append(templates, block)
		}
	}
	if len(templates) != 1 || len(templates[0].Lines) == 0 {
		t.Fatalf("%q has %d markdown code blocks, want one that is not empty", doctestutil.LightweightSpecFormatHeading, len(templates))
	}
	got := strings.TrimRight(templates[0].Lines[0], "\r\n")
	if want := contextCodeSpan(t, specMarkerRow); got != want {
		t.Errorf("the spec comment template starts with %q, want the spec marker %q", got, want)
	}
}

// requireOutsideBlocksSection returns the section of doc under heading, found
// outside code blocks, failing the test when it is missing or empty.
func requireOutsideBlocksSection(t *testing.T, name, doc, heading string) string {
	t.Helper()
	section := doctestutil.SectionOutsideBlocks(doc, heading)
	if strings.TrimSpace(section) == "" {
		t.Fatalf("%s has no section %q", name, heading)
	}
	return section
}
