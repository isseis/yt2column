// Text that must be Japanese, full-width, or a bidi control is written with
// \u escapes so the source stays ASCII; each value says what it is. The
// expected trees and strings are written by hand from the rules, never
// computed by a second implementation of them.

import assert from "node:assert/strict";
import { describe, it } from "node:test";

import {
  parseArticleMarkdown,
  type ArticleBlock,
  type ArticleInline,
  type ArticleLine,
  type LinkUrl,
} from "../src/core/articleMarkdown.ts";
import { maxResponseBytes } from "../src/core/exchange.ts";
import { randomMarkdownInputs, randomSeeds } from "./helpers/randomMarkdown.ts";

const text = (value: string): ArticleInline => ({ kind: "text", text: value });
const strong = (value: string): ArticleInline => ({
  kind: "strong",
  text: value,
});
const emphasis = (value: string): ArticleInline => ({
  kind: "emphasis",
  text: value,
});
const code = (value: string): ArticleInline => ({ kind: "code", text: value });
const link = (url: string): ArticleInline => ({
  kind: "link",
  url: url as LinkUrl,
});
const paragraph = (...lines: ArticleLine[]): ArticleBlock => ({
  kind: "paragraph",
  lines,
});
const title = (...line: ArticleLine): ArticleBlock => ({
  kind: "heading",
  level: "title",
  line,
});
const section = (...line: ArticleLine): ArticleBlock => ({
  kind: "heading",
  level: "section",
  line,
});
const bullets = (...items: ArticleLine[][]): ArticleBlock => ({
  kind: "bulletList",
  items,
});
const ordered = (start: number, ...items: ArticleLine[][]): ArticleBlock => ({
  kind: "orderedList",
  start,
  items,
});
const codeBlock = (...lines: string[]): ArticleBlock => ({
  kind: "codeBlock",
  lines,
});

/** The string a line shows: each piece's text, and a link's URL. */
function lineString(line: ArticleLine): string {
  return line
    .map((piece) => (piece.kind === "link" ? piece.url : piece.text))
    .join("");
}

/**
 * The article tree's string: the lines of a paragraph, an item or a code
 * block joined by one LF, the blocks and the items joined by nothing.
 */
function treeString(blocks: readonly ArticleBlock[]): string {
  return blocks
    .map((block) => {
      switch (block.kind) {
        case "heading":
          return lineString(block.line);
        case "paragraph":
          return block.lines.map(lineString).join("\n");
        case "bulletList":
        case "orderedList":
          return block.items
            .map((item) => item.map(lineString).join("\n"))
            .join("");
        case "codeBlock":
          return block.lines.join("\n");
        default: {
          const exhaustive: never = block;
          throw new Error(`unexpected block ${JSON.stringify(exhaustive)}`);
        }
      }
    })
    .join("");
}

describe("treeString", () => {
  it("joins lines by LF and blocks and items by nothing", () => {
    const blocks: ArticleBlock[] = [
      title(text("T")),
      paragraph([text("a")], [strong("b"), text("c")]),
      bullets([[text("x")], [text("y")]], [[link("https://a.example/")]]),
      codeBlock("p", "q"),
    ];
    assert.equal(treeString(blocks), "Ta\nbcx\nyhttps://a.example/p\nq");
  });
});

interface InlineCase {
  readonly name: string;
  readonly input: string;
  readonly pieces: ArticleLine;
  /** The string the line shows, written by hand from the input. */
  readonly shown: string;
}

