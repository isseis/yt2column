//go:build test

package pipeline

import (
	"context"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
	"unicode"

	"github.com/isseis/yt2column/internal/llm"
	"github.com/isseis/yt2column/internal/publisher"
	"github.com/isseis/yt2column/internal/publisher/testutil"
	"github.com/isseis/yt2column/internal/secret"
	"github.com/isseis/yt2column/internal/transcript"
	"github.com/isseis/yt2column/internal/transcript/testutil"
	"github.com/isseis/yt2column/internal/writer"
	"github.com/isseis/yt2column/internal/writer/testutil"
)

// unknownStage is a Stage value outside the defined range.
const unknownStage Stage = 99

// flipAfterFetch cancels the context after observing the Fetch call.
type flipAfterFetch struct {
	inner *transcripttestutil.FakeTranscriptSource
	flip  func()
}

func (s *flipAfterFetch) Fetch(ctx context.Context, videoURL string) (transcript.Transcript, error) {
	s.flip()
	return s.inner.Fetch(ctx, videoURL)
}

// flipAfterWrite cancels the context after observing the Write call.
type flipAfterWrite struct {
	inner *writertestutil.FakeArticleWriter
	flip  func()
}

func (w *flipAfterWrite) Write(ctx context.Context, t transcript.Transcript) (writer.Article, error) {
	w.flip()
	return w.inner.Write(ctx, t)
}

func TestPipelineSuccess(t *testing.T) {
	in := transcript.Transcript{
		VideoID:     "video-1",
		VideoURL:    "https://example.com/watch?v=video-1",
		Title:       "Title",
		ChannelName: "Channel",
		Description: "Description",
		Segments: []transcript.Segment{
			{StartMs: 0, Text: "first"},
			{StartMs: 1500, Text: "second"},
			{StartMs: 3000, Text: "third"},
		},
	}
	out := writer.Article{Title: "Article", Body: "Body", SourceURL: in.VideoURL, Model: "model"}

	src := &transcripttestutil.FakeTranscriptSource{Result: in}
	wr := &writertestutil.FakeArticleWriter{Result: out}
	pub := &publishertestutil.FakePublisher{}
	p, err := New(src, wr, pub)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	got, err := p.Run(context.Background(), in.VideoURL)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if !reflect.DeepEqual(got, out) {
		t.Errorf("Run returned %+v, want %+v", got, out)
	}

	if len(src.Calls) != 1 || len(wr.Calls) != 1 || len(pub.Calls) != 1 {
		t.Fatalf("call counts = source %d, writer %d, publisher %d; want 1 each", len(src.Calls), len(wr.Calls), len(pub.Calls))
	}
	if src.Calls[0].VideoURL != in.VideoURL {
		t.Errorf("source got videoURL %q, want %q", src.Calls[0].VideoURL, in.VideoURL)
	}
	if !reflect.DeepEqual(wr.Calls[0].Transcript, in) {
		t.Errorf("writer got %+v, want %+v", wr.Calls[0].Transcript, in)
	}
	if !reflect.DeepEqual(pub.Calls[0].Article, out) {
		t.Errorf("publisher got %+v, want %+v", pub.Calls[0].Article, out)
	}
}

func TestPipelineStageFailure(t *testing.T) {
	stageErr := errors.New("stage failed")

	cases := []struct {
		name         string
		fail         Stage
		wantSrcCalls int
		wantWrCalls  int
		wantPubCalls int
	}{
		{"transcript", StageTranscript, 1, 0, 0},
		{"write", StageWrite, 1, 1, 0},
		{"publish", StagePublish, 1, 1, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			src := &transcripttestutil.FakeTranscriptSource{}
			wr := &writertestutil.FakeArticleWriter{}
			pub := &publishertestutil.FakePublisher{}
			switch tc.fail {
			case StageTranscript:
				src.Err = stageErr
			case StageWrite:
				wr.Err = stageErr
			case StagePublish:
				pub.Err = stageErr
			}

			p, err := New(src, wr, pub)
			if err != nil {
				t.Fatalf("New returned error: %v", err)
			}
			if _, err := p.Run(context.Background(), "url"); err == nil {
				t.Fatal("Run returned nil error")
			}

			if len(src.Calls) != tc.wantSrcCalls {
				t.Errorf("source calls = %d, want %d", len(src.Calls), tc.wantSrcCalls)
			}
			if len(wr.Calls) != tc.wantWrCalls {
				t.Errorf("writer calls = %d, want %d", len(wr.Calls), tc.wantWrCalls)
			}
			if len(pub.Calls) != tc.wantPubCalls {
				t.Errorf("publisher calls = %d, want %d", len(pub.Calls), tc.wantPubCalls)
			}
		})
	}
}

