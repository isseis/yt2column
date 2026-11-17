// Package main is yt2column-extjob, a development command that runs the
// browser extension's article generation outside the browser: it passes an
// input file to the same JSON entry points the article generation module
// registers (internal/extjob), natively, so a problem seen in the extension
// can be reproduced and followed in a debugger. The LLM is chosen with
// -llm=claude (the production profile, with the API key read from the
// environment as the CLI reads it) or -llm=fake (a devfake scenario); there is
// no default, so a forgotten choice never becomes a paid call. It is never a
// production binary.
package main

import (
	"context"
	"os"

	"github.com/isseis/yt2column/internal/signalrun"
)

// main hands the process boundary to signalrun.Run: the first signal cancels
// the context, which ends the generation's calls. os.LookupEnv is passed here
// and nowhere else in this package's production code, so internal/config is
// the only code that reads a variable's value.
func main() {
	os.Exit(signalrun.Run(func(ctx context.Context) int {
		return run(ctx, os.Args[1:], os.LookupEnv, os.Stdout, os.Stderr, productionBackends())
	}))
}
