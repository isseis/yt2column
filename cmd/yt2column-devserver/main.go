// Package main is the entry point of yt2column-devserver, a development
// server: the same HTTP server, socket, and start lock as yt2column-server,
// but every article comes from a fixed fake LLM (internal/llm/devfake) chosen
// on the command line. It calls no LLM, reads no configuration file, and reads
// no secret. It is for trying the browser extension and the server without an
// LLM, and is never a production binary.
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
	// Ignore SIGPIPE before run writes its first line, as the production
	// server does, so a closed standard-error pipe does not end the process.
	signal.Ignore(syscall.SIGPIPE)
	os.Exit(signalrun.Run(func(ctx context.Context) int {
		return run(ctx, os.Args[1:], os.LookupEnv, os.Stderr, productionDeps())
	}))
}
