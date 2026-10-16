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

	"github.com/isseis/yt2column/internal/artifact"
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

// Token-usage line format. The line is defined in the form it appears after
// redact.Line: fixed prefix, the stage, the origin, and the state as
// whitespace-free words, then either the four counts or the reason, then the
// model name as the rest of the line (or nomodel). The stage word is
// writer.Stage.String(); the words here are the only place these spellings are
// defined.
const (
	// tokensPrefix begins every token-usage line.
	tokensPrefix = programName + ": tokens: "
	// Token-usage origin words: a call of this run, or a reused artifact.
	tokensRun    = "run"
	tokensReused = "reused"
	// Token-usage state words.
	tokensKnown   = "known"
	tokensUnknown = "unknown"
	// Token-usage count names, in the line's order.
	tokensInput      = "input="
	tokensCacheRead  = "cache_read="
	tokensCacheWrite = "cache_write="
	tokensOutput     = "output="
	// tokensReason prefixes the unknown reason.
	tokensReason = "reason="
	// tokensModel prefixes the model name; tokensNoModel replaces it when the
	// stage reported no model that passed the model-name rules.
	tokensModel   = "model="
	tokensNoModel = "nomodel"
)

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
	// Artifact and resume rejections, in the order they are checked.
	errArtifactsEmpty       = errors.New("--artifacts needs a path")
	errAfterNoArtifacts     = errors.New("--after requires --artifacts")
	errAfterEmpty           = errors.New("--after needs a stage")
	errAfterInvalid         = errors.New("--after must be generation, shorten, or review")
	errAfterPromptFlags     = errors.New("--after cannot be used with --system-prompt or --user-prompt")
	errAfterReviewReviewLog = errors.New("--after review cannot be used with --review-log")
	// Inside-cache-directory rejections, one per path kind. Each is wrapped
	// with the path and the cache directory.
	errOutInsideCache       = errors.New("--out is inside the cache directory")
	errReviewLogInsideCache = errors.New("--review-log is inside the cache directory")
	errArtifactsInsideCache = errors.New("--artifacts is inside the cache directory")
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
	artifacts           string
	after               string
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
	fs.StringVar(&opts.artifacts, "artifacts", "", "save each stage's output to this `path` directory and reuse it to resume; the directory must exist and be outside the cache directory")
	fs.StringVar(&opts.after, "after", "", "run only the stages after `stage` (generation, shorten, or review), reusing its saved output; requires --artifacts, and --after review does not accept --review-log")
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
	artifactsGiven, resume, need, err := resumeOptions(fs, opts)
	if err != nil {
		return usageError(errOut, "%v", err)
	}

	// A3: the configuration. Each rejected variable is reported on its own
	// line; a VarError never carries a value. Only the stages the run executes
	// contribute their provider's variables.
	cfg, err := config.LoadFor(lookup, need)
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

	// A4: a file output, the review log, and the artifacts directory must not
	// be inside the cache directory, where pruning or removing the cache could
	// delete them.
	if err := checkCacheDirs(opts, dest, reviewLogGiven, artifactsGiven, cfg.CacheDir()); err != nil {
		return usageError(errOut, "%v", err)
	}

	// A4.5: open the artifacts directory once, before yt-dlp and the LLM. A
	// missing or unusable directory is an execution failure (exit 1), not a
	// usage error, because it depends on the filesystem rather than the
	// argument's shape.
	// store stays a nil interface, never a typed nil, without --artifacts,
	// so the writer options of a run without it are the default ones.
	var store writer.ArtifactStore
	if artifactsGiven {
		opened, err := artifact.NewStore(opts.artifacts)
		if err != nil {
			return runFailure(errOut, "open the artifacts directory: %v", err)
		}
		defer func() { _ = opened.Close() }()
		store = opened
	}

	// A5: build the stages. writer.New only reads the template files, and
	// neither the publisher constructors nor the review log writer touches its
	// destination.
	articleWriter, reviewLog, err := buildWriter(d, cfg, opts, reviewLogGiven, store, resume)
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
			reportSuccess(errOut, dest, opts.out, "", opts.artifacts, result.Article)
		}
		reportRunError(errOut, dest, opts.out, err)
		return exitFailure
	}

	// C: the summary.
	reportSuccess(errOut, dest, opts.out, opts.reviewLog, opts.artifacts, result.Article)
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

