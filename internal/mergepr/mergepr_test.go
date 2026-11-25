//go:build test

package mergepr

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/isseis/yt2column/internal/numreserve"
)

const testPRJSON = `{"number":42,"title":"Test PR","state":"OPEN","headRefName":"feature/foo","headRefOid":"` + testHeadOID + `","baseRefName":"main","url":"https://github.com/isseis/yt2column/pull/42","body":"the PR description"}`

// fetchMainStep is the fetch of main into origin/main that comes right
// before each protocol-number check.
func fetchMainStep() commandStep {
	return gitStep("", "fetch", "origin", "+refs/heads/main:refs/remotes/origin/main")
}

func prepareTailSteps(workRoot, gitDir string) []commandStep {
	return []commandStep{
		gitStep("", "fetch", "origin"),
		fetchMainStep(),
		gitStep("abc subject\n\nbody\n", "log", "--format=%h %s%n%n%b", "origin/main.."+testHeadOID),
		gitStep(" a.txt | 1 +\n", "diff", "--stat", "origin/main..."+testHeadOID),
		gitStep(gitDir+"\n", "rev-parse", "--absolute-git-dir"),
		gitStep(workRoot+"\n", "rev-parse", "--show-toplevel"),
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

func TestPrepare(t *testing.T) {
	workRoot := t.TempDir()
	gitDir := filepath.Join(workRoot, ".git")
	if err := os.Mkdir(gitDir, 0o700); err != nil {
		t.Fatalf("create git dir: %v", err)
	}
	tool, runner := newTool(t, append([]commandStep{
		ghStep(testPRJSON, "pr", "view", "42", "--json", prViewFields),
	}, prepareTailSteps(workRoot, gitDir)...)...)

	prepared, err := tool.Prepare(t.Context(), "42")
	if err != nil {
		t.Fatalf("Prepare returned error: %v", err)
	}
	runner.done()
	t.Cleanup(func() { _ = os.RemoveAll(prepared.Dir) })

	if filepath.Dir(prepared.Dir) != gitDir {
		t.Errorf("prepared.Dir = %s, want a directory directly under the git directory %s", prepared.Dir, gitDir)
	}
	if prepared.State.WorkDir != prepared.Dir {
		t.Errorf("state.WorkDir = %q, want the prepared directory %q", prepared.State.WorkDir, prepared.Dir)
	}
	wantState := testState
	wantState.WorkDir = prepared.Dir
	if prepared.ProtocolCheck != testSummary.String() {
		t.Errorf("prepared.ProtocolCheck = %q, want the summary of the passed check %q", prepared.ProtocolCheck, testSummary.String())
	}
	if !reflect.DeepEqual(prepared.State, wantState) {
		t.Errorf("state = %+v, want %+v", prepared.State, wantState)
	}
	var saved State
	if err := json.Unmarshal([]byte(readFile(t, filepath.Join(prepared.Dir, stateFileName))), &saved); err != nil {
		t.Fatalf("parse state file: %v", err)
	}
	if !reflect.DeepEqual(saved, wantState) {
		t.Errorf("state file = %+v, want %+v", saved, wantState)
	}
	for name, want := range map[string]string{
		logFileName:    "abc subject\n\nbody\n",
		statFileName:   " a.txt | 1 +\n",
		bodyFileName:   "the PR description",
		markerFileName: markerContent,
		// The fake reading finds no correction line.
		correctionsFileName: "",
	} {
		if got := readFile(t, filepath.Join(prepared.Dir, name)); got != want {
			t.Errorf("%s = %q, want %q", name, got, want)
		}
	}
}

func TestPrepareCurrentBranch(t *testing.T) {
	workRoot := t.TempDir()
	gitDir := filepath.Join(workRoot, ".git")
	if err := os.Mkdir(gitDir, 0o700); err != nil {
		t.Fatalf("create git dir: %v", err)
	}
	tool, runner := newTool(t, append([]commandStep{
		ghStep(testPRJSON, "pr", "view", "--json", prViewFields),
	}, prepareTailSteps(workRoot, gitDir)...)...)

	prepared, err := tool.Prepare(t.Context(), "")
	if err != nil {
		t.Fatalf("Prepare returned error: %v", err)
	}
	runner.done()
	t.Cleanup(func() { _ = os.RemoveAll(prepared.Dir) })
}

// TestPrepareInLinkedWorktree verifies that the drafting material is created
// inside the active worktree even when the git directory lives in the primary
// checkout, as `git rev-parse --absolute-git-dir` reports for a linked worktree.
func TestPrepareInLinkedWorktree(t *testing.T) {
	workRoot := t.TempDir()
	gitDir := filepath.Join(t.TempDir(), "primary", ".git", "worktrees", "wt")
	if err := os.MkdirAll(gitDir, 0o700); err != nil {
		t.Fatalf("create git dir: %v", err)
	}
	tool, runner := newTool(t, append([]commandStep{
		ghStep(testPRJSON, "pr", "view", "42", "--json", prViewFields),
	}, prepareTailSteps(workRoot, gitDir)...)...)

	prepared, err := tool.Prepare(t.Context(), "42")
	if err != nil {
		t.Fatalf("Prepare returned error: %v", err)
	}
	runner.done()
	t.Cleanup(func() { _ = os.RemoveAll(prepared.Dir) })

	if filepath.Dir(prepared.Dir) != workRoot {
		t.Errorf("prepared.Dir = %s, want a directory inside the worktree %s", prepared.Dir, workRoot)
	}
}

// TestPreparePreservesTrailingWhitespaceInWorktreePath verifies that a worktree
// path whose last component ends in a space is used as git reports it. Trimming
// the reported path would name a sibling directory that does not exist.
func TestPreparePreservesTrailingWhitespaceInWorktreePath(t *testing.T) {
	workRoot := filepath.Join(t.TempDir(), "repo ")
	if err := os.Mkdir(workRoot, 0o700); err != nil {
		t.Fatalf("create worktree root: %v", err)
	}
	gitDir := filepath.Join(workRoot, ".git")
	if err := os.Mkdir(gitDir, 0o700); err != nil {
		t.Fatalf("create git dir: %v", err)
	}
	tool, runner := newTool(t, append([]commandStep{
		ghStep(testPRJSON, "pr", "view", "42", "--json", prViewFields),
	}, prepareTailSteps(workRoot, gitDir)...)...)

	prepared, err := tool.Prepare(t.Context(), "42")
	if err != nil {
		t.Fatalf("Prepare returned error: %v", err)
	}
	runner.done()
	t.Cleanup(func() { _ = os.RemoveAll(prepared.Dir) })

	if filepath.Dir(prepared.Dir) != gitDir {
		t.Errorf("prepared.Dir = %s, want a directory directly under the git directory %s", prepared.Dir, gitDir)
	}
}

// TestPrepareRejectsTrailingNewlineInWorktreePath verifies that a worktree path
// ending in a line break is rejected, because the line-oriented dir: and state:
// output records could not report it unambiguously. Git's single LF terminator
// is still stripped first, so the break is part of the value.
func TestPrepareRejectsTrailingNewlineInWorktreePath(t *testing.T) {
	workRoot := filepath.Join(t.TempDir(), "repo\n")
	if err := os.Mkdir(workRoot, 0o700); err != nil {
		t.Fatalf("create worktree root: %v", err)
	}
	gitDir := filepath.Join(workRoot, ".git")
	if err := os.Mkdir(gitDir, 0o700); err != nil {
		t.Fatalf("create git dir: %v", err)
	}
	tool, runner := newTool(t, append([]commandStep{
		ghStep(testPRJSON, "pr", "view", "42", "--json", prViewFields),
	}, prepareTailSteps(workRoot, gitDir)...)...)

	if _, err := tool.Prepare(t.Context(), "42"); !errors.Is(err, errUnprintablePath) {
		t.Fatalf("Prepare error = %v, want errUnprintablePath", err)
	}
	runner.done()
	entries, err := os.ReadDir(gitDir)
	if err != nil {
		t.Fatalf("read git dir: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("git dir holds %d entries after a rejected prepare, want none", len(entries))
	}
}

// TestPrepareRejectsTrailingCarriageReturnInWorktreePath verifies that a worktree path
// ending in a line break is rejected, because the line-oriented dir: and state:
// output records could not report it unambiguously. Git's single LF terminator
// is still stripped first, so the break is part of the value.
func TestPrepareRejectsTrailingCarriageReturnInWorktreePath(t *testing.T) {
	workRoot := filepath.Join(t.TempDir(), "repo\r")
	if err := os.Mkdir(workRoot, 0o700); err != nil {
		t.Fatalf("create worktree root: %v", err)
	}
	gitDir := filepath.Join(workRoot, ".git")
	if err := os.Mkdir(gitDir, 0o700); err != nil {
		t.Fatalf("create git dir: %v", err)
	}
	tool, runner := newTool(t, append([]commandStep{
		ghStep(testPRJSON, "pr", "view", "42", "--json", prViewFields),
	}, prepareTailSteps(workRoot, gitDir)...)...)

	if _, err := tool.Prepare(t.Context(), "42"); !errors.Is(err, errUnprintablePath) {
		t.Fatalf("Prepare error = %v, want errUnprintablePath", err)
	}
	runner.done()
	entries, err := os.ReadDir(gitDir)
	if err != nil {
		t.Fatalf("read git dir: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("git dir holds %d entries after a rejected prepare, want none", len(entries))
	}
}

// TestOutputLineStripsOneTerminator verifies that outputLine removes only the
// command's own record terminator, so a value that ends in whitespace or a
// carriage return survives.
func TestOutputLineStripsOneTerminator(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"lf", "main\n", "main"},
		{"no terminator", "main", "main"},
		{"trailing space", "repo \n", "repo "},
		{"value ends in newline", "repo\n\n", "repo\n"},
		{"carriage return is value data", "repo\r\n", "repo\r"},
		{"value ends in crlf", "repo\r\n\n", "repo\r\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := outputLine([]byte(tt.in)); got != tt.want {
				t.Errorf("outputLine(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestPrepareRejectsClosedPR(t *testing.T) {
	closed := `{"number":42,"state":"CLOSED","headRefName":"feature/foo","headRefOid":"` + testHeadOID + `","baseRefName":"main"}`
	tool, runner := newTool(t, ghStep(closed, "pr", "view", "42", "--json", prViewFields))

	if _, err := tool.Prepare(t.Context(), "42"); !errors.Is(err, errPRNotOpen) {
		t.Fatalf("Prepare error = %v, want errPRNotOpen", err)
	}
	runner.done()
}

var errCIRepo = errors.New("gh repo view failed")

// ciStops scripts the CI outcomes that must stop Prepare with errChecksFailed.
// wantCalls is the exact gh call sequence of the CI check, so a row that must
// not be retried (a permanent reason) fails if the wait polls again.
var ciStops = []struct {
	name      string
	repoErr   error
	replies   []ciReply
	wantCalls []string
	// wantInMessage is text the report must carry so the operator can find the
	// failing job; the wording is the observable contract here.
	wantInMessage string
}{
	{
		name:          "github-actions check failed",
		replies:       []ciReply{{out: ciResponse(testHeadOID, ciSuite("github-actions", "COMPLETED", "FAILURE", ciRun("test", "FAILURE")))}},
		wantCalls:     []string{"repo", "graphql"},
		wantInMessage: `"test" "FAILURE" https://github.com/isseis/yt2column/actions/runs/1/job/1`,
	},
	{
		name:          "head moved after prepare",
		replies:       []ciReply{{out: ciResponse(testOtherOID, ciSuite("github-actions", "COMPLETED", "SUCCESS", ciRun("test", "SUCCESS")))}},
		wantCalls:     []string{"repo", "graphql"},
		wantInMessage: "head-changed",
	},
	{
		name:          "response rejected",
		replies:       []ciReply{{out: "not json"}},
		wantCalls:     []string{"repo", "graphql"},
		wantInMessage: "rejected",
	},
	{
		name:      "repository not resolved",
		repoErr:   errCIRepo,
		wantCalls: []string{"repo"},
		// ghci reports a fixed message and never carries gh's own text.
		wantInMessage: "resolve repository",
	},
	{
		name:          "no github-actions suite appears",
		replies:       []ciReply{{out: ciResponse(testHeadOID)}},
		wantCalls:     nil, // the number of polls before the grace expires is not the contract
		wantInMessage: "deadline exceeded",
	},
}

func TestPrepareRejectsFailedChecks(t *testing.T) {
	for _, stop := range ciStops {
		t.Run(stop.name, func(t *testing.T) {
			tool, runner := newTool(t,
				ghStep(testPRJSON, "pr", "view", "42", "--json", prViewFields),
				gitStep("", "fetch", "origin"),
				fetchMainStep(),
			)
			ci := newFakeCI(t, stop.replies...)
			ci.repoErr = stop.repoErr
			tool.ci = ci

			_, err := tool.Prepare(t.Context(), "42")
			if !errors.Is(err, errChecksFailed) {
				t.Fatalf("Prepare error = %v, want errChecksFailed", err)
			}
			if !strings.Contains(err.Error(), stop.wantInMessage) {
				t.Errorf("Prepare error = %q, want it to contain %q", err, stop.wantInMessage)
			}
			if stop.wantCalls != nil && !slices.Equal(ci.calls, stop.wantCalls) {
				t.Errorf("CI calls = %v, want %v", ci.calls, stop.wantCalls)
			}
			runner.done()
		})
	}
}

// TestWaitChecksRefusesEmptyHead verifies that the head pin cannot be switched
// off by an empty head: no gh call is made.
func TestWaitChecksRefusesEmptyHead(t *testing.T) {
	tool, _ := newTool(t)
	ci := newFakeCI(t)
	tool.ci = ci
	state := testState
	state.HeadRefOID = ""
	if err := tool.waitChecks(t.Context(), state); !errors.Is(err, errChecksFailed) {
		t.Fatalf("waitChecks error = %v, want errChecksFailed", err)
	}
	if len(ci.calls) != 0 {
		t.Errorf("CI calls = %v, want none", ci.calls)
	}
}

// TestPrepareIgnoresReviewerChecks verifies that a failing check of a reviewer
// app (Sourcery's own failing check suite) does not stop Prepare while the
// GitHub Actions checks pass, and that a pending check is waited for.
func TestPrepareIgnoresReviewerChecks(t *testing.T) {
	workRoot := t.TempDir()
	gitDir := filepath.Join(workRoot, ".git")
	if err := os.Mkdir(gitDir, 0o700); err != nil {
		t.Fatalf("create git dir: %v", err)
	}
	reviewer := ciSuite("sourcery-ai", "COMPLETED", "FAILURE", ciRun("Sourcery review", "FAILURE"))
	pending := ciSuite("github-actions", "IN_PROGRESS", "", ciRun("test", "SUCCESS"))
	passed := ciSuite("github-actions", "COMPLETED", "SUCCESS", ciRun("test", "SUCCESS"))
	tool, runner := newTool(t, append([]commandStep{
		ghStep(testPRJSON, "pr", "view", "42", "--json", prViewFields),
	}, prepareTailSteps(workRoot, gitDir)...)...)
	ci := newFakeCI(t,
		ciReply{out: ciResponse(testHeadOID, reviewer, pending)},
		ciReply{out: ciResponse(testHeadOID, reviewer, passed)},
	)
	tool.ci = ci

	prepared, err := tool.Prepare(t.Context(), "42")
	if err != nil {
		t.Fatalf("Prepare returned error: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(prepared.Dir) })
	runner.done()
	if want := []string{"repo", "graphql", "graphql"}; !slices.Equal(ci.calls, want) {
		t.Errorf("CI calls = %v, want %v", ci.calls, want)
	}
}

func writeStateFile(t *testing.T, dir string) string {
	t.Helper()
	return writeStateFileRecording(t, dir, "")
}

func viewStep(state, base string) commandStep {
	return viewHeadStep(state, base, testHeadOID)
}

func viewHeadStep(state, base, head string) commandStep {
	common := `{"state":"` + state + `","baseRefName":"` + base + `","headRefOid":"` + head + `",`
	out := common + `"mergeCommit":null}`
	if state == mergedState {
		out = common + `"mergeCommit":{"oid":"` + testMergeOID + `"}}`
	}
	return ghStep(out, "pr", "view", "42", "--json", "state,baseRefName,headRefOid,mergeCommit")
}

func mergeStep() commandStep {
	return ghStep("", "pr", "merge", "42", "--squash", "--subject", "feat: subject (#42)", "--body-file", anyBodyFile, "--match-head-commit", testHeadOID)
}

// cleanupSteps is a cleanup run from the head branch in a checkout where no
// other worktree holds the base branch.
func cleanupSteps(localOID string) []commandStep {
	steps := []commandStep{
		viewStep(mergedState, "main"),
		gitStep("", "fetch", "--prune", "origin"),
		gitStep("feature/foo\n", "branch", "--show-current"),
		gitStep("worktree /repo\nHEAD "+testHeadOID+"\nbranch refs/heads/feature/foo\n\n", "worktree", "list", "--porcelain"),
		gitStep("", "switch", "main"),
		gitStep("", "merge", "--ff-only", "origin/main"),
		gitStep(localOID+"\n", "for-each-ref", "--format=%(objectname)", "refs/heads/feature/foo"),
	}
	if localOID == testHeadOID {
		steps = append(steps, gitStep("", "branch", "-D", "feature/foo"))
	}
	return steps
}

// mergeFiles prepares a real work directory, as Prepare does, holding the
// state file and the subject and body files the operator drafts there.
func mergeFiles(t *testing.T) (dir, statePath, subjectPath, bodyPath string) {
	t.Helper()
	dir, err := os.MkdirTemp(t.TempDir(), workDirPrefix)
	if err != nil {
		t.Fatalf("create work directory: %v", err)
	}
	return dir,
		writeStateFileRecording(t, dir, dir),
		writeTempFile(t, dir, "subject.txt", "feat: subject (#42)\n"),
		writeTempFile(t, dir, "body.txt", "body text\n")
}

func TestMerge(t *testing.T) {
	dir, statePath, subjectPath, bodyPath := mergeFiles(t)
	steps := []commandStep{viewStep(openState, "main"), fetchMainStep(), mergeStep()}
	tool, runner := newTool(t, append(steps, cleanupSteps(testHeadOID)...)...)

	report, err := tool.Merge(t.Context(), statePath, subjectPath, bodyPath, Drops{})
	if err != nil {
		t.Fatalf("Merge returned error: %v", err)
	}
	runner.done()
	want := Report{ProtocolCheck: testSummary.String(), MergeCommitOID: testMergeOID, BaseUpdated: true, LocalDeleted: true, Merged: true, CorrectionsChecked: true}
	if !reflect.DeepEqual(report, want) {
		t.Errorf("report = %+v, want %+v", report, want)
	}
	if _, err := os.Stat(dir); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("work directory %s still exists after merge, want it removed with the subject file it held", dir)
	}
}

func TestMergeRejectsLivePR(t *testing.T) {
	tests := []struct {
		name string
		view commandStep
		want error
	}{
		{"not open", viewStep(mergedState, "main"), errPRNotOpen},
		{"base changed", viewStep(openState, "develop"), errBaseChanged},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, statePath, subjectPath, bodyPath := mergeFiles(t)
			tool, runner := newTool(t, tt.view)

			if _, err := tool.Merge(t.Context(), statePath, subjectPath, bodyPath, Drops{}); !errors.Is(err, tt.want) {
				t.Fatalf("Merge error = %v, want %v", err, tt.want)
			}
			runner.done()
		})
	}
}

func TestMergeRejectsInvalidSubject(t *testing.T) {
	for _, subject := range []string{"", "\n", "line one\nline two\n"} {
		_, statePath, _, bodyPath := mergeFiles(t)
		subjectPath := writeTempFile(t, t.TempDir(), "subject.txt", subject)
		tool, runner := newTool(t)

		if _, err := tool.Merge(t.Context(), statePath, subjectPath, bodyPath, Drops{}); !errors.Is(err, errInvalidSubject) {
			t.Errorf("Merge(subject %q) error = %v, want errInvalidSubject", subject, err)
		}
		runner.done()
	}
}

func TestCleanupKeepsMovedLocalBranch(t *testing.T) {
	statePath := writeStateFile(t, t.TempDir())
	tool, runner := newTool(t, cleanupSteps(testOtherOID)...)

	report, err := tool.Cleanup(t.Context(), statePath, "")
	if !errors.Is(err, errLocalBranchMoved) {
		t.Fatalf("Cleanup error = %v, want errLocalBranchMoved", err)
	}
	runner.done()
	if report.LocalDeleted {
		t.Error("LocalDeleted = true, want false")
	}
}

func TestCleanupWithoutLocalBranch(t *testing.T) {
	statePath := writeStateFile(t, t.TempDir())
	tool, runner := newTool(t, cleanupSteps("")...)

	report, err := tool.Cleanup(t.Context(), statePath, "")
	if err != nil {
		t.Fatalf("Cleanup returned error: %v", err)
	}
	runner.done()
	want := Report{MergeCommitOID: testMergeOID, BaseUpdated: true, Merged: true}
	if !reflect.DeepEqual(report, want) {
		t.Errorf("report = %+v, want %+v", report, want)
	}
}

func TestCleanupRejectsUnmergedPR(t *testing.T) {
	statePath := writeStateFile(t, t.TempDir())
	tool, runner := newTool(t, viewStep(openState, "main"))

	if _, err := tool.Cleanup(t.Context(), statePath, ""); !errors.Is(err, errPRNotMerged) {
		t.Fatalf("Cleanup error = %v, want errPRNotMerged", err)
	}
	runner.done()
}

func TestCleanupRejectsMergedHeadMoved(t *testing.T) {
	statePath := writeStateFile(t, t.TempDir())
	tool, runner := newTool(t, viewHeadStep(mergedState, "main", testOtherOID))

	report, err := tool.Cleanup(t.Context(), statePath, "")
	if !errors.Is(err, errMergedHeadMoved) {
		t.Fatalf("Cleanup error = %v, want errMergedHeadMoved", err)
	}
	runner.done()
	if report.BaseUpdated || report.LocalDeleted {
		t.Errorf("report = %+v, want nothing changed locally", report)
	}
}

func TestCleanupOnBaseBranchSkipsSwitch(t *testing.T) {
	statePath := writeStateFile(t, t.TempDir())
	tool, runner := newTool(t,
		viewStep(mergedState, "main"),
		gitStep("", "fetch", "--prune", "origin"),
		gitStep("main\n", "branch", "--show-current"),
		gitStep("", "merge", "--ff-only", "origin/main"),
		gitStep(testHeadOID+"\n", "for-each-ref", "--format=%(objectname)", "refs/heads/feature/foo"),
		gitStep("", "branch", "-D", "feature/foo"),
	)

	if _, err := tool.Cleanup(t.Context(), statePath, ""); err != nil {
		t.Fatalf("Cleanup returned error: %v", err)
	}
	runner.done()
}

func TestCleanupLeavesBaseInAnotherWorktree(t *testing.T) {
	statePath := writeStateFile(t, t.TempDir())
	tool, runner := newTool(t,
		viewStep(mergedState, "main"),
		gitStep("", "fetch", "--prune", "origin"),
		gitStep("feature/foo\n", "branch", "--show-current"),
		gitStep("worktree /repo\nbranch refs/heads/main\n\nworktree /wt\nbranch refs/heads/feature/foo\n\n", "worktree", "list", "--porcelain"),
	)

	report, err := tool.Cleanup(t.Context(), statePath, "")
	if err != nil {
		t.Fatalf("Cleanup returned error: %v", err)
	}
	runner.done()
	if report.BaseUpdated || report.LocalDeleted || report.Note == "" {
		t.Errorf("report = %+v, want nothing changed locally and a note", report)
	}
}

func TestCleanupRemovesWorkDirectory(t *testing.T) {
	workDir, err := os.MkdirTemp(t.TempDir(), workDirPrefix)
	if err != nil {
		t.Fatalf("create work directory: %v", err)
	}
	statePath := writeStateFileRecording(t, workDir, workDir)
	// The operator drafts the subject and body into the prepared directory, so
	// the directory holds files Prepare did not write and must still go.
	writeTempFile(t, workDir, "subject.txt", "feat: subject (#42)\n")
	tool, runner := newTool(t, cleanupSteps(testHeadOID)...)

	if _, err := tool.Cleanup(t.Context(), statePath, ""); err != nil {
		t.Fatalf("Cleanup returned error: %v", err)
	}
	runner.done()
	if _, err := os.Stat(workDir); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("work directory %s still exists after cleanup, want removed", workDir)
	}
}

// TestCleanupKeepsUnrelatedFilesInWorkDirectory verifies that cleanup removes
// only the directory Prepare recorded for itself. A state file that was moved
// or copied into an unrelated directory that merely shares the work-dir prefix
// records a different work directory, so that directory and its files survive.
func TestCleanupKeepsUnrelatedFilesInWorkDirectory(t *testing.T) {
	workDir, err := os.MkdirTemp(t.TempDir(), workDirPrefix)
	if err != nil {
		t.Fatalf("create work directory: %v", err)
	}
	statePath := writeStateFileRecording(t, workDir, filepath.Join(t.TempDir(), workDirPrefix+"original"))
	unrelated := writeTempFile(t, workDir, "notes.md", "keep me")
	tool, runner := newTool(t, cleanupSteps(testHeadOID)...)

	if _, err := tool.Cleanup(t.Context(), statePath, ""); err != nil {
		t.Fatalf("Cleanup returned error: %v", err)
	}
	runner.done()
	if got := readFile(t, unrelated); got != "keep me" {
		t.Errorf("unrelated file = %q, want it left untouched", got)
	}
	if _, err := os.Stat(workDir); err != nil {
		t.Errorf("work directory %s removed while it held unrelated files: %v", workDir, err)
	}
}

func TestCleanupKeepsWorkDirectoryOnError(t *testing.T) {
	workDir, err := os.MkdirTemp(t.TempDir(), workDirPrefix)
	if err != nil {
		t.Fatalf("create work directory: %v", err)
	}
	statePath := writeStateFileRecording(t, workDir, workDir)
	tool, runner := newTool(t, cleanupSteps(testOtherOID)...)

	if _, err := tool.Cleanup(t.Context(), statePath, ""); !errors.Is(err, errLocalBranchMoved) {
		t.Fatalf("Cleanup error = %v, want errLocalBranchMoved", err)
	}
	runner.done()
	if _, err := os.Stat(workDir); err != nil {
		t.Errorf("work directory %s removed on a failed cleanup: %v", workDir, err)
	}
}

func TestDiscardRemovesWorkDirectory(t *testing.T) {
	workDir, err := os.MkdirTemp(t.TempDir(), workDirPrefix)
	if err != nil {
		t.Fatalf("create work directory: %v", err)
	}
	statePath := writeStateFileRecording(t, workDir, workDir)

	if err := Discard(statePath); err != nil {
		t.Fatalf("Discard returned error: %v", err)
	}
	if _, err := os.Stat(workDir); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("work directory %s still exists after discard, want removed", workDir)
	}
}

// TestDiscardKeepsUnrelatedDirectory verifies that Discard applies the same
// provenance check as cleanup: a state file that was moved or copied into an
// unrelated directory that merely shares the work-dir prefix records a
// different directory, so Discard refuses and that directory survives.
func TestDiscardKeepsUnrelatedDirectory(t *testing.T) {
	workDir, err := os.MkdirTemp(t.TempDir(), workDirPrefix)
	if err != nil {
		t.Fatalf("create work directory: %v", err)
	}
	statePath := writeStateFileRecording(t, workDir, filepath.Join(t.TempDir(), workDirPrefix+"original"))
	unrelated := writeTempFile(t, workDir, "notes.md", "keep me")

	if err := Discard(statePath); !errors.Is(err, errWorkDirMismatch) {
		t.Fatalf("Discard error = %v, want errWorkDirMismatch", err)
	}
	if got := readFile(t, unrelated); got != "keep me" {
		t.Errorf("unrelated file = %q, want it left untouched", got)
	}
	if _, err := os.Stat(workDir); err != nil {
		t.Errorf("work directory %s removed while it held unrelated files: %v", workDir, err)
	}
}

// TestDiscardKeepsForgedParentDirectory verifies that a state file whose
// workDir equals its own directory is still refused when that directory is not
// of the generated mergepr- shape, so a forged state cannot delete a checkout.
func TestDiscardKeepsForgedParentDirectory(t *testing.T) {
	repo := filepath.Join(t.TempDir(), "repo")
	if err := os.Mkdir(repo, 0o700); err != nil {
		t.Fatalf("create repo: %v", err)
	}
	statePath := writeStateFileRecording(t, repo, repo)
	keep := writeTempFile(t, repo, "main.go", "keep me")

	if err := Discard(statePath); !errors.Is(err, errWorkDirMismatch) {
		t.Fatalf("Discard error = %v, want errWorkDirMismatch", err)
	}
	if got := readFile(t, keep); got != "keep me" {
		t.Errorf("repo file = %q, want it left untouched", got)
	}
}

// TestDiscardKeepsForgedWorkDirectoryWithoutMarker verifies that a directory
// that shares the generated mergepr- shape is still refused when the marker
// file Prepare writes is missing or holds different content, so a forged state
// file cannot delete a same-shaped checkout or another valuable directory.
func TestDiscardKeepsForgedWorkDirectoryWithoutMarker(t *testing.T) {
	tests := []struct {
		name      string
		marker    string
		hasMarker bool
	}{
		{"missing marker", "", false},
		{"wrong marker", "not a mergepr work directory\n", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			workDir, err := os.MkdirTemp(t.TempDir(), workDirPrefix)
			if err != nil {
				t.Fatalf("create work directory: %v", err)
			}
			state := testState
			state.WorkDir = workDir
			data, err := json.Marshal(state)
			if err != nil {
				t.Fatalf("encode state: %v", err)
			}
			statePath := writeTempFile(t, workDir, stateFileName, string(data))
			keep := writeTempFile(t, workDir, "notes.md", "keep me")
			if tt.hasMarker {
				writeTempFile(t, workDir, markerFileName, tt.marker)
			}

			if err := Discard(statePath); !errors.Is(err, errWorkDirMismatch) {
				t.Fatalf("Discard error = %v, want errWorkDirMismatch", err)
			}
			if got := readFile(t, keep); got != "keep me" {
				t.Errorf("file = %q, want it left untouched", got)
			}
			if _, err := os.Stat(workDir); err != nil {
				t.Errorf("work directory %s removed without a valid marker: %v", workDir, err)
			}
		})
	}
}

func TestDiscardRejectsInvalidState(t *testing.T) {
	path := writeTempFile(t, t.TempDir(), "state.json", `{"number":42}`)
	if err := Discard(path); !errors.Is(err, errInvalidState) {
		t.Fatalf("Discard error = %v, want errInvalidState", err)
	}
}

func TestLoadStateRejectsIncompleteState(t *testing.T) {
	path := writeTempFile(t, t.TempDir(), "state.json", `{"number":42}`)
	if _, err := loadState(path); !errors.Is(err, errInvalidState) {
		t.Fatalf("loadState error = %v, want errInvalidState", err)
	}
}

// TestPrepareRemovesWorkDirOnWriteFailure verifies that a failed write after the
// work directory was allocated does not leave the directory behind.
func TestPrepareRemovesWorkDirOnWriteFailure(t *testing.T) {
	workRoot := t.TempDir()
	gitDir := filepath.Join(workRoot, ".git")
	if err := os.Mkdir(gitDir, 0o700); err != nil {
		t.Fatalf("create git dir: %v", err)
	}
	errDisk := errors.New("disk full")
	orig := writeFile
	t.Cleanup(func() { writeFile = orig })
	writeFile = func(string, []byte, os.FileMode) error { return errDisk }
	tool, runner := newTool(t, append([]commandStep{
		ghStep(testPRJSON, "pr", "view", "42", "--json", prViewFields),
	}, prepareTailSteps(workRoot, gitDir)...)...)

	_, err := tool.Prepare(t.Context(), "42")
	if !errors.Is(err, errDisk) {
		t.Fatalf("Prepare error = %v, want it to wrap %v", err, errDisk)
	}
	runner.done()
	entries, err := os.ReadDir(gitDir)
	if err != nil {
		t.Fatalf("read git dir: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("git dir still holds %d entries after a failed Prepare, want none", len(entries))
	}
}

// TestPrepareReportsFailedRemovalOfPartialWorkDir verifies that when a write
// fails and the removal of the partial work directory also fails, the returned
// error joins both failures and names the directory left behind, so the
// operator can locate and discard the partial preparation.
func TestPrepareReportsFailedRemovalOfPartialWorkDir(t *testing.T) {
	workRoot := t.TempDir()
	gitDir := filepath.Join(workRoot, ".git")
	if err := os.Mkdir(gitDir, 0o700); err != nil {
		t.Fatalf("create git dir: %v", err)
	}
	errDisk := errors.New("disk full")
	errRemove := errors.New("permission denied")
	origWrite, origRemove := writeFile, removeAllDir
	t.Cleanup(func() {
		writeFile = origWrite
		removeAllDir = origRemove
	})
	writeFile = func(string, []byte, os.FileMode) error { return errDisk }
	removeAllDir = func(string) error { return errRemove }
	tool, runner := newTool(t, append([]commandStep{
		ghStep(testPRJSON, "pr", "view", "42", "--json", prViewFields),
	}, prepareTailSteps(workRoot, gitDir)...)...)

	_, prepareErr := tool.Prepare(t.Context(), "42")
	if !errors.Is(prepareErr, errDisk) {
		t.Fatalf("Prepare error = %v, want it to wrap %v", prepareErr, errDisk)
	}
	if !errors.Is(prepareErr, errRemove) {
		t.Fatalf("Prepare error = %v, want it to wrap %v", prepareErr, errRemove)
	}
	runner.done()
	entries, err := os.ReadDir(gitDir)
	if err != nil {
		t.Fatalf("read git dir: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("git dir holds %d entries, want the one partial work directory left behind", len(entries))
	}
	if dir := filepath.Join(gitDir, entries[0].Name()); !strings.Contains(prepareErr.Error(), dir) {
		t.Errorf("Prepare error = %q, want it to name the left-behind directory %s", prepareErr, dir)
	}
}

// stopErrors are the two kinds of protocol-check stop: a number that is not
// origin/main's plus one, and something the check could not read.
var stopErrors = []struct {
	name string
	err  error
}{
	{"mismatch", &protocolMismatchError{MainOID: testOtherOID, HeadOID: testHeadOID}},
	{"unreadable", &protocolUnreadableError{Kind: unreadableHistory}},
}

// TestPrepareRunsProtocolCheck verifies that Prepare runs the protocol check
// right after fetching main, on the head and base gh reported, and that a
// stop happens before CI is awaited: the scripted runner fails the test on
// any later command, the git calls that locate the work directory included,
// so no work directory can be created.
func TestPrepareRunsProtocolCheck(t *testing.T) {
	const otherHead = "5555555555555555555555555555555555555555"
	pr := strings.ReplaceAll(strings.ReplaceAll(testPRJSON, testHeadOID, otherHead), `"baseRefName":"main"`, `"baseRefName":"release"`)
	lead := []commandStep{
		ghStep(pr, "pr", "view", "42", "--json", prViewFields),
		gitStep("", "fetch", "origin"),
		fetchMainStep(),
	}
	wantCall := []checkCall{{head: otherHead, base: "release", index: len(lead)}}

	t.Run("pass", func(t *testing.T) {
		workRoot := t.TempDir()
		gitDir := filepath.Join(workRoot, ".git")
		if err := os.Mkdir(gitDir, 0o700); err != nil {
			t.Fatalf("create git dir: %v", err)
		}
		tail := []commandStep{
			gitStep("", "log", "--format=%h %s%n%n%b", "origin/release.."+otherHead),
			gitStep("", "diff", "--stat", "origin/release..."+otherHead),
			gitStep(gitDir+"\n", "rev-parse", "--absolute-git-dir"),
			gitStep(workRoot+"\n", "rev-parse", "--show-toplevel"),
		}
		tool, runner, checker := newCheckedTool(t, nil, append(slices.Clone(lead), tail...)...)
		// The CI wait pins the head gh reported, which is the other head here.
		tool.ci = newFakeCI(t, ciReply{out: ciResponse(otherHead, ciSuite("github-actions", "COMPLETED", "SUCCESS", ciRun("test", "SUCCESS")))})
		checker.ci = tool.ci.(*fakeCI)
		prepared, err := tool.Prepare(t.Context(), "42")
		if err != nil {
			t.Fatalf("Prepare returned error: %v", err)
		}
		t.Cleanup(func() { _ = os.RemoveAll(prepared.Dir) })
		runner.done()
		if !slices.Equal(checker.calls, wantCall) {
			t.Errorf("check calls = %+v, want %+v", checker.calls, wantCall)
		}
	})
	for _, stop := range stopErrors {
		t.Run(stop.name, func(t *testing.T) {
			tool, runner, checker := newCheckedTool(t, stop.err, lead...)
			if _, err := tool.Prepare(t.Context(), "42"); !errors.Is(err, stop.err) {
				t.Fatalf("Prepare error = %v, want %v", err, stop.err)
			}
			if ci := tool.ci.(*fakeCI); len(ci.calls) != 0 {
				t.Errorf("CI calls after a protocol stop = %v, want none", ci.calls)
			}
			runner.done()
			if !slices.Equal(checker.calls, wantCall) {
				t.Errorf("check calls = %+v, want %+v", checker.calls, wantCall)
			}
		})
	}
}

// TestMergeRunsProtocolCheck verifies that Merge runs the protocol check
// after fetching main, on the head and base of the state file (the live PR
// reports another head, which must not be checked), and that a stop calls
// nothing after it, gh pr merge included.
func TestMergeRunsProtocolCheck(t *testing.T) {
	lead := []commandStep{viewHeadStep(openState, "main", testOtherOID), fetchMainStep()}
	wantCall := []checkCall{{head: testHeadOID, base: "main", index: len(lead)}}
	for _, stop := range stopErrors {
		t.Run(stop.name, func(t *testing.T) {
			dir, statePath, subjectPath, bodyPath := mergeFiles(t)
			tool, runner, checker := newCheckedTool(t, stop.err, lead...)
			report, err := tool.Merge(t.Context(), statePath, subjectPath, bodyPath, Drops{})
			if !errors.Is(err, stop.err) {
				t.Fatalf("Merge error = %v, want %v", err, stop.err)
			}
			runner.done()
			if !slices.Equal(checker.calls, wantCall) {
				t.Errorf("check calls = %+v, want %+v", checker.calls, wantCall)
			}
			if report != (Report{}) {
				t.Errorf("report = %+v, want empty after a stop", report)
			}
			if _, err := os.Stat(dir); err != nil {
				t.Errorf("work directory %s removed on a stopped merge, want it kept for discard: %v", dir, err)
			}
			// The command the operator is told to run is the contract: a
			// stop shows the discard command with this binary's --protocol.
			if !strings.Contains(err.Error(), discardCommand(statePath)) {
				t.Errorf("Merge error = %q, want it to show %q", err, discardCommand(statePath))
			}
		})
	}
}

// TestProtocolFetchFailureStops verifies that a failed fetch of main stops
// Prepare and Merge as unreadable without running the check or any later
// command.
func TestProtocolFetchFailureStops(t *testing.T) {
	failedFetch := fetchMainStep()
	failedFetch.err = errors.New("exit status 128")
	t.Run("prepare", func(t *testing.T) {
		tool, runner, checker := newCheckedTool(t, nil,
			ghStep(testPRJSON, "pr", "view", "42", "--json", prViewFields),
			gitStep("", "fetch", "origin"),
			failedFetch,
		)
		_, err := tool.Prepare(t.Context(), "42")
		assertFetchUnreadable(t, err)
		runner.done()
		if len(checker.calls) != 0 {
			t.Errorf("check calls = %+v, want none", checker.calls)
		}
	})
	t.Run("merge", func(t *testing.T) {
		_, statePath, subjectPath, bodyPath := mergeFiles(t)
		tool, runner, checker := newCheckedTool(t, nil, viewStep(openState, "main"), failedFetch)
		_, err := tool.Merge(t.Context(), statePath, subjectPath, bodyPath, Drops{})
		assertFetchUnreadable(t, err)
		// A failed fetch leaves the preparation usable, so the operator is
		// not told to discard it.
		if strings.Contains(err.Error(), discardCommand(statePath)) {
			t.Errorf("Merge error = %q, want no discard command after a failed fetch", err)
		}
		runner.done()
		if len(checker.calls) != 0 {
			t.Errorf("check calls = %+v, want none", checker.calls)
		}
	})
}

func assertFetchUnreadable(t *testing.T, err error) {
	t.Helper()
	unreadable, ok := errors.AsType[*protocolUnreadableError](err)
	if !ok {
		t.Fatalf("error = %v, want a protocolUnreadableError", err)
	}
	if unreadable.Kind != unreadableFetch {
		t.Errorf("kind = %v, want unreadableFetch", unreadable.Kind)
	}
}

// TestMergeKeepsSummaryWhenMergeFails verifies that a merge refused after the
// protocol check passed still reports the check's summary.
func TestMergeKeepsSummaryWhenMergeFails(t *testing.T) {
	_, statePath, subjectPath, bodyPath := mergeFiles(t)
	refused := mergeStep()
	refused.err = errors.New("exit status 1")
	tool, runner := newTool(t, viewStep(openState, "main"), fetchMainStep(), refused)
	report, err := tool.Merge(t.Context(), statePath, subjectPath, bodyPath, Drops{})
	if err == nil {
		t.Fatal("Merge returned nil, want the refused merge's error")
	}
	runner.done()
	if report.ProtocolCheck != testSummary.String() {
		t.Errorf("report.ProtocolCheck = %q, want %q", report.ProtocolCheck, testSummary.String())
	}
}

// numberStops are the two kinds of number-check stop: a finding on a number
// the PR adds, and a check that could not read its inputs.
var numberStops = []struct {
	name   string
	issues []numreserve.AddedNumberIssue
	err    error
}{
	{"finding", []numreserve.AddedNumberIssue{{Space: numreserve.SpaceTask, Kind: numreserve.AddedUnreserved, Path: "docs/tasks/0104_x"}}, nil},
	{"unreadable", nil, &numreserve.StopError{Kind: numreserve.StopMalformed, Step: "listing", Detail: "line \"x\" is not <40 hex digits><tab><ref>"}},
	{"unreachable", nil, &numreserve.StopError{Kind: numreserve.StopNetworkOrGitHub, Step: "listing", Detail: "git exited with status 128"}},
}

// assertNumberStop checks err is the stop numberStops row stop describes.
func assertNumberStop(t *testing.T, err error, issues []numreserve.AddedNumberIssue) {
	t.Helper()
	if issues == nil {
		if !errors.Is(err, errNumberUnreadable) {
			t.Fatalf("error = %v, want errNumberUnreadable", err)
		}
		return
	}
	found, ok := errors.AsType[*NumberIssuesError](err)
	if !ok {
		t.Fatalf("error = %v, want a *NumberIssuesError", err)
	}
	if !slices.Equal(found.Issues, issues) {
		t.Errorf("issues = %+v, want %+v", found.Issues, issues)
	}
}

// TestPrepareNumberCheck verifies that Prepare runs the number check after
// the CI wait, on the head and head branch gh reported, and that a stop
// happens before the work directory is located: the scripted runner fails
// the test on any later command. A PR into another base is not checked.
func TestPrepareNumberCheck(t *testing.T) {
	lead := []commandStep{
		ghStep(testPRJSON, "pr", "view", "42", "--json", prViewFields),
		gitStep("", "fetch", "origin"),
		fetchMainStep(),
	}
	// The CI check made its two gh calls (repo view and one query) before the
	// number check ran.
	wantCall := []numberCall{{head: testHeadOID, branch: "feature/foo", index: len(lead), ciCalls: 2}}
	t.Run("pass", func(t *testing.T) {
		workRoot := t.TempDir()
		gitDir := filepath.Join(workRoot, ".git")
		if err := os.Mkdir(gitDir, 0o700); err != nil {
			t.Fatalf("create git dir: %v", err)
		}
		tool, runner, numbers := newNumberedTool(t, nil, nil, append([]commandStep{lead[0]}, prepareTailSteps(workRoot, gitDir)...)...)
		prepared, err := tool.Prepare(t.Context(), "42")
		if err != nil {
			t.Fatalf("Prepare returned error: %v", err)
		}
		t.Cleanup(func() { _ = os.RemoveAll(prepared.Dir) })
		runner.done()
		if !slices.Equal(numbers.calls, wantCall) {
			t.Errorf("number check calls = %+v, want %+v", numbers.calls, wantCall)
		}
	})
	// Prepare does not tell one unreadable stop from another, so the
	// network row that only Merge treats differently is left out.
	for _, stop := range numberStops[:2] {
		t.Run(stop.name, func(t *testing.T) {
			tool, runner, numbers := newNumberedTool(t, stop.issues, stop.err, lead...)
			_, err := tool.Prepare(t.Context(), "42")
			assertNumberStop(t, err, stop.issues)
			runner.done()
			if !slices.Equal(numbers.calls, wantCall) {
				t.Errorf("number check calls = %+v, want %+v", numbers.calls, wantCall)
			}
		})
	}
	// Prepare reads the base from gh and Merge from the state file, so each
	// entry point has its own base-not-main row.
	t.Run("base not main", func(t *testing.T) {
		workRoot := t.TempDir()
		gitDir := filepath.Join(workRoot, ".git")
		if err := os.Mkdir(gitDir, 0o700); err != nil {
			t.Fatalf("create git dir: %v", err)
		}
		pr := strings.Replace(testPRJSON, `"baseRefName":"main"`, `"baseRefName":"release"`, 1)
		steps := []commandStep{
			ghStep(pr, "pr", "view", "42", "--json", prViewFields),
			gitStep("", "fetch", "origin"),
			fetchMainStep(),
			gitStep("", "log", "--format=%h %s%n%n%b", "origin/release.."+testHeadOID),
			gitStep("", "diff", "--stat", "origin/release..."+testHeadOID),
			gitStep(gitDir+"\n", "rev-parse", "--absolute-git-dir"),
			gitStep(workRoot+"\n", "rev-parse", "--show-toplevel"),
		}
		tool, runner, numbers := newNumberedTool(t, numberStops[0].issues, nil, steps...)
		prepared, err := tool.Prepare(t.Context(), "42")
		if err != nil {
			t.Fatalf("Prepare returned error: %v", err)
		}
		t.Cleanup(func() { _ = os.RemoveAll(prepared.Dir) })
		runner.done()
		if len(numbers.calls) != 0 {
			t.Errorf("number check calls = %+v, want none for a PR into release", numbers.calls)
		}
	})
}

// TestMergeNumberCheck verifies that Merge runs the number check after the
// protocol check and before gh pr merge, on the head and head branch of the
// state file (the live PR reports another head, which must not be checked),
// and that a stop runs nothing after it, keeps the work directory, and shows
// the discard command.
func TestMergeNumberCheck(t *testing.T) {
	lead := []commandStep{viewHeadStep(openState, "main", testOtherOID), fetchMainStep()}
	wantCall := []numberCall{{head: testHeadOID, branch: "feature/foo", index: len(lead)}}
	t.Run("pass", func(t *testing.T) {
		_, statePath, subjectPath, bodyPath := mergeFiles(t)
		tool, runner, numbers := newNumberedTool(t, nil, nil, append(append(slices.Clone(lead), mergeStep()), cleanupSteps(testHeadOID)...)...)
		if _, err := tool.Merge(t.Context(), statePath, subjectPath, bodyPath, Drops{}); err != nil {
			t.Fatalf("Merge returned error: %v", err)
		}
		runner.done()
		if !slices.Equal(numbers.calls, wantCall) {
			t.Errorf("number check calls = %+v, want %+v", numbers.calls, wantCall)
		}
	})
	for _, stop := range numberStops {
		t.Run(stop.name, func(t *testing.T) {
			dir, statePath, subjectPath, bodyPath := mergeFiles(t)
			tool, runner, numbers := newNumberedTool(t, stop.issues, stop.err, lead...)
			_, err := tool.Merge(t.Context(), statePath, subjectPath, bodyPath, Drops{})
			assertNumberStop(t, err, stop.issues)
			runner.done()
			if !slices.Equal(numbers.calls, wantCall) {
				t.Errorf("number check calls = %+v, want %+v", numbers.calls, wantCall)
			}
			if _, err := os.Stat(dir); err != nil {
				t.Errorf("work directory %s removed on a stopped merge, want it kept for discard: %v", dir, err)
			}
			// The command the operator is told to run is the contract: a
			// stop that waiting cannot fix shows the discard command, and a
			// network stop keeps the preparation for the same merge again.
			wantDiscard := stop.name != "unreachable"
			if got := strings.Contains(err.Error(), discardCommand(statePath)); got != wantDiscard {
				t.Errorf("Merge error = %q, shows %q: %t, want %t", err, discardCommand(statePath), got, wantDiscard)
			}
			if got := strings.Contains(err.Error(), "run the same merge command again"); got == wantDiscard {
				t.Errorf("Merge error = %q, tells to run the same merge again: %t, want %t", err, got, !wantDiscard)
			}
		})
	}
	t.Run("base not main", func(t *testing.T) {
		dir := t.TempDir()
		state := testState
		state.BaseRefName = "release"
		data, err := json.Marshal(state)
		if err != nil {
			t.Fatal(err)
		}
		statePath := writeTempFile(t, dir, stateFileName, string(data))
		subjectPath := writeTempFile(t, dir, "subject.txt", "feat: subject (#42)\n")
		bodyPath := writeTempFile(t, dir, "body.txt", "body text\n")
		refused := mergeStep()
		refused.err = errors.New("exit status 1")
		tool, runner, numbers := newNumberedTool(t, numberStops[0].issues, nil, viewStep(openState, "release"), fetchMainStep(), refused)
		if _, err := tool.Merge(t.Context(), statePath, subjectPath, bodyPath, Drops{}); !errors.Is(err, refused.err) {
			t.Fatalf("Merge error = %v, want the refused merge's error", err)
		}
		runner.done()
		if len(numbers.calls) != 0 {
			t.Errorf("number check calls = %+v, want none for a PR into release", numbers.calls)
		}
	})
}

// testCommitA and testCommitB are commits of the fake correction reading.
const (
	testCommitA = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	testCommitB = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
)

// testCorrection and testOtherCorrection are accepted correction lines.
const (
	testCorrection      = "Editorial correction: docs/dev/adr/0001-x.md: fixed a link; no decision changed"
	testOtherCorrection = "Editorial correction: docs/tasks/0001_x/02_architecture.md: renamed a step; no decision changed"
)

// correctedBody is a body that carries testCorrection as its block, between
// the summary and the trailers, as the command documents build it.
const correctedBody = "Summary line.\n\n" + testCorrection + "\n\nCo-Authored-By: Someone <someone@example.com>\n"

// sourceOf returns the fake correction reading of a Tool newCheckedTool made.
func sourceOf(t *testing.T, tool *Tool) *fakeCorrectionSource {
	t.Helper()
	source, ok := tool.corrections.(*fakeCorrectionSource)
	if !ok {
		t.Fatalf("corrections = %T, want the fake", tool.corrections)
	}
	return source
}

// TestPrepareWritesCorrections verifies that Prepare reads the correction
// lines of the range on the head and base gh reported, after the diff stat
// and before the work directory is located, writes the accepted lines (each
// byte sequence once) to corrections.txt, returns the rejected lines without
// stopping, and that a range it cannot read stops it with nothing created.
func TestPrepareWritesCorrections(t *testing.T) {
	newGitDir := func(t *testing.T) (string, string) {
		workRoot := t.TempDir()
		gitDir := filepath.Join(workRoot, ".git")
		if err := os.Mkdir(gitDir, 0o700); err != nil {
			t.Fatalf("create git dir: %v", err)
		}
		return workRoot, gitDir
	}
	// The reading runs after the PR view and the first four tail steps (the CI
	// wait is not a step of this runner).
	const readAt = 5
	t.Run("lines", func(t *testing.T) {
		workRoot, gitDir := newGitDir(t)
		tool, runner := newTool(t, append([]commandStep{
			ghStep(testPRJSON, "pr", "view", "42", "--json", prViewFields),
		}, prepareTailSteps(workRoot, gitDir)...)...)
		source := sourceOf(t, tool)
		rejected := "Editorial correction: mail me@example.com"
		source.lines = []correctionLine{
			{Commit: testCommitA, Line: 3, Text: []byte(testCorrection)},
			{Commit: testCommitA, Line: 4, Text: []byte(rejected)},
			{Commit: testCommitB, Line: 1, Text: []byte(testCorrection)},
			{Commit: testCommitB, Line: 5, Text: []byte(testOtherCorrection)},
		}
		prepared, err := tool.Prepare(t.Context(), "42")
		if err != nil {
			t.Fatalf("Prepare returned error: %v", err)
		}
		t.Cleanup(func() { _ = os.RemoveAll(prepared.Dir) })
		runner.done()
		if want := []sourceCall{{method: "rangeLines", index: readAt}}; !slices.Equal(source.calls, want) {
			t.Errorf("reading calls = %+v, want %+v", source.calls, want)
		}
		if source.gotBase != "main" || source.gotHead != testHeadOID {
			t.Errorf("reading base, head = %q, %q, want main, %s", source.gotBase, source.gotHead, testHeadOID)
		}
		if got, want := readFile(t, filepath.Join(prepared.Dir, correctionsFileName)), testCorrection+"\n"+testOtherCorrection+"\n"; got != want {
			t.Errorf("corrections.txt = %q, want %q", got, want)
		}
		if prepared.Accepted != 2 {
			t.Errorf("Accepted = %d, want 2", prepared.Accepted)
		}
		wantRejected := []RejectedLine{{Commit: testCommitA, Line: 4, Kinds: []RejectKind{RejectAt}, Shown: `"` + rejected + `"`}}
		if !reflect.DeepEqual(prepared.Rejected, wantRejected) {
			t.Errorf("Rejected = %+v, want %+v", prepared.Rejected, wantRejected)
		}
	})
	t.Run("unreadable", func(t *testing.T) {
		workRoot, gitDir := newGitDir(t)
		tail := prepareTailSteps(workRoot, gitDir)
		// The steps that locate the work directory must not run.
		tool, runner := newTool(t, append([]commandStep{
			ghStep(testPRJSON, "pr", "view", "42", "--json", prViewFields),
		}, tail[:readAt-1]...)...)
		source := sourceOf(t, tool)
		source.err = fmt.Errorf("%w: cat-file failed", errCorrectionUnreadable)
		if _, err := tool.Prepare(t.Context(), "42"); !errors.Is(err, errCorrectionUnreadable) {
			t.Fatalf("Prepare error = %v, want errCorrectionUnreadable", err)
		}
		runner.done()
		entries, err := os.ReadDir(gitDir)
		if err != nil {
			t.Fatalf("read git dir: %v", err)
		}
		if len(entries) != 0 {
			t.Errorf("git dir holds %d entries after an unreadable range, want none", len(entries))
		}
	})
}

// TestParagraph verifies that Paragraph writes the block with the drops
// applied into the work directory, without running any command, and that a
// rejected line no drop declares stops it before it writes.
func TestParagraph(t *testing.T) {
	lines := []correctionLine{
		{Commit: testCommitA, Line: 3, Text: []byte(testCorrection)},
		{Commit: testCommitA, Line: 4, Text: []byte("Editorial correction: <tag>")},
		{Commit: testCommitB, Line: 2, Text: []byte(testOtherCorrection)},
	}
	tests := []struct {
		name      string
		drops     Drops
		want      string
		wantLines int
		wantErr   error
	}{
		{
			name:      "drops applied",
			drops:     Drops{Accepted: []Drop{{Index: 2, Case: DropReverted}}, Rejected: []RejectedDrop{{Commit: testCommitA, Line: 4}}},
			want:      testCorrection + "\nDropped editorial corrections: 0 disputed by a reviewer, 1 reverted on the branch, 1 with rejected characters\n\n",
			wantLines: 2,
		},
		{
			name:    "undeclared rejected line",
			drops:   Drops{},
			wantErr: errCorrectionMismatch,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir, statePath, _, _ := mergeFiles(t)
			tool, runner := newTool(t)
			source := sourceOf(t, tool)
			source.lines = lines
			path, n, err := tool.Paragraph(t.Context(), statePath, tt.drops)
			runner.done()
			if source.gotBase != "main" || source.gotHead != testHeadOID {
				t.Errorf("range read on base %q, head %q, want main, %s from the state file", source.gotBase, source.gotHead, testHeadOID)
			}
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("Paragraph error = %v, want %v", err, tt.wantErr)
				}
				if _, err := os.Stat(filepath.Join(dir, paragraphFileName)); !errors.Is(err, os.ErrNotExist) {
					t.Errorf("paragraph.txt exists after a stop: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Paragraph returned error: %v", err)
			}
			if path != filepath.Join(dir, paragraphFileName) {
				t.Errorf("path = %s, want paragraph.txt in %s", path, dir)
			}
			if n != tt.wantLines {
				t.Errorf("lines = %d, want %d", n, tt.wantLines)
			}
			if got := readFile(t, path); got != tt.want {
				t.Errorf("paragraph.txt = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestParagraphKeepsForgedWorkDirectory verifies that Paragraph writes only
// into the directory Prepare made and marked: a state file in a directory
// without the generated shape, without the marker, or that records another
// directory gets errWorkDirMismatch, and no paragraph.txt appears.
func TestParagraphKeepsForgedWorkDirectory(t *testing.T) {
	tests := []struct {
		name  string
		setup func(t *testing.T) (dir, statePath string)
	}{
		{"forged parent", func(t *testing.T) (string, string) {
			repo := filepath.Join(t.TempDir(), "repo")
			if err := os.Mkdir(repo, 0o700); err != nil {
				t.Fatal(err)
			}
			return repo, writeStateFileRecording(t, repo, repo)
		}},
		{"no marker", func(t *testing.T) (string, string) {
			dir, err := os.MkdirTemp(t.TempDir(), workDirPrefix)
			if err != nil {
				t.Fatal(err)
			}
			statePath := writeStateFileRecording(t, dir, dir)
			if err := os.Remove(filepath.Join(dir, markerFileName)); err != nil {
				t.Fatal(err)
			}
			return dir, statePath
		}},
		{"unrelated directory", func(t *testing.T) (string, string) {
			dir, err := os.MkdirTemp(t.TempDir(), workDirPrefix)
			if err != nil {
				t.Fatal(err)
			}
			return dir, writeStateFileRecording(t, dir, filepath.Join(t.TempDir(), workDirPrefix+"original"))
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir, statePath := tt.setup(t)
			tool, runner := newTool(t)
			sourceOf(t, tool).lines = []correctionLine{{Commit: testCommitA, Line: 3, Text: []byte(testCorrection)}}
			if _, _, err := tool.Paragraph(t.Context(), statePath, Drops{}); !errors.Is(err, errWorkDirMismatch) {
				t.Fatalf("Paragraph error = %v, want errWorkDirMismatch", err)
			}
			runner.done()
			if _, err := os.Stat(filepath.Join(dir, paragraphFileName)); !errors.Is(err, os.ErrNotExist) {
				t.Errorf("paragraph.txt written into %s: %v", dir, err)
			}
		})
	}
}

// TestCheckBody verifies that CheckBody passes a body that carries the
// block, stops one that does not, and runs no command either way.
func TestCheckBody(t *testing.T) {
	tests := []struct {
		name      string
		subject   string
		body      string
		drops     Drops
		wantLines int
		wantErr   error
	}{
		{"block carried", "feat: subject (#42)\n", correctedBody, Drops{}, 1, nil},
		{"line dropped", "feat: subject (#42)\n", "Summary line.\n\nDropped editorial corrections: 1 disputed by a reviewer, 0 reverted on the branch, 0 with rejected characters\n", Drops{Accepted: []Drop{{Index: 1, Case: DropDisputed}}}, 1, nil},
		// Only the subject file's wiring can stop this row: the body is right.
		{"correction line in the subject", testCorrection + "\n", correctedBody, Drops{}, 0, errCorrectionMismatch},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir, statePath, _, _ := mergeFiles(t)
			subjectPath := writeTempFile(t, dir, "checked-subject.txt", tt.subject)
			bodyPath := writeTempFile(t, dir, "checked.txt", tt.body)
			tool, runner := newTool(t)
			source := sourceOf(t, tool)
			source.lines = []correctionLine{{Commit: testCommitA, Line: 3, Text: []byte(testCorrection)}}
			n, err := tool.CheckBody(t.Context(), statePath, subjectPath, bodyPath, tt.drops)
			runner.done()
			if source.gotBase != "main" || source.gotHead != testHeadOID {
				t.Errorf("range read on base %q, head %q, want main, %s from the state file", source.gotBase, source.gotHead, testHeadOID)
			}
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("CheckBody error = %v, want %v", err, tt.wantErr)
			}
			if n != tt.wantLines {
				t.Errorf("lines = %d, want %d", n, tt.wantLines)
			}
		})
	}
}

// bodyFileSeen is what the gh pr merge hook saw of the file it was given.
type bodyFileSeen struct {
	path    string
	content string
	mode    os.FileMode
}

// seeBodyFile returns a hook that records the --body-file the merge passes,
// then rewrites the caller's body file at rewrite (when not empty), so a test
// can tell the checked bytes from the file's later content.
func seeBodyFile(t *testing.T, seen *bodyFileSeen, rewrite string) func([]string) {
	return func(args []string) {
		i := slices.Index(args, "--body-file")
		if i < 0 || i+1 >= len(args) {
			t.Errorf("gh pr merge args %q have no --body-file value", args)
			return
		}
		seen.path = args[i+1]
		info, err := os.Stat(seen.path)
		if err != nil {
			t.Errorf("stat the body file: %v", err)
			return
		}
		seen.mode = info.Mode().Perm()
		seen.content = readFile(t, seen.path)
		if rewrite != "" {
			writeTempFile(t, filepath.Dir(rewrite), filepath.Base(rewrite), "Summary line.\n\n"+testOtherCorrection+"\n")
		}
	}
}

// TestMergePassesCheckedBody verifies that Merge hands gh the body it
// checked (not the file as rewritten during the check) through a file of mode
// 0600 outside the work directory, and that the file is gone when Merge
// returns, after a merge and after a refused one.
func TestMergePassesCheckedBody(t *testing.T) {
	for _, refused := range []bool{false, true} {
		t.Run(fmt.Sprintf("refused=%t", refused), func(t *testing.T) {
			dir, statePath, subjectPath, bodyPath := mergeFiles(t)
			writeTempFile(t, dir, filepath.Base(bodyPath), correctedBody)
			var seen bodyFileSeen
			merge := mergeStep()
			merge.hook = seeBodyFile(t, &seen, "")
			steps := []commandStep{viewStep(openState, "main"), fetchMainStep(), merge}
			if refused {
				steps[2].err = errors.New("exit status 1")
			} else {
				steps = append(steps, cleanupSteps(testHeadOID)...)
			}
			tool, runner := newTool(t, steps...)
			source := sourceOf(t, tool)
			source.lines = []correctionLine{{Commit: testCommitA, Line: 3, Text: []byte(testCorrection)}}
			source.merged = [][]byte{[]byte(testCorrection)}
			// A body file rewritten while the check reads the range must not
			// reach GitHub: the merge sends the bytes it read before.
			source.onRange = func() { writeTempFile(t, dir, filepath.Base(bodyPath), "Summary line.\n") }
			_, err := tool.Merge(t.Context(), statePath, subjectPath, bodyPath, Drops{})
			if (err != nil) != refused {
				t.Fatalf("Merge error = %v, want an error %t", err, refused)
			}
			runner.done()
			if seen.content != correctedBody {
				t.Errorf("body file held %q, want the checked body %q", seen.content, correctedBody)
			}
			if seen.mode != 0o600 {
				t.Errorf("body file mode = %v, want 0600", seen.mode)
			}
			if rel, err := filepath.Rel(dir, seen.path); err == nil && !strings.HasPrefix(rel, "..") {
				t.Errorf("body file %s is inside the work directory %s", seen.path, dir)
			}
			if _, err := os.Stat(seen.path); !errors.Is(err, os.ErrNotExist) {
				t.Errorf("body file %s still exists after Merge returned: %v", seen.path, err)
			}
		})
	}
}

// TestMergeStopsOnCorrectionCheck verifies that the correction check runs
// after the protocol check (a protocol stop leaves it unread) and before the
// number check (its stop leaves that check uncalled), that a stop never calls
// gh pr merge and keeps the work directory, and that it shows the discard
// command for a mismatch and the same merge again for an unreadable range.
func TestMergeStopsOnCorrectionCheck(t *testing.T) {
	lead := []commandStep{viewStep(openState, "main"), fetchMainStep()}
	accepted := correctionLine{Commit: testCommitA, Line: 3, Text: []byte(testCorrection)}
	tests := []struct {
		name        string
		lines       []correctionLine
		readErr     error
		wantErr     error
		wantDiscard bool
	}{
		{"missing line", []correctionLine{accepted, {Commit: testCommitB, Line: 2, Text: []byte(testOtherCorrection)}}, nil, errCorrectionMismatch, true},
		{"undeclared rejected line", []correctionLine{accepted, {Commit: testCommitB, Line: 2, Text: []byte("Editorial correction: \u200bhidden")}}, nil, errCorrectionMismatch, true},
		{"unreadable", nil, fmt.Errorf("%w: rev-list failed", errCorrectionUnreadable), errCorrectionUnreadable, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir, statePath, subjectPath, bodyPath := mergeFiles(t)
			writeTempFile(t, dir, filepath.Base(bodyPath), correctedBody)
			tool, runner, numbers := newNumberedTool(t, nil, nil, lead...)
			source := sourceOf(t, tool)
			source.lines, source.err = tt.lines, tt.readErr
			report, err := tool.Merge(t.Context(), statePath, subjectPath, bodyPath, Drops{})
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Merge error = %v, want %v", err, tt.wantErr)
			}
			runner.done()
			if want := []sourceCall{{method: "rangeLines", index: len(lead)}}; !slices.Equal(source.calls, want) {
				t.Errorf("reading calls = %+v, want %+v", source.calls, want)
			}
			if len(numbers.calls) != 0 {
				t.Errorf("number check calls = %+v, want none after a correction stop", numbers.calls)
			}
			if report.ProtocolCheck != testSummary.String() || report.MergeCommitOID != "" {
				t.Errorf("report = %+v, want the passed protocol check and no merge", report)
			}
			if _, err := os.Stat(dir); err != nil {
				t.Errorf("work directory %s removed on a stopped merge: %v", dir, err)
			}
			// The command the operator is told to run is the contract.
			if got := strings.Contains(err.Error(), discardCommand(statePath)); got != tt.wantDiscard {
				t.Errorf("Merge error = %q, shows the discard command: %t, want %t", err, got, tt.wantDiscard)
			}
			if got := strings.Contains(err.Error(), "run the same merge command again"); got == tt.wantDiscard {
				t.Errorf("Merge error = %q, tells to run the same merge again: %t, want %t", err, got, !tt.wantDiscard)
			}
		})
	}
	t.Run("after the protocol check", func(t *testing.T) {
		_, statePath, subjectPath, bodyPath := mergeFiles(t)
		tool, runner, _ := newCheckedTool(t, stopErrors[0].err, lead...)
		source := sourceOf(t, tool)
		if _, err := tool.Merge(t.Context(), statePath, subjectPath, bodyPath, Drops{}); !errors.Is(err, stopErrors[0].err) {
			t.Fatalf("Merge error = %v, want the protocol stop", err)
		}
		runner.done()
		if len(source.calls) != 0 {
			t.Errorf("reading calls = %+v, want none after a protocol stop", source.calls)
		}
	})
}

// TestMergePostMergeCheck verifies that the post-merge comparison runs after
// the cleanup on the merge commit the cleanup learned, against the bytes the
// merge checked (a body file rewritten during the merge changes nothing),
// that a difference is a CorrectionsNotCarriedError after the cleanup's own
// error, and that the work directory goes once the cleanup succeeded.
func TestMergePostMergeCheck(t *testing.T) {
	tests := []struct {
		name        string
		merged      []string
		localOID    string
		wantErrs    []error
		wantChecked bool
		wantDirGone bool
	}{
		{"carried", []string{testCorrection}, testHeadOID, nil, true, true},
		{"not carried", nil, testHeadOID, []error{errCorrectionsNotCarried}, false, true},
		{"cleanup stopped too", nil, testOtherOID, []error{errLocalBranchMoved, errCorrectionsNotCarried}, false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir, statePath, subjectPath, bodyPath := mergeFiles(t)
			writeTempFile(t, dir, filepath.Base(bodyPath), correctedBody)
			var seen bodyFileSeen
			merge := mergeStep()
			merge.hook = seeBodyFile(t, &seen, bodyPath)
			tool, runner := newTool(t, append([]commandStep{viewStep(openState, "main"), fetchMainStep(), merge}, cleanupSteps(tt.localOID)...)...)
			source := sourceOf(t, tool)
			source.lines = []correctionLine{{Commit: testCommitA, Line: 3, Text: []byte(testCorrection)}}
			for _, l := range tt.merged {
				source.merged = append(source.merged, []byte(l))
			}
			report, err := tool.Merge(t.Context(), statePath, subjectPath, bodyPath, Drops{})
			runner.done()
			if len(tt.wantErrs) == 0 && err != nil {
				t.Fatalf("Merge returned error: %v", err)
			}
			last := -1
			for _, want := range tt.wantErrs {
				if !errors.Is(err, want) {
					t.Fatalf("Merge error = %v, want it to wrap %v", err, want)
				}
				// The order of the reasons is the contract: the cleanup's
				// stop is read first.
				at := strings.Index(err.Error(), want.Error())
				if at <= last {
					t.Errorf("Merge error = %q, want %q after the earlier reasons", err, want)
				}
				last = at
			}
			if want := (sourceCall{method: "mergedLines", index: len(runner.steps)}); len(source.calls) != 2 || source.calls[1] != want {
				t.Errorf("reading calls = %+v, want the merge commit read after every command", source.calls)
			}
			if source.gotMerge != testMergeOID || source.gotBase != "main" {
				t.Errorf("merge commit read = %s on %s, want %s on main", source.gotMerge, source.gotBase, testMergeOID)
			}
			if report.CorrectionsChecked != tt.wantChecked || (tt.wantChecked && report.CorrectionsVerified != 1) {
				t.Errorf("report = %+v, want checked %t with 1 line", report, tt.wantChecked)
			}
			if _, err := os.Stat(dir); errors.Is(err, os.ErrNotExist) != tt.wantDirGone {
				t.Errorf("work directory gone: %t, want %t", errors.Is(err, os.ErrNotExist), tt.wantDirGone)
			}
		})
	}
}

// TestCleanupPostMergeCheck verifies that Cleanup runs the post-merge
// comparison only with a body file, and reports "not verified" without one.
func TestCleanupPostMergeCheck(t *testing.T) {
	tests := []struct {
		name        string
		withBody    bool
		merged      [][]byte
		wantErr     error
		wantChecked bool
		wantReads   int
	}{
		{"no body file", false, nil, nil, false, 0},
		{"carried", true, [][]byte{[]byte(testCorrection)}, nil, true, 1},
		{"not carried", true, nil, errCorrectionsNotCarried, false, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir, statePath, _, bodyPath := mergeFiles(t)
			writeTempFile(t, dir, filepath.Base(bodyPath), correctedBody)
			if !tt.withBody {
				bodyPath = ""
			}
			tool, runner := newTool(t, cleanupSteps(testHeadOID)...)
			source := sourceOf(t, tool)
			source.merged = tt.merged
			report, err := tool.Cleanup(t.Context(), statePath, bodyPath)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Cleanup error = %v, want %v", err, tt.wantErr)
			}
			runner.done()
			if len(source.calls) != tt.wantReads {
				t.Errorf("reading calls = %+v, want %d", source.calls, tt.wantReads)
			}
			if report.CorrectionsChecked != tt.wantChecked {
				t.Errorf("CorrectionsChecked = %t, want %t", report.CorrectionsChecked, tt.wantChecked)
			}
		})
	}
}

