//go:build test

// Package loopbacktest checks that a test endpoint is a loopback URL, so a
// test helper that redirects a client cannot be pointed at an external host.
// Built only with the test tag.
package loopbacktest

import (
	"errors"
	"fmt"
	"net"
	"net/url"
)

// Static errors of the test helper.
var (
	errTestEndpointNoHost      = errors.New("test endpoint has no host")
	errTestEndpointNotLoopback = errors.New("test endpoint is not a loopback address")
)

// ValidateURL returns an error unless net/url parses endpoint, its host is
// not empty, and the host is a loopback IP address.
func ValidateURL(endpoint string) error {
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return err
	}
	if parsed.Host == "" {
		return fmt.Errorf("%w: %q", errTestEndpointNoHost, endpoint)
	}
	ip := net.ParseIP(parsed.Hostname())
	if ip == nil || !ip.IsLoopback() {
		return fmt.Errorf("%w: %q", errTestEndpointNotLoopback, endpoint)
	}
	return nil
}
