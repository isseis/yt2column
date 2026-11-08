//go:build test

// Command _checks runs the one-off checks for task 0067: the terminology
// replacement of the pipeline unit and its related words keeps the target
// documents, the glossary, CLAUDE.md, the ADRs, and the change's scope as the
// approved documents require. It takes the base and head commits and reads each
// file at those commits, never the working tree.
//
// Run it from the repository root:
//
//	go build -tags test -o /tmp/0067check ./docs/tasks/0067_stage_terminology/_checks/
//	/tmp/0067check [<base> <head>]
//
// The commits default to the merge-base of origin/main and HEAD, and HEAD. It
// prints both commit IDs, every allowed wording with its count, and each
// violation as "file:line: text". It exits 0 when every check passes, 1 on a
// violation, and 2 when it cannot check its input. make test and CI do not run
// it; the directory starts with an underscore, which keeps it out of go list.
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/isseis/yt2column/internal/doctestutil"
)

const (
	dataPath     = "docs/tasks/0067_stage_terminology/_checks/data.json"
	taskDir      = "docs/tasks/0067_stage_terminology/"
	readmePath   = "README.md"
	readmeENPath = "README.en.md"
	packageRef   = "docs/dev/conventions/package_reference.md"
	extensionDir = "extension/"
	docsTest     = "cmd/yt2column/docs_test.go"
	adrPattern   = `^docs/dev/adr/[0-9][0-9][0-9][0-9]-[^/]+\.md$`
)

// data holds every Japanese string the checks compare together with the paths.
// The Go source keeps its own strings in English.
type data struct {
	Glossary        glossaryData        `json:"glossary"`
	PipelineProfile pipelineProfileData `json:"pipeline_profile"`
	Allowed         []allowedGroup      `json:"allowed"`
	Claude          claudeData          `json:"claude"`
	ADRHistory      adrHistoryData      `json:"adr_history"`
	TargetDirs      []string            `json:"target_dirs"`
	TargetFiles     []string            `json:"target_files"`
}

// glossaryData is the glossary's path and the words the check compares.
type glossaryData struct {
	Path    string `json:"path"`
	OldWord string `json:"old_word"`
	NewWord string `json:"new_word"`
	English string `json:"english"`
	OldName string `json:"old_name"`
}

// pipelineProfileData is the production profile's pipeline section and column.
type pipelineProfileData struct {
	Path    string `json:"path"`
	Section string `json:"section"`
	Column  string `json:"column"`
}

// claudeData is CLAUDE.md's path and the three phrase replacements.
type claudeData struct {
	Path         string   `json:"path"`
	Replacements []phrase `json:"replacements"`
}

// adrHistoryData is the ADR history heading and the assigned words.
type adrHistoryData struct {
	Heading string   `json:"heading"`
	Words   []string `json:"words"`
}

// allowedGroup is one wording that may keep the old word, with the number of
// times it must appear in its file.
type allowedGroup struct {
	File  string `json:"file"`
	Text  string `json:"text"`
	Count int    `json:"count"`
}

// phrase is one replacement in CLAUDE.md: the wording at the base commit and
// the wording at the head commit.
type phrase struct {
	Before string `json:"before"`
	After  string `json:"after"`
}

// violation is one failed check at a line of a file.
type violation struct {
	file string
	line int
	text string
}

var adrRE = regexp.MustCompile(adrPattern)

func main() {
	ok, err := run(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, "0067check:", err)
		os.Exit(2)
	}
	if !ok {
		os.Exit(1)
	}
}

