// Package claudebackend is the release Backend of the article generation
// module: the production profile's Claude clients, built directly with
// claude.New, and the Claude Models API key check. It does not use
// internal/llm/provider, which imports adapters that do not build for
// js/wasm; TestClientsMatchProvider keeps the two constructions equal.
package claudebackend

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/isseis/yt2column/internal/config"
	"github.com/isseis/yt2column/internal/extjob"
	"github.com/isseis/yt2column/internal/llm"
	"github.com/isseis/yt2column/internal/llm/claude"
)

// llmTimeout bounds one LLM call. It equals provider.LLMTimeout: see the
// package comment.
const llmTimeout = 15 * time.Minute

// keyCheckTimeout bounds the key check's HTTP call. extjob gives the call a
// context with the same limit; this is the client's own bound.
const keyCheckTimeout = 30 * time.Second

// ErrNotClaude is returned by NewClients for a configuration whose generation
// or review selection is not a Claude one, which the production profile never
// produces.
var ErrNotClaude = errors.New("claudebackend: the selection is not a Claude selection")

// backend builds the clients with claude.New and checks keys with
// claude.CheckKey; a test replaces the two to observe what they are given.
type backend struct {
	newClient func(claude.Options) (llm.LLMClient, error)
	checkKey  func(context.Context, claude.KeyCheckOptions) error
}

// New returns the Backend of the production profile's Claude clients and
// claude.CheckKey.
func New() extjob.Backend {
	return backend{newClient: claude.New, checkKey: claude.CheckKey}
}

// NewClients builds the generation and review clients of one run from cfg.
// Contract: each client's model and effort come from its own stage's
// selection, never from the other stage's, as in internal/llm/provider; only
// the API key and the workspace ID are shared.
func (b backend) NewClients(cfg config.LLMConfig) (extjob.Deps, error) {
	generation, err := b.build(cfg.Generation(), cfg)
	if err != nil {
		return extjob.Deps{}, fmt.Errorf("building the generation client: %w", err)
	}
	review, err := b.build(cfg.Review(), cfg)
	if err != nil {
		return extjob.Deps{}, fmt.Errorf("building the review client: %w", err)
	}
	return extjob.Deps{Generation: generation, Review: review}, nil
}

func (b backend) build(sel config.LLMSelection, cfg config.LLMConfig) (llm.LLMClient, error) {
	if sel.Provider() != config.ProviderClaude {
		return nil, ErrNotClaude
	}
	return b.newClient(claude.Options{
		APIKey:      cfg.AnthropicAPIKey(),
		Model:       sel.Model(),
		Effort:      sel.ClaudeEffort(),
		WorkspaceID: cfg.AnthropicWorkspaceID(),
		Timeout:     llmTimeout,
	})
}

// CheckKey asks the Models API about the key of cfg.
func (b backend) CheckKey(ctx context.Context, cfg config.LLMConfig) error {
	return b.checkKey(ctx, claude.KeyCheckOptions{
		APIKey:      cfg.AnthropicAPIKey(),
		WorkspaceID: cfg.AnthropicWorkspaceID(),
		Timeout:     keyCheckTimeout,
	})
}

// Info declares the Claude implementation.
func (backend) Info() extjob.BackendInfo { return extjob.ClaudeInfo() }
