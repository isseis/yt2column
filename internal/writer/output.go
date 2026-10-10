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
	// byteOrderMark is U+FEFF, which normalizeBody removes from the start of
	// the body part.
	byteOrderMark = "\uFEFF"
)

// bodyLineEndings rewrites the CommonMark line endings "\r\n" and "\r" to
// "\n". "\r\n" is listed first so it is replaced as one line ending.
var bodyLineEndings = strings.NewReplacer("\r\n", "\n", "\r", "\n")

// checkGeneratedText validates the generated text and returns the title and
// the body part (everything after the first line's "\n", passed through
// normalizeBody before its checks). Nothing else is repaired: a value that
// breaks a rule is rejected. Errors wrap ErrMalformedOutput and name the broken
// rule only, never a value, because the response is untrusted.
func checkGeneratedText(text string) (title, body string, err error) {
	if len(text) > maxTextBytes {
		return "", "", fmt.Errorf("%w: generated text exceeds the limit of %d bytes", ErrMalformedOutput, maxTextBytes)
	}
	if !utf8.ValidString(text) {
		return "", "", fmt.Errorf("%w: generated text is not valid UTF-8", ErrMalformedOutput)
	}
	firstLine, body, _ := strings.Cut(text, "\n")
	heading, ok := strings.CutPrefix(firstLine, titlePrefix)
	if !ok {
		return "", "", fmt.Errorf("%w: the first line is not a level-1 heading starting with %q", ErrMalformedOutput, titlePrefix)
	}
	title = strings.Trim(heading, titleTrimChars)
	if reason := displayStringProblem("title", title); reason != "" {
		return "", "", fmt.Errorf("%w: %s", ErrMalformedOutput, reason)
	}
	// A closing "#" sequence is neither interpreted nor removed.
	if strings.HasSuffix(title, "#") {
		return "", "", fmt.Errorf("%w: title ends with \"#\"", ErrMalformedOutput)
	}
	body = normalizeBody(body)
	if strings.TrimSpace(body) == "" {
		return "", "", fmt.Errorf("%w: body is empty or whitespace only", ErrMalformedOutput)
	}
	// normalizeBody removes one BOM, as cmark does. Another one left at the
	// start would be content to the check but could be stripped by a renderer,
	// opening a fence the check read as prose.
	if strings.HasPrefix(body, byteOrderMark) {
		return "", "", fmt.Errorf("%w: body %w", ErrMalformedOutput, errLeadingBOM)
	}
	if err := checkBodyMarkdown(body); err != nil {
		return "", "", fmt.Errorf("%w: body %w", ErrMalformedOutput, err)
	}
	return title, body, nil
}

// checkResponse validates a generation or shorten response: the generated text
// with checkGeneratedText, then Model and ModelVersion. It returns the title and
// the body part. Errors wrap ErrMalformedOutput and name the broken rule only,
// never a value, because the response is untrusted.
func checkResponse(resp llm.GenerateResponse) (title, body string, err error) {
	title, body, err = checkGeneratedText(resp.Text)
	if err != nil {
		return "", "", err
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

// normalizeBody is the only change made to the LLM output: it removes a
// leading U+FEFF and turns every line ending into "\n". CommonMark ignores a
// byte order mark at the start of a document and ends a line at all three
// endings, so the meaning is unchanged, but renderers differ on both (some
// keep a lone "\r" inside a line, some strip a leading BOM), which could open
// or close a fence differently from the check. The result is both what the
// checks read and what the article carries.
func normalizeBody(body string) string {
	return bodyLineEndings.Replace(strings.TrimPrefix(body, byteOrderMark))
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
	if reason := displayStringProblem(name, value); reason != "" {
		return fmt.Errorf("%w: %s", ErrMalformedOutput, reason)
	}
	return nil
}

// displayStringProblem applies the rules shared by the title, Model, and
// ModelVersion, which are shown as-is on terminals and destinations: it has
// a non-whitespace character, and no control character (Unicode Cc, which
// includes tab, CR, LF, and ESC). It returns the broken rule, or "" when the
// value is acceptable, and wraps no sentinel: the caller chooses whether the
// rejection is a malformed LLM output or an unpublishable article.
func displayStringProblem(name, value string) string {
	if strings.TrimSpace(value) == "" {
		return name + " is empty or whitespace only"
	}
	if strings.ContainsFunc(value, unicode.IsControl) {
		return name + " contains a control character"
	}
	return ""
}

// newArticle builds the article from validated values. Body is the body part
// as checkResponse returned it, bodySeparator, then the source block, whose
// URL comes from the validated VideoID, never from the generated text.
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
