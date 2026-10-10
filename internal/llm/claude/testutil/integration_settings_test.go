//go:build test

package claudetestutil

import (
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/isseis/yt2column/internal/llm/claudeparam"
)

const (
	testIntegrationKey       = "test-integration-key-0123456789"
	testProductionKey        = "test-production-key-0123456789"
	testIntegrationModel     = "claude-custom"
	testIntegrationEffort    = "medium"
	testIntegrationWorkspace = "wrkspc_integrationTESTWS01"
	testProductionWorkspace  = "wrkspc_productionPRODWS01"
	testOptInEnv             = "YT2COLUMN_EXAMPLE_INTEGRATION"
	testMakeTarget           = "test-integration-example"

	productionKeyEnv       = "ANTHROPIC_API_KEY" //nolint:gosec // the name of the production key variable, not a key
	productionWorkspaceEnv = "ANTHROPIC_WORKSPACE_ID"
)

// completeEnv is an environment in which the test runs with a workspace ID.
// The production key and workspace ID are set so a read of either would be
// visible.
func completeEnv() map[string]string {
	return map[string]string{
		testOptInEnv:           OptInValue,
		APIKeyEnv:              testIntegrationKey,
		ModelEnv:               testIntegrationModel,
		EffortEnv:              testIntegrationEffort,
		WorkspaceIDEnv:         testIntegrationWorkspace,
		productionKeyEnv:       testProductionKey,
		productionWorkspaceEnv: testProductionWorkspace,
	}
}

