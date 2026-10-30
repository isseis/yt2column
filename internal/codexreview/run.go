package codexreview

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/isseis/yt2column/internal/cmdrunner"
	"github.com/isseis/yt2column/internal/strictjson"
)

// The time limits of a run. They are variables only so test_helpers.go can
// shorten them; no argument or environment variable reaches them. Together
// they stay under the 600 seconds the agent's Bash tool allows a command:
// at most gitTimeout before codex, codexTimeout plus stopGrace and pipeWait
// for codex, and gitTimeout after it.
var (
	// codexTimeout bounds the review.
	codexTimeout = 480 * time.Second
	// stopGrace is how long codex's process group has to exit after it is
	// asked to terminate, before it is killed.
	stopGrace = 5 * time.Second
	// versionTimeout bounds codex --version.
	versionTimeout = 10 * time.Second
	// gitTimeout bounds all the git reads before codex runs, and separately
	// all of them after.
	gitTimeout = 30 * time.Second
)

const (
	// pipeWait bounds the wait for an output pipe still held by a process that
	// left codex's process group after the group was killed.
	pipeWait = time.Second
	// maxStreamBytes caps each of codex's standard output and standard error,
	// and each git output.
	maxStreamBytes = 16 << 20
	// maxMessageBytes caps the final message.
	maxMessageBytes = 64 << 10
	// maxVersionBytes caps the output of codex --version.
	maxVersionBytes = 1 << 10
	// lastMessageName is the file codex writes its final message to, in the
	// run directory.
	lastMessageName = "last-message.txt"
	// codexCommand is the command looked up on PATH. The lookup, like every
	// command the tool starts, uses the process's own PATH; main passes that
	// same environment to Run, and codexEnv keeps PATH.
	codexCommand = "codex"
	// gitFatalStatus is the exit status of a git command that dies, as
	// rev-parse does outside a worktree.
	gitFatalStatus = 128
)

// Detail tokens. Each names a cause within a Kind and is fixed text, never
// taken from codex's output.
const (
	detailBadArguments        = "bad_arguments"
	detailProtocolMismatch    = "protocol_mismatch"
	detailNotWorktreeRoot     = "not_worktree_root"
	detailBaseNotAncestor     = "base_not_ancestor"
	detailDirtyTree           = "dirty_tree"
	detailProjectCodexConfig  = "project_codex_config"
	detailGitFailed           = "git_failed"
	detailVersionFailed       = "codex_version_failed"
	detailVersionTimeout      = "codex_version_timeout"
	detailVersionUnreadable   = "codex_version_unreadable"
	detailRunDirFailed        = "run_dir_failed"
	detailRunFailed           = "run_failed"
	detailPipeHeld            = "pipe_held"
	detailSignaled            = "signaled"
	detailOutput              = "output"
	detailFinalMessage        = "final_message"
	detailNoTurnCompleted     = "no_turn_completed"
	detailManyTurnsCompleted  = "multiple_turn_completed"
	detailErrorEvent          = "error_event"
	detailTurnFailed          = "turn_failed"
	detailUnreadableEvent     = "unreadable_event"
	detailMessageMissing      = "missing"
	detailMessageEmpty        = "empty"
	detailMessageNotRegular   = "not_regular_file"
	detailMessageGrammar      = "grammar"
	detailFindingOutsideRoot  = "path_outside_root"
	detailExitStatusPrefix    = "exit_status_"
	eventTypeTurnCompleted    = "turn.completed"
	eventTypeTurnFailed       = "turn.failed"
	eventTypeError            = "error"
	projectCodexConfigDirName = ".codex"
)

// detailExitStatus is how Kinds lists the exit_status_<n> Detail tokens.
const detailExitStatus = detailExitStatusPrefix + "<n>"

// Kinds returns every Kind a run ends in, each with the Detail tokens a
// result of that kind may carry; a kind listed with none carries an empty
// Detail. The exit_status_<n> tokens are listed once, as detailExitStatus.
// The shared procedure's table of kinds is checked against it, so a kind
// added here and not to the table fails that guard.
func Kinds() map[Kind][]string {
	return map[Kind][]string{
		KindFindings:     nil,
		KindNoFindings:   nil,
		KindStateChanged: nil,
		KindUsage:        {detailBadArguments, detailProtocolMismatch},
		KindPrecondition: {
			detailNotWorktreeRoot, detailBaseNotAncestor, detailDirtyTree, detailProjectCodexConfig,
			detailGitFailed, detailVersionFailed, detailVersionTimeout, detailVersionUnreadable,
			detailRunDirFailed,
		},
		KindCodexNotFound: nil,
		KindTimeout:       nil,
		KindCodexFailed:   {detailExitStatus, detailSignaled, detailPipeHeld, detailRunFailed},
		KindIncomplete: {
			detailNoTurnCompleted, detailManyTurnsCompleted, detailErrorEvent, detailTurnFailed,
			detailUnreadableEvent,
		},
		KindOutputTooLarge: {detailOutput, detailFinalMessage},
		KindOutputEmpty:    {detailMessageMissing, detailMessageEmpty},
		KindUnreadable:     {detailMessageNotRegular, detailMessageGrammar, detailFindingOutsideRoot},
	}
}

