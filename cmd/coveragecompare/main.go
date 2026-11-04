// Package main is the entry point of the coverage comparison tool that
// `make coverage-compare` and `make ext-coverage-compare` build and run: it
// measures BASE and HEAD and compares the coverage of the product code
// function by function.
//
// The command line is exactly `go --base <rev>` or `ext --base <rev>`, with a
// non-empty <rev>. Standard output holds the result lines, ending with the
// final line coverage-compare.Write gives; standard error stays empty. The
// exit code is coveragecompare.ExitCode's: 0 pass, 1 fail, 3 stop.
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/isseis/yt2column/internal/coveragecompare"
	"github.com/isseis/yt2column/internal/signalrun"
)

// exitWriteFailed is the exit code when the result could not be written. It
// is none of the three the result lines stand for, so the make recipe writes
// the internal final line in their place. 2 is left out: Go exits with it on a
// runtime panic (cmd/codexreview/main.go gives the same reason).
const exitWriteFailed = 4

// flagBase is the one flag, given as its own argument followed by its value.
const flagBase = "--base"

// argCount is the length of the one accepted command line: the target, the
// flag, and its value.
const argCount = 3

// The stop reasons of a command line the tool does not accept: an empty BASE
// (what the make recipe passes when BASE was not on make's command line), and
// every other shape.
const (
	emptyBaseDetail = "BASE is not given; give it on make's command line, as BASE=<commit>"
	usageDetail     = "the command line must be: coveragecompare go|ext --base <rev>"
)

// heldSignals are the signals main keeps subscribed for the whole run.
var heldSignals = []os.Signal{syscall.SIGTERM, os.Interrupt, syscall.SIGHUP}

// compareFunc runs one comparison; coveragecompare.Run in production. The
// tests give run another, so they never run this repository's test suites.
type compareFunc func(context.Context, coveragecompare.Options) coveragecompare.Result

// main runs the tool with coveragecompare.Run.
func main() {
	os.Exit(runMain(coveragecompare.Run))
}

// runMain runs the tool under signalrun.Run, whose first signal cancels the
// run, and returns the exit code.
//
// Contract: no signal ends the process before compare has returned, so the
// work directory is removed and the user's state checked whatever signal
// comes. signalrun lets a second signal terminate a stuck process by default;
// holding the signals subscribed for the whole run keeps them from ending it,
// as cmd/codexreview/main.go does with the same lines. A SIGKILL is not
// covered.
func runMain(compare compareFunc) int {
	held := make(chan os.Signal, 1)
	for _, sig := range heldSignals {
		// An ignored signal stays ignored, as signalrun leaves it.
		if !signal.Ignored(sig) {
			signal.Notify(held, sig)
		}
	}
	return signalrun.Run(func(ctx context.Context) int {
		root, err := os.Getwd()
		if err != nil {
			r := coveragecompare.Result{
				Outcome: coveragecompare.OutcomeStop,
				Stop:    coveragecompare.StopInternal,
				Reasons: []coveragecompare.StopReason{{Detail: "reading the current directory failed", Excerpt: err.Error()}},
			}
			return write(os.Stdout, os.Stderr, r)
		}
		return run(ctx, os.Args[1:], root, os.Environ(), os.Stdout, os.Stderr, compare)
	})
}

// parse turns the command line into the comparison's target and BASE:
// exactly the target word, then --base and a non-empty value, in that order.
// Any other shape, the --base=<rev> spelling among them, is refused with the
// usage stop; the refusal keeps the target when the first argument names one,
// so the result's target line says which comparison was asked for.
func parse(args []string) (coveragecompare.Target, string, *coveragecompare.Result) {
	var target coveragecompare.Target
	if len(args) > 0 {
		switch args[0] {
		case "go":
			target = coveragecompare.TargetGo
		case "ext":
			target = coveragecompare.TargetExt
		default:
		}
	}
	if target == 0 || len(args) != argCount || args[1] != flagBase || args[2] == "" {
		detail := usageDetail
		if target != 0 && len(args) == argCount && args[1] == flagBase {
			detail = emptyBaseDetail
		}
		r := coveragecompare.Result{
			Outcome: coveragecompare.OutcomeStop,
			Stop:    coveragecompare.StopUsage,
			Target:  target,
			Reasons: []coveragecompare.StopReason{{Detail: detail}},
		}
		return 0, "", &r
	}
	return target, args[2], nil
}

// run parses args, runs compare with the repository root and the process
// environment, writes the result, and returns the exit code.
func run(ctx context.Context, args []string, root string, environ []string, stdout, stderr io.Writer, compare compareFunc) int {
	target, base, refused := parse(args)
	if refused != nil {
		return write(stdout, stderr, *refused)
	}
	r := compare(ctx, coveragecompare.Options{Target: target, Base: base, RepoRoot: root, Env: environ})
	return write(stdout, stderr, r)
}

// write prints r's result lines and returns the exit code of its outcome. A
// failed write is the one case that writes to standard error, since the
// result lines could not be written.
func write(stdout, stderr io.Writer, r coveragecompare.Result) int {
	if err := coveragecompare.Write(stdout, r); err != nil {
		_, _ = fmt.Fprintln(stderr, "coveragecompare: write the result:", err)
		return exitWriteFailed
	}
	return coveragecompare.ExitCode(r.Outcome)
}
