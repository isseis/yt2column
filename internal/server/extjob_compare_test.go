//go:build test

package server

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/isseis/yt2column/internal/articlecache"
	"github.com/isseis/yt2column/internal/extjob"
	"github.com/isseis/yt2column/internal/llm"
	"github.com/isseis/yt2column/internal/writer"
)

// This file is temporary. internal/extjob copies the server's input
// validation, result rendering, and failure classification, and its tests
// copy the server's test rows; a wrong copied expectation passes both
// suites, each against its own code. These tests give both implementations
// the same inputs and require the same outputs, so a difference fails. They
// are removed together with the server.

// TestExtjobAcceptsWhatTheServerAccepts feeds one table of inputs to the
// server's body validation and to extjob.Prepare and requires the same
// verdict and, for an accepted input, the same video ID.
func TestExtjobAcceptsWhatTheServerAccepts(t *testing.T) {
	ts := newTestServer(t)
	inputs := []extjob.StartInput{
		{VideoURL: testValidVideoURL, Transcript: testValidRaw},
		{VideoURL: "https://youtu.be/dQw4w9WgXcQ", Transcript: testValidRaw, Title: "T", ChannelName: "C", Description: "D"},
		{VideoURL: "https://m.youtube.com/shorts/dQw4w9WgXcQ", Transcript: testValidRaw},
		{VideoURL: testValidVideoURL, Transcript: testValidRaw, ChannelName: " \t"},
		{VideoURL: testValidVideoURL, Transcript: testValidRaw, ChannelName: "a\x1bb"},
		{VideoURL: testValidVideoURL, Transcript: testValidRaw, ChannelName: "a\u202eb"},
		{VideoURL: "", Transcript: testValidRaw},
		{VideoURL: "   ", Transcript: testValidRaw},
		{VideoURL: testValidVideoURL, Transcript: ""},
		{VideoURL: testValidVideoURL, Transcript: " \t "},
	}
	// Every row of the server's rejection tables goes through both, so a row
	// added there is compared too.
	for _, raw := range invalidVideoURLs {
		inputs = append(inputs, extjob.StartInput{VideoURL: raw, Transcript: testValidRaw})
	}
	for _, raw := range invalidTranscriptRaws {
		inputs = append(inputs, extjob.StartInput{VideoURL: testValidVideoURL, Transcript: raw})
	}
	for _, in := range inputs {
		body, err := json.Marshal(map[string]string{
			"videoUrl": in.VideoURL, "transcript": in.Transcript, "title": in.Title,
			"channelName": in.ChannelName, "description": in.Description, "mode": modePlain,
		})
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		served, rej := ts.validateBody(body)
		prepared, err := extjob.Prepare(in)
		if (rej == nil) != (err == nil) {
			t.Errorf("input %+v: server rejected = %v, extjob rejected = %v", in, rej != nil, err != nil)
			continue
		}
		if err != nil && !errors.Is(err, extjob.ErrInvalidInput) {
			t.Errorf("input %+v: extjob error = %v, want ErrInvalidInput", in, err)
		}
		if rej == nil && served.videoID != prepared.VideoID() {
			t.Errorf("input %+v: video ID server %q, extjob %q", in, served.videoID, prepared.VideoID())
		}
	}
}

// TestExtjobRendersWhatTheServerRenders gives the same articles to the
// server's article member and extjob's and requires every value to agree:
// the display Markdown and its flag, the neutralized heading values, the
// bytes of the file, the estimated cost, and the duration.
func TestExtjobRendersWhatTheServerRenders(t *testing.T) {
	mentions := testArticle()
	mentions.Title = "Talk @channel"
	mentions.ChannelName = "@handle"
	mentions.Body = "Hi @here and `@code`, <!channel> <@name> <a|b> AC/@x &#64;y https://www.youtube.com/@kept\n\nSource: " + testValidVideoURL + "\n"
	priced := testArticle()
	priced.RunUsage = pricedUsage()
	articles := map[string]writer.Article{"plain": testArticle(), "mentions": mentions, "priced": priced}
	for name, article := range articles {
		for _, title := range []string{"", "Live with @here https://www.youtube.com/@x"} {
			t.Run(name+"/"+title, func(t *testing.T) {
				duration := 5*time.Second + 250*time.Millisecond
				want, err := articleStatus(article, title, duration)
				if err != nil {
					t.Fatalf("articleStatus: %v", err)
				}
				got, err := extjob.ArticleResultForTest(article, title, duration)
				if err != nil {
					t.Fatalf("ArticleResultForTest: %v", err)
				}
				if got.Markdown != want.Markdown || got.MentionsNeutralized != want.MentionsNeutralized ||
					got.VideoTitle != want.VideoTitle || got.ChannelName != want.ChannelName ||
					got.File != want.File || got.DurationMs != want.DurationMs ||
					got.Cost.State != want.Cost.State || got.Cost.USD != want.Cost.USD {
					t.Errorf("extjob article member differs from the server's:\nextjob %+v\nserver %+v", got, *want)
				}
			})
		}
	}
}

// TestExtjobClassifiesWhatTheServerClassifies gives the same write errors to
// both classifications and requires the same failure. A rejected key is the
// one difference by design: the server reports it as llm_failed (the
// value it does not have), extjob as api_key_rejected, which extjob's own
// tests check.
func TestExtjobClassifiesWhatTheServerClassifies(t *testing.T) {
	errs := []error{
		writer.ErrPromptTooLarge,
		context.DeadlineExceeded,
		llm.ErrEmptyResponse,
		llm.ErrTruncated,
		writer.ErrInvalidTranscript,
		writer.ErrInvalidTemplate,
		errors.New("the provider refused"),
		// A wrapped prompt-too-large failure also matches ErrInvalidTemplate:
		// the order of the rules decides.
		errors.Join(writer.ErrPromptTooLarge, writer.ErrInvalidTemplate),
	}
	for _, err := range errs {
		if got, want := extjob.ClassifyWriteForTest(err).String(), failureCode(classifyWrite(err)); got != want {
			t.Errorf("%v: extjob %s, server %s", err, got, want)
		}
	}
	if got := extjob.ClassifyWriteForTest(llm.ErrAuthRejected); got != extjob.FailureAPIKeyRejected {
		t.Errorf("a rejected key: extjob %v, want api_key_rejected", got)
	}
	if got := failureCode(classifyWrite(llm.ErrAuthRejected)); got != "llm_failed" {
		t.Errorf("a rejected key: server %s, want llm_failed", got)
	}
}

// TestExtjobSizeLimitAndStepStatesMatchTheServer checks the size limit is the
// same number, and each step state has the same code, in the same position,
// so the one is not left behind when the other is edited.
func TestExtjobSizeLimitAndStepStatesMatchTheServer(t *testing.T) {
	if extjob.MaxResultBytesForTest != maxStatusResponseBytes {
		t.Errorf("extjob limit %d, server limit %d", extjob.MaxResultBytesForTest, maxStatusResponseBytes)
	}
	for _, state := range articlecache.StageStates() {
		if got, want := extjob.StepState(state).String(), stageStateCode(state); got != want {
			t.Errorf("step state %d: extjob %q, server %q", state, got, want)
		}
	}
}
