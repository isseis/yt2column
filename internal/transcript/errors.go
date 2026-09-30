package transcript

import (
	"errors"
	"fmt"
)

// Sentinel errors returned by the transcript stage. Fetch wraps these with
// context so callers can identify a failure with errors.Is.
var (
	ErrInvalidVideoURL = errors.New("invalid video URL")
	ErrYtDlpExec       = errors.New("yt-dlp execution failed")
	ErrParseSubtitles  = errors.New("parse subtitles")
	ErrParseInfo       = errors.New("parse video info")
	ErrNoSubtitles     = errors.New("no subtitles")
)

// ParseError keeps the path of the file that failed to parse so the caller can
// identify the damaged file. Unwrap returns ErrParseSubtitles or ErrParseInfo.
type ParseError struct {
	Path string
	Err  error
}

// Error implements the error interface.
func (e *ParseError) Error() string {
	return fmt.Sprintf("parse %s: %v", e.Path, e.Err)
}

// Unwrap returns the wrapped error, which matches ErrParseSubtitles or
// ErrParseInfo under errors.Is.
func (e *ParseError) Unwrap() error {
	return e.Err
}
