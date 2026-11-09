//go:build test

// Command _checks runs the one-off diff checks for task 0070: the ADR
// convention gains its criteria and editorial-correction rules without
// touching the parts it must keep byte for byte, and ADR-0039's decision
// section swaps its "handled in the issue" sentences for links to the
// convention, recorded by correction lines. It takes the base and head commits
// and reads every document at those commits, never the working tree; only its
// data file comes from the working tree, so a temporary commit without the
// data can be checked.
//
// Run it from the repository root:
//
//	go build -tags test -o /tmp/0070check ./docs/tasks/0070_adr_writing_criteria/_checks/
//	/tmp/0070check [<base> <head>]
//
// The commits default to the merge-base of origin/main and HEAD, and HEAD. It
// prints both commit IDs and the result of each check, and it exits 0 when
// every check passes, 1 on a violation, and 2 when it cannot read its input.
// make test and CI do not run it; the directory starts with an underscore,
// which keeps it out of go list.
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"slices"
	"strings"

	"github.com/isseis/yt2column/internal/doctestutil"
)

// correctionLead starts every correction line, for any document. A line that
// starts with it but is not a well-formed ADR-0039 correction line is a
// violation, so a malformed line cannot hide among the well-formed ones.
const correctionLead = "Editorial correction: "

const dataPath = "docs/tasks/0070_adr_writing_criteria/_checks/data.json"

// data holds every Japanese string the checks compare, together with the paths
// they read. The Go source keeps its own strings in English.
type data struct {
	TaskDir    string         `json:"task_dir"`
	Convention conventionData `json:"convention"`
	ADR        adrData        `json:"adr"`
}

// conventionData names the convention's changed sections, the section the
// change adds and the heading it follows, and the unchanged parts inside the
// changed sections.
type conventionData struct {
	Path            string     `json:"path"`
	ChangedSections []string   `json:"changed_sections"`
	AddedSection    string     `json:"added_section"`
	AddedAfter      string     `json:"added_after"`
	Parts           []partData `json:"parts"`
}

// partData is one unchanged part of a changed section: it runs from the start
// marker through the end marker, or through the end of the section when End is
// empty.
type partData struct {
	Name    string `json:"name"`
	Section string `json:"section"`
	Start   string `json:"start"`
	End     string `json:"end"`
}

// adrData is ADR-0039's path, the heading of its decision section, the
// sentence swaps the decision section takes, and the correction line's shape.
type adrData struct {
	Path             string            `json:"path"`
	Section          string            `json:"section"`
	Replacements     []replacementData `json:"replacements"`
	CorrectionPrefix string            `json:"correction_prefix"`
	CorrectionSuffix string            `json:"correction_suffix"`
}

// replacementData swaps one sentence inside the decision section's list item
// at a 1-based position. The position, not the sentence, picks the item,
// because two pairs of items share the same old sentence.
type replacementData struct {
	Item int    `json:"item"`
	From string `json:"from"`
	To   string `json:"to"`
}

// violation is one failed check.
type violation struct {
	name string
	file string
	text string
}

// check is one named check and the function that runs it.
type check struct {
	name string
	desc string
	run  func() ([]violation, error)
}