func TestPipelineStageError(t *testing.T) {
	original := errors.New("boom")

	cases := []struct {
		name      string
		want      Stage
		configure func(*transcripttestutil.FakeTranscriptSource, *writertestutil.FakeArticleWriter, *publishertestutil.FakePublisher)
	}{
		{"transcript", StageTranscript, func(s *transcripttestutil.FakeTranscriptSource, _ *writertestutil.FakeArticleWriter, _ *publishertestutil.FakePublisher) {
			s.Err = original
		}},
		{"write", StageWrite, func(_ *transcripttestutil.FakeTranscriptSource, w *writertestutil.FakeArticleWriter, _ *publishertestutil.FakePublisher) {
			w.Err = original
		}},
		{"publish", StagePublish, func(_ *transcripttestutil.FakeTranscriptSource, _ *writertestutil.FakeArticleWriter, p *publishertestutil.FakePublisher) {
			p.Err = original
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			src := &transcripttestutil.FakeTranscriptSource{}
			wr := &writertestutil.FakeArticleWriter{}
			pub := &publishertestutil.FakePublisher{}
			tc.configure(src, wr, pub)

			p, err := New(src, wr, pub)
			if err != nil {
				t.Fatalf("New returned error: %v", err)
			}
			_, err = p.Run(context.Background(), "url")

			stageErr, ok := errors.AsType[*StageError](err)
			if !ok {
				t.Fatalf("Run error = %v, want *StageError", err)
			}
			if stageErr.Stage != tc.want {
				t.Errorf("StageError.Stage = %v, want %v", stageErr.Stage, tc.want)
			}
			if !errors.Is(err, original) {
				t.Errorf("errors.Is(err, original) = false for %v", err)
			}
			if want := tc.name + ": boom"; err.Error() != want {
				t.Errorf("error = %q, want %q", err.Error(), want)
			}
		})
	}
}

func TestPipelineCanceled(t *testing.T) {
	t.Run("canceled before start", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		src := &transcripttestutil.FakeTranscriptSource{}
		pub := &publishertestutil.FakePublisher{}
		p, err := New(src, &writertestutil.FakeArticleWriter{}, pub)
		if err != nil {
			t.Fatalf("New returned error: %v", err)
		}

		_, err = p.Run(ctx, "url")
		if err != context.Canceled {
			t.Errorf("Run error = %v, want context.Canceled", err)
		}
		if _, ok := errors.AsType[*StageError](err); ok {
			t.Error("cancellation was wrapped in a StageError")
		}
		if len(src.Calls) != 0 {
			t.Error("source called after cancellation")
		}
		if len(pub.Calls) != 0 {
			t.Error("publisher called after cancellation")
		}
	})

	t.Run("canceled after fetch", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		srcFake := &transcripttestutil.FakeTranscriptSource{}
		src := &flipAfterFetch{inner: srcFake, flip: cancel}
		wrFake := &writertestutil.FakeArticleWriter{}
		pub := &publishertestutil.FakePublisher{}
		p, err := New(src, wrFake, pub)
		if err != nil {
			t.Fatalf("New returned error: %v", err)
		}

		_, err = p.Run(ctx, "url")
		if err != context.Canceled {
			t.Errorf("Run error = %v, want context.Canceled", err)
		}
		if _, ok := errors.AsType[*StageError](err); ok {
			t.Error("cancellation was wrapped in a StageError")
		}
		if len(srcFake.Calls) != 1 {
			t.Errorf("source calls = %d, want 1", len(srcFake.Calls))
		}
		if len(wrFake.Calls) != 0 {
			t.Error("writer called after cancellation")
		}
		if len(pub.Calls) != 0 {
			t.Error("publisher called after cancellation")
		}
	})

	t.Run("canceled after write", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		runUsage := writer.RunUsage{
			Generation: writer.StageUsage{Origin: writer.OriginRun, Model: "gen-model", Usage: testUsage(5, 7)},
		}
		wrFake := &writertestutil.FakeArticleWriter{Result: writer.Article{RunUsage: runUsage}}
		wr := &flipAfterWrite{inner: wrFake, flip: cancel}
		pub := &publishertestutil.FakePublisher{}
		p, err := New(&transcripttestutil.FakeTranscriptSource{}, wr, pub)
		if err != nil {
			t.Fatalf("New returned error: %v", err)
		}

		article, err := p.Run(ctx, "url")
		if article != (writer.Article{}) {
			t.Errorf("Run article = %+v, want the zero value", article)
		}
		if !errors.Is(err, context.Canceled) {
			t.Errorf("Run error = %v, want context.Canceled", err)
		}
		if _, ok := errors.AsType[*StageError](err); ok {
			t.Error("cancellation was wrapped in a StageError")
		}
		tue, ok := errors.AsType[*writer.TokenUsageError](err)
		if !ok {
			t.Fatalf("Run error = %v, want a *writer.TokenUsageError", err)
		}
		if tue.RunUsage != runUsage {
			t.Errorf("TokenUsageError.RunUsage = %+v, want %+v", tue.RunUsage, runUsage)
		}
		if len(wrFake.Calls) != 1 {
			t.Errorf("writer calls = %d, want 1", len(wrFake.Calls))
		}
		if len(pub.Calls) != 0 {
			t.Error("publisher called after cancellation")
		}
	})
}

