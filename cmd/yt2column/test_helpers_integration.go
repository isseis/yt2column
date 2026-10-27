//go:build test || integration

package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/isseis/yt2column/internal/config"
	"github.com/isseis/yt2column/internal/llm"
	deepseektestutil "github.com/isseis/yt2column/internal/llm/deepseek/testutil"
	"github.com/isseis/yt2column/internal/nilcheck"
	publishertestutil "github.com/isseis/yt2column/internal/publisher/testutil"
	"github.com/isseis/yt2column/internal/transcript"
	transcripttestutil "github.com/isseis/yt2column/internal/transcript/testutil"
)

const (
	integrationVideoID = "2tcCWM-sRBw"

	// integrationTestdata is the repository's testdata directory, which holds
	// the video's subtitles and info.json.
	integrationTestdata = "../../testdata"
)

// noStepsArgs disables both post-generation steps. A test that does not
// exercise the steps puts these first, so enabling the steps by default does
// not change what the run does. Shared by the test and integration builds.
var noStepsArgs = []string{"--no-shorten", "--no-review"}

// cliIntegrationOptions is how TestIntegrationCLI decides whether it runs. It
// has its own opt-in, matching its own make target, and a missing test API
// key fails it instead of skipping, so an opted-in run cannot pass without
// calling the API.
var cliIntegrationOptions = deepseektestutil.IntegrationOptions{
	OptInEnv:   deepseektestutil.CLIOptInEnv,
	MakeTarget: "test-integration-cli",
	MissingKey: deepseektestutil.MissingKeyFail,
}

// gateCLIIntegration reads the environment through getenv and calls body only
// when the CLI integration test runs. Otherwise it skips or fails t with the
// reason, which never contains the API key, and body, which calls run and the
// real LLM client, is never reached.
func gateCLIIntegration(t testing.TB, getenv func(string) string, body func(deepseektestutil.IntegrationSettings)) {
	t.Helper()
	settings := deepseektestutil.SettingsFrom(getenv, cliIntegrationOptions)
	switch settings.Action {
	case deepseektestutil.ActionRun:
		body(settings)
	case deepseektestutil.ActionFail:
		t.Fatal(settings.Reason)
	default:
		t.Skip(settings.Reason)
	}
}

// cliSlackDeepSeekOptions is how TestIntegrationCLISlack decides whether its
// DeepSeek part runs. It shares the webhook test's opt-in and target, and a
// missing test API key fails it, as for TestIntegrationCLI.
var cliSlackDeepSeekOptions = deepseektestutil.IntegrationOptions{
	OptInEnv:   publishertestutil.CLISlackIntegrationOptions.OptInEnv,
	MakeTarget: publishertestutil.CLISlackIntegrationOptions.MakeTarget,
	MissingKey: deepseektestutil.MissingKeyFail,
}

// gateCLISlackIntegration reads the environment through getenv and calls body
// only when both the DeepSeek decision and the webhook decision say the CLI
// webhook integration test runs. Otherwise it skips or fails t with the first
// reason, which never contains the API key or the Webhook URL, and body,
// which calls run, the real LLM client, and the real Webhook, is never
// reached.
func gateCLISlackIntegration(t testing.TB, getenv func(string) string, body func(deepseektestutil.IntegrationSettings, publishertestutil.IntegrationSettings)) {
	t.Helper()
	llmSettings := deepseektestutil.SettingsFrom(getenv, cliSlackDeepSeekOptions)
	switch llmSettings.Action {
	case deepseektestutil.ActionRun:
	case deepseektestutil.ActionFail:
		t.Fatal(llmSettings.Reason)
		return
	default:
		t.Skip(llmSettings.Reason)
		return
	}
	webhookSettings := publishertestutil.SettingsFrom(getenv, publishertestutil.CLISlackIntegrationOptions)
	switch webhookSettings.Action {
	case publishertestutil.ActionRun:
		body(llmSettings, webhookSettings)
	case publishertestutil.ActionFail:
		t.Fatal(webhookSettings.Reason)
	default:
		t.Skip(webhookSettings.Reason)
	}
}

// lookupFrom returns a config.LookupFunc over env, so a test supplies the
// whole environment run sees without changing the process environment. A
// name absent from env is unset.
func lookupFrom(env map[string]string) config.LookupFunc {
	return func(name string) (string, bool) {
		value, ok := env[name]
		return value, ok
	}
}

// integrationRun is one CLI run assembled for a test of the CLI: a seeded
// transcript cache, a yt-dlp tripwire, the environment run sees, and
// productionDeps() with the Generate calls counted.
type integrationRun struct {
	base           string
	cacheDir       string
	tripwireMarker string
	apiKey         string
	env            map[string]string
	deps           deps
	counter        *generateCounter
}

