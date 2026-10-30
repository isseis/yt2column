//go:build test

// Command readme verifies the change that made the README bilingual. It takes
// two commits, the merge-base and the head of the PR, and reads the files at
// each:
//
//	go run -tags test ./docs/tasks/0048_doc_language_policy/_checks/readme/ <merge-base> <head>
//
// It checks three things:
//
//  1. extension/src/core/i18n/ja.ts is the same at both commits: the Japanese
//     message keeps pointing at the README, which is now Japanese.
//  2. The translation of README.md kept its structure: the sequence of heading
//     levels, the content of every fenced code block, and the number of rows
//     of every table are the same at both commits. The allowed exceptions are
//     prose (the quoted popup text, the rewritten sentences whose subject was
//     the CLI) or links, which none of these compare; the program lists the
//     text before the first heading at the head and every link whose target
//     changed, for the PR review to read.
//  3. Each heading of README.md at the head is a Japanese entry of the
//     translation glossary, and the heading at the same position in
//     README.en.md equals that entry's English, ignoring case and allowing the
//     plural of the entry ("Reject reasons" for "reject reason"). A heading
//     that is the same ASCII text in both versions, the product name, is a
//     proper noun and needs no entry; it is listed.
//
// Run it from the repository root, once on the PR branch. make test and CI do
// not run it, and it is not maintained after the merge. It reads no
// environment variable.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"slices"
	"strings"

	"github.com/isseis/yt2column/internal/doctestutil"
)

const (
	readmePath   = "README.md"
	readmeEnPath = "README.en.md"
	jaCatalog    = "extension/src/core/i18n/ja.ts"
	glossaryPath = "docs/translation_glossary.md"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: readme <merge-base> <head>")
		os.Exit(2)
	}
	base, head := resolve(os.Args[1]), resolve(os.Args[2])
	fmt.Println("merge-base", base)
	fmt.Println("head      ", head)

	failed := false
	report := func(name string, problems []string) {
		if len(problems) == 0 {
			fmt.Println("OK  ", name)
			return
		}
		failed = true
		fmt.Println("FAIL", name)
		for _, p := range problems {
			fmt.Println("     -", p)
		}
	}

	report("ja.ts unchanged", checkCatalog(base, head))
	before, after := show(base, readmePath), show(head, readmePath)
	report("README.md keeps its structure", checkStructure(before, after))
	en := show(head, readmeEnPath)
	glossary := show(head, glossaryPath)
	problems, exempt := checkHeadings(after, en, glossary)
	report("README headings are in the glossary", problems)

	fmt.Println("---- exceptions to read in the review")
	fmt.Println("text before the first heading of README.md:")
	for line := range strings.Lines(beforeFirstHeading(after)) {
		if strings.TrimSpace(line) != "" {
			fmt.Println("     |", strings.TrimRight(line, "\n"))
		}
	}
	fmt.Println("link targets that changed after the first heading (old -> new, in order):")
	old, cur := linkTargets(fromFirstHeading(before)), linkTargets(fromFirstHeading(after))
	for i := range max(len(old), len(cur)) {
		var o, c string
		if i < len(old) {
			o = old[i]
		}
		if i < len(cur) {
			c = cur[i]
		}
		if o != c {
			fmt.Printf("     %d: %s -> %s\n", i+1, o, c)
		}
	}
	fmt.Println("headings exempt as proper nouns:")
	for _, h := range exempt {
		fmt.Println("     |", h)
	}
	if failed {
		os.Exit(1)
	}
}

// checkCatalog reports a difference in the Japanese catalog between base and
// head.
func checkCatalog(base, head string) []string {
	if show(base, jaCatalog) != show(head, jaCatalog) {
		return []string{jaCatalog + " differs"}
	}
	return nil
}

