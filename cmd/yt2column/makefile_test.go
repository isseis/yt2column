//go:build test

package main

import (
	"fmt"
	"io/fs"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/isseis/yt2column/internal/config"
	claudetestutil "github.com/isseis/yt2column/internal/llm/claude/testutil"
	deepseektestutil "github.com/isseis/yt2column/internal/llm/deepseek/testutil"
	"github.com/isseis/yt2column/internal/llm/provider"
	"github.com/isseis/yt2column/internal/publisher"
	publishertestutil "github.com/isseis/yt2column/internal/publisher/testutil"
)

const (
	// repositoryRoot is where the Makefile lives, relative to this package.
	repositoryRoot = "../.."

	// deepSeekIntegrationTarget is the make target that runs the DeepSeek
	// adapter's charged integration test.
	deepSeekIntegrationTarget = "test-integration-deepseek"

	// claudeIntegrationTarget is the make target that runs the Claude
	// adapter's charged integration test.
	claudeIntegrationTarget = "test-integration-claude"

	// transcriptIntegrationTarget is the make target that runs the
	// transcript integration test, which has no opt-in.
	transcriptIntegrationTarget = "test-integration"
)

// retiredLLMEnvs are the retired production LLM variables, among them the
// model variable the DeepSeek integration targets used before the rename. A
// target, or the code it builds, that still names one would take its model
// from a value a user set for the production configuration instead of the
// test's own variable. The names come from the config package, not from a
// copy here.
var retiredLLMEnvs = config.RetiredLLMVarsForTest()

// TestMakeTestIntegrationCLI checks the target that runs the CLI integration
// test. It makes one Generate call, so the -timeout value must exceed
// provider.LLMTimeout.
func TestMakeTestIntegrationCLI(t *testing.T) {
	deepseektestutil.CheckChargedTarget(t, deepseektestutil.ChargedTarget{
		Root:       repositoryRoot,
		Target:     cliIntegrationOptions.MakeTarget,
		OptInEnv:   cliIntegrationOptions.OptInEnv,
		Package:    "./cmd/yt2column",
		MinTimeout: provider.LLMTimeout,
	})
}

// TestMakeTestIntegrationCLISlack checks the target that runs the CLI webhook
// integration test. It makes one Generate call and posts at most the largest
// number of messages one article may become, so the -timeout value must
// exceed provider.LLMTimeout plus the longest those posts can take.
func TestMakeTestIntegrationCLISlack(t *testing.T) {
	deepseektestutil.CheckChargedTarget(t, deepseektestutil.ChargedTarget{
		Root:       repositoryRoot,
		Target:     publishertestutil.CLISlackIntegrationOptions.MakeTarget,
		OptInEnv:   publishertestutil.CLISlackIntegrationOptions.OptInEnv,
		Package:    "./cmd/yt2column",
		MinTimeout: provider.LLMTimeout + publisher.SlackPublishMaxDurationForTest(publisher.SlackMaxMessagesForTest),
	})
}

// TestMakeOptInsAreTargetSpecific pins that each integration target exports
// no opt-in but its own, so running one never arms another's test. That the
// own opt-in is exported is checked by each target's own test.
func TestMakeOptInsAreTargetSpecific(t *testing.T) {
	optIns := []string{
		deepseektestutil.DeepSeekOptInEnv,
		deepseektestutil.CLIOptInEnv,
		claudetestutil.OptInEnv,
		publishertestutil.SlackOptInEnv,
		publishertestutil.CLISlackOptInEnv,
	}
	for _, tc := range []struct {
		target string
		own    string // empty for a target with no opt-in
	}{
		{target: transcriptIntegrationTarget},
		{target: deepSeekIntegrationTarget, own: deepseektestutil.DeepSeekOptInEnv},
		{target: claudeIntegrationTarget, own: claudetestutil.OptInEnv},
		{target: cliIntegrationOptions.MakeTarget, own: deepseektestutil.CLIOptInEnv},
		{target: publishertestutil.SlackIntegrationOptions.MakeTarget, own: publishertestutil.SlackOptInEnv},
		{target: publishertestutil.CLISlackIntegrationOptions.MakeTarget, own: publishertestutil.CLISlackOptInEnv},
	} {
		t.Run(tc.target, func(t *testing.T) {
			_, invocation := deepseektestutil.RunMakeTarget(t, repositoryRoot, tc.target, nil,
				claudetestutil.OptInEnv, publishertestutil.SlackOptInEnv, publishertestutil.CLISlackOptInEnv)
			for _, optIn := range optIns {
				if value, ok := invocation.Env[optIn]; ok && optIn != tc.own {
					t.Errorf("make %s exported %s=%q", tc.target, optIn, value)
				}
			}
		})
	}
}

