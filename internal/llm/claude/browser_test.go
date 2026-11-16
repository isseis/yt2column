//go:build test

package claude

import (
	"slices"
	"testing"

	"github.com/isseis/yt2column/internal/llm/llmhttp/llmhttptest"
)

// TestEveryRequestGetsTheBrowserAccessHeader checks, from the source, that the
// header a js/wasm build adds reaches every request this package builds. A
// native run cannot see it: setBrowserAccess does nothing outside a js build.
// So the check reads the code: each function that builds a request calls
// setRequestHeaders, and only setRequestHeaders calls setBrowserAccess.
func TestEveryRequestGetsTheBrowserAccessHeader(t *testing.T) {
	funcs := llmhttptest.ParseDir(t, ".")

	builders := llmhttptest.Callers(funcs, "NewRequestWithContext")
	if want := []string{"newKeyCheckRequest", "newRequest"}; !slices.Equal(builders, want) {
		t.Fatalf("functions that build a request = %v, want %v; a new one must set its headers through setRequestHeaders and be listed here", builders, want)
	}
	if got := llmhttptest.Callers(funcs, "setRequestHeaders"); !slices.Equal(got, builders) {
		t.Errorf("callers of setRequestHeaders = %v, want every request builder %v", got, builders)
	}
	if got, want := llmhttptest.Callers(funcs, "setBrowserAccess"), []string{"setRequestHeaders"}; !slices.Equal(got, want) {
		t.Errorf("callers of setBrowserAccess = %v, want only %v", got, want)
	}

	// The header is for every request, so the call must not sit inside an if
	// or a closure, where a path could skip it.
	setHeaders := llmhttptest.Named(funcs, "setRequestHeaders")
	if len(setHeaders) != 1 {
		t.Fatalf("declarations of setRequestHeaders = %d, want 1", len(setHeaders))
	}
	if setHeaders[0].TopLevelCall("setBrowserAccess") < 0 {
		t.Error("setRequestHeaders does not call setBrowserAccess as an unconditional statement")
	}
}

// TestJSBrowserAccessHeader checks that the js build's setBrowserAccess sets
// the header the Messages API requires of a browser caller. The file is read,
// not run, because only a js build compiles it.
func TestJSBrowserAccessHeader(t *testing.T) {
	var jsDecls []llmhttptest.Func
	for _, f := range llmhttptest.Named(llmhttptest.ParseDir(t, "."), "setBrowserAccess") {
		if f.File == "browser_js.go" {
			jsDecls = append(jsDecls, f)
		}
	}
	if len(jsDecls) != 1 {
		t.Fatalf("setBrowserAccess declarations in browser_js.go = %d, want 1", len(jsDecls))
	}
	if !jsDecls[0].SetsHeader("anthropic-dangerous-direct-browser-access", "true") {
		t.Error(`browser_js.go: setBrowserAccess does not set "anthropic-dangerous-direct-browser-access" to "true"`)
	}
}
