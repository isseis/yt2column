//go:build test

package publisher

import (
	"flag"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/isseis/yt2column/internal/writer"
)

// update rewrites the golden files under testdata/ from renderFileArticle:
//
//	go test -tags test ./internal/publisher -run TestRenderFileArticleGolden -update
var update = flag.Bool("update", false, "rewrite the golden files under testdata/")

const sourceLine = "source: \"https://www.youtube.com/watch?v=abcdefghijk\"\n"

func TestRenderFileArticleLayout(t *testing.T) {
	full := costedArticle()
	full.ChannelName = "Example Channel"
	full.Shorten = &writer.StepModel{Model: "shorten-x", ModelVersion: "s1"}
	full.Review = &writer.ReviewResult{Model: writer.StepModel{Model: "review-x", ModelVersion: ""}}

	cases := []struct {
		name    string
		article writer.Article
		want    string
	}{
		{
			"every optional part",
			full,
			"---\n" +
				"channel: \"Example Channel\"\n" +
				sourceLine +
				"model: \"model-x\"\n" +
				"model_version: \"2026-01-01\"\n" +
				"shorten_model: \"shorten-x\"\n" +
				"shorten_model_version: \"s1\"\n" +
				"review_model: \"review-x\"\n" +
				"review_model_version: \"(none)\"\n" +
				"estimated_cost_usd: \"0.0023\"\n" +
				"---\n\n# A title\n\nExample Channel\n\n" + full.Body,
		},
		{
			"no channel name and no steps",
			validArticle(),
			"---\n" +
				sourceLine +
				"model: \"model-x\"\n" +
				"model_version: \"2026-01-01\"\n" +
				"estimated_cost_usd: \"unknown\"\n" +
				"---\n\n# A title\n\n" + validArticle().Body,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := renderFileArticle(tc.article); got != tc.want {
				t.Fatalf("renderFileArticle =\n%q\nwant\n%q", got, tc.want)
			}
		})
	}
}

// TestRenderFileArticleChannelLine checks the call site of
// escapeLeadingMarkdown: only the line under the title is escaped, while the
// front matter keeps the name as it is.
func TestRenderFileArticleChannelLine(t *testing.T) {
	article := validArticle()
	article.ChannelName = "# News"
	got := renderFileArticle(article)
	if want := "\n---\n\n# A title\n\n\\# News\n\n"; !strings.Contains(got, want) {
		t.Errorf("output lacks the escaped channel line %q:\n%s", want, got)
	}
	if want := "channel: \"# News\"\n"; !strings.Contains(got, want) {
		t.Errorf("output lacks the unescaped front matter value %q:\n%s", want, got)
	}
}

func TestRenderFileArticleOnlyOneStep(t *testing.T) {
	t.Run("only shorten", func(t *testing.T) {
		article := validArticle()
		article.Shorten = &writer.StepModel{Model: "shorten-x"}
		got := renderFileArticle(article)
		if !strings.Contains(got, "shorten_model: \"shorten-x\"\nshorten_model_version: \"(none)\"\n") ||
			strings.Contains(got, "review_") {
			t.Fatalf("front matter = %q", got)
		}
	})
	t.Run("only review", func(t *testing.T) {
		article := validArticle()
		article.Review = &writer.ReviewResult{Model: writer.StepModel{Model: "review-x", ModelVersion: "r1"}}
		got := renderFileArticle(article)
		if !strings.Contains(got, "review_model: \"review-x\"\nreview_model_version: \"r1\"\n") ||
			strings.Contains(got, "shorten_") {
			t.Fatalf("front matter = %q", got)
		}
	})
}

func TestRenderFileArticleCost(t *testing.T) {
	cases := []struct {
		name    string
		article writer.Article
		want    string
	}{
		{"known", costedArticle(), "estimated_cost_usd: \"0.0023\"\n"},
		{"unknown without a run usage", validArticle(), "estimated_cost_usd: \"unknown\"\n"},
		{"unknown for a model not in the table", unpricedArticle(), "estimated_cost_usd: \"unknown\"\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := renderFileArticle(tc.article)
			if strings.Count(got, "estimated_cost_usd") != 1 || !strings.Contains(got, tc.want) {
				t.Fatalf("renderFileArticle = %q, want one %q", got, tc.want)
			}
		})
	}
}

