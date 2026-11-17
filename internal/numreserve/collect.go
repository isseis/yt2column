package numreserve

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/isseis/yt2column/internal/cmdrunner"
	"github.com/isseis/yt2column/internal/tasknaming"
)

const (
	branchPrefix = "refs/heads/"
	mainBranch   = "main"
	tasksDir     = "docs/tasks/"
	adrDir       = "docs/dev/adr/"
	adrIndex     = "README.md"
	// maxListedUses bounds how many inputs a jump detail names.
	maxListedUses = 10
)

// errNoListingForm reports a directory readMainList has no form for, a
// mistake in the caller.
var errNoListingForm = errors.New("numreserve: no listing form for the directory")

// use is one input that uses a number outside the base: a branch name, or an
// ADR file on a branch or in the work tree. where is already quoted.
type use struct {
	number int
	where  string
}

// collected is what a run read before it creates a tag.
//
// Contract: base is the largest number on origin/main and in the record
// tags, never one taken from a branch or the work tree, so a stray branch
// cannot move the base the jump check measures from.
type collected struct {
	mainSHA string
	base    int
	others  []use
}

// largest returns the largest number in use.
func (c collected) largest() int {
	largest := c.base
	for _, u := range c.others {
		largest = max(largest, u.number)
	}
	return largest
}

// branch is one origin branch from git ls-remote.
type branch struct {
	name string
	sha  string
}

// remoteRefs is what git ls-remote returned for one space.
type remoteRefs struct {
	mainSHA  string
	branches []branch
	tags     []int
}

// lsRemoteLine is the one accepted line form: 40 lowercase hexadecimal
// digits, a tab, and a ref name.
var lsRemoteLine = regexp.MustCompile(`^([0-9a-f]{40})\t(\S+)$`)

// collect reads every input of the space (architecture 3.2).
func (t *Tool) collect(ctx context.Context, space Space, tagPrefix string) (collected, error) {
	switch space {
	case SpaceTask:
		return t.collectTask(ctx, tagPrefix)
	case SpaceADR:
		return t.collectADR(ctx, tagPrefix)
	default:
		return collected{}, fmt.Errorf("%w: %d", ErrInvalidSpace, int(space))
	}
}

// collectTask reads origin/main's docs/tasks/, the origin and local branch
// names, and the task record tags. It finds the work tree root first, as
// collectADR does, so a run outside a repository stops as local-git before
// git ls-remote could fail for the same reason and read as a network failure.
func (t *Tool) collectTask(ctx context.Context, tagPrefix string) (collected, error) {
	if _, err := t.workTreeRoot(ctx); err != nil {
		return collected{}, err
	}
	refs, err := t.listRemote(ctx, tagPrefix)
	if err != nil {
		return collected{}, err
	}
	if err := t.fetch(ctx, []branch{{name: mainBranch, sha: refs.mainSHA}}); err != nil {
		return collected{}, err
	}
	mainNumbers, err := t.readMainList(ctx, refs.mainSHA, tasksDir)
	if err != nil {
		return collected{}, err
	}
	c := collected{mainSHA: refs.mainSHA, base: maxOf(mainNumbers, refs.tags)}
	for _, b := range refs.branches {
		if n, _, ok := tasknaming.ParseBranch(b.name); ok {
			c.others = append(c.others, use{number: n.Int(), where: "origin branch " + quote(b.name)})
		}
	}
	local, err := t.localBranches(ctx)
	if err != nil {
		return collected{}, err
	}
	for _, name := range local {
		if n, _, ok := tasknaming.ParseBranch(name); ok {
			c.others = append(c.others, use{number: n.Int(), where: "local branch " + quote(name)})
		}
	}
	return c, nil
}

