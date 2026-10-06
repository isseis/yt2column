// Package main is the entry point of the yt2column CLI.
package main

import (
	"context"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/isseis/yt2column/internal/config"
)

// main hands the process boundary to runWithSignals. os.LookupEnv is passed
// here and nowhere else in this package's production code, so internal/config
// is the only code that reads a variable's value.
func main() {
	os.Exit(runWithSignals(os.Args[1:], os.LookupEnv, os.Stdout, os.Stderr, productionDeps()))
}

// subscribedSignals returns the signals that interrupt a run: SIGTERM always,
// and SIGINT and SIGHUP unless they were ignored when the process started.
// Subscribing replaces an inherited ignore, which would undo nohup (SIGHUP) or
// let a terminal Ctrl-C reach a job a shell started in the background
// (SIGINT), so an ignored signal is left alone.
func subscribedSignals(ignored func(os.Signal) bool) []os.Signal {
	signals := []os.Signal{syscall.SIGTERM}
	for _, sig := range []os.Signal{os.Interrupt, syscall.SIGHUP} {
		if !ignored(sig) {
			signals = append(signals, sig)
		}
	}
	return signals
}

// runWithSignals runs one invocation under a context that the first
// subscribed signal cancels, and returns the exit code.
//
// Contract: the subscription is dropped as soon as the context ends, which
// restores the default disposition (only signals not ignored at startup are
// subscribed), so a second signal terminates the process even if run is
// stuck.
func runWithSignals(args []string, lookup config.LookupFunc, stdout, stderr io.Writer, d deps) int {
	ctx, stop := signal.NotifyContext(context.Background(), subscribedSignals(signal.Ignored)...)
	defer stop()
	go func() {
		<-ctx.Done()
		stop()
	}()
	return run(ctx, args, lookup, stdout, stderr, d)
}
