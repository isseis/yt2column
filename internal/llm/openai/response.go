package openai

import (
	"bytes"
	"errors"
	"fmt"
	"strings"

	"github.com/isseis/yt2column/internal/llm"
	"github.com/isseis/yt2column/internal/llm/openaiparam"
	"github.com/isseis/yt2column/internal/strictjson"
)

// Top-level member names of a Responses API response that the adapter
// consumes. Every other top-level member is ignored, duplicates and differing
// kinds included. The usage and service_tier members are read separately from
// the body checks, so neither can reject an otherwise valid body.
const (
	keyModel             = "model"
	keyStatus            = "status"
	keyOutput            = "output"
	keyIncompleteDetails = "incomplete_details"
	keyError             = "error"
	keyUsage             = "usage"
	keyServiceTier       = "service_tier"
)

// Member names inside the output array, its message content, and
// incomplete_details.
const (
	keyType    = "type"
	keyRole    = "role"
	keyContent = "content"
	keyPhase   = "phase"
	keyText    = "text"
	keyRefusal = "refusal"
	keyReason  = "reason"
)

// Usage member names. Only the members that map onto the provider-independent
// categories are consumed; total_tokens and output_tokens_details are not
// added to any category.
const (
	keyInputTokens        = "input_tokens"
	keyInputTokensDetails = "input_tokens_details" //nolint:gosec // a JSON member name, not a credential
	keyCachedTokens       = "cached_tokens"        //nolint:gosec // a JSON member name, not a credential
	keyCacheWriteTokens   = "cache_write_tokens"   //nolint:gosec // a JSON member name, not a credential
	keyOutputTokens       = "output_tokens"
)

// Usage member paths an unknown usage names, relative to the usage object.
const (
	memberCachedTokens     = keyInputTokensDetails + "." + keyCachedTokens
	memberCacheWriteTokens = keyInputTokensDetails + "." + keyCacheWriteTokens
)

// Output element types. A message may carry the answer; a reasoning element is
// consumed only for its type and never contributes text.
const (
	outputMessage   = "message"
	outputReasoning = "reasoning"
)

// Message content element types.
const (
	contentOutputText = "output_text"
	contentRefusal    = "refusal"
)

// roleAssistant is the only message role a response may carry.
const roleAssistant = "assistant"

// Message phase values. A message without a phase (missing or null) is part
// of the final answer, as is a final_answer message; a commentary message is
// progress and never contributes text.
const (
	phaseCommentary  = "commentary"
	phaseFinalAnswer = "final_answer"
)

// Response and message status values that carry a defined meaning for the
// adapter, and the other documented response statuses, which an error message
// may name. Any other status is named "unknown", because it is untrusted.
const (
	statusCompleted  = "completed"
	statusIncomplete = "incomplete"
	statusFailed     = "failed"
	statusCancelled  = "cancelled"
	statusInProgress = "in_progress"
	statusQueued     = "queued"
	labelUnknown     = "unknown"
)

// Documented incomplete_details reasons. Only max_output_tokens is a
// truncation; any reason outside this list is named "unknown".
const (
	reasonMaxOutputTokens = "max_output_tokens"
	reasonContentFilter   = "content_filter"
)

// jsonWhitespace is the set of insignificant whitespace bytes of RFC 8259.
const jsonWhitespace = " \t\r\n"

// Shape errors of the response body. None of them carries a response value:
// every value of the body is untrusted.
var (
	errUnknownOutputType  = errors.New("output: unknown element type")
	errUnknownContentType = errors.New("content: unknown element type")
	errUnknownPhase       = errors.New("phase: not one of the accepted values")
	errRoleNotAssistant   = errors.New("role: not assistant")
	errIncompleteDetails  = errors.New("incomplete_details: must be an object exactly when status is incomplete")
	errErrorMember        = errors.New("error: not null or an object")
	errWhitespaceBody     = errors.New("response body is empty or whitespace only")
)

// requestSummary carries the request-side values that a truncation or
// unexpected-finish message names: the max_output_tokens that was sent and
// the effort. Neither is a secret.
type requestSummary struct {
	maxTokens int
	effort    openaiparam.Effort
}

