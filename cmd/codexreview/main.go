// Package main is the entry point of codexreview, the developer tool the
// local CLI review runs when the CodeRabbit CLI fails: it runs Codex CLI's
// review of BASE..HEAD once, with fixed arguments, and prints the judged
// result.
//
// The command line is exactly --protocol N --base SHA, each once. Standard
// output is one JSON object, the internal/codexreview Result; a one-line
// diagnostic goes to standard error. The exit code is 0 for findings and
// no_findings, 3 for state_changed, and 1 for every other kind and for a run
// that ends without a result, which leaves standard output empty.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"

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

// The flags.
const (
	flagProtocol = "protocol"
	flagBase     = "base"
)

var (
	errUsage     = errors.New("usage: codexreview --protocol N --base SHA")
	errFlagTwice = errors.New("flag given twice")
)

func main() {
	os.Exit(signalrun.Run(func(ctx context.Context) int {
		return run(ctx, os.Args[1:], os.Environ(), os.Stdout, os.Stderr)
	}))
}

// onceValue stores a flag that may be given only once, so a later value
// cannot silently replace an earlier one.
type onceValue struct {
	name  string
	value *string
	set   *bool
}

func (v onceValue) String() string { return "" }

func (v onceValue) Set(s string) error {
	if *v.set {
		return fmt.Errorf("%w: --%s", errFlagTwice, v.name)
	}
	*v.value, *v.set = s, true
	return nil
}

// parse checks the command line's shape: only --protocol and --base, each
// exactly once, and no positional argument. It returns the refusal's result
// when the line is refused; the base's form is checked by codexreview.Run.
func parse(args []string) (string, *codexreview.Result, error) {
	var protocol, base string
	var protocolSet, baseSet bool
	flags := flag.NewFlagSet("codexreview", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	flags.Var(onceValue{name: flagProtocol, value: &protocol, set: &protocolSet}, flagProtocol, "")
	flags.Var(onceValue{name: flagBase, value: &base, set: &baseSet}, flagBase, "")
	refuse := func(err error) (string, *codexreview.Result, error) {
		r := codexreview.BadArgumentsResult()
		return "", &r, err
	}
	if err := flags.Parse(args); err != nil {
		return refuse(fmt.Errorf("%w: %w", errUsage, err))
	}
	if !protocolSet || !baseSet || flags.NArg() != 0 {
		return refuse(errUsage)
	}
	if protocol != strconv.Itoa(codexreview.Protocol) {
		r := codexreview.ProtocolMismatchResult()
		return "", &r, fmt.Errorf("%w: --protocol does not match this binary's %d", errUsage, codexreview.Protocol)
	}
	return base, nil, nil
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