func run(args []string) (bool, error) {
	baseRev, headRev, err := revisions(args)
	if err != nil {
		return false, err
	}
	base, err := resolve(baseRev)
	if err != nil {
		return false, err
	}
	head, err := resolve(headRev)
	if err != nil {
		return false, err
	}
	fmt.Printf("base %s\nhead %s\n", base, head)

	d, err := loadData(head)
	if err != nil {
		return false, err
	}
	changed, err := diffPaths(base, head)
	if err != nil {
		return false, err
	}
	basePaths, err := treePaths(base)
	if err != nil {
		return false, err
	}
	headPaths, err := treePaths(head)
	if err != nil {
		return false, err
	}

	var violations []violation
	violations = append(violations, checkScope(changed)...)
	v, err := checkTargets(basePaths, head, headPaths, d)
	if err != nil {
		return false, err
	}
	violations = append(violations, v...)
	v, err = checkGlossary(base, head, d.Glossary)
	if err != nil {
		return false, err
	}
	violations = append(violations, v...)
	v, err = checkPipelineProfile(head, d.PipelineProfile)
	if err != nil {
		return false, err
	}
	violations = append(violations, v...)
	v, err = checkClaude(base, head, d.Claude)
	if err != nil {
		return false, err
	}
	violations = append(violations, v...)
	v, err = checkADRHistory(base, head, changed, basePaths, d.ADRHistory, d.Glossary.OldWord)
	if err != nil {
		return false, err
	}
	violations = append(violations, v...)

	if len(violations) > 0 {
		for _, one := range violations {
			fmt.Printf("%s:%d: %s\n", one.file, one.line, one.text)
		}
		fmt.Printf("FAIL %d violation(s)\n", len(violations))
		return false, nil
	}
	fmt.Println("OK every check passed")
	return true, nil
}

func revisions(args []string) (string, string, error) {
	switch len(args) {
	case 0:
		base, err := git("merge-base", "origin/main", "HEAD")
		return strings.TrimSpace(base), "HEAD", err
	case 2:
		return args[0], args[1], nil
	default:
		return "", "", errors.New("usage: 0067check [<base> <head>]")
	}
}

func resolve(rev string) (string, error) {
	rev = strings.TrimSpace(rev)
	out, err := git("rev-parse", "--verify", rev+"^{commit}")
	return strings.TrimSpace(out), err
}

// loadData reads the check's data at head, like the documents it checks, so a
// working-tree edit cannot change what the committed head is checked against.
func loadData(head string) (data, error) {
	var d data
	raw, err := read(head, dataPath)
	if err != nil {
		return d, err
	}
	if err := json.Unmarshal([]byte(raw), &d); err != nil {
		return d, err
	}
	return d, nil
}

// diffPaths returns the paths the head commit changed since the base commit,
// without rename detection or the user's diff settings.
func diffPaths(base, head string) ([]string, error) {
	return gitPaths("diff", "-z", "--no-renames", "--no-relative", "--no-ext-diff", "--name-only", base, head)
}

func treePaths(commit string) ([]string, error) {
	return gitPaths("ls-tree", "-r", "-z", "--name-only", commit)
}

// checkScope reports a change outside the scope the requirements allow.
func checkScope(changed []string) []violation {
	var out []violation
	for _, path := range changed {
		switch {
		case strings.HasPrefix(path, "docs/tasks/") && !strings.HasPrefix(path, taskDir):
			out = append(out, violation{path, 1, "a task document outside this task changed"})
		case path == readmePath || path == readmeENPath || path == packageRef:
			out = append(out, violation{path, 1, "a document that must not change changed"})
		case strings.HasPrefix(path, extensionDir):
			out = append(out, violation{path, 1, "a file under extension/ changed"})
		case strings.HasSuffix(path, ".go") && !strings.HasPrefix(path, taskDir) && path != docsTest:
			out = append(out, violation{path, 1, "a Go file other than the document test changed"})
		}
	}
	return out
}

// checkTargets verifies each target document: every allowed wording appears
// its stated number of times, and after removing the wordings no old word
// remains, counting an occurrence split by a line break. A target document
// present at the base and absent at the head is a violation, so deleting a
// file cannot make its wording pass.
func checkTargets(basePaths []string, head string, headPaths []string, d data) ([]violation, error) {
	baseSet := map[string]struct{}{}
	for _, path := range basePaths {
		baseSet[path] = struct{}{}
	}
	headSet := map[string]struct{}{}
	var targets []string
	for _, path := range headPaths {
		headSet[path] = struct{}{}
		if isTarget(path, d) {
			targets = append(targets, path)
		}
	}
	for _, path := range basePaths {
		if isTarget(path, d) {
			targets = append(targets, path)
		}
	}
	groups := map[string][]allowedGroup{}
	for _, g := range d.Allowed {
		groups[g.File] = append(groups[g.File], g)
		if _, ok := headSet[g.File]; !ok {
			targets = append(targets, g.File)
		}
	}
	targets = slices.Compact(slices.Sorted(slices.Values(targets)))

	var out []violation
	for _, path := range targets {
		if _, ok := headSet[path]; !ok {
			if _, was := baseSet[path]; was {
				out = append(out, violation{path, 1, "a target document was deleted"})
			} else {
				out = append(out, violation{path, 1, "an allowed wording's file is missing"})
			}
			continue
		}
		text, err := read(head, path)
		if err != nil {
			return nil, err
		}
		remaining := text
		for _, g := range groups[path] {
			found := strings.Count(text, g.Text)
			fmt.Printf("ALLOWED %s %d/%d %s\n", path, found, g.Count, g.Text)
			if found != g.Count {
				line := lineNumber(text, strings.Index(text, g.Text))
				out = append(out, violation{path, line, fmt.Sprintf("allowed wording appears %d times, want %d: %s", found, g.Count, g.Text)})
			}
			remaining = strings.ReplaceAll(remaining, g.Text, "\x00")
		}
		out = append(out, remainingWord(path, remaining, d.Glossary.OldWord)...)
	}
	return out, nil
}

