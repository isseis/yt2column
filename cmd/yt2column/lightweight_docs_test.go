//go:build test

package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

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
// rows, in order, and the values of its limit rows, keyed by what the row
// limits ("W added", "files").
type lightweightConditions struct {
	excludes, absents []string
	limits            map[string]string
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
	table := lightweightConditions{limits: map[string]string{}}
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
		case "limit":
			if len(fields) < 3 {
				t.Fatalf("the conditions table row %q has no value", strings.TrimSpace(line))
			}
			table.limits[strings.Join(fields[1:len(fields)-1], " ")] = fields[len(fields)-1]
		case "tool":
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

// lightweightSpecTemplate returns the lines of the spec comment template: the
// one markdown code block of lightweight-spec.md's format section, without
// their line endings.
func lightweightSpecTemplate(t *testing.T) []string {
	t.Helper()
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
	lines := make([]string, len(templates[0].Lines))
	for i, line := range templates[0].Lines {
		lines[i] = strings.TrimRight(line, "\r\n")
	}
	return lines
}

// TestLightweightSpecTemplateMarker checks that the first line of the spec
// comment template is the spec marker _context.md defines, so a comment
// written from the template is one the listing finds.
func TestLightweightSpecTemplateMarker(t *testing.T) {
	got := lightweightSpecTemplate(t)[0]
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

// lightweightCheckPath is the recheck script, from cmd/yt2column.
const lightweightCheckPath = "../../.claude/commands/_lib/lightweight-check.sh"

// lwCheckPrefix starts every line the recheck script prints.
const lwCheckPrefix = "lightweight-check: "

// lwTypeHeading is the spec comment heading the script reads the type under.
const lwTypeHeading = "### 類型"

// lwBranch is the fixture's lightweight branch.
const lwBranch = "octo-user/issue-1"

// lwTool is the deterministic command the tool-commit cases allow: lwtool -w
// appends the same lines to each file it is given and fails when one is not a
// file. lwOtherTool does the same but is on no tool row.
const (
	lwTool      = "lwtool"
	lwOtherTool = "othertool"
)

// lwBash is a bash the recheck script runs with.
type lwBash struct {
	name, path string
}

// lightweightBashes returns the bash on PATH and, when /bin/bash is a
// different bash 3.x (macOS), that one too, since /runissue runs the script
// with whichever bash the user's PATH gives. It fails t when bash is missing.
func lightweightBashes(t *testing.T) []lwBash {
	t.Helper()
	onPath, err := exec.LookPath("bash")
	if err != nil {
		t.Fatalf("bash is not on PATH: %v", err)
	}
	bashes := []lwBash{{"path-bash", onPath}}
	const system = "/bin/bash"
	out, err := exec.Command(system, "-c", "printf %s \"$BASH_VERSION\"").Output()
	if err != nil || !strings.HasPrefix(string(out), "3.") {
		return bashes
	}
	resolvedPath, errPath := filepath.EvalSymlinks(onPath)
	resolvedSystem, errSystem := filepath.EvalSymlinks(system)
	if errPath == nil && errSystem == nil && resolvedPath == resolvedSystem {
		return bashes
	}
	return append(bashes, lwBash{"bin-bash-3", system})
}

// lwFixture is one fixture repository: main holds the real recheck script and
// a conditions table, refs/remotes/origin/main points at it, and the
// lightweight branch forks from it at base. home, tmp, and gitTmp are the
// case's own empty HOME and TMPDIR directories (tmp is the script's TMPDIR,
// checked empty after each run); outside holds the spec and the script's copy,
// out of the repository; bin holds the test's tools.
type lwFixture struct {
	t                                     *testing.T
	repo, home, tmp, gitTmp, outside, bin string
	base                                  string
	conditions                            lightweightConditions
	excludedFile, excludedDir, absent     string
	limitFiles                            int
}

// lwFixtureFiles are the files of main besides the conditions document and
// the script, built from the limits so the cases can reach them exactly.
func lwFixtureFiles(f *lwFixture) map[string]string {
	files := map[string]string{
		"notes/long.txt":                  lwLines("long", 300),
		"notes/algo.txt":                  "b\nb\nc\nc\nc\nb\na\nb\n",
		"notes/space.txt":                 lwLines("space", 5),
		"notes/rename.txt":                lwLines("rename", min(f.limit("W added"), f.limit("W deleted"))),
		f.excludedFile:                    "excluded\n",
		f.excludedDir + "0001_old/old.md": "old\n",
	}
	for i := 1; i <= f.limitFiles+1; i++ {
		files[fmt.Sprintf("gen/f%02d.txt", i)] = "seed\n"
	}
	return files
}

// lwLines returns n lines "<prefix> <i>".
func lwLines(prefix string, n int) string {
	var b strings.Builder
	for i := 1; i <= n; i++ {
		fmt.Fprintf(&b, "%s %d\n", prefix, i)
	}
	return b.String()
}

// newLWFixture builds a fixture whose main holds doc as requirements_process.md
// (the real document when doc is ""). The limits and paths the cases use are
// read from the real document, as the script reads its copy.
func newLWFixture(t *testing.T, doc string) *lwFixture {
	t.Helper()
	realDoc := doctestutil.ReadDoc(t, requirementsProcessPath)
	if doc == "" {
		doc = realDoc
	}
	script, err := os.ReadFile(lightweightCheckPath)
	if err != nil {
		t.Fatalf("read the recheck script: %v", err)
	}
	f := &lwFixture{
		t: t, home: t.TempDir(), tmp: t.TempDir(), gitTmp: t.TempDir(), outside: t.TempDir(), bin: t.TempDir(),
		conditions: readLightweightConditions(t, realDoc),
	}
	f.repo = filepath.Join(t.TempDir(), "repo")
	f.limitFiles = f.limit("files")
	f.pickPaths()
	f.writeTools()
	f.gitIn(filepath.Dir(f.repo), "init", "-q", "-b", "main", f.repo)
	f.write("docs/dev/conventions/requirements_process.md", doc)
	f.write(".claude/commands/_lib/lightweight-check.sh", string(script))
	for path, content := range lwFixtureFiles(f) {
		f.write(path, content)
	}
	f.commit("base")
	f.git("remote", "add", "origin", filepath.Join(f.home, "no-such-remote"))
	f.git("update-ref", "refs/remotes/origin/main", "HEAD")
	f.git("switch", "-q", "-c", lwBranch)
	f.base = f.rev("HEAD")
	return f
}

// pickPaths chooses the excluded paths the exclusion cases change: the first
// exclude row that is a file (other than the two the fixture itself holds),
// docs/tasks/ (no lightweight change may write a task document), and the first
// exclude-absent row.
func (f *lwFixture) pickPaths() {
	f.t.Helper()
	own := map[string]struct{}{
		"docs/dev/conventions/requirements_process.md": {},
		".claude/commands/_lib/lightweight-check.sh":   {},
	}
	for _, path := range f.conditions.excludes {
		if _, skip := own[path]; !skip && !strings.HasSuffix(path, "/") && f.excludedFile == "" {
			f.excludedFile = path
		}
		if path == "docs/tasks/" {
			f.excludedDir = path
		}
	}
	if f.excludedFile == "" || f.excludedDir == "" || len(f.conditions.absents) == 0 {
		f.t.Fatalf("the conditions table lacks a file exclude row, the docs/tasks/ row, or an exclude-absent row")
	}
	f.absent = f.conditions.absents[0]
}

// limit returns the numeric value of the conditions table's limit row key.
func (f *lwFixture) limit(key string) int {
	f.t.Helper()
	value, ok := f.conditions.limits[key]
	if !ok {
		f.t.Fatalf("the conditions table has no %q limit", key)
	}
	n, err := strconv.Atoi(value)
	if err != nil {
		f.t.Fatalf("the conditions table's %q limit is %q, not a number", key, value)
	}
	return n
}

// writeTools writes lwtool and othertool into bin. Each appends 10 lines more
// than the M added limit, so a tool commit's lines exceed what a hand-written
// change may add.
func (f *lwFixture) writeTools() {
	f.t.Helper()
	tool := fmt.Sprintf(`#!/bin/sh
[ "$1" = -w ] || exit 2
shift
[ "$#" -gt 0 ] || exit 2
for f in "$@"; do
	[ -f "$f" ] || exit 1
	i=1
	while [ "$i" -le %d ]; do
		echo "generated $i" >>"$f"
		i=$((i + 1))
	done
done
`, f.limit("M added")+10)
	for _, name := range []string{lwTool, lwOtherTool} {
		if err := os.WriteFile(filepath.Join(f.bin, name), []byte(tool), 0o700); err != nil { //nolint:gosec // an executable test tool
			f.t.Fatalf("write %s: %v", name, err)
		}
	}
}

// env is the allowlisted environment of every git and script run: PATH (the
// test's tools first), the case's HOME and TMPDIR, and settings that keep the
// system configuration and prompts out. No variable of the test's own
// environment reaches git, so a GIT_DIR a pre-commit hook exports cannot point
// it at another repository.
func (f *lwFixture) env(tmp, path string) []string {
	if path == "" {
		path = f.bin + string(os.PathListSeparator) + os.Getenv("PATH")
	}
	return []string{
		"PATH=" + path, "HOME=" + f.home, "TMPDIR=" + tmp,
		"GIT_CONFIG_NOSYSTEM=1", "GIT_TERMINAL_PROMPT=0", "LC_ALL=C",
	}
}

// gitIn runs git in dir with the fixture's environment and a fixed author,
// failing t on an error, and returns its standard output.
func (f *lwFixture) gitIn(dir string, args ...string) string {
	f.t.Helper()
	return f.gitStdin(dir, "", args...)
}

func (f *lwFixture) gitStdin(dir, stdin string, args ...string) string {
	f.t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	cmd.Env = append(f.env(f.gitTmp, ""),
		"GIT_AUTHOR_NAME=Octo User", "GIT_AUTHOR_EMAIL=octo@example.invalid",
		"GIT_COMMITTER_NAME=Octo User", "GIT_COMMITTER_EMAIL=octo@example.invalid")
	cmd.Stdin = strings.NewReader(stdin)
	var stdout, stderr strings.Builder
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		f.t.Fatalf("git %v: %v: %s", args, err, stderr.String())
	}
	return stdout.String()
}

func (f *lwFixture) git(args ...string) string {
	f.t.Helper()
	return f.gitIn(f.repo, args...)
}

func (f *lwFixture) rev(name string) string {
	f.t.Helper()
	return strings.TrimSpace(f.git("rev-parse", "--verify", name))
}

// write writes content to path in the work tree, creating its directory.
func (f *lwFixture) write(path, content string) {
	f.t.Helper()
	full := filepath.Join(f.repo, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
		f.t.Fatalf("create the directory of %s: %v", path, err)
	}
	if err := os.WriteFile(full, []byte(content), 0o600); err != nil {
		f.t.Fatalf("write %s: %v", path, err)
	}
}

// read returns the content of path in the work tree.
func (f *lwFixture) read(path string) string {
	f.t.Helper()
	data, err := os.ReadFile(filepath.Join(f.repo, filepath.FromSlash(path)))
	if err != nil {
		f.t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

// commit stages every change and commits it with message.
func (f *lwFixture) commit(message string) {
	f.t.Helper()
	f.git("add", "-A")
	f.gitStdin(f.repo, message, "commit", "-q", "--allow-empty", "-F", "-")
}

// addLines writes a new file path of n lines.
func (f *lwFixture) addLines(path string, n int) {
	f.t.Helper()
	f.write(path, lwLines("added", n))
}

// deleteLines deletes the first n lines of notes/long.txt.
func (f *lwFixture) deleteLines(n int) {
	f.t.Helper()
	lines := strings.SplitAfter(f.read("notes/long.txt"), "\n")
	f.write("notes/long.txt", strings.Join(lines[n:], ""))
}

// runTool runs command, a tool of bin with its words, in the work tree.
func (f *lwFixture) runTool(command string) {
	f.t.Helper()
	words := strings.Fields(command)
	cmd := exec.Command(filepath.Join(f.bin, words[0]), words[1:]...) //nolint:gosec // a test tool with test-chosen arguments
	cmd.Dir = f.repo
	if out, err := cmd.CombinedOutput(); err != nil {
		f.t.Fatalf("%s: %v: %s", command, err, out)
	}
}

// toolCommit runs command and commits its changes as a tool commit recording
// it.
func (f *lwFixture) toolCommit(command string) {
	f.t.Helper()
	f.runTool(command)
	f.commit(lwToolMessage(command))
}

// lwToolMessage is a tool commit's message recording command.
func lwToolMessage(command string) string {
	return "refactor(issue-1): run the tool\n\nLightweight-Generated-By: " + command + "\n"
}

// genFiles returns the paths of the first n generated-file fixtures.
func genFiles(n int) string {
	paths := make([]string, n)
	for i := range n {
		paths[i] = fmt.Sprintf("gen/f%02d.txt", i+1)
	}
	return strings.Join(paths, " ")
}

// lwToolTable is the real document with its tool rows replaced by the one
// lwtool row the tool-commit cases allow.
func lwToolTable(t *testing.T) string {
	t.Helper()
	doc := doctestutil.ReadDoc(t, requirementsProcessPath)
	var out []string
	replaced := false
	for _, line := range strings.SplitAfter(doc, "\n") {
		if strings.HasPrefix(line, "tool ") {
			if !replaced {
				out = append(out, "tool "+lwTool+" -w\n")
				replaced = true
			}
			continue
		}
		out = append(out, line)
	}
	if !replaced {
		t.Fatal("requirements_process.md has no tool row to replace")
	}
	return strings.Join(out, "")
}

// lwEditTable returns the real document with the conditions table row old
// replaced by repl (which may be empty, removing the row, or two rows).
func lwEditTable(t *testing.T, old, repl string) string {
	t.Helper()
	doc := doctestutil.ReadDoc(t, requirementsProcessPath)
	if strings.Count(doc, "\n"+old+"\n") != 1 {
		t.Fatalf("requirements_process.md does not hold the row %q once", old)
	}
	return strings.Replace(doc, "\n"+old+"\n", "\n"+repl, 1)
}

// lightweightSpec is a spec comment written from the template, its type line
// (the first line that is not blank under the type heading) set to typ.
func lightweightSpec(t *testing.T, typ string) string {
	t.Helper()
	lines := lightweightSpecTemplate(t)
	heading := slices.Index(lines, lwTypeHeading)
	if heading < 0 {
		t.Fatalf("the spec comment template has no %q line", lwTypeHeading)
	}
	for i := heading + 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) != "" {
			lines[i] = typ
			return strings.Join(lines, "\n") + "\n"
		}
	}
	t.Fatalf("the spec comment template has no line under %q", lwTypeHeading)
	return ""
}

// lwRun is what one run of the recheck script printed and returned.
type lwRun struct {
	code   int
	lines  []string // stdout lines without lwCheckPrefix
	stdout string
	stderr string
}

func (r lwRun) String() string {
	return fmt.Sprintf("exit %d, stdout %q, stderr %q", r.code, r.stdout, r.stderr)
}

// verdict is the last line.
func (r lwRun) verdict() string {
	if len(r.lines) == 0 {
		return ""
	}
	return r.lines[len(r.lines)-1]
}

// field returns the value of key=value in the first line, the inputs line.
func (r lwRun) field(key string) string {
	if len(r.lines) == 0 {
		return ""
	}
	for word := range strings.FieldsSeq(r.lines[0]) {
		if value, ok := strings.CutPrefix(word, key+"="); ok {
			return value
		}
	}
	return ""
}

// lwRunOptions changes how check runs the script: in dir (a directory of the
// repository; its root when ""), and with PATH set to path (the default PATH
// when "").
type lwRunOptions struct {
	dir, path string
}

// check writes the script from refs/remotes/origin/main and spec into files
// outside the repository, as /runissue does, runs the script with bash, and
// checks that the run printed only prefixed lines, that its exit status
// matches its verdict, and that it left TMPDIR empty and no worktree behind.
func (f *lwFixture) check(bash lwBash, spec string, opts lwRunOptions) lwRun {
	f.t.Helper()
	script := filepath.Join(f.outside, "lightweight-check.sh")
	specFile := filepath.Join(f.outside, "spec.md")
	if err := os.WriteFile(script, []byte(f.git("show", "refs/remotes/origin/main:.claude/commands/_lib/lightweight-check.sh")), 0o600); err != nil {
		f.t.Fatalf("write the script: %v", err)
	}
	if err := os.WriteFile(specFile, []byte(spec), 0o600); err != nil {
		f.t.Fatalf("write the spec: %v", err)
	}
	dir := f.repo
	if opts.dir != "" {
		dir = filepath.Join(f.repo, opts.dir)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, bash.path, script, specFile) //nolint:gosec // bash with the test's script
	cmd.Dir = dir
	cmd.Env = f.env(f.tmp, opts.path)
	var stdout, stderr strings.Builder
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	run := lwRun{stdout: stdout.String(), stderr: stderr.String()}
	if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
		run.code = exitErr.ExitCode()
	} else if err != nil {
		f.t.Fatalf("run the script: %v", err)
	}
	for line := range strings.Lines(run.stdout) {
		rest, ok := strings.CutPrefix(strings.TrimSuffix(line, "\n"), lwCheckPrefix)
		if !ok {
			f.t.Errorf("the script printed a line without the prefix: %q", line)
			continue
		}
		run.lines = append(run.lines, rest)
	}
	word, reason, _ := strings.Cut(run.verdict(), " ")
	wantCode, known := map[string]int{"PASS": 0, "FAIL": 1, "STOP": 2}[word]
	if !known || (word == "PASS") != (reason == "") || run.code != wantCode {
		f.t.Errorf("the verdict %q and the exit status %d do not agree: %v", run.verdict(), run.code, run)
	}
	if entries, err := os.ReadDir(f.tmp); err != nil || len(entries) != 0 {
		f.t.Errorf("the script left TMPDIR with %d entries (error %v)", len(entries), err)
	}
	if n := strings.Count(f.git("worktree", "list", "--porcelain"), "worktree "); n != 1 {
		f.t.Errorf("git worktree list shows %d worktrees after the run, want the fixture's only", n)
	}
	return run
}

