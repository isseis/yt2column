//go:build test

package doctestutil

import (
	"slices"
	"testing"
)

// TestSectionByHeading pins SectionByHeading's boundaries: the heading line is
// excluded, the section ends before the next heading of the same or a higher
// level, a missing heading returns the empty string, and a '#' not followed by
// whitespace is not a heading.
func TestSectionByHeading(t *testing.T) {
	const doc = `# Top
top body
## Middle
middle body
### Deep
deep body
#### Deeper
deeper body
## Middle two
middle two body
#109 note
note body
###
hash body
# Top two
top two body`
	for _, tc := range []struct {
		name    string
		heading string
		want    string
	}{
		{
			name:    "level two ends before the next level two",
			heading: "## Middle",
			want:    "middle body\n### Deep\ndeep body\n#### Deeper\ndeeper body\n",
		},
		{
			name:    "level three includes a lower level four",
			heading: "### Deep",
			want:    "deep body\n#### Deeper\ndeeper body\n",
		},
		{
			name:    "level four keeps only its own body",
			heading: "#### Deeper",
			want:    "deeper body\n",
		},
		{
			name:    "level one ends before the next level one",
			heading: "# Top",
			want:    "top body\n## Middle\nmiddle body\n### Deep\ndeep body\n#### Deeper\ndeeper body\n## Middle two\nmiddle two body\n#109 note\nnote body\n###\nhash body\n",
		},
		{
			name:    "missing heading yields the empty string",
			heading: "## Absent",
			want:    "",
		},
		{
			name:    "hash without following whitespace is not a heading",
			heading: "#109 note",
			want:    "",
		},
		{
			name:    "bare hashes are not a heading",
			heading: "###",
			want:    "",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := SectionByHeading(doc, tc.heading); got != tc.want {
				t.Errorf("SectionByHeading(doc, %q) = %q, want %q", tc.heading, got, tc.want)
			}
		})
	}
}

// TestScanDocLines checks the flags of the fence scan: a fence line and the
// lines it encloses are in a block, and a '#' line inside a block stays in it.
// TestFencedBlocks here, and TestFirstCodeBlockAfter and
// TestOutputLayoutDocsContract in cmd/yt2column, observe the same scan only
// through a consumer, so none can tell a wrong flag from a wrong consumer.
func TestScanDocLines(t *testing.T) {
	lines := []string{"intro\n", "```sh\n", "# not a heading\n", "export A=1\n", "```not-closed\n", "```\n", "## Heading\n", "```text\n", "tail"}
	want := []DocLine{
		{Text: "intro", Fence: false, Block: false},
		{Text: "```sh", Fence: true, Block: true},
		{Text: "# not a heading", Fence: false, Block: true},
		{Text: "export A=1", Fence: false, Block: true},
		{Text: "```not-closed", Fence: false, Block: true},
		{Text: "```", Fence: true, Block: true},
		{Text: "## Heading", Fence: false, Block: false},
		{Text: "```text", Fence: true, Block: true},
		{Text: "tail", Fence: false, Block: true},
	}
	if got := ScanDocLines(lines); !slices.Equal(got, want) {
		t.Errorf("ScanDocLines = %#v, want %#v", got, want)
	}
}

// TestFencedBlocks checks the block extraction: a closing fence ends a block,
// an indented fence is a fence, blocks come back in order with their info
// string, and a block left unclosed at the end is reported as not closed.
func TestFencedBlocks(t *testing.T) {
	doc := "intro\n```sh\nexport A=1\n```\ntext\n  ```text\nx\n  ```\n```json\n{}\n"
	want := []FencedBlock{
		{Info: "sh", Lines: []string{"export A=1"}, Closed: true},
		{Info: "text", Lines: []string{"x"}, Closed: true},
		{Info: "json", Lines: []string{"{}"}, Closed: false},
	}
	if got := FencedBlocks(doc); !slices.EqualFunc(got, want, func(a, b FencedBlock) bool {
		return a.Info == b.Info && a.Closed == b.Closed && slices.Equal(a.Lines, b.Lines)
	}) {
		t.Errorf("FencedBlocks = %#v, want %#v", got, want)
	}
}

// TestAnchorFromHeading pins the anchor rule: lowercase, markup punctuation
// dropped, spaces turned into hyphens, digits and connector punctuation kept.
// The rows for headings the README already has were checked against GitHub's
// rendering of the README on 2026-10-26; the others pin the same rule for a
// heading the README does not have yet. The Japanese rows use only characters
// whose anchors were checked against four rendered documents on 2026-10-28:
// GitHub keeps the letters and drops a full-width parenthesis, a middle dot,
// and a full-width colon, as these rows expect.
func TestAnchorFromHeading(t *testing.T) {
	for _, tc := range []struct{ heading, want string }{
		{"## Setting up yt-dlp", "setting-up-yt-dlp"},
		{"### Installing the native messaging host", "installing-the-native-messaging-host"},
		{"## Cache and concurrency", "cache-and-concurrency"},
		{"### The `--out` file", "the---out-file"},
		{"### The Slack post", "the-slack-post"},
		{"### Using DeepSeek", "using-deepseek"},
		{"## Step 2", "step-2"},
		{"## prompt_v2 notes", "prompt_v2-notes"},
		{"## Step\u00a0²", "step²"},
		{"## 使い方", "使い方"},
		{"### 設定（`ファイル`）", "設定ファイル"},
		{"## 料金の推定・表示", "料金の推定表示"},
		{"## サーバ：起動と停止", "サーバ起動と停止"},
	} {
		if got := AnchorFromHeading(tc.heading); got != tc.want {
			t.Errorf("AnchorFromHeading(%q) = %q, want %q", tc.heading, got, tc.want)
		}
	}
}

// TestHeadingsOutsideBlocks checks that a line starting with '#' inside a code
// block is not a heading and that the rest come back in order.
func TestHeadingsOutsideBlocks(t *testing.T) {
	const doc = "# One\ntext\n```text\n# not a heading\n```\n## Two\n"
	want := []string{"# One", "## Two"}
	if got := HeadingsOutsideBlocks(doc); !slices.Equal(got, want) {
		t.Errorf("HeadingsOutsideBlocks(doc) = %q, want %q", got, want)
	}
}

// TestSectionOutsideBlocks checks the fence-aware section extraction: a '#' or
// '##' line inside a code block neither ends the section nor starts one.
func TestSectionOutsideBlocks(t *testing.T) {
	const doc = "# Top\ntop\n## Middle\nmiddle\n```text\n# not a heading\n## also not\n```\n### Deep\ndeep\n## Next\nnext\n"
	for _, tc := range []struct {
		name, heading, want string
	}{
		{"in-block headings do not end the section", "## Middle", "middle\n```text\n# not a heading\n## also not\n```\n### Deep\ndeep\n"},
		{"an in-block heading is not a section start", "## also not", ""},
		{"a missing heading yields the empty string", "## Absent", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := SectionOutsideBlocks(doc, tc.heading); got != tc.want {
				t.Errorf("SectionOutsideBlocks(doc, %q) = %q, want %q", tc.heading, got, tc.want)
			}
		})
	}
}
