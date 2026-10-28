//go:build test

package main

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/isseis/yt2column/internal/nmhost"
	"github.com/isseis/yt2column/internal/server"
)

// hostSourceDirs are the directories, relative to this package, whose Go
// files the host binary is built from and this test binary is not. Opening
// them makes go test count them as inputs of the cached result (see go help
// test), so a change to the host reruns this test. internal/nmhost is also
// imported here; it is listed so the rule does not depend on that import.
var hostSourceDirs = []string{"../yt2column-nmhost", "../../internal/nmhost"}

// hostPackage is the host's import path, built by buildHost.
const hostPackage = "github.com/isseis/yt2column/cmd/yt2column-nmhost"

// unreachableProxy is set as HTTP_PROXY in the host's environment, as a
// user's environment may set it. The host must reach the socket regardless;
// that it ignores the variable is internal/nmhost's test.
const unreachableProxy = "http://127.0.0.1:9"

// TestRelayEndToEnd connects the real host binary to the assembled server
// over the server's Unix socket, with a fake LLM client and the real webhook
// publisher and failure notifier posting to a loopback webhook. Every host
// run is checked to exit 0 having written exactly one frame to standard
// output, and nothing else.
func TestRelayEndToEnd(t *testing.T) {
	host := buildHost(t)

	// The job outlives the host: the accepted reply arrives and the host
	// exits while the LLM call is still blocked, and the article is posted
	// only after the call is released.
	t.Run("accepted_job_continues_after_host_exits", func(t *testing.T) {
		webhook, webhookURL := newWebhookServer(t)
		e := newServerEnv(t)
		gate := gatedLLM{entered: make(chan struct{}, 1), release: make(chan struct{})}
		var once sync.Once
		release := func() { once.Do(func() { close(gate.release) }) }
		srv := startServer(t, e, loopbackDeps(t, gate, webhookURL))
		// Registered after startServer, so it runs first and a failing test
		// does not leave the server's stop waiting on the blocked job.
		t.Cleanup(release)

		reply := runHost(t, host, e, requestBody(testValidRaw))
		accepted := requireServerReply(t, reply, 202)
		if accepted.Status != "accepted" || !requestIDPattern.MatchString(accepted.RequestID) {
			t.Errorf("accepted response = %+v, want status accepted and a request ID", accepted)
		}
		select {
		case <-gate.entered:
		case <-time.After(readyBound):
			t.Fatalf("the job's LLM call did not start within %v", readyBound)
		}
		release()
		webhook.waitForMessage(t, "A title", testNormalizedURL)
		// stop waits for the job, so a second article would be posted by now.
		if code := srv.stop(t); code != exitOK {
			t.Fatalf("run exit code = %d, want %d\nstderr:\n%s", code, exitOK, srv.stderr.String())
		}
		if got := webhook.messages(); len(got) != 1 {
			t.Errorf("messages = %q, want the one article", got)
		}
	})

	t.Run("invalid_transcript_notifies_failure", func(t *testing.T) {
		webhook, webhookURL := newWebhookServer(t)
		e := newServerEnv(t)
		startServer(t, e, loopbackDeps(t, validResponseLLM(), webhookURL))

		reply := runHost(t, host, e, requestBody("not a copied transcript"))
		rejected := requireServerReply(t, reply, 400)
		if rejected.Status != "rejected" || rejected.Reason != "invalid_transcript" || !requestIDPattern.MatchString(rejected.RequestID) {
			t.Fatalf("rejected response = %+v, want status rejected, reason invalid_transcript, and a request ID", rejected)
		}
		webhook.waitForMessage(t, "yt2column failure:", "request: "+rejected.RequestID)
	})

	t.Run("stopped_server_is_unavailable", func(t *testing.T) {
		e := newServerEnv(t)
		srv := startServer(t, e, testDeps(validResponseLLM()))
		if code := srv.stop(t); code != exitOK {
			t.Fatalf("run exit code = %d, want %d\nstderr:\n%s", code, exitOK, srv.stderr.String())
		}

		reply := runHost(t, host, e, requestBody(testValidRaw))
		if reply.Kind != "error" || reply.Error != "server_unavailable" {
			t.Fatalf("reply = %+v, want the server_unavailable error", reply)
		}
	})

	// The new form's three operations go through the real host: a
	// generation start, a status request, and a publish operation. A key
	// that would turn a status request into a publish operation if the host
	// embedded it unescaped, or that would point outside the key's segment, is
	// rejected as invalid_key by both operations, and publishes nothing.
	t.Run("new_form_operations", func(t *testing.T) {
		webhook, webhookURL := newWebhookServer(t)
		e := newServerEnv(t)
		startServer(t, e, loopbackDeps(t, validResponseLLM(), webhookURL))
		socket := e.socketPath(t)

		started := requireJobReply(t, runHost(t, host, e, `{"v":1,"op":"start","input":`+requestBody(testValidRaw)+`}`), 202)
		if started.Status != "accepted" || started.Key == "" {
			t.Fatalf("generation start response = %+v, want accepted with a key", started)
		}
		waitJobDone(t, socket, started.Key)
		status := requireJobReply(t, runHost(t, host, e, newFormKeyMessage(t, "status", started.Key)), 200)
		if status.Job.State != "done" || status.Job.Publish == nil || status.Job.Publish.State != "not_posted" {
			t.Fatalf("job status = %+v, want done and not posted", status.Job)
		}

		injected := []string{
			started.Key + "/publish", started.Key + "?x=1", "../articles", "../" + started.Key,
			started.Key + "/x", "%2F", "\u3042",
		}
		for _, op := range []string{"status", "publish"} {
			for _, key := range injected {
				if rejected := requireServerReply(t, runHost(t, host, e, newFormKeyMessage(t, op, key)), 400); rejected.Reason != "invalid_key" {
					t.Errorf("%s of %q = %+v, want reason invalid_key", op, key, rejected)
				}
			}
		}
		if after := doJobs(t, socket, http.MethodGet, server.JobsPath+"/"+started.Key, nil, http.StatusOK); after.Job.Publish == nil || after.Job.Publish.State != "not_posted" {
			t.Fatalf("job after the injected keys = %+v, want still not posted", after.Job)
		}
		if got := webhook.messages(); len(got) != 0 {
			t.Fatalf("messages after the injected keys = %q, want none", got)
		}

		published := requireJobReply(t, runHost(t, host, e, newFormKeyMessage(t, "publish", started.Key)), 202)
		if published.Status != "accepted" {
			t.Errorf("publish response = %+v, want accepted", published)
		}
		if state := waitPublished(t, socket, started.Key); state != "posted" {
			t.Errorf("publish state = %q, want posted", state)
		}
		if got := webhook.messages(); len(got) != 1 || !strings.Contains(got[0], "A title") {
			t.Errorf("messages = %q, want the one article", got)
		}
	})

	// The body is over the server's 4 MiB limit and far below the host's
	// 32 MiB limit, so the host relays it and the server rejects it.
	t.Run("body_over_server_limit_is_rejected_by_server", func(t *testing.T) {
		e := newServerEnv(t)
		startServer(t, e, testDeps(validResponseLLM()))

		body := requestBody(testValidRaw + strings.Repeat("x", server.MaxBodyBytesForTest))
		reply := runHost(t, host, e, body)
		if rejected := requireServerReply(t, reply, 413); rejected.Status != "rejected" || rejected.Reason != "body_too_large" {
			t.Fatalf("rejected response = %+v, want status rejected and reason body_too_large", rejected)
		}
	})
}