// TestMergeStopsAfterTheMergeKnown verifies what Merge reports when it stops
// at or after gh pr merge: a refused merge shows the cleanup command with
// the body file, so the cleanup can run the post-merge comparison; a merge
// whose cleanup could not read the PR reports the merge as done but not
// verified; and a body file that cannot be made stops before gh pr merge
// with the preparation kept for the same merge again.
func TestMergeStopsAfterTheMergeKnown(t *testing.T) {
	t.Run("refused", func(t *testing.T) {
		_, statePath, subjectPath, bodyPath := mergeFiles(t)
		refused := mergeStep()
		refused.err = errors.New("exit status 1")
		tool, runner := newTool(t, viewStep(openState, "main"), fetchMainStep(), refused)
		_, err := tool.Merge(t.Context(), statePath, subjectPath, bodyPath, Drops{})
		runner.done()
		// The command the operator is told to run is the contract.
		if err == nil || !strings.Contains(err.Error(), cleanupCommand(statePath, bodyPath)) {
			t.Errorf("Merge error = %v, want it to show %q", err, cleanupCommand(statePath, bodyPath))
		}
	})
	t.Run("cleanup cannot read the PR", func(t *testing.T) {
		_, statePath, subjectPath, bodyPath := mergeFiles(t)
		view := viewStep(mergedState, "main")
		view.err = errors.New("exit status 1")
		tool, runner := newTool(t, viewStep(openState, "main"), fetchMainStep(), mergeStep(), view)
		report, err := tool.Merge(t.Context(), statePath, subjectPath, bodyPath, Drops{})
		runner.done()
		if !errors.Is(err, view.err) {
			t.Fatalf("Merge error = %v, want the cleanup's error %v", err, view.err)
		}
		if !report.Merged || report.MergeCommitOID != "" || report.CorrectionsChecked {
			t.Errorf("report = %+v, want merged, no merge commit, not verified", report)
		}
	})
	t.Run("body file cannot be made", func(t *testing.T) {
		dir, statePath, subjectPath, bodyPath := mergeFiles(t)
		t.Setenv("TMPDIR", filepath.Join(t.TempDir(), "missing"))
		tool, runner := newTool(t, viewStep(openState, "main"), fetchMainStep())
		_, err := tool.Merge(t.Context(), statePath, subjectPath, bodyPath, Drops{})
		runner.done()
		if !errors.Is(err, os.ErrNotExist) {
			t.Errorf("Merge error = %v, want the missing temporary directory", err)
		}
		// The guidance shown to the operator is the contract.
		if err == nil || !strings.Contains(err.Error(), "run the same merge command again") {
			t.Errorf("Merge error = %v, want it to keep the preparation for the same merge", err)
		}
		if _, err := os.Stat(dir); err != nil {
			t.Errorf("work directory %s removed: %v", dir, err)
		}
	})
}

// TestCleanupUnreadableBodyFile verifies that Cleanup with a body file it
// cannot read stops before it touches anything.
func TestCleanupUnreadableBodyFile(t *testing.T) {
	_, statePath, _, _ := mergeFiles(t)
	tool, runner := newTool(t)
	if _, err := tool.Cleanup(t.Context(), statePath, filepath.Join(t.TempDir(), "missing.txt")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Cleanup error = %v, want the missing body file", err)
	}
	runner.done()
}
