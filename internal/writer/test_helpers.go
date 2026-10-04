//go:build test

package writer

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/isseis/yt2column/internal/llm"
	"github.com/isseis/yt2column/internal/llm/testutil"
	"github.com/isseis/yt2column/internal/transcript"
)

// Distinct markers placed in input values, so a test can tell where each
// value went (a prompt) or that it went nowhere (an error message).
const (
	markTitle       = "mark-title-q7Rz"
	markChannel     = "mark-channel-W2pk"
	markDescription = "mark-description-J9vx"
	markSegment     = "mark-segment-T4mc"

	markOutTitle        = "mark-out-title-B8nf"
	markOutBody         = "mark-out-body-K3sd"
	markOutModel        = "mark-out-model-Z5gh"
	markOutModelVersion = "mark-out-version-P6yl"
)

const (
	// testVideoID is a valid video ID used by validTranscript.
	testVideoID = "dQw4w9WgXcQ"
	// secondSegmentStartMs is the start time of validTranscript's second
	// segment; prompts must not depend on it.
	secondSegmentStartMs = 1500
)

// validTranscript returns a transcript that passes validation, with a
// distinct marker in each value a template may reference.
func validTranscript() transcript.Transcript {
	return transcript.Transcript{
		VideoID:     testVideoID,
		VideoURL:    "https://www.youtube.com/watch?v=" + testVideoID,
		Title:       markTitle,
		ChannelName: markChannel,
		Description: markDescription,
		Segments: []transcript.Segment{
			{StartMs: 0, Text: markSegment + "-1"},
			{StartMs: secondSegmentStartMs, Text: markSegment + "-2"},
		},
	}
}

// validResponse returns a response that passes validation, with a distinct
// marker in each of its values.
func validResponse() llm.GenerateResponse {
	return llm.GenerateResponse{
		Text:         "# " + markOutTitle + "\n" + markOutBody + "\n",
		Model:        markOutModel,
		ModelVersion: markOutModelVersion,
	}
}

// newFakeClient returns a fake LLM client that answers with validResponse.
func newFakeClient() *llmtestutil.FakeLLMClient {
	return &llmtestutil.FakeLLMClient{Result: validResponse()}
}

// mustNew builds an ArticleWriter or fails the test.
func mustNew(t *testing.T, client llm.LLMClient, opts Options) ArticleWriter {
	t.Helper()
	w, err := New(client, opts)
	if err != nil {
		t.Fatalf("New error = %v", err)
	}
	return w
}

// requireMalformed checks a rejection of the LLM response: the error matches
// ErrMalformedOutput, the article is the zero value, and the error text
// contains none of the response's values.
func requireMalformed(t *testing.T, a Article, err error) {
	t.Helper()
	if !errors.Is(err, ErrMalformedOutput) {
		t.Fatalf("Write error = %v, want ErrMalformedOutput", err)
	}
	if a != (Article{}) {
		t.Errorf("Write article = %+v, want the zero value", a)
	}
	for _, mark := range []string{markOutTitle, markOutBody, markOutModel, markOutModelVersion} {
		if strings.Contains(err.Error(), mark) {
			t.Errorf("Write error %q contains the response value marked %q", err, mark)
		}
	}
}

// overrideTargets lists, for each template, how to build Options that
// override only that template with the file at path.
var overrideTargets = []struct {
	name    string
	options func(path string) Options
}{
	{systemTemplateName, func(path string) Options { return Options{SystemTemplatePath: path} }},
	{userTemplateName, func(path string) Options { return Options{UserTemplatePath: path} }},
}

// writeOverrideFile writes content to a file in a fresh test temp directory
// and returns its path.
func writeOverrideFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "override.tmpl")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	return path
}

// requireNonRoot fails the test when it runs as root, where permission
// failures cannot be produced. Such a run is an unsupported environment, so it
// fails instead of being skipped silently.
func requireNonRoot(t *testing.T) {
	t.Helper()
	if os.Geteuid() == 0 {
		t.Fatal("unsupported test environment: permission-failure tests require a non-root user")
	}
}
