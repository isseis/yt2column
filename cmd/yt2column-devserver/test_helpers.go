//go:build test

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/isseis/yt2column/internal/config"
	"github.com/isseis/yt2column/internal/llm/devfake"
	"github.com/isseis/yt2column/internal/server"
	"github.com/isseis/yt2column/internal/udstest"
)

const (
	// readyBound bounds a wait for the server to listen, to log a line, or
	// for a run to return. It is far longer than any step takes, so a slow
	// race-enabled run does not fail by timing.
	readyBound = 30 * time.Second
	// pollInterval paces the bounded polling loops.
	pollInterval = 20 * time.Millisecond
	// loopbackTimeout bounds one request to the server.
	loopbackTimeout = 10 * time.Second
	// arrivalBuffer is how many arrivals a waitRecorder buffers: more than any
	// test's jobs make calls, so a call never blocks on recording its arrival.
	arrivalBuffer = 64
	// testRaw is a copied transcript the parser accepts.
	testRaw = "0:00\nHello\n"
)

// testEnv is the process environment of one run: HOME in a fresh short
// directory (so the socket path stays within the macOS limit and no two tests
// share it), any decoy variables a test adds, and a record of every name the
// run looked up.
type testEnv struct {
	home string
	vars map[string]string

	mu     sync.Mutex
	looked []string
}

// newTestEnv returns an environment whose only variable is HOME.
func newTestEnv(t *testing.T) *testEnv {
	t.Helper()
	home := udstest.Dir(t)
	return &testEnv{home: home, vars: map[string]string{"HOME": home}}
}

// lookup is the config.LookupFunc a run reads the environment with.
func (e *testEnv) lookup() config.LookupFunc {
	return func(name string) (string, bool) {
		e.mu.Lock()
		defer e.mu.Unlock()
		e.looked = append(e.looked, name)
		v, ok := e.vars[name]
		return v, ok
	}
}

// lookedUp returns the sorted names the run looked up, without repeats.
func (e *testEnv) lookedUp() []string {
	e.mu.Lock()
	defer e.mu.Unlock()
	names := slices.Clone(e.looked)
	slices.Sort(names)
	return slices.Compact(names)
}

// socketPath returns the socket path derived from HOME.
func (e *testEnv) socketPath() string {
	return filepath.Join(e.home, config.SocketPathTailForTest(runtime.GOOS))
}

// syncBuffer is a buffer a run writes and a test reads at the same time.
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

// runningServer is a run started by startServer.
type runningServer struct {
	env    *testEnv
	cancel context.CancelFunc
	done   chan struct{}
	code   int
	stderr *syncBuffer
}

// startServer calls run with args in a goroutine and returns once the server
// accepts connections on its socket. A cleanup stops the run.
func startServer(t *testing.T, env *testEnv, d deps, args ...string) *runningServer {
	t.Helper()
	stderr := &syncBuffer{}
	ctx, cancel := context.WithCancel(context.Background())
	r := &runningServer{env: env, cancel: cancel, done: make(chan struct{}), stderr: stderr}
	lookup := env.lookup()
	go func() {
		defer close(r.done)
		r.code = run(ctx, args, lookup, stderr, d)
	}()
	t.Cleanup(func() { r.stop(t) })
	deadline := time.Now().Add(readyBound)
	for time.Now().Before(deadline) {
		select {
		case <-r.done:
			t.Fatalf("run returned %d before it listened\nstderr:\n%s", r.code, stderr.String())
		default:
		}
		if conn, err := net.DialTimeout("unix", env.socketPath(), time.Second); err == nil {
			_ = conn.Close()
			return r
		}
		time.Sleep(pollInterval)
	}
	t.Fatalf("the server did not listen within %v\nstderr:\n%s", readyBound, stderr.String())
	return nil
}

// stop cancels the run, waits for it to return, and returns its exit code. It
// is safe to call more than once.
func (r *runningServer) stop(t *testing.T) int {
	t.Helper()
	r.cancel()
	select {
	case <-r.done:
		return r.code
	case <-time.After(readyBound):
		t.Fatalf("run did not return within %v of the cancellation", readyBound)
		return -1
	}
}

// waitStderr waits for the run's standard error to hold a line that contains
// sub, and returns that line.
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

// stderrLine returns the first line of output that contains sub, or "".
func stderrLine(output, sub string) string {
	for line := range strings.SplitSeq(output, "\n") {
		if strings.Contains(line, sub) {
			return line
		}
	}
	return ""
}

// runToEnd calls run with args and an environment of its own and returns the
// exit code and the standard error. run is expected to return without serving.
func runToEnd(t *testing.T, env *testEnv, d deps, args ...string) (code int, stderr string) {
	t.Helper()
	var buf syncBuffer
	done := make(chan int, 1)
	go func() { done <- run(t.Context(), args, env.lookup(), &buf, d) }()
	select {
	case code = <-done:
		return code, buf.String()
	case <-time.After(readyBound):
		t.Fatalf("run did not return within %v\nstderr:\n%s", readyBound, buf.String())
		return 0, ""
	}
}

