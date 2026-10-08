//go:build test || integration

package publishertestutil

import (
	"fmt"
	"time"
	"unicode/utf8"

	"github.com/isseis/yt2column/internal/config"
	"github.com/isseis/yt2column/internal/secret"
	"github.com/isseis/yt2column/internal/slackwebhook"
	"github.com/isseis/yt2column/internal/writer"
)

// Environment variables read by the webhook integration tests. The production
// SLACK_WEBHOOK_URL is never read: the tests post only to the test Webhook.
const (
	// SlackOptInEnv opts in to the SlackWebhookPublisher integration test;
	// `make test-integration-slack` exports it.
	SlackOptInEnv = "YT2COLUMN_SLACK_INTEGRATION"
	// CLISlackOptInEnv opts in to the CLI's webhook integration test;
	// `make test-integration-cli-slack` exports it.
	CLISlackOptInEnv = "YT2COLUMN_CLI_SLACK_INTEGRATION"
	// WebhookURLEnv holds the Incoming Webhook URL of a test channel.
	WebhookURLEnv = "YT2COLUMN_TEST_SLACK_WEBHOOK_URL"

	// OptInValue is the only opt-in value that runs a test.
	OptInValue = "1"

	godebugEnv = "GODEBUG"
)

// IntegrationOptions names the opt-in variable and the make target shown in
// the skip reason of one integration test.
type IntegrationOptions struct {
	OptInEnv   string
	MakeTarget string
}

// The options of the two webhook integration tests. The integration tests and
// the tests of their make targets both read them, so each opt-in and target
// name is written once.
var (
	SlackIntegrationOptions = IntegrationOptions{
		OptInEnv:   SlackOptInEnv,
		MakeTarget: "test-integration-slack",
	}
	CLISlackIntegrationOptions = IntegrationOptions{
		OptInEnv:   CLISlackOptInEnv,
		MakeTarget: "test-integration-cli-slack",
	}
)

// IntegrationAction is what the test does. The zero value skips, so a
// decision that was never made posts nothing.
type IntegrationAction int

// The integration actions.
const (
	ActionSkip IntegrationAction = iota
	ActionFail
	ActionRun
)

// IntegrationSettings is the outcome. Reason never holds the URL or any part
// of it; WebhookURL is set only when Action is ActionRun.
type IntegrationSettings struct {
	Action     IntegrationAction
	Reason     string
	WebhookURL secret.Secret
}

// SettingsFrom decides whether an integration test that posts to the real
// test Webhook runs. The checks run in a fixed order: an opt-in that is not
// exactly OptInValue skips (so a plain `go test -tags integration` or an IDE
// run never posts), a missing test Webhook URL fails, a URL that
// slackwebhook.ValidURL rejects fails, and an HTTP/2 debug setting that would
// print the URL path fails.
func SettingsFrom(getenv func(string) string, opts IntegrationOptions) IntegrationSettings {
	if getenv(opts.OptInEnv) != OptInValue {
		return IntegrationSettings{
			Action: ActionSkip,
			Reason: opts.OptInEnv + " is not " + OptInValue +
				": the integration test posts to the real test Webhook; run it with `make " + opts.MakeTarget + "`",
		}
	}
	value := getenv(WebhookURLEnv)
	if value == "" {
		return IntegrationSettings{
			Action: ActionFail,
			Reason: WebhookURLEnv + " is not set: set it to the Incoming Webhook URL of a test channel (SLACK_WEBHOOK_URL is not used)",
		}
	}
	if !slackwebhook.ValidURL(value) {
		return IntegrationSettings{
			Action: ActionFail,
			Reason: WebhookURLEnv + " is not an https URL with a host",
		}
	}
	if config.HTTP2DebugEnabledIn(getenv(godebugEnv)) {
		return IntegrationSettings{
			Action: ActionFail,
			Reason: godebugEnv + " contains http2debug=1 or http2debug=2, which would print the webhook URL path to standard error: unset it before running the integration test",
		}
	}
	webhookURL, err := secret.New(value)
	if err != nil {
		// Unreachable: secret.New rejects only the empty value, which is
		// handled above. Kept so an error is never dropped.
		return IntegrationSettings{Action: ActionFail, Reason: WebhookURLEnv + " cannot be held as a secret"}
	}
	return IntegrationSettings{Action: ActionRun, WebhookURL: webhookURL}
}

const (
	// integrationMessageRunes is the per-message limit of SlackWebhookPublisher
	// in code points; the single-message article is rendered to exactly this
	// length. TestIntegrationArticles pins it to the publisher's limit.
	integrationMessageRunes = 16383
	// integrationSplitBodyRunes sizes the split article's body so it becomes
	// several messages but stays far below the limit of ten.
	integrationSplitBodyRunes = 2*integrationMessageRunes + 5000
	// integrationMarkerLayout formats the run marker. Every field is
	// zero-padded, so the marker, and with it the title, has a fixed length.
	integrationMarkerLayout = "20060102T150405.000Z"

	integrationModel        = "yt2column-integration-fixed-article"
	integrationModelVersion = "fixed"
	integrationSourceURL    = "https://example.com/yt2column-integration"
)

// IntegrationArticles returns the two fixed articles the webhook integration
// test posts. single renders to exactly the per-message limit, so it is sent
// as one message of maximal length; split renders to more than twice the
// limit, so it is sent as several. Both contain Japanese and characters above
// U+FFFF, so the server's counting in code points is exercised, and neither
// contains mention syntax. Each title holds a fixed-length marker made from at,
// so a run's posts can be told apart in the test channel.
func IntegrationArticles(at time.Time) (single, split writer.Article) {
	marker := at.UTC().Format(integrationMarkerLayout)
	single = integrationArticle("yt2column integration test " + marker + " (single message)")
	single.Body = integrationBody(integrationMessageRunes - utf8.RuneCountInString(integrationHeader(single)))
	split = integrationArticle("yt2column integration test " + marker + " (split)")
	split.Body = integrationBody(integrationSplitBodyRunes)
	return single, split
}

// integrationArticle returns an article with title and the fixed metadata,
// and no body yet.
func integrationArticle(title string) writer.Article {
	return writer.Article{
		Title:        title,
		SourceURL:    integrationSourceURL,
		Model:        integrationModel,
		ModelVersion: integrationModelVersion,
	}
}

// integrationHeader is the text SlackWebhookPublisher puts before the body of
// a, mirroring the unexported publisher.renderArticle for a non-empty
// ModelVersion. TestIntegrationArticles fails if the two drift apart.
func integrationHeader(a writer.Article) string {
	return fmt.Sprintf("# %s\n\n- Model: %s\n- Model version: %s\n\n", a.Title, a.Model, a.ModelVersion)
}

// integrationBody returns a Markdown body of exactly n code points: a heading
// followed by numbered lines, cut at n.
func integrationBody(n int) string {
	runes := []rune("## Integration test body\n\n")
	for i := 1; len(runes) < n; i++ {
		// "Japanese sentence with U+1F600 and U+20BB7", in Japanese.
		runes = append(runes, []rune(fmt.Sprintf("- Line %04d: \u65e5\u672c\u8a9e\u306e\u6587\u3068 \U0001F600 \U00020BB7 \u3092\u542b\u3080\u884c\u3067\u3059\u3002\n", i))...)
	}
	return string(runes[:n])
}
