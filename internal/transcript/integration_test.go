//go:build integration

package transcript

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

// The integration test runs the real yt-dlp against the network. It is
// excluded from `make test` by its build tag and run by `make test-integration`,
// which supplies both variables. The test holds no default of its own.
const (
	integrationVideoURLEnv = "YT2COLUMN_TEST_VIDEO_URL"
	integrationVideoIDEnv  = "YT2COLUMN_TEST_VIDEO_ID"

	// integrationFetchTimeout bounds one real yt-dlp run.
	integrationFetchTimeout = 3 * time.Minute

	// integrationMarker is written into the replaced cache; a forced refresh
	// must not return it.
	integrationMarker = "yt2column-integration-cache-marker"
)

// TestIntegration fetches one real video, then checks that a second Fetch is
// served from the cache without running yt-dlp and that a forced refresh
// ignores the cache. The subtests share the first fetch and its cache to keep
// the number of requests to YouTube low, so they run in order and a failed
// fetch stops the rest.
func TestIntegration(t *testing.T) {
	// A missing URL fails instead of skipping: a skipped integration test is
	// indistinguishable from a passing one in the output.
	videoURL := os.Getenv(integrationVideoURLEnv)
	if videoURL == "" {
		t.Fatalf("%s is not set: set it to the URL of a video with Japanese subtitles, or run `make test-integration`", integrationVideoURLEnv)
	}
	wantID := os.Getenv(integrationVideoIDEnv)
	cacheDir := t.TempDir()

	var first Transcript
	if !t.Run("fetch", func(t *testing.T) {
		source := newIntegrationSource(t, Options{CacheDir: cacheDir})
		transcript, err := source.Fetch(context.Background(), videoURL)
		if err != nil {
			t.Fatalf("Fetch(%q) error = %v", videoURL, err)
		}
		assertUsableTranscript(t, transcript)
		if wantID != "" && transcript.VideoID != wantID {
			t.Errorf("VideoID = %q, want %q (%s)", transcript.VideoID, wantID, integrationVideoIDEnv)
		}
		first = transcript
	}) {
		t.Fatal("the first fetch failed; the cache subtests depend on it")
	}

	t.Run("cache_reuse", func(t *testing.T) {
		tripwire, marker := writeTripwire(t)
		source := newIntegrationSource(t, Options{CacheDir: cacheDir, YtDlpPath: tripwire})
		transcript, err := source.Fetch(context.Background(), videoURL)
		if err != nil {
			t.Fatalf("Fetch(%q) from the cache error = %v", videoURL, err)
		}
		if _, err := os.Lstat(marker); !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("yt-dlp ran on a cache hit: marker %s exists (error = %v)", marker, err)
		}
		if !reflect.DeepEqual(transcript, first) {
			t.Errorf("cached transcript differs from the first fetch:\n got %s\nwant %s", describeTranscript(transcript), describeTranscript(first))
		}
	})

	t.Run("force_refresh", func(t *testing.T) {
		replaceCacheWithMarker(t, cacheDir, first.VideoID)

		// Precondition: the replaced cache is valid and is what a plain
		// Fetch returns, so the marker's absence below is due to the forced
		// refresh and not to a rejected cache.
		tripwire, _ := writeTripwire(t)
		cached, err := newIntegrationSource(t, Options{CacheDir: cacheDir, YtDlpPath: tripwire}).
			Fetch(context.Background(), videoURL)
		if err != nil {
			t.Fatalf("Fetch(%q) from the replaced cache error = %v", videoURL, err)
		}
		if !containsMarker(cached) {
			t.Fatalf("the replaced cache is not served: %s", describeTranscript(cached))
		}

		source := newIntegrationSource(t, Options{CacheDir: cacheDir, ForceRefresh: true})
		transcript, err := source.Fetch(context.Background(), videoURL)
		if err != nil {
			t.Fatalf("Fetch(%q) with ForceRefresh error = %v", videoURL, err)
		}
		assertUsableTranscript(t, transcript)
		if containsMarker(transcript) {
			t.Errorf("ForceRefresh returned the replaced cache: %s", describeTranscript(transcript))
		}
	})
}

// newIntegrationSource builds a source with the integration timeout. An empty
// YtDlpPath runs yt-dlp from PATH, so a missing yt-dlp fails the test.
func newIntegrationSource(t *testing.T, opts Options) *YtDlpSource {
	t.Helper()
	opts.Timeout = integrationFetchTimeout
	source, err := NewYtDlpSource(opts)
	if err != nil {
		t.Fatalf("NewYtDlpSource() error = %v", err)
	}
	return source
}

// writeTripwire writes an executable that creates a marker file and fails.
// Used as YtDlpPath, it proves that Fetch did not run yt-dlp when the marker
// is absent afterwards.
func writeTripwire(t *testing.T) (tripwire, marker string) {
	t.Helper()
	dir := t.TempDir()
	tripwire = filepath.Join(dir, "yt-dlp-tripwire")
	marker = filepath.Join(dir, "ran")
	quoted := "'" + strings.ReplaceAll(marker, "'", `'\''`) + "'"
	script := "#!/bin/sh\n: > " + quoted + "\nexit 1\n"
	if err := os.WriteFile(tripwire, []byte(script), 0o700); err != nil {
		t.Fatalf("write tripwire: %v", err)
	}
	return tripwire, marker
}

// replaceCacheWithMarker overwrites the valid slot of id with a valid
// subtitle file and info.json whose every text field holds the marker.
func replaceCacheWithMarker(t *testing.T, cacheDir, id string) {
	t.Helper()
	state, err := readPointer(cacheDir, id)
	if err != nil {
		t.Fatalf("readPointer() error = %v", err)
	}
	slot, ok := state.validSlot()
	if !ok {
		t.Fatalf("no valid cache slot for %s after the first fetch", id)
	}
	slotDir := slotDirPath(cacheDir, id, slot)
	subtitles := `{"events":[{"tStartMs":0,"segs":[{"utf8":"` + integrationMarker + `"}]}]}`
	info := `{"id":"` + id + `","title":"` + integrationMarker + `","channel":"` + integrationMarker +
		`","description":"` + integrationMarker + `"}`
	for path, content := range map[string]string{
		subtitlesPath(slotDir, id): subtitles,
		infoPath(slotDir, id):      info,
	} {
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatalf("replace %s: %v", path, err)
		}
	}
}

// assertUsableTranscript checks what any real fetch must return.
func assertUsableTranscript(t *testing.T, transcript Transcript) {
	t.Helper()
	if transcript.Title == "" {
		t.Error("Title is empty")
	}
	if transcript.ChannelName == "" {
		t.Error("ChannelName is empty")
	}
	if len(transcript.Segments) == 0 {
		t.Error("Segments is empty")
	}
}

// containsMarker reports whether any text of the transcript holds the marker.
func containsMarker(transcript Transcript) bool {
	texts := []string{transcript.Title, transcript.ChannelName, transcript.Description}
	for _, segment := range transcript.Segments {
		texts = append(texts, segment.Text)
	}
	for _, text := range texts {
		if strings.Contains(text, integrationMarker) {
			return true
		}
	}
	return false
}

// describeTranscript summarizes a transcript for a failure message without
// dumping the whole subtitle text.
func describeTranscript(transcript Transcript) string {
	firstText := ""
	if len(transcript.Segments) > 0 {
		firstText = transcript.Segments[0].Text
	}
	return fmt.Sprintf("{VideoID:%q Title:%q ChannelName:%q segments:%d first:%q}",
		transcript.VideoID, transcript.Title, transcript.ChannelName, len(transcript.Segments), firstText)
}
