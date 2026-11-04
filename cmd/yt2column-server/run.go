package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"time"

	"github.com/isseis/yt2column/internal/config"
	"github.com/isseis/yt2column/internal/llm"
	"github.com/isseis/yt2column/internal/llm/provider"
	"github.com/isseis/yt2column/internal/publisher"
	"github.com/isseis/yt2column/internal/redact"
	"github.com/isseis/yt2column/internal/secret"
	"github.com/isseis/yt2column/internal/server"
	"github.com/isseis/yt2column/internal/serversock"
	"github.com/isseis/yt2column/internal/writer"
)

// Exit codes. The code is chosen by the step that failed, never by inspecting
// an error message: a failure before the server listens (the arguments, the
// configuration, HOME and the socket path derived from it included, or building
// a component) is exitUsage, and a failure to listen (the socket directory, the
// start lock, the name at the socket path) or to keep serving is exitFailure. A
// stop by a signal is exitOK.
const (
	exitOK      = 0
	exitFailure = 1
	exitUsage   = 2
)

// programName prefixes every line written to standard error before the server
// logger takes over.
const programName = "yt2column-server"

// deps are the constructors run substitutes in a test: the two LLM clients
// and the article publisher, the only components that reach the network, and
// the writer, so a test can observe the options it is built with. main passes
// productionDeps().
type deps struct {
	newLLMClient       func(config.LLMConfig) (llm.LLMClient, error)
	newReviewLLMClient func(config.LLMConfig) (llm.LLMClient, error)
	newWriter          func(llm.LLMClient, writer.Options) (writer.ProgressWriter, error)
	newPublisher       func(webhookURL secret.Secret) (server.WebhookPublisher, error)
}

// productionDeps returns the real constructors.
func productionDeps() deps {
	return deps{
		newLLMClient:       provider.New,
		newReviewLLMClient: provider.NewReview,
		newWriter:          writer.NewProgressWriter,
		newPublisher:       newPublisher,
	}
}

// newPublisher builds the SlackWebhookPublisher, the publisher the CLI's
// --slack uses and the server posts with. It returns a nil interface, never a
// typed nil, when construction fails, so server.New's nil check is not misled.
func newPublisher(webhookURL secret.Secret) (server.WebhookPublisher, error) {
	p, err := publisher.NewSlackWebhookPublisher(webhookURL)
	if err != nil {
		return nil, err
	}
	return p, nil
}

// writerOptions returns the writer options of the server: the same as a CLI
// run with no step flags, so both entry points generate the same article.
// The CLI builds its options from its flags, so the two are not shared code;
// the tests of both compare them with one value instead.
func writerOptions(review llm.LLMClient) writer.Options {
	return writer.Options{
		Shorten: writer.ShortenOptions{Enabled: true},
		Review:  writer.ReviewOptions{Enabled: true, Client: review},
	}
}

// stderrWriter writes the startup lines to standard error. Every line goes
// through redact.Line, which escapes non-printable characters and redacts the
// secrets known so far.
type stderrWriter struct {
	w       io.Writer
	secrets []string
}