func main() {
	ok, err := run(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, "0070check:", err)
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

	baseConv, err := readDoc(base, d.Convention.Path)
	if err != nil {
		return false, err
	}
	headConv, err := readDoc(head, d.Convention.Path)
	if err != nil {
		return false, err
	}

	checks := []check{
		{"scope", "every changed file is the convention, ADR-0039, or under the task directory", func() ([]violation, error) {
			return checkScope(base, head, d)
		}},
		{"structure", "the convention's headings are the base's with " + d.Convention.AddedSection + " added after " + d.Convention.AddedAfter, func() ([]violation, error) {
			return checkStructure(baseConv, headConv, d.Convention)
		}},
		{"unchanged-sections", "the text before the first level-2 heading and every section the task does not change are byte-identical", func() ([]violation, error) {
			return checkUnchangedSections(baseConv, headConv, d.Convention)
		}},
	}
	for _, p := range d.Convention.Parts {
		checks = append(checks, check{"part " + p.Name, "appears once, byte-identical, in " + p.Section, func() ([]violation, error) {
			return checkPart(baseConv, headConv, d.Convention.Path, p)
		}})
	}
	checks = append(checks,
		check{"part-order", "the unchanged parts of each changed section keep their order", func() ([]violation, error) {
			return checkPartOrder(baseConv, headConv, d.Convention)
		}},
		check{"adr-decision", d.ADR.Path + ": the head decision section is the base one with each listed sentence swapped in its item", func() ([]violation, error) {
			return checkADRDecision(base, head, d.ADR)
		}},
		check{"adr-outside", d.ADR.Path + ": everything outside the decision section is byte-identical", func() ([]violation, error) {
			return checkADROutside(base, head, d.ADR)
		}},
		check{"corrections", "the first non-merge commit that changes ADR-0039 changes nothing else and carries one correction line per swap", func() ([]violation, error) {
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
			fmt.Printf("FAIL %s %s: %s\n", v.name, v.file, v.text)
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
		return "", "", errors.New("usage: 0070check [<base> <head>]")
	}
}

func resolve(rev string) (string, error) {
	out, err := git("rev-parse", "--verify", strings.TrimSpace(rev)+"^{commit}")
	return strings.TrimSpace(out), err
}

// loadData reads the check's data from the working tree, not from the compared
// commits: a temporary commit made to break one check does not carry it.
func loadData() (data, error) {
	var d data
	raw, err := os.ReadFile(dataPath)
	if err != nil {
		return d, err
	}
	if err := json.Unmarshal(raw, &d); err != nil {
		return d, err
	}
	return d, nil
}

// doc is one document at one commit: its lines with their newlines, and the
// code-fence classification of each line.
type doc struct {
	path    string
	commit  string
	text    string
	lines   []string
	scanned []doctestutil.DocLine
}

func readDoc(commit, path string) (doc, error) {
	text, err := git("show", commit+":"+path)
	if err != nil {
		return doc{}, err
	}
	lines := slices.Collect(strings.Lines(text))
	return doc{path: path, commit: commit, text: text, lines: lines, scanned: doctestutil.ScanDocLines(lines)}, nil
}

// where names the document and commit in a read failure.
func (d doc) where() string {
	return d.path + " at " + d.commit
}

// sectionRange returns the line index of the heading and the index just past
// the section's last line. The heading must appear exactly once outside code
// blocks: SectionOutsideBlocks would return an empty section for a missing
// heading and the first one for a repeated heading, so a missing or repeated
// heading is a read failure here.
func sectionRange(d doc, heading string) (int, int, error) {
	want := strings.TrimSpace(heading)
	count := 0
	for _, h := range doctestutil.HeadingsOutsideBlocks(d.text) {
		if strings.TrimSpace(h) == want {
			count++
		}
	}
	if count != 1 {
		return 0, 0, fmt.Errorf("%s: heading %q appears %d times outside code blocks, want 1", d.where(), heading, count)
	}
	start := slices.IndexFunc(d.scanned, func(l doctestutil.DocLine) bool {
		return !l.Block && strings.TrimSpace(l.Text) == want
	})
	level := doctestutil.HeadingLevel(d.scanned[start].Text)
	end := len(d.lines)
	for i := start + 1; i < len(d.scanned); i++ {
		l := d.scanned[i]
		if lv := doctestutil.HeadingLevel(l.Text); !l.Block && lv > 0 && lv <= level {
			end = i
			break
		}
	}
	return start, end, nil
}

// sectionBody returns the section's text after its heading line.
func sectionBody(d doc, heading string) (string, error) {
	start, end, err := sectionRange(d, heading)
	if err != nil {
		return "", err
	}
	return strings.Join(d.lines[start+1:end], ""), nil
}

// sectionWithHeading returns the section's text with its heading line as
// written, so trailing spaces on the heading are compared too.
func sectionWithHeading(d doc, heading string) (string, error) {
	start, end, err := sectionRange(d, heading)
	if err != nil {
		return "", err
	}
	return strings.Join(d.lines[start:end], ""), nil
}

// level2Headings returns the trimmed level-2 headings outside code blocks.
func level2Headings(d doc) []string {
	var out []string
	for _, h := range doctestutil.HeadingsOutsideBlocks(d.text) {
		if doctestutil.HeadingLevel(h) == 2 {
			out = append(out, strings.TrimSpace(h))
		}
	}
	return out
}

// checkScope verifies that the range changes only the convention, ADR-0039,
// and files under the task directory.
func checkScope(base, head string, d data) ([]violation, error) {
	changed, err := changedFiles(base, head)
	if err != nil {
		return nil, err
	}
	var out []violation
	for _, path := range changed {
		if path == d.Convention.Path || path == d.ADR.Path || strings.HasPrefix(path, d.TaskDir) {
			continue
		}
		out = append(out, violation{"scope", path, "a file outside the allowed scope changed"})
	}
	return out, nil
}

// checkStructure verifies that the head's heading lines outside code blocks,
// of every level and byte for byte, are the base's with the added section
// inserted right after the heading it follows.
func checkStructure(base, head doc, c conventionData) ([]violation, error) {
	baseHeadings := doctestutil.HeadingsOutsideBlocks(base.text)
	if slices.Contains(baseHeadings, c.AddedSection) {
		return nil, fmt.Errorf("%s: the added heading %q already exists", base.where(), c.AddedSection)
	}
	i := slices.Index(baseHeadings, c.AddedAfter)
	if i < 0 {
		return nil, fmt.Errorf("%s: heading %q not found", base.where(), c.AddedAfter)
	}
	want := slices.Insert(slices.Clone(baseHeadings), i+1, c.AddedSection)
	got := doctestutil.HeadingsOutsideBlocks(head.text)
	if !slices.Equal(got, want) {
		return []violation{{"structure", head.path, fmt.Sprintf("headings are %q, want %q", got, want)}}, nil
	}
	return nil, nil
}

// checkUnchangedSections verifies that the text before the first level-2
// heading, and every level-2 section of the base outside the changed list,
// is byte-identical at the head.
func checkUnchangedSections(base, head doc, c conventionData) ([]violation, error) {
	var out []violation
	basePre, err := preamble(base)
	if err != nil {
		return nil, err
	}
	headPre, err := preamble(head)
	if err != nil {
		return nil, err
	}
	if basePre != headPre {
		out = append(out, violation{"unchanged-sections", head.path, "the text before the first level-2 heading changed"})
	}
	for _, heading := range level2Headings(base) {
		if slices.Contains(c.ChangedSections, heading) {
			continue
		}
		baseBody, err := sectionWithHeading(base, heading)
		if err != nil {
			return nil, err
		}
		headBody, err := sectionWithHeading(head, heading)
		if err != nil {
			return nil, err
		}
		if baseBody != headBody {
			out = append(out, violation{"unchanged-sections", head.path, fmt.Sprintf("section %q changed", heading)})
		}
	}
	return out, nil
}

// preamble returns the text before the first level-2 heading outside code
// blocks.
func preamble(d doc) (string, error) {
	i := slices.IndexFunc(d.scanned, func(l doctestutil.DocLine) bool {
		return !l.Block && doctestutil.HeadingLevel(l.Text) == 2
	})
	if i < 0 {
		return "", fmt.Errorf("%s: no level-2 heading", d.where())
	}
	return strings.Join(d.lines[:i], ""), nil
}

// extractPart cuts a part out of a section body. The start marker must appear
// exactly once in the section, and the end marker exactly once after it.
func extractPart(body string, p partData, where string) (string, error) {
	if n := strings.Count(body, p.Start); n != 1 {
		return "", fmt.Errorf("%s: part %s: start marker appears %d times in %q, want 1", where, p.Name, n, p.Section)
	}
	start := strings.Index(body, p.Start)
	if p.End == "" {
		return body[start:], nil
	}
	rest := body[start:]
	if n := strings.Count(rest, p.End); n != 1 {
		return "", fmt.Errorf("%s: part %s: end marker appears %d times after the start marker, want 1", where, p.Name, n)
	}
	return rest[:strings.Index(rest, p.End)+len(p.End)], nil
}

// checkPart verifies that the base part appears exactly once in the same
// section at the head.
func checkPart(base, head doc, path string, p partData) ([]violation, error) {
	baseBody, err := sectionBody(base, p.Section)
	if err != nil {
		return nil, err
	}
	part, err := extractPart(baseBody, p, base.where())
	if err != nil {
		return nil, err
	}
	headBody, err := sectionBody(head, p.Section)
	if err != nil {
		return nil, err
	}
	if n := strings.Count(headBody, part); n != 1 {
		return []violation{{"part " + p.Name, path, fmt.Sprintf("the base part appears %d times in %q at the head, want 1", n, p.Section)}}, nil
	}
	return nil, nil
}

// checkPartOrder verifies that, in each changed section, the parts that the
// head holds once each come in the base's order. A part missing from the head
// is reported by its own check, not here.
func checkPartOrder(base, head doc, c conventionData) ([]violation, error) {
	var out []violation
	for _, section := range c.ChangedSections {
		baseBody, err := sectionBody(base, section)
		if err != nil {
			return nil, err
		}
		headBody, err := sectionBody(head, section)
		if err != nil {
			return nil, err
		}
		baseEnd, headPrev := 0, -1
		for _, p := range c.Parts {
			if p.Section != section {
				continue
			}
			part, err := extractPart(baseBody, p, base.where())
			if err != nil {
				return nil, err
			}
			basePos := strings.Index(baseBody, part)
			if basePos < baseEnd {
				return nil, fmt.Errorf("%s: part %s overlaps or precedes an earlier part in the base", base.where(), p.Name)
			}
			baseEnd = basePos + len(part)
			if strings.Count(headBody, part) != 1 {
				continue
			}
			headPos := strings.Index(headBody, part)
			if headPos <= headPrev {
				out = append(out, violation{"part-order", head.path, fmt.Sprintf("part %s comes before an earlier part in %q", p.Name, section)})
			}
			headPrev = max(headPrev, headPos)
		}
	}
	return out, nil
}

// checkADRDecision verifies that the head decision section equals the base
// one with each listed sentence, found exactly once in its item at the base,
// swapped for its replacement. Comparing the whole section also checks the
// items no swap names and the number of items.
func checkADRDecision(base, head string, a adrData) ([]violation, error) {
	baseDoc, err := readDoc(base, a.Path)
	if err != nil {
		return nil, err
	}
	headDoc, err := readDoc(head, a.Path)
	if err != nil {
		return nil, err
	}
	baseStart, baseEnd, err := sectionRange(baseDoc, a.Section)
	if err != nil {
		return nil, err
	}
	headStart, headEnd, err := sectionRange(headDoc, a.Section)
	if err != nil {
		return nil, err
	}
	items, err := listItems(baseDoc.lines[baseStart+1 : baseEnd])
	if err != nil {
		return nil, fmt.Errorf("%s: %w", baseDoc.where(), err)
	}
	for _, r := range a.Replacements {
		if r.Item < 1 || r.Item > len(items) {
			return nil, fmt.Errorf("%s: the decision section has %d items, no item %d", baseDoc.where(), len(items), r.Item)
		}
		text := items[r.Item-1]
		if n := strings.Count(*text, r.From); n != 1 {
			return nil, fmt.Errorf("%s: item %d holds the old sentence %d times, want 1", baseDoc.where(), r.Item, n)
		}
		*text = strings.Replace(*text, r.From, r.To, 1)
	}
	want := strings.Join(baseDoc.lines[baseStart+1:baseEnd], "")
	got := strings.Join(headDoc.lines[headStart+1:headEnd], "")
	if got != want {
		return []violation{{"adr-decision", a.Path, "the decision section is not the base one with the listed swaps"}}, nil
	}
	return nil, nil
}

// listItems returns a pointer to the first line of each top-level list item,
// so a swap edits the lines in place. An item's sentence must sit on its first
// line; ADR-0039 writes each item on one line.
func listItems(lines []string) ([]*string, error) {
	var items []*string
	for i := range lines {
		if strings.HasPrefix(lines[i], "- ") {
			items = append(items, &lines[i])
		}
	}
	if len(items) == 0 {
		return nil, errors.New("the decision section has no list item")
	}
	return items, nil
}

// checkADROutside verifies that ADR-0039 outside its decision section is
// byte-identical, so no history entry was added.
func checkADROutside(base, head string, a adrData) ([]violation, error) {
	baseDoc, err := readDoc(base, a.Path)
	if err != nil {
		return nil, err
	}
	headDoc, err := readDoc(head, a.Path)
	if err != nil {
		return nil, err
	}
	baseMasked, err := maskSection(baseDoc, a.Section)
	if err != nil {
		return nil, err
	}
	headMasked, err := maskSection(headDoc, a.Section)
	if err != nil {
		return nil, err
	}
	if baseMasked != headMasked {
		return []violation{{"adr-outside", a.Path, "a part outside the decision section changed"}}, nil
	}
	return nil, nil
}

// maskSection joins the document with the section's body replaced by a marker.
func maskSection(d doc, heading string) (string, error) {
	start, end, err := sectionRange(d, heading)
	if err != nil {
		return "", err
	}
	return strings.Join(d.lines[:start+1], "") + "\x00section\x00" + strings.Join(d.lines[end:], ""), nil
}

// checkCorrections verifies the first non-merge commit in the range that
// changes ADR-0039: it exists, it changes nothing else, and its message has
// one correction line per swap. Later commits that change ADR-0039 revise the
// PR's own sentences and carry no correction line; their IDs are listed for
// the PR review. Merge commits are skipped: a commit that takes in main is not
// one of the PR's own changes to ADR-0039.
func checkCorrections(base, head string, a adrData) ([]violation, error) {
	commits, err := gitLines("rev-list", "--no-merges", "--reverse", base+".."+head)
	if err != nil {
		return nil, err
	}
	var adrCommits []string
	for _, commit := range commits {
		paths, err := diffPaths(commit)
		if err != nil {
			return nil, err
		}
		if slices.Contains(paths, a.Path) {
			adrCommits = append(adrCommits, commit)
		}
	}
	if len(adrCommits) == 0 {
		return []violation{{"corrections", a.Path, "no non-merge commit in the range changes ADR-0039"}}, nil
	}
	first := adrCommits[0]
	for _, later := range adrCommits[1:] {
		fmt.Printf("NOTE corrections: later commit %s changes ADR-0039; read it in the PR review\n", later)
	}
	var out []violation
	paths, err := diffPaths(first)
	if err != nil {
		return nil, err
	}
	if len(paths) != 1 {
		out = append(out, violation{"corrections", first, fmt.Sprintf("the commit that changes ADR-0039 also changes %v", paths)})
	}
	message, err := git("log", "-1", "--format=%B", first)
	if err != nil {
		return nil, err
	}
	lines := 0
	for line := range strings.SplitSeq(message, "\n") {
		switch {
		case isCorrectionLine(line, a):
			lines++
		case strings.HasPrefix(line, correctionLead):
			out = append(out, violation{"corrections", first, fmt.Sprintf("malformed correction line %q", line)})
		}
	}
	if lines != len(a.Replacements) {
		out = append(out, violation{"corrections", first, fmt.Sprintf("the commit carries %d correction lines, want %d", lines, len(a.Replacements))})
	}
	return out, nil
}

// isCorrectionLine reports whether line is a correction line for ADR-0039
// that starts in the first column, has a non-empty description, and holds
// neither the mention character nor the angle bracket that must not appear.
func isCorrectionLine(line string, a adrData) bool {
	if !strings.HasPrefix(line, a.CorrectionPrefix) || !strings.HasSuffix(line, a.CorrectionSuffix) {
		return false
	}
	middle := strings.TrimSuffix(strings.TrimPrefix(line, a.CorrectionPrefix), a.CorrectionSuffix)
	return strings.TrimSpace(middle) != "" && !strings.ContainsAny(line, "@<")
}

// changedFiles returns the sorted paths the range changed, without rename
// detection or the user's diff settings.
func changedFiles(base, head string) ([]string, error) {
	out, err := git("diff", "--no-renames", "--no-relative", "--no-ext-diff", "--name-only", base, head)
	if err != nil {
		return nil, err
	}
	var paths []string
	for line := range strings.SplitSeq(out, "\n") {
		if line != "" {
			paths = append(paths, line)
		}
	}
	slices.Sort(paths)
	return paths, nil
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
