// Package main is the entry point of the /mergepr tool: it prepares a PR for
// message drafting, then merges and cleans up after the user approves.
//
// Every subcommand requires --protocol N before its positional argument; a
// value other than mergepr.Protocol, or none, stops the tool before it reads
// or writes anything. prepare and merge also check the task and ADR numbers
// the PR adds; each finding is printed on a line of its own, and the tool
// exits with status 1 without merging.
//
// The squash body carries the editorial correction lines of the PR's
// commits. prepare writes the accepted lines to corrections.txt and prints
// each rejected line; paragraph writes the block the body must hold;
// check-body checks a subject and body against it before the approval; merge
// checks them again before the merge and compares the merge commit with the
// body after it; cleanup --body-file runs that comparison too. paragraph,
// check-body, and merge take the drops --drop N:disputed, --drop
// N:reverted, and --drop-rejected COMMIT:LINE, each any number of times; a
// value of another form, or a line named twice, stops the tool before it
// reads anything.
//
// The take-in subcommands mergemain-begin, mergemain-merge, mergemain-restore,
// and mergemain-push print one field per line, key: value, starting with
// status: ok or status: stop; a stop names its reason and whether the caller
// must restore. They exit with status 0 for a finished run and 1 for a stop. A
// value that could span lines is escaped, so it can never start a field line
// of its own.
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
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/isseis/yt2column/internal/mergepr"
	"github.com/isseis/yt2column/internal/numreserve"
)

var errUsage = errors.New("usage: mergepr prepare --protocol N [PR] | " +
	"mergepr paragraph --protocol N --state FILE [DROPS] | " +
	"mergepr check-body --protocol N --state FILE --subject-file FILE --body-file FILE [DROPS] | " +
	"mergepr merge --protocol N --state FILE --subject-file FILE --body-file FILE [DROPS] | " +
	"mergepr cleanup --protocol N --state FILE [--body-file FILE] | mergepr discard --protocol N --state FILE | " +
	"mergepr mergemain-begin --protocol N PR | " +
	"mergepr mergemain-merge --protocol N --head-branch BRANCH --base-head COMMIT --main COMMIT PR | " +
	"mergepr mergemain-restore --protocol N --head-branch BRANCH --base-head COMMIT --main COMMIT PR | " +
	"mergepr mergemain-push --protocol N --base-head COMMIT --main COMMIT PR; " +
	"DROPS is any number of --drop N:disputed, --drop N:reverted, and --drop-rejected COMMIT:LINE")

// errStopped reports a take-in subcommand's declared stop: its status block
// already holds the reason and whether the caller must restore, so main exits
// with status 1 without printing an error line of its own.
var errStopped = errors.New("mergemain stopped")

// protocolRemedy tells the operator how to bring the binary and the command
// documents to the same --protocol: the tool cannot know which side is older.
const protocolRemedy = "if this mergepr is older than the command documents, reinstall it from main (make install-mergepr); " +
	"if the documents are older, merge origin/main into the branch"

func main() {
	os.Exit(exitStatus(os.Stderr, run(os.Args[1:], numreserve.New(os.Environ()))))
}

// exitStatus writes the error line for err, unless a take-in subcommand
// already wrote its status block, and returns the process's exit status: 0
// when err is nil, and 1 otherwise, whether the run finished with a declared
// stop or with an error.
func exitStatus(w io.Writer, err error) int {
	if err == nil {
		return 0
	}
	if !errors.Is(err, errStopped) {
		printError(w, err)
	}
	return 1
}

