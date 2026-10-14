//go:build test

package transcript

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
)

// CopiedSource must satisfy the pipeline's TranscriptSource interface.
var _ TranscriptSource = (*CopiedSource)(nil)

const (
	copiedTestVideoID    = "dQw4w9WgXcQ"
	copiedTestOtherID    = "abcdefghijk"
	copiedTestRaw        = "0:00\n0:05\nこんにちは\n"
	copiedTestVideoURL   = "https://www.youtube.com/watch?v=dQw4w9WgXcQ"
	copiedTestOtherURL   = "https://www.youtube.com/watch?v=abcdefghijk"
	copiedTestYoutuBeURL = "https://youtu.be/dQw4w9WgXcQ"
)

func TestNewCopiedSource(t *testing.T) {
	t.Run("valid input succeeds", func(t *testing.T) {
		source, err := NewCopiedSource(CopiedInput{VideoID: copiedTestVideoID, Raw: copiedTestRaw})
		if err != nil {
			t.Fatalf("NewCopiedSource error = %v", err)
		}
		if source == nil {
			t.Fatal("NewCopiedSource returned a nil source with a nil error")
		}
	})

	t.Run("invalid raw wraps the format sentinel", func(t *testing.T) {
		source, err := NewCopiedSource(CopiedInput{VideoID: copiedTestVideoID, Raw: "not a transcript\n"})
		if !errors.Is(err, ErrCopiedTranscriptFormat) {
			t.Fatalf("error = %v, want ErrCopiedTranscriptFormat", err)
		}
		if errors.Is(err, ErrInvalidVideoURL) {
			t.Errorf("error = %v, also matches ErrInvalidVideoURL", err)
		}
		if source != nil {
			t.Errorf("source = %+v, want nil", source)
		}
	})

	rejects := []struct {
		name    string
		videoID string
	}{
		{name: "empty video ID", videoID: ""},
		{name: "ten-character video ID", videoID: "dQw4w9WgXc"},
		{name: "video ID with a slash", videoID: "dQw4w9WgXc/"},
	}
	for _, tc := range rejects {
		t.Run("invalid video ID/"+tc.name, func(t *testing.T) {
			source, err := NewCopiedSource(CopiedInput{VideoID: tc.videoID, Raw: copiedTestRaw})
			if !errors.Is(err, ErrInvalidVideoURL) {
				t.Fatalf("error = %v, want ErrInvalidVideoURL", err)
			}
			if errors.Is(err, ErrCopiedTranscriptFormat) {
				t.Errorf("error = %v, also matches ErrCopiedTranscriptFormat", err)
			}
			if source != nil {
				t.Errorf("source = %+v, want nil", source)
			}
		})
	}

	t.Run("both invalid checks the video ID first", func(t *testing.T) {
		source, err := NewCopiedSource(CopiedInput{VideoID: "", Raw: "not a transcript\n"})
		if !errors.Is(err, ErrInvalidVideoURL) {
			t.Fatalf("error = %v, want ErrInvalidVideoURL", err)
		}
		if errors.Is(err, ErrCopiedTranscriptFormat) {
			t.Errorf("error = %v, also matches ErrCopiedTranscriptFormat", err)
		}
		if source != nil {
			t.Errorf("source = %+v, want nil", source)
		}
	})
}