// responseBody is a response body that passed every shape check. It holds the
// values the finish classification needs, and no free text other than the
// final answer.
type responseBody struct {
	model            string
	status           string
	incompleteReason string // set only when status is incomplete
	hasError         bool   // the error member is an object
	text             string // the final answer, in output order, without separators
	unfinishedMsg    bool   // a message whose status is not completed
	hasRefusal       bool   // a refusal content element
}

// parseResponse validates one response body and builds the GenerateResponse.
// The body shape is checked first; a shape rejection returns the zero
// GenerateResponse, whose usage is unknown. Then the response is classified
// in this order: a truncation, an unexpected finish, an empty answer. Each of
// those three failures carries the rejected response's model and usage.
func parseResponse(data []byte, summary requestSummary) (llm.GenerateResponse, error) {
	// Only JSON whitespace counts: bytes.TrimSpace would also strip Unicode
	// spaces such as U+00A0, which are not JSON whitespace.
	if len(bytes.Trim(data, jsonWhitespace)) == 0 {
		return llm.GenerateResponse{}, invalidResponse(errWhitespaceBody)
	}
	object, err := strictjson.ParseObject(data)
	if err != nil {
		return llm.GenerateResponse{}, invalidResponse(err)
	}
	body, err := parseBody(object)
	if err != nil {
		return llm.GenerateResponse{}, invalidResponse(err)
	}
	// The usage is read after the body shape checks and in its own Collect, so
	// a broken usage cannot reject a response whose body is otherwise valid.
	rejected := llm.GenerateResponse{Model: body.model, Usage: parseUsage(object)}

	if body.status == statusIncomplete && body.incompleteReason == reasonMaxOutputTokens {
		return rejected, truncatedError(body, summary)
	}
	if cause := unexpectedCause(body); cause != "" {
		return rejected, fmt.Errorf("%w: %s, max_output_tokens %d, effort %s, final answer text present %t",
			llm.ErrUnexpectedFinishReason, cause, summary.maxTokens, summary.effort, hasAnswerText(body))
	}
	if !hasAnswerText(body) {
		return rejected, fmt.Errorf("%w: status is completed and no non-whitespace final answer text is present",
			llm.ErrEmptyResponse)
	}
	return llm.GenerateResponse{
		Text:         body.text,
		Model:        body.model,
		ModelVersion: "",
		Usage:        rejected.Usage,
	}, nil
}

// hasAnswerText reports whether the final answer has non-whitespace text.
// Reasoning and commentary never count as text.
func hasAnswerText(body responseBody) bool {
	return strings.TrimSpace(body.text) != ""
}

// unexpectedCause returns a fixed description of why a response that is not a
// truncation did not finish normally, or "" when it did. The description names
// a documented status or reason, or "unknown", and never quotes a response
// value.
func unexpectedCause(body responseBody) string {
	switch {
	case body.status == statusIncomplete:
		return "status is incomplete with reason " + reasonLabel(body.incompleteReason)
	case body.status != statusCompleted:
		return "status is " + statusLabel(body.status)
	case body.hasError:
		return "the response has an error object"
	case body.unfinishedMsg:
		return "a message is not completed"
	case body.hasRefusal:
		return "the response has a refusal"
	default:
		return ""
	}
}

// statusLabel names a response status for an error message: a documented
// status as it is, and any other value as "unknown".
func statusLabel(status string) string {
	switch status {
	case statusCompleted, statusIncomplete, statusFailed, statusCancelled, statusInProgress, statusQueued:
		return status
	default:
		return labelUnknown
	}
}

// reasonLabel names an incomplete_details reason for an error message: a
// documented reason as it is, and any other value as "unknown".
func reasonLabel(reason string) string {
	switch reason {
	case reasonMaxOutputTokens, reasonContentFilter:
		return reason
	default:
		return labelUnknown
	}
}

