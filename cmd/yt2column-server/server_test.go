//go:build test

package main

import (
	"bytes"
	"regexp"
	"strings"
	"testing"

	"github.com/isseis/yt2column/internal/server"
)

// requestIDPattern is the form of the request ID the server generates and the
// failure notifier accepts.
var requestIDPattern = regexp.MustCompile(`^[0-9a-f]{16}$`)

// TestServerEndToEnd runs the assembled server on real sockets with a fake LLM
// client and the real webhook publisher and failure notifier posting to a
// loopback webhook. It checks, in one server's lifetime: an accepted request
// posts the article under the normalized URL; a rejected transcript yields a
// failure notice that carries the response's request ID, so the server's ID
// and the notifier's check agree; and an oversized body with a wrong token is
// refused as unauthorized or by a cut connection, never as too large, and is
// not notified. A second start with the same HOME accepts the first token.
// How each start displays the token is TestTokenShownOnEveryStart's.
func TestServerEndToEnd(t *testing.T) {
	webhook, webhookURL := newWebhookServer(t)
	e := newServerEnv(t)
	d := loopbackDeps(t, validResponseLLM(), webhookURL)

	first := startServer(t, e, d)
	token := e.readToken(t)
	port := e.port(t)

	accepted := mustPostArticle(t, port, token, requestBody(testValidRaw), 202)
	if accepted.Status != "accepted" || !requestIDPattern.MatchString(accepted.RequestID) {
		t.Errorf("accepted response = %+v, want status accepted and a request ID", accepted)
	}
	article := webhook.waitForMessage(t, "A title", testNormalizedURL)
	if strings.Contains(article, "youtu.be") {
		t.Errorf("the article names the request's URL instead of the normalized one:\n%s", article)
	}

	rejected := mustPostArticle(t, port, token, requestBody("not a copied transcript"), 400)
	if rejected.Reason != "invalid_transcript" {
		t.Errorf("rejected reason = %q, want invalid_transcript", rejected.Reason)
	}
	notice := webhook.waitForMessage(t, "yt2column failure:", "request: "+rejected.RequestID)
	if !strings.Contains(notice, "video: <"+testNormalizedURL+">") {
		t.Errorf("the notice does not name the video:\n%s", notice)
	}

	// The body is refused before it is read, so the sender sees 401 or, when
	// the server closes the connection while the body is still being sent,
	// a cut connection. Either is recorded; 413 would mean the body was read.
	before := len(webhook.messages())
	oversized := bytes.Repeat([]byte("a"), server.MaxBodyBytesForTest+1)
	status, resp, err := postArticle(port, strings.Repeat("0", len(token)), bytes.NewReader(oversized))
	switch {
	case err != nil && status == 0:
		t.Logf("oversized body with a wrong token: the connection was cut (%v)", err)
	case status == 401:
		t.Logf("oversized body with a wrong token: 401 %+v", resp)
	default:
		t.Errorf("oversized body with a wrong token: status %d, error %v; want 401 or a cut connection", status, err)
	}

	if code := first.stop(t); code != exitOK {
		t.Fatalf("first run exit code = %d, want %d\nstderr:\n%s", code, exitOK, first.stderr.String())
	}
	if got := len(webhook.messages()); got != before {
		t.Errorf("webhook messages after the unauthorized request = %d, want %d (no notice)", got, before)
	}

	second := startServer(t, e, d)
	mustPostArticle(t, e.port(t), token, requestBody(testValidRaw), 202)
	// stop waits for the job, so its article has been posted when it returns.
	if code := second.stop(t); code != exitOK {
		t.Fatalf("second run exit code = %d, want %d\nstderr:\n%s", code, exitOK, second.stderr.String())
	}
	if posted := webhook.messages()[before:]; len(posted) != 1 || !strings.Contains(posted[0], "A title") {
		t.Errorf("messages posted by the second run = %q, want the one article", posted)
	}
	requireNoSecret(t, "first stderr", first.stderr.String(), token)
	requireNoSecret(t, "second stderr", second.stderr.String(), token)
}
