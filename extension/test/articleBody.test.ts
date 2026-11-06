// Text that must be Japanese is written with \u escapes so the source stays
// ASCII; each value says what it is. The expected strings are written by
// hand from the rules.

import assert from "node:assert/strict";
import { describe, it } from "node:test";

import type {
  ArticleBlock,
  ArticleInline,
  ArticleLine,
  LinkUrl,
} from "../src/core/articleMarkdown.ts";
import {
  buildArticleBody,
  drawArticleBody,
  renderArticleBody,
} from "../src/ui/articleBody.ts";
import { assertArticleDom } from "./helpers/articleDom.ts";
import { container } from "./helpers/dom.ts";

describe("renderArticleBody", () => {
  const doc = container().ownerDocument;

  // Article text an LLM may return: HTML that would run script or navigate,
  // and Markdown links that a renderer would turn into elements.
  const articles = [
    "<img src=x onerror=alert(1)>",
    '<a href="javascript:alert(1)">a link</a>',
    "<script>alert(1)</script>",
    "[a link](javascript:alert(1))",
    "[a link](https://example.com) and ![an image](https://example.com/x.png)",
    "# A heading\n\n<b>bold</b> and **bold**\n\n  indented line\n",
  ];

  for (const markdown of articles) {
    it(`shows ${JSON.stringify(markdown)} as text without creating an element or attribute`, () => {
      const body = renderArticleBody(doc, markdown);
      assert.equal(body.childElementCount, 0);
      assert.equal(body.textContent, markdown);
      // The element's own attributes are the renderer's, never the article's.
      assert.deepEqual(
        Array.from(body.attributes, (attribute) => attribute.name).sort(),
        ["class", "dir"],
      );
    });
  }

  it("takes its direction from the text and the article body's CSS class", () => {
    const body = renderArticleBody(doc, "text");
    assert.equal(body.tagName.toLowerCase(), "div");
    assert.equal(body.getAttribute("dir"), "auto");
    assert.equal(body.className, "article-body");
  });
});

const text = (value: string): ArticleInline => ({ kind: "text", text: value });
const link = (url: string): ArticleInline => ({
  kind: "link",
  url: url as LinkUrl,
});
const paragraph = (...lines: ArticleLine[]): ArticleBlock => ({
  kind: "paragraph",
  lines,
});

/** The lower-case names of element's children, text nodes as "#text". */
function childNames(element: Element): string[] {
  return Array.from(element.childNodes, (node) => node.nodeName.toLowerCase());
}

describe("buildArticleBody", () => {
  const doc = container().ownerDocument;

  it("makes a div of the article body class, left to right, in Japanese", () => {
    const body = buildArticleBody(doc, []);
    assert.equal(body.tagName.toLowerCase(), "div");
    assert.equal(body.getAttribute("class"), "article-body");
    assert.equal(body.getAttribute("dir"), "ltr");
    assert.equal(body.getAttribute("lang"), "ja");
    assertArticleDom(body, "empty");
  });

  it("draws the title heading as h2 and a section heading as h3", () => {
    const body = buildArticleBody(doc, [
      { kind: "heading", level: "title", line: [text("T")] },
      { kind: "heading", level: "section", line: [text("S")] },
    ]);
    assert.deepEqual(childNames(body), ["h2", "h3"]);
    assert.equal(body.children[0]?.textContent, "T");
    assert.equal(body.children[1]?.textContent, "S");
  });

  it("draws each inline piece as its element and a paragraph's lines with an LF between them", () => {
    const body = buildArticleBody(doc, [
      paragraph(
        [
          text("a"),
          { kind: "strong", text: "b" },
          { kind: "emphasis", text: "c" },
        ],
        [{ kind: "code", text: "**d**" }],
      ),
    ]);
    const p = body.children[0];
    assert.ok(p !== undefined);
    assert.equal(p.tagName.toLowerCase(), "p");
    assert.deepEqual(childNames(p), ["#text", "strong", "em", "#text", "code"]);
    assert.equal(p.textContent, "abc\n**d**");
  });

  it("draws a bullet list and an ordered list from its start, with nothing between the items", () => {
    const body = buildArticleBody(doc, [
      {
        kind: "bulletList",
        items: [[[text("a")]], [[text("b")], [text("c")]]],
      },
      {
        kind: "orderedList",
        start: 999_999_999,
        items: [[[text("x")]], [[text("y")]]],
      },
    ]);
    assert.deepEqual(childNames(body), ["ul", "ol"]);
    const [ul, ol] = Array.from(body.children);
    assert.ok(ul !== undefined && ol !== undefined);
    assert.deepEqual(childNames(ul), ["li", "li"]);
    assert.deepEqual(childNames(ol), ["li", "li"]);
    assert.equal(ul.children[1]?.textContent, "b\nc");
    assert.equal(ol.getAttribute("start"), "999999999");
    assertArticleDom(body, "lists");
  });

  it("draws a code block as one code element in a pre, its lines joined by LF", () => {
    const body = buildArticleBody(doc, [
      { kind: "codeBlock", lines: ["x", "  **y**"] },
    ]);
    const pre = body.children[0];
    assert.ok(pre !== undefined);
    assert.equal(pre.tagName.toLowerCase(), "pre");
    assert.deepEqual(childNames(pre), ["code"]);
    assert.deepEqual(childNames(pre.children[0] as Element), ["#text"]);
    assert.equal(pre.textContent, "x\n  **y**");
  });

  it("draws a link that shows and opens its URL in a new tab, with no referrer or opener, left to right", () => {
    const url = "https://www.youtube.com/watch?v=abcdefghijk";
    const body = buildArticleBody(doc, [paragraph([link(url)])]);
    const a = body.querySelector("a");
    assert.ok(a !== null);
    assert.equal(a.getAttribute("href"), url);
    assert.equal(a.getAttribute("target"), "_blank");
    assert.equal(a.getAttribute("rel"), "noopener noreferrer");
    assert.equal(a.getAttribute("dir"), "ltr");
    assert.equal(a.textContent, url);
    assertArticleDom(body, "link");
  });

  it("builds trees the parse never makes: an empty list, an item and a paragraph with no line", () => {
    const body = buildArticleBody(doc, [
      { kind: "bulletList", items: [] },
      { kind: "orderedList", start: 0, items: [[]] },
      paragraph(),
    ]);
    assert.deepEqual(childNames(body), ["ul", "ol", "p"]);
    assert.equal(body.textContent, "");
    assertArticleDom(body, "empty pieces");
  });

  it("puts nothing between blocks and items, so its text is the tree's lines alone", () => {
    const body = buildArticleBody(doc, [
      { kind: "heading", level: "title", line: [text("T")] },
      paragraph([text("a")], [text("b")]),
      { kind: "bulletList", items: [[[text("x")]], [[text("y")]]] },
      { kind: "codeBlock", lines: ["c"] },
    ]);
    assert.equal(body.textContent, "Ta\nbxyc");
  });
});

