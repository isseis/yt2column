package server

import (
	"context"

	"github.com/isseis/yt2column/internal/publisher"
)

// WebhookPublisher posts to the Webhook: Publish for the legacy /articles job,
// PostMessages for the publish operation. The production value is
// *publisher.SlackWebhookPublisher, and a test uses the real publisher against
// a loopback Webhook or a fake of this interface.
type WebhookPublisher interface {
	publisher.Publisher
	PostMessages(ctx context.Context, m publisher.SlackMessages, from int, delivered func(index int)) error
}
