//go:build test

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/isseis/yt2column/internal/config"
	"github.com/isseis/yt2column/internal/failure"
	"github.com/isseis/yt2column/internal/llm"
	"github.com/isseis/yt2column/internal/publisher"
	"github.com/isseis/yt2column/internal/secret"
	"github.com/isseis/yt2column/internal/server"
	"github.com/isseis/yt2column/internal/signalrun"
	"github.com/isseis/yt2column/internal/writer"
)

// Fixed values the tests share. The secrets carry markers, so a test can look
// for any part of them in the output.
const (
	testAPIKey        = "sk-unit-APIKEYVALUE-0123456789-TAIL8KEY" //nolint:gosec // a fake key the redaction checks look for
	apiKeyMarker      = "APIKEYVALUE"
	testAnthropicKey  = "sk-ant-ANTHROPICVALUE-0123456789-TAIL8ANT" //nolint:gosec // a fake key the redaction checks look for
	anthropicMarker   = "ANTHROPICVALUE"
	webhookPathMarker = "WEBHOOKVALUE"
	testWebhookPath   = "/services/T0/B0/" + webhookPathMarker + "-TAIL8HKS"
	// testWebhook passes slackwebhook.ValidURL, as config.LoadServer
	// requires; the tests never send to it.
	testWebhook = "https://hooks.slack.com" + testWebhookPath
	testModel   = "fake-model"
	// testVideoURL is not the normalized form, so a post that names the
	// normalized URL proves the server derived it.
	testVideoURL      = "https://youtu.be/dQw4w9WgXcQ?t=42"
	testNormalizedURL = "https://www.youtube.com/watch?v=dQw4w9WgXcQ"
	// testValidRaw is a copied transcript the parser accepts.
	testValidRaw = "0:00\nHello\n"
	// extensionOrigin is the one Origin the server accepts.
	extensionOrigin = "chrome-extension://clfmbbcdpnjcefbdihdoahomaifbabkk"
	// emptyReviewJSON is a valid review response that revises nothing.
	emptyReviewJSON = `{"revisions":[]}`
)

// Environment variables that switch the test binary into a server child
// process (see TestMain) and tell the child's fake LLM client where its
// markers are.
const (
	childModeEnv    = "YT2COLUMN_TEST_SERVER_MODE"
	childReadyEnv   = "YT2COLUMN_TEST_LLM_READY"
	childReleaseEnv = "YT2COLUMN_TEST_LLM_RELEASE"
)

// Child modes.
const (
	// childModeMain runs the production main, so a test observes exactly the
	// signal handling main installs. No request is sent to it, so its real
	// LLM client and publisher never send anything.
	childModeMain = "main"
	// childModeBlock runs signalrun.Run with an LLM client that waits until
	// the test creates the release marker, and a publisher and notifier that
	// send nothing.
	childModeBlock = "block"
)

// childExitUnknownMode is the child's exit code for an unknown mode.
const childExitUnknownMode = 99

const (
	// blockLifetime caps how long the blocking fake LLM client waits, so a
	// leaked child's job ends by itself (the child then keeps serving until
	// the cleanup kills it). It is far longer than any bound a test waits for.
	blockLifetime = 3 * time.Minute
	// readyBound bounds a wait for a server to listen or a fake to signal
	// readiness.
	readyBound = 30 * time.Second
	// exitBound bounds a wait for a run or a child to return.
	exitBound = 30 * time.Second
	// pollInterval paces the bounded polling loops.
	pollInterval = 20 * time.Millisecond
	// childWaitDelay bounds how long Wait drains a child's output after it
	// exits.
	childWaitDelay = 10 * time.Second
	// listenAttempts bounds the retries when a free port is taken between
	// probing it and listening on it.
	listenAttempts = 5
	// loopbackTimeout bounds one post to the loopback webhook and one request
	// to the server.
	loopbackTimeout = 10 * time.Second
)

