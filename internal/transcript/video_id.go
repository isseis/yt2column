package transcript

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

// videoIDPattern matches exactly the 11 characters a YouTube video ID may use.
var videoIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{11}$`)

// validateVideoURL validates a video URL and returns its video ID together with
// the normalized URL https://www.youtube.com/watch?v=<id>. It accepts only the
// supported YouTube URL forms and never repairs a rejected input.
func validateVideoURL(rawURL string) (videoID string, normalizedURL string, err error) {
	if strings.TrimSpace(rawURL) != rawURL {
		return "", "", fmt.Errorf("%w: URL must not have leading or trailing whitespace", ErrInvalidVideoURL)
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", "", fmt.Errorf("%w: malformed URL", ErrInvalidVideoURL)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", "", fmt.Errorf("%w: scheme must be http or https", ErrInvalidVideoURL)
	}
	if parsed.User != nil {
		// No accepted form carries userinfo, and a URL can smuggle credentials
		// in it, so reject it instead of ignoring it.
		return "", "", fmt.Errorf("%w: URL must not carry userinfo", ErrInvalidVideoURL)
	}
	// url.URL.Query silently drops pairs it cannot parse (a bad percent escape
	// or a semicolon), which would hide a second v parameter, so reject a
	// malformed query instead.
	query, err := url.ParseQuery(parsed.RawQuery)
	if err != nil {
		return "", "", fmt.Errorf("%w: malformed query", ErrInvalidVideoURL)
	}
	if len(query["v"]) > 1 {
		return "", "", fmt.Errorf("%w: multiple v parameters", ErrInvalidVideoURL)
	}

	segments := splitURLPath(parsed.Path)
	switch strings.ToLower(parsed.Hostname()) {
	case "youtu.be":
		videoID, err = videoIDFromPath(segments)
	case "youtube.com", "www.youtube.com", "m.youtube.com":
		videoID, err = videoIDFromYouTube(segments, query)
	default:
		return "", "", fmt.Errorf("%w: unsupported host", ErrInvalidVideoURL)
	}
	if err != nil {
		return "", "", err
	}
	normalizedURL, ok := NormalizedVideoURL(videoID)
	if !ok {
		return "", "", fmt.Errorf("%w: video ID must be 11 characters of [A-Za-z0-9_-]", ErrInvalidVideoURL)
	}
	return videoID, normalizedURL, nil
}

// NormalizedVideoURL returns https://www.youtube.com/watch?v=<videoID> when
// videoID is exactly 11 characters of [A-Za-z0-9_-], and false otherwise.
func NormalizedVideoURL(videoID string) (string, bool) {
	if !videoIDPattern.MatchString(videoID) {
		return "", false
	}
	return "https://www.youtube.com/watch?v=" + videoID, true
}

// videoIDFromPath extracts the video ID from a youtu.be path, which must be a
// single segment. Extra segments are rejected rather than normalized away.
func videoIDFromPath(segments []string) (string, error) {
	if len(segments) != 1 {
		return "", fmt.Errorf("%w: youtu.be path must be a single video ID", ErrInvalidVideoURL)
	}
	return segments[0], nil
}

// videoIDFromYouTube extracts the video ID from a youtube.com path. Only the
// watch, embed, shorts, and live forms are accepted.
func videoIDFromYouTube(segments []string, query url.Values) (string, error) {
	switch {
	case len(segments) == 1 && segments[0] == "watch":
		values := query["v"]
		if len(values) != 1 {
			return "", fmt.Errorf("%w: watch URL must have exactly one v parameter", ErrInvalidVideoURL)
		}
		return values[0], nil
	case len(segments) == 2 && (segments[0] == "embed" || segments[0] == "shorts" || segments[0] == "live"):
		return segments[1], nil
	default:
		return "", fmt.Errorf("%w: unsupported YouTube URL path", ErrInvalidVideoURL)
	}
}

// splitURLPath splits a URL path into its non-empty-separated segments without
// normalizing dot segments or a trailing slash.
func splitURLPath(path string) []string {
	trimmed := strings.TrimPrefix(path, "/")
	if trimmed == "" {
		return nil
	}
	return strings.Split(trimmed, "/")
}
