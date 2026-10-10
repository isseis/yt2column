package claude

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/isseis/yt2column/internal/llm"
	"github.com/isseis/yt2column/internal/llm/claudeparam"
	"github.com/isseis/yt2column/internal/secret"
)

// Fixed request values of the Messages API.
const (
	// endpoint is the fixed production destination. It is not configurable:
	// only the test helper (built with the test tag) replaces it, and only
	// with a loopback URL.
	endpoint = "https://api.anthropic.com/v1/messages"
	// anthropicVersion is the fixed value of the anthropic-version header.
	anthropicVersion = "2023-06-01"
	// defaultMaxOutputTokens is sent as max_tokens when the request leaves the
	// output limit to the provider (MaxOutputTokens 0). The Messages API
	// requires max_tokens, so a value must always be sent.
	defaultMaxOutputTokens = 16000
)

// Request header names. The adapter sets exactly these; it never sets
// Authorization or anthropic-beta.
const (
	headerAPIKey    = "x-api-key"
	headerVersion   = "anthropic-version"
	headerWorkspace = "anthropic-workspace-id"
)

// contentTypeJSON is the value of the Content-Type header.
const contentTypeJSON = "application/json"

// messageRoleUser is the only role the adapter sends.
const messageRoleUser = "user"

// messagesRequest is the request body sent to the Messages API. It has no
// fields for parameters outside the requirements (thinking, stream,
// temperature, ...), so none of them can be sent.
type messagesRequest struct {
	Model        string               `json:"model"`
	System       string               `json:"system"`
	Messages     []messagesRequestMsg `json:"messages"`
	MaxTokens    int                  `json:"max_tokens"`
	OutputConfig outputConfig         `json:"output_config"`
}

// messagesRequestMsg is one message of the request. Content is the prompt
// string itself, never an array of blocks, so no extra block can be sent.
type messagesRequestMsg struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// outputConfig carries the one output parameter the adapter sends.
type outputConfig struct {
	Effort string `json:"effort"`
}

// newRequestBody builds the JSON request body. It never assembles the prompts
// by string concatenation and never includes the API key or the workspace ID.
// max_tokens is the request's positive MaxOutputTokens, or the adapter default
// when the request leaves the limit to the provider.
func newRequestBody(model string, effort claudeparam.Effort, req llm.GenerateRequest) ([]byte, error) {
	maxTokens := req.MaxOutputTokens
	if maxTokens == 0 {
		maxTokens = defaultMaxOutputTokens
	}
	body := messagesRequest{
		Model:  model,
		System: req.SystemPrompt,
		Messages: []messagesRequestMsg{
			{Role: messageRoleUser, Content: req.UserPrompt},
		},
		MaxTokens:    maxTokens,
		OutputConfig: outputConfig{Effort: effort.String()},
	}
	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("encode request body: %w", err)
	}
	return data, nil
}

// checkAPIKey validates the shape of the API key: it must be present and made
// only of printable ASCII. This is one of exactly two places that call
// Reveal; the other sets the x-api-key header. The revealed string is
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

// newRequest builds the POST request with the JSON body and the required
// headers. It sets x-api-key through the other Reveal call site; the revealed
// string is used for the header and discarded. The anthropic-workspace-id
// header is set only when a workspace ID was specified. A header failure
// returns without sending the request.
func (c *client) newRequest(ctx context.Context, body []byte) (*http.Request, error) {
	//nolint:gosec // the endpoint is the fixed production URL or a loopback URL checked by the test helper
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	request.Header.Set("Content-Type", contentTypeJSON)
	request.Header.Set(headerVersion, anthropicVersion)
	apiKey, err := c.apiKey.Reveal()
	if err != nil {
		return nil, errRevealAPIKey
	}
	request.Header.Set(headerAPIKey, apiKey)
	if workspaceID, ok := c.workspaceID.Value(); ok {
		request.Header.Set(headerWorkspace, workspaceID)
	}
	return request, nil
}
