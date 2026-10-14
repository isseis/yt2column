//go:build test

package publisher

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/isseis/yt2column/internal/failure"
	"github.com/isseis/yt2column/internal/secret"
	"github.com/isseis/yt2column/internal/slackwebhook"
	"github.com/isseis/yt2column/internal/transcript"
)

// failureRequestIDForTest is a RequestID in the accepted shape: 16 lowercase
// hexadecimal characters.
const failureRequestIDForTest = "0123456789abcdef"

// failureVideoIDForTest is a video ID in the accepted shape: 11 characters of
// [A-Za-z0-9_-].
const failureVideoIDForTest = "dQw4w9WgXcQ"

// failureSentenceCase pairs a Kind with the fixed sentence the notice text must
// carry for it. The unlisted values must fall back to the unclassified
// sentence, so they appear here too.
type failureSentenceCase struct {
	kind     failure.Kind
	sentence string
}

// failureSentenceCases is the table of sentences, kept independent of
// failureSentence so a changed sentence fails these tests.
func failureSentenceCases() []failureSentenceCase {
	return []failureSentenceCase{
		{failure.KindUnknown, "the request failed for an unclassified reason"},
		{failure.KindBodyTooLarge, "the request body exceeded the size limit"},
		{failure.KindInvalidBody, "the request body was not a valid request"},
		{failure.KindInvalidVideoURL, "the video URL was not accepted"},
		{failure.KindInvalidTranscript, "the copied transcript was not in the expected format"},
		{failure.KindTranscriptTooLong, "the transcript was too long for the prompt limit"},
		{failure.KindLLMTimeout, "the LLM call timed out"},
		{failure.KindLLMEmpty, "the LLM returned an empty response"},
		{failure.KindLLMTruncated, "the LLM response was cut off before it finished"},
		{failure.KindLLM, "the LLM call failed"},
		{failure.KindPublish, "posting the article failed; part of it may already be in the channel"},
		{failure.KindInternal, "an internal error occurred"},
		{failure.Kind(-1), "the request failed for an unclassified reason"},
		{failure.KindInternal + 1, "the request failed for an unclassified reason"},
	}
}

// failureRejectionResponse is a response that does not acknowledge the notice.
// The status decides which shared sentinel the rejection also matches: a
// non-200 is ErrSlackHTTPStatus, a 200 with a wrong body is
// ErrSlackInvalidResponse.
type failureRejectionResponse struct {
	name   string
	status int
	body   string
}

// failureRejectionResponses is the shared table of responses that must be
// rejected, used by both the classification and the omission tests.
func failureRejectionResponses() []failureRejectionResponse {
	return []failureRejectionResponse{
		{"200 empty", http.StatusOK, ""},
		{"200 OK", http.StatusOK, "OK"},
		{"200 ok newline", http.StatusOK, "ok\n"},
		{"200 json", http.StatusOK, `{"ok":true}`},
		{"200 oversized", http.StatusOK, strings.Repeat("a", slackMaxResponseBytes+1)},
		{"400", http.StatusBadRequest, "bad request"},
		{"403", http.StatusForbidden, ""},
		{"500", http.StatusInternalServerError, "server error"},
	}
}

// validFailureNotice returns a notice every field of which is accepted.
func validFailureNotice() failure.Notice {
	return failure.Notice{RequestID: failureRequestIDForTest, VideoID: failureVideoIDForTest, Kind: failure.KindInternal}
}

// expectedFailureText builds the notice text for a sentence, with a video line
// only when videoURL is non-empty.
func expectedFailureText(sentence, videoURL string) string {
	text := "yt2column failure: " + sentence + "\nrequest: " + failureRequestIDForTest
	if videoURL != "" {
		text += "\nvideo: <" + videoURL + ">"
	}
	return text
}

// requireErrorPhrase fails unless err's message holds phrase. The phrase is the
// observable here: the logging caller tells whether the notice left the process
// from the message alone, so the wording is checked as text, not as a type.
func requireErrorPhrase(t *testing.T, err error, phrase string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected an error holding %q", phrase)
	}
	if !strings.Contains(err.Error(), phrase) {
		t.Fatalf("error %q does not hold %q", err.Error(), phrase)
	}
}

