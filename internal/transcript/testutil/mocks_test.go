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
	urls := []string{
		"https://example.com/watch?v=video-1",
		"https://example.com/watch?v=video-2",
	}

	for _, url := range urls {
		got, err := fake.Fetch(ctx, url)
		if err != nil {
			t.Fatalf("Fetch returned error: %v", err)
		}
		if !reflect.DeepEqual(got, result) {
			t.Errorf("Fetch result = %+v, want %+v", got, result)
		}
	}
	if len(fake.Calls) != len(urls) {
		t.Fatalf("recorded %d calls, want %d", len(fake.Calls), len(urls))
	}
	for i, url := range urls {
		if fake.Calls[i].Ctx != ctx {
			t.Errorf("call %d did not record the context", i)
		}
		if fake.Calls[i].VideoURL != url {
			t.Errorf("call %d videoURL = %q, want %q", i, fake.Calls[i].VideoURL, url)
		}
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
