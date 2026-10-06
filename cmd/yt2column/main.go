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

// signalBuffer holds the first signal and one more, so a second signal
// delivered before signal.Stop returns is kept for reraiseQueued.
const signalBuffer = 2

// runWithSignals runs one invocation under a context that the first
// subscribed signal cancels, and returns the exit code.
//
// Contract: the subscription is dropped by the first signal, before the
// context is canceled, which restores the default disposition (only signals
// not ignored at startup are subscribed), so a second signal terminates the
// process even if run is stuck. A second signal that arrived while the first
// was being handled is queued on the channel rather than lost; it is raised
// again once the default disposition is back. The subscription is also
// dropped when run returns without a signal.
func runWithSignals(args []string, lookup config.LookupFunc, stdout, stderr io.Writer, d deps) int {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	signals := make(chan os.Signal, signalBuffer)
	signal.Notify(signals, subscribedSignals(signal.Ignored)...)
	returned := make(chan struct{})
	handlerDone := make(chan struct{})
	go func() {
		defer close(handlerDone)
		select {
		case <-signals:
			signal.Stop(signals)
			cancel()
			reraiseQueued(signals)
		case <-returned:
		}
	}()

	code := run(ctx, args, lookup, stdout, stderr, d)
	signal.Stop(signals)
	close(returned)
	<-handlerDone
	return code
}

// reraiseQueued raises again a signal queued on signals, which must no longer
// be subscribed: once Stop has returned, nothing more is delivered to the
// channel, and the signal gets the default disposition.
func reraiseQueued(signals <-chan os.Signal) {
	select {
	case sig := <-signals:
		if s, ok := sig.(syscall.Signal); ok {
			_ = syscall.Kill(os.Getpid(), s)
		}
	default:
	}
}
