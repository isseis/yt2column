//go:build test

package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/isseis/yt2column/internal/config"
	"github.com/isseis/yt2column/internal/llm"
	"github.com/isseis/yt2column/internal/llm/claude"
	"github.com/isseis/yt2column/internal/llm/deepseek"
	"github.com/isseis/yt2column/internal/llm/llmhttp/llmhttptest"
	llmtestutil "github.com/isseis/yt2column/internal/llm/testutil"
)

const (
	// testModel is the model name loaded from the test environment.
	testModel = "deepseek-flash"
	// testClaudeModel is the model name of a claude test environment.
	testClaudeModel = "claude-opus-5-5"
	// testAPIKey is a fixed, non-secret value. No test uses a real key.
	testAPIKey = "sk-test-key-0123456789"
	// testClaudeWorkspaceID is a fixed, non-secret workspace id.
	testClaudeWorkspaceID = "wrkspc_provider_test"
	// stopFixture is the committed DeepSeek stop-response fixture, relative to
	// this package directory.
	stopFixture = "../../../testdata/deepseek_chat_completion_stop.json"
	// claudeEndTurnFixture is the committed Claude end-turn fixture, relative
	// to this package directory.
	claudeEndTurnFixture = "../../../testdata/claude_messages_end_turn.json"
)

// testProxy is the proxy TestMain installed. Tests read it to confirm that the
// production endpoints are routed to the blackhole listener.
var testProxy *llmhttptest.BlackholeProxy

// TestMain routes every non-loopback request through a blackhole listener via
// the proxy environment variables, so a unit test that accidentally targets a
// production endpoint fails without reaching the network.
func TestMain(m *testing.M) {
	proxy, err := llmhttptest.StartBlackholeProxy()
	if err != nil {
		fmt.Fprintf(os.Stderr, "start test proxy: %v\n", err)
		os.Exit(1)
	}
	testProxy = proxy
	for _, name := range []string{"HTTPS_PROXY", "https_proxy", "HTTP_PROXY", "http_proxy"} {
		if err := os.Setenv(name, proxy.URL()); err != nil {
			fmt.Fprintf(os.Stderr, "set %s: %v\n", name, err)
			os.Exit(1)
		}
	}
	for _, name := range []string{"NO_PROXY", "no_proxy"} {
		if err := os.Unsetenv(name); err != nil {
			fmt.Fprintf(os.Stderr, "unset %s: %v\n", name, err)
			os.Exit(1)
		}
	}
	code := m.Run()
	proxy.Stop()
	os.Exit(code)
}

// lookupFrom returns a config.LookupFunc over env. A key absent from env
// reports unset, so the test controls the environment without touching the
// process environment.
func lookupFrom(env map[string]string) config.LookupFunc {
	return func(name string) (string, bool) {
		value, ok := env[name]
		return value, ok
	}
}

// validEnv is an environment config.Load accepts with the deepseek provider.
func validEnv() map[string]string {
	return map[string]string{
		"YT2COLUMN_LLM_PROVIDER": "deepseek",
		"YT2COLUMN_MODEL":        testModel,
		"DEEPSEEK_API_KEY":       testAPIKey,
		"YT2COLUMN_CACHE_DIR":    "/tmp/yt2column-provider-test",
	}
}

// claudeValidEnv is an environment config.Load accepts with the claude
// provider.
func claudeValidEnv() map[string]string {
	return map[string]string{
		"YT2COLUMN_LLM_PROVIDER":  "claude",
		"YT2COLUMN_MODEL":         testClaudeModel,
		"ANTHROPIC_API_KEY":       testAPIKey,
		"YT2COLUMN_CLAUDE_EFFORT": "high",
		"YT2COLUMN_CACHE_DIR":     "/tmp/yt2column-provider-test",
	}
}

// loadConfig loads env into a Config or fails the test.
func loadConfig(t *testing.T, env map[string]string) config.Config {
	t.Helper()
	cfg, err := config.Load(lookupFrom(env))
	if err != nil {
		t.Fatalf("config.Load() error = %v", err)
	}
	return cfg
}

