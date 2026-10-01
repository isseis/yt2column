//go:build test

package mergepr

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

const testRemotePresent = testHeadOID + "\trefs/heads/feature/foo\n"

func newTool(t testing.TB, steps []commandStep) (*Tool, *fakeRunner) {
	t.Helper()
	runner := &fakeRunner{t: t, steps: steps}
	tool, err := New(runner)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	return tool, runner
}

func repoIdentitySteps() []commandStep {
	return []commandStep{
		gitStep([]string{"remote", "get-url", "origin"}, testFetchURLOut),
		gitStep([]string{"remote", "get-url", "--push", "origin"}, testFetchURLOut),
		ghStep([]string{"repo", "view", "--json", "nameWithOwner", "-q", ".nameWithOwner"}, testRepoViewOut),
	}
}

func prViewArgs() []string {
	return []string{"pr", "view", strconv.Itoa(testPRNumber), "--json", prViewFields, "-R", testRepoArg}
}

func prViewJSON(headName string, crossRepository bool) string {
	return fmt.Sprintf(`{"number":%d,"title":"Test PR","state":%q,"headRefName":%q,"headRefOid":%q,"baseRefName":%q,"isCrossRepository":%t,"url":"https://github.com/isseis/yt2column/pull/42"}`,
		testPRNumber, openState, headName, testHeadOID, "main", crossRepository)
}

func mergeViewArgs() []string {
	return []string{"pr", "view", strconv.Itoa(testPRNumber), "--json", mergeViewJSON, "-R", testRepoArg}
}

func mergeViewOut(state, headName, head, base string, crossRepository, merged bool) string {
	mergeCommit := "null"
	if merged {
		mergeCommit = fmt.Sprintf(`{"oid":%q}`, testMergeOID)
	}
	return fmt.Sprintf(`{"state":%q,"headRefName":%q,"headRefOid":%q,"baseRefName":%q,"isCrossRepository":%t,"mergeCommit":%s}`,
		state, headName, head, base, crossRepository, mergeCommit)
}

func checksArgs() []string {
	return []string{"pr", "checks", strconv.Itoa(testPRNumber), "--watch", "--fail-fast", "-R", testRepoArg}
}

func mergeArgs(subject, bodyPath string) []string {
	return []string{"pr", "merge", strconv.Itoa(testPRNumber), "--squash", "--subject", subject, "--body-file", bodyPath, "--match-head-commit", testHeadOID, "-R", testRepoArg}
}

func prepareSteps(logOut, statOut string) []commandStep {
	steps := repoIdentitySteps()
	steps = append(steps,
		ghStep(prViewArgs(), prViewJSON("feature/foo", false)),
		gitStep([]string{"check-ref-format", "--branch", "feature/foo"}, "feature/foo\n"),
		gitStep([]string{"check-ref-format", "--branch", "main"}, "main\n"),
		gitStep([]string{"status", "--porcelain", "--untracked-files=all"}, ""),
		gitStep([]string{"for-each-ref", "--format=%(objectname)", "refs/heads/feature/foo"}, testHeadOID+"\n"),
		gitStep([]string{"fetch", "origin"}, ""),
		ghStep(checksArgs(), ""),
		gitStep([]string{"log", "--no-show-signature", "--format=%h %s%n%n%b", "refs/remotes/origin/main.." + testHeadOID}, logOut),
		gitStep([]string{"diff", "--stat", "refs/remotes/origin/main..." + testHeadOID}, statOut),
	)
	return steps
}

func mergedViewStep() commandStep {
	return ghStep(mergeViewArgs(), mergeViewOut(mergedState, "feature/foo", testHeadOID, "main", false, true))
}

