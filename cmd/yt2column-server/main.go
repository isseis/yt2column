// Package main is the entry point of yt2column-server, the localhost server
// that turns the transcript the browser extension sends into an article and
// posts it to the webhook.
package main

import (
	"context"
	"os"

	"github.com/isseis/yt2column/internal/signalrun"
)

// main hands the process boundary to signalrun.Run: the first signal cancels
// the context, which stops the server after its in-flight jobs finish, and a
// second signal ends the process at once. os.LookupEnv is passed here and
// nowhere else in this package's production code, so internal/config is the
// only code that reads a variable's value.
func main() {
	os.Exit(signalrun.Run(func(ctx context.Context) int {
		return run(ctx, os.Args[1:], os.LookupEnv, os.Stdout, os.Stderr, productionDeps())
	}))
}
