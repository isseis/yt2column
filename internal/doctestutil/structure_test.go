//go:build test

package doctestutil

import (
	"errors"
	"slices"
	"testing"
)

// TestParseStructure pins what the structure counts, at the boundaries of the
// accepted forms, and which sentinel each rejected form wraps. The rejected
// forms are the ones where the shared scan and GitHub's Markdown can disagree.
func TestParseStructure(t *testing.T) {
	for _, tc := range []struct {
		name    string
		doc     string
		want    Structure
		wantErr error
	}{
		{
			name: "headings and code blocks in order",
			doc:  "# One\n## Two\n```sh\n# not a heading\n```\n### Three\n",
			want: Structure{Headings: []HeadingPos{{Line: 1, Level: 1}, {Line: 2, Level: 2}, {Line: 6, Level: 3}}, CodeBlocks: 1},
		},
		{
			name: "six hashes are a heading",
			doc:  "###### Six\n",
			want: Structure{Headings: []HeadingPos{{Line: 1, Level: 6}}},
		},
		{
			name: "a four-space indented hash is code",
			doc:  "    # not a heading\n",
			want: Structure{},
		},
		{
			name: "a dash line after a blank line is a thematic break",
			doc:  "text\n\n---\n",
			want: Structure{},
		},
		{
			name: "two code blocks are both counted",
			doc:  "```sh\na\n```\n## Between\n```json\nb\n```\n",
			want: Structure{Headings: []HeadingPos{{Line: 4, Level: 2}}, CodeBlocks: 2},
		},
		{
			name: "a setext underline inside a fence is code",
			doc:  "```text\ntext\n---\n```\n",
			want: Structure{CodeBlocks: 1},
		},
		{
			name: "an info string on a fence is allowed",
			doc:  "```sh\nexport A=1\n```\n",
			want: Structure{CodeBlocks: 1},
		},
		{
			name: "a three-backtick block may be closed by four backticks",
			doc:  "```text\nx\n````\n",
			want: Structure{CodeBlocks: 1},
		},
		{
			name: "a dash line after a heading is a thematic break",
			doc:  "# Heading\n---\n",
			want: Structure{Headings: []HeadingPos{{Line: 1, Level: 1}}},
		},
		{
			name: "a dash line after a code block is a thematic break",
			doc:  "text\n```\nx\n```\n---\n",
			want: Structure{CodeBlocks: 1},
		},
		{
			name: "a dash line after an empty code block is a thematic break",
			doc:  "text\n```\n```\n---\n",
			want: Structure{CodeBlocks: 1},
		},
		{
			name: "a dash line after a list item is a thematic break",
			doc:  "- item\n---\n",
			want: Structure{},
		},
		{
			name: "a dash line after a blockquote is a thematic break",
			doc:  "> quoted\n---\n",
			want: Structure{},
		},
		{
			name: "a line of only spaces is blank",
			doc:  "text\n   \n---\n",
			want: Structure{},
		},
		{
			name: "a thematic break before a dash line",
			doc:  "***\n---\n",
			want: Structure{},
		},
		{
			name: "a bare ordered list marker before a dash line",
			doc:  "1.\n---\n",
			want: Structure{},
		},
		{
			name: "a dash line after a table is a thematic break",
			doc:  "| a | b |\n|---|---|\n---\n",
			want: Structure{},
		},
		{
			name: "a dash line after an HTML block is not a heading",
			doc:  "<div>\n---\n",
			want: Structure{},
		},
		{
			name: "a dash line after indented code is a thematic break",
			doc:  "    code\n---\n",
			want: Structure{},
		},
		{
			name: "an indented hash line is code after a heading ends the list",
			doc:  "- item\n\n# Heading\n    ## code\n",
			want: Structure{Headings: []HeadingPos{{Line: 3, Level: 1}}},
		},
		{
			name: "an empty document has no structure",
			doc:  "",
			want: Structure{},
		},
		{
			name:    "seven hashes",
			doc:     "####### Seven\n",
			wantErr: ErrUnsupportedHeading,
		},
		{
			name:    "seven hashes with no space",
			doc:     "#######x\n",
			wantErr: ErrUnsupportedHeading,
		},
		{
			name:    "a bare hash line",
			doc:     "##\n",
			wantErr: ErrUnsupportedHeading,
		},
		{
			name:    "an indented heading",
			doc:     "   ## Indented\n",
			wantErr: ErrUnsupportedHeading,
		},
		{
			name:    "a heading in a list item",
			doc:     "- # Item heading\n",
			wantErr: ErrUnsupportedHeading,
		},
		{
			name:    "a heading in an ordered list item",
			doc:     "1. ## Ordered heading\n",
			wantErr: ErrUnsupportedHeading,
		},
		{
			name:    "a heading on a list item's continuation line",
			doc:     "- item\n\n    ## Nested\n",
			wantErr: ErrUnsupportedHeading,
		},
		{
			name:    "a setext equals underline",
			doc:     "Title\n===\n",
			wantErr: ErrUnsupportedHeading,
		},
		{
			name:    "a setext dash underline",
			doc:     "Title\n---\n",
			wantErr: ErrUnsupportedHeading,
		},
		{
			name:    "a setext underline under an indented paragraph continuation",
			doc:     "Title\n    continuation\n---\n",
			wantErr: ErrUnsupportedHeading,
		},
		{
			name:    "a pipe paragraph before a dash line",
			doc:     "| a | b |\n---\n",
			wantErr: ErrUnsupportedHeading,
		},
		{
			name:    "a fence with no closing fence",
			doc:     "```text\nx\n",
			wantErr: ErrUnclosedFence,
		},
		{
			name:    "an indented opening fence",
			doc:     "  ```text\nx\n  ```\n",
			wantErr: ErrUnsupportedFence,
		},
		{
			name:    "an indented closing fence",
			doc:     "```text\nx\n  ```\n",
			wantErr: ErrUnsupportedFence,
		},
		{
			name:    "a tilde fence",
			doc:     "~~~text\nx\n~~~\n",
			wantErr: ErrUnsupportedFence,
		},
		{
			name:    "a four-backtick opening fence",
			doc:     "````text\nx\n````\n",
			wantErr: ErrUnsupportedFence,
		},
		{
			name:    "an info string with a backtick",
			doc:     "```a`b\nx\n```\n",
			wantErr: ErrUnsupportedFence,
		},
		{
			name:    "a closing fence indented with a wide space",
			doc:     "```\nx\n\u3000```\n",
			wantErr: ErrUnsupportedFence,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseStructure(tc.doc)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("ParseStructure(%q) = %+v, %v; want %v", tc.doc, got, err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseStructure(%q) = %+v, %v; want no error", tc.doc, got, err)
			}
			if !slices.Equal(got.Headings, tc.want.Headings) || got.CodeBlocks != tc.want.CodeBlocks {
				t.Errorf("ParseStructure(%q) = %+v, want %+v", tc.doc, got, tc.want)
			}
		})
	}
}

