// Package main is the entry point of yt2column-server, the server that listens
// on a Unix domain socket, turns the transcript the browser extension sends
// through the native messaging host into an article, and posts it to the
// webhook.
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/isseis/yt2column/internal/signalrun"
)

// main hands the process boundary to signalrun.Run: the first signal cancels
// the context, which stops the server after its in-flight jobs finish, and a
// second signal ends the process at once. os.LookupEnv is passed here and
// nowhere else in this package's production code, so internal/config is the
// only code that reads a variable's value.
func main() {
	// Ignore SIGPIPE before run writes its first line: a write to a closed
	// pipe (for example standard error piped to a tee that has exited) then
	// returns EPIPE instead of ending the process, so the server keeps serving
	// and its in-flight jobs finish. Ignoring is inherited by child processes,
	// but the server starts none.
	signal.Ignore(syscall.SIGPIPE)
	os.Exit(signalrun.Run(func(ctx context.Context) int {
		return run(ctx, os.Args[1:], os.LookupEnv, os.Stderr, productionDeps())
	}))
}