// childEnvAllowlist names the parent variables a server child inherits.
// Everything else, including the developer's YT2COLUMN_* variables, secrets,
// HOME, and GODEBUG, is left out; the test sets what the child needs.
var childEnvAllowlist = []string{"PATH", "TMPDIR"}

var (
	errBlockExpired      = errors.New("the blocking fake reached its lifetime")
	errUnexpectedWebhook = errors.New("run passed a webhook URL other than the configured one")
)

// serverEnv is the environment of one server: a temporary HOME, which holds
// the token file, and the variables config.LoadServer reads. Every start of
// the same serverEnv uses the same token file.
type serverEnv struct {
	vars map[string]string
}

// newServerEnv returns the variables of a valid configuration with HOME in a
// fresh temporary directory. The port is set by each start.
func newServerEnv(t *testing.T) *serverEnv {
	t.Helper()
	return &serverEnv{vars: map[string]string{
		"YT2COLUMN_MODEL":   testModel,
		"DEEPSEEK_API_KEY":  testAPIKey,
		"SLACK_WEBHOOK_URL": testWebhook,
		"HOME":              t.TempDir(),
	}}
}

// lookup returns a config.LookupFunc over the variables.
func (e *serverEnv) lookup() config.LookupFunc {
	return func(name string) (string, bool) {
		value, ok := e.vars[name]
		return value, ok
	}
}

// setPort sets YT2COLUMN_SERVER_PORT.
func (e *serverEnv) setPort(port uint16) {
	e.vars["YT2COLUMN_SERVER_PORT"] = strconv.Itoa(int(port))
}

// port returns the port last set.
func (e *serverEnv) port(t *testing.T) uint16 {
	t.Helper()
	port, err := strconv.Atoi(e.vars["YT2COLUMN_SERVER_PORT"])
	if err != nil {
		t.Fatalf("the test set no port: %v", err)
	}
	return uint16(port) //nolint:gosec // set by setPort from a uint16
}

// tokenPath returns the token file's path, derived by config.LoadServer as
// the server derives it.
func (e *serverEnv) tokenPath(t *testing.T) string {
	t.Helper()
	cfg, err := config.LoadServer(e.lookup())
	if err != nil {
		t.Fatalf("LoadServer: %v", err)
	}
	return cfg.TokenPath()
}

// readToken returns the content of the token file.
func (e *serverEnv) readToken(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(e.tokenPath(t))
	if err != nil {
		t.Fatalf("read the token file: %v", err)
	}
	return string(data)
}

// requireIPv6Loopback skips the test when the IPv6 loopback is unavailable:
// the server listens on both loopback addresses and cannot start without it.
func requireIPv6Loopback(t *testing.T) {
	t.Helper()
	listener, err := net.Listen("tcp", "[::1]:0")
	if err != nil {
		t.Skipf("IPv6 loopback is unavailable, so the server cannot listen: %v", err)
	}
	_ = listener.Close()
}

// freePort returns a port that was free on both loopback addresses a moment
// ago. The caller retries when it is taken before the server listens.
func freePort(t *testing.T) uint16 {
	t.Helper()
	for range listenAttempts {
		ipv4, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("find a free port: %v", err)
		}
		port := ipv4.Addr().(*net.TCPAddr).Port
		ipv6, err := net.Listen("tcp", "[::1]:"+strconv.Itoa(port))
		_ = ipv4.Close()
		if err != nil {
			continue
		}
		_ = ipv6.Close()
		return uint16(port) //nolint:gosec // a TCP port fits in uint16
	}
	t.Fatal("no port is free on both loopback addresses")
	return 0
}

