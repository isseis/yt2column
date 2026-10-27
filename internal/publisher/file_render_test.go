//go:build test

package publisher

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/isseis/yt2column/internal/writer"
)

const sourceLine = "source: \"https://www.youtube.com/watch?v=abcdefghijk\"\n"

func TestRenderFileArticleLayout(t *testing.T) {
	full := costedArticle()
	full.ChannelName = "Example Channel"
	full.Shorten = &writer.StepModel{Model: "shorten-x", ModelVersion: "s1"}
	full.Review = &writer.ReviewResult{Model: writer.StepModel{Model: "review-x", ModelVersion: ""}}

	onlyShorten := validArticle()
	onlyShorten.Shorten = &writer.StepModel{Model: "shorten-x"}
	onlyReview := validArticle()
	onlyReview.Review = &writer.ReviewResult{Model: writer.StepModel{Model: "review-x", ModelVersion: "r1"}}

	cases := []struct {
		name    string
		article writer.Article
		want    string
	}{
		{
			"only shorten",
			onlyShorten,
			"---\n" + sourceLine +
				"model: \"model-x\"\nmodel_version: \"2026-01-01\"\n" +
				"shorten_model: \"shorten-x\"\nshorten_model_version: \"(none)\"\n" +
				"estimated_cost_usd: \"unknown\"\n---\n\n# A title\n\n" + onlyShorten.Body,
		},
		{
			"only review",
			onlyReview,
			"---\n" + sourceLine +
				"model: \"model-x\"\nmodel_version: \"2026-01-01\"\n" +
				"review_model: \"review-x\"\nreview_model_version: \"r1\"\n" +
				"estimated_cost_usd: \"unknown\"\n---\n\n# A title\n\n" + onlyReview.Body,
		},
		{
			"cost unknown for a model not in the table",
			unpricedArticle(),
			"---\n" + sourceLine +
				"model: \"model-x\"\nmodel_version: \"2026-01-01\"\n" +
				"estimated_cost_usd: \"unknown\"\n---\n\n# A title\n\n" + unpricedArticle().Body,
		},
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
// escapeLeadingMarkdown: the line under the title is escaped.
func TestRenderFileArticleChannelLine(t *testing.T) {
	article := validArticle()
	article.ChannelName = "# News"
	got := renderFileArticle(article)
	if want := "\n---\n\n# A title\n\n\\# News\n\n"; !strings.Contains(got, want) {
		t.Errorf("output lacks the escaped channel line %q:\n%s", want, got)
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
			if strings.ContainsAny(front, "\r\u0085\u2028\u2029\ufeff") {
				t.Fatalf("front matter has a raw line break: %q", front)
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
				if !ok || name != key || len(quoted) < 2 {
					t.Fatalf("line %d = %q, want key %q", i, line, key)
				}
				if !yamlEscape.MatchString(quoted[1 : len(quoted)-1]) {
					t.Fatalf("line %d uses an escape sequence YAML does not define: %s", i, quoted)
				}
				// The round trip only guards against a lossy replacement of
				// strconv.Quote; the YAML validity is the yamlEscape check above.
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

// TestFileContentMatchesFilePublisher checks, for every golden article, that
// FileContent returns byte for byte what FilePublisher writes, and that both
// are the committed golden file, which predates FileContent.
func TestFileContentMatchesFilePublisher(t *testing.T) {
	for _, tc := range goldenCases() {
		t.Run(tc.name, func(t *testing.T) {
			out := filepath.Join(t.TempDir(), "article.md")
			p, err := NewFilePublisher(out)
			if err != nil {
				t.Fatal(err)
			}
			if err := p.Publish(context.Background(), tc.article); err != nil {
				t.Fatalf("Publish error = %v", err)
			}
			written, err := os.ReadFile(out) //nolint:gosec // a path under t.TempDir
			if err != nil {
				t.Fatal(err)
			}
			got, err := FileContent(tc.article)
			if err != nil {
				t.Fatalf("FileContent error = %v", err)
			}
			if got != string(written) {
				t.Errorf("FileContent =\n%q\nwant the written file\n%q", got, written)
			}
			if want := readGolden(t, tc.name+".md"); got != want {
				t.Errorf("FileContent differs from %s", tc.name+".md")
			}
		})
	}
}

// TestFormatArticleCostUnknown checks the unknown result, which each caller
// replaces with its own placeholder. The known value reaches both callers and
// is pinned by the golden files.
func TestFormatArticleCostUnknown(t *testing.T) {
	if usd, known := FormatArticleCost(unpricedArticle().RunUsage); usd != "" || known {
		t.Errorf("FormatArticleCost = %q, %v, want \"\", false", usd, known)
	}
}
