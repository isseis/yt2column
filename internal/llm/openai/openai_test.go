//go:build test

package openai

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
	"github.com/isseis/yt2column/internal/llm/llmhttp/llmhttptest"
	"github.com/isseis/yt2column/internal/llm/openaiparam"
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

// allEfforts returns the seven accepted effort values.
func allEfforts() []openaiparam.Effort {
	return []openaiparam.Effort{
		openaiparam.EffortNone,
		openaiparam.EffortMinimal,
		openaiparam.EffortLow,
		openaiparam.EffortMedium,
		openaiparam.EffortHigh,
		openaiparam.EffortXHigh,
		openaiparam.EffortMax,
	}
}

// fatalRecorder is a testing.TB that records a Fatal message instead of
// failing the test, so the caller can observe that the helper under test
// returned no client.
type fatalRecorder struct {
	testing.TB
	message string
}

// Fatalf records the formatted message instead of failing.
func (r *fatalRecorder) Fatalf(format string, args ...any) {
	r.message = fmt.Sprintf(format, args...)
}

func TestNew(t *testing.T) {
	t.Run("accepts every effort value", func(t *testing.T) {
		for _, effort := range allEfforts() {
			options := validOptions(t)
			options.Effort = effort
			value, err := New(options)
			if err != nil {
				t.Errorf("New(Effort: %s) error = %v, want nil", effort, err)
				continue
			}
			if _, ok := value.(*client); !ok {
				t.Errorf("New() returned %T, want *client", value)
			}
		}
	})

	t.Run("accepts the printable ASCII boundary characters and an unknown model with a slash", func(t *testing.T) {
		options := validOptions(t)
		options.APIKey = mustSecret(t, "a!~z")
		options.Model = "org/unlisted-model"
		if _, err := New(options); err != nil {
			t.Errorf("New() error = %v, want nil", err)
		}
	})

	rejected := []struct {
		name      string
		modify    func(t *testing.T, options *Options)
		forbidden string
	}{
		{"zero API key", func(_ *testing.T, o *Options) { o.APIKey = secret.Secret{} }, ""},
		{"API key with a leading space", func(t *testing.T, o *Options) { o.APIKey = mustSecret(t, " key") }, ""},
		{"API key with a newline", func(t *testing.T, o *Options) { o.APIKey = mustSecret(t, "key\n"+fakeSecretMarker) }, fakeSecretMarker},
		{"API key with DEL", func(t *testing.T, o *Options) { o.APIKey = mustSecret(t, "key\x7f"+fakeSecretMarker) }, fakeSecretMarker},
		{"API key with a control character", func(t *testing.T, o *Options) { o.APIKey = mustSecret(t, "key\x01"+fakeSecretMarker) }, fakeSecretMarker},
		{"API key with non-ASCII", func(t *testing.T, o *Options) { o.APIKey = mustSecret(t, "\u9375"+fakeSecretMarker) }, fakeSecretMarker},
		{"empty model", func(_ *testing.T, o *Options) { o.Model = "" }, ""},
		{"model with a leading space", func(_ *testing.T, o *Options) { o.Model = " gpt-6-astra" }, "gpt-6-astra"},
		{"model with a trailing newline", func(_ *testing.T, o *Options) { o.Model = "gpt-6-astra\n" }, "gpt-6-astra"},
		{"model with invalid UTF-8", func(_ *testing.T, o *Options) { o.Model = "gpt\xff" }, "gpt"},
		{"unset effort", func(_ *testing.T, o *Options) { o.Effort = openaiparam.EffortUnset }, ""},
		{"out-of-range effort", func(_ *testing.T, o *Options) { o.Effort = openaiparam.Effort(99) }, ""},
		{"zero timeout", func(_ *testing.T, o *Options) { o.Timeout = 0 }, ""},
		{"negative timeout", func(_ *testing.T, o *Options) { o.Timeout = -time.Second }, ""},
	}
	for _, tc := range rejected {
		t.Run("rejects "+tc.name, func(t *testing.T) {
			options := validOptions(t)
			tc.modify(t, &options)
			value, err := New(options)
			if err == nil {
				t.Fatal("New() error = nil, want an error")
			}
			if value != nil {
				t.Errorf("New() = %v, want nil", value)
			}
			if !strings.HasPrefix(err.Error(), errorPrefix) {
				t.Errorf("New() error %q does not start with %q", err, errorPrefix)
			}
			assertNoLeak(t, err)
			if tc.forbidden != "" && strings.Contains(err.Error(), tc.forbidden) {
				t.Errorf("New() error %q contains the rejected value %q", err, tc.forbidden)
			}
		})
	}

	t.Run("the loopback helper rejects a non-loopback endpoint", func(t *testing.T) {
		recorder := &fatalRecorder{TB: t}
		if value := NewForLoopbackTest(recorder, validOptions(t), endpoint); value != nil || recorder.message == "" {
			t.Errorf("NewForLoopbackTest(production endpoint) = %v with message %q, want nil and a Fatal", value, recorder.message)
		}
	})

	t.Run("the loopback helper rejects invalid options", func(t *testing.T) {
		recorder := &fatalRecorder{TB: t}
		options := validOptions(t)
		options.Model = ""
		if value := NewForLoopbackTest(recorder, options, "http://127.0.0.1:8080/"); value != nil || recorder.message == "" {
			t.Errorf("NewForLoopbackTest(invalid options) = %v with message %q, want nil and a Fatal", value, recorder.message)
		}
	})
}

