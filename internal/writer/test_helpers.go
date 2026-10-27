//go:build test

package writer

import (
	"context"
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

	// Step markers, distinct from the generation markers above, so a test
	// can tell which step produced or carried a value.
	markGenTitle         = "mark-gen-title-A1b2"
	markGenBody          = "mark-gen-body-C3d4"
	markShortTitle       = "mark-short-title-E5f6"
	markShortBody        = "mark-short-body-G7h8"
	markShortenModel     = "mark-shorten-model-I9j0"
	markReviewModel      = "mark-review-model-K1l2"
	markStepModelVersion = "mark-step-version-M3n4"
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

// stepResponseText builds the generated text of the generation or shorten step
// from a title and a body part.
func stepResponseText(title, body string) string {
	return "# " + title + "\n" + body
}

// overLimitBody returns a body part one character over maxBodyChars, starting
// with the generation body marker.
func overLimitBody() string {
	return markGenBody + strings.Repeat("a", maxBodyChars+1-len(markGenBody))
}

// overLimitShortenBody returns a body part still over maxBodyChars after the
// shorten step, starting with the shorten body marker.
func overLimitShortenBody() string {
	return markShortBody + strings.Repeat("a", maxBodyChars+1-len(markShortBody))
}

// withStepsEnabled returns Options with both post-generation steps enabled.
func withStepsEnabled(review llm.LLMClient) Options {
	return Options{
		Shorten: ShortenOptions{Enabled: true},
		Review:  ReviewOptions{Enabled: true, Client: review},
	}
}

// newStepClient returns a fake sequence with TB set to t, so an unexpected
// extra call is reported instead of being silently absorbed.
func newStepClient(t *testing.T, steps ...llmtestutil.FakeLLMSequenceStep) *llmtestutil.FakeLLMSequence {
	t.Helper()
	return &llmtestutil.FakeLLMSequence{TB: t, Steps: steps}
}

// stepResult builds one fake sequence step from a response.
func stepResult(resp llm.GenerateResponse) llmtestutil.FakeLLMSequenceStep {
	return llmtestutil.FakeLLMSequenceStep{Result: resp}
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

// Generation-data field references, used to build the field substitution
// tables below.
const (
	fieldRefTitle       = ".Title"
	fieldRefChannelName = ".ChannelName"
	fieldRefDescription = ".Description"
	fieldRefTranscript  = ".Transcript"
)

// shortenFieldSubstitutions maps a generation-data field reference to a field
// the shorten data type also has, so the shared syntax tables apply to a
// shorten template.
var shortenFieldSubstitutions = map[string]string{
	fieldRefTitle:       ".ArticleTitle",
	fieldRefChannelName: ".ArticleBody",
	fieldRefDescription: ".TargetBodyChars",
	fieldRefTranscript:  ".MaxBodyChars",
}

// reviewFieldSubstitutions leaves the four generation field references as they
// are: reviewTemplateData embeds templateData, so it has all four.
var reviewFieldSubstitutions = map[string]string{
	fieldRefTitle:       fieldRefTitle,
	fieldRefChannelName: fieldRefChannelName,
	fieldRefDescription: fieldRefDescription,
	fieldRefTranscript:  fieldRefTranscript,
}

// stepTemplateTargets describes the four step templates the shared syntax and
// source-name tests exercise. fields substitutes a generation field reference
// for a field of the step's data type; options builds Options that enable the
// step with one template overridden; parse runs the construction check with the
// step's data type for an embedded default.
var stepTemplateTargets = []struct {
	name    string
	fields  map[string]string
	options func(path string) Options
	parse   func(src templateSource, text string) error
}{
	{
		shortenSystemTemplateName,
		shortenFieldSubstitutions,
		func(path string) Options {
			return Options{Shorten: ShortenOptions{Enabled: true, SystemTemplatePath: path}}
		},
		func(src templateSource, text string) error {
			_, err := parseTemplate[shortenTemplateData](src, text)
			return err
		},
	},
	{
		shortenUserTemplateName,
		shortenFieldSubstitutions,
		func(path string) Options {
			return Options{Shorten: ShortenOptions{Enabled: true, UserTemplatePath: path}}
		},
		func(src templateSource, text string) error {
			_, err := parseTemplate[shortenTemplateData](src, text)
			return err
		},
	},
	{
		reviewSystemTemplateName,
		reviewFieldSubstitutions,
		func(path string) Options {
			return Options{Review: ReviewOptions{Enabled: true, Client: &llmtestutil.FakeLLMClient{}, SystemTemplatePath: path}}
		},
		func(src templateSource, text string) error {
			_, err := parseTemplate[reviewTemplateData](src, text)
			return err
		},
	},
	{
		reviewUserTemplateName,
		reviewFieldSubstitutions,
		func(path string) Options {
			return Options{Review: ReviewOptions{Enabled: true, Client: &llmtestutil.FakeLLMClient{}, UserTemplatePath: path}}
		},
		func(src templateSource, text string) error {
			_, err := parseTemplate[reviewTemplateData](src, text)
			return err
		},
	},
}

// substituteFields replaces every generation field reference in text with the
// field the target's data type has. A nil map leaves text unchanged.
func substituteFields(text string, fields map[string]string) string {
	for from, to := range fields {
		text = strings.ReplaceAll(text, from, to)
	}
	return text
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

// fakeArtifactStore remembers Load and Save calls and returns configured
// values. It stands in for ArtifactStore in unit tests; Save returns a
// per-stage identifier so a test can tell that the writer used its return
// value as the next stage's input.
type fakeArtifactStore struct {
	loadResult ArtifactChain
	loadErr    error
	loadCalls  []fakeLoadCall
	saveErrs   map[Stage]error
	saveCalls  []fakeSaveCall
}

// fakeLoadCall is one recorded Load call.
type fakeLoadCall struct {
	videoID    string
	stage      Stage
	materialID string
}

// fakeSaveCall is one recorded Save call.
type fakeSaveCall struct {
	videoID string
	record  StageRecord
}

var _ ArtifactStore = (*fakeArtifactStore)(nil)

func (f *fakeArtifactStore) Load(_ context.Context, videoID string, stage Stage, materialID string) (ArtifactChain, error) {
	f.loadCalls = append(f.loadCalls, fakeLoadCall{videoID: videoID, stage: stage, materialID: materialID})
	if f.loadErr != nil {
		return ArtifactChain{}, f.loadErr
	}
	return f.loadResult, nil
}

func (f *fakeArtifactStore) Save(_ context.Context, videoID string, record StageRecord) (string, error) {
	f.saveCalls = append(f.saveCalls, fakeSaveCall{videoID: videoID, record: record})
	if err, ok := f.saveErrs[record.Stage]; ok {
		return "", err
	}
	return "id-" + record.Stage.String(), nil
}

// saveStages returns the stage of every recorded Save call, in order.
func (f *fakeArtifactStore) saveStages() []Stage {
	stages := make([]Stage, len(f.saveCalls))
	for i, call := range f.saveCalls {
		stages[i] = call.record.Stage
	}
	return stages
}

// saveFor returns the recorded Save call for stage (the zero call if none).
func (f *fakeArtifactStore) saveFor(stage Stage) fakeSaveCall {
	for _, call := range f.saveCalls {
		if call.record.Stage == stage {
			return call
		}
	}
	return fakeSaveCall{}
}

// progressEvent is one call a recordingObserver received.
type progressEvent struct {
	kind  string // "started", "finished", or "skipped"
	stage Stage
}

func (e progressEvent) String() string { return e.kind + " " + e.stage.String() }

// Shorthands for the expected events of a progress test.
func started(s Stage) progressEvent  { return progressEvent{"started", s} }
func finished(s Stage) progressEvent { return progressEvent{"finished", s} }
func skipped(s Stage) progressEvent  { return progressEvent{"skipped", s} }

// recordingObserver records every ProgressObserver call, in order.
type recordingObserver struct {
	events []progressEvent
}

var _ ProgressObserver = (*recordingObserver)(nil)

func (r *recordingObserver) StageStarted(s Stage)  { r.events = append(r.events, started(s)) }
func (r *recordingObserver) StageFinished(s Stage) { r.events = append(r.events, finished(s)) }
func (r *recordingObserver) StageSkipped(s Stage)  { r.events = append(r.events, skipped(s)) }

// mustNewProgress builds a ProgressWriter or fails the test.
func mustNewProgress(t *testing.T, client llm.LLMClient, opts Options) ProgressWriter {
	t.Helper()
	w, err := NewProgressWriter(client, opts)
	if err != nil {
		t.Fatalf("NewProgressWriter error = %v", err)
	}
	return w
}
