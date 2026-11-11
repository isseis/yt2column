//go:build test

package main

import (
	"net/http"
	"strings"
	"testing"

	"github.com/isseis/yt2column/internal/server"
)

// TestServerEndToEnd runs the assembled server on a real Unix socket with a
// fake LLM client. It checks, in one server's lifetime, that a generation
// start with no header beyond what net/http adds returns a key whose status
// becomes done with the generated article under the normalized URL. A second
// start with the same HOME,
// after a normal stop, serves a generation start and its status on the same
// socket path.
func TestServerEndToEnd(t *testing.T) {
	e := newServerEnv(t)
	d := testDeps(validResponseLLM())
	socket := e.socketPath(t)

	first := startServer(t, e, d)
	started := doJobs(t, socket, http.MethodPost, server.JobsPath, strings.NewReader(requestBody(testValidRaw)), http.StatusAccepted)
	if started.Status != "accepted" || started.Key == "" {
		t.Fatalf("generation start response = %+v, want accepted with a key", started)
	}
	status := waitJobDone(t, socket, started.Key)
	if status.Job.State != "done" || status.Job.Article == nil ||
		!strings.Contains(status.Job.Article.Markdown, "A title") || !strings.Contains(status.Job.Article.File, testNormalizedURL) ||
		strings.Contains(status.Job.Article.File, "youtu.be") {
		t.Errorf("job status = %+v, want done with the generated article under the normalized URL only", status.Job)
	}

	if code := first.stop(t); code != exitOK {
		t.Fatalf("first run exit code = %d, want %d\nstderr:\n%s", code, exitOK, first.stderr.String())
	}

	second := startServer(t, e, d)
	again := waitJobDone(t, socket, startGeneration(t, socket))
	if again.Job.State != "done" || again.Job.Article == nil || !strings.Contains(again.Job.Article.Markdown, "A title") {
		t.Errorf("second run's job status = %+v, want done with the generated article", again.Job)
	}
	if code := second.stop(t); code != exitOK {
		t.Fatalf("second run exit code = %d, want %d\nstderr:\n%s", code, exitOK, second.stderr.String())
	}
	requireNoSecret(t, "first stderr", first.stderr.String())
	requireNoSecret(t, "second stderr", second.stderr.String())
}
