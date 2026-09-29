// Package publisher defines the publishing stage: the interface that outputs an
// article to its destination.
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
