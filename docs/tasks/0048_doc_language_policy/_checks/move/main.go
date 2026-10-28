// Command move verifies the move of docs/dev/developer_guide into
// docs/dev/conventions and docs/dev/guides. It compares the merge-base of
// origin/main and HEAD with HEAD and prints one result per check, with the
// line of each difference that does not hold:
//
//   - the five convention documents are byte for byte the documents they were
//     moved from;
//   - the two Japanese guides are the documents they were renamed from, with
//     no line that differs and no .ja.md file left;
//   - docs/dev/developer_guide no longer exists;
//   - no reference to one of the moved files by its old path remains outside
//     this task's documents;
//   - a past task document changed only where a moved file's old path was
//     replaced;
//   - every relative link in the task documents, the instruction documents,
//     and docs/dev/ names a file that exists at HEAD;
//   - the two translated guides keep the heading levels, the code blocks, and
//     the inline code of the English originals.
//
// Run it once on the PR branch:
//
//	go run ./docs/tasks/0048_doc_language_policy/_checks/move/
//
// The Markdown scanning helpers are copies of the ones in
// cmd/yt2column/docs_test.go; a later phase moves both to a test-only package
// and this one-shot program is not maintained after the merge.
//
// make test and CI do not run it. It reads no environment variable and names
// no secret: the directory starts with an underscore, which keeps it out of
// go list but not out of the repository-wide environment access check.
package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
	"regexp"
	"slices"
	"sort"
	"strings"
)

const (
	oldDir        = "docs/dev/developer_guide"
	conventions   = "docs/dev/conventions"
	guides        = "docs/dev/guides"
	taskDirPrefix = "docs/tasks/0048_doc_language_policy"
)

// conventionsMoved are the files that moved without a rename.
var conventionsMoved = []string{
	"requirements_process.md",
	"test_organization.md",
	"mermaid_reference.md",
	"package_reference.md",
	"task_identification.md",
}

// guidesRenamed are the Japanese guides that became the guide of the same
// name, without the .ja suffix.
var guidesRenamed = []struct{ from, to string }{
	{"mergepr_guide.ja.md", "mergepr_guide.md"},
	{"fixprguard_guide.ja.md", "fixprguard_guide.md"},
}

// translated are the guides translated from English in this change. The second
// field is the name the file had before the move.
var translated = []struct{ from, to string }{
	{"pricing_update.md", "pricing_update.md"},
	{"claude_output_limit.md", "claude_output_limit.md"},
}

// pathRewrites maps an old path suffix to its new one, in the order they are
// applied. A change that changes anything else is not a path-only change. The
// suffix form catches both the repository paths and the relative links, which
// differ only in the directory prefix.
var pathRewrites = []struct{ old, new string }{
	{"developer_guide/requirements_process.md", "conventions/requirements_process.md"},
	{"developer_guide/test_organization.md", "conventions/test_organization.md"},
	{"developer_guide/mermaid_reference.md", "conventions/mermaid_reference.md"},
	{"developer_guide/package_reference.md", "conventions/package_reference.md"},
	{"developer_guide/task_identification.md", "conventions/task_identification.md"},
	{"developer_guide/mergepr_guide.md", "guides/mergepr_guide.md"},
	{"developer_guide/mergepr_guide.ja.md", "guides/mergepr_guide.md"},
	{"developer_guide/fixprguard_guide.md", "guides/fixprguard_guide.md"},
	{"developer_guide/fixprguard_guide.ja.md", "guides/fixprguard_guide.md"},
	{"developer_guide/pricing_update.md", "guides/pricing_update.md"},
	{"developer_guide/claude_output_limit.md", "guides/claude_output_limit.md"},
}

func main() {
	base, err := gitOutput("merge-base", "origin/main", "HEAD")
	if err != nil {
		fail("merge-base", err)
	}
	head, err := gitOutput("rev-parse", "HEAD")
	if err != nil {
		fail("rev-parse", err)
	}
	fmt.Printf("merge-base %s\n", base)
	fmt.Printf("head       %s\n", head)

	tree, err := listTree(head)
	if err != nil {
		fail("ls-tree", err)
	}

	checks := []struct {
		name string
		run  func(base string, tree []string) (notes, problems []string)
	}{
		{"conventions unchanged", checkConventions},
		{"guides renamed from the Japanese originals", checkGuides},
		{"old directory is gone", checkOldDirectory},
		{"no old file path remains", checkNoOldPaths},
		{"past task documents changed only at paths", checkPastTasks},
		{"relative links resolve", checkRelativeLinks},
		{"translations keep the structure", checkTranslations},
	}
	failed := 0
	for _, check := range checks {
		notes, problems := check.run(base, tree)
		for _, note := range notes {
			fmt.Printf("     %s\n", note)
		}
		if len(problems) == 0 {
			fmt.Printf("OK   %s\n", check.name)
			continue
		}
		failed++
		fmt.Printf("NG   %s: %d problem(s)\n", check.name, len(problems))
		for _, problem := range problems {
			fmt.Printf("     %s\n", problem)
		}
	}
	if failed > 0 {
		os.Exit(1)
	}
}

