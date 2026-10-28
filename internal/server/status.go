package server

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/isseis/yt2column/internal/articlecache"
	"github.com/isseis/yt2column/internal/publisher"
	"github.com/isseis/yt2column/internal/writer"
)

// maxStatusResponseBytes bounds the body of a status response. The completion
// check refuses an article whose longest status response would exceed it, so
// every status response the server writes fits, and the host relays twice this
// size.
const maxStatusResponseBytes = 256 << 10

// statusResponse is the body of a status response. Every value is a code, an
// identifier the server generated, a number, a boolean, or, for a done
// article, the article's two strings.
type statusResponse struct {
	Status            string    `json:"status"`
	RequestID         string    `json:"requestId"`
	WebhookConfigured bool      `json:"webhookConfigured"`
	Job               statusJob `json:"job"`
}

// statusJob is the job member. Which members it carries depends on State: none
// for unknown, Steps for running, Steps, Article, and Publish for done, and
// Steps and Failure for failed.
type statusJob struct {
	State   string         `json:"state"`
	Steps   *statusSteps   `json:"steps,omitempty"`
	Article *statusArticle `json:"article,omitempty"`
	Publish *statusPublish `json:"publish,omitempty"`
	Failure string         `json:"failure,omitempty"`
}

// statusSteps holds each step's state code. Its JSON member names are the step
// codes the documentation lists.
type statusSteps struct {
	Draft   string `json:"draft"`
	Shorten string `json:"shorten"`
	Review  string `json:"review"`
}

// statusArticle is a done article: the display Markdown, the --out file
// content, the cost, and the generation's duration.
type statusArticle struct {
	Markdown   string     `json:"markdown"`
	File       string     `json:"file"`
	Cost       statusCost `json:"cost"`
	DurationMs int64      `json:"durationMs"`
}

// statusCost is the article's cost. USD is set only when State is known.
type statusCost struct {
	State string `json:"state"`
	USD   string `json:"usd,omitempty"`
}

// statusPublish is a done article's publish state, with its reason for the
// failed and unpostable states.
type statusPublish struct {
	State  string `json:"state"`
	Reason string `json:"reason,omitempty"`
}

// The codes more than one set shares: a job and a step are both running,
// done, or failed, and a post can fail too.
const (
	codeRunning = "running"
	codeDone    = "done"
	codeFailed  = "failed"
	codeUnknown = "unknown"
)

// The cost state codes.
const (
	costStateKnown   = "known"
	costStateUnknown = codeUnknown
)

