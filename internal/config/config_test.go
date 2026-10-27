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
	if cfg.LLM().Model() != "deepseek-chat" {
		t.Errorf("Model() = %q, want deepseek-chat", cfg.LLM().Model())
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
	reviewClaude := with(validEnv(), reviewLLMEnv, "claude/claude-review")
	reviewClaude = with(reviewClaude, anthropicAPIKeyEnv, "sk-ant-test-key")
	generationClaude := with(claudeBoth, reviewLLMEnv, "deepseek/deepseek-review")
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

	t.Run("invalid provider reports only the LLM variable", func(t *testing.T) {
		env := with(validEnv(), llmEnv, "gemini/x")
		env = with(env, anthropicAPIKeyEnv, "sk-ant-key")
		env = with(env, claudeEffortEnv, "High")
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
		if cfg.LLM().ReviewProvider() != ProviderDeepSeek || cfg.LLM().ReviewModel() != "deepseek-chat" {
			t.Errorf("review = %v/%q, want the generation's deepseek/deepseek-chat",
				cfg.LLM().ReviewProvider(), cfg.LLM().ReviewModel())
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
			if cfg.LLM().Provider() != ProviderDeepSeek || cfg.LLM().Model() != "deepseek-chat" {
				t.Errorf("generation = %v/%q, want deepseek/deepseek-chat", cfg.LLM().Provider(), cfg.LLM().Model())
			}
			if cfg.LLM().ReviewProvider() != tc.wantProvider || cfg.LLM().ReviewModel() != tc.wantModel {
				t.Errorf("review = %v/%q, want %v/%q",
					cfg.LLM().ReviewProvider(), cfg.LLM().ReviewModel(), tc.wantProvider, tc.wantModel)
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
		env = with(env, claudeEffortEnv, "high")
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
		markClaudeEffort    = "MARK-CLAUDE-EFFORT-G7"
		markClaudeWorkspace = "MARK-CLAUDE-WKS-C8"
	)
	marks := []string{
		markLLM, markModel, markReviewLLM, markRetired, markAPIKey, markSlack,
		markCache, markYtDlp, markClaudeEffort, markClaudeWorkspace,
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
		{"LLM provider invalid", with(base, llmEnv, "gemini-"+markLLM+"/x"), nil},
		{"LLM without a separator", with(base, llmEnv, markLLM), nil},
		{"LLM with an empty model ID", with(base, llmEnv, "deepseek/"), nil},
		{"LLM empty", with(base, llmEnv, ""), nil},
		{"review LLM provider invalid", with(base, reviewLLMEnv, "gemini/"+markReviewLLM), nil},
		{"review LLM without a separator", with(base, reviewLLMEnv, markReviewLLM), nil},
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
				reviewLLMEnv:    "claude/",
				claudeEffortEnv: "High",
				cacheDirEnv:     "relative",
			},
			[]VarError{
				wantInvalid("YT2COLUMN_LLM_PROVIDER", reasonRetiredLLM),
				wantInvalid("YT2COLUMN_REVIEW_MODEL", reasonRetiredReviewLLM),
				wantInvalid(llmEnv, reasonLLMProvider),
				wantInvalid(reviewLLMEnv, reasonLLMForm),
				wantMissing(anthropicAPIKeyEnv),
				wantInvalid(claudeEffortEnv, reasonClaudeEffort),
				wantInvalid(cacheDirEnv, reasonDirRelative),
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
	if cfg.LLM().Model() != "deepseek-chat" {
		t.Errorf("LLM().Model() = %q, want deepseek-chat", cfg.LLM().Model())
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
		// A review value that would be rejected if read is neither required
		// nor read when the review step does not run.
		env := with(validEnv(), reviewLLMEnv, "gemini/")
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
		if cfg.LLM().ReviewProvider() != ProviderClaude || cfg.LLM().ReviewModel() != "claude-opus-5-5" {
			t.Errorf("review = %v/%q, want claude/claude-opus-5-5", cfg.LLM().ReviewProvider(), cfg.LLM().ReviewModel())
		}
		if cfg.LLM().ClaudeEffort() == claudeparam.EffortUnset {
			t.Error("ClaudeEffort() is unset, want the checked value")
		}
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
			with(with(validEnv(), reviewLLMEnv, "claude/claude-review"), anthropicAPIKeyEnv, "sk-ant-test-key"),
			Need{Generation: true},
		},
		{
			"review only with a claude generation",
			with(with(without(validClaudeEnv(), claudeEffortEnv), reviewLLMEnv, "deepseek/deepseek-review"), deepSeekAPIKeyEnv, "sk-test-key"),
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
		// --after review: not even an invalid LLM value is read, and with no
		// LLM variable at all no default is applied. Only the presence of the
		// retired variables is checked, whatever the stages.
		envs := map[string]map[string]string{
			"invalid values": {
				llmEnv:          "gemini/x",
				reviewLLMEnv:    "",
				claudeEffortEnv: "High",
				cacheDirEnv:     "/tmp/yt2column-test-cache",
			},
			"no LLM variable": {
				cacheDirEnv: "/tmp/yt2column-test-cache",
			},
		}
		llmVars := []string{
			llmEnv, reviewLLMEnv,
			deepSeekAPIKeyEnv, anthropicAPIKeyEnv, claudeEffortEnv, anthropicWorkspaceIDEnv,
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

	t.Run("review only ignores an unused generation provider's variables", func(t *testing.T) {
		// The review step has its own deepseek selection, so the claude
		// generation provider and its API key, effort, and workspace are not
		// read even though YT2COLUMN_LLM is set.
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
		if cfg.LLM().ReviewProvider() != ProviderDeepSeek {
			t.Errorf("ReviewProvider() = %v, want ProviderDeepSeek", cfg.LLM().ReviewProvider())
		}
	})
}

// defaultsBaseEnv is an environment with no LLM variable: the API keys of both
// providers, an explicit effort (so the tests do not depend on the effort
// default), and the non-LLM variable Load needs. A row adds the LLM variables
// it is about.
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
			got := want{llm.Provider(), llm.Model(), llm.ReviewProvider(), llm.ReviewModel()}
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

// TestLoadServerLLMDefaults checks the server entry point with no LLM
// variable. The rules themselves are covered by the Load tests, which
// LoadServer shares; the retired variables at the server entry point are in
// TestLoadRejectsRetiredLLMVars.
func TestLoadServerLLMDefaults(t *testing.T) {
	env := map[string]string{
		"HOME":             "/home/user",
		slackEnv:           "https://hooks.slack.com/services/T000/B000/XXXX",
		claudeEffortEnv:    "high",
		anthropicAPIKeyEnv: "sk-ant-test-key",
	}
	cfg, err := LoadServer(envLookup(env))
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

// TestLoadParsesLLMValue checks each value shape of architecture section 3.2
// for both LLM variables: an accepted value selects its provider and the
// model ID after the first "/" as it is, white space and invalid UTF-8
// included (the adapters reject those when a client is built); a rejected
// value names the variable with its sentinel and fixed reason.
func TestLoadParsesLLMValue(t *testing.T) {
	cases := []struct {
		value        string
		wantProvider Provider
		wantModel    string
		wantErr      *VarError // nil when accepted; Name is filled per variable
	}{
		{"deepseek/deepseek-flash", ProviderDeepSeek, "deepseek-flash", nil},
		{"claude/claude-sonnet-5-5", ProviderClaude, "claude-sonnet-5-5", nil},
		{"claude/vendor/model-x", ProviderClaude, "vendor/model-x", nil},
		{"claude/claude-opus-5-5 ", ProviderClaude, "claude-opus-5-5 ", nil},
		{"claude/ claude-opus-5-5", ProviderClaude, " claude-opus-5-5", nil},
		{"claude/\xffmodel", ProviderClaude, "\xffmodel", nil},
		{"", 0, "", &VarError{Reason: reasonUnset, Err: ErrMissing}},
		{"claude", 0, "", &VarError{Reason: reasonLLMForm, Err: ErrInvalid}},
		{"claude/", 0, "", &VarError{Reason: reasonLLMForm, Err: ErrInvalid}},
		{"deepseek/", 0, "", &VarError{Reason: reasonLLMForm, Err: ErrInvalid}},
		{"/claude-opus-5-5", 0, "", &VarError{Reason: reasonLLMProvider, Err: ErrInvalid}},
		{"gemini/x", 0, "", &VarError{Reason: reasonLLMProvider, Err: ErrInvalid}},
		{"gemini/", 0, "", &VarError{Reason: reasonLLMProvider, Err: ErrInvalid}},
		{"Claude/claude-opus-5-5", 0, "", &VarError{Reason: reasonLLMProvider, Err: ErrInvalid}},
		{" claude/claude-opus-5-5", 0, "", &VarError{Reason: reasonLLMProvider, Err: ErrInvalid}},
		{"claude /claude-opus-5-5", 0, "", &VarError{Reason: reasonLLMProvider, Err: ErrInvalid}},
	}
	stages := []struct {
		name     string
		selected func(LLMConfig) (Provider, string)
	}{
		{llmEnv, func(c LLMConfig) (Provider, string) { return c.Provider(), c.Model() }},
		{reviewLLMEnv, func(c LLMConfig) (Provider, string) { return c.ReviewProvider(), c.ReviewModel() }},
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
				provider, model := stage.selected(cfg.LLM())
				if provider != tc.wantProvider || model != tc.wantModel {
					t.Errorf("selection = %v/%q, want %v/%q", provider, model, tc.wantProvider, tc.wantModel)
				}
			})
		}
	}
}

// TestParseLLMProvider checks parseLLM's contract on a rejected value: the
// selection keeps a provider part that was accepted, so that provider's own
// variables are still checked, and holds ProviderUnset otherwise.
func TestParseLLMProvider(t *testing.T) {
	cases := []struct {
		value string
		want  Provider
	}{
		{"claude/", ProviderClaude},
		{"deepseek/", ProviderDeepSeek},
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
			if selection.provider != tc.want || selection.model != "" {
				t.Errorf("parseLLM() = %+v, want provider %v and no model", selection, tc.want)
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
// shrinking the cases of TestLoadRejectsRetiredLLMVars with it.
var retiredLLMVarsWant = []retiredLLMVar{
	{"YT2COLUMN_LLM_PROVIDER", reasonRetiredLLM},
	{"YT2COLUMN_MODEL", reasonRetiredLLM},
	{"YT2COLUMN_REVIEW_PROVIDER", reasonRetiredReviewLLM},
	{"YT2COLUMN_REVIEW_MODEL", reasonRetiredReviewLLM},
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
// (an empty model ID). The selection that is accepted still has its provider's
// variables checked.
func TestLoadProviderVarsFollowSelection(t *testing.T) {
	claudeVars := []string{anthropicAPIKeyEnv, claudeEffortEnv, anthropicWorkspaceIDEnv}
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
