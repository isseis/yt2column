//go:build js

package llmhttp

import "net/http"

// setFetchOptions asks the browser's fetch, which Go's net/http uses in a
// js/wasm build, to treat a redirect as a send failure. Go's own redirect
// policy does not apply there: fetch follows redirects before Go sees the
// response. "error" is used rather than "manual" because a manual redirect
// comes back as an opaque status-0 response that is hard to tell from an HTTP
// failure.
func setFetchOptions(request *http.Request) {
	request.Header.Set("js.fetch:redirect", "error")
}
