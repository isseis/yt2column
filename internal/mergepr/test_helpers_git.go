//go:build test

package mergepr

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/isseis/yt2column/internal/cmdrunner"
	"github.com/isseis/yt2column/internal/snapshottest"
)

// snapshotStore holds the shared takeIn states built once per test process by
// TestMain.
var snapshotStore *snapshottest.Store

// testMainRef is origin/main's remote-tracking ref, spelled out here rather
// than taken from the production constant, so a change to how the check names
// the ref cannot change the test's setup with it.
const testMainRef = "refs/remotes/origin/main"

// testRepoDirMode is the mode of the directories a test commit creates.
const testRepoDirMode = 0o750

// gitRepo is a temporary git repository for the protocol-number check. HOME
// and XDG_CONFIG_HOME point at an empty directory, so the user's git
// configuration (commit signing, hooks, grep settings) cannot reach the test
// unless the test writes it. Commits are made from whole snapshots with
// commit-tree, so a test states every file of every commit and can give a
// commit any parents, a merge commit or a root commit included.
//
// The helper never writes Go source of its own: the snapshots, declaration
// lines included, are built in the _test.go files, because the check reads
// this file as part of internal/mergepr.
type gitRepo struct {
	t    *testing.T
	Dir  string
	Home string
}

// newGitRepo creates an empty repository on branch main. Tests that use it
// change HOME and cannot run in parallel.
func newGitRepo(t *testing.T) *gitRepo {
	t.Helper()
	root := t.TempDir()
	r := &gitRepo{t: t, Dir: filepath.Join(root, "repo"), Home: filepath.Join(root, "home")}
	if err := os.MkdirAll(r.Home, 0o700); err != nil {
		t.Fatalf("create home: %v", err)
	}
	t.Setenv("HOME", r.Home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(r.Home, ".config"))
	r.gitIn(root, "init", "-q", "--template=", "-b", mainBranch, r.Dir)
	r.Git("config", "user.name", "Test")
	r.Git("config", "user.email", "test@example.com")
	r.Git("config", "commit.gpgSign", "false")
	r.Git("config", "core.ignorecase", "false")
	return r
}

// Git runs git in the repository for the test's own setup, outside the check,
// and returns its output without the final newline.
func (r *gitRepo) Git(args ...string) string {
	r.t.Helper()
	return r.gitIn(r.Dir, args...)
}

func (r *gitRepo) gitIn(dir string, args ...string) string {
	r.t.Helper()
	return r.gitInput(dir, nil, args...)
}

// gitInput runs git in dir with stdin as its standard input (none when nil).
func (r *gitRepo) gitInput(dir string, stdin []byte, args ...string) string {
	r.t.Helper()
	cmd := exec.Command(gitCommand, args...) //nolint:gosec // test setup runs git with fixed arguments
	cmd.Dir = dir
	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}
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