// checkInputs checks the inputs line of a run that got past the inputs: the
// type, the SHA-256 of the spec file's bytes, the fork commit, and the tree of
// HEAD.
func (f *lwFixture) checkInputs(run lwRun, typ, spec string) {
	f.t.Helper()
	sum := sha256.Sum256([]byte(spec))
	want := map[string]string{
		"type": typ, "spec-sha256": hex.EncodeToString(sum[:]),
		"base": f.base, "tree": f.rev("HEAD^{tree}"),
	}
	for key, value := range want {
		if got := run.field(key); got != value {
			f.t.Errorf("the inputs line has %s=%q, want %q: %v", key, got, value, run)
		}
	}
}

// lwCase is one fixture case: the branch it builds (on main holding table's
// document, the real one when table is nil), the spec type (or a whole spec),
// the verdict it expects, and lines the output must also hold.
type lwCase struct {
	name   string
	typ    string
	spec   func(t *testing.T) string
	table  func(t *testing.T) string
	branch func(f *lwFixture)
	want   string
	lines  func(f *lwFixture) []string
	after  func(f *lwFixture, bash lwBash, spec string, first lwRun)
}

// counts is the counts line for the given numbers and the type's limits.
func (f *lwFixture) counts(typ string, added, deleted, files int) string {
	f.t.Helper()
	return fmt.Sprintf("added=%d/%s deleted=%d/%s files=%d/%d", added, f.conditions.limits[typ+" added"],
		deleted, f.conditions.limits[typ+" deleted"], files, f.limitFiles)
}

