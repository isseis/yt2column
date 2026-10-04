// Package writer defines the article-writing stage: the article type and the
// interface that produces it from a transcript.
package writer

import (
	"context"

	"github.com/isseis/yt2column/internal/transcript"
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