// collectADR reads origin/main's docs/dev/adr/, every origin branch's
// docs/dev/adr/, HEAD's and the work tree's docs/dev/adr/, and the ADR
// record tags.
func (t *Tool) collectADR(ctx context.Context, tagPrefix string) (collected, error) {
	root, err := t.workTreeRoot(ctx)
	if err != nil {
		return collected{}, err
	}
	refs, err := t.listRemote(ctx, tagPrefix)
	if err != nil {
		return collected{}, err
	}
	if err := t.fetch(ctx, refs.branches); err != nil {
		return collected{}, err
	}
	mainNumbers, err := t.readMainList(ctx, refs.mainSHA, adrDir)
	if err != nil {
		return collected{}, err
	}
	c := collected{mainSHA: refs.mainSHA, base: maxOf(mainNumbers, refs.tags)}
	read := map[string][]treeEntry{}
	for _, b := range refs.branches {
		if b.name == mainBranch {
			continue
		}
		entries, ok := read[b.sha]
		if !ok {
			entries, err = t.listTree(ctx, b.sha, adrDir, "origin branch "+quote(b.name))
			if err != nil {
				return collected{}, err
			}
			read[b.sha] = entries
		}
		for _, e := range entries {
			if n, ok := adrNumber(e); ok {
				c.others = append(c.others, use{number: n, where: quote(e.path) + " on origin branch " + quote(b.name)})
			}
		}
	}
	head, err := t.listTree(ctx, "HEAD", adrDir, "HEAD")
	if err != nil {
		return collected{}, err
	}
	for _, e := range head {
		if n, ok := adrNumber(e); ok {
			c.others = append(c.others, use{number: n, where: quote(e.path) + " in HEAD"})
		}
	}
	names, err := readWorkTreeADRs(root)
	if err != nil {
		return collected{}, err
	}
	for _, name := range names {
		if n, ok := tasknaming.ParseADRFile(name); ok {
			c.others = append(c.others, use{number: n.Int(), where: quote(adrDir+name) + " in the work tree"})
		}
	}
	return c, nil
}

// listRemote reads origin's branches and the space's record tags with one
// git ls-remote.
//
// Contract: every line has the accepted form; a line under refs/heads/ is a
// branch and a line under tagPrefix is a record tag whose name must be
// exactly four ASCII digits (a nested name or a peeled ^{} line stops as
// malformed); any other line (git matches patterns at the end of a name) is
// skipped. origin must have main, and at most maxBranches branches.
func (t *Tool) listRemote(ctx context.Context, tagPrefix string) (remoteRefs, error) {
	const step = "listing origin's branches and record tags"
	out, err := t.runGit(ctx, "ls-remote", "origin", branchPrefix+"*", tagPrefix+"*")
	if err != nil {
		return remoteRefs{}, gitStop(err, step, true)
	}
	var refs remoteRefs
	for _, line := range lines(out) {
		m := lsRemoteLine.FindStringSubmatch(line)
		if m == nil {
			return remoteRefs{}, stop(StopMalformed, step, "line %s is not <40 hex digits><tab><ref>", quote(line))
		}
		sha, ref := m[1], m[2]
		switch {
		case strings.HasPrefix(ref, branchPrefix):
			name := strings.TrimPrefix(ref, branchPrefix)
			if name == mainBranch {
				refs.mainSHA = sha
			}
			refs.branches = append(refs.branches, branch{name: name, sha: sha})
		case strings.HasPrefix(ref, tagPrefix):
			n, err := tasknaming.ParseNumber(strings.TrimPrefix(ref, tagPrefix))
			if err != nil {
				return remoteRefs{}, stop(StopMalformed, step, "record tag %s is not four digits under %s", quote(ref), tagPrefix)
			}
			refs.tags = append(refs.tags, n.Int())
		}
	}
	if len(refs.branches) > t.bounds.maxBranches {
		return remoteRefs{}, stop(StopListLimit, step, "origin has %d branches, more than %d", len(refs.branches), t.bounds.maxBranches)
	}
	if refs.mainSHA == "" {
		return remoteRefs{}, stop(StopMalformed, step, "origin has no %s%s", branchPrefix, mainBranch)
	}
	return refs, nil
}

