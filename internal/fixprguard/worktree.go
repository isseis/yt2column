package fixprguard

import (
	"bytes"
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
	"time"

	"github.com/isseis/yt2column/internal/cmdrunner"
)

const (
	makeCommand = "make"
	// makeTimeout bounds each make target.
	makeTimeout = 30 * time.Minute
	// maxBuildOutputTail is how much of a failed target's output finish
	// returns.
	maxBuildOutputTail = 8 << 10
)

// The build check stages finish runs, in order. They are fixed here, never
// taken from an argument, so no -f, -C or other target can be passed.
const (
	StageFmt  = "fmt"
	StageTest = "test"
	StageLint = "lint"
)

// BuildCheckTargets returns the make targets finish runs, in order.
func BuildCheckTargets() []string { return []string{StageFmt, StageTest, StageLint} }

// CommitSubject is the subject of the commit finish makes. The round counting
// in the /fixpr document depends on it, so it is built only here.
func CommitSubject(pr PRNumber) string {
	return "fix: address PR #" + pr.String() + " review comments"
}

// Worktree guards the work tree and the git state of the repository in the
// current directory, which must be the top of the work tree. It runs git and
// make through run, which must use the environment GitEnv returns.
type Worktree struct {
	run  cmdrunner.Runner
	rand io.Reader
}

// NewWorktree returns a Worktree over run.
func NewWorktree(run cmdrunner.Runner) *Worktree {
	return &Worktree{run: run, rand: rand.Reader}
}

// CheckClean fails with ErrDirtyWorkTree when there is any uncommitted change,
// an untracked file included, so a round never mixes earlier edits into what
// it judges and commits.
func (w *Worktree) CheckClean(ctx context.Context) error {
	out, err := w.git(ctx, "status", "status", "--porcelain=v1", "-z", "--untracked-files=all", "--ignore-submodules=none")
	if err != nil {
		return err
	}
	if len(out) != 0 {
		return ErrDirtyWorkTree
	}
	return nil
}

// Begin checks that the work tree is clean and then takes the baseline. The
// command runs the fork check between the two, so it calls CheckClean and
// Baseline itself; Begin is the order without it.
func (w *Worktree) Begin(ctx context.Context) (Baseline, error) {
	if err := w.CheckClean(ctx); err != nil {
		return Baseline{}, err
	}
	return w.Baseline(ctx)
}

// ScopedChange is a changed path as inspect reports it. Path is empty when the
// path failed the path checks, which are never printed; Position (1-based,
// in path order) locates it instead. Added and Deleted are line counts unless
// Binary.
type ScopedChange struct {
	Position int        `json:"position"`
	Path     string     `json:"path,omitempty"`
	Kind     ChangeKind `json:"kind"`
	Added    int        `json:"added"`
	Deleted  int        `json:"deleted"`
	Binary   bool       `json:"binary"`
}

func scopedChanges(changes []change, keep func(change) bool) []ScopedChange {
	out := []ScopedChange{}
	for i, c := range changes {
		if !keep(c) {
			continue
		}
		s := ScopedChange{Position: i + 1, Kind: c.kind, Added: c.added, Deleted: c.deleted, Binary: c.binary}
		if validScopePath(c.path) {
			s.Path = c.path
		}
		out = append(out, s)
	}
	return out
}

// Inspection is the output of inspect. OutOfScope lists the changes outside
// the scope of the threads; when it is not empty the user is asked before
// finish runs.
type Inspection struct {
	Protocol   int            `json:"protocol"`
	JudgedSet  JudgedSetID    `json:"judgedSet"`
	Changes    int            `json:"changes"`
	OutOfScope []ScopedChange `json:"outOfScope"`
}

// Inspect compares the git state with the baseline (push target included),
// lists every change against the baseline's HEAD, and judges each against the
// scope of threads. threads comes from GitHub.ThreadPaths, never from the
// caller's arguments. It changes nothing.
func (w *Worktree) Inspect(ctx context.Context, base Baseline, threads []ThreadPath) (Inspection, error) {
	head, err := w.compareState(ctx, base, checkWithRemote)
	if err != nil {
		return Inspection{}, err
	}
	changes, err := w.enumerateChanges(ctx, head)
	if err != nil {
		return Inspection{}, err
	}
	return Inspection{
		Protocol:   Protocol,
		JudgedSet:  judgedSetID(changes),
		Changes:    len(changes),
		OutOfScope: scopedChanges(changes, func(c change) bool { return !inScope(c.path, c.kind, threads) }),
	}, nil
}