// isTarget reports whether path is one of the documents the terminology change
// covers. The glossary has its own check.
func isTarget(path string, d data) bool {
	if path == d.Glossary.Path {
		return false
	}
	if strings.HasSuffix(path, ".md") && slices.ContainsFunc(d.TargetDirs, func(dir string) bool {
		return strings.HasPrefix(path, dir)
	}) {
		return true
	}
	return slices.Contains(d.TargetFiles, path)
}

// remainingWord reports every occurrence of word in text, including an
// occurrence split by a line break and the next line's indentation.
func remainingWord(path, text, word string) []violation {
	norm, lineOf := foldLines(text)
	var out []violation
	for start := 0; start < len(norm); {
		i := strings.Index(norm[start:], word)
		if i < 0 {
			break
		}
		at := start + i
		out = append(out, violation{path, lineOf[at], "the old word remains: " + word})
		start = at + len(word)
	}
	return out
}

// foldLines joins text's lines with their leading indentation removed, so a
// word split by a line break is found, and maps each byte to its line.
func foldLines(text string) (string, []int) {
	var b strings.Builder
	var lineOf []int
	for i, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			b.WriteString("\x00")
			lineOf = append(lineOf, i+1)
			continue
		}
		b.WriteString(line)
		for range len(line) {
			lineOf = append(lineOf, i+1)
		}
	}
	return b.String(), lineOf
}

// lineNumber returns the 1-based line of byte offset in text.
func lineNumber(text string, offset int) int {
	if offset < 0 {
		return 1
	}
	return strings.Count(text[:offset], "\n") + 1
}

// firstLineWith returns the 1-based line of the first occurrence of substr.
func firstLineWith(text, substr string) int {
	return lineNumber(text, strings.Index(text, substr))
}

// checkGlossary verifies the glossary: the old word appears once, inside the
// old-name remark, and the stage row keeps its position with the new word, the
// English word, and the old name.
func checkGlossary(base, head string, g glossaryData) ([]violation, error) {
	baseText, err := read(base, g.Path)
	if err != nil {
		return nil, err
	}
	headText, err := read(head, g.Path)
	if err != nil {
		return nil, err
	}
	var out []violation
	oldNameLine := firstLineWith(headText, g.OldName)
	if n := strings.Count(headText, g.OldName); n != 1 {
		out = append(out, violation{g.Path, oldNameLine, fmt.Sprintf("the old name appears %d times, want 1: %s", n, g.OldName)})
	}
	rest := strings.ReplaceAll(headText, g.OldName, "\x00")
	out = append(out, remainingWord(g.Path, rest, g.OldWord)...)

	tables := doctestutil.MarkdownTables(headText)
	baseTable, baseRow := findRow(doctestutil.MarkdownTables(baseText), g.OldWord)
	if baseTable < 0 || baseRow < 0 {
		return nil, fmt.Errorf("%s: the old word has no table row at the base commit", g.Path)
	}
	newTable, newRow := findRow(tables, g.NewWord)
	stageLine := firstLineWith(headText, "| "+g.NewWord+" |")
	if newTable < 0 || newRow < 0 {
		out = append(out, violation{g.Path, stageLine, "the stage row is missing or appears more than once: " + g.NewWord})
	} else {
		if newTable != baseTable || newRow != baseRow {
			out = append(out, violation{g.Path, stageLine, "the stage row moved from the base commit's position"})
		}
		row := tables[newTable][newRow]
		if len(row) < 3 {
			out = append(out, violation{g.Path, stageLine, "the stage row has fewer than three cells"})
		} else {
			if doctestutil.StripBackticks(row[1]) != g.English {
				out = append(out, violation{g.Path, stageLine, "the stage row's English cell is not " + g.English})
			}
			if !strings.Contains(row[2], g.OldName) {
				out = append(out, violation{g.Path, stageLine, "the stage row's remark does not hold the old name: " + g.OldName})
			}
		}
	}
	if n := countRows(tables, g.OldWord); n > 0 {
		oldLine := firstLineWith(headText, "| "+g.OldWord+" |")
		out = append(out, violation{g.Path, oldLine, fmt.Sprintf("a row whose first cell is the old word remains (%d)", n)})
	}
	return out, nil
}