// afterStage maps an --after value to the writer.Stage it resumes from. The
// accepted values are exactly "generation", "shorten", and "review".
func afterStage(value string) (writer.Stage, bool) {
	switch value {
	case "generation":
		return writer.StageGeneration, true
	case "shorten":
		return writer.StageShorten, true
	case "review":
		return writer.StageReview, true
	default:
		return writer.StageUnset, false
	}
}

// resumeNeed maps the resume and step flags to the configuration the run needs
// (architecture section 3.7): without --after, both stages run; with --after,
// only the stages after the resume stage run, and --no-shorten and --no-review
// still suppress their step. after must already be a validated value.
func resumeNeed(after string, noShorten, noReview, afterGiven bool) config.Need {
	if !afterGiven {
		return config.Need{Generation: true, Review: true}
	}
	switch after {
	case "generation":
		return config.Need{Generation: !noShorten, Review: !noReview}
	case "shorten":
		return config.Need{Generation: false, Review: !noReview}
	default: // "review", already validated
		return config.Need{}
	}
}

// checkResumeFlags rejects the --artifacts and --after combinations that need
// no configuration, reading which flags appeared on the command line rather
// than their values. When several apply, the first in this order wins: an empty
// --artifacts, --after without --artifacts, an empty --after, an invalid
// --after, --after with a generation-only prompt, and --after review with
// --review-log. The --artifacts-inside-the-cache check needs the configuration
// and runs later.
func checkResumeFlags(fs *flag.FlagSet, opts *cliOptions, artifactsGiven, afterGiven bool) error {
	if artifactsGiven && opts.artifacts == "" {
		return errArtifactsEmpty
	}
	if !afterGiven {
		return nil
	}
	switch {
	case !artifactsGiven:
		return errAfterNoArtifacts
	case opts.after == "":
		return errAfterEmpty
	}
	stage, ok := afterStage(opts.after)
	if !ok {
		return fmt.Errorf("%w: %q", errAfterInvalid, opts.after)
	}
	if flagGiven(fs, "system-prompt") || flagGiven(fs, "user-prompt") {
		return errAfterPromptFlags
	}
	if stage == writer.StageReview && flagGiven(fs, "review-log") {
		return errAfterReviewReviewLog
	}
	return nil
}

// resumeOptions reads the --artifacts and --after flags, rejects their invalid
// combinations, and returns whether --artifacts was given, the stage to resume
// from, and the configuration the run needs.
func resumeOptions(fs *flag.FlagSet, opts *cliOptions) (artifactsGiven bool, resume writer.Stage, need config.Need, err error) {
	artifactsGiven = flagGiven(fs, "artifacts")
	afterGiven := flagGiven(fs, "after")
	if err = checkResumeFlags(fs, opts, artifactsGiven, afterGiven); err != nil {
		return false, writer.StageUnset, config.Need{}, err
	}
	if afterGiven {
		resume, _ = afterStage(opts.after)
	}
	return artifactsGiven, resume, resumeNeed(opts.after, opts.noShorten, opts.noReview, afterGiven), nil
}

// checkCacheDirs rejects an --out, --review-log, or --artifacts path inside the
// cache directory, where pruning or removing the cache could delete it.
func checkCacheDirs(opts *cliOptions, dest destination, reviewLogGiven, artifactsGiven bool, cacheDir string) error {
	if dest == destinationFile && outPathInsideCacheDir(opts.out, cacheDir) {
		return fmt.Errorf("%w %s; choose a path outside %s", errOutInsideCache, opts.out, cacheDir)
	}
	if reviewLogGiven && outPathInsideCacheDir(opts.reviewLog, cacheDir) {
		return fmt.Errorf("%w %s; choose a path outside %s", errReviewLogInsideCache, opts.reviewLog, cacheDir)
	}
	if artifactsGiven && outPathInsideCacheDir(opts.artifacts, cacheDir) {
		return fmt.Errorf("%w %s; choose a path outside %s", errArtifactsInsideCache, opts.artifacts, cacheDir)
	}
	return nil
}

// runFailure reports a failure that is not a usage or configuration error but
// happens before job.Run (an unusable artifacts directory) and returns
// exitFailure.
func runFailure(errOut *stderrWriter, format string, args ...any) int {
	errOut.line("%s: the run failed: %s", programName, fmt.Sprintf(format, args...))
	return exitFailure
}

