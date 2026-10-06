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

// Request is one run. Writer and Publisher are built by the caller. VideoURL
// is validated by transcript.ValidateVideoURL before Run.
type Request struct {
	VideoURL  string
	OutPath   string // checked before the lock; never written by Run
	CacheDir  string
	YtDlpPath string
	Refresh   bool
	KeepCache bool
	Writer    writer.ArticleWriter
	Publisher publisher.Publisher
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
	if err := precheckOutput(req.OutPath); err != nil {
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
	pipe, err := pipeline.New(source, req.Writer, req.Publisher)
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
	case req.OutPath == "":
		return fmt.Errorf("%w: OutPath is empty", errInvalidRequest)
	case nilcheck.IsNil(req.Writer):
		return fmt.Errorf("%w: Writer is nil", errInvalidRequest)
	case nilcheck.IsNil(req.Publisher):
		return fmt.Errorf("%w: Publisher is nil", errInvalidRequest)
	default:
		return nil
	}
}

// precheckOutput reports an output problem before any expensive work. It is
// best effort and shares no code with FilePublisher, which owns the overwrite
// guarantee: it only decides between failing early and proceeding. An existing
// output path wraps publisher.ErrOutputExists (a plain path or a symlink,
// dangling or not); an unusable parent wraps errOutputParent and not
// ErrOutputExists.
func precheckOutput(outPath string) error {
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