// truncatedError formats the truncation failure around llm.ErrTruncated. When
// the limit was spent before any final answer text, the message adds the
// guidance to lower the effort.
func truncatedError(body responseBody, summary requestSummary) error {
	hasText := hasAnswerText(body)
	diagnostic := fmt.Sprintf("status is incomplete with reason %s, max_output_tokens %d, effort %s, final answer text present %t",
		reasonMaxOutputTokens, summary.maxTokens, summary.effort, hasText)
	if !hasText {
		diagnostic += "; the output limit was spent before any final answer text, so lower the effort"
	}
	return fmt.Errorf("%w: %s", llm.ErrTruncated, diagnostic)
}

// invalidResponse wraps a shape failure around the ErrInvalidResponse
// sentinel. The cause is a fixed shape error or a strictjson error, which
// names a consumed key at most, never a value.
func invalidResponse(cause error) error {
	return fmt.Errorf("%w: %v", ErrInvalidResponse, cause)
}

// parseBody checks the shape of every consumed member of the top level and
// of the output array, and collects the values the finish classification
// needs. Every element is checked before anything is returned, so an invalid
// element after valid ones still rejects the whole body.
func parseBody(object strictjson.Object) (responseBody, error) {
	members, err := object.Collect(keyModel, keyStatus, keyOutput, keyIncompleteDetails, keyError)
	if err != nil {
		return responseBody{}, err
	}
	model, err := strictjson.RequiredString(members, keyModel)
	if err != nil {
		return responseBody{}, err
	}
	status, err := requiredString(members, keyStatus)
	if err != nil {
		return responseBody{}, err
	}
	incompleteReason, err := parseIncompleteDetails(members, status)
	if err != nil {
		return responseBody{}, err
	}
	hasError, err := parseErrorMember(members)
	if err != nil {
		return responseBody{}, err
	}
	body := responseBody{
		model:            model,
		status:           status,
		incompleteReason: incompleteReason,
		hasError:         hasError,
	}
	if err := parseOutput(members, &body); err != nil {
		return responseBody{}, err
	}
	return body, nil
}

// parseIncompleteDetails checks incomplete_details against the status: an
// object with a string reason when the status is incomplete, and missing or
// null otherwise. It returns the reason for an incomplete status.
func parseIncompleteDetails(members map[string]strictjson.Value, status string) (string, error) {
	value, present := members[keyIncompleteDetails]
	absent := !present || value.IsNull()
	if status != statusIncomplete {
		if !absent {
			return "", errIncompleteDetails
		}
		return "", nil
	}
	if absent {
		return "", errIncompleteDetails
	}
	details, err := value.AsObject()
	if err != nil {
		return "", fmt.Errorf("%s: %w", keyIncompleteDetails, err)
	}
	detailMembers, err := details.Collect(keyReason)
	if err != nil {
		return "", fmt.Errorf("%s: %w", keyIncompleteDetails, err)
	}
	reason, err := requiredString(detailMembers, keyReason)
	if err != nil {
		return "", fmt.Errorf("%s: %w", keyIncompleteDetails, err)
	}
	return reason, nil
}

// parseErrorMember checks the error member: missing, null, or an object.
// Its content is never read. It reports whether the member is an object.
func parseErrorMember(members map[string]strictjson.Value) (bool, error) {
	value, present := members[keyError]
	if !present || value.IsNull() {
		return false, nil
	}
	if _, err := value.AsObject(); err != nil {
		return false, errErrorMember
	}
	return true, nil
}

// parseOutput checks every element of the output array and records the final
// answer text, unfinished messages, and refusals in body.
func parseOutput(members map[string]strictjson.Value, body *responseBody) error {
	value, err := strictjson.Required(members, keyOutput)
	if err != nil {
		return err
	}
	elements, err := value.AsArray()
	if err != nil {
		return fmt.Errorf("%s: %w", keyOutput, err)
	}
	var text strings.Builder
	for _, element := range elements {
		item, err := element.AsObject()
		if err != nil {
			return fmt.Errorf("%s: %w", keyOutput, err)
		}
		itemType, err := elementType(item)
		if err != nil {
			return fmt.Errorf("%s: %w", keyOutput, err)
		}
		switch itemType {
		case outputReasoning:
			// Only the type is consumed; reasoning never contributes text.
		case outputMessage:
			if err := parseMessage(item, body, &text); err != nil {
				return err
			}
		default:
			return errUnknownOutputType
		}
	}
	body.text = text.String()
	return nil
}

