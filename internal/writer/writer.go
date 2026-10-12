// Package writer defines the article-writing stage: the article type and the
// interface that produces it from a transcript.
package writer

import (
	"context"
	"fmt"

	"github.com/isseis/yt2column/internal/llm"
	"github.com/isseis/yt2column/internal/nilcheck"
	"github.com/isseis/yt2column/internal/transcript"
	"github.com/isseis/yt2column/prompts"
)

// Article is the generated column article.
type Article struct {
	Title        string
	Body         string // Markdown
	SourceURL    string
	Model        string
	ModelVersion string // opaque, provider-defined; empty when the provider reports none
}

// StepModel is the model one step's LLM call reported.
type StepModel struct {
	Model        string
	ModelVersion string // empty when the provider reports none
}

// ArticleWriter generates a column article from a transcript.
// Implementations must return an error on failure and must not
// return an empty Article as a successful result.
type ArticleWriter interface {
	Write(ctx context.Context, t transcript.Transcript) (Article, error)
}

// Options configures the templates of the ArticleWriter returned by New.
// An empty path selects the embedded default template; a non-empty path
// names an override file that New reads once.
type Options struct {
	SystemTemplatePath string
	UserTemplatePath   string
}

// templateWriter is the ArticleWriter returned by New. Its fields are set
// once by New and never mutated afterwards.
type templateWriter struct {
	client llm.LLMClient
	system checkedTemplate[templateData]
	user   checkedTemplate[templateData]
}

// New validates client and the templates and returns an ArticleWriter.
// It never reads environment variables. The concrete type is unexported,
// so an ArticleWriter can only be obtained through New.
func New(client llm.LLMClient, opts Options) (ArticleWriter, error) {
	if nilcheck.IsNil(client) {
		return nil, errNilLLMClient
	}
	system, err := loadTemplate[templateData](templateSource{name: systemTemplateName, path: opts.SystemTemplatePath}, prompts.System())
	if err != nil {
		return nil, err
	}
	user, err := loadTemplate[templateData](templateSource{name: userTemplateName, path: opts.UserTemplatePath}, prompts.User())
	if err != nil {
		return nil, err
	}
	return &templateWriter{client: client, system: system, user: user}, nil
}

// Write validates t, expands both templates, calls the LLM client once, and
// validates its response before building the article. It returns the zero
// Article on every failure. The LLM client's error is wrapped as is, never
// with a sentinel of this package, so callers can tell it from a template or
// output failure.
func (w *templateWriter) Write(ctx context.Context, t transcript.Transcript) (Article, error) {
	if err := ctx.Err(); err != nil {
		return Article{}, fmt.Errorf("article generation not started: %w", err)
	}
	sourceURL, err := validateTranscript(t)
	if err != nil {
		return Article{}, err
	}
	data := newTemplateData(t)
	system, err := w.system.expand(data)
	if err != nil {
		return Article{}, err
	}
	user, err := w.user.expand(data)
	if err != nil {
		return Article{}, err
	}
	resp, err := w.client.Generate(ctx, llm.GenerateRequest{SystemPrompt: system, UserPrompt: user, MaxOutputTokens: 0})
	if err != nil {
		return Article{}, fmt.Errorf("LLM generation failed: %w", err)
	}
	title, body, err := checkResponse(resp)
	if err != nil {
		return Article{}, err
	}
	return newArticle(title, body, sourceURL, resp), nil
}