func cleanupSteps(remoteOut, localOID string) []commandStep {
	steps := repoIdentitySteps()
	steps = append(steps, gitStep([]string{"status", "--porcelain", "--untracked-files=all"}, ""))
	steps = append(steps, gitStep([]string{"ls-remote", "--heads", "origin", "refs/heads/feature/foo"}, remoteOut))
	if remoteOut != "" {
		steps = append(steps, gitStep([]string{"push", "--force-with-lease=refs/heads/feature/foo:" + testHeadOID, "origin", "--delete", "refs/heads/feature/foo"}, ""))
	}
	steps = append(steps,
		gitStep([]string{"switch", "main"}, ""),
		gitStep([]string{"fetch", "origin", "refs/heads/main:refs/remotes/origin/main"}, ""),
		gitStep([]string{"merge", "--ff-only", "refs/remotes/origin/main"}, ""),
		gitStep([]string{"rev-parse", "HEAD"}, testBaseOID+"\n"),
		gitStep([]string{"rev-parse", "refs/remotes/origin/main"}, testBaseOID+"\n"),
		gitStep([]string{"for-each-ref", "--format=%(objectname)", "refs/heads/feature/foo"}, localOID),
	)
	if localOID == testHeadOID {
		steps = append(steps, gitStep([]string{"branch", "-D", "feature/foo"}, ""))
	}
	if localOID == "" || localOID == testHeadOID {
		steps = append(steps, gitStep([]string{"fetch", "--prune", "origin"}, ""))
	}
	return steps
}

func TestNewRejectsNilRunner(t *testing.T) {
	if _, err := New(nil); !errors.Is(err, errNoRunner) {
		t.Fatalf("New(nil) error = %v, want errNoRunner", err)
	}
}

