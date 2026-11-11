//go:build test

package publisher

import (
	"errors"
	"math"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/isseis/yt2column/internal/writer"
)

// z is the U+200B the neutralization inserts, short so that a want string
// shows where it goes.
const z = zeroWidthSpace

// articleWith returns a valid article with the given title, channel name, and
// body.
func articleWith(title, channel, body string) writer.Article {
	a := validArticle()
	a.Title = title
	a.ChannelName = channel
	a.Body = body
	return a
}

// checkNeutralizedContract checks the contract of NeutralizedDisplayMarkdown
// for a against got: removing the inserted U+200B gives the display Markdown,
// and Neutralized reports whether any was inserted.
func checkNeutralizedContract(t *testing.T, a writer.Article, got NeutralizedDisplay) {
	t.Helper()
	display := renderDisplayMarkdown(a)
	if strings.ReplaceAll(got.Markdown, z, "") != strings.ReplaceAll(display, z, "") {
		t.Errorf("Markdown without U+200B differs from the display Markdown:\n%q\n%q", got.Markdown, display)
	}
	added := strings.Count(got.Markdown, z) - strings.Count(display, z)
	if got.Neutralized != (added > 0) {
		t.Errorf("Neutralized = %v, but %d U+200B were inserted", got.Neutralized, added)
	}
}