// recordedRequest is one request the recording server received.
type recordedRequest struct {
	model  string
	header http.Header
}

// requestLog collects the requests a recording server received.
type requestLog struct {
	mu       sync.Mutex
	requests []recordedRequest
}

// add appends one recorded request.
func (l *requestLog) add(r recordedRequest) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.requests = append(l.requests, r)
}

// only returns the single recorded request or fails the test.
func (l *requestLog) only(t *testing.T) recordedRequest {
	t.Helper()
	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.requests) != 1 {
		t.Fatalf("server received %d requests, want exactly 1", len(l.requests))
	}
	return l.requests[0]
}

// newRecordingServer starts a loopback server that records each request and
// answers it with the committed stop-response fixture.
func newRecordingServer(t *testing.T) (*httptest.Server, *requestLog) {
	t.Helper()
	body, err := os.ReadFile(stopFixture) //nolint:gosec // the path names a committed testdata fixture
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	log := &requestLog{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		var decoded struct {
			Model string `json:"model"`
		}
		if err := json.NewDecoder(req.Body).Decode(&decoded); err != nil {
			t.Errorf("decode request body: %v", err)
		}
		log.add(recordedRequest{model: decoded.Model, header: req.Header.Clone()})
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write(body); err != nil {
			t.Errorf("write response body: %v", err)
		}
	}))
	t.Cleanup(server.Close)
	return server, log
}

// TestNewDeepSeekSendsConfiguredRequest verifies that newClient builds the
// DeepSeek adapter with the configured API key, model, and LLMTimeout, and
// that the built client sends those values to the loopback destination.
func TestNewDeepSeekSendsConfiguredRequest(t *testing.T) {
	cfg := loadConfig(t, validEnv())
	server, log := newRecordingServer(t)

	var receivedTimeout time.Duration
	build := func(opts deepseek.Options) (llm.LLMClient, error) {
		receivedTimeout = opts.Timeout
		return deepseek.NewForLoopbackTest(t, opts, server.URL), nil
	}

	client, err := newClient(cfg.Provider(), cfg, builders{deepseek: build})
	if err != nil {
		t.Fatalf("newClient() error = %v", err)
	}
	if receivedTimeout != 15*time.Minute {
		t.Errorf("build received Timeout = %v, want 15m", receivedTimeout)
	}
	if _, err := client.Generate(context.Background(), llm.GenerateRequest{SystemPrompt: "system", UserPrompt: "user"}); err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	recorded := log.only(t)
	if recorded.model != testModel {
		t.Errorf("request model = %q, want %q", recorded.model, testModel)
	}
	if got := recorded.header.Get("Authorization"); got != "Bearer "+testAPIKey {
		t.Errorf("Authorization = %q, want the Bearer test key", got)
	}
}

// TestNewUsesDeepSeekAdapter verifies that New passes the production DeepSeek
// constructor to newClient by checking the dynamic type of the built client.
func TestNewUsesDeepSeekAdapter(t *testing.T) {
	cfg := loadConfig(t, validEnv())
	client, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	typ := reflect.TypeOf(client)
	if typ == nil || typ.Kind() != reflect.Pointer {
		t.Fatalf("New() returned %v, want a pointer", typ)
	}
	const wantPkg = "github.com/isseis/yt2column/internal/llm/deepseek"
	if got := typ.Elem().PkgPath(); got != wantPkg {
		t.Errorf("New() returned a client from %q, want %q", got, wantPkg)
	}
}

// TestNewUnknownProvider verifies that the zero and out-of-range provider
// values fail with errUnknownProvider and never call either adapter
// constructor.
func TestNewUnknownProvider(t *testing.T) {
	deepseekCalled, claudeCalled := false, false
	build := builders{
		deepseek: func(deepseek.Options) (llm.LLMClient, error) {
			deepseekCalled = true
			return nil, errors.New("the deepseek constructor must not be called")
		},
		claude: func(claude.Options) (llm.LLMClient, error) {
			claudeCalled = true
			return nil, errors.New("the claude constructor must not be called")
		},
	}
	cfg := loadConfig(t, validEnv())
	for _, provider := range []config.Provider{config.ProviderUnset, config.Provider(99)} {
		client, err := newClient(provider, cfg, build)
		if !errors.Is(err, errUnknownProvider) {
			t.Errorf("newClient(%d) error = %v, want errUnknownProvider", provider, err)
		}
		if client != nil {
			t.Errorf("newClient(%d) = %v, want nil", provider, client)
		}
	}
	if deepseekCalled || claudeCalled {
		t.Error("newClient called an adapter constructor for an unknown provider")
	}
}

