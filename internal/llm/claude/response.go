package claude

import (
	"bytes"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/isseis/yt2column/internal/llm"
	"github.com/isseis/yt2column/internal/llm/claudeparam"
	"github.com/isseis/yt2column/internal/strictjson"
)

// Response body member names of the Messages API. Only the members the
// adapter consumes are named; every other member is ignored, duplicates and
// differing kinds included.
const (
	keyModel      = "model"
	keyStopReason = "stop_reason"
	keyContent    = "content"
	keyType       = "type"
	keyText       = "text"
	keyError      = "error"
)

// Content block types the adapter accepts. A text block carries the answer;
// a thinking or redacted_thinking block is consumed only for its type and
// never contributes text.
const (
	blockText             = "text"
	blockThinking         = "thinking"
	blockRedactedThinking = "redacted_thinking"
)

// stop_reason values that carry a defined meaning for the adapter.
const (
	stopEndTurn   = "end_turn"
	stopMaxTokens = "max_tokens"
)

// jsonWhitespace is the set of insignificant whitespace bytes of RFC 8259.
const jsonWhitespace = " \t\r\n"

// maxReasonBytes caps how much of the untrusted stop_reason value an error
// message quotes, so it cannot flood the caller's terminal.
const maxReasonBytes = 64

// errTextBlockCount reports more than one text block. Which block would become
// the answer cannot be inferred, so the adapter rejects the response.
var errTextBlockCount = errors.New("more than one text block")

// errUnknownBlockType reports a content element whose type is not one of the
// accepted block types. The value is not included: it is untrusted.
var errUnknownBlockType = errors.New("content: unknown block type")

// requestSummary carries the request-side values that a truncation or
// unexpected finish-reason message names: the max_tokens that was sent and the
// effort. Neither is a secret.
type requestSummary struct {
	maxTokens int
	effort    claudeparam.Effort
}

// parseResponse validates one response body and builds the GenerateResponse.
// The checks run in the requirement order: body shape, stop reason, text. Any
// rejection returns the zero GenerateResponse.
func parseResponse(data []byte, summary requestSummary) (llm.GenerateResponse, error) {
	// Only JSON whitespace counts: bytes.TrimSpace would also strip Unicode
	// spaces such as U+00A0, which are not JSON whitespace.
	if len(bytes.Trim(data, jsonWhitespace)) == 0 {
		return llm.GenerateResponse{}, invalidResponse("response body is whitespace only (%d bytes)", len(data))
	}
	object, err := strictjson.ParseObject(data)
	if err != nil {
		return llm.GenerateResponse{}, invalidResponse("%v", err)
	}
	members, err := object.Collect(keyModel, keyStopReason, keyContent)
	if err != nil {
		return llm.GenerateResponse{}, topLevelFailure(object, err)
	}
	model, err := strictjson.RequiredString(members, keyModel)
	if err != nil {
		return llm.GenerateResponse{}, topLevelFailure(object, err)
	}
	stopReason, err := requiredString(members, keyStopReason)
	if err != nil {
		return llm.GenerateResponse{}, topLevelFailure(object, err)
	}
	text, hasTextBlock, err := parseContent(object, members)
	if err != nil {
		return llm.GenerateResponse{}, err
	}

	switch stopReason {
	case stopMaxTokens:
		return llm.GenerateResponse{}, truncatedError(stopReason, summary, hasTextBlock)
	case stopEndTurn:
	default:
		return llm.GenerateResponse{}, unexpectedFinishReasonError(stopReason, summary, hasTextBlock)
	}
	if !hasTextBlock || strings.TrimSpace(text) == "" {
		return llm.GenerateResponse{}, fmt.Errorf("%w: stop_reason is %s and no non-whitespace text block is present",
			llm.ErrEmptyResponse, quoteReason(stopReason))
	}
	return llm.GenerateResponse{
		Text:         text,
		Model:        model,
		ModelVersion: "",
	}, nil
}

