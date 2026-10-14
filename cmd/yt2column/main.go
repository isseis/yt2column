// Package main is the entry point of the yt2column CLI.
package main

import (
	"context"
	"os"

	"github.com/isseis/yt2column/internal/signalrun"
)

// main hands the process boundary to signalrun.Run. os.LookupEnv is passed
// here and nowhere else in this package's production code, so internal/config
// is the only code that reads a variable's value.
func main() {
	os.Exit(signalrun.Run(func(ctx context.Context) int {
		return run(ctx, os.Args[1:], os.LookupEnv, os.Stdout, os.Stderr, productionDeps())
	}))
}
