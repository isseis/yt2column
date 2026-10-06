//go:build test

package deepseektestutil

import (
	"maps"
	"slices"
	"strings"
	"testing"
)

const (
	testIntegrationKey   = "test-integration-key-0123456789"
	testProductionKey    = "test-production-key-0123456789"
	testIntegrationModel = "deepseek-custom"
	testOptInEnv         = "YT2COLUMN_EXAMPLE_INTEGRATION"
	testMakeTarget       = "test-integration-example"
)

// settingsCase is one environment and the decision SettingsFrom must make.
type settingsCase struct {
	name       string
	change     map[string]string
	wantAction IntegrationAction
	wantReason []string
}

// checkSettings runs SettingsFrom over complete with each case's changes
// applied, and checks the action, the reason, and the settings it carries.
// An empty value stands for both unset and empty, which getenv cannot tell
// apart.
func checkSettings(t *testing.T, opts IntegrationOptions, complete map[string]string, cases []settingsCase) {
	t.Helper()
	keys := []string{testIntegrationKey, testIntegrationKey[len(testIntegrationKey)-8:], testProductionKey, testProductionKey[len(testProductionKey)-8:]}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env := maps.Clone(complete)
			maps.Copy(env, tc.change)
			read := []string{}
			settings := SettingsFrom(func(name string) string {
				read = append(read, name)
				return env[name]
			}, opts)
			if slices.Contains(read, "DEEPSEEK_API_KEY") {
				t.Errorf("SettingsFrom read DEEPSEEK_API_KEY; it must use only %s", APIKeyEnv)
			}
			if settings.Action != tc.wantAction {
				t.Fatalf("Action = %d, want %d (reason %q)", settings.Action, tc.wantAction, settings.Reason)
			}
			for _, want := range tc.wantReason {
				if !strings.Contains(settings.Reason, want) {
					t.Errorf("Reason %q does not mention %q", settings.Reason, want)
				}
			}
			for _, key := range keys {
				if strings.Contains(settings.Reason, key) {
					t.Errorf("Reason %q contains an API key or its last eight characters", settings.Reason)
				}
			}
			if tc.wantAction != ActionRun {
				if settings.Model != "" {
					t.Errorf("settings for %d carry the model %q", settings.Action, settings.Model)
				}
				if _, err := settings.APIKey.Reveal(); err == nil {
					t.Errorf("settings for %d carry an API key", settings.Action)
				}
				return
			}
			if settings.Model != testIntegrationModel {
				t.Errorf("Model = %q, want %q", settings.Model, testIntegrationModel)
			}
			key, err := settings.APIKey.Reveal()
			if err != nil || key != testIntegrationKey {
				t.Errorf("APIKey is not the value of %s (Reveal error = %v)", APIKeyEnv, err)
			}
		})
	}
}

// completeEnv is an environment in which a test opted in through optIn runs.
// The production key is set so a read of it would be visible.
func completeEnv(optIn string) map[string]string {
	return map[string]string{
		optIn:              OptInValue,
		APIKeyEnv:          testIntegrationKey,
		ModelEnv:           testIntegrationModel,
		"DEEPSEEK_API_KEY": testProductionKey,
	}
}

