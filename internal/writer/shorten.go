package writer

import (
	"context"
	"fmt"

	"github.com/isseis/yt2column/internal/llm"
)

// runShorten runs the shorten step: it expands the shorten templates with the
// text's title, body, and length values, calls the generation LLM client once,
// and checks the response with the generation rules. A body still over the
// limit is ErrTooLong. Every failure is a *StepError.
func (w *templateWriter) runShorten(ctx context.Context, title, body string) (string, string, *StepModel, error) {
	data := shortenTemplateData{
		ArticleTitle:    title,
		ArticleBody:     body,
		BodyChars:       bodyChars(body),
		TargetBodyChars: targetBodyChars,
		MaxBodyChars:    maxBodyChars,
	}
	system, user, err := w.shorten.expand(data)
	if err != nil {
		return "", "", nil, stepError(StepShorten, err)
	}
	resp, err := w.client.Generate(ctx, llm.GenerateRequest{SystemPrompt: system, UserPrompt: user, MaxOutputTokens: 0})
	if err != nil {
		return "", "", nil, stepError(StepShorten, err)
	}
	title, body, err = checkResponse(resp)
	if err != nil {
		return "", "", nil, stepError(StepShorten, err)
	}
	if chars := bodyChars(body); chars > maxBodyChars {
		return "", "", nil, stepError(StepShorten, tooLongError(chars))
	}
	return title, body, &StepModel{Model: resp.Model, ModelVersion: resp.ModelVersion}, nil
}

// tooLongError is the ErrTooLong message of a body over the limit. It carries
// the counted length and the limit, both numbers, and never the article text.
func tooLongError(chars int) error {
	return fmt.Errorf("%w: %d characters, limit %d", ErrTooLong, chars, maxBodyChars)
}
