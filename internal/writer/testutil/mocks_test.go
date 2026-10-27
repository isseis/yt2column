//go:build test

package writertestutil

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"sync"
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

// recordingObserver records every ProgressObserver call as "<kind> <stage>".
type recordingObserver struct {
	events []string
}

func (r *recordingObserver) StageStarted(s writer.Stage) {
	r.events = append(r.events, "started "+s.String())
}

func (r *recordingObserver) StageFinished(s writer.Stage) {
	r.events = append(r.events, "finished "+s.String())
}

func (r *recordingObserver) StageSkipped(s writer.Stage) {
	r.events = append(r.events, "skipped "+s.String())
}

func TestFakeArticleWriterProgress(t *testing.T) {
	result := writer.Article{Title: "Title", Body: "Body", SourceURL: "url", Model: "model"}
	wantErr := errors.New("write failed")
	fake := &FakeArticleWriter{
		Result: result,
		Err:    wantErr,
		Progress: []FakeProgressStep{
			{Kind: FakeStageStarted, Stage: writer.StageGeneration},
			{Kind: FakeStageFinished, Stage: writer.StageGeneration},
			{Kind: FakeStageSkipped, Stage: writer.StageShorten},
		},
	}
	observer := &recordingObserver{}
	in := transcript.Transcript{VideoID: "video-1"}

	got, err := fake.WriteWithProgress(context.Background(), in, observer)
	if !errors.Is(err, wantErr) {
		t.Errorf("WriteWithProgress error = %v, want %v", err, wantErr)
	}
	if !reflect.DeepEqual(got, result) {
		t.Errorf("WriteWithProgress result = %+v, want %+v", got, result)
	}
	want := []string{"started generation", "finished generation", "skipped shorten"}
	if !slices.Equal(observer.events, want) {
		t.Errorf("events = %v, want %v", observer.events, want)
	}
	if calls := fake.RecordedCalls(); len(calls) != 1 || !reflect.DeepEqual(calls[0].Transcript, in) {
		t.Errorf("recorded calls = %+v, want the one call with %+v", calls, in)
	}
}

func TestFakeArticleWriterHoldsAtStep(t *testing.T) {
	type outcome struct {
		article writer.Article
		err     error
	}
	start := func(ctx context.Context, fake *FakeArticleWriter, observer *recordingObserver) <-chan outcome {
		done := make(chan outcome, 1)
		go func() {
			a, err := fake.WriteWithProgress(ctx, transcript.Transcript{}, observer)
			done <- outcome{a, err}
		}()
		return done
	}
	newFake := func(reached chan<- struct{}, release <-chan struct{}) *FakeArticleWriter {
		return &FakeArticleWriter{
			Result: writer.Article{Title: "Title"},
			Progress: []FakeProgressStep{
				{Kind: FakeStageStarted, Stage: writer.StageGeneration, Reached: reached, Release: release},
				{Kind: FakeStageFinished, Stage: writer.StageGeneration},
			},
		}
	}

	t.Run("released", func(t *testing.T) {
		// Two calls are held at once, as the server's two generation jobs
		// are; one close of release lets both go.
		reached, release := make(chan struct{}), make(chan struct{})
		fake := newFake(reached, release)
		observers := []*recordingObserver{{}, {}}
		dones := []<-chan outcome{start(context.Background(), fake, observers[0]), start(context.Background(), fake, observers[1])}

		<-reached
		<-reached
		// Both calls are held on release, so neither can have returned yet,
		// and neither has sent the next notification.
		for i, done := range dones {
			select {
			case o := <-done:
				t.Fatalf("call %d returned %+v before the release", i, o)
			default:
			}
			if want := []string{"started generation"}; !slices.Equal(observers[i].events, want) {
				t.Errorf("call %d events while held = %v, want %v", i, observers[i].events, want)
			}
		}
		close(release)
		for i, done := range dones {
			o := <-done
			if o.err != nil || o.article.Title != "Title" {
				t.Errorf("call %d = %+v, want the configured result", i, o)
			}
			if want := []string{"started generation", "finished generation"}; !slices.Equal(observers[i].events, want) {
				t.Errorf("call %d events = %v, want %v", i, observers[i].events, want)
			}
		}
	})

	t.Run("canceled", func(t *testing.T) {
		reached := make(chan struct{})
		fake := newFake(reached, make(chan struct{}))
		ctx, cancel := context.WithCancel(context.Background())
		observer := &recordingObserver{}
		done := start(ctx, fake, observer)

		<-reached
		cancel()
		o := <-done
		if !errors.Is(o.err, context.Canceled) {
			t.Errorf("WriteWithProgress error = %v, want context.Canceled", o.err)
		}
		if o.article != (writer.Article{}) {
			t.Errorf("WriteWithProgress article = %+v, want the zero value", o.article)
		}
		if want := []string{"started generation"}; !slices.Equal(observer.events, want) {
			t.Errorf("events = %v, want %v (the steps after the hold are not sent)", observer.events, want)
		}
	})
}

func TestFakeArticleWriterRejectsUnknownKind(t *testing.T) {
	fake := &FakeArticleWriter{Progress: []FakeProgressStep{{Stage: writer.StageGeneration}}}
	observer := &recordingObserver{}

	_, err := fake.WriteWithProgress(context.Background(), transcript.Transcript{}, observer)
	if !errors.Is(err, errUnknownProgressKind) {
		t.Errorf("WriteWithProgress error = %v, want errUnknownProgressKind", err)
	}
	if len(observer.events) != 0 {
		t.Errorf("events = %v, want none", observer.events)
	}
}

// TestFakeArticleWriterConcurrentCalls runs calls from several goroutines
// while reading the record; the race detector (make test) fails it when the
// record is not kept under the mutex.
func TestFakeArticleWriterConcurrentCalls(t *testing.T) {
	const calls = 8
	fake := &FakeArticleWriter{}
	var wg sync.WaitGroup
	for range calls {
		wg.Go(func() {
			_, _ = fake.WriteWithProgress(context.Background(), transcript.Transcript{}, &recordingObserver{})
		})
		wg.Go(func() {
			_ = fake.RecordedCalls()
		})
	}
	wg.Wait()
	if got := len(fake.RecordedCalls()); got != calls {
		t.Errorf("recorded %d calls, want %d", got, calls)
	}
}

func TestFakeArticleWriterRejectsNilObserver(t *testing.T) {
	fake := &FakeArticleWriter{Result: writer.Article{Title: "Title"}}
	var typedNil *recordingObserver
	for name, observer := range map[string]writer.ProgressObserver{"nil": nil, "typed nil": typedNil} {
		t.Run(name, func(t *testing.T) {
			a, err := fake.WriteWithProgress(context.Background(), transcript.Transcript{}, observer)
			if !errors.Is(err, errNilObserver) {
				t.Errorf("WriteWithProgress error = %v, want errNilObserver", err)
			}
			if a != (writer.Article{}) {
				t.Errorf("WriteWithProgress article = %+v, want the zero value", a)
			}
		})
	}
}
