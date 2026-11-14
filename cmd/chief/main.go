// Package main is the entry point of chief, the developer tool a Claude Code
// session drives to watch the Herdr workspaces of this repository.
//
// The command line is `chief <subcommand> [flags] --protocol N`, the
// subcommands start, snapshot, send, detail, and release. Standard output is
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
	"slices"
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
	cmdSend     = "send"
	cmdDetail   = "detail"
	cmdRelease  = "release"
)

// The flags.
const (
	flagProtocol  = "--protocol"
	flagWorkspace = "--workspace"
	flagCommand   = "--command"
	flagIssue     = "--issue"
	flagPR        = "--pr"
)

// subcommandFlags lists each subcommand's required and optional flags.
var subcommandFlags = map[string]struct{ required, optional []string }{
	cmdStart:    {required: []string{flagProtocol}},
	cmdSnapshot: {required: []string{flagProtocol}},
	cmdSend:     {required: []string{flagProtocol, flagWorkspace, flagCommand}, optional: []string{flagIssue, flagPR}},
	cmdDetail:   {required: []string{flagProtocol, flagWorkspace}},
	cmdRelease:  {required: []string{flagProtocol, flagWorkspace}},
}

var errUsage = errors.New("usage: chief start|snapshot --protocol N | " +
	"send --workspace W --command K [--issue N|--pr N] --protocol N | " +
	"detail --workspace W --protocol N | release --workspace W --protocol N")

func main() {
	os.Exit(signalrun.Run(func(ctx context.Context) int {
		env := chief.ReadEnv(os.LookupEnv)
		return run(ctx, os.Args[1:], env, os.Environ(), func(o cmdrunner.Options) cmdrunner.Runner { return cmdrunner.NewOS(o) }, time.Now, chief.SleepContext, os.Stdout, os.Stderr)
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

// parse returns the subcommand and its flags. Contract: every flag is one of
// the subcommand's, given once with a value, and every required flag is
// given; anything else is a usage error. The protocol is returned as text: a
// mismatch is a refusal, not a usage error.
func parse(args []string) (subcommand string, flags map[string]string, err error) {
	if len(args) == 0 {
		return "", nil, errUsage
	}
	spec, ok := subcommandFlags[args[0]]
	if !ok {
		return "", nil, fmt.Errorf("%w: unknown subcommand %q", errUsage, args[0])
	}
	rest := args[1:]
	if len(rest)%2 != 0 {
		return "", nil, errUsage
	}
	flags = map[string]string{}
	for i := 0; i < len(rest); i += 2 {
		name, value := rest[i], rest[i+1]
		if !slices.Contains(spec.required, name) && !slices.Contains(spec.optional, name) {
			return "", nil, fmt.Errorf("%w: %s does not take %q", errUsage, args[0], name)
		}
		if _, dup := flags[name]; dup {
			return "", nil, fmt.Errorf("%w: %s given twice", errUsage, name)
		}
		flags[name] = value
	}
	for _, name := range spec.required {
		if _, ok := flags[name]; !ok {
			return "", nil, fmt.Errorf("%w: %s needs %s", errUsage, args[0], name)
		}
	}
	return args[0], flags, nil
}

// run is the whole tool. env, environ, the runner factory, and the clock are
// passed in, so a test runs it without a real Herdr, git, gh, or home.
func run(ctx context.Context, args []string, env chief.Env, environ []string, factory chief.RunnerFactory, now func() time.Time, sleep func(context.Context, time.Duration) error, stdout, stderr io.Writer) int {
	subcommand, flags, err := parse(args)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "chief:", err)
		return exitUsage
	}
	protocol := flags[flagProtocol]
	var sendReq chief.SendRequest
	if subcommand == cmdSend {
		// A malformed request is a usage error, found before anything is
		// read.
		sendReq, err = chief.ParseSendRequest(flags[flagWorkspace], flags[flagCommand], flags[flagIssue], flags[flagPR])
		if err != nil {
			_, _ = fmt.Fprintln(stderr, "chief:", err)
			return exitUsage
		}
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
	if subcommand == cmdStart {
		return runStart(ctx, stdout, stderr, store, cmds, owner, now, refuse, fail)
	}
	if code, ok := requireOwner(store, owner, refuse, fail); !ok {
		return code
	}
	deps := chief.Deps{Env: env, Cmds: cmds, Store: store, Now: now, Sleep: sleep}
	out, err := dispatch(ctx, subcommand, flags, sendReq, deps, owner)
	if err != nil {
		return fail(err)
	}
	return write(stdout, stderr, out, exitOK)
}

// dispatch runs a subcommand other than start, once its caller is known to
// own the lock, and returns its output.
func dispatch(ctx context.Context, subcommand string, flags map[string]string, sendReq chief.SendRequest, deps chief.Deps, owner chief.Owner) (any, error) {
	switch subcommand {
	case cmdSnapshot:
		snap, err := chief.TakeSnapshot(ctx, deps, owner)
		return snap.Output(), err
	case cmdSend:
		out, err := chief.Send(ctx, deps, sendReq)
		return out.Output(), err
	case cmdDetail:
		out, err := chief.Detail(ctx, deps, flags[flagWorkspace])
		return out.Output(), err
	case cmdRelease:
		out, err := chief.Release(ctx, deps, flags[flagWorkspace])
		return out.Output(), err
	default:
		return nil, errUsage
	}
}

// requireOwner refuses a subcommand other than start when start has not run
// in this session or the caller is not the lock's owner.
func requireOwner(store chief.Store, owner chief.Owner, refuse func(string, string) int, fail func(error) int) (int, bool) {
	err := store.Require(owner)
	switch {
	case err == nil:
		return 0, true
	case errors.Is(err, chief.ErrNotStarted):
		return refuse("not_started", err.Error()), false
	case errors.Is(err, chief.ErrNotOwner):
		return refuse("not_owner", err.Error()), false
	case errors.Is(err, chief.ErrLockUnverifiable):
		return refuse("lock_unverifiable", err.Error()), false
	default:
		return fail(err), false
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
	out := result{Protocol: chief.Protocol, Outcome: "started", StateDir: store.Dir(), TookOver: res.TookOver, Removed: res.Removed}
	if res.CleanupError != nil {
		out.Message = "a finished session's notice record could not be removed: " + chief.Sanitize(res.CleanupError.Error())
	}
	return write(stdout, stderr, out, exitOK)
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
