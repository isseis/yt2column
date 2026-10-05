//go:build test

package writer

import (
	"errors"
	"strconv"
	"strings"
	"testing"
)

func TestCheckBodyMarkdown(t *testing.T) {
	cases := []struct {
		name string
		body string
		want error // nil to accept
	}{
		// Accepted forms.
		{"less-than before a space", "a < b", nil},
		{"less-than before a digit", "1 <2", nil},
		{"less-than before non-ASCII", "<東京>", nil},
		{"email autolink starting with a digit", "<1@a.bc>", nil},
		{"HTML in a code span", "`<details>`", nil},
		{"HTML in a double-backtick code span holding a backtick", "``a`<b>``", nil},
		{"HTML in code spans on two lines", "`<i>` x\ny `<b>`", nil},
		{"HTML in a code span after a less-than without greater-than", "`<i>` a < b `x`", nil},
		{"HTML in a closed fence", "```\n<details>\n```", nil},
		{"HTML in a closed tilde fence with backtick info", "~~~ a`b\n<div>\n~~~", nil},
		{"closed fence with info string", "本文\n```go\nfmt.Println()\n```\n", nil},
		{"fence closed by a longer run", "```\nx\n`````", nil},
		{"fence closed with three-space indent and trailing blanks", "```\nx\n   ``` \t", nil},
		{"tilde line inside a backtick fence", "```\n~~~\n<div>\n```", nil},
		{"fence lines left out of the code-span conditions", "~~~ [x]\ny\n~~~\n```go\nz\n```\n`<details>`", nil},
		{"URI autolink", "<https://example.com/>", nil},
		{"URI autolink with an upper-case scheme", "<HTTPS://E.EXAMPLE/>", nil},
		{"URI autolink with a two-letter scheme and nothing after the colon", "<ab:>", nil},
		{"URI autolink with a 32-character scheme", "<" + strings.Repeat("a", 32) + ":x>", nil},
		{"escaped less-than", `\<div>`, nil},
		{"three backslashes", `\\\<div>`, nil},

		// Raw HTML.
		{"unclosed details", "<details>\n本文", errRawHTML},
		{"closed details", "<details>x</details>", errRawHTML},
		{"div with an attribute", "<div hidden>", errRawHTML},
		{"inline span in a paragraph", "段落 <span hidden>x</span>", errRawHTML},
		{"comment", "<!-- -->", errRawHTML},
		{"script", "<script>", errRawHTML},
		{"unclosed comment in mid-line", "a <!--", errRawHTML},
		{"upper-case tag", "<DIV>", errRawHTML},
		{"end tag", "</div>", errRawHTML},
		{"processing instruction", "<?php", errRawHTML},
		{"HTML on a later line", "a\nb\n<div>", errRawHTML},

		// Over-rejection: accepted by CommonMark, rejected here.
		{"less-than before a letter", "x<y", errRawHTML},
		{"less-than in a link destination", "[a](http://a<b)", errRawHTML},
		{"HTML in an indented code block", "    <div>", errRawHTML},
		{"email autolink starting with a letter", "<user@example.com>", errRawHTML},
		{"code span across lines", "`<details>\n`", errRawHTML},
		{"code span with an unpaired backtick on the line", "`<details>` `", errRawHTML},
		{"code span with an unpaired backtick on another line", "`<details>`\n`", errRawHTML},
		{"code span with a backslash on the line", `\ ` + "`<b>`", errRawHTML},
		{"code span with an open bracket on another line", "[\n`<b>`", errRawHTML},
		{"code span with a close bracket on the line", "a] `<b>`", errRawHTML},
		{"code span with a backtick inside angle brackets on another line", "<1`@a.bc>\n`<b>`", errRawHTML},
		{"URI autolink with a backtick and no code span", "<https://e.example/`>", errRawHTML},
		{"URI autolink with a one-letter scheme", "<a:b>", errRawHTML},
		{"URI autolink with a 33-character scheme", "<" + strings.Repeat("a", 33) + ":x>", errRawHTML},
		{"URI autolink with a space", "<https://a b>", errRawHTML},
		{"URI autolink holding a less-than", "<ab:<script>", errRawHTML},
		{"URI autolink without greater-than", "<https://example.com/", errRawHTML},
		{"scheme running to the end of the line", "<https", errRawHTML},

		// Forms where another construct takes in a code span's backtick, so
		// CommonMark renders raw HTML.
		{"even backslashes before less-than", `\\<div>`, errRawHTML},
		{"escaped backtick", "\\`<details>`", errRawHTML},
		{"URI autolink holding a backtick", "<https://e.example/`><details>`", errRawHTML},
		{"link title holding a backtick", "[a](/u \"`\") <details>`", errRawHTML},
		{"image title holding a backtick", "![a](/u \"`\") <details>`", errRawHTML},
		{"link destination holding a backtick", "[a](</u`>) <details>`", errRawHTML},
		{"link title across lines", "[a](/u \"\n`\") <details>`", errRawHTML},
		{"image title across lines", "![a](/u \"\n`\") <details>`", errRawHTML},
		{"email autolink holding a backtick", "<1`@a.bc> <details>`", errRawHTML},
		{"email autolink starting with a backtick", "<`@a.bc> <script>`", errRawHTML},

		// Unclosed fences.
		{"unclosed fence with info string", "本文\n```go\nfmt.Println()", errUnclosedFence},
		{"unclosed tilde fence", "~~~\nx", errUnclosedFence},
		{"fence opened on the last line", "x\n```", errUnclosedFence},
		{"closing run shorter than the opening run", "````\nx\n```", errUnclosedFence},
		{"closing fence of the other character", "```\nx\n~~~", errUnclosedFence},
		{"closing fence indented by four spaces", "```\nx\n    ```", errUnclosedFence},
		{"closing fence indented by a tab", "```\nx\n\t```", errUnclosedFence},
		{"closing fence followed by text", "```\nx\n``` a", errUnclosedFence},
		{"HTML in an unclosed fence", "```\n<details>", errUnclosedFence},

		// Lines that look like a fence but do not open one at the top level.
		{"fence indented by one space", " ```\nx\n ```", errFenceLikeLine},
		{"fence indented by three spaces before an HTML block", "   ~~~\n~~~\n<div hidden>\n~~~", errFenceLikeLine},
		{"indented fence", "    ```\nx\n    ```", errFenceLikeLine},
		{"tab-indented tilde fence", "\t~~~\nx\n\t~~~", errFenceLikeLine},
		{"fence in a block quote", "> ```\n> x\n> ```", errFenceLikeLine},
		{"fence in a bullet list", "- ```\n  x\n  ```", errFenceLikeLine},
		{"fence in an ordered list", "1. ~~~\n   x\n   ~~~", errFenceLikeLine},
		{"backtick fence with a backtick in its info string", "```a`b\nx\n```", errFenceLikeLine},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := checkBodyMarkdown(tc.body)
			if tc.want == nil {
				if err != nil {
					t.Errorf("checkBodyMarkdown(%q) = %v, want nil", tc.body, err)
				}
				return
			}
			if !errors.Is(err, tc.want) {
				t.Errorf("checkBodyMarkdown(%q) = %v, want %v", tc.body, err, tc.want)
			}
		})
	}
}

