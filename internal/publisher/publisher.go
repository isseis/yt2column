// Package publisher defines the publishing stage: the Publisher interface,
// which FilePublisher implements by saving an article to a local file, and
// the renderers of an article's display Markdown, with its mentions
// neutralized, for the Viewer.
package publisher

import (
	"context"

	"github.com/isseis/yt2column/internal/writer"
)

// Publisher outputs an article to the destination.
// Implementations must return an error on failure and must not
// publish incomplete content.
type Publisher interface {
	Publish(ctx context.Context, article writer.Article) error
}
