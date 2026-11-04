//go:build test

package main

import (
	"net/http"
	"strings"
	"testing"

	"github.com/isseis/yt2column/internal/server"
)

// TestServerEndToEnd runs the assembled server on a real Unix socket with a
// fake LLM client and the real webhook publisher posting to a loopback
// webhook. It checks, in one server's lifetime, that a generation start with
// no header beyond what net/http adds returns a key whose status becomes done
// with the generated article, and that its publish operation posts the
// article once, under the normalized URL. A second start with the same HOME,
// after a normal stop, serves on the same socket path.
func TestServerEndToEnd(t *testing.T) {
	webhook, webhookURL := newWebhookServer(t)
	e := newServerEnv(t)
	d := loopbackDeps(t, validResponseLLM(), webhookURL)
	socket := e.socketPath(t)

	first := startServer(t, e, d)
	started := doJobs(t, socket, http.MethodPost, server.JobsPath, strings.NewReader(requestBody(testValidRaw)), http.StatusAccepted)
	if started.Status != "accepted" || started.Key == "" {
		t.Fatalf("generation start response = %+v, want accepted with a key", started)
	}
	status := waitJobDone(t, socket, started.Key)
	if status.Job.State != "done" || status.Job.Article == nil ||
		!strings.Contains(status.Job.Article.Markdown, "A title") || !strings.Contains(status.Job.Article.File, testNormalizedURL) {
		t.Errorf("job status = %+v, want done with the generated article", status.Job)
	}
	publish := doJobs(t, socket, http.MethodPost, server.JobsPath+"/"+started.Key+server.PublishSuffix, nil, http.StatusAccepted)
	if publish.Status != "accepted" {
		t.Errorf("publish response = %+v, want accepted", publish)
	}
	if state := waitPublished(t, socket, started.Key); state != "posted" {
		t.Errorf("publish state = %q, want posted", state)
	}
	if got := webhook.messages(); len(got) != 1 || !containsAll(got[0], []string{"A title", testNormalizedURL}) || strings.Contains(got[0], "youtu.be") {
		t.Errorf("messages posted by the publish operation = %q, want the one article under the normalized URL", got)
	}

	if code := first.stop(t); code != exitOK {
		t.Fatalf("first run exit code = %d, want %d\nstderr:\n%s", code, exitOK, first.stderr.String())
	}
	before := len(webhook.messages())

	second := startServer(t, e, d)
	generateAndPublish(t, socket)
	// stop waits for the post job, so its article has been posted when it
	// returns.
	if code := second.stop(t); code != exitOK {
		t.Fatalf("second run exit code = %d, want %d\nstderr:\n%s", code, exitOK, second.stderr.String())
	}
	if posted := webhook.messages()[before:]; len(posted) != 1 || !strings.Contains(posted[0], "A title") {
		t.Errorf("messages posted by the second run = %q, want the one article", posted)
	}
	requireNoSecret(t, "first stderr", first.stderr.String())
	requireNoSecret(t, "second stderr", second.stderr.String())
}
