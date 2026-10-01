package mergepr

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
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

// NewOSRunner returns the production Runner.
func NewOSRunner() Runner { return osRunner{} }

type osRunner struct{}

var _ Runner = osRunner{}

// Run implements Runner. It runs the command without a shell and never
// includes more than the bounded standard error in an error.
func (osRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...) //nolint:gosec // git and gh are trusted tools and no argument passes through a shell
	var stdout, stderr limitedBuffer
	stdout.limit = maxCommandOutputBytes
	stderr.limit = maxCommandErrorBytes
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.WaitDelay = commandWaitDelay
	command := name + " " + strings.Join(args, " ")
	if err := cmd.Run(); err != nil {
		message := strings.TrimSpace(stderr.buffer.String())
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
