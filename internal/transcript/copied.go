package transcript

import (
	"context"
	"errors"
	"fmt"
)

// CopiedInput is the input of NewCopiedSource. VideoID must already be
// validated by the caller (for the server, by ValidateVideoURL); Raw is the
// transcript text copied from YouTube's transcript panel, unmodified.
type CopiedInput struct {
	VideoID     string
	Raw         string
	Title       string
	ChannelName string
	Description string
}

// CopiedSource serves a transcript parsed from a CopiedInput. Its only field
// is the parsed Transcript; it has no field for the raw text. Build it with
// NewCopiedSource; a zero CopiedSource fails every Fetch.
type CopiedSource struct {
	transcript Transcript
}

// errCopiedSourceUninitialized is returned by Fetch on a zero CopiedSource, so
// the zero value cannot serve an empty Transcript as a success. It does not
// wrap ErrInvalidVideoURL, which means a different failure.
var errCopiedSourceUninitialized = errors.New("copied transcript source is not initialized")

// NewCopiedSource validates in.VideoID, parses in.Raw, and returns a source
// that serves the result. A VideoID that NormalizedVideoURL rejects returns an
// error wrapping ErrInvalidVideoURL; it is the caller's mistake, so classify
// it by the call site, never with errors.Is(err, ErrInvalidVideoURL). A Raw
// that matches neither form A nor form B returns an error wrapping
// ErrCopiedTranscriptFormat.
func NewCopiedSource(in CopiedInput) (*CopiedSource, error) {
	videoURL, ok := NormalizedVideoURL(in.VideoID)
	if !ok {
		return nil, fmt.Errorf("%w: copied transcript video ID must be 11 characters of [A-Za-z0-9_-]", ErrInvalidVideoURL)
	}
	segments, err := parseCopiedTranscript(in.Raw)
	if err != nil {
		return nil, err
	}
	return &CopiedSource{transcript: Transcript{
		VideoID:     in.VideoID,
		VideoURL:    videoURL,
		Title:       in.Title,
		ChannelName: in.ChannelName,
		Description: in.Description,
		Segments:    segments,
	}}, nil
}

// Fetch returns the parsed transcript when videoURL is exactly the normalized
// URL of the input VideoID. It never parses again.
func (s *CopiedSource) Fetch(ctx context.Context, videoURL string) (Transcript, error) {
	if err := ctx.Err(); err != nil {
		return Transcript{}, err
	}
	if len(s.transcript.Segments) == 0 {
		return Transcript{}, errCopiedSourceUninitialized
	}
	if videoURL != s.transcript.VideoURL {
		return Transcript{}, fmt.Errorf("%w: does not match the copied transcript's video", ErrInvalidVideoURL)
	}
	return s.transcript, nil
}
