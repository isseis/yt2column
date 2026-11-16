//go:build test

package config

import (
	"errors"
	"testing"

	"github.com/isseis/yt2column/internal/llm/claudeparam"
)

// TestLoadExtensionProductionProfile checks the extension's LLM configuration:
// both selections are the production profile (claude, claude-opus-5-5, high
// effort), the API key is set, the workspace ID is present exactly when a
// non-empty one was stored, and a value the CLI's loader rejects is rejected
// here too, unchanged.
//
// The lookup answers the only two names that can carry a value, so these rows
// also pin that no other setting — a provider, a model, an effort, or a cache
// directory — can come from the extension; TestExtensionLookup pins that the
// lookup answers nothing else.
func TestLoadExtensionProductionProfile(t *testing.T) {
	const (
		apiKey      = "sk-ant-test-key"
		workspaceID = "wrkspc_test0123456789"
	)
	accepted := []struct {
		name          string
		lookup        LookupFunc
		wantWorkspace string
	}{
		{"key and workspace ID", ExtensionLookup(apiKey, workspaceID), workspaceID},
		{"key without a stored workspace ID", ExtensionLookup(apiKey, ""), ""},
	}
	for _, tc := range accepted {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := LoadExtension(tc.lookup)
			if err != nil {
				t.Fatalf("LoadExtension() error = %v, want nil", err)
			}
			for name, selection := range map[string]LLMSelection{
				"generation": cfg.Generation(),
				"review":     cfg.Review(),
			} {
				if selection.Provider() != ProviderClaude {
					t.Errorf("%s provider = %v, want claude", name, selection.Provider())
				}
				if proxy := selection.Model(); proxy != "claude-opus-5-5" {
					t.Errorf("%s model = %q, want the production profile's model", name, proxy)
				}
				if effort := selection.ClaudeEffort(); effort != claudeparam.EffortHigh {
					t.Errorf("%s effort = %v, want high", name, effort)
				}
			}
			if got, ok := cfg.AnthropicWorkspaceID().Value(); got != tc.wantWorkspace || ok != (tc.wantWorkspace != "") {
				t.Errorf("workspace ID = (%q, %v), want (%q, %v)", got, ok, tc.wantWorkspace, tc.wantWorkspace != "")
			}
			if got, err := cfg.AnthropicAPIKey().Reveal(); err != nil || got != apiKey {
				t.Errorf("API key = (%q, %v), want the stored key", got, err)
			}
		})
	}

	rejected := []struct {
		name   string
		lookup LookupFunc
		want   error
	}{
		{"empty API key", ExtensionLookup("", workspaceID), ErrMissing},
		{"no API key", func(string) (string, bool) { return "", false }, ErrMissing},
		{
			"empty workspace ID stored as present",
			envLookup(map[string]string{anthropicAPIKeyEnv: apiKey, anthropicWorkspaceIDEnv: ""}),
			ErrMissing,
		},
		{"workspace ID with a space", ExtensionLookup(apiKey, "wrkspc_test 0123456789"), ErrInvalid},
		{"padded workspace ID", ExtensionLookup(apiKey, " "+workspaceID), ErrInvalid},
	}
	for _, tc := range rejected {
		t.Run("rejects "+tc.name, func(t *testing.T) {
			cfg, err := LoadExtension(tc.lookup)
			if !errors.Is(err, tc.want) {
				t.Fatalf("LoadExtension() error = %v, want %v", err, tc.want)
			}
			if cfg.Generation() != (LLMSelection{}) || cfg.Review() != (LLMSelection{}) {
				t.Errorf("LoadExtension() selections = (%+v, %+v), want the zero value on a rejection", cfg.Generation(), cfg.Review())
			}
		})
	}
}

// TestExtensionLookup checks that the lookup answers the two stored-setting
// names and nothing else: the API key always, the workspace ID only when a
// non-empty one is stored.
func TestExtensionLookup(t *testing.T) {
	const (
		apiKey      = "sk-ant-test-key"
		workspaceID = "wrkspc_test0123456789"
	)
	lookup := ExtensionLookup(apiKey, workspaceID)
	for name, want := range map[string]struct {
		value string
		ok    bool
	}{
		anthropicAPIKeyEnv:      {apiKey, true},
		anthropicWorkspaceIDEnv: {workspaceID, true},
		llmEnv:                  {},
		reviewLLMEnv:            {},
		deepSeekAPIKeyEnv:       {},
		openAIAPIKeyEnv:         {},
		cacheDirEnv:             {},
		ytDlpEnv:                {},
		godebugEnv:              {},
		"HOME":                  {},
	} {
		got, ok := lookup(name)
		if got != want.value || ok != want.ok {
			t.Errorf("lookup(%q) = (%q, %v), want (%q, %v)", name, got, ok, want.value, want.ok)
		}
	}

	t.Run("empty stored values", func(t *testing.T) {
		empty := ExtensionLookup("", "")
		if got, ok := empty(anthropicAPIKeyEnv); got != "" || !ok {
			t.Errorf("lookup of the API key = (%q, %v), want (\"\", true): the loader decides whether it is accepted", got, ok)
		}
		if got, ok := empty(anthropicWorkspaceIDEnv); got != "" || ok {
			t.Errorf("lookup of the workspace ID = (%q, %v), want (\"\", false): an empty stored value means not specified", got, ok)
		}
	})
}

// TestReadExtensionSettings checks that the development command's reader
// returns both stored values as they are, with whether each name was set, and
// makes no check of its own.
func TestReadExtensionSettings(t *testing.T) {
	for _, tc := range []struct {
		name             string
		env              map[string]string
		wantAPIKey       string
		wantAPIKeySet    bool
		wantWorkspace    string
		wantWorkspaceSet bool
	}{
		{"both set", map[string]string{anthropicAPIKeyEnv: "sk-ant-test-key", anthropicWorkspaceIDEnv: "wrkspc_test0123456789"}, "sk-ant-test-key", true, "wrkspc_test0123456789", true},
		{"neither set", map[string]string{}, "", false, "", false},
		{"workspace ID absent", map[string]string{anthropicAPIKeyEnv: "sk-ant-test-key"}, "sk-ant-test-key", true, "", false},
		{"unchecked values", map[string]string{anthropicAPIKeyEnv: " padded ", anthropicWorkspaceIDEnv: "not a workspace"}, " padded ", true, "not a workspace", true},
		{"empty values present", map[string]string{anthropicAPIKeyEnv: "", anthropicWorkspaceIDEnv: ""}, "", true, "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			settings := ReadExtensionSettings(envLookup(tc.env))
			if got, ok := settings.APIKey(); got != tc.wantAPIKey || ok != tc.wantAPIKeySet {
				t.Errorf("APIKey() = (%q, %v), want (%q, %v)", got, ok, tc.wantAPIKey, tc.wantAPIKeySet)
			}
			if got, ok := settings.WorkspaceID(); got != tc.wantWorkspace || ok != tc.wantWorkspaceSet {
				t.Errorf("WorkspaceID() = (%q, %v), want (%q, %v)", got, ok, tc.wantWorkspace, tc.wantWorkspaceSet)
			}
		})
	}
}