// buildHost builds the host binary into a temporary directory and returns its
// path. GOOS and GOARCH are pinned to this process's and GOFLAGS is cleared,
// so the developer's settings cannot build a binary this machine cannot run;
// the rest of the environment is inherited because go build needs HOME,
// GOCACHE, and the like.
func buildHost(t *testing.T) string {
	t.Helper()
	for _, dir := range hostSourceDirs {
		openGoFiles(t, dir)
	}
	goBin, err := exec.LookPath("go")
	if err != nil {
		t.Fatalf("find the go command: %v", err)
	}
	out := filepath.Join(t.TempDir(), "yt2column-nmhost")
	cmd := exec.Command(goBin, "build", "-o", out, hostPackage) //nolint:gosec // the go command, building this module's host
	cmd.Env = append(os.Environ(), "GOOS="+runtime.GOOS, "GOARCH="+runtime.GOARCH, "GOFLAGS=")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build the host: %v\n%s", err, output)
	}
	return out
}

// openGoFiles opens and closes every Go file in dir, so go test records them
// as inputs of this test's cached result.
func openGoFiles(t *testing.T, dir string) {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join(dir, "*.go"))
	if err != nil || len(paths) == 0 {
		t.Fatalf("list the Go files in %s: %v (found %d)", dir, err, len(paths))
	}
	for _, path := range paths {
		f, err := os.Open(path) //nolint:gosec // a source file of this module
		if err != nil {
			t.Fatalf("open %s: %v", path, err)
		}
		_ = f.Close()
	}
}

// hostReply is either shape of the host's reply.
type hostReply struct {
	V      int    `json:"v"`
	Kind   string `json:"kind"`
	Status int    `json:"status"`
	Body   string `json:"body"`
	Error  string `json:"error"`
}

