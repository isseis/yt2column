package devfake

import (
	_ "embed" // the fixed articles are embedded files
	"fmt"
)

// The fixed article texts. They are Japanese, the language of the articles
// the writer produces, so they are files rather than Go string literals. Each
// starts with a level-1 heading whose text starts with TitleMarker. They are
// invented and describe no real video, person, or organization.
var (
	//go:embed articles/standard.md
	standardText string
	//go:embed articles/long_draft.md
	longDraftText string
	//go:embed articles/long_shortened.md
	longShortenedText string
	//go:embed articles/elements.md
	elementsText string
)

// draftText returns the text the draft call answers for article.
func draftText(article Article) (string, error) {
	switch article {
	case ArticleStandard:
		return standardText, nil
	case ArticleLong:
		return longDraftText, nil
	case ArticleElements:
		return elementsText, nil
	default:
		return "", fmt.Errorf("devfake: draft of unknown article %d: %w", int(article), ErrUnexpectedCall)
	}
}