func TestCopiedSourceTranscript(t *testing.T) {
	t.Run("derives the video URL from the video ID", func(t *testing.T) {
		source := mustCopiedSource(t, CopiedInput{VideoID: copiedTestVideoID, Raw: copiedTestRaw})
		transcript, err := source.Fetch(context.Background(), copiedTestVideoURL)
		if err != nil {
			t.Fatalf("Fetch error = %v", err)
		}
		if transcript.VideoID != copiedTestVideoID {
			t.Errorf("VideoID = %q, want %q", transcript.VideoID, copiedTestVideoID)
		}
		if transcript.VideoURL != copiedTestVideoURL {
			t.Errorf("VideoURL = %q, want %q", transcript.VideoURL, copiedTestVideoURL)
		}
	})

	t.Run("keeps the metadata as it is", func(t *testing.T) {
		in := CopiedInput{
			VideoID:     copiedTestVideoID,
			Raw:         copiedTestRaw,
			Title:       "  a title  ",
			ChannelName: "",
			Description: string([]byte{0xff, 0xfe}),
		}
		transcript := mustCopiedTranscript(t, in)
		if transcript.Title != in.Title {
			t.Errorf("Title = %q, want %q", transcript.Title, in.Title)
		}
		if transcript.ChannelName != in.ChannelName {
			t.Errorf("ChannelName = %q, want %q", transcript.ChannelName, in.ChannelName)
		}
		if transcript.Description != in.Description {
			t.Errorf("Description = %q, want %q", transcript.Description, in.Description)
		}
	})

	t.Run("does not take the video URL from another field", func(t *testing.T) {
		in := CopiedInput{
			VideoID:     copiedTestVideoID,
			Raw:         copiedTestRaw,
			Title:       copiedTestOtherURL,
			ChannelName: copiedTestYoutuBeURL,
			Description: copiedTestOtherURL,
		}
		transcript := mustCopiedTranscript(t, in)
		if transcript.VideoURL != copiedTestVideoURL {
			t.Errorf("VideoURL = %q, want the value derived from VideoID %q", transcript.VideoURL, copiedTestVideoURL)
		}
	})

	t.Run("segments match a direct parse", func(t *testing.T) {
		transcript := mustCopiedTranscript(t, CopiedInput{VideoID: copiedTestVideoID, Raw: copiedTestRaw})
		want, err := parseCopiedTranscript(copiedTestRaw)
		if err != nil {
			t.Fatalf("parseCopiedTranscript error = %v", err)
		}
		if !reflect.DeepEqual(transcript.Segments, want) {
			t.Errorf("Segments = %+v, want %+v", transcript.Segments, want)
		}
	})
}

func TestCopiedSourceFetch(t *testing.T) {
	t.Run("returns only for the matching URL", func(t *testing.T) {
		source := mustCopiedSource(t, CopiedInput{VideoID: copiedTestVideoID, Raw: copiedTestRaw})
		transcript, err := source.Fetch(context.Background(), copiedTestVideoURL)
		if err != nil {
			t.Fatalf("Fetch error = %v", err)
		}
		if transcript.VideoID != copiedTestVideoID {
			t.Errorf("VideoID = %q, want %q", transcript.VideoID, copiedTestVideoID)
		}
	})

	rejects := []struct {
		name     string
		videoURL string
	}{
		{name: "empty", videoURL: ""},
		{name: "another video", videoURL: copiedTestOtherURL},
		{name: "youtu.be form", videoURL: copiedTestYoutuBeURL},
		{name: "http form", videoURL: "http://www.youtube.com/watch?v=dQw4w9WgXcQ"},
		{name: "extra query", videoURL: "https://www.youtube.com/watch?v=dQw4w9WgXcQ&t=1"},
	}
	for _, tc := range rejects {
		t.Run("reject/"+tc.name, func(t *testing.T) {
			source := mustCopiedSource(t, CopiedInput{VideoID: copiedTestVideoID, Raw: copiedTestRaw})
			transcript, err := source.Fetch(context.Background(), tc.videoURL)
			if !errors.Is(err, ErrInvalidVideoURL) {
				t.Fatalf("Fetch(%q) error = %v, want ErrInvalidVideoURL", tc.videoURL, err)
			}
			if !reflect.DeepEqual(transcript, Transcript{}) {
				t.Errorf("Fetch(%q) transcript = %+v, want the zero value", tc.videoURL, transcript)
			}
		})
	}

	t.Run("mismatch error omits the URL", func(t *testing.T) {
		source := mustCopiedSource(t, CopiedInput{VideoID: copiedTestVideoID, Raw: copiedTestRaw})
		_, err := source.Fetch(context.Background(), "https://evil.example/MARKER")
		if err == nil {
			t.Fatal("Fetch accepted a mismatched URL")
		}
		if strings.Contains(err.Error(), "MARKER") {
			t.Errorf("Error() = %q, must not contain the URL", err.Error())
		}
	})

	t.Run("canceled context is returned first", func(t *testing.T) {
		source := mustCopiedSource(t, CopiedInput{VideoID: copiedTestVideoID, Raw: copiedTestRaw})
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		for _, videoURL := range []string{copiedTestVideoURL, ""} {
			_, err := source.Fetch(ctx, videoURL)
			if !errors.Is(err, context.Canceled) {
				t.Errorf("Fetch(canceled, %q) error = %v, want context.Canceled", videoURL, err)
			}
		}
	})

	t.Run("zero value is not initialized", func(t *testing.T) {
		var source CopiedSource
		transcript, err := source.Fetch(context.Background(), "")
		if !errors.Is(err, errCopiedSourceUninitialized) {
			t.Fatalf("Fetch error = %v, want errCopiedSourceUninitialized", err)
		}
		if errors.Is(err, ErrInvalidVideoURL) {
			t.Errorf("error = %v, also matches ErrInvalidVideoURL", err)
		}
		if !reflect.DeepEqual(transcript, Transcript{}) {
			t.Errorf("transcript = %+v, want the zero value", transcript)
		}
	})
}

