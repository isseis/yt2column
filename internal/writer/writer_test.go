//go:build test

package writer

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/isseis/yt2column/internal/llm"
	"github.com/isseis/yt2column/internal/llm/testutil"
	"github.com/isseis/yt2column/internal/transcript"
	"github.com/isseis/yt2column/prompts"
)

func TestNewRejectsNilClient(t *testing.T) {
	cases := []struct {
		name   string
		client llm.LLMClient
	}{
		{"nil interface", nil},
		{"typed nil", (*llmtestutil.FakeLLMClient)(nil)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w, err := New(tc.client, Options{})
			if !errors.Is(err, errNilLLMClient) {
				t.Errorf("New error = %v, want %v", err, errNilLLMClient)
			}
			if w != nil {
				t.Errorf("New returned a writer %v, want nil", w)
			}
		})
	}
}

// writeRequest builds a writer with opts, writes validTranscript, and returns
// the request the LLM client received.
func writeRequest(t *testing.T, opts Options) llm.GenerateRequest {
	t.Helper()
	client := newFakeClient()
	w := mustNew(t, client, opts)
	if _, err := w.Write(context.Background(), validTranscript()); err != nil {
		t.Fatalf("Write error = %v", err)
	}
	if len(client.Calls) != 1 {
		t.Fatalf("Generate called %d times, want 1", len(client.Calls))
	}
	return client.Calls[0].Request
}

func TestNewUsesDefaultTemplates(t *testing.T) {
	// Overriding with the embedded text itself must change nothing, which
	// shows the default path uses the embedded templates without repeating
	// their expansion here.
	got := writeRequest(t, Options{})
	want := writeRequest(t, Options{
		SystemTemplatePath: writeOverrideFile(t, prompts.System()),
		UserTemplatePath:   writeOverrideFile(t, prompts.User()),
	})
	if got != want {
		t.Errorf("default request = %+v, want %+v", got, want)
	}
}

func TestNewOverridesEachTemplate(t *testing.T) {
	defaults := writeRequest(t, Options{})
	const override = "override {{.Title}}"
	wantOverride := "override " + markTitle

	t.Run("system only", func(t *testing.T) {
		got := writeRequest(t, Options{SystemTemplatePath: writeOverrideFile(t, override)})
		if got.SystemPrompt != wantOverride {
			t.Errorf("system prompt = %q, want %q", got.SystemPrompt, wantOverride)
		}
		if got.UserPrompt != defaults.UserPrompt {
			t.Errorf("user prompt = %q, want the default %q", got.UserPrompt, defaults.UserPrompt)
		}
	})
	t.Run("user only", func(t *testing.T) {
		got := writeRequest(t, Options{UserTemplatePath: writeOverrideFile(t, override)})
		if got.UserPrompt != wantOverride {
			t.Errorf("user prompt = %q, want %q", got.UserPrompt, wantOverride)
		}
		if got.SystemPrompt != defaults.SystemPrompt {
			t.Errorf("system prompt = %q, want the default %q", got.SystemPrompt, defaults.SystemPrompt)
		}
	})
}

func TestNewReadsOverrideOnce(t *testing.T) {
	changes := []struct {
		name   string
		change func(t *testing.T, path string)
	}{
		{"rewritten", func(t *testing.T, path string) {
			if err := os.WriteFile(path, []byte("changed {{.Title}}"), 0o600); err != nil {
				t.Fatalf("rewrite %s: %v", path, err)
			}
		}},
		{"removed", func(t *testing.T, path string) {
			if err := os.Remove(path); err != nil {
				t.Fatalf("remove %s: %v", path, err)
			}
		}},
	}
	for _, tc := range changes {
		t.Run(tc.name, func(t *testing.T) {
			path := writeOverrideFile(t, "original {{.Title}}")
			client := newFakeClient()
			w := mustNew(t, client, Options{SystemTemplatePath: path})
			tc.change(t, path)
			if _, err := w.Write(context.Background(), validTranscript()); err != nil {
				t.Fatalf("Write error = %v", err)
			}
			if got, want := client.Calls[0].Request.SystemPrompt, "original "+markTitle; got != want {
				t.Errorf("system prompt = %q, want %q", got, want)
			}
		})
	}
}

