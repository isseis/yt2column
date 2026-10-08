//go:build test

package publisher

import (
	"net/http"
	"testing"
	"time"

	"github.com/isseis/yt2column/internal/loopbacktest"
	"github.com/isseis/yt2column/internal/secret"
	"github.com/isseis/yt2column/internal/writer"
)

// SlackTestOptions sets the timing of a test publisher. Timeout must be
// positive; Interval may be zero, which sends the messages without waiting.
type SlackTestOptions struct {
	Timeout  time.Duration
	Interval time.Duration
}

// NewSlackWebhookPublisherForLoopbackTest builds a publisher that posts to
// endpoint, which must be a loopback URL (loopbacktest.ValidateURL); any other
// endpoint, or a non-positive Timeout, fails t. It does not apply
// slackwebhook.ValidURL, so the endpoint may use http. Built only with the
// test tag.
func NewSlackWebhookPublisherForLoopbackTest(t testing.TB, opts SlackTestOptions, endpoint string) *SlackWebhookPublisher {
	t.Helper()
	return newSlackWebhookPublisherForLoopbackTest(t, opts, endpoint, nil)
}

// newSlackWebhookPublisherForLoopbackTest is NewSlackWebhookPublisherForLoopbackTest
// with a transport seam. The endpoint may hold a Webhook URL path, so a
// failure message never repeats the validation error.
func newSlackWebhookPublisherForLoopbackTest(t testing.TB, opts SlackTestOptions, endpoint string, transport http.RoundTripper) *SlackWebhookPublisher {
	t.Helper()
	if err := loopbacktest.ValidateURL(endpoint); err != nil {
		t.Fatal("slack test publisher: the endpoint is not a loopback URL")
		return nil
	}
	if opts.Timeout <= 0 {
		t.Fatal("slack test publisher: Timeout must be positive")
		return nil
	}
	value, err := secret.New(endpoint)
	if err != nil {
		t.Fatal("slack test publisher: the endpoint is empty")
		return nil
	}
	return newSlackWebhookPublisher(value, opts.Timeout, opts.Interval, transport)
}

// SlackMessagesForTest returns the text of every message Publish sends for
// article. It runs the same preparation and touches no network, so a test in
// another package can pin the length and shape of the messages. Built only
// with the test tag.
func SlackMessagesForTest(article writer.Article) ([]string, error) {
	return prepareSlackMessages(article)
}
