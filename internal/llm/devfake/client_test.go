//go:build test

package devfake

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/isseis/yt2column/internal/llm"
)

// acceptedScenarios returns every scenario NewScenario accepts with a zero
// delay: each article with no failure and with each failure at each call.
func acceptedScenarios(t *testing.T) []Scenario {
	t.Helper()
	var out []Scenario
	for _, article := range Articles() {
		out = append(out, mustScenario(t, article, FailureNone, FailAtNone, 0))
		for _, failure := range Failures() {
			for _, at := range FailAts() {
				if s, err := NewScenario(article, failure, at, 0); err == nil {
					out = append(out, s)
				}
			}
		}
	}
	return out
}

// failureSentinel is the error each failure must wrap.
var failureSentinel = map[Failure]error{
	FailureEmpty:     llm.ErrEmptyResponse,
	FailureTruncated: llm.ErrTruncated,
	FailureAPIError:  ErrSimulatedAPIFailure,
	FailureTimeout:   context.DeadlineExceeded,
}

// checkCall checks one call's result: the scenario's failure when it fails at
// at, and otherwise a success carrying want.
func checkCall(t *testing.T, s Scenario, at FailAt, resp llm.GenerateResponse, err error, want string) {
	t.Helper()
	if s.failAt != at {
		if err != nil {
			t.Fatalf("call %v error = %v, want nil", at, err)
		}
		if want := (llm.GenerateResponse{Text: want, Model: ModelName}); resp != want {
			t.Errorf("call %v response = %+v, want %+v", at, resp, want)
		}
		return
	}
	if !errors.Is(err, failureSentinel[s.failure]) {
		t.Fatalf("call %v error = %v, want %v", at, err, failureSentinel[s.failure])
	}
	var want0 llm.GenerateResponse
	if s.failure == FailureEmpty || s.failure == FailureTruncated {
		want0 = llm.GenerateResponse{Model: ModelName}
	}
	if resp != want0 {
		t.Errorf("call %v failure response = %+v, want %+v", at, resp, want0)
	}
	if s.failure == FailureAPIError {
		for _, other := range []error{
			llm.ErrInvalidRequest, llm.ErrTruncated, llm.ErrUnexpectedFinishReason, llm.ErrEmptyResponse,
			context.DeadlineExceeded, context.Canceled,
		} {
			if errors.Is(err, other) {
				t.Errorf("API error %v also wraps %v", err, other)
			}
		}
	}
}

func TestJobResponses(t *testing.T) {
	ctx := context.Background()
	for _, s := range acceptedScenarios(t) {
		t.Run(s.LogValue().String(), func(t *testing.T) {
			job := s.NewJob()
			draft, err := draftText(s.article)
			if err != nil {
				t.Fatal(err)
			}

			resp, err := job.Generation().Generate(ctx, llm.GenerateRequest{})
			checkCall(t, s, FailAtDraft, resp, err, draft)

			resp, err = job.Generation().Generate(ctx, llm.GenerateRequest{})
			checkCall(t, s, FailAtShorten, resp, err, longShortenedText)

			resp, err = job.Review().Generate(ctx, llm.GenerateRequest{})
			checkCall(t, s, FailAtReview, resp, err, reviewResponse)

			if _, err := job.Generation().Generate(ctx, llm.GenerateRequest{}); !errors.Is(err, ErrUnexpectedCall) {
				t.Errorf("third generation call error = %v, want ErrUnexpectedCall", err)
			}
			if _, err := job.Review().Generate(ctx, llm.GenerateRequest{}); !errors.Is(err, ErrUnexpectedCall) {
				t.Errorf("second review call error = %v, want ErrUnexpectedCall", err)
			}
		})
	}

	t.Run("unknown failure fails secure", func(t *testing.T) {
		s := Scenario{failure: FailureTimeout + 1, failAt: FailAtDraft}
		if _, err := s.NewJob().Generation().Generate(ctx, llm.GenerateRequest{}); !errors.Is(err, ErrUnexpectedCall) {
			t.Errorf("Generate() error = %v, want ErrUnexpectedCall", err)
		}
	})
	t.Run("unknown article fails secure", func(t *testing.T) {
		s := Scenario{article: ArticleElements + 1}
		if _, err := s.NewJob().Generation().Generate(ctx, llm.GenerateRequest{}); !errors.Is(err, ErrUnexpectedCall) {
			t.Errorf("Generate() error = %v, want ErrUnexpectedCall", err)
		}
	})
}

// generate calls client and fails the test on an error.
func generate(t *testing.T, client llm.LLMClient) string {
	t.Helper()
	resp, err := client.Generate(context.Background(), llm.GenerateRequest{})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	return resp.Text
}

