//go:build !js

package llmhttp

import "net/http"

// setFetchOptions does nothing outside a js/wasm build: net/http applies the
// no-redirect policy itself (newNoRedirectClient).
func setFetchOptions(*http.Request) {}
