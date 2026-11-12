package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"sync"

	"github.com/isseis/yt2column/internal/config"
	"github.com/isseis/yt2column/internal/llm"
	"github.com/isseis/yt2column/internal/llm/devfake"
	"github.com/isseis/yt2column/internal/redact"
	"github.com/isseis/yt2column/internal/server"
	"github.com/isseis/yt2column/internal/serversock"
	"github.com/isseis/yt2column/internal/writer"
)

// Exit codes, chosen by the step that failed as in the production server: a
// failure before the server listens (the arguments, HOME, building a
// component) is exitUsage, a failure to listen or to keep serving is
// exitFailure, and a stop by a signal or a request for the usage is exitOK.
const (
	exitOK      = 0
	exitFailure = 1
	exitUsage   = 2
)

// programName prefixes every line written to standard error before the server
// logger takes over.
const programName = "yt2column-devserver"

// deps are what run substitutes in a test. scenario wraps the checked
// scenario, so a test can replace how its calls wait; main's production value
// changes nothing. newWriter lets a test observe the options the writers are
// built with and fail their construction.
type deps struct {
	scenario  func(devfake.Scenario) devfake.Scenario
	newWriter func(llm.LLMClient, writer.Options) (writer.ProgressWriter, error)
}

// productionDeps returns the real values: the scenario unchanged and the real
// writer.
func productionDeps() deps {
	return deps{
		scenario:  func(s devfake.Scenario) devfake.Scenario { return s },
		newWriter: writer.NewProgressWriter,
	}
}

// stderrWriter writes lines to standard error through redact.Line, which
// escapes non-printable characters. The development server holds no secret.
type stderrWriter struct {
	w io.Writer
}

// line writes one redacted line.
func (s *stderrWriter) line(format string, args ...any) {
	_, _ = io.WriteString(s.w, redact.Line(fmt.Sprintf(format, args...))+"\n")
}

// lockedWriter serializes the writes to w, so lines written by several
// loggers never interleave or race.
type lockedWriter struct {
	mu sync.Mutex
	w  io.Writer
}

// Write implements io.Writer.
func (l *lockedWriter) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.w.Write(p)
}

// run starts the development server and serves until ctx is canceled, then
// returns the process exit code. Nothing is written to standard output.
//
// Contract: the steps before the socket (the arguments, the scenario, HOME,
// the writer check, the server) touch no file and no network, so a rejection
// there leaves no socket directory or lock behind. The start lock is held from
// the moment Acquire returns until Serve has returned. The "listening" line is
// written only after the socket is bound, so a start that stops at the lock
// never says it runs on a fake. Only HOME is read from the environment.
//
// The start, the lock, and the stop notice copy the production server's
// runServer (cmd/yt2column-server/run.go) instead of sharing it; the stop
// notice is written by context.AfterFunc, and run waits for it once it has
// started, without the production server's guarantee that it ends before
// Serve returns.
func run(ctx context.Context, args []string, lookup config.LookupFunc, stderr io.Writer, d deps) int {
	stderr = &lockedWriter{w: stderr}
	errOut := &stderrWriter{w: stderr}

	scenario, err := parseArgs(args)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			_, _ = io.WriteString(stderr, usageText())
			return exitOK
		}
		errOut.line("%s: %v", programName, err)
		_, _ = io.WriteString(stderr, usageText())
		return exitUsage
	}
	scenario = d.scenario(scenario)

	cfg, err := config.LoadHost(lookup)
	if err != nil {
		errOut.line("%s: configuration: %v", programName, err)
		return exitUsage
	}

	// The writer is built once here so a failure to build it stops the start;
	// the jobs then build it again with the same inputs.
	jw := perJobWriter{scenario: scenario, newWriter: d.newWriter}
	if _, err := jw.jobWriter(); err != nil {
		errOut.line("%s: build the article writer: %v", programName, err)
		return exitUsage
	}
	srv, err := server.New(server.Options{Writer: jw, LogOutput: stderr})
	if err != nil {
		errOut.line("%s: build the server: %v", programName, err)
		return exitUsage
	}

	lock, err := serversock.Acquire(cfg.SocketPath())
	if err != nil {
		errOut.line("%s: %v%s", programName, err, listenHint(err))
		return exitFailure
	}
	listener, err := lock.Listen()
	if err != nil {
		errOut.line("%s: %v%s", programName, err, listenHint(err))
		releaseLock(errOut, lock)
		return exitFailure
	}
	logger := slog.New(slog.NewTextHandler(redact.Writer(stderr, nil), nil))
	logger.Info("listening",
		"socket", cfg.SocketPath(),
		"llm", "fake",
		"model", devfake.ModelName,
		"scenario", scenario,
	)

	noticed := make(chan struct{})
	stopNotice := context.AfterFunc(ctx, func() {
		defer close(noticed)
		logger.Info("stopping: the in-flight generations are canceled and their articles lost; the server exits once their calls return; send the signal again to exit at once")
	})
	err = srv.Serve(ctx, listener)
	// The notice is written at the cancellation, while Serve waits for the
	// generations. If the function has started it is waited for, so it never
	// comes after the lock is released or the stop line; if the context was
	// not canceled it never runs.
	if !stopNotice() {
		<-noticed
	}
	releaseLock(errOut, lock)
	if err != nil {
		logger.Error("serving failed", "error", err)
		return exitFailure
	}
	logger.Info("stopped")
	return exitOK
}

// listenHint returns the advice appended to a failure to prepare or bind the
// socket, chosen by the error's sentinel.
func listenHint(err error) string {
	switch {
	case errors.Is(err, serversock.ErrAlreadyRunning):
		return "; another server is running, or is stopping and waiting for its in-flight jobs"
	case errors.Is(err, serversock.ErrInUse):
		return "; stop the process that listens on the socket, or check the file at that path"
	default:
		return ""
	}
}

// releaseLock releases the start lock and reports a failure, which does not
// change the exit code: the lock is released when the process exits anyway.
func releaseLock(errOut *stderrWriter, lock *serversock.Lock) {
	if err := lock.Release(); err != nil {
		errOut.line("%s: release the start lock: %v", programName, err)
	}
}
