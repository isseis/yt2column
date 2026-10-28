//go:build test

package main

import (
	"net/http"
	"regexp"
	"strings"
	"testing"

	"github.com/isseis/yt2column/internal/server"
)

// requestIDPattern is the form of the request ID the server generates and the
// failure notifier accepts.
var requestIDPattern = regexp.MustCompile(`^[0-9a-f]{16}$`)

// TestServerEndToEnd runs the assembled server on a real Unix socket with a
// fake LLM client and the real webhook publisher and failure notifier posting
// to a loopback webhook. It checks, in one server's lifetime: a request with
// no header beyond what net/http adds is accepted and posts the article under
// the normalized URL; and a rejected transcript yields a failure notice that
// carries the response's request ID, so the server's ID and the notifier's
// check agree; and a generation start on the job API returns a key whose
// status becomes done with the generated article. A second start with the same HOME, after a normal stop, serves
// on the same socket path.
func TestServerEndToEnd(t *testing.T) {
	webhook, webhookURL := newWebhookServer(t)
	e := newServerEnv(t)
	d := loopbackDeps(t, validResponseLLM(), webhookURL)
	socket := e.socketPath(t)

	first := startServer(t, e, d)
	accepted := mustPostArticle(t, socket, requestBody(testValidRaw), 202)
	if accepted.Status != "accepted" || !requestIDPattern.MatchString(accepted.RequestID) {
		t.Errorf("accepted response = %+v, want status accepted and a request ID", accepted)
	}
	article := webhook.waitForMessage(t, "A title", testNormalizedURL)
	if strings.Contains(article, "youtu.be") {
		t.Errorf("the article names the request's URL instead of the normalized one:\n%s", article)
	}

	rejected := mustPostArticle(t, socket, requestBody("not a copied transcript"), 400)
	if rejected.Reason != "invalid_transcript" {
		t.Errorf("rejected reason = %q, want invalid_transcript", rejected.Reason)
	}
	notice := webhook.waitForMessage(t, "yt2column failure:", "request: "+rejected.RequestID)
	if !strings.Contains(notice, "video: <"+testNormalizedURL+">") {
		t.Errorf("the notice does not name the video:\n%s", notice)
	}

	started := doJobs(t, socket, http.MethodPost, server.JobsPath, strings.NewReader(requestBody(testValidRaw)), http.StatusAccepted)
	if started.Status != "accepted" || started.Key == "" {
		t.Fatalf("generation start response = %+v, want accepted with a key", started)
	}
	status := waitJobDone(t, socket, started.Key)
	if status.Job.State != "done" || status.Job.Article == nil ||
		!strings.Contains(status.Job.Article.Markdown, "A title") || !strings.Contains(status.Job.Article.File, testNormalizedURL) {
		t.Errorf("job status = %+v, want done with the generated article", status.Job)
	}
	posted := len(webhook.messages())
	publish := doJobs(t, socket, http.MethodPost, server.JobsPath+"/"+started.Key+server.PublishSuffix, nil, http.StatusAccepted)
	if publish.Status != "accepted" {
		t.Errorf("publish response = %+v, want accepted", publish)
	}
	if state := waitPublished(t, socket, started.Key); state != "posted" {
		t.Errorf("publish state = %q, want posted", state)
	}
	if got := webhook.messages()[posted:]; len(got) != 1 || !strings.Contains(got[0], "A title") {
		t.Errorf("messages posted by the publish operation = %q, want the one article", got)
	}

	if code := first.stop(t); code != exitOK {
		t.Fatalf("first run exit code = %d, want %d\nstderr:\n%s", code, exitOK, first.stderr.String())
	}
	before := len(webhook.messages())

	second := startServer(t, e, d)
	mustPostArticle(t, socket, requestBody(testValidRaw), 202)
	// stop waits for the job, so its article has been posted when it returns.
	if code := second.stop(t); code != exitOK {
		t.Fatalf("second run exit code = %d, want %d\nstderr:\n%s", code, exitOK, second.stderr.String())
	}
	if posted := webhook.messages()[before:]; len(posted) != 1 || !strings.Contains(posted[0], "A title") {
		t.Errorf("messages posted by the second run = %q, want the one article", posted)
	}
	requireNoSecret(t, "first stderr", first.stderr.String())
	requireNoSecret(t, "second stderr", second.stderr.String())
}
