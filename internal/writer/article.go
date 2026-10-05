package writer

import (
	"fmt"
	"strings"
	"unicode"
)

// CheckPublishable reports whether a publisher may output a. It wraps
// ErrInvalidArticle, names the broken rule only, and never puts a field value
// in the error, so a rejected article cannot leak its content into a log.
// ModelVersion may be empty; the other fields may not.
func (a Article) CheckPublishable() error {
	if reason := displayStringProblem("title", a.Title); reason != "" {
		return invalidArticle(reason)
	}
	if reason := displayStringProblem("model", a.Model); reason != "" {
		return invalidArticle(reason)
	}
	if a.ModelVersion != "" {
		if reason := displayStringProblem("model version", a.ModelVersion); reason != "" {
			return invalidArticle(reason)
		}
	}
	if strings.TrimSpace(a.Body) == "" {
		return invalidArticle("body is empty or whitespace only")
	}
	// A body is Markdown text: a line feed and a tab are content, any other
	// control character could drive a terminal that displays the file.
	if strings.ContainsFunc(a.Body, func(r rune) bool {
		return unicode.IsControl(r) && r != '\n' && r != '\t'
	}) {
		return invalidArticle("body contains a control character")
	}
	if a.SourceURL == "" {
		return invalidArticle("source URL is empty")
	}
	return nil
}

func invalidArticle(reason string) error {
	return fmt.Errorf("%w: %s", ErrInvalidArticle, reason)
}
