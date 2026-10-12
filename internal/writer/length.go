package writer

import (
	"strings"
	"unicode/utf8"
)

// maxBodyChars is the length limit of the body part, counted as bodyChars.
const maxBodyChars = 4000

// targetBodyChars is the length the shorten step aims for. It stays below
// maxBodyChars so the review step's paraphrasing can lengthen the body a
// little before the final length check.
const targetBodyChars = 3600

// bodyChars returns the number of characters of the body part: its code points
// without the line feeds. Markdown markers and spaces count, matching the
// check_article.py count of the 0008 evaluation. The body part has no "\r"
// because normalizeBody rewrote every line ending.
func bodyChars(body string) int {
	return utf8.RuneCountInString(body) - strings.Count(body, "\n")
}