// jobResponse is the part of a job API response the tests read.
type jobResponse struct {
	Status string `json:"status"`
	Reason string `json:"reason"`
	Key    string `json:"key"`
	Job    struct {
		State   string `json:"state"`
		Failure string `json:"failure"`
		Steps   *struct {
			Draft   string `json:"draft"`
			Shorten string `json:"shorten"`
			Review  string `json:"review"`
		} `json:"steps"`
		Article *struct {
			Markdown   string `json:"markdown"`
			VideoTitle string `json:"videoTitle"`
			Cost       struct {
				State string `json:"state"`
			} `json:"cost"`
		} `json:"article"`
	} `json:"job"`
}

// requestBody returns a plain generation start's body for the video with the
// given 11-character ID. Different IDs make different jobs: the server folds a
// repeated input into one record.
func requestBody(videoID string) string {
	data, err := json.Marshal(map[string]string{
		"videoUrl":   "https://www.youtube.com/watch?v=" + videoID,
		"transcript": testRaw,
		"mode":       "plain",
	})
	if err != nil {
		panic(err) // a map of strings always marshals
	}
	return string(data)
}

// doJobs sends method to target on the socket at path with body (nil for
// none) and decodes the response, failing the test unless its status is want.
func doJobs(t *testing.T, path, method, target string, body io.Reader, want int) jobResponse {
	t.Helper()
	req, err := http.NewRequest(method, "http://yt2column"+target, body)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	client := &http.Client{Transport: udstest.Transport(path), Timeout: loopbackTimeout}
	resp, err := client.Do(req)
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

// startGeneration starts a generation of the video and returns the job's key.
func startGeneration(t *testing.T, path, videoID string) string {
	t.Helper()
	return doJobs(t, path, http.MethodPost, server.JobsPath, strings.NewReader(requestBody(videoID)), http.StatusAccepted).Key
}

// waitJobDone polls the status of key until its job is no longer running and
// returns that response.
func waitJobDone(t *testing.T, path, key string) jobResponse {
	t.Helper()
	deadline := time.Now().Add(readyBound)
	for {
		resp := doJobs(t, path, http.MethodGet, server.JobsPath+"/"+key, nil, http.StatusOK)
		if resp.Job.State != "running" {
			return resp
		}
		if time.Now().After(deadline) {
			t.Fatalf("job %s still running after %v", key, readyBound)
		}
		time.Sleep(pollInterval)
	}
}

// waitRecorder replaces every call's wait. It records each call's delay, lets
// a test hold calls inside the wait, and never waits in real time.
type waitRecorder struct {
	mu      sync.Mutex
	delays  []time.Duration
	arrived chan struct{}
	// release, when non-nil, holds every call until it is closed or the
	// call's context is done.
	release chan struct{}
}

// newWaitRecorder returns a recorder that does not hold calls.
func newWaitRecorder() *waitRecorder {
	return &waitRecorder{arrived: make(chan struct{}, arrivalBuffer)}
}

// wait is the replacement wait function.
func (w *waitRecorder) wait(ctx context.Context, delay time.Duration) {
	w.mu.Lock()
	w.delays = append(w.delays, delay)
	release := w.release
	w.mu.Unlock()
	w.arrived <- struct{}{}
	if release != nil {
		select {
		case <-release:
		case <-ctx.Done():
		}
	}
}

// calls returns how many calls have reached the wait.
func (w *waitRecorder) calls() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return len(w.delays)
}

// recorded returns the delays the calls waited, in call order.
func (w *waitRecorder) recorded() []time.Duration {
	w.mu.Lock()
	defer w.mu.Unlock()
	return slices.Clone(w.delays)
}

// waitCalls waits until n calls have reached the wait since the recorder was
// made, counting from the arrivals the test has not consumed.
func (w *waitRecorder) waitCalls(t *testing.T, n int) {
	t.Helper()
	for range n {
		select {
		case <-w.arrived:
		case <-time.After(readyBound):
			t.Fatalf("fewer than %d calls reached the wait within %v", n, readyBound)
		}
	}
}

// deps returns production deps whose scenario waits with the recorder.
func (w *waitRecorder) deps() deps {
	d := productionDeps()
	d.scenario = func(s devfake.Scenario) devfake.Scenario { return devfake.WithWaitForTest(s, w.wait) }
	return d
}

// describe names a combination of the three selections for a subtest.
func describe(article devfake.Article, failure devfake.Failure, failAt devfake.FailAt) string {
	return fmt.Sprintf("%s/%s/%s", article, failure, failAt)
}
