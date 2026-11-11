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
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/isseis/yt2column/internal/config"
	"github.com/isseis/yt2column/internal/llm"
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
	testOpenAIKey     = "sk-openai-OPENAIKEYVALUE-0123456789-TAIL8OAI" //nolint:gosec // a fake key the redaction checks look for
	openAIMarker      = "OPENAIKEYVALUE"
	webhookPathMarker = "WEBHOOKVALUE"
	testWebhookPath   = "/services/T0/B0/" + webhookPathMarker + "-TAIL8HKS"
	// testWebhook passes slackwebhook.ValidURL, as config.LoadServer
	// requires; the tests never send to it.
	testWebhook = "https://hooks.slack.com" + testWebhookPath
	testModel   = "fake-model"
	// testOpenAIModel and testOpenAIReviewModel are the models of an openai
	// server configuration.
	testOpenAIModel       = "gpt-6-astra"
	testOpenAIReviewModel = "gpt-6.1-sol"
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
// process (see TestMain) and tell the child's blocking fake LLM client where
// its markers are.
const (
	childModeEnv    = "YT2COLUMN_TEST_SERVER_MODE"
	childReadyEnv   = "YT2COLUMN_TEST_GENERATION_READY"
	childReleaseEnv = "YT2COLUMN_TEST_GENERATION_RELEASE"
)

// Child modes.
const (
	// childModeMain runs the production main, so a test observes exactly the
	// signal handling main installs. No request is sent to it, so its real
	// LLM client never sends anything.
	childModeMain = "main"
	// childModeBlock runs signalrun.Run with an LLM client whose call waits
	// until the test creates the release marker.
	childModeBlock = "block"
)

// childExitUnknownMode is the child's exit code for an unknown mode.
const childExitUnknownMode = 99

const (
	// blockLifetime caps how long the blocking fake LLM client waits, so a
	// leaked child's generation job ends by itself (the child then keeps
	// serving until the cleanup kills it). It is far longer than any bound a
	// test waits for.
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
	// loopbackTimeout bounds one request to the server.
	loopbackTimeout = 10 * time.Second
)

// childEnvAllowlist names the parent variables a server child inherits.
// Everything else, including the developer's YT2COLUMN_* variables, secrets,
// HOME, and GODEBUG, is left out; the test sets what the child needs.
var childEnvAllowlist = []string{"PATH", "TMPDIR"}

var errBlockExpired = errors.New("the blocking fake reached its lifetime")

// serverEnv is the setting of one server: its environment (HOME, from which
// the configuration file's path and the socket path are derived, and GODEBUG)
// and the keys of its configuration file. Every start of the same serverEnv
// uses the same socket path.
type serverEnv struct {
	vars map[string]string // the process environment
	file map[string]string // the configuration file's keys and values
	// noFile makes writeConfig write nothing, so startServer keeps the
	// file a test created with config init.
	noFile bool
}

// newServerEnv returns a valid configuration with HOME in a fresh short
// directory under /tmp, so the socket path stays within the macOS limit and no
// two tests share it.
func newServerEnv(t *testing.T) *serverEnv {
	t.Helper()
	// The provider is explicit so the tests keep selecting DeepSeek, not the
	// default provider.
	return &serverEnv{
		vars: map[string]string{"HOME": udstest.Dir(t)},
		file: map[string]string{
			"YT2COLUMN_LLM":     "deepseek/" + testModel,
			"DEEPSEEK_API_KEY":  testAPIKey,
			"SLACK_WEBHOOK_URL": testWebhook,
		},
	}
}

// writeConfig writes the configuration file under HOME from the file's
// keys. A test calls it once, after its setup has changed the serverEnv and
// right before the run (or the child's start) reads the file. Nothing is
// written when HOME is unset or relative, since the server then stops before
// it reads the file.
func (e *serverEnv) writeConfig(t *testing.T) {
	t.Helper()
	if e.noFile {
		return
	}
	if home, ok := e.vars["HOME"]; ok && filepath.IsAbs(home) {
		config.WriteServerFileForTest(t, home, config.ServerFileForTest(e.file))
	}
}

// lookup returns a config.LookupFunc over the environment. It writes
// nothing; the configuration file comes from writeConfig.
func (e *serverEnv) lookup() config.LookupFunc {
	return func(name string) (string, bool) {
		value, ok := e.vars[name]
		return value, ok
	}
}

