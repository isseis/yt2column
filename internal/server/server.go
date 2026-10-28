// Package server implements the HTTP server, served on a Unix domain socket,
// that accepts the requests relayed from the browser extension. It serves two
// APIs: the legacy /articles endpoint, which validates a request and runs the
// article pipeline from generation through the Webhook post, and the job API,
// which generates an article into the in-memory article cache and reports its
// state. It decides every response from fixed values,
// the identifiers it generates, and a completed article, and it writes every
// log line through a redacting writer, so no secret and no transcript text can
// reach a response or a log.
package server

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/isseis/yt2column/internal/articlecache"
	"github.com/isseis/yt2column/internal/failure"
	"github.com/isseis/yt2column/internal/nilcheck"
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
	errNilArticleWriter    = errors.New("writer is nil")
	errNilWebhookPublisher = errors.New("webhook publisher is nil")
	errNilWebhookNotifier  = errors.New("webhook notifier is nil")
	errNilLogOutput        = errors.New("log output is nil")
)

// Options configures a Server. Writer and LogOutput are required.
type Options struct {
	Writer    writer.ProgressWriter
	Webhook   *WebhookOptions // nil: SLACK_WEBHOOK_URL is not configured
	LogOutput io.Writer       // usually os.Stderr; New wraps it in redact.Writer
	Secrets   []string        // from redact.SecretStrings; redacted from every log line
}

// WebhookOptions holds what posts to the Webhook. Every field is required.
type WebhookOptions struct {
	Publisher WebhookPublisher // the legacy /articles job and the publish operation
	Notifier  failure.Notifier // the legacy /articles failure notice
}

// Server is the article HTTP server. Build it with New and run it with Serve on
// a listener the caller opened.
type Server struct {
	writer writer.ProgressWriter
	// publisher and notifier post to the Webhook. Both are nil when
	// SLACK_WEBHOOK_URL is not configured, in which case the legacy endpoint
	// rejects every request: there is nowhere to post the article or its
	// failure notice. New copies them from Options, so a caller that changes
	// its WebhookOptions afterwards does not change this server.
	publisher WebhookPublisher
	notifier  failure.Notifier
	logger    *slog.Logger
	http      *http.Server

	// slots are the generation slots, shared by both APIs. A legacy request
	// takes one after the header checks and holds it until the end of its job
	// or its failure notice; a generation start takes one when the cache
	// starts a generation, and its generation job holds it until the article
	// is generated.
	slots chan struct{}
	// reading bounds the generation starts that read and validate a body at
	// once. It is separate from slots, so a resent request for a running or
	// done key gets its key while every generation slot is taken.
	reading chan struct{}
	// cache holds the job API's records, and keyer computes their keys with
	// this process's secret, which New draws and nothing outputs.
	cache *articlecache.Cache
	keyer articlecache.Keyer
	// jobs counts the in-flight jobs and failure notices, the generation jobs
	// included, so Serve waits for them.
	jobs sync.WaitGroup
	// jobCtx is the parent of every legacy job and notice context. It is not
	// the request context, which ends when the response is written, and Serve
	// cancels it only after every job and notice has finished.
	jobCtx    context.Context
	jobCancel context.CancelFunc
	// genCtx is the context of every generation job. Like jobCtx it outlives
	// the request; unlike jobCtx, Serve cancels it first when it shuts down,
	// since the cache is lost on exit and no one could read the article.
	genCtx    context.Context
	genCancel context.CancelFunc

	// beforeGenerationJob, when set, runs after Begin started a generation and
	// before the job takes it over. It is always nil outside a test, which
	// sets it to panic there to check the handler's cleanup.
	beforeGenerationJob func()
}

// internals are the values New sets for production and a test sets itself:
// the cache's record limit and clock, and the key secret.
type internals struct {
	capacity int
	clock    func() time.Time // nil selects time.Now
	secret   [32]byte
}

