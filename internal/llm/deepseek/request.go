package deepseek

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/isseis/yt2column/internal/llm"
	"github.com/isseis/yt2column/internal/secret"
)

// Message roles of the Chat Completions API.
const (
	roleSystem = "system"
	roleUser   = "user"
)

// chatRequest is the request body sent to the Chat Completions API. It has no
// fields for parameters outside the requirements (thinking, temperature, n,
// ...), so none of them can be sent.
type chatRequest struct {
	Model     string        `json:"model"`
	Messages  []chatMessage `json:"messages"`
	Stream    bool          `json:"stream"`
	MaxTokens *int          `json:"max_tokens,omitempty"`
}

// chatMessage is one message of the request. Prompts are carried unmodified.
type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// newChatRequestBody builds the JSON request body. It never assembles the
// prompts by string concatenation and never includes the API key.
// max_tokens is sent only for a positive MaxOutputTokens; a zero value leaves
// the output limit to the provider.
func newChatRequestBody(model string, req llm.GenerateRequest) ([]byte, error) {
	body := chatRequest{
		Model: model,
		Messages: []chatMessage{
			{Role: roleSystem, Content: req.SystemPrompt},
			{Role: roleUser, Content: req.UserPrompt},
		},
		Stream: false,
	}
	if req.MaxOutputTokens > 0 {
		limit := req.MaxOutputTokens
		body.MaxTokens = &limit
	}
	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("encode request body: %w", err)
	}
	return data, nil
}

// checkAPIKey validates the shape of the API key: it must be present and made
// only of printable ASCII. This is one of exactly two places that call
// Reveal; the other sets the Authorization header. The revealed string is
// discarded after the check and is never stored in a field or an error.
func checkAPIKey(apiKey secret.Secret) error {
	value, err := apiKey.Reveal()
	if err != nil {
		return errZeroAPIKey
	}
	for i := range len(value) {
		if value[i] < '!' || value[i] > '~' {
			return errInvalidAPIKey
		}
	}
	return nil
}

// newRequest builds the POST request with the JSON body and the two required
// headers. It sets the Authorization header through the other Reveal call
// site; the revealed string is used for the header and discarded. A header
// failure returns without sending the request.
func (c *client) newRequest(ctx context.Context, body []byte) (*http.Request, error) {
	//nolint:gosec // the endpoint is the fixed production URL or a loopback URL checked by the test helper
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")
	apiKey, err := c.apiKey.Reveal()
	if err != nil {
		return nil, errRevealAPIKey
	}
	request.Header.Set("Authorization", "Bearer "+apiKey)
	return request, nil
}
