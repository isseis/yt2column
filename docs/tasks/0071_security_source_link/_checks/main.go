//go:build test

// Command _checks runs the one-off checks for task 0071: the security guide's
// source-link bullet now states the guarantee for every article output,
// ADR-0034 drops the sentences that recorded the old wording, and that ADR
// change was made as an editorial correction. It takes the base and head
// commits and reads every document at those commits, never the working tree;
// only its data file comes from the working tree, so a temporary commit
// without the data can be checked. With -created it also compares the ADR
// change the branch made at the recorded commit with the ADR change it makes
// at the head.
//
// Run it from the repository root:
//
//	go build -tags test -o /tmp/0071check ./docs/tasks/0071_security_source_link/_checks/
//	/tmp/0071check [-created <commit>] [<base> <head>]
//
// The commits default to the merge-base of origin/main and HEAD, and HEAD. It
// prints both commit IDs and the result of each check, and it exits 0 when
// every check passes, 1 on a violation, and 2 when it cannot read its input.
// Without -created it reports the created-diff check as not run. make test and
// CI do not run it; the directory starts with an underscore, which keeps it
// out of go list.
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

const dataPath = "docs/tasks/0071_security_source_link/_checks/data.json"

// data holds every Japanese string the checks compare, together with the paths
// and the merge commit they read. The Go source keeps its own strings in
// English.
type data struct {
	Security    itemData `json:"security"`
	ADR         adrData  `json:"adr"`
	IssueLink   string   `json:"issue_link"`
	MergeCommit string   `json:"merge_commit"`
	TaskDir     string   `json:"task_dir"`
}

// itemData is one document's single list item, located by the beginnings of
// the unchanged list items around it.
type itemData struct {
	Path    string `json:"path"`
	Section string `json:"section"`
	Before  string `json:"before"`
	After   string `json:"after"`
	Prefix  string `json:"prefix"`
}

// adrData is the ADR's path, the directory it sits in, the heading of its
// decision section, the beginning of its decision section's last item, and the
// number of sentences that item keeps.
type adrData struct {
	Path        string `json:"path"`
	Dir         string `json:"dir"`
	Section     string `json:"section"`
	ItemStart   string `json:"item_start"`
	ItemPrefix  string `json:"item_prefix"`
	Sentences   int    `json:"sentences"`
	SentenceEnd string `json:"sentence_end"`
}

// violation is one failed check at a line of a file.
type violation struct {
	name string
	file string
	line int
	text string
}

// options is the parsed command line: the optional created commit and the two
// commits to compare.
type options struct {
	created string
	base    string
	head    string
}

func main() {
	ok, err := run(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, "0071check:", err)
		os.Exit(2)
	}
	if !ok {
		os.Exit(1)
	}
}