// socketPath returns the socket path the server derives from HOME. It does
// not call config.LoadServer, so it neither needs nor reads the
// configuration file.
func (e *serverEnv) socketPath(t *testing.T) string {
	t.Helper()
	home, ok := e.vars["HOME"]
	if !ok || !filepath.IsAbs(home) {
		t.Fatalf("no socket path for HOME %q", home)
	}
	return filepath.Join(home, config.SocketPathTailForTest(runtime.GOOS))
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
	e.writeConfig(t)
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

// logAttrName matches the name of one attribute of a slog text line.
var logAttrName = regexp.MustCompile(`(?:^| )([A-Za-z][A-Za-z0-9]*)=`)

// logAttrNames returns the sorted names of a slog text line's attributes,
// without the time, level, and msg the handler adds. It reads the lines these
// tests write, whose quoted values (the message, a path with a space) hold no
// word directly followed by "=".
func logAttrNames(line string) []string {
	var names []string
	for _, m := range logAttrName.FindAllStringSubmatch(line, -1) {
		switch m[1] {
		case "time", "level", "msg":
		default:
			names = append(names, m[1])
		}
	}
	slices.Sort(names)
	return names
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

// testDeps returns deps that build client for generation and a review client
// that revises nothing, so no test reaches the LLM API.
func testDeps(client llm.LLMClient) deps {
	return deps{
		newLLMClient:       func(config.LLMConfig) (llm.LLMClient, error) { return client, nil },
		newReviewLLMClient: func(config.LLMConfig) (llm.LLMClient, error) { return reviewLLM(), nil },
		newWriter:          writer.NewProgressWriter,
	}
}

// apiResponse is the JSON body the server answers with a rejection.
type apiResponse struct {
	Status    string `json:"status"`
	Reason    string `json:"reason"`
	RequestID string `json:"requestId"`
}

// requestBody returns a plain generation start's body for testVideoURL with
// raw as the transcript.
func requestBody(raw string) string {
	data, err := json.Marshal(map[string]string{"videoUrl": testVideoURL, "transcript": raw, "mode": "plain"})
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
// reason, a generation start's key, and a status response's job state and
// article.
type jobResponse struct {
	Status string `json:"status"`
	Reason string `json:"reason"`
	Key    string `json:"key"`
	Job    struct {
		State   string `json:"state"`
		Article *struct {
			Markdown            string  `json:"markdown"`
			MentionsNeutralized *bool   `json:"mentionsNeutralized"`
			VideoTitle          *string `json:"videoTitle"`
			ChannelName         *string `json:"channelName"`
			File                string  `json:"file"`
		} `json:"article"`
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

// startGeneration sends a generation start for requestBody(testValidRaw) to
// the socket at path, failing the test unless it is accepted, and returns
// the job's key.
func startGeneration(t *testing.T, path string) string {
	t.Helper()
	return doJobs(t, path, http.MethodPost, server.JobsPath, strings.NewReader(requestBody(testValidRaw)), http.StatusAccepted).Key
}

// blockingLLM is an LLM client whose call creates ready, then waits until
// release exists, ignoring its context as an LLM call already sent does, and
// then returns the context's error, or a valid response when the context was
// not canceled. It gives up after blockLifetime.
type blockingLLM struct {
	ready   string
	release string
}

// Generate implements llm.LLMClient.
func (b blockingLLM) Generate(ctx context.Context, _ llm.GenerateRequest) (llm.GenerateResponse, error) {
	writeMarker(b.ready)
	deadline := time.Now().Add(blockLifetime)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(b.release); err == nil {
			if err := ctx.Err(); err != nil {
				return llm.GenerateResponse{}, err
			}
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
	e.writeConfig(t)
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

// runChildToEnd runs the test binary as a childModeMain child with args and
// e's environment, waits up to exitBound for it to exit, and returns its exit
// code, standard output, and standard error. It writes no configuration file
// and waits for no socket.
func runChildToEnd(t *testing.T, e *serverEnv, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), exitBound)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], args...) //nolint:gosec // the test binary itself, re-run as the server
	cmd.Env = childEnv(childModeMain, e.vars)
	cmd.WaitDelay = childWaitDelay
	var out, errOut bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errOut
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			t.Fatalf("the child did not exit within %v", exitBound)
		}
		if _, ok := errors.AsType[*exec.ExitError](err); !ok {
			t.Fatalf("run the child: %v", err)
		}
	}
	return cmd.ProcessState.ExitCode(), out.String(), errOut.String()
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
// key.
func requireNoSecret(t *testing.T, name, output string) {
	t.Helper()
	for _, s := range []string{apiKeyMarker, anthropicMarker, openAIMarker} {
		if strings.Contains(output, s) {
			t.Errorf("%s holds a secret (%.8s...):\n%s", name, s, output)
		}
	}
}
