//go:build test

package deepseek

import (
	"testing"

	"github.com/isseis/yt2column/internal/llm"
)

// NewForLoopbackTest builds an llm.LLMClient that sends to endpoint, so a test
// in another package can point the adapter at a loopback server. The endpoint
// must be a loopback URL: every other endpoint fails the test without building
// a client. It performs the same construction validation as New, so a rejected
// Options value also fails the test. It is built only with the test tag.
func NewForLoopbackTest(t testing.TB, opts Options, endpoint string) llm.LLMClient {
	t.Helper()
	if err := validateLoopbackEndpoint(endpoint); err != nil {
		t.Fatalf("NewForLoopbackTest endpoint: %v", err)
	}
	value, err := New(opts)
	if err != nil {
		t.Fatalf("New(Options{Model: %q, Timeout: %v}) error = %v", opts.Model, opts.Timeout, err)
	}
	client, ok := value.(*client)
	if !ok {
		t.Fatalf("New returned %T, want *client", value)
	}
	client.endpoint = endpoint
	return client
}
