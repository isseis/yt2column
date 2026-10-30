//go:build test

package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/isseis/yt2column/internal/llm/claudeparam"
	"github.com/isseis/yt2column/internal/llm/deepseekparam"
)

// envLookup returns a LookupFunc over env. A key absent from env reports unset,
// so a test controls the environment without touching the process environment.
func envLookup(env map[string]string) LookupFunc {
	return func(name string) (string, bool) {
		value, ok := env[name]
		return value, ok
	}
}

// validEnv is an environment every rule accepts.
func validEnv() map[string]string {
	return map[string]string{
		llmEnv:            "deepseek/deepseek-chat",
		deepSeekAPIKeyEnv: "sk-test-key",
		slackEnv:          "https://hooks.slack.com/services/T000/B000/XXXX",
		cacheDirEnv:       "/tmp/yt2column-test-cache",
		ytDlpEnv:          "/usr/local/bin/yt-dlp",
	}
}

// validClaudeEnv is an environment every claude rule accepts. SLACK_WEBHOOK_URL
// is optional, so it is absent here.
func validClaudeEnv() map[string]string {
	return map[string]string{
		llmEnv:                  "claude/claude-opus-5-5",
		anthropicAPIKeyEnv:      "sk-ant-test-key",
		anthropicWorkspaceIDEnv: "wrkspc_test0123456789",
		cacheDirEnv:             "/tmp/yt2column-test-cache",
	}
}

// with returns a copy of env with name set to value.
func with(env map[string]string, name, value string) map[string]string {
	out := maps.Clone(env)
	out[name] = value
	return out
}

// without returns a copy of env with names removed.
func without(env map[string]string, names ...string) map[string]string {
	out := maps.Clone(env)
	for _, name := range names {
		delete(out, name)
	}
	return out
}

func TestLoadValid(t *testing.T) {
	env := validEnv()
	cfg, err := Load(envLookup(env))
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}
	if cfg.LLM().Generation().Provider() != ProviderDeepSeek {
		t.Errorf("Provider() = %v, want ProviderDeepSeek", cfg.LLM().Generation().Provider())
	}
	if cfg.LLM().Generation().Model() != "deepseek-chat" {
		t.Errorf("Model() = %q, want deepseek-chat", cfg.LLM().Generation().Model())
	}
	key, err := cfg.LLM().DeepSeekAPIKey().Reveal()
	if err != nil {
		t.Fatalf("DeepSeekAPIKey().Reveal() error = %v", err)
	}
	if key != env[deepSeekAPIKeyEnv] {
		t.Errorf("DeepSeekAPIKey() = %q, want %q", key, env[deepSeekAPIKeyEnv])
	}
	webhook, ok := cfg.SlackWebhookURL()
	if !ok {
		t.Fatal("SlackWebhookURL() reports unset, want set")
	}
	revealed, err := webhook.Reveal()
	if err != nil {
		t.Fatalf("SlackWebhookURL().Reveal() error = %v", err)
	}
	if revealed != env[slackEnv] {
		t.Errorf("SlackWebhookURL() = %q, want %q", revealed, env[slackEnv])
	}
	if cfg.CacheDir() != env[cacheDirEnv] {
		t.Errorf("CacheDir() = %q, want %q", cfg.CacheDir(), env[cacheDirEnv])
	}
	if cfg.YtDlpPath() != env[ytDlpEnv] {
		t.Errorf("YtDlpPath() = %q, want %q", cfg.YtDlpPath(), env[ytDlpEnv])
	}
}

// TestLoadDefaults checks the defaults of the variables other than the LLM
// ones: no Webhook, yt-dlp on PATH, and the derived cache directory. The LLM
// defaults are checked by TestLoadGenerationDefaults and TestLoadReviewDefaults.
func TestLoadDefaults(t *testing.T) {
	home := "/home/user"
	env := map[string]string{
		llmEnv:            "deepseek/deepseek-chat",
		deepSeekAPIKeyEnv: "sk-test-key",
		"HOME":            home,
		// XDG_CACHE_HOME is set so the test exercises the default on both
		// darwin and the other Unix systems without reading the process
		// environment.
		"XDG_CACHE_HOME": filepath.Join(home, ".cache"),
	}
	cfg, err := Load(envLookup(env))
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}
	if _, ok := cfg.SlackWebhookURL(); ok {
		t.Error("SlackWebhookURL() reports set, want unset")
	}
	if cfg.YtDlpPath() != "" {
		t.Errorf("YtDlpPath() = %q, want empty", cfg.YtDlpPath())
	}
	wantDir := filepath.Join(home, ".cache", "yt2column")
	if runtime.GOOS == "darwin" {
		wantDir = filepath.Join(home, "Library", "Caches", "yt2column")
	}
	if cfg.CacheDir() != wantDir {
		t.Errorf("CacheDir() = %q, want %q", cfg.CacheDir(), wantDir)
	}
}

func TestLoadMissing(t *testing.T) {
	base := validEnv()
	cases := []struct {
		name    string
		env     map[string]string
		wantVar string
	}{
		// An unset YT2COLUMN_LLM selects the default claude, whose key is
		// absent from the deepseek environment.
		{"LLM unset selects claude, whose API key is unset", without(base, llmEnv), anthropicAPIKeyEnv},
		{"API key unset with deepseek", without(base, deepSeekAPIKeyEnv), deepSeekAPIKeyEnv},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Load(envLookup(tc.env))
			if !errors.Is(err, ErrMissing) {
				t.Fatalf("Load() error = %v, want ErrMissing", err)
			}
			if !strings.Contains(err.Error(), tc.wantVar) {
				t.Errorf("Load() error = %q, want it to name %q", err, tc.wantVar)
			}
		})
	}
}

func TestLoadEmpty(t *testing.T) {
	vars := []string{llmEnv, reviewLLMEnv, deepSeekAPIKeyEnv, slackEnv, cacheDirEnv, ytDlpEnv}
	for _, name := range vars {
		t.Run(name, func(t *testing.T) {
			_, err := Load(envLookup(with(validEnv(), name, "")))
			if !errors.Is(err, ErrMissing) {
				t.Fatalf("Load() error = %v, want ErrMissing", err)
			}
			if !strings.Contains(err.Error(), name) {
				t.Errorf("Load() error = %q, want it to name %q", err, name)
			}
		})
	}
}

// TestLoadInvalid checks that Load rejects each invalid variable with
// ErrInvalid and names it.
func TestLoadInvalid(t *testing.T) {
	cases := []struct {
		name string
		vr   string
		bad  []string
	}{
		{"slack webhook", slackEnv, []string{
			"http://mattermost.example.com/hooks/x",
			"mattermost.example.com/hooks/x",
			"https:///hooks/x",
			"https://mattermost.example.com/hooks/x\n",
			"https://hooks.slack.com/services/%zz",
			"https://hooks.slack.com/%",
			"https://hooks.slack.com/services/x\x7f",
			"https://:443/x",
			" https://hooks.slack.com/services/x",
		}},
		{"cache dir", cacheDirEnv, []string{"cache", "./cache", "~/cache"}},
	}
	for _, tc := range cases {
		for _, value := range tc.bad {
			t.Run(fmt.Sprintf("%s %q", tc.name, value), func(t *testing.T) {
				_, err := Load(envLookup(with(validEnv(), tc.vr, value)))
				if !errors.Is(err, ErrInvalid) {
					t.Fatalf("Load() error = %v, want ErrInvalid", err)
				}
				if !strings.Contains(err.Error(), tc.vr) {
					t.Errorf("Load() error = %q, want it to name %q", err, tc.vr)
				}
			})
		}
	}
}

// TestLoadClaude checks that a claude environment loads, with and without a
// workspace ID, and each provider accessor. The effort values are in
// TestLoadParsesLLMValue.
func TestLoadClaude(t *testing.T) {
	t.Run("workspace id set", func(t *testing.T) {
		env := validClaudeEnv()
		cfg, err := Load(envLookup(env))
		if err != nil {
			t.Fatalf("Load() error = %v, want nil", err)
		}
		if cfg.LLM().Generation().Provider() != ProviderClaude {
			t.Errorf("Provider() = %v, want ProviderClaude", cfg.LLM().Generation().Provider())
		}
		key, err := cfg.LLM().AnthropicAPIKey().Reveal()
		if err != nil {
			t.Fatalf("AnthropicAPIKey().Reveal() error = %v", err)
		}
		if key != env[anthropicAPIKeyEnv] {
			t.Errorf("AnthropicAPIKey() = %q, want %q", key, env[anthropicAPIKeyEnv])
		}
		workspace, ok := cfg.LLM().AnthropicWorkspaceID().Value()
		if !ok {
			t.Fatal("AnthropicWorkspaceID() reports unset, want set")
		}
		if workspace != env[anthropicWorkspaceIDEnv] {
			t.Errorf("AnthropicWorkspaceID() = %q, want %q", workspace, env[anthropicWorkspaceIDEnv])
		}
	})

	t.Run("workspace id unset", func(t *testing.T) {
		cfg, err := Load(envLookup(without(validClaudeEnv(), anthropicWorkspaceIDEnv)))
		if err != nil {
			t.Fatalf("Load() error = %v, want nil", err)
		}
		if _, ok := cfg.LLM().AnthropicWorkspaceID().Value(); ok {
			t.Error("AnthropicWorkspaceID() reports set, want unset")
		}
	})
}

// TestLoadClaudeMissing checks that an unset or empty required claude variable
// wraps ErrMissing and that the rejected variable's name can be extracted.
func TestLoadClaudeMissing(t *testing.T) {
	cases := []struct {
		name    string
		env     map[string]string
		wantVar string
	}{
		{"API key unset", without(validClaudeEnv(), anthropicAPIKeyEnv), anthropicAPIKeyEnv},
		{"API key empty", with(validClaudeEnv(), anthropicAPIKeyEnv, ""), anthropicAPIKeyEnv},
		{"workspace id empty", with(validClaudeEnv(), anthropicWorkspaceIDEnv, ""), anthropicWorkspaceIDEnv},
		{
			"only the deepseek key",
			with(without(validClaudeEnv(), anthropicAPIKeyEnv), deepSeekAPIKeyEnv, "sk-test-key"),
			anthropicAPIKeyEnv,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Load(envLookup(tc.env))
			assertVarError(t, err, ErrMissing, tc.wantVar)
		})
	}
}

// varErrors returns every *VarError in err's tree. errors.AsType stops at the
// first match, and a joined load error holds one *VarError per rejected
// variable, so the tree is walked node by node.
func varErrors(err error) []*VarError {
	if err == nil {
		return nil
	}
	var found []*VarError
	if varErr, ok := errors.AsType[*VarError](err); ok && varErr == err {
		found = append(found, varErr)
	}
	if multi, ok := err.(interface{ Unwrap() []error }); ok {
		for _, child := range multi.Unwrap() {
			found = append(found, varErrors(child)...)
		}
		return found
	}
	return append(found, varErrors(errors.Unwrap(err))...)
}