// codexArgs returns the fixed arguments of the review. The base is the only
// value that varies; lastMessage is the file in the run directory.
//
// The user's configuration file, execpolicy rules, and the features that
// reach outside the shell sandbox (plugins, connectors, hooks, browser and
// computer use) are turned off; the sandbox is read-only and nothing asks for
// approval.
func codexArgs(base, lastMessage string) []string {
	args := []string{"exec", "review", "--base", base, "--ignore-user-config", "--ignore-rules"}
	for _, feature := range disabledFeatures {
		args = append(args, "--disable", feature)
	}
	return append(args,
		"-c", `sandbox_mode="read-only"`, "-c", `approval_policy="never"`,
		"-c", `shell_environment_policy.inherit="core"`,
		"--json", "-o", lastMessage)
}

// codexEnvNames are the only variables codex receives from the tool's
// environment: what it needs to find its programs and its login, to write
// temporary files, to read text, and to reach OpenAI through a proxy.
var codexEnvNames = []string{
	"PATH", "HOME", "USER", "LOGNAME", "SHELL", "TMPDIR",
	"LANG", "LC_ALL", "LC_CTYPE",
	"CODEX_HOME", "XDG_CONFIG_HOME", "XDG_CACHE_HOME", "XDG_DATA_HOME", "XDG_STATE_HOME",
	"HTTPS_PROXY", "https_proxy", "HTTP_PROXY", "http_proxy", "ALL_PROXY", "all_proxy", "NO_PROXY", "no_proxy",
	"SSL_CERT_FILE", "SSL_CERT_DIR",
}

// codexEnv returns the environment codex runs with.
//
// Contract: only the variables named in codexEnvNames pass, so a secret the
// caller's environment holds (an API key, a Webhook URL) is not in the
// environment of codex or of any command it runs, where printing the
// environment would send it to OpenAI. Optional git locks and terminal
// prompts are off, as for the tool's own git.
func codexEnv(environ []string) []string {
	added := []string{"GIT_OPTIONAL_LOCKS=0", "GIT_TERMINAL_PROMPT=0"}
	env := make([]string, 0, len(codexEnvNames)+len(added))
	for _, entry := range environ {
		name, _, _ := strings.Cut(entry, "=")
		if slices.Contains(codexEnvNames, name) {
			env = append(env, entry)
		}
	}
	return append(env, added...)
}

// disabledFeatures are the codex features turned off for the review.
var disabledFeatures = []string{
	"apps", "plugins", "remote_plugin", "hooks", "browser_use", "computer_use",
	"image_generation", "multi_agent", "tool_suggest", "skill_mcp_dependency_install",
}

var (
	// baseSHA is a full SHA-1 commit ID in lowercase hex.
	baseSHA = regexp.MustCompile(`^[0-9a-f]{40}$`)
	// versionLine is one line of printable ASCII.
	versionLine = regexp.MustCompile(`^[ -~]{1,200}$`)
)

var (
	// errInterrupted reports that ctx was canceled (a signal reached the
	// tool) and the state codex could have changed did not change.
	errInterrupted = errors.New("interrupted")
	// errAfterStateUnreadable reports that the git state after codex ran
	// could not be read, so whether codex changed it is unknown.
	errAfterStateUnreadable = errors.New("the git state after the review cannot be read")
)

// ValidBase reports whether base has the one form Run accepts: a full SHA-1
// commit ID in lowercase hex. The command documents' calls are checked with it.
func ValidBase(base string) bool { return baseSHA.MatchString(base) }

