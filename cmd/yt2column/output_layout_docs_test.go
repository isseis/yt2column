//go:build test

package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/isseis/yt2column/internal/llm"
	"github.com/isseis/yt2column/internal/publisher"
	"github.com/isseis/yt2column/internal/writer"
)

// Headings that label the README's two output examples. The guard finds each
// example as the first fenced code block after its heading line.
const (
	readmeFileExampleHeading  = "### The `--out` file"
	readmeSlackExampleHeading = "### The Slack post"
)

// outputLayoutArticle returns the fixed article the README's output examples
// show: a channel name, a mixed set of step models, and a known cost. The
// values are fictional.
func outputLayoutArticle() writer.Article {
	callTime := time.Date(2026, time.January, 5, 0, 0, 0, 0, time.UTC) // Monday 00:00 UTC, off-peak
	usage := func(model string, input, output uint64) writer.StageUsage {
		return writer.StageUsage{
			Origin:   writer.OriginRun,
			Model:    model,
			Usage:    llm.KnownUsage(llm.TokenCounts{Input: input, Output: output}),
			CallTime: callTime,
		}
	}
	return writer.Article{
		Title:       "Caching transcripts by video ID",
		ChannelName: "Example Channel",
		Body: "The cache key comes from the video ID alone.\n\n## One key per video\n\nTwo URLs for one video share one entry.\n\n" +
			"\u51fa\u5178: <https://www.youtube.com/watch?v=abcdefghijk>\n",
		SourceURL:    "https://www.youtube.com/watch?v=abcdefghijk",
		Model:        "deepseek-flash",
		ModelVersion: "v4",
		Shorten:      &writer.StepModel{Model: "deepseek-flash", ModelVersion: "v4"},
		Review:       &writer.ReviewResult{Model: writer.StepModel{Model: "claude-haiku-5-5", ModelVersion: "2026-01-01"}},
		RunUsage: writer.RunUsage{
			Generation: usage("deepseek-flash", 20000, 3000),
			Shorten:    usage("deepseek-flash", 4000, 2500),
			Review:     usage("claude-haiku-5-5", 22000, 500),
		},
	}
}

// firstCodeBlockAfter returns the content of the first fenced code block after
// the line that equals heading, and whether there is one before the next heading. It does not use
// sectionByHeading, because a '#' line inside the block would end that section.
func firstCodeBlockAfter(doc, heading string) (string, bool) {
	var b strings.Builder
	seenHeading, inBlock := false, false
	for line := range strings.Lines(doc) {
		trimmed := strings.TrimRight(line, "\n")
		switch {
		case inBlock && strings.HasPrefix(trimmed, "```"):
			return b.String(), true
		case inBlock:
			b.WriteString(line)
		case seenHeading && strings.HasPrefix(trimmed, "```"):
			inBlock = true
		case seenHeading && headingLevel(line) > 0:
			// The next heading ends the search, so a deleted example does not
			// pick up a later block.
			return "", false
		case strings.TrimSpace(trimmed) == heading:
			seenHeading = true
		}
	}
	return "", false
}

// TestOutputLayoutDocsContract checks that the README's two output examples
// equal the real output for the fixed article.
func TestOutputLayoutDocsContract(t *testing.T) {
	article := outputLayoutArticle()
	readme := readDoc(t, readmePath)

	// The --out example is what FilePublisher writes.
	path := filepath.Join(t.TempDir(), "article.md")
	pub, err := publisher.NewFilePublisher(path)
	if err != nil {
		t.Fatalf("NewFilePublisher: %v", err)
	}
	if err := pub.Publish(context.Background(), article); err != nil {
		t.Fatalf("Publish: %v", err)
	}
	written, err := os.ReadFile(path) //nolint:gosec // a path under t.TempDir
	if err != nil {
		t.Fatal(err)
	}
	checkReadmeExample(t, readme, readmeFileExampleHeading, string(written))

	// The Slack example is the single message the article is posted as.
	messages, err := publisher.SlackMessagesForTest(article)
	if err != nil {
		t.Fatalf("SlackMessagesForTest: %v", err)
	}
	if len(messages) != 1 {
		t.Fatalf("the fixed article posts as %d messages, want 1 so the example is the whole post", len(messages))
	}
	checkReadmeExample(t, readme, readmeSlackExampleHeading, messages[0])
}

// checkReadmeExample fails the test unless the example under heading equals
// want, ignoring trailing newlines.
func checkReadmeExample(t *testing.T, readme, heading, want string) {
	t.Helper()
	got, ok := firstCodeBlockAfter(readme, heading)
	if !ok {
		t.Fatalf("README has no code block under %q", heading)
	}
	if strings.TrimRight(got, "\n") != strings.TrimRight(want, "\n") {
		t.Errorf("README example under %q differs from the real output.\nREADME:\n%s\nreal:\n%s", heading, got, want)
	}
}

// TestFirstCodeBlockAfter checks the helper on input where a '#' line inside
// the block and a second block follow the heading.
func TestFirstCodeBlockAfter(t *testing.T) {
	const doc = "intro\n### Label\ntext\n```text\n# inside\nbody\n```\n```text\nsecond\n```\n"
	got, ok := firstCodeBlockAfter(doc, "### Label")
	if !ok || got != "# inside\nbody\n" {
		t.Errorf("firstCodeBlockAfter = %q, %v; want the first block only", got, ok)
	}
	if _, ok := firstCodeBlockAfter(doc, "### Missing"); ok {
		t.Error("firstCodeBlockAfter found a block under a missing heading")
	}
	if _, ok := firstCodeBlockAfter("### Label\n### Next\n```text\nx\n```\n", "### Label"); ok {
		t.Error("firstCodeBlockAfter ran past the next heading")
	}
	if _, ok := firstCodeBlockAfter("### Label\nno block\n", "### Label"); ok {
		t.Error("firstCodeBlockAfter found a block where there is none")
	}
}
