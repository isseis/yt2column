//go:build test

package publisher

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/isseis/yt2column/internal/llm"
	"github.com/isseis/yt2column/internal/writer"
)

// update rewrites the golden files under testdata/golden/ from the renderers:
//
//	go test -tags test ./internal/publisher -run Golden -update
var update = flag.Bool("update", false, "rewrite the golden files under testdata/golden/")

// goldenDir holds <case>.md (renderFileArticle) and <case>.slack.txt
// (renderSlackPost's text) for each case of TestRenderGolden. The files are
// compared byte for byte, so the pre-commit whitespace hooks skip this
// directory and .gitattributes marks it -text.
var goldenDir = filepath.Join("testdata", "golden")

// goldenCallTime is a Monday 00:00 UTC: outside deepseek-flash's peak window,
// so its rate does not depend on when the test runs.
var goldenCallTime = time.Date(2026, time.January, 5, 0, 0, 0, 0, time.UTC)

// goldenBody is the body of every golden case except where a case puts an '@'
// in it. It has the lead, three sections, and the source line, so the .md
// files also pass the mechanical article check of task 0008
// (check_article.py); that reading is kept in sync by hand (implementation
// plan, section 5.3) and no test runs the script.
const goldenBody = "Lead paragraph.\n\n## One\n\nText.\n\n## Two\n\nText.\n\n## Three\n\nText.\n\n" +
	"\u51fa\u5178: <https://www.youtube.com/watch?v=abcdefghijk>\n"

// goldenUsage returns a stage usage of model run in this call.
func goldenUsage(model string, input, output uint64) writer.StageUsage {
	return writer.StageUsage{
		Origin:   writer.OriginRun,
		Model:    model,
		Usage:    llm.KnownUsage(llm.TokenCounts{Input: input, Output: output}),
		CallTime: goldenCallTime,
	}
}

// goldenArticle returns an article whose three stages all ran with
// claude-haiku-5-5, with a channel name and a known cost.
func goldenArticle() writer.Article {
	const model = "claude-haiku-5-5"
	return writer.Article{
		Title:        "A title",
		ChannelName:  "Example Channel",
		Body:         goldenBody,
		SourceURL:    "https://www.youtube.com/watch?v=abcdefghijk",
		Model:        model,
		ModelVersion: "2026-01-01",
		Shorten:      &writer.StepModel{Model: model, ModelVersion: "2026-01-01"},
		Review:       &writer.ReviewResult{Model: writer.StepModel{Model: model, ModelVersion: "2026-01-01"}},
		RunUsage: writer.RunUsage{
			Generation: goldenUsage(model, 20000, 3000),
			Shorten:    goldenUsage(model, 4000, 2500),
			Review:     goldenUsage(model, 22000, 500),
		},
	}
}

// goldenCases returns the articles TestRenderGolden pins, by case name.
func goldenCases() []struct {
	name    string
	article writer.Article
} {
	withoutChannel := goldenArticle()
	withoutChannel.ChannelName = ""

	mixedModels := goldenArticle()
	mixedModels.Model = "deepseek-flash"
	mixedModels.ModelVersion = "v4"
	mixedModels.Shorten = &writer.StepModel{Model: "deepseek-flash", ModelVersion: "v4"}
	mixedModels.RunUsage.Generation = goldenUsage("deepseek-flash", 20000, 3000)
	mixedModels.RunUsage.Shorten = goldenUsage("deepseek-flash", 4000, 2500)

	shortenSkipped := mixedModels
	shortenSkipped.Shorten = nil
	shortenSkipped.RunUsage.Shorten = writer.StageUsage{}

	costUnknown := goldenArticle()
	costUnknown.Model = "model-x"
	costUnknown.ModelVersion = ""
	costUnknown.Shorten = &writer.StepModel{Model: "model-x"}
	costUnknown.Review = &writer.ReviewResult{Model: writer.StepModel{Model: "model-x"}}
	costUnknown.RunUsage = writer.RunUsage{
		Generation: goldenUsage("model-x", 20000, 3000),
		Shorten:    goldenUsage("model-x", 4000, 2500),
		Review:     goldenUsage("model-x", 22000, 500),
	}

	channel := func(name string) writer.Article {
		a := goldenArticle()
		a.ChannelName = name
		return a
	}

	mentions := goldenArticle()
	mentions.Title = "Notes for @alice"
	mentions.ChannelName = "@team"
	mentions.Body = "Thanks to @bob and \uff20everyone. See `config.yaml`.\n\n" +
		strings.TrimPrefix(goldenBody, "Lead paragraph.\n\n")

	midWord := goldenArticle()
	midWord.Title = "Inside Tech@Work"
	midWord.ChannelName = "Tech@Work"
	midWord.Body = "Write to user@example.com about Tech@Work.\n\n" +
		strings.TrimPrefix(goldenBody, "Lead paragraph.\n\n")

	return []struct {
		name    string
		article writer.Article
	}{
		{"same_model", goldenArticle()},
		{"without_channel", withoutChannel},
		{"mixed_models", mixedModels},
		{"shorten_skipped", shortenSkipped},
		{"cost_unknown", costUnknown},
		{"channel_heading", channel("# Channel")},
		{"channel_list", channel("- Channel")},
		{"channel_trailing_hash", channel("C#")},
		// mentions.slack.txt holds U+200B after each neutralized '@'; it is
		// invisible in most editors. TestRenderGoldenNeutralized checks where
		// it is.
		{"mentions", mentions},
		{"mid_word_at", midWord},
	}
}

// TestRenderGolden pins the whole output of both renderers for representative
// articles, so a reviewer can read the finished layout in testdata/golden/ and
// a change of the layout shows as a diff of those files. It does not replace
// the per-element tests in file_render_test.go and slack_render_test.go.
func TestRenderGolden(t *testing.T) {
	for _, tc := range goldenCases() {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.article.CheckPublishable(); err != nil {
				t.Fatalf("golden input is not publishable: %v", err)
			}
			checkGolden(t, tc.name+".md", renderFileArticle(tc.article))
			checkGolden(t, tc.name+".slack.txt", renderSlackPost(tc.article).text)
		})
	}
}

// TestRenderGoldenNeutralized checks the invisible part of mentions.slack.txt:
// U+200B follows each target '@' outside code, and nowhere else.
func TestRenderGoldenNeutralized(t *testing.T) {
	got := readGolden(t, "mentions.slack.txt")
	for _, want := range []string{"@\u200balice", "@\u200bteam", "@\u200bbob", "\uff20\u200beveryone"} {
		if !strings.Contains(got, want) {
			t.Errorf("mentions.slack.txt lacks %q", want)
		}
	}
	if n := strings.Count(got, zeroWidthSpace); n != 4 {
		t.Errorf("mentions.slack.txt has %d U+200B, want 4", n)
	}
	if strings.Contains(readGolden(t, "mid_word_at.slack.txt"), zeroWidthSpace) {
		t.Error("mid_word_at.slack.txt has U+200B; a mid-word '@' is not a target")
	}
}

// checkGolden compares got with the golden file name, after rewriting it when
// -update is set.
func checkGolden(t *testing.T, name, got string) {
	t.Helper()
	if *update {
		if err := os.WriteFile(filepath.Join(goldenDir, name), []byte(got), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if want := readGolden(t, name); got != want {
		t.Errorf("output differs from %s (rerun with -update and review the diff):\n%s", name, got)
	}
}

// readGolden returns the content of the golden file name.
func readGolden(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(goldenDir, name)) //nolint:gosec // a fixed path under testdata
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
