package main

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"sync"
	"time"

	"github.com/isseis/yt2column/internal/authtoken"
	"github.com/isseis/yt2column/internal/config"
	"github.com/isseis/yt2column/internal/failure"
	"github.com/isseis/yt2column/internal/llm"
	"github.com/isseis/yt2column/internal/llm/provider"
	"github.com/isseis/yt2column/internal/publisher"
	"github.com/isseis/yt2column/internal/redact"
	"github.com/isseis/yt2column/internal/secret"
	"github.com/isseis/yt2column/internal/server"
	"github.com/isseis/yt2column/internal/writer"
)

// Exit codes. The code is chosen by the step that failed, never by inspecting
// an error message: a failure before the server listens (the arguments, the
// configuration, the token file, or building a component) is exitUsage, and a
// failure to listen or to keep serving is exitFailure. A stop by a signal is
// exitOK.
const (
	exitOK      = 0
	exitFailure = 1
	exitUsage   = 2
)

// programName prefixes every line written to standard error before the server
// logger takes over.
const programName = "yt2column-server"

// llmCallsPerJob is the most LLM calls one job makes: generation, shorten, and
// review. With provider.LLMTimeout per call it bounds how long a stop can wait
// for an in-flight job.
const llmCallsPerJob = 3

// deps are the constructors run substitutes in a test: the two LLM clients,
// the article publisher, and the failure notifier, the only components that
// reach the network, and the writer, so a test can observe the options it is
// built with. main passes productionDeps().
type deps struct {
	newLLMClient       func(config.LLMConfig) (llm.LLMClient, error)
	newReviewLLMClient func(config.LLMConfig) (llm.LLMClient, error)
	newWriter          func(llm.LLMClient, writer.Options) (writer.ArticleWriter, error)
	newPublisher       func(webhookURL secret.Secret) (publisher.Publisher, error)
	newNotifier        func(webhookURL secret.Secret) (failure.Notifier, error)
}

// productionDeps returns the real constructors.
func productionDeps() deps {
	return deps{
		newLLMClient:       provider.New,
		newReviewLLMClient: provider.NewReview,
		newWriter:          writer.New,
		newPublisher:       newPublisher,
		newNotifier:        newNotifier,
	}
}

// newPublisher builds the SlackWebhookPublisher, the publisher the CLI's
// --slack uses. It returns a nil interface, never a typed nil, when
// construction fails, so server.New's nil check is not misled.
func newPublisher(webhookURL secret.Secret) (publisher.Publisher, error) {
	p, err := publisher.NewSlackWebhookPublisher(webhookURL)
	if err != nil {
		return nil, err
	}
	return p, nil
}

