//go:build test

// Command _checks runs the one-off checks for task 0078: ADR-0024's last
// consequence item now links to ADR-0018 and states the fact as it is today,
// only that item and one new history entry differ from the previous commit, and
// nothing outside ADR-0024 and the task directory changed. It takes the base
// and head commits as arguments and reads the ADR at those commits, never from
// the working tree; only its data file, and the blob IDs it prints for itself,
// come from the working tree, so a temporary commit without them can be
// checked.
//
// Run it from the repository root:
//
//	go build -tags test -o /tmp/0078check ./docs/tasks/0078_adr0024_server_env_result/_checks/
//	/tmp/0078check <base> <head>
//
// It prints both commit IDs, the blob IDs of main.go and data.json, and the
// result of each check. It exits 0 when every check passes, 1 on a violation,
// and 2 when it cannot read its input. make test and CI do not run it; the
// directory starts with an underscore, which keeps it out of go list.
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
	"time"

	"github.com/isseis/yt2column/internal/doctestutil"
)

const (
	checkDir   = "docs/tasks/0078_adr0024_server_env_result/_checks/"
	scriptPath = checkDir + "main.go"
	dataPath   = checkDir + "data.json"

	// itemMarker replaces the consequence items the comparison leaves out.
	itemMarker = "<the last consequence item>"
)

// data holds every Japanese string the checks compare, together with the paths
// and the link forms they look for. The Go source keeps its own strings in
// English.
type data struct {
	ADR     adrData `json:"adr"`
	TaskDir string  `json:"task_dir"`
}

// adrData names the ADR, its two sections, the markers of their list items, and
// the links the checks look for.
type adrData struct {
	Path                  string `json:"path"`
	ConsequenceHeading    string `json:"consequence_heading"`
	ConsequenceItemPrefix string `json:"consequence_item_prefix"`
	HistoryHeading        string `json:"history_heading"`
	HistoryItemPrefix     string `json:"history_item_prefix"`
	SourceLink            string `json:"source_link"`
	HistoryLinkOpen       string `json:"history_link_open"`
	HistoryLinkMiddle     string `json:"history_link_middle"`
	HistoryLinkClose      string `json:"history_link_close"`
	HistoryLink           string `json:"history_link"`
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
		fmt.Fprintln(os.Stderr, "0078check:", err)
		os.Exit(2)
	}
	if !ok {
		os.Exit(1)
	}
}