// testUsage is a known usage with the given input and output counts, for the
// tests that carry a run usage through the pipeline.
func testUsage(input, output uint64) llm.Usage {
	return llm.KnownUsage(llm.TokenCounts{Input: input, Output: output})
}

func TestPipelinePublishFailureCarriesRunUsage(t *testing.T) {
	runUsage := writer.RunUsage{
		Review: writer.StageUsage{Origin: writer.OriginRun, Model: "review-model", Usage: testUsage(3, 4)},
	}
	wr := &writertestutil.FakeArticleWriter{Result: writer.Article{RunUsage: runUsage}}
	publishErr := errors.New("publish boom")
	pub := &publishertestutil.FakePublisher{Err: publishErr}
	p, err := New(&transcripttestutil.FakeTranscriptSource{}, wr, pub)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	article, err := p.Run(context.Background(), "url")
	if article != (writer.Article{}) {
		t.Errorf("Run article = %+v, want the zero value", article)
	}
	stageErr, ok := errors.AsType[*StageError](err)
	if !ok {
		t.Fatalf("Run error = %v, want *StageError", err)
	}
	if stageErr.Stage != StagePublish {
		t.Errorf("StageError.Stage = %v, want %v", stageErr.Stage, StagePublish)
	}
	tue, ok := errors.AsType[*writer.TokenUsageError](err)
	if !ok {
		t.Fatalf("Run error = %v, want it to unwrap to a *writer.TokenUsageError", err)
	}
	if tue.RunUsage != runUsage {
		t.Errorf("TokenUsageError.RunUsage = %+v, want %+v", tue.RunUsage, runUsage)
	}
	if !errors.Is(err, publishErr) {
		t.Errorf("errors.Is(err, publish error) = false for %v", err)
	}
}

func TestPipelineNewNilStage(t *testing.T) {
	validSrc := &transcripttestutil.FakeTranscriptSource{}
	validWr := &writertestutil.FakeArticleWriter{}
	validPub := &publishertestutil.FakePublisher{}

	cases := []struct {
		name string
		src  transcript.TranscriptSource
		wr   writer.ArticleWriter
		pub  publisher.Publisher
		want string
	}{
		{"nil source", nil, validWr, validPub, "transcript"},
		{"nil writer", validSrc, nil, validPub, "write"},
		{"nil publisher", validSrc, validWr, nil, "publish"},
		{"typed-nil source", (*transcripttestutil.FakeTranscriptSource)(nil), validWr, validPub, "transcript"},
		{"typed-nil writer", validSrc, (*writertestutil.FakeArticleWriter)(nil), validPub, "write"},
		{"typed-nil publisher", validSrc, validWr, (*publishertestutil.FakePublisher)(nil), "publish"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p, err := New(tc.src, tc.wr, tc.pub)
			if err == nil {
				t.Fatal("New returned nil error")
			}
			if p != nil {
				t.Error("New returned a non-nil pipeline together with an error")
			}
			if !errors.Is(err, ErrNilStage) {
				t.Errorf("New error = %v, want ErrNilStage", err)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("New error = %q, want it to contain %q", err.Error(), tc.want)
			}
		})
	}
}

func TestPipelineZeroValueRun(t *testing.T) {
	t.Run("zero value", func(t *testing.T) {
		var p Pipeline
		_, err := p.Run(context.Background(), "url")
		if !errors.Is(err, ErrNilStage) {
			t.Errorf("Run error = %v, want ErrNilStage", err)
		}
	})

	t.Run("partial pipeline", func(t *testing.T) {
		src := &transcripttestutil.FakeTranscriptSource{}
		wr := &writertestutil.FakeArticleWriter{}
		p := &Pipeline{source: src, writer: wr} // publisher is nil

		_, err := p.Run(context.Background(), "url")
		if !errors.Is(err, ErrNilStage) {
			t.Errorf("Run error = %v, want ErrNilStage", err)
		}
		if len(src.Calls) != 0 {
			t.Error("source called despite an unset publisher")
		}
		if len(wr.Calls) != 0 {
			t.Error("writer called despite an unset publisher")
		}
	})
}

func TestStageString(t *testing.T) {
	cases := []struct {
		name  string
		stage Stage
		want  string
	}{
		{"zero value", StageUnknown, "unknown"},
		{"transcript", StageTranscript, "transcript"},
		{"write", StageWrite, "write"},
		{"publish", StagePublish, "publish"},
		{"unknown value", unknownStage, "unknown"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.stage.String(); got != tc.want {
				t.Errorf("Stage(%d).String() = %q, want %q", tc.stage, got, tc.want)
			}
		})
	}

	t.Run("StageError uses String", func(t *testing.T) {
		if got := (&StageError{Stage: StageTranscript, Err: errors.New("boom")}).Error(); got != "transcript: boom" {
			t.Errorf("StageError.Error() = %q, want %q", got, "transcript: boom")
		}
		if got := (&StageError{Stage: unknownStage, Err: errors.New("boom")}).Error(); got != "unknown: boom" {
			t.Errorf("StageError.Error() = %q, want %q", got, "unknown: boom")
		}
	})
}

