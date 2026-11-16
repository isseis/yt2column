//go:build test

package extjob

import (
	"encoding/json"
	"maps"
	"math"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/isseis/yt2column/internal/llm"
	"github.com/isseis/yt2column/internal/publisher"
	"github.com/isseis/yt2column/internal/writer"
	writertestutil "github.com/isseis/yt2column/internal/writer/testutil"
)

// pricedUsage is a generation usage whose model is in the rate table, so the
// article's cost is known.
func pricedUsage() writer.RunUsage {
	return writer.RunUsage{Generation: writer.StageUsage{
		Origin: writer.OriginRun,
		Model:  "claude-haiku-5-5",
		Usage:  llm.KnownUsage(llm.TokenCounts{Input: 23000}),
	}}
}

// doneResult runs the fake writer to the article and returns the article
// result and the result JSON's member names.
func doneResult(t *testing.T, article writer.Article, title string) (ArticleResult, []byte) {
	t.Helper()
	result := run(t.Context(), mustPrepare(t, StartInput{Title: title}),
		fakeWriterFactory(&writertestutil.FakeArticleWriter{Result: article}), time.Now, nil)
	got, ok := result.Article()
	if !ok {
		failure, _ := result.Failure()
		t.Fatalf("not done: %v", failure)
	}
	data, err := marshalResult(result)
	if err != nil {
		t.Fatalf("marshalResult: %v", err)
	}
	return got, data
}

// TestResultDoneArticle ports TestStatusDoneArticle: the Markdown and the file
// are what the publisher renders for the article (the file being what --out
// writes), and the cost is the formatted known cost.
func TestResultDoneArticle(t *testing.T) {
	article := testArticle()
	article.RunUsage = pricedUsage()
	got, _ := doneResult(t, article, "")
	display, err := publisher.NeutralizedDisplayMarkdown(article)
	if err != nil {
		t.Fatalf("NeutralizedDisplayMarkdown: %v", err)
	}
	wantFile, err := publisher.FileContent(article)
	if err != nil {
		t.Fatalf("FileContent: %v", err)
	}
	if got.Markdown != display.Markdown || got.File != wantFile {
		t.Errorf("markdown/file differ from the publisher's rendering")
	}
	usd, known := publisher.FormatArticleCost(article.RunUsage)
	if !known {
		t.Fatal("the priced usage has no known cost")
	}
	if got.Cost != (Cost{State: CostKnown, USD: usd}) {
		t.Errorf("cost = %+v, want known %s", got.Cost, usd)
	}
}

// TestResultCostUnknown ports TestStatusCostUnknown.
func TestResultCostUnknown(t *testing.T) {
	got, _ := doneResult(t, testArticle(), "")
	if got.Cost != (Cost{State: CostUnknown}) {
		t.Errorf("cost = %+v, want the unknown cost", got.Cost)
	}
}

// TestResultArticleNeutralized ports TestStatusArticleNeutralized: the
// result's members are exactly the ones the viewer reads, the Markdown and its
// flag come from NeutralizedDisplayMarkdown, the input's video title and the
// article's channel name are neutralized for the heading ("" for none), and
// the file is unchanged.
func TestResultArticleNeutralized(t *testing.T) {
	refused := testArticle()
	refused.Title = "Talk @channel"
	refused.ChannelName = "@handle"
	refused.Body = "Hi @here and `@code`, <!channel> <@name> <a|b> AC/@x &#64;y https://www.youtube.com/@kept\n\nSource: " + TestVideoURL + "\n"
	plain := testArticle()
	plain.ChannelName = ""
	cases := []struct {
		name        string
		article     writer.Article
		videoTitle  string
		wantTitle   string
		wantChannel string
	}{
		{"mentions", refused, "Live with @here https://www.youtube.com/@x", "Live with @\u200bhere https://www.youtube.com/@\u200bx", "@\u200bhandle"},
		{"no title and no channel", plain, "", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, data := doneResult(t, tc.article, tc.videoTitle)
			var top map[string]json.RawMessage
			if err := json.Unmarshal(data, &top); err != nil {
				t.Fatalf("decode %s: %v", data, err)
			}
			if want := []string{"article", "status", "steps"}; !slices.Equal(slices.Sorted(maps.Keys(top)), want) {
				t.Errorf("result members = %v, want %v", slices.Sorted(maps.Keys(top)), want)
			}
			var article map[string]json.RawMessage
			if err := json.Unmarshal(top["article"], &article); err != nil {
				t.Fatalf("decode the article: %v", err)
			}
			want := []string{"channelName", "cost", "durationMs", "file", "markdown", "mentionsNeutralized", "videoTitle"}
			if !slices.Equal(slices.Sorted(maps.Keys(article)), want) {
				t.Errorf("article members = %v, want %v", slices.Sorted(maps.Keys(article)), want)
			}
			display, err := publisher.NeutralizedDisplayMarkdown(tc.article)
			if err != nil {
				t.Fatalf("NeutralizedDisplayMarkdown: %v", err)
			}
			file, err := publisher.FileContent(tc.article)
			if err != nil {
				t.Fatalf("FileContent: %v", err)
			}
			if got.Markdown != display.Markdown || got.MentionsNeutralized != display.Neutralized {
				t.Errorf("markdown, mentionsNeutralized = %q, %v; want %q, %v", got.Markdown, got.MentionsNeutralized, display.Markdown, display.Neutralized)
			}
			if got.VideoTitle != tc.wantTitle || got.ChannelName != tc.wantChannel || got.File != file {
				t.Errorf("heading/file = %q, %q, file equal %v; want %q, %q", got.VideoTitle, got.ChannelName, got.File == file, tc.wantTitle, tc.wantChannel)
			}
		})
	}
}