// TestSlackFailureNotifyText checks the exact text for every
// Kind (enumerated and unlisted), with and without a video ID.
func TestSlackFailureNotifyText(t *testing.T) {
	videoURL, ok := transcript.NormalizedVideoURL(failureVideoIDForTest)
	if !ok {
		t.Fatalf("NormalizedVideoURL(%q) is not accepted", failureVideoIDForTest)
	}

	t.Run("video id present", func(t *testing.T) {
		for _, tc := range failureSentenceCases() {
			server := newSlackOKServer(t)
			n := newSlackFailureNotifierForLoopbackTest(t, time.Second, server.url(), nil)
			notice := failure.Notice{RequestID: failureRequestIDForTest, VideoID: failureVideoIDForTest, Kind: tc.kind}
			if err := n.Notify(context.Background(), notice); err != nil {
				t.Fatalf("Notify(kind %d): %v", tc.kind, err)
			}
			requests := server.recorder.all()
			if len(requests) != 1 {
				t.Fatalf("kind %d: %d requests, want 1", tc.kind, len(requests))
			}
			got := slackPayloadText(t, requests[0])
			want := expectedFailureText(tc.sentence, videoURL)
			if got != want {
				t.Fatalf("kind %d: text = %q, want %q", tc.kind, got, want)
			}
		}
	})

	t.Run("video id empty", func(t *testing.T) {
		for _, tc := range failureSentenceCases() {
			server := newSlackOKServer(t)
			n := newSlackFailureNotifierForLoopbackTest(t, time.Second, server.url(), nil)
			notice := failure.Notice{RequestID: failureRequestIDForTest, Kind: tc.kind}
			if err := n.Notify(context.Background(), notice); err != nil {
				t.Fatalf("Notify(kind %d): %v", tc.kind, err)
			}
			got := slackPayloadText(t, server.recorder.all()[0])
			want := expectedFailureText(tc.sentence, "")
			if got != want {
				t.Fatalf("kind %d: text = %q, want %q", tc.kind, got, want)
			}
			if strings.Contains(got, "video:") {
				t.Fatalf("kind %d: text = %q, must not carry a video line", tc.kind, got)
			}
		}
	})

	t.Run("video id ends with underscore", func(t *testing.T) {
		const videoID = "dQw4w9WgXc_"
		url, ok := transcript.NormalizedVideoURL(videoID)
		if !ok {
			t.Fatalf("NormalizedVideoURL(%q) is not accepted", videoID)
		}
		server := newSlackOKServer(t)
		n := newSlackFailureNotifierForLoopbackTest(t, time.Second, server.url(), nil)
		notice := failure.Notice{RequestID: failureRequestIDForTest, VideoID: videoID, Kind: failure.KindPublish}
		if err := n.Notify(context.Background(), notice); err != nil {
			t.Fatalf("Notify: %v", err)
		}
		got := slackPayloadText(t, server.recorder.all()[0])
		want := expectedFailureText("posting the article failed; part of it may already be in the channel", url)
		if got != want {
			t.Fatalf("text = %q, want %q", got, want)
		}
	})

	t.Run("sentences are distinct", func(t *testing.T) {
		seen := map[string]failure.Kind{}
		for _, tc := range failureSentenceCases() {
			if tc.kind < failure.KindUnknown || tc.kind > failure.KindInternal {
				continue
			}
			if other, ok := seen[tc.sentence]; ok {
				t.Fatalf("kinds %d and %d share the sentence %q", other, tc.kind, tc.sentence)
			}
			seen[tc.sentence] = tc.kind
		}
	})
}