// New validates opts and returns a Server. It rejects a nil collaborator (a
// typed nil included) and a nil log output with an error wrapping
// ErrInvalidOptions. It builds the slog logger and the
// http.Server ErrorLog itself, both writing through
// redact.Writer(opts.LogOutput, opts.Secrets), so no log path can bypass the
// redaction.
//
// New draws the article key secret from crypto/rand. It keeps the secret only
// in memory, so the keys of one process mean nothing to the next.
func New(opts Options) (*Server, error) {
	in := internals{capacity: articlecache.DefaultCapacity}
	// crypto/rand.Read never returns an error: it stops the process when it
	// cannot read randomness, so the server never starts with a weak secret.
	_, _ = rand.Read(in.secret[:])
	return newWithInternals(opts, in)
}

// newWithInternals is New with the internals given: New passes the production
// values, and a test passes a small limit, its own clock, and a fixed secret.
func newWithInternals(opts Options, in internals) (*Server, error) {
	if err := validateOptions(opts); err != nil {
		return nil, err
	}
	cache, err := articlecache.New(articlecache.Options{Capacity: in.capacity, Clock: in.clock})
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidOptions, err)
	}
	logger := slog.New(slog.NewTextHandler(redact.Writer(opts.LogOutput, opts.Secrets), nil))
	jobCtx, jobCancel := context.WithCancel(context.Background())
	genCtx, genCancel := context.WithCancel(context.Background())
	s := &Server{
		writer:    opts.Writer,
		logger:    logger,
		slots:     make(chan struct{}, maxInFlight),
		reading:   make(chan struct{}, maxReading),
		cache:     cache,
		keyer:     articlecache.NewKeyer(in.secret),
		jobCtx:    jobCtx,
		jobCancel: jobCancel,
		genCtx:    genCtx,
		genCancel: genCancel,
	}
	if opts.Webhook != nil {
		s.publisher = opts.Webhook.Publisher
		s.notifier = opts.Webhook.Notifier
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
	if opts.Webhook != nil {
		if nilcheck.IsNil(opts.Webhook.Publisher) {
			errs = append(errs, errNilWebhookPublisher)
		}
		if nilcheck.IsNil(opts.Webhook.Notifier) {
			errs = append(errs, errNilWebhookNotifier)
		}
	}
	if nilcheck.IsNil(opts.LogOutput) {
		errs = append(errs, errNilLogOutput)
	}
	if err := errors.Join(errs...); err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidOptions, err)
	}
	return nil
}

// webhookConfigured reports whether this server can post to the Webhook, which
// is what SLACK_WEBHOOK_URL being set means. A server without it rejects the
// legacy endpoint instead of starting a job nothing can post. New keeps the two
// collaborators either both set or both nil, so testing one is enough.
func (s *Server) webhookConfigured() bool {
	return s.publisher != nil
}

// Serve serves on listener until ctx is canceled or the listener fails, then
// cancels the job API's generation jobs, stops accepting connections, waits
// for in-flight requests, jobs, failure notices, and the canceled generations,
// and returns. The legacy jobs and their notices are not canceled. It returns
// nil after a cancellation and the listener error otherwise. Shutdown closes
// the listener; for a *net.UnixListener that also removes the socket.
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

	// The article cache is lost when the process exits, so a generation that
	// finished from here on could never be read: cancel them first, so a
	// running one stops its LLM call and a start accepted during the shutdown
	// begins already canceled. Each canceled job logs one line.
	s.genCancel()

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

	// The legacy jobs and notices are not canceled: each is bounded by its own
	// timeout (the LLM timeout, the per-message post timeout, and the notice
	// timeout). Every handler has returned, so each generation slot still
	// taken belongs to a legacy job, a notice, or a canceled generation: their
	// count is what the wait below waits for.
	s.logger.Info("stopped accepting connections; waiting for in-flight jobs and failure notices", "pending", len(s.slots))
	s.jobs.Wait()
	s.jobCancel()
	s.logger.Info("every job and failure notice has finished")
	return cause
}