func run(args []string) (bool, error) {
	baseArg, headArg, err := parseArgs(args)
	if err != nil {
		return false, err
	}
	base, err := resolve(baseArg)
	if err != nil {
		return false, err
	}
	head, err := resolve(headArg)
	if err != nil {
		return false, err
	}
	fmt.Printf("base %s\nhead %s\n", base, head)
	for _, path := range []string{scriptPath, dataPath} {
		id, err := blobID(path)
		if err != nil {
			return false, err
		}
		fmt.Printf("%s blob %s\n", path, id)
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

	checks := []struct {
		name string
		desc string
		run  func() ([]violation, error)
	}{
		{"consequence-link", "the last consequence item changed and links to " + d.ADR.SourceLink, func() ([]violation, error) {
			return checkConsequenceLink(base, head, d)
		}},
		{"consequence", "the two versions differ only in the last consequence item and the added history entry", func() ([]violation, error) {
			return checkConsequence(base, head, d)
		}},
		{"scope", "only " + d.ADR.Path + " changed outside the task directory, and no file was added or deleted outside it", func() ([]violation, error) {
			return checkScope(base, head, d)
		}},
		{"history-items", "the history section is the earlier items unchanged, followed by exactly one new item", func() ([]violation, error) {
			return checkHistoryItems(base, head, d)
		}},
		{"history-entry", "the new history item has the history entry's form and links to " + d.ADR.HistoryLink, func() ([]violation, error) {
			return checkHistoryEntry(base, head, d)
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

func parseArgs(args []string) (string, string, error) {
	if len(args) != 2 {
		return "", "", errors.New("usage: 0078check <base> <head>")
	}
	return args[0], args[1], nil
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

// located is one commit's version of the ADR: its lines and the indexes of the
// list items of its consequence and history sections.
type located struct {
	lines       []string
	consequence []int
	history     []int
}

// lastConsequence returns the index of the last consequence item.
func (l located) lastConsequence() (int, bool) {
	if len(l.consequence) == 0 {
		return 0, false
	}
	return l.consequence[len(l.consequence)-1], true
}

// items returns the text of the list items at indexes.
func (l located) items(indexes []int) []string {
	out := make([]string, len(indexes))
	for i, index := range indexes {
		out[i] = l.lines[index]
	}
	return out
}

// locate reads the ADR at commit and finds the list items of its consequence
// and history sections.
func locate(commit string, d data) (located, error) {
	doc, err := read(commit, d.ADR.Path)
	if err != nil {
		return located{}, err
	}
	scanned := doctestutil.ScanDocLines(slices.Collect(strings.Lines(doc)))
	consequence, err := sectionItems(scanned, d.ADR.ConsequenceHeading, d.ADR.ConsequenceItemPrefix)
	if err != nil {
		return located{}, fmt.Errorf("%s at %s: %w", d.ADR.Path, commit, err)
	}
	history, err := sectionItems(scanned, d.ADR.HistoryHeading, d.ADR.HistoryItemPrefix)
	if err != nil {
		return located{}, fmt.Errorf("%s at %s: %w", d.ADR.Path, commit, err)
	}
	lines := make([]string, len(scanned))
	for i, line := range scanned {
		lines[i] = line.Text
	}
	return located{lines: lines, consequence: consequence, history: history}, nil
}

// sectionItems returns the indexes of the list items in the named section, in
// order: every line that starts with prefix after the heading and before the
// next heading of the same or a higher level, outside fenced code blocks.
func sectionItems(scanned []doctestutil.DocLine, heading, prefix string) ([]int, error) {
	want := strings.TrimSpace(heading)
	start, level := -1, 0
	for i, line := range scanned {
		if line.Block || strings.TrimSpace(line.Text) != want {
			continue
		}
		start, level = i, doctestutil.HeadingLevel(line.Text)
		break
	}
	if start < 0 {
		return nil, fmt.Errorf("the heading %q is missing", heading)
	}
	var out []int
	for i := start + 1; i < len(scanned); i++ {
		line := scanned[i]
		if line.Block {
			continue
		}
		if l := doctestutil.HeadingLevel(line.Text); l > 0 && l <= level {
			break
		}
		if strings.HasPrefix(line.Text, prefix) {
			out = append(out, i)
		}
	}
	return out, nil
}

// checkConsequenceLink verifies that the head's last consequence item changed
// and links to ADR-0018.
func checkConsequenceLink(base, head string, d data) ([]violation, error) {
	baseDoc, err := locate(base, d)
	if err != nil {
		return nil, err
	}
	headDoc, err := locate(head, d)
	if err != nil {
		return nil, err
	}
	index, ok := headDoc.lastConsequence()
	if !ok {
		return []violation{{"consequence-link", d.ADR.Path, 1, "the consequence section has no list item"}}, nil
	}
	item := headDoc.lines[index]
	var out []violation
	if !strings.Contains(item, "]("+d.ADR.SourceLink+")") {
		out = append(out, violation{"consequence-link", d.ADR.Path, index + 1, "the last consequence item does not link to " + d.ADR.SourceLink})
	}
	if baseIndex, ok := baseDoc.lastConsequence(); ok && baseDoc.lines[baseIndex] == item {
		out = append(out, violation{"consequence-link", d.ADR.Path, index + 1, "the last consequence item did not change"})
	}
	return out, nil
}

// checkConsequence verifies that the two versions differ only in the last
// consequence item and the history entries the head added after the base's last
// one, and that the consequence section keeps its number of items.
func checkConsequence(base, head string, d data) ([]violation, error) {
	baseDoc, err := locate(base, d)
	if err != nil {
		return nil, err
	}
	headDoc, err := locate(head, d)
	if err != nil {
		return nil, err
	}
	baseIndex, baseHas := baseDoc.lastConsequence()
	headIndex, headHas := headDoc.lastConsequence()
	if !baseHas || !headHas {
		return []violation{{"consequence", d.ADR.Path, 1, "a version has no consequence list item"}}, nil
	}
	var out []violation
	if len(baseDoc.consequence) != len(headDoc.consequence) {
		out = append(out, violation{"consequence", d.ADR.Path, 1, fmt.Sprintf("the consequence section has %d items, want %d", len(headDoc.consequence), len(baseDoc.consequence))})
	}
	baseLines := slices.Clone(baseDoc.lines)
	baseLines[baseIndex] = itemMarker
	headLines := slices.Clone(headDoc.lines)
	headLines[headIndex] = itemMarker
	skip := addedHistory(baseDoc, headDoc)
	kept := make([]string, 0, len(headLines))
	for i, line := range headLines {
		if _, ok := skip[i]; !ok {
			kept = append(kept, line)
		}
	}
	if i := firstDifference(baseLines, kept); i >= 0 {
		want := "the end of the file"
		if i < len(baseLines) {
			want = fmt.Sprintf("the base commit's %q", baseLines[i])
		}
		got := "the end of the file"
		if i < len(kept) {
			got = fmt.Sprintf("%q", kept[i])
		}
		out = append(out, violation{"consequence", d.ADR.Path, i + 1, fmt.Sprintf("the line is %s, want %s", got, want)})
	}
	return out, nil
}

// addedHistory returns the line indexes of the history items the head added
// after the base's last one, when the two item lists start with the same items.
// When they do not, it returns none and the comparison reports the change.
func addedHistory(base, head located) map[int]struct{} {
	if len(head.history) <= len(base.history) {
		return nil
	}
	skip := map[int]struct{}{}
	for i, index := range head.history {
		if i < len(base.history) {
			if head.lines[index] != base.lines[base.history[i]] {
				return nil
			}
			continue
		}
		skip[index] = struct{}{}
	}
	return skip
}

// firstDifference returns the index of the first line that differs, or -1 when
// the two slices hold the same lines.
func firstDifference(want, got []string) int {
	for i := 0; i < len(want) && i < len(got); i++ {
		if want[i] != got[i] {
			return i
		}
	}
	if len(want) != len(got) {
		return min(len(want), len(got))
	}
	return -1
}

// checkScope verifies that the only file the range changed outside the task
// directory is ADR-0024, as a modification, and that no file was deleted.
func checkScope(base, head string, d data) ([]violation, error) {
	changed, err := changedFiles(base, head)
	if err != nil {
		return nil, err
	}
	var out []violation
	if status, ok := changed[d.ADR.Path]; !ok {
		out = append(out, violation{"scope", d.ADR.Path, 1, "the ADR did not change"})
	} else if status != 'M' {
		out = append(out, violation{"scope", d.ADR.Path, 1, fmt.Sprintf("the ADR's file status is %c, want a modification", status)})
	}
	paths := make([]string, 0, len(changed))
	for path := range changed {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, path := range paths {
		if path == d.ADR.Path {
			continue
		}
		if strings.HasPrefix(path, d.TaskDir) {
			continue
		}
		if changed[path] == 'D' {
			out = append(out, violation{"scope", path, 1, "the file was deleted"})
			continue
		}
		out = append(out, violation{"scope", path, 1, "a file outside the ADR and the task directory changed"})
	}
	return out, nil
}

// checkHistoryItems verifies that the history section is the base's items
// unchanged and in order, followed by exactly one new item.
func checkHistoryItems(base, head string, d data) ([]violation, error) {
	baseDoc, err := locate(base, d)
	if err != nil {
		return nil, err
	}
	headDoc, err := locate(head, d)
	if err != nil {
		return nil, err
	}
	baseItems := baseDoc.items(baseDoc.history)
	headItems := headDoc.items(headDoc.history)
	var out []violation
	if len(headItems) != len(baseItems)+1 {
		out = append(out, violation{"history-items", d.ADR.Path, 1, fmt.Sprintf("the history section has %d items, want %d", len(headItems), len(baseItems)+1)})
	}
	for i := 0; i < len(baseItems) && i < len(headItems); i++ {
		if baseItems[i] != headItems[i] {
			out = append(out, violation{"history-items", d.ADR.Path, headDoc.history[i] + 1, "an earlier history item changed or is out of order"})
		}
	}
	return out, nil
}

// checkHistoryEntry verifies the form of the new history item: a date, a gist,
// and a link to the requirements document.
func checkHistoryEntry(base, head string, d data) ([]violation, error) {
	baseDoc, err := locate(base, d)
	if err != nil {
		return nil, err
	}
	headDoc, err := locate(head, d)
	if err != nil {
		return nil, err
	}
	if len(headDoc.history) <= len(baseDoc.history) {
		return []violation{{"history-entry", d.ADR.Path, 1, "the history section has no new item"}}, nil
	}
	index := headDoc.history[len(headDoc.history)-1]
	if problem := historyEntryProblem(headDoc.lines[index], d); problem != "" {
		return []violation{{"history-entry", d.ADR.Path, index + 1, problem}}, nil
	}
	return nil, nil
}

// historyEntryProblem reports what is wrong with a history item's form, or ""
// when it has the form: a list item, a date, a gist, and a link to the
// requirements document.
func historyEntryProblem(item string, d data) string {
	prefix := d.ADR.HistoryItemPrefix
	if !strings.HasPrefix(item, prefix) {
		return "it is not a list item"
	}
	rest := strings.TrimPrefix(item, prefix)
	linkAt := strings.LastIndex(rest, d.ADR.HistoryLinkMiddle)
	if linkAt < 0 {
		return "it has no link"
	}
	if rest[linkAt+len(d.ADR.HistoryLinkMiddle):] != d.ADR.HistoryLink+d.ADR.HistoryLinkClose {
		return "it does not end with a link to " + d.ADR.HistoryLink
	}
	before := rest[:linkAt]
	openAt := strings.LastIndex(before, d.ADR.HistoryLinkOpen)
	if openAt < 0 {
		return "its link has the wrong form"
	}
	if before[openAt+len(d.ADR.HistoryLinkOpen):] == "" {
		return "its link has no label"
	}
	date, gist, ok := strings.Cut(before[:openAt], " ")
	if !ok || gist == "" {
		return "it has no gist after the date"
	}
	if _, err := time.Parse("2006-01-02", date); err != nil {
		return "its date is not YYYY-MM-DD"
	}
	return ""
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

func read(commit, path string) (string, error) {
	return git("show", commit+":"+path)
}

// blobID returns the blob ID of the working tree file at path, so the record
// names the content that ran.
func blobID(path string) (string, error) {
	out, err := git("hash-object", "--", path)
	return strings.TrimSpace(out), err
}

func resolve(rev string) (string, error) {
	out, err := git("rev-parse", "--verify", strings.TrimSpace(rev)+"^{commit}")
	return strings.TrimSpace(out), err
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
