package coveragecompare

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
)

// Run compares the coverage of BASE and HEAD and returns the result.
//
// Contract: Run never returns without removing the work directory, which
// holds everything it created, and without checking that the user's HEAD,
// branch, git status, and worktree list are as it found them; a failure of
// either replaces the result with an internal stop. The removal and the check
// run with a context that is not canceled, so they also happen after a
// signal. The stops before any test run come in the order usage,
// not-ancestor, dirty, toolchain; a test failure at BASE stops before HEAD
// is run; an internal failure stops where it happens; and judge orders the
// rest.
func Run(ctx context.Context, opts Options) Result {
	result := Result{Target: opts.Target, BaseArg: opts.Base}
	if !filepath.IsAbs(opts.RepoRoot) {
		return stopFromError(result, internalStop(PointNone, "the repository root is not an absolute path", errRepoRoot))
	}
	workDir, err := makeWorkDir(opts.Env)
	if err != nil {
		return stopFromError(result, internalStop(PointNone, "creating the work directory failed", err))
	}
	rc := newRunContext(opts.RepoRoot, workDir, opts.Env)
	var before *userState
	result = rc.measure(ctx, opts, result, &before)
	return rc.finish(ctx, result, before)
}

// errRepoRoot reports Options.RepoRoot that is not an absolute path.
var errRepoRoot = errors.New("Options.RepoRoot must be an absolute path")

// measure runs the checks, both suites, and the judging. before receives the
// user's state once the checks have read it.
func (rc *runContext) measure(ctx context.Context, opts Options, r Result, before **userState) Result {
	t, err := rc.target(opts.Target)
	if err != nil {
		return stopAt(ctx, r, err)
	}
	repo := rc.repoGit()
	ids, err := resolveCommits(ctx, repo, opts.Base)
	if err != nil {
		return stopAt(ctx, r, err)
	}
	r.BaseCommit, r.HeadCommit, r.HeadTree = ids.base, ids.head, ids.headTree
	if err := checkAncestor(ctx, repo, ids); err != nil {
		return stopAt(ctx, r, err)
	}
	if err := checkClean(ctx, repo); err != nil {
		return stopAt(ctx, r, err)
	}
	state, err := readUserState(ctx, repo)
	if err != nil {
		return stopAt(ctx, r, err)
	}
	*before = &state

	baseSrc, err := newCommitSource(ctx, repo, PointBase, ids.base)
	if err != nil {
		return stopAt(ctx, r, err)
	}
	headSrc, err := newCommitSource(ctx, repo, PointHead, ids.head)
	if err != nil {
		return stopAt(ctx, r, err)
	}
	if r.Toolchain, err = t.checkToolchain(ctx, baseSrc, headSrc); err != nil {
		return stopAt(ctx, r, err)
	}

	// HEAD's checkout is made first, so a failure of BASE's leaves one
	// complete checkout behind for the cleanup to remove.
	for _, side := range []struct {
		point  Point
		commit string
	}{{PointHead, ids.head}, {PointBase, ids.base}} {
		dir := rc.checkoutDir(side.point)
		if err := makeCheckout(ctx, repo, rc.repoRoot, dir, side.commit, rc.environ); err != nil {
			return stopAt(ctx, r, err)
		}
		if err := t.prepare(dir); err != nil {
			return stopAt(ctx, r, err)
		}
	}

	baseRaw, err := t.run(ctx, PointBase, rc.checkoutDir(PointBase))
	if err != nil {
		return stopAt(ctx, r, err)
	}
	baseResults, readErr := t.readResults(baseRaw, PointBase)
	if reasons := testFailure(baseRaw, baseResults, readErr, PointBase); len(reasons) > 0 {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return stopAt(ctx, r, ctxErr)
		}
		r.Base.Tests = baseResults.Tests
		return stopWith(r, StopTestFailed, reasons)
	}
	headRaw, err := t.run(ctx, PointHead, rc.checkoutDir(PointHead))
	if err != nil {
		return stopAt(ctx, r, err)
	}
	diff, err := readDiff(ctx, repo, ids, t.pathspec())
	if err != nil {
		return stopAt(ctx, r, err)
	}

	judged := judge(ctx, t, judgeInput{Base: baseRaw, Head: headRaw, Diff: diff, BaseSource: baseSrc, HeadSource: headSrc})
	if ctxErr := ctx.Err(); ctxErr != nil {
		return stopAt(ctx, r, ctxErr)
	}
	judged.Target, judged.BaseArg = r.Target, r.BaseArg
	judged.BaseCommit, judged.HeadCommit, judged.HeadTree = r.BaseCommit, r.HeadCommit, r.HeadTree
	judged.Toolchain = r.Toolchain
	return judged
}

