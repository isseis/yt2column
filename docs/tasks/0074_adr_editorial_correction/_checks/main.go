//go:build test

// Command _checks runs the one-off diff checks for task 0074: the
// definitions and the commands extend editorial corrections to ADRs without
// touching the parts they must keep byte for byte, fixprguard changes only a
// comment, the files the task leaves alone are unchanged, and ADR-0039 loses
// only its discrepancy statement, in a commit that records it with a
// correction line. It takes the base and head commits and reads every
// document at those commits, never the working tree; only its data file comes
// from the working tree, so a temporary commit without the data can be
// checked.
//
// Run it from the repository root:
//
//	go build -tags test -o /tmp/0074check ./docs/tasks/0074_adr_editorial_correction/_checks/
//	/tmp/0074check [<base> <head>]
//
// The commits default to the merge-base of origin/main and HEAD, and HEAD. It
// prints both commit IDs and the result of each check, and it exits 0 when
// every check passes, 1 on a violation, and 2 when it cannot read its input
// (a missing or repeated heading or marker, a failed git command, or a base
// that is not an ancestor of the head). make test and CI do not run it; the
// directory starts with an underscore, which keeps it out of go list.
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"go/scanner"
	"go/token"
	"os"
	"os/exec"
	"regexp"
	"slices"
	"strings"

	"github.com/isseis/yt2column/internal/doctestutil"
)

const dataPath = "docs/tasks/0074_adr_editorial_correction/_checks/data.json"

// Paths the checks read.
const (
	requirementsProcess = "docs/dev/conventions/requirements_process.md"
	fixpr               = ".claude/commands/fixpr.md"
	localCLIReview      = ".claude/commands/_lib/local-cli-review.md"
	writeGo             = "internal/fixprguard/write.go"
)

// Headings of the sections the checks compare.
const (
	recordingHeading = "#### Recording in the PR description"
	editingHeading   = "#### Editing an approved document"
)

// The text around the two changes to "Recording in the PR description". Each
// change sits between the end of a "before" marker and the start of an
// "after" marker; the check removes that span from both versions and
// compares the rest byte for byte. Taking the span from the end of the
// previous sentence (not from the start of a line) treats the base and the
// head alike: the line break and indent before the replaced sentence, and the
// space before the added one, fall inside the span in both versions, and the
// line break inside "The\n  line contains" stays outside it, so it is
// compared.
const (
	lineBefore      = "start any other line of a commit message with `Editorial correction: `."
	lineAfter       = "The\n  line contains no `@` or `<`"
	oldPathSentence = "`<document path>` is the repository-relative path of the task document."
	paraBefore      = "one item for each correction line."
	paraAfter       = "When a PR is created, the paragraph is"
)

// unchangedPart is a part of a document that must stay byte-identical: a
// list item (the start line and every following line indented deeper than
// it) or a table (the start line and every following line that is a row at
// the same indent). The part ends where its structure ends, so a line added
// at its end is part of the comparison.
type unchangedPart struct {
	name, path, start string
	table             bool
}

var unchangedParts = []unchangedPart{
	{
		name: "fixpr task-document bullet", path: fixpr,
		start: "- its `path` is a task document (`docs/tasks/*/0[123]_*.md`) that the work tree\n",
	},
	{
		name: "local-cli-review mention check", path: localCLIReview,
		start: "   - **Mention check.** Before committing a message with a\n",
	},
	{
		name: "local-cli-review failure-kind table", path: localCLIReview,
		start: "   | Kind | Detail | Exit code | Handling |\n", table: true,
	},
}

// deletedOnMain are files of unchangedFiles that task 0075 deleted on main
// before this branch took main in. They must be absent at both commits; the
// diff check then shows that this branch did not add them back.
var deletedOnMain = []string{".claude/commands/runissue.md"}