// Run runs one review of base..HEAD in the current directory, which must be a
// worktree root, and returns its result. environ is the tool's environment:
// git runs with it minus every GIT_* variable (cmdrunner.GitEnv), and codex
// with only the variables codexEnv keeps.
//
// Contract: codex runs at most once, and only after every precondition holds;
// a run that returns before it never consumes the Codex allowance. Once codex
// has started, the git state is compared after it stops, under a context the
// caller cannot cancel, and a difference is reported as KindStateChanged
// ahead of every other result. When that state cannot be read, or ctx was
// canceled and nothing changed, Run returns an error and no result: whether
// the worktree changed is unknown, or nobody is waiting for the review. Run
// writes only the run directory it creates; it never changes the worktree, a
// ref, or the stash.
func Run(ctx context.Context, base string, environ []string) (Result, error) {
	if !ValidBase(base) {
		return newResult(KindUsage, detailBadArguments), nil
	}
	preCtx, cancelPre := context.WithTimeout(context.WithoutCancel(ctx), gitTimeout)
	defer cancelPre()
	p, stop := prepare(ctx, preCtx, base, environ)
	if ctx.Err() != nil {
		// Nothing has run that could change the state, and nobody waits for
		// the result.
		if stop == nil {
			_ = os.Remove(p.runDir)
		}
		return Result{}, errInterrupted
	}
	if stop != nil {
		return *stop, nil
	}
	return p.review(ctx)
}

// prepared is what the preconditions established.
type prepared struct {
	base     string
	codexEnv []string
	root     string
	git      git
	version  string
	changed  map[string]struct{}
	before   gitState
	runDir   string
}

// prepare checks the preconditions and records the state before the review.
// The git reads run under preCtx, which ctx does not cancel; codex --version
// runs under ctx. A non-nil result means codex must not run.
func prepare(ctx, preCtx context.Context, base string, environ []string) (prepared, *Result) {
	stop := func(kind Kind, detail string) (prepared, *Result) {
		r := newResult(kind, detail)
		return prepared{}, &r
	}
	cwd, err := os.Getwd()
	if err != nil {
		return stop(KindPrecondition, detailNotWorktreeRoot)
	}
	g := newGit(cwd, cmdrunner.GitEnv(environ))
	cEnv := codexEnv(environ)
	root, err := worktreeRoot(preCtx, g, cwd)
	if exitErr, ok := errors.AsType[*cmdrunner.ExitError](err); (ok && exitErr.Code == gitFatalStatus && preCtx.Err() == nil) || (err == nil && root == "") {
		// git rev-parse exits with its fatal status outside a worktree.
		return stop(KindPrecondition, detailNotWorktreeRoot)
	}
	if err != nil {
		return stop(KindPrecondition, detailGitFailed)
	}
	if detail := checkBase(preCtx, g, base); detail != "" {
		return stop(KindPrecondition, detail)
	}
	status, err := g.run(preCtx, statusArgs...)
	if err != nil {
		return stop(KindPrecondition, detailGitFailed)
	}
	if len(status) > 0 {
		return stop(KindPrecondition, detailDirtyTree)
	}
	// Whether codex reads a project's .codex directory, and whether
	// --ignore-user-config covers it, is not known, so a branch that adds
	// one must not reach codex. Any answer other than "absent" stops.
	if _, err := os.Lstat(filepath.Join(root, projectCodexConfigDirName)); !errors.Is(err, fs.ErrNotExist) {
		return stop(KindPrecondition, detailProjectCodexConfig)
	}
	if _, err := exec.LookPath(codexCommand); err != nil {
		return stop(KindCodexNotFound, "")
	}
	version, detail := codexVersion(ctx, root, cEnv)
	if detail != "" {
		return stop(KindPrecondition, detail)
	}
	changed, err := readChangedFiles(preCtx, g, base)
	if err != nil {
		return stop(KindPrecondition, detailGitFailed)
	}
	// The state codex is compared against is read last, after codex
	// --version, and it must itself be clean: a change made after the first
	// check would otherwise be part of the baseline, reviewed and sent, and
	// never found by the comparison.
	before, err := readState(preCtx, g)
	if err != nil {
		return stop(KindPrecondition, detailGitFailed)
	}
	if before.status != "" {
		return stop(KindPrecondition, detailDirtyTree)
	}
	runDir, err := os.MkdirTemp("", "codexreview-")
	if err != nil {
		return stop(KindPrecondition, detailRunDirFailed)
	}
	return prepared{base: base, codexEnv: cEnv, root: root, git: g, version: version, changed: changed, before: before, runDir: runDir}, nil
}

// newGit returns the git runner the tool reads the repository with.
func newGit(dir string, env []string) git {
	return git{runner: cmdrunner.NewOS(cmdrunner.Options{
		MaxOutput: maxStreamBytes,
		WaitDelay: pipeWait,
		Dir:       dir,
		Env:       env,
	})}
}

