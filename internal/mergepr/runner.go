package mergepr

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const (
	// maxCommandOutputBytes bounds what one command may return. The workflow
	// has no use for a larger answer, and a hostile repository could otherwise
	// exhaust memory.
	maxCommandOutputBytes = 1 << 20
	// maxCommandErrorBytes bounds the standard error kept for error messages.
	maxCommandErrorBytes = 4 << 10
	// commandWaitDelay bounds how long Run waits for inherited pipes after the
	// child exits or its context is done.
	commandWaitDelay = 5 * time.Second
)

// allowedEnvVars is the allowlist of environment variables passed to git and
// gh. Following docs/dev/security.md's child-process rule, it deliberately
// omits unrelated secrets in the developer's environment (DEEPSEEK_API_KEY,
// SLACK_WEBHOOK_URL, ...) while keeping what git, ssh, and gh need to find
// their configuration and authenticate. Command-valued variables (GIT_SSH,
// GIT_SSH_COMMAND, GIT_ASKPASS, SSH_ASKPASS) are omitted: each names a program
// git or ssh would execute, so inheriting one from the developer's environment
// lets it run inside this workflow. SSH_AUTH_SOCK and SSH_AGENT_PID stay,
// because an agent socket carries no command of its own. PATH is not here; it
// is rebuilt by trustedPath so a checkout entry cannot supply a child tool.
var allowedEnvVars = []string{
	"HOME", "TMPDIR",
	"SSH_AUTH_SOCK", "SSH_AGENT_PID",
	"GIT_TERMINAL_PROMPT",
	"GH_HOST", "GH_CONFIG_DIR",
	"XDG_CONFIG_HOME", "XDG_CACHE_HOME",
	"HTTP_PROXY", "HTTPS_PROXY", "NO_PROXY", "ALL_PROXY",
	"http_proxy", "https_proxy", "no_proxy", "all_proxy",
	"LANG", "LC_ALL", "LC_CTYPE",
	"SSL_CERT_FILE", "SSL_CERT_DIR",
	"TERM",
}

// NewOSRunner returns the production Runner.
func NewOSRunner() Runner { return osRunner{} }

type osRunner struct{}

var _ Runner = osRunner{}

// Run implements Runner. It runs the command without a shell, with an explicit
// allowlisted environment, and never includes more than the bounded standard
// error in an error.
func (osRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	// Resolve the tool against a PATH with the checkout removed, so exec cannot
	// pick up a program the PR added to the repository or the working directory.
	resolved, err := resolveCommand(name)
	if err != nil {
		return nil, err
	}
	cmd := exec.CommandContext(ctx, resolved, args...) //nolint:gosec // the tool is resolved to a trusted absolute path and no argument passes through a shell
	var stdout, stderr limitedBuffer
	stdout.limit = maxCommandOutputBytes
	stderr.limit = maxCommandErrorBytes
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Env = childEnv()
	cmd.WaitDelay = commandWaitDelay
	command := redactCredentials(name + " " + strings.Join(args, " "))
	if err := cmd.Run(); err != nil {
		message := redactCredentials(strings.TrimSpace(stderr.buffer.String()))
		if message != "" {
			return nil, fmt.Errorf("%s: %w: %s", command, err, message)
		}
		return nil, fmt.Errorf("%s: %w", command, err)
	}
	if stdout.truncated {
		return nil, fmt.Errorf("%w: %s output exceeds %d bytes", errTooLarge, name, maxCommandOutputBytes)
	}
	return stdout.buffer.Bytes(), nil
}

// userinfoPattern matches the userinfo of a URL between "://" and "@".
var userinfoPattern = regexp.MustCompile(`://[^/@\s]*@`)

// redactCredentials removes URL userinfo, which can carry a token, before text
// reaches an error that main prints.
func redactCredentials(text string) string {
	return userinfoPattern.ReplaceAllString(text, "://[redacted]@")
}

// limitedBuffer keeps at most limit bytes and records whether more arrived.
// Write reports every byte as written so the child is drained instead of
// blocking or dying from SIGPIPE.
type limitedBuffer struct {
	buffer    bytes.Buffer
	limit     int
	truncated bool
}

// Write implements io.Writer.
func (b *limitedBuffer) Write(p []byte) (int, error) {
	room := b.limit - b.buffer.Len()
	if room <= 0 {
		b.truncated = true
		return len(p), nil
	}
	if len(p) > room {
		b.truncated = true
		_, _ = b.buffer.Write(p[:room])
		return len(p), nil
	}
	_, _ = b.buffer.Write(p)
	return len(p), nil
}

