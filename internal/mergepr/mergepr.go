// Package mergepr implements the mechanics behind the /mergepr command: it
// prepares a PR for message drafting, then squash-merges it and cleans up.
//
// It is an internal developer tool that trusts the local checkout and its
// configuration. Its only safety obligations are to merge exactly the head that
// was prepared (--match-head-commit), into the prepared base, after CI passed,
// and never to delete a local branch that holds commits outside the PR.
package mergepr

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	gitCommand = "git"
	ghCommand  = "gh"

	openState   = "OPEN"
	mergedState = "MERGED"

	stateFileName = "state.json"
	logFileName   = "log.txt"
	statFileName  = "stat.txt"
	bodyFileName  = "body.txt"

	workDirPrefix = "mergepr-"

	fileMode = 0o600

	prViewFields = "number,title,state,headRefName,headRefOid,baseRefName,url,body"
)

// Runner runs an external command without a shell and returns its standard
// output. Tests substitute a scripted fake; production uses NewOSRunner.
type Runner interface {
	Run(ctx context.Context, name string, args ...string) ([]byte, error)
}

// Tool implements the /mergepr mechanics against the repository in the current
// working directory.
type Tool struct {
	run Runner
}

// New returns a Tool that runs external commands through runner.
func New(runner Runner) *Tool {
	return &Tool{run: runner}
}

func (t *Tool) git(ctx context.Context, args ...string) ([]byte, error) {
	return t.run.Run(ctx, gitCommand, args...)
}

func (t *Tool) gh(ctx context.Context, args ...string) ([]byte, error) {
	return t.run.Run(ctx, ghCommand, args...)
}

// State pins the values Prepare saw, so Merge and Cleanup act on the same PR,
// head, and base. WorkDir is the absolute path of the directory Prepare
// created; Merge and Cleanup use it to confirm that the directory holding the
// state file is the one Prepare made before removing it.
type State struct {
	Number      int    `json:"number"`
	HeadRefName string `json:"headRefName"`
	HeadRefOID  string `json:"headRefOid"`
	BaseRefName string `json:"baseRefName"`
	Title       string `json:"title"`
	URL         string `json:"url"`
	WorkDir     string `json:"workDir"`
}

func (s State) number() string { return strconv.Itoa(s.Number) }

// Prepared is the result of Prepare: the pinned state and the directory holding
// state.json and the drafting material.
type Prepared struct {
	State State
	Dir   string
}

// Report describes what Merge or Cleanup did.
type Report struct {
	MergeCommitOID string
	BaseUpdated    bool
	LocalDeleted   bool
	// Note tells the operator what was left for them to do, if anything.
	Note string
}

