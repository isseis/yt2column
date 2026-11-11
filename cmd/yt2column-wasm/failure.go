//go:build js && wasm

package main

import (
	"context"
	"errors"

	"github.com/isseis/yt2column/internal/llm"
	"github.com/isseis/yt2column/internal/llm/claude"
	"github.com/isseis/yt2column/internal/writer"
)

// failureKind is the kind of a failure returned to the page. The kinds of the
// validation steps are chosen by the step that rejected; every other kind is
// chosen by matching a sentinel in classify, never by an error's text.
type failureKind string

const (
	kindInvalidInput     failureKind = "invalid_input"
	kindInvalidVideoURL  failureKind = "invalid_video_url"
	kindTranscriptFormat failureKind = "transcript_format"
	kindInvalidConfig    failureKind = "invalid_config"
	kindInvalidTranscr   failureKind = "invalid_transcript"
	kindPromptTooLarge   failureKind = "prompt_too_large"
	kindInvalidTemplate  failureKind = "invalid_template"
	kindInvalidRequest   failureKind = "invalid_request"
	kindHTTPStatus       failureKind = "http_status"
	kindTransport        failureKind = "transport"
	kindInvalidResponse  failureKind = "invalid_response"
	kindTruncated        failureKind = "truncated"
	kindEmptyResponse    failureKind = "empty_response"
	kindUnexpectedFinish failureKind = "unexpected_finish"
	kindMalformedOutput  failureKind = "malformed_output"
	kindInvalidReview    failureKind = "invalid_review"
	kindTooLong          failureKind = "too_long"
	kindUnpublishable    failureKind = "unpublishable"
	kindTimeout          failureKind = "timeout"
	kindCanceled         failureKind = "canceled"
	kindPanic            failureKind = "panic"
	kindUnknown          failureKind = "unknown"
)

// invalidConfigMessage is the fixed text of every invalid_config failure. The
// errors of claudeparam.ParseEffort and claude.New can quote the value they
// received, and an API key pasted into the wrong field would then be shown.
const invalidConfigMessage = "the API key, the workspace ID, a model, or an effort was not accepted"

// failureReport is the Failure object of the page interface.
type failureReport struct {
	Kind       failureKind `json:"kind"`
	HTTPStatus *int        `json:"httpStatus"`
	Message    string      `json:"message"`
}

func newFailure(kind failureKind, message string) *failureReport {
	return &failureReport{Kind: kind, Message: message}
}

// classify maps an error returned after validation to its failure. The cases
// are tried in order; llmhttp never wraps a context error together with an
// adapter sentinel, so the order only decides between the writer sentinels a
// post-generation step can wrap around an llm sentinel.
func classify(err error) *failureReport {
	report := &failureReport{Kind: classifyKind(err), Message: err.Error()}
	if statusErr, ok := errors.AsType[*claude.HTTPStatusError](err); ok {
		status := statusErr.StatusCode
		report.HTTPStatus = &status
	}
	return report
}

func classifyKind(err error) failureKind {
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return kindTimeout
	case errors.Is(err, context.Canceled):
		return kindCanceled
	case errors.Is(err, claude.ErrHTTPStatus):
		return kindHTTPStatus
	case errors.Is(err, claude.ErrTransport):
		return kindTransport
	case errors.Is(err, claude.ErrInvalidResponse):
		return kindInvalidResponse
	case errors.Is(err, llm.ErrInvalidRequest):
		return kindInvalidRequest
	case errors.Is(err, llm.ErrTruncated):
		return kindTruncated
	case errors.Is(err, llm.ErrEmptyResponse):
		return kindEmptyResponse
	case errors.Is(err, llm.ErrUnexpectedFinishReason):
		return kindUnexpectedFinish
	case errors.Is(err, writer.ErrInvalidTranscript):
		return kindInvalidTranscr
	case errors.Is(err, writer.ErrPromptTooLarge):
		return kindPromptTooLarge
	case errors.Is(err, writer.ErrInvalidTemplate):
		return kindInvalidTemplate
	case errors.Is(err, writer.ErrMalformedOutput), errors.Is(err, writer.ErrInvalidArticle):
		return kindMalformedOutput
	case errors.Is(err, writer.ErrInvalidReview):
		return kindInvalidReview
	case errors.Is(err, writer.ErrTooLong):
		return kindTooLong
	default:
		return kindUnknown
	}
}