// assertVarErrors checks that err holds exactly the rejections in want, in any
// order, comparing each one's name, sentinel, and fixed reason. A nil want
// expects err to be nil. Unlike assertVarError, which inspects only the first
// *VarError, it fails on a missing, an extra, or a duplicated rejection.
func assertVarErrors(t *testing.T, err error, want ...VarError) {
	t.Helper()
	got := make([]VarError, 0, len(want))
	for _, varErr := range varErrors(err) {
		got = append(got, *varErr)
	}
	if err != nil && len(got) == 0 {
		t.Fatalf("error = %v, want only *VarError rejections", err)
	}
	byName := func(a, b VarError) int { return strings.Compare(a.Name+a.Reason, b.Name+b.Reason) }
	want = slices.Clone(want)
	slices.SortFunc(got, byName)
	slices.SortFunc(want, byName)
	if !slices.Equal(got, want) {
		t.Errorf("rejections = %v, want %v", got, want)
	}
}

// TestLoadClaudeEffortDefault checks that a claude LLM value without an effort
// part selects the high effort for exactly the stages that use claude, for
// both entry points, and that a deepseek stage gets no claude effort.
func TestLoadClaudeEffortDefault(t *testing.T) {
	claudeBoth := validClaudeEnv()
	reviewClaude := with(validEnv(), reviewLLMEnv, "claude/claude-review")
	reviewClaude = with(reviewClaude, anthropicAPIKeyEnv, "sk-ant-test-key")
	generationClaude := with(claudeBoth, reviewLLMEnv, "deepseek/deepseek-review")
	generationClaude = with(generationClaude, deepSeekAPIKeyEnv, "sk-test-key")
	high, unset := claudeparam.EffortHigh, claudeparam.EffortUnset
	cases := []struct {
		name                       string
		env                        map[string]string
		wantGeneration, wantReview claudeparam.Effort
	}{
		{"both stages claude", claudeBoth, high, high},
		{"review claude only", reviewClaude, unset, high},
		{"generation claude only", generationClaude, high, unset},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := Load(envLookup(tc.env))
			if err != nil {
				t.Fatalf("Load() error = %v, want nil", err)
			}
			assertClaudeEfforts(t, cfg.LLM(), tc.wantGeneration, tc.wantReview)
		})
	}

	// LoadServer shares Load's LLM loading today; this pins the server entry
	// point, so a server-specific loader would still get the default.
	t.Run("LoadServer", func(t *testing.T) {
		env := without(claudeBoth, cacheDirEnv)
		env = with(env, "HOME", "/home/user")
		env = with(env, slackEnv, "https://hooks.slack.com/services/T000/B000/XXXX")
		cfg, err := LoadServer(envLookup(env))
		if err != nil {
			t.Fatalf("LoadServer() error = %v, want nil", err)
		}
		assertClaudeEfforts(t, cfg.LLM(), high, high)
	})
}

// assertClaudeEfforts checks the claude effort of each stage's selection.
func assertClaudeEfforts(t *testing.T, llm LLMConfig, wantGeneration, wantReview claudeparam.Effort) {
	t.Helper()
	if got := llm.Generation().ClaudeEffort(); got != wantGeneration {
		t.Errorf("Generation().ClaudeEffort() = %v, want %v", got, wantGeneration)
	}
	if got := llm.Review().ClaudeEffort(); got != wantReview {
		t.Errorf("Review().ClaudeEffort() = %v, want %v", got, wantReview)
	}
}

// TestLoadClaudeInvalid checks that an unusable claude provider variable wraps
// ErrInvalid and names the variable.
func TestLoadClaudeInvalid(t *testing.T) {
	t.Run("workspace id", func(t *testing.T) {
		for _, value := range []string{" wrkspc_x", "wrkspc_x\n", "wrkspc é"} {
			t.Run(fmt.Sprintf("%q", value), func(t *testing.T) {
				_, err := Load(envLookup(with(validClaudeEnv(), anthropicWorkspaceIDEnv, value)))
				assertVarError(t, err, ErrInvalid, anthropicWorkspaceIDEnv)
			})
		}
	})
}

// TestLoadClaudeReportsAllInvalid checks that a claude environment with several
// rejected provider variables reports every one, not only the first.
func TestLoadClaudeReportsAllInvalid(t *testing.T) {
	env := with(validClaudeEnv(), anthropicWorkspaceIDEnv, " wrkspc_x")
	env = without(env, anthropicAPIKeyEnv)
	_, err := Load(envLookup(env))
	if !errors.Is(err, ErrMissing) {
		t.Errorf("Load() error = %v, want it to wrap ErrMissing", err)
	}
	if !errors.Is(err, ErrInvalid) {
		t.Errorf("Load() error = %v, want it to wrap ErrInvalid", err)
	}
	for _, name := range []string{anthropicAPIKeyEnv, anthropicWorkspaceIDEnv} {
		if !strings.Contains(err.Error(), name) {
			t.Errorf("Load() error = %q, want it to name %q", err, name)
		}
	}
}

// TestLoadClaudeIgnoresDeepSeekKey checks that DEEPSEEK_API_KEY's value never
// changes a claude load, in both the success and the error case.
func TestLoadClaudeIgnoresDeepSeekKey(t *testing.T) {
	variants := []struct {
		name string
		env  map[string]string
	}{
		{"unset", without(validClaudeEnv(), deepSeekAPIKeyEnv)},
		{"empty", with(validClaudeEnv(), deepSeekAPIKeyEnv, "")},
		{"set", with(validClaudeEnv(), deepSeekAPIKeyEnv, "sk-test-key")},
	}
	t.Run("success", func(t *testing.T) {
		for _, variant := range variants {
			t.Run(variant.name, func(t *testing.T) {
				cfg, err := Load(envLookup(variant.env))
				if err != nil {
					t.Fatalf("Load() error = %v, want nil", err)
				}
				if cfg.LLM().Generation().Provider() != ProviderClaude {
					t.Errorf("Provider() = %v, want ProviderClaude", cfg.LLM().Generation().Provider())
				}
			})
		}
	})
	t.Run("error", func(t *testing.T) {
		for _, variant := range variants {
			t.Run(variant.name, func(t *testing.T) {
				env := without(variant.env, anthropicAPIKeyEnv)
				_, err := Load(envLookup(env))
				assertVarError(t, err, ErrMissing, anthropicAPIKeyEnv)
				if strings.Contains(err.Error(), deepSeekAPIKeyEnv) {
					t.Errorf("Load() error = %q, must not name %q", err, deepSeekAPIKeyEnv)
				}
			})
		}
	})
}

// TestLoadIgnoresOtherProviderVars checks that, with the deepseek provider,
// each claude provider variable is neither read nor rejected whatever its
// value, and that a rejected provider part is reported alone.
func TestLoadIgnoresOtherProviderVars(t *testing.T) {
	type variant struct {
		name  string
		value string
		set   bool
	}
	// The API key has no invalid shape beyond empty, so its variants stop at
	// unset, empty, and a present value.
	vars := []struct {
		name     string
		variants []variant
	}{
		{anthropicAPIKeyEnv, []variant{
			{"unset", "", false},
			{"empty", "", true},
			{"present", "sk-ant-key", true},
		}},
		{anthropicWorkspaceIDEnv, []variant{
			{"unset", "", false},
			{"empty", "", true},
			{"invalid", " wrkspc_x", true},
			{"valid", "wrkspc_x", true},
		}},
	}
	for _, v := range vars {
		for _, variant := range v.variants {
			t.Run(v.name+" "+variant.name, func(t *testing.T) {
				env := validEnv()
				if variant.set {
					env = with(env, v.name, variant.value)
				} else {
					env = without(env, v.name)
				}
				cfg, err := Load(envLookup(env))
				if err != nil {
					t.Fatalf("Load() error = %v, want nil", err)
				}
				if cfg.LLM().Generation().Provider() != ProviderDeepSeek {
					t.Errorf("Provider() = %v, want ProviderDeepSeek", cfg.LLM().Generation().Provider())
				}
				if _, err := cfg.LLM().AnthropicAPIKey().Reveal(); err == nil {
					t.Error("AnthropicAPIKey() is set for the deepseek provider")
				}
				assertClaudeEfforts(t, cfg.LLM(), claudeparam.EffortUnset, claudeparam.EffortUnset)
				if _, ok := cfg.LLM().AnthropicWorkspaceID().Value(); ok {
					t.Error("AnthropicWorkspaceID() is set for the deepseek provider")
				}
			})
		}
	}

	t.Run("invalid provider reports only the LLM variable", func(t *testing.T) {
		env := with(validEnv(), llmEnv, "gemini/x")
		env = with(env, anthropicAPIKeyEnv, "sk-ant-key")
		env = with(env, anthropicWorkspaceIDEnv, " wrkspc_x")
		_, err := Load(envLookup(env))
		assertVarErrors(t, err, wantInvalid(llmEnv, reasonLLMProvider))
	})
}

// TestLoadReview checks the review selection: YT2COLUMN_REVIEW_LLM unset
// inherits the generation selection, and a value of its own selects the review
// provider and model without changing the generation's. The rejected values
// are in TestLoadParsesLLMValue.
func TestLoadReview(t *testing.T) {
	t.Run("unset inherits the generation selection", func(t *testing.T) {
		cfg, err := Load(envLookup(validEnv()))
		if err != nil {
			t.Fatalf("Load() error = %v, want nil", err)
		}
		if cfg.LLM().Review().Provider() != ProviderDeepSeek || cfg.LLM().Review().Model() != "deepseek-chat" {
			t.Errorf("review = %v/%q, want the generation's deepseek/deepseek-chat",
				cfg.LLM().Review().Provider(), cfg.LLM().Review().Model())
		}
	})

	cases := []struct {
		name         string
		value        string
		wantProvider Provider
		wantModel    string
	}{
		{"same provider, another model", "deepseek/deepseek-review", ProviderDeepSeek, "deepseek-review"},
		{"another provider", "claude/claude-review", ProviderClaude, "claude-review"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env := with(validEnv(), reviewLLMEnv, tc.value)
			env = with(env, anthropicAPIKeyEnv, "sk-ant-review-key")
			cfg, err := Load(envLookup(env))
			if err != nil {
				t.Fatalf("Load() error = %v, want nil", err)
			}
			if cfg.LLM().Generation().Provider() != ProviderDeepSeek || cfg.LLM().Generation().Model() != "deepseek-chat" {
				t.Errorf("generation = %v/%q, want deepseek/deepseek-chat", cfg.LLM().Generation().Provider(), cfg.LLM().Generation().Model())
			}
			if cfg.LLM().Review().Provider() != tc.wantProvider || cfg.LLM().Review().Model() != tc.wantModel {
				t.Errorf("review = %v/%q, want %v/%q",
					cfg.LLM().Review().Provider(), cfg.LLM().Review().Model(), tc.wantProvider, tc.wantModel)
			}
		})
	}
}

// TestLoadReviewProviderRequiresItsKey checks that when the review provider
// differs from the generation provider, the review provider's API key variable
// is required even though the generation provider's key is present.
func TestLoadReviewProviderRequiresItsKey(t *testing.T) {
	claudeReview := func() map[string]string {
		env := validEnv()
		env = with(env, reviewLLMEnv, "claude/claude-review")
		return env
	}
	deepseekReview := func() map[string]string {
		return with(validClaudeEnv(), reviewLLMEnv, "deepseek/deepseek-review")
	}
	cases := []struct {
		name    string
		env     map[string]string
		wantVar string
	}{
		{"claude review key unset", claudeReview(), anthropicAPIKeyEnv},
		{"claude review key empty", with(claudeReview(), anthropicAPIKeyEnv, ""), anthropicAPIKeyEnv},
		{"deepseek review key unset", deepseekReview(), deepSeekAPIKeyEnv},
		{"deepseek review key empty", with(deepseekReview(), deepSeekAPIKeyEnv, ""), deepSeekAPIKeyEnv},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Load(envLookup(tc.env))
			assertVarError(t, err, ErrMissing, tc.wantVar)
		})
	}
}

