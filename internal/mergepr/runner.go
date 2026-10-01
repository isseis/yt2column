package mergepr

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
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
// their configuration and authenticate.
var allowedEnvVars = []string{
	"PATH", "HOME", "TMPDIR",
	"SSH_AUTH_SOCK", "SSH_AGENT_PID",
	"GIT_SSH", "GIT_SSH_COMMAND", "GIT_ASKPASS", "SSH_ASKPASS",
	"GIT_CONFIG_GLOBAL", "GIT_CONFIG_SYSTEM", "GIT_CONFIG_NOSYSTEM", "GIT_TERMINAL_PROMPT",
	"GH_TOKEN", "GITHUB_TOKEN", "GH_HOST", "GH_CONFIG_DIR",
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
	cmd := exec.CommandContext(ctx, name, args...) //nolint:gosec // git and gh are trusted tools and no argument passes through a shell
	var stdout, stderr limitedBuffer
	stdout.limit = maxCommandOutputBytes
	stderr.limit = maxCommandErrorBytes
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Env = allowlistEnv(os.Environ())
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
