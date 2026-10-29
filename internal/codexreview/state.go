package codexreview

import (
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"slices"
	"strings"

	"github.com/isseis/yt2column/internal/cmdrunner"
)

// The names of the compared items, as Result.Changed lists them.
const (
	itemStatus = "status"
	itemHead   = "head"
	itemRefs   = "refs"
	itemStash  = "stash"
)

// instructionFiles are the base names of the files codex reads its
// instructions from.
var instructionFiles = []string{"AGENTS.md", "AGENTS.override.md", "CLAUDE.md"}

// git runs git with the hardening options in the worktree root.
type git struct {
	runner cmdrunner.Runner
}

func (g git) run(ctx context.Context, args ...string) ([]byte, error) {
	return g.runner.Run(ctx, "git", append(cmdrunner.GitHardening(), args...)...)
}

// The git commands whose output is compared. Each pins the options that a
// user's configuration could otherwise change, so the comparison does not
// depend on it: status lists every untracked file (status.showUntrackedFiles
// cannot hide one) and every submodule change, the refs are sorted by name,
// and the stash list prints only each entry's commit.
var (
	statusArgs = []string{"status", "--porcelain=v1", "-z", "--untracked-files=all", "--ignore-submodules=none", "--no-renames"}
	refsArgs   = []string{"for-each-ref", "--sort=refname", "--format=%(refname)%00%(objectname)", "refs/heads", "refs/tags"}
	stashArgs  = []string{"stash", "list", "--no-show-signature", "--format=%H"}
)

// gitState is the repository state compared before and after codex runs.
type gitState struct {
	status string
	head   string
	refs   map[string]string // ref name to the object it points at
	stash  string
}

// readState reads the compared state.
func readState(ctx context.Context, g git) (gitState, error) {
	status, err := g.run(ctx, statusArgs...)
	if err != nil {
		return gitState{}, err
	}
	head, err := readHead(ctx, g)
	if err != nil {
		return gitState{}, err
	}
	refsOut, err := g.run(ctx, refsArgs...)
	if err != nil {
		return gitState{}, err
	}
	refs, err := parseRefs(refsOut)
	if err != nil {
		return gitState{}, err
	}
	stash, err := g.run(ctx, stashArgs...)
	if err != nil {
		return gitState{}, err
	}
	return gitState{status: string(status), head: head, refs: refs, stash: string(stash)}, nil
}

// readHead returns the branch HEAD is on (empty when detached) and the commit
// it points at, so a checkout of another branch at the same commit differs.
func readHead(ctx context.Context, g git) (string, error) {
	commit, err := g.run(ctx, "rev-parse", "--verify", "HEAD")
	if err != nil {
		return "", err
	}
	branch, err := g.run(ctx, "symbolic-ref", "-q", "HEAD")
	if exitErr, ok := errors.AsType[*cmdrunner.ExitError](err); ok && exitErr.Code == 1 {
		// Exit status 1 with -q means HEAD is detached.
		branch, err = nil, nil
	}
	if err != nil {
		return "", err
	}
	return string(branch) + string(commit), nil
}

// errRefsUnreadable reports for-each-ref output that is not one name and
// object per line.
var errRefsUnreadable = errors.New("unexpected for-each-ref output")

// parseRefs reads the output of refsArgs.
func parseRefs(out []byte) (map[string]string, error) {
	refs := make(map[string]string)
	for line := range strings.SplitSeq(strings.TrimSuffix(string(out), "\n"), "\n") {
		if line == "" {
			continue
		}
		name, object, ok := strings.Cut(line, "\x00")
		if !ok || name == "" || object == "" {
			return nil, errRefsUnreadable
		}
		refs[name] = object
	}
	return refs, nil
}

// changedItems returns the names of the items that differ between before and
// after, in a fixed order, and the refs that differ, sorted.
func changedItems(before, after gitState) (items, refs []string) {
	if before.status != after.status {
		items = append(items, itemStatus)
	}
	if before.head != after.head {
		items = append(items, itemHead)
	}
	for name, object := range before.refs {
		if after.refs[name] != object {
			refs = append(refs, name)
		}
	}
	for name := range after.refs {
		if _, ok := before.refs[name]; !ok {
			refs = append(refs, name)
		}
	}
	if len(refs) > 0 {
		items = append(items, itemRefs)
	}
	if before.stash != after.stash {
		items = append(items, itemStash)
	}
	slices.Sort(refs)
	return items, refs
}

// describeRefs names, for each changed ref, the worktree it is checked out
// in, if any, with the symbolic links of the worktree's path resolved. When
// the worktree list cannot be read the refs are named alone: the result
// stops the caller either way, and the list only helps the user find why.
func describeRefs(ctx context.Context, g git, refs []string) []string {
	checkedOut := map[string]string{}
	if out, err := g.run(ctx, "worktree", "list", "--porcelain", "-z"); err == nil {
		checkedOut = parseWorktrees(out)
	}
	described := make([]string, 0, len(refs))
	for _, ref := range refs {
		if path, ok := checkedOut[ref]; ok {
			described = append(described, ref+" (worktree "+path+")")
		} else {
			described = append(described, ref)
		}
	}
	return described
}

// parseWorktrees maps each branch checked out in a worktree to the worktree's
// path, from git worktree list --porcelain -z.
func parseWorktrees(out []byte) map[string]string {
	checkedOut := map[string]string{}
	var path string
	for field := range bytes.SplitSeq(out, []byte{0}) {
		if p, ok := bytes.CutPrefix(field, []byte("worktree ")); ok {
			path = string(p)
			if resolved, err := filepath.EvalSymlinks(path); err == nil {
				path = resolved
			}
		} else if ref, ok := bytes.CutPrefix(field, []byte("branch ")); ok {
			checkedOut[string(ref)] = path
		}
	}
	return checkedOut
}

// readChangedFiles returns the paths that base..HEAD changes, relative to the
// root with forward slashes. A rename is listed as its two paths.
func readChangedFiles(ctx context.Context, g git, base string) (map[string]struct{}, error) {
	out, err := g.run(ctx, "diff", "--name-only", "-z", "--no-renames", "--no-ext-diff", "--no-textconv", "--no-relative", base, "HEAD", "--")
	if err != nil {
		return nil, err
	}
	changed := map[string]struct{}{}
	for name := range strings.SplitSeq(string(out), "\x00") {
		if name != "" {
			changed[name] = struct{}{}
		}
	}
	return changed, nil
}

// touchesInstructions reports whether any changed path is an instruction file
// codex reads, in any directory.
func touchesInstructions(changed map[string]struct{}) bool {
	for name := range changed {
		if slices.Contains(instructionFiles, filepath.Base(filepath.FromSlash(name))) {
			return true
		}
	}
	return false
}
