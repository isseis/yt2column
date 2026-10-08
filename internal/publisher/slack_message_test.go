//go:build test

package publisher

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/isseis/yt2column/internal/writer"
)

// articleWithPostedRunes returns an article whose posted text is exactly total
// code points long, padded with fill.
func articleWithPostedRunes(t *testing.T, total int, fill rune) writer.Article {
	t.Helper()
	article := writer.Article{
		Title:        "Title",
		Body:         "body",
		SourceURL:    "https://example.com/v",
		Model:        "model",
		ModelVersion: "version",
	}
	base := renderArticle(article)
	fixed := len([]rune(base)) - len([]rune(article.Body))
	if total < fixed+1 {
		t.Fatalf("total %d is too small for a body", total)
	}
	article.Body = strings.Repeat(string(fill), total-fixed)
	if got := len([]rune(renderArticle(article))); got != total {
		t.Fatalf("built %d code points, want %d", got, total)
	}
	return article
}

// stripSlackDisplay removes the position marker of the i-th of n messages.
func stripSlackDisplay(messages []string, i int) string {
	if len(messages) == 1 {
		return messages[i]
	}
	return strings.TrimPrefix(messages[i], fmt.Sprintf("(%d/%d)\n\n", i+1, len(messages)))
}

func concatSlackFragments(messages []string) string {
	var b strings.Builder
	for i := range messages {
		b.WriteString(stripSlackDisplay(messages, i))
	}
	return b.String()
}

// checkSlackMessages asserts the invariants every successful split shares.
func checkSlackMessages(t *testing.T, messages []string, posted string) {
	t.Helper()
	for i, message := range messages {
		if got := len([]rune(message)); got > slackMaxMessageRunes {
			t.Fatalf("message %d: %d code points, want <= %d", i, got, slackMaxMessageRunes)
		}
		if !utf8.ValidString(message) {
			t.Fatalf("message %d is not valid UTF-8", i)
		}
		if isWhitespaceOnly([]rune(stripSlackDisplay(messages, i))) {
			t.Fatalf("message %d holds only whitespace", i)
		}
	}
	if len(messages) > 1 {
		for i := range messages {
			prefix := fmt.Sprintf("(%d/%d)\n\n", i+1, len(messages))
			if !strings.HasPrefix(messages[i], prefix) {
				t.Fatalf("message %d does not start with %q", i, prefix)
			}
		}
	}
	if got := concatSlackFragments(messages); got != posted {
		t.Fatalf("joining the messages with the markers removed does not reproduce the posted text")
	}
}

