package extjob

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/isseis/yt2column/internal/llm"
	"github.com/isseis/yt2column/internal/publisher"
	"github.com/isseis/yt2column/internal/writer"
)

// maxResultBytes bounds the JSON of a done result. The completion check
// refuses an article whose longest result JSON would exceed it, so every
// result a run returns fits the bound the job record and the page's checks
// apply.
const maxResultBytes = 256 << 10

// ArticleResult is a done article as the extension shows, saves, and copies
// it. The Markdown, the video title, and the channel name have their mentions
// neutralized, so neither the copy nor a selection of the viewer's page pastes
// a mention; the file is not changed. The cost is formatted by the function
// the CLI's --out file uses, so the two agree.
type ArticleResult struct {
	Markdown            string `json:"markdown"`
	MentionsNeutralized bool   `json:"mentionsNeutralized"`
	VideoTitle          string `json:"videoTitle"`
	ChannelName         string `json:"channelName"`
	File                string `json:"file"`
	Cost                Cost   `json:"cost"`
	DurationMs          int64  `json:"durationMs"`
}

// The Cost states.
const (
	CostKnown   = "known"
	CostUnknown = "unknown"
)

// Cost is the article's cost. USD is set only when State is CostKnown.
type Cost struct {
	State string `json:"state"`
	USD   string `json:"usd,omitempty"`
}

// Result is a run's outcome: an article, or a Failure with its steps. The
// zero Result is neither, and reports neither through its methods.
type Result struct {
	steps   Steps
	failed  bool
	failure Failure
	detail  InternalDetail
	article *ArticleResult
}

// Steps returns the state of each step when the run ended.
func (r Result) Steps() Steps { return r.steps }

// Article returns the done article, and false for a failed run.
func (r Result) Article() (ArticleResult, bool) {
	if r.article == nil {
		return ArticleResult{}, false
	}
	return *r.article, true
}

// Failure returns why the run failed, and false for a run that is done.
func (r Result) Failure() (Failure, bool) { return r.failure, r.failed }

// InternalDetail returns the kind of an internal failure, or DetailNone.
func (r Result) InternalDetail() InternalDetail { return r.detail }

// failedResult builds the result of a failed run. The step that was running
// becomes failed, so the failed step is the one that was started and never
// finished.
func failedResult(steps Steps, failure Failure, detail InternalDetail) Result {
	for _, field := range []*StepState{&steps.Draft, &steps.Shorten, &steps.Review} {
		if *field == StepRunning {
			*field = StepFailed
		}
	}
	return Result{steps: steps, failed: true, failure: failure, detail: detail}
}

// tracker is the writer.ProgressObserver of one run: it keeps the steps and
// passes each change on to the run's Observer.
type tracker struct {
	steps Steps
	obs   Observer
}

var _ writer.ProgressObserver = (*tracker)(nil)

func (t *tracker) set(stage writer.Stage, to StepState) {
	field := t.steps.field(stage)
	if field == nil {
		return
	}
	*field = to
	if t.obs != nil {
		t.obs.StepChanged(t.steps)
	}
}

func (t *tracker) StageStarted(s writer.Stage)  { t.set(s, StepRunning) }
func (t *tracker) StageFinished(s writer.Stage) { t.set(s, StepDone) }
func (t *tracker) StageSkipped(s writer.Stage)  { t.set(s, StepSkipped) }

// Run generates the article of p with deps and returns its result. It never
// panics to the caller: a panic becomes a Failure of FailureInternal with the
// detail DetailPanic. The writer is the one the CLI builds with no step flags,
// so both entry points generate the same article: the shorten step and the
// review step are on, and the embedded templates are used.
func Run(ctx context.Context, p Prepared, deps Deps, obs Observer) Result {
	return run(ctx, p, func() (writer.ProgressWriter, error) {
		return writer.NewProgressWriter(deps.Generation, writer.Options{
			Shorten: writer.ShortenOptions{Enabled: true},
			Review:  writer.ReviewOptions{Enabled: true, Client: deps.Review},
		})
	}, time.Now, obs)
}

// run is Run with the writer's construction and the clock as parameters, so a
// test drives the same flow with a fake writer.
func run(ctx context.Context, p Prepared, newWriter func() (writer.ProgressWriter, error), now func() time.Time, obs Observer) (res Result) {
	t := &tracker{obs: obs}
	defer func() {
		if recover() != nil {
			// The panic value is dropped: it can carry the transcript or a
			// key, and the record keeps only the kind.
			res = failedResult(t.steps, FailureInternal, DetailPanic)
		}
	}()
	started := now()

	tr, err := p.source.Fetch(ctx, p.videoURL)
	if err != nil {
		// A URL mismatch, which the validated input cannot cause, or the
		// cancellation.
		return failedResult(t.steps, FailureInternal, DetailNone)
	}
	w, err := newWriter()
	if err != nil {
		return failedResult(t.steps, FailureInternal, DetailNone)
	}
	article, err := w.WriteWithProgress(ctx, tr, t)
	if err != nil {
		return failedResult(t.steps, classifyWrite(err), DetailNone)
	}
	return complete(t.steps, article, p.videoTitle, now().Sub(started))
}

