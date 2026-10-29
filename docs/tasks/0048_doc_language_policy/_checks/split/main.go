//go:build test

// Command split verifies the split of README.md into README.md,
// docs/dev/guides/cli.md, and docs/dev/guides/development.md. It takes two
// commits, the merge-base and the commit that moved the text in English, and
// reads the documents at each:
//
//	go run -tags test ./docs/tasks/0048_doc_language_policy/_checks/split/ <merge-base> <english-move-commit> [<translated>]
//
// Every sentence, table row, and code block line of the README at the
// merge-base must be in at least one of the three documents at the move
// commit. A sentence is compared after its paragraph's line breaks are joined
// and the target of every link is removed, so a rewrapped line or a link that
// now points at another document is not a difference. Text the move added (a
// pointer to where a part went, the README's two-column reason table) is not
// a reason to fail.
//
// A sentence whose phrases went to different documents is listed in
// splits.json beside this file: its original text, and its parts in order,
// each with the document it went to and, when the destination rewords it (a
// capital letter, a subject), the text as it is there. The program checks that
// the parts joined are the original sentence and that each part is in its
// document. The list is read in the PR review against the architecture
// document's table 3-2.
//
// The output is the destination of each unit (R for README.md, C for cli.md, D
// for development.md, or the split list), the units found in two or more
// documents (read against AC-22's allowed duplicates), and the units found
// nowhere. Only a unit found nowhere, a split entry that does not hold, or a
// second version of cli.md or development.md under docs/dev/guides (AC-24)
// fails the run.
//
// With a third commit, the one where cli.md and development.md are in
// Japanese, it also checks that the translation kept each guide's structure:
// the sequence of heading levels, the content of every code block, and the
// number of rows of every table are the same as at the move commit.
//
// Run it from the repository root, once on the PR branch. make test and CI do
// not run it, and it is not maintained after the merge. It reads no
// environment variable: the directory starts with an underscore, which keeps
// it out of go list but not out of the repository-wide environment access
// check.
package main

import (
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"os/exec"
	"regexp"
	"slices"
	"strings"

	"github.com/isseis/yt2column/internal/doctestutil"
)

const (
	readmePath      = "README.md"
	cliPath         = "docs/dev/guides/cli.md"
	developmentPath = "docs/dev/guides/development.md"
	guidesDir       = "docs/dev/guides/"
	splitsPath      = "docs/tasks/0048_doc_language_policy/_checks/split/splits.json"
)

// destinations are the documents a unit can go to, in the order the output
// names them.
var destinations = []struct{ code, path string }{
	{"R", readmePath},
	{"C", cliPath},
	{"D", developmentPath},
}

// unitKind is what a unit of the old README is.
type unitKind int

const (
	kindSentence unitKind = iota + 1
	kindRow
	kindCode
)

func (k unitKind) String() string {
	switch k {
	case kindSentence:
		return "sentence"
	case kindRow:
		return "row"
	case kindCode:
		return "code"
	}
	return "invalid"
}

// unit is one sentence, table row, or code block line of the old README, with
// the line it starts on.
type unit struct {
	kind unitKind
	line int
	text string
}

// doc is one document at the move commit, in the forms the units are looked
// up in.
type doc struct {
	prose     string              // normalized whole text
	rows      map[string]struct{} // normalized table rows
	codeLines map[string]struct{} // code block lines
}

// split is one entry of splits.json.
type split struct {
	Original string `json:"original"`
	Parts    []struct {
		Doc  string `json:"doc"`
		Text string `json:"text"`
		As   string `json:"as,omitempty"`
	} `json:"parts"`
}