func TestParseGitHubRemote(t *testing.T) {
	cases := []struct {
		name      string
		raw       string
		wantOwner string
		wantRepo  string
		wantErr   bool
	}{
		{"https", "https://github.com/isseis/yt2column.git", "isseis", "yt2column", false},
		{"https without suffix", "https://github.com/isseis/yt2column", "isseis", "yt2column", false},
		{"https trailing slash", "https://github.com/isseis/yt2column/", "isseis", "yt2column", false},
		{"http", "http://github.com/isseis/yt2column", "isseis", "yt2column", false},
		{"host case", "https://GitHub.com/isseis/yt2column.git", "isseis", "yt2column", false},
		{"ssh url", "ssh://git@github.com/isseis/yt2column.git", "isseis", "yt2column", false},
		{"scp-like", "git@github.com:isseis/yt2column.git", "isseis", "yt2column", false},
		{"other host", "https://gitlab.com/isseis/yt2column.git", "", "", true},
		{"scp other host", "git@gitlab.com:isseis/yt2column.git", "", "", true},
		{"missing repo", "https://github.com/isseis", "", "", true},
		{"extra path", "https://github.com/isseis/yt2column/extra", "", "", true},
		{"empty", "", "", "", true},
		{"no scheme or host", "isseis/yt2column", "", "", true},
		{"file url", "file:///tmp/repo", "", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			owner, repo, err := parseGitHubRemote(tc.raw)
			if tc.wantErr {
				if !errors.Is(err, errInvalidRemote) {
					t.Fatalf("parseGitHubRemote(%q) error = %v, want errInvalidRemote", tc.raw, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseGitHubRemote(%q) error = %v, want nil", tc.raw, err)
			}
			if owner != tc.wantOwner || repo != tc.wantRepo {
				t.Errorf("parseGitHubRemote(%q) = %s/%s, want %s/%s", tc.raw, owner, repo, tc.wantOwner, tc.wantRepo)
			}
		})
	}
}

func TestParsePRArg(t *testing.T) {
	cases := []struct {
		name        string
		arg         string
		wantNumber  int
		wantCurrent bool
		wantErr     bool
	}{
		{"empty uses current branch", "", 0, true, false},
		{"number", "42", 42, false, false},
		{"number with spaces", " 42 ", 42, false, false},
		{"url", "https://github.com/isseis/yt2column/pull/7", 7, false, false},
		{"url other owner", "https://github.com/other/yt2column/pull/7", 0, false, true},
		{"url other repo", "https://github.com/isseis/other/pull/7", 0, false, true},
		{"url other host", "https://gitlab.com/isseis/yt2column/pull/7", 0, false, true},
		{"other path", "https://github.com/isseis/yt2column/issues/7", 0, false, true},
		{"negative", "-1", 0, false, true},
		{"zero", "0", 0, false, true},
		{"not a number", "abc", 0, false, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			number, current, err := parsePRArg(tc.arg, "isseis", "yt2column")
			if tc.wantErr {
				if !errors.Is(err, errInvalidPRArg) {
					t.Fatalf("parsePRArg(%q) error = %v, want errInvalidPRArg", tc.arg, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("parsePRArg(%q) error = %v, want nil", tc.arg, err)
			}
			if number != tc.wantNumber || current != tc.wantCurrent {
				t.Errorf("parsePRArg(%q) = (%d, %t), want (%d, %t)", tc.arg, number, current, tc.wantNumber, tc.wantCurrent)
			}
		})
	}
}

func TestCheckRefNameSyntax(t *testing.T) {
	cases := []struct {
		name    string
		ref     string
		wantErr bool
	}{
		{"branch", "main", false},
		{"nested", "feature/foo", false},
		{"space is left to git", "a b", false},
		{"empty", "", true},
		{"leading dash", "-B", true},
		{"reflog expression", "foo@{1}", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := checkRefNameSyntax(tc.ref)
			if tc.wantErr {
				if !errors.Is(err, errInvalidBranch) {
					t.Fatalf("checkRefNameSyntax(%q) error = %v, want errInvalidBranch", tc.ref, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("checkRefNameSyntax(%q) error = %v, want nil", tc.ref, err)
			}
		})
	}
}

func TestPrepareHappyPath(t *testing.T) {
	const (
		logOut  = "abc123 subject\n\nbody\n"
		statOut = " a.txt | 1 +\n"
	)
	dir := t.TempDir()
	tool, runner := newTool(t, prepareSteps(logOut, statOut))

	prepared, err := tool.Prepare(t.Context(), strconv.Itoa(testPRNumber), dir)
	if err != nil {
		t.Fatalf("Prepare returned error: %v", err)
	}
	runner.done()

	want := State{
		Number:      testPRNumber,
		Owner:       "isseis",
		Repo:        "yt2column",
		HeadRefName: "feature/foo",
		HeadRefOID:  testHeadOID,
		BaseRefName: "main",
		Title:       "Test PR",
		URL:         "https://github.com/isseis/yt2column/pull/42",
	}
	if !reflect.DeepEqual(prepared.State, want) {
		t.Errorf("state = %+v, want %+v", prepared.State, want)
	}
	if prepared.StatePath != filepath.Join(dir, stateFileName) {
		t.Errorf("StatePath = %q, want %q", prepared.StatePath, filepath.Join(dir, stateFileName))
	}
	if prepared.LogPath != filepath.Join(dir, logFileName) {
		t.Errorf("LogPath = %q, want %q", prepared.LogPath, filepath.Join(dir, logFileName))
	}
	if prepared.StatPath != filepath.Join(dir, statFileName) {
		t.Errorf("StatPath = %q, want %q", prepared.StatPath, filepath.Join(dir, statFileName))
	}

	data, err := os.ReadFile(prepared.StatePath)
	if err != nil {
		t.Fatalf("read state file: %v", err)
	}
	var got State
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal state file: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("state file = %+v, want %+v", got, want)
	}
	if data, err := os.ReadFile(prepared.LogPath); err != nil || string(data) != logOut {
		t.Errorf("log file = %q (err %v), want %q", data, err, logOut)
	}
	if data, err := os.ReadFile(prepared.StatPath); err != nil || string(data) != statOut {
		t.Errorf("stat file = %q (err %v), want %q", data, err, statOut)
	}
}

func TestPrepareRejectsPushURLMismatch(t *testing.T) {
	steps := []commandStep{
		gitStep([]string{"remote", "get-url", "origin"}, testFetchURLOut),
		gitStep([]string{"remote", "get-url", "--push", "origin"}, "git@github.com:fork/yt2column.git\n"),
	}
	tool, runner := newTool(t, steps)

	if _, err := tool.Prepare(t.Context(), strconv.Itoa(testPRNumber), t.TempDir()); !errors.Is(err, errRemoteMismatch) {
		t.Fatalf("Prepare error = %v, want errRemoteMismatch", err)
	}
	runner.done()
}

func TestPrepareRejectsSelectedRepoMismatch(t *testing.T) {
	steps := []commandStep{
		gitStep([]string{"remote", "get-url", "origin"}, testFetchURLOut),
		gitStep([]string{"remote", "get-url", "--push", "origin"}, testFetchURLOut),
		ghStep([]string{"repo", "view", "--json", "nameWithOwner", "-q", ".nameWithOwner"}, "other/repo\n"),
	}
	tool, runner := newTool(t, steps)

	if _, err := tool.Prepare(t.Context(), strconv.Itoa(testPRNumber), t.TempDir()); !errors.Is(err, errRepoMismatch) {
		t.Fatalf("Prepare error = %v, want errRepoMismatch", err)
	}
	runner.done()
}

func TestPrepareRejectsUnsafeHeadName(t *testing.T) {
	steps := repoIdentitySteps()
	steps = append(steps, ghStep(prViewArgs(), prViewJSON("-B", false)))
	tool, runner := newTool(t, steps)

	if _, err := tool.Prepare(t.Context(), strconv.Itoa(testPRNumber), t.TempDir()); !errors.Is(err, errInvalidBranch) {
		t.Fatalf("Prepare error = %v, want errInvalidBranch", err)
	}
	runner.done()
}

func TestPrepareRejectsGitInvalidName(t *testing.T) {
	steps := repoIdentitySteps()
	steps = append(steps,
		ghStep(prViewArgs(), prViewJSON("feature/foo", false)),
		commandStep{name: gitCommand, args: []string{"check-ref-format", "--branch", "feature/foo"}, err: errors.New("exit status 128")},
	)
	tool, runner := newTool(t, steps)

	if _, err := tool.Prepare(t.Context(), strconv.Itoa(testPRNumber), t.TempDir()); !errors.Is(err, errInvalidBranch) {
		t.Fatalf("Prepare error = %v, want errInvalidBranch", err)
	}
	runner.done()
}

func TestPrepareRejectsCrossRepository(t *testing.T) {
	steps := repoIdentitySteps()
	steps = append(steps, ghStep(prViewArgs(), prViewJSON("feature/foo", true)))
	tool, runner := newTool(t, steps)

	if _, err := tool.Prepare(t.Context(), strconv.Itoa(testPRNumber), t.TempDir()); !errors.Is(err, errCrossRepository) {
		t.Fatalf("Prepare error = %v, want errCrossRepository", err)
	}
	runner.done()
}

func TestPrepareRejectsDirtyWorktree(t *testing.T) {
	steps := repoIdentitySteps()
	steps = append(steps,
		ghStep(prViewArgs(), prViewJSON("feature/foo", false)),
		gitStep([]string{"check-ref-format", "--branch", "feature/foo"}, "feature/foo\n"),
		gitStep([]string{"check-ref-format", "--branch", "main"}, "main\n"),
		gitStep([]string{"status", "--porcelain", "--untracked-files=all"}, "?? untracked.txt\n"),
	)
	tool, runner := newTool(t, steps)

	if _, err := tool.Prepare(t.Context(), strconv.Itoa(testPRNumber), t.TempDir()); !errors.Is(err, errDirtyWorktree) {
		t.Fatalf("Prepare error = %v, want errDirtyWorktree", err)
	}
	runner.done()
}

func TestPrepareRejectsHeadBranchMismatch(t *testing.T) {
	steps := repoIdentitySteps()
	steps = append(steps,
		ghStep(prViewArgs(), prViewJSON("feature/foo", false)),
		gitStep([]string{"check-ref-format", "--branch", "feature/foo"}, "feature/foo\n"),
		gitStep([]string{"check-ref-format", "--branch", "main"}, "main\n"),
		gitStep([]string{"status", "--porcelain", "--untracked-files=all"}, ""),
		gitStep([]string{"for-each-ref", "--format=%(objectname)", "refs/heads/feature/foo"}, testOtherOID+"\n"),
	)
	tool, runner := newTool(t, steps)

	if _, err := tool.Prepare(t.Context(), strconv.Itoa(testPRNumber), t.TempDir()); !errors.Is(err, errHeadBranchMismatch) {
		t.Fatalf("Prepare error = %v, want errHeadBranchMismatch", err)
	}
	runner.done()
}

func TestPrepareRejectsChecksFailure(t *testing.T) {
	steps := repoIdentitySteps()
	steps = append(steps,
		ghStep(prViewArgs(), prViewJSON("feature/foo", false)),
		gitStep([]string{"check-ref-format", "--branch", "feature/foo"}, "feature/foo\n"),
		gitStep([]string{"check-ref-format", "--branch", "main"}, "main\n"),
		gitStep([]string{"status", "--porcelain", "--untracked-files=all"}, ""),
		gitStep([]string{"for-each-ref", "--format=%(objectname)", "refs/heads/feature/foo"}, testHeadOID+"\n"),
		gitStep([]string{"fetch", "origin"}, ""),
		commandStep{name: ghCommand, args: checksArgs(), err: errors.New("exit status 1")},
	)
	tool, runner := newTool(t, steps)

	if _, err := tool.Prepare(t.Context(), strconv.Itoa(testPRNumber), t.TempDir()); !errors.Is(err, errChecksFailed) {
		t.Fatalf("Prepare error = %v, want errChecksFailed", err)
	}
	runner.done()
}

func TestPrepareRejectsOversizedLog(t *testing.T) {
	steps := repoIdentitySteps()
	steps = append(steps,
		ghStep(prViewArgs(), prViewJSON("feature/foo", false)),
		gitStep([]string{"check-ref-format", "--branch", "feature/foo"}, "feature/foo\n"),
		gitStep([]string{"check-ref-format", "--branch", "main"}, "main\n"),
		gitStep([]string{"status", "--porcelain", "--untracked-files=all"}, ""),
		gitStep([]string{"for-each-ref", "--format=%(objectname)", "refs/heads/feature/foo"}, testHeadOID+"\n"),
		gitStep([]string{"fetch", "origin"}, ""),
		ghStep(checksArgs(), ""),
		gitStep([]string{"log", "--no-show-signature", "--format=%h %s%n%n%b", "refs/remotes/origin/main.." + testHeadOID}, strings.Repeat("x", maxLogBytes+1)),
	)
	tool, runner := newTool(t, steps)

	if _, err := tool.Prepare(t.Context(), strconv.Itoa(testPRNumber), t.TempDir()); !errors.Is(err, errTooLarge) {
		t.Fatalf("Prepare error = %v, want errTooLarge", err)
	}
	runner.done()
}

func TestMergeHappyPath(t *testing.T) {
	dir := t.TempDir()
	statePath := writeStateFile(t, dir)
	subjectPath := writeTempFile(t, dir, "subject.txt", "subject line\n")
	bodyPath := writeTempFile(t, dir, "body.txt", "body text\n")

	steps := []commandStep{
		ghStep(mergeViewArgs(), mergeViewOut(openState, "feature/foo", testHeadOID, "main", false, false)),
		ghStep(checksArgs(), ""),
		ghStep(mergeArgs("subject line", bodyPath), ""),
		ghStep(mergeViewArgs(), mergeViewOut(mergedState, "feature/foo", testHeadOID, "main", false, true)),
	}
	steps = append(steps, cleanupSteps(testRemotePresent, testHeadOID)...)
	tool, runner := newTool(t, steps)

	report, err := tool.Merge(t.Context(), statePath, subjectPath, bodyPath)
	if err != nil {
		t.Fatalf("Merge returned error: %v", err)
	}
	runner.done()

	want := Report{MergeCommitOID: testMergeOID, RemoteDeleted: true, LocalDeleted: true, BaseUpdated: true}
	if !reflect.DeepEqual(report, want) {
		t.Errorf("report = %+v, want %+v", report, want)
	}
}

func TestMergeRejectsHeadDrift(t *testing.T) {
	dir := t.TempDir()
	statePath := writeStateFile(t, dir)
	subjectPath := writeTempFile(t, dir, "subject.txt", "subject line\n")
	bodyPath := writeTempFile(t, dir, "body.txt", "body\n")

	steps := []commandStep{ghStep(mergeViewArgs(), mergeViewOut(openState, "feature/foo", testOtherOID, "main", false, false))}
	tool, runner := newTool(t, steps)

	if _, err := tool.Merge(t.Context(), statePath, subjectPath, bodyPath); !errors.Is(err, errHeadDrift) {
		t.Fatalf("Merge error = %v, want errHeadDrift", err)
	}
	runner.done()
}

func TestMergeRejectsHeadNameDrift(t *testing.T) {
	dir := t.TempDir()
	statePath := writeStateFile(t, dir)
	subjectPath := writeTempFile(t, dir, "subject.txt", "subject line\n")
	bodyPath := writeTempFile(t, dir, "body.txt", "body\n")

	steps := []commandStep{ghStep(mergeViewArgs(), mergeViewOut(openState, "other", testHeadOID, "main", false, false))}
	tool, runner := newTool(t, steps)

	if _, err := tool.Merge(t.Context(), statePath, subjectPath, bodyPath); !errors.Is(err, errHeadDrift) {
		t.Fatalf("Merge error = %v, want errHeadDrift", err)
	}
	runner.done()
}

func TestMergeRejectsBaseDrift(t *testing.T) {
	dir := t.TempDir()
	statePath := writeStateFile(t, dir)
	subjectPath := writeTempFile(t, dir, "subject.txt", "subject line\n")
	bodyPath := writeTempFile(t, dir, "body.txt", "body\n")

	steps := []commandStep{ghStep(mergeViewArgs(), mergeViewOut(openState, "feature/foo", testHeadOID, "release", false, false))}
	tool, runner := newTool(t, steps)

	if _, err := tool.Merge(t.Context(), statePath, subjectPath, bodyPath); !errors.Is(err, errBaseDrift) {
		t.Fatalf("Merge error = %v, want errBaseDrift", err)
	}
	runner.done()
}

func TestMergeRejectsCrossRepository(t *testing.T) {
	dir := t.TempDir()
	statePath := writeStateFile(t, dir)
	subjectPath := writeTempFile(t, dir, "subject.txt", "subject line\n")
	bodyPath := writeTempFile(t, dir, "body.txt", "body\n")

	steps := []commandStep{ghStep(mergeViewArgs(), mergeViewOut(openState, "feature/foo", testHeadOID, "main", true, false))}
	tool, runner := newTool(t, steps)

	if _, err := tool.Merge(t.Context(), statePath, subjectPath, bodyPath); !errors.Is(err, errCrossRepository) {
		t.Fatalf("Merge error = %v, want errCrossRepository", err)
	}
	runner.done()
}

func TestMergeRechecksChecks(t *testing.T) {
	dir := t.TempDir()
	statePath := writeStateFile(t, dir)
	subjectPath := writeTempFile(t, dir, "subject.txt", "subject line\n")
	bodyPath := writeTempFile(t, dir, "body.txt", "body\n")

	steps := []commandStep{
		ghStep(mergeViewArgs(), mergeViewOut(openState, "feature/foo", testHeadOID, "main", false, false)),
		{name: ghCommand, args: checksArgs(), err: errors.New("exit status 1")},
	}
	tool, runner := newTool(t, steps)

	if _, err := tool.Merge(t.Context(), statePath, subjectPath, bodyPath); !errors.Is(err, errChecksFailed) {
		t.Fatalf("Merge error = %v, want errChecksFailed", err)
	}
	runner.done()
}

func TestMergeResumesCleanupWhenMerged(t *testing.T) {
	dir := t.TempDir()
	statePath := writeStateFile(t, dir)
	subjectPath := writeTempFile(t, dir, "subject.txt", "subject line\n")
	bodyPath := writeTempFile(t, dir, "body.txt", "body\n")

	steps := []commandStep{ghStep(mergeViewArgs(), mergeViewOut(mergedState, "feature/foo", testHeadOID, "main", false, true))}
	steps = append(steps, cleanupSteps(testRemotePresent, testHeadOID)...)
	tool, runner := newTool(t, steps)

	report, err := tool.Merge(t.Context(), statePath, subjectPath, bodyPath)
	if err != nil {
		t.Fatalf("Merge returned error: %v", err)
	}
	runner.done()
	if report.MergeCommitOID != testMergeOID {
		t.Errorf("MergeCommitOID = %q, want %q", report.MergeCommitOID, testMergeOID)
	}
}

func TestMergeRejectsEmptySubject(t *testing.T) {
	dir := t.TempDir()
	statePath := writeStateFile(t, dir)
	subjectPath := writeTempFile(t, dir, "subject.txt", "\n")
	bodyPath := writeTempFile(t, dir, "body.txt", "body\n")

	tool, runner := newTool(t, nil)
	if _, err := tool.Merge(t.Context(), statePath, subjectPath, bodyPath); !errors.Is(err, errInvalidSubject) {
		t.Fatalf("Merge error = %v, want errInvalidSubject", err)
	}
	runner.done()
}

func TestCleanupRemoteGone(t *testing.T) {
	dir := t.TempDir()
	statePath := writeStateFile(t, dir)
	tool, runner := newTool(t, append([]commandStep{mergedViewStep()}, cleanupSteps("", testHeadOID)...))

	report, err := tool.Cleanup(t.Context(), statePath)
	if err != nil {
		t.Fatalf("Cleanup returned error: %v", err)
	}
	runner.done()
	if report.RemoteDeleted {
		t.Error("RemoteDeleted = true, want false when the remote branch is already gone")
	}
	if !report.LocalDeleted || !report.BaseUpdated {
		t.Errorf("report = %+v, want LocalDeleted and BaseUpdated", report)
	}
}

func TestCleanupRejectsMovedLocalBranch(t *testing.T) {
	dir := t.TempDir()
	statePath := writeStateFile(t, dir)
	tool, runner := newTool(t, append([]commandStep{mergedViewStep()}, cleanupSteps(testRemotePresent, testOtherOID)...))

	if _, err := tool.Cleanup(t.Context(), statePath); !errors.Is(err, errLocalBranchDrift) {
		t.Fatalf("Cleanup error = %v, want errLocalBranchDrift", err)
	}
	runner.done()
}

func TestCleanupRejectsBaseNotCurrent(t *testing.T) {
	dir := t.TempDir()
	statePath := writeStateFile(t, dir)
	steps := []commandStep{mergedViewStep()}
	steps = append(steps, repoIdentitySteps()...)
	steps = append(steps,
		gitStep([]string{"status", "--porcelain", "--untracked-files=all"}, ""),
		gitStep([]string{"ls-remote", "--heads", "origin", "refs/heads/feature/foo"}, testRemotePresent),
		gitStep([]string{"push", "--force-with-lease=refs/heads/feature/foo:" + testHeadOID, "origin", "--delete", "refs/heads/feature/foo"}, ""),
		gitStep([]string{"switch", "main"}, ""),
		gitStep([]string{"fetch", "origin", "refs/heads/main:refs/remotes/origin/main"}, ""),
		gitStep([]string{"merge", "--ff-only", "refs/remotes/origin/main"}, ""),
		gitStep([]string{"rev-parse", "HEAD"}, testOtherOID+"\n"),
		gitStep([]string{"rev-parse", "refs/remotes/origin/main"}, testBaseOID+"\n"),
		gitStep([]string{"log", "--oneline", "refs/remotes/origin/main..HEAD"}, "abc local\n"),
	)
	tool, runner := newTool(t, steps)

	if _, err := tool.Cleanup(t.Context(), statePath); !errors.Is(err, errBaseNotCurrent) {
		t.Fatalf("Cleanup error = %v, want errBaseNotCurrent", err)
	}
	runner.done()
}

func TestLoadStateRejectsUnsafeState(t *testing.T) {
	dir := t.TempDir()
	state := State{
		Number:      testPRNumber,
		Owner:       "isseis",
		Repo:        "yt2column",
		HeadRefName: "-B",
		HeadRefOID:  testHeadOID,
		BaseRefName: "main",
	}
	path := filepath.Join(dir, stateFileName)
	if err := writeState(path, state); err != nil {
		t.Fatalf("write state: %v", err)
	}
	if _, err := loadState(path); !errors.Is(err, errInvalidState) {
		t.Fatalf("loadState error = %v, want errInvalidState", err)
	}
}
