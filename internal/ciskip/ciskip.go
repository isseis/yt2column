// Package ciskip evaluates whether CI's heavy jobs may be skipped for a
// documentation-only pull request commit whose parent commit already passed CI
// against the same main commit.
//
// External-content inspection boundary ("Declare, don't infer" exception):
// this package inspects the content of external inputs that carry no typed
// envelope: GitHub Actions context strings (repository names, base ref, head
// SHA, PR number, run attempt, wasm-dep-dirs flag), git command output
// (rev-list parent lines, rev-parse commit OIDs, NUL-delimited diff paths),
// wasm-dep-dirs file lines, non-code file paths, has-extension-changes.sh
// output, and (for parent-CI verification) gh api -i HTTP status lines, Link
// headers, and ci-base-sha check-run annotation values. Every unmatched,
// malformed, or oversized input is rejected without normalization and maps to
// a non-skip Reason (fail-safe).
package ciskip

import (
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/isseis/yt2column/internal/cmdrunner"
)

// Reason is the declared verdict reason for a skip evaluation.
// Its zero value (ReasonUnspecified) means "do not skip" (fail-safe).
type Reason string

// Declared Reason values produced by CI skip evaluation.
const (
	ReasonUnspecified          Reason = ""
	ReasonSkipped              Reason = "ok"
	ReasonInvalidInput         Reason = "invalid_input"
	ReasonForkOrRepoMismatch   Reason = "fork_or_repo_mismatch"
	ReasonNonMainBase          Reason = "non_main_base"
	ReasonManualRerun          Reason = "manual_rerun"
	ReasonWasmDepDirsFailed    Reason = "wasm_dep_dirs_failed"
	ReasonMergeCommitMismatch  Reason = "merge_commit_mismatch"
	ReasonMainMovedAtCheckout  Reason = "main_moved_at_checkout"
	ReasonNotSingleParent      Reason = "not_single_parent"
	ReasonCodeChanged          Reason = "code_changed"
	ReasonExtensionChanged     Reason = "extension_changed"
	ReasonGitFailed            Reason = "git_failed"
	ReasonExtensionCheckFailed Reason = "extension_check_failed"
	ReasonAPIFailed            Reason = "api_failed"
	ReasonAPITimeout           Reason = "api_timeout"
	ReasonAPIOutputTooLarge    Reason = "api_output_too_large"
	ReasonAPIInvalidResponse   Reason = "api_invalid_response"
	ReasonAPIPaginationLimit   Reason = "api_pagination_limit"
	ReasonNoParentRun          Reason = "no_parent_run"
	ReasonAmbiguousPRLink      Reason = "ambiguous_pr_link"
	ReasonParentRunIncomplete  Reason = "parent_run_incomplete"
	ReasonParentRunBadResult   Reason = "parent_run_bad_result"
	ReasonParentJobsInvalid    Reason = "parent_jobs_invalid"
	ReasonParentBaseMissing    Reason = "parent_base_missing"
	ReasonParentBaseDuplicate  Reason = "parent_base_duplicate"
	ReasonMainAdvanced         Reason = "main_advanced"
)

// Input holds the raw GitHub Actions context values and local paths passed to Evaluate.
type Input struct {
	Repo          string
	HeadRepo      string
	BaseRepo      string
	BaseRef       string
	PRNumber      string
	HeadSHA       string
	RunAttempt    string
	DepDirsFile   string
	WasmDepDirsOK string
}

// Decision is the outcome of one skip evaluation.
// Whether heavy jobs may be skipped is derived solely from Reason via Skip(),
// so an inconsistent (skip=true, non-ok reason) state is unrepresentable.
// NoticeBaseSHA is non-empty only when the synthetic merge commit's first parent (HEAD^1)
// was resolved and HEAD^2 matched the validated HeadSHA on a same-repo main PR.
// HTTPStatus is non-zero only when a gh api call returned a parseable non-200 HTTP status code.
type Decision struct {
	Reason        Reason
	NoticeBaseSHA string
	ParentSHA     string
	ParentRunID   int64
	HTTPStatus    int
}

// Skip reports whether every precondition and verification succeeded (Reason == ReasonSkipped).
// Invariant: only ReasonSkipped yields true; the zero value and any unknown Reason fail safe.
func (d Decision) Skip() bool {
	return d.Reason == ReasonSkipped
}