func main() {
	if len(os.Args) != 3 && len(os.Args) != 4 {
		fmt.Fprintln(os.Stderr, "usage: split <merge-base> <english-move-commit> [<translated>]")
		os.Exit(2)
	}
	base, move := os.Args[1], os.Args[2]
	baseID, err := gitOutput("rev-parse", base)
	if err != nil {
		fail("rev-parse", err)
	}
	moveID, err := gitOutput("rev-parse", move)
	if err != nil {
		fail("rev-parse", err)
	}
	fmt.Printf("merge-base %s\n", baseID)
	fmt.Printf("move       %s\n", moveID)

	old, err := gitOutput("show", baseID+":"+readmePath)
	if err != nil {
		fail("read the old README", err)
	}
	docs := map[string]doc{}
	for _, d := range destinations {
		text, err := gitOutput("show", moveID+":"+d.path)
		if err != nil {
			fail("read "+d.path, err)
		}
		docs[d.code] = parseDoc(text)
	}
	data, err := os.ReadFile(splitsPath)
	if err != nil {
		fail("read the split list", err)
	}
	var splits []split
	if err := json.Unmarshal(data, &splits); err != nil {
		fail("parse the split list", err)
	}

	failed := false
	if problems := checkSplits(normalize(old), splits, docs); len(problems) > 0 {
		failed = true
		fmt.Printf("NG   split list: %d problem(s)\n", len(problems))
		for _, p := range problems {
			fmt.Printf("     %s\n", p)
		}
	} else {
		fmt.Printf("OK   split list: %d sentences\n", len(splits))
	}

	var missing, multiple []string
	counts := map[string]int{}
	fmt.Println("---- destinations")
	for _, u := range units(old) {
		dest := destinationsOf(u, docs, splits)
		label := strings.Join(dest, " ")
		if len(dest) == 0 {
			label = "MISSING"
		}
		counts[label]++
		entry := fmt.Sprintf("L%-5d %-8s %-8s %s", u.line, label, u.kind, u.text)
		fmt.Println(entry)
		switch {
		case len(dest) == 0:
			missing = append(missing, entry)
		case len(dest) > 1:
			multiple = append(multiple, entry)
		}
	}
	fmt.Println("---- counts")
	for _, label := range slices.Sorted(maps.Keys(counts)) {
		fmt.Printf("%-8s %d\n", label, counts[label])
	}
	fmt.Printf("---- in two or more documents: %d\n", len(multiple))
	for _, entry := range multiple {
		fmt.Println(entry)
	}
	if len(missing) > 0 {
		failed = true
		fmt.Printf("NG   found nowhere: %d unit(s)\n", len(missing))
		for _, entry := range missing {
			fmt.Printf("     %s\n", entry)
		}
	} else {
		fmt.Println("OK   every unit of the old README is in a document")
	}

	if problems := checkOtherVersions(moveID); len(problems) > 0 {
		failed = true
		fmt.Printf("NG   other versions of the guides: %s\n", strings.Join(problems, ", "))
	} else {
		fmt.Println("OK   no other version of cli.md or development.md")
	}
	if len(os.Args) == 4 {
		translatedID, err := gitOutput("rev-parse", os.Args[3])
		if err != nil {
			fail("rev-parse", err)
		}
		fmt.Printf("translated %s\n", translatedID)
		for _, path := range []string{cliPath, developmentPath} {
			if problems := checkTranslation(moveID, translatedID, path); len(problems) > 0 {
				failed = true
				fmt.Printf("NG   %s keeps its structure: %d problem(s)\n", path, len(problems))
				for _, p := range problems {
					fmt.Printf("     %s\n", p)
				}
			} else {
				fmt.Printf("OK   %s keeps its structure\n", path)
			}
		}
	}
	if failed {
		os.Exit(1)
	}
}

// fail prints an error that stops every check and exits non-zero.
func fail(what string, err error) {
	fmt.Fprintf(os.Stderr, "split: %s: %v\n", what, err)
	os.Exit(1)
}