// TestSlackFailureNotifyInvalidNotice checks that a malformed RequestID or
// VideoID is rejected as ErrSlackInvalidNotice without sending, even when ctx
// has already ended.
func TestSlackFailureNotifyInvalidNotice(t *testing.T) {
	cases := []struct {
		name   string
		notice failure.Notice
	}{
		{"request id empty", failure.Notice{RequestID: ""}},
		{"request id 15 chars", failure.Notice{RequestID: "0123456789abcde"}},
		{"request id 17 chars", failure.Notice{RequestID: "0123456789abcdef0"}},
		{"request id uppercase", failure.Notice{RequestID: "0123456789ABCDEF"}},
		{"request id mention syntax", failure.Notice{RequestID: "<!channel>"}},
		{"video id 10 chars", failure.Notice{RequestID: failureRequestIDForTest, VideoID: "dQw4w9WgXc"}},
		{"video id 12 chars", failure.Notice{RequestID: failureRequestIDForTest, VideoID: "dQw4w9WgXcQx"}},
		{"video id with slash", failure.Notice{RequestID: failureRequestIDForTest, VideoID: "dQw4w9Wg/cQ"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server := newSlackOKServer(t)
			n := newSlackFailureNotifierForLoopbackTest(t, time.Second, server.url(), nil)
			err := n.Notify(context.Background(), tc.notice)
			if !errors.Is(err, ErrSlackInvalidNotice) {
				t.Fatalf("err = %v, want ErrSlackInvalidNotice", err)
			}
			if got := server.recorder.count(); got != 0 {
				t.Fatalf("the server received %d requests, want 0", got)
			}
		})
	}

	t.Run("shape check precedes a canceled context", func(t *testing.T) {
		notices := []failure.Notice{
			{RequestID: "bad"},
			{RequestID: failureRequestIDForTest, VideoID: "too-short"},
		}
		for _, notice := range notices {
			server := newSlackOKServer(t)
			n := newSlackFailureNotifierForLoopbackTest(t, time.Second, server.url(), nil)
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			err := n.Notify(ctx, notice)
			if !errors.Is(err, ErrSlackInvalidNotice) {
				t.Fatalf("err = %v, want ErrSlackInvalidNotice despite the canceled context", err)
			}
			if got := server.recorder.count(); got != 0 {
				t.Fatalf("the server received %d requests, want 0", got)
			}
		}
	})
}

// TestSlackFailureNotifyZeroValue checks that a nil pointer and a zero-value
// notifier fail without sending and without panicking.
func TestSlackFailureNotifyZeroValue(t *testing.T) {
	notice := failure.Notice{RequestID: failureRequestIDForTest, Kind: failure.KindUnknown}

	t.Run("nil pointer", func(t *testing.T) {
		var n *SlackFailureNotifier
		if err := n.Notify(context.Background(), notice); !errors.Is(err, errZeroNotifier) {
			t.Fatalf("err = %v, want errZeroNotifier", err)
		}
	})

	t.Run("zero value", func(t *testing.T) {
		var n SlackFailureNotifier
		if err := n.Notify(context.Background(), notice); !errors.Is(err, errZeroNotifier) {
			t.Fatalf("err = %v, want errZeroNotifier", err)
		}
	})
}

