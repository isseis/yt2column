//go:build test

package writertestutil

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/isseis/yt2column/internal/transcript"
	"github.com/isseis/yt2column/internal/writer"
)

func TestFakeArticleWriterRecordsAndReturns(t *testing.T) {
	result := writer.Article{Title: "Title", Body: "Body", SourceURL: "url", Model: "model"}
	fake := &FakeArticleWriter{Result: result}
	ctx := context.Background()
	in := transcript.Transcript{VideoID: "video-1", Segments: []transcript.Segment{{StartMs: 1, Text: "a"}}}

	got, err := fake.Write(ctx, in)
	if err != nil {
		t.Fatalf("Write returned error: %v", err)
	}
	if !reflect.DeepEqual(got, result) {
		t.Errorf("Write result = %+v, want %+v", got, result)
	}
	if len(fake.Calls) != 1 {
		t.Fatalf("recorded %d calls, want 1", len(fake.Calls))
	}
	if fake.Calls[0].Ctx != ctx {
		t.Error("Write did not record the context")
	}
	if !reflect.DeepEqual(fake.Calls[0].Transcript, in) {
		t.Errorf("recorded transcript = %+v, want %+v", fake.Calls[0].Transcript, in)
	}
}

func TestFakeArticleWriterReturnsError(t *testing.T) {
	wantErr := errors.New("write failed")
	fake := &FakeArticleWriter{Err: wantErr}

	_, err := fake.Write(context.Background(), transcript.Transcript{})
	if !errors.Is(err, wantErr) {
		t.Errorf("Write error = %v, want %v", err, wantErr)
	}
}
