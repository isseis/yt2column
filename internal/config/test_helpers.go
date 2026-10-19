//go:build test

package config

import (
	"path/filepath"
	"slices"
)

// The server-relevant variable names and the socket path rule, exposed so the
// documentation guard in cmd/yt2column-server and the internal/serversock
// tests derive them from the code instead of copying them. Built only with the
// test tag.

// SlackWebhookEnvForTest is the environment variable naming the Webhook URL.
func SlackWebhookEnvForTest() string { return slackEnv }

// CacheDirEnvForTest is the environment variable naming the cache directory,
// which LoadServer does not read.
func CacheDirEnvForTest() string { return cacheDirEnv }

// YtDlpEnvForTest is the environment variable naming the yt-dlp path, which
// LoadServer does not read.
func YtDlpEnvForTest() string { return ytDlpEnv }

// SocketPathTailForTest returns the part of the socket path below HOME for
// goos, starting with a separator. It panics for an operating system with no
// rule.
func SocketPathTailForTest(goos string) string {
	parts, _, ok := socketRule(goos)
	if !ok {
		panic("config: no socket path rule for " + goos)
	}
	return string(filepath.Separator) + filepath.Join(slices.Concat(parts, []string{socketFileName})...)
}

// SocketPathMaxForTest returns the longest socket path, in bytes, accepted for
// goos. It panics for an operating system with no rule.
func SocketPathMaxForTest(goos string) int {
	_, limit, ok := socketRule(goos)
	if !ok {
		panic("config: no socket path rule for " + goos)
	}
	return limit
}