// gitOutput runs git with args and returns its standard output without the
// trailing newline.
func gitOutput(args ...string) (string, error) {
	out, err := exec.Command("git", args...).Output()
	if err != nil {
		return "", fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return strings.TrimSuffix(string(out), "\n"), nil
}

// linkRE matches an inline Markdown link and captures its text.
var linkRE = regexp.MustCompile(`\[([^\]]*)\]\([^)\s]+\)`)

// spaceRE matches a run of white space, line breaks included.
var spaceRE = regexp.MustCompile(`\s+`)

// normalize removes the target of every link and folds each run of white
// space to one space.
func normalize(text string) string {
	text = linkRE.ReplaceAllString(text, "[$1]")
	return strings.TrimSpace(spaceRE.ReplaceAllString(text, " "))
}

// markerRE matches a list item or block quote marker at the start of a line.
var markerRE = regexp.MustCompile(`^\s*(?:[-*]|\d+\.|>)\s+`)

// isTableLine reports whether line is a Markdown table row.
func isTableLine(line string) bool {
	trimmed := strings.TrimSpace(line)
	return strings.HasPrefix(trimmed, "|") && strings.HasSuffix(trimmed, "|")
}

// isSeparatorRow reports whether a table row is the "---" row under a header.
func isSeparatorRow(line string) bool {
	return strings.Trim(strings.TrimSpace(line), "|-: ") == ""
}

// parseDoc returns text in the forms units are looked up in.
func parseDoc(text string) doc {
	d := doc{prose: normalize(text), rows: map[string]struct{}{}, codeLines: map[string]struct{}{}}
	for _, line := range doctestutil.ScanDocLines(slices.Collect(strings.Lines(text))) {
		switch {
		case line.Block && !line.Fence:
			d.codeLines[strings.TrimRight(line.Text, " \t")] = struct{}{}
		case !line.Block && isTableLine(line.Text):
			d.rows[normalize(line.Text)] = struct{}{}
		}
	}
	return d
}

// units returns the sentences, table rows, and code block lines of text. A
// paragraph is the lines between blank lines, headings, tables, and code
// blocks; a list item starts a new one, and its marker is dropped.
func units(text string) []unit {
	var out []unit
	var para []string
	start := 0
	flush := func() {
		if len(para) > 0 {
			for _, s := range sentences(normalize(strings.Join(para, "\n"))) {
				out = append(out, unit{kind: kindSentence, line: start, text: s})
			}
			para = nil
		}
	}
	for i, line := range doctestutil.ScanDocLines(slices.Collect(strings.Lines(text))) {
		n := i + 1
		switch {
		case line.Fence:
			flush()
		case line.Block:
			if code := strings.TrimRight(line.Text, " \t"); strings.TrimSpace(code) != "" {
				out = append(out, unit{kind: kindCode, line: n, text: code})
			}
		case strings.TrimSpace(line.Text) == "" || doctestutil.HeadingLevel(line.Text) > 0:
			flush()
		case isTableLine(line.Text):
			flush()
			if !isSeparatorRow(line.Text) {
				out = append(out, unit{kind: kindRow, line: n, text: normalize(line.Text)})
			}
		case markerRE.MatchString(line.Text):
			flush()
			para, start = []string{markerRE.ReplaceAllString(line.Text, "")}, n
		default:
			if len(para) == 0 {
				start = n
			}
			para = append(para, line.Text)
		}
	}
	flush()
	return out
}

// sentenceEndRE matches the end of a sentence: a period, a question mark, or
// an exclamation mark, optional closing characters, white space, and the
// start of the next sentence. Splitting too often only makes shorter units,
// which are still looked up as text, so the rule errs that way.
var sentenceEndRE = regexp.MustCompile("[.?!][)\"'*]* +[A-Z`*(\"0-9]")

// sentences splits a normalized paragraph into sentences.
func sentences(para string) []string {
	var out []string
	for {
		loc := sentenceEndRE.FindStringIndex(para)
		if loc == nil {
			break
		}
		end := loc[1] - 1 // keep the next sentence's first character
		out = append(out, strings.TrimSpace(para[:end]))
		para = para[end:]
	}
	if s := strings.TrimSpace(para); s != "" {
		out = append(out, s)
	}
	return out
}

// destinationsOf returns the codes of the documents that hold u, or "split"
// when u is part of a sentence in the split list.
func destinationsOf(u unit, docs map[string]doc, splits []split) []string {
	var found []string
	for _, d := range destinations {
		target := docs[d.code]
		var ok bool
		switch u.kind {
		case kindSentence:
			ok = strings.Contains(target.prose, u.text)
		case kindRow:
			_, ok = target.rows[u.text]
		case kindCode:
			_, ok = target.codeLines[u.text]
		}
		if ok {
			found = append(found, d.code)
		}
	}
	if len(found) == 0 {
		for _, s := range splits {
			if strings.Contains(normalize(s.Original), u.text) {
				return []string{"split"}
			}
		}
	}
	return found
}

// partTrim is what a part's ends may carry that its destination drops: the
// space and the punctuation at the cut.
const partTrim = " ,;:."

// checkSplits checks each entry of the split list: its original is in the old
// README, its parts joined are the original, and each part is in its
// document.
func checkSplits(old string, splits []split, docs map[string]doc) []string {
	var problems []string
	for i, s := range splits {
		original := normalize(s.Original)
		name := fmt.Sprintf("entry %d %q", i+1, truncate(original))
		if !strings.Contains(old, original) {
			problems = append(problems, name+": the original is not in the old README")
		}
		var joined strings.Builder
		for _, part := range s.Parts {
			joined.WriteString(part.Text)
			target, ok := docs[part.Doc]
			if !ok {
				problems = append(problems, fmt.Sprintf("%s: unknown document %q", name, part.Doc))
				continue
			}
			want := part.Text
			if part.As != "" {
				want = part.As
			}
			want = strings.Trim(normalize(want), partTrim)
			if want == "" || !strings.Contains(target.prose, want) {
				problems = append(problems, fmt.Sprintf("%s: %s does not hold %q", name, part.Doc, want))
			}
		}
		if normalize(joined.String()) != original {
			problems = append(problems, name+": the parts joined are not the original")
		}
	}
	return problems
}

// checkOtherVersions returns any file under docs/dev/guides at rev that is
// another version of cli.md or development.md, such as cli.en.md.
func checkOtherVersions(rev string) []string {
	out, err := gitOutput("ls-tree", "--name-only", rev, guidesDir)
	if err != nil {
		fail("ls-tree", err)
	}
	var problems []string
	for name := range strings.SplitSeq(out, "\n") {
		base := strings.TrimPrefix(name, guidesDir)
		for _, stem := range []string{"cli.", "development."} {
			if strings.HasPrefix(base, stem) && base != stem+"md" {
				problems = append(problems, name)
			}
		}
	}
	for _, want := range []string{cliPath, developmentPath} {
		if !strings.Contains("\n"+out+"\n", "\n"+want+"\n") {
			problems = append(problems, want+" is missing")
		}
	}
	return problems
}

// checkTranslation compares path at the move commit with path at the
// translated commit: the sequence of heading levels outside code blocks, the
// info string and lines of every code block, and the row count of every
// table, in order.
func checkTranslation(move, translated, path string) []string {
	before, err := gitOutput("show", move+":"+path)
	if err != nil {
		fail("read "+path, err)
	}
	after, err := gitOutput("show", translated+":"+path)
	if err != nil {
		fail("read "+path, err)
	}
	var problems []string
	if b, a := headingLevels(before), headingLevels(after); !slices.Equal(b, a) {
		problems = append(problems, fmt.Sprintf("heading levels %v, were %v", a, b))
	}
	b, a := doctestutil.FencedBlocks(before), doctestutil.FencedBlocks(after)
	if len(b) != len(a) {
		problems = append(problems, fmt.Sprintf("%d code blocks, were %d", len(a), len(b)))
	} else {
		for i := range b {
			if b[i].Info != a[i].Info || !slices.Equal(b[i].Lines, a[i].Lines) {
				problems = append(problems, fmt.Sprintf("code block %d differs", i+1))
			}
		}
	}
	if b, a := tableSizes(before), tableSizes(after); !slices.Equal(b, a) {
		problems = append(problems, fmt.Sprintf("table row counts %v, were %v", a, b))
	}
	return problems
}

// headingLevels returns the level of each heading of doc outside code blocks.
func headingLevels(doc string) []int {
	var levels []int
	for _, heading := range doctestutil.HeadingsOutsideBlocks(doc) {
		levels = append(levels, doctestutil.HeadingLevel(heading))
	}
	return levels
}

// tableSizes returns the number of rows, the header included, of each table of
// doc.
func tableSizes(doc string) []int {
	var sizes []int
	for _, table := range doctestutil.MarkdownTables(doc) {
		sizes = append(sizes, len(table))
	}
	return sizes
}

// truncate shortens text for a problem line.
func truncate(text string) string {
	if len(text) > 60 {
		return text[:60] + "…"
	}
	return text
}