func TestNeutralizedDisplayMarkdown(t *testing.T) {
	cases := []struct {
		name    string
		article writer.Article
		want    string
	}{
		// The heading line and the channel name line.
		{"title", articleWith("Talk with @here", "", "Body.\n"), "# Talk with @" + z + "here\n\nBody.\n"},
		{"channel name", articleWith("T", "@handle", "Body.\n"), "# T\n\n@" + z + "handle\n\nBody.\n"},
		{"URL path in the title", articleWith("See https://www.youtube.com/@handle", "", "Body.\n"), "# See https://www.youtube.com/@handle\n\nBody.\n"},
		{"bracket in the title before a body URL", articleWith("A [draft]", "", "https://www.youtube.com/@handle\n"), "# A [draft]\n\nhttps://www.youtube.com/@" + z + "handle\n"},

		// The examples of the requirements.
		{"mentions", articleWith("T", "", "@channel @here @名前\n"), "# T\n\n@" + z + "channel @" + z + "here @" + z + "名前\n"},
		{"inside words", articleWith("T", "", "Tech@Work user@example.com\n"), "# T\n\nTech@Work user@example.com\n"},
		{"after a slash outside a URL", articleWith("T", "", "AC/@here\n"), "# T\n\nAC/@" + z + "here\n"},
		{"URL path", articleWith("T", "", "https://www.youtube.com/@handle\n"), "# T\n\nhttps://www.youtube.com/@handle\n"},
		{"URL path in angle brackets", articleWith("T", "", "<https://www.youtube.com/@handle>\n"), "# T\n\n<https://www.youtube.com/@handle>\n"},
		{"URL with no host", articleWith("T", "", "https://@channel\n"), "# T\n\nhttps://@" + z + "channel\n"},
		{"URL query", articleWith("T", "", "https://example.com/?@here\n"), "# T\n\nhttps://example.com/?@" + z + "here\n"},
		{"after a parenthesis in a URL", articleWith("T", "", "https://example.com)@here\n"), "# T\n\nhttps://example.com)@" + z + "here\n"},
		{"slash after a query", articleWith("T", "", "https://example.com?/@here\n"), "# T\n\nhttps://example.com?/@" + z + "here\n"},
		{"port", articleWith("T", "", "https://example.com:8080/@here\n"), "# T\n\nhttps://example.com:8080/@" + z + "here\n"},
		{"reference in a URL path", articleWith("T", "", "https://example.com/&#64;here\n"), "# T\n\nhttps://example.com/&#64;" + z + "here\n"},

		// The forms Mattermost does not autolink (U1 to U6).
		{"U1 letter before the scheme", articleWith("T", "", "xhttps://example.com/@here\n"), "# T\n\nxhttps://example.com/@" + z + "here\n"},
		{"U2 underscore in the host", articleWith("T", "", "https://a_b.example.com/@here\n"), "# T\n\nhttps://a_b.example.com/@" + z + "here\n"},
		{"U3 angle bracket before the slash", articleWith("T", "", "https://example.com<b/@here\n"), "# T\n\nhttps://example.com<b/@" + z + "here\n"},
		{"U4 bracket before the URL", articleWith("T", "", "[https://example.com/@here]\n"), "# T\n\n[https://example.com/@" + z + "here]\n"},
		{"U5 URL starting in a code span", articleWith("T", "", "`https://example.com`/@here\n"), "# T\n\n`https://example.com`/@" + z + "here\n"},
		{"U6 colon before the scheme", articleWith("T", "", ":https://example.com/@here 参照:https://example.com/@x\n"), "# T\n\n:https://example.com/@" + z + "here 参照:https://example.com/@" + z + "x\n"},

		// Escapes and character references (R1, R2).
		{"R1 escaped", articleWith("T", "", `\@channel`+"\n"), "# T\n\n\\@" + z + "channel\n"},
		{"R1 after a reference", articleWith("T", "", "&#97;@here\n"), "# T\n\n&#97;@" + z + "here\n"},
		{"R2 reference for @", articleWith("T", "", "&#64;channel\n"), "# T\n\n&#64;" + z + "channel\n"},
		{"R2 reference after @", articleWith("T", "", "@&#99;hannel\n"), "# T\n\n@" + z + "&#99;hannel\n"},
		{"R2 escape after @", articleWith("T", "", `@\_name`+"\n"), "# T\n\n@" + z + "\\_name\n"},
		{"named reference", articleWith("T", "", "&commat;here &amp; x\n"), "# T\n\n&commat;" + z + "here &amp;" + z + " x\n"},
		{"hexadecimal references", articleWith("T", "", "&#x40;a &#X40;b &#xFF20;c &#0064;d\n"), "# T\n\n&#x40;" + z + "a &#X40;" + z + "b &#xFF20;" + z + "c &#0064;" + z + "d\n"},
		{"references kept", articleWith("T", "", "a&#64;b &#65; &#; &; &#x; &#99999999999999999999999999;\n"), "# T\n\na&#64;b &#65; &#; &; &#x; &#99999999999999999999999999;\n"},

		// The Viewer drops markup when the reader selects and copies (S1 to S3).
		{"S1 emphasis after @", articleWith("T", "", "@**channel**\n"), "# T\n\n@" + z + "**channel**\n"},
		{"S1 code span after @", articleWith("T", "", "`@`here\n"), "# T\n\n`@" + z + "`here\n"},
		{"S2 code span before the scheme", articleWith("T", "", "`x`https://example.com/@here\n"), "# T\n\n`x`https://example.com/@" + z + "here\n"},
		{"S2 emphasis before the scheme", articleWith("T", "", "a*https://example.com/@here*\n"), "# T\n\na*https://example.com/@" + z + "here*\n"},
		{"S2 letter before an angle bracket", articleWith("T", "", "x<https://example.com/@here>\n"), "# T\n\nx<https://example.com/@" + z + "here>\n"},
		{"S3 bracket in an earlier paragraph", articleWith("T", "", "- [x\n\nhttps://example.com/@here\n"), "# T\n\n- [x\n\nhttps://example.com/@" + z + "here\n"},

		// Code is neutralized too.
		{"code span", articleWith("T", "", "Run `@here` now.\n"), "# T\n\nRun `@" + z + "here` now.\n"},
		{"fenced code block", articleWith("T", "", "```\n@here\n```\n"), "# T\n\n```\n@" + z + "here\n```\n"},

		// The sequences the Webhook post refuses.
		{"post refusals", articleWith("T", "", "<!channel> <@name> <a|b>\n"), "# T\n\n<!channel> <@" + z + "name> <a|b>\n"},
		{"look-alike", articleWith("T", "", "＠channel\n"), "# T\n\n＠" + z + "channel\n"},
		{"two URL paths", articleWith("T", "", "(https://example.com/@a/@b)\n"), "# T\n\n(https://example.com/@a/@b)\n"},
		{"URL path on the channel name line", articleWith("T", "https://www.youtube.com/@handle", "Body.\n"), "# T\n\nhttps://www.youtube.com/@handle\n\nBody.\n"},
		{"bracket after the URL start", articleWith("T", "", "https://example.com/[x/@a\n"), "# T\n\nhttps://example.com/[x/@a\n"},
		{"earlier start open across a refused one", articleWith("T", "", "https://a.example/xhttps://b/@c\n"), "# T\n\nhttps://a.example/xhttps://b/@c\n"},

		// Nothing to neutralize: the display Markdown is returned unchanged.
		{"no target", validArticle(), "# A title\n\n" + validArticle().Body},
		{"no target with a channel name", articleWith("A title", "Example Channel", validArticle().Body), "# A title\n\nExample Channel\n\n" + validArticle().Body},
		{"U+200B already present", articleWith("T", "", "a"+z+"b\n"), "# T\n\na" + z + "b\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := NeutralizedDisplayMarkdown(tc.article)
			if err != nil {
				t.Fatalf("NeutralizedDisplayMarkdown error = %v, want nil", err)
			}
			if got.Markdown != tc.want {
				t.Errorf("Markdown =\n%q\nwant\n%q", got.Markdown, tc.want)
			}
			if want := tc.want != renderDisplayMarkdown(tc.article); got.Neutralized != want {
				t.Errorf("Neutralized = %v, want %v", got.Neutralized, want)
			}
			checkNeutralizedContract(t, tc.article, got)
		})
	}
}