// assertVarError checks that err wraps sentinel and names exactly one variable,
// wantVar.
func assertVarError(t *testing.T, err error, sentinel error, wantVar string) {
	t.Helper()
	if !errors.Is(err, sentinel) {
		t.Fatalf("Load() error = %v, want it to wrap %v", err, sentinel)
	}
	varErr, ok := errors.AsType[*VarError](err)
	if !ok {
		t.Fatalf("Load() error = %v, want *VarError", err)
	}
	if varErr.Name != wantVar {
		t.Errorf("VarError.Name = %q, want %q", varErr.Name, wantVar)
	}
}

// TestLoadSlackWebhookAccepted checks that Load accepts the Webhook URL shapes
// the new rule allows and keeps the original value.
func TestLoadSlackWebhookAccepted(t *testing.T) {
	accepted := []string{
		"https://mattermost.example.com/hooks/xxxxxxxxxxxxxxxxxxxxxxxxxx",
		"https://hooks.slack.com/services/T000/B000/XXXX",
		"https://hooks.slack.com.example/services/x",
		"https://HOOKS.SLACK.COM/services/x",
		"https://hooks.slack.com/",
	}
	for i, value := range accepted {
		t.Run(fmt.Sprintf("case %d", i), func(t *testing.T) {
			cfg, err := Load(envLookup(with(validEnv(), slackEnv, value)))
			if err != nil {
				t.Fatalf("Load() error = %v, want nil", err)
			}
			webhook, ok := cfg.SlackWebhookURL()
			if !ok {
				t.Fatal("SlackWebhookURL() reports unset, want set")
			}
			revealed, err := webhook.Reveal()
			if err != nil {
				t.Fatalf("SlackWebhookURL().Reveal() error = %v", err)
			}
			if revealed != value {
				t.Errorf("SlackWebhookURL() = %q, want %q", revealed, value)
			}
		})
	}
}

// TestRequireSlackWebhookURL checks the missing and configured cases of
// RequireSlackWebhookURL.
func TestRequireSlackWebhookURL(t *testing.T) {
	t.Run("unset", func(t *testing.T) {
		cfg, err := Load(envLookup(without(validEnv(), slackEnv)))
		if err != nil {
			t.Fatalf("Load() error = %v, want nil", err)
		}
		_, err = cfg.RequireSlackWebhookURL()
		varErr, ok := errors.AsType[*VarError](err)
		if !ok {
			t.Fatalf("RequireSlackWebhookURL() error = %v, want *VarError", err)
		}
		if varErr.Name != slackEnv {
			t.Errorf("VarError.Name = %q, want %q", varErr.Name, slackEnv)
		}
		if !errors.Is(err, ErrMissing) {
			t.Errorf("RequireSlackWebhookURL() error = %v, want it to wrap ErrMissing", err)
		}
	})

	t.Run("set", func(t *testing.T) {
		const value = "https://mattermost.example.com/hooks/xxxxxxxxxxxxxxxxxxxxxxxxxx"
		cfg, err := Load(envLookup(with(validEnv(), slackEnv, value)))
		if err != nil {
			t.Fatalf("Load() error = %v, want nil", err)
		}
		webhook, err := cfg.RequireSlackWebhookURL()
		if err != nil {
			t.Fatalf("RequireSlackWebhookURL() error = %v, want nil", err)
		}
		revealed, err := webhook.Reveal()
		if err != nil {
			t.Fatalf("Reveal() error = %v", err)
		}
		if revealed != value {
			t.Errorf("Reveal() = %q, want %q", revealed, value)
		}
	})
}

// TestLoadErrorsOmitValues checks that a rejection names the variable but never
// the value, its mark, or its tail.
func TestLoadErrorsOmitValues(t *testing.T) {
	const (
		markLLM             = "MARK-LLM-A1"
		markModel           = "MARK-MODEL-B2"
		markReviewLLM       = "MARK-REVIEW-LLM-I9"
		markRetired         = "MARK-RETIRED-J0"
		markAPIKey          = "MARK-APIKEY-C3"
		markSlack           = "MARK-SLACK-D4"
		markCache           = "MARK-CACHE-E5"
		markYtDlp           = "MARK-YTDLP-F6"
		markClaudeWorkspace = "MARK-CLAUDE-WKS-C8"
		markEffort          = "MARK-EFFORT-G7"
		markLLMModel        = "MARK-LLM-MODEL-H1"
		markSecret          = "MARK-SECRET-K2"
	)
	marks := []string{
		markLLM, markModel, markReviewLLM, markRetired, markAPIKey, markSlack,
		markCache, markYtDlp, markClaudeWorkspace, markEffort, markLLMModel, markSecret,
	}
	base := map[string]string{
		llmEnv:            "deepseek/" + markModel,
		deepSeekAPIKeyEnv: markAPIKey,
		slackEnv:          "https://hooks.slack.com/services/" + markSlack,
		cacheDirEnv:       "/tmp/" + markCache,
		ytDlpEnv:          "/bin/" + markYtDlp,
	}
	// claudeBase selects the claude provider, whose own variables are the ones
	// under test in the claude rows below.
	claudeBase := map[string]string{
		llmEnv:                  "claude/" + markModel,
		anthropicAPIKeyEnv:      markAPIKey,
		anthropicWorkspaceIDEnv: "wrkspc_ok",
		cacheDirEnv:             "/tmp/" + markCache,
		ytDlpEnv:                "/bin/" + markYtDlp,
	}
	cases := []struct {
		name  string
		env   map[string]string
		tails []string
	}{
		{"LLM provider invalid", with(base, llmEnv, "gemini-"+markLLM+"/x"), nil},
		{"LLM without a separator", with(base, llmEnv, markLLM), nil},
		{"LLM with an empty model ID", with(base, llmEnv, "deepseek/"), nil},
		{"LLM empty", with(base, llmEnv, ""), nil},
		{"review LLM provider invalid", with(base, reviewLLMEnv, "gemini/"+markReviewLLM), nil},
		{"review LLM without a separator", with(base, reviewLLMEnv, markReviewLLM), nil},
		// The effort-part rows mark the effort part and the model ID apart, so
		// a rejection leaking either is caught; searching for the rejected
		// value itself could match the accepted values the reason lists. Each
		// of the three effort rules (too many "+", an empty model ID, an
		// unaccepted effort part) has a row for each LLM variable.
		{"claude effort part invalid", with(claudeBase, llmEnv, "claude/"+markLLMModel+"+"+markEffort), nil},
		{"deepseek effort part invalid", with(base, llmEnv, "deepseek/"+markLLMModel+"+"+markEffort), nil},
		{"LLM with two effort separators", with(base, llmEnv, "deepseek/"+markLLMModel+"+high+"+markEffort), nil},
		{"LLM with an empty model ID and an effort", with(claudeBase, llmEnv, "claude/+"+markEffort), nil},
		{"LLM effort part shaped like a secret", with(claudeBase, llmEnv, "claude/claude-opus-5-5+sk-ant-"+markSecret), nil},
		{"LLM model ID shaped like a secret", with(base, llmEnv, "deepseek/sk-"+markSecret+"+ultra"), nil},
		{"review claude effort part invalid", with(base, reviewLLMEnv, "claude/"+markLLMModel+"+"+markEffort), nil},
		{"review deepseek effort part invalid", with(base, reviewLLMEnv, "deepseek/"+markLLMModel+"+"+markEffort), nil},
		{"review LLM with two effort separators", with(base, reviewLLMEnv, "claude/"+markLLMModel+"+"+markEffort+"+low"), nil},
		{"review LLM with an empty model ID and an effort", with(base, reviewLLMEnv, "deepseek/+"+markEffort), nil},
		{"API key empty", with(base, deepSeekAPIKeyEnv, ""), nil},
		// Each slack webhook row also names the last 8 bytes of its value, so a
		// rejection that leaked only the tail would still be caught.
		{"slack webhook invalid", with(base, slackEnv, "http://"+markSlack), []string{"SLACK-D4"}},
		{"slack webhook no scheme", with(base, slackEnv, markSlack+"/hooks/x"), []string{"/hooks/x"}},
		{"slack webhook empty host", with(base, slackEnv, "https:///"+markSlack), []string{"SLACK-D4"}},
		{"slack webhook control character", with(base, slackEnv, "https://host/"+markSlack+"\n"), []string{"LACK-D4\n"}},
		{"slack webhook bad escape", with(base, slackEnv, "https://host/%zz"+markSlack), []string{"SLACK-D4"}},
		{"cache dir relative", with(base, cacheDirEnv, markCache), nil},
		{"cache dir absent", without(base, cacheDirEnv), nil},
		{"yt-dlp path empty", with(base, ytDlpEnv, ""), nil},
		{"claude workspace invalid", with(claudeBase, anthropicWorkspaceIDEnv, " "+markClaudeWorkspace), nil},
	}
	for _, v := range retiredLLMVars {
		cases = append(cases, struct {
			name  string
			env   map[string]string
			tails []string
		}{"retired " + v.name, with(base, v.name, "deepseek-"+markRetired), nil})
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Load(envLookup(tc.env))
			if err == nil {
				t.Fatal("Load() returned nil error")
			}
			for _, mark := range marks {
				if strings.Contains(err.Error(), mark) {
					t.Errorf("Load() error = %q, contains the value mark %q", err, mark)
				}
			}
			for _, tail := range tc.tails {
				if strings.Contains(err.Error(), tail) {
					t.Errorf("Load() error = %q, contains the value tail %q", err, tail)
				}
			}
		})
	}
}