// listening reports whether a connection to the IPv4 loopback on port is
// accepted.
func listening(port uint16) bool {
	conn, err := net.DialTimeout("tcp", "127.0.0.1:"+strconv.Itoa(int(port)), time.Second)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

// runningServer is a run started by startServer.
type runningServer struct {
	cancel context.CancelFunc
	done   chan struct{}
	code   int
	// stdout is synchronized, so a test can read what run has written while
	// the server keeps serving; stderr is read only after stop.
	stdout *syncBuffer
	stderr *bytes.Buffer
}

// syncBuffer is a buffer whose writes and reads are synchronized, so run's
// display can be read while run keeps serving (make test runs with -race).
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

// Write implements io.Writer.
func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

// String returns the bytes written so far.
func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// startServer calls run in a goroutine on a free port and returns once the
// server accepts connections. When the port is taken before run listens (run
// returns exitFailure without serving), it tries another. Each attempt gets a
// fresh stdout buffer, so the returned server's standard output holds only the
// listening attempt's display; the retries' standard error is kept for the
// diagnostic on failure. A cleanup stops the run.
func startServer(t *testing.T, e *serverEnv, d deps) *runningServer {
	t.Helper()
	requireIPv6Loopback(t)
	stderr := &bytes.Buffer{}
	for range listenAttempts {
		e.setPort(freePort(t))
		stdout := &syncBuffer{}
		ctx, cancel := context.WithCancel(context.Background())
		r := &runningServer{cancel: cancel, done: make(chan struct{}), stdout: stdout, stderr: stderr}
		lookup := e.lookup()
		go func() {
			defer close(r.done)
			r.code = run(ctx, nil, lookup, stdout, stderr, d)
		}()
		if waitListening(e.port(t), r.done) {
			t.Cleanup(func() { r.stop(t) })
			return r
		}
		cancel()
		<-r.done
		if r.code != exitFailure {
			t.Fatalf("run returned %d before listening, want it to listen\nstderr:\n%s", r.code, stderr.String())
		}
	}
	t.Fatalf("run could not listen after %d attempts\nstderr:\n%s", listenAttempts, stderr.String())
	return nil
}

// waitListening waits up to readyBound for port to accept connections. It
// returns false at once when done is closed.
func waitListening(port uint16, done <-chan struct{}) bool {
	deadline := time.Now().Add(readyBound)
	for time.Now().Before(deadline) {
		select {
		case <-done:
			return false
		default:
		}
		if listening(port) {
			return true
		}
		time.Sleep(pollInterval)
	}
	return false
}

// stop cancels the run and waits for it to return, and returns its exit code.
// It is safe to call more than once.
func (r *runningServer) stop(t *testing.T) int {
	t.Helper()
	r.cancel()
	select {
	case <-r.done:
		return r.code
	case <-time.After(exitBound):
		t.Fatalf("run did not return within %v of the cancellation", exitBound)
		return -1
	}
}

// funcLLM is an llm.LLMClient that runs the function.
type funcLLM func(ctx context.Context, req llm.GenerateRequest) (llm.GenerateResponse, error)

// Generate implements llm.LLMClient.
func (f funcLLM) Generate(ctx context.Context, req llm.GenerateRequest) (llm.GenerateResponse, error) {
	return f(ctx, req)
}

// validResponse is a generated text the real ArticleWriter accepts.
func validResponse() llm.GenerateResponse {
	return llm.GenerateResponse{Text: "# A title\n\nThe body.\n", Model: testModel, ModelVersion: "v1"}
}

// validResponseLLM returns a client that always succeeds.
func validResponseLLM() llm.LLMClient {
	return funcLLM(func(context.Context, llm.GenerateRequest) (llm.GenerateResponse, error) {
		return validResponse(), nil
	})
}

// reviewLLM returns a client that revises nothing.
func reviewLLM() llm.LLMClient {
	return funcLLM(func(context.Context, llm.GenerateRequest) (llm.GenerateResponse, error) {
		return llm.GenerateResponse{Text: emptyReviewJSON, Model: "review-model"}, nil
	})
}

// nopPublisher is a publisher that accepts every article and sends nothing.
type nopPublisher struct{}

// Publish implements publisher.Publisher.
func (nopPublisher) Publish(context.Context, writer.Article) error { return nil }

// nopNotifier is a notifier that accepts every notice and sends nothing.
type nopNotifier struct{}

// Notify implements failure.Notifier.
func (nopNotifier) Notify(context.Context, failure.Notice) error { return nil }

// testDeps returns deps that build client for generation, a review client
// that revises nothing, and a publisher and notifier that send nothing, so no
// test reaches the LLM API or the webhook the environment names.
func testDeps(client llm.LLMClient) deps {
	return deps{
		newLLMClient:       func(config.LLMConfig) (llm.LLMClient, error) { return client, nil },
		newReviewLLMClient: func(config.LLMConfig) (llm.LLMClient, error) { return reviewLLM(), nil },
		newWriter:          writer.New,
		newPublisher:       func(secret.Secret) (publisher.Publisher, error) { return nopPublisher{}, nil },
		newNotifier:        func(secret.Secret) (failure.Notifier, error) { return nopNotifier{}, nil },
	}
}

// loopbackDeps returns testDeps(client) with the real SlackWebhookPublisher
// and SlackFailureNotifier posting to serverURL, a loopback webhook. Both are
// built here, in the test goroutine, because their constructors may fail t;
// the deps return them after checking that run passes the configured webhook
// URL. The endpoint keeps the configured path, so it carries the same secret
// path the server redacts.
func loopbackDeps(t *testing.T, client llm.LLMClient, serverURL string) deps {
	t.Helper()
	endpoint := serverURL + testWebhookPath
	pub := publisher.NewSlackWebhookPublisherForLoopbackTest(t, publisher.SlackTestOptions{Timeout: loopbackTimeout}, endpoint)
	notifier := publisher.NewSlackFailureNotifierForLoopbackTest(t, loopbackTimeout, endpoint)
	d := testDeps(client)
	d.newPublisher = func(webhookURL secret.Secret) (publisher.Publisher, error) {
		if err := requireTestWebhook(webhookURL); err != nil {
			return nil, err
		}
		return pub, nil
	}
	d.newNotifier = func(webhookURL secret.Secret) (failure.Notifier, error) {
		if err := requireTestWebhook(webhookURL); err != nil {
			return nil, err
		}
		return notifier, nil
	}
	return d
}

// requireTestWebhook reports errUnexpectedWebhook unless webhookURL is
// testWebhook, the value the test environment configures.
func requireTestWebhook(webhookURL secret.Secret) error {
	if value, err := webhookURL.Reveal(); err != nil || value != testWebhook {
		return errUnexpectedWebhook
	}
	return nil
}

// webhookServer is a loopback webhook that records the text of every message
// it receives and answers 200 "ok".
type webhookServer struct {
	mu    sync.Mutex
	texts []string
}

// newWebhookServer starts a webhookServer and returns it with its URL.
func newWebhookServer(t *testing.T) (*webhookServer, string) {
	t.Helper()
	w := &webhookServer{}
	srv := httptest.NewServer(w)
	t.Cleanup(srv.Close)
	return w, srv.URL
}

// ServeHTTP records the message and answers it.
func (s *webhookServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Text string `json:"text"`
	}
	body, _ := io.ReadAll(r.Body)
	_ = json.Unmarshal(body, &payload)
	s.mu.Lock()
	s.texts = append(s.texts, payload.Text)
	s.mu.Unlock()
	_, _ = io.WriteString(w, "ok")
}

