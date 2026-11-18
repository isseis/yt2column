package numreserve

import (
	"context"
	"fmt"
	"path"
	"regexp"
	"slices"
	"strings"

	"github.com/isseis/yt2column/internal/tasknaming"
)

// The thresholds of the record check: a number a PR adds above its space's
// threshold must have a record tag. Each is the largest number in use on
// origin/main and on every origin branch (branch names included for tasks)
// when the check was introduced, so a PR made before numbers were decided by
// this tool is not stopped for lacking a record. The counting, its commands,
// and their output are recorded in section 7 of
// docs/tasks/0100_number_reservation/03_implementation_plan.md, under the
// threshold record heading.
const (
	taskThreshold = 103
	adrThreshold  = 51
)

// threshold returns the space's record-check threshold.
func (s Space) threshold() (int, error) {
	switch s {
	case SpaceTask:
		return taskThreshold, nil
	case SpaceADR:
		return adrThreshold, nil
	default:
		return 0, fmt.Errorf("%w: %d", ErrInvalidSpace, int(s))
	}
}

// dir returns the directory under which the space's numbers are kept.
func (s Space) dir() (string, error) {
	switch s {
	case SpaceTask:
		return tasksDir, nil
	case SpaceADR:
		return adrDir, nil
	default:
		return "", fmt.Errorf("%w: %d", ErrInvalidSpace, int(s))
	}
}

// word returns the argument that names the space.
func (s Space) word() string {
	switch s {
	case SpaceTask:
		return "task"
	case SpaceADR:
		return "adr"
	default:
		return "unknown-space"
	}
}

// spaces lists the declared spaces, in the order the check reads them.
var spaces = []Space{SpaceTask, SpaceADR}

// AddedIssueKind is the kind of one pre-merge finding. Its zero value is
// invalid.
type AddedIssueKind int

const (
	addedInvalid AddedIssueKind = iota
	// AddedUnreserved is a number above the threshold with no record tag.
	AddedUnreserved
	// AddedInUseOnMain is a number another entry of the freshly fetched
	// origin/main already uses.
	AddedInUseOnMain
	// AddedOnOtherBranch is a number another origin branch adds under
	// another name.
	AddedOnOtherBranch
	// AddedMalformed is an entry the PR adds under docs/tasks/ that is not a
	// task directory.
	AddedMalformed
)

// Word returns the word a report names the kind by, and false for the zero
// value or a value outside the declared ones.
func (k AddedIssueKind) Word() (string, bool) {
	switch k {
	case AddedUnreserved:
		return "unreserved", true
	case AddedInUseOnMain:
		return "in-use-on-main", true
	case AddedOnOtherBranch:
		return "on-other-branch", true
	case AddedMalformed:
		return "malformed", true
	default:
		return "", false
	}
}

// AddedNumberIssue is one finding of the pre-merge check. Path, UsedBy, and
// Branch come from tree entries and branch names anyone who can push wrote,
// so a report prints them only through Line, which quotes them.
type AddedNumberIssue struct {
	Space Space
	// Number is the number the PR adds; the zero value for AddedMalformed.
	Number tasknaming.Number
	Kind   AddedIssueKind
	// Path is the PR's directory or file.
	Path string
	// UsedBy is the directory or file that already uses the number, on
	// origin/main or on Branch; empty for AddedUnreserved and AddedMalformed.
	UsedBy string
	// Branch is the origin branch UsedBy is on, for AddedOnOtherBranch.
	Branch string
}

// Line returns the finding as one line: its kind, space, number, the PR's
// path, and what already uses the number. Every untrusted value is quoted, so
// the line cannot be split or forged by a name. A kind outside the declared
// ones still gives a line, so a report never drops a finding.
func (i AddedNumberIssue) Line() string {
	word, ok := i.Kind.Word()
	if !ok {
		return fmt.Sprintf("unknown-finding %s %s %s", i.Space.word(), i.Number.String(), quote(i.Path))
	}
	head := fmt.Sprintf("%s %s %s %s", word, i.Space.word(), i.Number.String(), quote(i.Path))
	switch i.Kind {
	case AddedUnreserved:
		threshold, _ := i.Space.threshold()
		return fmt.Sprintf("%s: above the threshold %04d and no record tag", head, threshold)
	case AddedInUseOnMain:
		return fmt.Sprintf("%s: origin/%s already has %s", head, mainBranch, quote(i.UsedBy))
	case AddedOnOtherBranch:
		return fmt.Sprintf("%s: origin branch %s adds %s", head, quote(i.Branch), quote(i.UsedBy))
	default:
		return fmt.Sprintf("%s %s %s: not a task directory", word, i.Space.word(), quote(i.Path))
	}
}