// TestIntegrationSettings checks the decision SettingsFrom makes in each
// environment: the action, that the reason names the variable behind it and
// holds neither key, and, when the test runs, every setting it carries. The
// production variables are set throughout and must never be read.
func TestIntegrationSettings(t *testing.T) {
	optInReason := []string{testOptInEnv, "make " + testMakeTarget}
	for _, tc := range []struct {
		name       string
		change     map[string]string
		unset      []string
		wantAction IntegrationAction
		wantReason []string
		// wantWorkspace is the workspace ID a run carries; empty means none.
		wantWorkspace string
	}{
		{name: "opt_in_unset", unset: []string{testOptInEnv}, wantAction: ActionSkip, wantReason: optInReason},
		{name: "opt_in_empty", change: map[string]string{testOptInEnv: ""}, wantAction: ActionSkip, wantReason: optInReason},
		{name: "opt_in_zero", change: map[string]string{testOptInEnv: "0"}, wantAction: ActionSkip, wantReason: optInReason},
		{name: "opt_in_true", change: map[string]string{testOptInEnv: "true"}, wantAction: ActionSkip, wantReason: optInReason},
		{name: "opt_in_padded", change: map[string]string{testOptInEnv: " 1"}, wantAction: ActionSkip, wantReason: optInReason},
		{name: "opt_in_checked_before_api_key", unset: []string{testOptInEnv, APIKeyEnv}, wantAction: ActionSkip, wantReason: optInReason},
		{name: "opt_in_checked_before_godebug", unset: []string{testOptInEnv}, change: map[string]string{GODEBUGEnv: "http2debug=1"}, wantAction: ActionSkip, wantReason: optInReason},
		{name: "api_key_unset_with_production_key_set", unset: []string{APIKeyEnv}, wantAction: ActionSkip, wantReason: []string{APIKeyEnv}},
		{name: "api_key_empty", change: map[string]string{APIKeyEnv: ""}, wantAction: ActionSkip, wantReason: []string{APIKeyEnv}},
		{name: "api_key_checked_before_model", unset: []string{APIKeyEnv, ModelEnv}, wantAction: ActionSkip, wantReason: []string{APIKeyEnv}},
		{name: "api_key_checked_before_godebug", unset: []string{APIKeyEnv}, change: map[string]string{GODEBUGEnv: "http2debug=2"}, wantAction: ActionSkip, wantReason: []string{APIKeyEnv}},
		{name: "model_unset", unset: []string{ModelEnv}, wantAction: ActionFail, wantReason: []string{ModelEnv, "make " + testMakeTarget}},
		{name: "model_empty", change: map[string]string{ModelEnv: ""}, wantAction: ActionFail, wantReason: []string{ModelEnv}},
		{name: "model_checked_before_effort", unset: []string{ModelEnv, EffortEnv}, wantAction: ActionFail, wantReason: []string{ModelEnv}},
		{name: "effort_unset", unset: []string{EffortEnv}, wantAction: ActionFail, wantReason: []string{EffortEnv, "make " + testMakeTarget}},
		{name: "effort_empty", change: map[string]string{EffortEnv: ""}, wantAction: ActionFail, wantReason: []string{EffortEnv}},
		{name: "effort_unknown", change: map[string]string{EffortEnv: "maximum"}, wantAction: ActionFail, wantReason: []string{EffortEnv}},
		{name: "effort_upper_case", change: map[string]string{EffortEnv: "LOW"}, wantAction: ActionFail, wantReason: []string{EffortEnv}},
		{name: "effort_checked_before_workspace", change: map[string]string{EffortEnv: "maximum", WorkspaceIDEnv: ""}, wantAction: ActionFail, wantReason: []string{EffortEnv}},
		{name: "workspace_empty", change: map[string]string{WorkspaceIDEnv: ""}, wantAction: ActionFail, wantReason: []string{WorkspaceIDEnv}},
		{name: "workspace_with_space", change: map[string]string{WorkspaceIDEnv: "wrkspc bad"}, wantAction: ActionFail, wantReason: []string{WorkspaceIDEnv}},
		{name: "workspace_non_ascii", change: map[string]string{WorkspaceIDEnv: "wrkspc_\u00e9"}, wantAction: ActionFail, wantReason: []string{WorkspaceIDEnv}},
		{name: "workspace_checked_before_godebug", change: map[string]string{WorkspaceIDEnv: "", GODEBUGEnv: "http2debug=1"}, wantAction: ActionFail, wantReason: []string{WorkspaceIDEnv}},
		{name: "workspace_unset_specifies_none_with_production_set", unset: []string{WorkspaceIDEnv}, wantAction: ActionRun},
		{name: "godebug_http2debug_1", change: map[string]string{GODEBUGEnv: "http2debug=1"}, wantAction: ActionFail, wantReason: []string{GODEBUGEnv, "http2debug"}},
		{name: "godebug_http2debug_2_among_others", change: map[string]string{GODEBUGEnv: "gctrace=1,http2debug=2"}, wantAction: ActionFail, wantReason: []string{GODEBUGEnv, "http2debug"}},
		{name: "godebug_unrelated", change: map[string]string{GODEBUGEnv: "http2client=0"}, wantAction: ActionRun, wantWorkspace: testIntegrationWorkspace},
		{name: "complete", wantAction: ActionRun, wantWorkspace: testIntegrationWorkspace},
	} {
		t.Run(tc.name, func(t *testing.T) {
			env := completeEnv()
			maps.Copy(env, tc.change)
			for _, name := range tc.unset {
				delete(env, name)
			}
			var read []string
			settings := SettingsFrom(func(name string) (string, bool) {
				read = append(read, name)
				value, ok := env[name]
				return value, ok
			}, IntegrationOptions{OptInEnv: testOptInEnv, MakeTarget: testMakeTarget})

			for _, name := range []string{productionKeyEnv, productionWorkspaceEnv} {
				if slices.Contains(read, name) {
					t.Errorf("SettingsFrom read %s; it must use only the test variables", name)
				}
			}
			if settings.Action != tc.wantAction {
				t.Fatalf("Action = %d, want %d (reason %q)", settings.Action, tc.wantAction, settings.Reason)
			}
			for _, want := range tc.wantReason {
				if !strings.Contains(settings.Reason, want) {
					t.Errorf("Reason %q does not mention %q", settings.Reason, want)
				}
			}
			for _, key := range []string{testIntegrationKey, testIntegrationKey[len(testIntegrationKey)-8:], testProductionKey, testProductionKey[len(testProductionKey)-8:]} {
				if strings.Contains(settings.Reason, key) {
					t.Errorf("Reason %q contains an API key or its last eight characters", settings.Reason)
				}
			}
			if tc.wantAction != ActionRun {
				_, keyErr := settings.APIKey.Reveal()
				_, specified := settings.WorkspaceID.Value()
				if keyErr == nil || settings.Model != "" || settings.Effort != claudeparam.EffortUnset || specified {
					t.Errorf("settings for %d carry values beyond the action and reason", settings.Action)
				}
				return
			}
			if settings.Model != testIntegrationModel {
				t.Errorf("Model = %q, want %q", settings.Model, testIntegrationModel)
			}
			if settings.Effort != claudeparam.EffortMedium {
				t.Errorf("Effort = %s, want %s", settings.Effort, claudeparam.EffortMedium)
			}
			if key, err := settings.APIKey.Reveal(); err != nil || key != testIntegrationKey {
				t.Errorf("APIKey is not the value of %s (Reveal error = %v)", APIKeyEnv, err)
			}
			workspace, specified := settings.WorkspaceID.Value()
			if workspace != tc.wantWorkspace || specified != (tc.wantWorkspace != "") {
				t.Errorf("WorkspaceID = %q (specified %t), want %q", workspace, specified, tc.wantWorkspace)
			}
		})
	}
}
