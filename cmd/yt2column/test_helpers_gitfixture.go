//go:build test

package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/isseis/yt2column/internal/snapshottest"
)

// gitFixture is a fixture git repository a test runs a command document's
// bash script on. home and tmp are the fixture's own empty HOME and TMPDIR
// (tmp is checked empty after each script run), so nothing of the test's own
// environment or configuration reaches git or the script.
type gitFixture struct {
	t               *testing.T
	repo, home, tmp string
	// noScripts is set on a fixture that builds a state shared between
	// bashes: a script run there would run in whichever bash happened to
	// build it, so runScript fails the test instead.
	noScripts bool
}

// snapshotStore holds the fixture states that tests build once per test
// process and copy (internal/snapshottest). TestMain sets it before any test
// runs and closes it after the last one.
var snapshotStore *snapshottest.Store

// fixtureDirMode is the mode of the directories write creates.
const fixtureDirMode = 0o750

// newGitFixture creates an empty repository whose first branch is main.
func newGitFixture(t *testing.T) *gitFixture {
	t.Helper()
	return newGitFixtureIn(t, t.TempDir())
}

// newGitFixtureIn creates an empty repository whose first branch is main,
// with its own HOME and TMPDIR, under dir (home, tmp, and repo). A state
// built for a snapshot is built in the snapshot's directory this way, and
// the repository is the only part a case uses of the copy.
func newGitFixtureIn(t *testing.T, dir string) *gitFixture {
	t.Helper()
	f := &gitFixture{t: t, home: filepath.Join(dir, "home"), tmp: filepath.Join(dir, "tmp"), repo: filepath.Join(dir, "repo")}
	for _, d := range []string{f.home, f.tmp} {
		if err := os.Mkdir(d, fixtureDirMode); err != nil {
			t.Fatalf("create %s: %v", d, err)
		}
	}
	f.gitIn(dir, "init", "-q", "-b", "main", f.repo)
	return f
}

// newGitFixtureFromSnapshot returns a fixture whose repository is a copy of
// sn's repository, in a new directory of t, and whose HOME and TMPDIR are new
// empty directories of t (the case's own, as for newGitFixture). The copy
// carries the snapshot's empty home and tmp too; the case does not use them.
// The index is not refreshed after the copy: the scripts under test run git
// with GIT_OPTIONAL_LOCKS=0 and read only what commits and refs say, and the
// git commands a case's setup runs refresh the index themselves.
func newGitFixtureFromSnapshot[V any](t *testing.T, sn snapshottest.Snapshot[V]) *gitFixture {
	t.Helper()
	return newGitFixtureFromSnapshotAt(t, sn, "repo")
}

// newGitFixtureFromSnapshotAt is newGitFixtureFromSnapshot for a snapshot
// whose repository is at repoRel (slash-separated) under its directory.
func newGitFixtureFromSnapshotAt[V any](t *testing.T, sn snapshottest.Snapshot[V], repoRel string) *gitFixture {
	t.Helper()
	copyDir := filepath.Join(t.TempDir(), "snapshot")
	sn.CopyTo(t, copyDir)
	return &gitFixture{t: t, home: t.TempDir(), tmp: t.TempDir(), repo: filepath.Join(copyDir, filepath.FromSlash(repoRel))}
}

// env is the allowlisted environment of every git and script run, so no
// variable of the test's own environment (a GIT_DIR a pre-commit hook
// exports) reaches them.
func (f *gitFixture) env() []string {
	return []string{
		"PATH=" + os.Getenv("PATH"), "HOME=" + f.home, "TMPDIR=" + f.tmp,
		"GIT_CONFIG_NOSYSTEM=1", "GIT_TERMINAL_PROMPT=0", "LC_ALL=C",
	}
}

// gitIn runs git in dir with the fixture's environment and a fixed author,
// failing t on an error, and returns its standard output.
func (f *gitFixture) gitIn(dir string, args ...string) string {
	f.t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", args...) //nolint:gosec // git with the test's own arguments
	cmd.Dir = dir
	cmd.Env = append(f.env(),
		"GIT_AUTHOR_NAME=Octo User", "GIT_AUTHOR_EMAIL=octo@example.invalid",
		"GIT_COMMITTER_NAME=Octo User", "GIT_COMMITTER_EMAIL=octo@example.invalid")
	var stdout, stderr strings.Builder
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		f.t.Fatalf("git %v: %v: %s", args, err, stderr.String())
	}
	return stdout.String()
}

func (f *gitFixture) git(args ...string) string {
	f.t.Helper()
	return f.gitIn(f.repo, args...)
}

func (f *gitFixture) rev(name string) string {
	f.t.Helper()
	return strings.TrimSpace(f.git("rev-parse", "--verify", name))
}

// write writes content to path in the work tree, creating its directory.
func (f *gitFixture) write(path, content string) {
	f.t.Helper()
	full := filepath.Join(f.repo, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(full), fixtureDirMode); err != nil {
		f.t.Fatalf("create the directory of %s: %v", path, err)
	}
	if err := os.WriteFile(full, []byte(content), 0o600); err != nil { //nolint:gosec // a path inside the fixture's temporary repository
		f.t.Fatalf("write %s: %v", path, err)
	}
}

// commit stages every change and commits it with message.
func (f *gitFixture) commit(message string) {
	f.t.Helper()
	f.git("add", "-A")
	f.git("commit", "-q", "--allow-empty", "-m", message)
}

// scriptRunOptions changes how runScript runs a script: in dir (relative to
// the repository; its root when ""), with stdin, and with extraEnv after the
// fixture's environment (a later entry wins).
type scriptRunOptions struct {
	dir, stdin string
	extraEnv   []string
}

// scriptRun is what one script run printed and returned.
type scriptRun struct {
	code   int
	stdout string
	stderr string
}

func (r scriptRun) String() string {
	return fmt.Sprintf("exit %d, stdout %q, stderr %q", r.code, r.stdout, r.stderr)
}

// runScript runs the script at the repository-relative path script (from
// this package's directory) with bash and args, and checks that the run left
// TMPDIR empty.
func (f *gitFixture) runScript(bash scriptBash, script string, args []string, opts scriptRunOptions) scriptRun {
	f.t.Helper()
	if f.noScripts {
		f.t.Fatalf("a script was run while building a state shared between bashes, so the state would depend on which bash built it")
	}
	abs, err := filepath.Abs(script)
	if err != nil {
		f.t.Fatalf("resolve the script path: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, bash.path, append([]string{abs}, args...)...) //nolint:gosec // bash with the test's script
	cmd.Dir = filepath.Join(f.repo, filepath.FromSlash(opts.dir))
	cmd.Env = append(f.env(), opts.extraEnv...)
	cmd.Stdin = strings.NewReader(opts.stdin)
	var stdout, stderr strings.Builder
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err = cmd.Run()
	run := scriptRun{stdout: stdout.String(), stderr: stderr.String()}
	if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
		run.code = exitErr.ExitCode()
	} else if err != nil {
		f.t.Fatalf("run the script: %v", err)
	}
	if entries, err := os.ReadDir(f.tmp); err != nil || len(entries) != 0 {
		f.t.Errorf("the script left TMPDIR with %d entries (error %v)", len(entries), err)
	}
	return run
}
