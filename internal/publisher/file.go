package publisher

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"

	"github.com/isseis/yt2column/internal/writer"
)

var (
	// ErrOutputExists reports that the output path already names something
	// (a file, a directory, or a symbolic link, dangling or not).
	ErrOutputExists = errors.New("output path already exists")
	// ErrNoHardLink reports that the output directory's file system cannot
	// create a hard link, which FilePublisher needs to publish without
	// overwriting.
	ErrNoHardLink = errors.New("cannot create a hard link in the output directory")

	errEmptyPath = errors.New("output path is empty")
)

// tempPrefix starts with a dot so a kept temporary file is hidden, and carries
// the tool name so a user can tell where it came from.
const tempPrefix = ".yt2column-"

// writeChunkSize bounds how many bytes are written between two checks of ctx.
const writeChunkSize = 32 * 1024

// articleFileMode is the permission of the published file and the review log.
// Both are non-secret documents built from a public video, and the process
// umask is not applied because the standard library cannot read it without
// changing it.
const articleFileMode = 0o644

// FilePublisher writes an article to one local file and never overwrites.
type FilePublisher struct {
	file *atomicFile
}

var _ Publisher = (*FilePublisher)(nil)

// NewFilePublisher returns a publisher for path. An empty path is an error.
func NewFilePublisher(path string) (*FilePublisher, error) {
	file, err := newAtomicFile(path, tempPrefix)
	if err != nil {
		return nil, err
	}
	return &FilePublisher{file: file}, nil
}

// atomicFile writes content to one new file and never overwrites.
//
// Contract: the output path only ever names a complete, synced file. The
// content is written to a temporary file in the same directory and given its
// final name with link(2), which fails with EEXIST when anything already has
// that name and does not follow symbolic links, so a file created between a
// check and the creation is never replaced.
type atomicFile struct {
	path string
	// prefix starts the temporary file's name. Each output kind passes its
	// own, so a kept temporary file on disk shows which kind wrote it.
	prefix string

	// Seams for tests; newAtomicFile sets the production values.
	wrapWriter func(io.Writer) io.Writer
	link       func(oldname, newname string) error
}

// newAtomicFile returns an atomicFile for path whose temporary files start
// with prefix. An empty path is an error.
func newAtomicFile(path, prefix string) (*atomicFile, error) {
	if path == "" {
		return nil, errEmptyPath
	}
	return &atomicFile{
		path:       path,
		prefix:     prefix,
		wrapWriter: func(w io.Writer) io.Writer { return w },
		link:       os.Link,
	}, nil
}

// KeptFileError reports that the complete article was left in a temporary
// file because linking it to the output path failed.
type KeptFileError struct {
	TempPath string
	Err      error
}

func (e *KeptFileError) Error() string {
	return fmt.Sprintf("the article was kept in %s: %v", e.TempPath, e.Err)
}

func (e *KeptFileError) Unwrap() error { return e.Err }

// Publish checks the article and creates the output path with its content.
func (p *FilePublisher) Publish(ctx context.Context, article writer.Article) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := article.CheckPublishable(); err != nil {
		return err
	}

	tempPath, err := p.file.writeTemp(ctx, renderFileArticle(article))
	if err != nil {
		return err
	}

	if err := p.file.link(tempPath, p.file.path); err != nil {
		// The article cost an LLM call, so it is kept and its path reported.
		return &KeptFileError{TempPath: tempPath, Err: classifyLinkError(err)}
	}

	// The output path is complete from here on, so nothing below may turn the
	// publish into a failure: the cleanup errors are ignored and ctx is not
	// consulted again.
	syncDir(filepath.Dir(p.file.path))
	_ = os.Remove(tempPath)
	return nil
}

// writeTemp writes content to a synced temporary file next to the output path
// and returns its path. On any failure the temporary file is removed.
func (a *atomicFile) writeTemp(ctx context.Context, content string) (string, error) {
	f, err := os.CreateTemp(filepath.Dir(a.path), a.prefix+"*.tmp")
	if err != nil {
		return "", fmt.Errorf("create a temporary file: %w", err)
	}
	tempPath := f.Name()
	fail := func(err error) (string, error) {
		_ = f.Close()
		_ = os.Remove(tempPath)
		return "", err
	}

	//nolint:gosec // the article and the review log are non-secret documents built from a public video
	if err := f.Chmod(articleFileMode); err != nil {
		return fail(fmt.Errorf("set the temporary file mode: %w", err))
	}
	w := a.wrapWriter(f)
	for rest := []byte(content); len(rest) > 0; {
		if err := ctx.Err(); err != nil {
			return fail(err)
		}
		n := min(len(rest), writeChunkSize)
		if _, err := w.Write(rest[:n]); err != nil {
			return fail(fmt.Errorf("write the temporary file: %w", err))
		}
		rest = rest[n:]
	}
	if err := ctx.Err(); err != nil {
		return fail(err)
	}
	if err := f.Sync(); err != nil {
		return fail(fmt.Errorf("sync the temporary file: %w", err))
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tempPath)
		return "", fmt.Errorf("close the temporary file: %w", err)
	}
	return tempPath, nil
}

// classifyLinkError marks the causes a user can act on with a sentinel and
// keeps any other cause as it is.
func classifyLinkError(err error) error {
	switch {
	case errors.Is(err, syscall.EEXIST):
		return fmt.Errorf("%w: %w", ErrOutputExists, err)
	case errors.Is(err, syscall.EPERM),
		errors.Is(err, syscall.ENOTSUP),
		errors.Is(err, syscall.EOPNOTSUPP),
		errors.Is(err, syscall.EXDEV),
		errors.Is(err, syscall.EMLINK):
		return fmt.Errorf("%w: %w", ErrNoHardLink, err)
	default:
		return err
	}
}

// syncDir makes the new directory entry durable; a failure is ignored because
// the article is already published.
func syncDir(dir string) {
	d, err := os.Open(dir) //nolint:gosec // dir is the parent of the user's --out path
	if err != nil {
		return
	}
	_ = d.Sync()
	_ = d.Close()
}