func TestSlackSplit(t *testing.T) {
	// Japanese and supplementary-plane fills make the byte length exceed the
	// code point count, so counting bytes would split these posts.
	for _, fill := range []rune{'a', '\u3042', '\U0001F600'} {
		t.Run(fmt.Sprintf("exactly the limit/%U", fill), func(t *testing.T) {
			article := articleWithPostedRunes(t, slackMaxMessageRunes, fill)
			posted := renderArticle(article)
			messages, err := prepareSlackMessages(article)
			if err != nil {
				t.Fatalf("prepareSlackMessages: %v", err)
			}
			if len(messages) != 1 {
				t.Fatalf("got %d messages, want 1", len(messages))
			}
			if strings.HasPrefix(messages[0], "(") {
				t.Fatalf("a single message carries a position marker: %q", messages[0])
			}
			checkSlackMessages(t, messages, posted)
		})

		t.Run(fmt.Sprintf("one over the limit/%U", fill), func(t *testing.T) {
			article := articleWithPostedRunes(t, slackMaxMessageRunes+1, fill)
			posted := renderArticle(article)
			messages, err := prepareSlackMessages(article)
			if err != nil {
				t.Fatalf("prepareSlackMessages: %v", err)
			}
			if len(messages) != 2 {
				t.Fatalf("got %d messages, want 2", len(messages))
			}
			checkSlackMessages(t, messages, posted)
		})
	}

	t.Run("many lines", func(t *testing.T) {
		article := validArticle()
		article.Body = strings.Repeat("lorem ipsum dolor\n", 4000)
		posted := renderArticle(article)
		messages, err := prepareSlackMessages(article)
		if err != nil {
			t.Fatalf("prepareSlackMessages: %v", err)
		}
		if len(messages) < 3 {
			t.Fatalf("got %d messages, want at least 3", len(messages))
		}
		checkSlackMessages(t, messages, posted)
		for i := range len(messages) - 1 {
			if frag := stripSlackDisplay(messages, i); !strings.HasSuffix(frag, "\n") {
				t.Fatalf("message %d was not cut after a newline", i)
			}
		}
	})

	t.Run("one long line without newlines", func(t *testing.T) {
		article := validArticle()
		article.Body = strings.Repeat("\u3042", 20000) + strings.Repeat("\U0001F600", 4000)
		posted := renderArticle(article)
		messages, err := prepareSlackMessages(article)
		if err != nil {
			t.Fatalf("prepareSlackMessages: %v", err)
		}
		if len(messages) < 2 {
			t.Fatalf("got %d messages, want at least 2", len(messages))
		}
		checkSlackMessages(t, messages, posted)
	})

	t.Run("avoids a whitespace-only fragment", func(t *testing.T) {
		article := validArticle()
		article.Body = "a" + strings.Repeat(" ", 20000) + "\n" + strings.Repeat("b", 20000)
		posted := renderArticle(article)
		messages, err := prepareSlackMessages(article)
		if err != nil {
			t.Fatalf("prepareSlackMessages: %v", err)
		}
		checkSlackMessages(t, messages, posted)
	})

	t.Run("keeps a rest that fits whole", func(t *testing.T) {
		// The first fragment is cut after the header; the rest fits in one
		// fragment and must not be cut again before its last line or its
		// trailing whitespace.
		for _, tail := range []string{"\nEnd.", "\n ", "\n\n"} {
			article := validArticle()
			article.Body = strings.Repeat("b", slackMaxMessageRunes-slackDisplayMaxRunes-10) + tail
			posted := renderArticle(article)
			messages, err := prepareSlackMessages(article)
			if err != nil {
				t.Fatalf("tail %q: prepareSlackMessages: %v", tail, err)
			}
			if len(messages) != 2 {
				t.Fatalf("tail %q: got %d messages, want 2", tail, len(messages))
			}
			checkSlackMessages(t, messages, posted)
		}
	})
}

func TestSlackSplitRejects(t *testing.T) {
	t.Run("too many messages", func(t *testing.T) {
		article := validArticle()
		article.Body = strings.Repeat(
			"a",
			(slackMaxMessages+1)*(slackMaxMessageRunes-slackDisplayMaxRunes),
		)
		if _, err := prepareSlackMessages(article); !errors.Is(err, ErrSlackUnsplittable) {
			t.Fatalf("err = %v, want ErrSlackUnsplittable", err)
		}
	})

	t.Run("whitespace-only fragment", func(t *testing.T) {
		article := validArticle()
		article.Body = strings.Repeat(" ", 20000) + "x"
		if _, err := prepareSlackMessages(article); !errors.Is(err, ErrSlackUnsplittable) {
			t.Fatalf("err = %v, want ErrSlackUnsplittable", err)
		}
	})
}

var slackMentionForms = []string{
	"@channel",
	"@all",
	"@here",
	"@everyone",
	"@someone",
	"<!channel>",
	"<!here>",
	"<!all>",
	"<!everyone>",
	"<@U12345678>",
	"<!subteam^S12345678>",
	"_@channel_",
	"__@here",
	":_@all",
	"a\u200B@here",
	"a\u00AD@here",
	"a\\@here",
	"\\@here",
	"<\u200B!channel>",
	"<\\\u200B!channel>",
	"<\\!\u200Bchannel>",
	"\uFF1C\\!channel>",
	"\uFF20here",
	"\uFE6Bhere",
	"\uFF1C!channel>",
	"&commat;everyone",
	"&#64;here",
	"&#x40;channel",
	"<https://example.com|\u3053\u3053>",
	"<a|b|c>",
	"Array<string|number>",
	"`<!channel>`",
}

