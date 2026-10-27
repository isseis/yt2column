//go:build test

package publisher

import (
	"errors"
	"testing"

	"github.com/isseis/yt2column/internal/writer"
)

func TestDisplayMarkdown(t *testing.T) {
	withChannel := validArticle()
	withChannel.ChannelName = "Example Channel"

	cases := []struct {
		name    string
		article writer.Article
		want    string
	}{
		{"without a channel name", validArticle(), "# A title\n\n" + validArticle().Body},
		{"with a channel name", withChannel, "# A title\n\nExample Channel\n\n" + withChannel.Body},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := DisplayMarkdown(tc.article)
			if err != nil {
				t.Fatalf("DisplayMarkdown error = %v, want nil", err)
			}
			if got != tc.want {
				t.Errorf("DisplayMarkdown =\n%q\nwant\n%q", got, tc.want)
			}
		})
	}
}

// TestDisplayMarkdownRejectsUnpublishable checks that DisplayMarkdown runs
// CheckPublishable: an article it rejects yields no text. FileContent's check
// is covered through FilePublisher by TestFilePublisherRejectsArticle.
func TestDisplayMarkdownRejectsUnpublishable(t *testing.T) {
	article := validArticle()
	article.Title = "a\nb"
	got, err := DisplayMarkdown(article)
	if !errors.Is(err, writer.ErrInvalidArticle) {
		t.Errorf("DisplayMarkdown error = %v, want ErrInvalidArticle", err)
	}
	if got != "" {
		t.Errorf("DisplayMarkdown = %q, want empty", got)
	}
}