// TestSlackFailureNotifyOmitsWebhookURL checks that no text and no
// error reachable through Unwrap() error or Unwrap() []error shows the Webhook
// URL or any of its sensitive parts.
func TestSlackFailureNotifyOmitsWebhookURL(t *testing.T) {
	t.Run("text omits the endpoint", func(t *testing.T) {
		cases := failureSentenceCases()
		server := newSlackOKServer(t)
		endpoint := server.url()
		n := newSlackFailureNotifierForLoopbackTest(t, time.Second, endpoint, nil)
		parts := append([]string{endpoint, slackWebhookPath, slackWebhookMarker}, slackwebhook.SensitiveParts(endpoint)...)
		for _, tc := range cases {
			notice := failure.Notice{RequestID: failureRequestIDForTest, VideoID: failureVideoIDForTest, Kind: tc.kind}
			if err := n.Notify(context.Background(), notice); err != nil {
				t.Fatalf("Notify(kind %d): %v", tc.kind, err)
			}
		}
		requests := server.recorder.all()
		if len(requests) != len(cases) {
			t.Fatalf("%d requests, want %d", len(requests), len(cases))
		}
		videoURL, _ := transcript.NormalizedVideoURL(failureVideoIDForTest)
		for i, req := range requests {
			text := slackPayloadText(t, req)
			for _, s := range parts {
				if s != "" && strings.Contains(text, s) {
					t.Fatalf("text %d leaks %q: %q", i, s, text)
				}
			}
			want := expectedFailureText(cases[i].sentence, videoURL)
			if text != want {
				t.Fatalf("text %d = %q, want %q", i, text, want)
			}
		}
	})

	t.Run("errors omit the endpoint", func(t *testing.T) {
		for _, tc := range failureRejectionResponses() {
			t.Run(tc.name, func(t *testing.T) {
				server := newSlackServer(t, func(w http.ResponseWriter, _ *http.Request, _ int) {
					w.WriteHeader(tc.status)
					_, _ = io.WriteString(w, tc.body)
				})
				endpoint := server.url()
				n := newSlackFailureNotifierForLoopbackTest(t, time.Second, endpoint, nil)
				requireErrorOmitsEndpoint(t, n.Notify(context.Background(), validFailureNotice()), endpoint)
			})
		}

		t.Run("redirect", func(t *testing.T) {
			target := newSlackOKServer(t)
			server := newSlackServer(t, func(w http.ResponseWriter, _ *http.Request, _ int) {
				w.Header().Set("Location", target.url())
				w.WriteHeader(http.StatusFound)
			})
			endpoint := server.url()
			n := newSlackFailureNotifierForLoopbackTest(t, time.Second, endpoint, nil)
			requireErrorOmitsEndpoint(t, n.Notify(context.Background(), validFailureNotice()), endpoint)
		})

		t.Run("timeout", func(t *testing.T) {
			server := newSlackServer(t, func(_ http.ResponseWriter, r *http.Request, _ int) { <-r.Context().Done() })
			endpoint := server.url()
			n := newSlackFailureNotifierForLoopbackTest(t, 100*time.Millisecond, endpoint, nil)
			requireErrorOmitsEndpoint(t, n.Notify(context.Background(), validFailureNotice()), endpoint)
		})

		t.Run("connection refused", func(t *testing.T) {
			server := newSlackOKServer(t)
			endpoint := server.url()
			server.server.Close()
			n := NewSlackFailureNotifierForLoopbackTest(t, time.Second, endpoint)
			requireErrorOmitsEndpoint(t, n.Notify(context.Background(), validFailureNotice()), endpoint)
		})

		t.Run("transport error naming the endpoint is withheld", func(t *testing.T) {
			server := newSlackOKServer(t)
			endpoint := server.url()
			transport := failingTransport{err: errors.New("dial " + endpoint + ": connection refused")}
			n := newSlackFailureNotifierForLoopbackTest(t, time.Second, endpoint, transport)
			err := n.Notify(context.Background(), validFailureNotice())
			requireErrorOmitsEndpoint(t, err, endpoint)
			if !errors.Is(err, ErrSlackTransport) || !strings.Contains(err.Error(), "withheld") {
				t.Fatalf("err = %v, want ErrSlackTransport with withheld details", err)
			}
		})

		t.Run("struct formatting", func(t *testing.T) {
			server := newSlackOKServer(t)
			n := NewSlackFailureNotifierForLoopbackTest(t, time.Second, server.url())
			for _, format := range []string{"%v", "%+v", "%#v", "%s"} {
				if got := fmt.Sprintf(format, n); strings.Contains(got, slackWebhookMarker) {
					t.Fatalf("%s output leaks the Webhook URL: %s", format, got)
				}
			}
		})
	})
}

// TestSlackFailureNotifyOmitsMention checks that no text is treated as a
// mention and none holds a character the mention syntax is built from.
func TestSlackFailureNotifyOmitsMention(t *testing.T) {
	server := newSlackOKServer(t)
	n := newSlackFailureNotifierForLoopbackTest(t, time.Second, server.url(), nil)
	for _, tc := range failureSentenceCases() {
		notice := failure.Notice{RequestID: failureRequestIDForTest, VideoID: failureVideoIDForTest, Kind: tc.kind}
		if err := n.Notify(context.Background(), notice); err != nil {
			t.Fatalf("Notify(kind %d): %v", tc.kind, err)
		}
	}
	for i, req := range server.recorder.all() {
		text := slackPayloadText(t, req)
		if err := rejectMention(text); err != nil {
			t.Fatalf("text %d is treated as a mention: %v (%q)", i, err, text)
		}
		for _, r := range []string{"@", "&", "|", "!"} {
			if strings.Contains(text, r) {
				t.Fatalf("text %d contains %q: %q", i, r, text)
			}
		}
	}
}

