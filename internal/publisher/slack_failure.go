package publisher

import (
	"context"
	"errors"
	"fmt"
	"regexp"

	"github.com/isseis/yt2column/internal/failure"
	"github.com/isseis/yt2column/internal/secret"
	"github.com/isseis/yt2column/internal/transcript"
)

// errZeroNotifier names a Notify call on a nil or zero-value notifier, which
// holds no webhook URL. It is unexported: the only way to reach it is a caller
// building the notifier wrong, and a caller only needs to log it.
var errZeroNotifier = errors.New("webhook: the notifier has no webhook URL")

// requestIDPattern matches the RequestID the server generates: 16 lowercase
// hexadecimal characters.
var requestIDPattern = regexp.MustCompile(`^[0-9a-f]{16}$`)

// SlackFailureNotifier posts a failure.Notice to a Slack-compatible Incoming
// Webhook (Mattermost is the verified target) as one silent message. It holds
// the webhook URL only inside its webhookClient and has no logger or writer:
// every failure is returned, never printed. Its fields are set once by the
// constructor and never mutated afterwards, so it is safe for concurrent use.
// Notify on the zero value or a nil pointer fails without sending.
type SlackFailureNotifier struct {
	client webhookClient
}

var _ failure.Notifier = (*SlackFailureNotifier)(nil)

// NewSlackFailureNotifier validates webhookURL with checkWebhookURL, as
// NewSlackWebhookPublisher does, and returns a notifier whose send is bounded
// by SlackPostTimeout. It never reads environment variables and never touches
// the network. A rejection never holds the URL or any part of it.
func NewSlackFailureNotifier(webhookURL secret.Secret) (*SlackFailureNotifier, error) {
	if err := checkWebhookURL(webhookURL); err != nil {
		return nil, err
	}
	return &SlackFailureNotifier{client: newWebhookClient(webhookURL, SlackPostTimeout, nil)}, nil
}

// Notify checks notice, builds the message text from the fixed sentence of
// notice.Kind, notice.RequestID, and the normalized URL of notice.VideoID,
// and posts it once. It never retries and never writes any output. An invalid
// notice is reported as ErrSlackInvalidNotice whatever ctx holds; a context
// that has already ended is reported before sending.
func (s *SlackFailureNotifier) Notify(ctx context.Context, notice failure.Notice) error {
	if s == nil || s.client.configured() != nil {
		return errZeroNotifier
	}
	text, err := failureNoticeText(notice)
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("webhook: the failure notice was not sent: %w", err)
	}
	if err := s.client.post(ctx, text); err != nil {
		if errors.Is(err, ErrSlackHTTPStatus) || errors.Is(err, ErrSlackInvalidResponse) {
			return fmt.Errorf("%w: %w", ErrSlackNoticeRejected, err)
		}
		return fmt.Errorf("webhook: the failure notice may have been posted: %w", err)
	}
	return nil
}

// failureNoticeText builds the message text for notice. It rejects a RequestID
// that is not 16 lowercase hexadecimal characters and a non-empty VideoID that
// transcript.NormalizedVideoURL does not accept, naming the field but never
// echoing its value.
func failureNoticeText(notice failure.Notice) (string, error) {
	if !requestIDPattern.MatchString(notice.RequestID) {
		return "", fmt.Errorf("%w: the request ID is not 16 lowercase hexadecimal characters", ErrSlackInvalidNotice)
	}
	text := "yt2column failure: " + failureSentence(notice.Kind) + "\nrequest: " + notice.RequestID
	if notice.VideoID == "" {
		return text, nil
	}
	url, ok := transcript.NormalizedVideoURL(notice.VideoID)
	if !ok {
		return "", fmt.Errorf("%w: the video ID is not 11 characters of [A-Za-z0-9_-]", ErrSlackInvalidNotice)
	}
	return text + "\nvideo: <" + url + ">", nil
}

// failureSentence returns the fixed sentence for kind. The switch names every
// non-zero failure.Kind; KindUnknown and any value it does not name fall to
// the default, so such a kind is still posted (as unclassified), not dropped.
func failureSentence(kind failure.Kind) string {
	switch kind {
	case failure.KindBodyTooLarge:
		return "the request body exceeded the size limit"
	case failure.KindInvalidBody:
		return "the request body was not a valid request"
	case failure.KindInvalidVideoURL:
		return "the video URL was not accepted"
	case failure.KindInvalidTranscript:
		return "the copied transcript was not in the expected format"
	case failure.KindTranscriptTooLong:
		return "the transcript was too long for the prompt limit"
	case failure.KindLLMTimeout:
		return "the LLM call timed out"
	case failure.KindLLMEmpty:
		return "the LLM returned an empty response"
	case failure.KindLLMTruncated:
		return "the LLM response was cut off before it finished"
	case failure.KindLLM:
		return "the LLM call failed"
	case failure.KindPublish:
		return "posting the article failed; part of it may already be in the channel"
	case failure.KindPublishMention:
		return "part of the article could be treated as a mention, so it was not posted; sending it again generates a different article"
	case failure.KindInternal:
		return "an internal error occurred"
	default:
		return "the request failed for an unclassified reason"
	}
}
