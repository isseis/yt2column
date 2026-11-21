package ciskip

import (
	"bytes"
	"context"
	"io"
	"os"
	"strings"
	"time"

	"github.com/isseis/yt2column/internal/cmdrunner"
)

const (
	originMainCommitRef       = "refs/remotes/origin/main^{commit}"
	hasExtensionChangesScript = "extension/scripts/has-extension-changes.sh"
	syntheticMergeTokens      = 3 // commit SHA + HEAD^1 + HEAD^2
	singleParentTokens        = 2 // commit SHA + single parent SHA
)

// runGit invokes git with cmdrunner.GitHardening prepended under timeout.
func runGit(ctx context.Context, runner cmdrunner.Runner, timeout time.Duration, args ...string) ([]byte, error) {
	cmdCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	full := append(cmdrunner.GitHardening(), args...)
	return runner.Run(cmdCtx, "git", full...)
}

// resolveSyntheticMergeBase verifies that HEAD is a two-parent synthetic merge commit
// whose second parent equals headSHA, and returns its first parent (HEAD^1).
// Invariant: NoticeBaseSHA is returned only after HEAD^2 == headSHA is proven.
func resolveSyntheticMergeBase(ctx context.Context, runner cmdrunner.Runner, timeout time.Duration, headSHA string) (string, Reason, bool) {
	out, err := runGit(ctx, runner, timeout, "rev-list", "--parents", "-n", "1", "HEAD")
	if err != nil {
		return "", ReasonGitFailed, false
	}
	tokens, ok := parseParentLine(out)
	if !ok {
		return "", ReasonGitFailed, false
	}
	if len(tokens) != syntheticMergeTokens {
		return "", ReasonMergeCommitMismatch, false
	}
	mergeBaseSHA, mergeHeadSHA := tokens[1], tokens[2]
	if mergeHeadSHA != headSHA {
		return "", ReasonMergeCommitMismatch, false
	}
	return mergeBaseSHA, ReasonUnspecified, true
}

// verifyOriginMain confirms that refs/remotes/origin/main^{commit} equals mergeBaseSHA.
// Invariant: rejects a checkout where main advanced after the synthetic merge commit was built.
func verifyOriginMain(ctx context.Context, runner cmdrunner.Runner, timeout time.Duration, mergeBaseSHA string) (Reason, bool) {
	out, err := runGit(ctx, runner, timeout, "rev-parse", "--verify", originMainCommitRef)
	if err != nil {
		return ReasonGitFailed, false
	}
	originMainSHA := strings.TrimSuffix(string(out), "\n")
	if !isHexSHA40(originMainSHA) {
		return ReasonGitFailed, false
	}
	if originMainSHA != mergeBaseSHA {
		return ReasonMainMovedAtCheckout, false
	}
	return ReasonUnspecified, true
}

// resolveSingleParent verifies that headSHA has exactly one parent and returns it.
// Invariant: root commits (0 parents) and merge commits (2+ parents) are rejected.
func resolveSingleParent(ctx context.Context, runner cmdrunner.Runner, timeout time.Duration, headSHA string) (string, Reason, bool) {
	out, err := runGit(ctx, runner, timeout, "rev-list", "--parents", "-n", "1", headSHA)
	if err != nil {
		return "", ReasonGitFailed, false
	}
	tokens, ok := parseParentLine(out)
	if !ok || tokens[0] != headSHA {
		return "", ReasonGitFailed, false
	}
	if len(tokens) != singleParentTokens {
		return "", ReasonNotSingleParent, false
	}
	return tokens[1], ReasonUnspecified, true
}

// parseParentLine parses a single line of space-separated 40-char lowercase hex SHAs
// produced by `git rev-list --parents -n 1`.
func parseParentLine(out []byte) ([]string, bool) {
	line := strings.TrimSuffix(string(out), "\n")
	if line == "" || strings.ContainsAny(line, "\r\n") {
		return nil, false
	}
	tokens := strings.Split(line, " ")
	for _, tok := range tokens {
		if !isHexSHA40(tok) {
			return nil, false
		}
	}
	return tokens, true
}