// fail prints an error that stops every check and exits non-zero.
func fail(what string, err error) {
	fmt.Fprintf(os.Stderr, "checks: %s: %v\n", what, err)
	os.Exit(1)
}

// gitOutput runs git with args and returns its standard output without the
// trailing newline.
func gitOutput(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return strings.TrimSuffix(string(out), "\n"), nil
}

// fileAt returns the content of name at rev, without the trailing newline.
func fileAt(rev, name string) (string, error) {
	return gitOutput("show", rev+":"+name)
}

// fileAtRaw returns the bytes of name at rev as git prints them, for a
// comparison that must also see a change in the trailing newline.
func fileAtRaw(rev, name string) (string, error) {
	cmd := exec.Command("git", "show", rev+":"+name)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git show %s:%s: %w", rev, name, err)
	}
	return string(out), nil
}

// listTree returns every file path of rev, sorted.
func listTree(rev string) ([]string, error) {
	out, err := gitOutput("ls-tree", "-r", "--name-only", "--full-tree", rev)
	if err != nil {
		return nil, err
	}
	if out == "" {
		return nil, nil
	}
	return strings.Split(out, "\n"), nil
}

// exists reports whether tree holds name.
func exists(tree []string, name string) bool {
	_, ok := slices.BinarySearch(tree, name)
	return ok
}

// rewritePaths applies every path rewrite to text.
func rewritePaths(text string) string {
	for _, rewrite := range pathRewrites {
		text = strings.ReplaceAll(text, rewrite.old, rewrite.new)
	}
	return text
}

// checkConventions compares each moved convention document with the document
// of the same name before the move. A byte difference is a problem: these
// documents hold no reference to the directory they lived in.
func checkConventions(base string, tree []string) (notes, problems []string) {
	for _, name := range conventionsMoved {
		oldPath := oldDir + "/" + name
		newPath := conventions + "/" + name
		if !exists(tree, newPath) {
			problems = append(problems, fmt.Sprintf("%s is missing", newPath))
			continue
		}
		before, err := fileAtRaw(base, oldPath)
		if err != nil {
			problems = append(problems, err.Error())
			continue
		}
		after, err := fileAtRaw("HEAD", newPath)
		if err != nil {
			problems = append(problems, err.Error())
			continue
		}
		if before != after {
			problems = append(problems, fmt.Sprintf("%s differs from %s", newPath, oldPath))
		}
	}
	return nil, problems
}

// checkGuides compares each renamed guide with the Japanese original and fails
// on any differing line, so a guide that is not the moved file cannot pass.
// A later change that adds the English content or rewrites a README section
// reference needs an explicit exception here.
func checkGuides(base string, tree []string) (notes, problems []string) {
	for _, name := range tree {
		if strings.HasPrefix(name, guides+"/") && strings.HasSuffix(name, ".ja.md") {
			problems = append(problems, fmt.Sprintf("%s is a leftover .ja.md file", name))
		}
	}
	for _, guide := range guidesRenamed {
		newPath := guides + "/" + guide.to
		if !exists(tree, newPath) {
			problems = append(problems, fmt.Sprintf("%s is missing", newPath))
			continue
		}
		before, err := fileAtRaw(base, oldDir+"/"+guide.from)
		if err != nil {
			problems = append(problems, err.Error())
			continue
		}
		after, err := fileAtRaw("HEAD", newPath)
		if err != nil {
			problems = append(problems, err.Error())
			continue
		}
		for _, line := range diffLines(rewritePaths(before), rewritePaths(after)) {
			problems = append(problems, fmt.Sprintf("%s: %s", newPath, line))
		}
	}
	return notes, problems
}

// checkOldDirectory checks that the old directory is gone, not turned into a
// directory of redirect stubs.
func checkOldDirectory(base string, tree []string) (notes, problems []string) {
	for _, name := range tree {
		if name == oldDir || strings.HasPrefix(name, oldDir+"/") {
			problems = append(problems, fmt.Sprintf("%s still exists", name))
		}
	}
	return nil, problems
}

// checkNoOldPaths searches every file at HEAD outside this task's documents for
// a reference to a moved file by its old path.
func checkNoOldPaths(base string, tree []string) (notes, problems []string) {
	for _, rewrite := range pathRewrites {
		args := []string{"grep", "-n", "-F", "-e", rewrite.old, "HEAD", "--", ":(exclude)" + taskDirPrefix}
		out, err := gitOutput(args...)
		if err != nil {
			var exitErr *exec.ExitError
			if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
				continue
			}
			problems = append(problems, fmt.Sprintf("git grep %s: %v", rewrite.old, err))
			continue
		}
		for _, line := range strings.Split(out, "\n") {
			problems = append(problems, "old path remains: "+line)
		}
	}
	return nil, problems
}

