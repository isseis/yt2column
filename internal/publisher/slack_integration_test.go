//go:build integration

package publisher_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/isseis/yt2column/internal/publisher"
	publishertestutil "github.com/isseis/yt2column/internal/publisher/testutil"
	"github.com/isseis/yt2column/internal/writer"
)

// TestIntegrationSlackWebhookPublisher posts the two fixed articles to the
// real test Webhook (a Mattermost test channel): one that is a single message
// of exactly the per-message limit, then one that is split into several. It
// is excluded from `make test` by its build tag and is run by
// `make test-integration-slack`, which sets the opt-in variable. The Webhook
// URL is never written to the output: the publisher's errors never hold it,
// and the failure messages name the article only.
func TestIntegrationSlackWebhookPublisher(t *testing.T) {
	settings := publishertestutil.SettingsFrom(os.Getenv, publishertestutil.SlackIntegrationOptions)
	switch settings.Action {
	case publishertestutil.ActionRun:
	case publishertestutil.ActionFail:
		t.Fatal(settings.Reason)
	default:
		t.Skip(settings.Reason)
	}
	p, err := publisher.NewSlackWebhookPublisher(settings.WebhookURL)
	if err != nil {
		t.Fatalf("NewSlackWebhookPublisher() error = %v", err)
	}

	single, split := publishertestutil.IntegrationArticles(time.Now())
	articles := []struct {
		name     string
		article  writer.Article
		messages func(int) bool
		want     string
	}{
		{name: "single", article: single, messages: func(n int) bool { return n == 1 }, want: "1"},
		{name: "split", article: split, messages: func(n int) bool { return n >= 2 }, want: "2 or more"},
	}
	// Every count is checked before anything is posted, so a wrong article
	// never reaches the channel.
	for _, a := range articles {
		n, err := publisher.SlackMessageCount(a.article)
		if err != nil {
			t.Fatalf("SlackMessageCount(%s) error = %v", a.name, err)
		}
		if !a.messages(n) {
			t.Fatalf("SlackMessageCount(%s) = %d, want %s", a.name, n, a.want)
		}
		t.Logf("%s: %d message(s), title %q", a.name, n, a.article.Title)
	}
	for _, a := range articles {
		if err := p.Publish(context.Background(), a.article); err != nil {
			t.Fatalf("Publish(%s) error = %v", a.name, err)
		}
	}
}
