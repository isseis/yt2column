//go:build test

package publisher

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/isseis/yt2column/internal/writer"
)

// reviewArticle returns a valid article carrying a review result for the given
// revisions. An empty revisions slice still sets Review, so the article records
// that the step ran.
func reviewArticle(revisions ...writer.Revision) writer.Article {
	a := validArticle()
	a.Review = &writer.ReviewResult{
		Model:     writer.StepModel{Model: "review-x", ModelVersion: "r1"},
		Revisions: revisions,
	}
	return a
}

func TestReviewLogWriterFormat(t *testing.T) {
	const header = "# Review log\n\n" +
		"- Article: A title\n" +
		"- Source: https://www.youtube.com/watch?v=abcdefghijk\n"

	requireOnlyReviewLog := func(t *testing.T, dir string) {
		t.Helper()
		if names := dirEntries(t, dir); len(names) != 1 || names[0] != "review.log" {
			t.Fatalf("directory entries = %v, want only review.log", names)
		}
	}

	t.Run("empty list", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "review.log")
		w, err := NewReviewLogWriter(path)
		if err != nil {
			t.Fatal(err)
		}
		article := reviewArticle()
		article.Review.Model.ModelVersion = ""
		if err := w.Write(context.Background(), article); err != nil {
			t.Fatalf("Write: %v", err)
		}
		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		want := header +
			"- Review model: review-x\n" +
			"- Review model version: (none)\n" +
			"- Revisions: 0\n" +
			"\nNo revisions.\n"
		if string(got) != want {
			t.Fatalf("content = %q, want %q", got, want)
		}
		requireOnlyReviewLog(t, dir)
	})

	t.Run("replacement", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "review.log")
		w, _ := NewReviewLogWriter(path)
		article := reviewArticle(writer.Revision{
			Before: "bad", Action: writer.ActionReplace, After: "good",
			Reason: writer.ReasonMeaningChange, Evidence: "e1",
		})
		if err := w.Write(context.Background(), article); err != nil {
			t.Fatalf("Write: %v", err)
		}
		got, _ := os.ReadFile(path)
		want := header +
			"- Review model: review-x\n" +
			"- Review model version: r1\n" +
			"- Revisions: 1\n" +
			"\n## 1. meaning_change\n" +
			"\nBefore:\n\n> bad\n" +
			"\nAfter:\n\n> good\n" +
			"\nEvidence:\n\n> e1\n"
		if string(got) != want {
			t.Fatalf("content = %q, want %q", got, want)
		}
		requireOnlyReviewLog(t, dir)
	})

	t.Run("no change", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "review.log")
		w, _ := NewReviewLogWriter(path)
		article := reviewArticle(writer.Revision{
			Before: "same", Action: writer.ActionReplace, After: "same",
			Reason: writer.ReasonMeaningChange, Evidence: "e1",
		})
		if err := w.Write(context.Background(), article); err != nil {
			t.Fatalf("Write: %v", err)
		}
		got, _ := os.ReadFile(path)
		want := header +
			"- Review model: review-x\n" +
			"- Review model version: r1\n" +
			"- Revisions: 1\n" +
			"\n## 1. meaning_change\n" +
			"\nBefore:\n\n> same\n" +
			"\nAfter:\n\n> same\n" +
			"\nNo change.\n" +
			"\nEvidence:\n\n> e1\n"
		if string(got) != want {
			t.Fatalf("content = %q, want %q", got, want)
		}
		requireOnlyReviewLog(t, dir)
	})

	t.Run("deletion with a multiline evidence", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "review.log")
		w, _ := NewReviewLogWriter(path)
		article := reviewArticle(writer.Revision{
			Before: "bad", Action: writer.ActionDelete,
			Reason: writer.ReasonUnsupported, Evidence: "line1\nline2",
		})
		if err := w.Write(context.Background(), article); err != nil {
			t.Fatalf("Write: %v", err)
		}
		got, _ := os.ReadFile(path)
		want := header +
			"- Review model: review-x\n" +
			"- Review model version: r1\n" +
			"- Revisions: 1\n" +
			"\n## 1. unsupported\n" +
			"\nBefore:\n\n> bad\n" +
			"\nDeleted.\n" +
			"\nEvidence:\n\n> line1\n> line2\n"
		if string(got) != want {
			t.Fatalf("content = %q, want %q", got, want)
		}
		requireOnlyReviewLog(t, dir)
	})
}

func TestReviewLogWriterNoOverwrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "review.log")
	mustWrite(t, path, "precious")

	w, _ := NewReviewLogWriter(path)
	err := w.Write(context.Background(), reviewArticle())
	if !errors.Is(err, ErrOutputExists) {
		t.Fatalf("err = %v, want ErrOutputExists", err)
	}
	if got, _ := os.ReadFile(path); string(got) != "precious" {
		t.Fatalf("existing file changed: %q", got)
	}
	if names := dirEntries(t, dir); len(names) != 1 || names[0] != "review.log" {
		t.Fatalf("directory entries = %v, want only review.log", names)
	}
}

func TestReviewLogWriterLinkFailure(t *testing.T) {
	dir := t.TempDir()
	link := func(_, _ string) error { return &os.LinkError{Op: "link", Err: syscall.EIO} }
	w := newReviewLogWriterWithSeams(t, filepath.Join(dir, "review.log"), link)

	err := w.Write(context.Background(), reviewArticle())
	if err == nil {
		t.Fatal("Write succeeded")
	}
	if _, ok := errors.AsType[*KeptFileError](err); ok {
		t.Fatal("review log write must not return KeptFileError")
	}
	requireNoEntries(t, dir)
}

func TestReviewLogWriterRejects(t *testing.T) {
	cases := []struct {
		name    string
		article writer.Article
		wantIs  error
	}{
		{
			name:    "nil review",
			article: validArticle(),
			wantIs:  writer.ErrInvalidArticle,
		},
		{
			name: "review failing its check",
			article: reviewArticle(writer.Revision{
				Before: "bad", Action: writer.ActionReplace, After: "good",
				Reason: writer.ReasonUnknown, Evidence: "e1",
			}),
			wantIs: writer.ErrInvalidReview,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			w, _ := NewReviewLogWriter(filepath.Join(dir, "review.log"))
			err := w.Write(context.Background(), tc.article)
			if !errors.Is(err, tc.wantIs) {
				t.Fatalf("err = %v, want %v", err, tc.wantIs)
			}
			if !errors.Is(err, writer.ErrInvalidArticle) {
				t.Fatalf("err = %v, want it to wrap ErrInvalidArticle", err)
			}
			requireNoEntries(t, dir)
		})
	}
}

func TestNewReviewLogWriterEmptyPath(t *testing.T) {
	w, err := NewReviewLogWriter("")
	if !errors.Is(err, errEmptyPath) || w != nil {
		t.Fatalf("NewReviewLogWriter(\"\") = %v, %v; want nil and errEmptyPath", w, err)
	}
}
