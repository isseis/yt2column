package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/isseis/yt2column/internal/llm"
	"github.com/isseis/yt2column/internal/llm/openaiparam"
)

// Fixed request values of the Responses API.
const (
	// endpoint is the fixed production destination. It is not configurable:
	// only the test helper (built with the test tag) replaces it, and only
	// with a loopback URL. The model is sent in the body, never in the URL.
	endpoint = "https://api.openai.com/v1/responses"
	// defaultMaxOutputTokens is sent as max_output_tokens when the request
	// leaves the output limit to the provider (MaxOutputTokens 0). The limit
	// covers reasoning and the answer together. The value is at least twice
	// the largest observed output, producing it at the slowest observed
	// output rate takes at most half of provider.LLMTimeout, and both
	// candidate models accept it; the measurement is recorded in
	// docs/tasks/0077_openai_llm_client/02_architecture.md, and
	// docs/dev/guides/openai_output_limit.md says when and how to re-measure
	// it.
	defaultMaxOutputTokens = 6080
	// serviceTierDefault is the Standard processing tier. The request always
	// names it, so the rates a usage is priced at never depend on a project
	// setting.
	serviceTierDefault = "default"
	// promptCacheTTL is the minimum prompt cache retention the request
	// always names, so the cache-write rate a usage is priced at is fixed.
	promptCacheTTL = "30m"
)

// Request header values. The adapter sets exactly Content-Type and
// Authorization.
const (
	contentTypeJSON     = "application/json"
	authorizationPrefix = "Bearer "
)

// Input roles the adapter sends: the system prompt and the user prompt keep
// their own roles.
const (
	inputRoleSystem = "system"
	inputRoleUser   = "user"
)

// responsesRequest is the request body sent to the Responses API. It has no
// fields for parameters outside the requirements (tools, previous_response_id,
// conversation, reasoning summary, ...), so none of them can be sent. Stream
// and Store are always false and are sent explicitly.
type responsesRequest struct {
	Model              string             `json:"model"`
	Input              []inputMessage     `json:"input"`
	Reasoning          reasoning          `json:"reasoning"`
	MaxOutputTokens    int                `json:"max_output_tokens"`
	Stream             bool               `json:"stream"`
	Store              bool               `json:"store"`
	ServiceTier        string             `json:"service_tier"`
	PromptCacheOptions promptCacheOptions `json:"prompt_cache_options"`
}

// inputMessage is one input message. Content is the prompt string itself,
// never an array of parts, so no extra part can be sent.
type inputMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// reasoning carries the one reasoning parameter the adapter sends.
type reasoning struct {
	Effort string `json:"effort"`
}

// promptCacheOptions carries the one prompt cache parameter the adapter sends.
type promptCacheOptions struct {
	TTL string `json:"ttl"`
}

// effectiveMaxTokens returns the max_output_tokens a request sends: the
// request's positive MaxOutputTokens, or the adapter default when the request
// leaves the output limit to the provider. The error messages name the same
// value, so both callers share this rule.
func effectiveMaxTokens(req llm.GenerateRequest) int {
	if req.MaxOutputTokens > 0 {
		return req.MaxOutputTokens
	}
	return defaultMaxOutputTokens
}

// newRequestBody builds the JSON request body. It never assembles the prompts
// by string concatenation and never includes the API key.
func newRequestBody(model string, effort openaiparam.Effort, req llm.GenerateRequest) ([]byte, error) {
	body := responsesRequest{
		Model: model,
		Input: []inputMessage{
			{Role: inputRoleSystem, Content: req.SystemPrompt},
			{Role: inputRoleUser, Content: req.UserPrompt},
		},
		Reasoning:          reasoning{Effort: effort.String()},
		MaxOutputTokens:    effectiveMaxTokens(req),
		Stream:             false,
		Store:              false,
		ServiceTier:        serviceTierDefault,
		PromptCacheOptions: promptCacheOptions{TTL: promptCacheTTL},
	}
	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("encode request body: %w", err)
	}
	return data, nil
}

// newRequest builds the POST request with the JSON body and the required
// headers. It sets Authorization through the package's only Reveal call site;
// the revealed string is used for the header and discarded. A header failure
// returns without sending the request.
func (c *client) newRequest(ctx context.Context, body []byte) (*http.Request, error) {
	//nolint:gosec // the endpoint is the fixed production URL or a loopback URL checked by the test helper
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	request.Header.Set("Content-Type", contentTypeJSON)
	apiKey, err := c.apiKey.Reveal()
	if err != nil {
		return nil, errRevealAPIKey
	}
	request.Header.Set("Authorization", authorizationPrefix+apiKey)
	return request, nil
}
