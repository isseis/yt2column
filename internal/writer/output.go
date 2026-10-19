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
	// maxChannelNameRunes caps a channel name, in code points. YouTube
	// documents no limit and secondary sources say 100, so the cap is twice
	// that: a smaller cap than the real one would reject a legitimate name.
	maxChannelNameRunes = 200
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

// CheckStageText validates a saved stage's title and body with the same rules
// checkGeneratedText applies to generated text. It also rejects a value
// checkGeneratedText would normalize (a trimmed title, a rewritten line ending,
// or a leading U+FEFF), so a loaded artifact carries exactly the values the
// checks read rather than the values they rewrote. It wraps ErrMalformedOutput
// and names the broken rule only, never a value.
func CheckStageText(title, body string) error {
	checkedTitle, checkedBody, err := checkGeneratedText(textString(title, body))
	if err != nil {
		return err
	}
	if checkedTitle != title || checkedBody != body {
		return fmt.Errorf("%w: the saved text is not in normalized form", ErrMalformedOutput)
	}
	return nil
}

// CheckModelText validates a saved artifact's model name or model version with
// the rules a response's Model and ModelVersion must satisfy. It is the
// response rule under an exported name, so a loader reuses it instead of
// restating it. An empty value is rejected; a caller that allows an empty
// model version checks only a non-empty one. It wraps ErrMalformedOutput and
// never puts the value in the error.
func CheckModelText(name, value string) error {
	return checkModelString(name, value)
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
	if err := checkModelPair(resp.Model, resp.ModelVersion); err != nil {
		return "", "", err
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

// checkModelPair applies the Model and optional ModelVersion rules. An empty
// ModelVersion is allowed; a non-empty one is checked like Model.
func checkModelPair(model, version string) error {
	if err := checkModelString("Model", model); err != nil {
		return err
	}
	if version == "" {
		return nil
	}
	return checkModelString("ModelVersion", version)
}

// checkModelString applies the size rule of Model and ModelVersion, then the
// rules shared with the title (which include valid UTF-8).
func checkModelString(name, value string) error {
	if len(value) > maxModelBytes {
		return fmt.Errorf("%w: %s exceeds the limit of %d bytes", ErrMalformedOutput, name, maxModelBytes)
	}
	if reason := displayStringProblem(name, value); reason != "" {
		return fmt.Errorf("%w: %s", ErrMalformedOutput, reason)
	}
	return nil
}

// displayStringProblem applies the rules shared by the title, Model,
// ModelVersion, and channel name, which are shown as-is on terminals and
// destinations: it is valid UTF-8, has a non-whitespace character, and has no
// control character (Unicode Cc, which includes tab, CR, LF, and ESC). It
// returns the broken rule, or "" when the value is acceptable, and wraps no
// sentinel: the caller chooses whether the rejection is a malformed LLM output
// or an unpublishable article.
func displayStringProblem(name, value string) string {
	if !utf8.ValidString(value) {
		return name + " is not valid UTF-8"
	}
	if strings.TrimSpace(value) == "" {
		return name + " is empty or whitespace only"
	}
	if strings.ContainsFunc(value, unicode.IsControl) {
		return name + " contains a control character"
	}
	return ""
}

// CheckChannelName returns nil for the empty name ("no channel name") and for
// a name that passes the display-string rules and has no bidirectional control
// character (U+202A to U+202E, U+2066 to U+2069, which reorder the displayed
// text) and at most maxChannelNameRunes code points. Otherwise it wraps
// ErrInvalidChannelName, names the broken rule only, and never puts the value
// in the error. ZWJ, ZWNJ, and the left-to-right and right-to-left marks are
// accepted: emoji and right-to-left names use them.
func CheckChannelName(name string) error {
	if name == "" {
		return nil
	}
	if reason := displayStringProblem("channel name", name); reason != "" {
		return fmt.Errorf("%w: %s", ErrInvalidChannelName, reason)
	}
	if strings.ContainsFunc(name, isBidiControl) {
		return fmt.Errorf("%w: channel name contains a bidirectional control character", ErrInvalidChannelName)
	}
	if utf8.RuneCountInString(name) > maxChannelNameRunes {
		return fmt.Errorf("%w: channel name exceeds the limit of %d code points", ErrInvalidChannelName, maxChannelNameRunes)
	}
	return nil
}

// isBidiControl reports whether r is a bidirectional embedding, override, or
// isolate control: U+202A to U+202E and U+2066 to U+2069.
func isBidiControl(r rune) bool {
	return (r >= '\u202A' && r <= '\u202E') || (r >= '\u2066' && r <= '\u2069')
}

// newArticle builds the article from validated values. ChannelName is the
// transcript's, unchanged. Body is the body part of the last checked text
// (from checkResponse or checkGeneratedText), bodySeparator, then the source
// block, whose URL comes from the validated VideoID, never from the generated
// text. model and modelVersion are the generation model when it ran, or the
// reused generation artifact's values.
func newArticle(title, channelName, body, sourceURL, model, modelVersion string, shorten *StepModel, review *ReviewResult, runUsage RunUsage) Article {
	return Article{
		Title:        title,
		ChannelName:  channelName,
		Body:         body + bodySeparator + sourceBlock(sourceURL),
		SourceURL:    sourceURL,
		Model:        model,
		ModelVersion: modelVersion,
		Shorten:      shorten,
		Review:       review,
		RunUsage:     runUsage,
	}
}

// textString joins a title and a body part into the text form that
// checkGeneratedText reads. The review step applies its revisions to it.
func textString(title, body string) string {
	return titlePrefix + title + "\n" + body
}

// sourceBlock is the fixed-format source line, an autolink to sourceURL,
// followed by one "\n".
func sourceBlock(sourceURL string) string {
	return sourceLabel + "<" + sourceURL + ">\n"
}
