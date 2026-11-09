//go:build test

package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/isseis/yt2column/internal/llm/claudeparam"
	"github.com/isseis/yt2column/internal/llm/deepseekparam"
	"github.com/isseis/yt2column/internal/udstest"
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
	if cfg.CacheDir() != env[cacheDirEnv] {
		t.Errorf("CacheDir() = %q, want %q", cfg.CacheDir(), env[cacheDirEnv])
	}
	if cfg.YtDlpPath() != env[ytDlpEnv] {
		t.Errorf("YtDlpPath() = %q, want %q", cfg.YtDlpPath(), env[ytDlpEnv])
	}
}

// TestLoadDefaults checks the defaults of the variables other than the LLM
// ones: yt-dlp on PATH and the derived cache directory. The LLM
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
	vars := []string{llmEnv, reviewLLMEnv, deepSeekAPIKeyEnv, cacheDirEnv, ytDlpEnv}
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
		env = with(env, "HOME", udstest.Dir(t))
		env = with(env, slackEnv, "https://hooks.slack.com/services/T000/B000/XXXX")
		cfg, err := LoadServer(ServerLookupForTest(t, env))
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
	assertVarErrors(t, err, wantMissing(anthropicAPIKeyEnv), wantInvalid(anthropicWorkspaceIDEnv, reasonAnthropicWorkspaceID))
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

