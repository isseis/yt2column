package deepseek

import (
	"bytes"
	"fmt"
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
	keyUsage             = "usage"
)

// Usage member names of the Chat Completions API that map onto the
// provider-independent categories. Only the members the adapter consumes are
// named; the cache-write category has no member and is always 0.
const (
	keyPromptCacheMissTokens = "prompt_cache_miss_tokens" //nolint:gosec // API usage member name, not a credential
	keyPromptCacheHitTokens  = "prompt_cache_hit_tokens"  //nolint:gosec // API usage member name, not a credential
	keyCompletionTokens      = "completion_tokens"        //nolint:gosec // API usage member name, not a credential
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

// parseResponse validates one response body and builds the GenerateResponse.
// The checks run in the requirement order: body shape, usage, finish reason,
// content. A body-shape rejection returns the zero GenerateResponse, whose
// usage is unknown. A truncation, an unexpected finish reason, or an empty
// response carries the rejected response's model and token usage with the
// error.
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
	// The usage is read after the body shape checks and in its own Collect, so
	// a broken usage cannot reject a response whose body is otherwise valid.
	usage := parseUsage(object)

	switch finishReason {
	case finishReasonLength:
		return llm.GenerateResponse{Model: model, Usage: usage},
			fmt.Errorf("%w: finish_reason is %s", llm.ErrTruncated, quoteReason(finishReason))
	case finishReasonStop:
	default:
		return llm.GenerateResponse{Model: model, Usage: usage},
			fmt.Errorf("%w: finish_reason is %s", llm.ErrUnexpectedFinishReason, quoteReason(finishReason))
	}
	if strings.TrimSpace(content) == "" {
		return llm.GenerateResponse{Model: model, Usage: usage},
			fmt.Errorf("%w: content is empty or whitespace only", llm.ErrEmptyResponse)
	}
	return llm.GenerateResponse{
		Text:         content,
		Model:        model,
		ModelVersion: modelVersion,
		Usage:        usage,
	}, nil
}

// parseUsage reads the response's top-level usage member into an llm.Usage.
// Unlike the response body, a missing, wrong-shaped, duplicated, or
// non-numeric usage never rejects the response: it makes the usage unknown,
// and the reason names a rule and a consumed member only, never a value. The
// unconsumed members are ignored, duplicates included.
func parseUsage(object strictjson.Object) llm.Usage {
	members, err := object.Collect(keyUsage)
	if err != nil {
		return llm.UnknownUsage(llm.RuleDuplicate, "")
	}
	value, present := members[keyUsage]
	if !present {
		return llm.UnknownUsage(llm.RuleMissing, "")
	}
	usage, err := value.AsObject()
	if err != nil {
		return llm.UnknownUsage(llm.RuleNotObject, "")
	}
	input, broken, ok := nonNegativeMember(usage, keyPromptCacheMissTokens)
	if !ok {
		return broken
	}
	cacheRead, broken, ok := nonNegativeMember(usage, keyPromptCacheHitTokens)
	if !ok {
		return broken
	}
	output, broken, ok := nonNegativeMember(usage, keyCompletionTokens)
	if !ok {
		return broken
	}
	// The Chat Completions API has no cache-write category, so it is 0.
	return llm.KnownUsage(llm.TokenCounts{
		Input:      input,
		CacheRead:  cacheRead,
		CacheWrite: 0,
		Output:     output,
	})
}

// nonNegativeMember reads one consumed usage member as a non-negative integer.
// ok is false when the member is missing, duplicated, or not a non-negative
// integer; broken is then the unknown usage naming the member and its rule.
func nonNegativeMember(object strictjson.Object, key string) (value uint64, broken llm.Usage, ok bool) {
	members, err := object.Collect(key)
	if err != nil {
		return 0, llm.UnknownUsage(llm.RuleDuplicate, key), false
	}
	raw, present := members[key]
	if !present {
		return 0, llm.UnknownUsage(llm.RuleMissing, key), false
	}
	number, err := raw.AsNonNegativeInt64()
	if err != nil {
		return 0, llm.UnknownUsage(llm.RuleNotNonNegativeInteger, key), false
	}
	// AsNonNegativeInt64 returns a value in [0, MaxInt64], so the conversion is
	// lossless.
	return uint64(number), llm.Usage{}, true //nolint:gosec // number is non-negative
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