describe("drawArticleBody", () => {
  /** The article element drawArticleBody draws for markdown. */
  function drawnBody(markdown: string): Element {
    const target = container();
    drawArticleBody(target, markdown);
    assert.equal(target.children.length, 1);
    const body = target.children[0];
    assert.ok(body !== undefined);
    return body;
  }

  it("shows the display Markdown without its marks", () => {
    // Japanese "source" before the source line.
    const markdown =
      "# Title **x**\n\n\\- Channel\n\nBody *a*\n`c` line\n\n## Section\n\n- one\n- two\n  more\n\n3. three\n\n```go\ncode  **x**\n```\n\n出典: <https://www.youtube.com/watch?v=abcdefghijk>\n";
    const body = drawnBody(markdown);
    assert.equal(
      body.textContent,
      "Title x- ChannelBody a\nc lineSectiononetwo\n  morethreecode  **x**出典: https://www.youtube.com/watch?v=abcdefghijk",
    );
    assertArticleDom(body, "display Markdown");
  });

  it("draws the source line's autolink as a link to its URL", () => {
    const url = "https://www.youtube.com/watch?v=abcdefghijk";
    // Japanese "source".
    const body = drawnBody(`出典: <${url}>`);
    const links = body.querySelectorAll("a");
    assert.equal(links.length, 1);
    assert.equal(links[0]?.getAttribute("href"), url);
    assertArticleDom(body, "source line");
  });

  // HTML and Markdown the rules do not take: each is drawn as the text it is,
  // in the blocks listed, and no tag becomes an element or an attribute.
  const asText: readonly {
    input: string;
    elements: readonly string[];
    shown: string;
  }[] = [
    { input: "<img src=x onerror=alert(1)>", elements: ["p"], shown: "" },
    {
      input: '<a href="javascript:alert(1)">x</a>',
      elements: ["p"],
      shown: "",
    },
    { input: "<script>alert(1)</script>", elements: ["p"], shown: "" },
    { input: "<!-- x -->", elements: ["p"], shown: "" },
    { input: "<div>", elements: ["p"], shown: "" },
    { input: "> quote", elements: ["p"], shown: "" },
    { input: "| a | b |\n| - | - |", elements: ["p"], shown: "" },
    { input: "---", elements: ["p"], shown: "" },
    { input: "***", elements: ["p"], shown: "" },
    { input: "___", elements: ["p"], shown: "" },
    { input: "- - -", elements: ["ul", "li"], shown: "- -" },
    { input: "* * *", elements: ["ul", "li"], shown: "* *" },
    { input: "### heading", elements: ["p"], shown: "" },
    { input: "    four spaces", elements: ["p"], shown: "" },
    { input: "Heading\n===", elements: ["p"], shown: "" },
    { input: "Heading\n---", elements: ["p"], shown: "" },
    { input: "~~strike~~", elements: ["p"], shown: "" },
    { input: "&amp; &#60;", elements: ["p"], shown: "" },
  ];
  for (const { input, elements, shown } of asText) {
    it(`draws ${JSON.stringify(input)} as text`, () => {
      const body = drawnBody(input);
      assertArticleDom(body, input);
      assert.deepEqual(
        Array.from(body.querySelectorAll("*"), (element) =>
          element.tagName.toLowerCase(),
        ),
        elements,
      );
      assert.equal(body.textContent, shown === "" ? input : shown);
    });
  }

  const notLinks = [
    "<http://example.com>",
    "<HTTPS://example.com/>",
    "<javascript:alert(1)>",
    "<https://>",
    "<https://example.com>",
    "<https://user:pass@example.com/>",
    "<https://example.com\\path>",
    // A full-width small letter e in the host.
    "<https://ｅxample.com/>",
    "<https://example.com/a b>",
    "<https://example.com/<x>",
    "[x](https://example.com/)",
    "[x][ref]",
    "![x](https://example.com/a.png)",
  ];
  for (const input of notLinks) {
    it(`makes no link of ${JSON.stringify(input)}`, () => {
      const body = drawnBody(input);
      assert.equal(body.querySelector("a"), null);
      assert.equal(body.textContent, input);
    });
  }
});