// TestLoadReportsAllInvalid checks that one load reports every rejection, each
// as its own element of the joined error: the retired variables, both LLM
// variables, a provider variable, and a common variable. A nested join would
// reach a caller that prints one error per line as a single line.
func TestLoadReportsAllInvalid(t *testing.T) {
	cases := []struct {
		name string
		env  map[string]string
		want []VarError
	}{
		{
			"every kind of rejection",
			map[string]string{
				"YT2COLUMN_LLM_PROVIDER": "deepseek",
				"YT2COLUMN_REVIEW_MODEL": "",
				llmEnv:                   "gemini/x",
				// The review provider part is accepted, so claude's own
				// variables are still checked.
				reviewLLMEnv:              "claude/",
				"YT2COLUMN_CLAUDE_EFFORT": "High",
				cacheDirEnv:               "relative",
			},
			[]VarError{
				wantInvalid("YT2COLUMN_LLM_PROVIDER", reasonRetiredLLM),
				wantInvalid("YT2COLUMN_REVIEW_MODEL", reasonRetiredReviewLLM),
				wantInvalid("YT2COLUMN_CLAUDE_EFFORT", reasonRetiredClaudeEffort),
				wantInvalid(llmEnv, reasonLLMProvider),
				wantInvalid(reviewLLMEnv, reasonLLMForm),
				wantMissing(anthropicAPIKeyEnv),
				wantInvalid(cacheDirEnv, reasonDirRelative),
			},
		},
		{
			// The retired effort variable next to its replacement with an
			// unaccepted effort part: both are reported, and the provider part
			// still selects claude, whose key is required.
			"retired claude effort and an invalid effort part",
			map[string]string{
				"YT2COLUMN_CLAUDE_EFFORT": "low",
				llmEnv:                    "claude/claude-opus-5-5+ultra",
				cacheDirEnv:               "/tmp/yt2column-test-cache",
			},
			[]VarError{
				wantInvalid("YT2COLUMN_CLAUDE_EFFORT", reasonRetiredClaudeEffort),
				wantInvalid(llmEnv, reasonClaudeEffortPart),
				wantMissing(anthropicAPIKeyEnv),
			},
		},
		{
			// A deepseek setting in the retired variables: their values are not
			// used, so the default claude is selected and its key is required.
			"retired deepseek setting",
			map[string]string{
				"YT2COLUMN_LLM_PROVIDER": "deepseek",
				"YT2COLUMN_MODEL":        "deepseek-flash",
				deepSeekAPIKeyEnv:        "sk-test-key",
				cacheDirEnv:              "/tmp/yt2column-test-cache",
			},
			[]VarError{
				wantInvalid("YT2COLUMN_LLM_PROVIDER", reasonRetiredLLM),
				wantInvalid("YT2COLUMN_MODEL", reasonRetiredLLM),
				wantMissing(anthropicAPIKeyEnv),
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Load(envLookup(tc.env))
			assertVarErrors(t, err, tc.want...)
			multi, ok := err.(interface{ Unwrap() []error })
			if !ok {
				t.Fatalf("Load() error = %T, want a joined error", err)
			}
			for _, child := range multi.Unwrap() {
				if _, ok := child.(*VarError); !ok {
					t.Errorf("joined element %T (%v), want each rejection as its own *VarError", child, child)
				}
			}
		})
	}
}

func TestConfigOutputRedactsSecrets(t *testing.T) {
	const (
		key       = "sk-distinctive-secret-9tRq"
		claudeKey = "sk-ant-distinctive-secret-9tRq"
		webhook   = "https://hooks.slack.com/services/distinctive-webhook-7Lwz"
	)
	deepseekEnv := validEnv()
	deepseekEnv[deepSeekAPIKeyEnv] = key
	deepseekEnv[slackEnv] = webhook
	claudeEnv := validClaudeEnv()
	claudeEnv[anthropicAPIKeyEnv] = claudeKey

	cases := []struct {
		name    string
		env     map[string]string
		secrets []string
		present func(*testing.T, Config)
	}{
		{
			"deepseek",
			deepseekEnv,
			[]string{key, webhook},
			func(t *testing.T, cfg Config) {
				t.Helper()
				if revealed, err := cfg.LLM().DeepSeekAPIKey().Reveal(); err != nil || revealed != key {
					t.Fatalf("DeepSeekAPIKey() = %q, %v, want %q", revealed, err, key)
				}
			},
		},
		{
			"claude",
			claudeEnv,
			[]string{claudeKey},
			func(t *testing.T, cfg Config) {
				t.Helper()
				if revealed, err := cfg.LLM().AnthropicAPIKey().Reveal(); err != nil || revealed != claudeKey {
					t.Fatalf("AnthropicAPIKey() = %q, %v, want %q", revealed, err, claudeKey)
				}
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := Load(envLookup(tc.env))
			if err != nil {
				t.Fatalf("Load() error = %v, want nil", err)
			}
			// Confirm the secret reached the Config before checking it is
			// hidden, so the check cannot pass because the value was never
			// stored.
			tc.present(t, cfg)

			outputs := map[string]string{
				"fmt %v":    fmt.Sprintf("%v", cfg),
				"fmt %+v":   fmt.Sprintf("%+v", cfg),
				"fmt %#v":   fmt.Sprintf("%#v", cfg),
				"slog text": slogOutput(t, cfg, false),
				"slog JSON": slogOutput(t, cfg, true),
			}
			// Config has only unexported fields, so encoding/json cannot reach a
			// secret; this is the structural half of the check.
			data, err := json.Marshal(cfg) //nolint:staticcheck // no exported fields is the point
			if err != nil {
				t.Fatalf("json.Marshal(Config) error = %v", err)
			}
			outputs["json"] = string(data)

			for name, out := range outputs {
				for _, secret := range tc.secrets {
					if strings.Contains(out, secret) {
						t.Errorf("%s output contains the secret %q: %q", name, secret, out)
					}
				}
			}
		})
	}
}

// slogOutput renders cfg through log/slog, in JSON when jsonFormat is true.
func slogOutput(t *testing.T, cfg Config, jsonFormat bool) string {
	t.Helper()
	var buf bytes.Buffer
	opts := &slog.HandlerOptions{}
	var handler slog.Handler
	if jsonFormat {
		handler = slog.NewJSONHandler(&buf, opts)
	} else {
		handler = slog.NewTextHandler(&buf, opts)
	}
	slog.New(handler).Info("config", slog.Any("config", cfg))
	return buf.String()
}

// validServerEnv is an environment LoadServer accepts: the CLI environment
// without the cache and yt-dlp variables the server does not use, plus HOME,
// from which the socket path is derived.
func validServerEnv() map[string]string {
	env := without(validEnv(), cacheDirEnv, ytDlpEnv)
	env["HOME"] = "/home/user"
	return env
}

// wantServerSocketPath is the socket path LoadServer derives from the HOME of
// validServerEnv on this operating system.
func wantServerSocketPath() string {
	if runtime.GOOS == "darwin" {
		return "/home/user/Library/Application Support/yt2column/run/server.sock"
	}
	return "/home/user/.local/state/yt2column/run/server.sock"
}

// TestLoadServerValid checks that LoadServer loads the LLM settings, the
// configured Webhook URL, the socket path derived from HOME, and the HTTP/2
// debug flag. The OS-specific socket path rules are fixed by TestSocketPath.
func TestLoadServerValid(t *testing.T) {
	env := with(validServerEnv(), godebugEnv, "http2debug=1")
	cfg, err := LoadServer(envLookup(env))
	if err != nil {
		t.Fatalf("LoadServer() error = %v, want nil", err)
	}
	if cfg.LLM().Generation().Provider() != ProviderDeepSeek {
		t.Errorf("LLM().Generation().Provider() = %v, want ProviderDeepSeek", cfg.LLM().Generation().Provider())
	}
	if cfg.LLM().Generation().Model() != "deepseek-chat" {
		t.Errorf("LLM().Generation().Model() = %q, want deepseek-chat", cfg.LLM().Generation().Model())
	}
	webhook, configured := cfg.WebhookURL()
	if !configured {
		t.Fatal("WebhookURL() reports unset, want set")
	}
	url, err := webhook.Reveal()
	if err != nil {
		t.Fatalf("WebhookURL().Reveal() error = %v", err)
	}
	if url != env[slackEnv] {
		t.Errorf("WebhookURL() = %q, want %q", url, env[slackEnv])
	}
	if want := wantServerSocketPath(); cfg.SocketPath() != want {
		t.Errorf("SocketPath() = %q, want %q", cfg.SocketPath(), want)
	}
	if !cfg.HTTP2DebugEnabled() {
		t.Error("HTTP2DebugEnabled() = false, want true")
	}
}

// TestLoadServerLLMRejected checks that an empty YT2COLUMN_LLM, or one with an
// empty model ID, ends the server load with a rejection naming the variable
// instead of falling back to the default. The other LLM rules are covered by
// the Load tests, which LoadServer shares.
func TestLoadServerLLMRejected(t *testing.T) {
	cases := []struct {
		value string
		want  VarError
	}{
		{"", wantMissing(llmEnv)},
		{"deepseek/", wantInvalid(llmEnv, reasonLLMForm)},
	}
	for _, tc := range cases {
		t.Run(fmt.Sprintf("%q", tc.value), func(t *testing.T) {
			_, err := LoadServer(envLookup(with(validServerEnv(), llmEnv, tc.value)))
			assertVarErrors(t, err, tc.want)
		})
	}
}

// TestLoadServerWebhookOptional checks that an unset SLACK_WEBHOOK_URL is
// accepted and reported as unconfigured, that an empty or invalid value is
// rejected, and that neither rejection holds the value.
func TestLoadServerWebhookOptional(t *testing.T) {
	const mark = "MARK-SERVER-WEBHOOK-D4"
	t.Run("unset", func(t *testing.T) {
		cfg, err := LoadServer(envLookup(without(validServerEnv(), slackEnv)))
		if err != nil {
			t.Fatalf("LoadServer() error = %v, want nil", err)
		}
		if _, configured := cfg.WebhookURL(); configured {
			t.Error("WebhookURL() reports set, want unset")
		}
		// The rest of the configuration is still loaded: the Webhook is the
		// only value an unset variable leaves out.
		if got, want := cfg.SocketPath(), wantServerSocketPath(); got != want {
			t.Errorf("SocketPath() = %q, want %q", got, want)
		}
	})
	t.Run("empty", func(t *testing.T) {
		_, err := LoadServer(envLookup(with(validServerEnv(), slackEnv, "")))
		assertVarError(t, err, ErrMissing, slackEnv)
	})
	t.Run("invalid", func(t *testing.T) {
		_, err := LoadServer(envLookup(with(validServerEnv(), slackEnv, "http://host/"+mark)))
		assertVarError(t, err, ErrInvalid, slackEnv)
		if strings.Contains(err.Error(), mark) {
			t.Errorf("LoadServer() error = %q, contains the value mark %q", err, mark)
		}
	})
}

// TestLoadServerIgnoresCacheAndYtdlp checks that LoadServer neither reads nor
// checks YT2COLUMN_CACHE_DIR and YT2COLUMN_YTDLP_PATH, even when Load would
// reject them.
func TestLoadServerIgnoresCacheAndYtdlp(t *testing.T) {
	env := with(validServerEnv(), cacheDirEnv, "relative-cache")
	env = with(env, ytDlpEnv, "")
	if _, err := LoadServer(envLookup(env)); err != nil {
		t.Fatalf("LoadServer() error = %v, want nil", err)
	}
	if _, err := Load(envLookup(env)); err == nil {
		t.Fatal("Load() error = nil, want a rejection of the cache directory or the yt-dlp path")
	}
}

// TestLoadServerSocketPathUnderivable checks that a HOME from which no socket
// path can be derived ends the load with a *VarError naming HOME and a zero
// ServerConfig. TestSocketPath fixes the input shapes, the OS-specific
// branches, and the length limit; here each kind of failure must reach
// LoadServer.
func TestLoadServerSocketPathUnderivable(t *testing.T) {
	const limit = 107 // the longest limit of any operating system
	cases := []struct {
		name     string
		env      map[string]string
		sentinel error
	}{
		{"unset", without(validServerEnv(), "HOME"), ErrMissing},
		{"too long", with(validServerEnv(), "HOME", "/"+strings.Repeat("h", limit)), ErrInvalid},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := LoadServer(envLookup(tc.env))
			assertVarError(t, err, tc.sentinel, "HOME")
			if cfg.SocketPath() != "" || cfg.LLM().Generation().Model() != "" {
				t.Errorf("LoadServer() returned a non-zero ServerConfig on error: socketPath=%q model=%q",
					cfg.SocketPath(), cfg.LLM().Generation().Model())
			}
		})
	}
}

