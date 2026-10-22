// Package main is the entry point of fixprguard, the guard tool /fixpr and
// /runpr call for every decision that must not depend on a session's reading
// of untrusted text: which review threads are trusted, which changes are in
// scope, whether the git state still matches the baseline, and the guarded
// commit, push and GitHub writes.
//
// Every subcommand is stateless. Standard output is JSON (fence prints the
// fenced block itself) and carries the protocol number; a one-line diagnostic
// goes to standard error. The exit code is 0 to continue, 1 to stop (the
// output's stop field names why), 2 for a usage error, and 3 when inspect
// needs the user's answer.
package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"time"

	"github.com/isseis/yt2column/internal/cmdrunner"
	"github.com/isseis/yt2column/internal/fixprguard"
)

// Exit codes.
const (
	exitContinue = 0
	exitStop     = 1
	exitUsage    = 2
	exitAsk      = 3
)

const (
	// ghMaxOutput is above the 16 MiB response cap the library enforces, so an
	// oversized response is rejected by that check, not cut short here.
	ghMaxOutput = 32 << 20
	// gitMaxOutput caps git's output; Worktree needs a cap so truncated output
	// cannot hide a change, and over it the run stops.
	gitMaxOutput = 64 << 20
	// makeMaxOutput caps a build check's output, which comes from code the
	// round may have changed; over it the target counts as failed without
	// output. Only the tail of a failed target is reported.
	makeMaxOutput = 64 << 20
	// waitDelay bounds the wait for a cancelled command's pipes.
	waitDelay = 10 * time.Second
	// maxFenceInput caps the file fence reads.
	maxFenceInput = 16 << 20
)

// The subcommands.
const (
	cmdHead    = "head"
	cmdList    = "list"
	cmdBegin   = "begin"
	cmdFence   = "fence"
	cmdInspect = "inspect"
	cmdFinish  = "finish"
	cmdPush    = "push"
	cmdReply   = "reply"
	cmdEditPR  = "edit-pr"
)

// The flags.
const (
	flagProtocol    = "protocol"
	flagReviewer    = "reviewer"
	flagAccept      = "accept"
	flagBaseline    = "baseline"
	flagThread      = "thread"
	flagJudgedSet   = "judged-set"
	flagCommit      = "commit"
	flagThreadID    = "thread-id"
	flagCommentID   = "comment-id"
	flagFingerprint = "fingerprint"
	flagBodyFile    = "body-file"
	flagTitleFile   = "title-file"
)

// positional says what positional argument a subcommand takes.
type positional int

const (
	argNone       positional = iota
	argOptionalPR            // a PR number; the current branch's PR when omitted
	argRequiredPR
	argFile
)

// spec declares a subcommand's flags, the ones it requires (--protocol is
// always required), its positional argument, and whether it runs gh.
type spec struct {
	flags    []string
	required []string
	arg      positional
	usesGH   bool
}

var specs = map[string]spec{
	cmdHead:    {arg: argOptionalPR, usesGH: true},
	cmdList:    {flags: []string{flagReviewer}, required: []string{flagReviewer}, arg: argOptionalPR, usesGH: true},
	cmdBegin:   {flags: []string{flagReviewer, flagAccept}, required: []string{flagReviewer}, arg: argOptionalPR, usesGH: true},
	cmdFence:   {arg: argFile},
	cmdInspect: {flags: []string{flagReviewer, flagBaseline, flagThread}, required: []string{flagReviewer, flagBaseline}, arg: argOptionalPR, usesGH: true},
	cmdFinish:  {flags: []string{flagBaseline, flagJudgedSet}, required: []string{flagBaseline, flagJudgedSet}, arg: argRequiredPR},
	cmdPush:    {flags: []string{flagBaseline, flagCommit}, required: []string{flagBaseline, flagCommit}},
	cmdReply: {
		flags:    []string{flagReviewer, flagThreadID, flagCommentID, flagFingerprint, flagBodyFile, flagAccept},
		required: []string{flagReviewer, flagThreadID, flagCommentID, flagFingerprint, flagBodyFile},
		arg:      argOptionalPR,
		usesGH:   true,
	},
	cmdEditPR: {flags: []string{flagTitleFile, flagBodyFile}, required: []string{flagTitleFile, flagBodyFile}, arg: argOptionalPR, usesGH: true},
}