/** Cases of one line that no block rule takes, so it is a paragraph of one line. */
const inlineCases: readonly InlineCase[] = [
  {
    name: "a speaker's name in strong emphasis",
    // Japanese "speaker", then a full-width colon.
    input: "**\u8a71\u8005**\uff1a",
    pieces: [strong("\u8a71\u8005"), text("\uff1a")],
    shown: "\u8a71\u8005\uff1a",
  },
  {
    name: "strong emphasis opened before an ideographic space",
    input: "**\u3000\u8a71\u8005**",
    pieces: [strong("\u3000\u8a71\u8005")],
    shown: "\u3000\u8a71\u8005",
  },
  {
    name: "emphasis",
    // Japanese "word".
    input: "*\u8a9e*",
    pieces: [emphasis("\u8a9e")],
    shown: "\u8a9e",
  },
  {
    name: "strong emphasis with no closing run",
    input: "x **a",
    pieces: [text("x **a")],
    shown: "x **a",
  },
  {
    name: "emphasis with no closing star",
    input: "x *a",
    pieces: [text("x *a")],
    shown: "x *a",
  },
  {
    name: "strong emphasis whose opening run is followed by a space",
    input: "** a**",
    pieces: [text("** a**")],
    shown: "** a**",
  },
  {
    name: "strong emphasis whose closing run follows a space",
    input: "**a **",
    pieces: [text("**a **")],
    shown: "**a **",
  },
  {
    name: "emphasis whose opening star is followed by a tab",
    input: "*\ta*",
    pieces: [text("*\ta*")],
    shown: "*\ta*",
  },
  {
    name: "emphasis whose closing star follows a tab",
    input: "*a\t*",
    pieces: [text("*a\t*")],
    shown: "*a\t*",
  },
  {
    name: "runs of three stars",
    input: "***\u8a9e***",
    pieces: [text("***\u8a9e***")],
    shown: "***\u8a9e***",
  },
  {
    name: "a run of four stars",
    input: "****",
    pieces: [text("****")],
    shown: "****",
  },
  {
    name: "underscores",
    input: "_\u8a9e_",
    pieces: [text("_\u8a9e_")],
    shown: "_\u8a9e_",
  },
  {
    name: "a closing run of three stars that does not close strong emphasis",
    input: "**a***",
    pieces: [text("**a***")],
    shown: "**a***",
  },
  {
    name: "emphasis that skips a closing star after a space",
    input: "*a *b* c",
    pieces: [emphasis("a *b"), text(" c")],
    shown: "a *b c",
  },
  {
    name: "a code span",
    input: "`x`",
    pieces: [code("x")],
    shown: "x",
  },
  {
    name: "a code span holding marks",
    input: "x `**a** <https://a.example/> \\*`",
    pieces: [text("x "), code("**a** <https://a.example/> \\*")],
    shown: "x **a** <https://a.example/> \\*",
  },
  {
    name: "a run of backticks with no closing run",
    input: "x ``a",
    pieces: [text("x ``a")],
    shown: "x ``a",
  },
  {
    name: "a code span closed by a run of the same length only",
    input: "x `a``b` c",
    pieces: [text("x "), code("a``b"), text(" c")],
    shown: "x a``b c",
  },
  {
    name: "a code span of two backticks holding one",
    input: "``a`b``",
    pieces: [code("a`b")],
    shown: "a`b",
  },
  {
    name: "a code span closed by an escaped backtick, as no escape applies inside",
    input: "`a\\`b",
    pieces: [code("a\\"), text("b")],
    shown: "a\\b",
  },
  {
    name: "an escaped star inside strong emphasis",
    input: "**a \\** b**",
    pieces: [strong("a ** b")],
    shown: "a ** b",
  },
  {
    name: "an escaped star before a closing run",
    input: "**a\\***",
    pieces: [strong("a*")],
    shown: "a*",
  },
  {
    name: "an escaped star before strong emphasis marks",
    input: "\\**a**",
    pieces: [text("**a**")],
    shown: "**a**",
  },
  {
    name: "escaped marks",
    input: "\\*a* \\# \\<https://a.example/> \\`x`",
    pieces: [text("*a* # <https://a.example/> `x`")],
    shown: "*a* # <https://a.example/> `x`",
  },
  {
    name: "an escaped backslash",
    input: "a\\\\*b*",
    pieces: [text("a\\"), emphasis("b")],
    shown: "a\\b",
  },
  {
    name: "a backslash before a character that is not ASCII punctuation",
    // Hiragana "a".
    input: "\\\u3042 \\a",
    pieces: [text("\\\u3042 \\a")],
    shown: "\\\u3042 \\a",
  },
  {
    name: "a backslash at the end of the line",
    input: "a\\",
    pieces: [text("a\\")],
    shown: "a\\",
  },
  {
    name: "a run of stars inside emphasis",
    input: "*a**b*",
    pieces: [emphasis("a**b")],
    shown: "a**b",
  },
  {
    name: "a code span and an autolink inside strong emphasis",
    input: "**a `b` <https://a.example/>**",
    pieces: [strong("a `b` <https://a.example/>")],
    shown: "a `b` <https://a.example/>",
  },
  {
    name: "an escape inside emphasis",
    input: "*a\\*b*",
    pieces: [emphasis("a*b")],
    shown: "a*b",
  },
  {
    name: "an autolink inside strong emphasis",
    input: "**<https://a.example/>**",
    pieces: [strong("<https://a.example/>")],
    shown: "<https://a.example/>",
  },
  {
    name: "strong emphasis closed before the end of an autolink",
    input: "**a <https://a.example/**>",
    pieces: [strong("a <https://a.example/"), text(">")],
    shown: "a <https://a.example/>",
  },
  {
    name: "a backtick inside emphasis",
    input: "*a `b* c`",
    pieces: [emphasis("a `b"), text(" c`")],
    shown: "a `b c`",
  },
  {
    name: "an autolink before strong emphasis marks it holds",
    input: "<https://a.example/**>**",
    pieces: [link("https://a.example/**"), text("**")],
    shown: "https://a.example/****",
  },
  {
    name: "the source line",
    // Japanese "source", then the source URL.
    input: "\u51fa\u5178: <https://www.youtube.com/watch?v=abcdefghijk>",
    pieces: [
      text("\u51fa\u5178: "),
      link("https://www.youtube.com/watch?v=abcdefghijk"),
    ],
    shown: "\u51fa\u5178: https://www.youtube.com/watch?v=abcdefghijk",
  },
  {
    name: "an autolink to a Punycode host",
    input: "<https://xn--nxasmq6b.com/>",
    pieces: [link("https://xn--nxasmq6b.com/")],
    shown: "https://xn--nxasmq6b.com/",
  },
  {
    name: "autolinks to local addresses",
    input: "<https://localhost/> <https://127.0.0.1:8443/>",
    pieces: [
      link("https://localhost/"),
      text(" "),
      link("https://127.0.0.1:8443/"),
    ],
    shown: "https://localhost/ https://127.0.0.1:8443/",
  },
  {
    name: "an autolink after a failed one",
    input: "<https://a.example<https://b.example/>",
    pieces: [text("<https://a.example"), link("https://b.example/")],
    shown: "<https://a.examplehttps://b.example/",
  },
];