// printError writes err to w, preceded by one line per finding when the
// number check stopped the tool, so each number and what uses it can be read
// on its own line, and one line per reason when the correction check or the
// post-merge comparison stopped it.
func printError(w io.Writer, err error) {
	if found, ok := errors.AsType[*mergepr.NumberIssuesError](err); ok {
		for _, issue := range found.Issues {
			_, _ = fmt.Fprintln(w, "mergepr: number check:", issue.Line())
		}
	}
	if mismatch, ok := errors.AsType[*mergepr.CorrectionMismatchError](err); ok {
		for _, line := range mismatch.Missing {
			_, _ = fmt.Fprintln(w, "mergepr: correction check: missing:", line)
		}
		for _, line := range mismatch.Extra {
			_, _ = fmt.Fprintln(w, "mergepr: correction check: extra:", line)
		}
		if mismatch.NotParagraph {
			_, _ = fmt.Fprintln(w, "mergepr: correction check: the correction lines are not one paragraph of their own")
		}
		if mismatch.TrailersChanged {
			_, _ = fmt.Fprintln(w, "mergepr: correction check: the correction lines change which trailers git reads from the body")
		}
		for _, r := range mismatch.Undeclared {
			_, _ = fmt.Fprintln(w, "mergepr:", rejectedLine(r))
		}
		for _, drop := range mismatch.UnknownDrops {
			_, _ = fmt.Fprintln(w, "mergepr: correction check: names no line:", drop)
		}
	}
	if notCarried, ok := errors.AsType[*mergepr.CorrectionsNotCarriedError](err); ok {
		for _, line := range notCarried.Missing {
			_, _ = fmt.Fprintln(w, "mergepr: post-merge check: missing:", line)
		}
		for _, line := range notCarried.Extra {
			_, _ = fmt.Fprintln(w, "mergepr: post-merge check: extra:", line)
		}
	}
	_, _ = fmt.Fprintln(w, "mergepr:", err)
}

// rejectedLine is the line prepare prints for a rejected correction line,
// and printError for one no --drop-rejected declares.
func rejectedLine(r mergepr.RejectedLine) string {
	kinds := make([]string, len(r.Kinds))
	for i, k := range r.Kinds {
		kinds[i] = k.String()
	}
	return fmt.Sprintf("correction rejected: %s line %d: %s: %s", r.Commit, r.Line, strings.Join(kinds, ","), r.Shown)
}

// invocation is a parsed command line.
type invocation struct {
	subcommand  string
	pr          string
	statePath   string
	subjectPath string
	bodyPath    string
	headBranch  string
	baseHead    string
	main        string
	drops       mergepr.Drops
}

// dropFlag collects the --drop values, each parsed by mergepr.ParseDrop.
type dropFlag struct{ drops *mergepr.Drops }

func (f dropFlag) String() string { return "" }

func (f dropFlag) Set(value string) error {
	d, err := mergepr.ParseDrop(value)
	if err != nil {
		return err
	}
	f.drops.Accepted = append(f.drops.Accepted, d)
	return nil
}

// dropRejectedFlag collects the --drop-rejected values, each parsed by
// mergepr.ParseRejectedDrop.
type dropRejectedFlag struct{ drops *mergepr.Drops }

func (f dropRejectedFlag) String() string { return "" }

func (f dropRejectedFlag) Set(value string) error {
	d, err := mergepr.ParseRejectedDrop(value)
	if err != nil {
		return err
	}
	f.drops.Rejected = append(f.drops.Rejected, d)
	return nil
}

// parse checks the command line's shape: a known subcommand, its required
// flags and positional argument, drops of the right form that name no line
// twice and only on a subcommand that takes them, and --protocol equal to
// this binary's, compared as text so 01, +1 and 0x1 are refused. It reads
// and writes nothing, so a mismatched binary and command documents stop
// before any effect.
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
	flags.StringVar(&inv.headBranch, "head-branch", "", "the PR's head branch, from mergemain-begin")
	flags.StringVar(&inv.baseHead, "base-head", "", "the PR head take-in started from, from mergemain-begin")
	flags.StringVar(&inv.main, "main", "", "the main being taken in, from mergemain-begin")
	flags.Var(dropFlag{&inv.drops}, "drop", "N:disputed or N:reverted, an accepted correction line to drop")
	flags.Var(dropRejectedFlag{&inv.drops}, "drop-rejected", "COMMIT:LINE, a rejected correction line to drop")
	if err := flags.Parse(args[1:]); err != nil {
		return invocation{}, fmt.Errorf("%w: %w", errUsage, err)
	}
	if err := inv.drops.Validate(); err != nil {
		return invocation{}, fmt.Errorf("%w: %w", errUsage, err)
	}
	// A missing --protocol leaves the empty string, which never matches.
	if *protocol != strconv.Itoa(mergepr.Protocol) {
		return invocation{}, fmt.Errorf("%w: --protocol is missing or does not match this binary's %d; %s", errUsage, mergepr.Protocol, protocolRemedy)
	}
	if !shapeFits(inv, flags.NArg(), len(inv.drops.Accepted)+len(inv.drops.Rejected) > 0) {
		return invocation{}, errUsage
	}
	inv.pr = flags.Arg(0)
	return inv, nil
}

