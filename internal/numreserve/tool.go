package numreserve

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/isseis/yt2column/internal/cmdrunner"
)

// Tool runs git and gh with its own bounds, whoever calls it.
type Tool struct {
	git     cmdrunner.InputRunner
	gh      cmdrunner.Runner
	bounds  bounds
	refused error
}

// ErrUnderTest reports that the production Tool was built inside go test.
// Every record tag the tool creates is permanent, so no test may reach the
// real git and gh through New; tests build a Tool on a fake runner instead.
var ErrUnderTest = errors.New("numreserve: the production tool refuses to run under go test")

// New returns the production Tool, which builds its own bounded runners from
// environ, the process environment the caller read.
//
// Contract: under go test it returns a Tool that fails every call with
// ErrUnderTest before running any command, so no test can create a real tag.
func New(environ []string) *Tool {
	if testing.Testing() {
		return &Tool{refused: ErrUnderTest}
	}
	b := defaultBounds()
	gitOpts, ghOpts := runnerOptions(environ, b)
	return &Tool{git: cmdrunner.NewOS(gitOpts), gh: cmdrunner.NewOS(ghOpts), bounds: b}
}

// ghRemoved are the variables taken out of gh's environment before ghAdded
// is appended. Forced color would wrap header names in escape sequences and
// make the rate-limit headers unreadable; a prompt would wait for a user who
// is not there; GH_REPO and GH_HOST would point gh elsewhere, as fixprguard
// and chief also refuse.
var (
	ghRemoved = []string{"CLICOLOR_FORCE", "GH_FORCE_TTY", "NO_COLOR", "GH_PROMPT_DISABLED", "GH_REPO", "GH_HOST"}
	ghAdded   = []string{"NO_COLOR=1", "GH_PROMPT_DISABLED=1"}
)

// runnerOptions returns the options of the git runner and of the gh runner.
// git runs in cmdrunner.GitEnv's environment, in a process group of its own
// so a time limit also stops the transport git starts (ssh, a remote
// helper); gh runs without color and without prompts. Both cap their output;
// the time limits are put on each call's context, not here.
func runnerOptions(environ []string, b bounds) (git, gh cmdrunner.Options) {
	ghEnv := make([]string, 0, len(environ)+len(ghAdded))
	for _, entry := range environ {
		key, _, _ := strings.Cut(entry, "=")
		if !slices.Contains(ghRemoved, key) {
			ghEnv = append(ghEnv, entry)
		}
	}
	ghEnv = append(ghEnv, ghAdded...)
	git = cmdrunner.Options{
		MaxOutput: b.maxOutput, WaitDelay: b.waitDelay, Env: cmdrunner.GitEnv(environ),
		ProcessGroup: true, GroupStopGrace: b.waitDelay,
	}
	gh = cmdrunner.Options{MaxOutput: b.maxOutput, WaitDelay: b.waitDelay, Env: ghEnv}
	return git, gh
}

// runGit runs one git command under the per-command time limit, with the
// hardening options before its arguments.
func (t *Tool) runGit(ctx context.Context, args ...string) ([]byte, error) {
	return t.runGitInput(ctx, nil, args...)
}

// runGitInput is runGit with standard input.
func (t *Tool) runGitInput(ctx context.Context, stdin []byte, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, t.bounds.commandTimeout)
	defer cancel()
	full := append(cmdrunner.GitHardening(), args...)
	if stdin == nil {
		return t.git.Run(ctx, "git", full...)
	}
	return t.git.RunInput(ctx, stdin, "git", full...)
}

// runGH runs one gh api -i command against github.com under the per-command
// time limit.
func (t *Tool) runGH(ctx context.Context, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, t.bounds.commandTimeout)
	defer cancel()
	return t.gh.Run(ctx, "gh", ghArgs(args...)...)
}

// ghArgs returns the arguments of one gh api -i call against github.com.
func ghArgs(args ...string) []string {
	return append([]string{"api", "--hostname", "github.com", "-i"}, args...)
}

// isTimeout reports whether err comes from a context that ended: the
// per-command limit, the per-run limit, or a cancellation.
func isTimeout(err error) bool {
	return errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled)
}

// gitStop maps a failed git command to a stop. A command that talks to
// origin fails as network-or-github, a local one as local-git; a time-out is
// network-or-github and an exceeded output cap is list-limit either way. The
// detail names the exit status only, never git's standard error.
func gitStop(err error, step string, remote bool) *StopError {
	switch {
	case isTimeout(err):
		return stop(StopNetworkOrGitHub, step, "git did not finish in time")
	case errors.Is(err, cmdrunner.ErrOutputTooLarge):
		return stop(StopListLimit, step, "git output exceeds the cap")
	}
	exitErr, ok := errors.AsType[*cmdrunner.ExitError](err)
	if !ok {
		return stop(StopLocalGit, step, "git could not be run")
	}
	if remote {
		return stop(StopNetworkOrGitHub, step, "git exited with status %d", exitErr.Code)
	}
	return stop(StopLocalGit, step, "git exited with status %d", exitErr.Code)
}
