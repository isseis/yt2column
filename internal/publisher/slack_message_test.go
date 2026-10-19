//go:build test

package publisher

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/isseis/yt2column/internal/writer"
)

// shortLastLine ends a long body with a short last line. The last line of a
// post stays in one message with the footnote, so a body of one long line
// cannot be split.
const shortLastLine = "\nEnd."

// articleWithPostedRunes returns an article whose posted text is exactly total
// code points long, padded with fill and ending in shortLastLine.
func articleWithPostedRunes(t *testing.T, total int, fill rune) writer.Article {
	t.Helper()
	article := writer.Article{
		Title:        "Title",
		Body:         "body",
		SourceURL:    "https://example.com/v",
		Model:        "model",
		ModelVersion: "version",
	}
	base := renderSlackPost(article).text
	fixed := len([]rune(base)) - len([]rune(article.Body))
	if total < fixed+1 {
		t.Fatalf("total %d is too small for a body", total)
	}
	tail := len([]rune(shortLastLine))
	if total < fixed+tail+1 {
		t.Fatalf("total %d is too small for a body", total)
	}
	article.Body = strings.Repeat(string(fill), total-fixed-tail) + shortLastLine
	if got := len([]rune(renderSlackPost(article).text)); got != total {
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
			posted := renderSlackPost(article).text
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
			posted := renderSlackPost(article).text
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
		posted := renderSlackPost(article).text
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
		article.Body = strings.Repeat("\u3042", 20000) + strings.Repeat("\U0001F600", 4000) + shortLastLine
		posted := renderSlackPost(article).text
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
		article.Body = "a" + strings.Repeat(" ", 20000) + "\n" + strings.Repeat("b", 20000) + shortLastLine
		posted := renderSlackPost(article).text
		messages, err := prepareSlackMessages(article)
		if err != nil {
			t.Fatalf("prepareSlackMessages: %v", err)
		}
		checkSlackMessages(t, messages, posted)
	})

	t.Run("keeps a rest that fits whole", func(t *testing.T) {
		// The first fragment is cut after the heading line; the rest is
		// sized to fill exactly one fragment and must not be cut again before
		// its last line or its trailing whitespace.
		capacity := slackMaxMessageRunes - slackDisplayMaxRunes
		for _, tail := range []string{"\nEnd.", "\n ", "\n\n"} {
			article := validArticle()
			article.Body = "b" + tail
			post := renderSlackPost(article).text
			heading := strings.Index(post, "\n\n") + 2
			filler := capacity - (utf8.RuneCountInString(post) - heading)
			article.Body = strings.Repeat("b", 1+filler) + tail
			posted := renderSlackPost(article).text
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

// TestSlackSplitLayout checks where the heading line and the footnote land
// when a post splits: the heading line starts the first message, the footnote
// ends the last, and no message holds nothing else than the footnote.
func TestSlackSplitLayout(t *testing.T) {
	capacity := slackMaxMessageRunes - slackDisplayMaxRunes
	article := validArticle()
	footnote := slackFootnote(article)

	check := func(t *testing.T, article writer.Article) []string {
		t.Helper()
		posted := renderSlackPost(article).text
		messages, err := prepareSlackMessages(article)
		if err != nil {
			t.Fatalf("prepareSlackMessages: %v", err)
		}
		checkSlackMessages(t, messages, posted)
		if len(messages) < 2 {
			t.Fatalf("got %d messages, want the post to split", len(messages))
		}
		if first := stripSlackDisplay(messages, 0); !strings.HasPrefix(first, "# ") {
			t.Fatalf("the first message does not start with the heading line")
		}
		for i, message := range messages {
			fragment := stripSlackDisplay(messages, i)
			if got, want := strings.Contains(fragment, footnote), i == len(messages)-1; got != want {
				t.Fatalf("message %d holds the footnote = %v, want %v", i, got, want)
			}
			if strings.TrimSpace(strings.TrimSuffix(fragment, footnote)) == "" {
				t.Fatalf("message %d (%d code points) holds nothing but the footnote", i, len([]rune(message)))
			}
		}
		return messages
	}

	t.Run("body lengths around the boundary", func(t *testing.T) {
		// The post first exceeds one message at some body length; sweeping
		// across it covers a cut falling right after the last newline of the
		// body, and every fragment boundary near it.
		heading := len([]rune("# " + article.Title + "\n\n"))
		for length := capacity - heading - 80; length <= capacity-heading+80; length++ {
			article := validArticle()
			article.Body = strings.Repeat("x", length) + shortLastLine
			if len([]rune(renderSlackPost(article).text)) <= slackMaxMessageRunes {
				continue
			}
			check(t, article)
		}
	})

	t.Run("ten messages", func(t *testing.T) {
		// The heading line is cut off on its own, then each full line fills a
		// fragment, and the short last line shares the last one with the
		// footnote.
		line := strings.Repeat("a", capacity-1) + "\n"
		article := validArticle()
		article.Body = strings.Repeat(line, slackMaxMessages-2) + "End."
		if got := len(check(t, article)); got != slackMaxMessages {
			t.Fatalf("got %d messages, want %d", got, slackMaxMessages)
		}
		article.Body = strings.Repeat(line, slackMaxMessages-1) + "End."
		if _, err := prepareSlackMessages(article); !errors.Is(err, ErrSlackUnsplittable) {
			t.Fatalf("err = %v, want ErrSlackUnsplittable", err)
		}
	})

	t.Run("heading line longer than a message", func(t *testing.T) {
		// The cut falls between the 'x' and the '@' of a mid-word "@here" in
		// the title; the post is accepted and no message starts with the '@'.
		article := validArticle()
		article.Title = strings.Repeat("a", capacity-3) + "x@here"
		for _, message := range check(t, article) {
			if err := rejectMention(message); err != nil {
				t.Fatalf("message: %v", err)
			}
		}
	})
}

func TestSlackSplitRejects(t *testing.T) {
	t.Run("too many messages", func(t *testing.T) {
		article := validArticle()
		// Every line fills a fragment, so the cut is not what refuses the
		// article: the number of fragments is.
		article.Body = strings.Repeat(
			strings.Repeat("a", slackMaxMessageRunes-slackDisplayMaxRunes-1)+"\n",
			slackMaxMessages+1,
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

// TestSlackSplitKeepTogether checks the keepTogetherFrom rules of the cutting
// layer. The inputs hold no '@', so the check layer cannot act on them.
func TestSlackSplitKeepTogether(t *testing.T) {
	capacity := slackMaxMessageRunes - slackDisplayMaxRunes
	a, b := strings.Repeat("a", 10), strings.Repeat("b", capacity)

	cases := []struct {
		name string
		post slackPost
		want []string // nil means ErrSlackUnsplittable
	}{
		// Without the bound the first cut would fall at capacity.
		{"cuts no later than keepTogetherFrom", slackPost{text: a + b, keepTogetherFrom: len(a)}, []string{a, b}},
		// A boundary pin, not a test of the bound: a front region that
		// exactly fills a fragment is one fragment, not rejected or re-cut.
		{
			"front region exactly fills a fragment",
			slackPost{text: strings.Repeat("a", capacity) + b, keepTogetherFrom: capacity},
			[]string{strings.Repeat("a", capacity), b},
		},
		// The region from keepTogetherFrom is one rune over a fragment.
		{"last region exceeds a fragment", slackPost{text: a + b + "b", keepTogetherFrom: len(a)}, nil},
		// The first cut falls after the newline; the second must stop at
		// keepTogetherFrom, which leaves the ten spaces alone.
		{
			"whitespace-only fragment cut at keepTogetherFrom",
			slackPost{text: "a\n" + strings.Repeat(" ", 10) + b, keepTogetherFrom: 2 + 10},
			nil,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if utf8.RuneCountInString(tc.post.text) <= slackMaxMessageRunes {
				t.Fatal("the post fits in one message")
			}
			got, err := cutSlackFragments(tc.post)
			if tc.want == nil {
				if !errors.Is(err, ErrSlackUnsplittable) {
					t.Fatalf("err = %v, want ErrSlackUnsplittable", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("cutSlackFragments: %v", err)
			}
			if !slices.Equal(got, tc.want) {
				t.Fatalf("fragment lengths = %d, want %d", runeLens(got), runeLens(tc.want))
			}
		})
	}
}

func runeLens(fragments []string) []int {
	lens := make([]int, len(fragments))
	for i, f := range fragments {
		lens[i] = utf8.RuneCountInString(f)
	}
	return lens
}

// TestSlackSplitAvoidsMidWordAt checks that a cut falling between a letter or
// a digit and an '@' moves back one rune, so the '@', which M2 accepts in the
// post, does not start a message.
func TestSlackSplitAvoidsMidWordAt(t *testing.T) {
	capacity := slackMaxMessageRunes - slackDisplayMaxRunes
	for _, word := range []string{"x@here", "1@here", "x\uFF20here"} {
		t.Run(word, func(t *testing.T) {
			// The body has no newline before its short last line, so the
			// first fragment is the heading line and the body would be cut
			// right before the '@'.
			article := validArticle()
			article.Body = strings.Repeat("b", capacity-1) + word + "b" + shortLastLine
			posted := renderSlackPost(article).text
			if err := rejectMention(posted); err != nil {
				t.Fatalf("the post itself is rejected: %v", err)
			}
			messages, err := prepareSlackMessages(article)
			if err != nil {
				t.Fatalf("prepareSlackMessages: %v", err)
			}
			checkSlackMessages(t, messages, posted)
			if len(messages) != 3 {
				t.Fatalf("got %d messages, want 3", len(messages))
			}
			if got := stripSlackDisplay(messages, 2); !strings.HasPrefix(got, word) {
				t.Fatalf("the last fragment does not start with %q", word)
			}
			for i, message := range messages {
				if err := rejectMention(message); err != nil {
					t.Fatalf("message %d: %v", i, err)
				}
			}
		})
	}
}

// TestSlackSplitRefusesUnmovableCut covers a cut that cannot move back: the
// fragment before it holds one rune. The cutting layer alone leaves the '@'
// at the start of a fragment, and the check layer refuses the split.
func TestSlackSplitRefusesUnmovableCut(t *testing.T) {
	capacity := slackMaxMessageRunes - slackDisplayMaxRunes
	// keepTogetherFrom sits on the '@', so after the cut that follows the
	// newline the next fragment can hold only "x"; the region from the '@'
	// exactly fills a fragment.
	front := strings.Repeat("y", 10) + "\n"
	post := slackPost{
		text:             front + "x@here" + strings.Repeat("z", capacity-5),
		keepTogetherFrom: len(front + "x"),
	}
	fragments, err := cutSlackFragments(post)
	if err != nil {
		t.Fatalf("cutSlackFragments: %v", err)
	}
	if want := []string{front, "x"}; len(fragments) != 3 || !slices.Equal(fragments[:2], want) ||
		!strings.HasPrefix(fragments[2], "@here") {
		t.Fatalf("fragment lengths = %d, want the cut right before the '@'", runeLens(fragments))
	}
	if _, err := splitSlackText(post); !errors.Is(err, ErrSlackUnsplittable) {
		t.Fatalf("err = %v, want ErrSlackUnsplittable", err)
	}
}

func TestCheckFragmentStarts(t *testing.T) {
	cases := []struct {
		name      string
		fragments []string
		wantErr   bool
	}{
		{"letter before the at sign", []string{"ab x", "@here c"}, true},
		{"digit before the at sign", []string{"ab 1", "@here"}, true},
		{"look-alike at sign", []string{"ab x", "\uFF20here"}, true},
		{"non-ASCII letter before the at sign", []string{"ab \u3055\u3093", "@here"}, true},
		{"non-ASCII name after the at sign", []string{"ab x", "@\u540d\u524d"}, true},
		{"in a later fragment", []string{"a", "b", "x", "@y"}, true},

		{"space before the at sign", []string{"ab ", "@here"}, false},
		{"no name character after the at sign", []string{"ab x", "@ here"}, false},
		{"lone at sign", []string{"ab x", "@"}, false},
		{"at sign inside the fragment", []string{"ab x", "y@here"}, false},
		{"at sign at the end of a fragment", []string{"ab x@", "here"}, false},
		{"first fragment", []string{"@here x", "y"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := checkFragmentStarts(tc.fragments)
			if tc.wantErr && !errors.Is(err, ErrSlackUnsplittable) {
				t.Fatalf("err = %v, want ErrSlackUnsplittable", err)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("err = %v, want nil", err)
			}
		})
	}
}

// slackNeutralizedForms hold a target '@' that renderSlackPost neutralizes in
// the title and in the prose of the body, so the post is accepted.
var slackNeutralizedForms = []string{
	"@channel",
	"@all",
	"@here",
	"@everyone",
	"@someone",
	"_@channel_",
	"__@here",
	":_@all",
	"foo_@bar",
	"x@@y",
	"a\u200B@here",
	"a\u00AD@here",
	"a\\@here",
	"\\@here",
	"\uFF20here",
	"\uFE6Bhere",
}

// slackRejectedForms hold what Mattermost or Slack may treat as a mention or
// rewrite and that no neutralization covers: M1, M3 and M4.
var slackRejectedForms = []string{
	"<!channel>",
	"<!here>",
	"<!all>",
	"<!everyone>",
	"<@U12345678>",
	"<!subteam^S12345678>",
	"<\u200B!channel>",
	"<\\\u200B!channel>",
	"<\\!\u200Bchannel>",
	"\uFF1C\\!channel>",
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
	for _, form := range slackRejectedForms {
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

	t.Run("in the footnote", func(t *testing.T) {
		article := validArticle()
		article.Shorten = &writer.StepModel{Model: "<@U12345678>", ModelVersion: "v1"}
		if _, err := prepareSlackMessages(article); !errors.Is(err, ErrSlackMention) {
			t.Fatalf("err = %v, want ErrSlackMention", err)
		}
	})

	t.Run("in a later fragment", func(t *testing.T) {
		article := validArticle()
		article.Body = strings.Repeat("\u3042", slackMaxMessageRunes) + "\n`@here`"
		if _, err := prepareSlackMessages(article); !errors.Is(err, ErrSlackMention) {
			t.Fatalf("err = %v, want ErrSlackMention", err)
		}
	})
}

// TestSlackMentionTargetsOutsideNeutralization checks the '@' characters that
// stay rejected: those in code, which renderSlackPost does not touch, and
// those right after a slash, which neutralization skips so a URL stays whole.
// A body whose code the renderer cannot judge keeps its targets too.
func TestSlackMentionTargetsOutsideNeutralization(t *testing.T) {
	bodies := map[string]string{
		"code span":               "Run `@here` now.",
		"fenced code block":       "```\n@here\n```\nDone.",
		"after a slash":           "See youtube.com/@name now.",
		"unjudgeable: open fence": "```\n@here",
		"unjudgeable: lone tick":  "It's a ` and @here.",
	}
	for name, body := range bodies {
		t.Run("Body/"+name, func(t *testing.T) {
			article := validArticle()
			article.Body = body
			if _, err := prepareSlackMessages(article); !errors.Is(err, ErrSlackMention) {
				t.Fatalf("err = %v, want ErrSlackMention", err)
			}
		})
	}
	t.Run("Title/after a slash", func(t *testing.T) {
		article := validArticle()
		article.Title = "youtube.com/@name"
		if _, err := prepareSlackMessages(article); !errors.Is(err, ErrSlackMention) {
			t.Fatalf("err = %v, want ErrSlackMention", err)
		}
	})
}

// TestSlackMentionNeutralized checks that a target '@' in the title, the
// channel name, the prose of the body and the footnote does not reject the
// post: the posted text holds a zero-width space after it, and no message
// still matches a mention rule.
func TestSlackMentionNeutralized(t *testing.T) {
	for _, form := range slackNeutralizedForms {
		for _, field := range []string{"Title", "ChannelName", "Body", "Model"} {
			t.Run(field+"/"+form, func(t *testing.T) {
				article := validArticle()
				switch field {
				case "Title":
					article.Title = form
				case "ChannelName":
					article.ChannelName = form
				case "Body":
					article.Body = form
				case "Model":
					article.Model = form
				}
				messages, err := prepareSlackMessages(article)
				if err != nil {
					t.Fatalf("prepareSlackMessages: %v", err)
				}
				if !strings.Contains(strings.Join(messages, ""), zeroWidthSpace) {
					t.Fatalf("the posted text holds no zero-width space: %q", messages)
				}
			})
		}
	}
}

// TestSlackMentionPositionAfterNeutralization checks that a rejection names
// the position in the text Publish sends: the zero-width space inserted after
// the '@' of "@c" counts as a column before the rejected '@' in the code span.
func TestSlackMentionPositionAfterNeutralization(t *testing.T) {
	article := validArticle()
	article.Body = "Hi @c and `@d`\n"
	_, err := prepareSlackMessages(article)
	if !errors.Is(err, ErrSlackMention) {
		t.Fatalf("err = %v, want ErrSlackMention", err)
	}
	// Line 1 is the heading line and line 2 is blank. In the sent form the
	// body line is "Hi @", U+200B, "c and `@d`", so the code span's '@' is in
	// column 13; counted before neutralization it would be column 12. The
	// wording is the observable contract of the position, so the message is
	// compared.
	if want := "rule M2 at line 3, column 13"; !strings.Contains(err.Error(), want) {
		t.Fatalf("err = %q, want it to name %q", err, want)
	}
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
		// An '@' right after a letter or a digit is inside a word, which
		// Mattermost does not read as a mention.
		"Tech@Work",
		"Mail user@example.com today.",
		"\u3055\u3093@xxx",
		"a1@b",
	}
	// Title and Body both reach the post verbatim today; the Title rows are
	// kept because the Slack renderer treats the title on its own path.
	for _, text := range accepted {
		for _, field := range []string{"Title", "Body"} {
			t.Run(field+"/"+text, func(t *testing.T) {
				article := validArticle()
				if field == "Title" {
					article.Title = text
				} else {
					article.Body = text
				}
				if _, err := prepareSlackMessages(article); err != nil {
					t.Fatalf("prepareSlackMessages: %v, want no error", err)
				}
			})
		}
	}
}

// TestCostLineHasNoMention checks that the cost line, in its known and unknown
// forms, matches none of the mention rules. The line is read from the
// renderer's output, not spelled out here. The publish path rejects a mention
// too; this pins the rule at the cost line itself.
func TestCostLineHasNoMention(t *testing.T) {
	for name, article := range map[string]writer.Article{
		"known":   costedArticle(),
		"unknown": validArticle(),
	} {
		t.Run(name, func(t *testing.T) {
			line := costLineIn(t, renderSlackPost(article).text)
			if match, ok := findMention(line); ok {
				t.Fatalf("the cost line %q matches mention rule %s", line, match.rule)
			}
		})
	}
}

// costLineIn returns the estimated-cost line of a rendered article, which the
// renderer always writes.
func costLineIn(t *testing.T, posted string) string {
	t.Helper()
	for line := range strings.SplitSeq(posted, "\n") {
		if strings.Contains(line, "Estimated cost") {
			return line
		}
	}
	t.Fatalf("no cost line in %q", posted)
	return ""
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
		{"channel name control character", func(a *writer.Article) { a.ChannelName = "a\nb" }, false},
		{"title invalid utf-8", func(a *writer.Article) { a.Title = "a\xffb" }, false},
		{"body invalid utf-8", func(a *writer.Article) { a.Body = "x\xffy" }, true},
		{"model invalid utf-8", func(a *writer.Article) { a.Model = "m\xff" }, false},
		{"model version invalid utf-8", func(a *writer.Article) { a.ModelVersion = "v\xff" }, false},
		{"shorten model invalid utf-8", func(a *writer.Article) { a.Shorten = &writer.StepModel{Model: "m\xff"} }, false},
		{"shorten model version invalid utf-8", func(a *writer.Article) {
			a.Shorten = &writer.StepModel{Model: "shorten-x", ModelVersion: "v\xff"}
		}, false},
		{"review model invalid utf-8", func(a *writer.Article) {
			a.Review = &writer.ReviewResult{Model: writer.StepModel{Model: "m\xff"}}
		}, false},
		{"review model version invalid utf-8", func(a *writer.Article) {
			a.Review = &writer.ReviewResult{Model: writer.StepModel{Model: "review-x", ModelVersion: "v\xff"}}
		}, false},
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
		{"mention", func(a *writer.Article) { a.Body = marker + " `@channel`\n" }},
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
	mention.Body = "`@channel`"
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
		// M2 does not undo the escape: the backslash separates words, and the
		// match is reported at the '@' itself.
		{"x\n\\@here", ruleM2, 2, 2},
		// V1 removes the backslash; the M1 match is reported at its index.
		{"x\n\\<!here>", ruleM1, 2, 1},
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
// prepareSlackMessages check posted alone: when posted holds no M1, M3 or M4
// match, no message does either, even with a near-match cut at a fragment
// boundary right after a position marker. M2 at a fragment boundary is pinned
// by TestSlackSplitAvoidsMidWordAt and TestCheckFragmentStarts.
func TestSlackMentionMessagesFollowPosted(t *testing.T) {
	capacity := slackMaxMessageRunes - slackDisplayMaxRunes
	// Each case is the text just before and just after a fragment boundary.
	// The body has no newline before its short last line, so the first
	// fragment is the heading line and the body is cut at exactly capacity
	// code points.
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
			article.Body = strings.Repeat("b", capacity-len([]rune(tc.before))) + tc.before + tc.after + "b" + shortLastLine
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

func TestMentionAts(t *testing.T) {
	cases := []struct {
		name string
		text string
		want []int
	}{
		{"at start", "@name", []int{0}},
		{"after a space", "hi @name", []int{3}},
		{"after punctuation", "(@name)", []int{1}},
		{"name starting with a digit", "@1abc", []int{0}},
		{"name starting with an underscore", "@_x", []int{0}},
		{"name starting with a dot", "@.x", []int{0}},
		{"name starting with a hyphen", "@-x", []int{0}},
		{"non-ASCII name", "@名前", []int{0}},
		{"two mentions", "@a @b", []int{0, 3}},

		{"after a letter", "Tech@Work", nil},
		{"email address", "user@example.com", nil},
		{"after a digit", "a1@b", nil},
		{"after a non-ASCII letter", "さん@xxx", nil},
		{"at end", "x @", nil},
		{"followed by a space", "@ name", nil},
		{"followed by punctuation", "@!", nil},
		{"followed by a zero-width space", "@" + zeroWidthSpace + "name", nil},
		{"empty", "", nil},

		{"after an underscore", "foo_@bar", []int{4}},
		{"after an at sign", "x@@y", []int{2}},
		{"after a zero-width space", "a" + zeroWidthSpace + "@here", []int{2}},
		{"after a backslash", `a\@here`, []int{2}},
		{"after a slash", "youtube.com/@name", []int{12}},
		{"after a colon", "x:@y", []int{2}},
		{"after a full-width colon", "a：@b", []int{2}},
		{"after emphasis markup", "_a_@here", []int{3}},
		{"after a newline", "a\n@x", []int{2}},
		{"after a combining mark", "e\u0301@x", []int{2}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := mentionAts([]rune(tc.text))
			if !slices.Equal(got, tc.want) {
				t.Errorf("mentionAts(%q) = %v, want %v", tc.text, got, tc.want)
			}
		})
	}
}