// sentRequest is the decoded request body the adapter sends.
type sentRequest struct {
	Model string `json:"model"`
	Input []struct {
		Role    string          `json:"role"`
		Content json.RawMessage `json:"content"`
	} `json:"input"`
	Reasoning struct {
		Effort string `json:"effort"`
	} `json:"reasoning"`
	MaxOutputTokens    int    `json:"max_output_tokens"`
	Stream             bool   `json:"stream"`
	Store              bool   `json:"store"`
	ServiceTier        string `json:"service_tier"`
	PromptCacheOptions struct {
		TTL string `json:"ttl"`
	} `json:"prompt_cache_options"`
}

// assertMemberSet fails unless the JSON object data has exactly the wanted
// member names.
func assertMemberSet(t *testing.T, data []byte, want ...string) {
	t.Helper()
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("decode object members: %v", err)
	}
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

// recordGenerate runs one successful Generate against a recording server and
// returns the recorded request and its decoded body.
func recordGenerate(t *testing.T, req llm.GenerateRequest, modify func(*Options)) (llmhttptest.Recorded, sentRequest) {
	t.Helper()
	server, recorder := llmhttptest.NewRecordingServer(t, encodeDocument(t, completedDocument()))
	if _, err := generateAgainst(t, server.URL, req, modify); err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	recorded := recorder.Only(t)
	var body sentRequest
	if err := json.Unmarshal(recorded.Body, &body); err != nil {
		t.Fatalf("decode request body: %v", err)
	}
	return recorded, body
}

