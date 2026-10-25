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

// TestLoadDefaults checks the defaults of the variables other than the LLM
// ones: no Webhook, yt-dlp on PATH, and the derived cache directory. The LLM
// defaults are checked by TestLoadGenerationDefaults and TestLoadReviewDefaults.
func TestLoadDefaults(t *testing.T) {
	home := "/home/user"
	env := map[string]string{
		providerEnv:       "deepseek",
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

	// The effort default must not make the API key optional, nor add a
	// rejection of the unset effort next to the API key's.
	t.Run("API key and effort unset", func(t *testing.T) {
		_, err := Load(envLookup(without(validClaudeEnv(), anthropicAPIKeyEnv, claudeEffortEnv)))
		assertVarError(t, err, ErrMissing, anthropicAPIKeyEnv)
		if slices.Contains(varErrorNames(err), claudeEffortEnv) {
			t.Errorf("Load() error = %v, names %s", err, claudeEffortEnv)
		}
	})
}

// varErrorNames returns the variable name of every *VarError in err's tree.
func varErrorNames(err error) []string {
	var names []string
	for _, varErr := range varErrors(err) {
		names = append(names, varErr.Name)
	}
	return names
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

// TestLoadClaudeEffortDefault checks that an unset YT2COLUMN_CLAUDE_EFFORT
// selects the high effort whenever a stage uses claude, for both entry points.
func TestLoadClaudeEffortDefault(t *testing.T) {
	claudeBoth := without(validClaudeEnv(), claudeEffortEnv)
	reviewClaude := with(validEnv(), reviewProviderEnv, "claude")
	reviewClaude = with(reviewClaude, reviewModelEnv, "claude-review")
	reviewClaude = with(reviewClaude, anthropicAPIKeyEnv, "sk-ant-test-key")
	generationClaude := with(claudeBoth, reviewProviderEnv, "deepseek")
	generationClaude = with(generationClaude, reviewModelEnv, "deepseek-review")
	generationClaude = with(generationClaude, deepSeekAPIKeyEnv, "sk-test-key")
	cases := []struct {
		name string
		env  map[string]string
	}{
		{"both stages claude", claudeBoth},
		{"review claude only", reviewClaude},
		{"generation claude only", generationClaude},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := Load(envLookup(tc.env))
			if err != nil {
				t.Fatalf("Load() error = %v, want nil", err)
			}
			if cfg.LLM().ClaudeEffort() != claudeparam.EffortHigh {
				t.Errorf("ClaudeEffort() = %v, want EffortHigh", cfg.LLM().ClaudeEffort())
			}
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
		if cfg.LLM().ClaudeEffort() != claudeparam.EffortHigh {
			t.Errorf("ClaudeEffort() = %v, want EffortHigh", cfg.LLM().ClaudeEffort())
		}
	})
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
		markReviewModel     = "MARK-REVIEW-MODEL-J0"
		markAPIKey          = "MARK-APIKEY-C3"
		markSlack           = "MARK-SLACK-D4"
		markCache           = "MARK-CACHE-E5"
		markYtDlp           = "MARK-YTDLP-F6"
		markClaudeEffort    = "MARK-CLAUDE-EFFORT-G7"
		markClaudeWorkspace = "MARK-CLAUDE-WKS-C8"
	)
	marks := []string{
		markProvider, markModel, markReviewProvider, markReviewModel, markAPIKey, markSlack,
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
		// Both model variables without a provider: the model-only rejection
		// must not carry either model's value.
		{"model without provider", withVars(without(base, providerEnv), map[string]string{reviewModelEnv: markReviewModel}), nil},
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

// validServerEnv is an environment LoadServer accepts: the CLI environment
// without the cache and yt-dlp variables the server does not use, plus HOME,
// from which the socket path is derived.
func validServerEnv() map[string]string {
	env := without(validEnv(), cacheDirEnv, ytDlpEnv)
	env["HOME"] = "/home/user"
	return env
}

// TestLoadServerValid checks that LoadServer loads the LLM settings, the
// required Webhook URL, the socket path derived from HOME, and the HTTP/2
// debug flag. The OS-specific socket path rules are fixed by TestSocketPath.
func TestLoadServerValid(t *testing.T) {
	env := with(validServerEnv(), godebugEnv, "http2debug=1")
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
	want := "/home/user/.local/state/yt2column/run/server.sock"
	if runtime.GOOS == "darwin" {
		want = "/home/user/Library/Application Support/yt2column/run/server.sock"
	}
	if cfg.SocketPath() != want {
		t.Errorf("SocketPath() = %q, want %q", cfg.SocketPath(), want)
	}
	if !cfg.HTTP2DebugEnabled() {
		t.Error("HTTP2DebugEnabled() = false, want true")
	}
}

// TestLoadServerModelMissing checks that a missing or empty model ends the
// load with an ErrMissing that names the variable. The claude provider and the
// other LLM rules are covered by the Load tests, which LoadServer shares.
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
			assertVarError(t, err, ErrMissing, modelEnv)
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
			if cfg.SocketPath() != "" || cfg.LLM().Model() != "" {
				t.Errorf("LoadServer() returned a non-zero ServerConfig on error: socketPath=%q model=%q",
					cfg.SocketPath(), cfg.LLM().Model())
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
		if cfg.LLM().Provider() != ProviderDeepSeek || cfg.LLM().ReviewProvider() != ProviderDeepSeek {
			t.Errorf("providers = %v/%v, want deepseek/deepseek", cfg.LLM().Provider(), cfg.LLM().ReviewProvider())
		}
	})

	t.Run("generation only leaves the review settings unread", func(t *testing.T) {
		// A review provider and an empty review model that would be rejected if
		// read are neither required nor read when the review step does not run.
		env := with(validEnv(), reviewProviderEnv, "gemini")
		env = with(env, reviewModelEnv, "")
		cfg, err := LoadFor(envLookup(env), Need{Generation: true})
		if err != nil {
			t.Fatalf("LoadFor() error = %v, want nil", err)
		}
		if cfg.LLM().ReviewProvider() != ProviderUnset {
			t.Errorf("ReviewProvider() = %v, want ProviderUnset (not read)", cfg.LLM().ReviewProvider())
		}
		if cfg.LLM().ReviewModel() != "" {
			t.Errorf("ReviewModel() = %q, want empty (not read)", cfg.LLM().ReviewModel())
		}
	})

	t.Run("generation only requires the generation settings", func(t *testing.T) {
		for _, name := range []string{modelEnv, deepSeekAPIKeyEnv} {
			t.Run(name, func(t *testing.T) {
				_, err := LoadFor(envLookup(without(validEnv(), name)), Need{Generation: true})
				assertVarError(t, err, ErrMissing, name)
			})
		}
	})

	t.Run("review only does not require the generation settings", func(t *testing.T) {
		// The review step has its own provider and model, so neither
		// YT2COLUMN_MODEL nor the generation provider is needed.
		env := map[string]string{
			reviewProviderEnv: "deepseek",
			reviewModelEnv:    "deepseek-review",
			deepSeekAPIKeyEnv: "sk-test-key",
			cacheDirEnv:       "/tmp/yt2column-test-cache",
		}
		cfg, err := LoadFor(envLookup(env), Need{Review: true})
		if err != nil {
			t.Fatalf("LoadFor() error = %v, want nil", err)
		}
		if cfg.LLM().ReviewProvider() != ProviderDeepSeek || cfg.LLM().ReviewModel() != "deepseek-review" {
			t.Errorf("review = %v/%q, want deepseek/deepseek-review", cfg.LLM().ReviewProvider(), cfg.LLM().ReviewModel())
		}
	})

	t.Run("review only checks the review provider's key", func(t *testing.T) {
		env := map[string]string{
			reviewProviderEnv: "deepseek",
			reviewModelEnv:    "deepseek-review",
			cacheDirEnv:       "/tmp/yt2column-test-cache",
		}
		_, err := LoadFor(envLookup(env), Need{Review: true})
		assertVarError(t, err, ErrMissing, deepSeekAPIKeyEnv)
	})

	t.Run("review only inherits the generation provider and model", func(t *testing.T) {
		env := without(validEnv(), reviewProviderEnv, reviewModelEnv)
		cfg, err := LoadFor(envLookup(env), Need{Review: true})
		if err != nil {
			t.Fatalf("LoadFor() error = %v, want nil", err)
		}
		if cfg.LLM().ReviewProvider() != ProviderDeepSeek {
			t.Errorf("ReviewProvider() = %v, want the inherited deepseek", cfg.LLM().ReviewProvider())
		}
		if cfg.LLM().ReviewModel() != env[modelEnv] {
			t.Errorf("ReviewModel() = %q, want the inherited %q", cfg.LLM().ReviewModel(), env[modelEnv])
		}
	})

	t.Run("review only inherits a claude generation provider", func(t *testing.T) {
		// The inherited claude provider brings its own variables into the
		// check even though generation does not run.
		cfg, err := LoadFor(envLookup(validClaudeEnv()), Need{Review: true})
		if err != nil {
			t.Fatalf("LoadFor() error = %v, want nil", err)
		}
		if cfg.LLM().ReviewProvider() != ProviderClaude || cfg.LLM().ReviewModel() != validClaudeEnv()[modelEnv] {
			t.Errorf("review = %v/%q, want claude/%q", cfg.LLM().ReviewProvider(), cfg.LLM().ReviewModel(), validClaudeEnv()[modelEnv])
		}
		if cfg.LLM().ClaudeEffort() == claudeparam.EffortUnset {
			t.Error("ClaudeEffort() is unset, want the checked value")
		}
	})

	t.Run("review only requires a model", func(t *testing.T) {
		// With no model anywhere and a provider that has no default model, the
		// review step has none.
		env := map[string]string{
			providerEnv:       "deepseek",
			deepSeekAPIKeyEnv: "sk-test-key",
			cacheDirEnv:       "/tmp/yt2column-test-cache",
		}
		_, err := LoadFor(envLookup(env), Need{Review: true})
		assertVarError(t, err, ErrMissing, reviewModelEnv)
	})

	// The effort is neither read nor defaulted when the claude stage is the
	// one the run skips.
	skippedClaude := []struct {
		name string
		env  map[string]string
		need Need
	}{
		{
			"generation only with a claude review",
			with(with(with(validEnv(), reviewProviderEnv, "claude"), reviewModelEnv, "claude-review"), anthropicAPIKeyEnv, "sk-ant-test-key"),
			Need{Generation: true},
		},
		{
			"review only with a claude generation",
			with(with(with(without(validClaudeEnv(), claudeEffortEnv), reviewProviderEnv, "deepseek"), reviewModelEnv, "deepseek-review"), deepSeekAPIKeyEnv, "sk-test-key"),
			Need{Review: true},
		},
	}
	for _, tc := range skippedClaude {
		for _, effort := range []struct {
			name  string
			value string
			set   bool
		}{
			{"unset", "", false},
			{"empty", "", true},
			{"invalid", "High", true},
		} {
			t.Run(tc.name+" effort "+effort.name, func(t *testing.T) {
				env := without(tc.env, claudeEffortEnv)
				if effort.set {
					env = with(env, claudeEffortEnv, effort.value)
				}
				cfg, err := LoadFor(envLookup(env), tc.need)
				if err != nil {
					t.Fatalf("LoadFor() error = %v, want nil", err)
				}
				if cfg.LLM().ClaudeEffort() != claudeparam.EffortUnset {
					t.Errorf("ClaudeEffort() = %v, want EffortUnset", cfg.LLM().ClaudeEffort())
				}
			})
		}
	}

	t.Run("no stage reads no LLM variable", func(t *testing.T) {
		// --after review: not even an invalid provider or model is read, and
		// with no LLM variable at all no default is applied.
		envs := map[string]map[string]string{
			"invalid values": {
				providerEnv:     "gemini",
				modelEnv:        "",
				claudeEffortEnv: "High",
				cacheDirEnv:     "/tmp/yt2column-test-cache",
			},
			"no LLM variable": {
				cacheDirEnv: "/tmp/yt2column-test-cache",
			},
		}
		llmVars := []string{
			providerEnv, modelEnv, reviewProviderEnv, reviewModelEnv,
			deepSeekAPIKeyEnv, anthropicAPIKeyEnv, claudeEffortEnv, anthropicWorkspaceIDEnv,
		}
		for name, env := range envs {
			t.Run(name, func(t *testing.T) {
				var read []string
				lookup := func(name string) (string, bool) {
					read = append(read, name)
					return envLookup(env)(name)
				}
				cfg, err := LoadFor(lookup, Need{})
				if err != nil {
					t.Fatalf("LoadFor() error = %v, want nil", err)
				}
				// The positive check keeps the negative one meaningful: a lookup
				// that recorded nothing would pass it.
				if !slices.Contains(read, cacheDirEnv) {
					t.Fatalf("read = %v, want it to include %s", read, cacheDirEnv)
				}
				for _, name := range llmVars {
					if slices.Contains(read, name) {
						t.Errorf("LoadFor(Need{}) read %s", name)
					}
				}
				llm := cfg.LLM()
				if llm.Provider() != ProviderUnset || llm.ReviewProvider() != ProviderUnset {
					t.Errorf("providers = %v/%v, want ProviderUnset/ProviderUnset", llm.Provider(), llm.ReviewProvider())
				}
				if llm.Model() != "" || llm.ReviewModel() != "" {
					t.Errorf("models = %q/%q, want empty", llm.Model(), llm.ReviewModel())
				}
			})
		}
	})

	t.Run("non-LLM variables are still checked", func(t *testing.T) {
		env := map[string]string{cacheDirEnv: "relative"}
		_, err := LoadFor(envLookup(env), Need{})
		assertVarError(t, err, ErrInvalid, cacheDirEnv)
	})

	t.Run("review only rejects an empty inherited provider", func(t *testing.T) {
		// The review step inherits the generation provider, so a present-empty
		// one is rejected rather than silently defaulted.
		env := map[string]string{
			providerEnv:       "",
			reviewModelEnv:    "deepseek-review",
			deepSeekAPIKeyEnv: "sk-test-key",
			cacheDirEnv:       "/tmp/yt2column-test-cache",
		}
		_, err := LoadFor(envLookup(env), Need{Review: true})
		assertVarError(t, err, ErrMissing, providerEnv)
	})

	t.Run("review only rejects an invalid inherited provider", func(t *testing.T) {
		env := map[string]string{
			providerEnv:       "gemini",
			reviewModelEnv:    "deepseek-review",
			deepSeekAPIKeyEnv: "sk-test-key",
			cacheDirEnv:       "/tmp/yt2column-test-cache",
		}
		_, err := LoadFor(envLookup(env), Need{Review: true})
		assertVarError(t, err, ErrInvalid, providerEnv)
	})

	t.Run("review only rejects an empty inherited model", func(t *testing.T) {
		env := map[string]string{
			providerEnv:       "deepseek",
			modelEnv:          "",
			deepSeekAPIKeyEnv: "sk-test-key",
			cacheDirEnv:       "/tmp/yt2column-test-cache",
		}
		_, err := LoadFor(envLookup(env), Need{Review: true})
		assertVarError(t, err, ErrMissing, modelEnv)
	})

	t.Run("review only ignores an unused generation provider's variables", func(t *testing.T) {
		// The review step has its own deepseek provider, so the claude
		// generation provider and its API key, effort, and workspace are not
		// read even though YT2COLUMN_LLM_PROVIDER is set.
		env := map[string]string{
			providerEnv:       "claude",
			reviewProviderEnv: "deepseek",
			reviewModelEnv:    "deepseek-review",
			deepSeekAPIKeyEnv: "sk-test-key",
			cacheDirEnv:       "/tmp/yt2column-test-cache",
		}
		cfg, err := LoadFor(envLookup(env), Need{Review: true})
		if err != nil {
			t.Fatalf("LoadFor() error = %v, want nil (the unused claude provider is not checked)", err)
		}
		if cfg.LLM().ReviewProvider() != ProviderDeepSeek {
			t.Errorf("ReviewProvider() = %v, want ProviderDeepSeek", cfg.LLM().ReviewProvider())
		}
	})
}