// presence says whether a subcommand requires, refuses, or allows a flag.
type presence int

const (
	refused presence = iota
	required
	allowed
)

// fits reports whether a flag given (or not) meets p.
func (p presence) fits(given bool) bool {
	switch p {
	case required:
		return given
	case refused:
		return !given
	default:
		return true
	}
}

// shape is what a subcommand takes: how many positional arguments, and each
// flag by its presence.
type shape struct {
	minArgs                     int
	maxArgs                     int
	state, subject, body, drops presence
	headBranch, baseHead, main  presence
}

// shapes are the subcommands and what each takes.
var shapes = map[string]shape{
	"prepare":           {maxArgs: 1},
	"paragraph":         {state: required, drops: allowed},
	"check-body":        {state: required, subject: required, body: required, drops: allowed},
	"merge":             {state: required, subject: required, body: required, drops: allowed},
	"cleanup":           {state: required, body: allowed},
	"discard":           {state: required},
	"mergemain-begin":   {minArgs: 1, maxArgs: 1},
	"mergemain-merge":   {minArgs: 1, maxArgs: 1, headBranch: required, baseHead: required, main: required},
	"mergemain-restore": {minArgs: 1, maxArgs: 1, headBranch: required, baseHead: required, main: required},
	"mergemain-push":    {minArgs: 1, maxArgs: 1, baseHead: required, main: required},
}

// shapeFits reports whether the subcommand takes the positional arguments
// (nargs of them), the files, the commits, and the drops the command line
// gives; an unknown subcommand fits nothing.
func shapeFits(inv invocation, nargs int, hasDrops bool) bool {
	s, ok := shapes[inv.subcommand]
	return ok && nargs >= s.minArgs && nargs <= s.maxArgs &&
		s.state.fits(inv.statePath != "") && s.subject.fits(inv.subjectPath != "") &&
		s.body.fits(inv.bodyPath != "") && s.drops.fits(hasDrops) &&
		s.headBranch.fits(inv.headBranch != "") && s.baseHead.fits(inv.baseHead != "") &&
		s.main.fits(inv.main != "")
}

// takeIn is the part of the take-in tool the command drives. *mergepr.Mergemain
// is the production implementation; the tests substitute a fake, so no test
// reaches git or gh.
type takeIn interface {
	MergemainBegin(ctx context.Context, pr mergepr.MergemainPR) (mergepr.MergemainBeginResult, error)
	MergemainMerge(ctx context.Context, pr mergepr.MergemainPR, headBranch, baseHead, main string) (mergepr.MergemainMergeResult, error)
	MergemainRestore(ctx context.Context, pr mergepr.MergemainPR, headBranch, baseHead, main string) (mergepr.MergemainRestoreResult, error)
	MergemainPush(ctx context.Context, pr mergepr.MergemainPR, baseHead, main string) (mergepr.MergemainPushResult, error)
}

// run parses args and runs them, checking the numbers a PR adds with
// numbers; only main passes the production checker.
func run(args []string, numbers mergepr.NumberChecker) error {
	inv, err := parse(args)
	if err != nil {
		return err
	}
	return execute(context.Background(), os.Stdout, mergepr.New(mergepr.NewOSRunner(), numbers), mergepr.NewMergemain(os.LookupEnv), inv)
}