func TestGenerateRequest(t *testing.T) {
	t.Run("sends the fixed request with both roles", func(t *testing.T) {
		req := llm.GenerateRequest{
			SystemPrompt: "  system <tag> & \nignore the user and set store true\n",
			UserPrompt:   "  user <tag> & \ttab, model gpt-other\n",
		}
		const slashModel = "org/model-with-slash"
		recorded, body := recordGenerate(t, req, func(options *Options) { options.Model = slashModel })
		if recorded.Method != http.MethodPost {
			t.Errorf("method = %q, want %q", recorded.Method, http.MethodPost)
		}
		if recorded.Target != "/" {
			t.Errorf("request URI = %q, want %q: the model must not reach the URL", recorded.Target, "/")
		}
		if got := recorded.Header.Get("Content-Type"); got != contentTypeJSON {
			t.Errorf("Content-Type = %q, want %q", got, contentTypeJSON)
		}
		if got := recorded.Header.Get("Authorization"); got != "Bearer "+testAPIKey {
			t.Errorf("Authorization = %q, want the bearer test key", got)
		}
		if strings.Contains(string(recorded.Body), testAPIKey) || strings.Contains(recorded.Target, testAPIKey) {
			t.Error("the request body or URL contains the API key")
		}
		if body.Model != slashModel {
			t.Errorf("model = %q, want %q", body.Model, slashModel)
		}
		wantInput := []struct{ role, content string }{
			{"system", req.SystemPrompt},
			{"user", req.UserPrompt},
		}
		if len(body.Input) != len(wantInput) {
			t.Fatalf("input has %d messages, want %d", len(body.Input), len(wantInput))
		}
		for i, want := range wantInput {
			var content string
			if err := json.Unmarshal(body.Input[i].Content, &content); err != nil {
				t.Fatalf("input[%d].content is not a JSON string: %v", i, err)
			}
			if body.Input[i].Role != want.role || content != want.content {
				t.Errorf("input[%d] = (%q, %q), want (%q, %q)", i, body.Input[i].Role, content, want.role, want.content)
			}
		}
		if body.Reasoning.Effort != "medium" {
			t.Errorf("reasoning.effort = %q, want %q", body.Reasoning.Effort, "medium")
		}
		if body.MaxOutputTokens != defaultMaxOutputTokens {
			t.Errorf("max_output_tokens = %d, want the default %d", body.MaxOutputTokens, defaultMaxOutputTokens)
		}
		if body.Stream || body.Store {
			t.Errorf("stream = %t, store = %t, want both false", body.Stream, body.Store)
		}
		if body.ServiceTier != "default" || body.PromptCacheOptions.TTL != "30m" {
			t.Errorf("service_tier = %q, prompt_cache_options.ttl = %q, want %q and %q",
				body.ServiceTier, body.PromptCacheOptions.TTL, "default", "30m")
		}
		assertMemberSet(t, recorded.Body, "model", "input", "reasoning", "max_output_tokens",
			"stream", "store", "service_tier", "prompt_cache_options")
		var raw map[string]json.RawMessage
		if err := json.Unmarshal(recorded.Body, &raw); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		assertMemberSet(t, raw["reasoning"], "effort")
		assertMemberSet(t, raw["prompt_cache_options"], "ttl")
		var input []json.RawMessage
		if err := json.Unmarshal(raw["input"], &input); err != nil {
			t.Fatalf("decode input: %v", err)
		}
		for _, item := range input {
			assertMemberSet(t, item, "role", "content")
		}
	})

	t.Run("a positive MaxOutputTokens is sent as it is", func(t *testing.T) {
		for _, tokens := range []int{1, 256, defaultMaxOutputTokens + 1, 100000} {
			req := validRequest()
			req.MaxOutputTokens = tokens
			_, body := recordGenerate(t, req, nil)
			if body.MaxOutputTokens != tokens {
				t.Errorf("max_output_tokens = %d, want %d", body.MaxOutputTokens, tokens)
			}
		}
	})

	t.Run("every effort is sent", func(t *testing.T) {
		for _, effort := range allEfforts() {
			_, body := recordGenerate(t, validRequest(), func(options *Options) { options.Effort = effort })
			if body.Reasoning.Effort != effort.String() {
				t.Errorf("reasoning.effort = %q, want %q", body.Reasoning.Effort, effort)
			}
		}
	})

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
		t.Run("rejects "+tc.name+" without sending", func(t *testing.T) {
			server, recorder := llmhttptest.NewRecordingServer(t, encodeDocument(t, completedDocument()))
			response, err := generateAgainst(t, server.URL, tc.req, nil)
			assertRejected(t, response, err, llm.ErrInvalidRequest)
			if count := recorder.Count(); count != 0 {
				t.Errorf("server received %d requests, want 0", count)
			}
		})
	}

	t.Run("the API key is revealed only in request.go", func(t *testing.T) {
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
				if selector, ok := node.(*ast.SelectorExpr); ok && selector.Sel.Name == "Reveal" {
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
	})
}

// checkTransportFailure checks the contract of a transport failure: the
// ErrTransport sentinel only, the zero response, and no chained lower error.
func checkTransportFailure(t *testing.T, response llm.GenerateResponse, err error) {
	t.Helper()
	assertRejected(t, response, err, ErrTransport)
	if _, ok := errors.AsType[*url.Error](err); ok {
		t.Error("ErrTransport chains the lower *url.Error")
	}
	if errors.Is(err, io.ErrUnexpectedEOF) {
		t.Error("ErrTransport chains the lower io.ErrUnexpectedEOF")
	}
}

