// Package server implements the HTTP server, served on a Unix domain socket,
// that accepts the article requests relayed from the browser extension,
// validates them, and runs the article pipeline. It
// decides every response from fixed values and the request ID it generates, and
// it writes every log line through a redacting writer, so no secret and no
// transcript text can reach a response or a log.
package server

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/isseis/yt2column/internal/failure"
	"github.com/isseis/yt2column/internal/nilcheck"
	"github.com/isseis/yt2column/internal/publisher"
	"github.com/isseis/yt2column/internal/redact"
	"github.com/isseis/yt2column/internal/writer"
)

// The http.Server timeouts and limits. A client sends a small header
// and a body of at most maxBodyBytes; the write timeout covers the body read,
// the body checks, the parse, and the response.
const (
	readHeaderTimeout = 5 * time.Second
	readTimeout       = 30 * time.Second
	writeTimeout      = 60 * time.Second
	idleTimeout       = 60 * time.Second
	maxHeaderBytes    = 64 << 10
)

// ErrInvalidOptions reports a Server built with a missing dependency or no log
// output. It is a caller mistake, not a request failure.
var ErrInvalidOptions = errors.New("invalid server options")

// The fixed reasons New rejects Options. Each names the missing or unusable
// field, and none holds a value.
var (
	errNilArticleWriter = errors.New("writer is nil")
	errNilPublisher     = errors.New("publisher is nil")
	errNilNotifier      = errors.New("notifier is nil")
	errNilLogOutput     = errors.New("log output is nil")
)

// Options configures a Server. Every field is required.
type Options struct {
	Writer    writer.ArticleWriter
	Publisher publisher.Publisher
	Notifier  failure.Notifier
	LogOutput io.Writer // usually os.Stderr; New wraps it in redact.Writer
	Secrets   []string  // from redact.SecretStrings; redacted from every log line
}

// Server is the article HTTP server. Build it with New and run it with Serve on
// a listener the caller opened.
type Server struct {
	writer    writer.ArticleWriter
	publisher publisher.Publisher
	notifier  failure.Notifier
	logger    *slog.Logger
	http      *http.Server

	// slots bounds the requests in progress. A request takes one
	// after the header checks and holds it until the end of its job or its
	// failure notice.
	slots chan struct{}
	// jobs counts the in-flight jobs and failure notices, so Serve waits for
	// them without canceling them.
	jobs sync.WaitGroup
	// jobCtx is the parent of every job and notice context. It is not the
	// request context, which ends when the response is written, and Serve
	// cancels it only after every job and notice has finished.
	jobCtx    context.Context
	jobCancel context.CancelFunc
}

// New validates opts and returns a Server. It rejects a nil collaborator (a
// typed nil included) and a nil log output with an error wrapping
// ErrInvalidOptions. It builds the slog logger and the
// http.Server ErrorLog itself, both writing through
// redact.Writer(opts.LogOutput, opts.Secrets), so no log path can bypass the
// redaction.
func New(opts Options) (*Server, error) {
	if err := validateOptions(opts); err != nil {
		return nil, err
	}
	logger := slog.New(slog.NewTextHandler(redact.Writer(opts.LogOutput, opts.Secrets), nil))
	jobCtx, jobCancel := context.WithCancel(context.Background())
	s := &Server{
		writer:    opts.Writer,
		publisher: opts.Publisher,
		notifier:  opts.Notifier,
		logger:    logger,
		slots:     make(chan struct{}, maxInFlight),
		jobCtx:    jobCtx,
		jobCancel: jobCancel,
	}
	s.http = &http.Server{
		Handler:                      http.HandlerFunc(s.handler),
		ReadHeaderTimeout:            readHeaderTimeout,
		ReadTimeout:                  readTimeout,
		WriteTimeout:                 writeTimeout,
		IdleTimeout:                  idleTimeout,
		MaxHeaderBytes:               maxHeaderBytes,
		DisableGeneralOptionsHandler: true,
		ErrorLog:                     slog.NewLogLogger(logger.Handler(), slog.LevelError),
	}
	return s, nil
}

// validateOptions rejects the Options New must not accept.
func validateOptions(opts Options) error {
	var errs []error
	if nilcheck.IsNil(opts.Writer) {
		errs = append(errs, errNilArticleWriter)
	}
	if nilcheck.IsNil(opts.Publisher) {
		errs = append(errs, errNilPublisher)
	}
	if nilcheck.IsNil(opts.Notifier) {
		errs = append(errs, errNilNotifier)
	}
	if nilcheck.IsNil(opts.LogOutput) {
		errs = append(errs, errNilLogOutput)
	}
	if err := errors.Join(errs...); err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidOptions, err)
	}
	return nil
}

// Serve serves on listener until ctx is canceled or the listener fails, then
// stops accepting connections, waits for in-flight requests, jobs, and failure
// notices, and returns. It returns nil after a cancellation and the listener
// error otherwise. Shutdown closes the listener; for a *net.UnixListener that
// also removes the socket.
func (s *Server) Serve(ctx context.Context, listener net.Listener) error {
	serveErr := make(chan error, 1)
	go func() {
		serveErr <- s.http.Serve(listener)
	}()

	var cause error
	served := false
	select {
	case <-ctx.Done():
	case cause = <-serveErr:
		served = true
	}

	// Shutdown with a context that is not canceled, so it waits for the
	// handlers (bounded by WriteTimeout) instead of returning at once. It
	// closes the listener, so the Serve call returns.
	shutdownCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := s.http.Shutdown(shutdownCtx); err != nil && cause == nil {
		cause = err
	}
	if !served {
		if err := <-serveErr; err != nil && !errors.Is(err, http.ErrServerClosed) && cause == nil {
			cause = err
		}
	}

	// Jobs and notices are not canceled: each is bounded by its own timeout
	// (the LLM timeout, the per-message post timeout, and the notice timeout).
	// Every handler has returned, so each slot still taken belongs to a job or
	// a notice: their count is what the wait below waits for.
	s.logger.Info("stopped accepting connections; waiting for in-flight jobs and failure notices", "pending", len(s.slots))
	s.jobs.Wait()
	s.jobCancel()
	s.logger.Info("every job and failure notice has finished")
	return cause
}
