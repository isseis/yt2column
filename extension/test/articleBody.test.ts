import assert from "node:assert/strict";
import { describe, it } from "node:test";

import { renderArticleBody } from "../src/ui/articleBody.ts";
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
