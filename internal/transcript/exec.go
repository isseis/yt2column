package transcript

import (
	"cmp"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"regexp"
	"slices"
	"strings"
	"syscall"
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

// Proxy environment variable names, shared by the allowlist and the stderr
// redaction, which must treat the same names as sensitive.
const (
	envHTTPProxy       = "HTTP_PROXY"
	envHTTPSProxy      = "HTTPS_PROXY"
	envNOProxy         = "NO_PROXY"
	envALLProxy        = "ALL_PROXY"
	envHTTPProxyLower  = "http_proxy"
	envHTTPSProxyLower = "https_proxy"
	envNOProxyLower    = "no_proxy"
	envALLProxyLower   = "all_proxy"
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
	envHTTPProxy,
	envHTTPSProxy,
	envNOProxy,
	envALLProxy,
	envHTTPProxyLower,
	envHTTPSProxyLower,
	envNOProxyLower,
	envALLProxyLower,
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
	// environment, never the parent's. inherited are passed to the child as
	// file descriptors 3, 4, ... in order. It returns the raw result and does
	// not classify a timeout or a cancellation, so Fetch inspects ctx after a
	// failed Run.
	Run(ctx context.Context, name string, args, env []string, inherited []*os.File, stderr io.Writer) error
}

// osExecutor is the production commandExecutor. It runs commands without a
// shell and with a bounded wait for the child's I/O pipes.
type osExecutor struct{}

var _ commandExecutor = osExecutor{}

// Run implements commandExecutor.
func (osExecutor) Run(ctx context.Context, name string, args, env []string, inherited []*os.File, stderr io.Writer) error {
	cmd := exec.CommandContext(ctx, name, args...) //nolint:gosec // the executable is a trusted setting and the arguments never pass through a shell
	if env == nil {
		// os/exec would make the child inherit the parent environment, which
		// could hand it secrets; fail safe with an explicit empty one.
		env = []string{}
	}
	cmd.Env = env
	cmd.ExtraFiles = inherited
	cmd.Stderr = stderr
	cmd.WaitDelay = execWaitDelay
	// Contract: the child leads a new process group, and a finished context
	// kills the whole group, never only the direct child. A wrapper script or a
	// single-file launcher would otherwise leave the real yt-dlp running after
	// Run returns, still holding every inherited file. This covers cancellation
	// only; a child that exits on its own is not followed by a group kill.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { return killProcessGroup(cmd.Process) }
	return cmd.Run()
}

// killProcessGroup sends SIGKILL to the process group led by process. A group
// that is already gone is reported as os.ErrProcessDone, as exec.Cmd expects.
func killProcessGroup(process *os.Process) error {
	err := syscall.Kill(-process.Pid, syscall.SIGKILL)
	if errors.Is(err, syscall.ESRCH) {
		return os.ErrProcessDone
	}
	if err != nil {
		// The group could not be signaled (for example only zombies remain);
		// still stop the direct child.
		return process.Kill()
	}
	return nil
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

// proxyEnvVars are the allowlisted variables whose values can carry
// credentials and must be redacted from captured output.
var proxyEnvVars = map[string]struct{}{
	envHTTPProxy:       {},
	envHTTPSProxy:      {},
	envNOProxy:         {},
	envALLProxy:        {},
	envHTTPProxyLower:  {},
	envHTTPSProxyLower: {},
	envNOProxyLower:    {},
	envALLProxyLower:   {},
}

// redactedMarker replaces credentials in captured output.
const redactedMarker = "[redacted]"

// userinfoPattern matches the userinfo of a URL between "://" and "@".
var userinfoPattern = regexp.MustCompile(`://[^/?#\s@]*@`)

// redactStderr removes credentials from captured standard error output before
// it is added to an error: the non-empty values of the proxy variables given
// to the child (including a value cut off at the cap) and URL userinfo. Every
// span is located in the original output and merged before any replacement, so
// a value that is a prefix of another cannot split it and leave part visible.
func redactStderr(stderr string, env []string) string {
	var spans [][2]int
	for _, entry := range env {
		name, value, ok := strings.Cut(entry, "=")
		if !ok || value == "" {
			continue
		}
		if _, isProxy := proxyEnvVars[name]; !isProxy {
			continue
		}
		for offset := 0; ; offset++ {
			index := strings.Index(stderr[offset:], value)
			if index < 0 {
				break
			}
			offset += index
			spans = append(spans, [2]int{offset, offset + len(value)})
		}
		if length := truncatedPrefixLength(stderr, value); length > 0 {
			spans = append(spans, [2]int{len(stderr) - length, len(stderr)})
		}
	}
	slices.SortFunc(spans, func(a, b [2]int) int { return cmp.Compare(a[0], b[0]) })
	var out strings.Builder
	written := 0
	for i := 0; i < len(spans); {
		start, end := spans[i][0], spans[i][1]
		for i++; i < len(spans) && spans[i][0] <= end; i++ {
			end = max(end, spans[i][1])
		}
		out.WriteString(stderr[written:start])
		out.WriteString(redactedMarker)
		written = end
	}
	out.WriteString(stderr[written:])
	return userinfoPattern.ReplaceAllString(out.String(), "://"+redactedMarker+"@")
}

// truncatedPrefixLength returns the length of the longest proper prefix of
// value that ends output, so a value cut off at the stderr cap does not leave
// its beginning in the output.
func truncatedPrefixLength(output, value string) int {
	for length := min(len(value)-1, len(output)); length > 0; length-- {
		if strings.HasSuffix(output, value[:length]) {
			return length
		}
	}
	return 0
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
