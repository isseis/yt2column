//go:build test

// Command _checks runs the one-off checks for task 0069: the display-language
// paragraph of README.md now covers both the popup and the viewer, its English
// counterpart follows it, and ADR-0029 drops the sentences that recorded the
// old wording. It takes the base and head commits and reads every document at
// those commits, never the working tree; only its data file comes from the
// working tree, so a temporary commit without the data can be checked.
//
// Run it from the repository root:
//
//	go build -tags test -o /tmp/0069check ./docs/tasks/0069_readme_display_language/_checks/
//	/tmp/0069check [<base> <head>]
//
// The commits default to the merge-base of origin/main and HEAD, and HEAD. It
// prints both commit IDs and the result of each acceptance criterion, and it
// exits 0 when every check passes, 1 on a violation, and 2 when it cannot read
// its input. make test and CI do not run it; the directory starts with an
// underscore, which keeps it out of go list.
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"slices"
	"sort"
	"strings"

	"github.com/isseis/yt2column/internal/doctestutil"
)

const dataPath = "docs/tasks/0069_readme_display_language/_checks/data.json"

// data holds every Japanese string the checks compare, together with the paths
// and the merge commit they read. The Go source keeps its own strings in
// English.
type data struct {
	Readme      paragraphData `json:"readme"`
	ReadmeEN    paragraphData `json:"readme_en"`
	ADR         adrData       `json:"adr"`
	MergeCommit string        `json:"merge_commit"`
	TaskDir     string        `json:"task_dir"`
}

// paragraphData is one document's display-language paragraph, located by the
// beginnings of the unchanged paragraphs around it.
type paragraphData struct {
	Path   string `json:"path"`
	Before string `json:"before"`
	After  string `json:"after"`
}

// adrData is ADR-0029's path, the directory it sits in, the heading of the
// decision section, and the correction line's shape.
type adrData struct {
	Path             string `json:"path"`
	Dir              string `json:"dir"`
	Section          string `json:"section"`
	CorrectionPrefix string `json:"correction_prefix"`
	CorrectionSuffix string `json:"correction_suffix"`
}

// violation is one failed check at a line of a file.
type violation struct {
	name string
	file string
	line int
	text string
}

