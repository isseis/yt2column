// Package publisher defines the publishing stage: the Publisher interface,
// which FilePublisher implements by saving an article to a local file, and
// the Slack-compatible Webhook sender. A Webhook post is not a Publisher: an
// article is checked and split by PrepareSlackMessages, and the messages are
// sent by SlackWebhookPublisher.PostMessages.
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
