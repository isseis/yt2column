//go:build test

package openai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/isseis/yt2column/internal/llm"
	"github.com/isseis/yt2column/internal/llm/llmhttp/llmhttptest"
	"github.com/isseis/yt2column/internal/llm/openaiparam"
	"github.com/isseis/yt2column/internal/secret"
)

const (
	// testAPIKey is a fixed, non-secret value. No test uses a real key.
	testAPIKey = "sk-test-api-key-0123456789abcdef" //nolint:gosec // a fixed fake key, not a credential
	// testModel is the model name used to build test clients.
	testModel = "gpt-6-astra"
	// testResponseModel is the model name the test responses report. It
	// differs from testModel, so a test can tell which one a result carries.
	testResponseModel = "gpt-6-astra-2026-11-01"
	// testAnswer is the final answer text of the base test response.
	testAnswer = "The sky scatters blue light."
	// testClientTimeout is the adapter timeout of a test client.
	testClientTimeout = 2 * time.Second
	// testTimeoutGrace is the margin allowed for Generate to return after
	// the adapter timeout has elapsed.
	testTimeoutGrace = 2 * time.Second
	// fakeSecretMarker stands in for a secret or free text that a response
	// carries in an untrusted value. No result or error may contain it.
	fakeSecretMarker = "sk-FAKE-SECRET-MARKER-7731" //nolint:gosec // a fake marker, not a credential
	// Paths to the saved real responses, relative to this package directory.
	capturedCompletedFixture  = "../../../testdata/openai_responses_completed.json"
	capturedIncompleteFixture = "../../../testdata/openai_responses_incomplete.json"
)

// The usage of the base test response and the counts it maps to: 120 input
// tokens of which 30 were read from and 50 written to the cache, and 40
// output tokens of which 25 were reasoning.
var testCounts = llm.TokenCounts{Input: 40, CacheRead: 30, CacheWrite: 50, Output: 40} //nolint:mnd // test response values

// mustSecret wraps value in a secret.Secret or fails the test.
func mustSecret(t *testing.T, value string) secret.Secret {
	t.Helper()
	apiKey, err := secret.New(value)
	if err != nil {
		t.Fatalf("secret.New(%q) error = %v", value, err)
	}
	return apiKey
}

// validOptions returns Options that New accepts.
func validOptions(t *testing.T) Options {
	t.Helper()
	return Options{
		APIKey:  mustSecret(t, testAPIKey),
		Model:   testModel,
		Effort:  openaiparam.EffortMedium,
		Timeout: testClientTimeout,
	}
}

// newTestClient builds a client for the given loopback endpoint by delegating
// to NewForLoopbackTest with the test defaults.
func newTestClient(t *testing.T, endpoint string, modify func(*Options)) *client {
	t.Helper()
	options := validOptions(t)
	if modify != nil {
		modify(&options)
	}
	value := NewForLoopbackTest(t, options, endpoint)
	client, ok := value.(*client)
	if !ok {
		t.Fatalf("NewForLoopbackTest returned %T, want *client", value)
	}
	return client
}

// validRequest returns a request that passes Validate.
func validRequest() llm.GenerateRequest {
	return llm.GenerateRequest{
		SystemPrompt: "Answer concisely.",
		UserPrompt:   "Why is the sky blue?",
	}
}

// generateAgainst builds a test client for serverURL and runs Generate with a
// background context.
func generateAgainst(t *testing.T, serverURL string, req llm.GenerateRequest, modify func(*Options)) (llm.GenerateResponse, error) {
	t.Helper()
	return newTestClient(t, serverURL, modify).Generate(context.Background(), req)
}

// reasoningItem returns a reasoning output element with members the adapter
// does not consume.
func reasoningItem() map[string]any {
	return map[string]any{keyType: outputReasoning, "id": "rs_test", "summary": []any{}}
}

// textPart returns an output_text content element.
func textPart(text string) map[string]any {
	return map[string]any{keyType: contentOutputText, keyText: text, "annotations": []any{}}
}

