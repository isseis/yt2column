//go:build test

package publisher

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/isseis/yt2column/internal/writer"
)

func validArticle() writer.Article {
	return writer.Article{
		Title:        "A title",
		Body:         "\nThe body.\n\nSource: https://www.youtube.com/watch?v=abcdefghijk\n",
		SourceURL:    "https://www.youtube.com/watch?v=abcdefghijk",
		Model:        "model-x",
		ModelVersion: "2026-01-01",
	}
}

// dirEntries lists the names in dir.
func dirEntries(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}

func requireNoEntries(t *testing.T, dir string) {
	t.Helper()
	if got := dirEntries(t, dir); len(got) != 0 {
		t.Fatalf("directory %s should be empty, has %v", dir, got)
	}
}

func TestFilePublisherWritesArticle(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "article.md")
	p, err := NewFilePublisher(out)
	if err != nil {
		t.Fatal(err)
	}
	article := validArticle()
	if err := p.Publish(context.Background(), article); err != nil {
		t.Fatalf("Publish: %v", err)
	}

	got, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	want := "# A title\n\n- Model: model-x\n- Model version: 2026-01-01\n\n" + article.Body
	if string(got) != want {
		t.Fatalf("content = %q, want %q", got, want)
	}
	if !strings.HasSuffix(string(got), article.Body) {
		t.Fatal("file must end with the body")
	}
	info, err := os.Stat(out)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o644 {
		t.Fatalf("mode = %v, want 0644", info.Mode().Perm())
	}
	if names := dirEntries(t, dir); len(names) != 1 || names[0] != "article.md" {
		t.Fatalf("directory entries = %v, want only article.md", names)
	}
}

func TestFilePublisherEmptyModelVersion(t *testing.T) {
	out := filepath.Join(t.TempDir(), "a.md")
	p, _ := NewFilePublisher(out)
	article := validArticle()
	article.ModelVersion = ""
	if err := p.Publish(context.Background(), article); err != nil {
		t.Fatalf("Publish: %v", err)
	}
	got, _ := os.ReadFile(out)
	if !strings.Contains(string(got), "- Model version: (none)\n") {
		t.Fatalf("content = %q", got)
	}
}

func TestFilePublisherExistingPath(t *testing.T) {
	cases := []struct {
		name  string
		setup func(t *testing.T, dir, out string)
		check func(t *testing.T, dir, out string)
	}{
		{
			name: "regular file",
			setup: func(t *testing.T, _, out string) {
				mustWrite(t, out, "precious")
			},
			check: func(t *testing.T, _, out string) {
				if got, _ := os.ReadFile(out); string(got) != "precious" {
					t.Fatalf("existing file changed: %q", got)
				}
			},
		},
		{
			name: "directory",
			setup: func(t *testing.T, _, out string) {
				if err := os.Mkdir(out, 0o700); err != nil {
					t.Fatal(err)
				}
			},
			check: func(t *testing.T, _, out string) {
				if info, err := os.Lstat(out); err != nil || !info.IsDir() {
					t.Fatalf("directory replaced: %v %v", info, err)
				}
			},
		},
		{
			name: "symlink to existing file",
			setup: func(t *testing.T, dir, out string) {
				mustWrite(t, filepath.Join(dir, "target"), "target content")
				if err := os.Symlink("target", out); err != nil {
					t.Fatal(err)
				}
			},
			check: func(t *testing.T, dir, _ string) {
				if got, _ := os.ReadFile(filepath.Join(dir, "target")); string(got) != "target content" {
					t.Fatalf("link target changed: %q", got)
				}
			},
		},
		{
			name: "symlink to missing path",
			setup: func(t *testing.T, _, out string) {
				if err := os.Symlink("missing", out); err != nil {
					t.Fatal(err)
				}
			},
			check: func(t *testing.T, dir, _ string) {
				if _, err := os.Lstat(filepath.Join(dir, "missing")); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("the dangling link's target was created: %v", err)
				}
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			out := filepath.Join(dir, "article.md")
			tc.setup(t, dir, out)
			p, _ := NewFilePublisher(out)

			err := p.Publish(context.Background(), validArticle())
			if !errors.Is(err, ErrOutputExists) {
				t.Fatalf("err = %v, want ErrOutputExists", err)
			}
			kept, ok := errors.AsType[*KeptFileError](err)
			if !ok {
				t.Fatalf("err = %T, want *KeptFileError", err)
			}
			got, readErr := os.ReadFile(kept.TempPath)
			if readErr != nil {
				t.Fatalf("kept file: %v", readErr)
			}
			if !strings.HasSuffix(string(got), validArticle().Body) {
				t.Fatalf("kept file is not the complete article: %q", got)
			}
			tc.check(t, dir, out)
		})
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestFilePublisherDirectoryFailure(t *testing.T) {
	t.Run("missing directory", func(t *testing.T) {
		out := filepath.Join(t.TempDir(), "missing", "a.md")
		p, _ := NewFilePublisher(out)
		err := p.Publish(context.Background(), validArticle())
		if !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("err = %v, want not-exist", err)
		}
		if _, statErr := os.Lstat(out); !errors.Is(statErr, os.ErrNotExist) {
			t.Fatalf("output was created: %v", statErr)
		}
	})
	t.Run("unwritable directory", func(t *testing.T) {
		requireNonRoot(t)
		dir := t.TempDir()
		chmodForTest(t, dir, 0o500)
		p, _ := NewFilePublisher(filepath.Join(dir, "a.md"))
		err := p.Publish(context.Background(), validArticle())
		if !errors.Is(err, os.ErrPermission) {
			t.Fatalf("err = %v, want permission error", err)
		}
		requireNoEntries(t, dir)
	})
}