// countRows returns the number of rows whose first cell is first.
func countRows(tables [][][]string, first string) int {
	n := 0
	for _, cells := range tables {
		for _, cell := range cells {
			if len(cell) > 0 && doctestutil.StripBackticks(cell[0]) == first {
				n++
			}
		}
	}
	return n
}

// findRow returns the first table and row whose first cell is first. It
// returns -1, -1 when there is none or more than one.
func findRow(tables [][][]string, first string) (int, int) {
	table, row := -1, -1
	for i, cells := range tables {
		for j, cell := range cells {
			if len(cell) == 0 || doctestutil.StripBackticks(cell[0]) != first {
				continue
			}
			if table >= 0 {
				return -1, -1
			}
			table, row = i, j
		}
	}
	return table, row
}

// checkPipelineProfile verifies the pipeline table's first column name.
func checkPipelineProfile(head string, p pipelineProfileData) ([]violation, error) {
	text, err := read(head, p.Path)
	if err != nil {
		return nil, err
	}
	section := doctestutil.SectionOutsideBlocks(text, p.Section)
	if section == "" {
		return nil, fmt.Errorf("%s: section %q not found", p.Path, p.Section)
	}
	tables := doctestutil.MarkdownTables(section)
	if len(tables) == 0 || len(tables[0]) == 0 || len(tables[0][0]) == 0 {
		return []violation{{p.Path, firstLineWith(text, "| "), "the pipeline section has no table"}}, nil
	}
	if got := doctestutil.StripBackticks(tables[0][0][0]); got != p.Column {
		return []violation{{p.Path, firstLineWith(text, "| "+got+" |"), fmt.Sprintf("the pipeline table's first column is %q, want %q", got, p.Column)}}, nil
	}
	fmt.Printf("OK   %s: the pipeline table's first column is %s\n", p.Path, p.Column)
	return nil, nil
}

// checkClaude verifies that CLAUDE.md changed in exactly the three places,
// allowing a space in a phrase to stand for a line break and indentation.
func checkClaude(base, head string, c claudeData) ([]violation, error) {
	baseText, err := read(base, c.Path)
	if err != nil {
		return nil, err
	}
	headText, err := read(head, c.Path)
	if err != nil {
		return nil, err
	}
	var out []violation
	headSkeleton := headText
	baseSkeleton := baseText
	for i, r := range c.Replacements {
		marker := "\x00" + strconv.Itoa(i) + "\x00"
		afterRE, err := phrasePattern(r.After)
		if err != nil {
			return nil, err
		}
		beforeRE, err := phrasePattern(r.Before)
		if err != nil {
			return nil, err
		}
		afterMatches := afterRE.FindAllStringIndex(headText, -1)
		if len(afterMatches) != 1 {
			line := 1
			if len(afterMatches) > 0 {
				line = lineNumber(headText, afterMatches[0][0])
			}
			out = append(out, violation{c.Path, line, fmt.Sprintf("the changed wording appears %d times, want 1: %s", len(afterMatches), r.After)})
		}
		beforeMatches := beforeRE.FindAllStringIndex(baseText, -1)
		if len(beforeMatches) != 1 {
			line := 1
			if len(beforeMatches) > 0 {
				line = lineNumber(baseText, beforeMatches[0][0])
			}
			out = append(out, violation{c.Path, line, fmt.Sprintf("the base wording appears %d times, want 1: %s", len(beforeMatches), r.Before)})
		}
		headSkeleton = afterRE.ReplaceAllLiteralString(headSkeleton, marker)
		baseSkeleton = beforeRE.ReplaceAllLiteralString(baseSkeleton, marker)
	}
	if headSkeleton != baseSkeleton {
		out = append(out, violation{c.Path, 1, "lines outside the three changed phrases differ from the base commit"})
	}
	if len(out) == 0 {
		fmt.Printf("OK   %s: only the three expected phrases changed\n", c.Path)
	}
	return out, nil
}