// TestCommonTypesFieldSets fixes the exported fields of the common data types.
func TestCommonTypesFieldSets(t *testing.T) {
	cases := []struct {
		name string
		typ  reflect.Type
		want map[string]string
	}{
		{"Segment", reflect.TypeFor[transcript.Segment](), map[string]string{
			"StartMs": "int64",
			"Text":    "string",
		}},
		{"Transcript", reflect.TypeFor[transcript.Transcript](), map[string]string{
			"VideoID":     "string",
			"VideoURL":    "string",
			"Title":       "string",
			"ChannelName": "string",
			"Description": "string",
			"Segments":    "[]transcript.Segment",
		}},
		{"GenerateRequest", reflect.TypeFor[llm.GenerateRequest](), map[string]string{
			"SystemPrompt":    "string",
			"UserPrompt":      "string",
			"MaxOutputTokens": "int",
		}},
		{"GenerateResponse", reflect.TypeFor[llm.GenerateResponse](), map[string]string{
			"Text":         "string",
			"Model":        "string",
			"ModelVersion": "string",
			"Usage":        "llm.Usage",
		}},
		{"Article", reflect.TypeFor[writer.Article](), map[string]string{
			"Title":        "string",
			"ChannelName":  "string",
			"Body":         "string",
			"SourceURL":    "string",
			"Model":        "string",
			"ModelVersion": "string",
			"Shorten":      "*writer.StepModel",
			"Review":       "*writer.ReviewResult",
			"RunUsage":     "writer.RunUsage",
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.typ.NumField() != len(tc.want) {
				t.Errorf("%s has %d fields, want %d", tc.name, tc.typ.NumField(), len(tc.want))
			}
			got := exportedFieldTypes(tc.typ)
			if !maps.Equal(got, tc.want) {
				t.Errorf("%s fields = %v, want %v", tc.name, got, tc.want)
			}
		})
	}
}

// TestInterfaceContracts checks that every stage method takes a context first.
func TestInterfaceContracts(t *testing.T) {
	ctxType := reflect.TypeFor[context.Context]()
	cases := []struct {
		name  string
		iface reflect.Type
	}{
		{"TranscriptSource", reflect.TypeFor[transcript.TranscriptSource]()},
		{"LLMClient", reflect.TypeFor[llm.LLMClient]()},
		{"ArticleWriter", reflect.TypeFor[writer.ArticleWriter]()},
		{"Publisher", reflect.TypeFor[publisher.Publisher]()},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.iface.NumMethod() != 1 {
				t.Fatalf("%s has %d methods, want 1", tc.name, tc.iface.NumMethod())
			}
			m := tc.iface.Method(0)
			if m.Type.NumIn() < 1 || m.Type.In(0) != ctxType {
				t.Errorf("%s.%s first parameter is not context.Context", tc.name, m.Name)
			}
		})
	}
}

// TestInterfaceDocComments checks the contract clauses in the interface docs.
func TestInterfaceDocComments(t *testing.T) {
	cases := []struct {
		path    string
		name    string
		clauses []string
	}{
		{"../transcript/transcript.go", "TranscriptSource", []string{
			"must return an error on failure",
			"must not return an empty Transcript as a successful result",
		}},
		{"../llm/llm.go", "LLMClient", []string{
			"must not return an empty response without an error",
			"report a failure with ErrInvalidRequest, ErrTruncated, ErrUnexpectedFinishReason, or ErrEmptyResponse",
			"matches context.DeadlineExceeded or context.Canceled",
			"carries the Model and the Usage of the rejected response, and an empty Text and ModelVersion",
			"With any other error it is the zero value, whose Usage is unknown",
		}},
		{"../writer/writer.go", "ArticleWriter", []string{
			"must return an error on failure",
			"must not return an empty Article as a successful result",
			"Write wraps every failure in a *TokenUsageError that carries the usage of the stages that ran, or were reused, before the failure",
		}},
		{"../publisher/publisher.go", "Publisher", []string{
			"must return an error on failure",
			"must not publish incomplete content",
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			doc := strings.Join(strings.Fields(interfaceDoc(t, tc.path, tc.name)), " ")
			for _, clause := range tc.clauses {
				if !strings.Contains(doc, clause) {
					t.Errorf("%s doc comment is missing %q; got:\n%s", tc.name, clause, doc)
				}
			}
			if hasCJK(doc) {
				t.Errorf("%s doc comment contains CJK text:\n%s", tc.name, doc)
			}
		})
	}
}

// TestSecretRevealExclusive checks the exported method set of secret.Secret.
func TestSecretRevealExclusive(t *testing.T) {
	allowed := map[string]struct{}{
		"Reveal":      {},
		"Format":      {},
		"String":      {},
		"GoString":    {},
		"LogValue":    {},
		"MarshalJSON": {},
	}
	typ := reflect.TypeFor[secret.Secret]()
	got := make(map[string]struct{}, typ.NumMethod())
	for m := range typ.Methods() {
		got[m.Name] = struct{}{}
	}
	for m := range reflect.TypeFor[*secret.Secret]().Methods() {
		got[m.Name] = struct{}{}
	}
	if !maps.Equal(got, allowed) {
		t.Errorf("secret.Secret exported methods = %v, want %v", slices.Sorted(maps.Keys(got)), slices.Sorted(maps.Keys(allowed)))
	}
}

