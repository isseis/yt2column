//go:build test

package config

import (
	"path/filepath"
	"slices"

	"github.com/isseis/yt2column/internal/llm/claudeparam"
	"github.com/isseis/yt2column/internal/llm/deepseekparam"
)

// The server-relevant variable names and the socket path rule, exposed so the
// documentation guard in cmd/yt2column-server and the internal/serversock
// tests derive them from the code instead of copying them. Built only with the
// test tag.

// SlackWebhookEnvForTest is the environment variable naming the Webhook URL.
func SlackWebhookEnvForTest() string { return slackEnv }

// CacheDirEnvForTest is the environment variable naming the cache directory,
// which LoadServer does not read.
func CacheDirEnvForTest() string { return cacheDirEnv }

// YtDlpEnvForTest is the environment variable naming the yt-dlp path, which
// LoadServer does not read.
func YtDlpEnvForTest() string { return ytDlpEnv }

// SocketPathTailForTest returns the part of the socket path below HOME for
// goos, starting with a separator. It panics for an operating system with no
// rule.
func SocketPathTailForTest(goos string) string {
	parts, _, ok := socketRule(goos)
	if !ok {
		panic("config: no socket path rule for " + goos)
	}
	return string(filepath.Separator) + filepath.Join(slices.Concat(parts, []string{socketFileName})...)
}

// SocketPathMaxForTest returns the longest socket path, in bytes, accepted for
// goos. It panics for an operating system with no rule.
func SocketPathMaxForTest(goos string) int {
	_, limit, ok := socketRule(goos)
	if !ok {
		panic("config: no socket path rule for " + goos)
	}
	return limit
}

// The LLM default and the retired-variable rejection, exposed so the CLI
// tests and the documentation guard in cmd/yt2column compare against the code
// instead of copying the values.

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

// DeepSeekAPIKeyEnvForTest returns the name of the DeepSeek API key variable.
func DeepSeekAPIKeyEnvForTest() string { return deepSeekAPIKeyEnv }

// EffortValuesByProviderForTest returns, for each provider part the LLM
// variables accept, the effort values parseLLM accepts for it.
func EffortValuesByProviderForTest() map[string][]string {
	return map[string][]string{
		providerClaude:   claudeparam.EffortValues(),
		providerDeepSeek: deepseekparam.EffortValues(),
	}
}

// RetiredLLMVarsForTest returns the names of the retired LLM variables, in the
// order their rejections are reported.
func RetiredLLMVarsForTest() []string {
	names := make([]string, 0, len(retiredLLMVars))
	for _, v := range retiredLLMVars {
		names = append(names, v.name)
	}
	return names
}

// RetiredLLMVarErrorForTest returns the text of the rejection of the retired
// variable name, as a configuration error prints it. It panics for a name that
// is not a retired variable.
func RetiredLLMVarErrorForTest(name string) string {
	for _, v := range retiredLLMVars {
		if v.name == name {
			return invalidVar(v.name, v.reason).Error()
		}
	}
	panic("config: not a retired LLM variable: " + name)
}
