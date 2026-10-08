package publisher

import (
	"errors"
	"fmt"
)

// Sentinels for SlackWebhookPublisher. Preparation rejections are returned as
// they are; failures after preparation are wrapped in *SlackPostError.
var (
	ErrSlackHTTPStatus      = errors.New("webhook: unexpected HTTP status")
	ErrSlackInvalidResponse = errors.New("webhook: invalid response")
	ErrSlackTransport       = errors.New("webhook: transport failure")
	ErrSlackUnsplittable    = errors.New("webhook: the article cannot be split into messages")
	ErrSlackMention         = errors.New("webhook: the article contains text that may be treated as a mention or rewritten by the server")
)

// SlackHTTPStatusError reports a response whose status is not 200. Reason and
// RequestID are identifiers read from the response body, each empty when the
// body did not carry one in an accepted shape.
type SlackHTTPStatusError struct {
	StatusCode int
	Reason     string
	RequestID  string
}

func (e *SlackHTTPStatusError) Error() string {
	message := fmt.Sprintf("webhook: unexpected HTTP status %d", e.StatusCode)
	switch {
	case e.Reason != "" && e.RequestID != "":
		return fmt.Sprintf("%s (id: %s, request_id: %s)", message, e.Reason, e.RequestID)
	case e.Reason != "":
		return fmt.Sprintf("%s (id: %s)", message, e.Reason)
	case e.RequestID != "":
		return fmt.Sprintf("%s (request_id: %s)", message, e.RequestID)
	default:
		return message + "; the response body carried no error identifier"
	}
}

func (e *SlackHTTPStatusError) Unwrap() error { return ErrSlackHTTPStatus }

// SlackPostError reports a failure after preparation: Posted of Total
// messages were confirmed posted and later messages were not sent. Attempted
// tells whether the HTTP client was called for message Posted+1; when it was,
// that message may have been posted.
type SlackPostError struct {
	Total     int
	Posted    int
	Attempted bool
	Err       error
}

func (e *SlackPostError) Error() string {
	if e.Attempted {
		return fmt.Sprintf(
			"webhook: posted %d of %d messages; message %d failed and may have been posted; the remaining messages were not sent: %v",
			e.Posted, e.Total, e.Posted+1, e.Err,
		)
	}
	return fmt.Sprintf(
		"webhook: posted %d of %d messages; message %d and the rest were not sent: %v",
		e.Posted, e.Total, e.Posted+1, e.Err,
	)
}

func (e *SlackPostError) Unwrap() error { return e.Err }
