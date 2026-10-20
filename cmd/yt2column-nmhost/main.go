// Package main is the entry point of yt2column-nmhost, the Native Messaging
// host. Started by the browser with the extension's origin, it relays one
// request from the extension to the server's Unix domain socket; started by
// the user with install or uninstall, it writes or removes the browser's host
// manifest.
package main

import (
	"os"
	"path/filepath"
)

// main wires the process boundary to run. os.LookupEnv, the standard
// streams, and the executable lookup are passed here and nowhere else, so
// this file is the only host code that touches standard output and
// internal/config is the only code that reads a variable's value.
func main() {
	os.Exit(run(os.Args[1:], os.LookupEnv, os.Stdin, os.Stdout, os.Stderr, executablePath))
}

// executablePath returns the absolute path of this binary with symbolic
// links resolved, which the manifest's "path" names.
func executablePath() (string, error) {
	path, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(path)
}