func TestLoadHTTP2Debug(t *testing.T) {
	cases := []struct {
		name string
		env  map[string]string
		want bool
	}{
		{"unset", without(validEnv(), godebugEnv), false},
		{"http2debug=1", with(validEnv(), godebugEnv, "http2debug=1"), true},
		{"http2debug=2", with(validEnv(), godebugEnv, "http2debug=2"), true},
		{"http2debug combined", with(validEnv(), godebugEnv, "madvdontneed=1,http2debug=1"), true},
		{"http2debug=0", with(validEnv(), godebugEnv, "http2debug=0"), false},
		{"http2debug=10 enables the log in net/http", with(validEnv(), godebugEnv, "http2debug=10"), true},
		{"space after the comma", with(validEnv(), godebugEnv, "madvdontneed=1, http2debug=1"), true},
		{"a longer setting name ending in http2debug", with(validEnv(), godebugEnv, "xhttp2debug=2"), true},
		{"other setting", with(validEnv(), godebugEnv, "madvdontneed=1"), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := Load(envLookup(tc.env))
			if err != nil {
				t.Fatalf("Load() error = %v, want nil", err)
			}
			if got := cfg.HTTP2DebugEnabled(); got != tc.want {
				t.Errorf("HTTP2DebugEnabled() = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestLoadFor checks that LoadFor reads and checks only the variables the
// run's stages need (architecture section 3.7). The CLI scenarios map to Need
// as: no --after -> {true,true}; --after generation -> {true,true}; --after
// generation --no-shorten -> {false,true}; --after generation --no-review ->
// {true,false}; --after shorten -> {false,true}; --after shorten --no-review ->
// {false,false}; --after review -> {false,false}.
func TestLoadFor(t *testing.T) {
	t.Run("both needs equal Load", func(t *testing.T) {
		cfg, err := LoadFor(envLookup(validEnv()), Need{Generation: true, Review: true})
		if err != nil {
			t.Fatalf("LoadFor() error = %v, want nil", err)
		}
		if cfg.LLM().Generation().Provider() != ProviderDeepSeek || cfg.LLM().Review().Provider() != ProviderDeepSeek {
			t.Errorf("providers = %v/%v, want deepseek/deepseek", cfg.LLM().Generation().Provider(), cfg.LLM().Review().Provider())
		}
	})

	t.Run("generation only leaves the review settings unread", func(t *testing.T) {
		// A review value that would be rejected if read is neither required
		// nor read when the review step does not run.
		env := with(validEnv(), reviewLLMEnv, "gemini/")
		cfg, err := LoadFor(envLookup(env), Need{Generation: true})
		if err != nil {
			t.Fatalf("LoadFor() error = %v, want nil", err)
		}
		if cfg.LLM().Review().Provider() != ProviderUnset {
			t.Errorf("Review().Provider() = %v, want ProviderUnset (not read)", cfg.LLM().Review().Provider())
		}
		if cfg.LLM().Review().Model() != "" {
			t.Errorf("Review().Model() = %q, want empty (not read)", cfg.LLM().Review().Model())
		}
	})

	t.Run("generation only requires the generation settings", func(t *testing.T) {
		cases := []struct {
			name string
			env  map[string]string
			want VarError
		}{
			{"LLM empty", with(validEnv(), llmEnv, ""), wantMissing(llmEnv)},
			{"API key unset", without(validEnv(), deepSeekAPIKeyEnv), wantMissing(deepSeekAPIKeyEnv)},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				_, err := LoadFor(envLookup(tc.env), Need{Generation: true})
				assertVarErrors(t, err, tc.want)
			})
		}
	})

	t.Run("review only checks the review provider's key", func(t *testing.T) {
		env := map[string]string{
			reviewLLMEnv: "deepseek/deepseek-review",
			cacheDirEnv:  "/tmp/yt2column-test-cache",
		}
		_, err := LoadFor(envLookup(env), Need{Review: true})
		assertVarErrors(t, err, wantMissing(deepSeekAPIKeyEnv))
	})

	t.Run("review only inherits a claude generation provider", func(t *testing.T) {
		// The inherited claude provider brings its own variables into the
		// check even though generation does not run.
		cfg, err := LoadFor(envLookup(validClaudeEnv()), Need{Review: true})
		if err != nil {
			t.Fatalf("LoadFor() error = %v, want nil", err)
		}
		if cfg.LLM().Review().Provider() != ProviderClaude || cfg.LLM().Review().Model() != "claude-opus-5-5" {
			t.Errorf("review = %v/%q, want claude/claude-opus-5-5", cfg.LLM().Review().Provider(), cfg.LLM().Review().Model())
		}
		if got := cfg.LLM().Review().ClaudeEffort(); got != claudeparam.EffortHigh {
			t.Errorf("Review().ClaudeEffort() = %v, want the inherited EffortHigh", got)
		}
	})

	// The value of the stage the run skips is not read, so neither an
	// accepted nor an unaccepted effort part reaches its selection or rejects
	// the run: the skipped stage holds the zero selection.
	skippedClaude := []struct {
		name string
		env  func(suffix string) map[string]string
		need Need
		// skipped returns the selection of the stage the run skips.
		skipped func(LLMConfig) LLMSelection
	}{
		{
			"generation only with a claude review",
			func(suffix string) map[string]string {
				return with(validEnv(), reviewLLMEnv, "claude/claude-review"+suffix)
			},
			Need{Generation: true},
			LLMConfig.Review,
		},
		{
			"review only with a claude generation",
			func(suffix string) map[string]string {
				env := with(validClaudeEnv(), llmEnv, "claude/claude-opus-5-5"+suffix)
				env = with(env, reviewLLMEnv, "deepseek/deepseek-review")
				return with(env, deepSeekAPIKeyEnv, "sk-test-key")
			},
			Need{Review: true},
			LLMConfig.Generation,
		},
	}
	for _, tc := range skippedClaude {
		for _, suffix := range []string{"", "+low", "+ultra"} {
			t.Run(tc.name+" "+suffix, func(t *testing.T) {
				cfg, err := LoadFor(envLookup(tc.env(suffix)), tc.need)
				if err != nil {
					t.Fatalf("LoadFor() error = %v, want nil", err)
				}
				if got := tc.skipped(cfg.LLM()); got != (LLMSelection{}) {
					t.Errorf("skipped stage selection = %+v, want the zero LLMSelection", got)
				}
			})
		}
	}

	t.Run("no stage reads no LLM variable", func(t *testing.T) {
		// --after review: not even an invalid LLM value is read, and with no
		// LLM variable at all no default is applied. Only the presence of the
		// retired variables is checked, whatever the stages.
		envs := map[string]map[string]string{
			"invalid values": {
				llmEnv:       "gemini/x",
				reviewLLMEnv: "",
				cacheDirEnv:  "/tmp/yt2column-test-cache",
			},
			"no LLM variable": {
				cacheDirEnv: "/tmp/yt2column-test-cache",
			},
		}
		llmVars := []string{
			llmEnv, reviewLLMEnv,
			deepSeekAPIKeyEnv, anthropicAPIKeyEnv, anthropicWorkspaceIDEnv,
		}
		for name, env := range envs {
			t.Run(name, func(t *testing.T) {
				var read []string
				cfg, err := LoadFor(recordingLookup(env, &read), Need{})
				if err != nil {
					t.Fatalf("LoadFor() error = %v, want nil", err)
				}
				// The positive checks keep the negative one meaningful: a lookup
				// that recorded nothing would pass it.
				if !slices.Contains(read, cacheDirEnv) {
					t.Fatalf("read = %v, want it to include %s", read, cacheDirEnv)
				}
				for _, v := range retiredLLMVars {
					if !slices.Contains(read, v.name) {
						t.Errorf("LoadFor(Need{}) did not check the retired %s", v.name)
					}
				}
				for _, name := range llmVars {
					if slices.Contains(read, name) {
						t.Errorf("LoadFor(Need{}) read %s", name)
					}
				}
				llm := cfg.LLM()
				if llm.Generation().Provider() != ProviderUnset || llm.Review().Provider() != ProviderUnset {
					t.Errorf("providers = %v/%v, want ProviderUnset/ProviderUnset", llm.Generation().Provider(), llm.Review().Provider())
				}
				if llm.Generation().Model() != "" || llm.Review().Model() != "" {
					t.Errorf("models = %q/%q, want empty", llm.Generation().Model(), llm.Review().Model())
				}
			})
		}
	})

	t.Run("non-LLM variables are still checked", func(t *testing.T) {
		env := map[string]string{cacheDirEnv: "relative"}
		_, err := LoadFor(envLookup(env), Need{})
		assertVarError(t, err, ErrInvalid, cacheDirEnv)
	})

	t.Run("review only ignores an unused generation provider's variables", func(t *testing.T) {
		// The review step has its own deepseek selection, so the claude
		// generation provider and its API key and workspace are not read even
		// though YT2COLUMN_LLM is set.
		env := map[string]string{
			llmEnv:            "claude/claude-opus-5-5",
			reviewLLMEnv:      "deepseek/deepseek-review",
			deepSeekAPIKeyEnv: "sk-test-key",
			cacheDirEnv:       "/tmp/yt2column-test-cache",
		}
		cfg, err := LoadFor(envLookup(env), Need{Review: true})
		if err != nil {
			t.Fatalf("LoadFor() error = %v, want nil (the unused claude provider is not checked)", err)
		}
		if cfg.LLM().Review().Provider() != ProviderDeepSeek {
			t.Errorf("Review().Provider() = %v, want ProviderDeepSeek", cfg.LLM().Review().Provider())
		}
	})
}

// defaultsBaseEnv is an environment with no LLM variable: the API keys of both
// providers and the non-LLM variable Load needs. A row adds the LLM variables
// it is about.
func defaultsBaseEnv() map[string]string {
	return map[string]string{
		anthropicAPIKeyEnv: "sk-ant-test-key",
		deepSeekAPIKeyEnv:  "sk-test-key",
		cacheDirEnv:        "/tmp/yt2column-test-cache",
	}
}

// withVars returns a copy of env with vars added.
func withVars(env, vars map[string]string) map[string]string {
	out := maps.Clone(env)
	maps.Copy(out, vars)
	return out
}

// wantMissing is the rejection of an unset or empty variable with no default.
func wantMissing(name string) VarError {
	return VarError{Name: name, Reason: reasonUnset, Err: ErrMissing}
}

// wantInvalid is the rejection of a present variable with the fixed reason.
func wantInvalid(name, reason string) VarError {
	return VarError{Name: name, Reason: reason, Err: ErrInvalid}
}

// The Need values of the run kinds: a run that generates (with the review
// configuration read, as without --after), a generation-only run, and a
// review-only resume.
var (
	needBoth       = Need{Generation: true, Review: true}
	needGeneration = Need{Generation: true}
	needReview     = Need{Review: true}
)

// TestLoadGenerationDefaults checks how the generation selection is resolved:
// an unset YT2COLUMN_LLM selects the default, a set one selects its own
// provider and model, and an empty one is rejected rather than defaulted. Each
// row runs both with and without the review stage, which does not change how
// the generation stage resolves. The value shapes are in
// TestLoadParsesLLMValue.
func TestLoadGenerationDefaults(t *testing.T) {
	const otherModel = "claude-other-test"
	cases := []struct {
		name string
		vars map[string]string
		// wantProvider and wantModel are checked only when wantErrs is nil.
		wantProvider Provider
		wantModel    string
		wantErrs     []VarError
	}{
		{"unset", nil, ProviderClaude, defaultClaudeModel, nil},
		{"claude, other model", map[string]string{llmEnv: "claude/" + otherModel}, ProviderClaude, otherModel, nil},
		{"deepseek", map[string]string{llmEnv: "deepseek/deepseek-chat"}, ProviderDeepSeek, "deepseek-chat", nil},
		{"empty", map[string]string{llmEnv: ""}, ProviderUnset, "", []VarError{wantMissing(llmEnv)}},
	}
	for _, need := range []Need{needBoth, needGeneration} {
		for _, tc := range cases {
			t.Run(fmt.Sprintf("%+v %s", need, tc.name), func(t *testing.T) {
				cfg, err := LoadFor(envLookup(withVars(defaultsBaseEnv(), tc.vars)), need)
				assertVarErrors(t, err, tc.wantErrs...)
				if err != nil {
					return
				}
				if got := cfg.LLM().Generation().Provider(); got != tc.wantProvider {
					t.Errorf("Provider() = %v, want %v", got, tc.wantProvider)
				}
				if got := cfg.LLM().Generation().Model(); got != tc.wantModel {
					t.Errorf("Model() = %q, want %q", got, tc.wantModel)
				}
			})
		}
	}
}

// TestLoadReviewDefaults checks how each stage's selection is resolved for
// each Need (architecture section 3.3), through all four accessors: a stage
// the run does not need holds the zero selection; an unset
// YT2COLUMN_REVIEW_LLM inherits the generation selection, the default
// included, and an inherited rejection is reported once, naming
// YT2COLUMN_LLM; a set YT2COLUMN_REVIEW_LLM is used on its own, and an empty
// one is rejected rather than inherited. Need{} is in TestLoadFor.
func TestLoadReviewDefaults(t *testing.T) {
	type want struct {
		provider       Provider
		model          string
		reviewProvider Provider
		reviewModel    string
	}
	const reviewValue = "deepseek/deepseek-review"
	allDefault := want{ProviderClaude, defaultClaudeModel, ProviderClaude, defaultClaudeModel}
	cases := []struct {
		name     string
		need     Need
		vars     map[string]string
		want     want
		wantErrs []VarError
	}{
		{"both, all unset", needBoth, nil, allDefault, nil},
		{
			"both, review set", needBoth,
			map[string]string{reviewLLMEnv: reviewValue},
			want{ProviderClaude, defaultClaudeModel, ProviderDeepSeek, "deepseek-review"},
			nil,
		},
		{
			"both, review unset inherits LLM", needBoth,
			map[string]string{llmEnv: "deepseek/deepseek-chat"},
			want{ProviderDeepSeek, "deepseek-chat", ProviderDeepSeek, "deepseek-chat"},
			nil,
		},
		{
			"both, review unset, LLM rejected once", needBoth,
			map[string]string{llmEnv: "gemini/x"},
			want{},
			[]VarError{wantInvalid(llmEnv, reasonLLMProvider)},
		},
		{"both, review empty", needBoth, map[string]string{reviewLLMEnv: ""}, want{}, []VarError{wantMissing(reviewLLMEnv)}},
		{
			"generation only, review not read", needGeneration,
			map[string]string{reviewLLMEnv: ""},
			want{ProviderClaude, defaultClaudeModel, ProviderUnset, ""},
			nil,
		},
		{
			"review only, review set, LLM not read", needReview,
			map[string]string{reviewLLMEnv: reviewValue, llmEnv: "gemini/x"},
			want{ProviderUnset, "", ProviderDeepSeek, "deepseek-review"},
			nil,
		},
		{"review only, all unset", needReview, nil, want{ProviderUnset, "", ProviderClaude, defaultClaudeModel}, nil},
		{
			"review only, review unset inherits LLM", needReview,
			map[string]string{llmEnv: "deepseek/deepseek-chat"},
			want{ProviderUnset, "", ProviderDeepSeek, "deepseek-chat"},
			nil,
		},
		{"review only, review unset, LLM empty", needReview, map[string]string{llmEnv: ""}, want{}, []VarError{wantMissing(llmEnv)}},
		{
			"review only, review unset, LLM provider invalid", needReview,
			map[string]string{llmEnv: "gemini/x"},
			want{},
			[]VarError{wantInvalid(llmEnv, reasonLLMProvider)},
		},
		{
			"review only, review unset, LLM model ID empty", needReview,
			map[string]string{llmEnv: "deepseek/"},
			want{},
			[]VarError{wantInvalid(llmEnv, reasonLLMForm)},
		},
		{"review only, review empty", needReview, map[string]string{reviewLLMEnv: ""}, want{}, []VarError{wantMissing(reviewLLMEnv)}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := LoadFor(envLookup(withVars(defaultsBaseEnv(), tc.vars)), tc.need)
			assertVarErrors(t, err, tc.wantErrs...)
			if err != nil {
				return
			}
			llm := cfg.LLM()
			got := want{llm.Generation().Provider(), llm.Generation().Model(), llm.Review().Provider(), llm.Review().Model()}
			if got != tc.want {
				t.Errorf("provider/model/review provider/review model = %+v, want %+v", got, tc.want)
			}
		})
	}
}

// TestLoadDefaultProviderVariables checks that a run whose providers come from
// the default reads and checks the claude provider's variables and never reads
// DEEPSEEK_API_KEY, whose empty value here would be rejected if it were read.
func TestLoadDefaultProviderVariables(t *testing.T) {
	cases := []struct {
		name     string
		need     Need
		env      map[string]string
		wantErrs []VarError
	}{
		{"generation and review", needBoth, map[string]string{anthropicAPIKeyEnv: "sk-ant-test-key"}, nil},
		{"generation only", needGeneration, map[string]string{anthropicAPIKeyEnv: "sk-ant-test-key"}, nil},
		{"review only", needReview, map[string]string{anthropicAPIKeyEnv: "sk-ant-test-key"}, nil},
		{"API key unset", needBoth, nil, []VarError{wantMissing(anthropicAPIKeyEnv)}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env := map[string]string{
				deepSeekAPIKeyEnv: "",
				cacheDirEnv:       "/tmp/yt2column-test-cache",
			}
			env = withVars(env, tc.env)
			var read []string
			lookup := func(name string) (string, bool) {
				read = append(read, name)
				return envLookup(env)(name)
			}
			_, err := LoadFor(lookup, tc.need)
			assertVarErrors(t, err, tc.wantErrs...)
			for _, name := range []string{anthropicAPIKeyEnv, anthropicWorkspaceIDEnv} {
				if !slices.Contains(read, name) {
					t.Errorf("LoadFor() did not read %s", name)
				}
			}
			if slices.Contains(read, deepSeekAPIKeyEnv) {
				t.Errorf("LoadFor() read %s, but no stage uses deepseek", deepSeekAPIKeyEnv)
			}
		})
	}
}

// TestLoadServerLLMDefaults checks the server entry point with no LLM
// variable. The rules themselves are covered by the Load tests, which
// LoadServer shares; the retired variables at the server entry point are in
// TestLoadRejectsRetiredLLMVars.
func TestLoadServerLLMDefaults(t *testing.T) {
	env := map[string]string{
		"HOME":             "/home/user",
		slackEnv:           "https://hooks.slack.com/services/T000/B000/XXXX",
		anthropicAPIKeyEnv: "sk-ant-test-key",
	}
	cfg, err := LoadServer(envLookup(env))
	if err != nil {
		t.Fatalf("LoadServer() error = %v, want nil", err)
	}
	llm := cfg.LLM()
	if llm.Generation().Provider() != ProviderClaude || llm.Review().Provider() != ProviderClaude {
		t.Errorf("providers = %v/%v, want ProviderClaude/ProviderClaude", llm.Generation().Provider(), llm.Review().Provider())
	}
	if llm.Generation().Model() != defaultClaudeModel || llm.Review().Model() != defaultClaudeModel {
		t.Errorf("models = %q/%q, want %q", llm.Generation().Model(), llm.Review().Model(), defaultClaudeModel)
	}
}

// TestLoadProductionDefaults checks that, with the Anthropic API key as the
// only LLM variable, every stage uses the production configuration: the claude
// provider, the default model, and the high effort.
func TestLoadProductionDefaults(t *testing.T) {
	env := map[string]string{
		anthropicAPIKeyEnv: "sk-ant-test-key",
		cacheDirEnv:        "/tmp/yt2column-test-cache",
	}
	cfg, err := Load(envLookup(env))
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}
	llm := cfg.LLM()
	if llm.Generation().Provider() != ProviderClaude || llm.Review().Provider() != ProviderClaude {
		t.Errorf("providers = %v/%v, want ProviderClaude/ProviderClaude", llm.Generation().Provider(), llm.Review().Provider())
	}
	if llm.Generation().Model() != defaultClaudeModel || llm.Review().Model() != defaultClaudeModel {
		t.Errorf("models = %q/%q, want %q", llm.Generation().Model(), llm.Review().Model(), defaultClaudeModel)
	}
	assertClaudeEfforts(t, llm, claudeparam.EffortHigh, claudeparam.EffortHigh)
}

