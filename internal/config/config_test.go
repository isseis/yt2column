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
	"strings"
	"testing"

	"github.com/isseis/yt2column/internal/llm/claudeparam"
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
		providerEnv:       "deepseek",
		modelEnv:          "deepseek-chat",
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
		providerEnv:             "claude",
		modelEnv:                "claude-opus-5-5",
		anthropicAPIKeyEnv:      "sk-ant-test-key",
		claudeEffortEnv:         "high",
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
	if cfg.LLM().Provider() != ProviderDeepSeek {
		t.Errorf("Provider() = %v, want ProviderDeepSeek", cfg.LLM().Provider())
	}
	if cfg.LLM().Model() != env[modelEnv] {
		t.Errorf("Model() = %q, want %q", cfg.LLM().Model(), env[modelEnv])
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

func TestLoadDefaults(t *testing.T) {
	home := "/home/user"
	env := map[string]string{
		modelEnv:          "deepseek-chat",
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
	if cfg.LLM().Provider() != ProviderDeepSeek {
		t.Errorf("Provider() = %v, want ProviderDeepSeek", cfg.LLM().Provider())
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
		{"model unset", without(base, modelEnv), modelEnv},
		{"provider unset and API key unset", without(base, providerEnv, deepSeekAPIKeyEnv), deepSeekAPIKeyEnv},
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
	vars := []string{providerEnv, modelEnv, deepSeekAPIKeyEnv, slackEnv, cacheDirEnv, ytDlpEnv}
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
		{"provider", providerEnv, []string{"DeepSeek", " deepseek", "deepseek ", "gemini", "Claude", "anthropic"}},
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

// TestLoadClaude checks that a claude environment loads with every effort
// value and each accessor, with and without a workspace ID.
func TestLoadClaude(t *testing.T) {
	efforts := []struct {
		value string
		want  claudeparam.Effort
	}{
		{"low", claudeparam.EffortLow},
		{"medium", claudeparam.EffortMedium},
		{"high", claudeparam.EffortHigh},
		{"xhigh", claudeparam.EffortXHigh},
		{"max", claudeparam.EffortMax},
	}
	for _, effort := range efforts {
		t.Run(effort.value, func(t *testing.T) {
			env := with(validClaudeEnv(), claudeEffortEnv, effort.value)
			cfg, err := Load(envLookup(env))
			if err != nil {
				t.Fatalf("Load() error = %v, want nil", err)
			}
			if cfg.LLM().Provider() != ProviderClaude {
				t.Errorf("Provider() = %v, want ProviderClaude", cfg.LLM().Provider())
			}
			if cfg.LLM().ClaudeEffort() != effort.want {
				t.Errorf("ClaudeEffort() = %v, want %v", cfg.LLM().ClaudeEffort(), effort.want)
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
	}

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
		{"effort unset", without(validClaudeEnv(), claudeEffortEnv), claudeEffortEnv},
		{"effort empty", with(validClaudeEnv(), claudeEffortEnv, ""), claudeEffortEnv},
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

// TestLoadClaudeInvalid checks that an unusable claude provider variable wraps
// ErrInvalid and names the variable.
func TestLoadClaudeInvalid(t *testing.T) {
	t.Run("effort", func(t *testing.T) {
		for _, value := range []string{"High", " high", "high\n", "none", "min", "medium-high"} {
			t.Run(fmt.Sprintf("%q", value), func(t *testing.T) {
				_, err := Load(envLookup(with(validClaudeEnv(), claudeEffortEnv, value)))
				assertVarError(t, err, ErrInvalid, claudeEffortEnv)
			})
		}
	})
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
	env := with(validClaudeEnv(), claudeEffortEnv, "High")
	env = with(env, anthropicWorkspaceIDEnv, " wrkspc_x")
	env = without(env, anthropicAPIKeyEnv)
	_, err := Load(envLookup(env))
	if !errors.Is(err, ErrMissing) {
		t.Errorf("Load() error = %v, want it to wrap ErrMissing", err)
	}
	if !errors.Is(err, ErrInvalid) {
		t.Errorf("Load() error = %v, want it to wrap ErrInvalid", err)
	}
	for _, name := range []string{anthropicAPIKeyEnv, claudeEffortEnv, anthropicWorkspaceIDEnv} {
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
				if cfg.LLM().Provider() != ProviderClaude {
					t.Errorf("Provider() = %v, want ProviderClaude", cfg.LLM().Provider())
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
// value, and that an invalid provider is reported alone.
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
		{claudeEffortEnv, []variant{
			{"unset", "", false},
			{"empty", "", true},
			{"invalid", "High", true},
			{"valid", "high", true},
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
				if cfg.LLM().Provider() != ProviderDeepSeek {
					t.Errorf("Provider() = %v, want ProviderDeepSeek", cfg.LLM().Provider())
				}
				if _, err := cfg.LLM().AnthropicAPIKey().Reveal(); err == nil {
					t.Error("AnthropicAPIKey() is set for the deepseek provider")
				}
				if cfg.LLM().ClaudeEffort() != claudeparam.EffortUnset {
					t.Errorf("ClaudeEffort() = %v, want EffortUnset", cfg.LLM().ClaudeEffort())
				}
				if _, ok := cfg.LLM().AnthropicWorkspaceID().Value(); ok {
					t.Error("AnthropicWorkspaceID() is set for the deepseek provider")
				}
			})
		}
	}

	t.Run("invalid provider reports only the provider", func(t *testing.T) {
		env := with(validEnv(), providerEnv, "gemini")
		env = with(env, anthropicAPIKeyEnv, "sk-ant-key")
		env = with(env, claudeEffortEnv, "High")
		env = with(env, anthropicWorkspaceIDEnv, " wrkspc_x")
		_, err := Load(envLookup(env))
		if !errors.Is(err, ErrInvalid) {
			t.Fatalf("Load() error = %v, want ErrInvalid", err)
		}
		if !strings.Contains(err.Error(), providerEnv) {
			t.Errorf("Load() error = %q, want it to name %q", err, providerEnv)
		}
		for _, name := range []string{anthropicAPIKeyEnv, claudeEffortEnv, anthropicWorkspaceIDEnv} {
			if strings.Contains(err.Error(), name) {
				t.Errorf("Load() error = %q, must not name %q", err, name)
			}
		}
	})
}

// TestLoadReview checks the review provider and model selection rules: both
// variables unset inherit the generation settings, a model alone reuses the
// generation provider, and a provider of its own requires a model.
func TestLoadReview(t *testing.T) {
	t.Run("both unset inherit the generation settings", func(t *testing.T) {
		cfg, err := Load(envLookup(validEnv()))
		if err != nil {
			t.Fatalf("Load() error = %v, want nil", err)
		}
		if cfg.LLM().ReviewProvider() != cfg.LLM().Provider() {
			t.Errorf("ReviewProvider() = %v, want Provider() = %v", cfg.LLM().ReviewProvider(), cfg.LLM().Provider())
		}
		if cfg.LLM().ReviewModel() != cfg.LLM().Model() {
			t.Errorf("ReviewModel() = %q, want Model() = %q", cfg.LLM().ReviewModel(), cfg.LLM().Model())
		}
	})

	t.Run("model only reuses the generation provider", func(t *testing.T) {
		const reviewModel = "deepseek-review"
		env := with(validEnv(), reviewModelEnv, reviewModel)
		cfg, err := Load(envLookup(env))
		if err != nil {
			t.Fatalf("Load() error = %v, want nil", err)
		}
		if cfg.LLM().ReviewProvider() != ProviderDeepSeek {
			t.Errorf("ReviewProvider() = %v, want ProviderDeepSeek", cfg.LLM().ReviewProvider())
		}
		if cfg.LLM().ReviewModel() != reviewModel {
			t.Errorf("ReviewModel() = %q, want %q", cfg.LLM().ReviewModel(), reviewModel)
		}
	})

	t.Run("provider and model set to a different provider", func(t *testing.T) {
		const reviewModel = "claude-review"
		env := validEnv()
		env = with(env, reviewProviderEnv, "claude")
		env = with(env, reviewModelEnv, reviewModel)
		env = with(env, anthropicAPIKeyEnv, "sk-ant-review-key")
		env = with(env, claudeEffortEnv, "high")
		cfg, err := Load(envLookup(env))
		if err != nil {
			t.Fatalf("Load() error = %v, want nil", err)
		}
		if cfg.LLM().Provider() != ProviderDeepSeek {
			t.Errorf("Provider() = %v, want ProviderDeepSeek", cfg.LLM().Provider())
		}
		if cfg.LLM().Model() != env[modelEnv] {
			t.Errorf("Model() = %q, want %q", cfg.LLM().Model(), env[modelEnv])
		}
		if cfg.LLM().ReviewProvider() != ProviderClaude {
			t.Errorf("ReviewProvider() = %v, want ProviderClaude", cfg.LLM().ReviewProvider())
		}
		if cfg.LLM().ReviewModel() != reviewModel {
			t.Errorf("ReviewModel() = %q, want %q", cfg.LLM().ReviewModel(), reviewModel)
		}
	})

	t.Run("provider without model", func(t *testing.T) {
		env := with(validEnv(), reviewProviderEnv, "deepseek")
		_, err := Load(envLookup(env))
		assertVarError(t, err, ErrMissing, reviewModelEnv)
	})

	t.Run("invalid provider", func(t *testing.T) {
		env := with(validEnv(), reviewProviderEnv, "gemini")
		_, err := Load(envLookup(env))
		assertVarError(t, err, ErrInvalid, reviewProviderEnv)
	})

	t.Run("empty provider", func(t *testing.T) {
		env := with(validEnv(), reviewProviderEnv, "")
		_, err := Load(envLookup(env))
		assertVarError(t, err, ErrMissing, reviewProviderEnv)
	})

	t.Run("empty model", func(t *testing.T) {
		env := with(validEnv(), reviewModelEnv, "")
		_, err := Load(envLookup(env))
		assertVarError(t, err, ErrMissing, reviewModelEnv)
	})
}

// TestLoadReviewProviderRequiresItsKey checks that when the review provider
// differs from the generation provider, the review provider's API key variable
// is required even though the generation provider's key is present.
func TestLoadReviewProviderRequiresItsKey(t *testing.T) {
	claudeReview := func() map[string]string {
		env := validEnv()
		env = with(env, reviewProviderEnv, "claude")
		env = with(env, reviewModelEnv, "claude-review")
		env = with(env, claudeEffortEnv, "high")
		return env
	}
	deepseekReview := func() map[string]string {
		env := validClaudeEnv()
		env = with(env, reviewProviderEnv, "deepseek")
		env = with(env, reviewModelEnv, "deepseek-review")
		return env
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
		markProvider        = "MARK-PROVIDER-A1"
		markModel           = "MARK-MODEL-B2"
		markReviewProvider  = "MARK-REVIEW-PROVIDER-I9"
		markAPIKey          = "MARK-APIKEY-C3"
		markSlack           = "MARK-SLACK-D4"
		markCache           = "MARK-CACHE-E5"
		markYtDlp           = "MARK-YTDLP-F6"
		markClaudeEffort    = "MARK-CLAUDE-EFFORT-G7"
		markClaudeWorkspace = "MARK-CLAUDE-WKS-C8"
	)
	marks := []string{
		markProvider, markModel, markReviewProvider, markAPIKey, markSlack,
		markCache, markYtDlp, markClaudeEffort, markClaudeWorkspace,
	}
	base := map[string]string{
		providerEnv:       "deepseek",
		modelEnv:          markModel,
		deepSeekAPIKeyEnv: markAPIKey,
		slackEnv:          "https://hooks.slack.com/services/" + markSlack,
		cacheDirEnv:       "/tmp/" + markCache,
		ytDlpEnv:          "/bin/" + markYtDlp,
	}
	// claudeBase selects the claude provider, whose own variables are the ones
	// under test in the claude rows below.
	claudeBase := map[string]string{
		providerEnv:             "claude",
		modelEnv:                markModel,
		anthropicAPIKeyEnv:      markAPIKey,
		claudeEffortEnv:         "high",
		anthropicWorkspaceIDEnv: "wrkspc_ok",
		cacheDirEnv:             "/tmp/" + markCache,
		ytDlpEnv:                "/bin/" + markYtDlp,
	}
	cases := []struct {
		name  string
		env   map[string]string
		tails []string
	}{
		{"provider invalid", with(base, providerEnv, "gemini-"+markProvider), nil},
		{"review provider invalid", with(base, reviewProviderEnv, "gemini-"+markReviewProvider), nil},
		{"model empty", with(base, modelEnv, ""), nil},
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
		{"claude effort invalid", with(claudeBase, claudeEffortEnv, "High-"+markClaudeEffort), nil},
		{"claude workspace invalid", with(claudeBase, anthropicWorkspaceIDEnv, " "+markClaudeWorkspace), nil},
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

func TestLoadReportsAllInvalid(t *testing.T) {
	env := validEnv()
	env = with(env, modelEnv, "")
	env = with(env, providerEnv, "gemini")
	env = with(env, slackEnv, "http://hooks.slack.com/services/x")

	_, err := Load(envLookup(env))
	if !errors.Is(err, ErrMissing) {
		t.Errorf("Load() error = %v, want it to wrap ErrMissing", err)
	}
	if !errors.Is(err, ErrInvalid) {
		t.Errorf("Load() error = %v, want it to wrap ErrInvalid", err)
	}
	for _, name := range []string{modelEnv, providerEnv, slackEnv} {
		if !strings.Contains(err.Error(), name) {
			t.Errorf("Load() error = %q, want it to name %q", err, name)
		}
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

// validServerEnv is an environment LoadServer accepts: the LLM settings, the
// required Webhook URL, and HOME, from which the token path is derived.
func validServerEnv() map[string]string {
	return map[string]string{
		providerEnv:       "deepseek",
		modelEnv:          "deepseek-chat",
		deepSeekAPIKeyEnv: "sk-test-key",
		slackEnv:          "https://hooks.slack.com/services/T000/B000/XXXX",
		"HOME":            "/home/user",
	}
}

// wantTokenPath returns the token path LoadServer derives from home on the
// test host's OS.
func wantTokenPath(home string) string {
	dir := filepath.Join(home, ".config", "yt2column")
	if runtime.GOOS == "darwin" {
		dir = filepath.Join(home, "Library", "Application Support", "yt2column")
	}
	return filepath.Join(dir, "server-token")
}

// TestLoadServerValid checks that LoadServer loads the LLM settings, the
// required Webhook URL, the configured port, and the derived token path.
func TestLoadServerValid(t *testing.T) {
	env := with(validServerEnv(), serverPortEnv, "9000")
	cfg, err := LoadServer(envLookup(env))
	if err != nil {
		t.Fatalf("LoadServer() error = %v, want nil", err)
	}
	if cfg.LLM().Provider() != ProviderDeepSeek {
		t.Errorf("LLM().Provider() = %v, want ProviderDeepSeek", cfg.LLM().Provider())
	}
	if cfg.LLM().Model() != env[modelEnv] {
		t.Errorf("LLM().Model() = %q, want %q", cfg.LLM().Model(), env[modelEnv])
	}
	webhook, err := cfg.WebhookURL().Reveal()
	if err != nil {
		t.Fatalf("WebhookURL().Reveal() error = %v", err)
	}
	if webhook != env[slackEnv] {
		t.Errorf("WebhookURL() = %q, want %q", webhook, env[slackEnv])
	}
	if cfg.Port() != 9000 {
		t.Errorf("Port() = %d, want 9000", cfg.Port())
	}
	if want := wantTokenPath(env["HOME"]); cfg.TokenPath() != want {
		t.Errorf("TokenPath() = %q, want %q", cfg.TokenPath(), want)
	}
	if cfg.HTTP2DebugEnabled() {
		t.Error("HTTP2DebugEnabled() = true, want false")
	}
}

// TestLoadServerDefaultPort checks that an unset port defaults to 8765.
func TestLoadServerDefaultPort(t *testing.T) {
	cfg, err := LoadServer(envLookup(validServerEnv()))
	if err != nil {
		t.Fatalf("LoadServer() error = %v, want nil", err)
	}
	if cfg.Port() != defaultServerPort {
		t.Errorf("Port() = %d, want %d", cfg.Port(), defaultServerPort)
	}
}

// TestLoadServerClaude checks that LoadServer applies the shared LLM rules to a
// claude environment.
func TestLoadServerClaude(t *testing.T) {
	env := validClaudeEnv()
	env = with(env, slackEnv, "https://hooks.slack.com/services/T000/B000/XXXX")
	env = with(env, "HOME", "/home/user")
	cfg, err := LoadServer(envLookup(env))
	if err != nil {
		t.Fatalf("LoadServer() error = %v, want nil", err)
	}
	if cfg.LLM().Provider() != ProviderClaude {
		t.Errorf("LLM().Provider() = %v, want ProviderClaude", cfg.LLM().Provider())
	}
	if _, err := cfg.LLM().AnthropicAPIKey().Reveal(); err != nil {
		t.Errorf("LLM().AnthropicAPIKey().Reveal() error = %v", err)
	}
}

// TestLoadServerModelMissing checks that a missing or empty model ends the
// load with an ErrMissing that names the variable.
func TestLoadServerModelMissing(t *testing.T) {
	cases := []struct {
		name string
		env  map[string]string
	}{
		{"unset", without(validServerEnv(), modelEnv)},
		{"empty", with(validServerEnv(), modelEnv, "")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := LoadServer(envLookup(tc.env))
			if !errors.Is(err, ErrMissing) {
				t.Fatalf("LoadServer() error = %v, want ErrMissing", err)
			}
			if !strings.Contains(err.Error(), modelEnv) {
				t.Errorf("LoadServer() error = %q, want it to name %q", err, modelEnv)
			}
		})
	}
}

// TestLoadServerWebhookRequired checks that SLACK_WEBHOOK_URL is required, that
// an invalid value is rejected, and that neither rejection holds the value.
func TestLoadServerWebhookRequired(t *testing.T) {
	const mark = "MARK-SERVER-WEBHOOK-D4"
	t.Run("unset", func(t *testing.T) {
		_, err := LoadServer(envLookup(without(validServerEnv(), slackEnv)))
		assertVarError(t, err, ErrMissing, slackEnv)
	})
	t.Run("empty", func(t *testing.T) {
		_, err := LoadServer(envLookup(with(validServerEnv(), slackEnv, "")))
		assertVarError(t, err, ErrMissing, slackEnv)
	})
	t.Run("invalid", func(t *testing.T) {
		_, err := LoadServer(envLookup(with(validServerEnv(), slackEnv, "http://host/"+mark)))
		if !errors.Is(err, ErrInvalid) {
			t.Fatalf("LoadServer() error = %v, want ErrInvalid", err)
		}
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

// TestServerPort fixes the accepted and rejected forms of YT2COLUMN_SERVER_PORT.
func TestServerPort(t *testing.T) {
	accepted := []struct {
		value string
		want  uint16
	}{
		{"1024", 1024},
		{"8765", 8765},
		{"65535", 65535},
	}
	for _, tc := range accepted {
		t.Run("accept "+tc.value, func(t *testing.T) {
			got, err := parseServerPort(tc.value)
			if err != nil {
				t.Fatalf("parseServerPort(%q) error = %v, want nil", tc.value, err)
			}
			if got != tc.want {
				t.Errorf("parseServerPort(%q) = %d, want %d", tc.value, got, tc.want)
			}
		})
	}

	rejected := []struct {
		value    string
		sentinel error
	}{
		{"", ErrMissing},
		{"08765", ErrInvalid},
		{"+8765", ErrInvalid},
		{" 8765", ErrInvalid},
		{"8765 ", ErrInvalid},
		{"0x2235", ErrInvalid},
		{"0", ErrInvalid},
		{"80", ErrInvalid},
		{"65536", ErrInvalid},
		{"-1", ErrInvalid},
		{"1.0", ErrInvalid},
		{"12a3", ErrInvalid},
	}
	for _, tc := range rejected {
		t.Run(fmt.Sprintf("reject %q", tc.value), func(t *testing.T) {
			_, err := parseServerPort(tc.value)
			if !errors.Is(err, tc.sentinel) {
				t.Fatalf("parseServerPort(%q) error = %v, want it to wrap %v", tc.value, err, tc.sentinel)
			}
		})
	}
}

// TestLoadServerPortRejected checks that LoadServer reports a rejected port and
// names the variable.
func TestLoadServerPortRejected(t *testing.T) {
	for _, value := range []string{"0", "08765", "65536", " 8765"} {
		t.Run(value, func(t *testing.T) {
			_, err := LoadServer(envLookup(with(validServerEnv(), serverPortEnv, value)))
			if !errors.Is(err, ErrInvalid) {
				t.Fatalf("LoadServer() error = %v, want ErrInvalid", err)
			}
			if !strings.Contains(err.Error(), serverPortEnv) {
				t.Errorf("LoadServer() error = %q, want it to name %q", err, serverPortEnv)
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