// runHost starts the host as the browser does, with the extension's origin as
// its argument and a minimal environment, writes message to its standard
// input as one Native Messaging frame, and returns the decoded reply. It
// fails the test unless the host exits 0 within exitBound and its standard
// output is exactly one frame. A cleanup that kills and reaps the host is
// registered as soon as it starts.
func runHost(t *testing.T, host string, e *serverEnv, message string) hostReply {
	t.Helper()
	frame := make([]byte, 4+len(message))
	binary.NativeEndian.PutUint32(frame, uint32(len(message))) //nolint:gosec // every message here is far below 4 GiB
	copy(frame[4:], message)

	cmd := exec.Command(host, nmhost.ExtensionOrigin) //nolint:gosec // the host binary buildHost built
	cmd.Env = []string{"HOME=" + e.vars["HOME"], "HTTP_PROXY=" + unreachableProxy}
	cmd.Stdin = bytes.NewReader(frame)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.WaitDelay = childWaitDelay
	if err := cmd.Start(); err != nil {
		t.Fatalf("start the host: %v", err)
	}
	done := make(chan struct{})
	go func() {
		_ = cmd.Wait()
		close(done)
	}()
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		<-done
	})
	select {
	case <-done:
	case <-time.After(exitBound):
		t.Fatalf("the host did not exit within %v", exitBound)
	}
	if code := cmd.ProcessState.ExitCode(); code != exitOK {
		t.Fatalf("host exit code = %d, want %d\nstderr:\n%s", code, exitOK, stderr.String())
	}
	return decodeOneFrame(t, stdout.Bytes(), stderr.String())
}

// decodeOneFrame decodes out as exactly one Native Messaging frame holding a
// host reply, failing the test on a short frame, bytes after the frame, a
// second JSON value in it, or an unknown member. The exact member set of each
// shape is internal/nmhost's test.
func decodeOneFrame(t *testing.T, out []byte, stderr string) hostReply {
	t.Helper()
	if len(out) < 4 {
		t.Fatalf("host stdout = %q, want one frame\nstderr:\n%s", out, stderr)
	}
	length := binary.NativeEndian.Uint32(out)
	if payload := out[4:]; uint64(len(payload)) != uint64(length) {
		t.Fatalf("host stdout holds %d bytes after a length of %d, want exactly one frame: %q", len(payload), length, out)
	}
	dec := json.NewDecoder(bytes.NewReader(out[4:]))
	dec.DisallowUnknownFields()
	var reply hostReply
	if err := dec.Decode(&reply); err != nil {
		t.Fatalf("decode the host reply %q: %v", out[4:], err)
	}
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		t.Fatalf("the host reply %q holds more than one JSON value", out[4:])
	}
	if reply.V != 1 {
		t.Fatalf("reply = %+v, want version 1", reply)
	}
	return reply
}

// newFormKeyMessage returns a new-form message for op (status or publish)
// naming key.
func newFormKeyMessage(t *testing.T, op, key string) string {
	t.Helper()
	data, err := json.Marshal(map[string]any{"v": 1, "op": op, "key": key})
	if err != nil {
		t.Fatalf("marshal the message: %v", err)
	}
	return string(data)
}

// requireJobReply fails the test unless reply relays a server response with
// status want, and returns the decoded job API response.
func requireJobReply(t *testing.T, reply hostReply, want int) jobResponse {
	t.Helper()
	if reply.Kind != "response" || reply.Status != want {
		t.Fatalf("reply = %+v, want a relayed server response with status %d", reply, want)
	}
	body, err := base64.StdEncoding.DecodeString(reply.Body)
	if err != nil {
		t.Fatalf("decode the relayed body %q: %v", reply.Body, err)
	}
	var resp jobResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("decode the server response %q: %v", body, err)
	}
	return resp
}

// TestStatusResponseFitsRelay checks that the host relays every status
// response whole: the server's status response limit is within the host's
// response limit, and the host's reply for a response at its limit fits the
// browser's message limit.
func TestStatusResponseFitsRelay(t *testing.T) {
	if server.MaxStatusResponseBytesForTest > nmhost.MaxResponseBytesForTest {
		t.Errorf("the server's status response limit %d exceeds the host's response limit %d",
			server.MaxStatusResponseBytesForTest, nmhost.MaxResponseBytesForTest)
	}
	if reply, limit := nmhost.LargestReplyForTest(t); reply > limit {
		t.Errorf("the host's largest reply is %d bytes, over the browser's message limit of %d", reply, limit)
	}
}

// requireServerReply fails the test unless reply relays a server response
// with status want, and returns the decoded server response.
func requireServerReply(t *testing.T, reply hostReply, want int) apiResponse {
	t.Helper()
	if reply.Kind != "response" || reply.Status != want {
		t.Fatalf("reply = %+v, want a relayed server response with status %d", reply, want)
	}
	body, err := base64.StdEncoding.DecodeString(reply.Body)
	if err != nil {
		t.Fatalf("decode the relayed body %q: %v", reply.Body, err)
	}
	var resp apiResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("decode the server response %q: %v", body, err)
	}
	return resp
}