func TestSlackMentionRejected(t *testing.T) {
	for _, form := range slackMentionForms {
		for _, field := range []string{"Title", "Body"} {
			t.Run(field+"/"+form, func(t *testing.T) {
				article := validArticle()
				if field == "Title" {
					article.Title = form
				} else {
					article.Body = form
				}
				_, err := prepareSlackMessages(article)
				if !errors.Is(err, ErrSlackMention) {
					t.Fatalf("err = %v, want ErrSlackMention", err)
				}
				if errors.Is(err, writer.ErrInvalidArticle) {
					t.Fatalf("err = %v, must not wrap writer.ErrInvalidArticle", err)
				}
			})
		}
	}

	t.Run("in Model", func(t *testing.T) {
		article := validArticle()
		article.Model = "<!channel>"
		if _, err := prepareSlackMessages(article); !errors.Is(err, ErrSlackMention) {
			t.Fatalf("err = %v, want ErrSlackMention", err)
		}
	})

	t.Run("at a fragment boundary", func(t *testing.T) {
		article := validArticle()
		article.Body = strings.Repeat("\u3042", slackMaxMessageRunes) + "@here"
		if _, err := prepareSlackMessages(article); !errors.Is(err, ErrSlackMention) {
			t.Fatalf("err = %v, want ErrSlackMention", err)
		}
	})
}

func TestSlackMentionAccepted(t *testing.T) {
	accepted := []string{
		"See <https://example.com/x> for details.",
		"Source: <https://www.youtube.com/watch?v=abcdefghijk>",
		"Join the ~town-square channel.",
		"The function takes an int and a string.",
		"Use a < b in this comparison.",
		"Price is $5 & tax is separate.",
		"The <algorithm> header is included.",
		"Reach the maintainers through the issue tracker.",
		"An empty link part: <|b> and <a|>.",
	}
	for _, body := range accepted {
		t.Run(body, func(t *testing.T) {
			article := validArticle()
			article.Body = body
			if _, err := prepareSlackMessages(article); err != nil {
				t.Fatalf("prepareSlackMessages: %v, want no error", err)
			}
		})
	}
}

func TestSlackPrepareRejectsInvalidArticle(t *testing.T) {
	cases := []struct {
		name               string
		mutate             func(*writer.Article)
		checkPublishableOK bool
	}{
		{"title empty", func(a *writer.Article) { a.Title = "" }, false},
		{"body whitespace only", func(a *writer.Article) { a.Body = " \n\t" }, false},
		{"body escape", func(a *writer.Article) { a.Body = "x\x1b[2J" }, false},
		{"source url empty", func(a *writer.Article) { a.SourceURL = "" }, false},
		{"title invalid utf-8", func(a *writer.Article) { a.Title = "a\xffb" }, true},
		{"body invalid utf-8", func(a *writer.Article) { a.Body = "x\xffy" }, true},
		{"model invalid utf-8", func(a *writer.Article) { a.Model = "m\xff" }, true},
		{"model version invalid utf-8", func(a *writer.Article) { a.ModelVersion = "v\xff" }, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			article := validArticle()
			tc.mutate(&article)
			if tc.checkPublishableOK {
				if err := article.CheckPublishable(); err != nil {
					t.Fatalf("CheckPublishable rejected before the UTF-8 layer: %v", err)
				}
			}
			if _, err := prepareSlackMessages(article); !errors.Is(err, writer.ErrInvalidArticle) {
				t.Fatalf("err = %v, want writer.ErrInvalidArticle", err)
			}
		})
	}
}

func TestSlackPrepareErrorsOmitContent(t *testing.T) {
	const marker = "MARKER-7f3a9c2e"

	cases := []struct {
		name   string
		mutate func(*writer.Article)
	}{
		{"mention", func(a *writer.Article) { a.Body = marker + " @channel\n" }},
		{"unsplittable", func(a *writer.Article) { a.Body = strings.Repeat(" ", 20000) + marker }},
		{"invalid article", func(a *writer.Article) { a.Title = "" }},
		{"invalid utf-8", func(a *writer.Article) { a.Model = marker + "\xff" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			article := validArticle()
			tc.mutate(&article)
			_, err := prepareSlackMessages(article)
			if err == nil {
				t.Fatalf("prepareSlackMessages returned no error")
			}
			if strings.Contains(err.Error(), marker) {
				t.Fatalf("error text %q leaks the article content", err.Error())
			}
			if strings.Contains(err.Error(), "@channel") {
				t.Fatalf("error text %q leaks the matched text", err.Error())
			}
		})
	}
}