// parseMessage checks one message element. Its output_text joins the final
// answer only when the message has no phase or the final_answer phase; a
// commentary message is checked the same way but contributes no text.
func parseMessage(item strictjson.Object, body *responseBody, text *strings.Builder) error {
	members, err := item.Collect(keyType, keyRole, keyStatus, keyContent, keyPhase)
	if err != nil {
		return fmt.Errorf("%s: %w", outputMessage, err)
	}
	role, err := requiredString(members, keyRole)
	if err != nil {
		return fmt.Errorf("%s: %w", outputMessage, err)
	}
	if role != roleAssistant {
		return errRoleNotAssistant
	}
	status, err := requiredString(members, keyStatus)
	if err != nil {
		return fmt.Errorf("%s: %w", outputMessage, err)
	}
	final, err := isFinalAnswer(members)
	if err != nil {
		return err
	}
	value, err := strictjson.Required(members, keyContent)
	if err != nil {
		return fmt.Errorf("%s: %w", outputMessage, err)
	}
	parts, err := value.AsArray()
	if err != nil {
		return fmt.Errorf("%s: %w", keyContent, err)
	}
	for _, part := range parts {
		partObject, err := part.AsObject()
		if err != nil {
			return fmt.Errorf("%s: %w", keyContent, err)
		}
		partType, err := elementType(partObject)
		if err != nil {
			return fmt.Errorf("%s: %w", keyContent, err)
		}
		switch partType {
		case contentOutputText:
			partText, err := partString(partObject, keyText)
			if err != nil {
				return err
			}
			if final {
				text.WriteString(partText)
			}
		case contentRefusal:
			// The refusal text is checked for its kind and then discarded:
			// it is never part of the result or of an error.
			if _, err := partString(partObject, keyRefusal); err != nil {
				return err
			}
			body.hasRefusal = true
		default:
			return errUnknownContentType
		}
	}
	if status != statusCompleted {
		body.unfinishedMsg = true
	}
	return nil
}

// isFinalAnswer reads a message's optional phase: missing, null, and
// final_answer are the final answer, commentary is not, and any other value
// is rejected.
func isFinalAnswer(members map[string]strictjson.Value) (bool, error) {
	value, present := members[keyPhase]
	if !present || value.IsNull() {
		return true, nil
	}
	phase, err := value.AsString()
	if err != nil {
		return false, fmt.Errorf("%s: %w", keyPhase, err)
	}
	switch phase {
	case phaseFinalAnswer:
		return true, nil
	case phaseCommentary:
		return false, nil
	default:
		return false, errUnknownPhase
	}
}

// elementType returns the required string type member of an output or
// content element.
func elementType(element strictjson.Object) (string, error) {
	members, err := element.Collect(keyType)
	if err != nil {
		return "", err
	}
	return requiredString(members, keyType)
}

// partString returns the required string member key of a content element.
// The empty string is shape-valid.
func partString(part strictjson.Object, key string) (string, error) {
	members, err := part.Collect(keyType, key)
	if err != nil {
		return "", fmt.Errorf("%s: %w", keyContent, err)
	}
	value, err := requiredString(members, key)
	if err != nil {
		return "", fmt.Errorf("%s: %w", keyContent, err)
	}
	return value, nil
}

// requiredString returns the named member as a string, rejecting a missing
// member, null, or another kind, but not the empty string, which the caller
// classifies.
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

