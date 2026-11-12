//go:build test

package devfake_test

import (
	"context"
	"strings"
	"testing"

	"github.com/isseis/yt2column/internal/llm/devfake"
	"github.com/isseis/yt2column/internal/transcript"
	"github.com/isseis/yt2column/internal/writer"
	writertestutil "github.com/isseis/yt2column/internal/writer/testutil"
)

// sampleTranscript is a transcript the writer accepts. The fake reads no
// prompt, so its content does not change the article.
func sampleTranscript(t *testing.T) transcript.Transcript {
	t.Helper()
	const videoID = "devfakeTest"
	url, ok := transcript.NormalizedVideoURL(videoID)
	if !ok {
		t.Fatalf("NormalizedVideoURL(%q) rejected the video ID", videoID)
	}
	return transcript.Transcript{
		VideoID:  videoID,
		VideoURL: url,
		Title:    "sample video",
		Segments: []transcript.Segment{{StartMs: 0, Text: "sample segment"}},
	}
}

// headingCount counts the section headings ("## ") of body.
func headingCount(body string) int {
	n := 0
	for line := range strings.Lines(body) {
		if strings.HasPrefix(line, "## ") {
			n++
		}
	}
	return n
}

// TestArticlesPassWriter runs each fixed article through the real writer with
// the production options. The writer rejects a body over its length limit
// after the shorten step and again after the review step, so an article that
// completes is within the limit.
func TestArticlesPassWriter(t *testing.T) {
	articles := map[devfake.Article]writer.Article{}
	for _, kind := range devfake.Articles() {
		t.Run(kind.String(), func(t *testing.T) {
			s, err := devfake.NewScenario(kind, devfake.FailureNone, devfake.FailAtNone, 0)
			if err != nil {
				t.Fatal(err)
			}
			job := s.NewJob()
			w, err := writer.NewProgressWriter(job.Generation(), writertestutil.DefaultOptions(job.Review()))
			if err != nil {
				t.Fatalf("NewProgressWriter() error = %v", err)
			}
			article, err := w.Write(context.Background(), sampleTranscript(t))
			if err != nil {
				t.Fatalf("Write() error = %v", err)
			}
			if !strings.HasPrefix(article.Title, devfake.TitleMarker) {
				t.Errorf("Title does not start with %q", devfake.TitleMarker)
			}
			if article.Model != devfake.ModelName {
				t.Errorf("Model = %q, want %q", article.Model, devfake.ModelName)
			}
			if article.Review == nil {
				t.Error("the review step did not run")
			}
			shortened := article.Shorten != nil
			if want := kind == devfake.ArticleLong; shortened != want {
				t.Errorf("shorten step ran = %v, want %v", shortened, want)
			}
			articles[kind] = article
		})
	}

	standard, ok1 := articles[devfake.ArticleStandard]
	long, ok2 := articles[devfake.ArticleLong]
	if !ok1 || !ok2 {
		t.Fatal("the standard or the long article did not complete")
	}
	if n := headingCount(standard.Body); n < 2 {
		t.Errorf("standard article has %d section headings, want at least 2", n)
	}
	if ns, nl := headingCount(standard.Body), headingCount(long.Body); nl <= ns {
		t.Errorf("long article has %d section headings, want more than the standard article's %d", nl, ns)
	}
}