// checkPastTasks applies the path rewrites to each past task file that changed
// and compares the result with the file at HEAD. A difference left after the
// rewrites is a change beyond a path.
func checkPastTasks(base string, tree []string) (notes, problems []string) {
	out, err := gitOutput("diff", "--name-only", base, "HEAD", "--", "docs/tasks")
	if err != nil {
		return nil, []string{fmt.Sprintf("git diff: %v", err)}
	}
	if out == "" {
		return nil, nil
	}
	for _, name := range strings.Split(out, "\n") {
		if strings.HasPrefix(name, taskDirPrefix+"/") {
			continue
		}
		before, err := fileAtRaw(base, name)
		if err != nil {
			problems = append(problems, err.Error())
			continue
		}
		after, err := fileAtRaw("HEAD", name)
		if err != nil {
			problems = append(problems, err.Error())
			continue
		}
		for _, line := range diffLines(rewritePaths(before), after) {
			problems = append(problems, fmt.Sprintf("%s: %s", name, line))
		}
	}
	return nil, problems
}

// checkRelativeLinks checks the relative links this change rewrote — those
// that point into the two new directories — in the documents an agent reads
// and in the past task documents, against the tree at HEAD.
func checkRelativeLinks(base string, tree []string) (notes, problems []string) {
	for _, name := range tree {
		if !strings.HasSuffix(name, ".md") || !isCheckedDoc(name) {
			continue
		}
		doc, err := fileAt("HEAD", name)
		if err != nil {
			problems = append(problems, err.Error())
			continue
		}
		for _, target := range relativeLinkTargets(doc) {
			if !isMovedTarget(target) {
				continue
			}
			resolved := path.Join(path.Dir(name), target)
			if !exists(tree, resolved) {
				problems = append(problems, fmt.Sprintf("%s: link %q resolves to %q, which does not exist", name, target, resolved))
			}
		}
	}
	return nil, problems
}

// isMovedTarget reports whether target is a link into docs/dev/conventions or
// docs/dev/guides, the links this change rewrote.
func isMovedTarget(target string) bool {
	return strings.Contains(target, "conventions/") || strings.Contains(target, "guides/")
}

// isCheckedDoc reports whether name is one of the documents a relative link is
// checked in: a past task document, an instruction or convention document, or
// a guide.
func isCheckedDoc(name string) bool {
	switch {
	case name == "CLAUDE.md", name == "AGENTS.md":
		return true
	case strings.HasPrefix(name, "docs/tasks/"), strings.HasPrefix(name, "docs/dev/"), strings.HasPrefix(name, ".claude/commands/"):
		return true
	}
	return false
}

// checkTranslations compares each translated guide with its English original:
// the heading levels in order, the fenced code blocks, and the inline code
// must be the same.
func checkTranslations(base string, tree []string) (notes, problems []string) {
	for _, guide := range translated {
		newPath := guides + "/" + guide.to
		oldPath := oldDir + "/" + guide.from
		if !exists(tree, newPath) {
			problems = append(problems, fmt.Sprintf("%s is missing", newPath))
			continue
		}
		before, err := fileAt(base, oldPath)
		if err != nil {
			problems = append(problems, err.Error())
			continue
		}
		after, err := fileAt("HEAD", newPath)
		if err != nil {
			problems = append(problems, err.Error())
			continue
		}
		problems = append(problems, compareStructure(oldPath, before, newPath, after)...)
	}
	return nil, problems
}

// compareStructure reports how after differs from before in the heading
// levels, the code blocks, and the inline code.
func compareStructure(oldPath, before, newPath, after string) []string {
	var problems []string
	oldLevels, oldBlocks, oldCode := structure(before)
	newLevels, newBlocks, newCode := structure(after)
	if !slices.Equal(oldLevels, newLevels) {
		problems = append(problems, fmt.Sprintf("%s: the heading levels %v differ from %s (%v)", newPath, newLevels, oldPath, oldLevels))
	}
	if !slices.Equal(normalizeBlocks(oldBlocks), normalizeBlocks(newBlocks)) {
		problems = append(problems, fmt.Sprintf("%s: the code blocks differ from %s", newPath, oldPath))
	}
	if !slices.Equal(oldCode, newCode) {
		problems = append(problems, fmt.Sprintf("%s: the inline code spans differ from %s", newPath, oldPath))
	}
	return problems
}