// JudgedSetError reports changes that differ from the judged set. Changes
// lists the changes found at the failed check, for the final report.
type JudgedSetError struct {
	Changes []ScopedChange
}

// Error names the rule only; the paths stay in Changes.
func (e *JudgedSetError) Error() string { return ErrJudgedSetChanged.Error() }

// Unwrap returns ErrJudgedSetChanged.
func (e *JudgedSetError) Unwrap() error { return ErrJudgedSetChanged }

func judgedSetChanged(changes []change) error {
	return &JudgedSetError{Changes: scopedChanges(changes, func(change) bool { return true })}
}

// BuildCheckError reports a failed build check stage with the tail of its
// output. The output comes from code the round may have changed, so it is
// fenced as untrusted text.
type BuildCheckError struct {
	Stage  string
	Output Block
}

// Error names the stage only.
func (e *BuildCheckError) Error() string { return ErrBuildChecksFailed.Error() + ": " + e.Stage }

// Unwrap returns ErrBuildChecksFailed.
func (e *BuildCheckError) Unwrap() error { return ErrBuildChecksFailed }

// FinishRequest names the PR the commit subject refers to.
type FinishRequest struct {
	PR PRNumber
}

// FinishResult is the output of finish. Commit is set only when Committed.
type FinishResult struct {
	Protocol  int    `json:"protocol"`
	Committed bool   `json:"committed"`
	Commit    string `json:"commit,omitempty"`
}

// Finish runs the build checks and commits exactly the judged set, enforcing
// this order in one call: the strict check that the work tree is the judged
// set and the git state the baseline; make fmt; recording the formatted set
// (formatting may change content but not which paths changed); make test and
// make lint; the strict check that nothing changed since formatting; staging
// only the judged paths, taken literally; the check that the index holds
// exactly the formatted set, undoing the staging otherwise; the commit; and
// the check that the commit's tree and the work tree are still the formatted
// set. The git state is compared before every git command that follows code
// the round could have changed. An empty judged set commits nothing.
func (w *Worktree) Finish(ctx context.Context, base Baseline, judged JudgedSetID, req FinishRequest) (FinishResult, error) {
	if _, err := ParseJudgedSetID(string(judged)); err != nil {
		return FinishResult{}, err
	}
	if !req.PR.usable() {
		return FinishResult{}, fmt.Errorf("%w: PR number", ErrInvalidInput)
	}
	judgedChanges, err := w.checkedChanges(ctx, base)
	if err != nil {
		return FinishResult{}, err
	}
	if judgedSetID(judgedChanges) != judged {
		return FinishResult{}, judgedSetChanged(judgedChanges)
	}
	if len(judgedChanges) == 0 {
		return FinishResult{Protocol: Protocol}, nil
	}
	if err := w.runMake(ctx, StageFmt); err != nil {
		return FinishResult{}, err
	}
	formatted, err := w.checkedChanges(ctx, base)
	if err != nil {
		return FinishResult{}, err
	}
	if !samePaths(judgedChanges, formatted) {
		return FinishResult{}, judgedSetChanged(formatted)
	}
	for _, stage := range []string{StageTest, StageLint} {
		if err := w.runMake(ctx, stage); err != nil {
			return FinishResult{}, err
		}
		// A target that rewrote the configuration stops the run before
		// the next target or git command reads it.
		if err := w.CheckState(ctx, base); err != nil {
			return FinishResult{}, err
		}
	}
	checked, err := w.checkedChanges(ctx, base)
	if err != nil {
		return FinishResult{}, err
	}
	if judgedSetID(checked) != judgedSetID(formatted) {
		return FinishResult{}, judgedSetChanged(checked)
	}
	if err := w.stage(ctx, formatted); err != nil {
		return FinishResult{}, err
	}
	if _, err := w.git(ctx, "commit", "commit", "-q", "-m", CommitSubject(req.PR)); err != nil {
		return FinishResult{}, errors.Join(err, w.unstage(ctx))
	}
	commit, err := w.checkCommit(ctx, base, formatted)
	if err != nil {
		return FinishResult{}, err
	}
	return FinishResult{Protocol: Protocol, Committed: true, Commit: commit}, nil
}

