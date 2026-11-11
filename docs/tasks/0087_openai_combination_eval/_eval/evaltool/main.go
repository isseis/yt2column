// Command evaltool computes and checks the values of task 0087's evaluation:
// the aggregate tables the evaluation record copies, the estimate and the
// spending bounds, the model rates, the scoring material, the launch gate that
// keeps paid runs within the approved limit, and the check that the record's
// copied tables equal the computed ones.
//
// It reads the CLI's standard-error lines and the run records the evaluation
// scripts write, never calls an LLM API, and never touches the network. Every
// subcommand writes only to the paths its flags name.
//
// Usage: evaltool <subcommand> [flags]. The subcommands are aggregate,
// estimate, rates, check-record, material, launch, and reproduce0018; each
// one's flags are listed by running it with -h.
package main

import (
	"errors"
	"fmt"
	"io"
	"os"
)

// errUsage reports a command line that names no known subcommand.
var errUsage = errors.New("usage: evaltool aggregate|estimate|rates|check-record|material|launch|reproduce0018 [flags]")

// subcommands maps each subcommand name to its entry point. Each entry point
// parses its own flags from args and writes its result to stdout.
var subcommands = map[string]func(args []string, stdout io.Writer) error{
	"aggregate":     runAggregate,
	"estimate":      runEstimate,
	"rates":         runRates,
	"check-record":  runCheckRecord,
	"material":      runMaterial,
	"launch":        runLaunch,
	"reproduce0018": runReproduce0018,
}

func main() {
	if err := dispatch(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "evaltool:", err)
		os.Exit(1)
	}
}

// dispatch runs the subcommand args[0] names.
func dispatch(args []string, stdout io.Writer) error {
	if len(args) == 0 {
		return errUsage
	}
	entry, ok := subcommands[args[0]]
	if !ok {
		return errUsage
	}
	return entry(args[1:], stdout)
}
