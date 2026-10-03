//go:build test

package deepseek

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
	"io/fs"
	"log/slog"
	"maps"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/isseis/yt2column/internal/llm"
	"github.com/isseis/yt2column/internal/secret"
)

// TestMain routes every non-loopback request through a blackhole listener via
// the proxy environment variables, so a unit test that accidentally targets
// the production endpoint fails without reaching the network.
func TestMain(m *testing.M) {
	proxy, err := startBlackholeProxy()
	if err != nil {
		fmt.Fprintf(os.Stderr, "start test proxy: %v\n", err)
		os.Exit(1)
	}
	testProxy = proxy
	for _, name := range []string{"HTTPS_PROXY", "https_proxy", "HTTP_PROXY", "http_proxy"} {
		if err := os.Setenv(name, proxy.url()); err != nil {
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
	proxy.stop()
	os.Exit(code)
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
		return Options{APIKey: mustSecret(t, testAPIKey), Model: testModel, Timeout: time.Second}
	}

	t.Run("accepts valid options", func(t *testing.T) {
		value, err := New(validOptions(t))
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}
		if _, ok := value.(*client); !ok {
			t.Fatalf("New() returned %T, want *client", value)
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
			// " key" and "key " also occur inside the fixed message prose
			// ("the API key ..."), so skip the verbatim check only for
			// those two; the newline and tab forms must not appear at all.
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
		models := []string{
			"",
			" deepseek-flash",
			"deepseek-flash ",
			"deepseek-flash\n",
			"\xff",
		}
		for _, model := range models {
			options := validOptions(t)
			options.Model = model
			value, err := New(options)
			assertConstructionFailure(t, value, err, model)
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

func TestNewTestClientRejectsNonLoopback(t *testing.T) {
	accepted := []string{
		"http://127.0.0.1:8080/chat",
		"http://[::1]:8080/chat",
	}
	for _, endpoint := range accepted {
		if err := validateLoopbackEndpoint(endpoint); err != nil {
			t.Errorf("validateLoopbackEndpoint(%q) error = %v, want nil", endpoint, err)
		}
	}
	rejected := []string{
		"https://api.deepseek.com/chat/completions",
		"http://192.168.1.10:8080/chat",
		"http://localhost:8080/chat",
		"http://[2001:db8::1]:8080/chat",
		"127.0.0.1:8080/chat",
	}
	for _, endpoint := range rejected {
		if err := validateLoopbackEndpoint(endpoint); err == nil {
			t.Errorf("validateLoopbackEndpoint(%q) error = nil, want a rejection", endpoint)
		}
	}
}

func TestGenerateSendsRequest(t *testing.T) {
	server, recorder := newRecordingServer(t, readFixture(t, testdataStopFixture))
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

	recorded := recorder.only(t)
	if recorded.method != http.MethodPost {
		t.Errorf("method = %q, want %q", recorded.method, http.MethodPost)
	}
	if recorded.target != "/" {
		t.Errorf("request URI = %q, want %q", recorded.target, "/")
	}
	if got := recorded.header.Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q, want %q", got, "application/json")
	}
	if got := recorded.header.Get("Authorization"); got != "Bearer "+testAPIKey {
		t.Errorf("Authorization = %q, want the Bearer test key", got)
	}
	if strings.Contains(string(recorded.body), testAPIKey) || strings.Contains(recorded.target, testAPIKey) {
		t.Error("the request body or URL contains the API key")
	}

	var body struct {
		Model    string `json:"model"`
		Messages []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"messages"`
		Stream *bool `json:"stream"`
	}
	if err := json.Unmarshal(recorded.body, &body); err != nil {
		t.Fatalf("decode request body: %v", err)
	}
	if body.Model != testModel {
		t.Errorf("model = %q, want %q", body.Model, testModel)
	}
	if len(body.Messages) != 2 {
		t.Fatalf("messages = %d, want 2", len(body.Messages))
	}
	if body.Messages[0].Role != "system" || body.Messages[0].Content != req.SystemPrompt {
		t.Errorf("messages[0] = %+v, want the system prompt unmodified", body.Messages[0])
	}
	if body.Messages[1].Role != "user" || body.Messages[1].Content != req.UserPrompt {
		t.Errorf("messages[1] = %+v, want the user prompt unmodified", body.Messages[1])
	}
	if body.Stream == nil || *body.Stream {
		t.Errorf("stream = %v, want false", body.Stream)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(recorded.body, &raw); err != nil {
		t.Fatalf("decode request body members: %v", err)
	}
	if _, ok := raw["thinking"]; ok {
		t.Error("request body contains a thinking member")
	}
	if _, ok := raw["max_tokens"]; ok {
		t.Error("request body contains max_tokens for MaxOutputTokens 0")
	}
}

func TestGenerateMaxTokens(t *testing.T) {
	cases := []struct {
		name     string
		tokens   int
		wantSent bool
	}{
		{"zero omits max_tokens", 0, false},
		{"positive sends max_tokens", 256, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server, recorder := newRecordingServer(t, readFixture(t, testdataStopFixture))
			req := validRequest()
			req.MaxOutputTokens = tc.tokens
			if _, err := generateAgainst(t, server.URL, req, nil); err != nil {
				t.Fatalf("Generate() error = %v", err)
			}
			var raw map[string]json.RawMessage
			if err := json.Unmarshal(recorder.only(t).body, &raw); err != nil {
				t.Fatalf("decode request body: %v", err)
			}
			value, ok := raw["max_tokens"]
			if !tc.wantSent {
				if ok {
					t.Errorf("max_tokens = %s, want it omitted", value)
				}
				return
			}
			if !ok {
				t.Fatal("max_tokens is missing from the request body")
			}
			var got int
			if err := json.Unmarshal(value, &got); err != nil {
				t.Fatalf("decode max_tokens: %v", err)
			}
			if got != tc.tokens {
				t.Errorf("max_tokens = %d, want %d", got, tc.tokens)
			}
		})
	}
}

func TestGenerateInvalidRequest(t *testing.T) {
	stop := readFixture(t, testdataStopFixture)
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
			server, recorder := newRecordingServer(t, stop)
			response, err := generateAgainst(t, server.URL, tc.req, nil)
			if !errors.Is(err, llm.ErrInvalidRequest) {
				t.Errorf("Generate() error = %v, want ErrInvalidRequest", err)
			}
			assertRejected(t, response, err)
			if count := recorder.count(); count != 0 {
				t.Errorf("server received %d requests, want 0", count)
			}
		})
	}

	t.Run("an invalid request wins over a done context", func(t *testing.T) {
		server, recorder := newRecordingServer(t, stop)
		client := newTestClient(t, server.URL, nil)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		response, err := client.Generate(ctx, llm.GenerateRequest{})
		if !errors.Is(err, llm.ErrInvalidRequest) {
			t.Errorf("Generate() error = %v, want ErrInvalidRequest", err)
		}
		assertRejected(t, response, err)
		if count := recorder.count(); count != 0 {
			t.Errorf("server received %d requests, want 0", count)
		}
	})

	t.Run("a done context sends nothing", func(t *testing.T) {
		server, recorder := newRecordingServer(t, stop)
		client := newTestClient(t, server.URL, nil)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		response, err := client.Generate(ctx, validRequest())
		if !errors.Is(err, context.Canceled) {
			t.Errorf("Generate() error = %v, want context.Canceled", err)
		}
		assertRejected(t, response, err)
		if count := recorder.count(); count != 0 {
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
		http.StatusTooManyRequests,
		http.StatusInternalServerError,
		http.StatusServiceUnavailable,
		http.StatusTemporaryRedirect,
	}
	for _, status := range statuses {
		t.Run(strconv.Itoa(status), func(t *testing.T) {
			const marker = "RESPONSE-BODY-MARKER"
			body := []byte("{ not json " + marker)
			server := newResponseServer(t, status, body)
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
			assertRejected(t, response, err)
		})
	}
}

func TestGenerateTimeout(t *testing.T) {
	const shortTimeout = 100 * time.Millisecond
	cases := []struct {
		name    string
		partial bool
	}{
		{"no response headers", false},
		{"partial response body", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server := newStallingServer(t, tc.partial)
			start := time.Now()
			response, err := generateAgainst(t, server.URL, validRequest(), func(options *Options) {
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
		server := newStallingServer(t, false)
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
		server, entered := newGateServer(t)
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
		server, recorder := newRecordingServer(t, readFixture(t, testdataStopFixture))
		client := newTestClient(t, server.URL, nil)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		response, err := client.Generate(ctx, validRequest())
		if !errors.Is(err, context.Canceled) {
			t.Errorf("Generate() error = %v, want context.Canceled", err)
		}
		assertRejected(t, response, err)
		if count := recorder.count(); count != 0 {
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
		proxy, err := startBlackholeProxy()
		if err != nil {
			t.Fatalf("start listener: %v", err)
		}
		t.Cleanup(proxy.stop)
		response, err := generateAgainst(t, proxy.url(), validRequest(), nil)
		checkTransportFailure(t, response, err)
	})

	t.Run("body truncated before Content-Length", func(t *testing.T) {
		response, err := generateAgainst(t, newTruncatedResponseServer(t), validRequest(), nil)
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
	productionURL := &url.URL{Scheme: "https", Host: "api.deepseek.com", Path: "/chat/completions"}
	proxyURL, err := http.ProxyFromEnvironment(&http.Request{URL: productionURL})
	if err != nil {
		t.Fatalf("ProxyFromEnvironment error = %v", err)
	}
	if proxyURL == nil || proxyURL.String() != testProxy.url() {
		t.Fatalf("proxy for %s = %v, want %s", productionURL, proxyURL, testProxy.url())
	}

	value, err := New(Options{APIKey: mustSecret(t, testAPIKey), Model: testModel, Timeout: testClientTimeout})
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

	before := testProxy.accepted.Load()
	response, err := client.Generate(context.Background(), validRequest())
	if !errors.Is(err, ErrTransport) {
		t.Errorf("Generate() error = %v, want ErrTransport", err)
	}
	assertRejected(t, response, err)
	if testProxy.accepted.Load() <= before {
		t.Error("the blackhole listener accepted no connection; the request did not use the test proxy")
	}
}

func TestGenerateSentinelsDistinct(t *testing.T) {
	stop := readFixture(t, testdataStopFixture)
	length := readFixture(t, testdataLengthFixture)
	cases := []struct {
		name string
		want error
		run  func(t *testing.T) (llm.GenerateResponse, error)
	}{
		{"invalid request", llm.ErrInvalidRequest, func(t *testing.T) (llm.GenerateResponse, error) {
			server, _ := newRecordingServer(t, stop)
			req := validRequest()
			req.UserPrompt = ""
			return generateAgainst(t, server.URL, req, nil)
		}},
		{"http status", ErrHTTPStatus, func(t *testing.T) (llm.GenerateResponse, error) {
			server := newResponseServer(t, http.StatusInternalServerError, []byte("boom"))
			return generateAgainst(t, server.URL, validRequest(), nil)
		}},
		{"invalid response", ErrInvalidResponse, func(t *testing.T) (llm.GenerateResponse, error) {
			server := newResponseServer(t, http.StatusOK, []byte("not json"))
			return generateAgainst(t, server.URL, validRequest(), nil)
		}},
		{"truncated", llm.ErrTruncated, func(t *testing.T) (llm.GenerateResponse, error) {
			server := newResponseServer(t, http.StatusOK, length)
			return generateAgainst(t, server.URL, validRequest(), nil)
		}},
		{"unexpected finish reason", llm.ErrUnexpectedFinishReason, func(t *testing.T) (llm.GenerateResponse, error) {
			body := replaceOnce(t, stop, `"finish_reason":"stop"`, `"finish_reason":"content_filter"`)
			server := newResponseServer(t, http.StatusOK, body)
			return generateAgainst(t, server.URL, validRequest(), nil)
		}},
		{"empty response", llm.ErrEmptyResponse, func(t *testing.T) (llm.GenerateResponse, error) {
			document := documentOf(t, stop)
			messageOf(t, document)[keyContent] = ""
			server := newResponseServer(t, http.StatusOK, encodeDocument(t, document))
			return generateAgainst(t, server.URL, validRequest(), nil)
		}},
		{"transport", ErrTransport, func(t *testing.T) (llm.GenerateResponse, error) {
			proxy, err := startBlackholeProxy()
			if err != nil {
				t.Fatalf("start listener: %v", err)
			}
			t.Cleanup(proxy.stop)
			return generateAgainst(t, proxy.url(), validRequest(), nil)
		}},
		{"deadline", context.DeadlineExceeded, func(t *testing.T) (llm.GenerateResponse, error) {
			server := newStallingServer(t, false)
			return generateAgainst(t, server.URL, validRequest(), func(options *Options) {
				options.Timeout = 100 * time.Millisecond
			})
		}},
		// A pre-canceled context already makes the transport return
		// context.Canceled without the adapter's own pre-send check, which
		// TestGenerateInvalidRequest's ordering case pins instead.
		{"canceled", context.Canceled, func(t *testing.T) (llm.GenerateResponse, error) {
			server, _ := newRecordingServer(t, stop)
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

func TestIntegrationSettings(t *testing.T) {
	const (
		testIntegrationKey   = "test-integration-key-0123456789"
		testProductionKey    = "test-production-key-0123456789"
		testIntegrationModel = "deepseek-custom"
	)
	// complete is an environment in which the integration test runs. Each
	// case overrides some of its variables; an empty value stands for both
	// unset and empty, which getenv cannot tell apart. The ordering cases
	// override two variables to show which check wins.
	complete := map[string]string{
		integrationOptInEnv:  integrationOptInValue,
		integrationAPIKeyEnv: testIntegrationKey,
		integrationModelEnv:  testIntegrationModel,
		"DEEPSEEK_API_KEY":   testProductionKey,
	}
	for _, tc := range []struct {
		name       string
		change     map[string]string
		wantAction integrationAction
		wantReason []string
	}{
		{name: "opt_in_missing", change: map[string]string{integrationOptInEnv: ""}, wantAction: integrationSkip, wantReason: []string{integrationOptInEnv, "make test-integration-deepseek"}},
		{name: "opt_in_zero", change: map[string]string{integrationOptInEnv: "0"}, wantAction: integrationSkip, wantReason: []string{integrationOptInEnv, "make test-integration-deepseek"}},
		{name: "opt_in_true", change: map[string]string{integrationOptInEnv: "true"}, wantAction: integrationSkip, wantReason: []string{integrationOptInEnv, "make test-integration-deepseek"}},
		{name: "opt_in_padded", change: map[string]string{integrationOptInEnv: " 1"}, wantAction: integrationSkip, wantReason: []string{integrationOptInEnv, "make test-integration-deepseek"}},
		{name: "opt_in_checked_before_api_key", change: map[string]string{integrationOptInEnv: "", integrationAPIKeyEnv: ""}, wantAction: integrationSkip, wantReason: []string{integrationOptInEnv}},
		{name: "api_key_missing_with_production_key_set", change: map[string]string{integrationAPIKeyEnv: ""}, wantAction: integrationSkip, wantReason: []string{integrationAPIKeyEnv}},
		{name: "api_key_checked_before_model", change: map[string]string{integrationAPIKeyEnv: "", integrationModelEnv: ""}, wantAction: integrationSkip, wantReason: []string{integrationAPIKeyEnv}},
		{name: "opt_in_checked_before_godebug", change: map[string]string{integrationOptInEnv: "", godebugEnv: "http2debug=1"}, wantAction: integrationSkip, wantReason: []string{integrationOptInEnv}},
		{name: "api_key_checked_before_godebug", change: map[string]string{integrationAPIKeyEnv: "", godebugEnv: "http2debug=1"}, wantAction: integrationSkip, wantReason: []string{integrationAPIKeyEnv}},
		{name: "model_missing", change: map[string]string{integrationModelEnv: ""}, wantAction: integrationFail, wantReason: []string{integrationModelEnv}},
		{name: "godebug_http2debug_1", change: map[string]string{godebugEnv: "http2debug=1"}, wantAction: integrationFail, wantReason: []string{godebugEnv, "http2debug=1"}},
		{name: "godebug_http2debug_2_among_others", change: map[string]string{godebugEnv: "gctrace=1,http2debug=2"}, wantAction: integrationFail, wantReason: []string{godebugEnv, "http2debug=2"}},
		{name: "godebug_unrelated", change: map[string]string{godebugEnv: "http2client=0"}, wantAction: integrationRun},
		{name: "complete", wantAction: integrationRun},
	} {
		t.Run(tc.name, func(t *testing.T) {
			env := maps.Clone(complete)
			maps.Copy(env, tc.change)
			read := []string{}
			settings := integrationSettingsFrom(func(name string) string {
				read = append(read, name)
				return env[name]
			})
			if slices.Contains(read, "DEEPSEEK_API_KEY") {
				t.Errorf("integrationSettingsFrom read DEEPSEEK_API_KEY; it must use only %s", integrationAPIKeyEnv)
			}
			if settings.action != tc.wantAction {
				t.Fatalf("action = %d, want %d (reason %q)", settings.action, tc.wantAction, settings.reason)
			}
			for _, want := range tc.wantReason {
				if !strings.Contains(settings.reason, want) {
					t.Errorf("reason %q does not mention %q", settings.reason, want)
				}
			}
			for _, key := range []string{testIntegrationKey, testProductionKey} {
				if strings.Contains(settings.reason, key) {
					t.Errorf("reason %q contains an API key", settings.reason)
				}
			}
			if tc.wantAction != integrationRun {
				if settings.model != "" {
					t.Errorf("settings for %d carry the model %q", settings.action, settings.model)
				}
				if _, err := settings.apiKey.Reveal(); err == nil {
					t.Errorf("settings for %d carry an API key", settings.action)
				}
				return
			}
			if settings.model != testIntegrationModel {
				t.Errorf("model = %q, want %q", settings.model, testIntegrationModel)
			}
			key, err := settings.apiKey.Reveal()
			if err != nil || key != testIntegrationKey {
				t.Errorf("apiKey is not the value of %s (Reveal error = %v)", integrationAPIKeyEnv, err)
			}
		})
	}
}

// firstLineIs reports an error unless the file at path exists and its first
// line is exactly want.
func firstLineIs(path, want string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	line, _, _ := strings.Cut(string(content), "\n")
	if line != want {
		return fmt.Errorf("%s: first line is %q, want %q", path, line, want)
	}
	return nil
}

// TestIntegrationTestBuildTag pins the build tags that keep the integration
// test, which calls the real DeepSeek API, out of `make test` and
// `make test-ci`, while its settings helper builds under both tags. The
// guard itself is checked to fail on a missing file and on a different first
// line.
func TestIntegrationTestBuildTag(t *testing.T) {
	for _, tc := range []struct {
		path string
		want string
	}{
		{path: "integration_test.go", want: "//go:build integration"},
		{path: "integration_env_test.go", want: "//go:build test || integration"},
	} {
		if err := firstLineIs(tc.path, tc.want); err != nil {
			t.Errorf("%s must start with %q: %v", tc.path, tc.want, err)
		}
	}

	dir := t.TempDir()
	if err := firstLineIs(filepath.Join(dir, "missing_test.go"), "//go:build integration"); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("missing file: error = %v, want fs.ErrNotExist", err)
	}
	wrong := filepath.Join(dir, "wrong_test.go")
	if err := os.WriteFile(wrong, []byte("//go:build test\n\npackage deepseek\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := firstLineIs(wrong, "//go:build integration"); err == nil {
		t.Error("wrong first line: error = nil, want a mismatch")
	}
}
