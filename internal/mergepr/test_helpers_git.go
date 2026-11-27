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
	"time"

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

// takeInBaseKind is the shared empty takeIn (a bare origin with branch main
// and a clone of it as the work tree).
var takeInBaseKind = snapshottest.NewKind[struct{}]("empty takeIn", nil)

// takeInEnv creates home and returns a git environment isolated from the
// user's and the system's git configuration.
func takeInEnv(t *testing.T, home string) []string {
	t.Helper()
	if err := os.MkdirAll(home, 0o700); err != nil {
		t.Fatalf("create home: %v", err)
	}
	env := slices.DeleteFunc(cmdrunner.GitEnv(os.Environ()), func(entry string) bool {
		return strings.HasPrefix(entry, "HOME=") || strings.HasPrefix(entry, "XDG_CONFIG_HOME=")
	})
	return append(env, "HOME="+home, "XDG_CONFIG_HOME="+filepath.Join(home, ".config"), "GIT_CONFIG_NOSYSTEM=1")
}

// newTakeInAt returns a takeIn pointing at root/origin.git and root/work with
// its own home directory and default PR, Env, and GHCalls.
func newTakeInAt(t *testing.T, root, home string) *takeIn {
	t.Helper()
	return &takeIn{
		t:      t,
		Origin: filepath.Join(root, "origin.git"),
		Work:   filepath.Join(root, "work"),
		PR:     takeInPR{State: openState, Base: mainBranch, Branch: takeInBranch},
		Env:    map[string]string{},
		env:    takeInEnv(t, home),
	}
}

// buildTakeInBase creates a bare origin with branch main and a clone of it as
// the work tree directly under dir. Its home lives in the builder's t.TempDir
// so only origin.git and work are stored in the snapshot directory.
func buildTakeInBase(t *testing.T, dir string) *takeIn {
	t.Helper()
	k := newTakeInAt(t, dir, filepath.Join(t.TempDir(), "home"))
	k.gitIn(dir, "init", "-q", "--bare", "--template=", "-b", mainBranch, k.Origin)
	k.gitIn(dir, "clone", "-q", "--template=", k.Origin, k.Work)
	for _, kv := range [][2]string{{"user.name", "Test"}, {"user.email", "test@example.com"}, {"commit.gpgSign", gitFalse}, {"core.ignorecase", gitFalse}} {
		k.Git("config", kv[0], kv[1])
	}
	return k
}

// loadTakeInBase returns the shared empty takeIn snapshot.
func loadTakeInBase(t *testing.T) snapshottest.Snapshot[struct{}] {
	t.Helper()
	return snapshottest.Load(t, snapshotStore, takeInBaseKind, nil, func(bt *testing.T, dir string) struct{} {
		buildTakeInBase(bt, dir)
		return struct{}{}
	})
}

// initTakeInDir copies the shared empty takeIn into dir (which already exists)
// so dir holds origin.git and work directly, rewrites remote.origin.url to
// dir/origin.git, and returns a takeIn whose home lives in t.TempDir().
func initTakeInDir(t *testing.T, dir string) *takeIn {
	t.Helper()
	sn := loadTakeInBase(t)
	staged := filepath.Join(dir, ".init")
	sn.CopyTo(t, staged)
	for _, name := range []string{"origin.git", "work"} {
		if err := os.Rename(filepath.Join(staged, name), filepath.Join(dir, name)); err != nil {
			t.Fatalf("move %s into snapshot dir: %v", name, err)
		}
	}
	if err := os.Remove(staged); err != nil {
		t.Fatalf("remove staging dir: %v", err)
	}
	k := newTakeInAt(t, dir, filepath.Join(t.TempDir(), "home"))
	k.Git("config", "remote.origin.url", k.Origin)
	return k
}

// refreshIndex refreshes the index stat cache after copying the work tree so
// git diff-files under GIT_OPTIONAL_LOCKS=0 sees only real changes. --unmerged
// keeps refreshing past conflicted entries; exit code 1 means some entries
// (such as unmerged paths) remain dirty, which is expected here.
func (k *takeIn) refreshIndex() {
	k.t.Helper()
	cmd := exec.Command(gitCommand, "update-index", "-q", "--unmerged", "--refresh") //nolint:gosec // test setup runs git with fixed arguments
	cmd.Dir = k.Work
	cmd.Env = k.env
	out, err := cmd.CombinedOutput()
	if err == nil {
		return
	}
	if exitErr, ok := errors.AsType[*exec.ExitError](err); ok && exitErr.ExitCode() == 1 {
		return
	}
	k.t.Fatalf("git update-index --refresh: %v: %s", err, out)
}

// copyTakeIn copies a shared takeIn snapshot into a fresh directory owned by t
// and adjusts its git state so remote.origin.url points at the copy's own
// origin and the index stat cache matches the copied work tree.
func copyTakeIn[V any](t *testing.T, sn snapshottest.Snapshot[V]) *takeIn {
	t.Helper()
	root := t.TempDir()
	stateDir := filepath.Join(root, "state")
	sn.CopyTo(t, stateDir)
	k := newTakeInAt(t, stateDir, filepath.Join(root, "home"))
	// A copied clone still names the builder's origin in remote.origin.url;
	// rewrite it to the copy's own origin before any test runs git against it.
	k.Git("config", "remote.origin.url", k.Origin)
	k.refreshIndex()
	return k
}

