//go:build test

package writer

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"text/template"
	"time"

	"github.com/isseis/yt2column/internal/llm/testutil"
	"github.com/isseis/yt2column/prompts"
)

// fifoTimeout bounds how long New may take on a FIFO with no writer before the
// test reports it as blocked.
const fifoTimeout = 5 * time.Second

// requireRejected checks that New failed with ErrInvalidTemplate joined with
// want, and returned no writer.
func requireRejected(t *testing.T, w ArticleWriter, err, want error) {
	t.Helper()
	if !errors.Is(err, ErrInvalidTemplate) {
		t.Errorf("New error = %v, want ErrInvalidTemplate", err)
	}
	if want != nil && !errors.Is(err, want) {
		t.Errorf("New error = %v, want it to also match %v", err, want)
	}
	if w != nil {
		t.Errorf("New returned a writer %v, want nil", w)
	}
}

// requireAccepted checks that New succeeded and returned a writer.
func requireAccepted(t *testing.T, w ArticleWriter, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("New error = %v, want nil", err)
	}
	if w == nil {
		t.Fatal("New returned a nil writer")
	}
}

// fakeOpenedFile is an openedFile whose Stat and Read fail as configured, to
// reach the failures a real regular file cannot produce on demand.
type fakeOpenedFile struct {
	info    fs.FileInfo
	statErr error
	readErr error
}

func (f fakeOpenedFile) Stat() (fs.FileInfo, error) { return f.info, f.statErr }

func (f fakeOpenedFile) Read([]byte) (int, error) { return 0, f.readErr }

func TestNewRejectsInvalidOverrideFile(t *testing.T) {
	cases := []struct {
		name string
		// path returns the override path; it may create files under dir.
		path func(t *testing.T, dir string) string
		want error
	}{
		{"missing file", func(_ *testing.T, dir string) string {
			return filepath.Join(dir, "missing.tmpl")
		}, fs.ErrNotExist},
		{"unreadable file", func(t *testing.T, _ string) string {
			requireNonRoot(t)
			path := writeOverrideFile(t, "{{.Title}}")
			if err := os.Chmod(path, 0); err != nil {
				t.Fatalf("chmod %s: %v", path, err)
			}
			return path
		}, fs.ErrPermission},
		{"directory", func(_ *testing.T, dir string) string {
			return dir
		}, errNotRegularFile},
		{"symlink to directory", func(t *testing.T, dir string) string {
			target := filepath.Join(dir, "target")
			if err := os.Mkdir(target, 0o700); err != nil {
				t.Fatalf("mkdir %s: %v", target, err)
			}
			link := filepath.Join(dir, "link")
			if err := os.Symlink(target, link); err != nil {
				t.Fatalf("symlink %s: %v", link, err)
			}
			return link
		}, errNotRegularFile},
	}
	// The disallowed-syntax examples of the requirements are covered, with
	// the checker's own error, by TestTemplateSyntaxAllowlist.
	contents := []struct {
		name    string
		content string
	}{
		{"empty", ""},
		{"whitespace only", " \t\r\n\u3000"},
		{"invalid UTF-8", "{{.Title}}\xff"},
		{"parse error", "{{.Title"},
	}
	for _, target := range overrideTargets {
		for _, tc := range cases {
			t.Run(target.name+"/"+tc.name, func(t *testing.T) {
				path := tc.path(t, t.TempDir())
				w, err := New(&llmtestutil.FakeLLMClient{}, target.options(path))
				requireRejected(t, w, err, tc.want)
			})
		}
		for _, tc := range contents {
			t.Run(target.name+"/"+tc.name, func(t *testing.T) {
				path := writeOverrideFile(t, tc.content)
				w, err := New(&llmtestutil.FakeLLMClient{}, target.options(path))
				requireRejected(t, w, err, nil)
			})
		}
		t.Run(target.name+"/symlink to regular file is followed", func(t *testing.T) {
			target2 := writeOverrideFile(t, "{{.Title}}")
			link := filepath.Join(t.TempDir(), "link")
			if err := os.Symlink(target2, link); err != nil {
				t.Fatalf("symlink %s: %v", link, err)
			}
			w, err := New(&llmtestutil.FakeLLMClient{}, target.options(link))
			requireAccepted(t, w, err)
		})
	}

	// Stat and read failures of an opened regular file cannot be produced on
	// demand, so they are reached through readOpenedFile with a fake file.
	t.Run("stat failure", func(t *testing.T) {
		statErr := &fs.PathError{Op: "fstat", Path: "override.tmpl", Err: syscall.EIO}
		_, err := readOpenedFile(templateSource{name: systemTemplateName, path: "override.tmpl"}, fakeOpenedFile{statErr: statErr})
		if !errors.Is(err, ErrInvalidTemplate) || !errors.Is(err, syscall.EIO) {
			t.Errorf("readOpenedFile error = %v, want ErrInvalidTemplate and EIO", err)
		}
	})
	t.Run("read failure", func(t *testing.T) {
		info, err := os.Stat(writeOverrideFile(t, "{{.Title}}"))
		if err != nil {
			t.Fatalf("stat: %v", err)
		}
		readErr := &fs.PathError{Op: "read", Path: "override.tmpl", Err: syscall.EIO}
		_, err = readOpenedFile(templateSource{name: systemTemplateName, path: "override.tmpl"}, fakeOpenedFile{info: info, readErr: readErr})
		if !errors.Is(err, ErrInvalidTemplate) || !errors.Is(err, syscall.EIO) {
			t.Errorf("readOpenedFile error = %v, want ErrInvalidTemplate and EIO", err)
		}
	})
}

