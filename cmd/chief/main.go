// Package main is the entry point of chief, the developer tool a Claude Code
// session drives to watch the Herdr workspaces of this repository.
//
// The command line is `chief <subcommand> --protocol N`. Standard output is
// one JSON value. The exit code is 0 for a finished run (a decision not to
// send is a finished run), 1 for a failure of the tool, 2 for a wrong
// command line, and 3 for a refusal that did nothing: outside Herdr, another
// chief holds the lock, start has not run, --protocol does not match, or the
// state directory's base is missing.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"time"

	"github.com/isseis/yt2column/internal/chief"
	"github.com/isseis/yt2column/internal/cmdrunner"
	"github.com/isseis/yt2column/internal/signalrun"
)

// Exit codes. 2 is a usage error; Go's own exit 2 after a runtime panic
// writes no result, so a caller reading JSON tells them apart.
const (
	exitOK      = 0
	exitFailure = 1
	exitUsage   = 2
	exitRefused = 3
)

// The subcommands this build knows.
const (
	cmdStart    = "start"
	cmdSnapshot = "snapshot"
)

const flagProtocol = "--protocol"

var errUsage = errors.New("usage: chief (start|snapshot) --protocol N")

func main() {
	os.Exit(signalrun.Run(func(ctx context.Context) int {
		env := chief.ReadEnv(os.LookupEnv)
		return run(ctx, os.Args[1:], env, os.Environ(), func(o cmdrunner.Options) cmdrunner.Runner { return cmdrunner.NewOS(o) }, time.Now, os.Stdout, os.Stderr)
	}))
}

// result is the JSON of a run that is not a snapshot.
type result struct {
	Protocol int    `json:"protocol"`
	Outcome  string `json:"outcome"`
	Reason   string `json:"reason,omitempty"`
	Message  string `json:"message,omitempty"`
	StateDir string `json:"state_dir,omitempty"`
	TookOver bool   `json:"took_over,omitempty"`
	Removed  int    `json:"removed_notices,omitempty"`
}

// parse returns the subcommand of a command line that is exactly a known
// subcommand and --protocol N, and the N given. The protocol is returned as
// text: a mismatch is a refusal, not a usage error.
func parse(args []string) (subcommand, protocol string, err error) {
	if len(args) != 3 || args[1] != flagProtocol {
		return "", "", errUsage
	}
	if args[0] != cmdStart && args[0] != cmdSnapshot {
		return "", "", fmt.Errorf("%w: unknown subcommand %q", errUsage, args[0])
	}
	return args[0], args[2], nil
}

// run is the whole tool. env, environ, the runner factory, and the clock are
// passed in, so a test runs it without a real Herdr, git, gh, or home.
func run(ctx context.Context, args []string, env chief.Env, environ []string, factory chief.RunnerFactory, now func() time.Time, stdout, stderr io.Writer) int {
	subcommand, protocol, err := parse(args)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "chief:", err)
		return exitUsage
	}
	refuse := func(reason, message string) int {
		return write(stdout, stderr, result{Protocol: chief.Protocol, Outcome: "refused", Reason: reason, Message: message}, exitRefused)
	}
	fail := func(err error) int {
		return write(stdout, stderr, result{Protocol: chief.Protocol, Outcome: "failed", Message: chief.Sanitize(err.Error())}, exitFailure)
	}
	if protocol != strconv.Itoa(chief.Protocol) {
		return refuse("protocol_mismatch", fmt.Sprintf("this chief speaks protocol %d, not %s; reinstall it", chief.Protocol, chief.Sanitize(protocol)))
	}
	// Outside Herdr nothing is read: no file, no command.
	if env.PaneID == "" {
		return refuse("not_in_herdr", "chief runs only in a Herdr pane")
	}
	base, err := chief.StateBase(env)
	if err != nil {
		return refuse("no_state_base", err.Error())
	}
	store, err := chief.OpenStore(base)
	switch {
	case errors.Is(err, chief.ErrNoStateBase):
		return refuse("no_state_base", err.Error())
	case err != nil:
		return fail(err)
	}
	cmds := chief.NewCommands(factory, environ)
	owner, err := chief.ResolveOwner(ctx, chief.NewHerdr(cmds), env.PaneID)
	switch {
	case errors.Is(err, chief.ErrNoSession):
		return refuse("no_session", err.Error())
	case err != nil:
		return fail(err)
	}
	switch subcommand {
	case cmdStart:
		return runStart(ctx, stdout, stderr, store, cmds, owner, now, refuse, fail)
	default:
		return runSnapshot(ctx, stdout, stderr, store, cmds, env, owner, now, refuse, fail)
	}
}

func runStart(ctx context.Context, stdout, stderr io.Writer, store chief.Store, cmds chief.Commands, owner chief.Owner, now func() time.Time, refuse func(string, string) int, fail func(error) int) int {
	res, err := store.Start(ctx, chief.NewHerdr(cmds), owner, now())
	switch {
	case errors.Is(err, chief.ErrLockHeld):
		return refuse("lock_held", err.Error())
	case errors.Is(err, chief.ErrLockUnverifiable):
		return refuse("lock_unverifiable", err.Error())
	case err != nil:
		return fail(err)
	}
	return write(stdout, stderr, result{Protocol: chief.Protocol, Outcome: "started", StateDir: store.Dir(), TookOver: res.TookOver, Removed: res.Removed}, exitOK)
}

func runSnapshot(ctx context.Context, stdout, stderr io.Writer, store chief.Store, cmds chief.Commands, env chief.Env, owner chief.Owner, now func() time.Time, refuse func(string, string) int, fail func(error) int) int {
	err := store.Require(owner)
	switch {
	case errors.Is(err, chief.ErrNotStarted):
		return refuse("not_started", err.Error())
	case errors.Is(err, chief.ErrNotOwner):
		return refuse("not_owner", err.Error())
	case errors.Is(err, chief.ErrLockUnverifiable):
		return refuse("lock_unverifiable", err.Error())
	case err != nil:
		return fail(err)
	}
	snap, err := chief.TakeSnapshot(ctx, chief.Deps{Env: env, Cmds: cmds, Store: store, Now: now}, owner)
	if err != nil {
		return fail(err)
	}
	return write(stdout, stderr, snap.Output(), exitOK)
}

// write prints v as one JSON line and returns code, or exitFailure when the
// output cannot be written.
func write(stdout, stderr io.Writer, v any, code int) int {
	data, err := json.Marshal(v)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "chief:", err)
		return exitFailure
	}
	if _, err := stdout.Write(append(data, '\n')); err != nil {
		_, _ = fmt.Fprintln(stderr, "chief: write the result:", err)
		return exitFailure
	}
	return code
}