// TestNeutralizedDisplayMarkdownURLPathConditions checks each of the five
// conditions of a URL path '/@': each input is meant to fail the condition
// its name gives, so its '@' is neutralized.
func TestNeutralizedDisplayMarkdownURLPathConditions(t *testing.T) {
	const kept = "x https://example.com/@here"
	got, err := NeutralizedDisplayMarkdown(articleWith("T", "", kept))
	if err != nil || got.Neutralized {
		t.Fatalf("baseline %q: Neutralized = %v, err = %v; want it kept", kept, got.Neutralized, err)
	}

	// Each body splits at its only '@' unit: before and after are the text
	// around the U+200B the unit gets.
	neutralized := []struct {
		name, before, after string
	}{
		{"1 not after a slash", "x https://example.com/-@", "here"},
		{"1 a reference", "x https://example.com/&#64;", "here"},
		{"2 no scheme", "x example.com/@", "here"},
		{"2 uppercase scheme", "x HTTPS://example.com/@", "here"},
		{"2 letter before the scheme", "xhttps://example.com/@", "here"},
		{"2 ASCII punctuation before the scheme", "x:https://example.com/@", "here"},
		{"2 letter before an angle bracket", "x<https://example.com/@", "here>"},
		{"2 angle bracket before an angle bracket", "x <<https://example.com/@", "here>"},
		{"4 newline before the slash", "x https://example.com/\n/@", "here"},
		{"3 empty host", "x https:///@", "here"},
		{"3 host starting with a hyphen", "x https://-example.com/@", "here"},
		{"3 underscore in the host", "x https://a_b.example.com/@", "here"},
		{"3 port after the host", "x https://example.com:8080/@", "here"},
		{"4 space", "x https://example.com/a b/@", "here"},
		{"4 ideographic space", "x https://example.com/a\u3000b/@", "here"},
		{"4 less-than", "x https://example.com/a<b/@", "here"},
		{"4 greater-than", "x https://example.com/a>b/@", "here"},
		{"4 question mark", "x https://example.com/a?b/@", "here"},
		{"4 number sign", "x https://example.com/a#b/@", "here"},
		{"4 backquote", "x https://example.com/a`b/@", "here"},
		{"5 bracket earlier", "[ x https://example.com/@", "here"},
	}
	for _, tc := range neutralized {
		t.Run(tc.name, func(t *testing.T) {
			got, err := NeutralizedDisplayMarkdown(articleWith("T", "", tc.before+tc.after))
			if err != nil {
				t.Fatalf("NeutralizedDisplayMarkdown error = %v", err)
			}
			if want := "# T\n\n" + tc.before + z + tc.after; got.Markdown != want {
				t.Errorf("Markdown =\n%q\nwant\n%q", got.Markdown, want)
			}
		})
	}

	// Condition 2's angle-bracket branch: a '<' right before the scheme
	// counts when one of the allowed runes precedes the '<'.
	keptAngle := []string{
		"<https://example.com/@here>",
		"x <https://example.com/@here>",
		"(<https://example.com/@here>)",
		"名<https://example.com/@here>",
	}
	for _, body := range keptAngle {
		t.Run("2 kept "+body, func(t *testing.T) {
			got, err := NeutralizedDisplayMarkdown(articleWith("T", "", body))
			if err != nil || got.Markdown != "# T\n\n"+body {
				t.Errorf("Markdown = %q, err = %v; want %q unchanged", got.Markdown, err, body)
			}
		})
	}
}

