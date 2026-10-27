//go:build test

package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/isseis/yt2column/internal/failure"
	failuretestutil "github.com/isseis/yt2column/internal/failure/testutil"
	"github.com/isseis/yt2column/internal/llm"
	"github.com/isseis/yt2column/internal/publisher"
	publishertestutil "github.com/isseis/yt2column/internal/publisher/testutil"
	"github.com/isseis/yt2column/internal/writer"
	writertestutil "github.com/isseis/yt2column/internal/writer/testutil"
)

// TestAcceptStartsJob checks that an accepted request starts the job and posts
// the article.
func TestAcceptStartsJob(t *testing.T) {
	ts := newTestServer(t)
	rec := ts.serveAccepted(t, validBody(testValidVideoURL, testValidRaw))
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202", rec.Code)
	}
	if len(ts.publisher.Calls) != 1 {
		t.Errorf("publisher calls = %d, want 1", len(ts.publisher.Calls))
	}
}

// TestJobTranscriptValues checks that the job uses the video ID and normalized
// URL derived from videoUrl, never the request's URL string.
func TestJobTranscriptValues(t *testing.T) {
	for _, raw := range []string{
		"https://youtu.be/dQw4w9WgXcQ",
		"https://www.youtube.com/watch?v=dQw4w9WgXcQ&t=42s",
		"https://m.youtube.com/shorts/dQw4w9WgXcQ",
	} {
		t.Run(raw, func(t *testing.T) {
			ts := newTestServer(t)
			rec := ts.serveAccepted(t, validBody(raw, testValidRaw))
			if rec.Code != http.StatusAccepted {
				t.Fatalf("status = %d, want 202", rec.Code)
			}
			got := ts.writer.Calls[0].Transcript
			if got.VideoID != testVideoID {
				t.Errorf("VideoID = %q, want %q", got.VideoID, testVideoID)
			}
			if got.VideoURL != testValidVideoURL {
				t.Errorf("VideoURL = %q, want %q", got.VideoURL, testValidVideoURL)
			}
			if got.VideoURL == raw {
				t.Errorf("VideoURL is the request string %q", raw)
			}
		})
	}
}

// TestSlotLimit checks that only maxInFlight jobs run at once and that a further
// request is refused with 429 while the in-flight jobs continue and finish.
func TestSlotLimit(t *testing.T) {
	gate := newGateWriter()
	pub := &countingPublisher{}
	notifier := &failuretestutil.FakeNotifier{}
	s := newCustomServer(t, gate, pub, notifier)

	serve := func() *httptest.ResponseRecorder {
		req := newRequest(http.MethodPost, ArticlesPath, strings.NewReader(validBody(testValidVideoURL, testValidRaw)))
		rec := httptest.NewRecorder()
		s.handler(rec, req)
		return rec
	}
	for i := range maxInFlight {
		if rec := serve(); rec.Code != http.StatusAccepted {
			t.Fatalf("request %d status = %d, want 202", i, rec.Code)
		}
	}
	for range maxInFlight {
		<-gate.entered
	}
	if rec := serve(); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("third request status = %d, want 429", rec.Code)
	}
	close(gate.release)
	s.jobs.Wait()
	if pub.count() != maxInFlight {
		t.Errorf("publisher calls = %d, want %d", pub.count(), maxInFlight)
	}
	if len(notifier.Calls) != 0 {
		t.Errorf("notifier calls = %d, want 0 for a slot limit", len(notifier.Calls))
	}
}

