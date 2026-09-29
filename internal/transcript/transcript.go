// Package transcript defines the transcript stage: the data passed between
// pipeline stages and the interface that fetches it.
package transcript

import "context"

// Segment is one subtitle segment.
type Segment struct {
	StartMs int64
	Text    string
}

// Transcript is the transcript body and video metadata.
type Transcript struct {
	VideoID     string
	VideoURL    string
	Title       string
	ChannelName string
	Description string
	Segments    []Segment
}

// TranscriptSource fetches a transcript for a video URL.
// Implementations must return an error on failure and must not
// return an empty Transcript as a successful result.
type TranscriptSource interface { //nolint:revive // TranscriptSource is the name fixed by the pipeline design
	Fetch(ctx context.Context, videoURL string) (Transcript, error)
}