// TestNewClaudeSendsConfiguredRequest verifies that newClient builds the Claude
// adapter with the configured API key, model, effort, and workspace ID, and
// that the built client sends those values to the loopback destination.
func TestNewClaudeSendsConfiguredRequest(t *testing.T) {
	body, err := os.ReadFile(claudeEndTurnFixture) //nolint:gosec // the path names a committed testdata fixture
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	efforts := []string{"low", "medium", "high", "xhigh", "max"}
	for _, effort := range efforts {
		t.Run(effort, func(t *testing.T) {
			env := claudeValidEnv()
			env["YT2COLUMN_CLAUDE_EFFORT"] = effort
			recorded := claudeRequestFor(t, env, body)
			if got := recorded.Header.Get("x-api-key"); got != testAPIKey {
				t.Errorf("x-api-key = %q, want the test key", got)
			}
			var decoded struct {
				Model        string `json:"model"`
				OutputConfig struct {
					Effort string `json:"effort"`
				} `json:"output_config"`
			}
			if err := json.Unmarshal(recorded.Body, &decoded); err != nil {
				t.Fatalf("decode request body: %v", err)
			}
			if decoded.Model != testClaudeModel {
				t.Errorf("request model = %q, want %q", decoded.Model, testClaudeModel)
			}
			if decoded.OutputConfig.Effort != effort {
				t.Errorf("output_config.effort = %q, want %q", decoded.OutputConfig.Effort, effort)
			}
		})
	}

	workspaces := []struct {
		name  string
		value string
	}{
		{"with workspace", testClaudeWorkspaceID},
		{"without workspace", ""},
	}
	for _, workspace := range workspaces {
		t.Run(workspace.name, func(t *testing.T) {
			env := claudeValidEnv()
			if workspace.value != "" {
				env["ANTHROPIC_WORKSPACE_ID"] = workspace.value
			}
			recorded := claudeRequestFor(t, env, body)
			if got := recorded.Header.Get("anthropic-workspace-id"); got != workspace.value {
				t.Errorf("anthropic-workspace-id = %q, want %q", got, workspace.value)
			}
		})
	}
}

// claudeRequestFor builds the Claude adapter for env through newClient against a
// recording loopback server that answers with body, sends one Generate, and
// returns the recorded request. It checks the LLMTimeout the constructor
// received, which no request field exposes.
func claudeRequestFor(t *testing.T, env map[string]string, body []byte) llmhttptest.Recorded {
	t.Helper()
	cfg := loadConfig(t, env)
	server, recorder := llmhttptest.NewRecordingServer(t, body)
	build := func(opts claude.Options) (llm.LLMClient, error) {
		if opts.Timeout != 15*time.Minute {
			t.Errorf("build received Timeout = %v, want 15m", opts.Timeout)
		}
		return claude.NewForLoopbackTest(t, opts, server.URL), nil
	}
	client, err := newClient(cfg.Provider(), cfg, builders{claude: build})
	if err != nil {
		t.Fatalf("newClient() error = %v", err)
	}
	if _, err := client.Generate(context.Background(), llm.GenerateRequest{SystemPrompt: "system", UserPrompt: "user"}); err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	return recorder.Only(t)
}

// TestNewUsesClaudeAdapter verifies that New passes the production Claude
// constructor to newClient by checking the dynamic type of the built client.
func TestNewUsesClaudeAdapter(t *testing.T) {
	cfg := loadConfig(t, claudeValidEnv())
	client, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	typ := reflect.TypeOf(client)
	if typ == nil || typ.Kind() != reflect.Pointer {
		t.Fatalf("New() returned %v, want a pointer", typ)
	}
	const wantPkg = "github.com/isseis/yt2column/internal/llm/claude"
	if got := typ.Elem().PkgPath(); got != wantPkg {
		t.Errorf("New() returned a client from %q, want %q", got, wantPkg)
	}
}

