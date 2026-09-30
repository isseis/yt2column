package transcript

import (
	"context"
	"io"
	"os/exec"
	"strings"
	"time"
)

const (
	// maxStderrBytes is the prefix of the child's standard error output kept
	// for error messages. Everything beyond it is read and discarded.
	maxStderrBytes = 4 << 10
	// execWaitDelay is the grace period Run allows for a child's I/O pipes to
	// close after the child exits or its context is done. Without it, a
	// descendant that inherited the pipes could block the caller well past the
	// timeout.
	execWaitDelay = 5 * time.Second
)

// allowedEnvVars is the fixed set of environment variables passed to yt-dlp.
// It is an allowlist so a secret added to the parent environment cannot leak
// into the child; it must never list an API key or the Webhook URL.
var allowedEnvVars = []string{
	"PATH",
	"HOME",
	"TMPDIR",
	"XDG_CONFIG_HOME",
	"XDG_CACHE_HOME",
	"HTTP_PROXY",
	"HTTPS_PROXY",
	"NO_PROXY",
	"ALL_PROXY",
	"http_proxy",
	"https_proxy",
	"no_proxy",
	"all_proxy",
	"LANG",
	"LC_ALL",
	"LC_CTYPE",
	"SSL_CERT_FILE",
	"SSL_CERT_DIR",
}

// commandExecutor runs an external command. Tests replace it to observe the
// arguments, the environment, and the standard error output.
type commandExecutor interface {
	// Run executes name with args and env. A nil env means an empty
	// environment, never the parent's. It returns the raw result and does not
	// classify a timeout or a cancellation, so Fetch inspects ctx after a
	// failed Run.
	Run(ctx context.Context, name string, args, env []string, stderr io.Writer) error
}

// osExecutor is the production commandExecutor. It runs commands without a
// shell and with a bounded wait for the child's I/O pipes.
type osExecutor struct{}

var _ commandExecutor = osExecutor{}

// Run implements commandExecutor.
func (osExecutor) Run(ctx context.Context, name string, args, env []string, stderr io.Writer) error {
	cmd := exec.CommandContext(ctx, name, args...) //nolint:gosec // the executable is a trusted setting and the arguments never pass through a shell
	if env == nil {
		// os/exec would make the child inherit the parent environment, which
		// could hand it secrets; fail safe with an explicit empty one.
		env = []string{}
	}
	cmd.Env = env
	cmd.Stderr = stderr
	cmd.WaitDelay = execWaitDelay
	return cmd.Run()
}

// cappedWriter keeps at most maxStderrBytes of what is written to it and
// discards the rest. Write never returns an error: a short write would stop
// the copier and leave the child blocked or killed by SIGPIPE instead of
// drained.
type cappedWriter struct {
	buf []byte
}

// Write implements io.Writer.
func (w *cappedWriter) Write(p []byte) (int, error) {
	if remaining := maxStderrBytes - len(w.buf); remaining > 0 {
		w.buf = append(w.buf, p[:min(remaining, len(p))]...)
	}
	return len(p), nil
}

// String returns the retained prefix of the output.
func (w *cappedWriter) String() string {
	return string(w.buf)
}

// allowlistEnv returns the allowlisted variables set in parent, in the order
// of allowedEnvVars. The result is never nil: a parent without any allowlisted
// variable yields an empty environment instead of an inherited one.
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