// checkedChanges compares the git state with the baseline (without the push
// target, which finish does not touch) and lists the changes against the
// baseline's HEAD.
func (w *Worktree) checkedChanges(ctx context.Context, base Baseline) ([]change, error) {
	head, err := w.compareState(ctx, base, checkLocal)
	if err != nil {
		return nil, err
	}
	return w.enumerateChanges(ctx, head)
}

// samePaths reports whether a and b change the same paths with the same kinds
// and modes; contents may differ.
func samePaths(a, b []change) bool {
	return slices.EqualFunc(a, b, func(x, y change) bool {
		return x.path == y.path && x.kind == y.kind && x.mode == y.mode
	})
}

// runMake runs one fixed make target under makeTimeout. A failure keeps only
// the stage and the fenced tail of the output.
func (w *Worktree) runMake(ctx context.Context, stage string) error {
	ctx, cancel := context.WithTimeout(ctx, makeTimeout)
	defer cancel()
	if _, err := w.run.Run(ctx, makeCommand, stage); err != nil {
		var output []byte
		if exitErr, ok := errors.AsType[*cmdrunner.ExitError](err); ok {
			output = slices.Concat(exitErr.Stdout, exitErr.Stderr)
			output = output[max(0, len(output)-maxBuildOutputTail):]
		}
		block, fenceErr := Fence(output, w.rand)
		if fenceErr != nil {
			return fmt.Errorf("%w: %s: %w", ErrBuildChecksFailed, stage, fenceErr)
		}
		return &BuildCheckError{Stage: stage, Output: block}
	}
	return nil
}

// stage adds exactly the paths of want with pathspec magic and globbing off,
// then checks that the index differs from HEAD by want alone and that no
// unstaged or untracked change remains. On a mismatch it undoes the staging.
// The caller has just compared the git state with the baseline.
func (w *Worktree) stage(ctx context.Context, want []change) error {
	paths := make([]string, len(want))
	for i, c := range want {
		paths[i] = c.path
	}
	if _, err := w.git(ctx, "add", append([]string{"--literal-pathspecs", "add", "--"}, paths...)...); err != nil {
		return errors.Join(err, w.unstage(ctx))
	}
	if err := w.checkStaged(ctx, want); err != nil {
		return errors.Join(err, w.unstage(ctx))
	}
	return nil
}

// checkStaged checks that the index differs from HEAD in exactly the paths of
// want and matches the work tree everywhere, with nothing untracked left.
// Together with the work tree being want, the index then holds want.
func (w *Worktree) checkStaged(ctx context.Context, want []change) error {
	cached, err := w.git(ctx, "diff --cached", "diff", "--cached", "--name-only", "--no-renames", "-z", "HEAD", "--")
	if err != nil {
		return err
	}
	staged := splitNUL(cached)
	slices.Sort(staged)
	paths := make([]string, len(want))
	for i, c := range want {
		paths[i] = c.path
	}
	if !slices.Equal(staged, paths) {
		return judgedSetChanged(want)
	}
	return w.checkNoLooseChanges(ctx, want)
}

// checkNoLooseChanges fails unless the index matches the work tree and no
// untracked file exists.
func (w *Worktree) checkNoLooseChanges(ctx context.Context, want []change) error {
	unstaged, err := w.git(ctx, "diff", "diff", "--name-only", "--no-renames", "--no-ext-diff", "--no-textconv", "-z", "--")
	if err != nil {
		return err
	}
	untracked, err := w.git(ctx, "ls-files", "ls-files", "--others", "--exclude-standard", "-z")
	if err != nil {
		return err
	}
	if len(unstaged) != 0 || len(untracked) != 0 {
		return judgedSetChanged(want)
	}
	return nil
}