// TestMakeTargetsDoNotReadRetiredModelEnv checks that the three targets that
// run the DeepSeek integration tests take the model from the test's own
// variable: with only a retired variable set and the test variable undefined,
// each target passes the default to go test.
func TestMakeTargetsDoNotReadRetiredModelEnv(t *testing.T) {
	for _, target := range []string{
		deepSeekIntegrationTarget,
		cliIntegrationOptions.MakeTarget,
		publishertestutil.CLISlackIntegrationOptions.MakeTarget,
	} {
		for _, retired := range retiredLLMEnvs {
			t.Run(target+"/"+retired, func(t *testing.T) {
				deepseektestutil.CheckRetiredModelEnvNotRead(t, repositoryRoot, target, retired)
			})
		}
	}
}

// retiredLLMEnvMentioned reports whether line names one of retiredLLMEnvs as
// a whole variable name. A longer name that only starts or ends with one, such
// as YT2COLUMN_MODEL_OLD, is another variable and allowed.
func retiredLLMEnvMentioned(line string) bool {
	return retiredLLMEnvRE.MatchString(line)
}

// retiredLLMEnvRE matches any of retiredLLMEnvs as a whole variable name.
var retiredLLMEnvRE = func() *regexp.Regexp {
	quoted := make([]string, 0, len(retiredLLMEnvs))
	for _, name := range retiredLLMEnvs {
		quoted = append(quoted, regexp.QuoteMeta(name))
	}
	return regexp.MustCompile(`(^|[^A-Za-z0-9_])(` + strings.Join(quoted, "|") + `)([^A-Za-z0-9_]|$)`)
}()

// TestRetiredLLMEnvMentioned pins the whole-name boundary of the guard's
// matcher: a retired name matches in the forms a Makefile line or a Go source
// line holds it, and a longer name that contains it does not. The boundary is
// shared by every alternative, so it is checked on one name, and each name is
// checked to match.
func TestRetiredLLMEnvMentioned(t *testing.T) {
	retired := retiredLLMEnvs[0]
	for _, tc := range []struct {
		name string
		line string
		want bool
	}{
		{name: "assignment", line: retired + " ?= deepseek-flash", want: true},
		{name: "export", line: "export " + retired + " := $(" + retired + ")", want: true},
		{name: "make_reference", line: "$(" + retired + ")", want: true},
		{name: "shell_reference", line: `"${` + retired + `}"`, want: true},
		{name: "name_starts_with_identifier", line: "DEFAULT_" + retired, want: false},
		{name: "name_ends_with_identifier", line: retired + "_OLD", want: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := retiredLLMEnvMentioned(tc.line); got != tc.want {
				t.Errorf("retiredLLMEnvMentioned(%q) = %t, want %t", tc.line, got, tc.want)
			}
		})
	}
	for _, name := range retiredLLMEnvs {
		if !retiredLLMEnvMentioned("$(" + name + ")") {
			t.Errorf("retiredLLMEnvMentioned does not match %s", name)
		}
	}
}

// TestRetiredModelEnvIsGone checks that the Makefile, where the DeepSeek
// integration targets are defined, the Go code under internal/llm/deepseek
// that they build, and the CLI integration test files of this package do not
// name a retired variable. The whole Makefile is scanned rather than the
// targets' lines, so a mention anywhere in it fails. Every scope must hold at
// least one file to scan, so a scope that finds none is a failure rather than
// a scan of nothing that passes.
func TestRetiredModelEnvIsGone(t *testing.T) {
	for _, scope := range []struct {
		name  string
		files func(t *testing.T) []string
	}{
		{name: "Makefile", files: func(*testing.T) []string { return []string{"Makefile"} }},
		{name: "internal/llm/deepseek", files: func(t *testing.T) []string { return goFilesUnder(t, "internal/llm/deepseek") }},
		{name: "cmd/yt2column integration", files: func(t *testing.T) []string {
			var files []string
			for _, file := range goFilesUnder(t, "cmd/yt2column") {
				if strings.Contains(filepath.Base(file), "integration") {
					files = append(files, file)
				}
			}
			return files
		}},
	} {
		t.Run(scope.name, func(t *testing.T) {
			files := scope.files(t)
			if len(files) == 0 {
				t.Fatalf("found no files in %s to scan", scope.name)
			}
			for _, file := range files {
				content, err := os.ReadFile(filepath.Join(repositoryRoot, file))
				if err != nil {
					t.Fatalf("read %s: %v", file, err)
				}
				for i, line := range slices.Collect(strings.Lines(string(content))) {
					if retiredLLMEnvMentioned(line) {
						t.Errorf("%s:%d names a retired variable: %s", file, i+1, strings.TrimSpace(line))
					}
				}
			}
		})
	}
}

