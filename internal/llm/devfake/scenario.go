// Package devfake is a fake LLM for development builds: llm.LLMClient
// implementations that return fixed articles and the failures a Scenario
// selects, without calling an LLM, the network, an external command, or the
// file system. It is not a test double: a development binary links it, so it
// is not in a testutil directory and carries no test build tag. It depends on
// internal/llm and the standard library only, so it also builds for js/wasm.
package devfake

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"
)

var (
	// ErrInvalidScenario reports a combination NewScenario does not accept.
	ErrInvalidScenario = errors.New("invalid fake scenario")
	// ErrSimulatedAPIFailure is the failure FailureAPIError returns. It wraps
	// no llm sentinel, so a caller classifies it as an LLM failure.
	ErrSimulatedAPIFailure = errors.New("simulated LLM API failure")
	// ErrUnexpectedCall reports a call the scenario does not expect: a third
	// call of the generation client or a second call of the review client.
	ErrUnexpectedCall = errors.New("unexpected call to the fake LLM client")
)

// Article selects the fixed article the fake returns.
type Article int

// The Article values.
const (
	ArticleStandard Article = iota // the zero value: an article the shorten step leaves alone
	ArticleLong                    // a draft over the body limit, so the shorten step runs
	ArticleElements                // an article holding every viewer Markdown rule the writer accepts
)

// Failure selects the failure the fake returns at FailAt.
type Failure int

// The Failure values.
const (
	FailureNone      Failure = iota // the zero value: no call fails
	FailureEmpty                    // wraps llm.ErrEmptyResponse
	FailureTruncated                // wraps llm.ErrTruncated
	FailureAPIError                 // wraps ErrSimulatedAPIFailure and no llm sentinel
	FailureTimeout                  // wraps context.DeadlineExceeded
)

// FailAt is the call a failure is returned at. It names a call of a Job's
// clients, not a writer step: the writer decides which step makes the call.
type FailAt int

// The FailAt values.
const (
	FailAtNone    FailAt = iota // the zero value: no call fails
	FailAtDraft                 // the first call of the generation client
	FailAtShorten               // the second call of the generation client
	FailAtReview                // the first call of the review client
)

// MaxDelay is the longest delay NewScenario accepts.
const MaxDelay = 600 * time.Second

// ModelName is the model every fake response reports. It is not in the
// pricing table, so the cost of a fake article is unknown.
const ModelName = "yt2column-devfake"

// TitleMarker starts the title of every fake article.
const TitleMarker = "[FAKE] "

// The spellings of the enumeration values. They are the command-line values
// of the development server and the values of its startup line, so the two
// cannot drift apart. The none values are only ever printed: a caller that
// accepts a spelling matches it against the accepted lists below, which do
// not hold them.
const (
	spellingNone = "none"

	spellingStandard = "standard"
	spellingLong     = "long"
	spellingElements = "elements"

	spellingEmpty     = "empty"
	spellingTruncated = "truncated"
	spellingAPIError  = "api-error"
	spellingTimeout   = "timeout"

	spellingDraft   = "draft"
	spellingShorten = "shorten"
	spellingReview  = "review"
)

// String returns the article's command-line spelling. A value outside the
// enumeration returns a string no accepted spelling equals.
func (a Article) String() string {
	switch a {
	case ArticleStandard:
		return spellingStandard
	case ArticleLong:
		return spellingLong
	case ArticleElements:
		return spellingElements
	default:
		return fmt.Sprintf("Article(%d)", int(a))
	}
}

// String returns the failure's command-line spelling, and "none" for
// FailureNone, which no command-line value selects. A value outside the
// enumeration returns a string no accepted spelling equals.
func (f Failure) String() string {
	switch f {
	case FailureNone:
		return spellingNone
	case FailureEmpty:
		return spellingEmpty
	case FailureTruncated:
		return spellingTruncated
	case FailureAPIError:
		return spellingAPIError
	case FailureTimeout:
		return spellingTimeout
	default:
		return fmt.Sprintf("Failure(%d)", int(f))
	}
}