// structure returns the heading levels outside code blocks, the fenced code
// blocks, and the sorted inline code spans of doc. A code span may wrap across
// lines, and a line break inside one is read as a single space.
func structure(doc string) (levels []int, blocks []string, code []string) {
	lines := scanLines(doc)
	var prose strings.Builder
	var block []string
	inBlock := false
	for _, line := range lines {
		switch {
		case line.fence && !inBlock:
			inBlock = true
			block = nil
			continue
		case line.fence && inBlock:
			inBlock = false
			blocks = append(blocks, strings.Join(block, "\n"))
			continue
		case inBlock:
			block = append(block, line.text)
			continue
		}
		if level := headingLevel(line.text); level > 0 {
			levels = append(levels, level)
		}
		prose.WriteString(line.text)
		prose.WriteString("\n")
	}
	if inBlock {
		blocks = append(blocks, strings.Join(block, "\n"))
	}
	for _, match := range codeSpanRE.FindAllStringSubmatch(prose.String(), -1) {
		code = append(code, strings.Join(strings.Fields(match[1]), " "))
	}
	sort.Strings(code)
	return levels, blocks, code
}

// normalizeBlocks keeps only the text of each link in the code blocks, so a
// link whose path changed is not a difference.
func normalizeBlocks(blocks []string) []string {
	normalized := make([]string, 0, len(blocks))
	for _, block := range blocks {
		normalized = append(normalized, markdownLinkRE.ReplaceAllString(block, "$1"))
	}
	return normalized
}

// relativeLinkTargets returns the target of each relative Markdown link
// outside a fenced code block, without its anchor. External and mailto:
// links, anchors in the same document, and <...> placeholders are not
// relative links.
func relativeLinkTargets(doc string) []string {
	var targets []string
	for _, line := range scanLines(doc) {
		if line.block {
			continue
		}
		for _, match := range markdownLinkRE.FindAllStringSubmatch(line.text, -1) {
			target := match[2]
			switch {
			case strings.HasPrefix(target, "#"),
				strings.HasPrefix(target, "<"),
				strings.HasPrefix(target, "mailto:"),
				strings.Contains(target, "://"):
				continue
			}
			file, _, _ := strings.Cut(target, "#")
			if file != "" {
				targets = append(targets, file)
			}
		}
	}
	return targets
}

// maxLineDiff bounds how many differing lines one comparison prints.
const maxLineDiff = 40

// diffLines returns the first maxLineDiff lines that differ between before and
// after, with their 1-based line numbers in each.
func diffLines(before, after string) []string {
	oldLines := strings.Split(before, "\n")
	newLines := strings.Split(after, "\n")
	var diffs []string
	for i := 0; i < len(oldLines) || i < len(newLines); i++ {
		oldLine, newLine := "", ""
		if i < len(oldLines) {
			oldLine = oldLines[i]
		}
		if i < len(newLines) {
			newLine = newLines[i]
		}
		if oldLine == newLine {
			continue
		}
		if len(diffs) == maxLineDiff {
			diffs = append(diffs, "(further differences are not printed)")
			break
		}
		diffs = append(diffs, fmt.Sprintf("before line %d: %q / after line %d: %q", i+1, oldLine, i+1, newLine))
	}
	return diffs
}

// docLine is one line of a document as the fence scan classifies it.
type docLine struct {
	text  string
	fence bool
	block bool
}

// scanLines classifies the lines of doc as inside or outside a fenced code
// block, with the same rule as the documentation tests: an opening fence may
// carry an info string, and a closing fence is backticks alone.
func scanLines(doc string) []docLine {
	var scanned []docLine
	inBlock := false
	for line := range strings.Lines(doc) {
		text := strings.TrimSuffix(line, "\n")
		trimmed := strings.TrimSpace(text)
		fence := false
		switch {
		case !inBlock && strings.HasPrefix(trimmed, "```"):
			fence, inBlock = true, true
		case inBlock && isClosingFence(trimmed):
			fence, inBlock = true, false
		}
		scanned = append(scanned, docLine{text: text, fence: fence, block: fence || inBlock})
	}
	return scanned
}

// isClosingFence reports whether line is a closing code fence: three or more
// backticks and nothing else.
func isClosingFence(line string) bool {
	return len(line) >= 3 && strings.Trim(line, "`") == ""
}

// headingLevel reports the level of a Markdown heading line, or zero when the
// line is not one.
func headingLevel(line string) int {
	n := 0
	for n < len(line) && line[n] == '#' {
		n++
	}
	if n == 0 || n == len(line) || (line[n] != ' ' && line[n] != '\t') {
		return 0
	}
	return n
}

// markdownLinkRE matches an inline Markdown link and captures its text and
// target.
var markdownLinkRE = regexp.MustCompile(`\[([^\]]*)\]\(([^)\s]+)\)`)

// codeSpanRE matches one inline code span and captures its content.
var codeSpanRE = regexp.MustCompile("`([^`]+)`")