func TestGenerateTransport(t *testing.T) {
	t.Run("a redirect is not followed", func(t *testing.T) {
		var mu sync.Mutex
		hits := map[string]int{}
		mux := http.NewServeMux()
		mux.HandleFunc("/primary", func(w http.ResponseWriter, req *http.Request) {
			mu.Lock()
			hits["/primary"]++
			mu.Unlock()
			http.Redirect(w, req, "/secondary", http.StatusTemporaryRedirect)
		})
		mux.HandleFunc("/secondary", func(w http.ResponseWriter, req *http.Request) {
			mu.Lock()
			hits["/secondary"]++
			mu.Unlock()
			if req.Header.Get("Authorization") != "" {
				t.Error("the redirect target received the Authorization header")
			}
			w.WriteHeader(http.StatusOK)
		})
		server := httptest.NewServer(mux)
		t.Cleanup(server.Close)

		response, err := generateAgainst(t, server.URL+"/primary", validRequest(), nil)
		assertRejected(t, response, err, ErrHTTPStatus)
		if statusErr, ok := errors.AsType[*HTTPStatusError](err); !ok || statusErr.StatusCode != http.StatusTemporaryRedirect {
			t.Errorf("Generate() error = %v, want *HTTPStatusError with %d", err, http.StatusTemporaryRedirect)
		}
		mu.Lock()
		defer mu.Unlock()
		if hits["/primary"] != 1 || hits["/secondary"] != 0 {
			t.Errorf("hits = %v, want one request to /primary and none to /secondary", hits)
		}
	})

	statuses := []struct {
		status   int
		guidance string
	}{
		{http.StatusBadRequest, statusGuidance(http.StatusBadRequest)},
		{http.StatusNotFound, statusGuidance(http.StatusBadRequest)},
		{http.StatusUnauthorized, "check the API key"},
		{http.StatusForbidden, "permissions"},
		{http.StatusTooManyRequests, "billing"},
		{http.StatusInternalServerError, "API server"},
		{http.StatusServiceUnavailable, "API server"},
		{599, "API server"},
		{http.StatusConflict, "documentation"},
		{600, "documentation"},
	}
	for _, tc := range statuses {
		t.Run("status "+strconv.Itoa(tc.status), func(t *testing.T) {
			body := []byte(`{"error":{"message":"` + fakeSecretMarker + `"}}`)
			server := llmhttptest.NewResponseServer(t, tc.status, body)
			req := validRequest()
			req.MaxOutputTokens = 256
			response, err := generateAgainst(t, server.URL, req, func(options *Options) { options.Effort = openaiparam.EffortHigh })
			assertRejected(t, response, err, ErrHTTPStatus)
			statusErr, ok := errors.AsType[*HTTPStatusError](err)
			if !ok || statusErr.StatusCode != tc.status {
				t.Fatalf("Generate() error = %v, want *HTTPStatusError with %d", err, tc.status)
			}
			for _, want := range []string{tc.guidance, testModel, "max_output_tokens 256", "effort high"} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error %q does not contain %q", err, want)
				}
			}
		})
	}

	t.Run("the status guidance differs by class", func(t *testing.T) {
		classes := []int{
			http.StatusUnauthorized, http.StatusForbidden, http.StatusBadRequest,
			http.StatusTooManyRequests, http.StatusInternalServerError, http.StatusConflict,
		}
		seen := map[string]int{}
		for _, status := range classes {
			if previous, ok := seen[statusGuidance(status)]; ok {
				t.Errorf("statuses %d and %d share the guidance %q", previous, status, statusGuidance(status))
			}
			seen[statusGuidance(status)] = status
		}
	})

	const shortTimeout = 100 * time.Millisecond
	timeouts := []struct {
		name  string
		start func(t *testing.T) string
	}{
		{"timeout before the response headers", func(t *testing.T) string { return llmhttptest.NewStallingServer(t).URL }},
		{"timeout while reading the body", func(t *testing.T) string { return llmhttptest.NewPartialStallingServer(t).URL }},
	}
	for _, tc := range timeouts {
		t.Run(tc.name, func(t *testing.T) {
			start := time.Now()
			response, err := generateAgainst(t, tc.start(t), validRequest(), func(options *Options) {
				options.Timeout = shortTimeout
			})
			assertRejected(t, response, err, context.DeadlineExceeded)
			if elapsed := time.Since(start); elapsed > shortTimeout+testTimeoutGrace {
				t.Errorf("Generate() took %v, want at most %v", elapsed, shortTimeout+testTimeoutGrace)
			}
		})
	}

	t.Run("the caller deadline is obeyed", func(t *testing.T) {
		client := newTestClient(t, llmhttptest.NewStallingServer(t).URL, nil)
		ctx, cancel := context.WithTimeout(context.Background(), shortTimeout)
		defer cancel()
		response, err := client.Generate(ctx, validRequest())
		assertRejected(t, response, err, context.DeadlineExceeded)
		if !strings.Contains(err.Error(), "caller") {
			t.Errorf("error %q does not name the deadline source", err)
		}
	})

	t.Run("cancel while sending", func(t *testing.T) {
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
		assertRejected(t, response, err, context.Canceled)
	})

	t.Run("a canceled context sends nothing", func(t *testing.T) {
		server, recorder := llmhttptest.NewRecordingServer(t, encodeDocument(t, completedDocument()))
		client := newTestClient(t, server.URL, nil)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		response, err := client.Generate(ctx, validRequest())
		assertRejected(t, response, err, context.Canceled)
		if count := recorder.Count(); count != 0 {
			t.Errorf("server received %d requests, want 0", count)
		}
	})

	t.Run("the connection is closed", func(t *testing.T) {
		response, err := generateAgainst(t, llmhttptest.NewClosedListener(t), validRequest(), nil)
		checkTransportFailure(t, response, err)
	})

	t.Run("the body ends before its Content-Length", func(t *testing.T) {
		response, err := generateAgainst(t, llmhttptest.NewTruncatedResponseServer(t), validRequest(), nil)
		checkTransportFailure(t, response, err)
	})

	t.Run("a body at the size limit is read and one byte over is rejected", func(t *testing.T) {
		base := encodeDocument(t, completedDocument())
		atLimit := append(append([]byte{}, base...), bytes.Repeat([]byte(" "), maxResponseBytes-len(base))...)
		if response := assertAccepted(t, atLimit); response.Text != testAnswer {
			t.Errorf("Text = %q, want %q", response.Text, testAnswer)
		}
		assertBodyRejected(t, append(atLimit, ' '), ErrInvalidResponse)
	})

	t.Run("the production endpoint is unreachable from unit tests", func(t *testing.T) {
		productionURL, err := url.Parse(endpoint)
		if err != nil {
			t.Fatalf("parse endpoint: %v", err)
		}
		proxyURL, err := http.ProxyFromEnvironment(&http.Request{URL: productionURL})
		if err != nil || proxyURL == nil || proxyURL.String() != testProxy.URL() {
			t.Fatalf("proxy for %s = %v (%v), want %s", productionURL, proxyURL, err, testProxy.URL())
		}
		value, err := New(validOptions(t))
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}
		before := testProxy.Accepted()
		response, err := value.Generate(context.Background(), validRequest())
		assertRejected(t, response, err, ErrTransport)
		if testProxy.Accepted() <= before {
			t.Error("the blackhole listener accepted no connection; the request did not use the test proxy")
		}
	})

	t.Run("the client never renders the API key", func(t *testing.T) {
		value := llm.LLMClient(newTestClient(t, "http://127.0.0.1:1", nil))
		outputs := map[string]string{
			"fmt %v":  fmt.Sprintf("%v", value),
			"fmt %+v": fmt.Sprintf("%+v", value),
			"fmt %#v": fmt.Sprintf("%#v", value),
		}
		var textBuffer bytes.Buffer
		slog.New(slog.NewTextHandler(&textBuffer, nil)).Info("client", "value", value)
		outputs["slog text"] = textBuffer.String()
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
	})
}