// Prepare resolves the PR (the current branch's when prArg is empty), waits for
// CI, and writes the state and drafting material into a fresh directory inside
// the active worktree, so the material stays within the checkout rather than in
// a shared temporary directory.
func (t *Tool) Prepare(ctx context.Context, prArg string) (Prepared, error) {
	args := []string{"pr", "view"}
	if prArg != "" {
		args = append(args, prArg)
	}
	out, err := t.gh(ctx, append(args, "--json", prViewFields)...)
	if err != nil {
		return Prepared{}, fmt.Errorf("read PR: %w", err)
	}
	var pr struct {
		State
		PRState string `json:"state"`
		Body    string `json:"body"`
	}
	if err := json.Unmarshal(out, &pr); err != nil {
		return Prepared{}, fmt.Errorf("parse PR: %w", err)
	}
	if pr.PRState != openState {
		return Prepared{}, fmt.Errorf("%w: state is %s", errPRNotOpen, pr.PRState)
	}
	state := pr.State
	if _, err := t.git(ctx, "fetch", "origin"); err != nil {
		return Prepared{}, fmt.Errorf("fetch origin: %w", err)
	}
	if _, err := t.gh(ctx, "pr", "checks", state.number(), "--watch", "--fail-fast"); err != nil {
		return Prepared{}, fmt.Errorf("%w: %w", errChecksFailed, err)
	}
	baseRef := "origin/" + state.BaseRefName
	logOut, err := t.git(ctx, "log", "--format=%h %s%n%n%b", baseRef+".."+state.HeadRefOID)
	if err != nil {
		return Prepared{}, fmt.Errorf("read commit log: %w", err)
	}
	statOut, err := t.git(ctx, "diff", "--stat", baseRef+"..."+state.HeadRefOID)
	if err != nil {
		return Prepared{}, fmt.Errorf("read diff stat: %w", err)
	}
	gitDir, err := t.git(ctx, "rev-parse", "--absolute-git-dir")
	if err != nil {
		return Prepared{}, fmt.Errorf("find the git directory: %w", err)
	}
	workRoot, err := t.git(ctx, "rev-parse", "--show-toplevel")
	if err != nil {
		return Prepared{}, fmt.Errorf("find the worktree root: %w", err)
	}
	dir, err := os.MkdirTemp(workDirBase(outputLine(workRoot), outputLine(gitDir)), workDirPrefix)
	if err != nil {
		return Prepared{}, fmt.Errorf("create work directory: %w", err)
	}
	state.WorkDir = dir
	stateOut, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return Prepared{}, fmt.Errorf("encode state: %w", err)
	}
	files := map[string][]byte{
		stateFileName: append(stateOut, '\n'),
		logFileName:   logOut,
		statFileName:  statOut,
		bodyFileName:  []byte(pr.Body),
	}
	for name, data := range files {
		if err := os.WriteFile(filepath.Join(dir, name), data, fileMode); err != nil {
			return Prepared{}, fmt.Errorf("write %s: %w", name, err)
		}
	}
	return Prepared{State: state, Dir: dir}, nil
}

// workDirBase returns a directory inside the active worktree in which to create
// the drafting directory. In the primary checkout the git directory is itself
// inside the worktree, so use it and keep the material out of git's view. A
// linked worktree's git directory lives in the primary checkout, so fall back
// to the worktree root to stay inside the active checkout.
func workDirBase(workRoot, gitDir string) string {
	rel, err := filepath.Rel(workRoot, gitDir)
	if err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return gitDir
	}
	return workRoot
}

// outputLine returns a command's output with exactly one trailing newline
// removed. Git terminates each record with a single LF, so any carriage return
// before that LF is part of the value: a worktree path whose final byte is a
// carriage return is emitted as "<path>\r\n", and stripping the carriage return
// too would name a different, usually nonexistent root. Unlike strings.TrimSpace
// it keeps leading and trailing spaces, tabs, and any newline that is part of
// the value, so a worktree path that legitimately ends in whitespace is
// preserved.
func outputLine(out []byte) string {
	s := string(out)
	if trimmed, ok := strings.CutSuffix(s, "\n"); ok {
		return trimmed
	}
	return s
}

// Merge squash-merges the prepared head into the prepared base with the
// approved message, then cleans up.
func (t *Tool) Merge(ctx context.Context, statePath, subjectPath, bodyPath string) (Report, error) {
	state, err := loadState(statePath)
	if err != nil {
		return Report{}, err
	}
	subject, err := readSubject(subjectPath)
	if err != nil {
		return Report{}, err
	}
	body, err := os.ReadFile(bodyPath) //nolint:gosec // an operator-supplied path
	if err != nil {
		return Report{}, fmt.Errorf("read body file: %w", err)
	}
	live, err := t.viewPR(ctx, state)
	if err != nil {
		return Report{}, err
	}
	if live.State != openState {
		return Report{}, fmt.Errorf("%w: state is %s; if it is already merged, run `mergepr cleanup --state %s`", errPRNotOpen, live.State, statePath)
	}
	if live.BaseRefName != state.BaseRefName {
		return Report{}, fmt.Errorf("%w: base is %s, prepared for %s; re-run prepare", errBaseChanged, live.BaseRefName, state.BaseRefName)
	}
	// --match-head-commit makes GitHub refuse the merge if the head moved after
	// prepare, so the merged content is exactly what CI passed and the message
	// describes.
	if _, err := t.gh(ctx, "pr", "merge", state.number(), "--squash", "--subject", subject, "--body", string(body), "--match-head-commit", state.HeadRefOID); err != nil {
		return Report{}, fmt.Errorf("merge PR: %w\ncheck the PR; if it was merged anyway, run `mergepr cleanup --state %s`", err, statePath)
	}
	report, err := t.cleanup(ctx, state)
	if err != nil {
		return report, err
	}
	_ = removeWorkDir(state, statePath)
	return report, nil
}

