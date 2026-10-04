package writer

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/isseis/yt2column/internal/llm"
)

const (
	// maxTextBytes caps the generated text.
	maxTextBytes = 1048576 // 1 MiB
	// maxModelBytes caps Model and ModelVersion, each.
	maxModelBytes = 256
	// titlePrefix starts the first line of the generated text: one "#" and
	// one space, a level-1 ATX heading.
	titlePrefix = "# "
	// titleTrimChars are removed around the title; nothing else is
	// normalized.
	titleTrimChars = " \t"
	// sourceLabel leads the source block. It is part of the article text,
	// which is Japanese, not a message, so it is not English.
	sourceLabel = "出典: "
	// bodySeparator always ends the generated body's last line and adds a
	// blank line, so the source block is its own paragraph however the body
	// ends.
	bodySeparator = "\n\n"
)

// checkResponse validates the generated text, Model, and ModelVersion in a
// fixed order and returns the title and the body part (everything after the
// first line's "\n"). Nothing is repaired: a value that breaks a rule is
// rejected. Errors wrap ErrMalformedOutput and name the broken rule only,
// never a value, because the response is untrusted.
func checkResponse(resp llm.GenerateResponse) (title, body string, err error) {
	if len(resp.Text) > maxTextBytes {
		return "", "", fmt.Errorf("%w: generated text exceeds the limit of %d bytes", ErrMalformedOutput, maxTextBytes)
	}
	if !utf8.ValidString(resp.Text) {
		return "", "", fmt.Errorf("%w: generated text is not valid UTF-8", ErrMalformedOutput)
	}
	firstLine, body, _ := strings.Cut(resp.Text, "\n")
	heading, ok := strings.CutPrefix(firstLine, titlePrefix)
	if !ok {
		return "", "", fmt.Errorf("%w: the first line is not a level-1 heading starting with %q", ErrMalformedOutput, titlePrefix)
	}
	title = strings.Trim(heading, titleTrimChars)
	if err := checkDisplayString("title", title); err != nil {
		return "", "", err
	}
	// A closing "#" sequence is neither interpreted nor removed.
	if strings.HasSuffix(title, "#") {
		return "", "", fmt.Errorf("%w: title ends with \"#\"", ErrMalformedOutput)
	}
	if strings.TrimSpace(body) == "" {
		return "", "", fmt.Errorf("%w: body is empty or whitespace only", ErrMalformedOutput)
	}
	if err := checkBodyMarkdown(body); err != nil {
		return "", "", fmt.Errorf("%w: body %w", ErrMalformedOutput, err)
	}
	if err := checkModelString("Model", resp.Model); err != nil {
		return "", "", err
	}
	if resp.ModelVersion != "" {
		if err := checkModelString("ModelVersion", resp.ModelVersion); err != nil {
			return "", "", err
		}
	}
	return title, body, nil
}

// checkModelString applies the size and UTF-8 rules of Model and
// ModelVersion, then the rules shared with the title.
func checkModelString(name, value string) error {
	if len(value) > maxModelBytes {
		return fmt.Errorf("%w: %s exceeds the limit of %d bytes", ErrMalformedOutput, name, maxModelBytes)
	}
	if !utf8.ValidString(value) {
		return fmt.Errorf("%w: %s is not valid UTF-8", ErrMalformedOutput, name)
	}
	return checkDisplayString(name, value)
}

// checkDisplayString applies the rules shared by the title, Model, and
// ModelVersion, which are shown as-is on terminals and destinations: it has
// a non-whitespace character, and no control character (Unicode Cc, which
// includes tab, CR, LF, and ESC).
func checkDisplayString(name, value string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("%w: %s is empty or whitespace only", ErrMalformedOutput, name)
	}
	if strings.ContainsFunc(value, unicode.IsControl) {
		return fmt.Errorf("%w: %s contains a control character", ErrMalformedOutput, name)
	}
	return nil
}

// newArticle builds the article from validated values. Body is the generated
// body unmodified, bodySeparator, then the source block, whose URL comes from
// the validated VideoID, never from the generated text.
func newArticle(title, body, sourceURL string, resp llm.GenerateResponse) Article {
	return Article{
		Title:        title,
		Body:         body + bodySeparator + sourceBlock(sourceURL),
		SourceURL:    sourceURL,
		Model:        resp.Model,
		ModelVersion: resp.ModelVersion,
	}
}

// sourceBlock is the fixed-format source line, an autolink to sourceURL,
// followed by one "\n".
func sourceBlock(sourceURL string) string {
	return sourceLabel + "<" + sourceURL + ">\n"
}
