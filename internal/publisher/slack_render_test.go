//go:build test

package publisher

import (
	"reflect"
	"strings"
	"testing"

	"github.com/isseis/yt2column/internal/writer"
)

func TestNeutralizeMentions(t *testing.T) {
	const z = zeroWidthSpace
	cases := []struct {
		name  string
		value string
		want  string
	}{
		{"expected value written with a literal U+200B", "hi @name", "hi @\u200bname"},
		{"mention", "hi @name", "hi @" + z + "name"},
		{"two mentions", "@a @b", "@" + z + "a @" + z + "b"},
		{"full-width at", "＠everyone", "＠" + z + "everyone"},
		{"small at", "﹫here", "﹫" + z + "here"},
		{"after underscore", "foo_@bar", "foo_@" + z + "bar"},
		{"unchanged when no at is a target", "Tech@Work a @ b @!", "Tech@Work a @ b @!"},
		{"unchanged after a slash", "youtube.com/@name", "youtube.com/@name"},
		{"slash exception only for that at", "/@a @b", "/@a @" + z + "b"},
		{"empty", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := neutralizeMentions(tc.value)
			if got != tc.want {
				t.Errorf("neutralizeMentions(%q) = %q, want %q", tc.value, got, tc.want)
			}
			if again := neutralizeMentions(got); again != got {
				t.Errorf("neutralizeMentions is not idempotent: %q then %q", got, again)
			}
		})
	}
}

func TestNeutralizeProse(t *testing.T) {
	const z = zeroWidthSpace
	cases := []struct {
		name string
		body string
		want string
	}{
		{"prose only", "see @name now", "see @" + z + "name now"},
		{"inside a code span", "see `@name` now", "see `@name` now"},
		{"inside and outside a code span", "`@a` and @b", "`@a` and @" + z + "b"},
		{"inside a fence", "x\n```\n@a\n```\n@b", "x\n```\n@a\n```\n@" + z + "b"},
		// A full-width at takes three bytes, so a rune index differs from the
		// byte offset after it. Comparing the rune index with the ranges would
		// move the code span's edge by two bytes per full-width at.
		{"full-width at before a code span", "＠＠ `＠a` ＠b", "＠＠ `＠a` ＠" + z + "b"},
		{"full-width at in a code span", "`＠a` ＠b ＠c", "`＠a` ＠" + z + "b ＠" + z + "c"},
		{"no target", "Tech@Work plain", "Tech@Work plain"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			prose, ok := writer.ProseRanges(tc.body)
			if !ok {
				t.Fatalf("ProseRanges(%q) could not judge the body", tc.body)
			}
			if got := neutralizeProse(tc.body, prose); got != tc.want {
				t.Errorf("neutralizeProse(%q) = %q, want %q", tc.body, got, tc.want)
			}
		})
	}
}

func TestNeutralizeProseGivenRanges(t *testing.T) {
	const body = "@a @b"
	cases := []struct {
		name  string
		prose []writer.ByteRange
		want  string
	}{
		{"no ranges means all code", nil, body},
		{"range over the first at only", []writer.ByteRange{{Start: 0, End: 2}}, "@" + zeroWidthSpace + "a @b"},
		{"range ending at the at", []writer.ByteRange{{Start: 0, End: 3}}, "@" + zeroWidthSpace + "a @b"},
		{"range covering both", []writer.ByteRange{{Start: 0, End: 5}}, "@" + zeroWidthSpace + "a @" + zeroWidthSpace + "b"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := neutralizeProse(body, tc.prose); got != tc.want {
				t.Errorf("neutralizeProse(%q, %v) = %q, want %q", body, tc.prose, got, tc.want)
			}
		})
	}
}

