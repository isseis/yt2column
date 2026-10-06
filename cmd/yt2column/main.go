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
// here and nowhere else in this package, so internal/config is the only code
// that reads a variable's value.
func main() {
	os.Exit(runWithSignals(os.Args[1:], os.LookupEnv, os.Stdout, os.Stderr, productionDeps()))
}

// subscribedSignals returns the signals that interrupt a run: SIGINT and
// SIGTERM always, and SIGHUP unless it was ignored when the process started.
// Subscribing to an ignored SIGHUP would undo nohup, so it is left alone.
func subscribedSignals(ignored func(os.Signal) bool) []os.Signal {
	signals := []os.Signal{os.Interrupt, syscall.SIGTERM}
	if !ignored(syscall.SIGHUP) {
		signals = append(signals, syscall.SIGHUP)
	}
	return signals
}

// runWithSignals runs one invocation under a context that the first
// subscribed signal cancels, and returns the exit code.
//
// Contract: the subscription is dropped as soon as the context ends, so a
// second signal gets the default disposition and terminates the process even
// if run is stuck.
func runWithSignals(args []string, lookup config.LookupFunc, stdout, stderr io.Writer, d deps) int {
	ctx, stop := signal.NotifyContext(context.Background(), subscribedSignals(signal.Ignored)...)
	defer stop()
	go func() {
		<-ctx.Done()
		stop()
	}()
	return run(ctx, args, lookup, stdout, stderr, d)
}
