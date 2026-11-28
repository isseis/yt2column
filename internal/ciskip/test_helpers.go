//go:build test

package ciskip

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/isseis/yt2column/internal/cmdrunner"
	cmdrunnertestutil "github.com/isseis/yt2column/internal/cmdrunner/testutil"
)

const (
	testGitCmd      = "git"
	testRepo        = "isseis/yt2column"
	testPRNumber    = "1073"
	testSHA1        = "1111111111111111111111111111111111111111"
	testSHA2        = "2222222222222222222222222222222222222222"
	testSHA3        = "3333333333333333333333333333333333333333"
	testSHA4        = "4444444444444444444444444444444444444444"
	testRunID       = int64(5001)
	testCheckJobID  = int64(9001)
	testRepoDirMode = 0o750
)

// gitArgs builds the full argument list runGit passes to cmdrunner.Runner.
func gitArgs(args ...string) []string {
	return append(cmdrunner.GitHardening(), args...)
}

// writeTempFile writes content to a file inside t.TempDir() and returns its path.
func writeTempFile(t *testing.T, name string, content []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	return path
}

// testGitRepo is an isolated temporary git repository for integration tests.
// HOME and XDG_CONFIG_HOME point at an empty directory under t.TempDir() so the
// user's global git configuration cannot affect commits or diffs.
type testGitRepo struct {
	t    *testing.T
	dir  string
	home string
}

func newTestGitRepo(t *testing.T) *testGitRepo {
	t.Helper()
	root := t.TempDir()
	r := &testGitRepo{
		t:    t,
		dir:  filepath.Join(root, "repo"),
		home: filepath.Join(root, "home"),
	}
	if err := os.MkdirAll(r.home, 0o700); err != nil {
		t.Fatalf("create home: %v", err)
	}
	t.Setenv("HOME", r.home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(r.home, ".config"))
	r.gitIn(root, "init", "-q", "--template=", "-b", "main", r.dir)
	r.git("config", "user.name", "Test")
	r.git("config", "user.email", "test@example.com")
	r.git("config", "commit.gpgSign", "false")
	r.git("config", "core.ignorecase", "false")
	return r
}

func (r *testGitRepo) git(args ...string) string {
	r.t.Helper()
	return r.gitIn(r.dir, args...)
}

func (r *testGitRepo) gitIn(dir string, args ...string) string {
	r.t.Helper()
	cmd := exec.Command("git", args...) //nolint:gosec // test setup invokes git with fixed arguments inside t.TempDir
	cmd.Dir = dir
	cmd.Env = cmdrunner.GitEnv(os.Environ())
	out, err := cmd.Output()
	if err != nil {
		stderr := ""
		if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
			stderr = string(exitErr.Stderr)
		}
		r.t.Fatalf("git %v: %v: %s", args, err, stderr)
	}
	return strings.TrimSuffix(string(out), "\n")
}

