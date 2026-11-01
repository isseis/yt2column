package fixprguard

import (
	"context"
	"crypto/rand"
	"crypto/sha1" //nolint:gosec // git's object IDs are SHA-1 in a SHA-1 repository
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io"
	"os"
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
// current directory, which must be the top of the work tree. gitRun runs git
// and must use the environment cmdrunner.GitEnv returns and an output cap, so
// truncated output cannot hide a change; listRun runs the listing of ignored
// files with the same environment and cap and must also set StderrIsFailure,
// since git skips an unreadable directory with only a warning on standard
// error; the other git calls cannot, since git add and git commit write there
// when they succeed. makeRun runs the build checks and needs no cap, since
// only the tail of a failed target's output is kept.
type Worktree struct {
	gitRun  cmdrunner.Runner
	listRun cmdrunner.Runner
	makeRun cmdrunner.Runner
	rand    io.Reader
}

// NewWorktree returns a Worktree that runs git through gitRun, the listing of
// ignored files through listRun, and make through makeRun.
func NewWorktree(gitRun, listRun, makeRun cmdrunner.Runner) *Worktree {
	return &Worktree{gitRun: gitRun, listRun: listRun, makeRun: makeRun, rand: rand.Reader}
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

// Begin checks that the work tree is clean, takes the baseline, and writes
// the begin record file. The command runs the fork check between the first
// two, so it calls CheckClean, Baseline and WriteBeginRecord itself; Begin is
// the order without it.
func (w *Worktree) Begin(ctx context.Context) (Baseline, error) {
	if err := w.CheckClean(ctx); err != nil {
		return Baseline{}, err
	}
	base, err := w.Baseline(ctx)
	if err != nil {
		return Baseline{}, err
	}
	if err := w.WriteBeginRecord(ctx, base); err != nil {
		return Baseline{}, err
	}
	return base, nil
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

// Inspect compares the git state with the baseline (the push target branch
// included), lists every change against the baseline's commit, and judges
// each against the scope of threads. threads comes from GitHub.ThreadPaths,
// never from the caller's arguments. It changes nothing.
func (w *Worktree) Inspect(ctx context.Context, base Baseline, threads []ThreadPath) (Inspection, error) {
	if _, err := w.compareState(ctx, base, base.head, checkWithRemote); err != nil {
		return Inspection{}, err
	}
	changes, err := w.changesAgainst(ctx, base)
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

// changesAgainst lists the changes against the tree of the baseline's commit.
func (w *Worktree) changesAgainst(ctx context.Context, base Baseline) ([]change, error) {
	tree, _, _, err := w.commitObject(ctx, base.head)
	if err != nil {
		return nil, err
	}
	return w.enumerateChanges(ctx, tree)
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

// FinishRequest names the PR the commit subject refers to and, optionally,
// the file holding the commit body. An empty BodyFile means the commit holds
// the fixed subject alone.
type FinishRequest struct {
	PR       PRNumber
	BodyFile string
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
// the check that the commit's tree is the formatted set. The git state is
// compared after every make target and after every git command that runs
// hooks. An empty judged set commits nothing.
//
// With a body file, the file is read and checked (readTextFile,
// checkPostText, checkCommitBody) before anything else touches the work tree,
// and a rejected file stops the run before any make target. The message is
// built in memory from the text that was checked, so a file rewritten during
// the build checks changes nothing, and git records it verbatim; the commit
// check then requires the stored message to be that text, so a commit-msg
// hook that rewrote it fails the run. Without a body file the commit and its
// checks are unchanged.
func (w *Worktree) Finish(ctx context.Context, base Baseline, judged JudgedSetID, req FinishRequest) (FinishResult, error) {
	if _, err := ParseJudgedSetID(string(judged)); err != nil {
		return FinishResult{}, err
	}
	if !req.PR.usable() {
		return FinishResult{}, fmt.Errorf("%w: PR number", ErrInvalidInput)
	}
	message, err := commitMessage(req)
	if err != nil {
		return FinishResult{}, err
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
	// A change whose content has no digest (a nested repository, say) could
	// change after this check without changing the ID.
	if slices.ContainsFunc(judgedChanges, func(c change) bool { return c.kind != ChangeDelete && c.digest == "" }) {
		return FinishResult{}, judgedSetChanged(judgedChanges)
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
	if err := w.runMake(ctx, StageTest); err != nil {
		return FinishResult{}, err
	}
	// make test may have rewritten the configuration; stop before make lint
	// runs git with it.
	if err := w.CheckState(ctx, base); err != nil {
		return FinishResult{}, err
	}
	if err := w.runMake(ctx, StageLint); err != nil {
		return FinishResult{}, err
	}
	checked, err := w.checkedChanges(ctx, base)
	if err != nil {
		return FinishResult{}, err
	}
	if judgedSetID(checked) != judgedSetID(formatted) {
		return FinishResult{}, judgedSetChanged(checked)
	}
	if err := w.stage(ctx, base, formatted); err != nil {
		return FinishResult{}, err
	}
	commit, err := w.commit(ctx, base, formatted, req.PR, message)
	if err != nil {
		return FinishResult{}, err
	}
	return FinishResult{Protocol: Protocol, Committed: true, Commit: commit}, nil
}

// commit commits the staged set and checks the commit (checkCommit). With a
// pinned message it commits that text verbatim; otherwise the fixed subject
// alone, as finish always did. A failed commit undoes the staging.
func (w *Worktree) commit(ctx context.Context, base Baseline, staged []change, pr PRNumber, message finishMessage) (string, error) {
	args := []string{"commit", "-q", "-m", CommitSubject(pr)}
	if message.pinned {
		args = []string{"commit", "-q", "--cleanup=verbatim", "-m", message.text}
	}
	if _, err := w.git(ctx, "commit", args...); err != nil {
		return "", w.undoStage(ctx, base, err)
	}
	return w.checkCommit(ctx, base, staged, message)
}

// finishMessage is the message finish commits with a body file: text is the
// whole message and pinned says the stored message must equal it. Without a
// body file it is the zero value, and the stored message is not compared.
type finishMessage struct {
	text   string
	pinned bool
}

// commitMessage reads and checks the body file of req, when it names one, and
// builds the message: the fixed subject, an empty line, and the body ending
// in exactly one LF. git commit -m adds an LF only to text that does not end
// in one, so the stored message is this text.
func commitMessage(req FinishRequest) (finishMessage, error) {
	if req.BodyFile == "" {
		return finishMessage{}, nil
	}
	body, err := readTextFile(req.BodyFile)
	if err != nil {
		return finishMessage{}, err
	}
	if err := checkPostText(body); err != nil {
		return finishMessage{}, err
	}
	if err := checkCommitBody(body); err != nil {
		return finishMessage{}, err
	}
	return finishMessage{text: CommitSubject(req.PR) + "\n\n" + strings.TrimSuffix(body, "\n") + "\n", pinned: true}, nil
}

// checkedChanges compares the git state with the baseline (without the push
// target branch, which finish does not touch) and lists the changes against
// the baseline's commit.
func (w *Worktree) checkedChanges(ctx context.Context, base Baseline) ([]change, error) {
	if _, err := w.compareState(ctx, base, base.head, checkLocal); err != nil {
		return nil, err
	}
	return w.changesAgainst(ctx, base)
}

// samePaths reports whether a and b change the same paths with the same kinds
// and modes; contents may differ.
func samePaths(a, b []change) bool {
	return slices.EqualFunc(a, b, func(x, y change) bool {
		return x.path == y.path && x.kind == y.kind && x.mode == y.mode
	})
}

// runMake runs one fixed make target under makeTimeout. A target that exits
// unsuccessfully, a timeout included, keeps only the stage and the fenced
// tail of its output; one that did not run keeps the stage only.
func (w *Worktree) runMake(ctx context.Context, stage string) error {
	ctx, cancel := context.WithTimeout(ctx, makeTimeout)
	defer cancel()
	_, err := w.makeRun.Run(ctx, makeCommand, stage)
	if err == nil {
		return nil
	}
	exitErr, ok := errors.AsType[*cmdrunner.ExitError](err)
	if !ok {
		return fmt.Errorf("%w: %s did not run", ErrBuildChecksFailed, stage)
	}
	output := slices.Concat(exitErr.Stdout, exitErr.Stderr)
	block, err := Fence(output[max(0, len(output)-maxBuildOutputTail):], w.rand)
	if err != nil {
		return fmt.Errorf("%w: %s output not fenced", ErrBuildChecksFailed, stage)
	}
	return &BuildCheckError{Stage: stage, Output: block}
}

func changePaths(changes []change) []string {
	paths := make([]string, len(changes))
	for i, c := range changes {
		paths[i] = c.path
	}
	return paths
}

// stage adds exactly the paths of want with pathspec magic and globbing off,
// then, after comparing the git state again (git add can run a
// post-index-change hook), checks that the index differs from HEAD by want
// alone and that no unstaged or untracked change remains. On a mismatch it
// undoes the staging. The caller has just compared the git state.
func (w *Worktree) stage(ctx context.Context, base Baseline, want []change) error {
	if _, err := w.git(ctx, "add", append([]string{"--literal-pathspecs", "add", "--"}, changePaths(want)...)...); err != nil {
		return w.undoStage(ctx, base, err)
	}
	if err := w.CheckState(ctx, base); err != nil {
		return err
	}
	if err := w.checkStaged(ctx, want); err != nil {
		return w.undoStage(ctx, base, err)
	}
	return nil
}

// checkStaged checks that the index differs from HEAD in exactly the paths of
// want and matches the work tree everywhere, with nothing untracked left.
func (w *Worktree) checkStaged(ctx context.Context, want []change) error {
	cached, err := w.git(ctx, "diff --cached", "diff", "--cached", "--name-only", "--no-renames", "-z", "HEAD", "--")
	if err != nil {
		return err
	}
	staged := splitNUL(cached)
	slices.Sort(staged)
	if !slices.Equal(staged, changePaths(want)) {
		return judgedSetChanged(want)
	}
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

// undoStage resets the index to HEAD, leaving the work tree as it is, after
// cause stopped the staging or the commit. Hooks may have run since the last
// comparison, so the git state is compared first; if the comparison fails
// (a change, or git failing to answer), no further git runs and the staging
// is left for the user to inspect.
func (w *Worktree) undoStage(ctx context.Context, base Baseline, cause error) error {
	if err := w.CheckState(ctx, base); err != nil {
		return errors.Join(err, cause)
	}
	_, err := w.git(ctx, "reset", "reset", "-q", "--")
	return errors.Join(cause, err)
}

// checkCommit checks the commit just made, first comparing the git state
// again since the commit ran hooks. The commit must be HEAD on the
// baseline's branch with the baseline's commit as its only parent, and its
// tree, read from the objects as stored, must differ from the baseline's tree
// in exactly want: each path with want's kind and mode and a blob whose
// content is the work tree's, still the content want was judged with. A hook
// that rewrote the index or the work tree, a filter that changed what was
// stored, and a replaced object all fail here, and no SHA is returned. When
// message is pinned, the stored message must also be its text; a commit whose
// message differs is not the commit finish made, as a wrong parent is not.
func (w *Worktree) checkCommit(ctx context.Context, base Baseline, want []change, message finishMessage) (string, error) {
	if err := w.CheckState(ctx, base); err != nil {
		return "", err
	}
	commit, branch, err := w.headState(ctx)
	if err != nil {
		return "", err
	}
	tree, parents, stored, err := w.commitObject(ctx, commit)
	if err != nil {
		return "", err
	}
	if branch == "" || digestString(branch) != base.branch || !slices.Equal(parents, []string{base.head}) ||
		(message.pinned && stored != message.text) {
		return "", &GitStateError{Parts: []string{PartHead}}
	}
	baseTree, _, _, err := w.commitObject(ctx, base.head)
	if err != nil {
		return "", err
	}
	out, err := w.git(ctx, "diff-tree", "diff-tree", "-r", "-z", "--raw", "--no-abbrev", "--no-renames", baseTree, tree, "--")
	if err != nil {
		return "", err
	}
	records, err := parseRawRecords(out)
	if err != nil {
		return "", err
	}
	if len(records) != len(want) {
		return "", judgedSetChanged(want)
	}
	byPath := make(map[string]rawRecord, len(records))
	for _, r := range records {
		byPath[r.path] = r
	}
	for _, c := range want {
		r, ok := byPath[c.path]
		if !ok || !committedAs(r, c) {
			return "", judgedSetChanged(want)
		}
	}
	return commit, nil
}

// committedAs reports whether a diff-tree record stores change c: a deletion
// for a deletion, and otherwise c's mode and a blob of the work tree's
// content, which must still have c's digest.
func committedAs(r rawRecord, c change) bool {
	if c.kind == ChangeDelete {
		return r.status == 'D'
	}
	if r.status == 'D' || r.newMode != c.mode {
		return false
	}
	blob, digest, err := blobOf(c.path, c.mode, len(r.newOID))
	return err == nil && digest == c.digest && blob == r.newOID
}

// blobOf returns the git blob ID (SHA-1 or SHA-256 by oidLength) of a work
// tree path as git would store it unfiltered, and the digest examine records
// for it: the content of a regular file, the target of a symbolic link.
func blobOf(p, mode string, oidLength int) (blob, digest string, err error) {
	var newHash func() hash.Hash
	switch oidLength {
	case sha1OIDLength:
		newHash = sha1.New
	case sha256OIDLength:
		newHash = sha256.New
	default:
		return "", "", fmt.Errorf("%w: object ID length", ErrGitFailed)
	}
	if mode == modeSymlink {
		target, err := os.Readlink(p)
		if err != nil {
			return "", "", err
		}
		return objectID(newHash(), []byte(target)), digestString(target), nil
	}
	f, err := openRegular(p, false)
	if err != nil {
		return "", "", err
	}
	defer func() { _ = f.Close() }()
	info, err := f.Stat()
	if err != nil {
		return "", "", err
	}
	object, content := newHash(), sha256.New()
	_, _ = fmt.Fprintf(object, "blob %d\x00", info.Size())
	n, err := io.Copy(io.MultiWriter(object, content), f)
	if err != nil || n != info.Size() {
		return "", "", fmt.Errorf("%w: read a committed path", ErrGitFailed)
	}
	return hex.EncodeToString(object.Sum(nil)), hex.EncodeToString(content.Sum(nil)), nil
}

// objectID hashes data as a git blob.
func objectID(h hash.Hash, data []byte) string {
	_, _ = fmt.Fprintf(h, "blob %d\x00", len(data))
	_, _ = h.Write(data)
	return hex.EncodeToString(h.Sum(nil))
}

// PushResult is the output of push.
type PushResult struct {
	Protocol int    `json:"protocol"`
	Pushed   bool   `json:"pushed"`
	Commit   string `json:"commit"`
}

// Push sends commit, the SHA finish returned, after checking right before it:
// the hooks, configuration and info match the baseline; HEAD is commit on the
// baseline's branch and its only parent is the baseline's commit; and the
// push target branch still has the baseline's OID. It pushes with an explicit
// refspec and without tags or submodules, so no configured refspec or push
// option decides what is sent. The check is of one moment; a process that
// rewrites the hooks between the check and the push is not stopped.
func (w *Worktree) Push(ctx context.Context, base Baseline, commit string) (PushResult, error) {
	if !validOID(commit) {
		return PushResult{}, fmt.Errorf("%w: commit", ErrInvalidInput)
	}
	target, err := w.compareState(ctx, base, commit, checkWithRemote)
	if err != nil {
		return PushResult{}, err
	}
	if _, parents, _, err := w.commitObject(ctx, commit); err != nil {
		return PushResult{}, err
	} else if !slices.Equal(parents, []string{base.head}) {
		return PushResult{}, &GitStateError{Parts: []string{PartHead}}
	}
	refspec := commit + ":" + headsPrefix + target.branch
	if _, err := w.git(ctx, "push", "push", "-q", "--no-follow-tags", "--recurse-submodules=no", "--", target.remote, refspec); err != nil {
		return PushResult{}, err
	}
	return PushResult{Protocol: Protocol, Pushed: true, Commit: commit}, nil
}
