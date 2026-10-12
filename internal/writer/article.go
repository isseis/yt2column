package writer

import (
	"fmt"
	"strings"
	"unicode"
)

// CheckPublishable reports whether a publisher may output a. It wraps
// ErrInvalidArticle, names the broken rule only, and never puts a field value
// in the error, so a rejected article cannot leak its content into a log.
// ModelVersion may be empty and Shorten and Review may be nil; the other fields
// may not be empty. A non-nil step result is checked like Model and
// ModelVersion, and Review also with ReviewResult.Check.
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
	if a.Shorten != nil {
		if err := checkStepModel("shorten", a.Shorten.Model, a.Shorten.ModelVersion); err != nil {
			return err
		}
	}
	if a.Review != nil {
		if err := checkStepModel("review", a.Review.Model.Model, a.Review.Model.ModelVersion); err != nil {
			return err
		}
		if err := a.Review.Check(); err != nil {
			return fmt.Errorf("%w: %w", ErrInvalidArticle, err)
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

// checkStepModel applies the display-string rules to a step's model name and
// its optional model version, both of which go into the output's model lines.
func checkStepModel(step, model, version string) error {
	if reason := displayStringProblem(step+" model", model); reason != "" {
		return invalidArticle(reason)
	}
	if version != "" {
		if reason := displayStringProblem(step+" model version", version); reason != "" {
			return invalidArticle(reason)
		}
	}
	return nil
}

func invalidArticle(reason string) error {
	return fmt.Errorf("%w: %s", ErrInvalidArticle, reason)
}
