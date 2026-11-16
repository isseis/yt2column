package config

import (
	"errors"
	"fmt"
	"log/slog"
)

// LoadExtension reads the LLM configuration of the article generation module
// through lookup, with the same loaders and checks as Load: the generation and
// review selections, the API key, and the optional workspace ID. Contract: the
// selections resolve to the production profile, the code's default, because
// ExtensionLookup answers no name that could select a provider, a model, or an
// effort. It joins every rejection together and returns the zero LLMConfig on
// any error, as Load does.
func LoadExtension(lookup LookupFunc) (LLMConfig, error) {
	llm, errs := loadLLM(lookup, Need{Generation: true, Review: true})
	if err := errors.Join(errs...); err != nil {
		return LLMConfig{}, err
	}
	return llm, nil
}

// ExtensionLookup returns the lookup LoadExtension takes for the extension's
// two stored values: it answers ANTHROPIC_API_KEY and, when workspaceID is not
// empty, ANTHROPIC_WORKSPACE_ID, and no other name.
//
// Contract: an empty workspaceID is answered as unset, because the extension
// stores an empty string for "no workspace ID" and the CLI's loader rejects a
// present empty value. The API key is answered even when it is empty, so the
// loader, not this function, decides whether the stored value is acceptable.
// Neither string is trimmed or otherwise changed.
func ExtensionLookup(apiKey, workspaceID string) LookupFunc {
	return func(name string) (string, bool) {
		switch name {
		case anthropicAPIKeyEnv:
			return apiKey, true
		case anthropicWorkspaceIDEnv:
			if workspaceID == "" {
				return "", false
			}
			return workspaceID, true
		default:
			return "", false
		}
	}
}

// ExtensionSettings are the two stored-settings values read through a lookup,
// unchecked, with whether each name was set. The development command
// (cmd/yt2column-extjob) puts them into the JSON the article generation module
// takes, which checks their shape itself; the running viewer passes the same
// two values from chrome.storage.local unchecked.
type ExtensionSettings struct {
	apiKey         string
	apiKeySet      bool
	workspaceID    string
	workspaceIDSet bool
}

// ReadExtensionSettings reads ANTHROPIC_API_KEY and ANTHROPIC_WORKSPACE_ID
// through lookup and nothing else. Contract: it makes no check, so a value the
// module rejects is returned as it was stored; an absent name is reported as
// absent rather than as an empty string.
func ReadExtensionSettings(lookup LookupFunc) ExtensionSettings {
	apiKey, apiKeySet := lookup(anthropicAPIKeyEnv)
	workspaceID, workspaceIDSet := lookup(anthropicWorkspaceIDEnv)
	return ExtensionSettings{
		apiKey:         apiKey,
		apiKeySet:      apiKeySet,
		workspaceID:    workspaceID,
		workspaceIDSet: workspaceIDSet,
	}
}

// APIKey returns the stored API key as read and whether its name was set.
func (s ExtensionSettings) APIKey() (string, bool) {
	return s.apiKey, s.apiKeySet
}

// WorkspaceID returns the stored workspace ID as read and whether its name was
// set.
func (s ExtensionSettings) WorkspaceID() (string, bool) {
	return s.workspaceID, s.workspaceIDSet
}

// redactedPresence describes a value by whether it was set, never by its text.
func redactedPresence(set bool) string {
	if set {
		return "set"
	}
	return "unset"
}

// String describes the settings without the API key or the workspace ID, so a
// %v, %+v, or %s of the struct, in an error path or a log, cannot print them.
func (s ExtensionSettings) String() string {
	return fmt.Sprintf("ExtensionSettings{apiKey: %s, workspaceID: %s}",
		redactedPresence(s.apiKeySet), redactedPresence(s.workspaceIDSet))
}

// GoString makes %#v print the same redacted form as String.
func (s ExtensionSettings) GoString() string { return s.String() }

// LogValue makes slog print the same redacted form as String.
func (s ExtensionSettings) LogValue() slog.Value { return slog.StringValue(s.String()) }

var (
	_ fmt.Stringer   = ExtensionSettings{}
	_ fmt.GoStringer = ExtensionSettings{}
	_ slog.LogValuer = ExtensionSettings{}
)