// yamlEscape matches the body of a double-quoted scalar that holds only the
// escape sequences strconv.Quote emits for valid UTF-8 and YAML defines.
var yamlEscape = regexp.MustCompile(`^(?:[^\\"]|\\[abfnrtv\\"]|\\x[0-9a-f]{2}|\\u[0-9a-f]{4}|\\U[0-9a-f]{8})*$`)

// TestRenderFileArticleQuoting writes values that would break an unquoted
// YAML line and checks that the front matter keeps its shape and each value
// reads back unchanged.
func TestRenderFileArticleQuoting(t *testing.T) {
	values := []string{
		`say "hi"`,
		`back\slash`,
		"key: value",
		"# not a comment",
		"---",
		"日本語のチャンネル",
		"emoji 😀",
		"bell\a backspace\b formfeed\f vtab\v",
		"del\x7f next-line\u0085",
		"separators\u2028\u2029 bom\ufeff nbsp\u00a0 tab\t",
		"\U0010ffff unassigned",
		"line\nbreak",
		"carriage\rreturn",
	}
	for _, value := range values {
		t.Run(strconv.Quote(value), func(t *testing.T) {
			article := validArticle()
			article.ChannelName = value
			article.Model = value
			article.ModelVersion = value
			article.SourceURL = value
			article.Shorten = &writer.StepModel{Model: value, ModelVersion: value}
			article.Review = &writer.ReviewResult{Model: writer.StepModel{Model: value, ModelVersion: value}}
			got := renderFileArticle(article)
			front, _, _ := strings.Cut(got, "\n---\n\n")
			if strings.Count(front, "\n") != 9 || strings.ContainsAny(front, "\r\u0085\u2028\u2029\ufeff") {
				t.Fatalf("front matter has an extra line or a raw line break: %q", front)
			}

			lines := strings.Split(got, "\n")
			wantKeys := []string{"---", "channel", "source", "model", "model_version", "shorten_model", "shorten_model_version", "review_model", "review_model_version", "estimated_cost_usd", "---"}
			if len(lines) < len(wantKeys) {
				t.Fatalf("output has %d lines: %q", len(lines), got)
			}
			for i, key := range wantKeys {
				line := lines[i]
				if key == "---" {
					if line != "---" {
						t.Fatalf("line %d = %q, want the delimiter", i, line)
					}
					continue
				}
				name, quoted, ok := strings.Cut(line, ": ")
				if !ok || name != key {
					t.Fatalf("line %d = %q, want key %q", i, line, key)
				}
				if !yamlEscape.MatchString(quoted[1 : len(quoted)-1]) {
					t.Fatalf("line %d uses an escape sequence YAML does not define: %s", i, quoted)
				}
				unquoted, err := strconv.Unquote(quoted)
				if err != nil {
					t.Fatalf("line %d value %s is not a quoted string: %v", i, quoted, err)
				}
				switch key {
				case "channel", "source", "model", "model_version", "shorten_model", "shorten_model_version", "review_model", "review_model_version":
					if unquoted != value {
						t.Fatalf("%s reads back as %q, want %q", key, unquoted, value)
					}
				}
			}
		})
	}
}