// phrasePattern returns a pattern that matches the phrase with each of its
// ASCII spaces standing for a run of white space, a line break included.
func phrasePattern(phrase string) (*regexp.Regexp, error) {
	parts := strings.Split(phrase, " ")
	for i, part := range parts {
		parts[i] = regexp.QuoteMeta(part)
	}
	return regexp.Compile(strings.Join(parts, `\s+`))
}

// checkADRHistory verifies that each changed ADR's history keeps its lines and
// changes only the old word, each occurrence to an assigned word.
func checkADRHistory(base, head string, changed, basePaths []string, h adrHistoryData, old string) ([]violation, error) {
	baseSet := map[string]struct{}{}
	for _, path := range basePaths {
		baseSet[path] = struct{}{}
	}
	var out []violation
	for _, path := range changed {
		if !adrRE.MatchString(path) {
			continue
		}
		headText, err := read(head, path)
		if err != nil {
			return nil, err
		}
		headLines, headStart, err := historyLines(headText, h.Heading)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		if _, ok := baseSet[path]; !ok {
			continue
		}
		baseText, err := read(base, path)
		if err != nil {
			return nil, err
		}
		baseLines, _, err := historyLines(baseText, h.Heading)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		if len(baseLines) != len(headLines) {
			out = append(out, violation{path, headStart, fmt.Sprintf("the history has %d lines, want %d", len(headLines), len(baseLines))})
			continue
		}
		for i := range baseLines {
			if !wordReplacement(baseLines[i], headLines[i], old, h.Words) {
				out = append(out, violation{path, headStart + i, "a history line changed beyond the word replacement"})
			}
		}
	}
	return out, nil
}

// historyLines returns the lines from the heading to the end of text. The
// heading must appear exactly once.
func historyLines(text, heading string) ([]string, int, error) {
	lines := strings.Split(text, "\n")
	found := -1
	for i, line := range lines {
		if strings.TrimSpace(line) != heading {
			continue
		}
		if found >= 0 {
			return nil, 0, fmt.Errorf("history heading %q appears more than once", heading)
		}
		found = i
	}
	if found < 0 {
		return nil, 0, fmt.Errorf("history heading %q not found", heading)
	}
	return lines[found:], found + 1, nil
}

// wordReplacement reports whether head equals base with each occurrence of the
// old word replaced by one of words.
func wordReplacement(base, head, old string, words []string) bool {
	if !strings.Contains(base, old) {
		return head == base
	}
	quoted := make([]string, len(words))
	for i, w := range words {
		quoted[i] = regexp.QuoteMeta(w)
	}
	parts := strings.Split(base, old)
	var b strings.Builder
	b.WriteString("^")
	b.WriteString(regexp.QuoteMeta(parts[0]))
	for _, part := range parts[1:] {
		b.WriteString("(?:")
		b.WriteString(strings.Join(quoted, "|"))
		b.WriteString(")")
		b.WriteString(regexp.QuoteMeta(part))
	}
	b.WriteString("$")
	re, err := regexp.Compile(b.String())
	if err != nil {
		return false
	}
	return re.MatchString(head)
}

func read(commit, path string) (string, error) {
	return git("show", commit+":"+path)
}

func git(args ...string) (string, error) {
	out, err := exec.Command("git", args...).Output()
	if err != nil {
		var exitErr *exec.ExitError
		msg := ""
		if errors.As(err, &exitErr) {
			msg = strings.TrimSpace(string(exitErr.Stderr))
		}
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, msg)
	}
	return string(out), nil
}

func gitPaths(args ...string) ([]string, error) {
	out, err := git(args...)
	if err != nil {
		return nil, err
	}
	paths := strings.Split(out, "\x00")
	return slices.DeleteFunc(paths, func(s string) bool { return s == "" }), nil
}