// newIntegrationRun seeds the cache with the video's testdata, installs the
// tripwire, and builds the environment and the deps for an integration test or
// the provider-selection test. The environment is built here and the process
// environment is not changed; the test API key is given to run as
// DEEPSEEK_API_KEY. The environment sets YT2COLUMN_LLM to deepseek explicitly,
// so the integration tests call DeepSeek rather than the default provider. A
// caller adds the variables its run needs.
func newIntegrationRun(t *testing.T, settings deepseektestutil.IntegrationSettings) integrationRun {
	t.Helper()
	key, err := settings.APIKey.Reveal()
	if err != nil {
		t.Fatal("the test API key cannot be revealed")
	}

	base := t.TempDir()
	cacheDir := filepath.Join(base, "cache")
	if err := os.Mkdir(cacheDir, 0o700); err != nil {
		t.Fatalf("create cache dir: %v", err)
	}
	subtitles, err := os.ReadFile(filepath.Join(integrationTestdata, integrationVideoID+".ja.json3"))
	if err != nil {
		t.Fatalf("read subtitles: %v", err)
	}
	info, err := os.ReadFile(filepath.Join(integrationTestdata, integrationVideoID+".info.json"))
	if err != nil {
		t.Fatalf("read info.json: %v", err)
	}
	if err := transcript.SeedCacheForTest(cacheDir, integrationVideoID, subtitles, info); err != nil {
		t.Fatalf("SeedCacheForTest: %v", err)
	}
	if len(videoCacheEntries(t, cacheDir)) == 0 {
		t.Fatal("the seeded cache has no entry for the video, so its removal cannot be checked")
	}
	binDir := filepath.Join(base, "bin")
	if err := os.Mkdir(binDir, 0o700); err != nil {
		t.Fatalf("create bin dir: %v", err)
	}
	tripwire, tripwireMarker := transcripttestutil.NewTripwire(t, binDir)

	env := map[string]string{
		"YT2COLUMN_LLM":        "deepseek/" + settings.Model,
		"DEEPSEEK_API_KEY":     key,
		"YT2COLUMN_CACHE_DIR":  cacheDir,
		"YT2COLUMN_YTDLP_PATH": tripwire,
	}
	// The real client is wrapped from the outside only, so the assembly stays
	// productionDeps() and the Generate calls can be counted.
	counter := &generateCounter{}
	d := productionDeps()
	newLLMClient := d.newLLMClient
	d.newLLMClient = func(cfg config.LLMConfig) (llm.LLMClient, error) {
		client, err := newLLMClient(cfg)
		return counter.wrap(client), err
	}
	// The review client is counted too, so the Generate count proves the steps
	// did not run: with them disabled only the generation client is built.
	newReviewLLMClient := d.newReviewLLMClient
	d.newReviewLLMClient = func(cfg config.LLMConfig) (llm.LLMClient, error) {
		client, err := newReviewLLMClient(cfg)
		return counter.wrap(client), err
	}
	return integrationRun{
		base:           base,
		cacheDir:       cacheDir,
		tripwireMarker: tripwireMarker,
		apiKey:         key,
		env:            env,
		deps:           d,
		counter:        counter,
	}
}

// videoCacheEntries returns the cache directory's entries for the video. It is
// not videoEntries: that helper also tolerates a cache path that is not a
// directory, because a test of this package makes it a regular file.
func videoCacheEntries(t *testing.T, cacheDir string) []string {
	t.Helper()
	var entries []string
	for _, name := range transcripttestutil.ListDir(t, cacheDir) {
		if strings.HasPrefix(name, integrationVideoID+".") {
			entries = append(entries, name)
		}
	}
	return entries
}

// generateCounter counts the Generate calls of every client it wraps.
type generateCounter struct {
	calls atomic.Int64
}

// wrap returns client with its Generate calls counted. A nil client, typed
// or not, is returned as a nil interface, so a failed construction still
// looks failed to the caller.
func (c *generateCounter) wrap(client llm.LLMClient) llm.LLMClient {
	if nilcheck.IsNil(client) {
		return nil
	}
	return &countingClient{inner: client, counter: c}
}

// count returns the number of Generate calls so far.
func (c *generateCounter) count() int64 {
	return c.calls.Load()
}

// countingClient forwards Generate to inner after counting the call.
type countingClient struct {
	inner   llm.LLMClient
	counter *generateCounter
}

// Generate implements llm.LLMClient.
func (c *countingClient) Generate(ctx context.Context, req llm.GenerateRequest) (llm.GenerateResponse, error) {
	c.counter.calls.Add(1)
	return c.inner.Generate(ctx, req)
}
