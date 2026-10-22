// Package cmdrunner runs external commands without a shell. It is the shared
// execution boundary of the developer tools (internal/mergepr and
// internal/fixprguard); tests substitute the fake in internal/cmdrunner/testutil.
package cmdrunner

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// Runner runs an external command without a shell and returns its standard
// output. It moved here from internal/mergepr; the zero Options keep the
// behavior it had there.
type Runner interface {
	Run(ctx context.Context, name string, args ...string) ([]byte, error)
}

// InputRunner also feeds the command's standard input.
type InputRunner interface {
	Runner
	RunInput(ctx context.Context, stdin []byte, name string, args ...string) ([]byte, error)
}

// Options bound and place a command. MaxOutput caps each of the captured
// standard output and standard error while still draining the rest, WaitDelay
// bounds the wait after cancellation, Dir is the working directory, and Env is
// the child's whole environment as KEY=VALUE entries. StderrIsFailure makes a
// command that exits successfully but writes to standard error fail with
// ErrUnexpectedStderr, for a command that reports a skipped input only as a
// warning. The zero value means no bound, the current directory, the inherited
// environment, and standard error discarded on success.
type Options struct {
	MaxOutput       int
	WaitDelay       time.Duration
	Dir             string
	Env             []string
	StderrIsFailure bool
}

// ExitError is the typed failure of a command that ran and exited unsuccessfully.
// It carries the exit code and the capped standard output and standard error,
// so a caller can map a failure to its own sentinel without parsing the error
// string. A command whose output exceeded MaxOutput fails with
// ErrOutputTooLarge instead.
type ExitError struct {
	Code   int
	Stdout []byte
	Stderr []byte

	cmdline string
	cause   error
}

// Error names the command, the exit status, and the standard error.
func (e *ExitError) Error() string {
	return fmt.Sprintf("%s: %v: %s", e.cmdline, e.cause, strings.TrimSpace(string(e.Stderr)))
}

// Unwrap returns the underlying *exec.ExitError.
func (e *ExitError) Unwrap() error { return e.cause }

// ErrOutputTooLarge reports that a command wrote more than Options.MaxOutput
// bytes to standard output or standard error.
var ErrOutputTooLarge = errors.New("command output exceeds the cap")

// ErrUnexpectedStderr reports that a command run with
// Options.StderrIsFailure exited successfully but wrote to standard error.
// The error does not carry what the command wrote; a command that exits
// unsuccessfully still fails with *ExitError, which does, so a caller that
// must not print standard error must not wrap or format that error either.
var ErrUnexpectedStderr = errors.New("command wrote to standard error")

// NewOS returns the production runner.
func NewOS(opts Options) InputRunner { return osRunner{opts: opts} }

type osRunner struct{ opts Options }

// Run implements Runner.
func (r osRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	return r.RunInput(ctx, nil, name, args...)
}

// RunInput implements InputRunner.
func (r osRunner) RunInput(ctx context.Context, stdin []byte, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...) //nolint:gosec // git, gh and make are trusted tools and no argument passes through a shell
	cmd.Dir = r.opts.Dir
	cmd.Env = r.opts.Env
	cmd.WaitDelay = r.opts.WaitDelay
	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}
	stdout := &cappedBuffer{max: r.opts.MaxOutput}
	stderr := &cappedBuffer{max: r.opts.MaxOutput}
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	cmdline := name + " " + strings.Join(args, " ")
	err := cmd.Run()
	if stdout.exceeded || stderr.exceeded {
		return nil, fmt.Errorf("%s: %w", cmdline, ErrOutputTooLarge)
	}
	if err != nil {
		if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
			typed := &ExitError{
				Code:    exitErr.ExitCode(),
				Stdout:  stdout.buf.Bytes(),
				Stderr:  stderr.buf.Bytes(),
				cmdline: cmdline,
				cause:   err,
			}
			// A command killed by cancellation reports exit code -1; joining
			// the context error lets a caller tell a timeout from a failure.
			if ctxErr := ctx.Err(); ctxErr != nil {
				return nil, errors.Join(ctxErr, typed)
			}
			return nil, typed
		}
		return nil, fmt.Errorf("%s: %w: %s", cmdline, err, strings.TrimSpace(stderr.buf.String()))
	}
	if r.opts.StderrIsFailure && stderr.buf.Len() > 0 {
		return nil, fmt.Errorf("%s: %w", cmdline, ErrUnexpectedStderr)
	}
	return stdout.buf.Bytes(), nil
}

// cappedBuffer keeps at most max bytes (no limit when max is not positive)
// and discards everything once the cap is crossed while still accepting every write, so the child never blocks on a
// full pipe.
type cappedBuffer struct {
	max      int
	buf      bytes.Buffer
	exceeded bool
}

func (c *cappedBuffer) Write(p []byte) (int, error) {
	if c.max <= 0 {
		return c.buf.Write(p)
	}
	if c.exceeded || len(p) > c.max-c.buf.Len() {
		// Past the cap nothing is kept: the caller gets ErrOutputTooLarge, not
		// a truncated prefix. Writes are still accepted so the child can finish.
		c.exceeded = true
		return len(p), nil
	}
	return c.buf.Write(p)
}
