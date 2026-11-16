//go:build !js

package claude

import "net/http"

// setBrowserAccess does nothing outside a js/wasm build: only a request made
// by a browser needs the header.
func setBrowserAccess(*http.Request) {}
