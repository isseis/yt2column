//go:build test

package publisher_test

import (
	"slices"
	"strings"
	"testing"
	"time"

	deepseektestutil "github.com/isseis/yt2column/internal/llm/deepseek/testutil"
	"github.com/isseis/yt2column/internal/publisher"
	publishertestutil "github.com/isseis/yt2column/internal/publisher/testutil"
	"github.com/isseis/yt2column/internal/writer"
)

// repositoryRoot is where the Makefile lives, relative to this package.
const repositoryRoot = "../.."

// TestMakeTestIntegrationSlack checks the target that runs the
// SlackWebhookPublisher integration test: the go test arguments, a -timeout
// above the longest the two fixed articles can take to post, its own opt-in
// and not the CLI's, no model name, and a notice that it posts to the real
// Webhook without the DeepSeek charge notice, since it calls no LLM.
func TestMakeTestIntegrationSlack(t *testing.T) {
	const timeoutPlaceholder = "<timeout>"
	wantArgs := []string{"-tags", "integration", "-count=1", "-timeout", timeoutPlaceholder, "-v", "./internal/publisher"}

	single, split := publishertestutil.IntegrationArticles(time.Now())
	var maxDuration time.Duration
	for _, article := range []writer.Article{single, split} {
		messages, err := publisher.SlackMessagesForTest(article)
		if err != nil {
			t.Fatalf("preparation of %q error = %v", article.Title, err)
		}
		maxDuration += publisher.SlackPublishMaxDurationForTest(len(messages))
	}

	output, invocation := deepseektestutil.RunMakeTarget(t, repositoryRoot, publishertestutil.SlackIntegrationOptions.MakeTarget, nil,
		publishertestutil.SlackOptInEnv, publishertestutil.CLISlackOptInEnv)
	if !strings.Contains(output, "posts to the real test Webhook") {
		t.Errorf("make output %q does not say that the target posts to the real test Webhook", output)
	}
	if strings.Contains(output, "incurs charges") {
		t.Errorf("make output %q carries the DeepSeek charge notice; the target calls no LLM", output)
	}
	args := slices.Clone(invocation.Args)
	if i := slices.Index(args, "-timeout"); i >= 0 && i+1 < len(args) {
		timeout, err := time.ParseDuration(args[i+1])
		if err != nil || timeout <= maxDuration {
			t.Errorf("-timeout %q (parse error %v), want a duration above %s", args[i+1], err, maxDuration)
		}
		args[i+1] = timeoutPlaceholder
	}
	if !slices.Equal(args, wantArgs) {
		t.Errorf("GOTEST arguments = %q, want %q", invocation.Args, wantArgs)
	}
	optIn := publishertestutil.SlackIntegrationOptions.OptInEnv
	if got, ok := invocation.Env[optIn]; !ok || got != publishertestutil.OptInValue {
		t.Errorf("%s = %q (set %t), want %q", optIn, got, ok, publishertestutil.OptInValue)
	}
	for _, unset := range []string{publishertestutil.CLISlackOptInEnv, deepseektestutil.ModelEnv} {
		if value, ok := invocation.Env[unset]; ok {
			t.Errorf("make %s exported %s=%q", publishertestutil.SlackIntegrationOptions.MakeTarget, unset, value)
		}
	}
}

// TestSlackIntegrationTestBuildTag pins the build tag that keeps the
// integration test, which posts to the real Webhook, out of `make test` while
// `make lint` still compiles it through `go vet -tags integration`.
func TestSlackIntegrationTestBuildTag(t *testing.T) {
	if err := deepseektestutil.FirstLineIs("slack_integration_test.go", "//go:build integration"); err != nil {
		t.Errorf("slack_integration_test.go must start with %q: %v", "//go:build integration", err)
	}
}