func exceeds(kind string, n, limit int) string {
	return fmt.Sprintf("%s %d exceeds the limit %d", kind, n, limit)
}

// lightweightCheckCases are the cases of the design's unit-test table and the
// plan's additional cases.
func lightweightCheckCases() []lwCase {
	tools := lwToolTable
	return []lwCase{
		// Limits: each type at its limits passes, and one line or file over
		// fails with the reason and the value.
		{name: "W at its limits", typ: "W", branch: func(f *lwFixture) {
			f.addLines("notes/new.txt", f.limit("W added"))
			f.deleteLines(f.limit("W deleted"))
			f.commit("docs: w")
		}, want: "PASS", lines: func(f *lwFixture) []string {
			return []string{f.counts("W", f.limit("W added"), f.limit("W deleted"), 2)}
		}},
		{name: "W added over", typ: "W", branch: func(f *lwFixture) {
			f.addLines("notes/new.txt", f.limit("W added")+1)
			f.commit("docs: w")
		}, want: "FAIL added", lines: func(f *lwFixture) []string {
			return []string{exceeds("added", f.limit("W added")+1, f.limit("W added"))}
		}},
		{name: "W deleted over", typ: "W", branch: func(f *lwFixture) {
			f.deleteLines(f.limit("W deleted") + 1)
			f.commit("docs: w")
		}, want: "FAIL deleted", lines: func(f *lwFixture) []string {
			return []string{exceeds("deleted", f.limit("W deleted")+1, f.limit("W deleted"))}
		}},
		{name: "D at its limits", typ: "D", branch: func(f *lwFixture) {
			f.addLines("notes/new.txt", f.limit("D added"))
			f.deleteLines(f.limit("D deleted"))
			f.commit("refactor: d")
		}, want: "PASS", lines: func(f *lwFixture) []string {
			return []string{f.counts("D", f.limit("D added"), f.limit("D deleted"), 2)}
		}},
		{name: "D deleted over", typ: "D", branch: func(f *lwFixture) {
			f.deleteLines(f.limit("D deleted") + 1)
			f.commit("refactor: d")
		}, want: "FAIL deleted", lines: func(f *lwFixture) []string {
			return []string{exceeds("deleted", f.limit("D deleted")+1, f.limit("D deleted"))}
		}},
		{name: "D added over", typ: "D", branch: func(f *lwFixture) {
			f.addLines("notes/new.txt", f.limit("D added")+1)
			f.commit("refactor: d")
		}, want: "FAIL added", lines: func(f *lwFixture) []string {
			return []string{exceeds("added", f.limit("D added")+1, f.limit("D added"))}
		}},
		{name: "M at its added limit with many deletions", typ: "M", branch: func(f *lwFixture) {
			f.addLines("notes/new.txt", f.limit("M added"))
			f.deleteLines(lwMDeleted(f))
			f.commit("refactor: m")
		}, want: "PASS", lines: func(f *lwFixture) []string {
			return []string{f.counts("M", f.limit("M added"), lwMDeleted(f), 2)}
		}},
		{name: "M added over", typ: "M", branch: func(f *lwFixture) {
			f.addLines("notes/new.txt", f.limit("M added")+1)
			f.commit("refactor: m")
		}, want: "FAIL added", lines: func(f *lwFixture) []string {
			return []string{exceeds("added", f.limit("M added")+1, f.limit("M added"))}
		}},
		{name: "files at the limit", typ: "M", branch: func(f *lwFixture) {
			for i := range f.limitFiles {
				f.addLines(fmt.Sprintf("notes/new%02d.txt", i), 1)
			}
			f.commit("refactor: m")
		}, want: "PASS", lines: func(f *lwFixture) []string {
			return []string{f.counts("M", f.limitFiles, 0, f.limitFiles)}
		}},
		{name: "files over", typ: "M", branch: func(f *lwFixture) {
			for i := range f.limitFiles + 1 {
				f.addLines(fmt.Sprintf("notes/new%02d.txt", i), 1)
			}
			f.commit("refactor: m")
		}, want: "FAIL files", lines: func(f *lwFixture) []string {
			return []string{exceeds("files", f.limitFiles+1, f.limitFiles)}
		}},
		{name: "two limits over", typ: "W", branch: func(f *lwFixture) {
			if f.limit("W added") < f.limitFiles {
				f.t.Fatal("the W added limit is below the files limit, so the case cannot spread its lines over the files")
			}
			f.addLines("notes/new00.txt", f.limit("W added")+1-f.limitFiles)
			for i := 1; i <= f.limitFiles; i++ {
				f.addLines(fmt.Sprintf("notes/new%02d.txt", i), 1)
			}
			f.commit("docs: w")
		}, want: "FAIL added,files", lines: func(f *lwFixture) []string {
			return []string{
				exceeds("added", f.limit("W added")+1, f.limit("W added")),
				exceeds("files", f.limitFiles+1, f.limitFiles),
			}
		}},

		// Counting: a rename is a deletion and an addition of two files, a
		// whitespace-only change is a changed line, and BASE is the fork point
		// even after origin/main moves on.
		{name: "a rename counts both files and all lines", typ: "W", branch: func(f *lwFixture) {
			f.git("mv", "notes/rename.txt", "notes/renamed.txt")
			f.commit("docs: rename")
		}, want: "PASS", lines: func(f *lwFixture) []string {
			n := min(f.limit("W added"), f.limit("W deleted"))
			return []string{f.counts("W", n, n, 2)}
		}},
		{name: "a whitespace-only change counts", typ: "W", branch: func(f *lwFixture) {
			f.write("notes/space.txt", strings.ReplaceAll(f.read("notes/space.txt"), "\n", " \n"))
			f.commit("docs: space")
		}, want: "PASS", lines: func(f *lwFixture) []string {
			return []string{f.counts("W", 5, 5, 1)}
		}},
		{name: "BASE is the fork point after origin/main moves on", typ: "W", branch: func(f *lwFixture) {
			f.addLines("notes/new.txt", 1)
			f.commit("docs: w")
			f.git("switch", "-q", "main")
			f.addLines("notes/main.txt", f.limit("W added")+1)
			f.write(f.excludedFile, "changed on main\n")
			f.deleteLines(f.limit("W deleted") + 1)
			f.commit("main moves on")
			f.git("update-ref", "refs/remotes/origin/main", "HEAD")
			f.git("switch", "-q", lwBranch)
		}, want: "PASS", lines: func(f *lwFixture) []string {
			return []string{f.counts("W", 1, 0, 1)}
		}},

		// Excluded paths.
		{name: "an excluded file changes", typ: "W", branch: func(f *lwFixture) {
			f.write(f.excludedFile, "changed\n")
			f.commit("docs: excluded")
		}, want: "FAIL excluded", lines: func(f *lwFixture) []string {
			return []string{"excluded " + f.excludedFile}
		}},
		{name: "a file is added under docs/tasks/", typ: "W", branch: func(f *lwFixture) {
			f.addLines(f.excludedDir+"0002_new/01_requirements.md", 1)
			f.commit("docs: task")
		}, want: "FAIL excluded", lines: func(f *lwFixture) []string {
			return []string{"excluded " + f.excludedDir + "0002_new/01_requirements.md"}
		}},
		{name: "a rename out of an excluded directory", typ: "W", branch: func(f *lwFixture) {
			f.git("mv", f.excludedDir+"0001_old/old.md", "notes/old.md")
			f.commit("docs: move")
		}, want: "FAIL excluded", lines: func(f *lwFixture) []string {
			return []string{"excluded " + f.excludedDir + "0001_old/old.md"}
		}},
		{name: "a Japanese file name under an excluded directory", typ: "W", branch: func(f *lwFixture) {
			f.addLines(f.excludedDir+"0001_old/日本語.md", 1)
			f.commit("docs: japanese")
		}, want: "FAIL excluded", lines: func(f *lwFixture) []string {
			return []string{"excluded " + f.excludedDir + "0001_old/日本語.md"}
		}},
		{name: "an exclude-absent path is created", typ: "W", branch: func(f *lwFixture) {
			f.addLines(f.absent, 1)
			f.commit("build: absent")
		}, want: "FAIL excluded", lines: func(f *lwFixture) []string {
			return []string{"excluded " + f.absent}
		}},
		{name: "a path an excluded file is only a prefix of", typ: "W", branch: func(f *lwFixture) {
			f.addLines(f.excludedFile+".bak", 1)
			f.commit("docs: bak")
		}, want: "PASS"},
		{name: "the branch loosens the conditions table", typ: "W", branch: func(f *lwFixture) {
			const doc = "docs/dev/conventions/requirements_process.md"
			loose := strings.Replace(f.read(doc), "\nexclude "+doc+"\n", "\n", 1)
			loose = strings.Replace(loose, "\nlimit W added "+f.conditions.limits["W added"]+"\n", "\nlimit W added 100000\n", 1)
			if loose == f.read(doc) {
				f.t.Fatal("the loosened table is the same as the real one")
			}
			f.write(doc, loose)
			f.addLines("notes/new.txt", f.limit("W added"))
			f.commit("docs: loosen")
		}, want: "FAIL added,excluded", lines: func(*lwFixture) []string {
			return []string{"excluded docs/dev/conventions/requirements_process.md"}
		}},

		// Changes that cannot be counted.
		{name: "a binary file", typ: "W", branch: func(f *lwFixture) {
			f.write("notes/data.bin", "a\x00b\n")
			f.commit("docs: binary")
		}, want: "FAIL binary", lines: func(*lwFixture) []string { return []string{"binary notes/data.bin"} }},
		{name: "a binary file a nested .gitattributes marks as text", typ: "W", branch: func(f *lwFixture) {
			f.write("notes/.gitattributes", "*.bin diff\n")
			f.write("notes/data.bin", "a\x00b\n")
			f.commit("docs: binary")
		}, want: "FAIL binary", lines: func(*lwFixture) []string { return []string{"binary notes/data.bin"} }},
		{name: "a symbolic link", typ: "W", branch: func(f *lwFixture) {
			if err := os.Symlink("long.txt", filepath.Join(f.repo, "notes", "link")); err != nil {
				f.t.Fatalf("create the link: %v", err)
			}
			f.commit("docs: link")
		}, want: "FAIL symlink", lines: func(*lwFixture) []string { return []string{"symlink notes/link"} }},
		{
			name: "a submodule", typ: "W", branch: lwAddSubmodule,
			want: "FAIL submodule", lines: func(*lwFixture) []string { return []string{"submodule notes/sub"} },
		},
		{name: "a mode change", typ: "W", branch: func(f *lwFixture) {
			f.git("update-index", "--chmod=+x", "notes/long.txt")
			if err := os.Chmod(filepath.Join(f.repo, "notes", "long.txt"), 0o700); err != nil { //nolint:gosec // the fixture's mode change
				f.t.Fatalf("chmod: %v", err)
			}
			f.gitStdin(f.repo, "docs: mode", "commit", "-q", "-F", "-")
		}, want: "FAIL mode", lines: func(*lwFixture) []string { return []string{"mode notes/long.txt 100644 100755"} }},

		// Tool commits, with the lwtool row as the only tool row.
		{name: "M tool lines over the limit, hand lines at it", typ: "M", table: tools, branch: func(f *lwFixture) {
			f.toolCommit(lwTool + " -w " + genFiles(f.limitFiles+1))
			f.write("gen/f01.txt", f.read("gen/f01.txt")+lwLines("hand", f.limit("M added")))
			f.commit("refactor: hand")
		}, want: "PASS", lines: func(f *lwFixture) []string {
			return []string{f.counts("M", f.limit("M added"), 0, 1)}
		}, after: func(f *lwFixture, _ lwBash, _ string, first lwRun) {
			if got, want := first.field("tool"), f.rev("HEAD~1"); got != want {
				f.t.Errorf("tool=%s, want the tool commit %s", got, want)
			}
		}},
		{name: "the record amended into the last tool commit", typ: "M", table: tools, branch: func(f *lwFixture) {
			f.toolCommit(lwTool + " -w gen/f01.txt")
		}, want: "PASS", after: func(f *lwFixture, bash lwBash, spec string, first lwRun) {
			record := lwToolMessage(lwTool+" -w gen/f01.txt") + "\nLightweight process: #1, type M\n" +
				lwCheckPrefix + strings.Join(first.lines, "\n"+lwCheckPrefix) + "\n"
			f.gitStdin(f.repo, record, "commit", "-q", "--amend", "-F", "-")
			again := f.check(bash, spec, lwRunOptions{})
			if again.verdict() != first.verdict() || again.code != first.code {
				f.t.Errorf("after the amend the verdict is %q (exit %d), want %q (exit %d)",
					again.verdict(), again.code, first.verdict(), first.code)
			}
		}},
		{name: "a tool commit touches an excluded path", typ: "M", table: tools, branch: func(f *lwFixture) {
			f.toolCommit(lwTool + " -w " + f.excludedFile)
		}, want: "FAIL excluded", lines: func(f *lwFixture) []string {
			return []string{"excluded " + f.excludedFile}
		}},
		{name: "two tool commits in a row", typ: "M", table: tools, branch: func(f *lwFixture) {
			f.toolCommit(lwTool + " -w gen/f01.txt")
			f.toolCommit(lwTool + " -w gen/f01.txt gen/f02.txt")
			f.addLines("notes/new.txt", 1)
			f.commit("refactor: hand")
		}, want: "PASS", lines: func(f *lwFixture) []string {
			return []string{f.counts("M", 1, 0, 1)}
		}, after: func(f *lwFixture, _ lwBash, _ string, first lwRun) {
			if got, want := first.field("tool"), f.rev("HEAD~1"); got != want {
				f.t.Errorf("tool=%s, want the second tool commit %s", got, want)
			}
		}},
		{name: "a tool commit does not reproduce", typ: "M", table: tools, branch: func(f *lwFixture) {
			f.runTool(lwTool + " -w gen/f01.txt")
			f.write("gen/f02.txt", "hand\n")
			f.commit(lwToolMessage(lwTool + " -w gen/f01.txt"))
		}, want: "STOP tool-mismatch"},
		{name: "a tool command fails", typ: "M", table: tools, branch: func(f *lwFixture) {
			f.write("gen/f01.txt", "hand\n")
			f.commit(lwToolMessage(lwTool + " -w gen/missing.txt"))
		}, want: "STOP tool-failed"},
		{name: "a hand-written commit before a tool commit", typ: "M", table: tools, branch: func(f *lwFixture) {
			f.addLines("notes/new.txt", 1)
			f.commit("refactor: hand")
			f.toolCommit(lwTool + " -w gen/f01.txt")
		}, want: "STOP tool-order"},
		{name: "a commit with two trailer lines", typ: "M", table: tools, branch: func(f *lwFixture) {
			f.runTool(lwTool + " -w gen/f01.txt")
			f.commit(lwToolMessage(lwTool+" -w gen/f01.txt") + "Lightweight-Generated-By: " + lwTool + " -w gen/f02.txt\n")
		}, want: "STOP tool-order"},
		{name: "a type W branch with a tool commit", typ: "W", table: tools, branch: func(f *lwFixture) {
			f.toolCommit(lwTool + " -w gen/f01.txt")
		}, want: "STOP tool-order"},
		{name: "a tool not on the allowlist", typ: "M", table: tools, branch: func(f *lwFixture) {
			f.toolCommit(lwOtherTool + " -w gen/f01.txt")
		}, want: "STOP tool-allowlist"},
		{name: "a word the allowlist's word is only a prefix of", typ: "M", table: tools, branch: func(f *lwFixture) {
			f.runTool(lwTool + " -w gen/f01.txt")
			f.commit(lwToolMessage(lwTool + " -wx gen/f01.txt"))
		}, want: "STOP tool-allowlist"},
		lwBadPathCase("an absolute path", "/etc/hosts"),
		lwBadPathCase("a parent element", "gen/../gen/f01.txt"),
		lwBadPathCase("a shell symbol", "gen/f01.txt;true"),

		// Inputs.
		{name: "a limit row is missing", typ: "W", table: func(t *testing.T) string {
			return lwEditTable(t, "limit W added "+readLightweightConditions(t, doctestutil.ReadDoc(t, requirementsProcessPath)).limits["W added"], "")
		}, branch: lwSmallChange, want: "STOP table"},
		{name: "a row is repeated", typ: "W", table: func(t *testing.T) string {
			return lwEditTable(t, "exclude docs/tasks/", "exclude docs/tasks/\nexclude docs/tasks/\n")
		}, branch: lwSmallChange, want: "STOP table"},
		{name: "the files limit is none", typ: "W", table: func(t *testing.T) string {
			files := readLightweightConditions(t, doctestutil.ReadDoc(t, requirementsProcessPath)).limits["files"]
			return lwEditTable(t, "limit files "+files, "limit files none\n")
		}, branch: lwSmallChange, want: "STOP table"},
		{name: "a row ends in CR", typ: "W", table: func(t *testing.T) string {
			return lwEditTable(t, "exclude docs/tasks/", "exclude docs/tasks/\r\n")
		}, branch: lwSmallChange, want: "STOP table"},
		{name: "no commit after the merge base", typ: "W", branch: func(*lwFixture) {}, want: "STOP empty"},
		{name: "a local attributes file", typ: "W", branch: func(f *lwFixture) {
			lwSmallChange(f)
			if err := os.WriteFile(filepath.Join(f.repo, ".git", "info", "attributes"), []byte("* -diff\n"), 0o600); err != nil {
				f.t.Fatalf("write the local attributes: %v", err)
			}
		}, want: "STOP local-attributes"},
		{name: "an untracked file", typ: "W", branch: func(f *lwFixture) {
			lwSmallChange(f)
			f.write("notes/untracked/file.txt", "untracked\n")
		}, want: "STOP dirty"},
		{name: "a modified file", typ: "W", branch: func(f *lwFixture) {
			lwSmallChange(f)
			f.write("notes/long.txt", "modified\n")
		}, want: "STOP dirty"},
		{name: "a spec with CRLF line endings", branch: lwSmallChange, want: "PASS", spec: func(t *testing.T) string {
			return strings.ReplaceAll(lightweightSpec(t, "W"), "\n", "\r\n")
		}},
		lwSpecCase("two lines under the type heading", func(spec string) string {
			return strings.Replace(spec, "\nW\n", "\nW\nD\n", 1)
		}),
		lwSpecCase("a lowercase type", func(spec string) string {
			return strings.Replace(spec, "\nW\n", "\nw\n", 1)
		}),
		lwSpecCase("two type headings", func(spec string) string {
			return spec + "\n" + lwTypeHeading + "\n\nW\n"
		}),
		lwSpecCase("the type heading inside a code block", func(spec string) string {
			return strings.Replace(spec, lwTypeHeading+"\n", "```\n"+lwTypeHeading+"\n```\n", 1)
		}),
	}
}

