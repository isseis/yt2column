//go:build test

package provider

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/isseis/yt2column/internal/config"
	"github.com/isseis/yt2column/internal/llm"
	"github.com/isseis/yt2column/internal/llm/deepseek"
	"github.com/isseis/yt2column/internal/secret"
)

const (
	// testModel is the model name loaded from the test environment.
	testModel = "deepseek-flash"
	// testAPIKey is a fixed, non-secret value. No test uses a real key.
	testAPIKey = "sk-test-key-0123456789"
	// stopFixture is the committed DeepSeek stop-response fixture, relative to
	// this package directory.
	stopFixture = "../../../testdata/deepseek_chat_completion_stop.json"
)

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

	client, err := newClient(cfg.Provider(), cfg.DeepSeekAPIKey(), cfg.Model(), build)
	if err != nil {
		t.Fatalf("newClient() error = %v", err)
	}
	if receivedTimeout != LLMTimeout {
		t.Errorf("build received Timeout = %v, want %v", receivedTimeout, LLMTimeout)
	}
	if _, err := client.Generate(context.Background(), llm.GenerateRequest{SystemPrompt: "system", UserPrompt: "user"}); err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	recorded := log.only(t)
	if recorded.model != testModel {
		t.Errorf("request model = %q, want %q", recorded.model, testModel)
	}
	if got, want := recorded.header.Get("Authorization"), "Bearer "+testAPIKey; got != want {
		t.Errorf("Authorization = %q, want %q", got, want)
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
// values fail with errUnknownProvider and never call the adapter constructor.
func TestNewUnknownProvider(t *testing.T) {
	buildCalled := false
	build := func(deepseek.Options) (llm.LLMClient, error) {
		buildCalled = true
		return nil, errors.New("the adapter constructor must not be called")
	}
	for _, provider := range []config.Provider{config.ProviderUnset, config.Provider(99)} {
		client, err := newClient(provider, secret.Secret{}, "model", build)
		if !errors.Is(err, errUnknownProvider) {
			t.Errorf("newClient(%d) error = %v, want errUnknownProvider", provider, err)
		}
		if client != nil {
			t.Errorf("newClient(%d) = %v, want nil", provider, client)
		}
	}
	if buildCalled {
		t.Error("newClient called the adapter constructor for an unknown provider")
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
