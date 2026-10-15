// Package server implements the localhost HTTP server that accepts the browser
// extension's requests, validates them, and runs the article pipeline. It
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
	"strconv"
	"sync"
	"time"

	"github.com/isseis/yt2column/internal/authtoken"
	"github.com/isseis/yt2column/internal/failure"
	"github.com/isseis/yt2column/internal/nilcheck"
	"github.com/isseis/yt2column/internal/publisher"
	"github.com/isseis/yt2column/internal/redact"
	"github.com/isseis/yt2column/internal/writer"
)

// The http.Server timeouts and limits. A loopback client sends a small header
// and a body of at most maxBodyBytes; the write timeout covers the body read,
// the body checks, the parse, and the response.
const (
	readHeaderTimeout = 5 * time.Second
	readTimeout       = 30 * time.Second
	writeTimeout      = 60 * time.Second
	idleTimeout       = 60 * time.Second
	maxHeaderBytes    = 64 << 10
)

// ErrInvalidOptions reports a Server built with a missing dependency, a zero
// token, a zero port, or no log output. It is a caller mistake, not a request
// failure.
var ErrInvalidOptions = errors.New("invalid server options")

// The fixed reasons New rejects Options. Each names the missing or unusable
// field, and none holds a value.
var (
	errNilArticleWriter = errors.New("writer is nil")
	errNilPublisher     = errors.New("publisher is nil")
	errNilNotifier      = errors.New("notifier is nil")
	errNilLogOutput     = errors.New("log output is nil")
	errZeroToken        = errors.New("token is zero")
	errZeroPort         = errors.New("port is zero")
)

// Options configures a Server. Every field is required.
type Options struct {
	Port      uint16 // the listening port, also used by the Host check
	Token     authtoken.Token
	Writer    writer.ArticleWriter
	Publisher publisher.Publisher
	Notifier  failure.Notifier
	LogOutput io.Writer // usually os.Stderr; New wraps it in redact.Writer
	Secrets   []string  // from redact.SecretStrings; redacted from every log line
}

// Server is the localhost HTTP server. Build it with New and run it with Listen
// and Serve.
type Server struct {
	port      uint16
	token     authtoken.Token
	writer    writer.ArticleWriter
	publisher publisher.Publisher
	notifier  failure.Notifier
	logger    *slog.Logger
	http      *http.Server

	// slots bounds the authenticated requests in progress. A request takes one
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
// typed nil included), a zero Token, a zero Port, and a nil log output with an
// error wrapping ErrInvalidOptions. It builds the slog logger and the
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
		port:      opts.Port,
		token:     opts.Token,
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
	if opts.Token.IsZero() {
		errs = append(errs, errZeroToken)
	}
	if opts.Port == 0 {
		errs = append(errs, errZeroPort)
	}
	if err := errors.Join(errs...); err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidOptions, err)
	}
	return nil
}

// Listen opens one listener on 127.0.0.1 and one on ::1, both on the server's
// port. When either fails, it closes the one it opened and returns the error.
func (s *Server) Listen() ([]net.Listener, error) {
	port := strconv.Itoa(int(s.port))
	ipv4, err := net.Listen("tcp", "127.0.0.1:"+port)
	if err != nil {
		return nil, fmt.Errorf("listen on 127.0.0.1:%s: %w", port, err)
	}
	ipv6, err := net.Listen("tcp", "[::1]:"+port)
	if err != nil {
		_ = ipv4.Close()
		return nil, fmt.Errorf("listen on [::1]:%s: %w", port, err)
	}
	return []net.Listener{ipv4, ipv6}, nil
}

// Serve serves on listeners until ctx is canceled or a listener fails, then
// stops accepting connections, waits for in-flight requests, jobs, and failure
// notices, and returns. It returns nil after a cancellation and the listener
// error otherwise.
func (s *Server) Serve(ctx context.Context, listeners []net.Listener) error {
	serveErr := make(chan error, len(listeners))
	for _, listener := range listeners {
		go func(l net.Listener) {
			serveErr <- s.http.Serve(l)
		}(listener)
	}

	var cause error
	select {
	case <-ctx.Done():
	case cause = <-serveErr:
	}

	// Shutdown with a context that is not canceled, so it waits for the
	// handlers (bounded by WriteTimeout) instead of returning at once. It
	// closes the listeners, so every Serve call returns.
	shutdownCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := s.http.Shutdown(shutdownCtx); err != nil && cause == nil {
		cause = err
	}

	received := 0
	if cause != nil {
		received = 1
	}
	for received < len(listeners) {
		err := <-serveErr
		received++
		if err != nil && !errors.Is(err, http.ErrServerClosed) && cause == nil {
			cause = err
		}
	}

	// Jobs and notices are not canceled: each is bounded by its own timeout
	// (the LLM timeout, the per-message post timeout, and the notice timeout).
	s.jobs.Wait()
	s.jobCancel()
	return cause
}
