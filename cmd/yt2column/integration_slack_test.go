//go:build integration

package main

import (
	"bytes"
	"context"
	"os"
	"regexp"
	"slices"
	"testing"

	deepseektestutil "github.com/isseis/yt2column/internal/llm/deepseek/testutil"
	publishertestutil "github.com/isseis/yt2column/internal/publisher/testutil"
	"github.com/isseis/yt2column/internal/slackwebhook"
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

	// Every part of the URL the CLI itself redacts is forbidden, not only
	// the whole URL and its tail.
	requireNoSecrets(t, slices.Concat(secretTails(r.apiKey), slackwebhook.SensitiveParts(webhookURL)), []outputPlace{
		{"standard output", stdout.String()},
		{"standard error", stderr.String()},
	})
	r.checkRun(t, code)
	// The output is shown to be free of the secrets above, and run escapes
	// the untrusted text it writes, so it can be logged: on failure to
	// explain it, on success to record the message count.
	summary := webhookSummaryPattern.FindString(stderr.String())
	if summary == "" {
		t.Errorf("standard error does not report a post to the webhook with a message count")
	}
	if code != 0 || summary == "" {
		t.Logf("standard error:\n%s", stderr.String())
		return
	}
	t.Log(summary)
}

// webhookSummaryPattern matches the line run writes after posting to the
// webhook, which the --out path never writes.
var webhookSummaryPattern = regexp.MustCompile(`posted the article to the webhook in [0-9]+ messages?`)