// evalLimits holds the operational bounds enforced on external commands and API paging.
type evalLimits struct {
	gitMaxOutput       int
	scriptMaxOutput    int
	ghMaxOutput        int
	commandTimeout     time.Duration
	runTimeout         time.Duration
	waitDelay          time.Duration
	groupStopGrace     time.Duration
	maxRunPages        int
	maxJobPages        int
	maxAnnotationPages int
}

const (
	defaultGitMaxOutput       = 16 << 20
	defaultScriptMaxOutput    = 64 << 10
	defaultGHMaxOutput        = 4 << 20
	defaultCommandTimeout     = 15 * time.Second
	defaultRunTimeout         = 45 * time.Second
	defaultWaitDelay          = 5 * time.Second
	defaultGroupStopGrace     = 5 * time.Second
	defaultMaxRunPages        = 3
	defaultMaxJobPages        = 2
	defaultMaxAnnotationPages = 3

	maxRepoLen       = 200
	maxBaseRefLen    = 100
	maxDecimalDigits = 10
	hexSHALen        = 40
	mainBaseRef      = "main"
)

// defaultLimits returns the production operational bounds.
func defaultLimits() evalLimits {
	return evalLimits{
		gitMaxOutput:       defaultGitMaxOutput,
		scriptMaxOutput:    defaultScriptMaxOutput,
		ghMaxOutput:        defaultGHMaxOutput,
		commandTimeout:     defaultCommandTimeout,
		runTimeout:         defaultRunTimeout,
		waitDelay:          defaultWaitDelay,
		groupStopGrace:     defaultGroupStopGrace,
		maxRunPages:        defaultMaxRunPages,
		maxJobPages:        defaultMaxJobPages,
		maxAnnotationPages: defaultMaxAnnotationPages,
	}
}

var (
	ghRemovedVars = []string{
		"CLICOLOR_FORCE",
		"GH_FORCE_TTY",
		"NO_COLOR",
		"GH_PROMPT_DISABLED",
		"GH_REPO",
		"GH_HOST",
		"GH_DEBUG",
		"GH_PAGER",
		"PAGER",
	}
	ghAddedVars = []string{
		"NO_COLOR=1",
		"GH_PROMPT_DISABLED=1",
	}
)

// runnerOptions builds the isolated, bounded cmdrunner.Options for git,
// has-extension-changes.sh, and gh from environ and limits.
// Invariant: git and script never receive GH_TOKEN, GITHUB_TOKEN, GIT_*, or CISKIP_*;
// gh never receives GH_HOST, GH_REPO, GH_DEBUG, GH_PAGER, PAGER, color/prompt overrides,
// GIT_*, or CISKIP_*, and always has NO_COLOR=1 and GH_PROMPT_DISABLED=1.
func runnerOptions(environ []string, limits evalLimits) (gitOpts, scriptOpts, ghOpts cmdrunner.Options) {
	localEnv := sanitizeGitAndScriptEnv(environ)
	gitOpts = cmdrunner.Options{
		MaxOutput:      limits.gitMaxOutput,
		WaitDelay:      limits.waitDelay,
		Env:            localEnv,
		ProcessGroup:   true,
		GroupStopGrace: limits.groupStopGrace,
	}
	// ProcessGroup stays false for script because stdin may exceed the pipe buffer
	// while has-extension-changes.sh is pure bash builtins with no child processes.
	scriptOpts = cmdrunner.Options{
		MaxOutput:       limits.scriptMaxOutput,
		WaitDelay:       limits.waitDelay,
		Env:             slices.Clone(localEnv),
		StderrIsFailure: true,
		ProcessGroup:    false,
	}
	ghOpts = cmdrunner.Options{
		MaxOutput:      limits.ghMaxOutput,
		WaitDelay:      limits.waitDelay,
		Env:            sanitizeGHEnv(environ),
		ProcessGroup:   true,
		GroupStopGrace: limits.groupStopGrace,
	}
	return gitOpts, scriptOpts, ghOpts
}

// sanitizeGitAndScriptEnv strips GIT_*, GH_TOKEN, GITHUB_TOKEN, and CISKIP_*
// and applies cmdrunner.GitEnv hardening.
func sanitizeGitAndScriptEnv(environ []string) []string {
	filtered := make([]string, 0, len(environ))
	for _, entry := range environ {
		key, _, _ := strings.Cut(entry, "=")
		if key == "GH_TOKEN" || key == "GITHUB_TOKEN" || strings.HasPrefix(key, "CISKIP_") {
			continue
		}
		filtered = append(filtered, entry)
	}
	return cmdrunner.GitEnv(filtered)
}

