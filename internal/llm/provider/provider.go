// Package provider builds the llm.LLMClient selected by the configuration. It
// is the only package that maps a config.Provider value to an adapter package.
package provider

import (
	"errors"
	"fmt"
	"time"

	"github.com/isseis/yt2column/internal/config"
	"github.com/isseis/yt2column/internal/llm"
	"github.com/isseis/yt2column/internal/llm/deepseek"
	"github.com/isseis/yt2column/internal/secret"
)

// LLMTimeout bounds one LLM call. DeepSeek holds a request for up to ten
// minutes before inference starts; the remaining five minutes cover the
// generation itself.
const LLMTimeout = 15 * time.Minute

// errUnknownProvider reports a Provider value whose construction is undefined.
// It is unexported: a Provider outside the enumerated values cannot come from
// config.Load, so only a package test needs to tell this rejection apart.
var errUnknownProvider = errors.New("unknown LLM provider")

// New builds the LLMClient for cfg.Provider(). It never reads environment
// variables; every value comes from the already-validated Config.
func New(cfg config.Config) (llm.LLMClient, error) {
	return newClient(cfg.Provider(), cfg.DeepSeekAPIKey(), cfg.Model(), deepseek.New)
}

// newClient builds the adapter for provider. It takes the provider directly so
// a package test can exercise the unknown-provider branch, which a Config from
// Load cannot produce. build is the adapter constructor: production passes
// deepseek.New, a test passes the loopback constructor or a spy. An unknown
// provider returns errUnknownProvider without calling build.
func newClient(provider config.Provider, apiKey secret.Secret, model string, build func(deepseek.Options) (llm.LLMClient, error)) (llm.LLMClient, error) {
	switch provider {
	case config.ProviderDeepSeek:
		client, err := build(deepseek.Options{
			APIKey:  apiKey,
			Model:   model,
			Timeout: LLMTimeout,
		})
		if err != nil {
			return nil, fmt.Errorf("building the deepseek client: %w", err)
		}
		return client, nil
	default:
		return nil, fmt.Errorf("%w: %d", errUnknownProvider, provider)
	}
}