// messageItem returns a completed assistant message with the given content
// parts. A phase of "" leaves the member out.
func messageItem(phase string, parts ...any) map[string]any {
	if parts == nil {
		parts = []any{}
	}
	message := map[string]any{
		keyType:    outputMessage,
		"id":       "msg_test",
		keyRole:    roleAssistant,
		keyStatus:  statusCompleted,
		keyContent: parts,
	}
	if phase != "" {
		message[keyPhase] = phase
	}
	return message
}

// usageDocument returns the usage object of the base test response.
//
//nolint:mnd // test response values
func usageDocument() map[string]any {
	return map[string]any{
		keyInputTokens: 120,
		keyInputTokensDetails: map[string]any{
			keyCachedTokens:     30,
			keyCacheWriteTokens: 50,
		},
		keyOutputTokens:         40,
		"output_tokens_details": map[string]any{"reasoning_tokens": 25},
		"total_tokens":          160,
	}
}

// completedDocument returns a completed response: a reasoning element and a
// final-answer message with testAnswer, the Standard tier, and the usage of
// usageDocument.
func completedDocument() map[string]any {
	return map[string]any{
		"id":                 "resp_test",
		"object":             "response",
		keyModel:             testResponseModel,
		keyStatus:            statusCompleted,
		keyIncompleteDetails: nil,
		keyError:             nil,
		keyServiceTier:       serviceTierDefault,
		keyOutput: []any{
			reasoningItem(),
			messageItem(phaseFinalAnswer, textPart(testAnswer)),
		},
		keyUsage: usageDocument(),
	}
}

// truncatedDocument returns a response that ended at the output limit before
// any message: only a reasoning element.
func truncatedDocument() map[string]any {
	document := completedDocument()
	document[keyStatus] = statusIncomplete
	document[keyIncompleteDetails] = map[string]any{keyReason: reasonMaxOutputTokens}
	document[keyOutput] = []any{reasoningItem()}
	return document
}

// encodeDocument marshals a document to JSON bytes. The keys of a map are
// sorted, so a test can find a member's encoding with replaceOnce.
func encodeDocument(t *testing.T, document map[string]any) []byte {
	t.Helper()
	data, err := json.Marshal(document)
	if err != nil {
		t.Fatalf("encode document: %v", err)
	}
	return data
}

// replaceOnce replaces the single occurrence of old in body, failing the test
// when the needle is absent or ambiguous.
func replaceOnce(t *testing.T, body []byte, old, replacement string) []byte {
	t.Helper()
	text := string(body)
	if count := strings.Count(text, old); count != 1 {
		t.Fatalf("needle %q appears %d times, want exactly 1", old, count)
	}
	return []byte(strings.Replace(text, old, replacement, 1))
}

// outputOf returns the output array of a document.
func outputOf(t *testing.T, document map[string]any) []any {
	t.Helper()
	output, ok := document[keyOutput].([]any)
	if !ok {
		t.Fatalf("output = %#v, want an array", document[keyOutput])
	}
	return output
}

// messageOf returns the first message element of a document.
func messageOf(t *testing.T, document map[string]any) map[string]any {
	t.Helper()
	for _, element := range outputOf(t, document) {
		if item, ok := element.(map[string]any); ok && item[keyType] == outputMessage {
			return item
		}
	}
	t.Fatal("no message element in output")
	return nil
}

// textPartOf returns the first content element of the first message.
func textPartOf(t *testing.T, document map[string]any) map[string]any {
	t.Helper()
	parts, ok := messageOf(t, document)[keyContent].([]any)
	if !ok || len(parts) == 0 {
		t.Fatal("the first message has no content")
	}
	part, ok := parts[0].(map[string]any)
	if !ok {
		t.Fatalf("content[0] = %#v, want an object", parts[0])
	}
	return part
}

// usageOf returns the usage object of a document.
func usageOf(t *testing.T, document map[string]any) map[string]any {
	t.Helper()
	usage, ok := document[keyUsage].(map[string]any)
	if !ok {
		t.Fatalf("usage = %#v, want an object", document[keyUsage])
	}
	return usage
}

// detailsOf returns the input_tokens_details object of a document's usage.
func detailsOf(t *testing.T, document map[string]any) map[string]any {
	t.Helper()
	details, ok := usageOf(t, document)[keyInputTokensDetails].(map[string]any)
	if !ok {
		t.Fatal("usage has no input_tokens_details object")
	}
	return details
}

