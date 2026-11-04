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
	"github.com/isseis/yt2column/internal/udstest"
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
	// emptyReviewJSON is a valid review response that revises nothing.
	emptyReviewJSON = `{"revisions":[]}`
)

// Environment variables that switch the test binary into a server child
// process (see TestMain) and tell the child's fake Webhook publisher where its
// markers are.
const (
	childModeEnv    = "YT2COLUMN_TEST_SERVER_MODE"
	childReadyEnv   = "YT2COLUMN_TEST_POST_READY"
	childReleaseEnv = "YT2COLUMN_TEST_POST_RELEASE"
)

// Child modes.
const (
	// childModeMain runs the production main, so a test observes exactly the
	// signal handling main installs. No request is sent to it, so its real
	// LLM client and publisher never send anything.
	childModeMain = "main"
	// childModeBlock runs signalrun.Run with an LLM client that succeeds at
	// once, a Webhook publisher whose post waits until the test creates the
	// release marker, and a notifier that sends nothing.
	childModeBlock = "block"
)

// childExitUnknownMode is the child's exit code for an unknown mode.
const childExitUnknownMode = 99

const (
	// blockLifetime caps how long the blocking fake publisher waits, so a
	// leaked child's post job ends by itself (the child then keeps serving
	// until the cleanup kills it). It is far longer than any bound a test
	// waits for.
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

// serverEnv is the environment of one server: HOME, from which the socket
// path is derived, and the other variables config.LoadServer reads. Every start
// of the same serverEnv uses the same socket path.
type serverEnv struct {
	vars map[string]string
}

// newServerEnv returns the variables of a valid configuration with HOME in a
// fresh short directory under /tmp, so the socket path stays within the macOS
// limit and no two tests share it.
func newServerEnv(t *testing.T) *serverEnv {
	t.Helper()
	// The provider is explicit so the tests keep selecting DeepSeek, not the
	// default provider.
	return &serverEnv{vars: map[string]string{
		"YT2COLUMN_LLM":     "deepseek/" + testModel,
		"DEEPSEEK_API_KEY":  testAPIKey,
		"SLACK_WEBHOOK_URL": testWebhook,
		"HOME":              udstest.Dir(t),
	}}
}

// lookup returns a config.LookupFunc over the variables.
func (e *serverEnv) lookup() config.LookupFunc {
	return func(name string) (string, bool) {
		value, ok := e.vars[name]
		return value, ok
	}
}

// socketPath returns the socket path, derived by config.LoadServer as the
// server derives it.
func (e *serverEnv) socketPath(t *testing.T) string {
	t.Helper()
	cfg, err := config.LoadServer(e.lookup())
	if err != nil {
		t.Fatalf("LoadServer: %v", err)
	}
	return cfg.SocketPath()
}

// listening reports whether a connection to the socket at path is accepted.
func listening(path string) bool {
	conn, err := net.DialTimeout("unix", path, time.Second)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

// syncBuffer is a buffer that a run writes and a test reads at the same time.
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

// String returns what was written so far.
func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// runningServer is a run started by startServer. stderr may be read while the
// run writes it.
type runningServer struct {
	cancel context.CancelFunc
	done   chan struct{}
	code   int
	stderr *syncBuffer
}

// startServer calls run in a goroutine and returns once the server accepts
// connections on its socket. A cleanup stops the run.
func startServer(t *testing.T, e *serverEnv, d deps) *runningServer {
	t.Helper()
	stderr := &syncBuffer{}
	ctx, cancel := context.WithCancel(context.Background())
	r := &runningServer{cancel: cancel, done: make(chan struct{}), stderr: stderr}
	lookup := e.lookup()
	go func() {
		defer close(r.done)
		r.code = run(ctx, nil, lookup, stderr, d)
	}()
	if !waitListening(e.socketPath(t), r.done) {
		cancel()
		<-r.done
		t.Fatalf("run returned %d or did not listen within %v\nstderr:\n%s", r.code, readyBound, stderr.String())
	}
	t.Cleanup(func() { r.stop(t) })
	return r
}

// waitListening waits up to readyBound for the socket at path to accept
// connections. It returns false at once when done is closed.
func waitListening(path string, done <-chan struct{}) bool {
	deadline := time.Now().Add(readyBound)
	for time.Now().Before(deadline) {
		select {
		case <-done:
			return false
		default:
		}
		if listening(path) {
			return true
		}
		time.Sleep(pollInterval)
	}
	return false
}

// waitStderr waits up to readyBound for the run's standard error to hold a
// line that contains sub, and returns that line.
func (r *runningServer) waitStderr(t *testing.T, sub string) string {
	t.Helper()
	deadline := time.Now().Add(readyBound)
	for time.Now().Before(deadline) {
		if line := stderrLine(r.stderr.String(), sub); line != "" {
			return line
		}
		time.Sleep(pollInterval)
	}
	t.Fatalf("standard error has no line with %q within %v:\n%s", sub, readyBound, r.stderr.String())
	return ""
}

// stderrLine returns the first line of output that contains sub, or "" when
// there is none.
func stderrLine(output, sub string) string {
	for line := range strings.SplitSeq(output, "\n") {
		if strings.Contains(line, sub) {
			return line
		}
	}
	return ""
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

// PostMessages implements server.WebhookPublisher.
func (nopPublisher) PostMessages(context.Context, publisher.SlackMessages, int, func(int)) error {
	return nil
}

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
		newWriter:          writer.NewProgressWriter,
		newPublisher:       func(secret.Secret) (server.WebhookPublisher, error) { return nopPublisher{}, nil },
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
	d.newPublisher = func(webhookURL secret.Secret) (server.WebhookPublisher, error) {
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

// containsAll reports whether s contains every string in subs.
func containsAll(s string, subs []string) bool {
	for _, sub := range subs {
		if !strings.Contains(s, sub) {
			return false
		}
	}
	return true
}

// apiResponse is the JSON body the server answers with a rejection.
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

// unixClient returns a client that sends every request to the socket at path,
// whatever the URL's host, and never through a proxy.
func unixClient(path string) *http.Client {
	return &http.Client{Transport: udstest.Transport(path), Timeout: loopbackTimeout}
}

// jobResponse is the part of a job API response the tests read: a rejection's
// reason, a generation start's key, and a status response's job state,
// article, and publish state.
type jobResponse struct {
	Status string `json:"status"`
	Reason string `json:"reason"`
	Key    string `json:"key"`
	Job    struct {
		State   string `json:"state"`
		Article *struct {
			Markdown string `json:"markdown"`
			File     string `json:"file"`
		} `json:"article"`
		Publish *struct {
			State string `json:"state"`
		} `json:"publish"`
	} `json:"job"`
}

// doJobs sends method to target on the socket at path with body (nil for
// none) and decodes the response, failing the test unless its status is want.
func doJobs(t *testing.T, path, method, target string, body io.Reader, want int) jobResponse {
	t.Helper()
	req, err := http.NewRequest(method, "http://yt2column"+target, body)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	resp, err := unixClient(path).Do(req)
	if err != nil {
		t.Fatalf("send %s %s: %v", method, target, err)
	}
	defer func() { _ = resp.Body.Close() }()
	var decoded jobResponse
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		t.Fatalf("decode the response to %s %s: %v", method, target, err)
	}
	if resp.StatusCode != want {
		t.Fatalf("%s %s status = %d (%+v), want %d", method, target, resp.StatusCode, decoded, want)
	}
	return decoded
}

// waitJobDone polls the status of key on the socket at path until its job is
// no longer running, and returns that response.
func waitJobDone(t *testing.T, path, key string) jobResponse {
	t.Helper()
	deadline := time.Now().Add(blockLifetime)
	for {
		resp := doJobs(t, path, http.MethodGet, server.JobsPath+"/"+key, nil, http.StatusOK)
		if resp.Job.State != "running" || time.Now().After(deadline) {
			return resp
		}
		time.Sleep(pollInterval)
	}
}

// waitPublished polls the status of key on the socket at path until its
// article is no longer posting, and returns its publish state.
func waitPublished(t *testing.T, path, key string) string {
	t.Helper()
	deadline := time.Now().Add(blockLifetime)
	for {
		resp := doJobs(t, path, http.MethodGet, server.JobsPath+"/"+key, nil, http.StatusOK)
		if resp.Job.Publish == nil {
			t.Fatalf("job = %+v, want a done article with a publish state", resp.Job)
		}
		if resp.Job.Publish.State != "posting" || time.Now().After(deadline) {
			return resp.Job.Publish.State
		}
		time.Sleep(pollInterval)
	}
}

// generateAndPublish generates an article for requestBody(testValidRaw) on
// the socket at path, waits until it is done, and sends its publish
// operation, failing the test unless the operation is accepted.
func generateAndPublish(t *testing.T, path string) {
	t.Helper()
	started := doJobs(t, path, http.MethodPost, server.JobsPath, strings.NewReader(requestBody(testValidRaw)), http.StatusAccepted)
	if done := waitJobDone(t, path, started.Key); done.Job.State != "done" {
		t.Fatalf("job = %+v, want done", done.Job)
	}
	doJobs(t, path, http.MethodPost, server.JobsPath+"/"+started.Key+server.PublishSuffix, nil, http.StatusAccepted)
}

// blockingPublisher is a Webhook publisher whose post creates ready, then
// waits until release exists, and then reports every message delivered. It
// gives up after blockLifetime. It ignores its context, as the server never
// cancels a post job's context.
type blockingPublisher struct {
	nopPublisher
	ready   string
	release string
}

// PostMessages implements server.WebhookPublisher.
func (b blockingPublisher) PostMessages(_ context.Context, m publisher.SlackMessages, from int, delivered func(int)) error {
	writeMarker(b.ready)
	deadline := time.Now().Add(blockLifetime)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(b.release); err == nil {
			for i := from; i < m.Len(); i++ {
				delivered(i)
			}
			return nil
		}
		time.Sleep(pollInterval)
	}
	return errBlockExpired
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
		d := testDeps(validResponseLLM())
		d.newPublisher = func(secret.Secret) (server.WebhookPublisher, error) {
			return blockingPublisher{ready: os.Getenv(childReadyEnv), release: os.Getenv(childReleaseEnv)}, nil
		}
		return signalrun.Run(func(ctx context.Context) int {
			return run(ctx, os.Args[1:], os.LookupEnv, os.Stderr, d)
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

// startServerChild starts the test binary as a server child in mode and
// returns once it accepts connections on its socket. Its environment is the
// allowlisted parent variables, e's variables, extra, and the mode. A cleanup
// that kills and reaps the child is registered as soon as it starts.
func startServerChild(t *testing.T, mode string, e *serverEnv, extra map[string]string) *serverChild {
	t.Helper()
	cmd := exec.Command(os.Args[0]) //nolint:gosec // the test binary itself, re-run as the server
	cmd.Env = childEnv(mode, e.vars, extra)
	cmd.WaitDelay = childWaitDelay
	c := &serverChild{cmd: cmd, done: make(chan struct{})}
	cmd.Stdout = &c.stdout
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
	if !waitListening(e.socketPath(t), c.done) {
		if c.exited() {
			t.Fatalf("the server child exited with %d before listening\nstderr:\n%s", c.cmd.ProcessState.ExitCode(), c.stderr.String())
		}
		t.Fatalf("the server child did not listen within %v", readyBound)
	}
	return c
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

// requireNoSecret fails the test when output holds any marked part of an API
// key or the webhook URL.
func requireNoSecret(t *testing.T, name, output string) {
	t.Helper()
	for _, s := range []string{apiKeyMarker, anthropicMarker, webhookPathMarker} {
		if strings.Contains(output, s) {
			t.Errorf("%s holds a secret (%.8s...):\n%s", name, s, output)
		}
	}
}
