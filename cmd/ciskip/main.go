// Package main is the entry point of ciskip, the CI-only tool that decides
// whether a pull request's heavy CI jobs may be skipped for a
// documentation-only head commit whose parent commit already passed CI against
// the same main commit.
//
// This is a CI-only tool, meant to be called only from the check-changes job
// of .github/workflows/ci.yml with "go run ./cmd/ciskip"; that job passes the
// GitHub Actions context values as flags. On standard output it writes one
// ::notice line, when the synthetic merge commit's first parent was resolved,
// and one diagnostic line. It writes exactly "skip=true\n" or "skip=false\n"
// to the file named by --result-file.
//
// Contract: the exit code is 0 for every completed evaluation, whether the
// heavy jobs may be skipped or must run, so a failed evaluation never fails
// the job. It is 2 only for a command line this tool cannot read or a result
// file it cannot write; standard error then holds one fixed line, never a flag
// value.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/isseis/yt2column/internal/ciskip"
	"github.com/isseis/yt2column/internal/signalrun"
)

// Exit codes.
const (
	exitEvaluated = 0
	exitUsage     = 2
)

const usage = "usage: ciskip --repo OWNER/REPO --head-repo OWNER/REPO --base-repo OWNER/REPO " +
	"--base-ref main --pr-number N --head-sha SHA --run-attempt N --dep-dirs PATH " +
	"--wasm-dep-dirs-ok true|false --result-file PATH"

// writeFailureLine is the one standard-error line for an unwritable result
// file. It names no path, because flag values are untrusted.
const writeFailureLine = "ciskip: the result file could not be written"

// evaluator decides one skip evaluation; ciskip.Evaluator is the production
// one, and tests inject a fake to observe the input and the output.
type evaluator interface {
	Evaluate(ctx context.Context, in ciskip.Input) ciskip.Decision
}

func main() {
	os.Exit(signalrun.Run(func(ctx context.Context) int {
		return run(ctx, os.Args[1:], ciskip.New(os.Environ()), os.Stdout, os.Stderr)
	}))
}

// run parses args, evaluates the skip decision with eval, and writes the
// notice, the diagnostic line, and the result file.
//
// Contract: the result file is written before the exit code is decided, so a
// caller that reads 0 may rely on the file's content.
func run(ctx context.Context, args []string, eval evaluator, stdout, stderr io.Writer) int {
	in, resultFile, ok := parseArgs(args)
	if !ok {
		_, _ = fmt.Fprintln(stderr, usage)
		return exitUsage
	}

	dec := eval.Evaluate(ctx, in)
	if dec.NoticeBaseSHA != "" {
		_, _ = fmt.Fprintf(stdout, "::notice title=ci-base-sha::%s\n", dec.NoticeBaseSHA)
	}
	_, _ = fmt.Fprintln(stdout, diagnosticLine(dec))

	if err := writeResult(resultFile, dec.Skip()); err != nil {
		_, _ = fmt.Fprintln(stderr, writeFailureLine)
		return exitUsage
	}
	return exitEvaluated
}

// parseArgs reads the flags into a ciskip.Input and returns the result-file
// path. A flag syntax error, an extra positional argument, or a missing or
// empty --result-file stops before anything is evaluated.
func parseArgs(args []string) (ciskip.Input, string, bool) {
	flags := flag.NewFlagSet("ciskip", flag.ContinueOnError)
	// Discard flag's own error text: it quotes flag values, which are
	// untrusted.
	flags.SetOutput(io.Discard)

	var in ciskip.Input
	flags.StringVar(&in.Repo, "repo", "", "owner/repository of this workflow run")
	flags.StringVar(&in.HeadRepo, "head-repo", "", "owner/repository of the pull request head")
	flags.StringVar(&in.BaseRepo, "base-repo", "", "owner/repository of the pull request base")
	flags.StringVar(&in.BaseRef, "base-ref", "", "base branch name")
	flags.StringVar(&in.PRNumber, "pr-number", "", "pull request number")
	flags.StringVar(&in.HeadSHA, "head-sha", "", "pull request head commit SHA")
	flags.StringVar(&in.RunAttempt, "run-attempt", "", "workflow run attempt")
	flags.StringVar(&in.DepDirsFile, "dep-dirs", "", "file listing the WASM dependency directories")
	flags.StringVar(&in.WasmDepDirsOK, "wasm-dep-dirs-ok", "", "whether the dependency listing succeeded")
	resultFile := flags.String("result-file", "", "file the decision is written to")

	if err := flags.Parse(args); err != nil {
		return ciskip.Input{}, "", false
	}
	if flags.NArg() > 0 || *resultFile == "" {
		return ciskip.Input{}, "", false
	}
	return in, *resultFile, true
}

// writeResult writes the decision to path in the exact form the workflow's
// final step compares against: one line, "skip=true" or "skip=false".
func writeResult(path string, skip bool) error {
	content := "skip=false\n"
	if skip {
		content = "skip=true\n"
	}
	//nolint:gosec // G304/G306: the path is the runner-provided temporary file named by --result-file, and the content is not secret.
	return os.WriteFile(path, []byte(content), 0o600)
}

// diagnosticLine is the one standard-output line that says whether the heavy
// jobs are skipped and, when they are not, why. It carries only declared
// Reason codes and bounded identifiers (a SHA prefix and an integer), never a
// response body, a token, or a raw flag value.
func diagnosticLine(dec ciskip.Decision) string {
	if dec.Skip() {
		return fmt.Sprintf("ciskip: decision=skip reason=%s parent=%s run=%d base=%s",
			dec.Reason, shortSHA(dec.ParentSHA), dec.ParentRunID, shortSHA(dec.NoticeBaseSHA))
	}
	line := fmt.Sprintf("ciskip: decision=run reason=%s", dec.Reason)
	if dec.ParentRunID > 0 {
		line += fmt.Sprintf(" run=%d", dec.ParentRunID)
	}
	if dec.HTTPStatus > 0 {
		line += fmt.Sprintf(" status=%d", dec.HTTPStatus)
	}
	return line
}

// shortSHALen is the number of a commit ID's characters the diagnostic line
// shows.
const shortSHALen = 12

// shortSHA is the short prefix the diagnostic line shows for a commit.
func shortSHA(sha string) string {
	if len(sha) > shortSHALen {
		return sha[:shortSHALen]
	}
	return sha
}
