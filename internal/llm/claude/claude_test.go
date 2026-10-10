//go:build test

package claude

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/isseis/yt2column/internal/llm"
	"github.com/isseis/yt2column/internal/llm/claudeparam"
	"github.com/isseis/yt2column/internal/llm/llmhttp/llmhttptest"
	"github.com/isseis/yt2column/internal/secret"
)

// testProxy is the proxy TestMain installed. Tests read it to confirm that the
// production endpoint is routed to the blackhole listener.
var testProxy *llmhttptest.BlackholeProxy

// TestMain routes every non-loopback request through a blackhole listener via
// the proxy environment variables, so a unit test that accidentally targets
// the production endpoint fails without reaching the network.
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

// allEfforts returns the five accepted effort values.
func allEfforts() []claudeparam.Effort {
	return []claudeparam.Effort{
		claudeparam.EffortLow,
		claudeparam.EffortMedium,
		claudeparam.EffortHigh,
		claudeparam.EffortXHigh,
		claudeparam.EffortMax,
	}
}

// assertConstructionFailure checks the common contract of a rejected Options
// value: non-nil error, nil client, the single prefix, no API key, and no
// occurrence of the forbidden value.
func assertConstructionFailure(t *testing.T, value llm.LLMClient, err error, forbidden string) {
	t.Helper()
	if err == nil {
		t.Fatal("New() error = nil, want an error")
	}
	if value != nil {
		t.Errorf("New() = %v, want nil", value)
	}
	assertErrorPrefix(t, err)
	assertNoAPIKey(t, err)
	if forbidden != "" && strings.Contains(err.Error(), forbidden) {
		t.Errorf("New() error %q contains the rejected value %q", err, forbidden)
	}
}

func TestNew(t *testing.T) {
	validOptions := func(t *testing.T) Options {
		t.Helper()
		return Options{
			APIKey:  mustSecret(t, testAPIKey),
			Model:   testModel,
			Effort:  claudeparam.EffortHigh,
			Timeout: time.Second,
		}
	}

	t.Run("accepts valid options with and without a workspace id", func(t *testing.T) {
		for _, workspace := range []claudeparam.WorkspaceID{{}, mustWorkspaceID(t, testWorkspaceID)} {
			options := validOptions(t)
			options.WorkspaceID = workspace
			value, err := New(options)
			if err != nil {
				t.Fatalf("New() error = %v", err)
			}
			if _, ok := value.(*client); !ok {
				t.Fatalf("New() returned %T, want *client", value)
			}
		}
	})

	t.Run("accepts every effort value", func(t *testing.T) {
		for _, effort := range allEfforts() {
			options := validOptions(t)
			options.Effort = effort
			if _, err := New(options); err != nil {
				t.Errorf("New(Effort: %s) error = %v, want nil", effort, err)
			}
		}
	})

	t.Run("rejects the zero APIKey", func(t *testing.T) {
		options := validOptions(t)
		options.APIKey = secret.Secret{}
		value, err := New(options)
		assertConstructionFailure(t, value, err, "")
	})

	t.Run("rejects API keys with non-printable ASCII", func(t *testing.T) {
		keys := []string{
			" key",
			"key ",
			"key\n",
			"key\t",
			"key\x7f",
			"key\x01",
			"\u9375",
		}
		for _, key := range keys {
			options := validOptions(t)
			options.APIKey = mustSecret(t, key)
			value, err := New(options)
			forbidden := key
			if key == " key" || key == "key " {
				forbidden = ""
			}
			assertConstructionFailure(t, value, err, forbidden)
		}
	})

	t.Run("accepts the printable ASCII boundary characters", func(t *testing.T) {
		options := validOptions(t)
		options.APIKey = mustSecret(t, "a!~z")
		if _, err := New(options); err != nil {
			t.Errorf("New() error = %v, want nil", err)
		}
	})

	t.Run("rejects invalid model names", func(t *testing.T) {
		for _, model := range []string{"", " claude-opus-5-5", "claude-opus-5-5 ", "claude-opus-5-5\n", "\xff"} {
			options := validOptions(t)
			options.Model = model
			value, err := New(options)
			assertConstructionFailure(t, value, err, model)
		}
	})

	t.Run("rejects unset and out-of-range efforts", func(t *testing.T) {
		for _, effort := range []claudeparam.Effort{claudeparam.EffortUnset, claudeparam.Effort(99)} {
			options := validOptions(t)
			options.Effort = effort
			value, err := New(options)
			assertConstructionFailure(t, value, err, "")
		}
	})

	t.Run("rejects non-positive timeouts", func(t *testing.T) {
		for _, timeout := range []time.Duration{0, -time.Second} {
			options := validOptions(t)
			options.Timeout = timeout
			value, err := New(options)
			assertConstructionFailure(t, value, err, "")
		}
	})
}