// validateDepDirsFile reads depDirsFile up to maxBytes and checks that it is
// non-empty, contains no NUL bytes, has at least one directory line ending in "/",
// and that every non-empty line ends in "/".
// Invariant: refuses empty or truncated dependency lists before calling has-extension-changes.sh,
// which itself returns false on an empty file.
func validateDepDirsFile(path string, maxBytes int) (Reason, bool) {
	if path == "" || maxBytes <= 0 {
		return ReasonWasmDepDirsFailed, false
	}
	f, err := os.Open(path) //nolint:gosec // depDirsFile path is supplied by the CI workflow's runner temp directory
	if err != nil {
		return ReasonWasmDepDirsFailed, false
	}
	defer func() { _ = f.Close() }()

	data, err := io.ReadAll(io.LimitReader(f, int64(maxBytes)+1))
	if err != nil || len(data) == 0 || len(data) > maxBytes {
		return ReasonWasmDepDirsFailed, false
	}
	if bytes.IndexByte(data, 0) >= 0 {
		return ReasonWasmDepDirsFailed, false
	}
	dirCount := 0
	for line := range strings.SplitSeq(string(data), "\n") {
		if line == "" {
			continue
		}
		if !strings.HasSuffix(line, "/") {
			return ReasonWasmDepDirsFailed, false
		}
		dirCount++
	}
	if dirCount == 0 {
		return ReasonWasmDepDirsFailed, false
	}
	return ReasonUnspecified, true
}

// parentDiff retrieves the NUL-delimited changed file list between parentSHA and headSHA
// with rename detection disabled and path quoting turned off.
// Invariant: empty output (0 bytes) is accepted as an empty commit; any non-empty output
// must terminate with NUL and contain no empty path entries.
func parentDiff(ctx context.Context, runner cmdrunner.Runner, timeout time.Duration, parentSHA, headSHA string) ([]byte, []string, Reason, bool) {
	raw, err := runGit(
		ctx, runner, timeout,
		"-c", "core.quotePath=false",
		"diff", "-z", "--no-renames", "--name-only",
		parentSHA, headSHA, "--",
	)
	if err != nil {
		return nil, nil, ReasonGitFailed, false
	}
	paths, ok := parseDiffZ(raw)
	if !ok {
		return nil, nil, ReasonGitFailed, false
	}
	return raw, paths, ReasonUnspecified, true
}

// parseDiffZ parses NUL-delimited paths from `git diff -z --name-only`.
func parseDiffZ(raw []byte) ([]string, bool) {
	if len(raw) == 0 {
		return nil, true
	}
	if raw[len(raw)-1] != 0 {
		return nil, false
	}
	parts := bytes.Split(raw[:len(raw)-1], []byte{0})
	paths := make([]string, 0, len(parts))
	for _, p := range parts {
		if len(p) == 0 {
			return nil, false
		}
		paths = append(paths, string(p))
	}
	return paths, true
}

// isNonCodePath reports whether path is excluded from Go code/lint checks under
// the same rule as .github/workflows/ci.yml: *.md, docs/*, .claude/*, or LICENSE.
// Invariant: empty paths and all other paths return false (treated as code).
func isNonCodePath(path string) bool {
	return strings.HasSuffix(path, ".md") ||
		strings.HasPrefix(path, "docs/") ||
		strings.HasPrefix(path, ".claude/") ||
		path == "LICENSE"
}

// checkExtensionChanges runs extension/scripts/has-extension-changes.sh with rawDiffZ
// on stdin and requires exact stdout "false\n" to pass.
// Invariant: any non-zero exit, unexpected stderr, timeout, or stdout other than "false\n"
// fails safe.
func checkExtensionChanges(ctx context.Context, runner cmdrunner.InputRunner, timeout time.Duration, depDirsFile string, rawDiffZ []byte) (Reason, bool) {
	cmdCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	out, err := runner.RunInput(cmdCtx, rawDiffZ, "bash", hasExtensionChangesScript, depDirsFile)
	if err != nil {
		return ReasonExtensionCheckFailed, false
	}
	switch string(out) {
	case "false\n":
		return ReasonUnspecified, true
	case "true\n":
		return ReasonExtensionChanged, false
	default:
		return ReasonExtensionCheckFailed, false
	}
}

// classifyParentDiff validates depDirsFile, inspects the parent-to-head diff,
// rejects any code file, and delegates extension classification for non-empty diffs.
// Invariant: depDirsFile validation always runs even for empty commits; empty commits
// skip invoking has-extension-changes.sh; any code file rejects before invoking it.
func classifyParentDiff(
	ctx context.Context,
	gitRunner cmdrunner.Runner,
	scriptRunner cmdrunner.InputRunner,
	timeout time.Duration,
	parentSHA, headSHA, depDirsFile string,
	maxDepDirsBytes int,
) (Reason, bool) {
	if reason, ok := validateDepDirsFile(depDirsFile, maxDepDirsBytes); !ok {
		return reason, false
	}
	raw, paths, reason, ok := parentDiff(ctx, gitRunner, timeout, parentSHA, headSHA)
	if !ok {
		return reason, false
	}
	if len(paths) == 0 {
		return ReasonUnspecified, true
	}
	for _, p := range paths {
		if !isNonCodePath(p) {
			return ReasonCodeChanged, false
		}
	}
	return checkExtensionChanges(ctx, scriptRunner, timeout, depDirsFile, raw)
}
