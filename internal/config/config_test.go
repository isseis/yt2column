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
		providerEnv: "deepseek",
		modelEnv:    "deepseek-chat",
		apiKeyEnv:   "sk-test-key",
		slackEnv:    "https://hooks.slack.com/services/T000/B000/XXXX",
		cacheDirEnv: "/tmp/yt2column-test-cache",
		ytDlpEnv:    "/usr/local/bin/yt-dlp",
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
	if cfg.Provider() != ProviderDeepSeek {
		t.Errorf("Provider() = %v, want ProviderDeepSeek", cfg.Provider())
	}
	if cfg.Model() != env[modelEnv] {
		t.Errorf("Model() = %q, want %q", cfg.Model(), env[modelEnv])
	}
	key, err := cfg.DeepSeekAPIKey().Reveal()
	if err != nil {
		t.Fatalf("DeepSeekAPIKey().Reveal() error = %v", err)
	}
	if key != env[apiKeyEnv] {
		t.Errorf("DeepSeekAPIKey() = %q, want %q", key, env[apiKeyEnv])
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
		modelEnv:  "deepseek-chat",
		apiKeyEnv: "sk-test-key",
		"HOME":    home,
		// XDG_CACHE_HOME is set so the test exercises the default on both
		// darwin and the other Unix systems without reading the process
		// environment.
		"XDG_CACHE_HOME": filepath.Join(home, ".cache"),
	}
	cfg, err := Load(envLookup(env))
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}
	if cfg.Provider() != ProviderDeepSeek {
		t.Errorf("Provider() = %v, want ProviderDeepSeek", cfg.Provider())
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
		{"provider unset and API key unset", without(base, providerEnv, apiKeyEnv), apiKeyEnv},
		{"API key unset with deepseek", without(base, apiKeyEnv), apiKeyEnv},
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
	vars := []string{providerEnv, modelEnv, apiKeyEnv, slackEnv, cacheDirEnv, ytDlpEnv}
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

func TestLoadInvalid(t *testing.T) {
	cases := []struct {
		name string
		vr   string
		bad  []string
	}{
		{"provider", providerEnv, []string{"DeepSeek", " deepseek", "deepseek ", "gemini", "claude"}},
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

func TestLoadErrorsOmitValues(t *testing.T) {
	const (
		markProvider = "MARK-PROVIDER-A1"
		markModel    = "MARK-MODEL-B2"
		markAPIKey   = "MARK-APIKEY-C3"
		markSlack    = "MARK-SLACK-D4"
		markCache    = "MARK-CACHE-E5"
		markYtDlp    = "MARK-YTDLP-F6"
	)
	marks := []string{markProvider, markModel, markAPIKey, markSlack, markCache, markYtDlp}
	base := map[string]string{
		providerEnv: "deepseek",
		modelEnv:    markModel,
		apiKeyEnv:   markAPIKey,
		slackEnv:    "https://hooks.slack.com/services/" + markSlack,
		cacheDirEnv: "/tmp/" + markCache,
		ytDlpEnv:    "/bin/" + markYtDlp,
	}
	cases := []struct {
		name string
		env  map[string]string
	}{
		{"provider invalid", with(base, providerEnv, "gemini-"+markProvider)},
		{"model empty", with(base, modelEnv, "")},
		{"API key empty", with(base, apiKeyEnv, "")},
		{"slack webhook invalid", with(base, slackEnv, "http://"+markSlack)},
		{"slack webhook no scheme", with(base, slackEnv, markSlack+"/hooks/x")},
		{"slack webhook empty host", with(base, slackEnv, "https:///"+markSlack)},
		{"slack webhook control character", with(base, slackEnv, "https://host/"+markSlack+"\n")},
		{"slack webhook bad escape", with(base, slackEnv, "https://host/%zz"+markSlack)},
		{"cache dir relative", with(base, cacheDirEnv, markCache)},
		{"cache dir absent", without(base, cacheDirEnv)},
		{"yt-dlp path empty", with(base, ytDlpEnv, "")},
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
		key     = "sk-distinctive-secret-9tRq"
		webhook = "https://hooks.slack.com/services/distinctive-webhook-7Lwz"
	)
	env := validEnv()
	env[apiKeyEnv] = key
	env[slackEnv] = webhook
	cfg, err := Load(envLookup(env))
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}

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
		if strings.Contains(out, key) {
			t.Errorf("%s output contains the API key: %q", name, out)
		}
		if strings.Contains(out, webhook) {
			t.Errorf("%s output contains the Webhook URL: %q", name, out)
		}
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