// fatalRecorder is a testing.TB that records a Fatal message instead of
// failing the test. Fatalf deliberately does not end the calling goroutine,
// so the helper under test keeps running and the caller can observe that it
// returned no client.
type fatalRecorder struct {
	testing.TB
	message string
}

// Fatalf records the formatted message instead of failing.
func (r *fatalRecorder) Fatalf(format string, args ...any) {
	r.message = fmt.Sprintf(format, args...)
}

func TestNewForLoopbackTest(t *testing.T) {
	server, recorder := llmhttptest.NewRecordingServer(t, readFixture(t, testdataEndTurnFixture))
	options := Options{
		APIKey:  mustSecret(t, testAPIKey),
		Model:   testModel,
		Effort:  claudeparam.EffortHigh,
		Timeout: testClientTimeout,
	}
	value := NewForLoopbackTest(t, options, server.URL)
	if _, err := value.Generate(context.Background(), validRequest()); err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if got := recorder.Count(); got != 1 {
		t.Errorf("server received %d requests, want 1", got)
	}
}

func TestNewForLoopbackTestRejectsNonLoopback(t *testing.T) {
	recorder := &fatalRecorder{TB: t}
	options := Options{
		APIKey:  mustSecret(t, testAPIKey),
		Model:   testModel,
		Effort:  claudeparam.EffortHigh,
		Timeout: testClientTimeout,
	}
	value := NewForLoopbackTest(recorder, options, "https://api.anthropic.com/v1/messages")
	if recorder.message == "" {
		t.Error("NewForLoopbackTest did not record a Fatal for a non-loopback endpoint")
	}
	if value != nil {
		t.Errorf("NewForLoopbackTest returned %v, want nil", value)
	}
}

func TestNewForLoopbackTestRejectsInvalidOptions(t *testing.T) {
	recorder := &fatalRecorder{TB: t}
	options := Options{
		APIKey:  mustSecret(t, testAPIKey),
		Model:   " claude-opus-5-5",
		Effort:  claudeparam.EffortHigh,
		Timeout: testClientTimeout,
	}
	value := NewForLoopbackTest(recorder, options, "http://127.0.0.1:8080/messages")
	if recorder.message == "" {
		t.Error("NewForLoopbackTest did not record a Fatal for an invalid Options value")
	}
	if value != nil {
		t.Errorf("NewForLoopbackTest returned %v, want nil", value)
	}
}

// assertMemberSet fails unless raw has exactly the wanted member names.
func assertMemberSet(t *testing.T, raw map[string]json.RawMessage, want ...string) {
	t.Helper()
	got := make([]string, 0, len(raw))
	for key := range raw {
		got = append(got, key)
	}
	slices.Sort(got)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Errorf("member set = %v, want %v", got, want)
	}
}