// TestSlackFailureNotifyPayload checks that the request is a POST with a JSON
// body whose members are exactly text and silent.
func TestSlackFailureNotifyPayload(t *testing.T) {
	server := newSlackOKServer(t)
	n := newSlackFailureNotifierForLoopbackTest(t, time.Second, server.url(), nil)
	if err := n.Notify(context.Background(), validFailureNotice()); err != nil {
		t.Fatalf("Notify: %v", err)
	}
	requests := server.recorder.all()
	if len(requests) != 1 {
		t.Fatalf("%d requests, want 1", len(requests))
	}
	requireSlackPayloadShape(t, requests[0])
}

// TestSlackFailureNotifyRejected checks that a response that does
// not acknowledge the notice maps to ErrSlackNoticeRejected (and the shared
// sentinel), and a transport failure does not.
func TestSlackFailureNotifyRejected(t *testing.T) {
	for _, tc := range failureRejectionResponses() {
		t.Run(tc.name, func(t *testing.T) {
			server := newSlackServer(t, func(w http.ResponseWriter, _ *http.Request, _ int) {
				w.WriteHeader(tc.status)
				_, _ = io.WriteString(w, tc.body)
			})
			n := newSlackFailureNotifierForLoopbackTest(t, time.Second, server.url(), nil)
			err := n.Notify(context.Background(), validFailureNotice())
			if err == nil {
				t.Fatal("Notify has no error")
			}
			if !errors.Is(err, ErrSlackNoticeRejected) {
				t.Fatalf("err = %v, want ErrSlackNoticeRejected", err)
			}
			wantShared := ErrSlackHTTPStatus
			if tc.status == http.StatusOK {
				wantShared = ErrSlackInvalidResponse
			}
			if !errors.Is(err, wantShared) {
				t.Fatalf("err = %v, want %v", err, wantShared)
			}
			requireErrorPhrase(t, err, "the server rejected the failure notice")
		})
	}

	t.Run("transport failure is not a rejection", func(t *testing.T) {
		server := newSlackOKServer(t)
		endpoint := server.url()
		server.server.Close()
		n := NewSlackFailureNotifierForLoopbackTest(t, time.Second, endpoint)
		err := n.Notify(context.Background(), validFailureNotice())
		if !errors.Is(err, ErrSlackTransport) {
			t.Fatalf("err = %v, want ErrSlackTransport", err)
		}
		if errors.Is(err, ErrSlackNoticeRejected) {
			t.Fatalf("a transport failure matched ErrSlackNoticeRejected: %v", err)
		}
		requireErrorPhrase(t, err, "the failure notice may have been posted")
	})
}

// TestSlackFailureNotifyRedirect checks that a 3xx is a rejection and the
// redirect target receives nothing.
func TestSlackFailureNotifyRedirect(t *testing.T) {
	target := newSlackOKServer(t)
	server := newSlackServer(t, func(w http.ResponseWriter, _ *http.Request, _ int) {
		w.Header().Set("Location", target.url())
		w.WriteHeader(http.StatusFound)
	})
	n := newSlackFailureNotifierForLoopbackTest(t, time.Second, server.url(), nil)
	err := n.Notify(context.Background(), validFailureNotice())
	if !errors.Is(err, ErrSlackNoticeRejected) {
		t.Fatalf("err = %v, want ErrSlackNoticeRejected", err)
	}
	if got := target.recorder.count(); got != 0 {
		t.Fatalf("the redirect target received %d requests, want 0", got)
	}
}