// childEnv is the allowlisted environment plus git config overrides that
// disable global and system configuration (so a global url.*.insteadOf or
// includeIf cannot redirect a fetch or a push), disable repository hooks and
// fsmonitor, and reset any repository credential helper before installing gh's
// own helper (a fixed, trusted command) for HTTPS. GitHub tokens are passed to
// both git and gh because gh's helper runs as a child of git; repository hooks,
// fsmonitor, credential helpers, sshCommand, and external diff are all disabled
// or rejected, so git cannot hand the tokens to PR-controlled code.
func childEnv() []string {
	env := allowlistEnv(os.Environ())
	env = append(env,
		"PATH="+trustedPath(),
		"GIT_CONFIG_GLOBAL=/dev/null",
		"GIT_CONFIG_SYSTEM=/dev/null",
		"GIT_CONFIG_NOSYSTEM=1",
		"GIT_CONFIG_COUNT=7",
		"GIT_CONFIG_KEY_0=core.hooksPath",
		"GIT_CONFIG_VALUE_0=/dev/null",
		"GIT_CONFIG_KEY_1=core.fsmonitor",
		"GIT_CONFIG_VALUE_1=false",
		"GIT_CONFIG_KEY_2=credential.helper",
		"GIT_CONFIG_VALUE_2=",
		"GIT_CONFIG_KEY_3=credential.https://github.com.helper",
		"GIT_CONFIG_VALUE_3=!gh auth git-credential",
		// Disable automatic maintenance so a repository-local executable hook
		// such as gc.recentObjectsHook cannot run during a fetch.
		"GIT_CONFIG_KEY_4=maintenance.auto",
		"GIT_CONFIG_VALUE_4=false",
		// Force a stat-checking status, so repository-local config cannot make
		// git status omit an edit and let a dirty worktree look clean.
		"GIT_CONFIG_KEY_5=core.ignoreStat",
		"GIT_CONFIG_VALUE_5=false",
		"GIT_CONFIG_KEY_6=core.untrackedCache",
		"GIT_CONFIG_VALUE_6=false",
	)
	for _, key := range []string{"GH_TOKEN", "GITHUB_TOKEN"} {
		if value, ok := os.LookupEnv(key); ok {
			env = append(env, key+"="+value)
		}
	}
	return env
}

// allowlistEnv returns the allowlisted variables set in parent, in the order of
// allowedEnvVars. It is never nil, so the child gets an explicit environment
// instead of inheriting every secret the developer has loaded.
func allowlistEnv(parent []string) []string {
	values := make(map[string]string, len(parent))
	for _, entry := range parent {
		if name, value, ok := strings.Cut(entry, "="); ok {
			values[name] = value
		}
	}
	env := make([]string, 0, len(allowedEnvVars))
	for _, name := range allowedEnvVars {
		if value, ok := values[name]; ok {
			env = append(env, name+"="+value)
		}
	}
	return env
}

// trustedPath is PATH with entries that resolve inside this worktree, or that
// are relative or empty (which name the working directory), removed. A
// repository that adds an executable named git, gh, or ssh, or a direnv-managed
// $PWD/bin entry, cannot then supply the tool a child resolves.
func trustedPath() string {
	entries := filepath.SplitList(os.Getenv("PATH"))
	trusted := make([]string, 0, len(entries))
	var resolvedRoot string
	if root, err := worktreeRoot(); err == nil {
		if resolved, err := resolveExisting(root); err == nil {
			resolvedRoot = resolved
		}
	}
	for _, entry := range entries {
		if entry == "" || !filepath.IsAbs(entry) {
			continue
		}
		if resolvedRoot != "" {
			if resolved, err := resolveExisting(entry); err == nil {
				if rel, err := filepath.Rel(resolvedRoot, resolved); err == nil && inside(rel) {
					continue
				}
			}
		}
		trusted = append(trusted, entry)
	}
	return strings.Join(trusted, string(filepath.ListSeparator))
}

// resolveCommand returns the absolute path of a bare command name looked up in
// trustedPath, so a checkout entry cannot be resolved. A name that already
// contains a separator is returned unchanged.
func resolveCommand(name string) (string, error) {
	if strings.ContainsRune(name, filepath.Separator) {
		return name, nil
	}
	for _, dir := range filepath.SplitList(trustedPath()) {
		candidate := filepath.Join(dir, name)
		info, err := os.Stat(candidate)
		if err != nil || info.IsDir() || info.Mode().Perm()&0o111 == 0 {
			continue
		}
		return candidate, nil
	}
	return "", fmt.Errorf("%w: %s", errToolNotFound, name)
}

// inside reports whether a filepath.Rel result names the root or a path below
// it, as opposed to a sibling reached through "..".
func inside(rel string) bool {
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}
