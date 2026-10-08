package publisher

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/isseis/yt2column/internal/secret"
	"github.com/isseis/yt2column/internal/slackwebhook"
	"github.com/isseis/yt2column/internal/strictjson"
	"github.com/isseis/yt2column/internal/writer"
)

// SlackPostTimeout bounds the send and the response read of one message.
const SlackPostTimeout = 30 * time.Second

const (
	// slackMessageInterval is how long to wait between two messages, so the
	// Webhook stays under the Slack and Mattermost rate limits.
	slackMessageInterval = time.Second
	// slackMaxResponseBytes bounds the response body. One byte past the limit
	// is read to detect an oversized body without buffering it all.
	slackMaxResponseBytes = 8192
	// slackOKResponse is the only 200 response body that counts as success.
	slackOKResponse = "ok"
)

// withheldTransportDetails replaces a transport error message that may carry
// part of the Webhook URL, so the whole message is dropped rather than partly
// redacted.
const withheldTransportDetails = "details withheld because they may contain the webhook URL"

// Static errors that name a failure before any message is sent and never hold
// the Webhook URL.
var (
	errZeroWebhookURL    = errors.New("webhook: the publisher has no webhook URL")
	errInvalidWebhookURL = errors.New("webhook: the webhook URL is not accepted")
	errSlackBuildRequest = errors.New("webhook: the request could not be built")
	errSlackPostTimeout  = errors.New("webhook: the post timeout elapsed")
)

// Response body members read to name a Mattermost AppError.
const (
	keySlackErrorID        = "id"
	keySlackErrorRequestID = "request_id"
)

// Identifier shapes accepted from a response body. Mattermost names an error
// with an id such as web.incoming_webhook.parse.app_error and a 26-character
// request id; Slack may answer with a short plain-text reason.
var (
	slackReasonPattern      = regexp.MustCompile(`^[a-z0-9._]{1,128}$`)
	slackRequestIDPattern   = regexp.MustCompile(`^[a-z0-9]{26}$`)
	slackPlainReasonPattern = regexp.MustCompile(`^[a-z0-9_]{1,64}$`)
)

// webhookPayload is the JSON body of one message. Only the members common to
// Mattermost and Slack are sent; silent is Mattermost's and is ignored by
// Slack and by Mattermost servers that predate it.
type webhookPayload struct {
	Text   string `json:"text"`
	Silent bool   `json:"silent"` // always true
}

// SlackWebhookPublisher posts an article to a Slack-compatible Incoming
// Webhook (Mattermost is the verified target) as one or more messages. It
// holds the webhook URL only as a secret.Secret, so printing the struct never
// shows the URL. Its fields are set once by the constructor and never mutated
// afterwards. Publish on the zero value fails without sending.
type SlackWebhookPublisher struct {
	webhookURL secret.Secret
	timeout    time.Duration
	interval   time.Duration
	httpClient *http.Client
}

var _ Publisher = (*SlackWebhookPublisher)(nil)

// NewSlackWebhookPublisher validates webhookURL with slackwebhook.ValidURL
// and returns a publisher with the production timeout and interval. It never
// reads environment variables and never touches the network. A rejection
// never holds the URL or any part of it.
func NewSlackWebhookPublisher(webhookURL secret.Secret) (*SlackWebhookPublisher, error) {
	value, err := webhookURL.Reveal()
	if err != nil {
		return nil, errZeroWebhookURL
	}
	if !slackwebhook.ValidURL(value) {
		return nil, errInvalidWebhookURL
	}
	return newSlackWebhookPublisher(webhookURL, SlackPostTimeout, slackMessageInterval, nil), nil
}

