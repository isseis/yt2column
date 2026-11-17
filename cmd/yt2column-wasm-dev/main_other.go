//go:build !js

package main

import "os"

// main reports whether the embedded scenario parses, for the build script to
// run before it builds the WebAssembly with the same -ldflags.
func main() {
	os.Exit(checkEmbedded(os.Stderr))
}
