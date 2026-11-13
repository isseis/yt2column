package chief

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/isseis/yt2column/internal/cmdrunner"
)

// RunnerFactory makes the runner for one external command from its options.
// The entry point passes one that returns cmdrunner.NewOS; tests pass a
// recorder that never starts a process.
type RunnerFactory func(cmdrunner.Options) cmdrunner.Runner

// Commands runs herdr, git, and gh. It is the only path by which this package
// starts an external command, so every command gets a time limit, an output
// cap, and the environment chosen here.
type Commands struct {
	newRunner RunnerFactory
	environ   []string
}

// NewCommands returns the command runner. environ is the environment the
// process was given; the entry point reads it, and each command receives a
// copy filtered for it.
func NewCommands(newRunner RunnerFactory, environ []string) Commands {
	return Commands{newRunner: newRunner, environ: slices.Clone(environ)}
}

// The time limits of the external commands. Each leaves room over the delays
// observed while sampling; a change edits one constant.
const (
	herdrTimeout = 10 * time.Second
	gitTimeout   = 30 * time.Second
	ghTimeout    = 30 * time.Second
)

// commandMaxOutput caps each of a command's standard output and standard
// error. The largest output read is a pane screen (tens of KiB) or a PR's
// JSON; anything near the cap is not one of them.
const commandMaxOutput = 4 << 20

// commandWaitDelay bounds the wait for a command's output after its time
// limit stops it.
const commandWaitDelay = 2 * time.Second

// The programs chief runs, by the names it finds them on PATH.
const (
	herdrProgram = "herdr"
	gitProgram   = "git"
	ghProgram    = "gh"
)

// ghTargetVariables point gh at another repository or host than the work
// tree's remote; chief removes them so a PR is always the task worktree's own.
var ghTargetVariables = []string{"GH_REPO", "GH_HOST"}

// errRelativeDir reports a git or gh call without an absolute working
// directory: both read the repository of their directory, so a relative one
// would depend on chief's own working directory.
var errRelativeDir = errors.New("command directory is not absolute")

// herdr runs herdr with args and returns its standard output.
func (c Commands) herdr(ctx context.Context, args ...string) ([]byte, error) {
	// A nil Env would let herdr inherit the process environment; the
	// environment passed in is the whole of it, even when empty.
	return c.run(ctx, herdrTimeout, "", append([]string{}, c.environ...), herdrProgram, args)
}

// git runs git in dir with the hardening options before args and with the git
// environment, and returns its standard output. Contract: git never runs
// without an absolute dir, never with an inherited GIT_* variable, and never
// without cmdrunner.GitHardening's options before the subcommand.
func (c Commands) git(ctx context.Context, dir string, args ...string) ([]byte, error) {
	if !filepath.IsAbs(dir) {
		return nil, fmt.Errorf("git in %q: %w", dir, errRelativeDir)
	}
	full := append(cmdrunner.GitHardening(), args...)
	return c.run(ctx, gitTimeout, dir, cmdrunner.GitEnv(c.environ), gitProgram, full)
}

// gh runs gh in dir and returns its standard output. Contract: gh never runs
// with GH_REPO or GH_HOST, so it reads the repository of dir's remote, and
// never with an inherited GIT_* variable, since gh starts git itself.
func (c Commands) gh(ctx context.Context, dir string, args ...string) ([]byte, error) {
	if !filepath.IsAbs(dir) {
		return nil, fmt.Errorf("gh in %q: %w", dir, errRelativeDir)
	}
	env := slices.DeleteFunc(cmdrunner.GitEnv(c.environ), func(entry string) bool {
		name, _, _ := strings.Cut(entry, "=")
		return slices.Contains(ghTargetVariables, name)
	})
	return c.run(ctx, ghTimeout, dir, env, ghProgram, args)
}

func (c Commands) run(ctx context.Context, limit time.Duration, dir string, env []string, name string, args []string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, limit)
	defer cancel()
	runner := c.newRunner(cmdrunner.Options{
		MaxOutput: commandMaxOutput,
		WaitDelay: commandWaitDelay,
		Dir:       dir,
		Env:       env,
	})
	return runner.Run(ctx, name, args...)
}