// testOnlyPackageDirs are packages that hold only test-only code but are not
// in a testutil/ directory, so the testutil rule does not reach them. The
// values are repository-relative, since the guard walks from two roots. Every
// non-_test.go file in such a directory must carry the "test" build
// constraint; TestPackageReferenceListsPackages requires a row for the
// package.
var testOnlyPackageDirs = []string{
	"internal/doctestutil",
	"internal/loopbacktest",
	"internal/llm/llmhttp/llmhttptest",
	"internal/maketestutil",
	"internal/udstest",
}

// TestFakesCarryBuildTag checks that every testutil file under internal, at
// any depth, every test_helpers*.go file under cmd and internal, and every
// source in a test-only package is test-only. A helper that integration tests
// also use carries "test || integration" instead of "test"; a test-only
// package source carries "test" alone. No other constraint is accepted.
func TestFakesCarryBuildTag(t *testing.T) {
	var testutilFiles, helperFiles, testOnlyFiles []string
	for _, root := range []string{"../../cmd", "../../internal"} {
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() || !strings.HasSuffix(path, ".go") {
				return nil
			}
			switch {
			case slices.Contains(strings.Split(filepath.ToSlash(filepath.Dir(path)), "/"), "testutil"):
				testutilFiles = append(testutilFiles, path)
			case strings.HasPrefix(d.Name(), "test_helpers"):
				helperFiles = append(helperFiles, path)
			case isTestOnlyPackageSource(t, path) && !strings.HasSuffix(d.Name(), "_test.go"):
				testOnlyFiles = append(testOnlyFiles, path)
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", root, err)
		}
	}
	if len(testutilFiles) != 25 {
		t.Fatalf("found %d testutil files, want 25: %v", len(testutilFiles), testutilFiles)
	}
	if len(helperFiles) == 0 {
		t.Fatal("found no test_helpers*.go files")
	}
	if len(testOnlyFiles) == 0 {
		t.Fatal("found no test-only package sources")
	}
	for _, path := range slices.Concat(testutilFiles, helperFiles) {
		t.Run(path, func(t *testing.T) {
			first := firstLineOf(t, path)
			if first != "//go:build test" && first != "//go:build test || integration" {
				t.Errorf("%s first line = %q, want %q or %q", path, first, "//go:build test", "//go:build test || integration")
			}
		})
	}
	for _, path := range testOnlyFiles {
		t.Run(path, func(t *testing.T) {
			if first := firstLineOf(t, path); first != "//go:build test" {
				t.Errorf("%s first line = %q, want %q", path, first, "//go:build test")
			}
		})
	}
}

// isTestOnlyPackageSource reports whether the package of the file at path is
// one of testOnlyPackageDirs.
func isTestOnlyPackageSource(t *testing.T, path string) bool {
	t.Helper()
	rel, err := filepath.Rel(filepath.Join("..", ".."), filepath.Dir(path))
	if err != nil {
		t.Fatalf("relative path of %s: %v", path, err)
	}
	return slices.Contains(testOnlyPackageDirs, filepath.ToSlash(rel))
}

// firstLineOf returns the first line of the file at path.
func firstLineOf(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	first, _, _ := strings.Cut(string(data), "\n")
	return first
}

// TestPackageReferenceListsPackages checks that package_reference.md lists every
// package with any non-test source under cmd/ and internal/ (including a
// package that holds only test-only sources, such as internal/loopbacktest),
// every testutil package, and prompts.
func TestPackageReferenceListsPackages(t *testing.T) {
	const root = "../.."
	found := map[string]bool{}
	add := func(dir string) {
		rel, err := filepath.Rel(root, dir)
		if err != nil {
			t.Fatalf("relative path of %s: %v", dir, err)
		}
		found[filepath.ToSlash(rel)] = true
	}
	for _, base := range []string{"cmd", "internal"} {
		err := filepath.WalkDir(filepath.Join(root, base), func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				if slices.Contains(strings.Split(filepath.ToSlash(path), "/"), "testutil") {
					add(path)
				}
				return nil
			}
			if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			add(filepath.Dir(path))
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", base, err)
		}
	}
	add(filepath.Join(root, "prompts"))
	if len(found) == 0 {
		t.Fatal("found no packages with production code")
	}

	doc, err := os.ReadFile("../../docs/dev/conventions/package_reference.md")
	if err != nil {
		t.Fatalf("read package_reference.md: %v", err)
	}
	listed := packageReferenceRows(string(doc))
	var missing []string
	for pkg := range found {
		if !listed[pkg] {
			missing = append(missing, pkg)
		}
	}
	if len(missing) > 0 {
		slices.Sort(missing)
		t.Errorf("package_reference.md has no row for: %v", missing)
	}
	var stale []string
	for pkg := range listed {
		if !found[pkg] {
			stale = append(stale, pkg)
		}
	}
	if len(stale) > 0 {
		slices.Sort(stale)
		t.Errorf("package_reference.md lists packages that have no production code: %v", stale)
	}
}

