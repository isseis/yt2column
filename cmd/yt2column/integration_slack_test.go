//go:build integration

package main

import (
	"bytes"
	"context"
	"os"
	"testing"

	deepseektestutil "github.com/isseis/yt2column/internal/llm/deepseek/testutil"
	publishertestutil "github.com/isseis/yt2column/internal/publisher/testutil"
)

// TestIntegrationCLISlack runs the CLI, assembled exactly as main assembles
// it, from a seeded transcript cache through the real DeepSeek API to the
// real test Webhook (a Mattermost test channel) with --slack. yt-dlp is a
// tripwire and is never started. It is excluded from `make test` by its build
// tag and is run by `make test-integration-cli-slack`, which sets the opt-in
// variable and the model name. It calls run once and makes one Generate call.
// Nothing the run wrote is printed until it is shown to hold neither the API
// key, nor the Webhook URL, nor the last eight characters of either, and the
// failure messages name the place only.
func TestIntegrationCLISlack(t *testing.T) {
	gateCLISlackIntegration(t, os.Getenv, func(llmSettings deepseektestutil.IntegrationSettings, webhookSettings publishertestutil.IntegrationSettings) {
		runIntegrationCLISlack(t, llmSettings, webhookSettings)
	})
}

// runIntegrationCLISlack is the body of TestIntegrationCLISlack once the
// environment says it runs.
func runIntegrationCLISlack(t *testing.T, llmSettings deepseektestutil.IntegrationSettings, webhookSettings publishertestutil.IntegrationSettings) {
	webhookURL, err := webhookSettings.WebhookURL.Reveal()
	if err != nil {
		t.Fatal("the test Webhook URL cannot be revealed")
	}
	r := newIntegrationRun(t, llmSettings)
	// The test Webhook URL is given to run as SLACK_WEBHOOK_URL in the
	// environment run sees; the process environment is not changed.
	r.env["SLACK_WEBHOOK_URL"] = webhookURL

	var stdout, stderr bytes.Buffer
	code := run(context.Background(), []string{"--slack", integrationVideoURL}, lookupFrom(r.env), &stdout, &stderr, r.deps)

	requireNoSecrets(t, []string{r.apiKey, webhookURL}, []outputPlace{
		{"standard output", stdout.String()},
		{"standard error", stderr.String()},
	})
	r.checkRun(t, code)
	if code != 0 {
		// The output is shown to be free of the secrets above, and run
		// escapes the untrusted text it writes, so it can explain the failure.
		t.Logf("standard error:\n%s", stderr.String())
	}
}