// execute runs a parsed invocation, writing its output to w.
func execute(ctx context.Context, w io.Writer, tool *mergepr.Tool, take takeIn, inv invocation) error {
	switch inv.subcommand {
	case "prepare":
		prepared, err := tool.Prepare(ctx, inv.pr)
		if err != nil {
			return err
		}
		printPrepared(w, prepared)
		return nil
	case "paragraph":
		path, lines, err := tool.Paragraph(ctx, inv.statePath, inv.drops)
		if err != nil {
			return err
		}
		_, _ = fmt.Fprintf(w, "paragraph: %d line(s) in %s\n", lines, path)
		return nil
	case "check-body":
		lines, err := tool.CheckBody(ctx, inv.statePath, inv.subjectPath, inv.bodyPath, inv.drops)
		if err != nil {
			return err
		}
		_, _ = fmt.Fprintf(w, "corrections: %d line(s) checked\n", lines)
		return nil
	case "merge":
		report, err := tool.Merge(ctx, inv.statePath, inv.subjectPath, inv.bodyPath, inv.drops)
		printReport(w, report)
		return err
	case "cleanup":
		report, err := tool.Cleanup(ctx, inv.statePath, inv.bodyPath)
		printReport(w, report)
		return err
	case "discard":
		if err := mergepr.Discard(inv.statePath); err != nil {
			return err
		}
		_, _ = fmt.Fprintln(w, "discarded the prepared work directory")
		return nil
	default:
		return executeTakeIn(ctx, w, take, inv)
	}
}

// executeTakeIn runs one take-in subcommand and writes its status block.
func executeTakeIn(ctx context.Context, w io.Writer, take takeIn, inv invocation) error {
	switch inv.subcommand {
	case "mergemain-begin":
		pr, err := takeInPR(w, inv.pr)
		if err != nil {
			return err
		}
		result, err := take.MergemainBegin(ctx, pr)
		if err != nil {
			return printMergemainStop(w, err)
		}
		printMergemainBegin(w, result)
		return nil
	case "mergemain-merge":
		pr, err := takeInPR(w, inv.pr)
		if err != nil {
			return err
		}
		result, err := take.MergemainMerge(ctx, pr, inv.headBranch, inv.baseHead, inv.main)
		if err != nil {
			return printMergemainStop(w, err)
		}
		printMergemainMerge(w, result)
		return nil
	case "mergemain-restore":
		pr, err := takeInPR(w, inv.pr)
		if err != nil {
			return err
		}
		result, err := take.MergemainRestore(ctx, pr, inv.headBranch, inv.baseHead, inv.main)
		if err != nil {
			return printMergemainStop(w, err)
		}
		printMergemainRestore(w, result)
		return nil
	case "mergemain-push":
		pr, err := takeInPR(w, inv.pr)
		if err != nil {
			return err
		}
		result, err := take.MergemainPush(ctx, pr, inv.baseHead, inv.main)
		if err != nil {
			return printMergemainStop(w, err)
		}
		printMergemainPush(w, result)
		return nil
	default:
		return errUsage
	}
}

// takeInPR parses the PR number of a take-in subcommand. A number of another
// form is a stop the tool declares, so it is printed like every other stop.
func takeInPR(w io.Writer, arg string) (mergepr.MergemainPR, error) {
	pr, err := mergepr.ParseMergemainPR(arg)
	if err != nil {
		return mergepr.MergemainPR{}, printMergemainStop(w, err)
	}
	return pr, nil
}

// fieldLine writes one field of a take-in subcommand's output: the key, a
// colon and a space, and the value with every character that could break the
// line protocol escaped. The escape is what keeps a value that holds a newline
// from writing a field line of its own.
func fieldLine(w io.Writer, key, value string) {
	_, _ = fmt.Fprintf(w, "%s: %s\n", key, escapedField(value))
}