// Commit makes a commit whose tree holds exactly files (path to content),
// with the given parents (none makes a root commit), and returns its ID. It
// moves no branch.
func (r *gitRepo) Commit(subject string, files map[string]string, parents ...string) string {
	r.t.Helper()
	entries, err := os.ReadDir(r.Dir)
	if err != nil {
		r.t.Fatalf("read repository: %v", err)
	}
	for _, e := range entries {
		if e.Name() == ".git" {
			continue
		}
		if err := os.RemoveAll(filepath.Join(r.Dir, e.Name())); err != nil {
			r.t.Fatalf("clear work tree: %v", err)
		}
	}
	for name, content := range files {
		path := filepath.Join(r.Dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), testRepoDirMode); err != nil {
			r.t.Fatalf("create dir for %s: %v", name, err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil { //nolint:gosec // a path inside the test's temporary repository
			r.t.Fatalf("write %s: %v", name, err)
		}
	}
	r.Git("add", "-A")
	tree := r.Git("write-tree")
	args := []string{"commit-tree", tree, "-m", subject}
	for _, p := range parents {
		args = append(args, "-p", p)
	}
	return r.Git(args...)
}

// RawCommit writes a commit object whose message is exactly message (any
// bytes, which git commit would rewrite), with an empty tree, the given
// parents, the committer time at (seconds since the epoch), and extra header
// lines (each without its LF) after the committer line, and returns its ID.
// It moves no branch.
func (r *gitRepo) RawCommit(message []byte, at int64, headers []string, parents ...string) string {
	r.t.Helper()
	tree := r.gitInput(r.Dir, []byte{}, "hash-object", "-t", "tree", "-w", "--stdin")
	var object bytes.Buffer
	fmt.Fprintf(&object, "tree %s\n", tree)
	for _, p := range parents {
		fmt.Fprintf(&object, "parent %s\n", p)
	}
	fmt.Fprintf(&object, "author Test <test@example.com> %d +0000\n", at)
	fmt.Fprintf(&object, "committer Test <test@example.com> %d +0000\n", at)
	for _, h := range headers {
		object.WriteString(h + "\n")
	}
	object.WriteString("\n")
	object.Write(message)
	return r.gitInput(r.Dir, object.Bytes(), "hash-object", "-t", "commit", "-w", "--stdin")
}

// SetMain points refs/remotes/origin/main at oid, as a fetch of main would.
func (r *gitRepo) SetMain(oid string) {
	r.t.Helper()
	r.Git("update-ref", testMainRef, oid)
}

// Check runs the production check in dir (the repository when empty).
func (r *gitRepo) Check(dir, head, base string) (protocolSummary, error) {
	r.t.Helper()
	if dir == "" {
		dir = r.Dir
	}
	return newProtocolCheck(newGitRunner(dir)).check(r.t.Context(), head, base)
}

// RemoveObject deletes the loose object file of oid, so reading it fails.
func (r *gitRepo) RemoveObject(oid string) {
	r.t.Helper()
	path := filepath.Join(r.Dir, ".git", "objects", oid[:2], oid[2:])
	if err := os.Remove(path); err != nil {
		r.t.Fatalf("remove object %s: %v", oid, err)
	}
}

// ShallowClone clones the repository's branch main through a file:// URL
// with depth 1 (a local path would ignore --depth) and returns the clone's
// directory. The clone has refs/remotes/origin/main.
func (r *gitRepo) ShallowClone(mainOID string) string {
	r.t.Helper()
	r.Git("update-ref", "refs/heads/"+mainBranch, mainOID)
	dir := filepath.Join(filepath.Dir(r.Dir), "shallow")
	r.gitIn(filepath.Dir(r.Dir), "clone", "-q", "--depth", "1", "--template=", "file://"+filepath.ToSlash(r.Dir), dir)
	return dir
}

// withT returns the repository bound to t, so a subtest's failure in a
// helper stops the subtest, not its parent.
func (r *gitRepo) withT(t *testing.T) *gitRepo {
	c := *r
	c.t = t
	return &c
}

// deletedFile, as the content of a path given to takeIn.Apply, removes the
// path instead of writing it.
const deletedFile = "\x00deleted"

// symlinkPrefix starts the content of a path given to takeIn.Apply that is
// written as a symbolic link to the rest of the content.
const symlinkPrefix = "\x00symlink:"

// gitFalse is git's false configuration value.
const gitFalse = "false"

// takeInBranch is the PR head branch of a takeIn repository.
const takeInBranch = "feature"

// takeInPR is the PR value the fake gh of a takeIn repository returns.
// HeadOID, when empty, is read from origin's branch at the time of the call.
type takeInPR struct {
	State   string
	Base    string
	Branch  string
	HeadOID string
	Cross   bool
	// OnView, when set, runs inside each gh pr view call, before the
	// response is built.
	OnView func()
}

// takeIn is a work tree cloned from a bare origin, for the take-in tool.
// Every git it runs, the tool's included, gets an environment whose HOME and
// XDG_CONFIG_HOME point at an empty directory and that skips the system
// configuration, so neither the user's nor the system's git configuration
// can reach the test, and tests that use it can run in parallel. The fake gh
// returns PR, and the fake environment is Env.
//
// Like gitRepo, the helper never writes Go source of its own; the tests give
// every file's content.
type takeIn struct {
	t      *testing.T
	Origin string
	Work   string
	PR     takeInPR
	Env    map[string]string
	// GHCalls records the arguments of each gh call.
	GHCalls [][]string
	env     []string
}

// newTakeIn creates a bare origin with branch main and a clone of it as the
// work tree.
func newTakeIn(t *testing.T) *takeIn {
	t.Helper()
	root := t.TempDir()
	home := filepath.Join(root, "home")
	if err := os.MkdirAll(home, 0o700); err != nil {
		t.Fatalf("create home: %v", err)
	}
	env := slices.DeleteFunc(cmdrunner.GitEnv(os.Environ()), func(entry string) bool {
		return strings.HasPrefix(entry, "HOME=") || strings.HasPrefix(entry, "XDG_CONFIG_HOME=")
	})
	env = append(env, "HOME="+home, "XDG_CONFIG_HOME="+filepath.Join(home, ".config"), "GIT_CONFIG_NOSYSTEM=1")
	k := &takeIn{
		t:      t,
		Origin: filepath.Join(root, "origin.git"),
		Work:   filepath.Join(root, "work"),
		PR:     takeInPR{State: openState, Base: mainBranch, Branch: takeInBranch},
		Env:    map[string]string{},
		env:    env,
	}
	k.gitIn(root, "init", "-q", "--bare", "--template=", "-b", mainBranch, k.Origin)
	k.gitIn(root, "clone", "-q", "--template=", k.Origin, k.Work)
	for _, kv := range [][2]string{{"user.name", "Test"}, {"user.email", "test@example.com"}, {"commit.gpgSign", gitFalse}, {"core.ignorecase", gitFalse}} {
		k.Git("config", kv[0], kv[1])
	}
	return k
}

// gitIn runs git in dir with the test's environment and returns its output
// without the final newline.
func (k *takeIn) gitIn(dir string, args ...string) string {
	k.t.Helper()
	cmd := exec.Command(gitCommand, args...) //nolint:gosec // test setup runs git with fixed arguments
	cmd.Dir = dir
	cmd.Env = k.env
	out, err := cmd.Output()
	if err != nil {
		stderr := ""
		if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
			stderr = string(exitErr.Stderr)
		}
		k.t.Fatalf("git %v: %v: %s", args, err, stderr)
	}
	return strings.TrimSuffix(string(out), "\n")
}

