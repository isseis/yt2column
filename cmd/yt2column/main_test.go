//go:build test

package main

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"testing"
)

// productionEndpoint is the DeepSeek URL a request from the real client would
// go to; TestMain proves the proxy setting covers it.
const productionEndpoint = "https://api.deepseek.com/chat/completions"

// TestMain first checks whether this process is a CLI child started by a
// signal test, and if so runs the CLI instead of the tests. Otherwise it
// points the proxy variables at a loopback address with no listener, so a test
// that reaches the real DeepSeek client by mistake fails without touching the
// network, and confirms the setting takes effect before running the tests.
func TestMain(m *testing.M) {
	if mode, ok := os.LookupEnv(childModeEnv); ok {
		os.Exit(runChildMode(mode))
	}

	proxyURL, err := closedLoopbackURL()
	if err != nil {
		fmt.Fprintf(os.Stderr, "reserve a loopback address: %v\n", err)
		os.Exit(1)
	}
	for _, name := range []string{"HTTPS_PROXY", "https_proxy", "HTTP_PROXY", "http_proxy"} {
		if err := os.Setenv(name, proxyURL); err != nil {
			fmt.Fprintf(os.Stderr, "set %s: %v\n", name, err)
			os.Exit(1)
		}
	}
	for _, name := range []string{"NO_PROXY", "no_proxy"} {
		if err := os.Unsetenv(name); err != nil {
			fmt.Fprintf(os.Stderr, "unset %s: %v\n", name, err)
			os.Exit(1)
		}
	}
	if err := confirmProxy(proxyURL); err != nil {
		fmt.Fprintf(os.Stderr, "proxy setting not in effect: %v\n", err)
		os.Exit(1)
	}
	os.Exit(m.Run())
}

// closedLoopbackURL returns the URL of a loopback address whose listener was
// opened and closed, so a connection to it is refused.
func closedLoopbackURL() (string, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", err
	}
	addr := listener.Addr().String()
	if err := listener.Close(); err != nil {
		return "", err
	}
	return "http://" + addr, nil
}

// confirmProxy checks that http.ProxyFromEnvironment sends a request for the
// production endpoint to want.
func confirmProxy(want string) error {
	req, err := http.NewRequest(http.MethodPost, productionEndpoint, http.NoBody)
	if err != nil {
		return err
	}
	got, err := http.ProxyFromEnvironment(req)
	if err != nil {
		return err
	}
	if got == nil || got.String() != want {
		return fmt.Errorf("proxy for %s is %v, want %s", productionEndpoint, got, want)
	}
	return nil
}