// parseContent validates the content array and returns its single text block's
// text and whether a text block was present. Each element is an object whose
// type decides how the rest is read: a text block also consumes text, while a
// thinking or redacted_thinking block consumes nothing else and is ignored. An
// unknown type is rejected rather than silently dropped. The empty text string
// is shape-valid and is classified by the caller's text check.
func parseContent(object strictjson.Object, members map[string]strictjson.Value) (string, bool, error) {
	contentValue, err := strictjson.Required(members, keyContent)
	if err != nil {
		return "", false, topLevelFailure(object, err)
	}
	blocks, err := contentValue.AsArray()
	if err != nil {
		return "", false, topLevelFailure(object, fmt.Errorf("%s: %w", keyContent, err))
	}
	var (
		text    string
		hasText bool
	)
	for _, block := range blocks {
		blockObject, err := block.AsObject()
		if err != nil {
			return "", false, topLevelFailure(object, fmt.Errorf("%s: %w", keyContent, err))
		}
		blockType, err := contentBlockType(object, blockObject)
		if err != nil {
			return "", false, err
		}
		switch blockType {
		case blockThinking, blockRedactedThinking:
			// Only the type is consumed; the block never contributes text.
		case blockText:
			if hasText {
				return "", false, topLevelFailure(object, errTextBlockCount)
			}
			blockText, err := textBlockText(object, blockObject)
			if err != nil {
				return "", false, err
			}
			text = blockText
			hasText = true
		default:
			return "", false, topLevelFailure(object, errUnknownBlockType)
		}
	}
	return text, hasText, nil
}

// contentBlockType returns the type member of one content element.
func contentBlockType(object, block strictjson.Object) (string, error) {
	members, err := block.Collect(keyType)
	if err != nil {
		return "", topLevelFailure(object, err)
	}
	value, err := strictjson.Required(members, keyType)
	if err != nil {
		return "", topLevelFailure(object, err)
	}
	blockType, err := value.AsString()
	if err != nil {
		return "", topLevelFailure(object, fmt.Errorf("%s: %w", keyType, err))
	}
	return blockType, nil
}

// textBlockText returns the text member of a text element.
func textBlockText(object, block strictjson.Object) (string, error) {
	members, err := block.Collect(keyText)
	if err != nil {
		return "", topLevelFailure(object, err)
	}
	value, err := strictjson.Required(members, keyText)
	if err != nil {
		return "", topLevelFailure(object, err)
	}
	text, err := value.AsString()
	if err != nil {
		return "", topLevelFailure(object, fmt.Errorf("%s: %w", keyText, err))
	}
	return text, nil
}

// requiredString returns the named member as a string, rejecting a missing
// member, null, or another kind, but not the empty string: an empty stop_reason
// is a shape-valid value classified by the stop-reason check.
func requiredString(members map[string]strictjson.Value, key string) (string, error) {
	value, err := strictjson.Required(members, key)
	if err != nil {
		return "", err
	}
	text, err := value.AsString()
	if err != nil {
		return "", fmt.Errorf("%s: %w", key, err)
	}
	return text, nil
}

// truncatedError formats the truncation failure around llm.ErrTruncated. When
// the limit was spent before any text block, the message adds the guidance to
// lower the effort.
func truncatedError(stopReason string, summary requestSummary, hasTextBlock bool) error {
	diagnostic := fmt.Sprintf("stop_reason is %s, max_tokens %d, effort %s, text block present %t",
		quoteReason(stopReason), summary.maxTokens, summary.effort, hasTextBlock)
	if !hasTextBlock {
		diagnostic += "; the output limit was spent before any text block, so lower the effort"
	}
	return fmt.Errorf("%w: %s", llm.ErrTruncated, diagnostic)
}

// unexpectedFinishReasonError formats the unexpected-finish-reason failure
// around llm.ErrUnexpectedFinishReason.
func unexpectedFinishReasonError(stopReason string, summary requestSummary, hasTextBlock bool) error {
	return fmt.Errorf("%w: stop_reason is %s, max_tokens %d, effort %s, text block present %t",
		llm.ErrUnexpectedFinishReason, quoteReason(stopReason), summary.maxTokens, summary.effort, hasTextBlock)
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

// quoteReason renders the untrusted stop_reason for an error message: quoted,
// with control characters escaped, and capped at maxReasonBytes bytes so it
// cannot flood the caller's terminal.
func quoteReason(reason string) string {
	if len(reason) > maxReasonBytes {
		reason = reason[:maxReasonBytes]
	}
	return strconv.Quote(reason)
}