// Cleanup updates the local base branch and deletes the local head branch after
// the PR is merged. The remote head branch is deleted by GitHub's "Automatically
// delete head branches" setting.
func (t *Tool) Cleanup(ctx context.Context, statePath string) (Report, error) {
	state, err := loadState(statePath)
	if err != nil {
		return Report{}, err
	}
	report, err := t.cleanup(ctx, state)
	if err != nil {
		return report, err
	}
	_ = removeWorkDir(state, statePath)
	return report, nil
}

// removeWorkDir deletes the prepared work directory. The directory is the unit
// the workflow owns: Prepare creates it, writes the material into it, and the
// operator drafts the subject and body files there, so it is removed whole
// rather than file by file. To keep an operator-supplied --state path from
// deleting an unrelated directory, it removes only the directory Prepare
// recorded in the state file, which is also the directory that still holds the
// state file; a state file that was moved or copied elsewhere therefore removes
// nothing. It returns errWorkDirMismatch in that case, so an explicit Discard
// can report it while Merge and Cleanup ignore a leftover directory after a
// cleanup that already succeeded.
func removeWorkDir(state State, statePath string) error {
	if state.WorkDir == "" {
		return errWorkDirMismatch
	}
	dir, err := filepath.Abs(filepath.Dir(statePath))
	if err != nil || dir != state.WorkDir {
		return errWorkDirMismatch
	}
	return os.RemoveAll(dir)
}

// Discard removes the prepared work directory without requiring the PR to be
// merged and without touching the PR or the local branches. Use it when a
// preparation can no longer be used, for example when the head moved before the
// merge, so its material does not linger inside the checkout. It applies the
// same provenance check as Merge and Cleanup: a state file that was moved or
// copied elsewhere names a different directory and removes nothing.
func Discard(statePath string) error {
	state, err := loadState(statePath)
	if err != nil {
		return err
	}
	return removeWorkDir(state, statePath)
}

func (t *Tool) cleanup(ctx context.Context, state State) (Report, error) {
	live, err := t.viewPR(ctx, state)
	if err != nil {
		return Report{}, err
	}
	if live.State != mergedState {
		return Report{}, fmt.Errorf("%w: state is %s; run cleanup again once it is merged", errPRNotMerged, live.State)
	}
	report := Report{MergeCommitOID: live.MergeCommit.OID}
	// A head force-pushed and merged after prepare means the local branch at
	// the prepared OID may hold commits the merge dropped, so touch nothing.
	if live.HeadRefOID != state.HeadRefOID {
		return report, fmt.Errorf("%w: merged %s, prepared %s; check the local branches yourself", errMergedHeadMoved, live.HeadRefOID, state.HeadRefOID)
	}
	if _, err := t.git(ctx, "fetch", "--prune", "origin"); err != nil {
		return report, fmt.Errorf("fetch origin: %w", err)
	}
	current, err := t.git(ctx, "branch", "--show-current")
	if err != nil {
		return report, fmt.Errorf("read current branch: %w", err)
	}
	if outputLine(current) != state.BaseRefName {
		elsewhere, err := t.checkedOutElsewhere(ctx, state.BaseRefName)
		if err != nil {
			return report, err
		}
		if elsewhere {
			report.Note = fmt.Sprintf("%s is checked out in another worktree; update it there and remove this worktree (or delete %s) yourself", state.BaseRefName, state.HeadRefName)
			return report, nil
		}
		if _, err := t.git(ctx, "switch", state.BaseRefName); err != nil {
			return report, fmt.Errorf("switch to %s: %w", state.BaseRefName, err)
		}
	}
	if _, err := t.git(ctx, "merge", "--ff-only", "origin/"+state.BaseRefName); err != nil {
		return report, fmt.Errorf("fast-forward %s: %w", state.BaseRefName, err)
	}
	report.BaseUpdated = true
	report.LocalDeleted, err = t.deleteLocalBranch(ctx, state)
	return report, err
}

