//go:build test

package mergepr

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

const testPRJSON = `{"number":42,"title":"Test PR","state":"OPEN","headRefName":"feature/foo","headRefOid":"` + testHeadOID + `","baseRefName":"main","url":"https://github.com/isseis/yt2column/pull/42","body":"the PR description"}`

func prepareTailSteps(workRoot, gitDir string) []commandStep {
	return []commandStep{
		gitStep("", "fetch", "origin"),
		ghStep("", "pr", "checks", "42", "--watch", "--fail-fast"),
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

func TestPrepareRejectsFailedChecks(t *testing.T) {
	failed := ghStep("", "pr", "checks", "42", "--watch", "--fail-fast")
	failed.err = errors.New("exit status 1")
	tool, runner := newTool(t,
		ghStep(testPRJSON, "pr", "view", "42", "--json", prViewFields),
		gitStep("", "fetch", "origin"),
		failed,
	)

	if _, err := tool.Prepare(t.Context(), "42"); !errors.Is(err, errChecksFailed) {
		t.Fatalf("Prepare error = %v, want errChecksFailed", err)
	}
	runner.done()
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
	return ghStep("", "pr", "merge", "42", "--squash", "--subject", "feat: subject (#42)", "--body", "body text\n", "--match-head-commit", testHeadOID)
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
	steps := []commandStep{viewStep(openState, "main"), mergeStep()}
	tool, runner := newTool(t, append(steps, cleanupSteps(testHeadOID)...)...)

	report, err := tool.Merge(t.Context(), statePath, subjectPath, bodyPath)
	if err != nil {
		t.Fatalf("Merge returned error: %v", err)
	}
	runner.done()
	want := Report{MergeCommitOID: testMergeOID, BaseUpdated: true, LocalDeleted: true}
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

			if _, err := tool.Merge(t.Context(), statePath, subjectPath, bodyPath); !errors.Is(err, tt.want) {
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

		if _, err := tool.Merge(t.Context(), statePath, subjectPath, bodyPath); !errors.Is(err, errInvalidSubject) {
			t.Errorf("Merge(subject %q) error = %v, want errInvalidSubject", subject, err)
		}
		runner.done()
	}
}

func TestCleanupKeepsMovedLocalBranch(t *testing.T) {
	statePath := writeStateFile(t, t.TempDir())
	tool, runner := newTool(t, cleanupSteps(testOtherOID)...)

	report, err := tool.Cleanup(t.Context(), statePath)
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

	report, err := tool.Cleanup(t.Context(), statePath)
	if err != nil {
		t.Fatalf("Cleanup returned error: %v", err)
	}
	runner.done()
	want := Report{MergeCommitOID: testMergeOID, BaseUpdated: true}
	if !reflect.DeepEqual(report, want) {
		t.Errorf("report = %+v, want %+v", report, want)
	}
}

func TestCleanupRejectsUnmergedPR(t *testing.T) {
	statePath := writeStateFile(t, t.TempDir())
	tool, runner := newTool(t, viewStep(openState, "main"))

	if _, err := tool.Cleanup(t.Context(), statePath); !errors.Is(err, errPRNotMerged) {
		t.Fatalf("Cleanup error = %v, want errPRNotMerged", err)
	}
	runner.done()
}

func TestCleanupRejectsMergedHeadMoved(t *testing.T) {
	statePath := writeStateFile(t, t.TempDir())
	tool, runner := newTool(t, viewHeadStep(mergedState, "main", testOtherOID))

	report, err := tool.Cleanup(t.Context(), statePath)
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

	if _, err := tool.Cleanup(t.Context(), statePath); err != nil {
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

	report, err := tool.Cleanup(t.Context(), statePath)
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

	if _, err := tool.Cleanup(t.Context(), statePath); err != nil {
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

	if _, err := tool.Cleanup(t.Context(), statePath); err != nil {
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

	if _, err := tool.Cleanup(t.Context(), statePath); !errors.Is(err, errLocalBranchMoved) {
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