// goFilesUnder returns the paths of the Go files under dir, relative to the
// repository root, in order.
func goFilesUnder(t *testing.T, dir string) []string {
	t.Helper()
	var files []string
	err := filepath.WalkDir(filepath.Join(repositoryRoot, dir), func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}
		rel, err := filepath.Rel(repositoryRoot, path)
		if err != nil {
			return err
		}
		files = append(files, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", dir, err)
	}
	slices.Sort(files)
	return files
}

// gateRecorder is a testing.TB that records Skip and Fatal instead of ending
// the test, so a test can observe what gateCLIIntegration decided and that it
// did not go on to call the body. It embeds a nil testing.TB, so any other
// method the gate might call (Skipf, Fatalf, SkipNow, ...) panics instead of
// reaching the real test and letting a case pass unchecked.
type gateRecorder struct {
	testing.TB
	skipped []string
	failed  []string
}

// Helper is a no-op; the gate calls it first.
func (r *gateRecorder) Helper() {}

// Skip records the reason instead of skipping.
func (r *gateRecorder) Skip(args ...any) {
	r.skipped = append(r.skipped, fmt.Sprint(args...))
}

// Fatal records the reason instead of failing.
func (r *gateRecorder) Fatal(args ...any) {
	r.failed = append(r.failed, fmt.Sprint(args...))
}

// TestCLIIntegrationSettings checks the decision the CLI integration test
// makes in each environment: it runs only with its own opt-in set to exactly
// 1, fails rather than skips without the test API key even when the
// production key is set, and fails on an HTTP/2 debug setting. When it does
// not run, the body that calls run and the LLM client is never reached, and
// the reason holds neither key nor the last eight characters of either.
func TestCLIIntegrationSettings(t *testing.T) {
	const (
		testKey       = "sk-integration-TESTKEYVALUE-0123456789-TAIL8TST"
		productionKey = testAPIKey
		model         = "deepseek-custom"
	)
	// The environment names the CLI opt-in directly, not through
	// cliIntegrationOptions, so a wrong opt-in in the options is caught.
	complete := map[string]string{
		deepseektestutil.CLIOptInEnv: deepseektestutil.OptInValue,
		deepseektestutil.APIKeyEnv:   testKey,
		deepseektestutil.ModelEnv:    model,
		"DEEPSEEK_API_KEY":           productionKey,
	}
	keys := []string{testKey, testKey[len(testKey)-8:], productionKey, productionKey[len(productionKey)-8:]}

	// outcomes counts what the gate did: called the body, skipped, failed.
	type outcomes struct{ ran, skipped, failed int }
	var (
		ran     = outcomes{ran: 1}
		skipped = outcomes{skipped: 1}
		failed  = outcomes{failed: 1}
	)
	for _, tc := range []struct {
		name   string
		change map[string]string
		want   outcomes
	}{
		{name: "opt_in_unset", change: map[string]string{deepseektestutil.CLIOptInEnv: ""}, want: skipped},
		{name: "opt_in_zero", change: map[string]string{deepseektestutil.CLIOptInEnv: "0"}, want: skipped},
		{name: "opt_in_true", change: map[string]string{deepseektestutil.CLIOptInEnv: "true"}, want: skipped},
		{name: "opt_in_padded", change: map[string]string{deepseektestutil.CLIOptInEnv: " 1"}, want: skipped},
		{name: "deepseek_opt_in_only", change: map[string]string{deepseektestutil.CLIOptInEnv: "", deepseektestutil.DeepSeekOptInEnv: deepseektestutil.OptInValue}, want: skipped},
		{name: "test_key_missing_with_production_key_set", change: map[string]string{deepseektestutil.APIKeyEnv: ""}, want: failed},
		{name: "godebug_http2debug_1", change: map[string]string{deepseektestutil.GODEBUGEnv: "http2debug=1"}, want: failed},
		{name: "godebug_http2debug_2_among_others", change: map[string]string{deepseektestutil.GODEBUGEnv: "gctrace=1,http2debug=2"}, want: failed},
		{name: "complete", want: ran},
	} {
		t.Run(tc.name, func(t *testing.T) {
			env := maps.Clone(complete)
			maps.Copy(env, tc.change)
			recorder := &gateRecorder{}
			var bodySettings []deepseektestutil.IntegrationSettings
			gateCLIIntegration(recorder, func(name string) string { return env[name] }, func(s deepseektestutil.IntegrationSettings) {
				bodySettings = append(bodySettings, s)
			})

			got := outcomes{ran: len(bodySettings), skipped: len(recorder.skipped), failed: len(recorder.failed)}
			if got != tc.want {
				t.Fatalf("gate outcomes = %+v, want %+v", got, tc.want)
			}
			for _, reason := range recorder.skipped {
				// A skip must point at the CLI's own opt-in and target, never
				// at the other charged target.
				for _, want := range []string{deepseektestutil.CLIOptInEnv, "make test-integration-cli`"} {
					if !strings.Contains(reason, want) {
						t.Errorf("skip reason %q does not mention %q", reason, want)
					}
				}
			}
			for _, reason := range slices.Concat(recorder.skipped, recorder.failed) {
				for _, key := range keys {
					if strings.Contains(reason, key) {
						t.Error("the reason holds an API key or its last eight characters")
					}
				}
			}
			if tc.want != ran {
				return
			}
			if bodySettings[0].Model != model {
				t.Errorf("Model = %q, want %q", bodySettings[0].Model, model)
			}
			if key, err := bodySettings[0].APIKey.Reveal(); err != nil || key != testKey {
				t.Errorf("APIKey is not the value of %s (Reveal error = %v)", deepseektestutil.APIKeyEnv, err)
			}
		})
	}
}