// deleteLocalBranch deletes the local head branch only while it still points at
// the merged head, so commits made after prepare are never lost.
func (t *Tool) deleteLocalBranch(ctx context.Context, state State) (bool, error) {
	out, err := t.git(ctx, "for-each-ref", "--format=%(objectname)", "refs/heads/"+state.HeadRefName)
	if err != nil {
		return false, fmt.Errorf("read local branch %s: %w", state.HeadRefName, err)
	}
	local := outputLine(out)
	if local == "" {
		return false, nil
	}
	if local != state.HeadRefOID {
		return false, fmt.Errorf("%w: %s is at %s, merged %s; delete it yourself if that is intended", errLocalBranchMoved, state.HeadRefName, local, state.HeadRefOID)
	}
	if _, err := t.git(ctx, "branch", "-D", state.HeadRefName); err != nil {
		return false, fmt.Errorf("delete local branch %s: %w", state.HeadRefName, err)
	}
	return true, nil
}

// checkedOutElsewhere reports whether a worktree has branch checked out. The
// caller asks only when this worktree is on another branch.
func (t *Tool) checkedOutElsewhere(ctx context.Context, branch string) (bool, error) {
	out, err := t.git(ctx, "worktree", "list", "--porcelain")
	if err != nil {
		return false, fmt.Errorf("list worktrees: %w", err)
	}
	want := "branch refs/heads/" + branch
	for line := range strings.Lines(string(out)) {
		if strings.TrimSpace(line) == want {
			return true, nil
		}
	}
	return false, nil
}

type prView struct {
	State       string `json:"state"`
	BaseRefName string `json:"baseRefName"`
	HeadRefOID  string `json:"headRefOid"`
	MergeCommit struct {
		OID string `json:"oid"`
	} `json:"mergeCommit"`
}

func (t *Tool) viewPR(ctx context.Context, state State) (prView, error) {
	out, err := t.gh(ctx, "pr", "view", state.number(), "--json", "state,baseRefName,headRefOid,mergeCommit")
	if err != nil {
		return prView{}, fmt.Errorf("read PR: %w", err)
	}
	var view prView
	if err := json.Unmarshal(out, &view); err != nil {
		return prView{}, fmt.Errorf("parse PR: %w", err)
	}
	return view, nil
}

func loadState(path string) (State, error) {
	data, err := os.ReadFile(path) //nolint:gosec // an operator-supplied path
	if err != nil {
		return State{}, fmt.Errorf("read state: %w", err)
	}
	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		return State{}, fmt.Errorf("%w: %w", errInvalidState, err)
	}
	if state.Number <= 0 || state.HeadRefName == "" || state.HeadRefOID == "" || state.BaseRefName == "" {
		return State{}, fmt.Errorf("%w: %s is not a state file written by prepare", errInvalidState, path)
	}
	return state, nil
}

func readSubject(path string) (string, error) {
	data, err := os.ReadFile(path) //nolint:gosec // an operator-supplied path
	if err != nil {
		return "", fmt.Errorf("read subject file: %w", err)
	}
	subject := strings.TrimRight(string(data), "\n")
	if subject == "" || strings.ContainsAny(subject, "\r\n") {
		return "", errInvalidSubject
	}
	return subject, nil
}