// newNotifier builds the SlackFailureNotifier, which posts to the same webhook
// as the articles. Like newPublisher, it never returns a typed nil.
func newNotifier(webhookURL secret.Secret) (failure.Notifier, error) {
	n, err := publisher.NewSlackFailureNotifier(webhookURL)
	if err != nil {
		return nil, err
	}
	return n, nil
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
// process exit code. Steps 1 to 3 touch no network and change no file other
// than creating the token file when it is absent. A failure there stops the
// start with exitUsage before the server listens, except a failure to display
// the token, which is written to standard error and does not stop the start.
// Standard output receives the shared token on every start; everything else
// goes to standard error.
func run(ctx context.Context, args []string, lookup config.LookupFunc, stdout, stderr io.Writer, d deps) int {
	// run's own logger and the server's write to stderr from different
	// goroutines, so each write is serialized here rather than relying on
	// what the writer tolerates.
	stderr = &lockedWriter{w: stderr}
	errOut := &stderrWriter{w: stderr}
	if len(args) != 0 {
		errOut.line("%s: takes no arguments; the configuration is read from environment variables (see README.md)", programName)
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
	errOut.secrets = redact.SecretStrings(cfg.WebhookURL(), llmCfg.DeepSeekAPIKey(), llmCfg.AnthropicAPIKey())
	if cfg.HTTP2DebugEnabled() {
		errOut.line("%s: warning: GODEBUG enables http2debug, so the Go HTTP/2 log may write the API key and the Webhook URL path to standard error", programName)
	}

	// 2: the shared token, displayed on every start before listening, so a
	// start whose listen then fails still shows the token.
	token, err := authtoken.LoadOrCreate(cfg.TokenPath(), rand.Reader)
	if err != nil {
		if _, ok := errors.AsType[*authtoken.FileError](err); ok {
			errOut.line("%s: shared token: %v; fix the file, or delete it and restart to create a new token", programName, err)
		} else {
			errOut.line("%s: shared token: %v", programName, err)
		}
		return exitUsage
	}
	secrets := redact.SecretStrings(cfg.WebhookURL(), llmCfg.DeepSeekAPIKey(), llmCfg.AnthropicAPIKey(), token.Value())
	errOut.secrets = secrets
	// The display is best effort: a failure to write it to standard output
	// does not stop the start, because the value stays in the token file.
	value, err := token.Value().Reveal()
	if err != nil {
		errOut.line("%s: shared token: the token value cannot be read: %v", programName, err)
		return exitUsage
	}
	if _, err := io.WriteString(stdout, value+"\n"); err != nil {
		errOut.line("%s: shared token: writing the token to standard output failed: %v; read it from %s", programName, err, cfg.TokenPath())
	} else {
		errOut.line("%s: wrote the shared token to standard output; it is shown on every start, so restart the server to see it again; paste it into the extension's options page; the token file is %s", programName, cfg.TokenPath())
	}

	// 3: the components. None of the constructors touches the network.
	srv, err := buildServer(d, cfg, token, stderr, secrets)
	if err != nil {
		errOut.line("%s: %v", programName, err)
		return exitUsage
	}

	// 4: listen on both loopback addresses.
	listeners, err := srv.Listen()
	if err != nil {
		errOut.line("%s: %v", programName, err)
		return exitFailure
	}
	logger := slog.New(slog.NewTextHandler(redact.Writer(stderr, secrets), nil))
	logger.Info("listening",
		"urls", listenURLs(listeners),
		"tokenFile", cfg.TokenPath(),
		"model", llmCfg.Model(),
		"reviewModel", llmCfg.ReviewModel(),
	)

	// 5: serve until ctx is canceled. The stop notice is logged as soon as
	// the cancellation arrives, not after the wait it announces.
	noticeDone := make(chan struct{})
	stopNotice := context.AfterFunc(ctx, func() {
		defer close(noticeDone)
		logger.Info(fmt.Sprintf("stopping: in-flight jobs and failure notices are allowed to finish, which can take up to about %d minutes; send the signal again to exit at once, which drops them",
			int(llmCallsPerJob*provider.LLMTimeout/time.Minute)))
	})
	err = srv.Serve(ctx, listeners)
	if !stopNotice() {
		<-noticeDone
	}
	if err != nil {
		logger.Error("serving failed", "error", err)
		return exitFailure
	}
	logger.Info("stopped")
	return exitOK
}

// buildServer builds the components and the server. logOutput and secrets
// become the server's redacted log.
func buildServer(d deps, cfg config.ServerConfig, token authtoken.Token, logOutput io.Writer, secrets []string) (*server.Server, error) {
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
	pub, err := d.newPublisher(cfg.WebhookURL())
	if err != nil {
		return nil, fmt.Errorf("build the publisher: %w", err)
	}
	notifier, err := d.newNotifier(cfg.WebhookURL())
	if err != nil {
		return nil, fmt.Errorf("build the failure notifier: %w", err)
	}
	srv, err := server.New(server.Options{
		Port:      cfg.Port(),
		Token:     token,
		Writer:    articleWriter,
		Publisher: pub,
		Notifier:  notifier,
		LogOutput: logOutput,
		Secrets:   secrets,
	})
	if err != nil {
		return nil, fmt.Errorf("build the server: %w", err)
	}
	return srv, nil
}

// listenURLs returns the endpoint URL on each listener.
func listenURLs(listeners []net.Listener) []string {
	urls := make([]string, 0, len(listeners))
	for _, l := range listeners {
		urls = append(urls, "http://"+l.Addr().String()+server.ArticlesPath)
	}
	return urls
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