// TestLoadErrorsOmitValues checks that a rejection names the variable but never
// the value, its mark, or its tail.
func TestLoadErrorsOmitValues(t *testing.T) {
	const (
		markLLM             = "MARK-LLM-A1"
		markModel           = "MARK-MODEL-B2"
		markReviewLLM       = "MARK-REVIEW-LLM-I9"
		markRetired         = "MARK-RETIRED-J0"
		markAPIKey          = "MARK-APIKEY-C3"
		markCache           = "MARK-CACHE-E5"
		markYtDlp           = "MARK-YTDLP-F6"
		markClaudeWorkspace = "MARK-CLAUDE-WKS-C8"
		markEffort          = "MARK-EFFORT-G7"
		markLLMModel        = "MARK-LLM-MODEL-H1"
		markSecret          = "MARK-SECRET-K2"
	)
	marks := []string{
		markLLM, markModel, markReviewLLM, markRetired, markAPIKey, markCache, markYtDlp, markClaudeWorkspace, markEffort, markLLMModel, markSecret,
	}
	base := map[string]string{
		llmEnv:            "deepseek/" + markModel,
		deepSeekAPIKeyEnv: markAPIKey,
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
		}{"retired " + v.name + " with the API key empty", with(with(base, v.name, "deepseek-"+markRetired), deepSeekAPIKeyEnv, ""), nil})
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
				wantInvalid(llmEnv, reasonLLMProvider),
				wantInvalid(reviewLLMEnv, reasonLLMForm),
				wantMissing(anthropicAPIKeyEnv),
				wantInvalid(cacheDirEnv, reasonDirRelative),
			},
		},
		{
			// The retired effort variable next to its replacement with an
			// unaccepted effort part: only the effort part is reported, and the
			// provider part still selects claude, whose key is required.
			"retired claude effort and an invalid effort part",
			map[string]string{
				"YT2COLUMN_CLAUDE_EFFORT": "low",
				llmEnv:                    "claude/claude-opus-5-5+ultra",
				cacheDirEnv:               "/tmp/yt2column-test-cache",
			},
			[]VarError{
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
	)
	deepseekEnv := validEnv()
	deepseekEnv[deepSeekAPIKeyEnv] = key
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
			[]string{key},
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
func slogOutput(t *testing.T, cfg any, jsonFormat bool) string {
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

// validServerEnv is a set of variables LoadServer accepts once
// ServerLookupForTest has written the configuration file keys among them: the
// CLI environment without the cache and yt-dlp variables the server does not
// use, plus HOME, from which the file's path and the socket path are derived.
// HOME is a fresh short directory, so the socket path stays within the macOS
// limit.
func validServerEnv(t *testing.T) map[string]string {
	env := without(validEnv(), cacheDirEnv, ytDlpEnv)
	env["HOME"] = udstest.Dir(t)
	return env
}

// wantServerSocketPath is the socket path LoadServer derives from home on
// this operating system.
func wantServerSocketPath(home string) string {
	if runtime.GOOS == "darwin" {
		return home + "/Library/Application Support/yt2column/run/server.sock"
	}
	return home + "/.local/state/yt2column/run/server.sock"
}

// fileKey is want as a rejection of a configuration file key: marked
// VarSourceConfigFile, with reason in place of want's when reason is set.
func fileKey(want VarError, reason string) VarError {
	want.Source = VarSourceConfigFile
	if reason != "" {
		want.Reason = reason
	}
	return want
}

// TestLoadServerValid checks that LoadServer loads the LLM settings, the
// configured Webhook URL, the socket path derived from HOME, and the HTTP/2
// debug flag. The OS-specific socket path rules are fixed by TestSocketPath.
func TestLoadServerValid(t *testing.T) {
	env := with(validServerEnv(t), godebugEnv, "http2debug=1")
	cfg, err := LoadServer(ServerLookupForTest(t, env))
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
	if want := wantServerSocketPath(env["HOME"]); cfg.SocketPath() != want {
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
		{"", fileKey(wantMissing(llmEnv), reasonEmpty)},
		{"deepseek/", fileKey(wantInvalid(llmEnv, reasonLLMForm), "")},
	}
	for _, tc := range cases {
		t.Run(fmt.Sprintf("%q", tc.value), func(t *testing.T) {
			_, err := LoadServer(ServerLookupForTest(t, with(validServerEnv(t), llmEnv, tc.value)))
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
		env := without(validServerEnv(t), slackEnv)
		cfg, err := LoadServer(ServerLookupForTest(t, env))
		if err != nil {
			t.Fatalf("LoadServer() error = %v, want nil", err)
		}
		if _, configured := cfg.WebhookURL(); configured {
			t.Error("WebhookURL() reports set, want unset")
		}
		// The rest of the configuration is still loaded: the Webhook is the
		// only value an unset variable leaves out.
		if got, want := cfg.SocketPath(), wantServerSocketPath(env["HOME"]); got != want {
			t.Errorf("SocketPath() = %q, want %q", got, want)
		}
	})
	t.Run("empty", func(t *testing.T) {
		_, err := LoadServer(ServerLookupForTest(t, with(validServerEnv(t), slackEnv, "")))
		assertVarErrors(t, err, fileKey(wantMissing(slackEnv), reasonEmpty))
	})
	t.Run("invalid", func(t *testing.T) {
		_, err := LoadServer(ServerLookupForTest(t, with(validServerEnv(t), slackEnv, "http://host/"+mark)))
		assertVarError(t, err, ErrInvalid, slackEnv)
		if strings.Contains(err.Error(), mark) {
			t.Errorf("LoadServer() error = %q, contains the value mark %q", err, mark)
		}
	})
}

// TestServerConfigOutputRedactsWebhook checks that no rendering of a
// ServerConfig holds the Webhook URL it carries.
func TestServerConfigOutputRedactsWebhook(t *testing.T) {
	const webhook = "https://hooks.slack.com/services/distinctive-webhook-7Lwz"
	cfg, err := LoadServer(ServerLookupForTest(t, with(validServerEnv(t), slackEnv, webhook)))
	if err != nil {
		t.Fatalf("LoadServer() error = %v, want nil", err)
	}
	// Confirm the URL reached the ServerConfig before checking it is hidden,
	// so the check cannot pass because the value was never stored.
	if url, configured := cfg.WebhookURL(); !configured {
		t.Fatal("WebhookURL() reports unset, want set")
	} else if revealed, err := url.Reveal(); err != nil || revealed != webhook {
		t.Fatalf("WebhookURL().Reveal() = %q, %v, want %q", revealed, err, webhook)
	}

	outputs := map[string]string{
		"fmt %v":    fmt.Sprintf("%v", cfg),
		"fmt %+v":   fmt.Sprintf("%+v", cfg),
		"fmt %#v":   fmt.Sprintf("%#v", cfg),
		"slog text": slogOutput(t, cfg, false),
		"slog JSON": slogOutput(t, cfg, true),
	}
	data, err := json.Marshal(cfg) //nolint:staticcheck // no exported fields is the point
	if err != nil {
		t.Fatalf("json.Marshal(ServerConfig) error = %v", err)
	}
	outputs["json"] = string(data)
	for name, out := range outputs {
		if strings.Contains(out, webhook) {
			t.Errorf("%s output contains the Webhook URL: %q", name, out)
		}
	}
}

// TestLoadServerIgnoresCacheAndYtdlp checks that LoadServer neither reads nor
// checks YT2COLUMN_CACHE_DIR and YT2COLUMN_YTDLP_PATH, even when Load would
// reject them.
func TestLoadServerIgnoresCacheAndYtdlp(t *testing.T) {
	env := with(validServerEnv(t), cacheDirEnv, "relative-cache")
	env = with(env, ytDlpEnv, "")
	if _, err := LoadServer(ServerLookupForTest(t, env)); err != nil {
		t.Fatalf("LoadServer() error = %v, want nil", err)
	}
	if _, err := Load(envLookup(env)); err == nil {
		t.Fatal("Load() error = nil, want a rejection of the cache directory or the yt-dlp path")
	}
}

// TestLoadServerSocketPathUnderivable checks that a HOME from which no path
// can be derived ends the load with a *VarError naming HOME, its reason, and a
// zero ServerConfig. An unset HOME stops at the configuration file's path, the
// first stage; a HOME too long for the socket path passes the file's stages
// and stops at the socket path. TestSocketPath fixes the input shapes, the
// OS-specific branches, and the length limit; here each kind of failure must
// reach LoadServer.
func TestLoadServerSocketPathUnderivable(t *testing.T) {
	const limit = 107 // the longest limit of any operating system
	tooLong := validServerEnv(t)
	tooLong["HOME"] += "/" + strings.Repeat("h", limit)
	cases := []struct {
		name     string
		env      map[string]string
		sentinel error
		reason   string
	}{
		{"unset", without(validServerEnv(t), "HOME"), ErrMissing, reasonConfigPathAbsent},
		{"too long", tooLong, ErrInvalid, reasonSocketPathTooLong},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := LoadServer(ServerLookupForTest(t, tc.env))
			assertVarError(t, err, tc.sentinel, "HOME")
			if varErr, ok := errors.AsType[*VarError](err); ok && varErr.Reason != tc.reason {
				t.Errorf("VarError.Reason = %q, want %q", varErr.Reason, tc.reason)
			}
			if cfg.SocketPath() != "" || cfg.LLM().Generation().Model() != "" {
				t.Errorf("LoadServer() returned a non-zero ServerConfig on error: socketPath=%q model=%q",
					cfg.SocketPath(), cfg.LLM().Generation().Model())
			}
		})
	}
}

// TestLoadHTTP2Debug checks which GODEBUG values both entry points report as
// enabling the HTTP/2 log.
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
			// The server reads GODEBUG from the environment too, while its
			// other settings come from the configuration file.
			server, err := LoadServer(ServerLookupForTest(t, with(tc.env, "HOME", udstest.Dir(t))))
			if err != nil {
				t.Fatalf("LoadServer() error = %v, want nil", err)
			}
			if got := server.HTTP2DebugEnabled(); got != tc.want {
				t.Errorf("LoadServer().HTTP2DebugEnabled() = %v, want %v", got, tc.want)
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
		// LLM variable at all no default is applied. The retired variables
		// are not read either.
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
				for _, name := range slices.Concat(llmVars, retiredVarNames) {
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

// TestLoadServerLLMDefaults checks the server entry point with no LLM key.
// The rules themselves are covered by the Load tests, which LoadServer
// shares.
func TestLoadServerLLMDefaults(t *testing.T) {
	env := map[string]string{
		"HOME":             udstest.Dir(t),
		slackEnv:           "https://hooks.slack.com/services/T000/B000/XXXX",
		anthropicAPIKeyEnv: "sk-ant-test-key",
	}
	cfg, err := LoadServer(ServerLookupForTest(t, env))
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
			got := strings.Split(list, effortValuesSeparator)
			if !slices.Equal(got, tc.want) {
				t.Errorf("listed values = %q, want %q", got, tc.want)
			}
			// Each listed value is one parseLLM accepts for the provider.
			for _, value := range got {
				if _, err := parseLLM(llmEnv, tc.name+"/model+"+value); err != nil {
					t.Errorf("parseLLM(%s/model+%s) error = %v, want the listed value accepted", tc.name, value, err)
				}
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
	// The wording "whose provider is claude" is the observable contract of
	// the line a user reads, so it is matched as written.
	fragments := []string{
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

// TestLoadForIgnoresRetiredVars checks that the retired LLM variables, set
// empty or with a value that would select deepseek, stop no load whatever the
// stages and change nothing it returns: they are not read.
func TestLoadForIgnoresRetiredVars(t *testing.T) {
	needs := []struct {
		name string
		need Need
	}{
		{"both", needBoth}, {"generation", needGeneration}, {"review", needReview}, {"none", Need{}},
	}
	bases := []struct {
		name string
		env  map[string]string
	}{
		{"LLM unset", defaultsBaseEnv()},
		{"LLM set", with(defaultsBaseEnv(), llmEnv, "claude/claude-opus-5-5")},
		{"both stages deepseek", withVars(defaultsBaseEnv(), map[string]string{
			llmEnv: "deepseek/deepseek-flash", reviewLLMEnv: "deepseek/deepseek-flash", deepSeekAPIKeyEnv: "sk-test-key",
		})},
	}
	for _, base := range bases {
		for _, n := range needs {
			want, err := LoadFor(envLookup(base.env), n.need)
			if err != nil {
				t.Fatalf("%s %s: LoadFor() without retired variables error = %v, want nil", base.name, n.name, err)
			}
			for _, value := range []string{"deepseek", ""} {
				t.Run(fmt.Sprintf("%s %s retired=%q", base.name, n.name, value), func(t *testing.T) {
					env := maps.Clone(base.env)
					for _, name := range retiredVarNames {
						env[name] = value
					}
					got, err := LoadFor(envLookup(env), n.need)
					if err != nil {
						t.Fatalf("LoadFor() error = %v, want nil", err)
					}
					if viewLLM(got.LLM()) != viewLLM(want.LLM()) {
						t.Errorf("LoadFor() = %+v, want %+v as without the retired variables", viewLLM(got.LLM()), viewLLM(want.LLM()))
					}
				})
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

// TestVarErrorSource checks the two forms of a rejection's text: the zero
// Source keeps "<Name>: <Reason>", and VarSourceConfigFile names the key as a
// configuration file key, never as an environment variable.
func TestVarErrorSource(t *testing.T) {
	cases := []struct {
		source VarSource
		want   string
	}{
		{VarSourceUnspecified, llmEnv + ": " + reasonLLMForm},
		{VarSourceConfigFile, "configuration file key " + llmEnv + ": " + reasonLLMForm},
		// A value outside the enum falls back to the plain form.
		{VarSource(99), llmEnv + ": " + reasonLLMForm},
	}
	for _, tc := range cases {
		err := &VarError{Name: llmEnv, Reason: reasonLLMForm, Err: ErrInvalid, Source: tc.source}
		if got := err.Error(); got != tc.want {
			t.Errorf("Source %d: Error() = %q, want %q", tc.source, got, tc.want)
		}
	}
}

// serverEnvWithFile writes data as the configuration file under a fresh
// short HOME and returns that environment, which holds HOME alone.
func serverEnvWithFile(t *testing.T, data []byte) map[string]string {
	t.Helper()
	home := udstest.Dir(t)
	WriteServerFileForTest(t, home, data)
	return map[string]string{"HOME": home}
}

// serverView is the comparable part of a ServerConfig: the two selections,
// the revealed keys, the workspace ID, and the revealed Webhook URL. viewLLM
// fills all but the Webhook URL, which an LLMConfig does not hold.
type serverView struct {
	generation, review     LLMSelection
	deepSeekKey, claudeKey string
	workspaceID            claudeparam.WorkspaceID
	webhook                string
}

// viewServer returns the comparable part of a ServerConfig.
func viewServer(cfg ServerConfig) serverView {
	v := viewLLM(cfg.LLM())
	if url, ok := cfg.WebhookURL(); ok {
		v.webhook, _ = url.Reveal()
	}
	return v
}

// viewLLM returns the comparable part of an LLMConfig.
func viewLLM(c LLMConfig) serverView {
	deepSeek, _ := c.DeepSeekAPIKey().Reveal()
	claude, _ := c.AnthropicAPIKey().Reveal()
	return serverView{
		generation: c.Generation(), review: c.Review(),
		deepSeekKey: deepSeek, claudeKey: claude, workspaceID: c.AnthropicWorkspaceID(),
	}
}

// TestLoadServerReadsFile checks that the keys of the configuration file take
// effect with the rules of the environment variables: the Anthropic API key
// alone selects the default for every stage, and a deepseek YT2COLUMN_LLM with
// its key selects deepseek for the generation and, by inheritance, the review.
func TestLoadServerReadsFile(t *testing.T) {
	t.Run("Anthropic API key only", func(t *testing.T) {
		env := serverEnvWithFile(t, []byte(anthropicAPIKeyEnv+"=sk-ant-file\n"))
		cfg, err := LoadServer(envLookup(env))
		if err != nil {
			t.Fatalf("LoadServer() error = %v, want nil", err)
		}
		want := LLMSelection{provider: ProviderClaude, model: defaultClaudeModel, claudeEffort: claudeparam.EffortHigh}
		got := viewServer(cfg)
		if got.generation != want || got.review != want {
			t.Errorf("selections = %+v/%+v, want %+v for both", got.generation, got.review, want)
		}
		if got.claudeKey != "sk-ant-file" {
			t.Errorf("Anthropic API key = %q, want the file's", got.claudeKey)
		}
	})
	t.Run("deepseek", func(t *testing.T) {
		env := serverEnvWithFile(t, []byte(llmEnv+"=deepseek/deepseek-flash\n"+deepSeekAPIKeyEnv+"=sk-file\n"))
		cfg, err := LoadServer(envLookup(env))
		if err != nil {
			t.Fatalf("LoadServer() error = %v, want nil", err)
		}
		want := LLMSelection{provider: ProviderDeepSeek, model: "deepseek-flash"}
		got := viewServer(cfg)
		if got.generation != want || got.review != want {
			t.Errorf("selections = %+v/%+v, want %+v for both", got.generation, got.review, want)
		}
		if got.deepSeekKey != "sk-file" {
			t.Errorf("DeepSeek API key = %q, want the file's", got.deepSeekKey)
		}
	})
}

// TestLoadServerValueRejections checks that the validation reports every
// value it rejects, each marked as a configuration file key, and returns the
// zero ServerConfig. It holds the requirements' examples, YT2COLUMN_LLM=Claude/x
// and SLACK_WEBHOOK_URL=http://example.com/hook; the review stage selects
// claude, so the missing Anthropic API key is reported too.
func TestLoadServerValueRejections(t *testing.T) {
	file := map[string]string{
		llmEnv:                  "Claude/x",
		reviewLLMEnv:            "claude/",
		anthropicWorkspaceIDEnv: "a b",
		slackEnv:                "http://example.com/hook",
	}
	cfg, err := LoadServer(envLookup(serverEnvWithFile(t, ServerFileForTest(file))))
	assertVarErrors(t, err,
		fileKey(wantInvalid(llmEnv, reasonLLMProvider), ""),
		fileKey(wantInvalid(reviewLLMEnv, reasonLLMForm), ""),
		fileKey(wantMissing(anthropicAPIKeyEnv), reasonNotSet),
		fileKey(wantInvalid(anthropicWorkspaceIDEnv, reasonAnthropicWorkspaceID), ""),
		fileKey(wantInvalid(slackEnv, reasonSlackWebhook), ""),
	)
	if got := viewServer(cfg); got != (serverView{}) || cfg.SocketPath() != "" {
		t.Errorf("LoadServer() returned a non-zero ServerConfig on error: %+v", got)
	}
}

// TestLoadServerEmptyValues checks that an empty value in the file is handled
// as an empty environment variable is: a key the configuration uses is
// rejected as empty, and the key of a provider no stage uses is not read.
func TestLoadServerEmptyValues(t *testing.T) {
	t.Run("used key", func(t *testing.T) {
		_, err := LoadServer(envLookup(serverEnvWithFile(t, []byte(anthropicAPIKeyEnv+"=\n"))))
		assertVarErrors(t, err, fileKey(wantMissing(anthropicAPIKeyEnv), reasonEmpty))
	})
	t.Run("unused provider key", func(t *testing.T) {
		data := []byte(anthropicAPIKeyEnv + "=sk-ant-file\n" + deepSeekAPIKeyEnv + "=\n")
		if _, err := LoadServer(envLookup(serverEnvWithFile(t, data))); err != nil {
			t.Fatalf("LoadServer() error = %v, want nil", err)
		}
	})
}

// retiredVarNames are the retired LLM variables, written out here: the
// server must ignore them in its environment.
var retiredVarNames = []string{
	"YT2COLUMN_LLM_PROVIDER", "YT2COLUMN_MODEL",
	"YT2COLUMN_REVIEW_PROVIDER", "YT2COLUMN_REVIEW_MODEL", "YT2COLUMN_CLAUDE_EFFORT",
}

// TestLoadServerIgnoresEnvSettings checks the requirements' examples of a
// setting only the process environment holds: an API key there is not set in
// the file, and an LLM choice there selects nothing. That the server reads no
// such variable at all, empty or not, is TestLoadServerReadsOnlyHomeAndGodebug.
func TestLoadServerIgnoresEnvSettings(t *testing.T) {
	t.Run("API key only in the environment", func(t *testing.T) {
		env := serverEnvWithFile(t, []byte(serverFileTemplate))
		env[anthropicAPIKeyEnv] = "sk-ant-env"
		_, err := LoadServer(envLookup(env))
		assertVarErrors(t, err, fileKey(wantMissing(anthropicAPIKeyEnv), reasonNotSet))
	})
	t.Run("LLM choice only in the environment", func(t *testing.T) {
		env := serverEnvWithFile(t, []byte(anthropicAPIKeyEnv+"=sk-ant-file\n"))
		env[llmEnv] = "deepseek/deepseek-flash"
		env[deepSeekAPIKeyEnv] = "sk-env"
		cfg, err := LoadServer(envLookup(env))
		if err != nil {
			t.Fatalf("LoadServer() error = %v, want nil", err)
		}
		if got := cfg.LLM().Generation(); got.Provider() != ProviderClaude || got.Model() != defaultClaudeModel {
			t.Errorf("generation = %+v, want the default %s", got, defaultLLMValue)
		}
	})
}

// TestLoadServerFileRejections checks that each refusal of the configuration
// file ends the load with a *FileError of its class naming the file's path,
// and the zero ServerConfig, and that the size limit is inclusive.
func TestLoadServerFileRejections(t *testing.T) {
	valid := []byte(anthropicAPIKeyEnv + "=sk-ant-file\n")
	// padded returns valid followed by a comment line, size bytes in all.
	padded := func(size int) []byte {
		return slices.Concat(valid, []byte("#"), bytes.Repeat([]byte("x"), size-len(valid)-1))
	}
	cases := []struct {
		name     string
		setup    func(t *testing.T, path string) // changes the valid file at path
		sentinel error
		line     int
	}{
		{"missing", func(t *testing.T, path string) { removeForTest(t, path) }, ErrConfigFileMissing, 0},
		{"symbolic link", func(t *testing.T, path string) {
			target := filepath.Join(filepath.Dir(path), "target.env")
			writeModeForTest(t, target, valid, 0o600)
			removeForTest(t, path)
			if err := os.Symlink(target, path); err != nil {
				t.Fatal(err)
			}
		}, ErrConfigFileUnsafe, 0},
		{"directory", func(t *testing.T, path string) {
			removeForTest(t, path)
			if err := os.Mkdir(path, 0o700); err != nil {
				t.Fatal(err)
			}
		}, ErrConfigFileUnsafe, 0},
		{"mode 0644", func(t *testing.T, path string) { chmodForTest(t, path, 0o644) }, ErrConfigFileUnsafe, 0},
		{"unreadable", func(t *testing.T, path string) {
			skipIfRoot(t)
			dir := filepath.Dir(path)
			chmodForTest(t, dir, 0o000)
			t.Cleanup(func() { chmodForTest(t, dir, 0o700) })
		}, ErrConfigFileUnreadable, 0},
		{"format", func(t *testing.T, path string) { writeModeForTest(t, path, []byte("not a setting\n"), 0o600) }, ErrConfigFileFormat, 1},
		{"one byte over the limit", func(t *testing.T, path string) {
			writeModeForTest(t, path, padded(serverFileMaxSize+1), 0o600)
		}, ErrConfigFileFormat, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			home := udstest.Dir(t)
			path := WriteServerFileForTest(t, home, valid)
			tc.setup(t, path)
			cfg, err := LoadServer(envLookup(map[string]string{"HOME": home}))
			if !errors.Is(err, tc.sentinel) {
				t.Fatalf("LoadServer() error = %v, want it to wrap %v", err, tc.sentinel)
			}
			fileErr, ok := errors.AsType[*FileError](err)
			if !ok {
				t.Fatalf("LoadServer() error = %v, want a *FileError", err)
			}
			if fileErr.Path != path || fileErr.Line != tc.line {
				t.Errorf("FileError path, line = %q, %d, want %q, %d", fileErr.Path, fileErr.Line, path, tc.line)
			}
			if viewServer(cfg) != (serverView{}) || cfg.SocketPath() != "" {
				t.Errorf("LoadServer() returned a non-zero ServerConfig on error")
			}
		})
	}
	// A rejected line stops the load before the values are validated: the
	// invalid value and the missing API key of this file are not reported.
	t.Run("format stops before the values", func(t *testing.T) {
		data := []byte(llmEnv + "=deepseek/\nnot a setting\n")
		_, err := LoadServer(envLookup(serverEnvWithFile(t, data)))
		if _, ok := errors.AsType[*VarError](err); ok || !errors.Is(err, ErrConfigFileFormat) {
			t.Errorf("LoadServer() error = %v, want only the format rejection", err)
		}
	})
	t.Run("exactly the limit", func(t *testing.T) {
		data := padded(serverFileMaxSize)
		if len(data) != serverFileMaxSize {
			t.Fatalf("the file has %d bytes, want %d", len(data), serverFileMaxSize)
		}
		if _, err := LoadServer(envLookup(serverEnvWithFile(t, data))); err != nil {
			t.Fatalf("LoadServer() error = %v, want nil", err)
		}
	})
}

// removeForTest removes path.
func removeForTest(t *testing.T, path string) {
	t.Helper()
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
}

// chmodForTest sets path's mode.
func chmodForTest(t *testing.T, path string, mode os.FileMode) {
	t.Helper()
	if err := os.Chmod(path, mode); err != nil {
		t.Fatal(err)
	}
}

// writeModeForTest writes data to path and sets its mode, whatever the
// umask.
func writeModeForTest(t *testing.T, path string, data []byte, mode os.FileMode) {
	t.Helper()
	if err := os.WriteFile(path, data, mode); err != nil {
		t.Fatal(err)
	}
	chmodForTest(t, path, mode)
}

// TestLoadServerReadsOnlyHomeAndGodebug checks that the server's load reads
// HOME and GODEBUG from the environment, and no other variable, even when
// the environment holds the six keys and the retired LLM variables, which
// then leave the configuration as the file alone gives it.
func TestLoadServerReadsOnlyHomeAndGodebug(t *testing.T) {
	env := serverEnvWithFile(t, []byte(anthropicAPIKeyEnv+"=sk-ant-file\n"))
	want, err := LoadServer(envLookup(env))
	if err != nil {
		t.Fatalf("LoadServer() without environment settings error = %v, want nil", err)
	}
	for _, name := range slices.Concat(serverFileKeys, retiredVarNames) {
		env[name] = ""
	}
	var read []string
	got, err := LoadServer(recordingLookup(env, &read))
	if err != nil {
		t.Fatalf("LoadServer() error = %v, want nil", err)
	}
	if viewServer(got) != viewServer(want) {
		t.Errorf("configuration = %+v, want the file's %+v", viewServer(got), viewServer(want))
	}
	if !slices.Contains(read, "HOME") {
		t.Errorf("read = %v, want it to include HOME", read)
	}
	for _, name := range read {
		if name != "HOME" && name != godebugEnv {
			t.Errorf("LoadServer read %q from the environment, want only HOME and %s", name, godebugEnv)
		}
	}
}

// TestLoadForIgnoresServerFile checks that the CLI's load reads the
// environment alone: a configuration file under HOME, valid with other
// values, malformed, or with mode 0644, changes nothing.
func TestLoadForIgnoresServerFile(t *testing.T) {
	home := udstest.Dir(t)
	env := with(validEnv(), "HOME", home)
	want, err := Load(envLookup(env))
	if err != nil {
		t.Fatalf("Load() without a configuration file error = %v, want nil", err)
	}
	needs := []Need{needBoth, needGeneration, needReview, {}}
	wantFor := make([]serverView, len(needs))
	for i, need := range needs {
		cfg, err := LoadFor(envLookup(env), need)
		if err != nil {
			t.Fatalf("LoadFor(%+v) without a configuration file error = %v, want nil", need, err)
		}
		wantFor[i] = viewLLM(cfg.LLM())
	}
	files := []struct {
		name string
		data []byte
		mode os.FileMode
	}{
		{"other values", ServerFileForTest(map[string]string{
			llmEnv: "claude/claude-file", anthropicAPIKeyEnv: "sk-ant-file", deepSeekAPIKeyEnv: "sk-file",
		}), 0o600},
		{"malformed", []byte("export " + llmEnv + "=\"x\"\n"), 0o600},
		{"mode 0644", ServerFileForTest(map[string]string{llmEnv: "claude/claude-file"}), 0o644},
	}
	for _, f := range files {
		t.Run(f.name, func(t *testing.T) {
			path := WriteServerFileForTest(t, home, f.data)
			chmodForTest(t, path, f.mode)
			for i, need := range needs {
				got, err := LoadFor(envLookup(env), need)
				if err != nil {
					t.Fatalf("LoadFor(%+v) error = %v, want nil", need, err)
				}
				if viewLLM(got.LLM()) != wantFor[i] {
					t.Errorf("LoadFor(%+v) = %+v, want %+v as without the file", need, viewLLM(got.LLM()), wantFor[i])
				}
			}
			got, err := Load(envLookup(env))
			if err != nil {
				t.Fatalf("Load() error = %v, want nil", err)
			}
			if viewLLM(got.LLM()) != viewLLM(want.LLM()) || got.CacheDir() != want.CacheDir() {
				t.Errorf("Load() = %+v, want %+v as without the file", viewLLM(got.LLM()), viewLLM(want.LLM()))
			}
		})
	}
}