// unchangedFiles are the files the task must not change at all.
var unchangedFiles = []string{
	".claude/commands/_lib/task-pr-handoff.md",
	".claude/commands/runpr.md",
	".claude/commands/runissue.md",
	".claude/commands/mergepr.md",
	"CLAUDE.md",
	"docs/dev/adr/0036-task-document-status-records.md",
}

// correctionLead starts every correction line.
const correctionLead = "Editorial correction: "

// data holds every string the checks compare that is not ASCII: the ADR's
// Japanese text and fixpr.md's Phase 6 heading, which has a dash. The Go
// source keeps its own strings in English.
type data struct {
	Phase6Heading string  `json:"fixpr_phase6_heading"`
	ADR           adrData `json:"adr"`
}

// adrData is ADR-0039's path, the heading of its decision section, the
// 1-based list item that loses the sentences, and the sentences, which are
// consecutive in that item.
type adrData struct {
	Path             string   `json:"path"`
	DecisionHeading  string   `json:"decision_heading"`
	Item             int      `json:"item"`
	RemovedSentences []string `json:"removed_sentences"`
}

// violation is one failed check.
type violation struct {
	name, where, text string
}

// check is one named check and the function that runs it.
type check struct {
	name, desc string
	run        func() ([]violation, error)
}

func main() {
	ok, err := run(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, "0074check:", err)
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
	d, err := loadData()
	if err != nil {
		return false, err
	}
	ok, err := isAncestor(base, head)
	if err != nil {
		return false, err
	}
	if !ok {
		return false, errors.New("the base commit is not an ancestor of the head commit")
	}

	checks := []check{
		{"recording", requirementsProcess + ": \"Recording in the PR description\" is byte-identical outside the replaced <document path> sentence and the added paragraph sentence, and the old sentence is gone", func() ([]violation, error) {
			return checkRecording(base, head)
		}},
		{"editing", requirementsProcess + ": \"Editing an approved document\" is byte-identical", func() ([]violation, error) {
			return checkSection(base, head, requirementsProcess, editingHeading)
		}},
		{"phase6", fixpr + ": Phase 6 is byte-identical", func() ([]violation, error) {
			return checkSection(base, head, fixpr, d.Phase6Heading)
		}},
	}
	for _, p := range unchangedParts {
		checks = append(checks, check{"part " + p.name, p.path + ": appears once, byte-identical", func() ([]violation, error) {
			return checkPart(base, head, p)
		}})
	}
	checks = append(checks,
		check{"unchanged-files", "the files the task leaves alone have no diff", func() ([]violation, error) {
			return checkUnchangedFiles(base, head)
		}},
		check{"fixprguard", "internal/fixprguard/ and cmd/fixprguard/ change only write.go, and write.go only in comments", func() ([]violation, error) {
			return checkFixprguard(base, head)
		}},
		check{"adr-0039", d.ADR.Path + ": the head is the base with the listed sentences removed from the decision section's item", func() ([]violation, error) {
			return checkADR(base, head, d.ADR)
		}},
		check{"corrections", "the first non-merge commit that changes ADR-0039 changes nothing else and carries a well-formed correction line for it", func() ([]violation, error) {
			return checkCorrections(base, head, d.ADR)
		}},
	)

	var violations []violation
	for _, c := range checks {
		found, err := c.run()
		if err != nil {
			return false, fmt.Errorf("%s: %w", c.name, err)
		}
		if len(found) == 0 {
			fmt.Printf("OK   %s: %s\n", c.name, c.desc)
			continue
		}
		fmt.Printf("FAIL %s: %s\n", c.name, c.desc)
		violations = append(violations, found...)
	}
	if len(violations) > 0 {
		for _, v := range violations {
			fmt.Printf("FAIL %s %s: %s\n", v.name, v.where, v.text)
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
		base, err := git("merge-base", "refs/remotes/origin/main", "HEAD")
		return strings.TrimSpace(base), "HEAD", err
	case 2:
		return args[0], args[1], nil
	default:
		return "", "", errors.New("usage: 0074check [<base> <head>]")
	}
}

func resolve(rev string) (string, error) {
	out, err := git("rev-parse", "--verify", strings.TrimSpace(rev)+"^{commit}")
	return strings.TrimSpace(out), err
}

// loadData reads the check's data from the working tree, not from the
// compared commits: a temporary commit made to break one check does not carry
// it.
func loadData() (data, error) {
	var d data
	raw, err := os.ReadFile(dataPath)
	if err != nil {
		return d, err
	}
	if err := json.Unmarshal(raw, &d); err != nil {
		return d, err
	}
	if d.Phase6Heading == "" || d.ADR.Path == "" || d.ADR.DecisionHeading == "" || d.ADR.Item < 1 || len(d.ADR.RemovedSentences) == 0 {
		return d, fmt.Errorf("%s lacks a value", dataPath)
	}
	return d, nil
}

// readFile returns path's content at commit.
func readFile(commit, path string) (string, error) {
	return git("cat-file", "blob", commit+":"+path)
}

// requirePaths fails when a path is in neither commit, so a stale path cannot
// make a diff check pass by matching nothing.
func requirePaths(base, head string, paths ...string) error {
	for _, commit := range []string{base, head} {
		for _, path := range paths {
			if _, err := git("cat-file", "-e", commit+":"+path); err != nil {
				return fmt.Errorf("%s is not in %s: %w", path, commit, err)
			}
		}
	}
	return nil
}

// section returns the section under heading, heading line included. The
// heading must appear exactly once outside code blocks; a missing or repeated
// heading is a read failure.
func section(text, heading, where string) (string, error) {
	want := strings.TrimSpace(heading)
	count := 0
	for _, h := range doctestutil.HeadingsOutsideBlocks(text) {
		if strings.TrimSpace(h) == want {
			count++
		}
	}
	if count != 1 {
		return "", fmt.Errorf("%s: heading %q appears %d times outside code blocks, want 1", where, heading, count)
	}
	lines := slices.Collect(strings.Lines(text))
	scanned := doctestutil.ScanDocLines(lines)
	start := slices.IndexFunc(scanned, func(l doctestutil.DocLine) bool {
		return !l.Block && strings.TrimSpace(l.Text) == want
	})
	level := doctestutil.HeadingLevel(scanned[start].Text)
	end := len(lines)
	for i := start + 1; i < len(scanned); i++ {
		if lv := doctestutil.HeadingLevel(scanned[i].Text); !scanned[i].Block && lv > 0 && lv <= level {
			end = i
			break
		}
	}
	return strings.Join(lines[start:end], ""), nil
}

// sectionAt reads the section under heading of path at commit.
func sectionAt(commit, path, heading string) (string, error) {
	text, err := readFile(commit, path)
	if err != nil {
		return "", err
	}
	return section(text, heading, path+" at "+commit)
}

// cutBetween returns text without the span from the end of before to the
// start of after, and the span. Each marker must appear exactly once, before
// ahead of after; anything else is a read failure.
func cutBetween(text, before, after, where string) (rest, span string, err error) {
	if n := strings.Count(text, before); n != 1 {
		return "", "", fmt.Errorf("%s: marker %q appears %d times, want 1", where, before, n)
	}
	if n := strings.Count(text, after); n != 1 {
		return "", "", fmt.Errorf("%s: marker %q appears %d times, want 1", where, after, n)
	}
	from := strings.Index(text, before) + len(before)
	to := strings.Index(text, after)
	if to < from {
		return "", "", fmt.Errorf("%s: marker %q comes before %q", where, after, before)
	}
	return text[:from] + text[to:], text[from:to], nil
}

// checkRecording compares "Recording in the PR description" with the two
// changed spans removed, and checks that the old <document path> sentence was
// the base's span and is nowhere in the head's section.
func checkRecording(base, head string) ([]violation, error) {
	cut := func(commit string) (sec, rest, lineSpan string, err error) {
		sec, err = sectionAt(commit, requirementsProcess, recordingHeading)
		if err != nil {
			return "", "", "", err
		}
		where := requirementsProcess + " at " + commit
		rest, lineSpan, err = cutBetween(sec, lineBefore, lineAfter, where)
		if err != nil {
			return "", "", "", err
		}
		rest, _, err = cutBetween(rest, paraBefore, paraAfter, where)
		return sec, rest, lineSpan, err
	}
	_, baseRest, baseSpan, err := cut(base)
	if err != nil {
		return nil, err
	}
	headSection, headRest, _, err := cut(head)
	if err != nil {
		return nil, err
	}
	var out []violation
	if baseRest != headRest {
		out = append(out, violation{"recording", requirementsProcess, "the section differs outside the two changed spans"})
	}
	if strings.TrimSpace(baseSpan) != oldPathSentence {
		out = append(out, violation{"recording", requirementsProcess, fmt.Sprintf("the base's replaced span is %q, not the old sentence", baseSpan)})
	}
	if strings.Contains(headSection, oldPathSentence) {
		out = append(out, violation{"recording", requirementsProcess, "the head still holds the old <document path> sentence"})
	}
	return out, nil
}

// checkSection compares the section under heading of path.
func checkSection(base, head, path, heading string) ([]violation, error) {
	b, err := sectionAt(base, path, heading)
	if err != nil {
		return nil, err
	}
	h, err := sectionAt(head, path, heading)
	if err != nil {
		return nil, err
	}
	if b != h {
		return []violation{{"section", path, fmt.Sprintf("%q differs", heading)}}, nil
	}
	return nil, nil
}

// extractPart returns p's text in text. The start line must appear exactly
// once.
func extractPart(text string, p unchangedPart, where string) (string, error) {
	if n := strings.Count(text, p.start); n != 1 {
		return "", fmt.Errorf("%s: start of %s appears %d times, want 1", where, p.name, n)
	}
	from := strings.Index(text, p.start)
	indent := strings.Repeat(" ", len(p.start)-len(strings.TrimLeft(p.start, " ")))
	part := p.start
	for line := range strings.Lines(text[from+len(p.start):]) {
		var inPart bool
		if p.table {
			inPart = strings.HasPrefix(line, indent+"|")
		} else {
			inPart = strings.HasPrefix(line, indent+" ") && strings.TrimSpace(line) != ""
		}
		if !inPart {
			break
		}
		part += line
	}
	return part, nil
}

func checkPart(base, head string, p unchangedPart) ([]violation, error) {
	var parts [2]string
	for i, commit := range []string{base, head} {
		text, err := readFile(commit, p.path)
		if err != nil {
			return nil, err
		}
		if parts[i], err = extractPart(text, p, p.path+" at "+commit); err != nil {
			return nil, err
		}
	}
	if parts[0] != parts[1] {
		return []violation{{"part", p.path, p.name + " differs"}}, nil
	}
	return nil, nil
}

func checkUnchangedFiles(base, head string) ([]violation, error) {
	var present []string
	for _, path := range unchangedFiles {
		if !slices.Contains(deletedOnMain, path) {
			present = append(present, path)
			continue
		}
		if _, err := git("cat-file", "-e", base+":"+path); err == nil {
			return nil, fmt.Errorf("%s, listed as deleted on main, is in %s", path, base)
		}
	}
	if err := requirePaths(base, head, present...); err != nil {
		return nil, err
	}
	changed, err := changedFiles(base, head, unchangedFiles...)
	if err != nil {
		return nil, err
	}
	var out []violation
	for _, path := range changed {
		out = append(out, violation{"unchanged-files", path, "changed"})
	}
	return out, nil
}

// checkFixprguard checks that only write.go changed under the two fixprguard
// directories, and that its token sequence without comments is unchanged.
func checkFixprguard(base, head string) ([]violation, error) {
	if err := requirePaths(base, head, "internal/fixprguard", "cmd/fixprguard", writeGo); err != nil {
		return nil, err
	}
	changed, err := changedFiles(base, head, "internal/fixprguard/", "cmd/fixprguard/")
	if err != nil {
		return nil, err
	}
	var out []violation
	for _, path := range changed {
		if path != writeGo {
			out = append(out, violation{"fixprguard", path, "changed"})
		}
	}
	var tokens [2][]string
	for i, commit := range []string{base, head} {
		src, err := readFile(commit, writeGo)
		if err != nil {
			return nil, err
		}
		if tokens[i], err = codeTokens(src, writeGo+" at "+commit); err != nil {
			return nil, err
		}
	}
	if !slices.Equal(tokens[0], tokens[1]) {
		out = append(out, violation{"fixprguard", writeGo, "the code tokens differ, not only the comments"})
	}
	return out, nil
}

// directiveRE matches a comment that is a directive to the go tool or the
// compiler (//go:build, //go:embed, //line, ...), which changes the build
// though it is a comment.
var directiveRE = regexp.MustCompile(`^//(line |[a-z0-9]+:\S)`)

// codeTokens returns src's tokens without its comments, each as its kind and
// literal; a directive comment is kept. The scanner inserts the automatic
// semicolons; their literal ("\n" or ";") does not change with a comment at
// the end of a line.
func codeTokens(src, where string) ([]string, error) {
	fset := token.NewFileSet()
	file := fset.AddFile(where, fset.Base(), len(src))
	var errs []string
	var s scanner.Scanner
	s.Init(file, []byte(src), func(pos token.Position, msg string) {
		errs = append(errs, fmt.Sprintf("%s: %s", pos, msg))
	}, scanner.ScanComments)
	var out []string
	for {
		_, tok, lit := s.Scan()
		if tok == token.EOF {
			break
		}
		if tok == token.COMMENT && !directiveRE.MatchString(lit) {
			continue
		}
		if tok == token.SEMICOLON {
			lit = ";"
		}
		out = append(out, tok.String()+" "+lit)
	}
	if len(errs) > 0 {
		return nil, fmt.Errorf("scan %s: %s", where, strings.Join(errs, "; "))
	}
	return out, nil
}

// checkADR checks that the head ADR-0039 is the base with the sentences, which
// are consecutive in the base's decision item, removed, and that none of them
// is left.
func checkADR(base, head string, a adrData) ([]violation, error) {
	baseText, err := readFile(base, a.Path)
	if err != nil {
		return nil, err
	}
	headText, err := readFile(head, a.Path)
	if err != nil {
		return nil, err
	}
	removed := strings.Join(a.RemovedSentences, "")
	if n := strings.Count(baseText, removed); n != 1 {
		return nil, fmt.Errorf("%s at %s: the removed sentences appear %d times together, want 1", a.Path, base, n)
	}
	decision, err := section(baseText, a.DecisionHeading, a.Path+" at "+base)
	if err != nil {
		return nil, err
	}
	item, err := listItem(decision, a.Item)
	if err != nil {
		return nil, fmt.Errorf("%s at %s: %w", a.Path, base, err)
	}
	var out []violation
	if !strings.Contains(item, removed) {
		out = append(out, violation{"adr-0039", a.Path, fmt.Sprintf("the sentences are not in item %d of the base's decision section", a.Item)})
	}
	if headText != strings.Replace(baseText, removed, "", 1) {
		out = append(out, violation{"adr-0039", a.Path, "the head is not the base with only the sentences removed"})
	}
	for _, s := range a.RemovedSentences {
		if strings.Contains(headText, s) {
			out = append(out, violation{"adr-0039", a.Path, fmt.Sprintf("the head still holds %q", s)})
		}
	}
	return out, nil
}

// listItem returns the n-th (1-based) top-level list item of section: the
// line starting with "-" at the first column, with its continuation lines.
func listItem(section string, n int) (string, error) {
	var items []string
	for line := range strings.Lines(section) {
		switch {
		case strings.HasPrefix(line, "-"):
			items = append(items, line)
		case len(items) > 0 && strings.HasPrefix(line, " "):
			items[len(items)-1] += line
		}
	}
	if n > len(items) {
		return "", fmt.Errorf("the decision section has %d list items, want at least %d", len(items), n)
	}
	return items[n-1], nil
}

// checkCorrections checks the first non-merge commit of the range that
// changes ADR-0039: it changes nothing else, every line starting with the
// correction lead is a well-formed correction line for ADR-0039, and there is
// at least one.
func checkCorrections(base, head string, a adrData) ([]violation, error) {
	commits, err := gitLines("rev-list", "--no-merges", "--topo-order", "--reverse", base+".."+head)
	if err != nil {
		return nil, err
	}
	first := ""
	for _, commit := range commits {
		paths, err := diffPaths(commit)
		if err != nil {
			return nil, err
		}
		if slices.Contains(paths, a.Path) {
			if first == "" {
				first = commit
				continue
			}
			fmt.Printf("NOTE corrections: later commit %s changes ADR-0039; read it in the PR review\n", commit)
		}
	}
	if first == "" {
		return []violation{{"corrections", a.Path, "no non-merge commit in the range changes ADR-0039"}}, nil
	}
	var out []violation
	paths, err := diffPaths(first)
	if err != nil {
		return nil, err
	}
	if len(paths) != 1 {
		out = append(out, violation{"corrections", first, fmt.Sprintf("the commit that changes ADR-0039 also changes %v", paths)})
	}
	message, err := git("-c", "log.showSignature=false", "log", "-1", "--format=%B", first)
	if err != nil {
		return nil, err
	}
	prefix := correctionLead + a.Path + ": "
	const suffix = "; no decision changed"
	lines := 0
	for line := range strings.SplitSeq(message, "\n") {
		if !strings.HasPrefix(line, correctionLead) {
			continue
		}
		middle := strings.TrimSuffix(strings.TrimPrefix(line, prefix), suffix)
		if strings.HasPrefix(line, prefix) && strings.HasSuffix(line, suffix) &&
			strings.TrimSpace(middle) != "" && !strings.ContainsAny(line, "@<") {
			lines++
			continue
		}
		out = append(out, violation{"corrections", first, fmt.Sprintf("malformed correction line %q", line)})
	}
	if lines == 0 {
		out = append(out, violation{"corrections", first, "the commit carries no correction line for ADR-0039"})
	}
	return out, nil
}

// changedFiles returns the paths under the pathspecs that the range changed,
// without rename detection or the user's diff settings.
func changedFiles(base, head string, pathspecs ...string) ([]string, error) {
	args := append([]string{"diff", "--no-renames", "--no-relative", "--no-ext-diff", "--name-only", base, head, "--"}, pathspecs...)
	return gitLines(args...)
}

// diffPaths returns the paths a commit changed against its parent.
func diffPaths(commit string) ([]string, error) {
	return gitLines("diff-tree", "--no-commit-id", "--name-only", "-r", "--no-renames", commit)
}

// isAncestor reports whether anc is an ancestor of desc.
func isAncestor(anc, desc string) (bool, error) {
	err := exec.Command("git", "merge-base", "--is-ancestor", anc, desc).Run()
	if err == nil {
		return true, nil
	}
	if exitErr, ok := errors.AsType[*exec.ExitError](err); ok && exitErr.ExitCode() == 1 {
		return false, nil
	}
	return false, fmt.Errorf("git merge-base --is-ancestor %s %s: %w", anc, desc, err)
}

func git(args ...string) (string, error) {
	out, err := exec.Command("git", args...).Output()
	if err != nil {
		msg := ""
		if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
			msg = strings.TrimSpace(string(exitErr.Stderr))
		}
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, msg)
	}
	return string(out), nil
}

func gitLines(args ...string) ([]string, error) {
	out, err := git(args...)
	if err != nil {
		return nil, err
	}
	out = strings.TrimSuffix(out, "\n")
	if out == "" {
		return nil, nil
	}
	return strings.Split(out, "\n"), nil
}
