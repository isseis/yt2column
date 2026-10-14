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

// newLoopbackWebhookClient validates a loopback endpoint and builds the shared
// send path for it. endpoint may hold a Webhook URL path, so a failure message
// never repeats the validation error. It calls t.Fatal and returns false when
// the endpoint is not loopback, the timeout is not positive, or the endpoint is
// empty, so a caller can return nil instead of a half-built client.
func newLoopbackWebhookClient(t testing.TB, timeout time.Duration, endpoint string, transport http.RoundTripper) (webhookClient, bool) {
	t.Helper()
	if err := loopbacktest.ValidateURL(endpoint); err != nil {
		t.Fatal("slack test client: the endpoint is not a loopback URL")
		return webhookClient{}, false
	}
	if timeout <= 0 {
		t.Fatal("slack test client: Timeout must be positive")
		return webhookClient{}, false
	}
	value, err := secret.New(endpoint)
	if err != nil {
		t.Fatal("slack test client: the endpoint is empty")
		return webhookClient{}, false
	}
	return newWebhookClient(value, timeout, transport), true
}

// newSlackWebhookPublisherForLoopbackTest is NewSlackWebhookPublisherForLoopbackTest
// with a transport seam.
func newSlackWebhookPublisherForLoopbackTest(t testing.TB, opts SlackTestOptions, endpoint string, transport http.RoundTripper) *SlackWebhookPublisher {
	t.Helper()
	client, ok := newLoopbackWebhookClient(t, opts.Timeout, endpoint, transport)
	if !ok {
		return nil
	}
	return newSlackWebhookPublisher(opts.Interval, client)
}

// SlackMessagesForTest returns the text of every message Publish sends for
// article. It runs the same preparation and touches no network, so a test in
// another package can pin the length and shape of the messages. Built only
// with the test tag.
func SlackMessagesForTest(article writer.Article) ([]string, error) {
	return prepareSlackMessages(article)
}

// Limits of the preparation stage, for tests in other packages that size an
// article or a timeout. Built only with the test tag.
const (
	SlackMaxMessageRunesForTest = slackMaxMessageRunes
	SlackMaxMessagesForTest     = slackMaxMessages
)

// SlackPublishMaxDurationForTest returns the longest a production publisher
// can take to post messages messages (at least one): each send is bounded by
// SlackPostTimeout, and the production interval separates consecutive sends.
// Built only with the test tag.
func SlackPublishMaxDurationForTest(messages int) time.Duration {
	return time.Duration(messages)*SlackPostTimeout + time.Duration(messages-1)*slackMessageInterval
}