/** Autolink forms that do not become links; each line shows unchanged. */
const notLinks: readonly { name: string; input: string }[] = [
  { name: "an http URL", input: "<http://example.com>" },
  { name: "an upper-case scheme", input: "<HTTPS://example.com/>" },
  { name: "a javascript URL", input: "<javascript:alert(1)>" },
  { name: "a URL with no host", input: "<https://>" },
  {
    name: "a URL whose serialization adds a slash",
    input: "<https://example.com>",
  },
  {
    name: "a user name and a password",
    input: "<https://user:pass@example.com/>",
  },
  { name: "a user name only", input: "<https://user@a.example/>" },
  { name: "a password only", input: "<https://:pass@a.example/>" },
  { name: "a backslash", input: "<https://example.com\\path>" },
  {
    name: "a full-width letter in the host",
    input: "<https://\uff45xample.com/>",
  },
  { name: "a space", input: "<https://example.com/a b>" },
  { name: "a less-than sign", input: "<https://example.com/<x>" },
  { name: "a control character", input: "<https://a.example/\u0007>" },
  { name: "the default port", input: "<https://a.example:443/>" },
  { name: "no closing sign", input: "<https://a.example/" },
  { name: "an inline link", input: "[x](https://example.com/)" },
  { name: "a reference link", input: "[x][ref]" },
  { name: "an image", input: "![x](https://example.com/a.png)" },
];

