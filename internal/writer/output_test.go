//go:build test

package writer

import (
	"context"
	"strings"
	"testing"

	"github.com/isseis/yt2column/internal/llm"
	"github.com/isseis/yt2column/internal/llm/testutil"
)

// testSourceURL is the source URL of validTranscript.
const testSourceURL = "https://www.youtube.com/watch?v=" + testVideoID

// testSourceBlock is the source block of validTranscript, spelled out here
// rather than built with sourceBlock so the fixed format is pinned.
const testSourceBlock = "出典: <" + testSourceURL + ">\n"

// writeResponse writes validTranscript with a client that answers resp.
func writeResponse(t *testing.T, resp llm.GenerateResponse) (Article, error) {
	t.Helper()
	w := mustNew(t, &llmtestutil.FakeLLMClient{Result: resp}, Options{})
	return w.Write(context.Background(), validTranscript())
}

// responseWithText is validResponse with its Text replaced.
func responseWithText(text string) llm.GenerateResponse {
	resp := validResponse()
	resp.Text = text
	return resp
}

func TestWriteArticle(t *testing.T) {
	cases := []struct {
		name      string
		resp      llm.GenerateResponse
		wantTitle string
		wantBody  string // the generated body, before the separator
	}{
		{
			"requirement example",
			llm.GenerateResponse{Text: "# 見出しのタイトル\n\n本文の段落\n", Model: "m-1", ModelVersion: "fp-1"},
			"見出しのタイトル", "\n本文の段落\n",
		},
		{
			"empty ModelVersion",
			llm.GenerateResponse{Text: "# 見出しのタイトル\n\n本文の段落\n", Model: "m-1"},
			"見出しのタイトル", "\n本文の段落\n",
		},
		{
			"Markdown and surrounding whitespace kept",
			responseWithText("# T\n  \n## Sub\n\n- item\n- [link](https://example.com/)\n\n\tend  \n\n"),
			"T", "  \n## Sub\n\n- item\n- [link](https://example.com/)\n\n\tend  \n\n",
		},
		{
			"spaces and tabs trimmed from title",
			responseWithText("#  タイトル  \n本文"),
			"タイトル", "本文",
		},
		{
			"tab trimmed from title",
			responseWithText("# \tタイトル\t\n本文"),
			"タイトル", "本文",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a, err := writeResponse(t, tc.resp)
			if err != nil {
				t.Fatalf("Write error = %v", err)
			}
			want := Article{
				Title:        tc.wantTitle,
				Body:         tc.wantBody + "\n\n" + testSourceBlock,
				SourceURL:    testSourceURL,
				Model:        tc.resp.Model,
				ModelVersion: tc.resp.ModelVersion,
			}
			if a != want {
				t.Errorf("Write article = %+v, want %+v", a, want)
			}
		})
	}
}

func TestWriteSourceBlock(t *testing.T) {
	const otherURL = "https://www.youtube.com/watch?v=XXXXXXXXXXX"
	bodies := []struct{ name, body string }{
		{"no trailing newline", "本文"},
		{"one trailing newline", "本文\n"},
		{"several trailing newlines", "本文\n\n\n"},
		{"trailing CR", "本文\r"},
		{"fake source line", "本文\n\n出典: " + otherURL},
		{"fake source autolink", "本文\n\n出典: <" + otherURL + ">\n"},
	}
	for _, tc := range bodies {
		t.Run(tc.name, func(t *testing.T) {
			a, err := writeResponse(t, responseWithText("# T\n"+tc.body))
			if err != nil {
				t.Fatalf("Write error = %v", err)
			}
			if a.SourceURL != testSourceURL {
				t.Errorf("SourceURL = %q, want %q", a.SourceURL, testSourceURL)
			}
			// The separator puts a blank line before the block however the
			// body ends.
			if want := tc.body + "\n\n" + testSourceBlock; a.Body != want {
				t.Errorf("Body = %q, want %q", a.Body, want)
			}
		})
	}
}