// generateBody runs Generate against a server that answers with a 200 and
// body.
func generateBody(t *testing.T, body []byte) (llm.GenerateResponse, error) {
	t.Helper()
	server := llmhttptest.NewResponseServer(t, http.StatusOK, body)
	return generateAgainst(t, server.URL, validRequest(), nil)
}

// assertAccepted runs Generate on body and requires success.
func assertAccepted(t *testing.T, body []byte) llm.GenerateResponse {
	t.Helper()
	response, err := generateBody(t, body)
	if err != nil {
		t.Fatalf("Generate() error = %v, want nil", err)
	}
	return response
}

// assertSingleSentinel checks that err matches exactly one of the adapter
// sentinels and the two context errors.
func assertSingleSentinel(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("error = nil, want a sentinel error")
	}
	sentinels := []error{
		llm.ErrInvalidRequest,
		llm.ErrTruncated,
		llm.ErrUnexpectedFinishReason,
		llm.ErrEmptyResponse,
		ErrHTTPStatus,
		ErrInvalidResponse,
		ErrTransport,
		context.DeadlineExceeded,
		context.Canceled,
	}
	matches := 0
	for _, sentinel := range sentinels {
		if errors.Is(err, sentinel) {
			matches++
		}
	}
	if matches != 1 {
		t.Errorf("error %v matches %d sentinels, want exactly 1", err, matches)
	}
}

// assertNoLeak checks that neither the test API key nor the fake secret
// marker appears in any rendered form of err.
func assertNoLeak(t *testing.T, err error) {
	t.Helper()
	for _, format := range []string{"%v", "%+v", "%#v"} {
		output := fmt.Sprintf(format, err)
		for _, forbidden := range []string{testAPIKey, fakeSecretMarker} {
			if strings.Contains(output, forbidden) {
				t.Errorf("rendered error contains %q: %q", forbidden, output)
			}
		}
	}
}

// assertFailure checks the common contract of a failure: the wanted
// sentinel and no other, the single error prefix, no leaked value, and the
// wanted response.
func assertFailure(t *testing.T, response llm.GenerateResponse, err, want error, wantResponse llm.GenerateResponse) {
	t.Helper()
	if !errors.Is(err, want) {
		t.Errorf("Generate() error = %v, want %v", err, want)
	}
	assertSingleSentinel(t, err)
	if message := err.Error(); !strings.HasPrefix(message, errorPrefix) || strings.Count(message, errorPrefix) != 1 {
		t.Errorf("error message %q does not carry the prefix %q exactly once", message, errorPrefix)
	}
	assertNoLeak(t, err)
	if response != wantResponse {
		t.Errorf("Generate() response = %+v, want %+v", response, wantResponse)
	}
}

// assertRejected checks a failure that returns the zero response.
func assertRejected(t *testing.T, response llm.GenerateResponse, err, want error) {
	t.Helper()
	assertFailure(t, response, err, want, llm.GenerateResponse{})
}

// assertBodyRejected runs Generate on body and requires a failure with the
// zero response, returning the error.
func assertBodyRejected(t *testing.T, body []byte, want error) error {
	t.Helper()
	response, err := generateBody(t, body)
	assertRejected(t, response, err, want)
	return err
}

// standardUsage returns the known usage of counts under the confirmed
// Standard tier.
func standardUsage(t *testing.T, counts llm.TokenCounts) llm.Usage {
	t.Helper()
	usage, err := llm.KnownUsageWithPricingContext(counts, llm.PricingContextStandard30m)
	if err != nil {
		t.Fatalf("KnownUsageWithPricingContext error = %v", err)
	}
	return usage
}

// assertBodyRejectedWithResponse runs Generate on body and requires a failure
// that carries the response model and wantUsage, returning the error.
func assertBodyRejectedWithResponse(t *testing.T, body []byte, want error, wantUsage llm.Usage) error {
	t.Helper()
	response, err := generateBody(t, body)
	assertFailure(t, response, err, want, llm.GenerateResponse{Model: testResponseModel, Usage: wantUsage})
	return err
}