// defaultsBaseEnv is an environment with no provider or model variable: the
// API keys of both providers, an explicit effort (so the tests do not depend
// on the effort default), and the non-LLM variable Load needs. A row adds the
// provider and model variables it is about.
func defaultsBaseEnv() map[string]string {
	return map[string]string{
		anthropicAPIKeyEnv: "sk-ant-test-key",
		claudeEffortEnv:    "high",
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

// wantInvalidProvider is the rejection of a provider variable whose value is
// not an accepted provider.
func wantInvalidProvider(name string) VarError {
	return VarError{Name: name, Reason: reasonProvider, Err: ErrInvalid}
}

// wantModelOnly is the rejection of a model variable set without a provider
// variable for the stage that uses it.
var wantModelOnly = VarError{Name: providerEnv, Reason: reasonModelWithoutProvider, Err: ErrMissing}

// The Need values of the run kinds: a run that generates (with the review
// configuration read, as without --after), a generation-only run, and a
// review-only resume.
var (
	needBoth       = Need{Generation: true, Review: true}
	needGeneration = Need{Generation: true}
	needReview     = Need{Review: true}
)

// TestLoadGenerationDefaults checks how the generation stage's provider and
// model are resolved: an unset YT2COLUMN_LLM_PROVIDER selects claude, an unset
// YT2COLUMN_MODEL selects the default model only when the provider is claude,
// and an empty or invalid variable is rejected rather than defaulted. Each row
// runs both with and without the review stage, which does not change how the
// generation stage resolves. The rows that set a model without a provider are
// in TestLoadModelWithoutProvider.
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
		{"provider and model unset", nil, ProviderClaude, defaultClaudeModel, nil},
		{"claude, model unset", map[string]string{providerEnv: "claude"}, ProviderClaude, defaultClaudeModel, nil},
		{"claude, other model", map[string]string{providerEnv: "claude", modelEnv: otherModel}, ProviderClaude, otherModel, nil},
		{"deepseek, model set", map[string]string{providerEnv: "deepseek", modelEnv: "deepseek-chat"}, ProviderDeepSeek, "deepseek-chat", nil},
		{
			"deepseek, model unset",
			map[string]string{providerEnv: "deepseek"},
			ProviderUnset, "",
			[]VarError{wantMissing(modelEnv)},
		},
		{
			"claude, model empty",
			map[string]string{providerEnv: "claude", modelEnv: ""},
			ProviderUnset, "",
			[]VarError{wantMissing(modelEnv)},
		},
		{
			"deepseek, model empty",
			map[string]string{providerEnv: "deepseek", modelEnv: ""},
			ProviderUnset, "",
			[]VarError{wantMissing(modelEnv)},
		},
		{
			"provider empty, model unset",
			map[string]string{providerEnv: ""},
			ProviderUnset, "",
			[]VarError{wantMissing(providerEnv), wantMissing(modelEnv)},
		},
		{
			"provider invalid, model unset",
			map[string]string{providerEnv: "gemini"},
			ProviderUnset, "",
			[]VarError{wantInvalidProvider(providerEnv), wantMissing(modelEnv)},
		},
		{
			"provider invalid, model empty",
			map[string]string{providerEnv: "gemini", modelEnv: ""},
			ProviderUnset, "",
			[]VarError{wantInvalidProvider(providerEnv), wantMissing(modelEnv)},
		},
	}
	for _, need := range []Need{needBoth, needGeneration} {
		for _, tc := range cases {
			t.Run(fmt.Sprintf("%+v %s", need, tc.name), func(t *testing.T) {
				cfg, err := LoadFor(envLookup(withVars(defaultsBaseEnv(), tc.vars)), need)
				assertVarErrors(t, err, tc.wantErrs...)
				if err != nil {
					return
				}
				if got := cfg.LLM().Provider(); got != tc.wantProvider {
					t.Errorf("Provider() = %v, want %v", got, tc.wantProvider)
				}
				if got := cfg.LLM().Model(); got != tc.wantModel {
					t.Errorf("Model() = %q, want %q", got, tc.wantModel)
				}
			})
		}
	}
}