func TestCopiedSourceHoldsOnlyTranscript(t *testing.T) {
	t.Run("the only field is a Transcript", func(t *testing.T) {
		typ := reflect.TypeFor[CopiedSource]()
		if typ.NumField() != 1 {
			t.Fatalf("CopiedSource has %d fields, want 1", typ.NumField())
		}
		if field := typ.Field(0); field.Type != reflect.TypeFor[Transcript]() {
			t.Errorf("field %q has type %v, want transcript.Transcript", field.Name, field.Type)
		}
	})

	t.Run("Fetch returns the same value twice", func(t *testing.T) {
		source := mustCopiedSource(t, CopiedInput{VideoID: copiedTestVideoID, Raw: copiedTestRaw})
		first, err := source.Fetch(context.Background(), copiedTestVideoURL)
		if err != nil {
			t.Fatalf("first Fetch error = %v", err)
		}
		second, err := source.Fetch(context.Background(), copiedTestVideoURL)
		if err != nil {
			t.Fatalf("second Fetch error = %v", err)
		}
		if !reflect.DeepEqual(first, second) {
			t.Errorf("Fetch returned %+v then %+v", first, second)
		}
	})
}

func TestNewCopiedSourceErrorOmitsInput(t *testing.T) {
	t.Run("format error omits the raw text", func(t *testing.T) {
		_, err := NewCopiedSource(CopiedInput{VideoID: copiedTestVideoID, Raw: "0:24MARKER_BODY"})
		if err == nil {
			t.Fatal("NewCopiedSource accepted invalid raw text")
		}
		for _, marker := range []string{"MARKER_BODY", "0:24"} {
			if strings.Contains(err.Error(), marker) {
				t.Errorf("Error() = %q, must not contain %q", err.Error(), marker)
			}
		}
	})

	t.Run("format error omits the metadata", func(t *testing.T) {
		_, err := NewCopiedSource(CopiedInput{
			VideoID:     copiedTestVideoID,
			Raw:         "0:24MARKER_BODY",
			Title:       "MARKER_TITLE",
			ChannelName: "MARKER_CHANNEL",
			Description: "MARKER_DESCRIPTION",
		})
		if err == nil {
			t.Fatal("NewCopiedSource accepted invalid raw text")
		}
		for _, marker := range []string{"MARKER_BODY", "MARKER_TITLE", "MARKER_CHANNEL", "MARKER_DESCRIPTION"} {
			if strings.Contains(err.Error(), marker) {
				t.Errorf("Error() = %q, must not contain %q", err.Error(), marker)
			}
		}
	})

	t.Run("video ID error omits the video ID", func(t *testing.T) {
		_, err := NewCopiedSource(CopiedInput{VideoID: "markerXYZ", Raw: copiedTestRaw})
		if err == nil {
			t.Fatal("NewCopiedSource accepted an invalid video ID")
		}
		if strings.Contains(err.Error(), "markerXYZ") {
			t.Errorf("Error() = %q, must not contain the video ID", err.Error())
		}
	})
}

func mustCopiedSource(t *testing.T, in CopiedInput) *CopiedSource {
	t.Helper()
	source, err := NewCopiedSource(in)
	if err != nil {
		t.Fatalf("NewCopiedSource(%+v) error = %v", in, err)
	}
	return source
}

func mustCopiedTranscript(t *testing.T, in CopiedInput) Transcript {
	t.Helper()
	videoURL, ok := NormalizedVideoURL(in.VideoID)
	if !ok {
		t.Fatalf("test video ID %q is not valid", in.VideoID)
	}
	transcript, err := mustCopiedSource(t, in).Fetch(context.Background(), videoURL)
	if err != nil {
		t.Fatalf("Fetch error = %v", err)
	}
	return transcript
}