// packageReferenceRows returns the package name (the first code span) of every
// table row in package_reference.md.
func packageReferenceRows(doc string) map[string]bool {
	rows := map[string]bool{}
	for line := range strings.Lines(doc) {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "|") {
			continue
		}
		if m := backtickPattern.FindStringSubmatch(trimmed); m != nil {
			rows[m[1]] = true
		}
	}
	return rows
}

// TestPromptsREADMEMatchesContract checks that prompts/README.md states the
// template contract internal/writer enforces: the names a template may
// reference, the builtin functions it may call, and the template and prompt
// size limits. The contract is read from the writer source, not copied here,
// so the README cannot drift from the code unnoticed.
func TestPromptsREADMEMatchesContract(t *testing.T) {
	contract := readWriterTemplateContract(t, "../writer")
	data, err := os.ReadFile("../../prompts/README.md")
	if err != nil {
		t.Fatalf("read README: %v", err)
	}
	sections := markdownSections(string(data))

	t.Run("names", func(t *testing.T) {
		tables := []struct{ dataType, section string }{
			{"templateData", "生成のテンプレートで参照できる値"},
			{"shortenTemplateData", "字数の調整のテンプレートで参照できる値"},
			{"reviewTemplateData", "検証・推敲のテンプレートで参照できる値"},
		}
		for _, table := range tables {
			t.Run(table.dataType, func(t *testing.T) {
				var got []string
				for line := range strings.Lines(sections[table.section]) {
					if strings.HasPrefix(line, "| `") {
						got = append(got, backtickTokens(line)[0])
					}
				}
				fields, ok := contract.fields[table.dataType]
				if !ok || len(fields) == 0 {
					t.Fatalf("no fields read for %s", table.dataType)
				}
				want := make([]string, 0, len(fields))
				for _, field := range fields {
					want = append(want, "."+field)
				}
				if !slices.Equal(slices.Sorted(slices.Values(got)), slices.Sorted(slices.Values(want))) {
					t.Errorf("README names in %q = %v, want %v", table.section, got, want)
				}
			})
		}
	})
	t.Run("functions", func(t *testing.T) {
		got := backtickTokens(sections["使える関数"])
		if !slices.Equal(slices.Sorted(slices.Values(got)), slices.Sorted(slices.Values(contract.funcs))) {
			t.Errorf("README functions = %v, want %v", got, contract.funcs)
		}
	})
	t.Run("limits", func(t *testing.T) {
		rows := []struct {
			prefix string
			limit  int64
		}{
			{"| テンプレート", contract.maxTemplateBytes},
			{"| 展開したプロンプト", contract.maxPromptBytes},
		}
		for _, row := range rows {
			var line string
			for l := range strings.Lines(sections["上限"]) {
				if strings.HasPrefix(l, row.prefix) {
					line = l
				}
			}
			if want := strconv.FormatInt(row.limit, 10) + " バイト"; !strings.Contains(line, want) {
				t.Errorf("README limit row %q = %q, want it to contain %q", row.prefix, line, want)
			}
		}
	})
	t.Run("body lengths", func(t *testing.T) {
		section := sections["字数の調整のテンプレートで参照できる値"]
		for _, want := range []int64{contract.targetBodyChars, contract.maxBodyChars} {
			if !strings.Contains(section, strconv.FormatInt(want, 10)) {
				t.Errorf("README shorten value table does not state the body length %d", want)
			}
		}
	})
}

// TestWriterImports checks that the article writer stays provider
// independent: no Go file directly in internal/writer or prompts, tests
// included, directly imports the network packages or an LLM provider
// package. The LLM test double package is allowed only from test-only files;
// imported from a production file it would break the normal build.
func TestWriterImports(t *testing.T) {
	const (
		module      = "github.com/isseis/yt2column/"
		llmPrefix   = module + "internal/llm/"
		llmTestutil = module + "internal/llm/testutil"
	)
	var paths []string
	for _, pattern := range []string{"../writer/*.go", "../../prompts/*.go"} {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			t.Fatalf("glob %s: %v", pattern, err)
		}
		paths = append(paths, matches...)
	}
	if len(paths) == 0 {
		t.Fatal("found no Go files in internal/writer and prompts")
	}
	fset := token.NewFileSet()
	for _, path := range paths {
		file, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly|parser.ParseComments)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		testOnly := strings.HasSuffix(path, "_test.go") || isTestOnlyFile(file)
		for _, imp := range file.Imports {
			ipath, err := strconv.Unquote(imp.Path.Value)
			if err != nil {
				t.Fatalf("%s: unquote %s: %v", path, imp.Path.Value, err)
			}
			switch {
			case ipath == "net" || strings.HasPrefix(ipath, "net/"):
				t.Errorf("%s imports %s", path, ipath)
			case ipath == llmTestutil && testOnly:
			case strings.HasPrefix(ipath, llmPrefix):
				t.Errorf("%s imports %s", path, ipath)
			}
		}
	}
}

