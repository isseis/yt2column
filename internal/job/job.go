// Package job runs one end-to-end job: the output pre-checks, the cache
// directory lock, a prune, the pipeline, and the cache cleanup. It holds no
// terminal or flag handling, so a caller other than the CLI can use it.
package job

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/isseis/yt2column/internal/cachelock"
	"github.com/isseis/yt2column/internal/nilcheck"
	"github.com/isseis/yt2column/internal/pipeline"
	"github.com/isseis/yt2column/internal/publisher"
	"github.com/isseis/yt2column/internal/transcript"
	"github.com/isseis/yt2column/internal/writer"
)

// YtDlpTimeout bounds one yt-dlp run.
const YtDlpTimeout = 5 * time.Minute

// outputKind distinguishes the two output kinds. The zero value is invalid, so
// an Output that was never built cannot run.
type outputKind int

const (
	outputKindInvalid outputKind = iota
	outputKindFile
	outputKindRemote
)

// Output is where Run sends the article: the Publisher together with what Run
// needs to know about it. The zero value is rejected by Run, so an output that
// was never chosen cannot run.
type Output struct {
	kind outputKind
	// path is the file output's target; it is empty for a remote output.
	path string
	pub  publisher.Publisher
}

// FileOutput is an output to a local file. Run pre-checks path before any side
// effect; p must write to path (the caller builds both from the same value).
func FileOutput(path string, p publisher.Publisher) Output {
	return Output{kind: outputKindFile, path: path, pub: p}
}

// RemoteOutput is an output that needs no local pre-check, such as a Webhook.
// The kind is defined by that property, not by where the article goes.
func RemoteOutput(p publisher.Publisher) Output {
	return Output{kind: outputKindRemote, pub: p}
}

// Request is one run. Writer and the Output's Publisher are built by the
// caller. VideoURL is validated by transcript.ValidateVideoURL before Run.
type Request struct {
	VideoURL  string
	CacheDir  string
	YtDlpPath string
	Refresh   bool
	KeepCache bool
	Writer    writer.ArticleWriter
	Output    Output
}

// Result describes a published run. Warnings holds the prune and cache-removal
// failures that do not fail the run.
type Result struct {
	Article  writer.Article
	Warnings []error
}

var (
	errInvalidRequest = errors.New("invalid job request")
	errOutputParent   = errors.New("output parent directory is not usable")
)

// Run executes one run: it validates the request, pre-checks the output path,
// takes the cache directory lock, prunes dangling entries, runs the pipeline,
// and removes the video's cache unless KeepCache is set. A nil error means the
// article was published.
func Run(ctx context.Context, req Request) (Result, error) {
	if err := validateRequest(req); err != nil {
		return Result{}, err
	}
	if err := precheckOutput(req.Output); err != nil {
		return Result{}, err
	}
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}

	lock, err := cachelock.Acquire(req.CacheDir)
	if err != nil {
		return Result{}, err
	}
	defer func() { _ = lock.Close() }()

	source, err := transcript.NewYtDlpSource(transcript.Options{
		CacheDir:       req.CacheDir,
		YtDlpPath:      req.YtDlpPath,
		Timeout:        YtDlpTimeout,
		ForceRefresh:   req.Refresh,
		InheritedFiles: []*os.File{lock.File()},
	})
	if err != nil {
		return Result{}, err
	}
	pipe, err := pipeline.New(source, req.Writer, req.Output.pub)
	if err != nil {
		return Result{}, err
	}
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}

	var result Result
	if err := source.PruneCache(ctx); err != nil {
		result.Warnings = append(result.Warnings, fmt.Errorf("prune the cache: %w", err))
	}

	article, err := pipe.Run(ctx, req.VideoURL)
	if err != nil {
		// Return the warnings gathered so far (a prune failure) so the caller
		// can still report them when the pipeline fails.
		return result, err
	}
	result.Article = article

	if !req.KeepCache {
		if err := source.RemoveCache(ctx, req.VideoURL); err != nil {
			result.Warnings = append(result.Warnings, fmt.Errorf("remove the cache: %w", err))
		}
	}
	return result, nil
}

// validateRequest checks the fields Run cannot build without. It does not
// create anything.
func validateRequest(req Request) error {
	switch {
	case req.CacheDir == "":
		return fmt.Errorf("%w: CacheDir is empty", errInvalidRequest)
	case req.VideoURL == "":
		return fmt.Errorf("%w: VideoURL is empty", errInvalidRequest)
	case nilcheck.IsNil(req.Writer):
		return fmt.Errorf("%w: Writer is nil", errInvalidRequest)
	}

	switch req.Output.kind {
	case outputKindFile:
		switch {
		case req.Output.path == "":
			return fmt.Errorf("%w: the file output needs a path", errInvalidRequest)
		case nilcheck.IsNil(req.Output.pub):
			return fmt.Errorf("%w: the output needs a Publisher", errInvalidRequest)
		}
	case outputKindRemote:
		if nilcheck.IsNil(req.Output.pub) {
			return fmt.Errorf("%w: the output needs a Publisher", errInvalidRequest)
		}
	default:
		return fmt.Errorf("%w: the output was not chosen", errInvalidRequest)
	}
	return nil
}

// precheckOutput reports an output problem before any expensive work. It is
// best effort and shares no code with FilePublisher, which owns the overwrite
// guarantee: it only decides between failing early and proceeding. An existing
// output path wraps publisher.ErrOutputExists (a plain path or a symlink,
// dangling or not); an unusable parent wraps errOutputParent and not
// ErrOutputExists. A remote output has nothing local to check.
func precheckOutput(output Output) error {
	switch output.kind {
	case outputKindFile:
		return precheckFileOutput(output.path)
	case outputKindRemote:
		return nil
	default:
		// validateRequest already rejected the zero value.
		return nil
	}
}

// precheckFileOutput is the file-output pre-check.
func precheckFileOutput(outPath string) error {
	if _, err := os.Lstat(outPath); err == nil {
		return &pipeline.StageError{
			Stage: pipeline.StagePublish,
			Err:   fmt.Errorf("%w: %s", publisher.ErrOutputExists, outPath),
		}
	}

	info, err := os.Stat(filepath.Dir(outPath))
	if err != nil {
		// A parent that is missing, unsearchable, or has a non-directory
		// component can never receive the file, so reject it now instead of
		// running yt-dlp and the LLM first. Any other lookup failure is
		// indeterminate, so it proceeds (best effort).
		if errors.Is(err, fs.ErrNotExist) ||
			errors.Is(err, syscall.ENOTDIR) ||
			errors.Is(err, fs.ErrPermission) {
			return &pipeline.StageError{Stage: pipeline.StagePublish, Err: fmt.Errorf("%w: the directory cannot be used", errOutputParent)}
		}
		return nil
	}
	if !info.IsDir() {
		return &pipeline.StageError{Stage: pipeline.StagePublish, Err: fmt.Errorf("%w: it is not a directory", errOutputParent)}
	}
	return nil
}