// TestCLIIntegrationTestBuildTag pins the build tag that keeps the CLI
// integration test, which calls the real DeepSeek API, out of `make test`
// while `make lint` still compiles it through `go vet -tags integration`.
func TestCLIIntegrationTestBuildTag(t *testing.T) {
	if err := deepseektestutil.FirstLineIs("integration_test.go", "//go:build integration"); err != nil {
		t.Errorf("integration_test.go must start with %q: %v", "//go:build integration", err)
	}
}

// TestCLISlackIntegrationSettings checks how the CLI webhook integration test
// combines its two decisions: it runs only with its own opt-in and when both
// the DeepSeek and the webhook decisions run, and a failure of either layer
// (here the missing test API key and the missing test Webhook URL, each with
// the production value set) fails it. Each decision's own rules (opt-in
// values, GODEBUG, URL shape) are tested with that decision. When it does not
// run, the body that calls run, the LLM client, and the Webhook is never
// reached, and the reason holds neither key nor URL nor the last eight
// characters of any.
func TestCLISlackIntegrationSettings(t *testing.T) {
	const (
		testKey           = "sk-integration-TESTKEYVALUE-0123456789-TAIL8TST"
		productionKey     = testAPIKey
		model             = "deepseek-custom"
		testWebhook       = "https://mattermost.example.com/hooks/clislackhookkeyHOOKTAIL"
		productionWebhook = "https://hooks.slack.com/services/T000/B000/prodhookkeyPRODTAIL"
	)
	// The environment names the opt-in directly, not through the options, so
	// a wrong opt-in in the options is caught.
	complete := map[string]string{
		publishertestutil.CLISlackOptInEnv: publishertestutil.OptInValue,
		deepseektestutil.APIKeyEnv:         testKey,
		deepseektestutil.ModelEnv:          model,
		publishertestutil.WebhookURLEnv:    testWebhook,
		"DEEPSEEK_API_KEY":                 productionKey,
		"SLACK_WEBHOOK_URL":                productionWebhook,
	}
	var forbidden []string
	for _, s := range []string{testKey, productionKey, testWebhook, productionWebhook} {
		forbidden = append(forbidden, s, s[len(s)-8:])
	}

	type outcomes struct{ ran, skipped, failed int }
	var (
		ran     = outcomes{ran: 1}
		skipped = outcomes{skipped: 1}
		failed  = outcomes{failed: 1}
	)
	for _, tc := range []struct {
		name   string
		change map[string]string
		want   outcomes
	}{
		{name: "opt_in_unset", change: map[string]string{publishertestutil.CLISlackOptInEnv: ""}, want: skipped},
		{name: "cli_opt_in_only", change: map[string]string{publishertestutil.CLISlackOptInEnv: "", deepseektestutil.CLIOptInEnv: deepseektestutil.OptInValue}, want: skipped},
		{name: "slack_opt_in_only", change: map[string]string{publishertestutil.CLISlackOptInEnv: "", publishertestutil.SlackOptInEnv: publishertestutil.OptInValue}, want: skipped},
		{name: "test_key_missing_with_production_key_set", change: map[string]string{deepseektestutil.APIKeyEnv: ""}, want: failed},
		{name: "test_webhook_missing_with_production_webhook_set", change: map[string]string{publishertestutil.WebhookURLEnv: ""}, want: failed},
		{name: "complete", want: ran},
	} {
		t.Run(tc.name, func(t *testing.T) {
			env := maps.Clone(complete)
			maps.Copy(env, tc.change)
			recorder := &gateRecorder{}
			type call struct {
				llm     deepseektestutil.IntegrationSettings
				webhook publishertestutil.IntegrationSettings
			}
			var calls []call
			gateCLISlackIntegration(recorder, func(name string) string { return env[name] }, func(l deepseektestutil.IntegrationSettings, w publishertestutil.IntegrationSettings) {
				calls = append(calls, call{l, w})
			})

			got := outcomes{ran: len(calls), skipped: len(recorder.skipped), failed: len(recorder.failed)}
			if got != tc.want {
				t.Fatalf("gate outcomes = %+v, want %+v", got, tc.want)
			}
			for _, reason := range recorder.skipped {
				// A skip must point at this test's own opt-in and target.
				for _, want := range []string{publishertestutil.CLISlackOptInEnv, "make test-integration-cli-slack`"} {
					if !strings.Contains(reason, want) {
						t.Errorf("skip reason %q does not mention %q", reason, want)
					}
				}
			}
			for _, reason := range slices.Concat(recorder.skipped, recorder.failed) {
				for _, s := range forbidden {
					if strings.Contains(reason, s) {
						t.Error("the reason holds an API key, a webhook URL, or the last eight characters of one")
					}
				}
			}
			if tc.want != ran {
				return
			}
			if calls[0].llm.Model != model {
				t.Errorf("Model = %q, want %q", calls[0].llm.Model, model)
			}
			if key, err := calls[0].llm.APIKey.Reveal(); err != nil || key != testKey {
				t.Errorf("APIKey is not the value of %s (Reveal error = %v)", deepseektestutil.APIKeyEnv, err)
			}
			if url, err := calls[0].webhook.WebhookURL.Reveal(); err != nil || url != testWebhook {
				t.Errorf("WebhookURL is not the value of %s (Reveal error = %v)", publishertestutil.WebhookURLEnv, err)
			}
		})
	}
}