// TestConfigImports checks that the configuration loader and the parameter
// packages stay independent of the network and of the LLM adapters: no
// non-test Go file directly in internal/config, internal/llm/claudeparam, or
// internal/llm/deepseekparam directly imports the network packages. A direct
// import of an adapter would pull net/http into the CLI's startup path. The two
// parameter packages may import neither parameter package nor an adapter;
// internal/config may import the two parameter packages.
func TestConfigImports(t *testing.T) {
	const (
		module            = "github.com/isseis/yt2column/"
		llmPrefix         = module + "internal/llm/"
		claudeparamPath   = module + "internal/llm/claudeparam"
		deepseekparamPath = module + "internal/llm/deepseekparam"
	)
	patterns := []struct {
		glob             string
		parameterPackage bool
	}{
		{"../config/*.go", false},
		{"../llm/claudeparam/*.go", true},
		{"../llm/deepseekparam/*.go", true},
	}
	fset := token.NewFileSet()
	for _, pattern := range patterns {
		matches, err := filepath.Glob(pattern.glob)
		if err != nil {
			t.Fatalf("glob %s: %v", pattern.glob, err)
		}
		// A pattern that matches nothing would silently drop its package from
		// the check, so a renamed or moved package must fail here.
		if len(matches) == 0 {
			t.Fatalf("glob %s matched no files", pattern.glob)
		}
		for _, path := range matches {
			if strings.HasSuffix(path, "_test.go") {
				continue
			}
			file, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly|parser.ParseComments)
			if err != nil {
				t.Fatalf("parse %s: %v", path, err)
			}
			if isTestOnlyFile(file) {
				continue
			}
			for _, imp := range file.Imports {
				ipath, err := strconv.Unquote(imp.Path.Value)
				if err != nil {
					t.Fatalf("%s: unquote %s: %v", path, imp.Path.Value, err)
				}
				switch {
				case ipath == "net" || strings.HasPrefix(ipath, "net/"):
					t.Errorf("%s imports %s", path, ipath)
				case !strings.HasPrefix(ipath, llmPrefix):
					// A package outside internal/llm, including the standard library.
				case ipath == claudeparamPath, ipath == deepseekparamPath:
					if pattern.parameterPackage {
						t.Errorf("%s imports %s; a parameter package must not import another parameter package", path, ipath)
					}
				default:
					t.Errorf("%s imports %s", path, ipath)
				}
			}
		}
	}
}

// templateDataTypes are the data types whose field allowlist the contract
// pins, one per template group.
var templateDataTypes = []string{"templateData", "shortenTemplateData", "reviewTemplateData"}

// writerTemplateContract is the template contract as declared in the
// internal/writer source.
type writerTemplateContract struct {
	fields           map[string][]string // data type name -> exported field names, embedded fields promoted
	funcs            []string            // keys of allowedFuncs
	maxTemplateBytes int64
	maxPromptBytes   int64
	targetBodyChars  int64
	maxBodyChars     int64
}

// readWriterTemplateContract parses the production Go files in dir (test
// files and files built only with the test tag are skipped) and extracts the
// contract. It resolves each data type's fields from the parsed struct types,
// promoting the fields of an embedded struct. A declaration that is missing or
// not in the expected shape fails the test.
func readWriterTemplateContract(t *testing.T, dir string) writerTemplateContract {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join(dir, "*.go"))
	if err != nil {
		t.Fatalf("glob %s: %v", dir, err)
	}
	contract := writerTemplateContract{fields: make(map[string][]string)}
	typeSpecs := make(map[string]*ast.TypeSpec)
	fset := token.NewFileSet()
	var files []*ast.File
	for _, path := range paths {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		if isTestOnlyFile(file) {
			continue
		}
		files = append(files, file)
	}
	for _, file := range files {
		ast.Inspect(file, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.TypeSpec:
				if _, ok := n.Type.(*ast.StructType); ok {
					typeSpecs[n.Name.Name] = n
				}
			case *ast.ValueSpec:
				for i, name := range n.Names {
					switch name.Name {
					case "allowedFuncs":
						contract.funcs = mapLiteralKeys(t, n.Values[i])
					case "maxTemplateBytes":
						contract.maxTemplateBytes = intLiteral(t, n.Values[i])
					case "maxPromptBytes":
						contract.maxPromptBytes = intLiteral(t, n.Values[i])
					case "targetBodyChars":
						contract.targetBodyChars = intLiteral(t, n.Values[i])
					case "maxBodyChars":
						contract.maxBodyChars = intLiteral(t, n.Values[i])
					}
				}
			}
			return true
		})
	}
	for _, name := range templateDataTypes {
		spec, ok := typeSpecs[name]
		if !ok {
			t.Fatalf("template data type %s not found in %s", name, dir)
		}
		contract.fields[name] = structFieldNames(t, spec, typeSpecs)
	}
	if len(contract.funcs) == 0 || contract.maxTemplateBytes == 0 || contract.maxPromptBytes == 0 ||
		contract.targetBodyChars == 0 || contract.maxBodyChars == 0 {
		t.Fatalf("incomplete template contract in %s: %+v", dir, contract)
	}
	return contract
}

