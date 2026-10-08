//go:build test

package publishertestutil

import (
	"maps"
	"slices"
	"strings"
	"testing"
)

const (
	testWebhookURL       = "https://mattermost.example.com/hooks/testhookkey0123456789TAIL8TST"
	testProductionURL    = "https://hooks.slack.com/services/T000/B000/productionkeyPRODTAIL"
	testOptInEnv         = "YT2COLUMN_EXAMPLE_INTEGRATION"
	testMakeTarget       = "test-integration-example"
	productionWebhookEnv = "SLACK_WEBHOOK_URL"
)

// TestSlackSettingsFrom checks the decision a webhook integration test makes
// in each environment: it runs only with its own opt-in set to exactly 1,
// fails rather than skips without a valid test Webhook URL even when the
// production one is set, and fails on an HTTP/2 debug setting. The URL and
// GODEBUG rules themselves belong to slackwebhook.ValidURL and
// config.HTTP2DebugEnabledIn and are tested there; one row each shows they
// are applied. The reason never holds a URL or its last eight characters, and
// only a run carries the URL.
func TestSlackSettingsFrom(t *testing.T) {
	opts := IntegrationOptions{OptInEnv: testOptInEnv, MakeTarget: testMakeTarget}
	// The production URL is set so a read of it would be visible.
	complete := map[string]string{
		testOptInEnv:         OptInValue,
		WebhookURLEnv:        testWebhookURL,
		productionWebhookEnv: testProductionURL,
	}
	invalidURL := "http://mattermost.example.com/hooks/invalidkeyINVTAIL8"
	forbidden := []string{}
	for _, url := range []string{testWebhookURL, testProductionURL, invalidURL} {
		forbidden = append(forbidden, url, url[len(url)-8:])
	}
	optInReason := []string{testOptInEnv, "make " + testMakeTarget}

	for _, tc := range []struct {
		name       string
		change     map[string]string
		wantAction IntegrationAction
		wantReason []string
	}{
		{name: "opt_in_unset", change: map[string]string{testOptInEnv: ""}, wantAction: ActionSkip, wantReason: optInReason},
		{name: "opt_in_zero", change: map[string]string{testOptInEnv: "0"}, wantAction: ActionSkip, wantReason: optInReason},
		{name: "opt_in_true", change: map[string]string{testOptInEnv: "true"}, wantAction: ActionSkip, wantReason: optInReason},
		{name: "opt_in_padded", change: map[string]string{testOptInEnv: " 1"}, wantAction: ActionSkip, wantReason: optInReason},
		{name: "other_opt_in_only", change: map[string]string{testOptInEnv: "", CLISlackOptInEnv: OptInValue}, wantAction: ActionSkip, wantReason: optInReason},
		{name: "opt_in_checked_before_url", change: map[string]string{testOptInEnv: "", WebhookURLEnv: ""}, wantAction: ActionSkip, wantReason: optInReason},
		{name: "url_missing_with_production_url_set", change: map[string]string{WebhookURLEnv: ""}, wantAction: ActionFail, wantReason: []string{WebhookURLEnv, productionWebhookEnv + " is not used"}},
		{name: "url_checked_before_godebug", change: map[string]string{WebhookURLEnv: "", godebugEnv: "http2debug=1"}, wantAction: ActionFail, wantReason: []string{WebhookURLEnv + " is not set"}},
		{name: "url_not_https", change: map[string]string{WebhookURLEnv: invalidURL}, wantAction: ActionFail, wantReason: []string{WebhookURLEnv, "https"}},
		{name: "godebug_http2debug_1", change: map[string]string{godebugEnv: "http2debug=1"}, wantAction: ActionFail, wantReason: []string{godebugEnv}},
		{name: "complete", wantAction: ActionRun},
	} {
		t.Run(tc.name, func(t *testing.T) {
			env := maps.Clone(complete)
			maps.Copy(env, tc.change)
			read := []string{}
			settings := SettingsFrom(func(name string) string {
				read = append(read, name)
				return env[name]
			}, opts)
			if slices.Contains(read, productionWebhookEnv) {
				t.Errorf("SettingsFrom read %s; it must use only %s", productionWebhookEnv, WebhookURLEnv)
			}
			if settings.Action != tc.wantAction {
				t.Fatalf("Action = %d, want %d (reason %q)", settings.Action, tc.wantAction, settings.Reason)
			}
			for _, want := range tc.wantReason {
				if !strings.Contains(settings.Reason, want) {
					t.Errorf("Reason %q does not mention %q", settings.Reason, want)
				}
			}
			for _, s := range forbidden {
				if strings.Contains(settings.Reason, s) {
					t.Errorf("Reason %q contains a webhook URL or its last eight characters", settings.Reason)
				}
			}
			got, err := settings.WebhookURL.Reveal()
			if tc.wantAction != ActionRun {
				if err == nil {
					t.Errorf("settings for %d carry a webhook URL", settings.Action)
				}
				return
			}
			if err != nil || got != testWebhookURL {
				t.Errorf("WebhookURL is not the value of %s (Reveal error = %v)", WebhookURLEnv, err)
			}
		})
	}
}
