//go:build test

package config

import (
	"maps"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

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

// DefaultClaudeEffortForTest returns the effort a claude stage uses when its
// LLM variable's value has no effort part.
func DefaultClaudeEffortForTest() string { return defaultClaudeEffort.String() }

// EffortValuesByProviderForTest returns, for each provider part the LLM
// variables accept, the effort values parseLLM accepts for it.
func EffortValuesByProviderForTest() map[string][]string {
	return map[string][]string{
		providerClaude:   claudeparam.EffortValues(),
		providerDeepSeek: deepseekparam.EffortValues(),
	}
}

// The server configuration file, exposed so the tests outside this package
// write the file the server reads instead of copying its path rule.

// ServerFilePathForTest returns the configuration file's path under home on
// this operating system. It panics when no path can be derived from home.
func ServerFilePathForTest(home string) string {
	loc, err := serverFileLocationFor(runtime.GOOS, func(name string) (string, bool) {
		if name == "HOME" {
			return home, true
		}
		return "", false
	})
	if err != nil {
		panic("config: no configuration file path for HOME " + home + ": " + err.Error())
	}
	return loc.path
}

// ServerFileKeysForTest returns the keys the configuration file accepts.
func ServerFileKeysForTest() []string { return slices.Clone(serverFileKeys) }

// ServerFileMaxSizeForTest returns the largest configuration file accepted,
// in bytes.
func ServerFileMaxSizeForTest() int { return serverFileMaxSize }

// WriteServerFileForTest writes data to the configuration file under home,
// replacing a file already there, and returns its path. The modes are set
// with chmod, so they do not depend on the umask: 0700 for the file's
// directory and 0600 for the file. It does not use privatefile.MkdirBelow,
// which changes the process's umask.
func WriteServerFileForTest(t testing.TB, home string, data []byte) string {
	t.Helper()
	path := ServerFilePathForTest(home)
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil { //nolint:gosec // a test's directory under its own HOME
		t.Fatalf("create the configuration file's directory: %v", err)
	}
	if err := os.Chmod(dir, 0o700); err != nil { //nolint:gosec // a test's directory under its own HOME
		t.Fatalf("chmod the configuration file's directory: %v", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil { //nolint:gosec // a test's file under its own HOME
		t.Fatalf("write the configuration file: %v", err)
	}
	if err := os.Chmod(path, 0o600); err != nil { //nolint:gosec // a test's file under its own HOME
		t.Fatalf("chmod the configuration file: %v", err)
	}
	return path
}

// ServerFileForTest returns the configuration file's content for values: one
// KEY=VALUE line per entry, in key order.
func ServerFileForTest(values map[string]string) []byte {
	var b strings.Builder
	for _, key := range slices.Sorted(maps.Keys(values)) {
		b.WriteString(key + "=" + values[key] + "\n")
	}
	return []byte(b.String())
}

// ServerLookupForTest splits vars into the keys the configuration file
// accepts and the rest. It writes the keys to the configuration file under
// vars' HOME, when HOME is absolute, and returns a LookupFunc over the rest,
// the environment LoadServer reads. When HOME is unset or relative nothing is
// written, since LoadServer stops before it reads the file.
func ServerLookupForTest(t testing.TB, vars map[string]string) LookupFunc {
	t.Helper()
	file := map[string]string{}
	env := map[string]string{}
	for name, value := range vars {
		if slices.Contains(serverFileKeys, name) {
			file[name] = value
		} else {
			env[name] = value
		}
	}
	if home, ok := env["HOME"]; ok && filepath.IsAbs(home) {
		WriteServerFileForTest(t, home, ServerFileForTest(file))
	}
	return func(name string) (string, bool) {
		value, ok := env[name]
		return value, ok
	}
}
