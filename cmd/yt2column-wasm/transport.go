//go:build js && wasm

package main

import (
	"net/http"
	"net/url"
	"sync"
)

// browserAccessHeaderName is the header Anthropic requires before it answers
// a request sent directly from a browser.
const browserAccessHeaderName = "anthropic-dangerous-direct-browser-access"

// browserTransport wraps http.DefaultTransport, which the Claude adapter and
// llmhttp use because their client leaves Transport nil. It adds the browser
// access header only while a call that asked for it runs, and records the URL
// of the last response, which the js transport updates when fetch followed a
// redirect.
//
// Contract: the header is added only between setCall(true) and the next
// setCall; a probe call always runs with it off, so the header never reaches
// the probe server. Only one call runs at a time (busy in main.go).
type browserTransport struct {
	base http.RoundTripper

	mu           sync.Mutex
	addHeader    bool
	lastResponse *url.URL
}

// setCall starts a call: it chooses whether the header is added and forgets
// the previous call's response URL.
func (t *browserTransport) setCall(addHeader bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.addHeader = addHeader
	t.lastResponse = nil
}

// lastResponseURL returns the URL of the last response of this call, or nil
// when no response arrived.
func (t *browserTransport) lastResponseURL() *url.URL {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.lastResponse
}

func (t *browserTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	t.mu.Lock()
	addHeader := t.addHeader
	t.mu.Unlock()
	if addHeader {
		// RoundTrip must not modify the caller's request.
		req = req.Clone(req.Context())
		req.Header.Set(browserAccessHeaderName, "true")
	}
	response, err := t.base.RoundTrip(req)
	if err != nil {
		return nil, err
	}
	if response.Request != nil && response.Request.URL != nil {
		final := *response.Request.URL
		t.mu.Lock()
		t.lastResponse = &final
		t.mu.Unlock()
	}
	return response, nil
}