// complete runs the completion check on a generated article and returns the
// result. The checks run in this order:
//
//  1. CheckPublishable: an article no output can render fails as invalid.
//  2. The result size: an article whose result JSON, with the input's video
//     title, could exceed maxResultBytes fails as too large, so no result
//     is ever refused for its size.
//
// An article that passes both is done, whatever mentions or length it has.
func complete(steps Steps, article writer.Article, videoTitle string, duration time.Duration) Result {
	if err := article.CheckPublishable(); err != nil {
		return failedResult(steps, FailureArticleInvalid, DetailNone)
	}
	size, err := maxResultSize(article, videoTitle)
	if err != nil {
		// CheckPublishable passed, so rendering cannot fail.
		return failedResult(steps, FailureInternal, DetailNone)
	}
	if size > maxResultBytes {
		return failedResult(steps, FailureArticleTooLarge, DetailNone)
	}
	result, err := articleResult(article, videoTitle, duration)
	if err != nil {
		return failedResult(steps, FailureInternal, DetailNone)
	}
	return Result{steps: steps, article: &result}
}

// classifyWrite maps a write-stage failure to the run's failure. The order
// matters: writer wraps a prompt-too-large failure with ErrInvalidTemplate as
// well, so ErrPromptTooLarge is checked first. A 401 also matches the Claude
// adapter's ErrHTTPStatus, so a rule by HTTP status added later goes after
// llm.ErrAuthRejected.
// A canceled context is llm_failed, as on the server, where only the shutdown
// cancels a run.
func classifyWrite(err error) Failure {
	switch {
	case errors.Is(err, writer.ErrPromptTooLarge):
		return FailureTranscriptTooLong
	case errors.Is(err, context.DeadlineExceeded):
		return FailureLLMTimeout
	case errors.Is(err, llm.ErrEmptyResponse):
		return FailureLLMEmpty
	case errors.Is(err, llm.ErrTruncated):
		return FailureLLMTruncated
	case errors.Is(err, llm.ErrAuthRejected):
		return FailureAPIKeyRejected
	case errors.Is(err, writer.ErrInvalidTranscript), errors.Is(err, writer.ErrInvalidTemplate):
		// With the embedded templates and a checked transcript these cannot
		// happen, so a match means the run built something wrong.
		return FailureInternal
	default:
		return FailureLLM
	}
}

// articleResult renders a done article's member. The Markdown, the video
// title, and the channel name have their mentions neutralized; the file is not
// changed.
func articleResult(a writer.Article, videoTitle string, duration time.Duration) (ArticleResult, error) {
	display, err := publisher.NeutralizedDisplayMarkdown(a)
	if err != nil {
		return ArticleResult{}, fmt.Errorf("render the display Markdown: %w", err)
	}
	file, err := publisher.FileContent(a)
	if err != nil {
		return ArticleResult{}, fmt.Errorf("render the file content: %w", err)
	}
	cost := Cost{State: CostUnknown}
	if usd, known := publisher.FormatArticleCost(a.RunUsage); known {
		cost = Cost{State: CostKnown, USD: usd}
	}
	return ArticleResult{
		Markdown:            display.Markdown,
		MentionsNeutralized: display.Neutralized,
		VideoTitle:          publisher.NeutralizeHeading(videoTitle),
		ChannelName:         publisher.NeutralizeHeading(a.ChannelName),
		File:                file,
		Cost:                cost,
		DurationMs:          duration.Milliseconds(),
	}, nil
}

// resultJSON is the JSON of a generate result: a done one carries article, a
// failed one carries failure and, for an internal failure, internalDetail.
type resultJSON struct {
	Status         string         `json:"status"`
	Steps          stepsJSON      `json:"steps"`
	Article        *ArticleResult `json:"article,omitempty"`
	Failure        string         `json:"failure,omitempty"`
	InternalDetail InternalDetail `json:"internalDetail,omitempty"`
}

// marshalResult returns the generate result JSON of r.
func marshalResult(r Result) ([]byte, error) {
	out := resultJSON{Steps: r.steps.json()}
	if r.failed {
		out.Status = statusFailed
		out.Failure = r.failure.String()
		out.InternalDetail = r.detail
	} else {
		out.Status = statusDone
		out.Article = r.article
	}
	data, err := json.Marshal(out)
	if err != nil {
		return nil, fmt.Errorf("marshal the result: %w", err)
	}
	return data, nil
}

// maxResultSize returns the size of the longest done result JSON of a
// run started with videoTitle can get. The values other than the article and
// the title that a result carries vary only within fixed bounds, so it
// renders a result with each of them at its longest: the longest duration and
// every step state. The article's strings, the title, and the cost do not
// change once they are rendered, and the rendering neutralizes the strings as
// a real result does, so the bytes the neutralization adds are counted.
func maxResultSize(a writer.Article, videoTitle string) (int, error) {
	article, err := articleResult(a, videoTitle, time.Duration(math.MaxInt64))
	if err != nil {
		return 0, err
	}
	largest := 0
	for _, state := range allStepStates {
		data, err := marshalResult(Result{
			steps:   Steps{Draft: state, Shorten: state, Review: state},
			article: &article,
		})
		if err != nil {
			return 0, err
		}
		largest = max(largest, len(data))
	}
	return largest, nil
}