// messages returns the text of every message received so far.
func (s *webhookServer) messages() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.texts)
}

// waitForMessage waits up to readyBound for a message that contains every
// string in want, and returns it.
func (s *webhookServer) waitForMessage(t *testing.T, want ...string) string {
	t.Helper()
	deadline := time.Now().Add(readyBound)
	for time.Now().Before(deadline) {
		for _, text := range s.messages() {
			if containsAll(text, want) {
				return text
			}
		}
		time.Sleep(pollInterval)
	}
	t.Fatalf("no webhook message contains %q within %v; received:\n%s", want, readyBound, strings.Join(s.messages(), "\n---\n"))
	return ""
}

// containsAll reports whether s contains every string in subs.
func containsAll(s string, subs []string) bool {
	for _, sub := range subs {
		if !strings.Contains(s, sub) {
			return false
		}
	}
	return true
}

// apiResponse is the JSON body the server answers with.
type apiResponse struct {
	Status    string `json:"status"`
	Reason    string `json:"reason"`
	RequestID string `json:"requestId"`
}

// requestBody returns a request body for testVideoURL with raw as the
// transcript.
func requestBody(raw string) string {
	data, err := json.Marshal(map[string]string{"videoUrl": testVideoURL, "transcript": raw})
	if err != nil {
		panic(err) // a map of strings always marshals
	}
	return string(data)
}

