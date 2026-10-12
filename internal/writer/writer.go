// Package writer defines the article-writing stage: the article type and the
// interface that produces it from a transcript.
package writer

import (
	"context"
	"fmt"

	"github.com/isseis/yt2column/internal/llm"
	"github.com/isseis/yt2column/internal/nilcheck"
	"github.com/isseis/yt2column/internal/transcript"
	"github.com/isseis/yt2column/prompts"
)

// Article is the generated column article. It stays comparable with ==:
// the step results are pointers, nil when the step called no LLM.
type Article struct {
	Title        string
	Body         string // Markdown
	SourceURL    string
	Model        string        // the generation model
	ModelVersion string        // opaque, provider-defined; empty when the provider reports none
	Shorten      *StepModel    // nil when the shorten step called no LLM
	Review       *ReviewResult // nil when the review step did not run
}

// StepModel is the model one step's LLM call reported.
type StepModel struct {
	Model        string
	ModelVersion string // empty when the provider reports none
}

// ArticleWriter generates a column article from a transcript.
// Implementations must return an error on failure and must not
// return an empty Article as a successful result.
type ArticleWriter interface {
	Write(ctx context.Context, t transcript.Transcript) (Article, error)
}

// Options configures the ArticleWriter returned by New. An empty template path
// selects the embedded default template; a non-empty path names an override
// file that New reads once. The zero value generates the article only:
// neither post-generation step runs.
type Options struct {
	SystemTemplatePath string
	UserTemplatePath   string
	Shorten            ShortenOptions
	Review             ReviewOptions
}

// ShortenOptions configures the shorten step, which uses the generation
// LLMClient passed to New. Template paths follow the rule of
// Options.SystemTemplatePath: empty selects the embedded default.
type ShortenOptions struct {
	Enabled            bool
	SystemTemplatePath string
	UserTemplatePath   string
}

// ReviewOptions configures the review step. Client is required when Enabled is
// true and must be nil when it is false.
type ReviewOptions struct {
	Enabled            bool
	Client             llm.LLMClient
	SystemTemplatePath string
	UserTemplatePath   string
}

// stepTemplates is the checked system and user template pair of one step. Only
// New builds one, and it expands the pair with the step's own data type.
type stepTemplates[T any] struct {
	system checkedTemplate[T]
	user   checkedTemplate[T]
}

// expand expands both templates of the pair with data.
func (p stepTemplates[T]) expand(data T) (system, user string, err error) {
	system, err = p.system.expand(data)
	if err != nil {
		return "", "", err
	}
	user, err = p.user.expand(data)
	return system, user, err
}

// templateWriter is the ArticleWriter returned by New. Its fields are set once
// by New and never mutated afterwards. A nil shorten or review means the step
// is disabled.
type templateWriter struct {
	client       llm.LLMClient
	generation   stepTemplates[templateData]
	shorten      *stepTemplates[shortenTemplateData]
	review       *stepTemplates[reviewTemplateData]
	reviewClient llm.LLMClient
}

// New validates client, the options, and the templates of the enabled steps,
// and returns an ArticleWriter. It never reads environment variables. The
// concrete type is unexported, so an ArticleWriter can only be obtained
// through New.
func New(client llm.LLMClient, opts Options) (ArticleWriter, error) {
	if nilcheck.IsNil(client) {
		return nil, errNilLLMClient
	}
	if err := validateStepOptions(opts); err != nil {
		return nil, err
	}
	system, err := loadTemplate[templateData](templateSource{name: systemTemplateName, path: opts.SystemTemplatePath}, prompts.System())
	if err != nil {
		return nil, err
	}
	user, err := loadTemplate[templateData](templateSource{name: userTemplateName, path: opts.UserTemplatePath}, prompts.User())
	if err != nil {
		return nil, err
	}
	w := &templateWriter{
		client:     client,
		generation: stepTemplates[templateData]{system: system, user: user},
	}
	if opts.Shorten.Enabled {
		shorten, err := loadStepTemplates[shortenTemplateData](
			shortenSystemTemplateName, opts.Shorten.SystemTemplatePath, prompts.ShortenSystem(),
			shortenUserTemplateName, opts.Shorten.UserTemplatePath, prompts.ShortenUser(),
		)
		if err != nil {
			return nil, err
		}
		w.shorten = &shorten
	}
	if opts.Review.Enabled {
		review, err := loadStepTemplates[reviewTemplateData](
			reviewSystemTemplateName, opts.Review.SystemTemplatePath, prompts.ReviewSystem(),
			reviewUserTemplateName, opts.Review.UserTemplatePath, prompts.ReviewUser(),
		)
		if err != nil {
			return nil, err
		}
		w.review = &review
		w.reviewClient = opts.Review.Client
	}
	return w, nil
}

