package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/isseis/yt2column/internal/config"
	"github.com/isseis/yt2column/internal/job"
	"github.com/isseis/yt2column/internal/llm"
	"github.com/isseis/yt2column/internal/llm/provider"
	"github.com/isseis/yt2column/internal/pipeline"
	"github.com/isseis/yt2column/internal/publisher"
	"github.com/isseis/yt2column/internal/secret"
	"github.com/isseis/yt2column/internal/slackwebhook"
	"github.com/isseis/yt2column/internal/transcript"
	"github.com/isseis/yt2column/internal/writer"
)

// Exit codes. The code is chosen by the step that failed, never by inspecting
// an error message: a failure before any side effect is a usage or
// configuration error, a failure returned by job.Run is a run failure.
const (
	exitOK      = 0
	exitFailure = 1
	exitUsage   = 2
)

// programName prefixes every line written to standard error.
const programName = "yt2column"

// usageHint ends every usage or configuration error.
const usageHint = programName + ": run '" + programName + " -h' for usage"

// deps are the constructors run uses. main passes productionDeps(); a unit test
// substitutes fakes.
type deps struct {
	newLLMClient      func(config.Config) (llm.LLMClient, error)
	newWriter         func(llm.LLMClient, writer.Options) (writer.ArticleWriter, error)
	newFilePublisher  func(path string) (publisher.Publisher, error)
	newSlackPublisher func(webhookURL secret.Secret) (publisher.Publisher, error)
}

// productionDeps returns the real constructors. main and the integration test
// both use it, so the test runs the same assembly as the CLI.
func productionDeps() deps {
	return deps{
		newLLMClient:      provider.New,
		newWriter:         writer.New,
		newFilePublisher:  newFilePublisher,
		newSlackPublisher: newSlackPublisher,
	}
}

// newFilePublisher builds the FilePublisher for path. It returns a nil
// interface, never a typed nil, when construction fails, so a caller checking
// the interface against nil is not misled.
func newFilePublisher(path string) (publisher.Publisher, error) {
	p, err := publisher.NewFilePublisher(path)
	if err != nil {
		return nil, err
	}
	return p, nil
}

// newSlackPublisher builds the SlackWebhookPublisher for webhookURL. Like
// newFilePublisher, it returns a nil interface, never a typed nil, when
// construction fails.
func newSlackPublisher(webhookURL secret.Secret) (publisher.Publisher, error) {
	p, err := publisher.NewSlackWebhookPublisher(webhookURL)
	if err != nil {
		return nil, err
	}
	return p, nil
}

// destination is where run sends the article. The zero value is invalid, so a
// destination that was never chosen cannot be acted on.
type destination int

const (
	destinationInvalid destination = iota
	destinationFile
	destinationWebhook
)

// Rejections of the --out and --slack combination, in the order they are
// checked.
var (
	errSlackFalse         = errors.New("--slack=false is not accepted; omit --slack instead")
	errOutEmpty           = errors.New("--out needs a path")
	errOutAndSlack        = errors.New("--out and --slack cannot be used together")
	errNoDestination      = errors.New("one of --out or --slack is required")
	errInvalidDestination = errors.New("no output destination was chosen")
)

// cliOptions holds the parsed flags.
type cliOptions struct {
	out          string
	slack        bool
	refresh      bool
	keepCache    bool
	systemPrompt string
	userPrompt   string
}