// newSlackWebhookPublisher builds a publisher around webhookURL. transport, when
// non-nil, replaces the default transport; it lets a test observe or fail the
// send. A 3xx response is returned as the response instead of being followed,
// so the Webhook URL is never sent to a redirect target.
func newSlackWebhookPublisher(webhookURL secret.Secret, timeout, interval time.Duration, transport http.RoundTripper) *SlackWebhookPublisher {
	return &SlackWebhookPublisher{
		webhookURL: webhookURL,
		timeout:    timeout,
		interval:   interval,
		httpClient: &http.Client{
			Transport: transport,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
}

// Publish checks article, rejects mention syntax and syntax the server would
// rewrite, splits it into messages, and posts them in order with silent set,
// waiting between messages. It sends nothing when a preparation check fails.
// After preparation, every failure is a *SlackPostError that tells how many
// of the messages were confirmed posted.
func (p *SlackWebhookPublisher) Publish(ctx context.Context, article writer.Article) error {
	webhookURL, err := p.webhookURL.Reveal()
	if err != nil {
		return errZeroWebhookURL
	}
	messages, err := prepareSlackMessages(article)
	if err != nil {
		return err
	}
	parts := slackwebhook.SensitiveParts(webhookURL)

	total := len(messages)
	posted := 0
	for i, message := range messages {
		if i > 0 {
			if err := waitContext(ctx, p.interval); err != nil {
				return &SlackPostError{Total: total, Posted: posted, Attempted: false, Err: err}
			}
		}
		if err := ctx.Err(); err != nil {
			return &SlackPostError{Total: total, Posted: posted, Attempted: false, Err: err}
		}
		if err := p.postMessage(ctx, webhookURL, message, parts); err != nil {
			return &SlackPostError{Total: total, Posted: posted, Attempted: true, Err: err}
		}
		posted++
	}
	return nil
}

// postMessage sends one message and validates the response. Its timeout covers
// the send and the whole body read; http.Client.Timeout is deliberately not
// used, so a timeout is distinguishable from another transport failure.
func (p *SlackWebhookPublisher) postMessage(parent context.Context, webhookURL, message string, parts []string) error {
	callCtx, cancel := context.WithTimeoutCause(parent, p.timeout, errSlackPostTimeout)
	defer cancel()

	body, err := json.Marshal(webhookPayload{Text: message, Silent: true})
	if err != nil {
		return errSlackBuildRequest
	}
	request, err := newSlackRequest(callCtx, webhookURL, body)
	if err != nil {
		return errSlackBuildRequest
	}
	response, err := p.httpClient.Do(request)
	if err != nil {
		return p.postFailure(callCtx, err, parts)
	}
	defer func() { _ = response.Body.Close() }()

	data, readErr := readSlackResponse(response.Body)
	if response.StatusCode == http.StatusOK {
		if readErr != nil {
			return p.postFailure(callCtx, readErr, parts)
		}
		if len(data) > slackMaxResponseBytes || string(data) != slackOKResponse {
			return fmt.Errorf("%w: the server did not acknowledge the post with %q", ErrSlackInvalidResponse, slackOKResponse)
		}
		return nil
	}

	// A non-200 status is a failure whatever the body holds, so a body read
	// failure only drops the identifiers.
	statusErr := &SlackHTTPStatusError{StatusCode: response.StatusCode}
	if readErr == nil && len(data) <= slackMaxResponseBytes {
		statusErr.Reason, statusErr.RequestID = slackErrorIdentifiers(data, parts)
	}
	return statusErr
}

// newSlackRequest builds the POST request. The URL was validated when the
// publisher was built, so a failure here is not expected; the *url.Error it
// would carry is never returned because it holds the Webhook URL.
func newSlackRequest(ctx context.Context, webhookURL string, body []byte) (*http.Request, error) {
	//nolint:gosec // webhookURL is validated at construction; a test helper may use a loopback http URL
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, webhookURL, bytes.NewReader(body))
	if err != nil {
		return nil, errSlackBuildRequest
	}
	request.Header.Set("Content-Type", "application/json")
	return request, nil
}

// readSlackResponse reads at most one byte past the limit, which is enough to
// detect an oversized body without buffering it all.
func readSlackResponse(body io.Reader) ([]byte, error) {
	return io.ReadAll(io.LimitReader(body, slackMaxResponseBytes+1))
}

// postFailure maps a send or body-read failure. The call context is checked
// first, so a timeout or cancellation is reported as such even when the
// transport returns another error. A transport cause is never %w-chained:
// the returned error must match exactly one sentinel, and the cause could
// itself contain a context error.
func (p *SlackWebhookPublisher) postFailure(callCtx context.Context, cause error, parts []string) error {
	if err := callCtx.Err(); err != nil {
		if errors.Is(context.Cause(callCtx), errSlackPostTimeout) {
			return fmt.Errorf("webhook: the post timed out after %s: %w", p.timeout, err)
		}
		return fmt.Errorf("webhook: the post was interrupted: %w", err)
	}
	return slackTransportError(cause, parts)
}

// slackTransportError keeps the inner error's message so connection refusals,
// name resolution failures, and TLS failures stay distinguishable, but drops
// the whole message when it may hold part of the Webhook URL.
func slackTransportError(cause error, parts []string) error {
	inner := cause
	if urlErr, ok := errors.AsType[*url.Error](cause); ok && urlErr.Err != nil {
		inner = urlErr.Err
	}
	details := inner.Error()
	if containsSensitivePart(details, parts) {
		details = withheldTransportDetails
	}
	return fmt.Errorf("%w: %s", ErrSlackTransport, details)
}

// slackErrorIdentifiers reads the error identifiers a non-200 response may
// carry: a Mattermost AppError's id and request_id, or a plain-text reason.
// A value that could hold or be held by a sensitive part of the Webhook URL is
// dropped, so the URL never appears in an error.
func slackErrorIdentifiers(data []byte, parts []string) (reason, requestID string) {
	if object, err := strictjson.ParseObject(data); err == nil {
		if members, err := object.Collect(keySlackErrorID, keySlackErrorRequestID); err == nil {
			if value, ok := members[keySlackErrorID]; ok {
				if text, err := value.AsString(); err == nil && slackReasonPattern.MatchString(text) {
					reason = text
				}
			}
			if value, ok := members[keySlackErrorRequestID]; ok {
				if text, err := value.AsString(); err == nil && slackRequestIDPattern.MatchString(text) {
					requestID = text
				}
			}
		}
	}
	if reason == "" {
		if text := string(data); slackPlainReasonPattern.MatchString(text) {
			reason = text
		}
	}
	if containsSensitivePart(reason, parts) {
		reason = ""
	}
	if containsSensitivePart(requestID, parts) {
		requestID = ""
	}
	return reason, requestID
}

// containsSensitivePart reports whether text holds or is held by any sensitive
// part. A part shorter than 8 bytes is never produced, so short strings are
// not redacted by accident.
func containsSensitivePart(text string, parts []string) bool {
	if text == "" {
		return false
	}
	for _, part := range parts {
		if strings.Contains(text, part) || strings.Contains(part, text) {
			return true
		}
	}
	return false
}

// waitContext waits for d, or returns ctx's error when it ends first. A
// non-positive d waits for nothing and only reports an already-ended ctx.
func waitContext(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
