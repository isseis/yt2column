package publisher

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/isseis/yt2column/internal/writer"
)

// reviewLogTempPrefix distinguishes the review log's temporary files from the
// article's on disk. It carries the tool name so a user can tell where it came
// from.
const reviewLogTempPrefix = ".yt2column-reviewlog-"

// ReviewLogWriter writes the review list of an article to one new file and
// never overwrites, with the same temporary-file-and-link(2) procedure as
// FilePublisher. Unlike FilePublisher it removes the temporary file when
// linking fails: the review log is a record of an article that was already
// published, so a leftover file would only be mistaken for an article.
type ReviewLogWriter struct {
	file *atomicFile
}

// NewReviewLogWriter returns a writer for path. An empty path is an error.
func NewReviewLogWriter(path string) (*ReviewLogWriter, error) {
	file, err := newAtomicFile(path, reviewLogTempPrefix)
	if err != nil {
		return nil, err
	}
	return &ReviewLogWriter{file: file}, nil
}

// Path returns the file path, for the caller's pre-check.
func (w *ReviewLogWriter) Path() string { return w.file.path }

// Write checks a with Article.CheckPublishable (which includes
// ReviewResult.Check), rejecting a nil Review, and creates the file. On any
// failure, the temporary file is removed.
func (w *ReviewLogWriter) Write(ctx context.Context, a writer.Article) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if a.Review == nil {
		return fmt.Errorf("%w: the article has no review result", writer.ErrInvalidArticle)
	}
	if err := a.CheckPublishable(); err != nil {
		return err
	}

	tempPath, err := w.file.writeTemp(ctx, renderReviewLog(a))
	if err != nil {
		return err
	}

	if err := w.file.link(tempPath, w.file.path); err != nil {
		_ = os.Remove(tempPath)
		return classifyLinkError(err)
	}

	// The output path is complete from here on, so nothing below may turn the
	// write into a failure: the cleanup errors are ignored and ctx is not
	// consulted again.
	syncDir(filepath.Dir(w.file.path))
	_ = os.Remove(tempPath)
	return nil
}

// renderReviewLog returns the text ReviewLogWriter writes for a. Every value
// goes inside a blockquote, so a value containing Markdown structural
// characters cannot change the file's headings.
func renderReviewLog(a writer.Article) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Review log\n\n- Article: %s\n- Source: %s\n- Review model: %s\n- Review model version: %s\n- Revisions: %d\n",
		a.Title, a.SourceURL, a.Review.Model.Model, modelVersionOrNone(a.Review.Model.ModelVersion), len(a.Review.Revisions))
	if len(a.Review.Revisions) == 0 {
		b.WriteString("\nNo revisions.\n")
		return b.String()
	}
	for i, revision := range a.Review.Revisions {
		reason, _ := revision.Reason.WireValue()
		fmt.Fprintf(&b, "\n## %d. %s\n\nBefore:\n\n%s\n", i+1, reason, quoteBlock(revision.Before))
		if revision.Action == writer.ActionDelete {
			b.WriteString("\nDeleted.\n")
		} else {
			fmt.Fprintf(&b, "\nAfter:\n\n%s\n", quoteBlock(revision.After))
			if revision.NoChange() {
				b.WriteString("\nNo change.\n")
			}
		}
		fmt.Fprintf(&b, "\nEvidence:\n\n%s\n", quoteBlock(revision.Evidence))
	}
	return b.String()
}

// quoteBlock renders each line of value as a Markdown blockquote line.
func quoteBlock(value string) string {
	lines := strings.Split(value, "\n")
	for i, line := range lines {
		lines[i] = "> " + line
	}
	return strings.Join(lines, "\n")
}
