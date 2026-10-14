package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/isseis/yt2column/internal/config"
	"github.com/isseis/yt2column/internal/job"
	"github.com/isseis/yt2column/internal/llm"
	"github.com/isseis/yt2column/internal/llm/provider"
	"github.com/isseis/yt2column/internal/pipeline"
	"github.com/isseis/yt2column/internal/publisher"
	"github.com/isseis/yt2column/internal/redact"
	"github.com/isseis/yt2column/internal/secret"
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
	newLLMClient       func(config.LLMConfig) (llm.LLMClient, error)
	newReviewLLMClient func(config.LLMConfig) (llm.LLMClient, error)
	newWriter          func(llm.LLMClient, writer.Options) (writer.ArticleWriter, error)
	newFilePublisher   func(path string) (publisher.Publisher, error)
	newSlackPublisher  func(webhookURL secret.Secret) (publisher.Publisher, error)
	newReviewLogWriter func(path string) (*publisher.ReviewLogWriter, error)
}

// productionDeps returns the real constructors. main and the integration test
// both use it, so the test runs the same assembly as the CLI.
func productionDeps() deps {
	return deps{
		newLLMClient:       provider.New,
		newReviewLLMClient: provider.NewReview,
		newWriter:          writer.New,
		newFilePublisher:   newFilePublisher,
		newSlackPublisher:  newSlackPublisher,
		newReviewLogWriter: publisher.NewReviewLogWriter,
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
	// Review-log rejections, in the order they are checked.
	errReviewLogNoReview = errors.New("--review-log cannot be used with --no-review")
	errReviewLogEmpty    = errors.New("--review-log needs a path")
	errReviewLogSameFile = errors.New("--out and --review-log cannot name the same file")
)

// cliOptions holds the parsed flags.
type cliOptions struct {
	out                 string
	slack               bool
	refresh             bool
	keepCache           bool
	systemPrompt        string
	userPrompt          string
	noShorten           bool
	noReview            bool
	reviewLog           string
	shortenSystemPrompt string
	shortenUserPrompt   string
	reviewSystemPrompt  string
	reviewUserPrompt    string
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
	fs.BoolVar(&opts.noShorten, "no-shorten", false, "do not shorten an over-long body, and do not check the final length")
	fs.BoolVar(&opts.noReview, "no-review", false, "do not review the article against the source material")
	fs.StringVar(&opts.reviewLog, "review-log", "", "write the review step's list of changes to this `path`; requires the review step")
	fs.StringVar(&opts.shortenSystemPrompt, "shorten-system-prompt", "", "read the shorten system prompt template from this `path` instead of the built-in one")
	fs.StringVar(&opts.shortenUserPrompt, "shorten-user-prompt", "", "read the shorten user prompt template from this `path` instead of the built-in one")
	fs.StringVar(&opts.reviewSystemPrompt, "review-system-prompt", "", "read the review system prompt template from this `path` instead of the built-in one")
	fs.StringVar(&opts.reviewUserPrompt, "review-user-prompt", "", "read the review user prompt template from this `path` instead of the built-in one")
	return fs, opts
}

// flagGiven reports whether name was set on the command line.
func flagGiven(fs *flag.FlagSet, name string) bool {
	given := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == name {
			given = true
		}
	})
	return given
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
	_, _ = io.WriteString(s.w, redact.Line(fmt.Sprintf(format, args...), s.secrets...)+"\n")
}

