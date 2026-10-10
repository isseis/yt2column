package writer

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/isseis/yt2column/internal/transcript"
)

// maxPromptBytes caps each expanded prompt (system and user separately).
const maxPromptBytes = 1048576 // 1 MiB

// templateData is the only value passed to a generation template. Its fields
// are the four values such a template may reference; the template checks
// derive the allowed field names from this type.
type templateData struct {
	Title       string
	ChannelName string
	Description string
	Transcript  string
}

// validateTranscript checks t in a fixed order and returns the normalized
// video URL, which is built from the validated VideoID, never taken from
// VideoURL. Errors wrap ErrInvalidTranscript and name the failed field only;
// the values come from untrusted input and are never included.
func validateTranscript(t transcript.Transcript) (string, error) {
	sourceURL, ok := transcript.NormalizedVideoURL(t.VideoID)
	if !ok {
		return "", fmt.Errorf("%w: VideoID is not 11 characters of [A-Za-z0-9_-]", ErrInvalidTranscript)
	}
	if t.VideoURL != sourceURL {
		return "", fmt.Errorf("%w: VideoURL is not the normalized URL of VideoID", ErrInvalidTranscript)
	}
	if len(t.Segments) == 0 {
		return "", fmt.Errorf("%w: Segments is empty", ErrInvalidTranscript)
	}
	for i, seg := range t.Segments {
		if strings.TrimSpace(seg.Text) == "" {
			return "", fmt.Errorf("%w: Segments[%d].Text is empty or whitespace only", ErrInvalidTranscript, i)
		}
	}
	fields := []struct{ name, value string }{
		{"Title", t.Title},
		{"ChannelName", t.ChannelName},
		{"Description", t.Description},
	}
	for _, f := range fields {
		if !utf8.ValidString(f.value) {
			return "", fmt.Errorf("%w: %s is not valid UTF-8", ErrInvalidTranscript, f.name)
		}
	}
	for i, seg := range t.Segments {
		if !utf8.ValidString(seg.Text) {
			return "", fmt.Errorf("%w: Segments[%d].Text is not valid UTF-8", ErrInvalidTranscript, i)
		}
	}
	return sourceURL, nil
}

// newTemplateData builds the template data from a validated transcript. The
// transcript text is every Segment.Text, unmodified and in order, joined by
// one "\n"; StartMs is never included.
func newTemplateData(t transcript.Transcript) templateData {
	texts := make([]string, len(t.Segments))
	for i, seg := range t.Segments {
		texts[i] = seg.Text
	}
	return templateData{
		Title:       t.Title,
		ChannelName: t.ChannelName,
		Description: t.Description,
		Transcript:  strings.Join(texts, "\n"),
	}
}

// boundedWriter holds at most limit bytes. A write that would take the total
// past limit is refused whole with errPromptTooLarge and nothing of it is
// kept, so expansion stops before the prompt can grow beyond the limit.
type boundedWriter struct {
	buf   bytes.Buffer
	limit int
}

func (w *boundedWriter) Write(p []byte) (int, error) {
	if len(p) > w.limit-w.buf.Len() {
		return 0, errPromptTooLarge
	}
	return w.buf.Write(p)
}

// expand executes the template with data into a buffer capped at maxPromptBytes. An
// execution error, an oversized prompt, and a blank prompt all wrap
// ErrInvalidTemplate. The execution error text is quoted, so control
// characters in it cannot reach a terminal unescaped.
func (c checkedTemplate[T]) expand(data T) (string, error) {
	w := &boundedWriter{limit: maxPromptBytes}
	if err := c.tmpl.Execute(w, data); err != nil {
		if errors.Is(err, errPromptTooLarge) {
			return "", fmt.Errorf("%w: %s prompt: %w of %d bytes; the template or the transcript values it embeds are too large",
				ErrInvalidTemplate, c.tmpl.Name(), errPromptTooLarge, maxPromptBytes)
		}
		return "", fmt.Errorf("%w: %s template failed to expand: %q", ErrInvalidTemplate, c.tmpl.Name(), err.Error())
	}
	prompt := w.buf.String()
	if strings.TrimSpace(prompt) == "" {
		return "", fmt.Errorf("%w: %s prompt is empty or whitespace only", ErrInvalidTemplate, c.tmpl.Name())
	}
	return prompt, nil
}
