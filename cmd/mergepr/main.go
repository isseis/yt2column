// Package main is the entry point of the /mergepr tool: it prepares a PR for
// message drafting, then merges and cleans up after the user approves.
//
// Every subcommand requires --protocol N before its positional argument; a
// value other than mergepr.Protocol, or none, stops the tool before it reads
// or writes anything. prepare and merge also check the task and ADR numbers
// the PR adds; each finding is printed on a line of its own, and the tool
// exits with status 1 without merging.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"

	"github.com/isseis/yt2column/internal/mergepr"
	"github.com/isseis/yt2column/internal/numreserve"
)

var errUsage = errors.New("usage: mergepr prepare --protocol N [PR] | " +
	"mergepr merge --protocol N --state FILE --subject-file FILE --body-file FILE | " +
	"mergepr cleanup --protocol N --state FILE | mergepr discard --protocol N --state FILE")

// protocolRemedy tells the operator how to bring the binary and the command
// documents to the same --protocol: the tool cannot know which side is older.
const protocolRemedy = "if this mergepr is older than the command documents, reinstall it from main (make install-mergepr); " +
	"if the documents are older, merge origin/main into the branch"

func main() {
	if err := run(os.Args[1:], numreserve.New(os.Environ())); err != nil {
		printError(os.Stderr, err)
		os.Exit(1)
	}
}

// printError writes err to w, preceded by one line per finding when the
// number check stopped the tool, so each number and what uses it can be read
// on its own line.
func printError(w io.Writer, err error) {
	if found, ok := errors.AsType[*mergepr.NumberIssuesError](err); ok {
		for _, issue := range found.Issues {
			_, _ = fmt.Fprintln(w, "mergepr: number check:", issue.Line())
		}
	}
	_, _ = fmt.Fprintln(w, "mergepr:", err)
}

// invocation is a parsed command line.
type invocation struct {
	subcommand  string
	pr          string
	statePath   string
	subjectPath string
	bodyPath    string
}

// parse checks the command line's shape: a known subcommand, its required
// flags and positional argument, and --protocol equal to this binary's,
// compared as text so 01, +1 and 0x1 are refused. It reads and writes
// nothing, so a mismatched binary and command documents stop before any
// effect.
func parse(args []string) (invocation, error) {
	if len(args) == 0 {
		return invocation{}, errUsage
	}
	inv := invocation{subcommand: args[0]}
	flags := flag.NewFlagSet(inv.subcommand, flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	protocol := flags.String("protocol", "", "this binary's protocol number")
	flags.StringVar(&inv.statePath, "state", "", "state file written by prepare")
	flags.StringVar(&inv.subjectPath, "subject-file", "", "file holding the squash subject")
	flags.StringVar(&inv.bodyPath, "body-file", "", "file holding the squash body")
	if err := flags.Parse(args[1:]); err != nil {
		return invocation{}, fmt.Errorf("%w: %w", errUsage, err)
	}
	// A missing --protocol leaves the empty string, which never matches.
	if *protocol != strconv.Itoa(mergepr.Protocol) {
		return invocation{}, fmt.Errorf("%w: --protocol is missing or does not match this binary's %d; %s", errUsage, mergepr.Protocol, protocolRemedy)
	}
	switch inv.subcommand {
	case "prepare":
		if flags.NArg() > 1 || inv.statePath != "" || inv.subjectPath != "" || inv.bodyPath != "" {
			return invocation{}, errUsage
		}
		inv.pr = flags.Arg(0)
	case "merge":
		if flags.NArg() != 0 || inv.statePath == "" || inv.subjectPath == "" || inv.bodyPath == "" {
			return invocation{}, errUsage
		}
	case "cleanup", "discard":
		if flags.NArg() != 0 || inv.statePath == "" || inv.subjectPath != "" || inv.bodyPath != "" {
			return invocation{}, errUsage
		}
	default:
		return invocation{}, errUsage
	}
	return inv, nil
}

// run parses args and runs them, checking the numbers a PR adds with
// numbers; only main passes the production checker.
func run(args []string, numbers mergepr.NumberChecker) error {
	inv, err := parse(args)
	if err != nil {
		return err
	}
	return execute(context.Background(), mergepr.New(mergepr.NewOSRunner(), numbers), inv)
}

// execute runs a parsed invocation.
func execute(ctx context.Context, tool *mergepr.Tool, inv invocation) error {
	switch inv.subcommand {
	case "prepare":
		prepared, err := tool.Prepare(ctx, inv.pr)
		if err != nil {
			return err
		}
		s := prepared.State
		fmt.Printf("PR #%d: %s\n", s.Number, s.Title)
		fmt.Printf("url:   %s\n", s.URL)
		fmt.Printf("head:  %s at %s\n", s.HeadRefName, s.HeadRefOID)
		fmt.Printf("base:  %s\n", s.BaseRefName)
		fmt.Printf("protocol: %s\n", prepared.ProtocolCheck)
		fmt.Println("CI:    all checks passed")
		fmt.Printf("dir:   %s (state.json, log.txt, stat.txt, body.txt)\n", prepared.Dir)
		fmt.Printf("state: %s\n", filepath.Join(prepared.Dir, "state.json"))
		return nil
	case "merge":
		report, err := tool.Merge(ctx, inv.statePath, inv.subjectPath, inv.bodyPath)
		printReport(report)
		return err
	case "cleanup":
		report, err := tool.Cleanup(ctx, inv.statePath)
		printReport(report)
		return err
	case "discard":
		if err := mergepr.Discard(inv.statePath); err != nil {
			return err
		}
		fmt.Println("discarded the prepared work directory")
		return nil
	default:
		return errUsage
	}
}

func printReport(report mergepr.Report) {
	if report.ProtocolCheck != "" {
		fmt.Printf("protocol:             %s\n", report.ProtocolCheck)
	}
	if report.MergeCommitOID == "" {
		return
	}
	fmt.Printf("merge commit:         %s\n", report.MergeCommitOID)
	fmt.Printf("base branch updated:  %t\n", report.BaseUpdated)
	fmt.Printf("local branch deleted: %t\n", report.LocalDeleted)
	if report.Note != "" {
		fmt.Printf("note: %s\n", report.Note)
	}
}