func TestSettingsFrom(t *testing.T) {
	optInReason := []string{testOptInEnv, "make " + testMakeTarget}
	// common holds the cases on which the missing-key action has no bearing.
	// The ordering cases override two variables to show which check wins.
	common := []settingsCase{
		{name: "opt_in_missing", change: map[string]string{testOptInEnv: ""}, wantAction: ActionSkip, wantReason: optInReason},
		{name: "opt_in_zero", change: map[string]string{testOptInEnv: "0"}, wantAction: ActionSkip, wantReason: optInReason},
		{name: "opt_in_true", change: map[string]string{testOptInEnv: "true"}, wantAction: ActionSkip, wantReason: optInReason},
		{name: "opt_in_padded", change: map[string]string{testOptInEnv: " 1"}, wantAction: ActionSkip, wantReason: optInReason},
		{name: "opt_in_checked_before_api_key", change: map[string]string{testOptInEnv: "", APIKeyEnv: ""}, wantAction: ActionSkip, wantReason: optInReason},
		{name: "opt_in_checked_before_godebug", change: map[string]string{testOptInEnv: "", GODEBUGEnv: "http2debug=1"}, wantAction: ActionSkip, wantReason: optInReason},
		{name: "model_missing", change: map[string]string{ModelEnv: ""}, wantAction: ActionFail, wantReason: []string{ModelEnv, "make " + testMakeTarget}},
		{name: "godebug_http2debug_1", change: map[string]string{GODEBUGEnv: "http2debug=1"}, wantAction: ActionFail, wantReason: []string{GODEBUGEnv, "http2debug=1"}},
		{name: "godebug_http2debug_2_among_others", change: map[string]string{GODEBUGEnv: "gctrace=1,http2debug=2"}, wantAction: ActionFail, wantReason: []string{GODEBUGEnv, "http2debug=2"}},
		{name: "godebug_unrelated", change: map[string]string{GODEBUGEnv: "http2client=0"}, wantAction: ActionRun},
		{name: "complete", wantAction: ActionRun},
	}
	// missingKey holds the cases where the test API key is missing, with the
	// action each missing-key setting must take.
	missingKey := func(action IntegrationAction) []settingsCase {
		return []settingsCase{
			{name: "api_key_missing_with_production_key_set", change: map[string]string{APIKeyEnv: ""}, wantAction: action, wantReason: []string{APIKeyEnv}},
			{name: "api_key_checked_before_model", change: map[string]string{APIKeyEnv: "", ModelEnv: ""}, wantAction: action, wantReason: []string{APIKeyEnv}},
			{name: "api_key_checked_before_godebug", change: map[string]string{APIKeyEnv: "", GODEBUGEnv: "http2debug=1"}, wantAction: action, wantReason: []string{APIKeyEnv}},
		}
	}

	for _, tc := range []struct {
		name          string
		missingKey    MissingKeyAction
		wantMissingAs IntegrationAction
	}{
		{name: "missing_key_skip", missingKey: MissingKeySkip, wantMissingAs: ActionSkip},
		{name: "missing_key_fail", missingKey: MissingKeyFail, wantMissingAs: ActionFail},
	} {
		t.Run(tc.name, func(t *testing.T) {
			opts := IntegrationOptions{OptInEnv: testOptInEnv, MakeTarget: testMakeTarget, MissingKey: tc.missingKey}
			checkSettings(t, opts, completeEnv(testOptInEnv), slices.Concat(common, missingKey(tc.wantMissingAs)))
		})
	}

	t.Run("zero_missing_key_fails", func(t *testing.T) {
		// MissingKey is left at its zero value.
		opts := IntegrationOptions{OptInEnv: testOptInEnv, MakeTarget: testMakeTarget}
		checkSettings(t, opts, completeEnv(testOptInEnv), missingKey(ActionFail))
	})

	t.Run("out_of_range_missing_key_fails", func(t *testing.T) {
		opts := IntegrationOptions{OptInEnv: testOptInEnv, MakeTarget: testMakeTarget, MissingKey: MissingKeySkip + 1}
		checkSettings(t, opts, completeEnv(testOptInEnv), missingKey(ActionFail))
	})

	// Another test's opt-in, even with everything else in place, must not run
	// this one: an implementation that accepts either opt-in would arm a
	// charged test the user did not ask for.
	t.Run("other_opt_in_does_not_run", func(t *testing.T) {
		opts := IntegrationOptions{OptInEnv: testOptInEnv, MakeTarget: testMakeTarget}
		checkSettings(t, opts, completeEnv(DeepSeekOptInEnv), []settingsCase{
			{name: "deepseek_opt_in_only", wantAction: ActionSkip, wantReason: optInReason},
		})
	})
}