// TestLoadReviewDefaults checks how the review stage's provider and model are
// resolved in a run that generates and in a review-only resume: both review
// variables unset inherit the generation stage, the defaults included; a
// review provider of its own uses that provider's default model, never the
// generation model; and an empty or invalid variable is rejected rather than
// defaulted. The rows that set a model without a provider are in
// TestLoadModelWithoutProvider.
func TestLoadReviewDefaults(t *testing.T) {
	const otherModel = "claude-other-test"
	type want struct {
		provider       Provider
		model          string
		reviewProvider Provider
		reviewModel    string
	}
	allDefault := want{ProviderClaude, defaultClaudeModel, ProviderClaude, defaultClaudeModel}
	cases := []struct {
		name     string
		need     Need
		vars     map[string]string
		want     want
		wantErrs []VarError
	}{
		{"all unset", needBoth, nil, allDefault, nil},
		{"review only, all unset", needReview, nil, allDefault, nil},
		{"review only, generation claude", needReview, map[string]string{providerEnv: "claude"}, allDefault, nil},
		{
			"review only, generation deepseek without a model", needReview,
			map[string]string{providerEnv: "deepseek"},
			want{},
			[]VarError{wantMissing(reviewModelEnv)},
		},
		{
			"review provider claude, generation deepseek", needBoth,
			map[string]string{providerEnv: "deepseek", modelEnv: "deepseek-chat", reviewProviderEnv: "claude"},
			want{ProviderDeepSeek, "deepseek-chat", ProviderClaude, defaultClaudeModel},
			nil,
		},
		{
			"review provider claude, generation claude with another model", needBoth,
			map[string]string{providerEnv: "claude", modelEnv: otherModel, reviewProviderEnv: "claude"},
			want{ProviderClaude, otherModel, ProviderClaude, defaultClaudeModel},
			nil,
		},
		{
			"review only, review provider claude", needReview,
			map[string]string{reviewProviderEnv: "claude"},
			want{ProviderUnset, "", ProviderClaude, defaultClaudeModel},
			nil,
		},
		{
			"review only, review provider claude, review model set", needReview,
			map[string]string{reviewProviderEnv: "claude", reviewModelEnv: otherModel},
			want{ProviderUnset, "", ProviderClaude, otherModel},
			nil,
		},
		{
			"review provider deepseek, review model unset", needBoth,
			map[string]string{providerEnv: "deepseek", modelEnv: "deepseek-chat", reviewProviderEnv: "deepseek"},
			want{},
			[]VarError{wantMissing(reviewModelEnv)},
		},
		{
			"review only, review provider deepseek, review model unset", needReview,
			map[string]string{reviewProviderEnv: "deepseek"},
			want{},
			[]VarError{wantMissing(reviewModelEnv)},
		},
		{
			"review provider claude, review model empty", needBoth,
			map[string]string{providerEnv: "claude", reviewProviderEnv: "claude", reviewModelEnv: ""},
			want{},
			[]VarError{wantMissing(reviewModelEnv)},
		},
		{
			"review provider empty, review model unset", needBoth,
			map[string]string{providerEnv: "claude", reviewProviderEnv: ""},
			want{},
			[]VarError{wantMissing(reviewProviderEnv)},
		},
		{
			"review only, review provider empty, review model unset", needReview,
			map[string]string{reviewProviderEnv: ""},
			want{},
			[]VarError{wantMissing(reviewProviderEnv)},
		},
		{
			"review provider invalid, review model unset", needBoth,
			map[string]string{providerEnv: "claude", reviewProviderEnv: "gemini"},
			want{},
			[]VarError{wantInvalidProvider(reviewProviderEnv)},
		},
		{
			"review only, review provider claude, review model empty", needReview,
			map[string]string{reviewProviderEnv: "claude", reviewModelEnv: ""},
			want{},
			[]VarError{wantMissing(reviewModelEnv)},
		},
		{
			"review only, review provider invalid, review model unset", needReview,
			map[string]string{reviewProviderEnv: "gemini"},
			want{},
			[]VarError{wantInvalidProvider(reviewProviderEnv)},
		},
		{
			"review provider invalid, review model empty", needBoth,
			map[string]string{providerEnv: "claude", reviewProviderEnv: "gemini", reviewModelEnv: ""},
			want{},
			[]VarError{wantInvalidProvider(reviewProviderEnv), wantMissing(reviewModelEnv)},
		},
		{
			"review only, generation provider empty", needReview,
			map[string]string{providerEnv: ""},
			want{},
			[]VarError{wantMissing(providerEnv), wantMissing(reviewModelEnv)},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := LoadFor(envLookup(withVars(defaultsBaseEnv(), tc.vars)), tc.need)
			assertVarErrors(t, err, tc.wantErrs...)
			if err != nil {
				return
			}
			llm := cfg.LLM()
			got := want{llm.Provider(), llm.Model(), llm.ReviewProvider(), llm.ReviewModel()}
			if got != tc.want {
				t.Errorf("provider/model/review provider/review model = %+v, want %+v", got, tc.want)
			}
		})
	}
}