// TestLoadParsesLLMValue checks each value shape of architecture section 3.2
// for both LLM variables: an accepted value selects its provider, the model ID
// after the first "/" and before the "+" as it is, white space and invalid
// UTF-8 included (the adapters reject those when a client is built), and the
// effort part, or the provider's default without one; a rejected value names
// the variable with its sentinel and fixed reason, once, for the first rule it
// breaks.
func TestLoadParsesLLMValue(t *testing.T) {
	// claude and deepSeek build an accepted row's selection.
	claude := func(model string, effort claudeparam.Effort) LLMSelection {
		return LLMSelection{provider: ProviderClaude, model: model, claudeEffort: effort}
	}
	deepSeek := func(model string, effort deepseekparam.Effort) LLMSelection {
		return LLMSelection{provider: ProviderDeepSeek, model: model, deepSeekEffort: effort}
	}
	rejected := func(reason string) *VarError { return &VarError{Reason: reason, Err: ErrInvalid} }
	cases := []struct {
		value   string
		want    LLMSelection
		wantErr *VarError // nil when accepted; Name is filled per variable
	}{
		// Values without "+": the effort is the provider's default.
		{"deepseek/deepseek-flash", deepSeek("deepseek-flash", deepseekparam.EffortUnspecified), nil},
		{"claude/claude-sonnet-5-5", claude("claude-sonnet-5-5", claudeparam.EffortHigh), nil},
		{"claude/vendor/model-x", claude("vendor/model-x", claudeparam.EffortHigh), nil},
		{"claude/claude-opus-5-5 ", claude("claude-opus-5-5 ", claudeparam.EffortHigh), nil},
		{"claude/ claude-opus-5-5", claude(" claude-opus-5-5", claudeparam.EffortHigh), nil},
		{"claude/\xffmodel", claude("\xffmodel", claudeparam.EffortHigh), nil},
		{"", LLMSelection{}, &VarError{Reason: reasonUnset, Err: ErrMissing}},
		{"claude", LLMSelection{}, rejected(reasonLLMForm)},
		{"claude/", LLMSelection{}, rejected(reasonLLMForm)},
		{"deepseek/", LLMSelection{}, rejected(reasonLLMForm)},
		{"/claude-opus-5-5", LLMSelection{}, rejected(reasonLLMProvider)},
		{"gemini/x", LLMSelection{}, rejected(reasonLLMProvider)},
		{"gemini/", LLMSelection{}, rejected(reasonLLMProvider)},
		{"Claude/claude-opus-5-5", LLMSelection{}, rejected(reasonLLMProvider)},
		{" claude/claude-opus-5-5", LLMSelection{}, rejected(reasonLLMProvider)},
		{"claude /claude-opus-5-5", LLMSelection{}, rejected(reasonLLMProvider)},
		// Every effort each provider accepts.
		{"claude/claude-sonnet-5-5+low", claude("claude-sonnet-5-5", claudeparam.EffortLow), nil},
		{"claude/claude-opus-5-5+medium", claude("claude-opus-5-5", claudeparam.EffortMedium), nil},
		{"claude/claude-opus-5-5+high", claude("claude-opus-5-5", claudeparam.EffortHigh), nil},
		{"claude/claude-opus-5-5+xhigh", claude("claude-opus-5-5", claudeparam.EffortXHigh), nil},
		{"claude/claude-opus-5-5+max", claude("claude-opus-5-5", claudeparam.EffortMax), nil},
		{"deepseek/deepseek-flash+none", deepSeek("deepseek-flash", deepseekparam.EffortNone), nil},
		{"deepseek/deepseek-flash+low", deepSeek("deepseek-flash", deepseekparam.EffortLow), nil},
		{"deepseek/deepseek-flash+high", deepSeek("deepseek-flash", deepseekparam.EffortHigh), nil},
		{"deepseek/deepseek-flash+max", deepSeek("deepseek-flash", deepseekparam.EffortMax), nil},
		// "+" is looked for after the first "/" only, and the model ID keeps
		// its later "/" and its white space.
		{"claude/vendor/model-x+medium", claude("vendor/model-x", claudeparam.EffortMedium), nil},
		{"claude/claude-opus-5-5 +high", claude("claude-opus-5-5 ", claudeparam.EffortHigh), nil},
		{"claude+high/claude-opus-5-5", LLMSelection{}, rejected(reasonLLMProvider)},
		// Too many "+".
		{"claude/claude-opus-5-5+high+low", LLMSelection{}, rejected(reasonLLMEffortSeparator)},
		{"claude/claude-opus-5-5++high", LLMSelection{}, rejected(reasonLLMEffortSeparator)},
		// An empty effort part, or one the provider does not accept exactly.
		{"claude/claude-opus-5-5+", LLMSelection{}, rejected(reasonClaudeEffortPart)},
		{"deepseek/deepseek-flash+", LLMSelection{}, rejected(reasonDeepSeekEffortPart)},
		{"claude/claude-opus-5-5+High", LLMSelection{}, rejected(reasonClaudeEffortPart)},
		{"claude/claude-opus-5-5+ high", LLMSelection{}, rejected(reasonClaudeEffortPart)},
		{"claude/claude-opus-5-5+high ", LLMSelection{}, rejected(reasonClaudeEffortPart)},
		{"claude/claude-opus-5-5+none", LLMSelection{}, rejected(reasonClaudeEffortPart)},
		{"claude/model+x/y", LLMSelection{}, rejected(reasonClaudeEffortPart)},
		{"deepseek/deepseek-flash+medium", LLMSelection{}, rejected(reasonDeepSeekEffortPart)},
		{"deepseek/deepseek-flash+xhigh", LLMSelection{}, rejected(reasonDeepSeekEffortPart)},
		{"deepseek/deepseek-flash+ultra", LLMSelection{}, rejected(reasonDeepSeekEffortPart)},
		// An empty model ID with an effort.
		{"claude/+high", LLMSelection{}, rejected(reasonLLMForm)},
		// Values breaking several rules: the first rule in the order decides.
		{"claude/+", LLMSelection{}, rejected(reasonLLMForm)},
		{"claude/+ultra", LLMSelection{}, rejected(reasonLLMForm)},
		{"deepseek/deepseek-flash+a+b", LLMSelection{}, rejected(reasonLLMEffortSeparator)},
	}
	stages := []struct {
		name     string
		selected func(LLMConfig) LLMSelection
	}{
		{llmEnv, LLMConfig.Generation},
		{reviewLLMEnv, LLMConfig.Review},
	}
	for _, stage := range stages {
		for _, tc := range cases {
			t.Run(fmt.Sprintf("%s=%q", stage.name, tc.value), func(t *testing.T) {
				cfg, err := Load(envLookup(with(defaultsBaseEnv(), stage.name, tc.value)))
				if tc.wantErr != nil {
					want := *tc.wantErr
					want.Name = stage.name
					assertVarErrors(t, err, want)
					return
				}
				if err != nil {
					t.Fatalf("Load() error = %v, want nil", err)
				}
				if got := stage.selected(cfg.LLM()); got != tc.want {
					t.Errorf("selection = %+v, want %+v", got, tc.want)
				}
			})
		}
	}
}