// ghTargetVariables point gh at another repository or host than the work
// tree's; with either set, the tool would judge one repository's threads and
// change another's files.
var ghTargetVariables = []string{"GH_REPO", "GH_HOST"}

var (
	errUsage     = errors.New("usage")
	errFlagTwice = errors.New("flag given twice")
)

// guidePath is where the parse diagnostic points for the usage.
const guidePath = "docs/dev/developer_guide/fixprguard_guide.md"

// env is what a run reaches outside the process; tests replace each part.
type env struct {
	gh        cmdrunner.InputRunner
	git       cmdrunner.Runner
	make      cmdrunner.Runner
	stdout    io.Writer
	stderr    io.Writer
	rand      io.Reader
	lookupEnv func(string) (string, bool)
}

func main() {
	os.Exit(run(context.Background(), os.Args[1:], osEnv()))
}

// osEnv runs every command with an output cap and without the inherited
// GIT_* variables: gh and the make targets start git too, and GIT_DIR or
// GIT_WORK_TREE would point it at another repository than the one the tool
// checks and pushes.
func osEnv() env {
	gitEnv := fixprguard.GitEnv(os.Environ())
	return env{
		gh:        cmdrunner.NewOS(cmdrunner.Options{MaxOutput: ghMaxOutput, WaitDelay: waitDelay, Env: gitEnv}),
		git:       cmdrunner.NewOS(cmdrunner.Options{MaxOutput: gitMaxOutput, WaitDelay: waitDelay, Env: gitEnv}),
		make:      cmdrunner.NewOS(cmdrunner.Options{MaxOutput: makeMaxOutput, WaitDelay: waitDelay, Env: gitEnv}),
		stdout:    os.Stdout,
		stderr:    os.Stderr,
		rand:      rand.Reader,
		lookupEnv: os.LookupEnv,
	}
}

// invocation is a parsed command line. Flag values are still raw strings;
// execute checks each against its grammar.
type invocation struct {
	name   string
	values map[string]string
	multi  map[string][]string
	arg    string
}

// multiFlags are the flags that may be given more than once; each collects
// every value in order. --reviewer is one of them: a name given twice is
// refused by fixprguard.ParseReviewers, not here, so the one check of the
// list covers both the command line and the documents' guard.
var multiFlags = map[string]struct{}{flagAccept: {}, flagThread: {}, flagReviewer: {}}

// flagValue stores one flag of an invocation. A multi flag (multiFlags)
// collects every value; any other flag given twice is an error, so a later
// value cannot silently replace an earlier one.
type flagValue struct {
	inv   *invocation
	name  string
	multi bool
}

func (f flagValue) String() string { return "" }

func (f flagValue) Set(s string) error {
	if f.multi {
		f.inv.multi[f.name] = append(f.inv.multi[f.name], s)
		return nil
	}
	if _, dup := f.inv.values[f.name]; dup {
		return fmt.Errorf("%w: --%s", errFlagTwice, f.name)
	}
	f.inv.values[f.name] = s
	return nil
}