// TestSlackFailureNotifyTimeout checks that the notifier's own bound ends a
// send whose caller context is still alive, and an ended context sends nothing.
func TestSlackFailureNotifyTimeout(t *testing.T) {
	t.Run("own bound ends a slow send", func(t *testing.T) {
		server := newSlackServer(t, func(_ http.ResponseWriter, r *http.Request, _ int) { <-r.Context().Done() })
		n := newSlackFailureNotifierForLoopbackTest(t, 100*time.Millisecond, server.url(), nil)
		ctx, cancel := context.WithTimeout(context.Background(), time.Hour)
		defer cancel()
		start := time.Now()
		err := n.Notify(ctx, validFailureNotice())
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("err = %v, want context.DeadlineExceeded", err)
		}
		if elapsed := time.Since(start); elapsed > 5*time.Second {
			t.Fatalf("Notify took %s, want well under the caller's one-hour deadline", elapsed)
		}
		requireErrorPhrase(t, err, "the failure notice may have been posted")
	})

	t.Run("canceled context sends nothing", func(t *testing.T) {
		// cancelableContext reports the cancellation through Err without
		// closing Done, so only Notify's own pre-check (not the HTTP layer)
		// can keep the request from being sent.
		server := newSlackOKServer(t)
		n := newSlackFailureNotifierForLoopbackTest(t, time.Second, server.url(), nil)
		ctx := &cancelableContext{Context: context.Background()}
		ctx.cancel(context.Canceled)
		err := n.Notify(ctx, validFailureNotice())
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("err = %v, want context.Canceled", err)
		}
		if got := server.recorder.count(); got != 0 {
			t.Fatalf("the server received %d requests, want 0", got)
		}
		requireErrorPhrase(t, err, "the failure notice was not sent")
	})
}

// TestSlackFailureNotifyNoRetry checks that each failure sends at most one
// request, and an ended context sends none.
func TestSlackFailureNotifyNoRetry(t *testing.T) {
	t.Run("rejected response sends once", func(t *testing.T) {
		server := newSlackServer(t, func(w http.ResponseWriter, _ *http.Request, _ int) {
			w.WriteHeader(http.StatusInternalServerError)
		})
		n := newSlackFailureNotifierForLoopbackTest(t, time.Second, server.url(), nil)
		_ = n.Notify(context.Background(), validFailureNotice())
		if got := server.recorder.count(); got != 1 {
			t.Fatalf("the server received %d requests, want 1", got)
		}
	})

	t.Run("redirect sends once", func(t *testing.T) {
		target := newSlackOKServer(t)
		server := newSlackServer(t, func(w http.ResponseWriter, _ *http.Request, _ int) {
			w.Header().Set("Location", target.url())
			w.WriteHeader(http.StatusFound)
		})
		n := newSlackFailureNotifierForLoopbackTest(t, time.Second, server.url(), nil)
		_ = n.Notify(context.Background(), validFailureNotice())
		if got := server.recorder.count(); got != 1 {
			t.Fatalf("the server received %d requests, want 1", got)
		}
	})

	t.Run("timeout sends once", func(t *testing.T) {
		server := newSlackServer(t, func(_ http.ResponseWriter, r *http.Request, _ int) { <-r.Context().Done() })
		n := newSlackFailureNotifierForLoopbackTest(t, 100*time.Millisecond, server.url(), nil)
		_ = n.Notify(context.Background(), validFailureNotice())
		if got := server.recorder.count(); got != 1 {
			t.Fatalf("the server received %d requests, want 1", got)
		}
	})

	t.Run("canceled context sends none", func(t *testing.T) {
		server := newSlackOKServer(t)
		n := newSlackFailureNotifierForLoopbackTest(t, time.Second, server.url(), nil)
		ctx := &cancelableContext{Context: context.Background()}
		ctx.cancel(context.Canceled)
		_ = n.Notify(ctx, validFailureNotice())
		if got := server.recorder.count(); got != 0 {
			t.Fatalf("the server received %d requests, want 0", got)
		}
	})
}

