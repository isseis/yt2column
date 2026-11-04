package coveragecompare

import (
	"context"
	"errors"
	"slices"
	"strings"

	"github.com/isseis/yt2column/internal/cmdrunner"
)

// userState is what the run must leave as it found in the user's repository:
// its HEAD commit, its current branch (empty when HEAD is detached), its git
// status, and the paths of the worktrees git has registered. The other
// worktrees' HEADs and branches are not compared, because the user may work in
// them while the run goes on.
type userState struct {
	head      string
	branch    string
	status    string
	worktrees []string
	// statusArgs is the status command the state was read with, which the
	// state after the run is read with too. It is not compared.
	statusArgs []string
}

// The names the stop reasons give the compared items.
const (
	stateHead      = "HEAD"
	stateBranch    = "branch"
	stateStatus    = "git status"
	stateWorktrees = "worktree list"
)

// readUserState reads the user's state. A part that cannot be read stops as
// internal and names the part. status is the status command userStatusArgs
// returns.
func readUserState(ctx context.Context, git gitRunner, status []string) (userState, error) {
	state := userState{statusArgs: status}
	head, err := resolveObject(ctx, git, "HEAD^{commit}")
	if err != nil {
		return userState{}, commandStop(ctx, PointNone, "reading the user's "+stateHead+" failed", err)
	}
	state.head = head
	branch, err := git.run(ctx, "symbolic-ref", "-q", "HEAD")
	switch exitErr, isExit := errors.AsType[*cmdrunner.ExitError](err); {
	case err == nil:
		state.branch = strings.TrimSuffix(string(branch), "\n")
	case isExit && exitErr.Code == 1 && ctx.Err() == nil:
		// HEAD is detached: there is no current branch.
	default:
		return userState{}, commandStop(ctx, PointNone, "reading the user's "+stateBranch+" failed", err)
	}
	out, err := git.run(ctx, status...)
	if err != nil {
		return userState{}, commandStop(ctx, PointNone, "reading the user's "+stateStatus+" failed", err)
	}
	state.status = string(out)
	list, err := git.run(ctx, "worktree", "list", "--porcelain")
	if err != nil {
		return userState{}, commandStop(ctx, PointNone, "reading the user's "+stateWorktrees+" failed", err)
	}
	for line := range strings.SplitSeq(string(list), "\n") {
		if path, ok := strings.CutPrefix(line, "worktree "); ok {
			state.worktrees = append(state.worktrees, path)
		}
	}
	slices.Sort(state.worktrees)
	return state, nil
}

// changed returns the names of the items that differ between s and after.
func (s userState) changed(after userState) []string {
	var items []string
	if s.head != after.head {
		items = append(items, stateHead)
	}
	if s.branch != after.branch {
		items = append(items, stateBranch)
	}
	if s.status != after.status {
		items = append(items, stateStatus)
	}
	if !slices.Equal(s.worktrees, after.worktrees) {
		items = append(items, stateWorktrees)
	}
	return items
}