// validateStepOptions rejects the step combinations New must not accept: a
// review step whose client is missing, a review client set while the step is
// disabled, and a template path for a disabled step. Silently ignoring such a
// setting would hide the caller's mistake.
func validateStepOptions(opts Options) error {
	if opts.Review.Enabled {
		if nilcheck.IsNil(opts.Review.Client) {
			return errReviewClientNil
		}
	} else if !nilcheck.IsNil(opts.Review.Client) {
		return errReviewClientSet
	}
	if !opts.Shorten.Enabled && (opts.Shorten.SystemTemplatePath != "" || opts.Shorten.UserTemplatePath != "") {
		return errDisabledStepTemplate
	}
	if !opts.Review.Enabled && (opts.Review.SystemTemplatePath != "" || opts.Review.UserTemplatePath != "") {
		return errDisabledStepTemplate
	}
	return nil
}

// loadStepTemplates loads and checks the system and user template of one step.
func loadStepTemplates[T any](systemName, systemPath, systemEmbedded, userName, userPath, userEmbedded string) (stepTemplates[T], error) {
	system, err := loadTemplate[T](templateSource{name: systemName, path: systemPath}, systemEmbedded)
	if err != nil {
		return stepTemplates[T]{}, err
	}
	user, err := loadTemplate[T](templateSource{name: userName, path: userPath}, userEmbedded)
	if err != nil {
		return stepTemplates[T]{}, err
	}
	return stepTemplates[T]{system: system, user: user}, nil
}

// Write validates t, expands the generation templates, calls the generation
// LLM client once, and validates its response; then it runs the steps Options
// enabled, in order (shorten, review, the final length check), and builds the
// article. It returns the zero Article on every failure. An LLM client's error
// is wrapped as is, never with a sentinel of this package, so callers can tell
// it from a template or output failure; every failure of a step is a
// *StepError.
func (w *templateWriter) Write(ctx context.Context, t transcript.Transcript) (Article, error) {
	if err := ctx.Err(); err != nil {
		return Article{}, fmt.Errorf("article generation not started: %w", err)
	}
	sourceURL, err := validateTranscript(t)
	if err != nil {
		return Article{}, err
	}
	data := newTemplateData(t)
	system, user, err := w.generation.expand(data)
	if err != nil {
		return Article{}, err
	}
	resp, err := w.client.Generate(ctx, llm.GenerateRequest{SystemPrompt: system, UserPrompt: user, MaxOutputTokens: 0})
	if err != nil {
		return Article{}, fmt.Errorf("LLM generation failed: %w", err)
	}
	title, body, err := checkResponse(resp)
	if err != nil {
		return Article{}, err
	}
	var shortenModel *StepModel
	if w.shorten != nil && bodyChars(body) > maxBodyChars {
		if err := ctx.Err(); err != nil {
			return Article{}, stepError(StepShorten, err)
		}
		title, body, shortenModel, err = w.runShorten(ctx, title, body)
		if err != nil {
			return Article{}, err
		}
	}
	var reviewResult *ReviewResult
	if w.review != nil {
		if err := ctx.Err(); err != nil {
			return Article{}, stepError(StepReview, err)
		}
		title, body, reviewResult, err = w.runReview(ctx, data, title, body)
		if err != nil {
			return Article{}, err
		}
	}
	if w.shorten != nil {
		if chars := bodyChars(body); chars > maxBodyChars {
			return Article{}, stepError(StepReview, tooLongError(chars))
		}
	}
	return newArticle(title, body, sourceURL, resp, shortenModel, reviewResult), nil
}