// TestParseLLMProvider checks parseLLM's contract on a rejected value: the
// selection keeps a provider part that was accepted, so that provider's own
// variables are still checked, with no model ID and no effort, and holds
// ProviderUnset otherwise.
func TestParseLLMProvider(t *testing.T) {
	cases := []struct {
		value string
		want  Provider
	}{
		{"claude/", ProviderClaude},
		{"deepseek/", ProviderDeepSeek},
		{"claude/+high", ProviderClaude},
		{"claude/claude-opus-5-5+high+low", ProviderClaude},
		{"claude/claude-opus-5-5+ultra", ProviderClaude},
		{"deepseek/deepseek-flash+a+b", ProviderDeepSeek},
		{"deepseek/deepseek-flash+medium", ProviderDeepSeek},
		{"", ProviderUnset},
		{"claude", ProviderUnset},
		{"gemini/x", ProviderUnset},
	}
	for _, tc := range cases {
		t.Run(fmt.Sprintf("%q", tc.value), func(t *testing.T) {
			selection, err := parseLLM(llmEnv, tc.value)
			if err == nil {
				t.Fatal("parseLLM() error = nil, want a rejection")
			}
			if want := (LLMSelection{provider: tc.want}); selection != want {
				t.Errorf("parseLLM() = %+v, want %+v", selection, want)
			}
		})
	}
}