func run(args []string) (bool, error) {
	opts, err := parseArgs(args)
	if err != nil {
		return false, err
	}
	baseRev := opts.base
	if baseRev == "" {
		out, err := git("merge-base", "origin/main", "HEAD")
		if err != nil {
			return false, err
		}
		baseRev = strings.TrimSpace(out)
	}
	base, err := resolve(baseRev)
	if err != nil {
		return false, err
	}
	head, err := resolve(opts.head)
	if err != nil {
		return false, err
	}
	fmt.Printf("base %s\nhead %s\n", base, head)
	created := ""
	if opts.created != "" {
		created, err = resolve(opts.created)
		if err != nil {
			return false, err
		}
		fmt.Printf("created %s\n", created)
	}

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

	// A nil run marks a check the options leave out.
	checks := []struct {
		name string
		desc string
		run  func() ([]violation, error)
	}{
		{"source-link-item", d.Security.Path + ": only the source-link item changed, and it is one list item between its neighbours in the section", func() ([]violation, error) {
			return checkItem(base, head, d)
		}},
		{"adr-item", d.ADR.Path + ": the decision section's last item is its first sentences and nothing else in the ADR changed", func() ([]violation, error) {
			return checkADRItem(base, head, d)
		}},
		{"adr-scope", "only ADRs that link the issue at the base commit changed, each change deletes characters only, and no file was added, deleted, or renamed", func() ([]violation, error) {
			return checkADRScope(base, head, d)
		}},
		{"corrections", "each changed ADR has an editorial correction line in a commit on the range", func() ([]violation, error) {
			return checkCorrections(base, head, d)
		}},
		{"created-diff", "the ADR change from the commit given by -created to the head is the same", nil},
		{"merge-order", d.MergeCommit + " is an ancestor of the head commit", func() ([]violation, error) {
			return checkMergeCommit(head, d)
		}},
		{"scope", "every changed file is the security guide, an ADR, or under the task directory", func() ([]violation, error) {
			return checkScope(base, head, d)
		}},
	}
	if created != "" {
		checks[4].run = func() ([]violation, error) {
			return checkCreatedDiff(created, head, d)
		}
	}

	var violations []violation
	notRun := false
	for _, check := range checks {
		if check.run == nil {
			fmt.Printf("NOT RUN %s: %s\n", check.name, check.desc)
			notRun = true
			continue
		}
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
	if notRun {
		fmt.Println("OK every run check passed; created-diff was not run")
	} else {
		fmt.Println("OK every check passed")
	}
	return true, nil
}

func parseArgs(args []string) (options, error) {
	var opts options
	var positional []string
	for i := 0; i < len(args); i++ {
		if args[i] == "-created" {
			if i+1 >= len(args) {
				return options{}, errors.New("-created needs a commit")
			}
			opts.created = args[i+1]
			i++
			continue
		}
		if strings.HasPrefix(args[i], "-") && args[i] != "-" {
			return options{}, fmt.Errorf("unknown option %q", args[i])
		}
		positional = append(positional, args[i])
	}
	switch len(positional) {
	case 0:
		opts.head = "HEAD"
	case 2:
		opts.base, opts.head = positional[0], positional[1]
	default:
		return options{}, errors.New("usage: 0071check [-created <commit>] [<base> <head>]")
	}
	return opts, nil
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

// located is a document's lines and the line index of the item it holds.
type located struct {
	lines []string
	index int
}

// text returns the item's line without its line ending.
func (l located) text() string {
	return l.lines[l.index]
}

// line returns the item's one-based line number.
func (l located) line() int {
	return l.index + 1
}

// readItem reads the security guide at one commit and locates its source-link
// item: the only list item between two unchanged list items, inside the named
// section.
func readItem(commit string, spec itemData) (located, error) {
	doc, err := read(commit, spec.Path)
	if err != nil {
		return located{}, err
	}
	lines := strings.Split(doc, "\n")
	iBefore, err := uniqueLine(lines, spec.Before)
	if err != nil {
		return located{}, fmt.Errorf("%s at %s: before marker: %w", spec.Path, commit, err)
	}
	iAfter, err := uniqueLine(lines, spec.After)
	if err != nil {
		return located{}, fmt.Errorf("%s at %s: after marker: %w", spec.Path, commit, err)
	}
	if iAfter <= iBefore {
		return located{}, fmt.Errorf("%s at %s: the markers are out of order", spec.Path, commit)
	}
	if iAfter-iBefore != 2 {
		return located{}, fmt.Errorf("%s at %s: %d line(s) between the markers, want one list item", spec.Path, commit, iAfter-iBefore-1)
	}
	index := iBefore + 1
	if !strings.HasPrefix(lines[index], spec.Prefix) {
		return located{}, fmt.Errorf("%s at %s: the item is not a list item", spec.Path, commit)
	}
	section := doctestutil.SectionOutsideBlocks(doc, spec.Section)
	if !slices.Contains(strings.Split(section, "\n"), lines[index]) {
		return located{}, fmt.Errorf("%s at %s: the item is outside the section %q", spec.Path, commit, spec.Section)
	}
	return located{lines: lines, index: index}, nil
}

// readADRItem reads the ADR at one commit and locates the last list item of
// its decision section, which must be the item that starts with the recorded
// beginning.
func readADRItem(commit string, spec adrData) (located, error) {
	doc, err := read(commit, spec.Path)
	if err != nil {
		return located{}, err
	}
	lines := strings.Split(doc, "\n")
	index, err := uniqueLine(lines, spec.ItemStart)
	if err != nil {
		return located{}, fmt.Errorf("%s at %s: item: %w", spec.Path, commit, err)
	}
	if !strings.HasPrefix(lines[index], spec.ItemPrefix) {
		return located{}, fmt.Errorf("%s at %s: the item is not a list item", spec.Path, commit)
	}
	sectionLines := strings.Split(doctestutil.SectionOutsideBlocks(doc, spec.Section), "\n")
	if !slices.Contains(sectionLines, lines[index]) {
		return located{}, fmt.Errorf("%s at %s: the item is outside the section %q", spec.Path, commit, spec.Section)
	}
	last := ""
	for _, line := range sectionLines {
		if strings.HasPrefix(line, spec.ItemPrefix) {
			last = line
		}
	}
	if last != lines[index] {
		return located{}, fmt.Errorf("%s at %s: the item is not the last list item of the section", spec.Path, commit)
	}
	return located{lines: lines, index: index}, nil
}

// maskItem joins lines with the item replaced by a marker, so the text around
// it can be compared byte for byte.
func maskItem(l located) string {
	masked := slices.Clone(l.lines)
	masked[l.index] = "\x00item\x00"
	return strings.Join(masked, "\n")
}

// checkItem verifies that security.md changes only inside the source-link item
// and the head item is still one list item inside its section.
func checkItem(base, head string, d data) ([]violation, error) {
	baseItem, err := readItem(base, d.Security)
	if err != nil {
		return nil, err
	}
	headItem, err := readItem(head, d.Security)
	if err != nil {
		return nil, err
	}
	if maskItem(baseItem) == maskItem(headItem) {
		return nil, nil
	}
	return []violation{{"source-link-item", d.Security.Path, 1, "text outside the source-link item changed"}}, nil
}

// checkADRItem verifies that the decision section's last item at the head is
// the item at the base cut at the end of the recorded number of sentences, and
// every other byte of the ADR is the same.
func checkADRItem(base, head string, d data) ([]violation, error) {
	baseItem, err := readADRItem(base, d.ADR)
	if err != nil {
		return nil, err
	}
	headItem, err := readADRItem(head, d.ADR)
	if err != nil {
		return nil, err
	}
	want, err := cutAtSentence(baseItem.text(), d.ADR.SentenceEnd, d.ADR.Sentences)
	if err != nil {
		return nil, fmt.Errorf("%s at %s: %w", d.ADR.Path, base, err)
	}
	if headItem.text() != want {
		return []violation{{"adr-item", d.ADR.Path, headItem.line(), fmt.Sprintf("the last item is %q, want %q", headItem.text(), want)}}, nil
	}
	return nil, nil
}

// cutAtSentence returns s up to and including the end of its n-th sentence.
func cutAtSentence(s, end string, n int) (string, error) {
	pos := 0
	for i := 0; i < n; i++ {
		j := strings.Index(s[pos:], end)
		if j < 0 {
			return "", fmt.Errorf("the item has fewer than %d sentences ending in %q", n, end)
		}
		pos += j + len(end)
	}
	return s[:pos], nil
}

// checkADRScope verifies that every changed file under the ADR directory is a
// modification of an ADR that links the issue at the base commit, and that the
// change only deletes characters: every head line is a base line with
// characters removed, and the other base lines are gone.
func checkADRScope(base, head string, d data) ([]violation, error) {
	changed, err := changedFiles(base, head)
	if err != nil {
		return nil, err
	}
	var changedADRs []string
	for path := range changed {
		if strings.HasPrefix(path, d.ADR.Dir) {
			changedADRs = append(changedADRs, path)
		}
	}
	sort.Strings(changedADRs)
	if len(changedADRs) == 0 {
		return []violation{{"adr-scope", d.ADR.Dir, 1, "no ADR changed"}}, nil
	}
	linked, err := adrPathsLinking(base, d)
	if err != nil {
		return nil, err
	}
	var out []violation
	for _, path := range changedADRs {
		if status := changed[path]; status != 'M' {
			out = append(out, violation{"adr-scope", path, 1, fmt.Sprintf("the file status is %c, want a modification", status)})
			continue
		}
		if !slices.Contains(linked, path) {
			out = append(out, violation{"adr-scope", path, 1, "the ADR does not link the issue at the base commit"})
		}
		baseText, err := read(base, path)
		if err != nil {
			return nil, err
		}
		headText, err := read(head, path)
		if err != nil {
			return nil, err
		}
		if err := deletionOnly(baseText, headText); err != nil {
			out = append(out, violation{"adr-scope", path, 1, err.Error()})
		}
	}
	return out, nil
}

// deletionOnly reports whether headText can be made from baseText by deleting
// characters inside lines and deleting whole lines. Each head line is matched
// to the earliest remaining base line that contains it as a subsequence; the
// base lines left over are the deleted ones.
func deletionOnly(baseText, headText string) error {
	baseLines := strings.Split(baseText, "\n")
	headLines := strings.Split(headText, "\n")
	pos := 0
	for _, line := range headLines {
		found := -1
		for i := pos; i < len(baseLines); i++ {
			if isSubsequence(line, baseLines[i]) {
				found = i
				break
			}
		}
		if found < 0 {
			return fmt.Errorf("the head line %q is not a base line with characters removed", line)
		}
		pos = found + 1
	}
	return nil
}

// isSubsequence reports whether s is t with zero or more characters removed.
func isSubsequence(s, t string) bool {
	want := []rune(s)
	i := 0
	for _, r := range t {
		if i < len(want) && want[i] == r {
			i++
		}
	}
	return i == len(want)
}

// adrPathsLinking returns the files under the ADR directory that link the
// issue at the base commit, as the same git grep the design names.
func adrPathsLinking(base string, d data) ([]string, error) {
	out, err := gitNoMatch("grep", "-l", "-E", d.IssueLink+"([^0-9]|$)", base, "--", d.ADR.Dir)
	if err != nil {
		return nil, err
	}
	var paths []string
	for _, line := range strings.Split(strings.TrimSuffix(out, "\n"), "\n") {
		if line == "" {
			continue
		}
		_, path, ok := strings.Cut(line, ":")
		if !ok {
			return nil, fmt.Errorf("unexpected git grep output %q", line)
		}
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return paths, nil
}

// checkCorrections verifies that every changed ADR has an editorial correction
// line carrying its path in a non-merge commit's message on the range.
func checkCorrections(base, head string, d data) ([]violation, error) {
	changed, err := changedFiles(base, head)
	if err != nil {
		return nil, err
	}
	var changedADRs []string
	for path := range changed {
		if strings.HasPrefix(path, d.ADR.Dir) {
			changedADRs = append(changedADRs, path)
		}
	}
	sort.Strings(changedADRs)
	if len(changedADRs) == 0 {
		return nil, nil
	}
	commits, err := gitLines("rev-list", "--no-merges", base+".."+head)
	if err != nil {
		return nil, err
	}
	var out []violation
	for _, path := range changedADRs {
		found := false
		for _, commit := range commits {
			message, err := git("log", "-1", "--format=%B", commit)
			if err != nil {
				return nil, err
			}
			for _, line := range strings.Split(message, "\n") {
				if isCorrectionLine(line, path) {
					found = true
				}
			}
		}
		if !found {
			out = append(out, violation{"corrections", path, 1, "no commit on the range carries an editorial correction line for this ADR"})
		}
	}
	return out, nil
}

const (
	correctionPrefix = "Editorial correction: "
	correctionSuffix = "; no decision changed"
)

// isCorrectionLine reports whether line is an editorial correction line for
// path with a non-empty description, without the mention and code-span
// characters that must not appear.
func isCorrectionLine(line, path string) bool {
	prefix := correctionPrefix + path + ": "
	if !strings.HasPrefix(line, prefix) || !strings.HasSuffix(line, correctionSuffix) {
		return false
	}
	middle := strings.TrimSuffix(strings.TrimPrefix(line, prefix), correctionSuffix)
	if strings.TrimSpace(middle) == "" || strings.ContainsAny(line, "@<") {
		return false
	}
	return true
}

// checkCreatedDiff verifies that the ADR change the branch makes at the
// recorded commit is the same as the ADR change it makes at the head.
func checkCreatedDiff(created, head string, d data) ([]violation, error) {
	ok, err := isAncestor(created, head)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("the -created commit %s is not an ancestor of the head commit %s", created, head)
	}
	createdChanges, err := ownADRChanges(created, d)
	if err != nil {
		return nil, err
	}
	headChanges, err := ownADRChanges(head, d)
	if err != nil {
		return nil, err
	}
	if equalChanges(createdChanges, headChanges) {
		return nil, nil
	}
	text := fmt.Sprintf("the ADR change differs: at created %s, at head %s", describeChanges(createdChanges), describeChanges(headChanges))
	return []violation{{"created-diff", d.ADR.Dir, 1, text}}, nil
}

// fileChange is the content of the lines one or more commits removed from and
// added to one file, in commit and diff order.
type fileChange struct {
	removed []string
	added   []string
}

// ownADRChanges returns the ADR changes the branch itself makes at commit: the
// changes of the non-merge commits on the first-parent chain from the commit
// back to the first commit that is an ancestor of origin/main. A merge commit
// is skipped, so the ADR changes a take-in of main brings are not compared.
func ownADRChanges(commit string, d data) (map[string]fileChange, error) {
	chain, err := firstParentChain(commit)
	if err != nil {
		return nil, err
	}
	changes := map[string]fileChange{}
	for _, one := range chain {
		parents, err := gitLines("rev-list", "--parents", "-n", "1", one)
		if err != nil {
			return nil, err
		}
		if len(parents) != 1 {
			return nil, fmt.Errorf("git rev-list --parents -n 1 %s returned %d lines, want 1", one, len(parents))
		}
		if fields := strings.Fields(parents[0]); len(fields) > 2 {
			continue
		}
		out, err := git("diff", "--no-renames", "--no-ext-diff", "--no-color", "-U0", one+"^", one, "--", d.ADR.Dir)
		if err != nil {
			return nil, err
		}
		parsed, err := parseUnified(out)
		if err != nil {
			return nil, err
		}
		for path, change := range parsed {
			acc := changes[path]
			acc.removed = append(acc.removed, change.removed...)
			acc.added = append(acc.added, change.added...)
			changes[path] = acc
		}
	}
	return changes, nil
}

// firstParentChain returns the commits from the first ancestor of origin/main
// (excluded) to commit (included), following first parents.
func firstParentChain(commit string) ([]string, error) {
	var chain []string
	cur := commit
	for {
		ok, err := isAncestor(cur, "origin/main")
		if err != nil {
			return nil, err
		}
		if ok {
			break
		}
		chain = append(chain, cur)
		out, err := gitLines("rev-list", "--parents", "-n", "1", cur)
		if err != nil {
			return nil, err
		}
		if len(out) != 1 {
			return nil, fmt.Errorf("git rev-list --parents -n 1 %s returned %d lines, want 1", cur, len(out))
		}
		fields := strings.Fields(out[0])
		if len(fields) < 2 {
			break
		}
		cur = fields[1]
	}
	slices.Reverse(chain)
	return chain, nil
}

// parseUnified collects the removed and added line contents of a unified diff
// without context, per file.
func parseUnified(diff string) (map[string]fileChange, error) {
	changes := map[string]fileChange{}
	path := ""
	inBody := false
	for _, line := range strings.Split(diff, "\n") {
		switch {
		case strings.HasPrefix(line, "diff --git "):
			fields := strings.Fields(line)
			if len(fields) != 4 || !strings.HasPrefix(fields[2], "a/") {
				return nil, fmt.Errorf("unexpected diff header %q", line)
			}
			path = strings.TrimPrefix(fields[2], "a/")
			inBody = false
		case !inBody:
			if strings.HasPrefix(line, "@@ ") {
				inBody = true
			}
		case strings.HasPrefix(line, "@@ "):
			continue
		case strings.HasPrefix(line, "+"):
			change := changes[path]
			change.added = append(change.added, line[1:])
			changes[path] = change
		case strings.HasPrefix(line, "-"):
			change := changes[path]
			change.removed = append(change.removed, line[1:])
			changes[path] = change
		}
	}
	return changes, nil
}

// equalChanges reports whether two ADR change maps hold the same content.
func equalChanges(a, b map[string]fileChange) bool {
	if len(a) != len(b) {
		return false
	}
	for path, one := range a {
		other, ok := b[path]
		if !ok || !slices.Equal(one.removed, other.removed) || !slices.Equal(one.added, other.added) {
			return false
		}
	}
	return true
}

// describeChanges names each changed file with its removed and added line
// counts.
func describeChanges(changes map[string]fileChange) string {
	paths := make([]string, 0, len(changes))
	for path := range changes {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	if len(paths) == 0 {
		return "no change"
	}
	parts := make([]string, 0, len(paths))
	for _, path := range paths {
		change := changes[path]
		parts = append(parts, fmt.Sprintf("%s (-%d +%d)", path, len(change.removed), len(change.added)))
	}
	return strings.Join(parts, ", ")
}

// checkMergeCommit verifies that the earlier PR's merge commit is an ancestor
// of the head commit.
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

// checkScope verifies that every file the range changed is the security guide,
// an ADR, or under the task directory.
func checkScope(base, head string, d data) ([]violation, error) {
	changed, err := changedFiles(base, head)
	if err != nil {
		return nil, err
	}
	var out []violation
	for _, path := range sortedKeys(changed) {
		if path == d.Security.Path || strings.HasPrefix(path, d.ADR.Dir) || strings.HasPrefix(path, d.TaskDir) {
			continue
		}
		out = append(out, violation{"scope", path, 1, "a file outside the allowed scope changed"})
	}
	return out, nil
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

// sortedKeys returns the keys of m in ascending order.
func sortedKeys(m map[string]byte) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
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

// gitNoMatch is git for a command that uses exit status 1 to mean no match.
func gitNoMatch(args ...string) (string, error) {
	out, err := exec.Command("git", args...).Output()
	if err == nil {
		return string(out), nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
		return "", nil
	}
	return "", fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
}

// gitLines runs git and splits its output into lines.
func gitLines(args ...string) ([]string, error) {
	out, err := git(args...)
	if err != nil {
		return nil, err
	}
	return strings.Split(strings.TrimSuffix(out, "\n"), "\n"), nil
}