// TestCompareStructures pins which difference comes first: a level at the first
// differing heading (with both positions), the end of the shorter list, the
// code block count, or both.
func TestCompareStructures(t *testing.T) {
	for _, tc := range []struct {
		name        string
		left, right Structure
		want        StructureDiff
	}{
		{
			name:  "matching headings and code blocks",
			left:  Structure{Headings: []HeadingPos{{Line: 1, Level: 1}, {Line: 3, Level: 2}}, CodeBlocks: 1},
			right: Structure{Headings: []HeadingPos{{Line: 1, Level: 1}, {Line: 3, Level: 2}}, CodeBlocks: 1},
			want:  StructureDiff{Kind: StructureSame, Index: -1, LeftCodeBlocks: 1, RightCodeBlocks: 1},
		},
		{
			name:  "the same levels on different lines match",
			left:  Structure{Headings: []HeadingPos{{Line: 1, Level: 1}, {Line: 2, Level: 2}}},
			right: Structure{Headings: []HeadingPos{{Line: 1, Level: 1}, {Line: 9, Level: 2}}},
			want:  StructureDiff{Kind: StructureSame, Index: -1},
		},
		{
			name:  "a level differs in the middle",
			left:  Structure{Headings: []HeadingPos{{Line: 1, Level: 1}, {Line: 3, Level: 2}, {Line: 6, Level: 3}}},
			right: Structure{Headings: []HeadingPos{{Line: 1, Level: 1}, {Line: 3, Level: 3}, {Line: 6, Level: 3}}},
			want:  StructureDiff{Kind: StructureHeadingsDiffer, Index: 1, Left: HeadingPos{Line: 3, Level: 2}, Right: HeadingPos{Line: 3, Level: 3}},
		},
		{
			name:  "one list is a prefix of the other",
			left:  Structure{Headings: []HeadingPos{{Line: 1, Level: 1}, {Line: 2, Level: 2}}},
			right: Structure{Headings: []HeadingPos{{Line: 1, Level: 1}, {Line: 2, Level: 2}, {Line: 4, Level: 3}}},
			want:  StructureDiff{Kind: StructureHeadingsDiffer, Index: 2, Right: HeadingPos{Line: 4, Level: 3}},
		},
		{
			name:  "the other list is a prefix",
			left:  Structure{Headings: []HeadingPos{{Line: 1, Level: 1}, {Line: 2, Level: 2}, {Line: 4, Level: 3}}},
			right: Structure{Headings: []HeadingPos{{Line: 1, Level: 1}, {Line: 2, Level: 2}}},
			want:  StructureDiff{Kind: StructureHeadingsDiffer, Index: 2, Left: HeadingPos{Line: 4, Level: 3}},
		},
		{
			name:  "only the code block count differs",
			left:  Structure{Headings: []HeadingPos{{Line: 1, Level: 1}}, CodeBlocks: 2},
			right: Structure{Headings: []HeadingPos{{Line: 1, Level: 1}}, CodeBlocks: 1},
			want:  StructureDiff{Kind: StructureCodeBlocksDiffer, Index: -1, LeftCodeBlocks: 2, RightCodeBlocks: 1},
		},
		{
			name:  "headings and code blocks both differ",
			left:  Structure{Headings: []HeadingPos{{Line: 1, Level: 1}, {Line: 3, Level: 3}}, CodeBlocks: 1},
			right: Structure{Headings: []HeadingPos{{Line: 1, Level: 1}, {Line: 3, Level: 2}}, CodeBlocks: 2},
			want:  StructureDiff{Kind: StructureBothDiffer, Index: 1, Left: HeadingPos{Line: 3, Level: 3}, Right: HeadingPos{Line: 3, Level: 2}, LeftCodeBlocks: 1, RightCodeBlocks: 2},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := CompareStructures(tc.left, tc.right); got != tc.want {
				t.Errorf("CompareStructures(%+v, %+v) = %+v, want %+v", tc.left, tc.right, got, tc.want)
			}
		})
	}

	// Headings that differ only in wording, and whose lines shift when one
	// version gains a line, are the same structure: the translation of a
	// heading must not change the comparison.
	t.Run("headings that differ only in wording and line match", func(t *testing.T) {
		left, err := ParseStructure("# One\n## Two\n")
		if err != nil {
			t.Fatalf("ParseStructure: %v", err)
		}
		right, err := ParseStructure("# Usage\n\n## Settings\n")
		if err != nil {
			t.Fatalf("ParseStructure: %v", err)
		}
		if got, want := CompareStructures(left, right), (StructureDiff{Kind: StructureSame, Index: -1}); got != want {
			t.Errorf("CompareStructures = %+v, want %+v", got, want)
		}
	})
}