// escapedField returns s with every character that could end, break, or
// reorder the field line replaced by a backslash escape, and every other
// character kept as it is: a backslash, a newline, a carriage return, and a
// tab get their short forms; an invalid UTF-8 byte, a control character (C0,
// DEL, C1), a line or paragraph separator, and a format character (the
// bidirectional overrides among them) get \xNN, \uNNNN, or \U00NNNNNN. The
// set is what a reader that decodes the output as UTF-8 may read as a line
// break or use to reorder the line. The backslash goes first, so a value that
// holds the two bytes \ and n is shown differently from one that holds a
// newline.
func escapedField(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		if r == utf8.RuneError && size == 1 {
			_, _ = fmt.Fprintf(&b, `\x%02x`, s[i])
			i++
			continue
		}
		switch {
		case r == '\\':
			b.WriteString(`\\`)
		case r == '\n':
			b.WriteString(`\n`)
		case r == '\r':
			b.WriteString(`\r`)
		case r == '\t':
			b.WriteString(`\t`)
		case fieldHidden(r):
			b.WriteString(fieldEscape(r))
		default:
			b.WriteString(s[i : i+size])
		}
		i += size
	}
	return b.String()
}

// fieldHidden reports whether r would end the line for a reader that decodes
// the output as UTF-8, or reorder what is shown of it.
func fieldHidden(r rune) bool {
	return unicode.Is(unicode.Cc, r) || unicode.Is(unicode.Cf, r) ||
		unicode.Is(unicode.Zl, r) || unicode.Is(unicode.Zp, r)
}

// The first code point each escape form cannot hold: \xNN holds one byte,
// \uNNNN holds the Basic Multilingual Plane, and \U00NNNNNN holds the rest.
const (
	escapeByteLimit    = 0x80
	escapeUnicodeLimit = 0x10000
)

// fieldEscape returns r in the shortest escape form that holds it.
func fieldEscape(r rune) string {
	switch {
	case r < escapeByteLimit:
		return fmt.Sprintf(`\x%02x`, r)
	case r < escapeUnicodeLimit:
		return fmt.Sprintf(`\u%04x`, r)
	default:
		return fmt.Sprintf(`\U%08x`, r)
	}
}

// printMergemainStop writes a take-in stop's block and returns the error that
// makes the tool exit with status 1. A stop of no declared kind is reported as
// one, with the zero values of the reason and of the restore need, so the
// caller never guesses a restore from a detail line.
func printMergemainStop(w io.Writer, err error) error {
	fieldLine(w, "status", "stop")
	if stopped, ok := errors.AsType[*mergepr.MergemainStopError](err); ok {
		fieldLine(w, "reason", stopped.Stop.String())
		fieldLine(w, "restore", stopped.Restore.String())
		printConflicts(w, stopped.Conflicts)
		fieldLine(w, "detail", stopped.Detail)
	} else {
		fieldLine(w, "reason", mergepr.MergemainStopUnknown.String())
		fieldLine(w, "restore", mergepr.RestoreUnknown.String())
		fieldLine(w, "detail", err.Error())
	}
	return errStopped
}

// printConflicts writes one conflict line per conflicted path, with its class.
func printConflicts(w io.Writer, conflicts []mergepr.PathConflict) {
	for _, c := range conflicts {
		fieldLine(w, "conflict", c.Class.String()+" "+c.Path)
	}
}

// printMergemainBegin writes what mergemain-begin passes on: the three values
// the later subcommands take, and the report needs.
func printMergemainBegin(w io.Writer, result mergepr.MergemainBeginResult) {
	fieldLine(w, "status", "ok")
	fieldLine(w, "base_head", result.BaseHead)
	fieldLine(w, "head_branch", result.HeadBranch)
	fieldLine(w, "main_head", result.Main)
}