// lwMDeleted is how many lines the M case deletes: many when the M deleted
// limit is none, the limit otherwise.
func lwMDeleted(f *lwFixture) int {
	if f.conditions.limits["M deleted"] == "none" {
		return 250
	}
	return f.limit("M deleted")
}

// lwSmallChange commits a one-line change any type allows.
func lwSmallChange(f *lwFixture) {
	f.addLines("notes/new.txt", 1)
	f.commit("docs: small")
}

// lwAddSubmodule commits a gitlink at notes/sub, with the empty directory an
// uninitialized submodule has.
func lwAddSubmodule(f *lwFixture) {
	f.git("update-index", "--add", "--cacheinfo", "160000,"+f.base+",notes/sub")
	if err := os.MkdirAll(filepath.Join(f.repo, "notes", "sub"), 0o750); err != nil {
		f.t.Fatalf("create the submodule directory: %v", err)
	}
	f.gitStdin(f.repo, "build: submodule", "commit", "-q", "-F", "-")
}

// lwBadPathCase is a tool commit whose command has word after the allowed
// words, where word is not a plain path inside the repository.
func lwBadPathCase(name, word string) lwCase {
	return lwCase{name: "a tool command with " + name, typ: "M", table: lwToolTable, branch: func(f *lwFixture) {
		f.runTool(lwTool + " -w gen/f01.txt")
		f.commit(lwToolMessage(lwTool + " -w gen/f01.txt " + word))
	}, want: "STOP tool-allowlist"}
}

