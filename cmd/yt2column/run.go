package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/isseis/yt2column/internal/config"
	"github.com/isseis/yt2column/internal/job"
	"github.com/isseis/yt2column/internal/llm"
	"github.com/isseis/yt2column/internal/llm/provider"
	"github.com/isseis/yt2column/internal/pipeline"
	"github.com/isseis/yt2column/internal/publisher"
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
	newLLMClient func(config.Config) (llm.LLMClient, error)
	newWriter    func(llm.LLMClient, writer.Options) (writer.ArticleWriter, error)
	newPublisher func(path string) (publisher.Publisher, error)
}

// productionDeps returns the real constructors. main and the integration test
// both use it, so the test runs the same assembly as the CLI.
func productionDeps() deps {
	return deps{
		newLLMClient: provider.New,
		newWriter:    writer.New,
		newPublisher: newFilePublisher,
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

// cliOptions holds the parsed flags.
type cliOptions struct {
	out          string
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
	fs.StringVar(&opts.out, "out", "", "write the article to this `path` (required); it must not exist, its directory must exist, and it must be outside the cache directory")
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
	b.WriteString("Generates a column article from a YouTube video's subtitles and writes it to the --out file.\n")
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
// with exitUsage before the cache directory, yt-dlp, or the LLM API is touched.
// Only job.Run (step B) acts, and its failure exits with exitFailure. Standard
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

	// A2: exactly one video URL, in a form yt-dlp would accept, and --out.
	if fs.NArg() != 1 {
		return usageError(errOut, "expected exactly one video URL after the flags, got %d arguments", fs.NArg())
	}
	_, videoURL, err := transcript.ValidateVideoURL(fs.Arg(0))
	if err != nil {
		return usageError(errOut, "%v", err)
	}
	if opts.out == "" {
		return usageError(errOut, "--out is required")
	}

	// A3: the configuration. Each rejected variable is reported on its own
	// line; a VarError never carries a value.
	cfg, err := config.Load(lookup)
	if err != nil {
		for _, e := range splitJoined(err) {
			errOut.line("%s: configuration: %v", programName, e)
		}
		errOut.line("%s", usageHint)
		return exitUsage
	}
	errOut.secrets = configuredSecrets(cfg)
	if cfg.HTTP2DebugEnabled() {
		// Written before any LLM call: the Go HTTP/2 log bypasses sanitize.
		errOut.line("%s: warning: GODEBUG enables http2debug, so the Go HTTP/2 log may write the API key to standard error", programName)
	}

	// A4: the output must not be inside the cache directory, where pruning or
	// removing the cache could delete it.
	if outPathInsideCacheDir(opts.out, cfg.CacheDir()) {
		return usageError(errOut, "--out %s is inside the cache directory %s; choose a path outside it", opts.out, cfg.CacheDir())
	}

	// A5: build the stages. writer.New only reads the template files.
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
	pub, err := d.newPublisher(opts.out)
	if err != nil {
		return usageError(errOut, "build the publisher: %v", err)
	}

	// B: run the job. Warnings are reported whether or not it succeeded.
	result, err := job.Run(ctx, job.Request{
		VideoURL:  videoURL,
		OutPath:   opts.out,
		CacheDir:  cfg.CacheDir(),
		YtDlpPath: cfg.YtDlpPath(),
		Refresh:   opts.refresh,
		KeepCache: opts.keepCache,
		Writer:    articleWriter,
		Publisher: pub,
	})
	for _, warning := range result.Warnings {
		errOut.line("%s: warning: %v", programName, warning)
	}
	if err != nil {
		reportRunError(errOut, err)
		return exitFailure
	}

	// C: the summary.
	modelVersion := result.Article.ModelVersion
	if modelVersion == "" {
		modelVersion = "(none)"
	}
	errOut.line("%s: wrote the article to %s (model: %s, model version: %s)", programName, opts.out, result.Article.Model, modelVersion)
	return exitOK
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
}

// minutes renders d as a whole number of minutes.
func minutes(d time.Duration) string {
	return fmt.Sprintf("%d-minute", int(d/time.Minute))
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
// redacted. An unset secret contributes nothing.
func configuredSecrets(cfg config.Config) []string {
	var secrets []string
	if key, err := cfg.DeepSeekAPIKey().Reveal(); err == nil {
		secrets = append(secrets, key)
	}
	if url, ok := cfg.SlackWebhookURL(); ok {
		if value, err := url.Reveal(); err == nil {
			secrets = append(secrets, value)
		}
	}
	return secrets
}