// TestLoadModelWithoutProvider checks the rejection of a model variable set
// for a stage whose provider was not set by a variable: before the default
// provider became claude such a setting used deepseek, so it must stop before
// any request instead of reaching Anthropic. The rejection is reported once
// per run, next to the other rejections, and is distinct from the rejection of
// an empty provider variable.
func TestLoadModelWithoutProvider(t *testing.T) {
	cases := []struct {
		name     string
		need     Need
		vars     map[string]string
		wantErrs []VarError
	}{
		{"generation model", needBoth, map[string]string{modelEnv: "deepseek-chat"}, []VarError{wantModelOnly}},
		{"generation only, generation model", needGeneration, map[string]string{modelEnv: "deepseek-chat"}, []VarError{wantModelOnly}},
		{
			"generation model empty", needBoth,
			map[string]string{modelEnv: ""},
			[]VarError{wantModelOnly, wantMissing(modelEnv)},
		},
		{
			"generation only, generation model empty", needGeneration,
			map[string]string{modelEnv: ""},
			[]VarError{wantModelOnly, wantMissing(modelEnv)},
		},
		{"review model", needBoth, map[string]string{reviewModelEnv: "deepseek-chat"}, []VarError{wantModelOnly}},
		{"review only, review model", needReview, map[string]string{reviewModelEnv: "deepseek-chat"}, []VarError{wantModelOnly}},
		{
			"review model empty", needBoth,
			map[string]string{reviewModelEnv: ""},
			[]VarError{wantModelOnly, wantMissing(reviewModelEnv)},
		},
		{
			"review only, review model empty", needReview,
			map[string]string{reviewModelEnv: ""},
			[]VarError{wantModelOnly, wantMissing(reviewModelEnv)},
		},
		{"review only, inherited model", needReview, map[string]string{modelEnv: "deepseek-chat"}, []VarError{wantModelOnly}},
		{
			"review only, inherited model empty", needReview,
			map[string]string{modelEnv: ""},
			[]VarError{wantModelOnly, wantMissing(modelEnv), wantMissing(reviewModelEnv)},
		},
		{
			"both models", needBoth,
			map[string]string{modelEnv: "deepseek-chat", reviewModelEnv: "deepseek-review"},
			[]VarError{wantModelOnly},
		},
		{
			"default provider's variables are still checked", needBoth,
			map[string]string{modelEnv: "deepseek-chat", anthropicAPIKeyEnv: "", claudeEffortEnv: "High", anthropicWorkspaceIDEnv: ""},
			[]VarError{
				wantModelOnly, wantMissing(anthropicAPIKeyEnv), wantMissing(anthropicWorkspaceIDEnv),
				{Name: claudeEffortEnv, Reason: reasonClaudeEffort, Err: ErrInvalid},
			},
		},
		// A provider variable that is present, whatever its value, decides the
		// provider, so the model-only rejection does not occur.
		{
			"generation provider deepseek", needBoth,
			map[string]string{providerEnv: "deepseek", modelEnv: "deepseek-chat", reviewModelEnv: "deepseek-review"},
			nil,
		},
		{"generation provider claude", needBoth, map[string]string{providerEnv: "claude", modelEnv: "claude-other-test"}, nil},
		{
			"generation provider empty", needBoth,
			map[string]string{providerEnv: "", modelEnv: "deepseek-chat"},
			[]VarError{wantMissing(providerEnv)},
		},
		{
			"generation provider invalid", needBoth,
			map[string]string{providerEnv: "gemini", modelEnv: "deepseek-chat"},
			[]VarError{wantInvalidProvider(providerEnv)},
		},
		{
			"generation only, generation provider empty", needGeneration,
			map[string]string{providerEnv: "", modelEnv: "deepseek-chat"},
			[]VarError{wantMissing(providerEnv)},
		},
		{
			"generation only, generation provider invalid", needGeneration,
			map[string]string{providerEnv: "gemini", modelEnv: "deepseek-chat"},
			[]VarError{wantInvalidProvider(providerEnv)},
		},
		{
			"review only, inherited provider empty", needReview,
			map[string]string{providerEnv: "", modelEnv: "deepseek-chat"},
			[]VarError{wantMissing(providerEnv)},
		},
		{
			"review only, inherited provider invalid", needReview,
			map[string]string{providerEnv: "gemini", modelEnv: "deepseek-chat"},
			[]VarError{wantInvalidProvider(providerEnv)},
		},
		// The review stage is not needed, so its model variable is not read.
		{"generation only, review model", needGeneration, map[string]string{reviewModelEnv: "deepseek-review"}, nil},
		{
			"review provider set", needBoth,
			map[string]string{reviewProviderEnv: "deepseek", reviewModelEnv: "deepseek-review"},
			nil,
		},
		{
			"review only, review provider empty", needReview,
			map[string]string{reviewProviderEnv: "", reviewModelEnv: "deepseek-review"},
			[]VarError{wantMissing(reviewProviderEnv)},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := LoadFor(envLookup(withVars(defaultsBaseEnv(), tc.vars)), tc.need)
			assertVarErrors(t, err, tc.wantErrs...)
		})
	}
}