// commit builds a commit whose tree holds exactly files (slash-separated path -> content)
// with the given parents (none creates a root commit) and returns its 40-char SHA.
func (r *testGitRepo) commit(subject string, files map[string]string, parents ...string) string {
	r.t.Helper()
	entries, err := os.ReadDir(r.dir)
	if err != nil {
		r.t.Fatalf("read repo: %v", err)
	}
	for _, e := range entries {
		if e.Name() == ".git" {
			continue
		}
		if err := os.RemoveAll(filepath.Join(r.dir, e.Name())); err != nil {
			r.t.Fatalf("clear work tree: %v", err)
		}
	}
	for name, content := range files {
		path := filepath.Join(r.dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), testRepoDirMode); err != nil {
			r.t.Fatalf("mkdir for %q: %v", name, err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil { //nolint:gosec // test setup writes inside t.TempDir
			r.t.Fatalf("write %q: %v", name, err)
		}
	}
	r.git("add", "-A")
	tree := r.git("write-tree")
	args := []string{"commit-tree", tree, "-m", subject}
	for _, p := range parents {
		args = append(args, "-p", p)
	}
	return r.git(args...)
}

// setHead detaches HEAD at oid.
func (r *testGitRepo) setHead(oid string) {
	r.t.Helper()
	r.git("update-ref", "--no-deref", "HEAD", oid)
}

// setOriginMain points refs/remotes/origin/main at oid.
func (r *testGitRepo) setOriginMain(oid string) {
	r.t.Helper()
	r.git("update-ref", "refs/remotes/origin/main", oid)
}

// newRealScriptRunner returns a real cmdrunner.InputRunner rooted at the
// repository root, reading hasExtensionChangesScript from the test process so
// Go's test cache invalidates when the script changes.
func newRealScriptRunner(t *testing.T) cmdrunner.InputRunner {
	t.Helper()
	repoRoot := filepath.Join("..", "..")
	// Read the script from this process before running bash: a file the test
	// process opens is part of the test's cache key, so editing the script
	// alone cannot leave a cached PASS behind.
	scriptPath := filepath.Join(repoRoot, filepath.FromSlash(hasExtensionChangesScript))
	if _, err := os.ReadFile(scriptPath); err != nil { //nolint:gosec // test helper reads the fixed repository script path
		t.Fatalf("read %s: %v", hasExtensionChangesScript, err)
	}
	_, scriptOpts, _ := runnerOptions(os.Environ(), defaultLimits())
	scriptOpts.Dir = repoRoot
	return cmdrunner.NewOS(scriptOpts)
}

// runners returns real cmdrunner instances: git pointed at r.dir and script pointed
// at the repository root (two levels above internal/ciskip).
func (r *testGitRepo) runners() (cmdrunner.Runner, cmdrunner.InputRunner) {
	r.t.Helper()
	gitOpts, _, _ := runnerOptions(os.Environ(), defaultLimits())
	gitOpts.Dir = r.dir
	return cmdrunner.NewOS(gitOpts), newRealScriptRunner(r.t)
}

// ghAPIArgs builds the full argument list ghAPIGet passes to cmdrunner.Runner.
func ghAPIArgs(endpoint string) []string {
	return []string{"api", "--hostname", ghHostname, "-i", endpoint}
}

// ghHTTPResponse builds a `gh api -i` response with an LF-terminated status line
// and CRLF-terminated headers followed by a CRLF blank line and body.
func ghHTTPResponse(statusLine string, extraHeaders []string, body string) string {
	var b strings.Builder
	b.WriteString(statusLine)
	b.WriteByte('\n')
	b.WriteString("Content-Type: application/json\r\n")
	for _, h := range extraHeaders {
		b.WriteString(h)
		b.WriteString("\r\n")
	}
	b.WriteString("\r\n")
	b.WriteString(body)
	return b.String()
}

func runsEndpoint(page int) string {
	return fmt.Sprintf(
		"repos/%s/actions/workflows/ci.yml/runs?event=pull_request&head_sha=%s&per_page=100&page=%d",
		testRepo, testSHA4, page,
	)
}

func jobsEndpoint(runID int64, page int) string {
	return fmt.Sprintf(
		"repos/%s/actions/runs/%d/jobs?filter=latest&per_page=100&page=%d",
		testRepo, runID, page,
	)
}

func annotationsEndpoint(page int) string {
	return fmt.Sprintf(
		"repos/%s/check-runs/%d/annotations?per_page=100&page=%d",
		testRepo, testCheckJobID, page,
	)
}

// validEvalInput builds a valid Input whose HEAD^1 is testSHA1, HeadSHA is testSHA2,
// synthetic merge is testSHA3, and parent commit is testSHA4.
func validEvalInput(depFile string) Input {
	return Input{
		Repo:          testRepo,
		HeadRepo:      testRepo,
		BaseRepo:      testRepo,
		BaseRef:       "main",
		PRNumber:      testPRNumber,
		HeadSHA:       testSHA2,
		RunAttempt:    "1",
		DepDirsFile:   depFile,
		WasmDepDirsOK: "true",
	}
}

// localGitPassSteps returns the four git steps for a synthetic merge commit
// (HEAD -> testSHA3, HEAD^1 -> testSHA1, HEAD^2 -> testSHA2), matching origin/main,
// single parent testSHA4, and the given NUL-delimited parent diff output.
func localGitPassSteps(diffOut string) []cmdrunnertestutil.Step {
	return []cmdrunnertestutil.Step{
		{
			Name: testGitCmd,
			Args: gitArgs("rev-list", "--parents", "-n", "1", "HEAD"),
			Out:  testSHA3 + " " + testSHA1 + " " + testSHA2 + "\n",
		},
		{
			Name: testGitCmd,
			Args: gitArgs("rev-parse", "--verify", originMainCommitRef),
			Out:  testSHA1 + "\n",
		},
		{
			Name: testGitCmd,
			Args: gitArgs("rev-list", "--parents", "-n", "1", testSHA2),
			Out:  testSHA2 + " " + testSHA4 + "\n",
		},
		{
			Name: testGitCmd,
			Args: gitArgs("-c", "core.quotePath=false", "diff", "-z", "--no-renames", "--name-only", testSHA4, testSHA2, "--"),
			Out:  diffOut,
		},
	}
}

// ghHappySteps returns the three single-page GitHub API responses for a passing parent CI.
func ghHappySteps(runID int64) []cmdrunnertestutil.Step {
	runsBody := fmt.Sprintf(`{
		"total_count": 1,
		"workflow_runs": [{
			"id": %d,
			"path": ".github/workflows/ci.yml",
			"event": "pull_request",
			"head_sha": %q,
			"status": "completed",
			"conclusion": "success",
			"created_at": "2026-11-21T10:00:00Z",
			"run_started_at": "2026-11-21T10:00:05Z",
			"repository": {"full_name": %q},
			"head_repository": {"full_name": %q},
			"pull_requests": [{"number": 1073}]
		}]
	}`, runID, testSHA4, testRepo, testRepo)

	jobsBody := fmt.Sprintf(`{
		"total_count": 4,
		"jobs": [
			{"id": %d, "name": "Check changed files", "head_sha": %q, "status": "completed", "conclusion": "success"},
			{"id": 9002, "name": "Test", "head_sha": %q, "status": "completed", "conclusion": "success"},
			{"id": 9003, "name": "Code Quality & Lint", "head_sha": %q, "status": "completed", "conclusion": "success"},
			{"id": 9004, "name": "Browser extension", "head_sha": %q, "status": "completed", "conclusion": "skipped"}
		]
	}`, testCheckJobID, testSHA4, testSHA4, testSHA4, testSHA4)

	annBody := fmt.Sprintf(`[
		{"annotation_level": "notice", "title": "ci-base-sha", "message": %q}
	]`, testSHA1)

	return []cmdrunnertestutil.Step{
		{
			Name: "gh",
			Args: ghAPIArgs(runsEndpoint(1)),
			Out:  ghHTTPResponse("HTTP/2.0 200 OK", nil, runsBody),
		},
		{
			Name: "gh",
			Args: ghAPIArgs(jobsEndpoint(runID, 1)),
			Out:  ghHTTPResponse("HTTP/2.0 200 OK", nil, jobsBody),
		},
		{
			Name: "gh",
			Args: ghAPIArgs(annotationsEndpoint(1)),
			Out:  ghHTTPResponse("HTTP/2.0 200 OK", nil, annBody),
		},
	}
}