func TestNewOverrideFileFIFO(t *testing.T) {
	for _, target := range overrideTargets {
		t.Run(target.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "override.fifo")
			if err := syscall.Mkfifo(path, 0o600); err != nil {
				t.Fatalf("mkfifo %s: %v", path, err)
			}
			type result struct {
				w   ArticleWriter
				err error
			}
			done := make(chan result, 1)
			go func() {
				w, err := New(&llmtestutil.FakeLLMClient{}, target.options(path))
				done <- result{w, err}
			}()
			select {
			case r := <-done:
				requireRejected(t, r.w, r.err, errNotRegularFile)
			case <-time.After(fifoTimeout):
				// Opening the write side releases a reader blocked in open,
				// so the goroutine ends before the test does.
				writer, err := os.OpenFile(path, os.O_WRONLY|syscall.O_NONBLOCK, 0)
				if err != nil {
					t.Fatalf("New did not return within %v on a FIFO with no writer, and opening the write side failed: %v", fifoTimeout, err)
				}
				_ = writer.Close()
				select {
				case <-done:
				case <-time.After(fifoTimeout):
				}
				t.Fatalf("New did not return within %v on a FIFO with no writer", fifoTimeout)
			}
		})
	}
}

func TestNewOverrideFileSizeLimit(t *testing.T) {
	atLimit := strings.Repeat("a", maxTemplateBytes)
	for _, target := range overrideTargets {
		t.Run(target.name+"/at limit", func(t *testing.T) {
			w, err := New(&llmtestutil.FakeLLMClient{}, target.options(writeOverrideFile(t, atLimit)))
			requireAccepted(t, w, err)
		})
		t.Run(target.name+"/over limit", func(t *testing.T) {
			w, err := New(&llmtestutil.FakeLLMClient{}, target.options(writeOverrideFile(t, atLimit+"a")))
			requireRejected(t, w, err, nil)
		})
	}
}

// largeFile is a regular openedFile with remaining bytes of content, far
// more than the limit; it is finite so a reader without the bound fails the
// test instead of exhausting memory.
type largeFile struct {
	info      fs.FileInfo
	remaining int
}

func (f *largeFile) Stat() (fs.FileInfo, error) { return f.info, nil }

func (f *largeFile) Read(p []byte) (int, error) {
	if f.remaining == 0 {
		return 0, io.EOF
	}
	n := min(len(p), f.remaining)
	for i := range n {
		p[i] = 'a'
	}
	f.remaining -= n
	return n, nil
}

