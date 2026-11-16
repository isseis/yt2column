//go:build js

package claude

import "net/http"

// setBrowserAccess adds the header the Messages API requires before it accepts
// a request that comes from a browser. Without it, the API refuses a call made
// from an extension page.
func setBrowserAccess(request *http.Request) {
	request.Header.Set("anthropic-dangerous-direct-browser-access", "true")
}
