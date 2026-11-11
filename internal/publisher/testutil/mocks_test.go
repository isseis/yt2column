//go:build test

package publishertestutil

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"testing"

	"github.com/isseis/yt2column/internal/writer"
)

func TestFakePublisherRecords(t *testing.T) {
	fake := &FakePublisher{}
	ctx := context.Background()
	inputs := []writer.Article{
		{Title: "Title", Body: "Body", SourceURL: "url", Model: "model"},
		{Title: "Title 2", Body: "Body 2", SourceURL: "url-2", Model: "model-2"},
	}

	for _, in := range inputs {
		if err := fake.Publish(ctx, in); err != nil {
			t.Fatalf("Publish returned error: %v", err)
		}
	}
	if len(fake.Calls) != len(inputs) {
		t.Fatalf("recorded %d calls, want %d", len(fake.Calls), len(inputs))
	}
	for i, in := range inputs {
		if fake.Calls[i].Ctx != ctx {
			t.Errorf("call %d did not record the context", i)
		}
		if !reflect.DeepEqual(fake.Calls[i].Article, in) {
			t.Errorf("call %d article = %+v, want %+v", i, fake.Calls[i].Article, in)
		}
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

// TestFakePublisherConcurrentCalls runs calls from several goroutines while
// reading the records; the race detector (make test) fails it when the
// records are not kept under the mutex.
func TestFakePublisherConcurrentCalls(t *testing.T) {
	const calls = 8
	fake := &FakePublisher{}
	var wg sync.WaitGroup
	for range calls {
		wg.Go(func() { _ = fake.Publish(context.Background(), writer.Article{}) })
		wg.Go(func() { _ = fake.RecordedCalls() })
	}
	wg.Wait()
	if got := len(fake.RecordedCalls()); got != calls {
		t.Errorf("recorded %d Publish calls, want %d", got, calls)
	}
}