// worktreeRoot returns the worktree root with its symbolic links resolved,
// or "" when cwd is not that root.
func worktreeRoot(ctx context.Context, g git, cwd string) (string, error) {
	out, err := g.run(ctx, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", err
	}
	root, err := filepath.EvalSymlinks(strings.TrimSuffix(string(out), "\n"))
	if err != nil {
		return "", err
	}
	here, err := filepath.EvalSymlinks(cwd)
	if err != nil {
		return "", err
	}
	if here != root {
		return "", nil
	}
	return root, nil
}

// checkBase returns "" when base is a commit that HEAD descends from, and the
// Detail token of the failed precondition otherwise. codex --base reviews the
// difference from the merge base of BASE and HEAD, which is BASE only then.
func checkBase(ctx context.Context, g git, base string) string {
	if _, err := g.run(ctx, "rev-parse", "--verify", "--quiet", base+"^{commit}"); err != nil {
		if exitErr, ok := errors.AsType[*cmdrunner.ExitError](err); ok && exitErr.Code == 1 {
			return detailBaseNotAncestor
		}
		return detailGitFailed
	}
	if _, err := g.run(ctx, "merge-base", "--is-ancestor", base, "HEAD"); err != nil {
		if exitErr, ok := errors.AsType[*cmdrunner.ExitError](err); ok && exitErr.Code == 1 {
			return detailBaseNotAncestor
		}
		return detailGitFailed
	}
	return ""
}

// codexVersion runs codex --version and returns its one line, or the Detail
// token of the failure.
func codexVersion(ctx context.Context, root string, env []string) (string, string) {
	vctx, cancel := context.WithTimeout(ctx, versionTimeout)
	defer cancel()
	out, err := codexRunner(root, env, maxVersionBytes).Run(vctx, codexCommand, "--version")
	switch {
	case errors.Is(vctx.Err(), context.DeadlineExceeded):
		return "", detailVersionTimeout
	case err != nil:
		return "", detailVersionFailed
	}
	line := strings.TrimSuffix(string(out), "\n")
	if !versionLine.MatchString(line) {
		return "", detailVersionUnreadable
	}
	return line, ""
}

// codexRunner runs codex in its own process group, so stopping it also stops
// the shell commands it is running.
func codexRunner(root string, env []string, maxOutput int) cmdrunner.Runner {
	return cmdrunner.NewOS(cmdrunner.Options{
		MaxOutput:      maxOutput,
		WaitDelay:      pipeWait,
		Dir:            root,
		Env:            env,
		ProcessGroup:   true,
		GroupStopGrace: stopGrace,
	})
}

// review runs codex and judges the run. The order of the judgments is the
// order of precedence: a state change, an interruption, the tool's own
// timeout, then the run's failures, and only then the final message.
func (p prepared) review(ctx context.Context) (Result, error) {
	runCtx, cancelRun := context.WithTimeout(ctx, codexTimeout)
	defer cancelRun()
	stdout, runErr := codexRunner(p.root, p.codexEnv, maxStreamBytes).Run(runCtx, codexCommand, codexArgs(p.base, filepath.Join(p.runDir, lastMessageName))...)
	// Judged now, by the tool's own clock: the limit keeps running while the
	// state is read below, and a run that finished in time must not turn
	// into a timeout then.
	timedOut := errors.Is(runCtx.Err(), context.DeadlineExceeded)
	cancelRun()

	afterCtx, cancelAfter := context.WithTimeout(context.WithoutCancel(ctx), gitTimeout)
	defer cancelAfter()
	after, err := readState(afterCtx, p.git)
	if err != nil {
		return Result{}, fmt.Errorf("%w: %w", errAfterStateUnreadable, err)
	}
	if items, refs := changedItems(p.before, after); len(items) > 0 {
		r := p.result(KindStateChanged, "")
		r.Changed = items
		r.ChangedRefs = describeRefs(afterCtx, p.git, refs)
		return r, nil
	}
	if ctx.Err() != nil {
		return Result{}, errInterrupted
	}
	if timedOut {
		return p.result(KindTimeout, ""), nil
	}
	if runErr != nil {
		return p.result(runFailure(runErr)), nil
	}
	if detail := checkEvents(stdout); detail != "" {
		return p.result(KindIncomplete, detail), nil
	}
	return p.judgeMessage(), nil
}