// fetch brings the heads of branches into the object store without updating
// any ref, local tag, or FETCH_HEAD, so two runs in worktrees of one
// repository never contend for a ref lock.
//
// Contract: a failure stops the run. The detail names the branches whose
// heads are not present after the failed fetch, read with git cat-file,
// never git's standard error; a fetch fails as a whole, so in a clone that
// lacked other heads too, it names those as well as the one that caused it.
func (t *Tool) fetch(ctx context.Context, branches []branch) error {
	const step = "fetching branch heads from origin"
	var shas []string
	for _, b := range branches {
		if !slices.Contains(shas, b.sha) {
			shas = append(shas, b.sha)
		}
	}
	args := append([]string{"fetch", "--no-tags", "--no-write-fetch-head", "origin"}, shas...)
	_, err := t.runGit(ctx, args...)
	if err == nil {
		return nil
	}
	if _, isExit := errors.AsType[*cmdrunner.ExitError](err); !isExit || isTimeout(err) {
		return gitStop(err, step, true)
	}
	missing, checkErr := t.missingObjects(ctx, shas)
	if checkErr != nil {
		return checkErr
	}
	var names []string
	for _, b := range branches {
		if slices.Contains(missing, b.sha) {
			names = append(names, quote(b.name))
		}
	}
	if len(names) == 0 {
		return gitStop(err, step, true)
	}
	return stop(StopNetworkOrGitHub, step, "could not fetch the head of origin branch %s", listed(names))
}

// missingObjects returns the SHAs git cat-file reports missing. Its result
// only shapes the detail of a stop that happens either way, so a line of any
// other form is not counted as missing.
func (t *Tool) missingObjects(ctx context.Context, shas []string) ([]string, error) {
	const step = "checking which branch heads are missing"
	stdin := []byte(strings.Join(shas, "\n") + "\n")
	out, err := t.runGitInput(ctx, stdin, "cat-file", "--batch-check")
	if err != nil {
		return nil, gitStop(err, step, false)
	}
	var missing []string
	for _, line := range lines(out) {
		if sha, ok := strings.CutSuffix(line, " missing"); ok {
			missing = append(missing, sha)
		}
	}
	return missing, nil
}

// treeEntry is one entry of git ls-tree.
type treeEntry struct {
	kind string
	path string
}

// lsTreeEntry is the one accepted entry form of git ls-tree -z: a mode, a
// space, the object type, a space, 40 hexadecimal digits, a tab, and the path.
var lsTreeEntry = regexp.MustCompile(`^[0-7]{6} (blob|tree|commit) [0-9a-f]{40}\t(.+)$`)

// listTree lists the entries directly under dir in rev.
//
// Contract: git ls-tree runs with --full-tree, so the listing does not depend
// on the directory the tool runs in (without it git resolves dir from the
// current directory and lists nothing, exit status 0, outside the root), and
// with -z, so a path is never quoted. Every entry must have the accepted form
// and lie directly under dir; anything else stops as malformed.
func (t *Tool) listTree(ctx context.Context, rev, dir, what string) ([]treeEntry, error) {
	step := "reading " + dir + " of " + what
	out, err := t.runGit(ctx, "ls-tree", "--full-tree", "-z", rev, "--", dir)
	if err != nil {
		return nil, gitStop(err, step, false)
	}
	if len(out) == 0 {
		return nil, nil
	}
	records, ok := bytes.CutSuffix(out, []byte{0})
	if !ok {
		return nil, stop(StopMalformed, step, "the listing does not end with NUL")
	}
	var entries []treeEntry
	for record := range strings.SplitSeq(string(records), "\x00") {
		m := lsTreeEntry.FindStringSubmatch(record)
		if m == nil {
			return nil, stop(StopMalformed, step, "entry %s is not <mode> <type> <sha><tab><path>", quote(record))
		}
		name, under := strings.CutPrefix(m[2], dir)
		if !under || name == "" || strings.Contains(name, "/") {
			return nil, stop(StopMalformed, step, "entry %s is not directly under %s", quote(m[2]), dir)
		}
		entries = append(entries, treeEntry{kind: m[1], path: m[2]})
	}
	return entries, nil
}

