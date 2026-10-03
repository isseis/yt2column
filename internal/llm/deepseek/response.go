package deepseek

import (
	"bytes"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/isseis/yt2column/internal/llm"
	"github.com/isseis/yt2column/internal/strictjson"
)

// Response body member names of the Chat Completions API. Only the members
// the adapter consumes are named; every other member is ignored, duplicates
// and differing kinds included.
const (
	keyModel             = "model"
	keyChoices           = "choices"
	keySystemFingerprint = "system_fingerprint"
	keyFinishReason      = "finish_reason"
	keyMessage           = "message"
	keyContent           = "content"
	keyError             = "error"
)

// finish_reason values that carry a defined meaning for the adapter.
const (
	finishReasonStop   = "stop"
	finishReasonLength = "length"
)

// maxReasonBytes caps how much of the untrusted finish_reason value an error
// message quotes, so it cannot flood the caller's terminal.
const maxReasonBytes = 64

// jsonWhitespace is the set of insignificant whitespace bytes of RFC 8259.
const jsonWhitespace = " \t\r\n"

// readResponseBody reads at most maxResponseBytes+1 bytes. One byte past the
// limit is enough to detect an oversized body without buffering it all.
func readResponseBody(body io.Reader) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(body, maxResponseBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxResponseBytes {
		return nil, fmt.Errorf("%w: response body exceeds the %d-byte limit", ErrInvalidResponse, maxResponseBytes)
	}
	return data, nil
}

// parseResponse validates one response body and builds the GenerateResponse.
// The checks run in the requirement order: body shape, finish reason, content.
// Any rejection returns the zero GenerateResponse.
func parseResponse(data []byte) (llm.GenerateResponse, error) {
	// Only JSON whitespace counts: bytes.TrimSpace would also strip Unicode
	// spaces such as U+00A0, which are not JSON whitespace.
	if len(bytes.Trim(data, jsonWhitespace)) == 0 {
		return llm.GenerateResponse{}, invalidResponse("response body is whitespace only (%d bytes)", len(data))
	}
	object, err := strictjson.ParseObject(data)
	if err != nil {
		return llm.GenerateResponse{}, invalidResponse("%v", err)
	}
	members, err := object.Collect(keyModel, keyChoices, keySystemFingerprint)
	if err != nil {
		return llm.GenerateResponse{}, topLevelFailure(object, err)
	}
	model, err := strictjson.RequiredString(members, keyModel)
	if err != nil {
		return llm.GenerateResponse{}, topLevelFailure(object, err)
	}
	modelVersion, _, err := strictjson.OptionalString(members, keySystemFingerprint)
	if err != nil {
		return llm.GenerateResponse{}, topLevelFailure(object, err)
	}
	choicesValue, err := strictjson.Required(members, keyChoices)
	if err != nil {
		return llm.GenerateResponse{}, topLevelFailure(object, err)
	}
	choices, err := choicesValue.AsArray()
	if err != nil {
		return llm.GenerateResponse{}, topLevelFailure(object, err)
	}
	if len(choices) != 1 {
		return llm.GenerateResponse{}, topLevelFailure(object, fmt.Errorf("%w: got %d", errChoiceCount, len(choices)))
	}
	choice, err := choices[0].AsObject()
	if err != nil {
		return llm.GenerateResponse{}, topLevelFailure(object, err)
	}
	finishReason, content, err := parseChoice(object, choice)
	if err != nil {
		return llm.GenerateResponse{}, err
	}

	switch finishReason {
	case finishReasonLength:
		return llm.GenerateResponse{}, fmt.Errorf("%w: finish_reason is %s", llm.ErrTruncated, quoteReason(finishReason))
	case finishReasonStop:
	default:
		return llm.GenerateResponse{}, fmt.Errorf("%w: finish_reason is %s", llm.ErrUnexpectedFinishReason, quoteReason(finishReason))
	}
	if strings.TrimSpace(content) == "" {
		return llm.GenerateResponse{}, fmt.Errorf("%w: content is empty or whitespace only", llm.ErrEmptyResponse)
	}
	return llm.GenerateResponse{
		Text:         content,
		Model:        model,
		ModelVersion: modelVersion,
	}, nil
}

// parseChoice validates choices[0]'s consumed members and returns the finish
// reason and the content. finish_reason is read before message and content so
// a later shape failure can name it in the diagnostic; it is read as a plain
// string, so its value (the empty string included) is classified by the
// caller's finish-reason check rather than by the shape checks here.
func parseChoice(object, choice strictjson.Object) (string, string, error) {
	choiceMembers, err := choice.Collect(keyFinishReason, keyMessage)
	if err != nil {
		return "", "", topLevelFailure(object, err)
	}
	finishReasonValue, err := strictjson.Required(choiceMembers, keyFinishReason)
	if err != nil {
		return "", "", topLevelFailure(object, err)
	}
	finishReason, err := finishReasonValue.AsString()
	if err != nil {
		return "", "", topLevelFailure(object, fmt.Errorf("%s: %w", keyFinishReason, err))
	}
	messageValue, err := strictjson.Required(choiceMembers, keyMessage)
	if err != nil {
		return "", "", shapeFailure(object, finishReason, err)
	}
	message, err := messageValue.AsObject()
	if err != nil {
		return "", "", shapeFailure(object, finishReason, err)
	}
	messageMembers, err := message.Collect(keyContent)
	if err != nil {
		return "", "", shapeFailure(object, finishReason, err)
	}
	contentValue, err := strictjson.Required(messageMembers, keyContent)
	if err != nil {
		return "", "", shapeFailure(object, finishReason, err)
	}
	content, err := contentValue.AsString()
	if err != nil {
		return "", "", shapeFailure(object, finishReason, err)
	}
	return finishReason, content, nil
}

// invalidResponse formats a response validation failure around the
// ErrInvalidResponse sentinel.
func invalidResponse(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalidResponse, fmt.Sprintf(format, args...))
}

// topLevelFailure adds the presence of a top-level error member to a
// validation failure, without its value, so a provider error response is
// recognizable.
func topLevelFailure(object strictjson.Object, err error) error {
	diagnostic := err.Error()
	if object.Has(keyError) {
		diagnostic += fmt.Sprintf("; response has a top-level %q member", keyError)
	}
	return invalidResponse("%s", diagnostic)
}

// shapeFailure formats a body-shape failure that happened after finish_reason
// was read: the diagnostic names the finish reason and, when present, the
// top-level error member, without the error member's value.
func shapeFailure(object strictjson.Object, finishReason string, err error) error {
	diagnostic := fmt.Sprintf("%v (finish_reason %s)", err, quoteReason(finishReason))
	if object.Has(keyError) {
		diagnostic += fmt.Sprintf("; response has a top-level %q member", keyError)
	}
	return invalidResponse("%s", diagnostic)
}

// quoteReason renders the untrusted finish_reason for an error message:
// quoted, with control characters escaped, and capped at maxReasonBytes
// bytes so it cannot flood the caller's terminal.
func quoteReason(reason string) string {
	if len(reason) > maxReasonBytes {
		reason = reason[:maxReasonBytes]
	}
	return strconv.Quote(reason)
}