// line writes one redacted line.
func (s *stderrWriter) line(format string, args ...any) {
	_, _ = io.WriteString(s.w, redact.Line(fmt.Sprintf(format, args...), s.secrets...)+"\n")
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

// run starts the server and serves until ctx is canceled, then returns the
// process exit code. Steps 1 and 2 touch no network and no file; a failure
// there stops the start with exitUsage. Step 3 prepares the socket directory,
// takes the start lock, and binds the socket; a failure there is exitFailure.
// Nothing is written to standard output; every line goes to standard error.
//
// Contract: the start lock is held from step 3 until Serve has returned, that
// is, until every in-flight job has finished, so a server
// started during the shutdown stops with serversock.ErrAlreadyRunning instead
// of running jobs beside this one. lock stays reachable until Release, so the
// lock file is never closed by a finalizer before then.
func run(ctx context.Context, args []string, lookup config.LookupFunc, stderr io.Writer, d deps) int {
	// run's own logger and the server's write to stderr from different
	// goroutines, so each write is serialized here rather than relying on
	// what the writer tolerates.
	stderr = &lockedWriter{w: stderr}
	errOut := &stderrWriter{w: stderr}
	if len(args) != 0 {
		errOut.line("%s: takes no arguments; the configuration is read from environment variables (see README.en.md)", programName)
		return exitUsage
	}

	// 1: the configuration. Each rejected variable is reported on its own
	// line; a VarError never carries a value.
	cfg, err := config.LoadServer(lookup)
	if err != nil {
		for _, e := range splitJoined(err) {
			errOut.line("%s: configuration: %v", programName, e)
		}
		return exitUsage
	}
	llmCfg := cfg.LLM()
	webhookURL, webhookConfigured := cfg.WebhookURL()
	secrets := redact.SecretStrings(webhookURL, llmCfg.DeepSeekAPIKey(), llmCfg.AnthropicAPIKey())
	errOut.secrets = secrets
	if cfg.HTTP2DebugEnabled() {
		errOut.line("%s: warning: GODEBUG enables http2debug, so the Go HTTP/2 log may write the API key and the Webhook URL path to standard error", programName)
	}

	// 2: the components. None of the constructors touches the network.
	srv, err := buildServer(d, cfg, stderr, secrets)
	if err != nil {
		errOut.line("%s: %v", programName, err)
		return exitUsage
	}

	// 3: the socket directory, the start lock, and the socket.
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
	logger := slog.New(slog.NewTextHandler(redact.Writer(stderr, secrets), nil))
	logger.Info("listening",
		"socket", cfg.SocketPath(),
		"model", llmCfg.Generation().Model(),
		"reviewModel", llmCfg.Review().Model(),
		"webhookConfigured", webhookConfigured,
	)
	if !webhookConfigured {
		// The server now starts without the Webhook, where it used to stop, so
		// a forgotten setting has to show up here instead.
		logger.Warn("no Webhook is configured, so the publish operation answers webhook_not_configured")
	}

	// 4: serve until ctx is canceled. The stop notice is logged as soon as
	// the cancellation arrives, while Serve is still waiting for in-flight
	// jobs, not after the wait it announces.
	notice := afterCancel(ctx, stopNoticeLogger(logger))
	err = srv.Serve(ctx, listener)
	notice.finish()
	// Serve has waited for every job, so the start lock
	// can be released now and not before.
	releaseLock(errOut, lock)
	if err != nil {
		logger.Error("serving failed", "error", err)
		return exitFailure
	}
	logger.Info("stopped")
	return exitOK
}

// listenHint returns the advice appended to a failure to prepare or bind the
// socket, chosen by the error's sentinel. An *serversock.UnsafeError already
// says that nothing was repaired, so it gets none.
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

// stopNoticeLogger returns the function that logs the stop notice. The wait
// it names is the longest one post job can take, publisher.SlackMaxPostDuration:
// the post jobs run in parallel and are never retried by themselves.
func stopNoticeLogger(logger *slog.Logger) func() {
	return func() {
		logger.Info(fmt.Sprintf("stopping: the in-flight generations are canceled and their articles lost; the posts sent from the viewer are allowed to finish, which can take up to about %d minutes; send the signal again to exit at once, which drops them",
			approxMinutes(publisher.SlackMaxPostDuration)))
	}
}

// approxMinutes returns d rounded to the nearest minute, and at least 1, so
// the stop notice never names 0 minutes.
func approxMinutes(d time.Duration) int {
	return max(1, int(d.Round(time.Minute)/time.Minute))
}

// cancelHook runs f once ctx is canceled. Contract: if ctx was canceled
// before finish is called, f has run to completion when finish returns;
// f never runs after finish returns.
type cancelHook struct {
	ctx  context.Context
	f    func()
	stop func() bool
	done chan struct{}
}

// afterCancel registers f to run once ctx is canceled. The caller calls
// finish on the result after the work ctx bounds has returned.
func afterCancel(ctx context.Context, f func()) *cancelHook {
	h := &cancelHook{ctx: ctx, f: f, done: make(chan struct{})}
	h.stop = context.AfterFunc(ctx, h.run)
	return h
}

// run is called by context.AfterFunc, in its own goroutine, once ctx is
// canceled.
func (h *cancelHook) run() {
	defer close(h.done)
	h.f()
}

// finish is called after the work ctx bounds has returned. Whether ctx was
// canceled is read from ctx.Err, not from the result of AfterFunc's stop: a
// cancellation closes ctx.Done before it schedules AfterFunc callbacks, so
// work that returned on ctx.Done can call stop in that gap, and stop would
// then report that it prevented f, dropping a call the cancellation owed.
func (h *cancelHook) finish() {
	if h.ctx.Err() == nil && h.stop() {
		return
	}
	<-h.done
}

// buildServer builds the components and the server. logOutput and secrets
// become the server's redacted log.
func buildServer(d deps, cfg config.ServerConfig, logOutput io.Writer, secrets []string) (*server.Server, error) {
	client, err := d.newLLMClient(cfg.LLM())
	if err != nil {
		return nil, fmt.Errorf("build the LLM client: %w", err)
	}
	review, err := d.newReviewLLMClient(cfg.LLM())
	if err != nil {
		return nil, fmt.Errorf("build the review LLM client: %w", err)
	}
	articleWriter, err := d.newWriter(client, writerOptions(review))
	if err != nil {
		return nil, fmt.Errorf("build the article writer: %w", err)
	}
	opts := server.Options{
		Writer:    articleWriter,
		LogOutput: logOutput,
		Secrets:   secrets,
	}
	// A server without SLACK_WEBHOOK_URL still serves the operations that do
	// not post, so it is built without a publisher: the publish operation
	// rejects what needs one.
	if webhookURL, configured := cfg.WebhookURL(); configured {
		pub, err := d.newPublisher(webhookURL)
		if err != nil {
			return nil, fmt.Errorf("build the publisher: %w", err)
		}
		opts.Webhook = &server.WebhookOptions{Publisher: pub}
	}
	srv, err := server.New(opts)
	if err != nil {
		return nil, fmt.Errorf("build the server: %w", err)
	}
	return srv, nil
}

// joinedError is the shape of an error built by errors.Join.
type joinedError interface {
	error
	Unwrap() []error
}

// splitJoined returns the errors joined by errors.Join, or err alone.
func splitJoined(err error) []error {
	if joined, ok := errors.AsType[joinedError](err); ok {
		return joined.Unwrap()
	}
	return []error{err}
}
