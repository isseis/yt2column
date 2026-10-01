//go:build test

package mergepr

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
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

func headRefOut(oid string) string {
	if oid == "" {
		return ""
	}
	return "refs/heads/feature/foo " + oid + "\n"
}

func repoIdentitySteps() []commandStep {
	return []commandStep{
		gitStep([]string{"config", "--local", "--null", "--includes", "--list"}, ""),
		gitStep([]string{"remote", "get-url", "origin"}, testFetchURLOut),
		gitStep([]string{"remote", "get-url", "--push", "--all", "origin"}, testFetchURLOut),
		ghStep([]string{"repo", "view", "--json", "nameWithOwner,url"}, testRepoViewOut),
	}
}

func prViewArgs() []string {
	return []string{"pr", "view", strconv.Itoa(testPRNumber), "--json", prViewFields, "-R", testRepoArg}
}

func prViewJSON(headName, body string, crossRepository bool) string {
	return fmt.Sprintf(`{"number":%d,"title":"Test PR","state":%q,"headRefName":%q,"headRefOid":%q,"baseRefName":"main","isCrossRepository":%t,"url":"https://github.com/isseis/yt2column/pull/42","body":%q}`,
		testPRNumber, openState, headName, testHeadOID, crossRepository, body)
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

func mergeArgs(subject, body string) []string {
	return []string{"pr", "merge", strconv.Itoa(testPRNumber), "--squash", "--subject", subject, "--body", body, "--match-head-commit", testHeadOID, "-R", testRepoArg}
}

func prepareSteps(logOut, statOut, body string) []commandStep {
	steps := repoIdentitySteps()
	steps = append(steps,
		ghStep(prViewArgs(), prViewJSON("feature/foo", body, false)),
		gitStep([]string{"check-ref-format", "--branch", "feature/foo"}, "feature/foo\n"),
		gitStep([]string{"check-ref-format", "--branch", "main"}, "main\n"),
		gitStep([]string{"status", "--porcelain", "--untracked-files=all"}, ""),
		gitStep([]string{"symbolic-ref", "--quiet", "HEAD"}, "refs/heads/feature/foo\n"),
		baseFreeStep(),
		gitStep([]string{"for-each-ref", "--format=%(refname) %(objectname)", "refs/heads/feature/foo"}, headRefOut(testHeadOID)),
		gitStep([]string{"fetch", testFetchURL, "+refs/heads/main:refs/remotes/origin/main", "+refs/heads/feature/foo:refs/remotes/origin/feature/foo"}, ""),
		gitStep([]string{"diff", "--quiet", "refs/remotes/origin/main", "--", ":(top).claude/commands/mergepr.md"}, ""),
		gitStep([]string{"diff", "--quiet", "refs/remotes/origin/main..." + testHeadOID, "--", ":(top)cmd/mergepr", ":(top)internal/mergepr", ":(top).claude/commands/mergepr.md"}, ""),
		gitStep([]string{"fetch", testFetchURL, testRefsWildcard}, ""),
		ghStep(checksArgs(), ""),
		gitStep([]string{"log", "--no-show-signature", "--format=%h %s%n%n%b", "refs/remotes/origin/main.." + testHeadOID}, logOut),
		gitStep([]string{"diff", "--stat", "refs/remotes/origin/main..." + testHeadOID}, statOut),
	)
	return steps
}

func mergePreflightSteps() []commandStep {
	steps := repoIdentitySteps()
	return append(steps,
		gitStep([]string{"status", "--porcelain", "--untracked-files=all"}, ""),
		gitStep([]string{"symbolic-ref", "--quiet", "HEAD"}, "refs/heads/feature/foo\n"),
		baseFreeStep(),
	)
}

// baseFreeStep lists only this worktree, on the head branch, so no other
// worktree holds the base branch.
func baseFreeStep() commandStep {
	return gitStep([]string{"worktree", "list", "--porcelain"}, "worktree /repo\nHEAD "+testHeadOID+"\nbranch refs/heads/feature/foo\n\n")
}

func mergedViewStep() commandStep {
	return ghStep(mergeViewArgs(), mergeViewOut(mergedState, "feature/foo", testHeadOID, "main", false, true))
}

// cleanupSteps is the cleanup body after the merge view and repository identity
// are already in hand; standalone Cleanup prepends those.
func cleanupSteps(remoteOut, localOID string) []commandStep {
	steps := []commandStep{}
	steps = append(steps, gitStep([]string{"status", "--porcelain", "--untracked-files=all"}, ""))
	steps = append(steps, gitStep([]string{"symbolic-ref", "--quiet", "HEAD"}, "refs/heads/feature/foo\n"))
	steps = append(steps, baseFreeStep())
	steps = append(steps, gitStep([]string{"fetch", testFetchURL, "+refs/heads/main:refs/remotes/origin/main"}, ""))
	steps = append(steps, gitStep([]string{"merge-base", "--is-ancestor", testMergeOID, "refs/remotes/origin/main"}, ""))
	steps = append(steps, gitStep([]string{"ls-remote", "--heads", testFetchURL, "refs/heads/feature/foo"}, remoteOut))
	if remoteOut != "" {
		steps = append(steps,
			gitStep([]string{"push", "--force-with-lease=refs/heads/feature/foo:" + testHeadOID, testFetchURL, "--delete", "refs/heads/feature/foo"}, ""),
		)
	}
	steps = append(steps,
		gitStep([]string{"switch", "--no-overwrite-ignore", "main"}, ""),
		gitStep([]string{"fetch", testFetchURL, "+refs/heads/main:refs/remotes/origin/main"}, ""),
		gitStep([]string{"merge", "--ff-only", "--no-overwrite-ignore", "refs/remotes/origin/main"}, ""),
		gitStep([]string{"rev-parse", "HEAD"}, testBaseOID+"\n"),
		gitStep([]string{"rev-parse", "refs/remotes/origin/main"}, testBaseOID+"\n"),
		gitStep([]string{"for-each-ref", "--format=%(refname) %(objectname)", "refs/heads/feature/foo"}, headRefOut(localOID)),
	)
	if localOID == testHeadOID {
		steps = append(steps,
			gitStep([]string{"worktree", "list", "--porcelain"}, "worktree /repo\nHEAD "+testBaseOID+"\nbranch refs/heads/main\n\n"),
			gitStep([]string{"update-ref", "-d", "refs/heads/feature/foo", testHeadOID}, ""),
		)
	}
	if localOID == "" || localOID == testHeadOID {
		steps = append(steps, gitStep([]string{"fetch", "--prune", testFetchURL, testRefsWildcard}, ""))
	}
	return steps
}

// standaloneCleanupSteps is a full Cleanup run: merge view, identity, body.
func standaloneCleanupSteps(remoteOut, localOID string) []commandStep {
	steps := []commandStep{mergedViewStep()}
	steps = append(steps, repoIdentitySteps()...)
	return append(steps, cleanupSteps(remoteOut, localOID)...)
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
		{"host case", "https://GitHub.com/isseis/yt2column.git", "isseis", "yt2column", false},
		{"ssh url", "ssh://git@github.com/isseis/yt2column.git", "isseis", "yt2column", false},
		{"ssh with password is rejected", "ssh://git:secret@github.com/isseis/yt2column.git", "", "", true},
		{"scp-like", "git@github.com:isseis/yt2column.git", "isseis", "yt2column", false},
		{"scp non-git user is rejected", "ghp_secret@github.com:isseis/yt2column.git", "", "", true},
		{"http is rejected", "http://github.com/isseis/yt2column", "", "", true},
		{"http with credentials is rejected", "http://token@github.com/isseis/yt2column", "", "", true},
		{"https with credentials is rejected", "https://token@github.com/isseis/yt2column.git", "", "", true},
		{"query credentials are rejected", "https://github.com/isseis/yt2column.git?access_token=secret", "", "", true},
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

func TestErrorsDoNotIncludeRemoteURL(t *testing.T) {
	const secret = "ghp_do_not_print"
	_, _, err := parseGitHubRemote("https://" + secret + "@github.com/isseis/yt2column.git")
	if !errors.Is(err, errInvalidRemote) {
		t.Fatalf("parseGitHubRemote credential URL error = %v, want errInvalidRemote", err)
	}
	if strings.Contains(err.Error(), secret) {
		t.Errorf("error %q includes the credential", err)
	}
	_, _, err = parseGitHubRemote("https://" + secret + "@gitlab.com/isseis/yt2column.git")
	if !errors.Is(err, errInvalidRemote) {
		t.Fatalf("parseGitHubRemote other host error = %v, want errInvalidRemote", err)
	}
	if strings.Contains(err.Error(), secret) {
		t.Errorf("error %q includes the credential", err)
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
		{"url different case", "https://github.com/ISSeis/YT2Column/pull/7", 7, false, false},
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

func TestSameRepo(t *testing.T) {
	if !sameRepo("isseis", "yt2column", "ISSeis", "YT2Column") {
		t.Error("sameRepo should compare case-insensitively")
	}
	if sameRepo("isseis", "yt2column", "isseis", "other") {
		t.Error("sameRepo accepted different repositories")
	}
}

func TestPrepareRejectsChangedTool(t *testing.T) {
	steps := repoIdentitySteps()
	steps = append(steps,
		ghStep(prViewArgs(), prViewJSON("feature/foo", "body", false)),
		gitStep([]string{"check-ref-format", "--branch", "feature/foo"}, "feature/foo\n"),
		gitStep([]string{"check-ref-format", "--branch", "main"}, "main\n"),
		gitStep([]string{"status", "--porcelain", "--untracked-files=all"}, ""),
		gitStep([]string{"symbolic-ref", "--quiet", "HEAD"}, "refs/heads/feature/foo\n"),
		baseFreeStep(),
		gitStep([]string{"for-each-ref", "--format=%(refname) %(objectname)", "refs/heads/feature/foo"}, headRefOut(testHeadOID)),
		gitStep([]string{"fetch", testFetchURL, "+refs/heads/main:refs/remotes/origin/main", "+refs/heads/feature/foo:refs/remotes/origin/feature/foo"}, ""),
		gitStep([]string{"diff", "--quiet", "refs/remotes/origin/main", "--", ":(top).claude/commands/mergepr.md"}, ""),
		commandStep{name: gitCommand, args: []string{"diff", "--quiet", "refs/remotes/origin/main..." + testHeadOID, "--", ":(top)cmd/mergepr", ":(top)internal/mergepr", ":(top).claude/commands/mergepr.md"}, err: errors.New("exit status 1")},
	)
	tool, runner := newTool(t, steps)

	if _, err := tool.Prepare(t.Context(), strconv.Itoa(testPRNumber), t.TempDir()); !errors.Is(err, errToolChanged) {
		t.Fatalf("Prepare error = %v, want errToolChanged", err)
	}
	runner.done()
}

func TestLocalHeadOIDUsesExactRef(t *testing.T) {
	t.Run("picks the exact ref", func(t *testing.T) {
		steps := []commandStep{
			gitStep([]string{"for-each-ref", "--format=%(refname) %(objectname)", "refs/heads/feature/foo"},
				"refs/heads/feature/foo/bar "+testOtherOID+"\nrefs/heads/feature/foo "+testHeadOID+"\n"),
		}
		tool, runner := newTool(t, steps)
		oid, err := tool.localHeadOID(t.Context(), "feature/foo")
		if err != nil || oid != testHeadOID {
			t.Fatalf("localHeadOID = (%q, %v), want %q", oid, err, testHeadOID)
		}
		runner.done()
	})
	t.Run("ignores a descendant-only match", func(t *testing.T) {
		steps := []commandStep{
			gitStep([]string{"for-each-ref", "--format=%(refname) %(objectname)", "refs/heads/feature/foo"},
				"refs/heads/feature/foo/bar "+testOtherOID+"\n"),
		}
		tool, runner := newTool(t, steps)
		oid, err := tool.localHeadOID(t.Context(), "feature/foo")
		if err != nil || oid != "" {
			t.Fatalf("localHeadOID = (%q, %v), want an empty OID", oid, err)
		}
		runner.done()
	})
}

func TestPrepareRejectsChangedCommand(t *testing.T) {
	steps := repoIdentitySteps()
	steps = append(steps,
		ghStep(prViewArgs(), prViewJSON("feature/foo", "body", false)),
		gitStep([]string{"check-ref-format", "--branch", "feature/foo"}, "feature/foo\n"),
		gitStep([]string{"check-ref-format", "--branch", "main"}, "main\n"),
		gitStep([]string{"status", "--porcelain", "--untracked-files=all"}, ""),
		gitStep([]string{"symbolic-ref", "--quiet", "HEAD"}, "refs/heads/feature/foo\n"),
		baseFreeStep(),
		gitStep([]string{"for-each-ref", "--format=%(refname) %(objectname)", "refs/heads/feature/foo"}, headRefOut(testHeadOID)),
		gitStep([]string{"fetch", testFetchURL, "+refs/heads/main:refs/remotes/origin/main", "+refs/heads/feature/foo:refs/remotes/origin/feature/foo"}, ""),
		commandStep{name: gitCommand, args: []string{"diff", "--quiet", "refs/remotes/origin/main", "--", ":(top).claude/commands/mergepr.md"}, err: errors.New("exit status 1")},
	)
	tool, runner := newTool(t, steps)

	if _, err := tool.Prepare(t.Context(), strconv.Itoa(testPRNumber), t.TempDir()); !errors.Is(err, errCommandChanged) {
		t.Fatalf("Prepare error = %v, want errCommandChanged", err)
	}
	runner.done()
}

func TestAllowedConfig(t *testing.T) {
	cases := []struct {
		key  string
		want bool
	}{
		{"remote.origin.url", true},
		{"remote.origin.fetch", true},
		{"branch.main.merge", true},
		{"core.repositoryformatversion", true},
		{"core.logallrefupdates", true},
		{"extensions.worktreeConfig", true},
		{"user.email", true},
		{"remote.origin.uploadpack", false},
		{"remote.origin.receivepack", false},
		{"credential.helper", false},
		{"credential.https://github.com.helper", false},
		{"core.sshCommand", false},
		{"core.hooksPath", false},
		{"core.fsmonitor", false},
		{"diff.external", false},
		{"diff.mydriver.textconv", false},
		{"filter.mydriver.clean", false},
		{"alias.x", false},
		{"url.git@github.com:.insteadOf", false},
		{"includeIf.onbranch:main.path", false},
	}
	for _, tc := range cases {
		if got := allowedConfig(tc.key); got != tc.want {
			t.Errorf("allowedConfig(%q) = %t, want %t", tc.key, got, tc.want)
		}
	}
}

func TestPrepareRejectsUnsupportedWorktreeConfig(t *testing.T) {
	for _, value := range []string{"true", "yes", "on", "1"} {
		t.Run(value, func(t *testing.T) {
			steps := []commandStep{
				gitStep([]string{"config", "--local", "--null", "--includes", "--list"}, "extensions.worktreeConfig\n"+value+"\x00"),
				gitStep([]string{"config", "--worktree", "--null", "--list"}, "core.sshCommand\n!./tracked\x00"),
			}
			tool, runner := newTool(t, steps)

			if _, err := tool.Prepare(t.Context(), strconv.Itoa(testPRNumber), t.TempDir()); !errors.Is(err, errUnsupportedConfig) {
				t.Fatalf("Prepare error = %v, want errUnsupportedConfig", err)
			}
			runner.done()
		})
	}
}

func TestPrepareRejectsTempDirInsideWorktree(t *testing.T) {
	root, err := worktreeRoot()
	if err != nil {
		t.Fatalf("worktreeRoot returned error: %v", err)
	}
	t.Setenv("TMPDIR", root)
	t.Cleanup(func() {
		leftovers, _ := filepath.Glob(filepath.Join(root, "mergepr-*"))
		for _, path := range leftovers {
			_ = os.RemoveAll(path)
		}
	})
	tool, runner := newTool(t, nil)

	if _, err := tool.Prepare(t.Context(), strconv.Itoa(testPRNumber), ""); !errors.Is(err, errWorkDirInWorktree) {
		t.Fatalf("Prepare error = %v, want errWorkDirInWorktree", err)
	}
	runner.done()
}

func TestWriteFileEnforcesMode(t *testing.T) {
	path := filepath.Join(t.TempDir(), "file.txt")
	if err := os.WriteFile(path, []byte("old"), 0o644); err != nil {
		t.Fatalf("seed file: %v", err)
	}
	if err := writeFile(path, []byte("new")); err != nil {
		t.Fatalf("writeFile: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Errorf("mode = %o, want 600", got)
	}
}

func TestPrepareRejectsWorkDirInWorktree(t *testing.T) {
	root, err := worktreeRoot()
	if err != nil {
		t.Fatalf("worktreeRoot returned error: %v", err)
	}
	tool, runner := newTool(t, nil)

	if _, err := tool.Prepare(t.Context(), strconv.Itoa(testPRNumber), filepath.Join(root, "mergepr-work")); !errors.Is(err, errWorkDirInWorktree) {
		t.Fatalf("Prepare error = %v, want errWorkDirInWorktree", err)
	}
	runner.done()
}

func TestPrepareRejectsSymlinkedWorkDir(t *testing.T) {
	root, err := worktreeRoot()
	if err != nil {
		t.Fatalf("worktreeRoot returned error: %v", err)
	}
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(filepath.Join(root, "internal"), link); err != nil {
		t.Skipf("cannot create symlink: %v", err)
	}
	tool, runner := newTool(t, nil)

	if _, err := tool.Prepare(t.Context(), strconv.Itoa(testPRNumber), link); !errors.Is(err, errWorkDirInWorktree) {
		t.Fatalf("Prepare error = %v, want errWorkDirInWorktree", err)
	}
	runner.done()
}

func TestPrepareHappyPath(t *testing.T) {
	const (
		logOut  = "abc123 subject\n\nbody\n"
		statOut = " a.txt | 1 +\n"
		body    = "the PR description"
	)
	dir := t.TempDir()
	tool, runner := newTool(t, prepareSteps(logOut, statOut, body))

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
	if prepared.BodyPath != filepath.Join(dir, bodyFileName) {
		t.Errorf("BodyPath = %q, want %q", prepared.BodyPath, filepath.Join(dir, bodyFileName))
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
	if data, err := os.ReadFile(prepared.BodyPath); err != nil || string(data) != body {
		t.Errorf("body file = %q (err %v), want %q", data, err, body)
	}
}

// currentBranchPrepareSteps is prepareSteps for a run without a PR argument:
// the current branch is resolved first and passed to gh as the selector,
// because gh refuses to infer it when -R is given.
func currentBranchPrepareSteps(prOut string) []commandStep {
	steps := repoIdentitySteps()
	steps = append(steps,
		gitStep([]string{"symbolic-ref", "--quiet", "HEAD"}, "refs/heads/feature/foo\n"),
		ghStep([]string{"pr", "view", "feature/foo", "--json", prViewFields, "-R", testRepoArg}, prOut),
	)
	rest := prepareSteps("log\n", "stat\n", "body")
	return append(steps, rest[len(repoIdentitySteps())+1:]...)
}

func TestPrepareCurrentBranch(t *testing.T) {
	tool, runner := newTool(t, currentBranchPrepareSteps(prViewJSON("feature/foo", "body", false)))

	prepared, err := tool.Prepare(t.Context(), "", t.TempDir())
	if err != nil {
		t.Fatalf("Prepare returned error: %v", err)
	}
	runner.done()
	if prepared.State.Number != testPRNumber || prepared.State.HeadRefName != "feature/foo" {
		t.Errorf("state = %+v, want PR #%d on feature/foo", prepared.State, testPRNumber)
	}
}

func TestPrepareCurrentBranchRejectsOtherHead(t *testing.T) {
	steps := repoIdentitySteps()
	steps = append(steps,
		gitStep([]string{"symbolic-ref", "--quiet", "HEAD"}, "refs/heads/feature/foo\n"),
		ghStep([]string{"pr", "view", "feature/foo", "--json", prViewFields, "-R", testRepoArg}, prViewJSON("feature/bar", "body", false)),
	)
	tool, runner := newTool(t, steps)

	if _, err := tool.Prepare(t.Context(), "", t.TempDir()); !errors.Is(err, errInvalidPRArg) {
		t.Fatalf("Prepare error = %v, want errInvalidPRArg", err)
	}
	runner.done()
}

func TestPrepareCreatesNamedWorkDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "new", "work")
	tool, runner := newTool(t, prepareSteps("log\n", "stat\n", "body"))

	prepared, err := tool.Prepare(t.Context(), strconv.Itoa(testPRNumber), dir)
	if err != nil {
		t.Fatalf("Prepare returned error: %v", err)
	}
	runner.done()
	if _, err := os.Stat(prepared.StatePath); err != nil {
		t.Errorf("state file: %v", err)
	}
}

func TestPrepareRejectsPushURLMismatch(t *testing.T) {
	steps := []commandStep{
		gitStep([]string{"config", "--local", "--null", "--includes", "--list"}, ""),
		gitStep([]string{"remote", "get-url", "origin"}, testFetchURLOut),
		gitStep([]string{"remote", "get-url", "--push", "--all", "origin"}, "git@github.com:fork/yt2column.git\n"),
	}
	tool, runner := newTool(t, steps)

	if _, err := tool.Prepare(t.Context(), strconv.Itoa(testPRNumber), t.TempDir()); !errors.Is(err, errRemoteMismatch) {
		t.Fatalf("Prepare error = %v, want errRemoteMismatch", err)
	}
	runner.done()
}

func TestPrepareRejectsExtraPushURL(t *testing.T) {
	steps := []commandStep{
		gitStep([]string{"config", "--local", "--null", "--includes", "--list"}, ""),
		gitStep([]string{"remote", "get-url", "origin"}, testFetchURLOut),
		gitStep([]string{"remote", "get-url", "--push", "--all", "origin"}, "git@github.com:isseis/yt2column.git\ngit@github.com:fork/yt2column.git\n"),
	}
	tool, runner := newTool(t, steps)

	if _, err := tool.Prepare(t.Context(), strconv.Itoa(testPRNumber), t.TempDir()); !errors.Is(err, errRemoteMismatch) {
		t.Fatalf("Prepare error = %v, want errRemoteMismatch", err)
	}
	runner.done()
}

func TestPrepareRejectsUnsupportedConfig(t *testing.T) {
	steps := []commandStep{
		gitStep([]string{"config", "--local", "--null", "--includes", "--list"}, "url.https://ghp_secret@github.com/fork/repo.git#a=b.insteadof\nhttps://github.com/\x00"),
	}
	tool, runner := newTool(t, steps)

	_, err := tool.Prepare(t.Context(), strconv.Itoa(testPRNumber), t.TempDir())
	if !errors.Is(err, errUnsupportedConfig) {
		t.Fatalf("Prepare error = %v, want errUnsupportedConfig", err)
	}
	if strings.Contains(err.Error(), "ghp_secret") {
		t.Errorf("error %q echoes the credential in the config key", err)
	}
	runner.done()
}

func TestPrepareRejectsHTTPRemote(t *testing.T) {
	steps := []commandStep{
		gitStep([]string{"config", "--local", "--null", "--includes", "--list"}, ""),
		gitStep([]string{"remote", "get-url", "origin"}, "http://github.com/isseis/yt2column.git\n"),
	}
	tool, runner := newTool(t, steps)

	if _, err := tool.Prepare(t.Context(), strconv.Itoa(testPRNumber), t.TempDir()); !errors.Is(err, errInvalidRemote) {
		t.Fatalf("Prepare error = %v, want errInvalidRemote", err)
	}
	runner.done()
}

func TestPrepareRejectsSelectedRepoMismatch(t *testing.T) {
	steps := []commandStep{
		gitStep([]string{"config", "--local", "--null", "--includes", "--list"}, ""),
		gitStep([]string{"remote", "get-url", "origin"}, testFetchURLOut),
		gitStep([]string{"remote", "get-url", "--push", "--all", "origin"}, testFetchURLOut),
		ghStep([]string{"repo", "view", "--json", "nameWithOwner,url"}, `{"nameWithOwner":"other/repo","url":"https://github.com/other/repo"}`),
	}
	tool, runner := newTool(t, steps)

	if _, err := tool.Prepare(t.Context(), strconv.Itoa(testPRNumber), t.TempDir()); !errors.Is(err, errRepoMismatch) {
		t.Fatalf("Prepare error = %v, want errRepoMismatch", err)
	}
	runner.done()
}

func TestPrepareRejectsGHSelHostMismatch(t *testing.T) {
	steps := []commandStep{
		gitStep([]string{"config", "--local", "--null", "--includes", "--list"}, ""),
		gitStep([]string{"remote", "get-url", "origin"}, testFetchURLOut),
		gitStep([]string{"remote", "get-url", "--push", "--all", "origin"}, testFetchURLOut),
		ghStep([]string{"repo", "view", "--json", "nameWithOwner,url"}, `{"nameWithOwner":"isseis/yt2column","url":"https://github.example.com/isseis/yt2column"}`),
	}
	tool, runner := newTool(t, steps)

	if _, err := tool.Prepare(t.Context(), strconv.Itoa(testPRNumber), t.TempDir()); !errors.Is(err, errRepoMismatch) {
		t.Fatalf("Prepare error = %v, want errRepoMismatch", err)
	}
	runner.done()
}

func TestPrepareRejectsUnsafeHeadName(t *testing.T) {
	steps := repoIdentitySteps()
	steps = append(steps, ghStep(prViewArgs(), prViewJSON("-B", "body", false)))
	tool, runner := newTool(t, steps)

	if _, err := tool.Prepare(t.Context(), strconv.Itoa(testPRNumber), t.TempDir()); !errors.Is(err, errInvalidBranch) {
		t.Fatalf("Prepare error = %v, want errInvalidBranch", err)
	}
	runner.done()
}

func TestPrepareRejectsGitInvalidName(t *testing.T) {
	steps := repoIdentitySteps()
	steps = append(steps,
		ghStep(prViewArgs(), prViewJSON("feature/foo", "body", false)),
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
	steps = append(steps, ghStep(prViewArgs(), prViewJSON("feature/foo", "body", true)))
	tool, runner := newTool(t, steps)

	if _, err := tool.Prepare(t.Context(), strconv.Itoa(testPRNumber), t.TempDir()); !errors.Is(err, errCrossRepository) {
		t.Fatalf("Prepare error = %v, want errCrossRepository", err)
	}
	runner.done()
}

func TestPrepareRejectsDirtyWorktree(t *testing.T) {
	steps := repoIdentitySteps()
	steps = append(steps,
		ghStep(prViewArgs(), prViewJSON("feature/foo", "body", false)),
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

func TestPrepareRejectsDetachedHead(t *testing.T) {
	steps := repoIdentitySteps()
	steps = append(steps,
		ghStep(prViewArgs(), prViewJSON("feature/foo", "body", false)),
		gitStep([]string{"check-ref-format", "--branch", "feature/foo"}, "feature/foo\n"),
		gitStep([]string{"check-ref-format", "--branch", "main"}, "main\n"),
		gitStep([]string{"status", "--porcelain", "--untracked-files=all"}, ""),
		commandStep{name: gitCommand, args: []string{"symbolic-ref", "--quiet", "HEAD"}, err: errors.New("exit status 1")},
	)
	tool, runner := newTool(t, steps)

	if _, err := tool.Prepare(t.Context(), strconv.Itoa(testPRNumber), t.TempDir()); !errors.Is(err, errDetachedHead) {
		t.Fatalf("Prepare error = %v, want errDetachedHead", err)
	}
	runner.done()
}

func TestPrepareRejectsHeadBranchMismatch(t *testing.T) {
	steps := repoIdentitySteps()
	steps = append(steps,
		ghStep(prViewArgs(), prViewJSON("feature/foo", "body", false)),
		gitStep([]string{"check-ref-format", "--branch", "feature/foo"}, "feature/foo\n"),
		gitStep([]string{"check-ref-format", "--branch", "main"}, "main\n"),
		gitStep([]string{"status", "--porcelain", "--untracked-files=all"}, ""),
		gitStep([]string{"symbolic-ref", "--quiet", "HEAD"}, "refs/heads/feature/foo\n"),
		baseFreeStep(),
		gitStep([]string{"for-each-ref", "--format=%(refname) %(objectname)", "refs/heads/feature/foo"}, headRefOut(testOtherOID)),
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
		ghStep(prViewArgs(), prViewJSON("feature/foo", "body", false)),
		gitStep([]string{"check-ref-format", "--branch", "feature/foo"}, "feature/foo\n"),
		gitStep([]string{"check-ref-format", "--branch", "main"}, "main\n"),
		gitStep([]string{"status", "--porcelain", "--untracked-files=all"}, ""),
		gitStep([]string{"symbolic-ref", "--quiet", "HEAD"}, "refs/heads/feature/foo\n"),
		baseFreeStep(),
		gitStep([]string{"for-each-ref", "--format=%(refname) %(objectname)", "refs/heads/feature/foo"}, headRefOut(testHeadOID)),
		gitStep([]string{"fetch", testFetchURL, "+refs/heads/main:refs/remotes/origin/main", "+refs/heads/feature/foo:refs/remotes/origin/feature/foo"}, ""),
		gitStep([]string{"diff", "--quiet", "refs/remotes/origin/main", "--", ":(top).claude/commands/mergepr.md"}, ""),
		gitStep([]string{"diff", "--quiet", "refs/remotes/origin/main..." + testHeadOID, "--", ":(top)cmd/mergepr", ":(top)internal/mergepr", ":(top).claude/commands/mergepr.md"}, ""),
		gitStep([]string{"fetch", testFetchURL, testRefsWildcard}, ""),
		commandStep{name: ghCommand, args: checksArgs(), err: errors.New("exit status 1")},
	)
	tool, runner := newTool(t, steps)

	if _, err := tool.Prepare(t.Context(), strconv.Itoa(testPRNumber), t.TempDir()); !errors.Is(err, errChecksFailed) {
		t.Fatalf("Prepare error = %v, want errChecksFailed", err)
	}
	runner.done()
}

func TestPrepareRejectsOversizedLog(t *testing.T) {
	steps := prepareSteps(strings.Repeat("x", maxLogBytes+1), "", "body")
	steps = steps[:len(steps)-1] // the diff does not run once the log is too large
	tool, runner := newTool(t, steps)

	if _, err := tool.Prepare(t.Context(), strconv.Itoa(testPRNumber), t.TempDir()); !errors.Is(err, errTooLarge) {
		t.Fatalf("Prepare error = %v, want errTooLarge", err)
	}
	runner.done()
}

func TestPrepareRejectsOversizedBody(t *testing.T) {
	steps := prepareSteps("", "", strings.Repeat("x", maxBodyBytes+1))
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

	steps := mergePreflightSteps()
	steps = append(steps,
		ghStep(mergeViewArgs(), mergeViewOut(openState, "feature/foo", testHeadOID, "main", false, false)),
		ghStep(checksArgs(), ""),
		ghStep(mergeViewArgs(), mergeViewOut(openState, "feature/foo", testHeadOID, "main", false, false)),
		ghStep(mergeArgs("subject line", "body text\n"), ""),
		mergedViewStep(),
	)
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

func TestMergeRejectsRepoMismatch(t *testing.T) {
	dir := t.TempDir()
	statePath := writeStateFile(t, dir)
	subjectPath := writeTempFile(t, dir, "subject.txt", "subject line\n")
	bodyPath := writeTempFile(t, dir, "body.txt", "body\n")

	steps := []commandStep{
		gitStep([]string{"config", "--local", "--null", "--includes", "--list"}, ""),
		gitStep([]string{"remote", "get-url", "origin"}, "git@github.com:fork/yt2column.git\n"),
		gitStep([]string{"remote", "get-url", "--push", "--all", "origin"}, "git@github.com:fork/yt2column.git\n"),
		ghStep([]string{"repo", "view", "--json", "nameWithOwner,url"}, `{"nameWithOwner":"fork/yt2column","url":"https://github.com/fork/yt2column"}`),
	}
	tool, runner := newTool(t, steps)

	if _, err := tool.Merge(t.Context(), statePath, subjectPath, bodyPath); !errors.Is(err, errRepoMismatch) {
		t.Fatalf("Merge error = %v, want errRepoMismatch", err)
	}
	runner.done()
}

func TestMergeRejectsDirtyWorktree(t *testing.T) {
	dir := t.TempDir()
	statePath := writeStateFile(t, dir)
	subjectPath := writeTempFile(t, dir, "subject.txt", "subject line\n")
	bodyPath := writeTempFile(t, dir, "body.txt", "body\n")

	steps := repoIdentitySteps()
	steps = append(steps, gitStep([]string{"status", "--porcelain", "--untracked-files=all"}, "?? edit.txt\n"))
	tool, runner := newTool(t, steps)

	if _, err := tool.Merge(t.Context(), statePath, subjectPath, bodyPath); !errors.Is(err, errDirtyWorktree) {
		t.Fatalf("Merge error = %v, want errDirtyWorktree", err)
	}
	runner.done()
}

func TestMergeRejectsHeadDrift(t *testing.T) {
	dir := t.TempDir()
	statePath := writeStateFile(t, dir)
	subjectPath := writeTempFile(t, dir, "subject.txt", "subject line\n")
	bodyPath := writeTempFile(t, dir, "body.txt", "body\n")

	steps := mergePreflightSteps()
	steps = append(steps, ghStep(mergeViewArgs(), mergeViewOut(openState, "feature/foo", testOtherOID, "main", false, false)))
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

	steps := mergePreflightSteps()
	steps = append(steps, ghStep(mergeViewArgs(), mergeViewOut(openState, "other", testHeadOID, "main", false, false)))
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

	steps := mergePreflightSteps()
	steps = append(steps, ghStep(mergeViewArgs(), mergeViewOut(openState, "feature/foo", testHeadOID, "release", false, false)))
	tool, runner := newTool(t, steps)

	if _, err := tool.Merge(t.Context(), statePath, subjectPath, bodyPath); !errors.Is(err, errBaseDrift) {
		t.Fatalf("Merge error = %v, want errBaseDrift", err)
	}
	runner.done()
}

func TestMergeRejectsDriftAfterChecks(t *testing.T) {
	dir := t.TempDir()
	statePath := writeStateFile(t, dir)
	subjectPath := writeTempFile(t, dir, "subject.txt", "subject line\n")
	bodyPath := writeTempFile(t, dir, "body.txt", "body\n")

	steps := mergePreflightSteps()
	steps = append(steps,
		ghStep(mergeViewArgs(), mergeViewOut(openState, "feature/foo", testHeadOID, "main", false, false)),
		ghStep(checksArgs(), ""),
		ghStep(mergeViewArgs(), mergeViewOut(openState, "feature/foo", testHeadOID, "release", false, false)),
	)
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

	steps := mergePreflightSteps()
	steps = append(steps, ghStep(mergeViewArgs(), mergeViewOut(openState, "feature/foo", testHeadOID, "main", true, false)))
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

	steps := mergePreflightSteps()
	steps = append(steps,
		ghStep(mergeViewArgs(), mergeViewOut(openState, "feature/foo", testHeadOID, "main", false, false)),
		commandStep{name: ghCommand, args: checksArgs(), err: errors.New("exit status 1")},
	)
	tool, runner := newTool(t, steps)

	if _, err := tool.Merge(t.Context(), statePath, subjectPath, bodyPath); !errors.Is(err, errChecksFailed) {
		t.Fatalf("Merge error = %v, want errChecksFailed", err)
	}
	runner.done()
}

func TestMergeReportsQueued(t *testing.T) {
	dir := t.TempDir()
	statePath := writeStateFile(t, dir)
	subjectPath := writeTempFile(t, dir, "subject.txt", "subject line\n")
	bodyPath := writeTempFile(t, dir, "body.txt", "body text\n")

	steps := mergePreflightSteps()
	steps = append(steps,
		ghStep(mergeViewArgs(), mergeViewOut(openState, "feature/foo", testHeadOID, "main", false, false)),
		ghStep(checksArgs(), ""),
		ghStep(mergeViewArgs(), mergeViewOut(openState, "feature/foo", testHeadOID, "main", false, false)),
		ghStep(mergeArgs("subject line", "body text\n"), ""),
		ghStep(mergeViewArgs(), mergeViewOut(openState, "feature/foo", testHeadOID, "main", false, false)),
	)
	tool, runner := newTool(t, steps)

	if _, err := tool.Merge(t.Context(), statePath, subjectPath, bodyPath); !errors.Is(err, errMergeQueued) {
		t.Fatalf("Merge error = %v, want errMergeQueued", err)
	}
	runner.done()
}

func TestMergeReportsCleanupFailure(t *testing.T) {
	dir := t.TempDir()
	statePath := writeStateFile(t, dir)
	subjectPath := writeTempFile(t, dir, "subject.txt", "subject line\n")
	bodyPath := writeTempFile(t, dir, "body.txt", "body text\n")

	steps := mergePreflightSteps()
	steps = append(steps,
		ghStep(mergeViewArgs(), mergeViewOut(openState, "feature/foo", testHeadOID, "main", false, false)),
		ghStep(checksArgs(), ""),
		ghStep(mergeViewArgs(), mergeViewOut(openState, "feature/foo", testHeadOID, "main", false, false)),
		ghStep(mergeArgs("subject line", "body text\n"), ""),
		ghStep(mergeViewArgs(), mergeViewOut(mergedState, "feature/foo", testHeadOID, "main", false, true)),
		gitStep([]string{"status", "--porcelain", "--untracked-files=all"}, ""),
		gitStep([]string{"symbolic-ref", "--quiet", "HEAD"}, "refs/heads/feature/foo\n"),
		baseFreeStep(),
		gitStep([]string{"fetch", testFetchURL, "+refs/heads/main:refs/remotes/origin/main"}, ""),
		gitStep([]string{"merge-base", "--is-ancestor", testMergeOID, "refs/remotes/origin/main"}, ""),
		gitStep([]string{"ls-remote", "--heads", testFetchURL, "refs/heads/feature/foo"}, testRemotePresent),
		commandStep{name: gitCommand, args: []string{"push", "--force-with-lease=refs/heads/feature/foo:" + testHeadOID, testFetchURL, "--delete", "refs/heads/feature/foo"}, err: errors.New("exit status 1")},
	)
	tool, runner := newTool(t, steps)

	_, err := tool.Merge(t.Context(), statePath, subjectPath, bodyPath)
	if !errors.Is(err, errMergeCleanup) {
		t.Fatalf("Merge error = %v, want errMergeCleanup", err)
	}
	if !strings.Contains(err.Error(), testMergeOID) {
		t.Errorf("Merge error %q does not report the merge commit", err)
	}
	runner.done()
}

func TestMergeResumesCleanupWhenMerged(t *testing.T) {
	dir := t.TempDir()
	statePath := writeStateFile(t, dir)
	subjectPath := writeTempFile(t, dir, "subject.txt", "subject line\n")
	bodyPath := writeTempFile(t, dir, "body.txt", "body\n")

	steps := mergePreflightSteps()
	steps = append(steps, mergedViewStep())
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

func TestMergeRejectsOversizedBody(t *testing.T) {
	dir := t.TempDir()
	statePath := writeStateFile(t, dir)
	subjectPath := writeTempFile(t, dir, "subject.txt", "subject line\n")
	bodyPath := writeTempFile(t, dir, "body.txt", strings.Repeat("x", maxBodyBytes+1))

	tool, runner := newTool(t, nil)
	if _, err := tool.Merge(t.Context(), statePath, subjectPath, bodyPath); !errors.Is(err, errTooLarge) {
		t.Fatalf("Merge error = %v, want errTooLarge", err)
	}
	runner.done()
}

func TestCleanupRejectsDroppedMerge(t *testing.T) {
	dir := t.TempDir()
	statePath := writeStateFile(t, dir)
	steps := []commandStep{mergedViewStep()}
	steps = append(steps, repoIdentitySteps()...)
	steps = append(steps,
		gitStep([]string{"status", "--porcelain", "--untracked-files=all"}, ""),
		gitStep([]string{"symbolic-ref", "--quiet", "HEAD"}, "refs/heads/feature/foo\n"),
		baseFreeStep(),
		gitStep([]string{"fetch", testFetchURL, "+refs/heads/main:refs/remotes/origin/main"}, ""),
		commandStep{name: gitCommand, args: []string{"merge-base", "--is-ancestor", testMergeOID, "refs/remotes/origin/main"}, err: errors.New("exit status 1")},
	)
	tool, runner := newTool(t, steps)

	if _, err := tool.Cleanup(t.Context(), statePath); !errors.Is(err, errMergeMissing) {
		t.Fatalf("Cleanup error = %v, want errMergeMissing", err)
	}
	runner.done()
}

func TestCleanupRemoteGone(t *testing.T) {
	dir := t.TempDir()
	statePath := writeStateFile(t, dir)
	tool, runner := newTool(t, standaloneCleanupSteps("", testHeadOID))

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
	tool, runner := newTool(t, standaloneCleanupSteps(testRemotePresent, testOtherOID))

	if _, err := tool.Cleanup(t.Context(), statePath); !errors.Is(err, errLocalBranchDrift) {
		t.Fatalf("Cleanup error = %v, want errLocalBranchDrift", err)
	}
	runner.done()
}

func TestCleanupRejectsDirtyWorktree(t *testing.T) {
	dir := t.TempDir()
	statePath := writeStateFile(t, dir)
	steps := []commandStep{mergedViewStep()}
	steps = append(steps, repoIdentitySteps()...)
	steps = append(steps, gitStep([]string{"status", "--porcelain", "--untracked-files=all"}, "?? edit.txt\n"))
	tool, runner := newTool(t, steps)

	if _, err := tool.Cleanup(t.Context(), statePath); !errors.Is(err, errDirtyWorktree) {
		t.Fatalf("Cleanup error = %v, want errDirtyWorktree", err)
	}
	runner.done()
}

func TestCleanupRejectsDetachedHead(t *testing.T) {
	dir := t.TempDir()
	statePath := writeStateFile(t, dir)
	steps := []commandStep{mergedViewStep()}
	steps = append(steps, repoIdentitySteps()...)
	steps = append(steps,
		gitStep([]string{"status", "--porcelain", "--untracked-files=all"}, ""),
		commandStep{name: gitCommand, args: []string{"symbolic-ref", "--quiet", "HEAD"}, err: errors.New("exit status 1")},
	)
	tool, runner := newTool(t, steps)

	if _, err := tool.Cleanup(t.Context(), statePath); !errors.Is(err, errDetachedHead) {
		t.Fatalf("Cleanup error = %v, want errDetachedHead", err)
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
		gitStep([]string{"symbolic-ref", "--quiet", "HEAD"}, "refs/heads/feature/foo\n"),
		baseFreeStep(),
		gitStep([]string{"fetch", testFetchURL, "+refs/heads/main:refs/remotes/origin/main"}, ""),
		gitStep([]string{"merge-base", "--is-ancestor", testMergeOID, "refs/remotes/origin/main"}, ""),
		gitStep([]string{"ls-remote", "--heads", testFetchURL, "refs/heads/feature/foo"}, testRemotePresent),
		gitStep([]string{"push", "--force-with-lease=refs/heads/feature/foo:" + testHeadOID, testFetchURL, "--delete", "refs/heads/feature/foo"}, ""),
		gitStep([]string{"switch", "--no-overwrite-ignore", "main"}, ""),
		gitStep([]string{"fetch", testFetchURL, "+refs/heads/main:refs/remotes/origin/main"}, ""),
		gitStep([]string{"merge", "--ff-only", "--no-overwrite-ignore", "refs/remotes/origin/main"}, ""),
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

func TestCleanupRejectsCheckedOutLocalBranch(t *testing.T) {
	dir := t.TempDir()
	statePath := writeStateFile(t, dir)
	steps := []commandStep{mergedViewStep()}
	steps = append(steps, repoIdentitySteps()...)
	steps = append(steps,
		gitStep([]string{"status", "--porcelain", "--untracked-files=all"}, ""),
		gitStep([]string{"symbolic-ref", "--quiet", "HEAD"}, "refs/heads/feature/foo\n"),
		baseFreeStep(),
		gitStep([]string{"fetch", testFetchURL, "+refs/heads/main:refs/remotes/origin/main"}, ""),
		gitStep([]string{"merge-base", "--is-ancestor", testMergeOID, "refs/remotes/origin/main"}, ""),
		gitStep([]string{"ls-remote", "--heads", testFetchURL, "refs/heads/feature/foo"}, testRemotePresent),
		gitStep([]string{"push", "--force-with-lease=refs/heads/feature/foo:" + testHeadOID, testFetchURL, "--delete", "refs/heads/feature/foo"}, ""),
		gitStep([]string{"switch", "--no-overwrite-ignore", "main"}, ""),
		gitStep([]string{"fetch", testFetchURL, "+refs/heads/main:refs/remotes/origin/main"}, ""),
		gitStep([]string{"merge", "--ff-only", "--no-overwrite-ignore", "refs/remotes/origin/main"}, ""),
		gitStep([]string{"rev-parse", "HEAD"}, testBaseOID+"\n"),
		gitStep([]string{"rev-parse", "refs/remotes/origin/main"}, testBaseOID+"\n"),
		gitStep([]string{"for-each-ref", "--format=%(refname) %(objectname)", "refs/heads/feature/foo"}, headRefOut(testHeadOID)),
		gitStep([]string{"worktree", "list", "--porcelain"}, "worktree /repo\nHEAD "+testHeadOID+"\nbranch refs/heads/feature/foo\n\n"),
	)
	tool, runner := newTool(t, steps)

	if _, err := tool.Cleanup(t.Context(), statePath); !errors.Is(err, errBranchCheckedOut) {
		t.Fatalf("Cleanup error = %v, want errBranchCheckedOut", err)
	}
	runner.done()
}

func TestDiffHappyPath(t *testing.T) {
	dir := t.TempDir()
	statePath := writeStateFile(t, dir)
	const patch = "diff --git a/a.txt b/a.txt\n"
	steps := []commandStep{gitStep([]string{"--literal-pathspecs", "diff", "--no-ext-diff", "--no-textconv", "refs/remotes/origin/main..." + testHeadOID, "--", "a.txt"}, patch)}
	tool, runner := newTool(t, steps)

	got, err := tool.Diff(t.Context(), statePath, "a.txt")
	if err != nil {
		t.Fatalf("Diff returned error: %v", err)
	}
	runner.done()
	if string(got) != patch {
		t.Errorf("Diff = %q, want %q", got, patch)
	}
}

func TestDiffRejectsEmptyPath(t *testing.T) {
	dir := t.TempDir()
	statePath := writeStateFile(t, dir)
	tool, runner := newTool(t, nil)

	if _, err := tool.Diff(t.Context(), statePath, ""); !errors.Is(err, errInvalidPath) {
		t.Fatalf("Diff error = %v, want errInvalidPath", err)
	}
	runner.done()
}

func TestDiffAllowsDashPath(t *testing.T) {
	dir := t.TempDir()
	statePath := writeStateFile(t, dir)
	const patch = "diff --git a/-notes.md b/-notes.md\n"
	steps := []commandStep{gitStep([]string{"--literal-pathspecs", "diff", "--no-ext-diff", "--no-textconv", "refs/remotes/origin/main..." + testHeadOID, "--", "-notes.md"}, patch)}
	tool, runner := newTool(t, steps)

	got, err := tool.Diff(t.Context(), statePath, "-notes.md")
	if err != nil {
		t.Fatalf("Diff returned error: %v", err)
	}
	runner.done()
	if string(got) != patch {
		t.Errorf("Diff = %q, want %q", got, patch)
	}
}

func TestDiffRejectsOversizedDiff(t *testing.T) {
	dir := t.TempDir()
	statePath := writeStateFile(t, dir)
	steps := []commandStep{gitStep([]string{"--literal-pathspecs", "diff", "--no-ext-diff", "--no-textconv", "refs/remotes/origin/main..." + testHeadOID, "--", "a.txt"}, strings.Repeat("x", maxDiffBytes+1))}
	tool, runner := newTool(t, steps)

	if _, err := tool.Diff(t.Context(), statePath, "a.txt"); !errors.Is(err, errTooLarge) {
		t.Fatalf("Diff error = %v, want errTooLarge", err)
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

// TestRequireToolUnchangedFromSubdirectory runs the tool check against a real
// repository from a subdirectory: Git resolves an unanchored pathspec against
// the current directory, so a PR that changes cmd/mergepr would pass there.
func TestRequireToolUnchangedFromSubdirectory(t *testing.T) {
	if _, err := exec.LookPath(gitCommand); err != nil {
		t.Skip("git is not on PATH")
	}
	repo := t.TempDir()
	runner := NewOSRunner()
	git := func(args ...string) string {
		t.Helper()
		full := append([]string{"-C", repo, "-c", "user.name=Test", "-c", "user.email=test@example.com"}, args...)
		out, err := runner.Run(t.Context(), gitCommand, full...)
		if err != nil {
			t.Fatalf("git %v: %v", args, err)
		}
		return strings.TrimSpace(string(out))
	}
	write := func(rel, content string) {
		t.Helper()
		path := filepath.Join(repo, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatalf("mkdir %s: %v", rel, err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}

	git("init", "-q", "-b", trustedBranch)
	write(commandFile, "command\n")
	write("cmd/mergepr/main.go", "package main\n")
	write("internal/keep.txt", "keep\n")
	git("add", "-A")
	git("commit", "-q", "-m", "base")
	git("switch", "-q", "-c", "feature")
	write("cmd/mergepr/main.go", "package main\n\n// changed\n")
	git("commit", "-q", "-am", "change the tool")
	headOID := git("rev-parse", "HEAD")
	git("switch", "-q", trustedBranch)

	t.Chdir(filepath.Join(repo, "internal"))
	tool, err := New(runner)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	err = tool.requireToolUnchanged(t.Context(), identity{FetchURL: repo}, trustedBranch, "feature", headOID)
	if !errors.Is(err, errToolChanged) {
		t.Fatalf("requireToolUnchanged error = %v, want errToolChanged", err)
	}
}

// baseElsewhereStep lists a second worktree that has the base branch checked
// out, which git switch would refuse.
func baseElsewhereStep() commandStep {
	return gitStep([]string{"worktree", "list", "--porcelain"},
		"worktree /repo\nHEAD "+testHeadOID+"\nbranch refs/heads/feature/foo\n\nworktree /other\nHEAD "+testBaseOID+"\nbranch refs/heads/main\n\n")
}

func TestPrepareRejectsBaseCheckedOutElsewhere(t *testing.T) {
	steps := repoIdentitySteps()
	steps = append(steps,
		ghStep(prViewArgs(), prViewJSON("feature/foo", "body", false)),
		gitStep([]string{"check-ref-format", "--branch", "feature/foo"}, "feature/foo\n"),
		gitStep([]string{"check-ref-format", "--branch", "main"}, "main\n"),
		gitStep([]string{"status", "--porcelain", "--untracked-files=all"}, ""),
		gitStep([]string{"symbolic-ref", "--quiet", "HEAD"}, "refs/heads/feature/foo\n"),
		baseElsewhereStep(),
	)
	tool, runner := newTool(t, steps)

	if _, err := tool.Prepare(t.Context(), strconv.Itoa(testPRNumber), t.TempDir()); !errors.Is(err, errBaseCheckedOut) {
		t.Fatalf("Prepare error = %v, want errBaseCheckedOut", err)
	}
	runner.done()
}

// Running from the worktree that holds the base is the supported way around
// errBaseCheckedOut, so it must not list worktrees at all.
func TestPrepareOnBaseBranch(t *testing.T) {
	steps := prepareSteps("log\n", "stat\n", "body")
	for i, step := range steps {
		if step.name == gitCommand && step.args[0] == "symbolic-ref" {
			steps[i].out = "refs/heads/main\n"
			steps = append(steps[:i+1], steps[i+2:]...) // drop baseFreeStep
			break
		}
	}
	tool, runner := newTool(t, steps)

	if _, err := tool.Prepare(t.Context(), strconv.Itoa(testPRNumber), t.TempDir()); err != nil {
		t.Fatalf("Prepare returned error: %v", err)
	}
	runner.done()
}

func TestMergeRejectsBaseCheckedOutElsewhere(t *testing.T) {
	dir := t.TempDir()
	statePath := writeStateFile(t, dir)
	subjectPath := writeTempFile(t, dir, "subject.txt", "subject line\n")
	bodyPath := writeTempFile(t, dir, "body.txt", "body\n")

	steps := repoIdentitySteps()
	steps = append(steps,
		gitStep([]string{"status", "--porcelain", "--untracked-files=all"}, ""),
		gitStep([]string{"symbolic-ref", "--quiet", "HEAD"}, "refs/heads/feature/foo\n"),
		baseElsewhereStep(),
	)
	tool, runner := newTool(t, steps)

	if _, err := tool.Merge(t.Context(), statePath, subjectPath, bodyPath); !errors.Is(err, errBaseCheckedOut) {
		t.Fatalf("Merge error = %v, want errBaseCheckedOut", err)
	}
	runner.done()
}

func TestCleanupRejectsBaseCheckedOutElsewhere(t *testing.T) {
	dir := t.TempDir()
	statePath := writeStateFile(t, dir)
	steps := []commandStep{mergedViewStep()}
	steps = append(steps, repoIdentitySteps()...)
	steps = append(steps,
		gitStep([]string{"status", "--porcelain", "--untracked-files=all"}, ""),
		gitStep([]string{"symbolic-ref", "--quiet", "HEAD"}, "refs/heads/feature/foo\n"),
		baseElsewhereStep(),
	)
	tool, runner := newTool(t, steps)

	if _, err := tool.Cleanup(t.Context(), statePath); !errors.Is(err, errBaseCheckedOut) {
		t.Fatalf("Cleanup error = %v, want errBaseCheckedOut", err)
	}
	runner.done()
}