func TestStageModels(t *testing.T) {
	gen, short, rev := writer.StageGeneration, writer.StageShorten, writer.StageReview
	cases := []struct {
		name    string
		shorten *writer.StepModel
		review  *writer.ReviewResult
		want    []stageModel
	}{
		{"generation only", nil, nil, []stageModel{{"g", []writer.Stage{gen}}}},
		{
			"shorten with the same model and a different version", &writer.StepModel{Model: "g", ModelVersion: "v2"}, nil,
			[]stageModel{{"g", []writer.Stage{gen, short}}},
		},
		{
			"same model for all stages", &writer.StepModel{Model: "g"}, &writer.ReviewResult{Model: writer.StepModel{Model: "g"}},
			[]stageModel{{"g", []writer.Stage{gen, short, rev}}},
		},
		{
			"review model differs", &writer.StepModel{Model: "g"}, &writer.ReviewResult{Model: writer.StepModel{Model: "r"}},
			[]stageModel{{"g", []writer.Stage{gen, short}}, {"r", []writer.Stage{rev}}},
		},
		{
			"shorten model differs, review reuses generation", &writer.StepModel{Model: "s"}, &writer.ReviewResult{Model: writer.StepModel{Model: "g"}},
			[]stageModel{{"g", []writer.Stage{gen, rev}}, {"s", []writer.Stage{short}}},
		},
		{
			"review only", nil, &writer.ReviewResult{Model: writer.StepModel{Model: "r"}},
			[]stageModel{{"g", []writer.Stage{gen}}, {"r", []writer.Stage{rev}}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := stageModels(writer.Article{Model: "g", Shorten: tc.shorten, Review: tc.review})
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("stageModels = %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestRenderSlackPostLayout(t *testing.T) {
	article := costedArticle()
	article.Body = "First.\n\nSource: https://example.com\n"
	article.ModelVersion = "2026-01-01"
	const foot = "Models: model-x (generation) · Estimated cost: unknown"
	cases := []struct {
		name   string
		mutate func(a *writer.Article)
		want   string
	}{
		{
			"with channel", func(a *writer.Article) { a.ChannelName = "Chan" },
			"# A title（Chan）\n\nFirst.\n\nSource: https://example.com\n\n" + "Models: model-x (generation) · Estimated cost: unknown",
		},
		{
			"without channel", func(*writer.Article) {},
			"# A title\n\nFirst.\n\nSource: https://example.com\n\n" + foot,
		},
		{
			"version empty gives the same text", func(a *writer.Article) { a.ModelVersion = "" },
			"# A title\n\nFirst.\n\nSource: https://example.com\n\n" + foot,
		},
		{"all stages, two models", func(a *writer.Article) {
			a.Shorten = &writer.StepModel{Model: "model-x", ModelVersion: "9"}
			a.Review = &writer.ReviewResult{Model: writer.StepModel{Model: "rev", ModelVersion: "3"}}
		}, "# A title\n\nFirst.\n\nSource: https://example.com\n\n" +
			"Models: model-x (generation, shorten) · rev (review) · Estimated cost: unknown"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a := article
			a.RunUsage = writer.RunUsage{}
			tc.mutate(&a)
			if got := renderSlackPost(a).text; got != tc.want {
				t.Errorf("text = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestRenderSlackPostCost(t *testing.T) {
	if got := renderSlackPost(costedArticle()).text; !strings.HasSuffix(got, "Estimated cost: 0.0023 USD") {
		t.Errorf("known cost: %q", got)
	}
	if got := renderSlackPost(unpricedArticle()).text; !strings.HasSuffix(got, "Estimated cost: unknown") {
		t.Errorf("unknown cost: %q", got)
	}
}

func TestRenderSlackPostKeepTogetherFrom(t *testing.T) {
	cases := []struct {
		name     string
		body     string
		wantTail string // text from keepTogetherFrom up to the footnote
	}{
		{"several lines", "A\n\nB\n\nSource: x\n", "Source: x"},
		{"trailing newlines are not a line", "A\n\nSource: x\n\n\n", "Source: x"},
		{"single line", "Only", "Only"},
		// A target before the last line gains a U+200B, which shifts the rune
		// index; counting on the raw body would point one rune too early.
		{"neutralized at before the last line", "@a\n\nLast\n", "Last"},
		{"multibyte body", "あ\nいう", "いう"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a := validArticle()
			a.Title = "題"
			a.Body = tc.body
			post := renderSlackPost(a)
			rest := string([]rune(post.text)[post.keepTogetherFrom:])
			if !strings.HasPrefix(rest, tc.wantTail+"\n\nModels: ") {
				t.Errorf("text from keepTogetherFrom = %q, want it to start with %q then the footnote", rest, tc.wantTail)
			}
		})
	}
}

func TestRenderSlackPostNeutralizesMentions(t *testing.T) {
	const z = zeroWidthSpace
	a := validArticle()
	a.Title = "T @name"
	a.ChannelName = "＠everyone"
	a.Model = "m_@here"
	a.Body = "x @body `@code`\n\n```\n@fence\n```\n\nSource: Tech@Work\n"
	got := renderSlackPost(a).text
	want := "# T @" + z + "name（＠" + z + "everyone）\n\n" +
		"x @" + z + "body `@code`\n\n```\n@fence\n```\n\nSource: Tech@Work\n\n" +
		"Models: m_@" + z + "here (generation) · Estimated cost: unknown"
	if got != want {
		t.Errorf("text = %q, want %q", got, want)
	}
}

func TestRenderSlackPostKeepsMidWordAt(t *testing.T) {
	a := validArticle()
	a.Title = "Tech@Work"
	a.ChannelName = "user@example.com"
	a.Body = "Mail me@example.com\n"
	want := "# Tech@Work\uff08user@example.com\uff09\n\nMail me@example.com\n\nModels: model-x (generation) \u00b7 Estimated cost: unknown"
	if got := renderSlackPost(a).text; got != want {
		t.Errorf("text = %q, want %q", got, want)
	}
}

func TestRenderSlackPostUnjudgedBody(t *testing.T) {
	a := validArticle()
	a.Body = "```\n@here\n"
	if _, ok := writer.ProseRanges(a.Body); ok {
		t.Fatal("ProseRanges judged an unclosed fence")
	}
	if got := renderSlackPost(a).text; !strings.HasPrefix(got, "# A title\n\n```\n@here\n\nModels: ") {
		t.Errorf("an unjudged body must stay as it is: %q", got)
	}
}