// String returns the call's command-line spelling, the same as the job API's
// step code, and "none" for FailAtNone, which no command-line value selects. A
// value outside the enumeration returns a string no accepted spelling equals.
func (f FailAt) String() string {
	switch f {
	case FailAtNone:
		return spellingNone
	case FailAtDraft:
		return spellingDraft
	case FailAtShorten:
		return spellingShorten
	case FailAtReview:
		return spellingReview
	default:
		return fmt.Sprintf("FailAt(%d)", int(f))
	}
}

// Articles returns every article a caller may select, in declaration order.
func Articles() []Article {
	return []Article{ArticleStandard, ArticleLong, ArticleElements}
}

// Failures returns every failure a caller may select, in declaration order.
// FailureNone is not in it: no failure is selected by omitting one.
func Failures() []Failure {
	return []Failure{FailureEmpty, FailureTruncated, FailureAPIError, FailureTimeout}
}

// FailAts returns every call a caller may select a failure at, in declaration
// order. FailAtNone is not in it: no call is selected by omitting one.
func FailAts() []FailAt {
	return []FailAt{FailAtDraft, FailAtShorten, FailAtReview}
}

// waitFunc waits for one call's delay. It returns when the delay has passed
// or ctx is done, whichever comes first; the caller checks ctx afterwards.
type waitFunc func(ctx context.Context, delay time.Duration)

// Scenario is a checked scenario. Its fields are unexported, so a Scenario
// can only come from NewScenario; the zero value is the standard article
// with no failure and no delay.
type Scenario struct {
	article Article
	failure Failure
	failAt  FailAt
	delay   time.Duration
	// wait is nil except in a copy a test made; nil selects waitDelay.
	wait waitFunc
}

// NewScenario checks the combination and returns the Scenario. It rejects,
// with an error wrapping ErrInvalidScenario, a value outside an enumeration,
// a failure without a FailAt or a FailAt without a failure, FailAtShorten
// with an article other than ArticleLong, and a delay that is negative or
// over MaxDelay.
func NewScenario(article Article, failure Failure, failAt FailAt, delay time.Duration) (Scenario, error) {
	switch article {
	case ArticleStandard, ArticleLong, ArticleElements:
	default:
		return Scenario{}, fmt.Errorf("%w: unknown article %d", ErrInvalidScenario, int(article))
	}
	switch failure {
	case FailureNone, FailureEmpty, FailureTruncated, FailureAPIError, FailureTimeout:
	default:
		return Scenario{}, fmt.Errorf("%w: unknown failure %d", ErrInvalidScenario, int(failure))
	}
	switch failAt {
	case FailAtNone, FailAtDraft, FailAtShorten, FailAtReview:
	default:
		return Scenario{}, fmt.Errorf("%w: unknown failing call %d", ErrInvalidScenario, int(failAt))
	}
	if (failure == FailureNone) != (failAt == FailAtNone) {
		return Scenario{}, fmt.Errorf("%w: a failure and its failing call must be given together", ErrInvalidScenario)
	}
	// The shorten call happens only for a draft over the body limit, which
	// only the long article has; any other article would never fail.
	if failAt == FailAtShorten && article != ArticleLong {
		return Scenario{}, fmt.Errorf("%w: only the long article reaches the shorten call", ErrInvalidScenario)
	}
	if delay < 0 || delay > MaxDelay {
		return Scenario{}, fmt.Errorf("%w: delay is outside 0 to %v", ErrInvalidScenario, MaxDelay)
	}
	return Scenario{article: article, failure: failure, failAt: failAt, delay: delay}, nil
}

// LogValue reports the scenario with the command-line spellings: article,
// fail ("none" when no call fails), failStep (only when a call fails), and
// delay in whole seconds.
func (s Scenario) LogValue() slog.Value {
	attrs := []slog.Attr{
		slog.String("article", s.article.String()),
		slog.String("fail", s.failure.String()),
	}
	if s.failure != FailureNone {
		attrs = append(attrs, slog.String("failStep", s.failAt.String()))
	}
	attrs = append(attrs, slog.Int64("delay", int64(s.delay/time.Second)))
	return slog.GroupValue(attrs...)
}

var _ slog.LogValuer = Scenario{}
