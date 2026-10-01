package mergepr

import "errors"

// Sentinel errors. Callers and tests match them with errors.Is.
var (
	errNoRunner           = errors.New("no command runner")
	errInvalidPRArg       = errors.New("invalid PR argument")
	errInvalidRemote      = errors.New("origin is not a GitHub remote")
	errRemoteMismatch     = errors.New("origin fetch and push URLs name different repositories")
	errRepoMismatch       = errors.New("gh selects a repository other than origin")
	errDetachedHead       = errors.New("HEAD is detached")
	errInvalidBranch      = errors.New("unsafe or invalid branch name")
	errDirtyWorktree      = errors.New("worktree is not clean")
	errHeadBranchMismatch = errors.New("local head branch does not match headRefOid")
	errPRNotOpen          = errors.New("PR is not open")
	errPRNotMerged        = errors.New("PR is not merged")
	errCrossRepository    = errors.New("cross-repository PRs are not supported")
	errTooLarge           = errors.New("input is too large to draft from safely")
	errChecksFailed       = errors.New("CI checks failed")
	errInvalidState       = errors.New("invalid state")
	errHeadDrift          = errors.New("PR head changed after prepare")
	errBaseDrift          = errors.New("PR base changed after prepare")
	errInvalidSubject     = errors.New("squash subject is not a non-empty single line")
	errInvalidPath        = errors.New("invalid file path")
	errNoMergeCommit      = errors.New("merge commit was not recorded")
	errLocalBranchDrift   = errors.New("local head branch moved")
	errBranchCheckedOut   = errors.New("local head branch is checked out in another worktree")
	errBaseNotCurrent     = errors.New("local base branch is not at origin")
)
