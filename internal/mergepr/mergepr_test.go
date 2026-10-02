//go:build test

package mergepr

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

const (
	testHeadOID  = "2222222222222222222222222222222222222222"
	testOtherOID = "4444444444444444444444444444444444444444"
	testMergeOID = "3333333333333333333333333333333333333333"
)

var testState = State{
	Number:      42,
	HeadRefName: "feature/foo",
	HeadRefOID:  testHeadOID,
	BaseRefName: "main",
	Title:       "Test PR",
	URL:         "https://github.com/isseis/yt2column/pull/42",
}

const testPRJSON = `{"number":42,"title":"Test PR","state":"OPEN","headRefName":"feature/foo","headRefOid":"` + testHeadOID + `","baseRefName":"main","url":"https://github.com/isseis/yt2column/pull/42","body":"the PR description"}`

func prepareTailSteps() []commandStep {
	return []commandStep{
		gitStep("", "fetch", "origin"),
		ghStep("", "pr", "checks", "42", "--watch", "--fail-fast"),
		gitStep("abc subject\n\nbody\n", "log", "--format=%h %s%n%n%b", "origin/main.."+testHeadOID),
		gitStep(" a.txt | 1 +\n", "diff", "--stat", "origin/main..."+testHeadOID),
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
	tool, runner := newTool(t, append([]commandStep{
		ghStep(testPRJSON, "pr", "view", "42", "--json", prViewFields),
	}, prepareTailSteps()...)...)

	prepared, err := tool.Prepare(t.Context(), "42")
	if err != nil {
		t.Fatalf("Prepare returned error: %v", err)
	}
	runner.done()
	t.Cleanup(func() { _ = os.RemoveAll(prepared.Dir) })

	if !reflect.DeepEqual(prepared.State, testState) {
		t.Errorf("state = %+v, want %+v", prepared.State, testState)
	}
	var saved State
	if err := json.Unmarshal([]byte(readFile(t, filepath.Join(prepared.Dir, stateFileName))), &saved); err != nil {
		t.Fatalf("parse state file: %v", err)
	}
	if !reflect.DeepEqual(saved, testState) {
		t.Errorf("state file = %+v, want %+v", saved, testState)
	}
	for name, want := range map[string]string{
		logFileName:  "abc subject\n\nbody\n",
		statFileName: " a.txt | 1 +\n",
		bodyFileName: "the PR description",
	} {
		if got := readFile(t, filepath.Join(prepared.Dir, name)); got != want {
			t.Errorf("%s = %q, want %q", name, got, want)
		}
	}
}

func TestPrepareCurrentBranch(t *testing.T) {
	tool, runner := newTool(t, append([]commandStep{
		ghStep(testPRJSON, "pr", "view", "--json", prViewFields),
	}, prepareTailSteps()...)...)

	prepared, err := tool.Prepare(t.Context(), "")
	if err != nil {
		t.Fatalf("Prepare returned error: %v", err)
	}
	runner.done()
	t.Cleanup(func() { _ = os.RemoveAll(prepared.Dir) })
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
	data, err := json.Marshal(testState)
	if err != nil {
		t.Fatalf("encode state: %v", err)
	}
	return writeTempFile(t, dir, stateFileName, string(data))
}

func viewStep(state, base string) commandStep {
	out := `{"state":"` + state + `","baseRefName":"` + base + `","mergeCommit":null}`
	if state == mergedState {
		out = `{"state":"MERGED","baseRefName":"` + base + `","mergeCommit":{"oid":"` + testMergeOID + `"}}`
	}
	return ghStep(out, "pr", "view", "42", "--json", "state,baseRefName,mergeCommit")
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

func mergeFiles(t *testing.T) (string, string, string) {
	t.Helper()
	dir := t.TempDir()
	return writeStateFile(t, dir),
		writeTempFile(t, dir, "subject.txt", "feat: subject (#42)\n"),
		writeTempFile(t, dir, "body.txt", "body text\n")
}

func TestMerge(t *testing.T) {
	statePath, subjectPath, bodyPath := mergeFiles(t)
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
			statePath, subjectPath, bodyPath := mergeFiles(t)
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
		statePath, _, bodyPath := mergeFiles(t)
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

func TestLoadStateRejectsIncompleteState(t *testing.T) {
	path := writeTempFile(t, t.TempDir(), "state.json", `{"number":42}`)
	if _, err := loadState(path); !errors.Is(err, errInvalidState) {
		t.Fatalf("loadState error = %v, want errInvalidState", err)
	}
}