// stopAt returns r stopped by err, or by the interruption when ctx is done,
// whatever err says: a read that failed because the run was canceled is
// reported as the cancellation.
func stopAt(ctx context.Context, r Result, err error) Result {
	if ctxErr := ctx.Err(); ctxErr != nil {
		return stopFromError(r, interruptedStop(ctxErr))
	}
	return stopFromError(r, err)
}

// target returns the comparison opts names. A value outside the two is the
// caller's mistake and fails as internal.
func (rc *runContext) target(kind Target) (target, error) {
	switch kind {
	case TargetGo:
		return goTarget{rc: rc}, nil
	case TargetExt:
		deps := &dependencyGuard{userModules: filepath.Join(rc.repoRoot, "extension", "node_modules")}
		return extTarget{ranges: helperRanges{rc: rc}, rc: rc, deps: deps}, nil
	default:
		return nil, internalStop(PointNone, "the target is neither go nor ext", errUnknownTarget)
	}
}

// errUnknownTarget reports a Target outside TargetGo and TargetExt.
var errUnknownTarget = errors.New("unknown target")

// readDiff returns the diff from BASE to HEAD over pathspec, between the
// pinned IDs.
//
// Contract: the output's shape does not depend on the user's configuration.
// The global and system configuration are not read (gitEnv); the repository's
// own .git/config is, so every diff.* setting that changes the output is
// overridden here: no external diff or textconv, no rename detection (a
// renamed file is a deletion and an addition), no color, no context and no
// merged hunks, paths from the root with the a/ and b/ prefixes, the myers
// algorithm with the indent heuristic, and unquoted non-ASCII paths.
// Whitespace is never ignored, so a changed line is never taken for an
// unchanged one.
func readDiff(ctx context.Context, repo gitRunner, ids pinnedCommits, pathspec []string) ([]byte, error) {
	args := []string{
		"-c", "core.quotePath=false", "diff",
		"--no-ext-diff", "--no-textconv", "--no-renames", "--no-color",
		"--unified=0", "--inter-hunk-context=0", "--no-relative",
		"--src-prefix=a/", "--dst-prefix=b/",
		"--diff-algorithm=myers", "--indent-heuristic",
		ids.base, ids.head, "--",
	}
	out, err := repo.run(ctx, append(args, pathspec...)...)
	if err != nil {
		return nil, commandStop(ctx, PointNone, "reading the diff from BASE to HEAD failed", err)
	}
	return out, nil
}

// finish removes the work directory and compares the user's state with the
// state read before the run, when the run got that far. Either failure
// replaces the result with an internal stop that names what failed or what
// changed, after the reasons of an internal stop the run already made.
func (rc *runContext) finish(ctx context.Context, r Result, before *userState) Result {
	quiet := context.WithoutCancel(ctx)
	var reasons []StopReason
	if err := os.RemoveAll(rc.workDir); err != nil {
		reasons = append(reasons, StopReason{Detail: "removing the work directory failed", Excerpt: err.Error()})
	}
	if before != nil {
		after, err := readUserState(quiet, rc.repoGit())
		if stop, ok := errors.AsType[*StopError](err); ok {
			reasons = append(reasons, stop.Reasons...)
		} else if err != nil {
			reasons = append(reasons, StopReason{Detail: "reading the user's state after the run failed", Excerpt: err.Error()})
		} else {
			for _, item := range before.changed(after) {
				reasons = append(reasons, StopReason{Detail: "the user's " + item + " changed during the run"})
			}
		}
	}
	if len(reasons) == 0 {
		return r
	}
	if r.Outcome == OutcomeStop && r.Stop == StopInternal {
		// Keep the failure the run stopped for ahead of what the cleanup
		// found.
		reasons = append(slices.Clone(r.Reasons), reasons...)
	}
	r.Deleted, r.Lost = nil, nil
	return stopWith(r, StopInternal, reasons)
}