describe("parseArticleMarkdown inline", () => {
  for (const { name, input, pieces, shown } of inlineCases) {
    it(`parses ${name}`, () => {
      const blocks = parseArticleMarkdown(input);
      assert.deepEqual(blocks, [paragraph(pieces)]);
      assert.equal(treeString(blocks), shown);
    });
  }

  for (const { name, input } of notLinks) {
    it(`keeps ${name} as text: ${JSON.stringify(input)}`, () => {
      assert.deepEqual(parseArticleMarkdown(input), [paragraph([text(input)])]);
    });
  }
});

interface BlockCase {
  readonly name: string;
  readonly input: string;
  readonly blocks: readonly ArticleBlock[];
  /** The article tree's string, written by hand from the input. */
  readonly shown: string;
}

const blockCases: readonly BlockCase[] = [
  {
    name: "the title and section headings",
    input: "# Title **x**\n## Section",
    blocks: [title(text("Title "), strong("x")), section(text("Section"))],
    shown: "Title xSection",
  },
  {
    name: "headings marked with a tab and followed by more blanks",
    input: "#\tA\n##  \tB ",
    blocks: [title(text("A")), section(text("B "))],
    shown: "AB ",
  },
  {
    name: "lines that are not headings",
    input: "### x\n#x\n#\n# \n##\t \n #a",
    blocks: [
      paragraph(
        [text("### x")],
        [text("#x")],
        [text("#")],
        [text("# ")],
        [text("##\t ")],
        [text(" #a")],
      ),
    ],
    shown: "### x\n#x\n#\n# \n##\t \n #a",
  },
  {
    name: "paragraphs with their line breaks and spaces",
    input: "a\n  b  c\n \t\nd",
    blocks: [paragraph([text("a")], [text("  b  c")]), paragraph([text("d")])],
    shown: "a\n  b  cd",
  },
  {
    name: "a heading that ends a paragraph",
    input: "a\n# T\nb",
    blocks: [paragraph([text("a")]), title(text("T")), paragraph([text("b")])],
    shown: "aTb",
  },
  {
    name: "a carriage return kept as a character",
    input: "# T\r\na\r\n\r\nb",
    blocks: [
      title(text("T\r")),
      paragraph([text("a\r")], [text("\r")], [text("b")]),
    ],
    shown: "T\ra\r\n\r\nb",
  },
  {
    name: "bullet lists split where the marker changes",
    input: "- a\n- b\n\n- c\n* d\n+ e",
    blocks: [
      bullets([[text("a")]], [[text("b")]], [[text("c")]]),
      bullets([[text("d")]]),
      bullets([[text("e")]]),
    ],
    shown: "abcde",
  },
  {
    name: "items whose text keeps its spaces, and empty items",
    input: "-  a\n- \n- - -\n* * *",
    blocks: [
      bullets([[text(" a")]], [[]], [[text("- -")]]),
      bullets([[text("* *")]]),
    ],
    shown: " a- -* *",
  },
  {
    name: "lines that are not items",
    input: "-a\n1.a\n1\\. a\n\\- a",
    blocks: [
      paragraph([text("-a")], [text("1.a")], [text("1. a")], [text("- a")]),
    ],
    shown: "-a\n1.a\n1. a\n- a",
  },
  {
    name: "an ordered list numbered from its first item",
    input: "3. a\n3. b",
    blocks: [ordered(3, [[text("a")]], [[text("b")]])],
    shown: "ab",
  },
  {
    name: "ordered lists split where the delimiter changes",
    input: "1. a\n\n2) b\n3) c",
    blocks: [
      ordered(1, [[text("a")]]),
      ordered(2, [[text("b")]], [[text("c")]]),
    ],
    shown: "abc",
  },
  {
    name: "a start number with leading zeros",
    input: "007. a",
    blocks: [ordered(7, [[text("a")]])],
    shown: "a",
  },
  {
    name: "a start number of nine digits",
    input: "999999999) a",
    blocks: [ordered(999999999, [[text("a")]])],
    shown: "a",
  },
  {
    name: "a line of ten digits",
    input: "1234567890. a",
    blocks: [paragraph([text("1234567890. a")])],
    shown: "1234567890. a",
  },
  {
    name: "a bullet list and an ordered list next to each other",
    input: "- a\n1. b\n- c",
    blocks: [
      bullets([[text("a")]]),
      ordered(1, [[text("b")]]),
      bullets([[text("c")]]),
    ],
    shown: "abc",
  },
  {
    name: "continuation lines of an item, a nested list among them",
    input: "- a\nb **c**\n  - d\n- e",
    blocks: [
      bullets(
        [[text("a")], [text("b "), strong("c")], [text("  - d")]],
        [[text("e")]],
      ),
    ],
    shown: "a\nb c\n  - de",
  },
  {
    name: "a line after a blank line that is not a continuation",
    input: "1. a\n\nb\n- c",
    blocks: [
      ordered(1, [[text("a")]]),
      paragraph([text("b")]),
      bullets([[text("c")]]),
    ],
    shown: "abc",
  },
  {
    name: "a paragraph ended by an item",
    input: "a\n- b",
    blocks: [paragraph([text("a")]), bullets([[text("b")]])],
    shown: "ab",
  },
  {
    name: "a code block with its marks, spaces and info string",
    input: "```go\n**a** # <https://a.example/> \\*\n\n  x\n```\nb",
    blocks: [
      codeBlock("**a** # <https://a.example/> \\*", "", "  x"),
      paragraph([text("b")]),
    ],
    shown: "**a** # <https://a.example/> \\*\n\n  xb",
  },
  {
    name: "a code block closed by a line of three spaces and a longer run",
    input: "```\na\n   `````  \t",
    blocks: [codeBlock("a")],
    shown: "a",
  },
  {
    name: "a fence not closed by a line of four spaces",
    input: "```\na\n    ```",
    blocks: [paragraph([text("```")], [text("a")], [text("    ```")])],
    shown: "```\na\n    ```",
  },
  {
    name: "a fence not closed by a shorter run",
    input: "````\na\n```",
    blocks: [paragraph([text("````")], [text("a")], [text("```")])],
    shown: "````\na\n```",
  },
  {
    name: "a fence not closed by the other mark",
    input: "```\na\n~~~",
    blocks: [paragraph([text("```")], [text("a")], [text("~~~")])],
    shown: "```\na\n~~~",
  },
  {
    name: "a fence not closed by a run with text after it",
    input: "~~~\na\n~~~x",
    blocks: [paragraph([text("~~~")], [text("a")], [text("~~~x")])],
    shown: "~~~\na\n~~~x",
  },
  {
    name: "a tilde fence holding a backtick fence",
    input: "~~~ x\n```\n~~~",
    blocks: [codeBlock("```")],
    shown: "```",
  },
  {
    name: "an empty code block",
    input: "```\n```",
    blocks: [codeBlock()],
    shown: "",
  },
  {
    name: "a backtick fence whose info string has a backtick",
    input: "```a`b\nx\n```",
    blocks: [paragraph([text("```a`b")], [text("x")], [text("```")])],
    shown: "```a`b\nx\n```",
  },
  {
    name: "an unclosed fence followed by a heading",
    input: "```js\n# T",
    blocks: [paragraph([text("```js")]), title(text("T"))],
    shown: "```jsT",
  },
  {
    name: "a later fence that closes after an unclosed one",
    input: "~~~\n```\na\n```",
    blocks: [paragraph([text("~~~")]), codeBlock("a")],
    shown: "~~~a",
  },
  {
    name: "a code block that ends a list",
    input: "- a\n```\nx\n```",
    blocks: [bullets([[text("a")]]), codeBlock("x")],
    shown: "ax",
  },
  {
    name: "an unclosed fence as the continuation of an item",
    input: "- a\n```",
    blocks: [bullets([[text("a")], [text("```")]])],
    shown: "a\n```",
  },
  {
    name: "HTML tags kept as text",
    input:
      '<img src=x onerror=alert(1)>\n<a href="javascript:alert(1)">x</a>\n<script>alert(1)</script>\n<!-- x -->\n<div>',
    blocks: [
      paragraph(
        [text("<img src=x onerror=alert(1)>")],
        [text('<a href="javascript:alert(1)">x</a>')],
        [text("<script>alert(1)</script>")],
        [text("<!-- x -->")],
        [text("<div>")],
      ),
    ],
    shown:
      '<img src=x onerror=alert(1)>\n<a href="javascript:alert(1)">x</a>\n<script>alert(1)</script>\n<!-- x -->\n<div>',
  },
  {
    name: "syntax the rules do not draw",
    // Japanese "quote" and "heading".
    input:
      "> \u5f15\u7528\n| a | b |\n---\n***\n___\n### \u898b\u51fa\u3057\n    code\n~~x~~\n&amp; &#60;",
    blocks: [
      paragraph(
        [text("> \u5f15\u7528")],
        [text("| a | b |")],
        [text("---")],
        [text("***")],
        [text("___")],
        [text("### \u898b\u51fa\u3057")],
        [text("    code")],
        [text("~~x~~")],
        [text("&amp; &#60;")],
      ),
    ],
    shown:
      "> \u5f15\u7528\n| a | b |\n---\n***\n___\n### \u898b\u51fa\u3057\n    code\n~~x~~\n&amp; &#60;",
  },
  {
    name: "a Setext heading underline",
    input: "a\n===\nb\n---",
    blocks: [paragraph([text("a")], [text("===")], [text("b")], [text("---")])],
    shown: "a\n===\nb\n---",
  },
];