// TestModelWithoutProvider checks the decision over the origins of the stages
// a run needs, including the origin no resolver should leave (originNone for a
// provider), which must reject rather than let the model through.
func TestModelWithoutProvider(t *testing.T) {
	cases := []struct {
		name   string
		stages []stageOrigins
		reject bool
	}{
		{"no stage", nil, false},
		{"both from variables", []stageOrigins{{originVariable, originVariable}}, false},
		{"model variable, default provider", []stageOrigins{{originDefault, originVariable}}, true},
		{"model variable, unrecorded provider", []stageOrigins{{originNone, originVariable}}, true},
		{"default model", []stageOrigins{{originDefault, originDefault}}, false},
		{"provider variable, default model", []stageOrigins{{originVariable, originDefault}}, false},
		{"no model", []stageOrigins{{originDefault, originNone}}, false},
		{
			"only the second stage matches",
			[]stageOrigins{{originVariable, originVariable}, {originDefault, originVariable}},
			true,
		},
		{
			"both stages match",
			[]stageOrigins{{originDefault, originVariable}, {originDefault, originVariable}},
			true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := modelWithoutProvider(tc.stages...)
			if !tc.reject {
				if err != nil {
					t.Errorf("modelWithoutProvider() = %v, want nil", err)
				}
				return
			}
			assertVarErrors(t, err, wantModelOnly)
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
				claudeEffortEnv:   "high",
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
			for _, name := range []string{anthropicAPIKeyEnv, claudeEffortEnv, anthropicWorkspaceIDEnv} {
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

// TestLoadServerLLMDefaults checks the server entry point with no provider or
// model variable and with a deepseek setting that relied on the old default.
// The rules themselves are covered by the Load tests, which LoadServer shares.
func TestLoadServerLLMDefaults(t *testing.T) {
	base := map[string]string{
		"HOME":             "/home/user",
		slackEnv:           "https://hooks.slack.com/services/T000/B000/XXXX",
		claudeEffortEnv:    "high",
		anthropicAPIKeyEnv: "sk-ant-test-key",
	}
	t.Run("defaults", func(t *testing.T) {
		cfg, err := LoadServer(envLookup(base))
		if err != nil {
			t.Fatalf("LoadServer() error = %v, want nil", err)
		}
		llm := cfg.LLM()
		if llm.Provider() != ProviderClaude || llm.ReviewProvider() != ProviderClaude {
			t.Errorf("providers = %v/%v, want ProviderClaude/ProviderClaude", llm.Provider(), llm.ReviewProvider())
		}
		if llm.Model() != defaultClaudeModel || llm.ReviewModel() != defaultClaudeModel {
			t.Errorf("models = %q/%q, want %q", llm.Model(), llm.ReviewModel(), defaultClaudeModel)
		}
	})
	t.Run("deepseek setting without a provider", func(t *testing.T) {
		env := without(base, anthropicAPIKeyEnv)
		env = withVars(env, map[string]string{modelEnv: "deepseek-chat", deepSeekAPIKeyEnv: "sk-test-key"})
		_, err := LoadServer(envLookup(env))
		assertVarErrors(t, err, wantModelOnly, wantMissing(anthropicAPIKeyEnv))
	})
}

// TestLoadProductionDefaults checks that, with the Anthropic API key as the
// only LLM variable, every stage uses the production configuration: the claude
// provider, the default model, and the high effort. It is the one LLM test
// that leaves YT2COLUMN_CLAUDE_EFFORT unset, so it also depends on the effort
// default.
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
	if llm.Provider() != ProviderClaude || llm.ReviewProvider() != ProviderClaude {
		t.Errorf("providers = %v/%v, want ProviderClaude/ProviderClaude", llm.Provider(), llm.ReviewProvider())
	}
	if llm.Model() != defaultClaudeModel || llm.ReviewModel() != defaultClaudeModel {
		t.Errorf("models = %q/%q, want %q", llm.Model(), llm.ReviewModel(), defaultClaudeModel)
	}
	if llm.ClaudeEffort() != claudeparam.EffortHigh {
		t.Errorf("ClaudeEffort() = %v, want EffortHigh", llm.ClaudeEffort())
	}
}