func TestWriteRejectsMalformedText(t *testing.T) {
	const title, body = markOutTitle, markOutBody
	cases := []struct{ name, text string }{
		{"empty", ""},
		{"whitespace only", " \n\t\u3000"},
		{"invalid UTF-8", "# " + title + "\n" + body + "\xff"},
		{"first line not a heading", title + "\n" + body},
		{"leading blank line", "\n# " + title + "\n" + body},
		{"level-2 heading", "## " + title + "\n" + body},
		{"no space after #", "#" + title + "\n" + body},
		{"code fence first", "```markdown\n# " + title + "\n" + body + "\n```"},
		{"Setext heading", title + "\n===\n" + body},
		{"empty title", "# \n" + body},
		{"spaces and tab only title", "#  \t\n" + body},
		{"ideographic space only title", "# \u3000\n" + body},
		{"title with CR", "# " + title + "\r\n" + body},
		{"title with ESC", "# " + title + "\x1b[31m\n" + body},
		{"title ending with #", "# " + title + " #\n" + body},
		{"title ending with # before spaces", "# " + title + " # \n" + body},
		{"title ending with # before a tab", "# " + title + " #\t\n" + body},
		{"no body", "# " + title},
		{"empty body", "# " + title + "\n"},
		{"whitespace-only body", "# " + title + "\n \n\t\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a, err := writeResponse(t, responseWithText(tc.text))
			requireMalformed(t, a, err)
		})
	}
}

func TestWriteRejectsInvalidModel(t *testing.T) {
	cases := []struct {
		name                string
		model, modelVersion string
	}{
		{"empty Model", "", markOutModelVersion},
		{"ideographic space Model", "\u3000", markOutModelVersion},
		{"invalid UTF-8 Model", markOutModel + "\xff", markOutModelVersion},
		{"Model with ESC", markOutModel + "\x1b[31m", markOutModelVersion},
		{"Model with newline", markOutModel + "\nfake", markOutModelVersion},
		{"space ModelVersion", markOutModel, " "},
		{"invalid UTF-8 ModelVersion", markOutModel, markOutModelVersion + "\xff"},
		{"ModelVersion with ESC", markOutModel, markOutModelVersion + "\x1b[0m"},
		{"ModelVersion with newline", markOutModel, markOutModelVersion + "\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := validResponse()
			resp.Model, resp.ModelVersion = tc.model, tc.modelVersion
			a, err := writeResponse(t, resp)
			requireMalformed(t, a, err)
		})
	}
}

// padTo returns prefix followed by 'a' bytes up to n bytes in total.
func padTo(prefix string, n int) string {
	return prefix + strings.Repeat("a", n-len(prefix))
}

func TestWriteOutputSizeLimits(t *testing.T) {
	textPrefix := "# " + markOutTitle + "\n" + markOutBody
	cases := []struct {
		name   string
		modify func(resp *llm.GenerateResponse, size int)
		limit  int
	}{
		{"Text", func(resp *llm.GenerateResponse, size int) { resp.Text = padTo(textPrefix, size) }, maxTextBytes},
		{"Model", func(resp *llm.GenerateResponse, size int) { resp.Model = padTo(markOutModel, size) }, maxModelBytes},
		{"ModelVersion", func(resp *llm.GenerateResponse, size int) { resp.ModelVersion = padTo(markOutModelVersion, size) }, maxModelBytes},
	}
	for _, tc := range cases {
		t.Run(tc.name+"/at limit", func(t *testing.T) {
			resp := validResponse()
			tc.modify(&resp, tc.limit)
			if _, err := writeResponse(t, resp); err != nil {
				t.Errorf("Write error = %v, want nil", err)
			}
		})
		t.Run(tc.name+"/over limit", func(t *testing.T) {
			resp := validResponse()
			tc.modify(&resp, tc.limit+1)
			a, err := writeResponse(t, resp)
			requireMalformed(t, a, err)
		})
	}
}

func TestWriteRejectsMarkdownHazards(t *testing.T) {
	const body = markOutBody
	rejected := []struct{ name, body string }{
		{"unclosed backtick fence", body + "\n```go\nfmt.Println()"},
		{"unclosed tilde fence", body + "\n~~~\n" + body},
		{"unclosed details", "<details>\n" + body},
		{"closed details", "<details>" + body + "</details>"},
		{"div hidden", "<div hidden>" + body + "</div>"},
		{"inline span hidden", body + " <span hidden>x</span>"},
		{"comment", body + "\n<!-- " + body + " -->"},
		{"script", body + "\n<script>"},
	}
	for _, tc := range rejected {
		t.Run(tc.name, func(t *testing.T) {
			a, err := writeResponse(t, responseWithText("# "+markOutTitle+"\n"+tc.body))
			requireMalformed(t, a, err)
		})
	}
	accepted := []struct{ name, body string }{
		{"details in a code span", body + " `<details>`"},
		{"details in a closed fence", body + "\n```\n<details>\n```\n"},
		{"URI autolink", body + " <https://example.com/>"},
		{"closed fence", body + "\n```go\nfmt.Println()\n```"},
	}
	for _, tc := range accepted {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := writeResponse(t, responseWithText("# "+markOutTitle+"\n"+tc.body)); err != nil {
				t.Errorf("Write error = %v, want nil", err)
			}
		})
	}
}