describe("parseArticleMarkdown blocks", () => {
  for (const { name, input, blocks, shown } of blockCases) {
    it(`parses ${name}`, () => {
      const parsed = parseArticleMarkdown(input);
      assert.deepEqual(parsed, blocks);
      assert.equal(treeString(parsed), shown);
    });
  }

  it("parses an empty string to no blocks", () => {
    assert.deepEqual(parseArticleMarkdown(""), []);
  });
});

describe("parseArticleMarkdown display Markdown", () => {
  // The shape the server writes: the title heading, the channel name line
  // with its leading escape, the body, and the source line with its LF.
  const cases: readonly BlockCase[] = [
    {
      name: "with a channel name line that escapes a list marker",
      input:
        "# Title\n\n\\- Channel <https://example.com/x>\n\nBody **bold**.\n\n## Section\n\n- point\n\n\u51fa\u5178: <https://www.youtube.com/watch?v=abcdefghijk>\n",
      blocks: [
        title(text("Title")),
        paragraph([text("- Channel "), link("https://example.com/x")]),
        paragraph([text("Body "), strong("bold"), text(".")]),
        section(text("Section")),
        bullets([[text("point")]]),
        paragraph([
          text("\u51fa\u5178: "),
          link("https://www.youtube.com/watch?v=abcdefghijk"),
        ]),
      ],
      shown:
        "Title- Channel https://example.com/xBody bold.Sectionpoint\u51fa\u5178: https://www.youtube.com/watch?v=abcdefghijk",
    },
    {
      name: "with a channel name line that escapes an ordered list marker",
      input: "# T\n\n1\\. Channel\n\nBody\n",
      blocks: [
        title(text("T")),
        paragraph([text("1. Channel")]),
        paragraph([text("Body")]),
      ],
      shown: "T1. ChannelBody",
    },
    {
      name: "with a channel name line that escapes a heading mark",
      input: "# T\n\n\\# Channel\n\nBody\n",
      blocks: [
        title(text("T")),
        paragraph([text("# Channel")]),
        paragraph([text("Body")]),
      ],
      shown: "T# ChannelBody",
    },
  ];
  for (const { name, input, blocks, shown } of cases) {
    it(`parses the display Markdown ${name}`, () => {
      const parsed = parseArticleMarkdown(input);
      assert.deepEqual(parsed, blocks);
      assert.equal(treeString(parsed), shown);
    });
  }
});