// readMainList reads the numbers of origin/main's dir (architecture 3.2.1).
//
// Contract: every entry must be of the form the space keeps there (a task
// directory under docs/tasks/; an ADR file, or the index, under
// docs/dev/adr/), and the listing must hold at least one number, because an
// empty listing cannot be told from a misread one.
func (t *Tool) readMainList(ctx context.Context, mainSHA, dir string) ([]int, error) {
	what := "origin/" + mainBranch
	step := "reading " + dir + " of " + what
	entries, err := t.listTree(ctx, mainSHA, dir, what)
	if err != nil {
		return nil, err
	}
	var numbers []int
	for _, e := range entries {
		name := path.Base(e.path)
		switch dir {
		case tasksDir:
			n, ok := tasknaming.ParseTaskDir(name)
			if e.kind != "tree" || !ok {
				return nil, stop(StopMalformed, step, "entry %s is not a task directory", quote(e.path))
			}
			numbers = append(numbers, n.Int())
		case adrDir:
			if e.kind == "blob" && name == adrIndex {
				continue
			}
			n, ok := adrNumber(e)
			if !ok {
				return nil, stop(StopMalformed, step, "entry %s is not an ADR file", quote(e.path))
			}
			numbers = append(numbers, n)
		default:
			return nil, fmt.Errorf("%w: %q", errNoListingForm, dir)
		}
	}
	if len(numbers) == 0 {
		return nil, stop(StopMalformed, step, "the listing holds no number")
	}
	return numbers, nil
}

// adrNumber returns the number of an ADR file entry.
func adrNumber(e treeEntry) (int, bool) {
	if e.kind != "blob" {
		return 0, false
	}
	n, ok := tasknaming.ParseADRFile(path.Base(e.path))
	return n.Int(), ok
}

// localBranches returns the names of the local branches.
func (t *Tool) localBranches(ctx context.Context) ([]string, error) {
	const step = "listing local branches"
	out, err := t.runGit(ctx, "for-each-ref", "--format=%(refname)", branchPrefix)
	if err != nil {
		return nil, gitStop(err, step, false)
	}
	var names []string
	for _, line := range lines(out) {
		name, ok := strings.CutPrefix(line, branchPrefix)
		if !ok {
			return nil, stop(StopMalformed, step, "line %s is not under %s", quote(line), branchPrefix)
		}
		names = append(names, name)
	}
	return names, nil
}

// workTreeRoot returns the root of the work tree the tool runs in.
func (t *Tool) workTreeRoot(ctx context.Context) (string, error) {
	const step = "finding the work tree root"
	out, err := t.runGit(ctx, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", gitStop(err, step, false)
	}
	root, ok := strings.CutSuffix(string(out), "\n")
	if !ok || !filepath.IsAbs(root) || strings.Contains(root, "\n") {
		return "", stop(StopLocalGit, step, "git did not print one absolute path")
	}
	return root, nil
}

// readWorkTreeADRs returns the entry names of the work tree's docs/dev/adr/.
func readWorkTreeADRs(root string) ([]string, error) {
	entries, err := os.ReadDir(filepath.Join(root, filepath.FromSlash(adrDir)))
	if err != nil {
		return nil, stop(StopLocalGit, "reading the work tree's "+adrDir, "the directory cannot be read")
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names, nil
}

// lines splits newline-terminated output into its lines. Output that does
// not end in a newline keeps its last line.
func lines(out []byte) []string {
	text := strings.TrimSuffix(string(out), "\n")
	if text == "" {
		return nil
	}
	return strings.Split(text, "\n")
}

func maxOf(groups ...[]int) int {
	largest := 0
	for _, g := range groups {
		for _, n := range g {
			largest = max(largest, n)
		}
	}
	return largest
}

// listed joins already-quoted values, naming at most maxListedUses of them.
func listed(values []string) string {
	if len(values) <= maxListedUses {
		return strings.Join(values, ", ")
	}
	return fmt.Sprintf("%s, and %d more", strings.Join(values[:maxListedUses], ", "), len(values)-maxListedUses)
}