// commitID is the form of a full commit ID git prints and mergepr pins.
var commitID = regexp.MustCompile(`^[0-9a-f]{40}$`)

// added is one number the PR adds.
type added struct {
	number tasknaming.Number
	path   string
}

// CheckAdded returns the issues of the numbers the commit head adds over its
// merge-base with the freshly fetched origin/main, excluding headBranch from
// the other branches. An empty result means the merge may go on; an error
// means it may not.
//
// Contract: it writes nothing on GitHub or in a ref. What the PR adds is
// measured against the merge-base, never the latest origin/main, so an entry
// main renamed after the PR branched is not counted as added. When the PR
// adds no number it reads nothing beyond what that measure needs (origin's
// branch list, the merge-base, and the two trees), so a record tag of a form
// it does not accept cannot stop a PR that adds none; otherwise it reads the
// record tags of only the spaces the PR adds numbers to. The latest
// origin/main's entries of another form are skipped, not stopped on, so a PR
// that removes one can still merge. The whole run is bounded by the per-run
// time limit.
func (t *Tool) CheckAdded(ctx context.Context, head, headBranch string) ([]AddedNumberIssue, error) {
	if t.refused != nil {
		return nil, t.refused
	}
	const step = "checking the numbers the PR adds"
	if !commitID.MatchString(head) {
		return nil, stop(StopMalformed, step, "head %s is not a full commit ID", quote(head))
	}
	ctx, cancel := context.WithTimeout(ctx, t.bounds.runTimeout)
	defer cancel()
	if _, err := t.workTreeRoot(ctx); err != nil {
		return nil, err
	}
	first, err := t.listRemote(ctx)
	if err != nil {
		return nil, err
	}
	if err := t.fetch(ctx, []branch{{name: mainBranch, sha: first.mainSHA}, {name: headBranch, sha: head}}); err != nil {
		return nil, err
	}
	base, err := t.mergeBase(ctx, head, first.mainSHA)
	if err != nil {
		return nil, err
	}
	var issues []AddedNumberIssue
	addedBySpace := map[Space][]added{}
	var prefixes []string
	for _, space := range spaces {
		numbers, malformed, err := t.addedIn(ctx, space, head, base)
		if err != nil {
			return nil, err
		}
		issues = append(issues, malformed...)
		if len(numbers) == 0 {
			continue
		}
		addedBySpace[space] = numbers
		prefix, err := space.TagPrefix()
		if err != nil {
			return nil, err
		}
		prefixes = append(prefixes, prefix)
	}
	if len(prefixes) == 0 {
		return issues, nil
	}
	refs, err := t.listRemote(ctx, prefixes...)
	if err != nil {
		return nil, err
	}
	if err := t.fetch(ctx, refs.branches); err != nil {
		return nil, err
	}
	trees := map[string][]treeEntry{}
	for _, space := range spaces {
		numbers := addedBySpace[space]
		if len(numbers) == 0 {
			continue
		}
		found, err := t.checkSpace(ctx, space, numbers, refs, headBranch, trees)
		if err != nil {
			return nil, err
		}
		issues = append(issues, found...)
	}
	return issues, nil
}

// addedIn returns the numbers the space gains from base to head, and, for
// the task space, a finding for each entry head adds under docs/tasks/ that
// is not a task directory. An ADR-space entry of another form is left to
// TestADRRecords, which checks that directory's form in CI.
func (t *Tool) addedIn(ctx context.Context, space Space, head, base string) ([]added, []AddedNumberIssue, error) {
	dir, err := space.dir()
	if err != nil {
		return nil, nil, err
	}
	headEntries, err := t.listTree(ctx, head, dir, "the PR head")
	if err != nil {
		return nil, nil, err
	}
	baseEntries, err := t.listTree(ctx, base, dir, "the merge base")
	if err != nil {
		return nil, nil, err
	}
	var numbers []added
	var malformed []AddedNumberIssue
	for _, e := range headEntries {
		if slices.ContainsFunc(baseEntries, func(b treeEntry) bool { return b.path == e.path }) {
			continue
		}
		n, ok := entryNumber(space, e)
		switch {
		case ok:
			numbers = append(numbers, added{number: n, path: e.path})
		case space == SpaceTask:
			malformed = append(malformed, AddedNumberIssue{Space: space, Kind: AddedMalformed, Path: e.path})
		}
	}
	return numbers, malformed, nil
}

