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
	inputs := []transcript.Transcript{
		{VideoID: "video-1", Segments: []transcript.Segment{{StartMs: 1, Text: "a"}}},
		{VideoID: "video-2", Segments: []transcript.Segment{{StartMs: 2, Text: "b"}}},
	}

	for _, in := range inputs {
		got, err := fake.Write(ctx, in)
		if err != nil {
			t.Fatalf("Write returned error: %v", err)
		}
		if !reflect.DeepEqual(got, result) {
			t.Errorf("Write result = %+v, want %+v", got, result)
		}
	}
	if len(fake.Calls) != len(inputs) {
		t.Fatalf("recorded %d calls, want %d", len(fake.Calls), len(inputs))
	}
	for i, in := range inputs {
		if fake.Calls[i].Ctx != ctx {
			t.Errorf("call %d did not record the context", i)
		}
		if !reflect.DeepEqual(fake.Calls[i].Transcript, in) {
			t.Errorf("call %d transcript = %+v, want %+v", i, fake.Calls[i].Transcript, in)
		}
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