// lwSpecCase is a spec the type cannot be read from: the W spec edited by
// edit.
func lwSpecCase(name string, edit func(string) string) lwCase {
	return lwCase{name: name, branch: lwSmallChange, want: "STOP type", spec: func(t *testing.T) string {
		spec := lightweightSpec(t, "W")
		edited := edit(spec)
		if edited == spec {
			t.Fatalf("the edit of %q left the spec unchanged", name)
		}
		return edited
	}}
}

// TestLightweightCheck runs the recheck script, as /runissue runs it, on
// fixture repositories: each case's last line and exit status, the counts and
// the values behind a FAIL, and that every run leaves no temporary file or
// worktree, and that the user's git configuration changes nothing. Every case
// runs with each bash lightweightBashes returns.
func TestLightweightCheck(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Fatalf("git is not on PATH: %v", err)
	}
	for _, bash := range lightweightBashes(t) {
		t.Run(bash.name, func(t *testing.T) {
			t.Parallel()
			for _, c := range lightweightCheckCases() {
				t.Run(c.name, func(t *testing.T) {
					t.Parallel()
					doc := ""
					if c.table != nil {
						doc = c.table(t)
					}
					f := newLWFixture(t, doc)
					c.branch(f)
					spec := ""
					if c.spec != nil {
						spec = c.spec(t)
					} else {
						spec = lightweightSpec(t, c.typ)
					}
					run := f.check(bash, spec, lwRunOptions{})
					if run.verdict() != c.want {
						t.Fatalf("verdict %q, want %q: %v", run.verdict(), c.want, run)
					}
					if !strings.HasPrefix(c.want, "STOP") {
						typ := c.typ
						if typ == "" {
							typ = "W"
						}
						f.checkInputs(run, typ, spec)
					}
					if c.lines != nil {
						for _, line := range c.lines(f) {
							if !slices.Contains(run.lines, line) {
								t.Errorf("the output has no line %q: %v", line, run)
							}
						}
					}
					if c.after != nil {
						c.after(f, bash, spec, run)
					}
				})
			}
			t.Run("no sha256sum on PATH", func(t *testing.T) {
				t.Parallel()
				f := newLWFixture(t, "")
				lwSmallChange(f)
				path := lwPathWithout(t, "sha256sum", "git", "mktemp", "rm", "tr", "shasum")
				spec := lightweightSpec(t, "W")
				run := f.check(bash, spec, lwRunOptions{path: path})
				if run.verdict() != "PASS" {
					t.Fatalf("verdict %q, want PASS: %v", run.verdict(), run)
				}
				f.checkInputs(run, "W", spec)
			})
			checkIgnoresGitConfig(t, bash)
		})
	}
}