// unstage resets the index to HEAD, leaving the work tree as it is.
func (w *Worktree) unstage(ctx context.Context) error {
	_, err := w.git(ctx, "reset", "reset", "-q", "--")
	return err
}

// checkCommit checks the commit just made, first comparing the git state
// again since the commit ran hooks: its only parent is the baseline's HEAD, the work tree and index match it, and the work tree against the
// baseline's HEAD is still want. A commit hook that rewrote the index or the
// work tree fails here, and no SHA is returned.
func (w *Worktree) checkCommit(ctx context.Context, base Baseline, want []change) (string, error) {
	if err := w.CheckState(ctx, base); err != nil {
		return "", err
	}
	commit, _, err := w.headState(ctx)
	if err != nil {
		return "", err
	}
	if err := w.checkParent(ctx, commit, base.head); err != nil {
		return "", err
	}
	if err := w.checkNoLooseChanges(ctx, want); err != nil {
		return "", err
	}
	cached, err := w.git(ctx, "diff --cached", "diff", "--cached", "--name-only", "-z", "HEAD", "--")
	if err != nil {
		return "", err
	}
	if len(cached) != 0 {
		return "", judgedSetChanged(want)
	}
	got, err := w.enumerateChanges(ctx, base.head)
	if err != nil {
		return "", err
	}
	if judgedSetID(got) != judgedSetID(want) {
		return "", judgedSetChanged(got)
	}
	return commit, nil
}

// checkParent fails with ErrGitStateChanged unless commit has exactly one
// parent and it is parent.
func (w *Worktree) checkParent(ctx context.Context, commit, parent string) error {
	out, err := w.git(ctx, "rev-list", "rev-list", "--parents", "-n", "1", commit, "--")
	if err != nil {
		return err
	}
	fields := strings.Fields(string(out))
	if len(fields) != 2 || fields[0] != commit || fields[1] != parent {
		return &GitStateError{Parts: []string{PartHead}}
	}
	return nil
}

func splitNUL(out []byte) []string {
	var parts []string
	for p := range bytes.SplitSeq(out, []byte{0}) {
		if len(p) > 0 {
			parts = append(parts, string(p))
		}
	}
	return parts
}

// PushResult is the output of push.
type PushResult struct {
	Protocol int    `json:"protocol"`
	Pushed   bool   `json:"pushed"`
	Commit   string `json:"commit"`
}

// Push sends commit, the SHA finish returned, after checking right before it:
// HEAD is commit and its only parent is the baseline's HEAD on the same
// branch; the hooks, configuration and info match the baseline; and the push
// target and its branch OID are the baseline's. It pushes with an explicit
// refspec, so no configured push refspec decides what is sent. The check is
// of one moment; a process that rewrites the hooks between the check and the
// push is not stopped.
func (w *Worktree) Push(ctx context.Context, base Baseline, commit string) (PushResult, error) {
	if !validOID(commit) {
		return PushResult{}, fmt.Errorf("%w: commit", ErrInvalidInput)
	}
	if err := w.CheckState(ctx, base); err != nil {
		return PushResult{}, err
	}
	if err := w.checkIndexFlags(ctx); err != nil {
		return PushResult{}, err
	}
	head, branch, err := w.headState(ctx)
	if err != nil {
		return PushResult{}, err
	}
	if head != commit || branch == "" || digestString(branch) != base.branch {
		return PushResult{}, &GitStateError{Parts: []string{PartHead}}
	}
	if err := w.checkParent(ctx, commit, base.head); err != nil {
		return PushResult{}, err
	}
	target, err := w.resolvePushTarget(ctx, branch)
	if err != nil {
		return PushResult{}, err
	}
	oid, err := w.remoteOID(ctx, target)
	if err != nil {
		return PushResult{}, err
	}
	if target.id() != base.remoteID || oid != base.remoteOID {
		return PushResult{}, &GitStateError{Parts: []string{PartRemote}}
	}
	if _, err := w.git(ctx, "push", "push", "-q", "--", target.remote, commit+":"+headsPrefix+target.branch); err != nil {
		return PushResult{}, err
	}
	return PushResult{Protocol: Protocol, Pushed: true, Commit: commit}, nil
}