/** Repeats unit until the string is length characters long. */
function repeatTo(unit: string, length: number): string {
  return unit.repeat(Math.ceil(length / unit.length)).slice(0, length);
}

/** One line of backtick runs that grow by one: `a``a```a and so on. */
function growingBacktickRuns(length: number): string {
  let line = "";
  for (let run = 1; line.length < length; run++) {
    line += `${"`".repeat(run)}a`;
  }
  return line.slice(0, length);
}

describe("parseArticleMarkdown performance", () => {
  // Each input is as long as the largest response the extension accepts and
  // is built outside the timed call. A parse whose time grows with the
  // square of the input takes far longer than the limit on these; one that
  // grows with the input takes tens of milliseconds.
  const limitMs = 1000;
  const size = maxResponseBytes;
  const articleParagraph =
    "# Title\n\n## Section\n\n**Speaker**: a *word* and `code` <https://a.example/x>.\n- item\n1. item\n\n```\ncode\n```\n\n";
  const inputs: readonly { name: string; build: () => string }[] = [
    { name: "a line of stars", build: () => "*".repeat(size) },
    { name: "a line of backticks", build: () => "`".repeat(size) },
    {
      name: "a line of autolink openings",
      build: () => repeatTo("<https://", size),
    },
    { name: "a line of backslashes", build: () => "\\".repeat(size) },
    {
      name: "a line of unclosed strong emphasis",
      build: () => repeatTo("**a", size),
    },
    { name: "a line of unclosed emphasis", build: () => repeatTo("*a", size) },
    {
      name: "a line of strong emphasis closings after spaces",
      build: () => repeatTo("**a ", size),
    },
    {
      name: "a line of emphasis closings after spaces",
      build: () => repeatTo("*a ", size),
    },
    {
      name: "a line of growing backtick runs",
      build: () => growingBacktickRuns(size),
    },
    {
      name: "lines of unclosed fences",
      // The last line is padded with letters, so no line closes a fence.
      build: () => {
        const lines = "```x\n".repeat(Math.floor((size - 1) / 5));
        return lines + "a".repeat(size - lines.length);
      },
    },
    {
      name: "a line mixing the marks",
      build: () => repeatTo("**a *b `c <https://d \\", size),
    },
    {
      name: "lines of article paragraphs",
      build: () => repeatTo(articleParagraph, size),
    },
  ];
  for (const { name, build } of inputs) {
    // The time is reported, so the margin below the limit can be read in CI.
    it(`parses ${name} within ${limitMs} ms`, { timeout: 10_000 }, (t) => {
      const input = build();
      assert.equal(input.length, size);
      const start = performance.now();
      parseArticleMarkdown(input);
      const elapsed = performance.now() - start;
      t.diagnostic(`parse time: ${elapsed.toFixed(1)} ms`);
      assert.ok(elapsed < limitMs, `${name}: ${elapsed.toFixed(0)} ms`);
    });
  }
});

