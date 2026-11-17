// Package main is the entry point of numreserve, the developer tool that
// decides a collision-free task or ADR number by creating a record tag on
// GitHub.
//
// The command line is exactly one word, "task" or "adr". On success standard
// output is the four-digit number on one line, standard error names the
// record tag created and the origin/main commit it points at, and the exit
// code is 0. When the tool stops, standard output is empty, standard error is
// one line "numreserve: stop <reason>: <step>: <detail>", and the exit code is
// 1. A command line of any other form exits with 2 before anything runs.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/isseis/yt2column/internal/numreserve"
	"github.com/isseis/yt2column/internal/signalrun"
)

// Exit codes.
const (
	exitDecided = 0
	exitStopped = 1
	exitUsage   = 2
)

const usage = "usage: numreserve task|adr"

// reserver decides a number; numreserve.Tool is the production one.
type reserver interface {
	Reserve(ctx context.Context, space numreserve.Space) (numreserve.Reservation, error)
}

func main() {
	os.Exit(signalrun.Run(func(ctx context.Context) int {
		return run(ctx, os.Args[1:], numreserve.New(os.Environ()), os.Stdout, os.Stderr)
	}))
}

// run parses args, decides a number with r, and prints the result.
//
// Contract: the exit code is 0 only when the number was written to stdout;
// every other outcome writes one line to stderr.
func run(ctx context.Context, args []string, r reserver, stdout, stderr io.Writer) int {
	if len(args) != 1 {
		_, _ = fmt.Fprintln(stderr, usage)
		return exitUsage
	}
	space, err := numreserve.ParseSpace(args[0])
	if err != nil {
		_, _ = fmt.Fprintln(stderr, usage)
		return exitUsage
	}
	reservation, err := r.Reserve(ctx, space)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, stopLine(err))
		return exitStopped
	}
	if reservation.Number.String() == "" {
		_, _ = fmt.Fprintln(stderr, "numreserve: no number was decided")
		return exitStopped
	}
	created := fmt.Sprintf("numreserve: created %s at %s", reservation.Tag, reservation.Commit)
	if _, err := fmt.Fprintln(stdout, reservation.Number.String()); err != nil {
		// The tag exists, so the number is used up either way: name it on
		// stderr and fail, rather than let a caller read an empty stdout as
		// success.
		_, _ = fmt.Fprintf(stderr, "%s, but the number could not be written to standard output\n", created)
		return exitStopped
	}
	_, _ = fmt.Fprintln(stderr, created)
	return exitDecided
}

// stopLine is the one standard-error line for a run that decided no number.
// Only a StopError of a declared kind carries a reason word.
func stopLine(err error) string {
	stopErr, ok := errors.AsType[*numreserve.StopError](err)
	if !ok {
		return "numreserve: " + err.Error()
	}
	reason, ok := stopErr.Kind.Reason()
	if !ok {
		return fmt.Sprintf("numreserve: stopped: %s: %s", stopErr.Step, stopErr.Detail)
	}
	return fmt.Sprintf("numreserve: stop %s: %s: %s", reason, stopErr.Step, stopErr.Detail)
}
