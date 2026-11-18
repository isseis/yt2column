//go:build test

package config

import (
	"github.com/isseis/yt2column/internal/llm/claudeparam"
	"github.com/isseis/yt2column/internal/llm/deepseekparam"
	"github.com/isseis/yt2column/internal/llm/openaiparam"
)

// The LLM default, exposed so the CLI tests and the documentation guard in
// cmd/yt2column compare against the code instead of copying the values.

// DefaultLLMValueForTest returns the YT2COLUMN_LLM value used when the
// variable is unset.
func DefaultLLMValueForTest() string { return defaultLLMValue }

// DefaultClaudeModelForTest returns the model ID of the selection used when
// YT2COLUMN_LLM is unset.
func DefaultClaudeModelForTest() string { return defaultClaudeModel }

// The LLM variable names, exposed so the documentation guard in cmd/yt2column
// derives them from the code instead of copying them.

// LLMEnvForTest returns the name of the generation LLM variable.
func LLMEnvForTest() string { return llmEnv }

// ReviewLLMEnvForTest returns the name of the review LLM variable.
func ReviewLLMEnvForTest() string { return reviewLLMEnv }

// AnthropicAPIKeyEnvForTest returns the name of the Anthropic API key variable.
func AnthropicAPIKeyEnvForTest() string { return anthropicAPIKeyEnv }

// DefaultClaudeEffortForTest returns the effort a claude stage uses when its
// LLM variable's value has no effort part.
func DefaultClaudeEffortForTest() string { return defaultClaudeEffort.String() }

// DefaultOpenAIEffortForTest returns the effort an openai stage uses when its
// LLM variable's value has no effort part.
func DefaultOpenAIEffortForTest() string { return defaultOpenAIEffort.String() }

// EffortValuesByProviderForTest returns, for each provider part the LLM
// variables accept, the effort values parseLLM accepts for it.
func EffortValuesByProviderForTest() map[string][]string {
	return map[string][]string{
		providerClaude:   claudeparam.EffortValues(),
		providerDeepSeek: deepseekparam.EffortValues(),
		providerOpenAI:   openaiparam.EffortValues(),
	}
}