// TestEffortPartReasonsListValues checks that each effort-part rejection lists
// exactly the values its provider's parameter package accepts, in order. The
// list is split out of the reason rather than searched for: "high" is part of
// "xhigh", so a substring search would pass with "high" missing.
func TestEffortPartReasonsListValues(t *testing.T) {
	cases := []struct {
		name   string
		reason string
		want   []string
	}{
		{providerClaude, reasonClaudeEffortPart, claudeparam.EffortValues()},
		{providerDeepSeek, reasonDeepSeekEffortPart, deepseekparam.EffortValues()},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, list, found := strings.Cut(tc.reason, effortValuesPrefix)
			if !found {
				t.Fatalf("reason %q has no %q", tc.reason, effortValuesPrefix)
			}
			if got := strings.Split(list, effortValuesSeparator); !slices.Equal(got, tc.want) {
				t.Errorf("listed values = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestRetiredClaudeEffortReason checks the code values the rejection of
// YT2COLUMN_CLAUDE_EFFORT names: both LLM variables, the claude provider the
// effort moves to, the value to set while YT2COLUMN_LLM is unset, and the
// claude default effort. The default effort is searched for with the words in
// front of it, because "high" is part of "xhigh".
func TestRetiredClaudeEffortReason(t *testing.T) {
	fragments := []string{
		llmEnv,
		reviewLLMEnv,
		"whose provider is " + providerClaude,
		llmEnv + "=" + defaultLLMValue + effortSeparator + "<effort>",
		"a " + providerClaude + " stage uses " + defaultClaudeEffort.String(),
	}
	for _, fragment := range fragments {
		if !strings.Contains(reasonRetiredClaudeEffort, fragment) {
			t.Errorf("reasonRetiredClaudeEffort = %q, want it to contain %q", reasonRetiredClaudeEffort, fragment)
		}
	}
}

// TestLoadInheritsEffort checks the effort of each stage for each Need: an
// unset YT2COLUMN_REVIEW_LLM inherits the generation selection, effort
// included, and an inherited rejection is reported once; a set one uses its
// own value, whose omitted effort is the provider's default rather than the
// generation's. A stage the run does not need holds the zero selection.
func TestLoadInheritsEffort(t *testing.T) {
	claude := func(model string, effort claudeparam.Effort) LLMSelection {
		return LLMSelection{provider: ProviderClaude, model: model, claudeEffort: effort}
	}
	deepSeek := func(effort deepseekparam.Effort) LLMSelection {
		return LLMSelection{provider: ProviderDeepSeek, model: "deepseek-flash", deepSeekEffort: effort}
	}
	cases := []struct {
		name                       string
		need                       Need
		vars                       map[string]string
		wantGeneration, wantReview LLMSelection
		wantErrs                   []VarError
	}{
		{
			"both, review unset inherits the effort", needBoth,
			map[string]string{llmEnv: "claude/claude-sonnet-5-5+medium"},
			claude("claude-sonnet-5-5", claudeparam.EffortMedium), claude("claude-sonnet-5-5", claudeparam.EffortMedium), nil,
		},
		{
			"both, review unset inherits no effort", needBoth,
			map[string]string{llmEnv: "claude/claude-sonnet-5-5"},
			claude("claude-sonnet-5-5", claudeparam.EffortHigh), claude("claude-sonnet-5-5", claudeparam.EffortHigh), nil,
		},
		{
			"both, review unset inherits a deepseek effort", needBoth,
			map[string]string{llmEnv: "deepseek/deepseek-flash+max"},
			deepSeek(deepseekparam.EffortMax), deepSeek(deepseekparam.EffortMax), nil,
		},
		{
			"both, review set without an effort uses the claude default", needBoth,
			map[string]string{llmEnv: "claude/claude-opus-5-5+low", reviewLLMEnv: "claude/claude-opus-5-5"},
			claude("claude-opus-5-5", claudeparam.EffortLow), claude("claude-opus-5-5", claudeparam.EffortHigh), nil,
		},
		{
			"both, review set without an effort sends none to deepseek", needBoth,
			map[string]string{llmEnv: "deepseek/deepseek-flash+low", reviewLLMEnv: "deepseek/deepseek-flash"},
			deepSeek(deepseekparam.EffortLow), deepSeek(deepseekparam.EffortUnspecified), nil,
		},
		{
			"both, review unset, LLM effort rejected once", needBoth,
			map[string]string{llmEnv: "claude/claude-opus-5-5+ultra"},
			LLMSelection{},
			LLMSelection{},
			[]VarError{wantInvalid(llmEnv, reasonClaudeEffortPart)},
		},
		{
			"generation only, review not read", needGeneration,
			map[string]string{llmEnv: "claude/claude-opus-5-5+low", reviewLLMEnv: "claude/claude-opus-5-5+ultra"},
			claude("claude-opus-5-5", claudeparam.EffortLow),
			LLMSelection{},
			nil,
		},
		{
			"review only, review unset inherits the effort", needReview,
			map[string]string{llmEnv: "claude/claude-sonnet-5-5+medium"},
			LLMSelection{},
			claude("claude-sonnet-5-5", claudeparam.EffortMedium), nil,
		},
		{
			"review only, review unset inherits a deepseek effort", needReview,
			map[string]string{llmEnv: "deepseek/deepseek-flash+none"},
			LLMSelection{},
			deepSeek(deepseekparam.EffortNone), nil,
		},
		{
			"review only, review set without an effort uses the claude default", needReview,
			map[string]string{llmEnv: "claude/claude-opus-5-5+low", reviewLLMEnv: "claude/claude-opus-5-5"},
			LLMSelection{},
			claude("claude-opus-5-5", claudeparam.EffortHigh), nil,
		},
		{
			"review only, review unset, LLM effort rejected", needReview,
			map[string]string{llmEnv: "claude/claude-opus-5-5+ultra"},
			LLMSelection{},
			LLMSelection{},
			[]VarError{wantInvalid(llmEnv, reasonClaudeEffortPart)},
		},
		{
			"review only, review set, LLM not read", needReview,
			map[string]string{llmEnv: "claude/claude-opus-5-5+ultra", reviewLLMEnv: "deepseek/deepseek-flash+high"},
			LLMSelection{},
			deepSeek(deepseekparam.EffortHigh), nil,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := LoadFor(envLookup(withVars(defaultsBaseEnv(), tc.vars)), tc.need)
			assertVarErrors(t, err, tc.wantErrs...)
			if err != nil {
				return
			}
			if got := cfg.LLM().Generation(); got != tc.wantGeneration {
				t.Errorf("Generation() = %+v, want %+v", got, tc.wantGeneration)
			}
			if got := cfg.LLM().Review(); got != tc.wantReview {
				t.Errorf("Review() = %+v, want %+v", got, tc.wantReview)
			}
		})
	}
}

// TestLoadReviewOnlyReadsLLM checks, by the variables read, when a review-only
// run reads YT2COLUMN_LLM: not when YT2COLUMN_REVIEW_LLM is set, and when it
// is unset, because the review step inherits the generation selection.
func TestLoadReviewOnlyReadsLLM(t *testing.T) {
	cases := []struct {
		name     string
		env      map[string]string
		wantRead bool
	}{
		{"review set", with(defaultsBaseEnv(), reviewLLMEnv, "deepseek/deepseek-review"), false},
		{"review unset", defaultsBaseEnv(), true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var read []string
			if _, err := LoadFor(recordingLookup(tc.env, &read), needReview); err != nil {
				t.Fatalf("LoadFor() error = %v, want nil", err)
			}
			if !slices.Contains(read, reviewLLMEnv) {
				t.Fatalf("read = %v, want it to include %s", read, reviewLLMEnv)
			}
			if got := slices.Contains(read, llmEnv); got != tc.wantRead {
				t.Errorf("read %s = %v, want %v (read = %v)", llmEnv, got, tc.wantRead, read)
			}
		})
	}
}

// retiredLLMVarsWant is the retired variables and the replacement each one's
// reason names, written out here rather than read from retiredLLMVars, so a
// variable dropped from that table fails TestRetiredLLMVars instead of
// shrinking the cases of TestLoadRejectsRetiredLLMVars with it. The order is
// pinned too: the CLI's and the server's tests take YT2COLUMN_CLAUDE_EFFORT as
// the last entry of RetiredLLMVarsForTest.
var retiredLLMVarsWant = []retiredLLMVar{
	{"YT2COLUMN_LLM_PROVIDER", reasonRetiredLLM},
	{"YT2COLUMN_MODEL", reasonRetiredLLM},
	{"YT2COLUMN_REVIEW_PROVIDER", reasonRetiredReviewLLM},
	{"YT2COLUMN_REVIEW_MODEL", reasonRetiredReviewLLM},
	{"YT2COLUMN_CLAUDE_EFFORT", reasonRetiredClaudeEffort},
}

// TestRetiredLLMVars checks the table of retired variables against the list
// above, the reporting order included.
func TestRetiredLLMVars(t *testing.T) {
	if !slices.Equal(retiredLLMVars, retiredLLMVarsWant) {
		t.Errorf("retiredLLMVars = %v, want %v", retiredLLMVars, retiredLLMVarsWant)
	}
}

// TestLoadRejectsRetiredLLMVars checks that a retired variable, set with a
// value or empty, stops every load whatever the stages, and the server load,
// with one ErrInvalid naming it and the reason naming its replacement. The
// rows with YT2COLUMN_LLM set check that the retired variable is rejected
// even when the new one is present, so its value can never decide anything.
func TestLoadRejectsRetiredLLMVars(t *testing.T) {
	loaders := []struct {
		name string
		load func(map[string]string) error
	}{
		{"LoadFor both", func(env map[string]string) error { _, err := LoadFor(envLookup(env), needBoth); return err }},
		{"LoadFor generation", func(env map[string]string) error { _, err := LoadFor(envLookup(env), needGeneration); return err }},
		{"LoadFor review", func(env map[string]string) error { _, err := LoadFor(envLookup(env), needReview); return err }},
		{"LoadFor none", func(env map[string]string) error { _, err := LoadFor(envLookup(env), Need{}); return err }},
		{"LoadServer", func(env map[string]string) error {
			env = withVars(without(env, cacheDirEnv), map[string]string{
				"HOME":   "/home/user",
				slackEnv: "https://hooks.slack.com/services/T000/B000/XXXX",
			})
			_, err := LoadServer(envLookup(env))
			return err
		}},
	}
	bases := []struct {
		name string
		env  map[string]string
	}{
		{"LLM unset", defaultsBaseEnv()},
		{"LLM set", with(defaultsBaseEnv(), llmEnv, "claude/claude-opus-5-5")},
		// Both stages deepseek: the retired claude effort variable stops even
		// a run in which no stage uses claude.
		{"both stages deepseek", withVars(defaultsBaseEnv(), map[string]string{
			llmEnv: "deepseek/deepseek-flash", reviewLLMEnv: "deepseek/deepseek-flash",
		})},
	}
	for _, v := range retiredLLMVarsWant {
		for _, value := range []string{"deepseek", ""} {
			for _, base := range bases {
				for _, loader := range loaders {
					t.Run(fmt.Sprintf("%s=%q %s %s", v.name, value, base.name, loader.name), func(t *testing.T) {
						err := loader.load(with(base.env, v.name, value))
						assertVarErrors(t, err, wantInvalid(v.name, v.reason))
					})
				}
			}
		}
	}
}

// TestLoadProviderVarsFollowSelection checks which provider variables a load
// reads when an LLM variable is rejected: none for a stage whose provider part
// was rejected, and the provider's own when the provider part was accepted
// (an empty model ID, too many "+", or an unaccepted effort part). The
// selection that is accepted still has its provider's variables checked.
func TestLoadProviderVarsFollowSelection(t *testing.T) {
	claudeVars := []string{anthropicAPIKeyEnv, anthropicWorkspaceIDEnv}
	cases := []struct {
		name        string
		need        Need
		vars        map[string]string
		wantErrs    []VarError
		wantRead    []string
		wantNotRead []string
	}{
		{
			"LLM empty, review inherits", needBoth,
			map[string]string{llmEnv: ""},
			[]VarError{wantMissing(llmEnv)},
			nil, append([]string{deepSeekAPIKeyEnv}, claudeVars...),
		},
		{
			"LLM without a separator, review inherits", needBoth,
			map[string]string{llmEnv: "claude"},
			[]VarError{wantInvalid(llmEnv, reasonLLMForm)},
			nil, append([]string{deepSeekAPIKeyEnv}, claudeVars...),
		},
		{
			"LLM with an empty model ID", needBoth,
			map[string]string{llmEnv: "claude/"},
			[]VarError{wantInvalid(llmEnv, reasonLLMForm), wantMissing(anthropicAPIKeyEnv)},
			claudeVars,
			[]string{deepSeekAPIKeyEnv},
		},
		{
			// The review step inherits the rejected generation value, whose
			// accepted provider part still selects that provider's variables.
			"review only, inherited LLM with an empty model ID", needReview,
			map[string]string{llmEnv: "claude/"},
			[]VarError{wantInvalid(llmEnv, reasonLLMForm), wantMissing(anthropicAPIKeyEnv)},
			claudeVars,
			[]string{deepSeekAPIKeyEnv},
		},
		{
			// Exactly two rejections: the effort part and the key of the
			// provider it still selects.
			"LLM with an unaccepted claude effort part", needBoth,
			map[string]string{llmEnv: "claude/claude-opus-5-5+ultra"},
			[]VarError{wantInvalid(llmEnv, reasonClaudeEffortPart), wantMissing(anthropicAPIKeyEnv)},
			claudeVars,
			[]string{deepSeekAPIKeyEnv},
		},
		{
			"LLM with an unaccepted deepseek effort part", needBoth,
			map[string]string{llmEnv: "deepseek/deepseek-flash+medium"},
			[]VarError{wantInvalid(llmEnv, reasonDeepSeekEffortPart), wantMissing(deepSeekAPIKeyEnv)},
			[]string{deepSeekAPIKeyEnv},
			claudeVars,
		},
		{
			"LLM with an empty model ID and an effort", needBoth,
			map[string]string{llmEnv: "claude/+high"},
			[]VarError{wantInvalid(llmEnv, reasonLLMForm), wantMissing(anthropicAPIKeyEnv)},
			claudeVars,
			[]string{deepSeekAPIKeyEnv},
		},
		{
			"review LLM with two effort separators", needBoth,
			map[string]string{llmEnv: "claude/claude-opus-5-5", reviewLLMEnv: "deepseek/deepseek-flash+a+b"},
			[]VarError{
				wantInvalid(reviewLLMEnv, reasonLLMEffortSeparator),
				wantMissing(anthropicAPIKeyEnv), wantMissing(deepSeekAPIKeyEnv),
			},
			append([]string{deepSeekAPIKeyEnv}, claudeVars...),
			nil,
		},
		{
			"LLM provider invalid, review set", needBoth,
			map[string]string{llmEnv: "gemini/x", reviewLLMEnv: "deepseek/deepseek-flash"},
			[]VarError{wantInvalid(llmEnv, reasonLLMProvider), wantMissing(deepSeekAPIKeyEnv)},
			[]string{deepSeekAPIKeyEnv},
			claudeVars,
		},
		{
			"review provider invalid", needBoth,
			map[string]string{llmEnv: "deepseek/deepseek-flash", reviewLLMEnv: "gemini/x"},
			[]VarError{wantInvalid(reviewLLMEnv, reasonLLMProvider), wantMissing(deepSeekAPIKeyEnv)},
			[]string{deepSeekAPIKeyEnv},
			claudeVars,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// No provider variable is set, so each one read is also reported
			// missing; the rows list those reports in wantErrs.
			env := withVars(map[string]string{cacheDirEnv: "/tmp/yt2column-test-cache"}, tc.vars)
			var read []string
			_, err := LoadFor(recordingLookup(env, &read), tc.need)
			assertVarErrors(t, err, tc.wantErrs...)
			for _, name := range tc.wantRead {
				if !slices.Contains(read, name) {
					t.Errorf("LoadFor() did not read %s", name)
				}
			}
			for _, name := range tc.wantNotRead {
				if slices.Contains(read, name) {
					t.Errorf("LoadFor() read %s", name)
				}
			}
		})
	}
}