func main() {
	ok, err := run(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, "0069check:", err)
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

	checks := []struct {
		name string
		desc string
		run  func() ([]violation, error)
	}{
		{"readme", d.Readme.Path + ": text outside the display-language paragraph is unchanged, the paragraph is one block, and every changed file is in scope", func() ([]violation, error) {
			return checkReadme(base, head, d)
		}},
		{"readme-en", d.ReadmeEN.Path + ": only the sync marker and the display-language paragraph changed, and the paragraph is one block", func() ([]violation, error) {
			return checkReadmeEN(base, head, d.ReadmeEN)
		}},
		{"adr", d.ADR.Path + ": the decision section's last item is its first sentence and nothing else in the ADR changed", func() ([]violation, error) {
			return checkADR(base, head, d)
		}},
		{"corrections", "each non-merge commit that changes ADR-0029 carries exactly one editorial correction line", func() ([]violation, error) {
			return checkCorrections(base, head, d)
		}},
		{"merge-order", d.MergeCommit + " is an ancestor of the head commit", func() ([]violation, error) {
			return checkMergeCommit(head, d)
		}},
	}

	var violations []violation
	for _, check := range checks {
		found, err := check.run()
		if err != nil {
			return false, err
		}
		if len(found) == 0 {
			fmt.Printf("OK   %s: %s\n", check.name, check.desc)
			continue
		}
		violations = append(violations, found...)
	}
	if len(violations) > 0 {
		for _, one := range violations {
			fmt.Printf("FAIL %s %s:%d: %s\n", one.name, one.file, one.line, one.text)
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
		return "", "", errors.New("usage: 0069check [<base> <head>]")
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

// checkReadme verifies that README.md changes only inside the
// display-language paragraph, the head paragraph is one block, and every file
// the range changes is README.md, README.en.md, ADR-0029, or under the task
// directory.
func checkReadme(base, head string, d data) ([]violation, error) {
	var out []violation
	changed, err := changedFiles(base, head)
	if err != nil {
		return nil, err
	}
	allowed := []string{d.Readme.Path, d.ReadmeEN.Path, d.ADR.Path}
	for _, path := range sortedKeys(changed) {
		if slices.Contains(allowed, path) || strings.HasPrefix(path, d.TaskDir) {
			continue
		}
		out = append(out, violation{"readme", path, 1, "a file outside the allowed scope changed"})
	}

	baseP, err := readParagraph(d.Readme, base)
	if err != nil {
		return nil, err
	}
	headP, err := readParagraph(d.Readme, head)
	if err != nil {
		return nil, err
	}
	if !singleParagraph(headP) {
		out = append(out, violation{"readme", d.Readme.Path, headP.start + 1, "the display-language paragraph is not one block"})
	}
	if maskParagraph(baseP) != maskParagraph(headP) {
		out = append(out, violation{"readme", d.Readme.Path, 1, "text outside the display-language paragraph changed"})
	}
	return out, nil
}

// checkReadmeEN verifies that README.en.md changes only in its first line
// (the sync marker) and its display-language paragraph, and the head paragraph
// is one block.
func checkReadmeEN(base, head string, p paragraphData) ([]violation, error) {
	baseP, err := readParagraph(p, base)
	if err != nil {
		return nil, err
	}
	headP, err := readParagraph(p, head)
	if err != nil {
		return nil, err
	}
	// The first line is the sync marker, which the translation updates.
	baseP.lines[0] = "\x00marker\x00"
	headP.lines[0] = "\x00marker\x00"
	var out []violation
	if !singleParagraph(headP) {
		out = append(out, violation{"readme-en", p.Path, headP.start + 1, "the display-language paragraph is not one block"})
	}
	if maskParagraph(baseP) != maskParagraph(headP) {
		out = append(out, violation{"readme-en", p.Path, 1, "text outside the sync marker and the display-language paragraph changed"})
	}
	return out, nil
}

// located is a document's lines and the line indexes of its paragraph.
type located struct {
	lines []string
	start int
	end   int
}

// readParagraph reads one document at one commit and locates its paragraph.
func readParagraph(p paragraphData, commit string) (located, error) {
	text, err := read(commit, p.Path)
	if err != nil {
		return located{}, err
	}
	lines := strings.Split(text, "\n")
	start, end, err := paragraphRange(lines, p.Before, p.After)
	if err != nil {
		return located{}, fmt.Errorf("%s at %s: %w", p.Path, commit, err)
	}
	return located{lines: lines, start: start, end: end}, nil
}

// singleParagraph reports whether the located block has no blank line inside.
func singleParagraph(l located) bool {
	for i := l.start; i <= l.end; i++ {
		if strings.TrimSpace(l.lines[i]) == "" {
			return false
		}
	}
	return true
}

// paragraphRange returns the line indexes of the text between two marker
// paragraphs. The paragraph itself is not a marker: its wording changes, while
// the paragraphs around it stay the same. A blank line inside the returned
// range is left for the caller to report (singleParagraph); a missing or
// repeated marker, markers out of order, or no separating blank line is a read
// failure.
func paragraphRange(lines []string, before, after string) (int, int, error) {
	iBefore, err := uniqueLine(lines, before)
	if err != nil {
		return 0, 0, fmt.Errorf("before marker: %w", err)
	}
	iAfter, err := uniqueLine(lines, after)
	if err != nil {
		return 0, 0, fmt.Errorf("after marker: %w", err)
	}
	if iAfter <= iBefore {
		return 0, 0, errors.New("the markers are out of order")
	}
	start := iBefore + 1
	for start < iAfter && strings.TrimSpace(lines[start]) != "" {
		start++
	}
	if start == iAfter {
		return 0, 0, errors.New("the before marker has no separating blank line")
	}
	for start < iAfter && strings.TrimSpace(lines[start]) == "" {
		start++
	}
	end := iAfter - 1
	for end > start && strings.TrimSpace(lines[end]) == "" {
		end--
	}
	if end >= iAfter-1 {
		return 0, 0, errors.New("the paragraph has no separating blank line before the after marker")
	}
	if start > end {
		return 0, 0, errors.New("the paragraph is empty")
	}
	return start, end, nil
}

// maskParagraph joins lines with the paragraph replaced by a marker, so the
// text around it can be compared byte for byte.
func maskParagraph(l located) string {
	masked := make([]string, 0, len(l.lines)-l.end+l.start)
	masked = append(masked, l.lines[:l.start]...)
	masked = append(masked, "\x00paragraph\x00")
	masked = append(masked, l.lines[l.end+1:]...)
	return strings.Join(masked, "\n")
}

// uniqueLine returns the index of the only line starting with prefix.
func uniqueLine(lines []string, prefix string) (int, error) {
	found := -1
	for i, line := range lines {
		if !strings.HasPrefix(line, prefix) {
			continue
		}
		if found >= 0 {
			return 0, fmt.Errorf("%q appears more than once", prefix)
		}
		found = i
	}
	if found < 0 {
		return 0, fmt.Errorf("%q not found", prefix)
	}
	return found, nil
}

// checkADR verifies that under the ADR directory only ADR-0029
// changed, with no addition or deletion, and the decision section's last item
// at the head is the item at the base cut at the end of its first sentence,
// which the closing bold marker delimits. Every other byte of the ADR is the
// same.
func checkADR(base, head string, d data) ([]violation, error) {
	changed, err := changedFiles(base, head)
	if err != nil {
		return nil, err
	}
	var out []violation
	var inDir []string
	for path := range changed {
		if strings.HasPrefix(path, d.ADR.Dir) {
			inDir = append(inDir, path)
		}
	}
	sort.Strings(inDir)
	if len(inDir) != 1 || inDir[0] != d.ADR.Path {
		out = append(out, violation{"adr", d.ADR.Dir, 1, fmt.Sprintf("changed files under the ADR directory are %v, want only %s", inDir, d.ADR.Path)})
	}
	if status := changed[d.ADR.Path]; status == 'A' || status == 'D' {
		out = append(out, violation{"adr", d.ADR.Path, 1, "ADR-0029 was added or deleted"})
	}

	baseText, err := read(base, d.ADR.Path)
	if err != nil {
		return nil, err
	}
	headText, err := read(head, d.ADR.Path)
	if err != nil {
		return nil, err
	}
	baseLines := strings.Split(baseText, "\n")
	headLines := strings.Split(headText, "\n")
	baseItem, err := adrSectionItem(baseLines, d.ADR.Section)
	if err != nil {
		return nil, fmt.Errorf("%s at %s: %w", d.ADR.Path, base, err)
	}
	headItem, err := adrSectionItem(headLines, d.ADR.Section)
	if err != nil {
		return nil, fmt.Errorf("%s at %s: %w", d.ADR.Path, head, err)
	}
	want, err := firstSentence(baseItem.text)
	if err != nil {
		return nil, fmt.Errorf("%s at %s: %w", d.ADR.Path, base, err)
	}
	if headItem.text != want {
		out = append(out, violation{"adr", d.ADR.Path, headItem.line, fmt.Sprintf("the last item is %q, want the first sentence %q", headItem.text, want)})
	}
	if maskLine(baseLines, baseItem.index) != maskLine(headLines, headItem.index) {
		out = append(out, violation{"adr", d.ADR.Path, 1, "a part of ADR-0029 outside the last item of the decision section changed"})
	}
	return out, nil
}

// item is one list item line: its index and its text.
type item struct {
	index int
	line  int
	text  string
}

// adrSectionItem returns the last list item of the section that starts at
// heading. The section ends at the next heading of the same or a higher level.
func adrSectionItem(lines []string, heading string) (item, error) {
	iHeading, err := uniqueLine(lines, heading)
	if err != nil {
		return item{}, fmt.Errorf("section heading: %w", err)
	}
	level := doctestutil.HeadingLevel(lines[iHeading])
	end := len(lines)
	for i := iHeading + 1; i < len(lines); i++ {
		if l := doctestutil.HeadingLevel(lines[i]); l > 0 && l <= level {
			end = i
			break
		}
	}
	found := -1
	for i := iHeading + 1; i < end; i++ {
		if strings.HasPrefix(lines[i], "- ") {
			found = i
		}
	}
	if found < 0 {
		return item{}, fmt.Errorf("the section %q has no list item", heading)
	}
	return item{index: found, line: found + 1, text: lines[found]}, nil
}

// firstSentence returns s up to and including the closing marker of its first
// bold span.
func firstSentence(s string) (string, error) {
	open := strings.Index(s, "**")
	if open < 0 {
		return "", errors.New("the item has no bold span")
	}
	rest := s[open+2:]
	closing := strings.Index(rest, "**")
	if closing < 0 {
		return "", errors.New("the item's bold span is not closed")
	}
	return s[:open+2+closing+2], nil
}

// maskLine joins lines with the line at index replaced by a marker.
func maskLine(lines []string, index int) string {
	masked := slices.Clone(lines)
	masked[index] = "\x00item\x00"
	return strings.Join(masked, "\n")
}

// checkCorrections verifies that at least one non-merge commit in the range
// changes ADR-0029, each such commit changes nothing else, and each carries
// exactly one editorial correction line in its message.
func checkCorrections(base, head string, d data) ([]violation, error) {
	commits, err := gitLines("rev-list", "--no-merges", base+".."+head)
	if err != nil {
		return nil, err
	}
	var out []violation
	adrCommits := 0
	for _, commit := range commits {
		paths, err := diffPaths(commit)
		if err != nil {
			return nil, err
		}
		if !slices.Contains(paths, d.ADR.Path) {
			continue
		}
		adrCommits++
		if len(paths) != 1 {
			out = append(out, violation{"corrections", commit, 1, fmt.Sprintf("the commit that changes ADR-0029 also changes %v", paths)})
		}
		message, err := git("log", "-1", "--format=%B", commit)
		if err != nil {
			return nil, err
		}
		lines := 0
		for _, line := range strings.Split(message, "\n") {
			if isCorrectionLine(line, d.ADR) {
				lines++
			}
		}
		if lines != 1 {
			out = append(out, violation{"corrections", commit, 1, fmt.Sprintf("the commit carries %d editorial correction lines, want 1", lines)})
		}
	}
	if adrCommits == 0 {
		out = append(out, violation{"corrections", d.ADR.Path, 1, "no non-merge commit in the range changes ADR-0029"})
	}
	return out, nil
}

// isCorrectionLine reports whether line is an editorial correction line for
// ADR-0029 with a non-empty description, without the mention and code-span
// characters that must not appear.
func isCorrectionLine(line string, a adrData) bool {
	if !strings.HasPrefix(line, a.CorrectionPrefix) || !strings.HasSuffix(line, a.CorrectionSuffix) {
		return false
	}
	middle := strings.TrimSuffix(strings.TrimPrefix(line, a.CorrectionPrefix), a.CorrectionSuffix)
	if strings.TrimSpace(middle) == "" || strings.ContainsAny(line, "@<") {
		return false
	}
	return true
}

// checkMergeCommit verifies that the pull request that removes the sentences
// from ADR-0029 is merged only after the earlier PR's merge commit.
func checkMergeCommit(head string, d data) ([]violation, error) {
	ok, err := isAncestor(d.MergeCommit, head)
	if err != nil {
		return nil, err
	}
	if !ok {
		return []violation{{"merge-order", d.MergeCommit, 1, "the merge commit is not an ancestor of the head commit"}}, nil
	}
	return nil, nil
}

// changedFiles returns the status letter of each path the range changed,
// without rename detection or the user's diff settings.
func changedFiles(base, head string) (map[string]byte, error) {
	out, err := git("diff", "--no-renames", "--no-relative", "--no-ext-diff", "--name-status", base, head)
	if err != nil {
		return nil, err
	}
	changed := map[string]byte{}
	for _, line := range strings.Split(out, "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("unexpected name-status line %q", line)
		}
		changed[parts[1]] = parts[0][0]
	}
	return changed, nil
}

// diffPaths returns the paths a commit changed against its first parent.
func diffPaths(commit string) ([]string, error) {
	return gitLines("diff-tree", "--no-commit-id", "--name-only", "-r", "--no-renames", commit)
}

// sortedKeys returns the keys of m in ascending order.
func sortedKeys(m map[string]byte) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func read(commit, path string) (string, error) {
	return git("show", commit+":"+path)
}

// isAncestor reports whether anc is an ancestor of desc.
func isAncestor(anc, desc string) (bool, error) {
	cmd := exec.Command("git", "merge-base", "--is-ancestor", anc, desc)
	err := cmd.Run()
	if err == nil {
		return true, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
		return false, nil
	}
	return false, fmt.Errorf("git merge-base --is-ancestor %s %s: %w", anc, desc, err)
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

func gitLines(args ...string) ([]string, error) {
	out, err := git(args...)
	if err != nil {
		return nil, err
	}
	return strings.Split(strings.TrimSuffix(out, "\n"), "\n"), nil
}