// linearWorkSize is the size of the TestCheckBodyMarkdownLinearWork bodies:
// eight times what Write accepts, so that a judgment that rereads the line
// for every character does about linearWorkSize^2/2 steps and runs for hours
// instead of about a second.
const linearWorkSize = 8 * maxTextBytes

// linearWorkInputs are units repeated into a body, each built so that one way
// of making the judgment slower than linear rereads the line or the body.
var linearWorkInputs = []struct {
	name string
	unit string
}{
	// Each '<' has no '>' after it: a search for the next '>' from every
	// '<' reads the rest of the line.
	{"less-than without greater-than", "<"},
	// Each '<' lies inside a code span: deciding that by reading the line
	// from its start again for every '<' rereads everything before it.
	{"HTML in code spans", "`<b>` "},
	// Backslashes before each '<': counting them from the start of the line
	// rereads it.
	{"escaped less-than", `\<a`},
	// One code span per line: rechecking the body-wide code-span conditions
	// for every line rereads the body.
	{"code spans on many lines", "`<b>`\n"},
}

// TestCheckBodyMarkdownLinearWork checks that the judgment is linear without
// measuring time. It asserts only the result; an implementation slower than
// linear fails it by exceeding the go test timeout by orders of magnitude,
// so machine load cannot change the outcome. BenchmarkCheckBodyMarkdown
// measures the same inputs for inspection.
func TestCheckBodyMarkdownLinearWork(t *testing.T) {
	for _, in := range linearWorkInputs {
		t.Run(in.name, func(t *testing.T) {
			body := strings.Repeat(in.unit, linearWorkSize/len(in.unit))
			if err := checkBodyMarkdown(body); err != nil {
				t.Errorf("checkBodyMarkdown error = %v, want nil", err)
			}
		})
	}
}

// BenchmarkCheckBodyMarkdown reports the cost of the judgment at growing
// sizes; ns/op should grow in proportion to the size. It is not run by make
// test and decides no pass or fail.
func BenchmarkCheckBodyMarkdown(b *testing.B) {
	for _, in := range linearWorkInputs {
		for _, size := range []int{maxTextBytes / 16, maxTextBytes / 4, maxTextBytes} {
			body := strings.Repeat(in.unit, size/len(in.unit))
			b.Run(in.name+"/"+strconv.Itoa(size), func(b *testing.B) {
				for b.Loop() {
					_ = checkBodyMarkdown(body)
				}
			})
		}
	}
}
