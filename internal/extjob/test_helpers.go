//go:build test

package extjob

import (
	"context"
	"testing"

	"github.com/isseis/yt2column/internal/config"
	"github.com/isseis/yt2column/internal/writer"
)

// The values the tests of this package share.
const (
	TestVideoID  = "dQw4w9WgXcQ"
	TestVideoURL = "https://www.youtube.com/watch?v=dQw4w9WgXcQ"
	TestRawText  = "0:00\nHello\n"
)

// MaxResultBytesForTest is the size limit of a done result's JSON.
const MaxResultBytesForTest = maxResultBytes

// MaxResultSizeForTest returns the longest done result JSON of a, in bytes.
func MaxResultSizeForTest(a writer.Article, videoTitle string) (int, error) {
	return maxResultSize(a, videoTitle)
}

// fakeBackend is a Backend that records what it is given and returns what the
// test sets.
type fakeBackend struct {
	checkErr    error
	clientsErr  error
	deps        Deps
	info        BackendInfo
	checkCalls  int
	checkCtx    context.Context
	clientCalls int
	lastConfig  config.LLMConfig
}

func (b *fakeBackend) NewClients(cfg config.LLMConfig) (Deps, error) {
	b.clientCalls++
	b.lastConfig = cfg
	return b.deps, b.clientsErr
}

func (b *fakeBackend) CheckKey(ctx context.Context, cfg config.LLMConfig) error {
	b.checkCalls++
	b.checkCtx = ctx
	b.lastConfig = cfg
	return b.checkErr
}

func (b *fakeBackend) Info() BackendInfo { return b.info }

// testArticle returns an article every check accepts.
func testArticle() writer.Article {
	return writer.Article{
		Title:       "Title",
		ChannelName: "Channel",
		Body:        "Body text.\n\n" + "Source: " + TestVideoURL + "\n",
		SourceURL:   TestVideoURL,
		Model:       "model",
	}
}

// mustPrepare prepares the standard input or fails the test.
func mustPrepare(t *testing.T, in StartInput) Prepared {
	t.Helper()
	if in.VideoURL == "" {
		in.VideoURL = TestVideoURL
	}
	if in.Transcript == "" {
		in.Transcript = TestRawText
	}
	p, err := Prepare(in)
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	return p
}

// recordingObserver records every step change of a run.
type recordingObserver struct{ changes []Steps }

func (o *recordingObserver) StepChanged(s Steps) { o.changes = append(o.changes, s) }

// fakeWriterFactory returns the newWriter argument of run that gives w.
func fakeWriterFactory(w writer.ProgressWriter) func() (writer.ProgressWriter, error) {
	return func() (writer.ProgressWriter, error) { return w, nil }
}
