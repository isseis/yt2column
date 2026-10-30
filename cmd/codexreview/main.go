// Package main is the entry point of codexreview, the developer tool the
// local CLI review runs when the CodeRabbit CLI fails: it runs Codex CLI's
// review of BASE..HEAD once, with fixed arguments, and prints the judged
// result.
//
// The command line is exactly --protocol N --base SHA, each once. Standard
// output is one JSON object, the internal/codexreview Result. A refusal and a
// run that ends without a result write a one-line diagnostic to standard
// error. The exit code is 0 for findings and no_findings, 3 for
// state_changed, and 1 for every other kind and for a run that ends without
// a result, which leaves standard output empty.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"slices"
	"strconv"
	"syscall"

	"github.com/isseis/yt2column/internal/codexreview"
	"github.com/isseis/yt2column/internal/signalrun"
)

// Exit codes. 2 is left out: Go exits with it on a runtime panic, and so does
// the flag package by default, and neither writes a result. 126 and 127 are
// the shell's.
const (
	exitReviewed     = 0
	exitFailure      = 1
	exitStateChanged = 3
)

// The flags, each given once, as its own argument followed by its value.
const (
	flagProtocol = "--protocol"
	flagBase     = "--base"
	// argCount is the length of the one accepted command line: two flags,
	// each followed by its value.
	argCount = 4
)

var errUsage = errors.New("usage: codexreview --protocol N --base SHA")

// heldSignals are the signals main keeps subscribed for the whole run.
var heldSignals = []os.Signal{syscall.SIGTERM, os.Interrupt, syscall.SIGHUP}

// main runs the tool under signalrun.Run, whose first signal cancels the run.
//
// Contract: no signal ends the process before Run has stopped codex's
// process group. signalrun lets a second signal terminate a stuck process by
// default; here that would end the tool during the grace before the group
// is killed, leaving codex and its shell commands running. Holding the
// signals subscribed keeps them from terminating the process, and the run
// stays bounded by its own limits. A SIGKILL to the tool is not covered.
func main() {
	held := make(chan os.Signal, 1)
	for _, sig := range heldSignals {
		// An ignored signal stays ignored, as signalrun leaves it.
		if !signal.Ignored(sig) {
			signal.Notify(held, sig)
		}
	}
	os.Exit(signalrun.Run(func(ctx context.Context) int {
		return run(ctx, os.Args[1:], os.Environ(), os.Stdout, os.Stderr)
	}))
}

// parse checks the command line: exactly --protocol N --base SHA, in either
// order, each once, with no other argument and no other spelling of a flag.
// A --protocol other than Protocol is reported first, so a stale install is
// named even when a newer document adds a flag. It returns the refusal's
// result when the line is refused; the base's form is checked by
// codexreview.Run.
func parse(args []string) (string, *codexreview.Result, error) {
	if i := slices.Index(args, flagProtocol); i >= 0 && i+1 < len(args) && args[i+1] != strconv.Itoa(codexreview.Protocol) {
		r := codexreview.ProtocolMismatchResult()
		return "", &r, fmt.Errorf("%w: --protocol does not match this binary's %d", errUsage, codexreview.Protocol)
	}
	refuse := func() (string, *codexreview.Result, error) {
		r := codexreview.BadArgumentsResult()
		return "", &r, errUsage
	}
	if len(args) != argCount {
		return refuse()
	}
	values := map[string]string{}
	for i := 0; i < len(args); i += 2 {
		name := args[i]
		if _, dup := values[name]; dup || (name != flagProtocol && name != flagBase) {
			return refuse()
		}
		values[name] = args[i+1]
	}
	return values[flagBase], nil, nil
}

// run parses args, runs the review, writes its result, and returns the exit
// code. environ is the tool's environment, passed on to codexreview.Run.
func run(ctx context.Context, args, environ []string, stdout, stderr io.Writer) int {
	base, refused, err := parse(args)
	if refused != nil {
		_, _ = fmt.Fprintln(stderr, "codexreview:", err)
		return write(stdout, stderr, *refused)
	}
	r, err := codexreview.Run(ctx, base, environ)
	if err != nil {
		// No result: whether the worktree changed is unknown, or the run was
		// interrupted. The caller stops on the empty output.
		_, _ = fmt.Fprintln(stderr, "codexreview:", err)
		return exitFailure
	}
	return write(stdout, stderr, r)
}

// write prints r as one JSON object and returns the exit code of its kind.
func write(stdout, stderr io.Writer, r codexreview.Result) int {
	data, err := json.Marshal(r)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "codexreview:", err)
		return exitFailure
	}
	if _, err := stdout.Write(append(data, '\n')); err != nil {
		_, _ = fmt.Fprintln(stderr, "codexreview: write the result:", err)
		return exitFailure
	}
	return exitCode(r.Kind)
}

// exitCode maps a kind to the exit code. An unknown kind is a failure, never
// a review.
func exitCode(kind codexreview.Kind) int {
	switch kind {
	case codexreview.KindFindings, codexreview.KindNoFindings:
		return exitReviewed
	case codexreview.KindStateChanged:
		return exitStateChanged
	default:
		return exitFailure
	}
}
