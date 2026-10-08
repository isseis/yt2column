//go:build test

package publisher

import (
	"errors"
	"strings"
	"testing"
)

func TestSlackPostErrorMessage(t *testing.T) {
	cases := []struct {
		name      string
		err       *SlackPostError
		wantParts []string
		notParts  []string
	}{
		{
			name:      "attempted",
			err:       &SlackPostError{Total: 3, Posted: 1, Attempted: true, Err: ErrSlackTransport},
			wantParts: []string{"posted 1 of 3", "message 2", "may have been posted", ErrSlackTransport.Error()},
		},
		{
			name:      "not attempted",
			err:       &SlackPostError{Total: 3, Posted: 1, Attempted: false, Err: ErrSlackTransport},
			wantParts: []string{"posted 1 of 3", "message 2", "were not sent", ErrSlackTransport.Error()},
			notParts:  []string{"may have been posted"},
		},
		{
			name:      "single message",
			err:       &SlackPostError{Total: 1, Posted: 0, Attempted: true, Err: ErrSlackInvalidResponse},
			wantParts: []string{"posted 0 of 1", "message 1", "may have been posted"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.err.Error()
			for _, want := range tc.wantParts {
				if !strings.Contains(got, want) {
					t.Fatalf("Error() = %q, want it to contain %q", got, want)
				}
			}
			for _, unwanted := range tc.notParts {
				if strings.Contains(got, unwanted) {
					t.Fatalf("Error() = %q, want it not to contain %q", got, unwanted)
				}
			}
		})
	}
}

func TestSlackPostErrorFields(t *testing.T) {
	err := error(&SlackPostError{Total: 3, Posted: 1, Attempted: true, Err: ErrSlackTransport})

	got, ok := errors.AsType[*SlackPostError](err)
	if !ok {
		t.Fatalf("errors.AsType[*SlackPostError] = false, want true")
	}
	if got.Total != 3 || got.Posted != 1 || !got.Attempted {
		t.Fatalf("fields = %d/%d/%v, want 3/1/true", got.Total, got.Posted, got.Attempted)
	}
	if !errors.Is(err, ErrSlackTransport) {
		t.Fatalf("errors.Is(err, ErrSlackTransport) = false, want true")
	}
}

func TestSlackHTTPStatusErrorMessage(t *testing.T) {
	cases := []struct {
		name      string
		err       *SlackHTTPStatusError
		wantParts []string
	}{
		{
			name:      "status only",
			err:       &SlackHTTPStatusError{StatusCode: 400},
			wantParts: []string{"400", "no error identifier"},
		},
		{
			name:      "reason",
			err:       &SlackHTTPStatusError{StatusCode: 400, Reason: "web.incoming_webhook.parse.app_error"},
			wantParts: []string{"400", "web.incoming_webhook.parse.app_error"},
		},
		{
			name:      "request id",
			err:       &SlackHTTPStatusError{StatusCode: 500, RequestID: "0123456789abcdefghijklmnop"},
			wantParts: []string{"500", "0123456789abcdefghijklmnop"},
		},
		{
			name:      "reason and request id",
			err:       &SlackHTTPStatusError{StatusCode: 403, Reason: "web.incoming_webhook.general.app_error", RequestID: "0123456789abcdefghijklmnop"},
			wantParts: []string{"403", "web.incoming_webhook.general.app_error", "0123456789abcdefghijklmnop"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.err.Error()
			for _, want := range tc.wantParts {
				if !strings.Contains(got, want) {
					t.Fatalf("Error() = %q, want it to contain %q", got, want)
				}
			}
			if !errors.Is(tc.err, ErrSlackHTTPStatus) {
				t.Fatalf("errors.Is(err, ErrSlackHTTPStatus) = false, want true")
			}
		})
	}
}
