// Package provider builds the llm.LLMClient selected by the configuration. It
// is the only package that maps a config.Provider value to an adapter package.
package provider

import (
	"errors"
	"fmt"
	"time"

	"github.com/isseis/yt2column/internal/config"
	"github.com/isseis/yt2column/internal/llm"
	"github.com/isseis/yt2column/internal/llm/claude"
	"github.com/isseis/yt2column/internal/llm/deepseek"
)

// LLMTimeout bounds one LLM call, for every provider. DeepSeek holds a request
// for up to ten minutes before inference starts; the remaining five minutes
// cover the generation itself. The Claude adapter's default output limit is
// sized so that a non-streaming call producing that many tokens at the slowest
// observed output rate finishes within half of this timeout; a test in this
// package pins that relation, and
// docs/dev/guides/claude_output_limit.md describes how to re-measure
// it.
const LLMTimeout = 15 * time.Minute

// errUnknownProvider reports a Provider value whose construction is undefined.
// It is unexported: a Provider outside the enumerated values cannot come from
// config.Load, so only a package test needs to tell this rejection apart.
var errUnknownProvider = errors.New("unknown LLM provider")

// New builds the generation LLMClient from cfg.Generation(): its provider,
// model ID, and effort. It never reads environment variables; every value
// comes from the already-validated LLMConfig.
func New(cfg config.LLMConfig) (llm.LLMClient, error) {
	return newGeneration(cfg, productionBuilders())
}

// NewReview builds the review LLMClient from cfg.Review(): its provider, model
// ID, and effort. It never reads environment variables; every value comes from
// the already-validated LLMConfig.
func NewReview(cfg config.LLMConfig) (llm.LLMClient, error) {
	return newReview(cfg, productionBuilders())
}

// builders are the adapter constructors. Production passes deepseek.New and
// claude.New; a test passes a loopback constructor or a spy.
type builders struct {
	deepseek func(deepseek.Options) (llm.LLMClient, error)
	claude   func(claude.Options) (llm.LLMClient, error)
}

// productionBuilders returns the real adapter constructors.
func productionBuilders() builders {
	return builders{deepseek: deepseek.New, claude: claude.New}
}

// newGeneration builds the generation client from cfg.Generation(). It is New
// with the constructors as a parameter, so a test sends to a loopback server
// through the same stage selection as production.
func newGeneration(cfg config.LLMConfig, build builders) (llm.LLMClient, error) {
	return newClient(cfg.Generation(), cfg, build)
}

// newReview builds the review client from cfg.Review(). It is NewReview with
// the constructors as a parameter, for the same reason as newGeneration.
func newReview(cfg config.LLMConfig, build builders) (llm.LLMClient, error) {
	return newClient(cfg.Review(), cfg, build)
}

// newClient builds the adapter for one stage's selection, so newGeneration
// and newReview share it. Contract: the provider, the model ID, and the effort
// all come from sel, never from the other stage, so one stage's effort cannot
// reach the other's client; only the values the stages share (the API keys
// and the workspace ID) come from cfg. build holds the adapter constructors.
// A selection whose provider is not enumerated (the zero LLMSelection, which
// a Config from Load never holds for a stage it builds) returns
// errUnknownProvider without calling any of them.
func newClient(sel config.LLMSelection, cfg config.LLMConfig, build builders) (llm.LLMClient, error) {
	switch sel.Provider() {
	case config.ProviderDeepSeek:
		client, err := build.deepseek(deepseek.Options{
			APIKey:  cfg.DeepSeekAPIKey(),
			Model:   sel.Model(),
			Effort:  sel.DeepSeekEffort(),
			Timeout: LLMTimeout,
		})
		if err != nil {
			return nil, fmt.Errorf("building the deepseek client: %w", err)
		}
		return client, nil
	case config.ProviderClaude:
		client, err := build.claude(claude.Options{
			APIKey:      cfg.AnthropicAPIKey(),
			Model:       sel.Model(),
			Effort:      sel.ClaudeEffort(),
			WorkspaceID: cfg.AnthropicWorkspaceID(),
			Timeout:     LLMTimeout,
		})
		if err != nil {
			return nil, fmt.Errorf("building the claude client: %w", err)
		}
		return client, nil
	default:
		return nil, fmt.Errorf("%w: %d", errUnknownProvider, sel.Provider())
	}
}