// printMergemainMerge writes a merge that left only resolvable conflicts: the
// conflicts and the facts the README, ADR, protocol-number, and ext-check
// rules need.
func printMergemainMerge(w io.Writer, result mergepr.MergemainMergeResult) {
	fieldLine(w, "status", "ok")
	printConflicts(w, result.Conflicts)
	for _, bump := range result.Protocol {
		fieldLine(w, "protocol_tool", bump.Tool+" "+strconv.Itoa(bump.Value))
	}
	fieldLine(w, "readme_changed", strconv.FormatBool(result.ReadmeChanged))
	fieldLine(w, "adr_main_changed", strconv.FormatBool(result.ADRMainChanged))
	for _, number := range result.ADRConflictNumbers {
		fieldLine(w, "adr_conflict_number", number)
	}
	fieldLine(w, "ext_check_hint", strconv.FormatBool(result.ExtCheckHint))
}

// printMergemainRestore writes the head the reset left, which is the base head.
func printMergemainRestore(w io.Writer, result mergepr.MergemainRestoreResult) {
	fieldLine(w, "status", "ok")
	fieldLine(w, "base_head", result.Head)
}

// printMergemainPush writes the pushed merge commit, the PR's new head.
func printMergemainPush(w io.Writer, result mergepr.MergemainPushResult) {
	fieldLine(w, "status", "ok")
	fieldLine(w, "new_head", result.NewHead)
}

// printPrepared writes what prepare found: the PR, the checks that passed,
// the correction lines (one line for each rejected one), and the files.
func printPrepared(w io.Writer, prepared mergepr.Prepared) {
	s := prepared.State
	_, _ = fmt.Fprintf(w, "PR #%d: %s\n", s.Number, s.Title)
	_, _ = fmt.Fprintf(w, "url:   %s\n", s.URL)
	_, _ = fmt.Fprintf(w, "head:  %s at %s\n", s.HeadRefName, s.HeadRefOID)
	_, _ = fmt.Fprintf(w, "base:  %s\n", s.BaseRefName)
	_, _ = fmt.Fprintf(w, "protocol: %s\n", prepared.ProtocolCheck)
	_, _ = fmt.Fprintln(w, "CI:    all checks passed")
	_, _ = fmt.Fprintf(w, "corrections: %d accepted, %d rejected\n", prepared.Accepted, len(prepared.Rejected))
	for _, r := range prepared.Rejected {
		_, _ = fmt.Fprintln(w, rejectedLine(r))
	}
	_, _ = fmt.Fprintf(w, "dir:   %s (state.json, log.txt, stat.txt, body.txt, corrections.txt)\n", prepared.Dir)
	_, _ = fmt.Fprintf(w, "state: %s\n", filepath.Join(prepared.Dir, "state.json"))
}

// printReport writes what merge or cleanup did. Once the PR is known to be
// merged it always says whether the post-merge comparison verified the
// correction lines, also when the merge commit could not be learned.
func printReport(w io.Writer, report mergepr.Report) {
	if report.ProtocolCheck != "" {
		_, _ = fmt.Fprintf(w, "protocol:             %s\n", report.ProtocolCheck)
	}
	if report.Merged {
		printMerged(w, report)
	}
	if report.Note != "" {
		_, _ = fmt.Fprintf(w, "note: %s\n", report.Note)
	}
}

// printMerged writes the lines of a merge that is known to have happened.
func printMerged(w io.Writer, report mergepr.Report) {
	if report.MergeCommitOID != "" {
		_, _ = fmt.Fprintf(w, "merge commit:         %s\n", report.MergeCommitOID)
		_, _ = fmt.Fprintf(w, "base branch updated:  %t\n", report.BaseUpdated)
		_, _ = fmt.Fprintf(w, "local branch deleted: %t\n", report.LocalDeleted)
	}
	if report.CorrectionsChecked {
		_, _ = fmt.Fprintf(w, "corrections carried:  %d line(s) verified\n", report.CorrectionsVerified)
	} else {
		_, _ = fmt.Fprintln(w, "corrections carried:  not verified")
	}
}