func TestGenerateSendsRequest(t *testing.T) {
	server, recorder := llmhttptest.NewRecordingServer(t, readFixture(t, testdataEndTurnFixture))
	req := llm.GenerateRequest{
		SystemPrompt:    "  system <tag> & \nsecond line\n",
		UserPrompt:      "  user <tag> & \ttab\n",
		MaxOutputTokens: 0,
	}
	response, err := generateAgainst(t, server.URL, req, nil)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if response.Text == "" {
		t.Error("Generate() returned an empty Text, want the fixture content")
	}

	recorded := recorder.Only(t)
	if recorded.Method != http.MethodPost {
		t.Errorf("method = %q, want %q", recorded.Method, http.MethodPost)
	}
	if recorded.Target != "/" {
		t.Errorf("request URI = %q, want %q", recorded.Target, "/")
	}
	if got := recorded.Header.Get("Content-Type"); got != contentTypeJSON {
		t.Errorf("Content-Type = %q, want %q", got, contentTypeJSON)
	}
	if got := recorded.Header.Get(headerAPIKey); got != testAPIKey {
		t.Errorf("x-api-key = %q, want the test key", got)
	}
	if got := recorded.Header.Get(headerVersion); got != anthropicVersion {
		t.Errorf("anthropic-version = %q, want %q", got, anthropicVersion)
	}
	if got := recorded.Header.Get("anthropic-beta"); got != "" {
		t.Errorf("anthropic-beta = %q, want it unset", got)
	}
	if got := recorded.Header.Get("Authorization"); got != "" {
		t.Errorf("Authorization = %q, want it unset", got)
	}
	if got := recorded.Header.Get(headerWorkspace); got != "" {
		t.Errorf("anthropic-workspace-id = %q, want it unset", got)
	}
	if strings.Contains(string(recorded.Body), testAPIKey) || strings.Contains(recorded.Target, testAPIKey) {
		t.Error("the request body or URL contains the API key")
	}

	var body struct {
		Model    string `json:"model"`
		System   string `json:"system"`
		Messages []struct {
			Role    string          `json:"role"`
			Content json.RawMessage `json:"content"`
		} `json:"messages"`
		MaxTokens    int `json:"max_tokens"`
		OutputConfig struct {
			Effort string `json:"effort"`
		} `json:"output_config"`
	}
	if err := json.Unmarshal(recorded.Body, &body); err != nil {
		t.Fatalf("decode request body: %v", err)
	}
	if body.Model != testModel {
		t.Errorf("model = %q, want %q", body.Model, testModel)
	}
	if body.System != req.SystemPrompt {
		t.Errorf("system = %q, want the system prompt unmodified %q", body.System, req.SystemPrompt)
	}
	if len(body.Messages) != 1 {
		t.Fatalf("messages = %d, want 1", len(body.Messages))
	}
	if body.Messages[0].Role != "user" {
		t.Errorf("messages[0].role = %q, want %q", body.Messages[0].Role, "user")
	}
	var content string
	if err := json.Unmarshal(body.Messages[0].Content, &content); err != nil {
		t.Fatalf("messages[0].content is not a JSON string: %v (%s)", err, body.Messages[0].Content)
	}
	if content != req.UserPrompt {
		t.Errorf("messages[0].content = %q, want the user prompt unmodified %q", content, req.UserPrompt)
	}
	if body.MaxTokens != defaultMaxOutputTokens {
		t.Errorf("max_tokens = %d, want %d for MaxOutputTokens 0", body.MaxTokens, defaultMaxOutputTokens)
	}
	if body.OutputConfig.Effort != claudeparam.EffortHigh.String() {
		t.Errorf("output_config.effort = %q, want %q", body.OutputConfig.Effort, claudeparam.EffortHigh)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(recorded.Body, &raw); err != nil {
		t.Fatalf("decode request body members: %v", err)
	}
	assertMemberSet(t, raw, "model", "system", "messages", "max_tokens", "output_config")
	var outputConfig map[string]json.RawMessage
	if err := json.Unmarshal(raw["output_config"], &outputConfig); err != nil {
		t.Fatalf("decode output_config: %v", err)
	}
	assertMemberSet(t, outputConfig, "effort")
}

func TestGenerateMaxTokens(t *testing.T) {
	cases := []struct {
		name   string
		tokens int
		want   int
	}{
		{"zero sends the default", 0, defaultMaxOutputTokens},
		{"positive sends its value", 256, 256},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server, recorder := llmhttptest.NewRecordingServer(t, readFixture(t, testdataEndTurnFixture))
			req := validRequest()
			req.MaxOutputTokens = tc.tokens
			if _, err := generateAgainst(t, server.URL, req, nil); err != nil {
				t.Fatalf("Generate() error = %v", err)
			}
			var raw map[string]json.RawMessage
			if err := json.Unmarshal(recorder.Only(t).Body, &raw); err != nil {
				t.Fatalf("decode request body: %v", err)
			}
			var got int
			if err := json.Unmarshal(raw["max_tokens"], &got); err != nil {
				t.Fatalf("decode max_tokens: %v", err)
			}
			if got != tc.want {
				t.Errorf("max_tokens = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestGenerateEffort(t *testing.T) {
	for _, effort := range allEfforts() {
		t.Run(effort.String(), func(t *testing.T) {
			server, recorder := llmhttptest.NewRecordingServer(t, readFixture(t, testdataEndTurnFixture))
			_, err := generateAgainst(t, server.URL, validRequest(), func(options *Options) {
				options.Effort = effort
			})
			if err != nil {
				t.Fatalf("Generate() error = %v", err)
			}
			var body struct {
				OutputConfig struct {
					Effort string `json:"effort"`
				} `json:"output_config"`
			}
			if err := json.Unmarshal(recorder.Only(t).Body, &body); err != nil {
				t.Fatalf("decode request body: %v", err)
			}
			if body.OutputConfig.Effort != effort.String() {
				t.Errorf("output_config.effort = %q, want %q", body.OutputConfig.Effort, effort)
			}
		})
	}
}

func TestGenerateWorkspaceHeader(t *testing.T) {
	t.Run("a configured workspace id is sent and stays out of the body and URL", func(t *testing.T) {
		server, recorder := llmhttptest.NewRecordingServer(t, readFixture(t, testdataEndTurnFixture))
		req := validRequest()
		_, err := generateAgainst(t, server.URL, req, func(options *Options) {
			options.WorkspaceID = mustWorkspaceID(t, testWorkspaceID)
		})
		if err != nil {
			t.Fatalf("Generate() error = %v", err)
		}
		recorded := recorder.Only(t)
		if got := recorded.Header.Get(headerWorkspace); got != testWorkspaceID {
			t.Errorf("anthropic-workspace-id = %q, want %q", got, testWorkspaceID)
		}
		if strings.Contains(string(recorded.Body), testWorkspaceID) {
			t.Error("the request body contains the workspace id")
		}
		if strings.Contains(recorded.Target, testWorkspaceID) {
			t.Error("the request URL contains the workspace id")
		}
	})

	t.Run("no configured workspace id sends no header", func(t *testing.T) {
		server, recorder := llmhttptest.NewRecordingServer(t, readFixture(t, testdataEndTurnFixture))
		if _, err := generateAgainst(t, server.URL, validRequest(), nil); err != nil {
			t.Fatalf("Generate() error = %v", err)
		}
		if got := recorder.Only(t).Header.Get(headerWorkspace); got != "" {
			t.Errorf("anthropic-workspace-id = %q, want it unset", got)
		}
	})
}

func TestGenerateInvalidRequest(t *testing.T) {
	body := readFixture(t, testdataEndTurnFixture)
	invalid := []struct {
		name string
		req  llm.GenerateRequest
	}{
		{"empty SystemPrompt", llm.GenerateRequest{UserPrompt: "user"}},
		{"empty UserPrompt", llm.GenerateRequest{SystemPrompt: "system"}},
		{"invalid SystemPrompt UTF-8", llm.GenerateRequest{SystemPrompt: "sys\xff", UserPrompt: "user"}},
		{"invalid UserPrompt UTF-8", llm.GenerateRequest{SystemPrompt: "system", UserPrompt: "user\xff"}},
		{"negative MaxOutputTokens", llm.GenerateRequest{SystemPrompt: "system", UserPrompt: "user", MaxOutputTokens: -1}},
	}
	for _, tc := range invalid {
		t.Run(tc.name, func(t *testing.T) {
			server, recorder := llmhttptest.NewRecordingServer(t, body)
			response, err := generateAgainst(t, server.URL, tc.req, nil)
			if !errors.Is(err, llm.ErrInvalidRequest) {
				t.Errorf("Generate() error = %v, want ErrInvalidRequest", err)
			}
			assertRejected(t, response, err)
			if count := recorder.Count(); count != 0 {
				t.Errorf("server received %d requests, want 0", count)
			}
		})
	}

	t.Run("an invalid request wins over a done context", func(t *testing.T) {
		server, recorder := llmhttptest.NewRecordingServer(t, body)
		client := newTestClient(t, server.URL, nil)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		response, err := client.Generate(ctx, llm.GenerateRequest{})
		if !errors.Is(err, llm.ErrInvalidRequest) {
			t.Errorf("Generate() error = %v, want ErrInvalidRequest", err)
		}
		assertRejected(t, response, err)
		if count := recorder.Count(); count != 0 {
			t.Errorf("server received %d requests, want 0", count)
		}
	})

	t.Run("a done context sends nothing", func(t *testing.T) {
		server, recorder := llmhttptest.NewRecordingServer(t, body)
		client := newTestClient(t, server.URL, nil)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		response, err := client.Generate(ctx, validRequest())
		if !errors.Is(err, context.Canceled) {
			t.Errorf("Generate() error = %v, want context.Canceled", err)
		}
		assertRejected(t, response, err)
		if count := recorder.Count(); count != 0 {
			t.Errorf("server received %d requests, want 0", count)
		}
	})
}

func TestGenerateNoRedirect(t *testing.T) {
	var mu sync.Mutex
	hits := map[string]int{}
	mux := http.NewServeMux()
	mux.HandleFunc("/primary", func(w http.ResponseWriter, req *http.Request) {
		mu.Lock()
		hits["/primary"]++
		mu.Unlock()
		http.Redirect(w, req, "/secondary", http.StatusTemporaryRedirect)
	})
	mux.HandleFunc("/secondary", func(w http.ResponseWriter, _ *http.Request) {
		mu.Lock()
		hits["/secondary"]++
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	response, err := generateAgainst(t, server.URL+"/primary", validRequest(), nil)
	if !errors.Is(err, ErrHTTPStatus) {
		t.Errorf("Generate() error = %v, want ErrHTTPStatus", err)
	}
	statusErr, ok := errors.AsType[*HTTPStatusError](err)
	if !ok {
		t.Fatalf("Generate() error = %v, want *HTTPStatusError", err)
	}
	if statusErr.StatusCode != http.StatusTemporaryRedirect {
		t.Errorf("StatusCode = %d, want %d", statusErr.StatusCode, http.StatusTemporaryRedirect)
	}
	assertRejected(t, response, err)
	mu.Lock()
	defer mu.Unlock()
	if hits["/secondary"] != 0 {
		t.Errorf("redirect target was requested %d times, want 0", hits["/secondary"])
	}
}

func TestGenerateHTTPStatus(t *testing.T) {
	statuses := []int{
		http.StatusBadRequest,
		http.StatusUnauthorized,
		http.StatusPaymentRequired,
		http.StatusForbidden,
		http.StatusNotFound,
		http.StatusRequestEntityTooLarge,
		http.StatusTooManyRequests,
		http.StatusInternalServerError,
		statusOverloaded,
		http.StatusTemporaryRedirect,
	}
	for _, status := range statuses {
		t.Run(strconv.Itoa(status), func(t *testing.T) {
			const marker = "RESPONSE-BODY-MARKER"
			body := []byte("{ not json " + marker)
			server := llmhttptest.NewResponseServer(t, status, body)
			req := validRequest()
			req.MaxOutputTokens = 256
			response, err := generateAgainst(t, server.URL, req, nil)
			if !errors.Is(err, ErrHTTPStatus) {
				t.Errorf("Generate() error = %v, want ErrHTTPStatus", err)
			}
			if errors.Is(err, ErrInvalidResponse) {
				t.Errorf("Generate() error = %v, must not match ErrInvalidResponse", err)
			}
			statusErr, ok := errors.AsType[*HTTPStatusError](err)
			if !ok {
				t.Fatalf("Generate() error = %v, want *HTTPStatusError", err)
			}
			if statusErr.StatusCode != status {
				t.Errorf("StatusCode = %d, want %d", statusErr.StatusCode, status)
			}
			if strings.Contains(err.Error(), marker) {
				t.Errorf("error %q contains the response body", err)
			}
			if !strings.Contains(err.Error(), testModel) {
				t.Errorf("error %q does not name the model", err)
			}
			if !strings.Contains(err.Error(), "max_tokens 256") {
				t.Errorf("error %q does not name the sent max_tokens", err)
			}
			if !strings.Contains(err.Error(), claudeparam.EffortHigh.String()) {
				t.Errorf("error %q does not name the effort", err)
			}
			assertRejected(t, response, err)
		})
	}
}

func TestHTTPStatusErrorGuidance(t *testing.T) {
	// A configuration error must read differently from an account or
	// server-side problem.
	guidance := map[int]string{}
	for _, status := range []int{http.StatusBadRequest, http.StatusUnauthorized, http.StatusNotFound, http.StatusTooManyRequests, statusOverloaded} {
		guidance[status] = statusGuidance(status)
	}
	seen := map[string]int{}
	for status, text := range guidance {
		if text == "" {
			t.Errorf("statusGuidance(%d) is empty", status)
		}
		if other, ok := seen[text]; ok {
			t.Errorf("statusGuidance(%d) and statusGuidance(%d) share the guidance %q", status, other, text)
		}
		seen[text] = status
	}
}

func TestGenerateTimeout(t *testing.T) {
	const shortTimeout = 100 * time.Millisecond
	cases := []struct {
		name  string
		start func(t *testing.T) string
	}{
		{"no response headers", func(t *testing.T) string {
			return llmhttptest.NewStallingServer(t).URL
		}},
		{"partial response body", func(t *testing.T) string {
			return llmhttptest.NewPartialStallingServer(t).URL
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			start := time.Now()
			response, err := generateAgainst(t, tc.start(t), validRequest(), func(options *Options) {
				options.Timeout = shortTimeout
			})
			elapsed := time.Since(start)
			if !errors.Is(err, context.DeadlineExceeded) {
				t.Errorf("Generate() error = %v, want context.DeadlineExceeded", err)
			}
			assertRejected(t, response, err)
			if elapsed > shortTimeout+testTimeoutGrace {
				t.Errorf("Generate() took %v, want at most %v", elapsed, shortTimeout+testTimeoutGrace)
			}
			if !strings.Contains(err.Error(), shortTimeout.String()) {
				t.Errorf("error %q does not name the adapter timeout %v", err, shortTimeout)
			}
		})
	}

	t.Run("caller deadline comes first", func(t *testing.T) {
		server := llmhttptest.NewStallingServer(t)
		client := newTestClient(t, server.URL, nil)
		ctx, cancel := context.WithTimeout(context.Background(), shortTimeout)
		defer cancel()
		start := time.Now()
		response, err := client.Generate(ctx, validRequest())
		elapsed := time.Since(start)
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("Generate() error = %v, want context.DeadlineExceeded", err)
		}
		assertRejected(t, response, err)
		if elapsed > shortTimeout+testTimeoutGrace {
			t.Errorf("Generate() took %v, want at most %v", elapsed, shortTimeout+testTimeoutGrace)
		}
		if strings.Contains(err.Error(), testClientTimeout.String()) {
			t.Errorf("error %q claims the adapter timeout although the caller deadline came first", err)
		}
		if !strings.Contains(err.Error(), "caller") {
			t.Errorf("error %q does not name the deadline source", err)
		}
	})
}

func TestGenerateCanceled(t *testing.T) {
	t.Run("cancel while running", func(t *testing.T) {
		server, entered := llmhttptest.NewGateServer(t)
		client := newTestClient(t, server.URL, nil)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		var (
			response llm.GenerateResponse
			err      error
			done     = make(chan struct{})
		)
		go func() {
			response, err = client.Generate(ctx, validRequest())
			close(done)
		}()
		<-entered
		cancel()
		<-done

		if !errors.Is(err, context.Canceled) {
			t.Errorf("Generate() error = %v, want context.Canceled", err)
		}
		assertRejected(t, response, err)
		if !strings.Contains(err.Error(), "caller") {
			t.Errorf("error %q does not name the cancellation source", err)
		}
	})

	t.Run("pre-canceled context sends nothing", func(t *testing.T) {
		server, recorder := llmhttptest.NewRecordingServer(t, readFixture(t, testdataEndTurnFixture))
		client := newTestClient(t, server.URL, nil)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		response, err := client.Generate(ctx, validRequest())
		if !errors.Is(err, context.Canceled) {
			t.Errorf("Generate() error = %v, want context.Canceled", err)
		}
		assertRejected(t, response, err)
		if count := recorder.Count(); count != 0 {
			t.Errorf("server received %d requests, want 0", count)
		}
	})
}

// checkTransportFailure checks the contract of a transport failure: the
// ErrTransport sentinel only, no context error, no invalid response, the zero
// response, and no chained lower error.
func checkTransportFailure(t *testing.T, response llm.GenerateResponse, err error) {
	t.Helper()
	if !errors.Is(err, ErrTransport) {
		t.Errorf("Generate() error = %v, want ErrTransport", err)
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		t.Errorf("Generate() error = %v, must not match a context error", err)
	}
	if errors.Is(err, ErrInvalidResponse) {
		t.Errorf("Generate() error = %v, must not match ErrInvalidResponse", err)
	}
	if _, ok := errors.AsType[*url.Error](err); ok {
		t.Error("ErrTransport chains the lower *url.Error")
	}
	if errors.Is(err, io.ErrUnexpectedEOF) {
		t.Error("ErrTransport chains the lower io.ErrUnexpectedEOF")
	}
	assertRejected(t, response, err)
}

func TestGenerateTransportFailure(t *testing.T) {
	t.Run("listener closes the connection", func(t *testing.T) {
		response, err := generateAgainst(t, llmhttptest.NewClosedListener(t), validRequest(), nil)
		checkTransportFailure(t, response, err)
	})

	t.Run("body truncated before Content-Length", func(t *testing.T) {
		response, err := generateAgainst(t, llmhttptest.NewTruncatedResponseServer(t), validRequest(), nil)
		checkTransportFailure(t, response, err)
	})
}

func TestClientOutputDoesNotLeakAPIKey(t *testing.T) {
	value := llm.LLMClient(newTestClient(t, "http://127.0.0.1:1", nil))
	outputs := map[string]string{
		"fmt %v":  fmt.Sprintf("%v", value),
		"fmt %+v": fmt.Sprintf("%+v", value),
		"fmt %#v": fmt.Sprintf("%#v", value),
	}
	var textBuffer bytes.Buffer
	slog.New(slog.NewTextHandler(&textBuffer, nil)).Info("client", "value", value)
	outputs["slog text"] = textBuffer.String()
	var jsonBuffer bytes.Buffer
	slog.New(slog.NewJSONHandler(&jsonBuffer, nil)).Info("client", "value", value)
	outputs["slog json"] = jsonBuffer.String()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("json.Marshal(client) error = %v", err)
	}
	outputs["encoding/json"] = string(encoded)

	for name, output := range outputs {
		if strings.Contains(output, testAPIKey) {
			t.Errorf("%s output contains the API key: %q", name, output)
		}
	}
}

func TestRevealOnlyInRequestFile(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read package directory: %v", err)
	}
	fileSet := token.NewFileSet()
	positions := map[string]int{}
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fileSet, name, nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		ast.Inspect(file, func(node ast.Node) bool {
			selector, ok := node.(*ast.SelectorExpr)
			if ok && selector.Sel.Name == "Reveal" {
				positions[name]++
			}
			return true
		})
	}
	if positions["request.go"] != 2 {
		t.Errorf("request.go calls Reveal %d times, want 2: %v", positions["request.go"], positions)
	}
	for name, count := range positions {
		if name != "request.go" {
			t.Errorf("%s references Reveal %d times; it must stay in request.go", name, count)
		}
	}
}

func TestUnitTestsCannotReachProductionEndpoint(t *testing.T) {
	productionURL := &url.URL{Scheme: "https", Host: "api.anthropic.com", Path: "/v1/messages"}
	proxyURL, err := http.ProxyFromEnvironment(&http.Request{URL: productionURL})
	if err != nil {
		t.Fatalf("ProxyFromEnvironment error = %v", err)
	}
	if proxyURL == nil || proxyURL.String() != testProxy.URL() {
		t.Fatalf("proxy for %s = %v, want %s", productionURL, proxyURL, testProxy.URL())
	}

	value, err := New(Options{
		APIKey:  mustSecret(t, testAPIKey),
		Model:   testModel,
		Effort:  claudeparam.EffortHigh,
		Timeout: testClientTimeout,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	client, ok := value.(*client)
	if !ok {
		t.Fatalf("New() returned %T, want *client", value)
	}
	if client.httpClient.Transport != nil {
		t.Fatal("client uses a custom Transport; refusing to send to the production endpoint")
	}

	before := testProxy.Accepted()
	response, err := client.Generate(context.Background(), validRequest())
	if !errors.Is(err, ErrTransport) {
		t.Errorf("Generate() error = %v, want ErrTransport", err)
	}
	assertRejected(t, response, err)
	if testProxy.Accepted() <= before {
		t.Error("the blackhole listener accepted no connection; the request did not use the test proxy")
	}
}

func TestGenerateSentinelsDistinct(t *testing.T) {
	endTurn := readFixture(t, testdataEndTurnFixture)
	maxTokens := readFixture(t, testdataMaxTokensFixture)
	cases := []struct {
		name string
		want error
		run  func(t *testing.T) (llm.GenerateResponse, error)
	}{
		{"invalid request", llm.ErrInvalidRequest, func(t *testing.T) (llm.GenerateResponse, error) {
			server, _ := llmhttptest.NewRecordingServer(t, endTurn)
			req := validRequest()
			req.UserPrompt = ""
			return generateAgainst(t, server.URL, req, nil)
		}},
		{"http status", ErrHTTPStatus, func(t *testing.T) (llm.GenerateResponse, error) {
			server := llmhttptest.NewResponseServer(t, http.StatusInternalServerError, []byte("boom"))
			return generateAgainst(t, server.URL, validRequest(), nil)
		}},
		{"invalid response", ErrInvalidResponse, func(t *testing.T) (llm.GenerateResponse, error) {
			server := llmhttptest.NewResponseServer(t, http.StatusOK, []byte("not json"))
			return generateAgainst(t, server.URL, validRequest(), nil)
		}},
		{"truncated", llm.ErrTruncated, func(t *testing.T) (llm.GenerateResponse, error) {
			server := llmhttptest.NewResponseServer(t, http.StatusOK, maxTokens)
			return generateAgainst(t, server.URL, validRequest(), nil)
		}},
		{"unexpected finish reason", llm.ErrUnexpectedFinishReason, func(t *testing.T) (llm.GenerateResponse, error) {
			body := replaceOnce(t, endTurn, `"stop_reason":"end_turn"`, `"stop_reason":"refusal"`)
			server := llmhttptest.NewResponseServer(t, http.StatusOK, body)
			return generateAgainst(t, server.URL, validRequest(), nil)
		}},
		{"empty response", llm.ErrEmptyResponse, func(t *testing.T) (llm.GenerateResponse, error) {
			document := documentOf(t, endTurn)
			document[keyContent] = []any{blockOfType(t, document, blockThinking)}
			server := llmhttptest.NewResponseServer(t, http.StatusOK, encodeDocument(t, document))
			return generateAgainst(t, server.URL, validRequest(), nil)
		}},
		{"transport", ErrTransport, func(t *testing.T) (llm.GenerateResponse, error) {
			return generateAgainst(t, llmhttptest.NewClosedListener(t), validRequest(), nil)
		}},
		{"deadline", context.DeadlineExceeded, func(t *testing.T) (llm.GenerateResponse, error) {
			server := llmhttptest.NewStallingServer(t)
			return generateAgainst(t, server.URL, validRequest(), func(options *Options) {
				options.Timeout = 100 * time.Millisecond
			})
		}},
		{"canceled", context.Canceled, func(t *testing.T) (llm.GenerateResponse, error) {
			server, _ := llmhttptest.NewRecordingServer(t, endTurn)
			client := newTestClient(t, server.URL, nil)
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			return client.Generate(ctx, validRequest())
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			response, err := tc.run(t)
			if !errors.Is(err, tc.want) {
				t.Errorf("error = %v, want %v", err, tc.want)
			}
			assertRejected(t, response, err)
		})
	}
}
