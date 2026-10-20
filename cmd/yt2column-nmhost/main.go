// Package main is the entry point of yt2column-nmhost, the Native Messaging
// host. Started by the browser with the extension's origin, it relays one
// request from the extension to the server's Unix domain socket; started by
// the user with install or uninstall, it writes or removes the browser's host
// manifest.
package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"runtime"

	"github.com/isseis/yt2column/internal/config"
	"github.com/isseis/yt2column/internal/nmhost"
)

// Exit codes. A relay that wrote its reply, host errors included, exits 0.
const (
	exitOK      = 0
	exitFailure = 1 // no reply could be written, or install/uninstall failed
	exitUsage   = 2 // the arguments are wrong, or HOME is unusable for install/uninstall
)

const usage = `usage: yt2column-nmhost install chrome|brave
       yt2column-nmhost uninstall chrome|brave
The browser starts the host with the extension's origin to relay a request.
`

// darwinSharedNote is written after the manifest path on macOS, where Chrome
// and Brave read one manifest file.
const darwinSharedNote = "On macOS, Chrome and Brave share this manifest file: install for either browser registers the host with both, and uninstall for either removes it from both.\n"

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

// run chooses the role from args by exact match and returns the exit code.
// Contract: only the relay writes to stdout, and only when args is exactly
// the extension's origin; every other invocation leaves stdout empty and
// does not read stdin. Every role reads HOME and no other variable through
// lookup.
func run(args []string, lookup config.LookupFunc, stdin io.Reader, stdout, stderr io.Writer, executable func() (string, error)) int {
	switch {
	case len(args) == 1 && args[0] == nmhost.ExtensionOrigin:
		return relay(lookup, stdin, stdout, stderr)
	case len(args) == 2 && args[0] == "install":
		return install(args[1], lookup, stderr, executable)
	case len(args) == 2 && args[0] == "uninstall":
		return uninstall(args[1], lookup, stderr)
	default:
		_, _ = fmt.Fprint(stderr, usage)
		return exitUsage
	}
}

// relay forwards one message to the server's socket.
// Contract: when config.LoadHost fails, no connection is attempted and the
// body is not read: the relay is given a Dialer that fails at once, so a
// message within the limits gets the server_unavailable reply and the exit
// code is 0, like any other host error. The cause goes to the log.
func relay(lookup config.LookupFunc, stdin io.Reader, stdout, stderr io.Writer) int {
	log := slog.New(slog.NewTextHandler(stderr, nil))
	var dial nmhost.Dialer
	cfg, err := config.LoadHost(lookup)
	if err != nil {
		log.Error("config", "error", err.Error())
		dial = func(context.Context) (net.Conn, error) { return nil, err }
	} else {
		log.Info("config", "socket", cfg.SocketPath())
		dial = nmhost.SocketDialer(cfg.SocketPath())
	}
	opts := nmhost.RelayOptions{Dial: dial, Log: stderr, Timeout: nmhost.RelayTimeout}
	if err := nmhost.Relay(context.Background(), stdin, stdout, opts); err != nil {
		return exitFailure
	}
	return exitOK
}

// install writes the manifest of the named browser, pointing at this
// executable, and reports the manifest path on stderr.
func install(name string, lookup config.LookupFunc, stderr io.Writer, executable func() (string, error)) int {
	browser, home, code := browserAndHome(name, lookup, stderr)
	if code != exitOK {
		return code
	}
	exe, err := executable()
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "yt2column-nmhost: cannot locate this executable: %v\n", err)
		return exitFailure
	}
	path, err := nmhost.Install(home, browser, exe)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "yt2column-nmhost: install failed: %v\n", err)
		return exitFailure
	}
	_, _ = fmt.Fprintf(stderr, "Installed the native messaging host manifest: %s\n", path)
	writeSharedNote(stderr)
	return exitOK
}

// uninstall removes the manifest of the named browser when this host wrote
// it, and reports the manifest path on stderr.
func uninstall(name string, lookup config.LookupFunc, stderr io.Writer) int {
	browser, home, code := browserAndHome(name, lookup, stderr)
	if code != exitOK {
		return code
	}
	path, removed, err := nmhost.Uninstall(home, browser)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "yt2column-nmhost: uninstall failed: %v\n", err)
		return exitFailure
	}
	if removed {
		_, _ = fmt.Fprintf(stderr, "Removed the native messaging host manifest: %s\n", path)
	} else {
		_, _ = fmt.Fprintf(stderr, "No native messaging host manifest to remove: %s\n", path)
	}
	writeSharedNote(stderr)
	return exitOK
}

// browserAndHome parses the browser name and loads HOME for install and
// uninstall. A failure of either is a usage error; HOME is read only after
// the name is accepted.
func browserAndHome(name string, lookup config.LookupFunc, stderr io.Writer) (nmhost.Browser, string, int) {
	browser, err := nmhost.ParseBrowser(name)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "yt2column-nmhost: %v\n%s", err, usage)
		return nmhost.BrowserUnset, "", exitUsage
	}
	home, err := config.LoadHostHome(lookup)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "yt2column-nmhost: %v\n", err)
		return nmhost.BrowserUnset, "", exitUsage
	}
	return browser, home, exitOK
}

// writeSharedNote tells the user, on macOS, that both browsers read the one
// manifest file just written or removed.
func writeSharedNote(stderr io.Writer) {
	if runtime.GOOS == "darwin" {
		_, _ = fmt.Fprint(stderr, darwinSharedNote)
	}
}
