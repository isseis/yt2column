package codexreview

import (
	"bytes"
	"context"
	"errors"
	"fmt"
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

// statusArgs is the git status command whose output is compared. It pins the
// options that a user's configuration could otherwise change, so the
// comparison does not depend on it: it lists every untracked file
// (status.showUntrackedFiles cannot hide one) and every submodule change.
var statusArgs = []string{"status", "--porcelain=v1", "-z", "--untracked-files=all", "--ignore-submodules=none", "--no-renames"}

// gitState is the state of this worktree compared before and after codex
// runs: its status, and where HEAD is. Other branches, tags, and the stash
// are shared by every worktree of the repository and are not compared, so
// work in another worktree does not stop the review.
type gitState struct {
	status    string // the output of statusArgs, compared as it is
	branchRef string // the ref of the branch HEAD is on, such as "refs/heads/main"; empty when HEAD is detached
	commit    string // the object ID HEAD resolves to
}

// readState reads the compared state.
func readState(ctx context.Context, g git) (gitState, error) {
	status, err := g.run(ctx, statusArgs...)
	if err != nil {
		return gitState{}, err
	}
	branchRef, commit, err := readHead(ctx, g)
	if err != nil {
		return gitState{}, err
	}
	return gitState{status: string(status), branchRef: branchRef, commit: commit}, nil
}

// readHead runs rev-parse and symbolic-ref and returns what parseHead reads
// from their output: the ref of the branch HEAD is on (empty when detached)
// and the commit HEAD resolves to, so a checkout of another branch at the same
// commit differs.
func readHead(ctx context.Context, g git) (branchRef, commit string, err error) {
	commitOut, err := g.run(ctx, "rev-parse", "--verify", "HEAD")
	if err != nil {
		return "", "", err
	}
	branchOut, err := g.run(ctx, "symbolic-ref", "-q", "HEAD")
	detached := false
	if exitErr, ok := errors.AsType[*cmdrunner.ExitError](err); ok && exitErr.Code == 1 {
		// Exit status 1 with -q means HEAD is detached; parseHead checks that
		// it printed nothing.
		branchOut, err, detached = exitErr.Stdout, nil, true
	}
	if err != nil {
		return "", "", err
	}
	return parseHead(commitOut, branchOut, detached)
}

// errHeadUnreadable reports rev-parse or symbolic-ref output that is not the
// one line of its fixed form.
var errHeadUnreadable = errors.New("unexpected HEAD output")

// parseHead checks the output of rev-parse --verify HEAD and of
// symbolic-ref -q HEAD against their fixed forms. detached is true when
// symbolic-ref exited with status 1, and branchOut must then be empty.
//
// Contract: output of any other form is rejected, never repaired, and none of
// it reaches the error. The commit is a full SHA-1 object ID in lowercase hex,
// the only form Run accepts as the base. The branch is a ref under
// refs/heads/: HEAD pointing anywhere else is unreadable, so the caller stops
// instead of comparing a ref that is not a branch.
func parseHead(commitOut, branchOut []byte, detached bool) (branchRef, commit string, err error) {
	commit, ok := oneLine(commitOut)
	if !ok || !ValidBase(commit) {
		return "", "", fmt.Errorf("rev-parse: %w", errHeadUnreadable)
	}
	if detached {
		if len(branchOut) != 0 {
			return "", "", fmt.Errorf("symbolic-ref: %w", errHeadUnreadable)
		}
		return "", commit, nil
	}
	branchRef, ok = oneLine(branchOut)
	name, isBranch := strings.CutPrefix(branchRef, "refs/heads/")
	if !ok || !isBranch || name == "" || strings.ContainsRune(branchRef, 0) {
		return "", "", fmt.Errorf("symbolic-ref: %w", errHeadUnreadable)
	}
	return branchRef, commit, nil
}

// oneLine returns out without its newline when out is exactly one line that
// ends in one newline.
func oneLine(out []byte) (string, bool) {
	line, ok := bytes.CutSuffix(out, []byte("\n"))
	if !ok || bytes.ContainsRune(line, '\n') {
		return "", false
	}
	return string(line), true
}

// changedItems returns the names of the items that differ between before and
// after, in the order status, head, refs, and the refs that differ: at most
// HEAD's branch. HEAD's branch moved when HEAD is on the same branch before
// and after and resolves to another commit; that is reported as head and
// refs, whichever worktree moved it. A change of the branch HEAD is on,
// detached included, is head alone.
func changedItems(before, after gitState) (items, refs []string) {
	if before.status != after.status {
		items = append(items, itemStatus)
	}
	if before.branchRef != after.branchRef || before.commit != after.commit {
		items = append(items, itemHead)
	}
	if before.branchRef == after.branchRef && before.branchRef != "" && before.commit != after.commit {
		items = append(items, itemRefs)
		refs = []string{before.branchRef}
	}
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
	// Case is ignored: on a case-insensitive file system codex finds
	// agents.md when it looks for AGENTS.md.
	for name := range changed {
		base := filepath.Base(filepath.FromSlash(name))
		if slices.ContainsFunc(instructionFiles, func(f string) bool { return strings.EqualFold(f, base) }) {
			return true
		}
	}
	return false
}