func TestDefaultTemplatesEmbedAllValues(t *testing.T) {
	req := writeRequest(t, Options{})
	both := req.SystemPrompt + req.UserPrompt
	for _, mark := range []string{markTitle, markChannel, markDescription, markSegment} {
		if !strings.Contains(both, mark) {
			t.Errorf("default prompts do not embed the value marked %q", mark)
		}
	}
}

func TestWriteCallsGenerateOnce(t *testing.T) {
	type ctxKey struct{}
	ctx := context.WithValue(context.Background(), ctxKey{}, "write-ctx")
	client := newFakeClient()
	w := mustNew(t, client, Options{})
	if _, err := w.Write(ctx, validTranscript()); err != nil {
		t.Fatalf("Write error = %v", err)
	}
	if len(client.Calls) != 1 {
		t.Fatalf("Generate called %d times, want 1", len(client.Calls))
	}
	call := client.Calls[0]
	if call.Ctx.Value(ctxKey{}) != "write-ctx" {
		t.Error("Generate did not receive the ctx passed to Write")
	}
	if call.Request.MaxOutputTokens != 0 {
		t.Errorf("MaxOutputTokens = %d, want 0", call.Request.MaxOutputTokens)
	}
}

func TestWriteReportsLLMError(t *testing.T) {
	errProvider := errors.New("provider-specific failure")
	cases := []struct {
		name   string
		target error
		result llm.GenerateResponse
	}{
		{"truncated", llm.ErrTruncated, llm.GenerateResponse{}},
		{"empty response", llm.ErrEmptyResponse, llm.GenerateResponse{}},
		{"deadline", context.DeadlineExceeded, llm.GenerateResponse{}},
		{"provider sentinel", errProvider, llm.GenerateResponse{}},
		{"error with valid text", llm.ErrTruncated, validResponse()},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			client := &llmtestutil.FakeLLMClient{Result: tc.result, Err: fmt.Errorf("fake: %w", tc.target)}
			w := mustNew(t, client, Options{})
			a, err := w.Write(context.Background(), validTranscript())
			if !errors.Is(err, tc.target) {
				t.Errorf("Write error = %v, want it to match %v", err, tc.target)
			}
			for _, sentinel := range []error{ErrInvalidTemplate, ErrInvalidTranscript, ErrMalformedOutput} {
				if errors.Is(err, sentinel) {
					t.Errorf("Write error = %v, want it not to match %v", err, sentinel)
				}
			}
			if a != (Article{}) {
				t.Errorf("Write article = %+v, want the zero value", a)
			}
			if len(client.Calls) != 1 {
				t.Errorf("Generate called %d times, want 1", len(client.Calls))
			}
		})
	}
}

func TestWriteContextDone(t *testing.T) {
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	expired, cancelExpired := context.WithDeadline(context.Background(), time.Unix(0, 0))
	t.Cleanup(cancelExpired)
	contexts := []struct {
		name string
		ctx  context.Context
		want error
	}{
		{"canceled", canceled, context.Canceled},
		{"deadline exceeded", expired, context.DeadlineExceeded},
	}
	// Each input would be rejected on its own, so only a ctx check that runs
	// before transcript validation and expansion reports the ctx error.
	inputs := []struct {
		name string
		opts func(t *testing.T) Options
		tr   func() transcript.Transcript
	}{
		{"invalid transcript", func(*testing.T) Options { return Options{} }, func() transcript.Transcript {
			tr := validTranscript()
			tr.Segments = nil
			return tr
		}},
		{"failing expansion", func(t *testing.T) Options {
			return Options{SystemTemplatePath: writeOverrideFile(t, "{{index .Title 100}}")}
		}, validTranscript},
	}
	for _, c := range contexts {
		for _, in := range inputs {
			t.Run(c.name+"/"+in.name, func(t *testing.T) {
				client := newFakeClient()
				w := mustNew(t, client, in.opts(t))
				a, err := w.Write(c.ctx, in.tr())
				if !errors.Is(err, c.want) {
					t.Errorf("Write error = %v, want %v", err, c.want)
				}
				if a != (Article{}) {
					t.Errorf("Write article = %+v, want the zero value", a)
				}
				if len(client.Calls) != 0 {
					t.Errorf("Generate called %d times, want 0", len(client.Calls))
				}
			})
		}
	}
}