// checkSpace judges the numbers the PR adds to one space against the record
// tags, the latest origin/main, and every other origin branch.
func (t *Tool) checkSpace(ctx context.Context, space Space, numbers []added, refs remoteRefs, headBranch string, trees map[string][]treeEntry) ([]AddedNumberIssue, error) {
	dir, err := space.dir()
	if err != nil {
		return nil, err
	}
	threshold, err := space.threshold()
	if err != nil {
		return nil, err
	}
	prefix, err := space.TagPrefix()
	if err != nil {
		return nil, err
	}
	mainEntries, err := t.listTree(ctx, refs.mainSHA, dir, "origin/"+mainBranch)
	if err != nil {
		return nil, err
	}
	var issues []AddedNumberIssue
	for _, a := range numbers {
		if a.number.Int() > threshold && !slices.Contains(refs.tags[prefix], a.number.Int()) {
			issues = append(issues, AddedNumberIssue{Space: space, Number: a.number, Kind: AddedUnreserved, Path: a.path})
		}
		for _, e := range sameNumber(space, a, mainEntries, nil) {
			issues = append(issues, AddedNumberIssue{Space: space, Number: a.number, Kind: AddedInUseOnMain, Path: a.path, UsedBy: e.path})
		}
	}
	for _, b := range refs.branches {
		if b.name == mainBranch || b.name == headBranch {
			continue
		}
		entries, err := t.branchTree(ctx, b, dir, trees)
		if err != nil {
			return nil, err
		}
		for _, a := range numbers {
			for _, e := range sameNumber(space, a, entries, mainEntries) {
				issues = append(issues, AddedNumberIssue{Space: space, Number: a.number, Kind: AddedOnOtherBranch, Path: a.path, UsedBy: e.path, Branch: b.name})
			}
		}
	}
	return issues, nil
}

// sameNumber returns the entries of the space's form that carry a's number
// under another name, leaving out every entry whose path is in skip (the
// latest origin/main's entries, when judging another branch).
func sameNumber(space Space, a added, entries, skip []treeEntry) []treeEntry {
	var found []treeEntry
	for _, e := range entries {
		if e.path == a.path || slices.ContainsFunc(skip, func(s treeEntry) bool { return s.path == e.path }) {
			continue
		}
		if n, ok := entryNumber(space, e); ok && n == a.number {
			found = append(found, e)
		}
	}
	return found
}

// branchTree lists dir on branch b, reading each head and directory once.
func (t *Tool) branchTree(ctx context.Context, b branch, dir string, trees map[string][]treeEntry) ([]treeEntry, error) {
	key := b.sha + "\x00" + dir
	if entries, ok := trees[key]; ok {
		return entries, nil
	}
	entries, err := t.listTree(ctx, b.sha, dir, "origin branch "+quote(b.name))
	if err != nil {
		return nil, err
	}
	trees[key] = entries
	return entries, nil
}

// entryNumber returns the number of a tree entry of the space's form: a task
// directory under docs/tasks/, or an ADR file under docs/dev/adr/.
func entryNumber(space Space, e treeEntry) (tasknaming.Number, bool) {
	name := path.Base(e.path)
	switch space {
	case SpaceTask:
		if e.kind != "tree" {
			return tasknaming.Number{}, false
		}
		return tasknaming.ParseTaskDir(name)
	case SpaceADR:
		if e.kind != "blob" {
			return tasknaming.Number{}, false
		}
		return tasknaming.ParseADRFile(name)
	default:
		return tasknaming.Number{}, false
	}
}

// mergeBase returns the merge-base of head and main.
//
// Contract: the output must be exactly one full commit ID and a newline;
// anything else stops as malformed. A failure, including no common ancestor,
// stops as local-git.
func (t *Tool) mergeBase(ctx context.Context, head, main string) (string, error) {
	const step = "finding the merge base of the PR head and origin/main"
	out, err := t.runGit(ctx, "merge-base", head, main)
	if err != nil {
		return "", gitStop(err, step, false)
	}
	base, ok := strings.CutSuffix(string(out), "\n")
	if !ok || !commitID.MatchString(base) {
		return "", stop(StopMalformed, step, "git did not print one commit ID")
	}
	return base, nil
}