// TestNewDeepSeekIgnoresClaudeVars verifies that the claude provider variables
// do not change the deepseek.Options newClient builds for the deepseek
// provider.
func TestNewDeepSeekIgnoresClaudeVars(t *testing.T) {
	modified := maps.Clone(validEnv())
	modified["ANTHROPIC_API_KEY"] = "sk-ant-key"
	modified["YT2COLUMN_CLAUDE_EFFORT"] = "high"
	modified["ANTHROPIC_WORKSPACE_ID"] = "wrkspc_x"

	baseline := deepseekOptionsFor(t, validEnv())
	got := deepseekOptionsFor(t, modified)
	if got.Model != baseline.Model {
		t.Errorf("Model = %q, want %q", got.Model, baseline.Model)
	}
	if got.Timeout != baseline.Timeout {
		t.Errorf("Timeout = %v, want %v", got.Timeout, baseline.Timeout)
	}
	gotKey, err := got.APIKey.Reveal()
	if err != nil {
		t.Fatalf("APIKey.Reveal() error = %v", err)
	}
	wantKey, err := baseline.APIKey.Reveal()
	if err != nil {
		t.Fatalf("APIKey.Reveal() error = %v", err)
	}
	if gotKey != wantKey {
		t.Errorf("APIKey = %q, want %q", gotKey, wantKey)
	}
}

// deepseekOptionsFor builds the deepseek adapter for env through newClient and
// returns the Options the constructor received.
func deepseekOptionsFor(t *testing.T, env map[string]string) deepseek.Options {
	t.Helper()
	cfg := loadConfig(t, env)
	var received deepseek.Options
	build := func(opts deepseek.Options) (llm.LLMClient, error) {
		received = opts
		return &llmtestutil.FakeLLMClient{}, nil
	}
	if _, err := newClient(cfg.Provider(), cfg, builders{deepseek: build}); err != nil {
		t.Fatalf("newClient() error = %v", err)
	}
	return received
}

// TestUnitTestsCannotReachProductionEndpoint verifies that a client New builds
// with the production Claude endpoint cannot reach it: the proxy environment
// routes the request to the blackhole listener.
func TestUnitTestsCannotReachProductionEndpoint(t *testing.T) {
	productionURL := &url.URL{Scheme: "https", Host: "api.anthropic.com", Path: "/v1/messages"}
	proxyURL, err := http.ProxyFromEnvironment(&http.Request{URL: productionURL})
	if err != nil {
		t.Fatalf("ProxyFromEnvironment error = %v", err)
	}
	if proxyURL == nil || proxyURL.String() != testProxy.URL() {
		t.Fatalf("proxy for %s = %v, want %s", productionURL, proxyURL, testProxy.URL())
	}

	client, err := New(loadConfig(t, claudeValidEnv()))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	before := testProxy.Accepted()
	if _, err := client.Generate(context.Background(), llm.GenerateRequest{SystemPrompt: "system", UserPrompt: "user"}); err == nil {
		t.Error("Generate() error = nil, want a failure")
	}
	if testProxy.Accepted() <= before {
		t.Error("the blackhole listener accepted no connection; the request did not use the test proxy")
	}
}

// TestNewPaddedModel verifies that New reports the adapter's padded-model
// rejection so a caller can tell it apart with errors.Is.
func TestNewPaddedModel(t *testing.T) {
	env := validEnv()
	env["YT2COLUMN_MODEL"] = " deepseek-flash"
	cfg := loadConfig(t, env)
	client, err := New(cfg)
	if !errors.Is(err, deepseek.ErrPaddedModel) {
		t.Errorf("New() error = %v, want it to wrap deepseek.ErrPaddedModel", err)
	}
	if client != nil {
		t.Errorf("New() = %v, want nil", client)
	}
}
