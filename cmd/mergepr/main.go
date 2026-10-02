// Package main is the entry point of the /mergepr tool: it prepares a PR for
// message drafting, then merges and cleans up after the user approves.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/isseis/yt2column/internal/mergepr"
)

var errUsage = errors.New("usage: mergepr prepare [PR] | mergepr merge --state FILE --subject-file FILE --body-file FILE | mergepr cleanup --state FILE")

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "mergepr:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		return errUsage
	}
	tool := mergepr.New(mergepr.NewOSRunner())
	ctx := context.Background()
	flags := flag.NewFlagSet(args[0], flag.ContinueOnError)
	statePath := flags.String("state", "", "state file written by prepare")
	subjectPath := flags.String("subject-file", "", "file holding the squash subject")
	bodyPath := flags.String("body-file", "", "file holding the squash body")
	if err := flags.Parse(args[1:]); err != nil {
		return err
	}
	switch args[0] {
	case "prepare":
		if flags.NArg() > 1 {
			return errUsage
		}
		prepared, err := tool.Prepare(ctx, flags.Arg(0))
		if err != nil {
			return err
		}
		s := prepared.State
		fmt.Printf("PR #%d: %s\n", s.Number, s.Title)
		fmt.Printf("url:   %s\n", s.URL)
		fmt.Printf("head:  %s at %s\n", s.HeadRefName, s.HeadRefOID)
		fmt.Printf("base:  %s\n", s.BaseRefName)
		fmt.Println("CI:    all checks passed")
		fmt.Printf("dir:   %s (state.json, log.txt, stat.txt, body.txt)\n", prepared.Dir)
		fmt.Printf("state: %s\n", filepath.Join(prepared.Dir, "state.json"))
		return nil
	case "merge":
		if *statePath == "" || *subjectPath == "" || *bodyPath == "" {
			return errUsage
		}
		report, err := tool.Merge(ctx, *statePath, *subjectPath, *bodyPath)
		printReport(report)
		return err
	case "cleanup":
		if *statePath == "" {
			return errUsage
		}
		report, err := tool.Cleanup(ctx, *statePath)
		printReport(report)
		return err
	default:
		return errUsage
	}
}

func printReport(report mergepr.Report) {
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