// TestSlotReleasedOnce checks that a slot is released after a rejected body, a
// failed body read, and a handler panic.
func TestSlotReleasedOnce(t *testing.T) {
	t.Run("rejected_body", func(t *testing.T) {
		ts := newTestServer(t)
		ts.serve(ts.request(t, `{"videoUrl":1}`))
		ts.waitJobs()
		if len(ts.slots) != 0 {
			t.Errorf("slots held = %d, want 0", len(ts.slots))
		}
		if len(ts.notifier.Calls) != 1 || ts.notifier.Calls[0].Notice.Kind != failure.KindInvalidBody {
			t.Errorf("notifier calls = %+v, want one KindInvalidBody", ts.notifier.Calls)
		}
	})
	t.Run("read_failure", func(t *testing.T) {
		ts := newTestServer(t)
		req := newRequest(http.MethodPost, ArticlesPath, iotest.ErrReader(errors.New("cut")))
		rec := httptest.NewRecorder()
		func() {
			defer func() {
				if p := recover(); p != http.ErrAbortHandler {
					t.Errorf("handler panic = %v, want http.ErrAbortHandler", p)
				}
			}()
			ts.handler(rec, req)
		}()
		if rec.Code != http.StatusOK || rec.Body.Len() != 0 || rec.Flushed {
			t.Errorf("response written (status %d, body %q), want none", rec.Code, rec.Body.String())
		}
		if len(ts.slots) != 0 {
			t.Errorf("slots held = %d, want 0", len(ts.slots))
		}
		if len(ts.notifier.Calls) != 0 {
			t.Errorf("notifier calls = %d, want 0 for a body-read failure", len(ts.notifier.Calls))
		}
	})
	t.Run("handler_panic", func(t *testing.T) {
		ts := newTestServer(t)
		req := newRequest(http.MethodPost, ArticlesPath, panickingReader{})
		rec := ts.serve(req)
		ts.waitJobs()
		if rec.Code != http.StatusInternalServerError {
			t.Errorf("status = %d, want 500", rec.Code)
		}
		if body := decodeBody(t, rec); body.Reason != "internal_error" {
			t.Errorf("reason = %q, want internal_error", body.Reason)
		}
		if len(ts.notifier.Calls) != 1 || ts.notifier.Calls[0].Notice.Kind != failure.KindInternal {
			t.Errorf("notifier calls = %+v, want one KindInternal", ts.notifier.Calls)
		}
		if len(ts.slots) != 0 {
			t.Errorf("slots held = %d, want 0", len(ts.slots))
		}
	})
}

// TestFailureClassification checks that each pipeline failure maps to the kind
// the design table names.
func TestFailureClassification(t *testing.T) {
	tests := []struct {
		name      string
		writerErr error
		pubErr    error
		want      failure.Kind
	}{
		{"prompt_too_large", writer.ErrPromptTooLarge, nil, failure.KindTranscriptTooLong},
		{"llm_timeout", context.DeadlineExceeded, nil, failure.KindLLMTimeout},
		{"llm_empty", llm.ErrEmptyResponse, nil, failure.KindLLMEmpty},
		{"llm_truncated", llm.ErrTruncated, nil, failure.KindLLMTruncated},
		{"invalid_transcript", writer.ErrInvalidTranscript, nil, failure.KindInternal},
		{"invalid_template", writer.ErrInvalidTemplate, nil, failure.KindInternal},
		{"malformed_output", writer.ErrMalformedOutput, nil, failure.KindLLM},
		{"too_long", writer.ErrTooLong, nil, failure.KindLLM},
		{"publish", nil, errors.New("publish failed"), failure.KindPublish},
		{"publish_mention", nil, fmt.Errorf("%w: rule M2 at line 1, column 3", publisher.ErrSlackMention), failure.KindPublishMention},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fakeWriter := &writertestutil.FakeArticleWriter{Err: tc.writerErr}
			fakePub := &publishertestutil.FakePublisher{Err: tc.pubErr}
			notifier := &failuretestutil.FakeNotifier{}
			s := newCustomServer(t, fakeWriter, fakePub, notifier)
			req := newRequest(http.MethodPost, ArticlesPath, strings.NewReader(validBody(testValidVideoURL, testValidRaw)))
			s.handler(httptest.NewRecorder(), req)
			s.jobs.Wait()
			if len(notifier.Calls) != 1 {
				t.Fatalf("notifier calls = %d, want 1", len(notifier.Calls))
			}
			if got := notifier.Calls[0].Notice.Kind; got != tc.want {
				t.Errorf("kind = %d, want %d", got, tc.want)
			}
		})
	}
}

// TestReasonToKind checks the rejectReason to failure.Kind mapping for every
// reason value.
func TestReasonToKind(t *testing.T) {
	want := map[rejectReason]failure.Kind{
		reasonInternal:          failure.KindInternal,
		reasonBodyTooLarge:      failure.KindBodyTooLarge,
		reasonInvalidBody:       failure.KindInvalidBody,
		reasonInvalidVideoURL:   failure.KindInvalidVideoURL,
		reasonInvalidTranscript: failure.KindInvalidTranscript,
		reasonNotFound:          failure.KindUnknown,
		reasonMethodNotAllowed:  failure.KindUnknown,
		reasonTooManyRequests:   failure.KindUnknown,
		// A server without a Webhook has no one to notify.
		reasonWebhookNotConfigured: failure.KindUnknown,
	}
	for reason := range reasonCount {
		expected, ok := want[reason]
		if !ok {
			t.Fatalf("no expected kind for reason %d", reason)
		}
		if got := reasonToKind(reason); got != expected {
			t.Errorf("reasonToKind(%d) = %d, want %d", reason, got, expected)
		}
	}
}

