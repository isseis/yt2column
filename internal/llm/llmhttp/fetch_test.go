//go:build test

package llmhttp

import (
	"slices"
	"testing"

	"github.com/isseis/yt2column/internal/llm/llmhttp/llmhttptest"
)

// TestEverySendGetsTheBrowserFetchOptions checks, from the source, that the
// js/wasm-only redirect option reaches every request this package sends. A
// native run cannot see it: setFetchOptions does nothing outside a js build.
// So the check reads the code: the one place a request is sent, roundTrip,
// calls setFetchOptions as an unconditional statement before the send, and both
// Post and Status send through roundTrip. What it cannot show is that the
// option reaches the request actually sent: the request variable is passed
// along by name only.
func TestEverySendGetsTheBrowserFetchOptions(t *testing.T) {
	funcs := llmhttptest.ParseDir(t, ".")

	if got, want := llmhttptest.Callers(funcs, "Do"), []string{"roundTrip"}; !slices.Equal(got, want) {
		t.Errorf("functions that send a request = %v, want only %v", got, want)
	}
	if got, want := llmhttptest.Callers(funcs, "setFetchOptions"), []string{"roundTrip"}; !slices.Equal(got, want) {
		t.Errorf("callers of setFetchOptions = %v, want only %v", got, want)
	}
	if got, want := llmhttptest.Callers(funcs, "roundTrip"), []string{"Post", "Status"}; !slices.Equal(got, want) {
		t.Errorf("callers of roundTrip = %v, want %v", got, want)
	}

	// The option must be set before the request is sent, and on every path: a
	// call made after Do, or one inside an if, would leave a request without it.
	roundTrip := llmhttptest.Named(funcs, "roundTrip")
	if len(roundTrip) != 1 {
		t.Fatalf("declarations of roundTrip = %d, want 1", len(roundTrip))
	}
	set := roundTrip[0].TopLevelCall("setFetchOptions")
	send := roundTrip[0].FirstStatementCalling("Do")
	if set < 0 || send < 0 || set >= send {
		t.Errorf("in roundTrip, setFetchOptions is statement %d and the send is statement %d (-1: absent or not a top-level call); setFetchOptions must be an unconditional statement before the send", set, send)
	}
}

// TestJSFetchOptionsRejectRedirects checks that the js build's
// setFetchOptions asks fetch to fail on a redirect. The file is read, not
// run, because only a js build compiles it.
func TestJSFetchOptionsRejectRedirects(t *testing.T) {
	var jsDecls []llmhttptest.Func
	for _, f := range llmhttptest.Named(llmhttptest.ParseDir(t, "."), "setFetchOptions") {
		if f.File == "fetch_js.go" {
			jsDecls = append(jsDecls, f)
		}
	}
	if len(jsDecls) != 1 {
		t.Fatalf("setFetchOptions declarations in fetch_js.go = %d, want 1", len(jsDecls))
	}
	if !jsDecls[0].SetsHeader("js.fetch:redirect", "error") {
		t.Error(`fetch_js.go: setFetchOptions does not set the "js.fetch:redirect" header to "error"`)
	}
}
