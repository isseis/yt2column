//go:build test

package config

// The server-relevant variable names and the token file's name, exposed so the
// documentation guard in cmd/yt2column-server derives them from the code
// instead of copying them. Built only with the test tag.

// ServerPortEnvForTest is the environment variable naming the listening port.
func ServerPortEnvForTest() string { return serverPortEnv }

// SlackWebhookEnvForTest is the environment variable naming the Webhook URL.
func SlackWebhookEnvForTest() string { return slackEnv }

// CacheDirEnvForTest is the environment variable naming the cache directory,
// which LoadServer does not read.
func CacheDirEnvForTest() string { return cacheDirEnv }

// YtDlpEnvForTest is the environment variable naming the yt-dlp path, which
// LoadServer does not read.
func YtDlpEnvForTest() string { return ytDlpEnv }

// TokenFileNameForTest is the shared token file's name under the configuration
// directory.
func TokenFileNameForTest() string { return tokenFileName }
