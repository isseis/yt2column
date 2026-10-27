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