// parse checks the command line's shape: a known subcommand, only its own
// flags, the required flags present, the positional argument it takes, and
// --protocol equal to this binary's. Values are checked by execute.
func parse(args []string) (invocation, error) {
	if len(args) == 0 {
		return invocation{}, errUsage
	}
	sp, ok := specs[args[0]]
	if !ok {
		return invocation{}, errUsage
	}
	inv := invocation{name: args[0], values: map[string]string{}, multi: map[string][]string{}}
	flags := flag.NewFlagSet(inv.name, flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	for _, name := range append([]string{flagProtocol}, sp.flags...) {
		_, multi := multiFlags[name]
		flags.Var(flagValue{inv: &inv, name: name, multi: multi}, name, "")
	}
	if err := flags.Parse(args[1:]); err != nil {
		return invocation{}, fmt.Errorf("%w: %w", errUsage, err)
	}
	// A required flag is present when it has a value, single or collected.
	for _, name := range append([]string{flagProtocol}, sp.required...) {
		_, single := inv.values[name]
		if !single && len(inv.multi[name]) == 0 {
			return invocation{}, fmt.Errorf("%w: --%s is required", errUsage, name)
		}
	}
	if inv.values[flagProtocol] != strconv.Itoa(fixprguard.Protocol) {
		return invocation{}, fmt.Errorf("%w: --protocol does not match this binary's %d", errUsage, fixprguard.Protocol)
	}
	switch sp.arg {
	case argNone:
		if flags.NArg() != 0 {
			return invocation{}, errUsage
		}
	case argOptionalPR:
		if flags.NArg() > 1 {
			return invocation{}, errUsage
		}
		inv.arg = flags.Arg(0)
	case argRequiredPR, argFile:
		if flags.NArg() != 1 {
			return invocation{}, errUsage
		}
		inv.arg = flags.Arg(0)
	}
	// An empty argument is not an omitted one: it is refused, not read as
	// the current branch's PR.
	if flags.NArg() == 1 && inv.arg == "" {
		return invocation{}, fmt.Errorf("%w: empty argument", errUsage)
	}
	return inv, nil
}

// run parses args, runs the subcommand, writes its output, and returns the
// exit code.
func run(ctx context.Context, args []string, e env) int {
	inv, err := parse(args)
	if err != nil {
		_, _ = fmt.Fprintf(e.stderr, "fixprguard: %v (see %s)\n", err, guidePath)
		return exitUsage
	}
	out, code, err := execute(ctx, inv, e)
	if err != nil {
		return stop(e, err)
	}
	if text, ok := out.(string); ok {
		_, _ = io.WriteString(e.stdout, text)
		return code
	}
	data, err := encode(out)
	if err != nil {
		return stop(e, err)
	}
	_, _ = e.stdout.Write(data)
	return code
}

// stop reports err: a usage error (a value outside its grammar, or a text
// file reply or edit-pr refuses) exits 2 with the diagnostic only; any other
// error exits 1 with the stop output. A reply that may have been posted is a
// stop whatever it wraps, so the caller does not post it again.
func stop(e env, err error) int {
	_, _ = fmt.Fprintln(e.stderr, "fixprguard:", err)
	if !errors.Is(err, fixprguard.ErrReplyNotResolved) &&
		(errors.Is(err, fixprguard.ErrInvalidInput) || errors.Is(err, errUsage)) {
		return exitUsage
	}
	data, encErr := encode(stopOutput(err))
	if encErr != nil {
		data, _ = encode(stopReport{Protocol: fixprguard.Protocol, Stop: fixprguard.StopFailed})
	}
	_, _ = e.stdout.Write(data)
	return exitStop
}

// stopReport is the output of a subcommand that stopped. The details are set
// only for the stop kinds that carry them.
type stopReport struct {
	Protocol int                       `json:"protocol"`
	Stop     string                    `json:"stop"`
	Parts    []string                  `json:"parts,omitempty"`
	Changes  []fixprguard.ScopedChange `json:"changes,omitempty"`
	Stage    string                    `json:"stage,omitempty"`
	// Output is the fenced tail of a failed build check's output: text from
	// code the round may have changed.
	Output string `json:"output,omitempty"`
}

func stopOutput(err error) stopReport {
	report := stopReport{Protocol: fixprguard.Protocol, Stop: fixprguard.StopKind(err)}
	if stateErr, ok := errors.AsType[*fixprguard.GitStateError](err); ok {
		report.Parts = stateErr.Parts
	}
	if setErr, ok := errors.AsType[*fixprguard.JudgedSetError](err); ok {
		report.Changes = setErr.Changes
	}
	if buildErr, ok := errors.AsType[*fixprguard.BuildCheckError](err); ok {
		report.Stage = buildErr.Stage
		report.Output = buildErr.Output.Text
	}
	return report
}

// encode writes v as one line of JSON, without escaping HTML characters.
func encode(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// execute runs one parsed subcommand. It returns the output (a string is
// printed as it is) and the exit code for success.
func execute(ctx context.Context, inv invocation, e env) (any, int, error) {
	if specs[inv.name].usesGH {
		for _, name := range ghTargetVariables {
			if _, set := e.lookupEnv(name); set {
				return nil, 0, fmt.Errorf("%w: %s is set; unset it so gh targets this work tree's repository", fixprguard.ErrInvalidInput, name)
			}
		}
	}
	w := fixprguard.NewWorktree(e.git, e.make)
	switch inv.name {
	case cmdHead:
		return head(ctx, inv, e)
	case cmdList:
		return list(ctx, inv, e)
	case cmdBegin:
		return begin(ctx, inv, e, w)
	case cmdFence:
		return fence(inv, e)
	case cmdInspect:
		return inspect(ctx, inv, e, w)
	case cmdFinish:
		return finish(ctx, inv, w)
	case cmdPush:
		return push(ctx, inv, w)
	case cmdReply:
		return reply(ctx, inv, e)
	case cmdEditPR:
		return editPR(ctx, inv, e)
	default:
		return nil, 0, errUsage
	}
}

// prNumber returns the PR the command line names, or, when it names none,
// the current branch's PR as gh resolves it.
func prNumber(ctx context.Context, g *fixprguard.GitHub, arg string) (fixprguard.PRNumber, error) {
	if arg == "" {
		return g.CurrentPR(ctx)
	}
	return fixprguard.ParsePRNumber(arg)
}

// githubFor builds the GitHub of a subcommand that judges threads, from every
// --reviewer value. Each subcommand calls it before touching git or GitHub,
// so an empty, malformed, or repeated name stops the run there. The error
// names the flag and the rule, never a value.
func githubFor(inv invocation, e env) (*fixprguard.GitHub, error) {
	reviewers, err := fixprguard.ParseReviewers(inv.multi[flagReviewer])
	if err != nil {
		return nil, fmt.Errorf("--%s: %w", flagReviewer, err)
	}
	return fixprguard.NewGitHub(e.gh, reviewers)
}

func accepts(raw []string) ([]fixprguard.Accept, error) {
	out := make([]fixprguard.Accept, 0, len(raw))
	for _, s := range raw {
		a, err := fixprguard.ParseAccept(s)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, nil
}

// validatePR checks a PR argument's grammar before any command runs; an
// omitted one is resolved later through gh.
func validatePR(arg string) error {
	if arg == "" {
		return nil
	}
	_, err := fixprguard.ParsePRNumber(arg)
	return err
}

func head(ctx context.Context, inv invocation, e env) (any, int, error) {
	if err := validatePR(inv.arg); err != nil {
		return nil, 0, err
	}
	g, err := fixprguard.NewGitHubWithoutReviewer(e.gh)
	if err != nil {
		return nil, 0, err
	}
	pr, err := prNumber(ctx, g, inv.arg)
	if err != nil {
		return nil, 0, err
	}
	info, err := g.Head(ctx, pr)
	return info, exitContinue, err
}

func list(ctx context.Context, inv invocation, e env) (any, int, error) {
	if err := validatePR(inv.arg); err != nil {
		return nil, 0, err
	}
	g, err := githubFor(inv, e)
	if err != nil {
		return nil, 0, err
	}
	pr, err := prNumber(ctx, g, inv.arg)
	if err != nil {
		return nil, 0, err
	}
	threads, err := g.List(ctx, pr)
	return threads, exitContinue, err
}

// beginOutput is the output of begin. Threads is the fenced block of the
// threads to process, bodies included, as one line of JSON between the
// delimiter lines; every other member is the tool's own, with no body.
type beginOutput struct {
	Protocol          int                          `json:"protocol"`
	Baseline          fixprguard.Baseline          `json:"baseline"`
	Number            int                          `json:"number"`
	CapHit            bool                         `json:"capHit"`
	ProcessCount      int                          `json:"processCount"`
	Untrusted         []fixprguard.UntrustedThread `json:"untrusted"`
	Changed           []fixprguard.ChangedThread   `json:"changed"`
	AcceptNotFound    []string                     `json:"acceptNotFound"`
	PermissionLookups fixprguard.LookupBudget      `json:"permissionLookups"`
	Nonce             string                       `json:"nonce"`
	Threads           string                       `json:"threads"`
}

// begin runs in this order: the work tree must be clean (before anything
// reaches GitHub), the PR head must be in this repository, then the baseline
// is taken and the threads are judged and fetched. The threads to process are
// encoded as JSON, which escapes every newline and quote in a body, and then
// fenced, so no body can end the block.
func begin(ctx context.Context, inv invocation, e env, w *fixprguard.Worktree) (any, int, error) {
	if err := validatePR(inv.arg); err != nil {
		return nil, 0, err
	}
	g, err := githubFor(inv, e)
	if err != nil {
		return nil, 0, err
	}
	named, err := accepts(inv.multi[flagAccept])
	if err != nil {
		return nil, 0, err
	}
	if err := w.CheckClean(ctx); err != nil {
		return nil, 0, err
	}
	pr, err := prNumber(ctx, g, inv.arg)
	if err != nil {
		return nil, 0, err
	}
	if _, err := g.Head(ctx, pr); err != nil {
		return nil, 0, err
	}
	base, err := w.Baseline(ctx)
	if err != nil {
		return nil, 0, err
	}
	round, err := g.Begin(ctx, pr, named)
	if err != nil {
		return nil, 0, err
	}
	threads, err := json.Marshal(round.Threads)
	if err != nil {
		return nil, 0, err
	}
	block, err := fixprguard.Fence(threads, e.rand)
	if err != nil {
		return nil, 0, err
	}
	return beginOutput{
		Protocol:          fixprguard.Protocol,
		Baseline:          base,
		Number:            round.Number,
		CapHit:            round.CapHit,
		ProcessCount:      len(round.Threads),
		Untrusted:         round.Untrusted,
		Changed:           round.Changed,
		AcceptNotFound:    round.AcceptNotFound,
		PermissionLookups: round.PermissionLookups,
		Nonce:             block.Nonce,
		Threads:           block.Text,
	}, exitContinue, nil
}

// fence prints the file's content in a fenced block. Only a regular file
// (not a symbolic link, FIFO or device) of at most maxFenceInput bytes is
// read.
func fence(inv invocation, e env) (any, int, error) {
	f, err := fixprguard.OpenRegularFile(inv.arg)
	if err != nil {
		return nil, 0, fmt.Errorf("%w: fence file is not a readable regular file", fixprguard.ErrInvalidInput)
	}
	defer func() { _ = f.Close() }()
	data, err := io.ReadAll(io.LimitReader(f, maxFenceInput+1))
	if err != nil {
		return nil, 0, fmt.Errorf("%w: fence file not readable", fixprguard.ErrInvalidInput)
	}
	if len(data) > maxFenceInput {
		return nil, 0, fmt.Errorf("%w: fence file size", fixprguard.ErrInvalidInput)
	}
	block, err := fixprguard.Fence(data, e.rand)
	if err != nil {
		return nil, 0, err
	}
	return block.Text, exitContinue, nil
}

// inspect compares the git state with the baseline before gh runs (gh starts
// git, which would run with a rewritten configuration), then takes the
// threads' paths from GitHub, never from the caller, and judges the changes.
// Out-of-scope changes exit 3: the user is asked before finish.
func inspect(ctx context.Context, inv invocation, e env, w *fixprguard.Worktree) (any, int, error) {
	if err := validatePR(inv.arg); err != nil {
		return nil, 0, err
	}
	g, err := githubFor(inv, e)
	if err != nil {
		return nil, 0, err
	}
	base, err := fixprguard.ParseBaseline(inv.values[flagBaseline])
	if err != nil {
		return nil, 0, err
	}
	threads, err := accepts(inv.multi[flagThread])
	if err != nil {
		return nil, 0, err
	}
	if err := w.CheckState(ctx, base); err != nil {
		return nil, 0, err
	}
	pr, err := prNumber(ctx, g, inv.arg)
	if err != nil {
		return nil, 0, err
	}
	paths, err := g.ThreadPaths(ctx, pr, threads)
	if err != nil {
		return nil, 0, err
	}
	inspection, err := w.Inspect(ctx, base, paths)
	if err != nil {
		return nil, 0, err
	}
	if len(inspection.OutOfScope) > 0 {
		return inspection, exitAsk, nil
	}
	return inspection, exitContinue, nil
}

// finish takes the PR number on the command line: it reads nothing from
// GitHub, so it cannot resolve the current branch's PR.
func finish(ctx context.Context, inv invocation, w *fixprguard.Worktree) (any, int, error) {
	base, err := fixprguard.ParseBaseline(inv.values[flagBaseline])
	if err != nil {
		return nil, 0, err
	}
	judged, err := fixprguard.ParseJudgedSetID(inv.values[flagJudgedSet])
	if err != nil {
		return nil, 0, err
	}
	pr, err := fixprguard.ParsePRNumber(inv.arg)
	if err != nil {
		return nil, 0, err
	}
	result, err := w.Finish(ctx, base, judged, fixprguard.FinishRequest{PR: pr})
	return result, exitContinue, err
}

func push(ctx context.Context, inv invocation, w *fixprguard.Worktree) (any, int, error) {
	base, err := fixprguard.ParseBaseline(inv.values[flagBaseline])
	if err != nil {
		return nil, 0, err
	}
	result, err := w.Push(ctx, base, inv.values[flagCommit])
	return result, exitContinue, err
}

// writeOutput is the output of reply and edit-pr.
type writeOutput struct {
	Protocol int  `json:"protocol"`
	Written  bool `json:"written"`
}

func reply(ctx context.Context, inv invocation, e env) (any, int, error) {
	if err := validatePR(inv.arg); err != nil {
		return nil, 0, err
	}
	g, err := githubFor(inv, e)
	if err != nil {
		return nil, 0, err
	}
	commentID, err := fixprguard.ParseCommentID(inv.values[flagCommentID])
	if err != nil {
		return nil, 0, err
	}
	named, err := accepts(inv.multi[flagAccept])
	if err != nil {
		return nil, 0, err
	}
	pr, err := prNumber(ctx, g, inv.arg)
	if err != nil {
		return nil, 0, err
	}
	if err := g.Reply(ctx, fixprguard.ReplyRequest{
		PR:          pr,
		ThreadID:    inv.values[flagThreadID],
		CommentID:   commentID,
		Fingerprint: fixprguard.Fingerprint(inv.values[flagFingerprint]),
		BodyFile:    inv.values[flagBodyFile],
		Accept:      named,
	}); err != nil {
		return nil, 0, err
	}
	return writeOutput{Protocol: fixprguard.Protocol, Written: true}, exitContinue, nil
}

func editPR(ctx context.Context, inv invocation, e env) (any, int, error) {
	if err := validatePR(inv.arg); err != nil {
		return nil, 0, err
	}
	g, err := fixprguard.NewGitHubWithoutReviewer(e.gh)
	if err != nil {
		return nil, 0, err
	}
	pr, err := prNumber(ctx, g, inv.arg)
	if err != nil {
		return nil, 0, err
	}
	if err := g.EditPR(ctx, fixprguard.EditPRRequest{PR: pr, TitleFile: inv.values[flagTitleFile], BodyFile: inv.values[flagBodyFile]}); err != nil {
		return nil, 0, err
	}
	return writeOutput{Protocol: fixprguard.Protocol, Written: true}, exitContinue, nil
}