// run executes one CLI invocation and returns the process exit code. Steps A1
// to A5 have no side effect: they parse and validate the arguments and the
// configuration and build the stages, so a usage or configuration error exits
// with exitUsage before the cache directory, yt-dlp, the LLM API, or the
// webhook is touched. Only job.Run (step B) acts, and its failure exits with
// exitFailure. Standard output only ever receives the -h/--help usage.
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
	reviewLogGiven := flagGiven(fs, "review-log")
	if err := checkReviewLog(reviewLogGiven, opts, dest); err != nil {
		return usageError(errOut, "%v", err)
	}

	// A3: the configuration. Each rejected variable is reported on its own
	// line; a VarError never carries a value.
	cfg, err := config.Load(lookup)
	if err != nil {
		return configurationError(errOut, err)
	}
	webhookURL, _ := cfg.SlackWebhookURL()
	errOut.secrets = redact.SecretStrings(webhookURL, cfg.LLM().DeepSeekAPIKey(), cfg.LLM().AnthropicAPIKey())
	if dest == destinationWebhook {
		if webhookURL, err = cfg.RequireSlackWebhookURL(); err != nil {
			return configurationError(errOut, err)
		}
	}
	warnHTTP2Debug(errOut, cfg, dest)

	// A4: a file output must not be inside the cache directory, where pruning
	// or removing the cache could delete it. The review log is checked the same
	// way.
	if dest == destinationFile && outPathInsideCacheDir(opts.out, cfg.CacheDir()) {
		return usageError(errOut, "--out %s is inside the cache directory %s; choose a path outside it", opts.out, cfg.CacheDir())
	}
	if reviewLogGiven && outPathInsideCacheDir(opts.reviewLog, cfg.CacheDir()) {
		return usageError(errOut, "--review-log %s is inside the cache directory %s; choose a path outside it", opts.reviewLog, cfg.CacheDir())
	}

	// A5: build the stages. writer.New only reads the template files, and
	// neither the publisher constructors nor the review log writer touches its
	// destination.
	articleWriter, reviewLog, err := buildWriter(d, cfg, opts, reviewLogGiven)
	if err != nil {
		return usageError(errOut, "%v", err)
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
		ReviewLog: reviewLog,
	})
	for _, warning := range result.Warnings {
		errOut.line("%s: warning: %v", programName, warning)
	}
	if err != nil {
		// A review-log failure happens after the article was published, so the
		// summary is written first and reportRunError reports the failure on
		// its own, without the "the run failed:" line.
		if _, isReviewLog := errors.AsType[*job.ReviewLogError](err); isReviewLog {
			reportSuccess(errOut, dest, opts.out, "", result.Article)
		}
		reportRunError(errOut, dest, opts.out, err)
		return exitFailure
	}

	// C: the summary.
	reportSuccess(errOut, dest, opts.out, opts.reviewLog, result.Article)
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

// checkReviewLog rejects the --review-log combinations that cannot be used:
// with --no-review, with an empty path, and naming the same file as --out.
// reviewLogGiven is whether --review-log was set on the command line, never
// read from its value, so the choice cannot depend on what a value happens to
// hold. The same-file rule is textual: a different alias of the same file is
// caught only when link(2) fails after the publish.
func checkReviewLog(reviewLogGiven bool, opts *cliOptions, dest destination) error {
	if !reviewLogGiven {
		return nil
	}
	switch {
	case opts.noReview:
		return errReviewLogNoReview
	case opts.reviewLog == "":
		return errReviewLogEmpty
	case dest == destinationFile && filepath.Clean(opts.reviewLog) == filepath.Clean(opts.out):
		return errReviewLogSameFile
	default:
		return nil
	}
}

