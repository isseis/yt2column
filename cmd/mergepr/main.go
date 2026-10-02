// Package main is the entry point of the /mergepr tool: it prepares a PR for
// message drafting, then merges and cleans up after the user approves.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"runtime/debug"

	"github.com/isseis/yt2column/internal/mergepr"
)

var (
	errUsage           = errors.New("usage: mergepr prepare [--work-dir DIR] [PR] | mergepr merge --state FILE --subject-file FILE --body-file FILE | mergepr cleanup --state FILE | mergepr diff --state FILE -- PATH")
	errNoBuildRevision = errors.New("mergepr: cannot determine the build revision; build from a git checkout of main")
)

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
	// Bind the binary to the commit it was built from, so a stale installation
	// stops instead of merging with superseded checks. A build without VCS
	// metadata cannot be verified and is refused.
	revision := buildRevision()
	if revision == "" {
		return errNoBuildRevision
	}
	tool, err := mergepr.NewWithRevision(mergepr.NewOSRunner(), revision)
	if err != nil {
		return err
	}
	ctx := context.Background()
	switch args[0] {
	case "prepare":
		return runPrepare(ctx, tool, args[1:])
	case "merge":
		return runMerge(ctx, tool, args[1:])
	case "cleanup":
		return runCleanup(ctx, tool, args[1:])
	case "diff":
		return runDiff(ctx, tool, args[1:])
	default:
		return fmt.Errorf("%w: unknown subcommand %q", errUsage, args[0])
	}
}

func runPrepare(ctx context.Context, tool *mergepr.Tool, args []string) error {
	flags := flag.NewFlagSet("prepare", flag.ContinueOnError)
	workDir := flags.String("work-dir", "", "directory for state and drafting material (default: a fresh temporary directory)")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() > 1 {
		return fmt.Errorf("%w: prepare takes at most one PR argument", errUsage)
	}
	prepared, err := tool.Prepare(ctx, flags.Arg(0), *workDir)
	if err != nil {
		return err
	}
	fmt.Printf("PR #%d: %s\n", prepared.State.Number, prepared.State.Title)
	fmt.Printf("url:   %s\n", prepared.State.URL)
	fmt.Printf("head:  %s at %s\n", prepared.State.HeadRefName, prepared.State.HeadRefOID)
	fmt.Printf("base:  %s\n", prepared.State.BaseRefName)
	fmt.Println("CI:    all checks passed")
	fmt.Printf("state: %s\n", prepared.StatePath)
	fmt.Printf("log:   %s\n", prepared.LogPath)
	fmt.Printf("stat:  %s\n", prepared.StatPath)
	fmt.Printf("body:  %s\n", prepared.BodyPath)
	return nil
}

func runMerge(ctx context.Context, tool *mergepr.Tool, args []string) error {
	flags := flag.NewFlagSet("merge", flag.ContinueOnError)
	statePath := flags.String("state", "", "state file written by prepare")
	subjectPath := flags.String("subject-file", "", "file holding the squash subject")
	bodyPath := flags.String("body-file", "", "file holding the squash body")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *statePath == "" || *subjectPath == "" || *bodyPath == "" {
		return fmt.Errorf("%w: merge requires --state, --subject-file, and --body-file", errUsage)
	}
	report, err := tool.Merge(ctx, *statePath, *subjectPath, *bodyPath)
	if err != nil {
		return err
	}
	printReport(report)
	return nil
}

func runCleanup(ctx context.Context, tool *mergepr.Tool, args []string) error {
	flags := flag.NewFlagSet("cleanup", flag.ContinueOnError)
	statePath := flags.String("state", "", "state file written by prepare")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *statePath == "" {
		return fmt.Errorf("%w: cleanup requires --state", errUsage)
	}
	report, err := tool.Cleanup(ctx, *statePath)
	if err != nil {
		return err
	}
	printReport(report)
	return nil
}

func runDiff(ctx context.Context, tool *mergepr.Tool, args []string) error {
	flags := flag.NewFlagSet("diff", flag.ContinueOnError)
	statePath := flags.String("state", "", "state file written by prepare")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *statePath == "" || flags.NArg() != 1 {
		return fmt.Errorf("%w: diff requires --state and one path after --", errUsage)
	}
	patch, err := tool.Diff(ctx, *statePath, flags.Arg(0))
	if err != nil {
		return err
	}
	if _, err := os.Stdout.Write(patch); err != nil {
		return err
	}
	return nil
}

// buildRevision returns the VCS revision Go embedded at build time, or an empty
// string when the binary was built without version control metadata.
func buildRevision() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return ""
	}
	return revisionFromSettings(info.Settings)
}

// revisionFromSettings extracts the VCS revision, returning empty for a build
// from a modified checkout: it shares main's revision but not its code, so it
// cannot be verified against main.
func revisionFromSettings(settings []debug.BuildSetting) string {
	revision := ""
	modified := false
	for _, setting := range settings {
		switch setting.Key {
		case "vcs.revision":
			revision = setting.Value
		case "vcs.modified":
			modified = setting.Value == "true"
		}
	}
	if modified {
		return ""
	}
	return revision
}

func printReport(report mergepr.Report) {
	fmt.Printf("merge commit:          %s\n", report.MergeCommitOID)
	fmt.Printf("remote branch deleted: %t\n", report.RemoteDeleted)
	fmt.Printf("local branch deleted:  %t\n", report.LocalDeleted)
	fmt.Printf("base branch updated:   %t\n", report.BaseUpdated)
}
