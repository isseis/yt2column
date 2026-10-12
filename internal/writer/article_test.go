//go:build test

package writer

import (
	"errors"
	"strings"
	"testing"
)

// validPublishableArticle is an article every rule accepts.
func validPublishableArticle() Article {
	return Article{
		Title:        "A title",
		Body:         "\nline one\n\tindented\n",
		SourceURL:    testSourceURL,
		Model:        "model-x",
		ModelVersion: "2026-01-01",
	}
}

func TestArticleCheckPublishable(t *testing.T) {
	cases := []struct {
		name    string
		mutate  func(a *Article)
		wantErr bool
	}{
		{"valid", func(*Article) {}, false},
		{"model version empty", func(a *Article) { a.ModelVersion = "" }, false},
		{"body with newline and tab", func(a *Article) { a.Body = "a\n\tb\n" }, false},
		{"title empty", func(a *Article) { a.Title = "" }, true},
		{"title whitespace only", func(a *Article) { a.Title = " 　 " }, true},
		{"title newline", func(a *Article) { a.Title = "a\nb" }, true},
		{"title tab", func(a *Article) { a.Title = "a\tb" }, true},
		{"title carriage return", func(a *Article) { a.Title = "a\rb" }, true},
		{"title escape", func(a *Article) { a.Title = "a\x1bb" }, true},
		{"model empty", func(a *Article) { a.Model = "" }, true},
		{"model whitespace only", func(a *Article) { a.Model = "  " }, true},
		{"model control", func(a *Article) { a.Model = "m\rx" }, true},
		{"model version whitespace only", func(a *Article) { a.ModelVersion = " " }, true},
		{"model version control", func(a *Article) { a.ModelVersion = "v\x00" }, true},
		{"body empty", func(a *Article) { a.Body = "" }, true},
		{"body whitespace only", func(a *Article) { a.Body = "\n\t \n" }, true},
		{"body escape", func(a *Article) { a.Body = "text\x1b[2J" }, true},
		{"body carriage return", func(a *Article) { a.Body = "a\rb" }, true},
		{"body delete", func(a *Article) { a.Body = "a\x7fb" }, true},
		{"source URL empty", func(a *Article) { a.SourceURL = "" }, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a := validPublishableArticle()
			tc.mutate(&a)
			err := a.CheckPublishable()
			if !tc.wantErr {
				if err != nil {
					t.Fatalf("CheckPublishable() = %v, want nil", err)
				}
				return
			}
			if !errors.Is(err, ErrInvalidArticle) {
				t.Fatalf("CheckPublishable() = %v, want ErrInvalidArticle", err)
			}
			if errors.Is(err, ErrMalformedOutput) {
				t.Errorf("CheckPublishable() error wraps ErrMalformedOutput: %v", err)
			}
		})
	}
}

func TestArticleCheckPublishableSteps(t *testing.T) {
	withSteps := func() Article {
		a := validPublishableArticle()
		a.Shorten = &StepModel{Model: "shorten-model"}
		a.Review = &ReviewResult{Model: StepModel{Model: "review-model", ModelVersion: "v1"}}
		return a
	}

	accepted := []struct {
		name   string
		mutate func(a *Article)
	}{
		{"no steps", func(*Article) {}},
		{"both steps", func(a *Article) { *a = withSteps() }},
		{"review with no revisions", func(a *Article) {
			*a = withSteps()
			a.Review.Revisions = nil
		}},
		{"step model versions empty", func(a *Article) {
			*a = withSteps()
			a.Shorten.ModelVersion = ""
			a.Review.Model.ModelVersion = ""
		}},
	}
	for _, tc := range accepted {
		t.Run("accepts/"+tc.name, func(t *testing.T) {
			a := validPublishableArticle()
			tc.mutate(&a)
			if err := a.CheckPublishable(); err != nil {
				t.Errorf("CheckPublishable() = %v, want nil", err)
			}
		})
	}

	rejected := []struct {
		name       string
		mutate     func(a *Article)
		wantReview bool
	}{
		{"shorten model control character", func(a *Article) {
			*a = withSteps()
			a.Shorten.Model = "shorten\rx"
		}, false},
		{"shorten model version control character", func(a *Article) {
			*a = withSteps()
			a.Shorten.ModelVersion = "v\x00"
		}, false},
		{"review model control character", func(a *Article) {
			*a = withSteps()
			a.Review.Model.Model = "review\x1b[31m"
		}, false},
		{"review model version control character", func(a *Article) {
			*a = withSteps()
			a.Review.Model.ModelVersion = "v\n"
		}, false},
		{"review result check fails", func(a *Article) {
			*a = withSteps()
			a.Review.Revisions = []Revision{{Before: "b", Action: ActionUnknown, Reason: ReasonMeaningChange, Evidence: "e"}}
		}, true},
	}
	for _, tc := range rejected {
		t.Run("rejects/"+tc.name, func(t *testing.T) {
			a := validPublishableArticle()
			tc.mutate(&a)
			err := a.CheckPublishable()
			if !errors.Is(err, ErrInvalidArticle) {
				t.Fatalf("CheckPublishable() = %v, want ErrInvalidArticle", err)
			}
			if got := errors.Is(err, ErrInvalidReview); got != tc.wantReview {
				t.Errorf("CheckPublishable() matches ErrInvalidReview = %v, want %v", got, tc.wantReview)
			}
		})
	}
}

func TestArticleCheckPublishableOmitsValues(t *testing.T) {
	const mark = "mark-secret-Q4wd"
	cases := []struct {
		name   string
		label  string
		mutate func(a *Article)
	}{
		{"title", "title", func(a *Article) { a.Title = mark + "\n" }},
		{"model", "model", func(a *Article) { a.Model = mark + "\n" }},
		{"model version", "model version", func(a *Article) { a.ModelVersion = mark + "\n" }},
		{"body", "body", func(a *Article) { a.Body = mark + "\x1b" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a := validPublishableArticle()
			tc.mutate(&a)
			err := a.CheckPublishable()
			if !errors.Is(err, ErrInvalidArticle) {
				t.Fatalf("CheckPublishable() = %v, want ErrInvalidArticle", err)
			}
			if strings.Contains(err.Error(), mark) {
				t.Errorf("error %q contains the field value", err)
			}
			if !strings.Contains(err.Error(), tc.label+" contains a control character") {
				t.Errorf("error %q does not name the %s and the broken rule", err, tc.label)
			}
		})
	}
}