func TestSlackMessageCount(t *testing.T) {
	article := validArticle()
	got, err := SlackMessageCount(article)
	if err != nil {
		t.Fatalf("SlackMessageCount: %v", err)
	}
	if got != 1 {
		t.Fatalf("SlackMessageCount = %d, want 1", got)
	}

	long := articleWithPostedRunes(t, slackMaxMessageRunes+1, 'a')
	got, err = SlackMessageCount(long)
	if err != nil {
		t.Fatalf("SlackMessageCount: %v", err)
	}
	want, err := prepareSlackMessages(long)
	if err != nil {
		t.Fatalf("prepareSlackMessages: %v", err)
	}
	if got != len(want) || got < 2 {
		t.Fatalf("SlackMessageCount = %d, want %d (>= 2)", got, len(want))
	}

	mention := validArticle()
	mention.Body = "@channel"
	if _, err := SlackMessageCount(mention); !errors.Is(err, ErrSlackMention) {
		t.Fatalf("err = %v, want ErrSlackMention", err)
	}
}

func TestSlackMentionPosition(t *testing.T) {
	cases := []struct {
		text         string
		rule         string
		line, column int
	}{
		{"ab\ncd @here", ruleM2, 2, 4},
		// V1 removes the backslash; the match is reported at its index.
		{"x\n\\@here", ruleM2, 2, 1},
		// V2 removes U+200B; the index still counts it.
		{"\u200B<!here>", ruleM1, 1, 2},
		{"a\nb\nc &#64;", ruleM3, 3, 3},
		// The earliest match wins across the rules.
		{"<a|b> @here", ruleM4, 1, 1},
	}
	for _, tc := range cases {
		match, ok := findMention(tc.text)
		if !ok {
			t.Fatalf("%q: no match", tc.text)
		}
		line, column := lineAndColumn([]rune(tc.text), match.index)
		if match.rule != tc.rule || line != tc.line || column != tc.column {
			t.Fatalf("%q: got %s at %d:%d, want %s at %d:%d",
				tc.text, match.rule, line, column, tc.rule, tc.line, tc.column)
		}
	}
}

// TestSlackMentionMessagesFollowPosted pins the premise that lets
// prepareSlackMessages check posted alone: when posted holds no M1-M4 match,
// no message does either, even with a near-match cut at a fragment boundary
// right after a position marker.
func TestSlackMentionMessagesFollowPosted(t *testing.T) {
	capacity := slackMaxMessageRunes - slackDisplayMaxRunes
	// Each case is the text just before and just after a fragment boundary.
	// The body has no newline, so the first fragment is the header and the
	// body is cut at exactly capacity code points.
	cases := []struct{ before, after string }{
		{"a", `\!channel>`},
		{"a", "here"},
		{"a", "#64;here"},
		{"a", "lt;x"},
		{"a", "x|y>"},
		{"\u200b", "!channel>"},
	}
	for _, tc := range cases {
		t.Run(tc.before+"|"+tc.after, func(t *testing.T) {
			article := validArticle()
			article.Body = strings.Repeat("b", capacity-len([]rune(tc.before))) + tc.before + tc.after + "b"
			messages, err := prepareSlackMessages(article)
			if err != nil {
				t.Fatalf("prepareSlackMessages: %v", err)
			}
			if len(messages) != 3 || !strings.HasPrefix(stripSlackDisplay(messages, 2), tc.after) {
				t.Fatalf("the fragment boundary is not before %q", tc.after)
			}
			for i, message := range messages {
				if err := rejectMention(message); err != nil {
					t.Fatalf("message %d: %v", i, err)
				}
			}
		})
	}
}