// loopbackClient sends to the server directly, never through a proxy.
var loopbackClient = &http.Client{Transport: &http.Transport{}, Timeout: loopbackTimeout}

// postArticle sends body to the server on port with the headers the extension
// sends and token as the shared token, and returns the status and the decoded
// response. A transport error is returned rather than failing the test, so a
// caller can record a cut connection.
func postArticle(port uint16, token string, body io.Reader) (int, apiResponse, error) {
	target := "http://127.0.0.1:" + strconv.Itoa(int(port)) + server.ArticlesPath
	req, err := http.NewRequest(http.MethodPost, target, body)
	if err != nil {
		return 0, apiResponse{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", extensionOrigin)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := loopbackClient.Do(req)
	if err != nil {
		return 0, apiResponse{}, err
	}
	defer func() { _ = resp.Body.Close() }()
	var decoded apiResponse
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return resp.StatusCode, apiResponse{}, fmt.Errorf("decode the response: %w", err)
	}
	return resp.StatusCode, decoded, nil
}

// mustPostArticle is postArticle that fails the test on a transport error and
// on a status other than want.
func mustPostArticle(t *testing.T, port uint16, token, body string, want int) apiResponse {
	t.Helper()
	status, resp, err := postArticle(port, token, strings.NewReader(body))
	if err != nil {
		t.Fatalf("post an article: %v", err)
	}
	if status != want {
		t.Fatalf("status = %d (%+v), want %d", status, resp, want)
	}
	return resp
}

// blockingLLM creates ready, then waits until release exists, and then
// succeeds. It gives up after blockLifetime. It ignores its context, as the
// server never cancels a job's context.
type blockingLLM struct {
	ready   string
	release string
}

// Generate implements llm.LLMClient.
func (b blockingLLM) Generate(context.Context, llm.GenerateRequest) (llm.GenerateResponse, error) {
	writeMarker(b.ready)
	deadline := time.Now().Add(blockLifetime)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(b.release); err == nil {
			return validResponse(), nil
		}
		time.Sleep(pollInterval)
	}
	return llm.GenerateResponse{}, errBlockExpired
}

// writeMarker creates path by renaming a temporary file, so a reader never
// sees it half-written. Failures are ignored: a missing marker makes the
// waiting test fail with the marker's name.
func writeMarker(path string) {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, nil, 0o600); err != nil {
		return
	}
	_ = os.Rename(tmp, path)
}

// runChildMode runs the server in a child process started by
// startServerChild and returns its exit code. The configuration comes from
// the child's environment.
func runChildMode(mode string) int {
	switch mode {
	case childModeMain:
		main()
		return exitFailure // main exits; this line is never reached
	case childModeBlock:
		d := testDeps(blockingLLM{ready: os.Getenv(childReadyEnv), release: os.Getenv(childReleaseEnv)})
		return signalrun.Run(func(ctx context.Context) int {
			return run(ctx, os.Args[1:], os.LookupEnv, os.Stdout, os.Stderr, d)
		})
	default:
		_, _ = fmt.Fprintf(os.Stderr, "unknown child mode %q\n", mode)
		return childExitUnknownMode
	}
}

// serverChild is a server child process started by startServerChild.
type serverChild struct {
	cmd    *exec.Cmd
	stdout bytes.Buffer
	stderr bytes.Buffer
	done   chan struct{}
}

// startServerChild starts the test binary as a server child in mode on a free
// port and returns once it accepts connections, retrying on another port when
// the child cannot listen. Its environment is the allowlisted parent
// variables, e's variables, extra, and the mode. A cleanup that kills and
// reaps each child is registered as soon as it starts.
func startServerChild(t *testing.T, mode string, e *serverEnv, extra map[string]string) *serverChild {
	t.Helper()
	return startServerChildWithStdout(t, mode, e, extra, nil)
}