// TestNotifyNoticeFields checks that a failure notice carries only the request
// ID, the validated video ID, and the kind.
func TestNotifyNoticeFields(t *testing.T) {
	ts := newTestServer(t)
	ts.writer.Err = writer.ErrMalformedOutput
	req := ts.request(t, validBody(testValidVideoURL, testValidRaw))
	rec := ts.serve(req)
	ts.waitJobs()
	if len(ts.notifier.Calls) != 1 {
		t.Fatalf("notifier calls = %d, want 1", len(ts.notifier.Calls))
	}
	notice := ts.notifier.Calls[0].Notice
	if notice.RequestID != decodeBody(t, rec).RequestID {
		t.Errorf("notice RequestID = %q, want the response's %q", notice.RequestID, decodeBody(t, rec).RequestID)
	}
	if !requestIDShape.MatchString(notice.RequestID) {
		t.Errorf("notice RequestID = %q, want 16 lowercase hexadecimal characters", notice.RequestID)
	}
	if notice.VideoID != testVideoID {
		t.Errorf("notice VideoID = %q, want %q", notice.VideoID, testVideoID)
	}
	if notice.Kind != failure.KindLLM {
		t.Errorf("notice Kind = %d, want KindLLM", notice.Kind)
	}
	if rendered := fmt.Sprintf("%+v", notice); strings.Contains(rendered, testMarker) {
		t.Errorf("notice carries a secret: %s", rendered)
	}
}

// TestNotifyErrorContinues checks that a failed notice is logged and the server
// keeps serving.
func TestNotifyErrorContinues(t *testing.T) {
	for _, notifyErr := range []error{errors.New("notice failed"), context.DeadlineExceeded} {
		t.Run(notifyErr.Error(), func(t *testing.T) {
			ts := newTestServer(t)
			ts.notifier.Err = notifyErr
			ts.writer.Err = writer.ErrMalformedOutput
			ts.serve(ts.request(t, validBody(testValidVideoURL, testValidRaw)))
			ts.waitJobs()
			if !strings.Contains(ts.logs.String(), "failure notice failed") {
				t.Errorf("the failed notice was not logged:\n%s", ts.logs.String())
			}
			ts.writer.Err = nil
			rec := ts.serveAccepted(t, validBody(testValidVideoURL, testValidRaw))
			if rec.Code != http.StatusAccepted {
				t.Errorf("next request status = %d, want 202", rec.Code)
			}
			if len(ts.publisher.Calls) != 1 {
				t.Errorf("publisher calls = %d, want 1", len(ts.publisher.Calls))
			}
		})
	}
}

// TestPanicRecycledAsInternal checks that a job-goroutine panic is recovered
// and reported as KindInternal without changing the already-sent 202.
func TestPanicRecycledAsInternal(t *testing.T) {
	notifier := &failuretestutil.FakeNotifier{}
	s := newCustomServer(t, panickingWriter{}, &publishertestutil.FakePublisher{}, notifier)
	req := newRequest(http.MethodPost, ArticlesPath, strings.NewReader(validBody(testValidVideoURL, testValidRaw)))
	rec := httptest.NewRecorder()
	s.handler(rec, req)
	s.jobs.Wait()
	if rec.Code != http.StatusAccepted {
		t.Errorf("status = %d, want 202", rec.Code)
	}
	if len(notifier.Calls) != 1 || notifier.Calls[0].Notice.Kind != failure.KindInternal {
		t.Errorf("notifier calls = %+v, want one KindInternal", notifier.Calls)
	}
	if len(s.slots) != 0 {
		t.Errorf("slots held = %d, want 0", len(s.slots))
	}
}

// TestNotifyPanicLogged checks that a notifier panic is logged and does not
// crash the process or escape the job goroutine.
func TestNotifyPanicLogged(t *testing.T) {
	logs := &strings.Builder{}
	s, err := New(Options{
		Writer:    panickingWriter{},
		Webhook:   &WebhookOptions{Publisher: &publishertestutil.FakePublisher{}, Notifier: panickingNotifier{}},
		LogOutput: logs,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	req := newRequest(http.MethodPost, ArticlesPath, strings.NewReader(validBody(testValidVideoURL, testValidRaw)))
	s.handler(httptest.NewRecorder(), req)
	s.jobs.Wait()
	if !strings.Contains(logs.String(), "failure notice panicked") {
		t.Errorf("the notifier panic was not logged:\n%s", logs.String())
	}
	if len(s.slots) != 0 {
		t.Errorf("slots held = %d, want 0", len(s.slots))
	}
}