// parseUsage reads the response's top-level usage member into an llm.Usage,
// with the pricing context the response confirms. Unlike the response body, a
// missing, wrong-shaped, duplicated, or inconsistent usage never rejects the
// response: it makes the usage unknown, and the reason names a rule and a
// consumed member only, never a value. Unconsumed members are ignored,
// duplicates included.
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
	input, broken, ok := readCount(usage, keyInputTokens, keyInputTokens)
	if !ok {
		return broken
	}
	details, broken, ok := readDetails(usage)
	if !ok {
		return broken
	}
	cacheRead, broken, ok := readCount(details, keyCachedTokens, memberCachedTokens)
	if !ok {
		return broken
	}
	cacheWrite, broken, ok := readCount(details, keyCacheWriteTokens, memberCacheWriteTokens)
	if !ok {
		return broken
	}
	output, broken, ok := readCount(usage, keyOutputTokens, keyOutputTokens)
	if !ok {
		return broken
	}
	// Each count is at most MaxInt64, so the sum cannot overflow uint64.
	if cacheRead+cacheWrite > input {
		return llm.UnknownUsage(llm.RuleInconsistent, keyInputTokensDetails)
	}
	counts := llm.TokenCounts{
		Input:      input - cacheRead - cacheWrite,
		CacheRead:  cacheRead,
		CacheWrite: cacheWrite,
		Output:     output,
	}
	known, err := llm.KnownUsageWithPricingContext(counts, pricingContext(object))
	if err != nil {
		// pricingContext returns only enumerated values, so this cannot
		// happen; a usage that cannot be built is reported as not read
		// rather than as zero tokens.
		return llm.UnknownUsage(llm.RuleNotRead, "")
	}
	return known
}

// pricingContext reports the condition the usage was produced under. The
// request always names the Standard tier and the 30-minute cache retention, so
// the condition is confirmed when the response has exactly one service_tier
// member whose value is the string "default". A missing, duplicated, null,
// non-string, or other value leaves the condition unverified; it never
// rejects the response, and the value is never kept.
func pricingContext(object strictjson.Object) llm.PricingContext {
	members, err := object.Collect(keyServiceTier)
	if err != nil {
		return llm.PricingContextUnverified
	}
	value, present := members[keyServiceTier]
	if !present {
		return llm.PricingContextUnverified
	}
	tier, err := value.AsString()
	if err != nil || tier != serviceTierDefault {
		return llm.PricingContextUnverified
	}
	return llm.PricingContextStandard30m
}

// readDetails reads the required input_tokens_details object of the usage.
// ok is false when it is missing, duplicated, or not an object; broken is then
// the unknown usage naming it.
func readDetails(usage strictjson.Object) (details strictjson.Object, broken llm.Usage, ok bool) {
	members, err := usage.Collect(keyInputTokensDetails)
	if err != nil {
		return strictjson.Object{}, llm.UnknownUsage(llm.RuleDuplicate, keyInputTokensDetails), false
	}
	value, present := members[keyInputTokensDetails]
	if !present {
		return strictjson.Object{}, llm.UnknownUsage(llm.RuleMissing, keyInputTokensDetails), false
	}
	details, err = value.AsObject()
	if err != nil {
		return strictjson.Object{}, llm.UnknownUsage(llm.RuleNotObject, keyInputTokensDetails), false
	}
	return details, llm.Usage{}, true
}

// readCount reads one consumed count of object as a non-negative integer. null
// is rejected like any other non-integer. ok is false when the count is
// missing, duplicated, or not a non-negative integer that fits int64; broken
// is then the unknown usage naming member and its rule.
func readCount(object strictjson.Object, key, member string) (value uint64, broken llm.Usage, ok bool) {
	members, err := object.Collect(key)
	if err != nil {
		return 0, llm.UnknownUsage(llm.RuleDuplicate, member), false
	}
	raw, present := members[key]
	if !present {
		return 0, llm.UnknownUsage(llm.RuleMissing, member), false
	}
	number, err := raw.AsNonNegativeInt64()
	if err != nil {
		return 0, llm.UnknownUsage(llm.RuleNotNonNegativeInteger, member), false
	}
	// AsNonNegativeInt64 returns a value in [0, MaxInt64], so the conversion is
	// lossless.
	return uint64(number), llm.Usage{}, true //nolint:gosec // number is non-negative
}