// renderStatus returns the status response body for snap. For a done record
// it renders the article's Markdown and file with the publisher's functions;
// their error is returned, which for an article the completion check passed
// means an implementation mistake.
func renderStatus(requestID string, webhookConfigured bool, snap articlecache.Snapshot) ([]byte, error) {
	job := statusJob{State: jobStateCode(snap.State)}
	switch snap.State {
	case articlecache.JobRunning:
		job.Steps = stepsStatus(snap.Stages)
	case articlecache.JobDone:
		article, err := articleStatus(snap.Article, snap.Duration)
		if err != nil {
			return nil, err
		}
		job.Steps = stepsStatus(snap.Stages)
		job.Article = article
		job.Publish = publishStatus(snap)
	case articlecache.JobFailed:
		job.Steps = stepsStatus(snap.Stages)
		job.Failure = failureCode(snap.Failure)
	default:
		// An unknown key carries no member but its state.
	}
	data, err := json.Marshal(statusResponse{
		Status:            "ok",
		RequestID:         requestID,
		WebhookConfigured: webhookConfigured,
		Job:               job,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal the status response: %w", err)
	}
	return data, nil
}

// stepsStatus returns the code of each step's state.
func stepsStatus(st articlecache.Stages) *statusSteps {
	return &statusSteps{
		Draft:   stageStateCode(st.Draft),
		Shorten: stageStateCode(st.Shorten),
		Review:  stageStateCode(st.Review),
	}
}

// articleStatus renders a done article's member. The cost is formatted by the
// function the --out file and the Slack footnote use, so the three agree.
func articleStatus(a writer.Article, duration time.Duration) (*statusArticle, error) {
	markdown, err := publisher.DisplayMarkdown(a)
	if err != nil {
		return nil, fmt.Errorf("render the display Markdown: %w", err)
	}
	file, err := publisher.FileContent(a)
	if err != nil {
		return nil, fmt.Errorf("render the file content: %w", err)
	}
	cost := statusCost{State: costStateUnknown}
	if usd, known := publisher.FormatArticleCost(a.RunUsage); known {
		cost = statusCost{State: costStateKnown, USD: usd}
	}
	return &statusArticle{Markdown: markdown, File: file, Cost: cost, DurationMs: duration.Milliseconds()}, nil
}

// publishStatus returns a done record's publish state, with the reason the
// state carries.
func publishStatus(snap articlecache.Snapshot) *statusPublish {
	p := &statusPublish{State: publishStateCode(snap.Publish)}
	switch snap.Publish {
	case articlecache.PublishUnpostable:
		p.Reason = unpostableCode(snap.Unpostable)
	case articlecache.PublishFailed:
		p.Reason = publishFailureCode(snap.PublishFailure)
	default:
		// The other states carry no reason.
	}
	return p
}

// maxStatusSize returns the size of the longest status response a done record
// of a can get. The values other than the article that a response can carry
// vary only within fixed bounds, so it renders a response with each of them at
// its longest: a request ID of the generated length, a false
// webhookConfigured, the longest duration, each step state a done record can
// hold, and every publish state with every reason. The article's strings and
// its cost do not change once it is stored.
func maxStatusSize(a writer.Article) (int, error) {
	requestID := strings.Repeat("0", hex.EncodedLen(requestIDBytes))
	duration := time.Duration(math.MaxInt64)
	largest := 0
	for _, step := range []articlecache.StageState{articlecache.StageDone, articlecache.StageSkipped} {
		for _, publish := range articlecache.PublishStates() {
			for _, unpostable := range articlecache.UnpostableReasons() {
				for _, publishFailure := range articlecache.PublishFailureReasons() {
					data, err := renderStatus(requestID, false, articlecache.Snapshot{
						State:          articlecache.JobDone,
						Stages:         articlecache.Stages{Draft: step, Shorten: step, Review: step},
						Article:        a,
						Duration:       duration,
						Publish:        publish,
						Unpostable:     unpostable,
						PublishFailure: publishFailure,
					})
					if err != nil {
						return 0, err
					}
					largest = max(largest, len(data))
				}
			}
		}
	}
	return largest, nil
}

// jobStateCode returns the code of a job state. A value outside the
// enumeration is unknown, the state that claims nothing.
func jobStateCode(s articlecache.JobState) string {
	switch s {
	case articlecache.JobRunning:
		return codeRunning
	case articlecache.JobDone:
		return codeDone
	case articlecache.JobFailed:
		return codeFailed
	default:
		return codeUnknown
	}
}

// stageStateCode returns the code of a step's state. A value outside the
// enumeration is pending, the state that claims nothing.
func stageStateCode(s articlecache.StageState) string {
	switch s {
	case articlecache.StageRunning:
		return codeRunning
	case articlecache.StageDone:
		return codeDone
	case articlecache.StageSkipped:
		return "skipped"
	case articlecache.StageFailed:
		return codeFailed
	default:
		return "pending"
	}
}

// failureCode returns the code of a generation failure reason. A value outside
// the enumeration is internal.
func failureCode(r articlecache.FailureReason) string {
	switch r {
	case articlecache.FailureTranscriptTooLong:
		return "transcript_too_long"
	case articlecache.FailureLLMTimeout:
		return "llm_timeout"
	case articlecache.FailureLLMEmpty:
		return "llm_empty"
	case articlecache.FailureLLMTruncated:
		return "llm_truncated"
	case articlecache.FailureLLM:
		return "llm_failed"
	case articlecache.FailureArticleInvalid:
		return "article_invalid"
	case articlecache.FailureArticleTooLarge:
		return "article_too_large"
	default:
		return "internal"
	}
}

// publishStateCode returns the code of a publish state. A value outside the
// enumeration is unpostable, which neither sends nor claims a delivery.
func publishStateCode(s articlecache.PublishState) string {
	switch s {
	case articlecache.PublishNotPosted:
		return "not_posted"
	case articlecache.PublishPosting:
		return "posting"
	case articlecache.PublishPosted:
		return "posted"
	case articlecache.PublishFailed:
		return codeFailed
	default:
		return "unpostable"
	}
}

// unpostableCode returns the code of an unpostable reason. A value outside the
// enumeration is other.
func unpostableCode(r articlecache.UnpostableReason) string {
	switch r {
	case articlecache.UnpostableMention:
		return "mention"
	default:
		return "other"
	}
}

// publishFailureCode returns the code of a publish failure reason. A value
// outside the enumeration is internal.
func publishFailureCode(r articlecache.PublishFailureReason) string {
	switch r {
	case articlecache.PublishFailureWebhook:
		return "webhook"
	default:
		return "internal"
	}
}
