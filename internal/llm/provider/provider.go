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
// cover the generation itself. A non-streaming Claude call within the Claude
// adapter's default output limit was measured to finish well inside it.
const LLMTimeout = 15 * time.Minute

// errUnknownProvider reports a Provider value whose construction is undefined.
// It is unexported: a Provider outside the enumerated values cannot come from
// config.Load, so only a package test needs to tell this rejection apart.
var errUnknownProvider = errors.New("unknown LLM provider")

// New builds the LLMClient for cfg.Provider(). It never reads environment
// variables; every value comes from the already-validated Config.
func New(cfg config.Config) (llm.LLMClient, error) {
	return newClient(cfg.Provider(), cfg, builders{deepseek: deepseek.New, claude: claude.New})
}

// builders are the adapter constructors. Production passes deepseek.New and
// claude.New; a test passes a loopback constructor or a spy.
type builders struct {
	deepseek func(deepseek.Options) (llm.LLMClient, error)
	claude   func(claude.Options) (llm.LLMClient, error)
}

// newClient builds the adapter for provider. It takes the provider directly so
// a package test can exercise the unknown-provider branch, which a Config from
// Load cannot produce. build holds the adapter constructors: production passes
// deepseek.New and claude.New, a test passes loopback constructors or spies. An
// unknown provider returns errUnknownProvider without calling any of them.
func newClient(provider config.Provider, cfg config.Config, build builders) (llm.LLMClient, error) {
	switch provider {
	case config.ProviderDeepSeek:
		client, err := build.deepseek(deepseek.Options{
			APIKey:  cfg.DeepSeekAPIKey(),
			Model:   cfg.Model(),
			Timeout: LLMTimeout,
		})
		if err != nil {
			return nil, fmt.Errorf("building the deepseek client: %w", err)
		}
		return client, nil
	case config.ProviderClaude:
		client, err := build.claude(claude.Options{
			APIKey:      cfg.AnthropicAPIKey(),
			Model:       cfg.Model(),
			Effort:      cfg.ClaudeEffort(),
			WorkspaceID: cfg.AnthropicWorkspaceID(),
			Timeout:     LLMTimeout,
		})
		if err != nil {
			return nil, fmt.Errorf("building the claude client: %w", err)
		}
		return client, nil
	default:
		return nil, fmt.Errorf("%w: %d", errUnknownProvider, provider)
	}
}