// lwPathWithout returns a PATH of one directory holding links to the named
// commands only, and checks that absent is not among them.
func lwPathWithout(t *testing.T, absent string, commands ...string) string {
	t.Helper()
	dir := t.TempDir()
	for _, name := range commands {
		target, err := exec.LookPath(name)
		if err != nil {
			t.Fatalf("%s is not on PATH: %v", name, err)
		}
		if err := os.Symlink(target, filepath.Join(dir, name)); err != nil {
			t.Fatalf("link %s: %v", name, err)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, absent)); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("%s is in the restricted PATH", absent)
	}
	return dir
}

// lwConfigScenario is a branch, a git configuration a user could have, and
// the plain git output that configuration changes. textconv and diff.external
// are not among them: they change a patch, but neither the --numstat nor the
// --raw output the script reads.
type lwConfigScenario struct {
	name   string
	typ    string
	table  func(t *testing.T) string
	branch func(f *lwFixture)
	dir    string // the directory the script and plain run in
	config func(f *lwFixture) [][2]string
	plain  func(f *lwFixture) string
	after  func(f *lwFixture)
}

// lwConfigScenarios are the settings the script must override, each with a
// branch whose verdict plain git would get wrong under it.
func lwConfigScenarios() []lwConfigScenario {
	plainNumstat := func(dir string) func(f *lwFixture) string {
		return func(f *lwFixture) string {
			return f.gitIn(filepath.Join(f.repo, dir), "diff", "--numstat", f.base, "HEAD")
		}
	}
	return []lwConfigScenario{
		{
			// Under histogram the algo.txt change is 6 lines each way, not 3,
			// which would put the added lines over the limit.
			name: "diff.algorithm", typ: "W",
			branch: func(f *lwFixture) {
				f.write("notes/algo.txt", "a\nb\nb\nb\nc\nb\nc\nc\n")
				f.addLines("notes/new.txt", f.limit("W added")-3)
				f.commit("docs: algo")
			},
			config: func(*lwFixture) [][2]string { return [][2]string{{"diff.algorithm", "histogram"}} },
			plain:  plainNumstat("."),
		},
		{
			name: "diff.ignoreSubmodules", typ: "W", branch: lwAddSubmodule,
			config: func(*lwFixture) [][2]string { return [][2]string{{"diff.ignoreSubmodules", "all"}} },
			plain: func(f *lwFixture) string {
				return f.git("diff", "--raw", f.base, "HEAD")
			},
		},
		{
			name: "diff.relative", typ: "W", dir: "notes",
			branch: func(f *lwFixture) {
				f.write(f.excludedFile, "changed\n")
				f.addLines("notes/new.txt", 1)
				f.commit("docs: relative")
			},
			config: func(*lwFixture) [][2]string { return [][2]string{{"diff.relative", "true"}} },
			plain:  plainNumstat("notes"),
		},
		{
			// A fake gpg adds a trailer line to what git log shows of the
			// signed hand-written commit, which would make it a tool commit
			// after a hand-written one.
			name: "log.showSignature", typ: "M", table: lwToolTable,
			branch: func(f *lwFixture) {
				f.toolCommit(lwTool + " -w gen/f01.txt")
				f.addLines("notes/new.txt", 1)
				f.commit("refactor: hand")
				f.signHead()
			},
			config: func(f *lwFixture) [][2]string {
				gpg := filepath.Join(f.bin, "fake-gpg")
				script := "#!/bin/sh\necho 'Lightweight-Generated-By: " + lwTool + " -w gen/f02.txt' >&2\necho '[GNUPG:] GOODSIG 0 x'\n"
				if err := os.WriteFile(gpg, []byte(script), 0o700); err != nil { //nolint:gosec // an executable test tool
					f.t.Fatalf("write the fake gpg: %v", err)
				}
				return [][2]string{{"log.showSignature", "true"}, {"gpg.program", gpg}}
			},
			plain: func(f *lwFixture) string { return f.git("log", "-1", "--format=%B", "HEAD") },
		},
		{
			// A threshold of one byte makes git treat every file as binary,
			// which would turn a text change into a FAIL.
			name: "core.bigFileThreshold", typ: "W", branch: lwSmallChange,
			config: func(*lwFixture) [][2]string { return [][2]string{{"core.bigFileThreshold", "1"}} },
			plain:  plainNumstat("."),
		},
		{
			name: "status.showUntrackedFiles", typ: "W",
			branch: func(f *lwFixture) {
				lwSmallChange(f)
				f.write("notes/untracked.txt", "untracked\n")
			},
			config: func(*lwFixture) [][2]string { return [][2]string{{"status.showUntrackedFiles", "no"}} },
			plain:  func(f *lwFixture) string { return f.git("status", "--porcelain") },
		},
		{
			// The hook would also change the reproduction's worktree, so
			// running it would turn the PASS into a STOP.
			name: "core.hooksPath", typ: "M", table: lwToolTable,
			branch: func(f *lwFixture) { f.toolCommit(lwTool + " -w gen/f01.txt") },
			config: func(f *lwFixture) [][2]string {
				hooks := filepath.Join(f.outside, "hooks")
				if err := os.MkdirAll(hooks, 0o750); err != nil {
					f.t.Fatalf("create the hooks directory: %v", err)
				}
				hook := "#!/bin/sh\necho ran >>'" + f.hookMarker() + "'\necho hook >>gen/f01.txt\n"
				if err := os.WriteFile(filepath.Join(hooks, "post-checkout"), []byte(hook), 0o700); err != nil { //nolint:gosec // an executable test hook
					f.t.Fatalf("write the hook: %v", err)
				}
				return [][2]string{{"core.hooksPath", hooks}}
			},
			plain: func(f *lwFixture) string {
				wt := filepath.Join(f.gitTmp, "plain-worktree")
				f.git("worktree", "add", "-q", "--detach", wt, "HEAD")
				f.git("worktree", "remove", "--force", wt)
				_, err := os.Stat(f.hookMarker())
				_ = os.Remove(f.hookMarker())
				return fmt.Sprintf("hook ran: %t", err == nil)
			},
			after: func(f *lwFixture) {
				if _, err := os.Stat(f.hookMarker()); !errors.Is(err, fs.ErrNotExist) {
					f.t.Errorf("the script ran the repository's hook (marker: %v)", err)
				}
			},
		},
	}
}

