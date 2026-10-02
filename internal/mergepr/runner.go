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
	// has no use for a larger answer, and a runaway command could otherwise
	// exhaust memory.
	maxCommandOutputBytes = 1 << 20
	// maxCommandErrorBytes bounds the standard error kept for error messages.
	maxCommandErrorBytes = 4 << 10
	// commandWaitDelay bounds how long Run waits for inherited pipes after the
	// child exits or its context is done.
	commandWaitDelay = 5 * time.Second
)

// NewOSRunner returns the production Runner.
func NewOSRunner() Runner { return osRunner{} }

type osRunner struct{}

var _ Runner = osRunner{}

// Run implements Runner. It runs the command without a shell and never includes
// more than the bounded standard error in an error.
func (osRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...) //nolint:gosec // git and gh are trusted tools and no argument passes through a shell
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

// gitRedirectVars change which repository or index Git acts on. The tool always
// acts on the checkout it runs in, so it drops them from the child environment
// even though it otherwise inherits the developer's environment.
var gitRedirectVars = map[string]struct{}{
	"GIT_DIR":                          {},
	"GIT_WORK_TREE":                    {},
	"GIT_INDEX_FILE":                   {},
	"GIT_OBJECT_DIRECTORY":             {},
	"GIT_ALTERNATE_OBJECT_DIRECTORIES": {},
	"GIT_COMMON_DIR":                   {},
	"GIT_NAMESPACE":                    {},
	"GIT_PREFIX":                       {},
	"GIT_CEILING_DIRECTORIES":          {},
	"GIT_DISCOVERY_ACROSS_FILESYSTEM":  {},
}

// childEnv returns the developer's environment without the variables that
// redirect Git to another repository.
func childEnv() []string {
	parent := os.Environ()
	env := make([]string, 0, len(parent))
	for _, entry := range parent {
		name, _, _ := strings.Cut(entry, "=")
		if _, skip := gitRedirectVars[name]; skip {
			continue
		}
		env = append(env, entry)
	}
	return env
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