// newFlagSet defines every flag in one place, so run and the documentation
// test read the same definitions. Its output is discarded: run writes the
// usage to standard output and parse errors to standard error itself.
func newFlagSet() (*flag.FlagSet, *cliOptions) {
	fs := flag.NewFlagSet(programName, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	opts := &cliOptions{}
	fs.StringVar(&opts.out, "out", "", "write the article to this `path`; it must not exist, its directory must exist, and it must be outside the cache directory")
	fs.BoolVar(&opts.slack, "slack", false, "post the article to the Slack-compatible Incoming Webhook (e.g. Mattermost) configured in the environment (see README)")
	fs.BoolVar(&opts.refresh, "refresh", false, "ignore the cached transcript, run yt-dlp again, and replace the cache")
	fs.BoolVar(&opts.keepCache, "keep-cache", false, "keep this video's cache after a successful run")
	fs.StringVar(&opts.systemPrompt, "system-prompt", "", "read the system prompt template from this `path` instead of the built-in one")
	fs.StringVar(&opts.userPrompt, "user-prompt", "", "read the user prompt template from this `path` instead of the built-in one")
	return fs, opts
}

// writeUsage writes the usage text for fs to w.
func writeUsage(w io.Writer, fs *flag.FlagSet) {
	var b strings.Builder
	fmt.Fprintf(&b, "Usage: %s [flags] <video URL>\n\n", programName)
	b.WriteString("Generates a column article from a YouTube video's subtitles and either writes it to the --out file\n")
	b.WriteString("or posts it to a webhook with --slack; exactly one of the two is required.\n")
	b.WriteString("Flags must come before the video URL. The configuration is read from environment variables; see README.md.\n\n")
	b.WriteString("Flags:\n")
	fs.VisitAll(func(f *flag.Flag) {
		name, usage := flag.UnquoteUsage(f)
		if name != "" {
			fmt.Fprintf(&b, "  --%s <%s>\n", f.Name, name)
		} else {
			fmt.Fprintf(&b, "  --%s\n", f.Name)
		}
		fmt.Fprintf(&b, "        %s\n", usage)
	})
	b.WriteString("  -h, --help\n        show this help and exit\n")
	_, _ = io.WriteString(w, b.String())
}

// stderrWriter writes sanitized lines to standard error. Every line the CLI
// writes there goes through line, which escapes non-printable characters and
// redacts the secrets known so far.
type stderrWriter struct {
	w       io.Writer
	secrets []string
}

// line writes one sanitized line.
func (s *stderrWriter) line(format string, args ...any) {
	_, _ = io.WriteString(s.w, sanitize(fmt.Sprintf(format, args...), s.secrets...)+"\n")
}

// run executes one CLI invocation and returns the process exit code. Steps A1
// to A5 have no side effect: they parse and validate the arguments and the
// configuration and build the stages, so a usage or configuration error exits
// with exitUsage before the cache directory, yt-dlp, the LLM API, or the
// webhook is touched. Only job.Run (step B) acts, and its failure exits with exitFailure. Standard
// output only ever receives the -h/--help usage.
func run(ctx context.Context, args []string, lookup config.LookupFunc, stdout, stderr io.Writer, d deps) int {
	errOut := &stderrWriter{w: stderr}

	// A1: parse the flags.
	fs, opts := newFlagSet()
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			writeUsage(stdout, fs)
			return exitOK
		}
		return usageError(errOut, "%v", err)
	}

	// A2: exactly one video URL, in a form yt-dlp would accept, and the
	// destination.
	if fs.NArg() != 1 {
		return usageError(errOut, "expected exactly one video URL after the flags, got %d arguments", fs.NArg())
	}
	_, videoURL, err := transcript.ValidateVideoURL(fs.Arg(0))
	if err != nil {
		return usageError(errOut, "%v", err)
	}
	dest, err := chooseDestination(fs, opts)
	if err != nil {
		return usageError(errOut, "%v", err)
	}

	// A3: the configuration. Each rejected variable is reported on its own
	// line; a VarError never carries a value.
	cfg, err := config.Load(lookup)
	if err != nil {
		return configurationError(errOut, err)
	}
	errOut.secrets = configuredSecrets(cfg)
	var webhookURL secret.Secret
	if dest == destinationWebhook {
		if webhookURL, err = cfg.RequireSlackWebhookURL(); err != nil {
			return configurationError(errOut, err)
		}
	}
	warnHTTP2Debug(errOut, cfg, dest)

	// A4: a file output must not be inside the cache directory, where pruning
	// or removing the cache could delete it.
	if dest == destinationFile && outPathInsideCacheDir(opts.out, cfg.CacheDir()) {
		return usageError(errOut, "--out %s is inside the cache directory %s; choose a path outside it", opts.out, cfg.CacheDir())
	}

	// A5: build the stages. writer.New only reads the template files, and
	// neither publisher constructor touches its destination.
	client, err := d.newLLMClient(cfg)
	if err != nil {
		return usageError(errOut, "build the LLM client: %v", err)
	}
	articleWriter, err := d.newWriter(client, writer.Options{
		SystemTemplatePath: opts.systemPrompt,
		UserTemplatePath:   opts.userPrompt,
	})
	if err != nil {
		return usageError(errOut, "build the article writer: %v", err)
	}
	output, err := buildOutput(d, dest, opts.out, webhookURL)
	if err != nil {
		return usageError(errOut, "build the publisher: %v", err)
	}

	// B: run the job. Warnings are reported whether or not it succeeded.
	result, err := job.Run(ctx, job.Request{
		VideoURL:  videoURL,
		CacheDir:  cfg.CacheDir(),
		YtDlpPath: cfg.YtDlpPath(),
		Refresh:   opts.refresh,
		KeepCache: opts.keepCache,
		Writer:    articleWriter,
		Output:    output,
	})
	for _, warning := range result.Warnings {
		errOut.line("%s: warning: %v", programName, warning)
	}
	if err != nil {
		reportRunError(errOut, err)
		return exitFailure
	}

	// C: the summary.
	reportSuccess(errOut, dest, opts.out, result.Article)
	return exitOK
}