// Git runs git in the work tree and returns its output without the final
// newline.
func (k *takeIn) Git(args ...string) string {
	k.t.Helper()
	return k.gitIn(k.Work, args...)
}

// OriginGit runs git in the bare origin.
func (k *takeIn) OriginGit(args ...string) string {
	k.t.Helper()
	return k.gitIn(k.Origin, args...)
}

// Apply writes each file of the work tree (content, a symbolic link, or a
// removal) and stages every change.
func (k *takeIn) Apply(files map[string]string) {
	k.t.Helper()
	for name, content := range files {
		p := filepath.Join(k.Work, filepath.FromSlash(name))
		if err := os.RemoveAll(p); err != nil {
			k.t.Fatalf("remove %s: %v", name, err)
		}
		if content == deletedFile {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(p), testRepoDirMode); err != nil {
			k.t.Fatalf("create dir for %s: %v", name, err)
		}
		if target, ok := strings.CutPrefix(content, symlinkPrefix); ok {
			if err := os.Symlink(target, p); err != nil {
				k.t.Fatalf("symlink %s: %v", name, err)
			}
			continue
		}
		if err := os.WriteFile(p, []byte(content), 0o600); err != nil { //nolint:gosec // a path inside the test's temporary work tree
			k.t.Fatalf("write %s: %v", name, err)
		}
	}
	k.Git("add", "-A")
}

// Commit applies files, commits them, and returns the commit ID.
func (k *takeIn) Commit(subject string, files map[string]string) string {
	k.t.Helper()
	k.Apply(files)
	k.Git("commit", "-q", "--no-verify", "--allow-empty", "-m", subject)
	return k.Git("rev-parse", "HEAD")
}

// Setup builds the usual history: base on main, branch from it on the PR
// head branch, then main on main, each pushed to origin, and leaves the work
// tree on the PR head branch. It returns the base head (the branch's commit)
// and main's commit.
func (k *takeIn) Setup(base, branch, main map[string]string) (baseHead, mainOID string) {
	k.t.Helper()
	k.Commit("base", base)
	k.Git("push", "-q", "origin", mainBranch)
	k.Git("switch", "-q", "-c", takeInBranch)
	baseHead = k.Commit("branch", branch)
	k.Git("push", "-q", "origin", takeInBranch)
	k.Git("switch", "-q", mainBranch)
	mainOID = k.Commit("main", main)
	k.Git("push", "-q", "origin", mainBranch)
	k.Git("switch", "-q", takeInBranch)
	return baseHead, mainOID
}

// Tool returns the take-in tool running git in the work tree, with the fake
// gh and environment.
func (k *takeIn) Tool() *Mergemain {
	return k.ToolWith(k.GitRunner())
}

// GitRunner returns a Runner of git in the work tree with the test's
// environment, the one the tool runs with.
func (k *takeIn) GitRunner() Runner {
	return cmdrunner.NewOS(cmdrunner.Options{Dir: k.Work, Env: k.env})
}

// ToolWith returns the take-in tool running git through git, with the fake
// gh and environment.
func (k *takeIn) ToolWith(git Runner) *Mergemain {
	lookup := func(key string) (string, bool) {
		v, ok := k.Env[key]
		return v, ok
	}
	return newMergemain(git, takeInGH{k: k}, lookup)
}

// recordingRunner runs commands through run and records each argument list.
type recordingRunner struct {
	run   Runner
	calls [][]string
}

// Run implements Runner.
func (r *recordingRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	r.calls = append(r.calls, slices.Clone(args))
	return r.run.Run(ctx, name, args...)
}

// takeInGH is the fake gh of a takeIn repository.
type takeInGH struct{ k *takeIn }

// Run implements Runner: it answers gh pr view with the PR value.
func (g takeInGH) Run(_ context.Context, name string, args ...string) ([]byte, error) {
	k := g.k
	k.GHCalls = append(k.GHCalls, slices.Clone(args))
	if name != ghCommand {
		k.t.Fatalf("fake gh called as %s", name)
	}
	if k.PR.OnView != nil {
		k.PR.OnView()
	}
	head := k.PR.HeadOID
	if head == "" {
		head = k.OriginGit("rev-parse", headsPrefix+k.PR.Branch)
	}
	return fmt.Appendf(nil, `{"state":%q,"baseRefName":%q,"headRefName":%q,"headRefOid":%q,"isCrossRepository":%t}`,
		k.PR.State, k.PR.Base, k.PR.Branch, head, k.PR.Cross), nil
}
