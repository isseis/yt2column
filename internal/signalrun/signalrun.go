// Package signalrun runs a program's main function under a context that the
// first subscribed signal cancels.
package signalrun

import (
	"context"
	"os"
	"os/signal"
	"syscall"
)

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

// Run calls run under a context that the first subscribed signal cancels, and
// returns its result. (Moved from cmd/yt2column/main.go runWithSignals,
// unchanged in behavior.)
//
// Contract: the subscription is dropped by the first signal, before the
// context is canceled, which restores the default disposition (only signals
// not ignored at startup are subscribed), so a second signal terminates the
// process even if run is stuck. A second signal that arrived while the first
// was being handled is queued on the channel rather than lost; it is raised
// again once the default disposition is back. The subscription is also
// dropped when run returns without a signal.
func Run(run func(ctx context.Context) int) int {
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

	code := run(ctx)
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