// chooseDestination picks the destination from which of --out and --slack
// appear on the command line, never from their values, so the choice cannot
// depend on what a value happens to hold. A rejected combination returns its
// error; when several apply, the first in this order wins: --slack=false, an
// empty --out, both flags, neither flag.
func chooseDestination(fs *flag.FlagSet, opts *cliOptions) (destination, error) {
	var outGiven, slackGiven bool
	fs.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "out":
			outGiven = true
		case "slack":
			slackGiven = true
		}
	})
	switch {
	case slackGiven && !opts.slack:
		return destinationInvalid, errSlackFalse
	case outGiven && opts.out == "":
		return destinationInvalid, errOutEmpty
	case outGiven && slackGiven:
		return destinationInvalid, errOutAndSlack
	case outGiven:
		return destinationFile, nil
	case slackGiven:
		return destinationWebhook, nil
	default:
		return destinationInvalid, errNoDestination
	}
}

// configurationError reports each configuration error joined in err on its
// own line and returns exitUsage.
func configurationError(errOut *stderrWriter, err error) int {
	for _, e := range splitJoined(err) {
		errOut.line("%s: configuration: %v", programName, e)
	}
	errOut.line("%s", usageHint)
	return exitUsage
}

// warnHTTP2Debug warns, before any LLM call or post, that the Go HTTP/2 log
// bypasses sanitize: it may write the API key, and with a webhook destination
// the Webhook URL path as well.
func warnHTTP2Debug(errOut *stderrWriter, cfg config.Config, dest destination) {
	if !cfg.HTTP2DebugEnabled() {
		return
	}
	errOut.line("%s: warning: GODEBUG enables http2debug, so the Go HTTP/2 log may write the API key to standard error", programName)
	if dest == destinationWebhook {
		errOut.line("%s: warning: GODEBUG enables http2debug, so the Go HTTP/2 log may write the Webhook URL path to standard error", programName)
	}
}

// buildOutput builds the publisher for dest and the job output around it. An
// invalid destination is rejected rather than defaulted to either kind.
func buildOutput(d deps, dest destination, outPath string, webhookURL secret.Secret) (job.Output, error) {
	switch dest {
	case destinationFile:
		pub, err := d.newFilePublisher(outPath)
		if err != nil {
			return job.Output{}, err
		}
		return job.FileOutput(outPath, pub), nil
	case destinationWebhook:
		pub, err := d.newSlackPublisher(webhookURL)
		if err != nil {
			return job.Output{}, err
		}
		return job.RemoteOutput(pub), nil
	default:
		return job.Output{}, errInvalidDestination
	}
}

// reportSuccess writes the summary of a published run. The message count of a
// webhook post is recomputed from the article; it is "unknown" when that
// fails, which only a fake publisher that accepts any article can cause.
func reportSuccess(errOut *stderrWriter, dest destination, outPath string, article writer.Article) {
	modelVersion := article.ModelVersion
	if modelVersion == "" {
		modelVersion = "(none)"
	}
	if dest == destinationWebhook {
		count := "unknown"
		if n, err := publisher.SlackMessageCount(article); err == nil {
			count = strconv.Itoa(n)
		}
		errOut.line("%s: posted the article to the webhook in %s messages (model: %s, model version: %s)", programName, count, article.Model, modelVersion)
		return
	}
	errOut.line("%s: wrote the article to %s (model: %s, model version: %s)", programName, outPath, article.Model, modelVersion)
}

// usageError reports a usage or configuration error and returns exitUsage.
func usageError(errOut *stderrWriter, format string, args ...any) int {
	errOut.line("%s: %s", programName, fmt.Sprintf(format, args...))
	errOut.line("%s", usageHint)
	return exitUsage
}

