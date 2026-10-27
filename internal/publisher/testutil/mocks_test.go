//go:build test

package publishertestutil

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/isseis/yt2column/internal/publisher"
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

// twoMessages returns the prepared messages of an article whose post splits
// into two messages.
func twoMessages(t *testing.T) publisher.SlackMessages {
	t.Helper()
	m, err := publisher.PrepareSlackMessages(writer.Article{
		Title: "Title",
		// About 1.2 messages of text, cut after a newline.
		Body:      strings.Repeat("a\n", publisher.SlackMaxMessageRunesForTest*3/5),
		SourceURL: "https://www.youtube.com/watch?v=abcdefghijk",
		Model:     "model",
	})
	if err != nil {
		t.Fatalf("PrepareSlackMessages: %v", err)
	}
	if m.Len() != 2 {
		t.Fatalf("messages = %d, want 2", m.Len())
	}
	return m
}

func TestFakePublisherPostMessages(t *testing.T) {
	m := twoMessages(t)
	ctx := context.Background()
	cases := []struct {
		name          string
		postErr       error
		from          int
		wantDelivered []int
	}{
		{"from the first", nil, 0, []int{0, 1}},
		{"from the second", nil, 1, []int{1}},
		{"from the total", nil, 2, nil},
		{"failure", errors.New("post failed"), 0, nil},
		{"from below zero", nil, -1, nil},
		{"from past the total", nil, 3, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fake := &FakePublisher{PostErr: tc.postErr}
			var delivered []int
			err := fake.PostMessages(ctx, m, tc.from, func(i int) { delivered = append(delivered, i) })
			wantErr := tc.postErr
			if tc.from < 0 || tc.from > m.Len() {
				wantErr = ErrFakePostRefused
			}
			if !errors.Is(err, wantErr) {
				t.Errorf("PostMessages error = %v, want %v", err, wantErr)
			}
			if !slices.Equal(delivered, tc.wantDelivered) {
				t.Errorf("delivered = %v, want %v", delivered, tc.wantDelivered)
			}
			calls := fake.RecordedPostCalls()
			if len(calls) != 1 || calls[0].Ctx != ctx || calls[0].From != tc.from || calls[0].Messages.Len() != m.Len() {
				t.Errorf("recorded post calls = %+v, want the one call from %d", calls, tc.from)
			}
			if len(fake.RecordedCalls()) != 0 {
				t.Errorf("recorded Publish calls = %+v, want none", fake.RecordedCalls())
			}
		})
	}
}

func TestFakePublisherPostMessagesRefuses(t *testing.T) {
	cases := []struct {
		name      string
		m         publisher.SlackMessages
		delivered func(int)
	}{
		{"messages not prepared", publisher.SlackMessages{}, func(int) {}},
		{"nil delivered", twoMessages(t), nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fake := &FakePublisher{}
			if err := fake.PostMessages(context.Background(), tc.m, 0, tc.delivered); !errors.Is(err, ErrFakePostRefused) {
				t.Errorf("PostMessages error = %v, want ErrFakePostRefused", err)
			}
		})
	}
}

// TestFakePublisherConcurrentCalls runs calls from several goroutines while
// reading the records; the race detector (make test) fails it when the
// records are not kept under the mutex.
func TestFakePublisherConcurrentCalls(t *testing.T) {
	const calls = 8
	m := twoMessages(t)
	fake := &FakePublisher{}
	var wg sync.WaitGroup
	for range calls {
		wg.Go(func() { _ = fake.Publish(context.Background(), writer.Article{}) })
		wg.Go(func() { _ = fake.PostMessages(context.Background(), m, 0, func(int) {}) })
		wg.Go(func() { _, _ = fake.RecordedCalls(), fake.RecordedPostCalls() })
	}
	wg.Wait()
	if got, gotPost := len(fake.RecordedCalls()), len(fake.RecordedPostCalls()); got != calls || gotPost != calls {
		t.Errorf("recorded %d Publish and %d PostMessages calls, want %d each", got, gotPost, calls)
	}
}