func isTestOnlyFile(file *ast.File) bool {
	for _, group := range file.Comments {
		for _, c := range group.List {
			if c.Text == "//go:build test" {
				return true
			}
		}
	}
	return false
}

// structFieldNames returns the field names a template of this data type may
// reference: the named exported fields, and, for an embedded struct, its own
// fields in place of the embedding field.
func structFieldNames(t *testing.T, spec *ast.TypeSpec, typeSpecs map[string]*ast.TypeSpec) []string {
	t.Helper()
	st, ok := spec.Type.(*ast.StructType)
	if !ok {
		t.Fatalf("%s is not a struct", spec.Name.Name)
	}
	var names []string
	for _, field := range st.Fields.List {
		if len(field.Names) > 0 {
			for _, name := range field.Names {
				if name.IsExported() {
					names = append(names, name.Name)
				}
			}
			continue
		}
		// An embedded struct: its fields are promoted and referenced
		// directly by a template.
		ident, ok := field.Type.(*ast.Ident)
		if !ok {
			t.Fatalf("%s has an embedded field that is not a named struct: %T", spec.Name.Name, field.Type)
		}
		embedded, ok := typeSpecs[ident.Name]
		if !ok {
			t.Fatalf("%s embeds the unknown type %s", spec.Name.Name, ident.Name)
		}
		names = append(names, structFieldNames(t, embedded, typeSpecs)...)
	}
	return names
}

func mapLiteralKeys(t *testing.T, expr ast.Expr) []string {
	t.Helper()
	lit, ok := expr.(*ast.CompositeLit)
	if !ok {
		t.Fatalf("allowedFuncs is not a composite literal: %T", expr)
	}
	keys := make([]string, 0, len(lit.Elts))
	for _, elt := range lit.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			t.Fatalf("allowedFuncs element is not key: value: %T", elt)
		}
		key, ok := kv.Key.(*ast.BasicLit)
		if !ok || key.Kind != token.STRING {
			t.Fatalf("allowedFuncs key is not a string literal: %T", kv.Key)
		}
		s, err := strconv.Unquote(key.Value)
		if err != nil {
			t.Fatalf("unquote %s: %v", key.Value, err)
		}
		keys = append(keys, s)
	}
	return keys
}

func intLiteral(t *testing.T, expr ast.Expr) int64 {
	t.Helper()
	lit, ok := expr.(*ast.BasicLit)
	if !ok || lit.Kind != token.INT {
		t.Fatalf("limit is not an integer literal: %T", expr)
	}
	v, err := strconv.ParseInt(lit.Value, 0, 64)
	if err != nil {
		t.Fatalf("parse %s: %v", lit.Value, err)
	}
	return v
}

// markdownSections maps each "## " heading of a Markdown document to the
// text up to the next "## " heading.
func markdownSections(doc string) map[string]string {
	sections := make(map[string]string)
	var heading string
	for line := range strings.Lines(doc) {
		if h, ok := strings.CutPrefix(line, "## "); ok {
			heading = strings.TrimSpace(h)
			continue
		}
		if heading != "" {
			sections[heading] += line
		}
	}
	return sections
}

var backtickPattern = regexp.MustCompile("`([^`]+)`")

func backtickTokens(s string) []string {
	var tokens []string
	for _, m := range backtickPattern.FindAllStringSubmatch(s, -1) {
		tokens = append(tokens, m[1])
	}
	return tokens
}

func exportedFieldTypes(t reflect.Type) map[string]string {
	fields := make(map[string]string, t.NumField())
	for f := range t.Fields() {
		if f.IsExported() {
			fields[f.Name] = f.Type.String()
		}
	}
	return fields
}

func interfaceDoc(t *testing.T, path, name string) string {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.TYPE {
			continue
		}
		for _, spec := range gen.Specs {
			typeSpec, ok := spec.(*ast.TypeSpec)
			if !ok || typeSpec.Name.Name != name {
				continue
			}
			if _, ok := typeSpec.Type.(*ast.InterfaceType); !ok {
				t.Fatalf("%s: %s is not an interface", path, name)
			}
			if gen.Doc == nil {
				t.Fatalf("%s: %s has no doc comment", path, name)
			}
			return gen.Doc.Text()
		}
	}
	t.Fatalf("%s: interface %s not found", path, name)
	return ""
}

func hasCJK(s string) bool {
	for _, r := range s {
		if unicode.Is(unicode.Han, r) || unicode.Is(unicode.Hiragana, r) ||
			unicode.Is(unicode.Katakana, r) || unicode.Is(unicode.Hangul, r) {
			return true
		}
	}
	return false
}