// newTakeIn returns a copy of the shared empty takeIn (a bare origin with
// branch main and a clone of it as the work tree). A test that calls it must
// not use t.Setenv: see snapshottest.Load.
func newTakeIn(t *testing.T) *takeIn {
	t.Helper()
	return copyTakeIn(t, loadTakeInBase(t))
}

// gitIn runs git in dir with the test's environment and returns its output
// without the final newline.
func (k *takeIn) gitIn(dir string, args ...string) string {
	k.t.Helper()
	return k.gitInput(dir, nil, args...)
}

// gitInput runs git in dir with stdin as its standard input (none when nil)
// and the test's environment.
func (k *takeIn) gitInput(dir string, stdin []byte, args ...string) string {
	k.t.Helper()
	cmd := exec.Command(gitCommand, args...) //nolint:gosec // test setup runs git with fixed arguments
	cmd.Dir = dir
	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}
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

const (
	fastImportMarkBase   = 1
	fastImportMarkBranch = 2
	fastImportMarkMain   = 3
)

// Setup builds the usual history: base on main, branch from it on the PR
// head branch, then main on main, each pushed to origin, and leaves the work
// tree on the PR head branch. It returns the base head (the branch's commit)
// and main's commit.
//
// Invariant: the resulting origin and work refs, commit trees, subjects,
// parent relationships, index, and work tree must match committing and pushing
// base, branch, and main sequentially.
func (k *takeIn) Setup(base, branch, main map[string]string) (baseHead, mainOID string) {
	k.t.Helper()
	ts := time.Now().Unix()
	var originStream bytes.Buffer
	writeFastImportCommit(&originStream, headsPrefix+mainBranch, fastImportMarkBase, 0, "base", ts, base)
	writeFastImportCommit(&originStream, headsPrefix+takeInBranch, fastImportMarkBranch, fastImportMarkBase, "branch", ts, branch)
	writeFastImportCommit(&originStream, headsPrefix+mainBranch, fastImportMarkMain, fastImportMarkBase, "main", ts, main)

	marksPath := filepath.Join(k.t.TempDir(), "marks")
	k.gitInput(k.Origin, originStream.Bytes(), "fast-import", "--quiet", "--export-marks="+marksPath)

	var workStream bytes.Buffer
	workStream.Write(originStream.Bytes())
	workStream.WriteString("reset refs/remotes/origin/" + mainBranch + "\nfrom :3\n")
	workStream.WriteString("reset refs/remotes/origin/" + takeInBranch + "\nfrom :2\n")
	k.gitInput(k.Work, workStream.Bytes(), "fast-import", "--quiet")
	k.Git("checkout", "-q", "-f", takeInBranch)

	marks := readFastImportMarks(k.t, marksPath)
	return marks[":2"], marks[":3"]
}

// writeFastImportCommit appends one git fast-import commit command to buf.
// parentMark == 0 writes a root commit; otherwise it writes `from :<parentMark>`.
func writeFastImportCommit(buf *bytes.Buffer, ref string, mark, parentMark int, subject string, ts int64, files map[string]string) {
	fmt.Fprintf(buf, "commit %s\nmark :%d\n", ref, mark)
	fmt.Fprintf(buf, "author Test <test@example.com> %d +0000\n", ts)
	fmt.Fprintf(buf, "committer Test <test@example.com> %d +0000\n", ts)
	msg := subject + "\n"
	fmt.Fprintf(buf, "data %d\n%s\n", len(msg), msg)
	if parentMark != 0 {
		fmt.Fprintf(buf, "from :%d\n", parentMark)
	}
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		content := files[name]
		quoted := quoteFastImportPath(name)
		if content == deletedFile {
			fmt.Fprintf(buf, "D %s\n", quoted)
			continue
		}
		if target, ok := strings.CutPrefix(content, symlinkPrefix); ok {
			fmt.Fprintf(buf, "M 120000 inline %s\ndata %d\n%s\n", quoted, len(target), target)
			continue
		}
		fmt.Fprintf(buf, "M 100644 inline %s\ndata %d\n%s\n", quoted, len(content), content)
	}
}

// quoteFastImportPath formats a slash-separated path as a C-quoted string for
// git fast-import so spaces, quotes, backslashes, and control bytes are safe.
func quoteFastImportPath(name string) string {
	var b strings.Builder
	b.WriteByte('"')
	for i := range len(name) {
		switch ch := name[i]; ch {
		case '\\', '"':
			b.WriteByte('\\')
			b.WriteByte(ch)
		case '\n':
			b.WriteString(`\n`)
		default:
			b.WriteByte(ch)
		}
	}
	b.WriteByte('"')
	return b.String()
}

// readFastImportMarks parses a git fast-import --export-marks file into a map
// from mark name (":1", ":2", ...) to commit OID.
func readFastImportMarks(t *testing.T, path string) map[string]string {
	t.Helper()
	data, err := os.ReadFile(path) //nolint:gosec // marks file created in t.TempDir()
	if err != nil {
		t.Fatalf("read fast-import marks: %v", err)
	}
	marks := make(map[string]string)
	for line := range strings.SplitSeq(strings.TrimSpace(string(data)), "\n") {
		if line == "" {
			continue
		}
		mark, oid, ok := strings.Cut(line, " ")
		if !ok || oid == "" {
			t.Fatalf("unexpected fast-import mark line: %q", line)
		}
		marks[mark] = oid
	}
	return marks
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