// buildWriter builds the ArticleWriter for the requested steps and, when
// reviewLogGiven is set, the review log writer. The review LLM client is built
// only when the review step runs, so --no-review never builds one.
func buildWriter(d deps, cfg config.Config, opts *cliOptions, reviewLogGiven bool) (writer.ArticleWriter, *publisher.ReviewLogWriter, error) {
	client, err := d.newLLMClient(cfg.LLM())
	if err != nil {
		return nil, nil, fmt.Errorf("build the LLM client: %w", err)
	}
	var reviewClient llm.LLMClient
	if !opts.noReview {
		if reviewClient, err = d.newReviewLLMClient(cfg.LLM()); err != nil {
			return nil, nil, fmt.Errorf("build the review LLM client: %w", err)
		}
	}
	articleWriter, err := d.newWriter(client, writer.Options{
		SystemTemplatePath: opts.systemPrompt,
		UserTemplatePath:   opts.userPrompt,
		Shorten: writer.ShortenOptions{
			Enabled:            !opts.noShorten,
			SystemTemplatePath: opts.shortenSystemPrompt,
			UserTemplatePath:   opts.shortenUserPrompt,
		},
		Review: writer.ReviewOptions{
			Enabled:            !opts.noReview,
			Client:             reviewClient,
			SystemTemplatePath: opts.reviewSystemPrompt,
			UserTemplatePath:   opts.reviewUserPrompt,
		},
	})
	if err != nil {
		return nil, nil, fmt.Errorf("build the article writer: %w", err)
	}
	var reviewLog *publisher.ReviewLogWriter
	if reviewLogGiven {
		if reviewLog, err = d.newReviewLogWriter(opts.reviewLog); err != nil {
			return nil, nil, fmt.Errorf("build the review log writer: %w", err)
		}
	}
	return articleWriter, reviewLog, nil
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
// bypasses redact.Line: it may write the API key, and with a webhook
// destination the Webhook URL path as well.
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
// fails, which only a fake publisher that accepts any article can cause. It
// adds one line per post-generation step that ran, and the review log path
// when one was written. A reviewLogPath that is empty writes nothing.
func reportSuccess(errOut *stderrWriter, dest destination, outPath, reviewLogPath string, article writer.Article) {
	modelVersion := orNone(article.ModelVersion)
	switch dest {
	case destinationFile:
		errOut.line("%s: wrote the article to %s (model: %s, model version: %s)", programName, outPath, article.Model, modelVersion)
	case destinationWebhook:
		count := "unknown messages"
		if n, err := publisher.SlackMessageCount(article); err == nil {
			count = messageCount(n)
		}
		errOut.line("%s: posted the article to the webhook in %s (model: %s, model version: %s)", programName, count, article.Model, modelVersion)
	default:
		// Unreachable: buildOutput rejects an invalid destination before
		// anything is published. Claim no destination rather than guess one.
		errOut.line("%s: published the article (model: %s, model version: %s)", programName, article.Model, modelVersion)
	}
	reportStepSummary(errOut, article)
	if reviewLogPath != "" {
		errOut.line("%s: wrote the review log to %s", programName, reviewLogPath)
	}
}

// reportStepSummary writes one line per post-generation step that ran. The
// shorten line names its model; the review line gives the revision count and
// the count of each reason kind.
func reportStepSummary(errOut *stderrWriter, article writer.Article) {
	if article.Shorten != nil {
		errOut.line("%s: the shorten step ran (model: %s, model version: %s)",
			programName, article.Shorten.Model, orNone(article.Shorten.ModelVersion))
	}
	if article.Review != nil {
		errOut.line("%s: the review step returned %s", programName, revisionCounts(article.Review.Revisions))
	}
}

// revisionCounts renders the revision count and the count of each reason kind
// in one line, listing only the kinds with a count, in enum order.
func revisionCounts(revisions []writer.Revision) string {
	if len(revisions) == 0 {
		return "no revisions"
	}
	counts := make(map[writer.RevisionReason]int)
	for _, revision := range revisions {
		counts[revision.Reason]++
	}
	parts := make([]string, 0, len(counts))
	for _, reason := range []writer.RevisionReason{
		writer.ReasonUnsupported,
		writer.ReasonMeaningChange,
		writer.ReasonSpeaker,
		writer.ReasonMisrecognition,
		writer.ReasonUnintelligible,
	} {
		if n := counts[reason]; n > 0 {
			value, _ := reason.WireValue()
			parts = append(parts, fmt.Sprintf("%s %d", value, n))
		}
	}
	noun := "revisions"
	if len(revisions) == 1 {
		noun = "revision"
	}
	return fmt.Sprintf("%d %s (%s)", len(revisions), noun, strings.Join(parts, ", "))
}

// orNone renders an empty model version as (none), as the published output
// does.
func orNone(version string) string {
	if version == "" {
		return "(none)"
	}
	return version
}

// messageCount renders n as a count of posted messages, using the singular
// noun for one.
func messageCount(n int) string {
	if n == 1 {
		return "1 message"
	}
	return strconv.Itoa(n) + " messages"
}

// usageError reports a usage or configuration error and returns exitUsage.
func usageError(errOut *stderrWriter, format string, args ...any) int {
	errOut.line("%s: %s", programName, fmt.Sprintf(format, args...))
	errOut.line("%s", usageHint)
	return exitUsage
}

// reportRunError reports a job.Run failure, naming the pipeline stage when
// there is one (a review log failure after the publish is reported first, on
// its own), and adds the hint that matches the failure. A deadline is
// attributed to the yt-dlp, LLM, or webhook post timeout by the failed stage;
// this holds because the context run receives has no deadline of its own.
func reportRunError(errOut *stderrWriter, dest destination, outPath string, err error) {
	if reviewLogErr, ok := errors.AsType[*job.ReviewLogError](err); ok {
		reportReviewLogFailure(errOut, dest, outPath, reviewLogErr)
		return
	}
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
	reportStepHints(errOut, err)
	reportWebhookHints(errOut, err)
}

// reportReviewLogFailure reports a review-log write failure that happened
// after the article was published. It never says the run failed: the article
// is complete, and running again would publish a new one.
func reportReviewLogFailure(errOut *stderrWriter, dest destination, outPath string, reviewLogErr *job.ReviewLogError) {
	article := "written to " + outPath
	if dest == destinationWebhook {
		article = "posted to the webhook"
	}
	errOut.line("%s: the article was %s, but writing the review log to %s failed: %v",
		programName, article, reviewLogErr.Path, reviewLogErr.Err)
	errOut.line("%s: do not run again: the article is already published", programName)
}

// reportStepHints adds the hints for the post-generation step failures: an
// over-long body suggests --no-shorten, and a rejected review response
// suggests --no-review. The step is read from the error's type, never from its
// message.
func reportStepHints(errOut *stderrWriter, err error) {
	if errors.Is(err, writer.ErrTooLong) {
		errOut.line("%s: --no-shorten skips the shorten step and the final length check, so the article is published even if it is over the limit", programName)
	}
	if stepErr, ok := errors.AsType[*writer.StepError](err); ok && stepErr.Step == writer.StepReview && errors.Is(err, writer.ErrMalformedOutput) {
		errOut.line("%s: --no-review skips the review step, so the article is published as generated", programName)
	}
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