// checkStructure compares the heading levels, the code blocks, and the table
// row counts of two versions of a document.
func checkStructure(before, after string) []string {
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

// checkHeadings checks each heading of ja against the glossary and the
// heading at the same position of en against the entry's English. It returns
// the problems and the headings exempt as proper nouns.
func checkHeadings(ja, en, glossary string) (problems, exempt []string) {
	table := doctestutil.FindTable(glossary, "日本語")
	if table == nil {
		return []string{"the glossary has no term table"}, nil
	}
	english := map[string]string{}
	for _, row := range table[1:] {
		if len(row) >= 2 {
			english[row[0]] = row[1]
		}
	}
	jaHeadings, enHeadings := doctestutil.HeadingsOutsideBlocks(ja), doctestutil.HeadingsOutsideBlocks(en)
	if len(jaHeadings) != len(enHeadings) {
		problems = append(problems, fmt.Sprintf("README.md has %d headings, README.en.md %d", len(jaHeadings), len(enHeadings)))
	}
	for i, heading := range jaHeadings {
		text := headingText(heading)
		var enText string
		if i < len(enHeadings) {
			enText = headingText(enHeadings[i])
		}
		if text == enText && isASCII(text) {
			exempt = append(exempt, heading)
			continue
		}
		want, ok := english[text]
		if !ok {
			problems = append(problems, fmt.Sprintf("heading %d %q is not in the glossary", i+1, text))
			continue
		}
		if !strings.EqualFold(enText, want) && !strings.EqualFold(enText, want+"s") {
			problems = append(problems, fmt.Sprintf("heading %d: README.en.md has %q, the glossary %q", i+1, enText, want))
		}
	}
	return problems, exempt
}

// headingText returns a heading line without its '#' marker.
func headingText(heading string) string {
	return strings.TrimSpace(strings.TrimLeft(heading, "#"))
}

func isASCII(s string) bool {
	for _, r := range s {
		if r > 0x7f {
			return false
		}
	}
	return true
}

// beforeFirstHeading returns the lines of doc before its first heading.
func beforeFirstHeading(doc string) string {
	head, _ := splitAtFirstHeading(doc)
	return head
}

// fromFirstHeading returns doc from its first heading on.
func fromFirstHeading(doc string) string {
	_, rest := splitAtFirstHeading(doc)
	return rest
}

// splitAtFirstHeading splits doc before its first heading line.
func splitAtFirstHeading(doc string) (head, rest string) {
	offset := 0
	for line := range strings.Lines(doc) {
		if doctestutil.HeadingLevel(strings.TrimRight(line, "\n")) > 0 {
			return doc[:offset], doc[offset:]
		}
		offset += len(line)
	}
	return doc, ""
}

// linkRE matches an inline Markdown link and captures its target.
var linkRE = regexp.MustCompile(`\[[^\]]*\]\(([^)\s]+)\)`)

// linkTargets returns the target of every link of doc outside code blocks.
func linkTargets(doc string) []string {
	var targets []string
	for _, line := range doctestutil.ScanDocLines(slices.Collect(strings.Lines(doc))) {
		if line.Block {
			continue
		}
		for _, m := range linkRE.FindAllStringSubmatch(line.Text, -1) {
			targets = append(targets, m[1])
		}
	}
	return targets
}

// headingLevels returns the level of each heading of doc outside code blocks.
func headingLevels(doc string) []int {
	var levels []int
	for _, heading := range doctestutil.HeadingsOutsideBlocks(doc) {
		levels = append(levels, doctestutil.HeadingLevel(heading))
	}
	return levels
}

// tableSizes returns the number of rows, the header included, of each table.
func tableSizes(doc string) []int {
	var sizes []int
	for _, table := range doctestutil.MarkdownTables(doc) {
		sizes = append(sizes, len(table))
	}
	return sizes
}

// show returns the file at path in rev, or stops the run.
func show(rev, path string) string {
	out, err := exec.Command("git", "show", rev+":"+path).Output()
	if err != nil {
		fmt.Fprintf(os.Stderr, "readme: git show %s:%s: %v\n", rev, path, err)
		os.Exit(1)
	}
	return string(out)
}

// resolve returns the full commit ID of rev, or stops the run.
func resolve(rev string) string {
	out, err := exec.Command("git", "rev-parse", "--verify", rev+"^{commit}").Output()
	if err != nil {
		fmt.Fprintf(os.Stderr, "readme: git rev-parse %s: %v\n", rev, err)
		os.Exit(1)
	}
	return strings.TrimSpace(string(out))
}