// reportRunError reports a job.Run failure, naming the pipeline stage when
// there is one, and adds the hint that matches the failure. A deadline is
// attributed to the yt-dlp or LLM timeout by the failed stage; this holds
// because the context run receives has no deadline of its own.
func reportRunError(errOut *stderrWriter, err error) {
	stageErr, isStage := errors.AsType[*pipeline.StageError](err)
	if isStage {
		errOut.line("%s: the %s stage failed: %v", programName, stageErr.Stage, stageErr.Err)
	} else {
		errOut.line("%s: the run failed: %v", programName, err)
	}

	switch {
	case errors.Is(err, context.DeadlineExceeded) && isStage && stageErr.Stage == pipeline.StageTranscript:
		errOut.line("%s: yt-dlp timed out after the %s limit", programName, minutes(job.YtDlpTimeout))
	case errors.Is(err, context.DeadlineExceeded) && isStage && stageErr.Stage == pipeline.StageWrite:
		errOut.line("%s: the LLM call timed out after the %s limit", programName, minutes(provider.LLMTimeout))
	case errors.Is(err, context.Canceled):
		errOut.line("%s: the run was interrupted; the video's cache was not removed", programName)
	}
	if errors.Is(err, transcript.ErrParseSubtitles) || errors.Is(err, transcript.ErrParseInfo) {
		errOut.line("%s: the cached transcript is invalid; run again with --refresh to fetch it again", programName)
	}
	if kept, ok := errors.AsType[*publisher.KeptFileError](err); ok {
		errOut.line("%s: the finished article was kept in %s", programName, kept.TempPath)
	}
	reportWebhookHints(errOut, err)
}

// reportWebhookHints adds the hints for a failed webhook post: which messages
// stay in the channel, what a rejecting status may mean, a post timeout, and
// a rejected mention. A failure of any other publisher matches none of them.
func reportWebhookHints(errOut *stderrWriter, err error) {
	if postErr, ok := errors.AsType[*publisher.SlackPostError](err); ok && postErr.Posted >= 1 {
		const rerun = "running again generates a new article and posts all of it from the first message"
		if postErr.Attempted {
			errOut.line("%s: at least %d of %d messages stay in the channel (message %d may also have been posted); %s",
				programName, postErr.Posted, postErr.Total, postErr.Posted+1, rerun)
		} else {
			errOut.line("%s: %d of %d messages stay in the channel; %s", programName, postErr.Posted, postErr.Total, rerun)
		}
	}
	if statusErr, ok := errors.AsType[*publisher.SlackHTTPStatusError](err); ok && rejectedPostStatus(statusErr.StatusCode) {
		hint := "the server rejected the post; Mattermost does not report which of these it was: " +
			"the Incoming Webhook was deleted or disabled, its channel was deleted, locked, or does not accept posts, " +
			"or the payload was invalid. Check the webhook settings and the server log"
		if statusErr.RequestID != "" {
			hint += " (request ID: " + statusErr.RequestID + ")"
		}
		errOut.line("%s: %s", programName, hint)
	}
	if stageErr, ok := errors.AsType[*pipeline.StageError](err); ok && stageErr.Stage == pipeline.StagePublish &&
		errors.Is(err, context.DeadlineExceeded) {
		errOut.line("%s: the webhook post timed out after the %s limit", programName, seconds(publisher.SlackPostTimeout))
	}
	if errors.Is(err, publisher.ErrSlackMention) {
		errOut.line("%s: the article was not posted and was discarded; part of it could be treated as a mention or rewritten by the server. "+
			"Run again to generate a new article, or use --out to inspect what is generated", programName)
	}
}

// rejectedPostStatus reports whether Mattermost answers a webhook post with
// status for a reason it does not name: a deleted, disabled, or unusable
// webhook or channel, or an invalid payload.
func rejectedPostStatus(status int) bool {
	switch status {
	case http.StatusBadRequest, http.StatusForbidden, http.StatusNotFound, http.StatusNotImplemented:
		return true
	default:
		return false
	}
}

// minutes renders d as a whole number of minutes.
func minutes(d time.Duration) string {
	return fmt.Sprintf("%d-minute", int(d/time.Minute))
}

// seconds renders d as a whole number of seconds.
func seconds(d time.Duration) string {
	return fmt.Sprintf("%d-second", int(d/time.Second))
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

// configuredSecrets returns the secret values in cfg, so every later line is
// redacted. An unset secret contributes nothing. The Webhook URL also
// contributes its slackwebhook.SensitiveParts, so a fragment such as its path
// alone is redacted too, not only its last characters.
func configuredSecrets(cfg config.Config) []string {
	var secrets []string
	if key, err := cfg.DeepSeekAPIKey().Reveal(); err == nil {
		secrets = append(secrets, key)
	}
	if url, ok := cfg.SlackWebhookURL(); ok {
		if value, err := url.Reveal(); err == nil {
			secrets = append(secrets, value)
			secrets = append(secrets, slackwebhook.SensitiveParts(value)...)
		}
	}
	return secrets
}