// TestCLISlackIntegrationTestBuildTag pins the build tag that keeps the CLI
// webhook integration test, which calls the real DeepSeek API and posts to the
// real Webhook, out of `make test` while `make lint` still compiles it.
func TestCLISlackIntegrationTestBuildTag(t *testing.T) {
	if err := deepseektestutil.FirstLineIs("integration_slack_test.go", "//go:build integration"); err != nil {
		t.Errorf("integration_slack_test.go must start with %q: %v", "//go:build integration", err)
	}
}

// TestMakeBuildBuildsEveryBinary checks the recipes `make -n` expands for the
// build and all targets: each builds the CLI, the server, and the Native
// Messaging host, and no line runs the host binary with install or uninstall,
// because registering the host is the user's explicit step. The recipes are
// read after expansion, since the Makefile names the packages through
// variables.
func TestMakeBuildBuildsEveryBinary(t *testing.T) {
	makePath, err := exec.LookPath("make")
	if err != nil {
		t.Fatalf("make is not on PATH: %v", err)
	}
	packages := []string{"./cmd/yt2column", "./cmd/yt2column-server", "./cmd/yt2column-nmhost"}
	for _, target := range []string{"build", "all"} {
		t.Run(target, func(t *testing.T) {
			cmd := exec.Command(makePath, "-n", target) //nolint:gosec // make from PATH with a fixed target
			cmd.Dir = repositoryRoot
			// Only PATH and HOME, so the outer make's MAKEFLAGS and variables
			// do not change the expansion.
			cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "HOME=" + os.Getenv("HOME")}
			output, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("make -n %s error = %v, output:\n%s", target, err, output)
			}
			built := map[string]struct{}{}
			for line := range strings.Lines(string(output)) {
				fields := strings.Fields(line)
				if len(fields) >= 2 && fields[1] == "build" {
					built[fields[len(fields)-1]] = struct{}{}
				}
				for i, field := range fields[:max(len(fields)-1, 0)] {
					if strings.HasSuffix(field, "yt2column-nmhost") && (fields[i+1] == "install" || fields[i+1] == "uninstall") {
						t.Errorf("make -n %s runs the host with %s: %q", target, fields[i+1], strings.TrimSpace(line))
					}
				}
			}
			for _, pkg := range packages {
				if _, ok := built[pkg]; !ok {
					t.Errorf("make -n %s does not build %s; output:\n%s", target, pkg, output)
				}
			}
		})
	}
}