// buildWriter builds the ArticleWriter for the requested steps and, when
// reviewLogGiven is set, the review log writer. It builds the generation LLM
// client only when a stage that uses it runs (generation, or shorten with a
// resume at or before it) and the review client only when the review step runs,
// and passes nil for a client it does not build. Client building follows the
// stages the run executes, never config.Need, which only widens the
// configuration check (a run without --after still checks the review settings
// under --no-review). store and resume configure artifact saving and resuming.
func buildWriter(d deps, cfg config.Config, opts *cliOptions, reviewLogGiven bool, store writer.ArtifactStore, resume writer.Stage) (writer.ArticleWriter, *publisher.ReviewLogWriter, error) {
	generationRuns := resume == writer.StageUnset
	shortenRuns := !opts.noShorten && (resume == writer.StageUnset || resume == writer.StageGeneration)
	reviewRuns := !opts.noReview && resume != writer.StageReview

	var client llm.LLMClient
	var err error
	if generationRuns || shortenRuns {
		if client, err = d.newLLMClient(cfg.LLM()); err != nil {
			return nil, nil, fmt.Errorf("build the LLM client: %w", err)
		}
	}
	var reviewClient llm.LLMClient
	if reviewRuns {
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
		Artifacts: writer.ArtifactOptions{Store: store, Resume: resume},
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

// tokensOriginName returns the origin word of a line and whether the stage has
// a usage to report. OriginNone, and any value outside the enumeration, report
// nothing.
func tokensOriginName(origin writer.UsageOrigin) (string, bool) {
	switch origin {
	case writer.OriginRun:
		return tokensRun, true
	case writer.OriginReused:
		return tokensReused, true
	default:
		return "", false
	}
}

// tokensLine renders one stage's token-usage line, or "" when the stage has no
// usage to report. The unknown reason is llm.Usage.UnknownReason: one
// whitespace-free token that never carries a response value. The model name is
// the rest of the line, so a model with spaces has no ambiguous boundary.
func tokensLine(stage writer.Stage, stageUsage writer.StageUsage) string {
	origin, ok := tokensOriginName(stageUsage.Origin)
	if !ok {
		return ""
	}
	var b strings.Builder
	b.WriteString(tokensPrefix)
	b.WriteString(stage.String())
	b.WriteByte(' ')
	b.WriteString(origin)
	b.WriteByte(' ')
	if counts, known := stageUsage.Usage.Counts(); known {
		fmt.Fprintf(&b, "%s %s%d %s%d %s%d %s%d",
			tokensKnown,
			tokensInput, counts.Input,
			tokensCacheRead, counts.CacheRead,
			tokensCacheWrite, counts.CacheWrite,
			tokensOutput, counts.Output)
	} else {
		fmt.Fprintf(&b, "%s %s%s", tokensUnknown, tokensReason, stageUsage.Usage.UnknownReason())
	}
	b.WriteByte(' ')
	if stageUsage.Model == "" {
		b.WriteString(tokensNoModel)
	} else {
		b.WriteString(tokensModel)
		b.WriteString(stageUsage.Model)
	}
	return b.String()
}

// reportTokens writes one token-usage line per stage, in stage order, for each
// stage that ran or reused an artifact. A stage that ran no LLM writes no line.
func reportTokens(errOut *stderrWriter, runUsage writer.RunUsage) {
	for _, entry := range []struct {
		stage writer.Stage
		usage writer.StageUsage
	}{
		{writer.StageGeneration, runUsage.Generation},
		{writer.StageShorten, runUsage.Shorten},
		{writer.StageReview, runUsage.Review},
	} {
		if line := tokensLine(entry.stage, entry.usage); line != "" {
			errOut.line("%s", line)
		}
	}
}

// reportSuccess writes the summary of a published run. The message count of a
// webhook post is recomputed from the article; it is "unknown" when that
// fails, which only a fake publisher that accepts any article can cause. It
// adds one line per post-generation step that ran, the review log path when one
// was written, the artifacts directory when one was used, and one token-usage
// line per stage that ran or was reused. An empty path writes nothing.
func reportSuccess(errOut *stderrWriter, dest destination, outPath, reviewLogPath, artifactsDir string, article writer.Article) {
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
	if artifactsDir != "" {
		errOut.line("%s: saved the artifacts to %s", programName, artifactsDir)
	}
	reportTokens(errOut, article.RunUsage)
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
	if usageErr, ok := errors.AsType[*writer.TokenUsageError](err); ok {
		reportTokens(errOut, usageErr.RunUsage)
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