// TestNeutralizedDisplayMarkdownKeptAts checks the two rules that leave an
// '@' unchanged, each with an input only that rule can keep: before the
// result is checked, the input is shown to escape the other rule.
func TestNeutralizedDisplayMarkdownKeptAts(t *testing.T) {
	t.Run("inside a word", func(t *testing.T) {
		const body = "Tech@Work"
		at := strings.Index(body, "@")
		// The URL path rule would not keep it: it does not follow '/' and no
		// URL starts before it.
		if body[at-1] == '/' || strings.Contains(body, "://") {
			t.Fatalf("%q could be a URL path '/@'", body)
		}
		got, err := NeutralizedDisplayMarkdown(articleWith("T", "", body))
		if err != nil || got.Markdown != "# T\n\n"+body || got.Neutralized {
			t.Errorf("Markdown = %q, Neutralized = %v, err = %v; want %q unchanged", got.Markdown, got.Neutralized, err, body)
		}
	})
	t.Run("URL path", func(t *testing.T) {
		const body = "x https://www.youtube.com/@handle"
		// The word rule would not keep it: the '@' is a target.
		if !slices.Contains(mentionAtsOf([]rune(body)), strings.Index(body, "@")) {
			t.Fatalf("the '@' of %q is not a target", body)
		}
		got, err := NeutralizedDisplayMarkdown(articleWith("T", "", body))
		if err != nil || got.Markdown != "# T\n\n"+body || got.Neutralized {
			t.Errorf("Markdown = %q, Neutralized = %v, err = %v; want %q unchanged", got.Markdown, got.Neutralized, err, body)
		}
	})
}

// TestNeutralizedDisplayMarkdownLinearTime checks that the time grows
// linearly with the input: for an input four times as long it must take less
// than eight times as long (a linear pass takes about four times, a pass that
// searches back from each '@' or each URL start about sixteen). The minimum
// of several runs keeps a slow run from deciding, and no absolute limit is
// set, because make test runs with the race detector.
func TestNeutralizedDisplayMarkdownLinearTime(t *testing.T) {
	// Each unit is one long line: a pass that searches back to the line start
	// or the text start takes time quadratic in its length. Without '[' every
	// URL start opens and every "/@" is judged inside an open URL; with '['
	// the starts after the first are refused by condition 5.
	units := map[string]string{
		"open URLs":   "x https://a.example/@b/@c @d &#64;e /@f AC/@g ://h ",
		"after a '['": "x https://a.example/@b/@c @d &#64;e /@f AC/@g ://h [i ",
	}
	for name, unit := range units {
		t.Run(name, func(t *testing.T) {
			timeOf := func(repeat int) time.Duration {
				a := articleWith("T", "", strings.Repeat(unit, repeat))
				best := time.Duration(math.MaxInt64)
				// One more run than measured warms up the allocator.
				for i := range 6 {
					start := time.Now()
					if _, err := NeutralizedDisplayMarkdown(a); err != nil {
						t.Fatalf("NeutralizedDisplayMarkdown error = %v", err)
					}
					if i > 0 {
						best = min(best, time.Since(start))
					}
				}
				return best
			}
			const n = 2000
			short, long := timeOf(n), timeOf(4*n)
			if long >= 8*short {
				t.Errorf("time for %d units = %v, for %d units = %v: ratio %.1f, want under 8", n, short, 4*n, long, float64(long)/float64(short))
			}
		})
	}
}

// TestNeutralizedDisplayMarkdownRejectsUnpublishable checks that
// NeutralizedDisplayMarkdown runs CheckPublishable: an article it rejects
// yields no text. FileContent's check is covered through FilePublisher by
// TestFilePublisherRejectsArticle.
func TestNeutralizedDisplayMarkdownRejectsUnpublishable(t *testing.T) {
	article := validArticle()
	article.Title = "a\nb"
	got, err := NeutralizedDisplayMarkdown(article)
	if !errors.Is(err, writer.ErrInvalidArticle) {
		t.Errorf("NeutralizedDisplayMarkdown error = %v, want ErrInvalidArticle", err)
	}
	if got != (NeutralizedDisplay{}) {
		t.Errorf("NeutralizedDisplayMarkdown = %+v, want the zero value", got)
	}
}

func TestNeutralizeHeading(t *testing.T) {
	cases := []struct {
		name, value, want string
	}{
		{"mention", "Live with @here", "Live with @" + z + "here"},
		{"URL path", "https://www.youtube.com/@handle", "https://www.youtube.com/@" + z + "handle"},
		{"reference", "&#64;channel", "&#64;" + z + "channel"},
		{"look-alike", "＠channel", "＠" + z + "channel"},
		{"inside a word", "Tech@Work", "Tech@Work"},
		{"empty", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := NeutralizeHeading(tc.value); got != tc.want {
				t.Errorf("NeutralizeHeading(%q) = %q, want %q", tc.value, got, tc.want)
			}
		})
	}
}