/** Every link URL in the article tree. */
function linkUrls(blocks: readonly ArticleBlock[]): string[] {
  const lines: ArticleLine[] = [];
  for (const block of blocks) {
    switch (block.kind) {
      case "heading":
        lines.push(block.line);
        break;
      case "paragraph":
        lines.push(...block.lines);
        break;
      case "bulletList":
      case "orderedList":
        for (const item of block.items) {
          lines.push(...item);
        }
        break;
      case "codeBlock":
        break;
      default: {
        const exhaustive: never = block;
        throw new Error(`unexpected block ${JSON.stringify(exhaustive)}`);
      }
    }
  }
  return lines.flatMap((line) =>
    line.flatMap((piece) => (piece.kind === "link" ? [piece.url] : [])),
  );
}

describe("parseArticleMarkdown random input", () => {
  for (const seed of randomSeeds) {
    it(`parses every input of seed ${seed} without throwing, to links of printable ASCII`, () => {
      let links = 0;
      randomMarkdownInputs(seed).forEach((input, index) => {
        const where = `seed ${seed}, input ${index}: ${JSON.stringify(input)}`;
        let blocks: readonly ArticleBlock[];
        try {
          blocks = parseArticleMarkdown(input);
        } catch (error) {
          assert.fail(`${where} threw ${String(error)}`);
        }
        for (const url of linkUrls(blocks)) {
          assert.match(url, /^[\x21-\x7e]+$/, where);
          links++;
        }
      });
      assert.ok(links > 0, `seed ${seed} made no link`);
    });
  }
});
