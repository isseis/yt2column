//go:build test

package publishertestutil

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/isseis/yt2column/internal/writer"
)

func TestFakePublisherRecordsAndReturns(t *testing.T) {
	fake := &FakePublisher{}
	ctx := context.Background()
	in := writer.Article{Title: "Title", Body: "Body", SourceURL: "url", Model: "model"}

	if err := fake.Publish(ctx, in); err != nil {
		t.Fatalf("Publish returned error: %v", err)
	}
	if len(fake.Calls) != 1 {
		t.Fatalf("recorded %d calls, want 1", len(fake.Calls))
	}
	if fake.Calls[0].Ctx != ctx {
		t.Error("Publish did not record the context")
	}
	if !reflect.DeepEqual(fake.Calls[0].Article, in) {
		t.Errorf("recorded article = %+v, want %+v", fake.Calls[0].Article, in)
	}
}

func TestFakePublisherReturnsError(t *testing.T) {
	wantErr := errors.New("publish failed")
	fake := &FakePublisher{Err: wantErr}

	err := fake.Publish(context.Background(), writer.Article{})
	if !errors.Is(err, wantErr) {
		t.Errorf("Publish error = %v, want %v", err, wantErr)
	}
}