func TestJobsIndependent(t *testing.T) {
	s := mustScenario(t, ArticleLong, FailureNone, FailAtNone, 0)

	t.Run("interleaved calls", func(t *testing.T) {
		first, second := s.NewJob(), s.NewJob()
		for _, job := range []Job{first, second} {
			if got := generate(t, job.Generation()); got != longDraftText {
				t.Error("first call of a job did not return the draft")
			}
		}
		for _, job := range []Job{first, second} {
			if got := generate(t, job.Generation()); got != longShortenedText {
				t.Error("second call of a job did not return the shortened draft")
			}
		}
	})

	t.Run("overlapping calls", func(t *testing.T) {
		// Each call reports that it is waiting and then waits for release,
		// so both jobs are inside their first call before either returns.
		entered := make(chan struct{})
		release := make(chan struct{})
		held := WithWaitForTest(s, func(context.Context, time.Duration) {
			entered <- struct{}{}
			<-release
		})
		jobs := []Job{held.NewJob(), held.NewJob()}
		texts := make([]string, len(jobs))
		errs := make([]error, len(jobs))
		var wg sync.WaitGroup
		for i, job := range jobs {
			wg.Go(func() {
				resp, err := job.Generation().Generate(context.Background(), llm.GenerateRequest{})
				texts[i], errs[i] = resp.Text, err
			})
		}
		for range jobs {
			<-entered
		}
		close(release)
		wg.Wait()
		for i := range jobs {
			if errs[i] != nil || texts[i] != longDraftText {
				t.Errorf("job %d first call = (text equal to draft: %v, error %v), want the draft", i, texts[i] == longDraftText, errs[i])
			}
		}
	})

	t.Run("job after a finished job", func(t *testing.T) {
		before := s.NewJob()
		generate(t, before.Generation())
		generate(t, before.Generation())
		if got := generate(t, s.NewJob().Generation()); got != longDraftText {
			t.Error("first call of a later job did not return the draft")
		}
	})
}

func TestIgnoresRequest(t *testing.T) {
	requests := []llm.GenerateRequest{
		{},
		{SystemPrompt: "system", UserPrompt: "user", MaxOutputTokens: 4096},
		{SystemPrompt: "other", UserPrompt: "shorten this", MaxOutputTokens: 1},
		{SystemPrompt: "\xff", UserPrompt: "", MaxOutputTokens: -1},
	}
	scenarios := []Scenario{
		mustScenario(t, ArticleStandard, FailureNone, FailAtNone, 0),
		mustScenario(t, ArticleLong, FailureAPIError, FailAtShorten, 0),
	}
	for _, s := range scenarios {
		t.Run(s.LogValue().String(), func(t *testing.T) {
			var want [3]llm.GenerateResponse
			var wantErr [3]bool
			for i, req := range requests {
				job := s.NewJob()
				calls := []llm.LLMClient{job.Generation(), job.Generation(), job.Review()}
				for c, client := range calls {
					resp, err := client.Generate(context.Background(), req)
					if i == 0 {
						want[c], wantErr[c] = resp, err != nil
						continue
					}
					if resp != want[c] || (err != nil) != wantErr[c] {
						t.Errorf("request %d call %d = (%+v, %v), want the result of the first request", i, c, resp, err)
					}
					if err != nil && !errors.Is(err, ErrSimulatedAPIFailure) && !errors.Is(err, ErrUnexpectedCall) {
						t.Errorf("request %d call %d error = %v, want the scenario's failure", i, c, err)
					}
				}
			}
		})
	}
}

// waitRecorder replaces a scenario's wait and records each delay it is asked
// to wait for. It returns at once.
type waitRecorder struct {
	mu     sync.Mutex
	delays []time.Duration
}

func (r *waitRecorder) wait(_ context.Context, d time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.delays = append(r.delays, d)
}

func (r *waitRecorder) recorded() []time.Duration {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]time.Duration(nil), r.delays...)
}

func TestDelayAndCancel(t *testing.T) {
	for _, delay := range []time.Duration{0, 3 * time.Second} {
		t.Run("every call waits for the delay "+delay.String(), func(t *testing.T) {
			var rec waitRecorder
			s := WithWaitForTest(mustScenario(t, ArticleLong, FailureEmpty, FailAtReview, delay), rec.wait)
			job := s.NewJob()
			generate(t, job.Generation())
			generate(t, job.Generation())
			if _, err := job.Review().Generate(context.Background(), llm.GenerateRequest{}); err == nil {
				t.Fatal("review call error = nil, want the scenario's failure")
			}
			got := rec.recorded()
			if len(got) != 3 {
				t.Fatalf("wait called %d times, want 3 (one per call, the failing call included)", len(got))
			}
			for i, d := range got {
				if d != delay {
					t.Errorf("wait %d delay = %v, want %v", i, d, delay)
				}
			}
		})
	}

	t.Run("a deadline ends the real wait", func(t *testing.T) {
		s := mustScenario(t, ArticleStandard, FailureNone, FailAtNone, MaxDelay)
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		t.Cleanup(cancel)
		done := make(chan error, 1)
		go func() {
			_, err := s.NewJob().Generation().Generate(ctx, llm.GenerateRequest{})
			done <- err
		}()
		select {
		case err := <-done:
			if !errors.Is(err, context.DeadlineExceeded) {
				t.Errorf("Generate() error = %v, want context.DeadlineExceeded", err)
			}
		case <-time.After(30 * time.Second):
			t.Fatal("Generate() did not return after its context's deadline")
		}
	})

	t.Run("a canceled context fails a call with no delay", func(t *testing.T) {
		s := mustScenario(t, ArticleStandard, FailureNone, FailAtNone, 0)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		resp, err := s.NewJob().Generation().Generate(ctx, llm.GenerateRequest{})
		if !errors.Is(err, context.Canceled) {
			t.Errorf("Generate() error = %v, want context.Canceled", err)
		}
		if resp != (llm.GenerateResponse{}) {
			t.Errorf("Generate() response = %+v, want the zero value", resp)
		}
	})
}
