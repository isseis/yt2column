// Package writer defines the article-writing stage: the article type and the
// interface that produces it from a transcript.
package writer

import (
	"context"
	"errors"
	"text/template"

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
	system *template.Template
	user   *template.Template
}

// New validates client and the templates and returns an ArticleWriter.
// It never reads environment variables. The concrete type is unexported,
// so an ArticleWriter can only be obtained through New.
func New(client llm.LLMClient, opts Options) (ArticleWriter, error) {
	if nilcheck.IsNil(client) {
		return nil, errNilLLMClient
	}
	system, err := loadTemplate(templateSource{name: systemTemplateName, path: opts.SystemTemplatePath}, prompts.System())
	if err != nil {
		return nil, err
	}
	user, err := loadTemplate(templateSource{name: userTemplateName, path: opts.UserTemplatePath}, prompts.User())
	if err != nil {
		return nil, err
	}
	return &templateWriter{client: client, system: system, user: user}, nil
}

// errWriteNotImplemented is returned by the provisional Write below.
var errWriteNotImplemented = errors.New("article generation is not implemented yet")

// Write is a provisional implementation that always fails, so no unchecked
// article can be returned. It is replaced by the generation path (plan step
// 3-3) before this change is merged.
func (w *templateWriter) Write(_ context.Context, _ transcript.Transcript) (Article, error) {
	return Article{}, errWriteNotImplemented
}