// hookMarker is the file the core.hooksPath scenario's hook leaves when it
// runs, outside the repository and the script's TMPDIR.
func (f *lwFixture) hookMarker() string {
	return filepath.Join(f.outside, "hook-ran")
}

// signHead replaces the branch's head with the same commit carrying a
// gpgsig header, so git log verifies it with gpg.program.
func (f *lwFixture) signHead() {
	f.t.Helper()
	header, message, ok := strings.Cut(f.git("cat-file", "commit", "HEAD"), "\n\n")
	if !ok {
		f.t.Fatal("the head commit has no message")
	}
	signed := header + "\ngpgsig -----BEGIN PGP SIGNATURE-----\n \n x\n -----END PGP SIGNATURE-----\n\n" + message
	commit := strings.TrimSpace(f.gitStdin(f.repo, signed, "hash-object", "-t", "commit", "-w", "--stdin"))
	f.git("update-ref", "refs/heads/"+lwBranch, commit)
}

// checkIgnoresGitConfig sets, in the fixture repository's configuration and
// then in the global one, each setting a user could have that changes what
// plain git prints for the scenario's branch, after checking that it does
// change that output. The script's output must be the same as without the
// setting.
func checkIgnoresGitConfig(t *testing.T, bash lwBash) {
	for _, scope := range []string{"repository", "global"} {
		for _, s := range lwConfigScenarios() {
			t.Run("git config/"+scope+"/"+s.name, func(t *testing.T) {
				t.Parallel()
				doc := ""
				if s.table != nil {
					doc = s.table(t)
				}
				f := newLWFixture(t, doc)
				s.branch(f)
				spec := lightweightSpec(t, s.typ)
				before := s.plain(f)
				want := f.check(bash, spec, lwRunOptions{dir: s.dir})
				file := filepath.Join(f.repo, ".git", "config")
				if scope == "global" {
					file = filepath.Join(f.home, ".gitconfig")
				}
				for _, kv := range s.config(f) {
					f.git("config", "--file", file, kv[0], kv[1])
				}
				if after := s.plain(f); after == before {
					t.Fatalf("%s does not change plain git's output %q, so the scenario checks nothing", s.name, before)
				}
				got := f.check(bash, spec, lwRunOptions{dir: s.dir})
				if !slices.Equal(got.lines, want.lines) || got.code != want.code {
					t.Errorf("with %s set the script printed %v, without it %v", s.name, got, want)
				}
				if s.after != nil {
					s.after(f)
				}
			})
		}
	}
}