// TestReadOpenedFileBound checks that an override file is read only one byte
// past the limit, so a huge file is rejected without being loaded.
func TestReadOpenedFileBound(t *testing.T) {
	info, err := os.Stat(writeOverrideFile(t, "{{.Title}}"))
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	data, err := readOpenedFile(templateSource{name: systemTemplateName, path: "override.tmpl"}, &largeFile{info: info, remaining: 4 * maxTemplateBytes})
	if err != nil {
		t.Fatalf("readOpenedFile error = %v", err)
	}
	if len(data) != maxTemplateBytes+1 {
		t.Errorf("readOpenedFile read %d bytes, want %d", len(data), maxTemplateBytes+1)
	}
}

func TestTemplateSyntaxAllowlist(t *testing.T) {
	accepted := []struct{ name, text string }{
		{"text", "動画タイトル:"},
		{"comment", "{{/* メモ */}}x"},
		{"field Title", "{{.Title}}"},
		{"field ChannelName", "{{.ChannelName}}"},
		{"field Description", "{{.Description}}"},
		{"field Transcript", "{{.Transcript}}"},
		{"if else", "{{if .Description}}a{{else}}b{{end}}"},
		{"else if", "{{if .Title}}a{{else if .Description}}b{{else}}c{{end}}"},
		{"string constant", `{{"x"}}`},
		{"number constant", "{{100}}"},
		{"bool constant", "{{true}}"},
		{"parentheses", `{{if (eq .Title "")}}a{{end}}`},
		{"pipeline", "{{.Title | len}}"},
		{"func and", "{{and .Title .Description}}"},
		{"func or", "{{or .Title .Description}}"},
		{"func not", "{{not .Title}}"},
		{"func eq", `{{eq .Title "a"}}`},
		{"func ne", `{{ne .Title "a"}}`},
		{"func lt", "{{lt (len .Title) 3}}"},
		{"func le", "{{le (len .Title) 3}}"},
		{"func gt", "{{gt (len .Title) 3}}"},
		{"func ge", "{{ge (len .Title) 3}}"},
		{"func len", "{{len .Title}}"},
		{"func index", "{{index .Title 100}}"},
	}
	syntaxRejected := []struct{ name, text string }{
		{"unknown field", "{{.APIKey}}"},
		{"field of a field", "{{.Title.Foo}}"},
		{"dot", "{{.}}"},
		{"root variable", "{{$}}"},
		{"field of root variable", "{{$.Title}}"},
		{"variable declaration", "{{$x := .Title}}"},
		{"with", "{{with .Title}}{{.}}{{end}}"},
		{"range", "{{range .Title}}a{{end}}"},
		{"template", `{{template "x"}}`},
		// break and continue parse only inside range, which is rejected
		// first; these rows pin that they cannot slip through either.
		{"break inside range", "{{range .Title}}{{break}}{{end}}"},
		{"continue inside range", "{{range .Title}}{{continue}}{{end}}"},
		{"invalid UTF-8 string constant", `{{"\xff"}}`},
		{"nil", "{{eq .Title nil}}"},
		{"field chain", "{{(.Title).Foo}}"},
		{"func print", "{{print .Title}}"},
		{"func printf", `{{printf "%1000000000s" .Title}}`},
		{"func printf with %s", `{{printf "%s" .Title}}`},
		{"func println", "{{println .Title}}"},
		{"func slice", "{{slice .Title 0 1}}"},
		{"func html", "{{html .Title}}"},
		{"func js", "{{js .Title}}"},
		{"func urlquery", "{{urlquery .Title}}"},
		{"func call", "{{call .Title}}"},
		{"in if condition", "{{if .APIKey}}a{{end}}"},
		{"in if body", "{{if .Description}}{{.APIKey}}{{end}}"},
		{"in else body", "{{if .Title}}a{{else}}{{.APIKey}}{{end}}"},
		{"in else if condition", "{{if .Title}}a{{else if .APIKey}}b{{end}}"},
		{"in else if body", "{{if .Title}}a{{else if .Description}}{{.APIKey}}{{end}}"},
		{"in parentheses", `{{if (eq .APIKey "")}}a{{end}}`},
		{"in pipeline", `{{.Title | printf "%s"}}`},
	}
	// definitionRejected texts contain NAME, replaced by the name of the
	// overridden template. checkSyntax reports whether everything but the
	// definition is allowlisted syntax.
	definitionRejected := []struct {
		name, text  string
		checkSyntax bool
	}{
		{"define with another name", `{{define "x"}}a{{end}}b`, true},
		{"block", `{{block "x" .}}{{end}}`, false},
		{"define with own name, empty body", `{{define "NAME"}}a{{end}}`, true},
		{"empty define with own name", `a{{define "NAME"}}{{end}}`, true},
		// A define named like the probe parse is caught only by the first
		// parse, so each of the two parses is exercised on its own.
		{"define with probe name, empty body", `{{define "NAME` + probeNameSuffix + `"}}a{{end}}`, true},
		{"empty define with probe name", `a{{define "NAME` + probeNameSuffix + `"}}{{end}}`, true},
	}

	for _, target := range overrideTargets {
		for _, tc := range accepted {
			t.Run(target.name+"/accept/"+tc.name, func(t *testing.T) {
				w, err := New(&llmtestutil.FakeLLMClient{}, target.options(writeOverrideFile(t, tc.text)))
				requireAccepted(t, w, err)
			})
		}
		for _, tc := range syntaxRejected {
			t.Run(target.name+"/reject/"+tc.name, func(t *testing.T) {
				w, err := New(&llmtestutil.FakeLLMClient{}, target.options(writeOverrideFile(t, tc.text)))
				requireRejected(t, w, err, errDisallowedSyntax)
			})
		}
		for _, tc := range definitionRejected {
			text := strings.ReplaceAll(tc.text, "NAME", target.name)
			t.Run(target.name+"/reject/"+tc.name, func(t *testing.T) {
				if tc.checkSyntax {
					// Without the definition check, the syntax walk alone
					// would accept this text.
					tmpl, err := template.New(target.name).Parse(text)
					if err != nil {
						t.Fatalf("parse %q: %v", text, err)
					}
					if err := (syntaxChecker{tree: tmpl.Tree}).check(tmpl.Root); err != nil {
						t.Fatalf("syntax check of %q = %v, want nil", text, err)
					}
				}
				w, err := New(&llmtestutil.FakeLLMClient{}, target.options(writeOverrideFile(t, text)))
				requireRejected(t, w, err, errTemplateDefinition)
			})
		}
	}
}