// runFailure maps an error of a codex run that was neither stopped by the
// tool nor interrupted to its Kind and Detail.
func runFailure(err error) (Kind, string) {
	if errors.Is(err, cmdrunner.ErrOutputTooLarge) {
		return KindOutputTooLarge, detailOutput
	}
	if exitErr, ok := errors.AsType[*cmdrunner.ExitError](err); ok {
		if exitErr.Code < 0 {
			return KindCodexFailed, detailSignaled
		}
		return KindCodexFailed, detailExitStatusPrefix + strconv.Itoa(exitErr.Code)
	}
	if errors.Is(err, exec.ErrWaitDelay) {
		// codex exited, but a process that left its group kept the output open.
		return KindCodexFailed, detailPipeHeld
	}
	return KindCodexFailed, detailRunFailed
}

// checkEvents returns "" when the JSONL output declares exactly one completed
// turn and no error or failed turn, and the Detail token of the first broken
// rule otherwise. Lines are told apart only by their declared type member; a
// line that is not one strict JSON object with a string type breaks the
// output.
func checkEvents(stdout []byte) string {
	completed := 0
	var sawError, sawFailed bool
	for line := range strings.SplitSeq(string(stdout), "\n") {
		if line == "" {
			continue
		}
		eventType, ok := readEventType([]byte(line))
		if !ok {
			return detailUnreadableEvent
		}
		switch eventType {
		case eventTypeTurnCompleted:
			completed++
		case eventTypeError:
			sawError = true
		case eventTypeTurnFailed:
			sawFailed = true
		}
	}
	switch {
	case sawError:
		return detailErrorEvent
	case sawFailed:
		return detailTurnFailed
	case completed == 0:
		return detailNoTurnCompleted
	case completed > 1:
		return detailManyTurnsCompleted
	}
	return ""
}

// readEventType returns the type member of one JSONL line.
func readEventType(line []byte) (string, bool) {
	obj, err := strictjson.ParseObject(line)
	if err != nil {
		return "", false
	}
	members, err := obj.Collect("type")
	if err != nil {
		return "", false
	}
	eventType, err := strictjson.RequiredString(members, "type")
	if err != nil {
		return "", false
	}
	return eventType, true
}

// judgeMessage reads the final message of a completed run and matches it
// against the accepted forms.
func (p prepared) judgeMessage() Result {
	path := filepath.Join(p.runDir, lastMessageName)
	info, err := os.Lstat(path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return p.result(KindOutputEmpty, detailMessageMissing)
	case err != nil || !info.Mode().IsRegular():
		return p.result(KindUnreadable, detailMessageNotRegular)
	case info.Size() > maxMessageBytes:
		return p.result(KindOutputTooLarge, detailFinalMessage)
	}
	data, err := os.ReadFile(path) //nolint:gosec // the file is in the run directory this tool created
	if err != nil {
		return p.result(KindUnreadable, detailMessageNotRegular)
	}
	if isBlank(string(data)) {
		return p.result(KindOutputEmpty, detailMessageEmpty)
	}
	form, raw := parseFinalMessage(string(data))
	switch form {
	case formNoFindings:
		return p.result(KindNoFindings, "")
	case formFindings:
		findings, ok := locateFindings(p.root, p.changed, raw)
		if !ok {
			return p.result(KindUnreadable, detailFindingOutsideRoot)
		}
		r := p.result(KindFindings, "")
		r.Findings = findings
		return r
	default:
		return p.result(KindUnreadable, detailMessageGrammar)
	}
}

// result returns a Result of a run that reached codex.
func (p prepared) result(kind Kind, detail string) Result {
	r := newResult(kind, detail)
	r.CodexVersion = p.version
	r.RunDir = p.runDir
	r.InstructionsChanged = touchesInstructions(p.changed)
	return r
}

// BadArgumentsResult returns the result of a command line codexreview
// refuses for its shape: an unknown or repeated flag, a missing flag, or a
// positional argument.
func BadArgumentsResult() Result { return newResult(KindUsage, detailBadArguments) }

// ProtocolMismatchResult returns the result of a command line whose
// --protocol is not Protocol.
func ProtocolMismatchResult() Result { return newResult(KindUsage, detailProtocolMismatch) }

// newResult returns a Result with empty, not null, lists.
func newResult(kind Kind, detail string) Result {
	return Result{
		Protocol:    Protocol,
		Kind:        kind,
		Detail:      detail,
		Changed:     []string{},
		ChangedRefs: []string{},
		Findings:    []Finding{},
	}
}
