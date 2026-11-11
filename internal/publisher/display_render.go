package publisher

import (
	"fmt"
	"strings"

	"github.com/isseis/yt2column/internal/writer"
)

// DisplayMarkdown returns the Markdown the Viewer displays for a: the title
// heading, the channel name line when there is one, and the body (which ends
// with the source block). It runs the same CheckPublishable check as
// FileContent. The strings are placed as they are: nothing is converted to
// HTML or sanitized here.
func DisplayMarkdown(a writer.Article) (string, error) {
	if err := a.CheckPublishable(); err != nil {
		return "", err
	}
	return renderDisplayMarkdown(a), nil
}

// NeutralizedDisplay is the display Markdown of an article with its mentions
// neutralized, as the Viewer displays and copies it.
type NeutralizedDisplay struct {
	// Markdown is the display Markdown with U+200B inserted after each
	// neutralized '@' unit; removing those insertions gives the display
	// Markdown renderDisplayMarkdown returns.
	Markdown string
	// Neutralized reports whether at least one U+200B was inserted. A U+200B
	// the display Markdown already held does not count.
	Neutralized bool
}

// NeutralizedDisplayMarkdown runs CheckPublishable on a and returns its
// neutralized display Markdown: every target '@' unit of the title heading,
// the channel name line, and the body, code included, gets a U+200B, except
// a URL path '/@'. Mattermost then reads none of them as a mention, whether
// the reader pastes the copied Markdown or text selected from the rendered
// article, which has lost its markup. The rule is in neutralizeAts.
func NeutralizedDisplayMarkdown(a writer.Article) (NeutralizedDisplay, error) {
	if err := a.CheckPublishable(); err != nil {
		return NeutralizedDisplay{}, err
	}
	markdown, neutralized := neutralizeAts(renderDisplayMarkdown(a), true)
	return NeutralizedDisplay{Markdown: markdown, Neutralized: neutralized}, nil
}

// NeutralizeHeading returns value, a video title or channel name for the
// Viewer's heading, with every target '@' unit neutralized, URL-path ones
// included: a heading rarely holds a URL, and keeping its rule simple is
// worth more than keeping such a URL whole.
func NeutralizeHeading(value string) string {
	neutralized, _ := neutralizeAts(value, false)
	return neutralized
}

// renderDisplayMarkdown returns the title heading, a blank line, the channel
// name line and a blank line when there is a channel name, then the body.
// renderFileArticle writes it after the front matter, so it is byte for byte
// the part of the --out file after the front matter. The channel name line
// goes through escapeLeadingMarkdown, so the rendered line equals the channel
// name.
func renderDisplayMarkdown(a writer.Article) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n", a.Title)
	if a.ChannelName != "" {
		fmt.Fprintf(&b, "%s\n\n", escapeLeadingMarkdown(a.ChannelName))
	}
	b.WriteString(a.Body)
	return b.String()
}