// TestNewSlackFailureNotifier checks that construction accepts the valid URL
// shapes, rejects the zero value and the invalid shapes without echoing the
// URL, and never sends a request.
func TestNewSlackFailureNotifier(t *testing.T) {
	t.Run("accepted url shapes", func(t *testing.T) {
		for _, raw := range []string{
			"https://mattermost.example.com/hooks/xxxxxxxxxxxxxxxxxxxxxxxxxx",
			"https://hooks.slack.com/services/T000/B000/XXXX",
		} {
			value, err := secret.New(raw)
			if err != nil {
				t.Fatalf("secret.New: %v", err)
			}
			got, err := NewSlackFailureNotifier(value)
			if err != nil {
				t.Fatalf("NewSlackFailureNotifier(%q): %v", raw, err)
			}
			if got == nil {
				t.Fatalf("NewSlackFailureNotifier(%q) returned nil", raw)
			}
		}
	})

	t.Run("zero value", func(t *testing.T) {
		if _, err := NewSlackFailureNotifier(secret.Secret{}); err == nil {
			t.Fatal("NewSlackFailureNotifier(zero) has no error")
		}
	})

	rejected := []string{
		"http://mattermost.example.com/hooks/x",
		"mattermost.example.com/hooks/x",
		"https:///hooks/x",
		"https://mattermost.example.com/hooks/x\n",
	}
	for _, raw := range rejected {
		t.Run(fmt.Sprintf("reject %q", raw), func(t *testing.T) {
			value, err := secret.New(raw)
			if err != nil {
				t.Fatalf("secret.New: %v", err)
			}
			_, err = NewSlackFailureNotifier(value)
			if err == nil {
				t.Fatalf("NewSlackFailureNotifier(%q) has no error", raw)
			}
			requireErrorOmitsEndpoint(t, err, raw)
		})
	}

	t.Run("construction sends nothing", func(t *testing.T) {
		server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Errorf("construction sent a request to %s", r.URL.Path)
			_, _ = io.WriteString(w, slackOKResponse)
		}))
		t.Cleanup(server.Close)
		value, err := secret.New(server.URL + slackWebhookPath)
		if err != nil {
			t.Fatalf("secret.New: %v", err)
		}
		if _, err := NewSlackFailureNotifier(value); err != nil {
			t.Fatalf("NewSlackFailureNotifier: %v", err)
		}
	})
}

// TestNewSlackFailureNotifierForLoopbackTestRejects checks that the loopback
// test constructor rejects a non-loopback endpoint, a non-positive timeout, and
// an empty endpoint without echoing the endpoint.
func TestNewSlackFailureNotifierForLoopbackTestRejects(t *testing.T) {
	cases := []struct {
		name     string
		timeout  time.Duration
		endpoint string
	}{
		{"not loopback", time.Second, "https://mattermost.example.com" + slackWebhookPath},
		{"non-positive timeout", 0, "http://127.0.0.1:1" + slackWebhookPath},
		{"empty endpoint", time.Second, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			recorder := &fatalRecorder{TB: t}
			value := NewSlackFailureNotifierForLoopbackTest(recorder, tc.timeout, tc.endpoint)
			if recorder.message == "" {
				t.Fatal("the constructor did not reject the endpoint")
			}
			if value != nil {
				t.Fatalf("the constructor returned %v, want nil", value)
			}
			if strings.Contains(recorder.message, slackWebhookMarker) {
				t.Fatalf("the failure message %q holds the endpoint path", recorder.message)
			}
		})
	}
}

// TestSlackFailureNotifyReuse checks that a failed Notify does not break the
// next one.
func TestSlackFailureNotifyReuse(t *testing.T) {
	server := newSlackServer(t, func(w http.ResponseWriter, _ *http.Request, n int) {
		if n == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		_, _ = io.WriteString(w, slackOKResponse)
	})
	n := newSlackFailureNotifierForLoopbackTest(t, time.Second, server.url(), nil)
	if err := n.Notify(context.Background(), validFailureNotice()); !errors.Is(err, ErrSlackNoticeRejected) {
		t.Fatalf("first Notify err = %v, want ErrSlackNoticeRejected", err)
	}
	if err := n.Notify(context.Background(), validFailureNotice()); err != nil {
		t.Fatalf("second Notify err = %v, want nil", err)
	}
	if got := server.recorder.count(); got != 2 {
		t.Fatalf("the server received %d requests, want 2", got)
	}
}

// TestSlackFailureNotifyConcurrent checks that one notifier serves concurrent
// callers: every Notify succeeds and every request arrives.
func TestSlackFailureNotifyConcurrent(t *testing.T) {
	const calls = 8
	server := newSlackOKServer(t)
	n := newSlackFailureNotifierForLoopbackTest(t, time.Second, server.url(), nil)
	var wg sync.WaitGroup
	errs := make(chan error, calls)
	for range calls {
		wg.Go(func() {
			errs <- n.Notify(context.Background(), validFailureNotice())
		})
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("Notify: %v", err)
		}
	}
	if got := server.recorder.count(); got != calls {
		t.Fatalf("the server received %d requests, want %d", got, calls)
	}
}