// sanitizeGHEnv strips host/repo/color/prompt/debug/pager, GIT_*, and CISKIP_*
// variables and pins NO_COLOR=1 and GH_PROMPT_DISABLED=1.
func sanitizeGHEnv(environ []string) []string {
	env := make([]string, 0, len(environ)+len(ghAddedVars))
	for _, entry := range environ {
		key, _, _ := strings.Cut(entry, "=")
		if slices.Contains(ghRemovedVars, key) ||
			strings.HasPrefix(key, "GIT_") ||
			strings.HasPrefix(key, "CISKIP_") {
			continue
		}
		env = append(env, entry)
	}
	return append(env, ghAddedVars...)
}

// validateBaseContext checks the core PR identity preconditions that must hold
// before resolving HEAD^1 / HEAD^2 and emitting a ci-base-sha notice.
// Invariant: rejects fork PRs, malformed repository names, non-main bases, and
// malformed head SHAs without trimming or normalizing any value.
func validateBaseContext(in Input) (Reason, bool) {
	if !isValidRepo(in.Repo) || !isValidRepo(in.BaseRepo) || !isValidRepo(in.HeadRepo) {
		return ReasonForkOrRepoMismatch, false
	}
	if in.Repo != in.BaseRepo || in.BaseRepo != in.HeadRepo {
		return ReasonForkOrRepoMismatch, false
	}
	if len(in.BaseRef) > maxBaseRefLen {
		return ReasonInvalidInput, false
	}
	if in.BaseRef != mainBaseRef {
		return ReasonNonMainBase, false
	}
	if !isHexSHA40(in.HeadSHA) {
		return ReasonInvalidInput, false
	}
	return ReasonUnspecified, true
}

// validateRemainingContext checks the remaining PR context inputs after the
// synthetic merge commit check.
// Invariant: PRNumber and RunAttempt must be strict positive decimal integers;
// RunAttempt > 1 maps to ReasonManualRerun; WasmDepDirsOK must be exact "true".
func validateRemainingContext(in Input) (int64, Reason, bool) {
	prNumber, ok := parsePositiveDecimal(in.PRNumber, maxDecimalDigits)
	if !ok {
		return 0, ReasonInvalidInput, false
	}
	attempt, ok := parsePositiveDecimal(in.RunAttempt, maxDecimalDigits)
	if !ok {
		return 0, ReasonInvalidInput, false
	}
	if attempt > 1 {
		return 0, ReasonManualRerun, false
	}
	switch in.WasmDepDirsOK {
	case "true":
	case "false":
		return 0, ReasonWasmDepDirsFailed, false
	default:
		return 0, ReasonInvalidInput, false
	}
	if in.DepDirsFile == "" {
		return 0, ReasonWasmDepDirsFailed, false
	}
	return prNumber, ReasonUnspecified, true
}

// isValidRepo reports whether s is an ASCII owner/repo identifier within maxRepoLen,
// with non-empty segments distinct from "." and "..".
func isValidRepo(s string) bool {
	if len(s) == 0 || len(s) > maxRepoLen {
		return false
	}
	owner, name, ok := strings.Cut(s, "/")
	if !ok {
		return false
	}
	return isValidRepoSegment(owner) && isValidRepoSegment(name)
}

func isValidRepoSegment(seg string) bool {
	if seg == "" || seg == "." || seg == ".." {
		return false
	}
	for i := range len(seg) {
		c := seg[i]
		isAlphaNum := (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
		if !isAlphaNum && c != '.' && c != '_' && c != '-' {
			return false
		}
	}
	return true
}

// isHexSHA40 reports whether s is exactly 40 lowercase ASCII hex digits.
func isHexSHA40(s string) bool {
	if len(s) != hexSHALen {
		return false
	}
	for i := range len(s) {
		c := s[i]
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

// parsePositiveDecimal parses a strict base-10 positive integer with no sign,
// no whitespace, no leading zeros, and at most maxDigits digits.
func parsePositiveDecimal(s string, maxDigits int) (int64, bool) {
	if len(s) == 0 || len(s) > maxDigits || s[0] < '1' || s[0] > '9' {
		return 0, false
	}
	for i := 1; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return 0, false
		}
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil || n <= 0 {
		return 0, false
	}
	return n, true
}
