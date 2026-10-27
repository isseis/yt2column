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
	escapedChannel := validArticle()
	escapedChannel.ChannelName = "# News"

	cases := []struct {
		name    string
		article writer.Article
		want    string
	}{
		{"without a channel name", validArticle(), "# A title\n\n" + validArticle().Body},
		{"with a channel name", withChannel, "# A title\n\nExample Channel\n\n" + withChannel.Body},
		{"channel name escaped", escapedChannel, "# A title\n\n\\# News\n\n" + escapedChannel.Body},
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

// TestRenderersRejectUnpublishable checks that the two exported renderers run
// CheckPublishable: an article it rejects yields no text.
func TestRenderersRejectUnpublishable(t *testing.T) {
	article := validArticle()
	article.Title = "a\nb"
	renderers := map[string]func(writer.Article) (string, error){
		"DisplayMarkdown": DisplayMarkdown,
		"FileContent":     FileContent,
	}
	for name, render := range renderers {
		t.Run(name, func(t *testing.T) {
			got, err := render(article)
			if !errors.Is(err, writer.ErrInvalidArticle) {
				t.Errorf("%s error = %v, want ErrInvalidArticle", name, err)
			}
			if got != "" {
				t.Errorf("%s = %q, want empty", name, got)
			}
		})
	}
}