// TestResultFailedMembers checks a failed result carries exactly status,
// steps, and failure, plus internalDetail for an internal failure, and no
// article.
func TestResultFailedMembers(t *testing.T) {
	members := func(r Result) []string {
		data, err := marshalResult(r)
		if err != nil {
			t.Fatalf("marshalResult: %v", err)
		}
		var top map[string]json.RawMessage
		if err := json.Unmarshal(data, &top); err != nil {
			t.Fatalf("decode: %v", err)
		}
		return slices.Sorted(maps.Keys(top))
	}
	if got, want := members(failedResult(Steps{}, FailureLLM, DetailNone)), []string{"failure", "status", "steps"}; !slices.Equal(got, want) {
		t.Errorf("members = %v, want %v", got, want)
	}
	if got, want := members(failedResult(Steps{}, FailureInternal, DetailPanic)), []string{"failure", "internalDetail", "status", "steps"}; !slices.Equal(got, want) {
		t.Errorf("members = %v, want %v", got, want)
	}
}

// articlesAtResultLimit returns an article whose largest result JSON, started
// with videoTitle, is exactly the limit, and the same article a byte larger.
// The body, which the result holds twice (the Markdown and the file), takes
// most of the room: start, then 'a's; the model, which it holds once in the
// file, takes the last byte.
func articlesAtResultLimit(t *testing.T, videoTitle, start string) (at, over writer.Article) {
	t.Helper()
	size := func(a writer.Article) int {
		t.Helper()
		n, err := MaxResultSizeForTest(a, videoTitle)
		if err != nil {
			t.Fatalf("MaxResultSizeForTest: %v", err)
		}
		return n
	}
	at = testArticle()
	at.Body = start
	base := size(at)
	at.Body += strings.Repeat("a", (MaxResultBytesForTest-base)/2)
	at.Model += strings.Repeat("m", MaxResultBytesForTest-size(at))
	if got := size(at); got != MaxResultBytesForTest {
		t.Fatalf("the article's largest result is %d bytes, want exactly %d", got, MaxResultBytesForTest)
	}
	over = at
	over.Model += "m"
	if got := size(over); got != MaxResultBytesForTest+1 {
		t.Fatalf("the larger article's largest result is %d bytes, want %d", got, MaxResultBytesForTest+1)
	}
	return at, over
}

// TestResultWithinLimit ports TestStatusResponseWithinLimit: an article whose
// largest result JSON is exactly the limit is done, one a byte larger fails as
// article_too_large, and the done result's JSON fits the limit with the values
// the run varies at their longest. The second case fills the body and the
// title with '@' units, so the bytes the neutralization adds and the title
// must both be counted.
func TestResultWithinLimit(t *testing.T) {
	cases := []struct {
		name       string
		videoTitle string
		line       string
		lines      int
	}{
		{"plain", "", strings.Repeat("a", 999) + "\n", 120},
		{"mentions and a long title", strings.Repeat("@t ", 1000), strings.Repeat("@a ", 300) + "\n", 40},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			at, over := articlesAtResultLimit(t, tc.videoTitle, strings.Repeat(tc.line, tc.lines))
			p := mustPrepare(t, StartInput{Title: tc.videoTitle})
			resultOf := func(a writer.Article) Result {
				return run(t.Context(), p, fakeWriterFactory(&writertestutil.FakeArticleWriter{Result: a}), time.Now, nil)
			}
			done := resultOf(at)
			if _, ok := done.Article(); !ok {
				t.Fatalf("the article at the limit is not done")
			}
			if failure, failed := resultOf(over).Failure(); !failed || failure != FailureArticleTooLarge {
				t.Errorf("the article a byte over: failure = %v, %v; want article_too_large", failure, failed)
			}
			longest := done
			longest.steps = Steps{Draft: StepSkipped, Shorten: StepSkipped, Review: StepSkipped}
			article := *longest.article
			article.DurationMs = time.Duration(math.MaxInt64).Milliseconds()
			longest.article = &article
			data, err := marshalResult(longest)
			if err != nil {
				t.Fatalf("marshalResult: %v", err)
			}
			if len(data) > MaxResultBytesForTest {
				t.Errorf("result is %d bytes, over the %d limit", len(data), MaxResultBytesForTest)
			}
		})
	}
}