func TestDefaultTemplatesPassChecks(t *testing.T) {
	w, err := New(&llmtestutil.FakeLLMClient{}, Options{})
	requireAccepted(t, w, err)

	// Each default is checked on its own, so breaking either one fails here
	// even if New were to skip one of them.
	defaults := []struct{ name, text string }{
		{systemTemplateName, prompts.System()},
		{userTemplateName, prompts.User()},
	}
	for _, d := range defaults {
		t.Run(d.name, func(t *testing.T) {
			if _, err := parseTemplate(templateSource{name: d.name}, d.text); err != nil {
				t.Errorf("default %s template fails the checks: %v", d.name, err)
			}
		})
	}
}

func TestDefaultSystemTemplateHeadingInstruction(t *testing.T) {
	for line := range strings.Lines(prompts.System()) {
		if strings.HasPrefix(line, "# ") {
			return
		}
	}
	t.Error("default system template has no line starting with \"# \" to show the title heading")
}

// TestInvalidTemplateErrorNamesSource checks that a rejection says which
// template failed and whether it was the embedded default or an override
// file, naming the file.
func TestInvalidTemplateErrorNamesSource(t *testing.T) {
	for _, target := range overrideTargets {
		t.Run(target.name+"/embedded default", func(t *testing.T) {
			_, err := parseTemplate(templateSource{name: target.name}, "")
			if want := target.name + " template (embedded default)"; err == nil || !strings.Contains(err.Error(), want) {
				t.Errorf("parseTemplate error = %v, want it to contain %q", err, want)
			}
		})
		t.Run(target.name+"/override file", func(t *testing.T) {
			path := writeOverrideFile(t, "")
			_, err := New(&llmtestutil.FakeLLMClient{}, target.options(path))
			if want := fmt.Sprintf("%s template (override file %q)", target.name, path); err == nil || !strings.Contains(err.Error(), want) {
				t.Errorf("New error = %v, want it to contain %q", err, want)
			}
		})
	}
}
