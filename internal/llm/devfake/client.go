package devfake

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/isseis/yt2column/internal/llm"
)

// reviewResponse is the review client's answer: a list with no revision,
// which passes the review step's checks for every article and leaves the
// headings alone.
const reviewResponse = `{"revisions":[]}`

// Job is the pair of clients one generation job uses. Its fields are
// unexported, so a Job can only come from Scenario.NewJob, and its clients
// share no state with any other Job.
type Job struct {
	generation llm.LLMClient
	review     llm.LLMClient
}

// NewJob returns new clients for one generation job. Each call counts only
// its own calls, so jobs running at the same time, or one after another, get
// the same responses.
func (s Scenario) NewJob() Job {
	return Job{generation: &generationClient{scenario: s}, review: &reviewClient{scenario: s}}
}

// Generation returns the client of the draft call and the shorten call.
func (j Job) Generation() llm.LLMClient { return j.generation }

// Review returns the client of the review call.
func (j Job) Review() llm.LLMClient { return j.review }

var (
	_ llm.LLMClient = (*generationClient)(nil)
	_ llm.LLMClient = (*reviewClient)(nil)
)

// callCounter numbers the calls of one client. It holds a lock, so the
// numbering does not rely on the caller making the calls in order.
type callCounter struct {
	mu    sync.Mutex
	calls int
}

// next returns the 1-based number of this call.
func (c *callCounter) next() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.calls++
	return c.calls
}

// The numbers of the generation client's calls in one job: the writer makes the
// draft call first and the shorten call, when it makes one, second.
const (
	draftCall   = 1
	shortenCall = 2
)

// generationClient answers the draft call first and the shorten call second.
// The request is never read: the scenario alone decides the answer.
type generationClient struct {
	scenario Scenario
	counter  callCounter
}

// Generate returns the draft on the first call and the long article's
// shortened draft on the second (only the long draft is over the body limit,
// so the writer makes the second call for no other article), after the delay, or the scenario's failure at its call. A third
// call is ErrUnexpectedCall.
func (c *generationClient) Generate(ctx context.Context, _ llm.GenerateRequest) (llm.GenerateResponse, error) {
	n := c.counter.next()
	if err := c.scenario.waitDelay(ctx); err != nil {
		return llm.GenerateResponse{}, err
	}
	switch n {
	case draftCall:
		return c.scenario.respond(FailAtDraft, draftText)
	case shortenCall:
		return c.scenario.respond(FailAtShorten, func(Article) (string, error) { return longShortenedText, nil })
	default:
		return llm.GenerateResponse{}, fmt.Errorf("devfake: generation call %d: %w", n, ErrUnexpectedCall)
	}
}

// reviewClient answers the review call. The request is never read.
type reviewClient struct {
	scenario Scenario
	counter  callCounter
}

// Generate returns a review with no revision on the first call, after the
// delay, or the scenario's failure at FailAtReview. A second call is
// ErrUnexpectedCall.
func (c *reviewClient) Generate(ctx context.Context, _ llm.GenerateRequest) (llm.GenerateResponse, error) {
	n := c.counter.next()
	if err := c.scenario.waitDelay(ctx); err != nil {
		return llm.GenerateResponse{}, err
	}
	if n != 1 {
		return llm.GenerateResponse{}, fmt.Errorf("devfake: review call %d: %w", n, ErrUnexpectedCall)
	}
	return c.scenario.respond(FailAtReview, func(Article) (string, error) { return reviewResponse, nil })
}

// waitDelay waits for the scenario's delay and then reports ctx's error, if
// any. Every call goes through it, a zero delay included, so a test that
// replaced the wait sees every call, and a call whose context ended never
// succeeds.
func (s Scenario) waitDelay(ctx context.Context) error {
	wait := s.wait
	if wait == nil {
		wait = sleepContext
	}
	wait(ctx, s.delay)
	return ctx.Err()
}

// sleepContext returns after delay or when ctx is done, whichever comes
// first. A zero delay returns at once without a timer.
func sleepContext(ctx context.Context, delay time.Duration) {
	if delay <= 0 {
		return
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-timer.C:
	case <-ctx.Done():
	}
}

// respond answers the call at: the scenario's failure when it fails at this
// call, and otherwise the text text returns for the scenario's article.
func (s Scenario) respond(at FailAt, text func(Article) (string, error)) (llm.GenerateResponse, error) {
	if s.failAt == at {
		return s.fail()
	}
	body, err := text(s.article)
	if err != nil {
		return llm.GenerateResponse{}, err
	}
	return llm.GenerateResponse{Text: body, Model: ModelName}, nil
}

// fail returns the scenario's failure in the shape the llm.LLMClient contract
// gives it: the model and an empty text with an empty response or a
// truncation, and the zero response otherwise. Every message says the failure
// is simulated, so a log tells it from a real LLM failure.
func (s Scenario) fail() (llm.GenerateResponse, error) {
	switch s.failure {
	case FailureEmpty:
		return llm.GenerateResponse{Model: ModelName}, fmt.Errorf("devfake: simulated: %w", llm.ErrEmptyResponse)
	case FailureTruncated:
		return llm.GenerateResponse{Model: ModelName}, fmt.Errorf("devfake: simulated: %w", llm.ErrTruncated)
	case FailureAPIError:
		return llm.GenerateResponse{}, fmt.Errorf("devfake: %w", ErrSimulatedAPIFailure)
	case FailureTimeout:
		return llm.GenerateResponse{}, fmt.Errorf("devfake: simulated: %w", context.DeadlineExceeded)
	default:
		return llm.GenerateResponse{}, fmt.Errorf("devfake: unknown failure %d: %w", int(s.failure), ErrUnexpectedCall)
	}
}