// TestStructureDiffString pins the failure message: it names the first differing
// heading with both versions' positions, or says a version has no heading there,
// and both code block counts when they differ. The message is the only
// observable of a StructureDiff to a failing test, so its wording is compared
// as written.
func TestStructureDiffString(t *testing.T) {
	for _, tc := range []struct {
		name string
		diff StructureDiff
		want string
	}{
		{
			name: "headings and code blocks",
			diff: StructureDiff{
				Kind:            StructureBothDiffer,
				Index:           1,
				Left:            HeadingPos{Line: 12, Level: 3},
				Right:           HeadingPos{Line: 14, Level: 2},
				LeftCodeBlocks:  7,
				RightCodeBlocks: 6,
			},
			want: "headings differ at heading 2: left line 12 level 3, right line 14 level 2; code blocks differ: left 7, right 6",
		},
		{
			name: "a shorter heading list",
			diff: StructureDiff{Kind: StructureHeadingsDiffer, Index: 2, Right: HeadingPos{Line: 3, Level: 4}},
			want: "headings differ at heading 3: left missing, right line 3 level 4",
		},
		{
			name: "only code blocks",
			diff: StructureDiff{Kind: StructureCodeBlocksDiffer, Index: -1, LeftCodeBlocks: 3, RightCodeBlocks: 2},
			want: "code blocks differ: left 3, right 2",
		},
		{
			name: "matching structures",
			diff: StructureDiff{Kind: StructureSame, Index: -1},
			want: "structures match",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.diff.String(); got != tc.want {
				t.Errorf("String() = %q, want %q", got, tc.want)
			}
		})
	}
}
