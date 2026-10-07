// Package slackwebhook holds the rule a Slack-compatible Incoming Webhook URL
// (Mattermost or Slack) must meet and names the parts of a URL that must never
// appear in output. It is shared by internal/config, internal/publisher,
// cmd/yt2column, and the integration tests, so each rule is written once.
package slackwebhook

import (
	"net/url"
	"strings"
)

// exposedTail is how many trailing characters of a URL are sensitive on their
// own, matching the CLI's redaction of secret values.
const exposedTail = 8

// ValidURL reports whether value is an acceptable Webhook URL: net/url parses
// it (which rejects control characters), its scheme is "https", and its host
// is not empty. A port-only authority such as "https://:443/x" is rejected
// because url.URL.Hostname returns an empty host for it.
func ValidURL(value string) bool {
	parsed, err := url.Parse(value)
	if err != nil {
		return false
	}
	return parsed.Scheme == "https" && parsed.Hostname() != ""
}

// SensitiveParts returns the strings derived from a webhook URL that must
// never appear in an error or a log: the URL, its path, its query, and its
// userinfo, each both as given and as net/url escapes it; the last path
// segment; and the last 8 characters of the URL, taken by character and by
// byte. Parts shorter than 8 bytes, other than those last 8 characters, are
// omitted, so a trivial path such as "/" never becomes a part.
func SensitiveParts(value string) []string {
	if value == "" {
		return nil
	}
	seen := map[string]bool{}
	var parts []string
	// addTail keeps any non-empty part, so the last 8 characters survive even
	// when they are shorter than 8 bytes.
	addTail := func(part string) {
		if part == "" || seen[part] {
			return
		}
		seen[part] = true
		parts = append(parts, part)
	}
	// addPart drops a part shorter than 8 bytes, so "a" and "/" from
	// "https://host/a" never redact every occurrence of those characters.
	addPart := func(part string) {
		if len(part) < exposedTail || seen[part] {
			return
		}
		seen[part] = true
		parts = append(parts, part)
	}

	for _, tail := range tails(value) {
		addTail(tail)
	}
	addPart(value)
	if parsed, err := url.Parse(value); err == nil {
		addPart(parsed.Path)
		addPart(parsed.EscapedPath())
		addPart(parsed.RawQuery)
		addPart(parsed.Query().Encode())
		if parsed.User != nil {
			addPart(parsed.User.String())
		}
		addPart(rawUserinfo(value))
		addPart(lastPathSegment(parsed.Path))
		addPart(parsed.String())
	}
	return parts
}

// tails returns the last exposedTail characters of value taken by character
// (so a value ending in multi-byte characters is matched by rune) and by byte
// (so a value with invalid UTF-8 is still matched). A value no longer than the
// tail has no tail.
func tails(value string) []string {
	var out []string
	if runes := []rune(value); len(runes) > exposedTail {
		out = append(out, string(runes[len(runes)-exposedTail:]))
	}
	if len(value) > exposedTail {
		out = append(out, value[len(value)-exposedTail:])
	}
	return out
}

// rawUserinfo returns the userinfo as it appears in value, or "" when there is
// none. It is the counterpart to url.User.String, which re-encodes the decoded
// userinfo, so both the given and the written form become parts.
func rawUserinfo(value string) string {
	_, authority, found := strings.Cut(value, "://")
	if !found {
		return ""
	}
	if end := strings.IndexAny(authority, "/?#"); end >= 0 {
		authority = authority[:end]
	}
	at := strings.IndexByte(authority, '@')
	if at < 0 {
		return ""
	}
	return authority[:at]
}

// lastPathSegment returns the part of path after its last slash, ignoring a
// trailing slash, or "" when there is none.
func lastPathSegment(path string) string {
	path = strings.TrimRight(path, "/")
	if path == "" {
		return ""
	}
	if i := strings.LastIndexByte(path, '/'); i >= 0 {
		return path[i+1:]
	}
	return path
}