func TestFilePublisherRejectsArticle(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(a *writer.Article)
	}{
		{"title empty", func(a *writer.Article) { a.Title = "" }},
		{"title blank", func(a *writer.Article) { a.Title = "  \t" }},
		{"title newline", func(a *writer.Article) { a.Title = "a\nb" }},
		{"title carriage return", func(a *writer.Article) { a.Title = "a\rb" }},
		{"title escape", func(a *writer.Article) { a.Title = "a\x1b[2Jb" }},
		{"body empty", func(a *writer.Article) { a.Body = "" }},
		{"body blank", func(a *writer.Article) { a.Body = " \n\t" }},
		{"body escape", func(a *writer.Article) { a.Body = "x\x1b[2J" }},
		{"body carriage return", func(a *writer.Article) { a.Body = "x\ry" }},
		{"model empty", func(a *writer.Article) { a.Model = "" }},
		{"model blank", func(a *writer.Article) { a.Model = " " }},
		{"model control", func(a *writer.Article) { a.Model = "m\x07" }},
		{"model version control", func(a *writer.Article) { a.ModelVersion = "v\n1" }},
		{"source URL empty", func(a *writer.Article) { a.SourceURL = "" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			p, _ := NewFilePublisher(filepath.Join(dir, "a.md"))
			article := validArticle()
			tc.mutate(&article)
			err := p.Publish(context.Background(), article)
			if !errors.Is(err, writer.ErrInvalidArticle) {
				t.Fatalf("err = %v, want ErrInvalidArticle", err)
			}
			requireNoEntries(t, dir)
		})
	}
}

func TestFilePublisherCanceled(t *testing.T) {
	dir := t.TempDir()
	p, _ := NewFilePublisher(filepath.Join(dir, "a.md"))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := p.Publish(ctx, validArticle()); !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	requireNoEntries(t, dir)

	// Only the check at the start of Publish can act here: the output
	// directory is missing, so any later step would fail differently.
	missing, _ := NewFilePublisher(filepath.Join(dir, "missing", "a.md"))
	if err := missing.Publish(ctx, validArticle()); !errors.Is(err, context.Canceled) {
		t.Fatalf("missing directory: err = %v, want context.Canceled", err)
	}
}

func TestNewFilePublisherEmptyPath(t *testing.T) {
	p, err := NewFilePublisher("")
	if err == nil || p != nil {
		t.Fatalf("NewFilePublisher(\"\") = %v, %v; want nil and an error", p, err)
	}
}

func TestFilePublisherWriteFailure(t *testing.T) {
	// Larger than one write chunk, so a failure or a cancellation can land
	// in the middle of the content.
	big := validArticle()
	big.Body = "\n" + strings.Repeat("line of text\n", 10_000)

	t.Run("write error after some bytes", func(t *testing.T) {
		dir := t.TempDir()
		p := newFilePublisherWithSeams(t, filepath.Join(dir, "a.md"), failAfter(100, nil), nil)
		err := p.Publish(context.Background(), big)
		if !errors.Is(err, errInjectedWrite) {
			t.Fatalf("err = %v, want the injected write error", err)
		}
		requireNoEntries(t, dir)
	})
	t.Run("context canceled mid-write", func(t *testing.T) {
		dir := t.TempDir()
		ctx, cancel := context.WithCancel(context.Background())
		t.Cleanup(cancel)
		// The first chunk is written in full, then the context ends; the
		// next chunk must observe it.
		p := newFilePublisherWithSeams(t, filepath.Join(dir, "a.md"), cancelAfterFirstWrite(cancel), nil)
		err := p.Publish(ctx, big)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("err = %v, want context.Canceled", err)
		}
		requireNoEntries(t, dir)
	})
}

func TestFilePublisherCanceledDuringLastWrite(t *testing.T) {
	// The whole article fits in one chunk, so only the check after the last
	// write can notice that the context ended during it.
	dir := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	p := newFilePublisherWithSeams(t, filepath.Join(dir, "a.md"), cancelAfterFirstWrite(cancel), nil)
	if err := p.Publish(ctx, validArticle()); !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	requireNoEntries(t, dir)
}

func TestFilePublisherLinkFailure(t *testing.T) {
	cases := []struct {
		name      string
		errno     syscall.Errno
		wantNoLnk bool
	}{
		{"EPERM", syscall.EPERM, true},
		{"ENOTSUP", syscall.ENOTSUP, true},
		{"EOPNOTSUPP", syscall.EOPNOTSUPP, true},
		{"EXDEV", syscall.EXDEV, true},
		{"EMLINK", syscall.EMLINK, true},
		{"EACCES", syscall.EACCES, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			link := func(_, _ string) error { return &os.LinkError{Op: "link", Err: tc.errno} }
			p := newFilePublisherWithSeams(t, filepath.Join(dir, "a.md"), nil, link)

			err := p.Publish(context.Background(), validArticle())
			kept, ok := errors.AsType[*KeptFileError](err)
			if !ok {
				t.Fatalf("err = %v, want *KeptFileError", err)
			}
			if got := errors.Is(err, ErrNoHardLink); got != tc.wantNoLnk {
				t.Fatalf("errors.Is(ErrNoHardLink) = %v, want %v", got, tc.wantNoLnk)
			}
			if errors.Is(err, ErrOutputExists) {
				t.Fatal("must not wrap ErrOutputExists")
			}
			if !errors.Is(err, tc.errno) {
				t.Fatalf("cause %v is lost", tc.errno)
			}
			if _, statErr := os.Stat(kept.TempPath); statErr != nil {
				t.Fatalf("temp file not kept: %v", statErr)
			}
			if _, statErr := os.Lstat(filepath.Join(dir, "a.md")); !errors.Is(statErr, os.ErrNotExist) {
				t.Fatalf("output exists: %v", statErr)
			}
		})
	}
}