func TestEscapeLeadingMarkdown(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"plain name", "Example Channel", "Example Channel"},
		{"japanese name", "例のチャンネル", "例のチャンネル"},
		{"empty", "", ""},
		{"heading", "# News", `\# News`},
		{"heading without text", "#", `\#`},
		{"six hashes", "###### x", `\###### x`},
		{"seven hashes is not a heading", "####### x", "####### x"},
		{"hashtag is not a heading", "#news", "#news"},
		{"quote", "> quoted", `\> quoted`},
		{"dash bullet", "- item", `\- item`},
		{"plus bullet", "+ item", `\+ item`},
		{"star bullet", "* item", `\* item`},
		{"lone dash", "-", `\-`},
		{"dash without space is not a bullet", "-ish", "-ish"},
		{"thematic break of dashes", "---", `\---`},
		{"thematic break of stars with spaces", "* * *", `\* * *`},
		{"thematic break of underscores", "___", `\___`},
		{"two underscores are not a break", "__", "__"},
		{"underscore name", "_name_", "_name_"},
		{"ordered list with dot", "1. first", `1\. first`},
		{"ordered list with parenthesis", "12) first", `12\) first`},
		{"ten digits are not a list marker", "1234567890. x", "1234567890. x"},
		{"digits without a delimiter", "2024 news", "2024 news"},
		{"year then dot without space", "2024.5", "2024.5"},
		{"backtick fence", "```go", "\\```go"},
		{"tilde fence", "~~~", `\~~~`},
		{"two backticks are not a fence", "``x``", "``x``"},
		{"html block", "<div>", `\<div>`},
		{"html comment", "<!-- x", `\<!-- x`},
		{"less-than before a space", "< 3", "< 3"},
		{"reference definition", "[a]: http://example.com", `\[a]: http://example.com`},
		{"bracket without a definition", "[a] b", "[a] b"},
		{"closing html tag", "</div>", `\</div>`},
		{"processing instruction", "<?php", `\<?php`},
		{"declaration", "<!DOCTYPE x", `\<!DOCTYPE x`},
		{"less-than before a digit", "<3", "<3"},
		{"heading with a tab", "#\tx", "\\#\tx"},
		{"bullet with a tab", "-\tx", "\\-\tx"},
		{"thematic break of stars", "***", `\***`},
		{"thematic break of spaced dashes", "- - -", `\- - -`},
		{"thematic break of spaced underscores", "_ _ _", `\_ _ _`},
		{"stars then text is not a break", "***x", "***x"},
		{"empty ordered list item", "1.", `1\.`},
		{"zero is a list number", "0. x", `0\. x`},
		{"footnote definition", "[^1]: x", `\[^1]: x`},
		{"leading backslash", `\# x`, `\\# x`},
		{"four leading spaces", "    code", "code"},
		{"leading spaces before a heading", "  # x", `\# x`},
		{"leading tab", "\tx", "x"},
		{"only spaces", "   ", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := escapeLeadingMarkdown(tc.in); got != tc.want {
				t.Fatalf("escapeLeadingMarkdown(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// TestRenderFileArticleGolden pins whole files. The golden files are also the
// input of the mechanical article check of task 0008 (check_article.py), so
// the front matter the CLI writes stays readable by it.
func TestRenderFileArticleGolden(t *testing.T) {
	withChannel := costedArticle()
	withChannel.ChannelName = "Example Channel"
	withChannel.Body = "Lead paragraph.\n\n## One\n\nText.\n\n## Two\n\nText.\n\n## Three\n\nText.\n\n" +
		"\u51fa\u5178: <https://www.youtube.com/watch?v=abcdefghijk>\n"
	withChannel.Shorten = &writer.StepModel{Model: "claude-haiku-5-5", ModelVersion: "s1"}
	withChannel.Review = &writer.ReviewResult{Model: writer.StepModel{Model: "claude-haiku-5-5"}}

	withoutChannel := withChannel
	withoutChannel.ChannelName = ""
	withoutChannel.Shorten = nil
	withoutChannel.Review = nil

	cases := []struct {
		file    string
		article writer.Article
	}{
		{"file_article_with_channel.md", withChannel},
		{"file_article_without_channel.md", withoutChannel},
	}
	for _, tc := range cases {
		t.Run(tc.file, func(t *testing.T) {
			path := filepath.Join("testdata", tc.file)
			got := renderFileArticle(tc.article)
			if *update {
				if err := os.WriteFile(path, []byte(got), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			want, err := os.ReadFile(path) //nolint:gosec // a fixed path under testdata
			if err != nil {
				t.Fatal(err)
			}
			if got != string(want) {
				t.Fatalf("renderFileArticle differs from %s:\n%s", path, got)
			}
		})
	}
}
