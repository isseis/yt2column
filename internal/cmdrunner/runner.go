// Package cmdrunner runs external commands without a shell. It is the shared
// execution boundary of the developer tools (internal/mergepr,
// internal/fixprguard, and internal/codexreview); tests substitute the fake in
// internal/cmdrunner/testutil. It also holds the environment and the global
// options those tools run git with (GitEnv and GitHardening).
package cmdrunner

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
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
// warning. ProcessGroup starts the command as the leader of a new process
// group and stops the whole group, not only the direct child: a done context
// asks the group to terminate (SIGTERM) and kills it (SIGKILL) when the direct
// child has not exited after GroupStopGrace (at once when it is zero), and the
// direct child's exit is followed by a kill of the group before the output is
// drained. With ProcessGroup, WaitDelay bounds only the wait for the output
// pipes after the group is killed. The zero value means no bound, the current
// directory, the inherited environment, standard error discarded on success,
// and only the direct child stopped on cancellation.
type Options struct {
	MaxOutput       int
	WaitDelay       time.Duration
	Dir             string
	Env             []string
	StderrIsFailure bool
	ProcessGroup    bool
	GroupStopGrace  time.Duration
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
	stdout := &cappedBuffer{max: r.opts.MaxOutput}
	stderr := &cappedBuffer{max: r.opts.MaxOutput}
	var err error
	if r.opts.ProcessGroup {
		err = r.runGroup(ctx, stdin, stdout, stderr, name, args)
	} else {
		err = r.runDirect(ctx, stdin, stdout, stderr, name, args)
	}
	cmdline := name + " " + strings.Join(args, " ")
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

// runDirect runs the command as a child of this process, stopping only the
// direct child on cancellation.
func (r osRunner) runDirect(ctx context.Context, stdin []byte, stdout, stderr io.Writer, name string, args []string) error {
	cmd := exec.CommandContext(ctx, name, args...) //nolint:gosec // git, gh and make are trusted tools and no argument passes through a shell
	cmd.Dir = r.opts.Dir
	cmd.Env = r.opts.Env
	cmd.WaitDelay = r.opts.WaitDelay
	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return cmd.Run()
}

// runGroup runs the command as the leader of a new process group.
//
// Contract: when it returns, the group has been sent SIGKILL after the direct
// child exited, so no descendant that stayed in the group keeps running next
// to the caller's next step or keeps the output pipes open. A done context
// sends the group SIGTERM, then SIGKILL once GroupStopGrace has passed without
// the direct child exiting. The result follows the direct child's exit status;
// a descendant still holding a pipe after the kill (one that left the group)
// is waited for at most WaitDelay.
//
// The output goes through pipes this function owns, not through exec's copy
// goroutines, so Wait returns when the direct child exits instead of when the
// pipes close; the group kill comes between the two.
func (r osRunner) runGroup(ctx context.Context, stdin []byte, stdout, stderr io.Writer, name string, args []string) error {
	cmd := exec.Command(name, args...) //nolint:gosec // the callers' commands are fixed tools and no argument passes through a shell
	cmd.Dir = r.opts.Dir
	cmd.Env = r.opts.Env
	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	outR, outW, err := os.Pipe()
	if err != nil {
		return err
	}
	errR, errW, err := os.Pipe()
	if err != nil {
		_ = outR.Close()
		_ = outW.Close()
		return err
	}
	cmd.Stdout = outW
	cmd.Stderr = errW
	startErr := cmd.Start()
	// The child holds its own copies of the write ends; closing ours lets the
	// readers see end of file once every process holding them is gone.
	_ = outW.Close()
	_ = errW.Close()
	if startErr != nil {
		_ = outR.Close()
		_ = errR.Close()
		return startErr
	}
	var copies sync.WaitGroup
	copies.Go(func() { _, _ = io.Copy(stdout, outR) })
	copies.Go(func() { _, _ = io.Copy(stderr, errR) })
	drained := make(chan struct{})
	go func() {
		copies.Wait()
		close(drained)
	}()

	stopped, waitErr := waitOrStop(ctx, cmd, r.opts.GroupStopGrace)
	signalGroup(cmd.Process, syscall.SIGKILL)

	pipesClosed := waitClosed(drained, r.opts.WaitDelay)
	// Closing the read ends ends any copy still blocked on a pipe held by a
	// process outside the group; wait for the copies so the buffers are not
	// written after this returns.
	_ = outR.Close()
	_ = errR.Close()
	<-drained
	switch {
	case waitErr != nil:
		return waitErr
	case stopped:
		return ctx.Err()
	case !pipesClosed:
		return exec.ErrWaitDelay
	}
	return nil
}

// waitOrStop waits for the direct child to exit. When ctx is done first, it
// sends the group SIGTERM and, if the child has not exited after grace,
// SIGKILL, and reports that it stopped the group.
func waitOrStop(ctx context.Context, cmd *exec.Cmd, grace time.Duration) (stopped bool, err error) {
	exited := make(chan error, 1)
	go func() { exited <- cmd.Wait() }()
	select {
	case err := <-exited:
		return false, err
	case <-ctx.Done():
	}
	signalGroup(cmd.Process, syscall.SIGTERM)
	timer := time.NewTimer(grace)
	defer timer.Stop()
	select {
	case err := <-exited:
		return true, err
	case <-timer.C:
	}
	signalGroup(cmd.Process, syscall.SIGKILL)
	return true, <-exited
}

// waitClosed waits for done, at most limit when limit is positive, and
// reports whether it was closed.
func waitClosed(done <-chan struct{}, limit time.Duration) bool {
	if limit <= 0 {
		<-done
		return true
	}
	timer := time.NewTimer(limit)
	defer timer.Stop()
	select {
	case <-done:
		return true
	case <-timer.C:
		return false
	}
}

// signalGroup sends sig to the process group led by process. A group that is
// already gone needs nothing; when the group cannot be signaled for another
// reason (for example only zombies remain), the direct child still gets sig.
func signalGroup(process *os.Process, sig syscall.Signal) {
	err := syscall.Kill(-process.Pid, sig)
	if err != nil && !errors.Is(err, syscall.ESRCH) {
		_ = process.Signal(sig)
	}
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
