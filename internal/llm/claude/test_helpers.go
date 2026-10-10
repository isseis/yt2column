//go:build test

package claude

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/isseis/yt2column/internal/llm"
	"github.com/isseis/yt2column/internal/llm/claudeparam"
	"github.com/isseis/yt2column/internal/secret"
)

const (
	// testAPIKey is a fixed, non-secret value. No test uses a real key.
	testAPIKey = "test-api-key-0123456789abcdef"
	// testModel is the model name used to build test clients.
	testModel = "claude-opus-5-5"
	// testWorkspaceID is a fixed, non-secret workspace id.
	testWorkspaceID = "wrkspc_test0123456789"
	// testClientTimeout is the adapter timeout of a test client.
	testClientTimeout = 2 * time.Second
	// testTimeoutGrace is the margin allowed for Generate to return after
	// the adapter timeout has elapsed.
	testTimeoutGrace = 2 * time.Second
	// Paths to the committed fixtures, relative to this package directory.
	testdataEndTurnFixture = "../../../testdata/claude_messages_end_turn.json"
	//nolint:gosec // the value is a testdata path, not a credential
	testdataMaxTokensFixture = "../../../testdata/claude_messages_max_tokens.json"
)

// mustSecret wraps value in a secret.Secret or fails the test.
func mustSecret(t *testing.T, value string) secret.Secret {
	t.Helper()
	apiKey, err := secret.New(value)
	if err != nil {
		t.Fatalf("secret.New(%q) error = %v", value, err)
	}
	return apiKey
}

// mustWorkspaceID parses value into a workspace ID or fails the test.
func mustWorkspaceID(t *testing.T, value string) claudeparam.WorkspaceID {
	t.Helper()
	workspaceID, err := claudeparam.ParseWorkspaceID(value)
	if err != nil {
		t.Fatalf("ParseWorkspaceID(%q) error = %v", value, err)
	}
	return workspaceID
}

// newTestClient builds a client for the given loopback endpoint by delegating
// to NewForLoopbackTest with the test defaults. The address must be a loopback
// URL; every other address fails the test without building a client.
func newTestClient(t *testing.T, endpoint string, modify func(*Options)) *client {
	t.Helper()
	options := Options{
		APIKey:  mustSecret(t, testAPIKey),
		Model:   testModel,
		Effort:  claudeparam.EffortHigh,
		Timeout: testClientTimeout,
	}
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

// readFixture returns the bytes of a committed testdata fixture.
func readFixture(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path) //nolint:gosec // the path names a committed testdata fixture
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return data
}

// generateAgainst builds a test client for serverURL and runs Generate with a
// background context.
func generateAgainst(t *testing.T, serverURL string, req llm.GenerateRequest, modify func(*Options)) (llm.GenerateResponse, error) {
	t.Helper()
	return newTestClient(t, serverURL, modify).Generate(context.Background(), req)
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

// documentOf decodes a response body into a generic document for targeted
// mutation.
func documentOf(t *testing.T, body []byte) map[string]any {
	t.Helper()
	var document map[string]any
	if err := json.Unmarshal(body, &document); err != nil {
		t.Fatalf("decode fixture: %v", err)
	}
	return document
}

// encodeDocument marshals a mutated document back to JSON bytes.
func encodeDocument(t *testing.T, document map[string]any) []byte {
	t.Helper()
	data, err := json.Marshal(document)
	if err != nil {
		t.Fatalf("encode document: %v", err)
	}
	return data
}

// contentOf returns the content array of a decoded document.
func contentOf(t *testing.T, document map[string]any) []any {
	t.Helper()
	content, ok := document[keyContent].([]any)
	if !ok {
		t.Fatalf("content = %#v, want an array", document[keyContent])
	}
	return content
}

// blockOfType returns the first content element of the given type.
func blockOfType(t *testing.T, document map[string]any, blockType string) map[string]any {
	t.Helper()
	for _, element := range contentOf(t, document) {
		block, ok := element.(map[string]any)
		if !ok {
			continue
		}
		if block[keyType] == blockType {
			return block
		}
	}
	t.Fatalf("no %s block found in content", blockType)
	return nil
}

// textBlockOf returns the first text block of a decoded document.
func textBlockOf(t *testing.T, document map[string]any) map[string]any {
	t.Helper()
	return blockOfType(t, document, blockText)
}

// assertSingleSentinel checks that err matches exactly one of the seven
// adapter sentinels and the two context errors.
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

// assertErrorPrefix checks the "claude: " prefix appears once.
func assertErrorPrefix(t *testing.T, err error) {
	t.Helper()
	message := err.Error()
	if !strings.HasPrefix(message, errorPrefix) {
		t.Errorf("error message %q does not start with %q", message, errorPrefix)
	}
	if strings.Count(message, errorPrefix) != 1 {
		t.Errorf("error message %q contains %q %d times, want 1", message, errorPrefix, strings.Count(message, errorPrefix))
	}
}

// assertNoAPIKey checks the test API key appears in none of the error's
// rendered forms.
func assertNoAPIKey(t *testing.T, err error) {
	t.Helper()
	rendered := []string{err.Error()}
	for _, format := range []string{"%v", "%+v", "%#v"} {
		rendered = append(rendered, fmt.Sprintf(format, err))
	}
	for _, output := range rendered {
		if strings.Contains(output, testAPIKey) {
			t.Errorf("rendered error contains the API key: %q", output)
		}
	}
}

// assertRejected checks the common contract of a failure case: exactly one
// sentinel, the single error prefix, no API key, and the zero response.
func assertRejected(t *testing.T, response llm.GenerateResponse, err error) {
	t.Helper()
	assertSingleSentinel(t, err)
	assertErrorPrefix(t, err)
	assertNoAPIKey(t, err)
	if response != (llm.GenerateResponse{}) {
		t.Errorf("Generate() response = %+v, want the zero value", response)
	}
}
