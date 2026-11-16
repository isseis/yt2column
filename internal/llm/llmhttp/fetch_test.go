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
// calls setFetchOptions, and both Post and Status send through roundTrip.
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