// startServerChildWithStdout is startServerChild, but when stdout is non-nil
// the child's standard output is that writer instead of the captured buffer.
// The closed-pipe test passes the write end of a pipe whose reader it has
// already closed, so the child's display write reaches a closed pipe.
func startServerChildWithStdout(t *testing.T, mode string, e *serverEnv, extra map[string]string, stdout io.Writer) *serverChild {
	t.Helper()
	requireIPv6Loopback(t)
	for range listenAttempts {
		e.setPort(freePort(t))
		cmd := exec.Command(os.Args[0]) //nolint:gosec // the test binary itself, re-run as the server
		cmd.Env = childEnv(mode, e.vars, extra)
		cmd.WaitDelay = childWaitDelay
		c := &serverChild{cmd: cmd, done: make(chan struct{})}
		if stdout != nil {
			cmd.Stdout = stdout
		} else {
			cmd.Stdout = &c.stdout
		}
		cmd.Stderr = &c.stderr
		if err := cmd.Start(); err != nil {
			t.Fatalf("start the server child: %v", err)
		}
		go func() {
			_ = cmd.Wait()
			close(c.done)
		}()
		t.Cleanup(func() {
			_ = cmd.Process.Kill()
			<-c.done
		})
		if waitListening(e.port(t), c.done) {
			return c
		}
		if !c.exited() {
			t.Fatalf("the server child did not listen within %v", readyBound)
		}
		if code := c.cmd.ProcessState.ExitCode(); code != exitFailure {
			t.Fatalf("the server child exited with %d before listening\nstderr:\n%s", code, c.stderr.String())
		}
	}
	t.Fatalf("the server child could not listen after %d attempts", listenAttempts)
	return nil
}

// childEnv builds a child's environment from the allowlist, the variable
// maps, and mode.
func childEnv(mode string, vars ...map[string]string) []string {
	var out []string
	for _, name := range childEnvAllowlist {
		if value, ok := os.LookupEnv(name); ok {
			out = append(out, name+"="+value)
		}
	}
	for _, m := range vars {
		for name, value := range m {
			out = append(out, name+"="+value)
		}
	}
	return append(out, childModeEnv+"="+mode)
}

// signal sends sig to the child.
func (c *serverChild) signal(t *testing.T, sig os.Signal) {
	t.Helper()
	if err := c.cmd.Process.Signal(sig); err != nil {
		t.Fatalf("send %v to the server child: %v", sig, err)
	}
}

// exited reports whether the child has been reaped.
func (c *serverChild) exited() bool {
	select {
	case <-c.done:
		return true
	default:
		return false
	}
}

// wait waits up to exitBound for the child to exit and returns its state.
// Its output may be read afterwards.
func (c *serverChild) wait(t *testing.T) *os.ProcessState {
	t.Helper()
	select {
	case <-c.done:
		return c.cmd.ProcessState
	case <-time.After(exitBound):
		t.Fatalf("the server child did not exit within %v; its stderr is not shown because the child still writes it", exitBound)
		return nil
	}
}

// waitForFile waits up to readyBound for path to appear, naming it on
// failure.
func waitForFile(t *testing.T, path string) {
	t.Helper()
	deadline := time.Now().Add(readyBound)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(path); err == nil {
			return
		}
		time.Sleep(pollInterval)
	}
	t.Fatalf("%s did not appear within %v", path, readyBound)
}

// requireNoSecret fails the test when output holds the shared token or any
// marked part of an API key or the webhook URL.
func requireNoSecret(t *testing.T, name, output, token string) {
	t.Helper()
	for _, s := range []string{token, apiKeyMarker, anthropicMarker, webhookPathMarker} {
		if s != "" && strings.Contains(output, s) {
			t.Errorf("%s holds a secret (%.8s...):\n%s", name, s, output)
		}
	}
}
