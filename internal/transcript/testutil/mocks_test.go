//go:build test

package transcripttestutil

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/isseis/yt2column/internal/transcript"
)

func TestFakeTranscriptSourceRecordsAndReturns(t *testing.T) {
	result := transcript.Transcript{
		VideoID:  "video-1",
		Segments: []transcript.Segment{{StartMs: 1000, Text: "hello"}},
	}
	fake := &FakeTranscriptSource{Result: result}
	ctx := context.Background()

	got, err := fake.Fetch(ctx, "https://example.com/watch?v=video-1")
	if err != nil {
		t.Fatalf("Fetch returned error: %v", err)
	}
	if !reflect.DeepEqual(got, result) {
		t.Errorf("Fetch result = %+v, want %+v", got, result)
	}
	if len(fake.Calls) != 1 {
		t.Fatalf("recorded %d calls, want 1", len(fake.Calls))
	}
	if fake.Calls[0].Ctx != ctx {
		t.Error("Fetch did not record the context")
	}
	if want := "https://example.com/watch?v=video-1"; fake.Calls[0].VideoURL != want {
		t.Errorf("recorded videoURL = %q, want %q", fake.Calls[0].VideoURL, want)
	}
}

func TestFakeTranscriptSourceReturnsError(t *testing.T) {
	wantErr := errors.New("fetch failed")
	fake := &FakeTranscriptSource{Err: wantErr}

	_, err := fake.Fetch(context.Background(), "url")
	if !errors.Is(err, wantErr) {
		t.Errorf("Fetch error = %v, want %v", err, wantErr)
	}
}
