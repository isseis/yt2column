// The one place the article's body is drawn. The display Markdown is parsed
// into the article tree (src/core/articleMarkdown.ts), and the tree alone is
// built into elements: the builder never receives the Markdown string and
// chooses each element by the tree's kind, so the elements and attributes it
// makes are a fixed list. Text goes in as text nodes and nothing is parsed as
// HTML. The rest of the viewer -- the state, polling, saving and copying --
// uses the article's values unchanged and never reaches into this element.

import { parseArticleMarkdown } from "../core/articleMarkdown.ts";
import type {
  ArticleBlock,
  ArticleInline,
  ArticleItem,
  ArticleLine,
  LinkUrl,
} from "../core/articleMarkdown.ts";
import { drawOnKeyChange } from "./drawOnChange.ts";

/**
 * The link of an autolink. This is the only function that sets an href, and
 * it takes only a LinkUrl, a URL that met every link condition. The link
 * shows the same string it opens, in a new tab with no referrer and no
 * opener, and runs left to right whatever the text around it.
 */
function linkElement(doc: Document, url: LinkUrl): HTMLElement {
  const link = doc.createElement("a");
  link.setAttribute("href", url);
  link.setAttribute("target", "_blank");
  link.setAttribute("rel", "noopener noreferrer");
  link.setAttribute("dir", "ltr");
  link.append(doc.createTextNode(url));
  return link;
}

/** An element of name holding text as its one text node. */
function textElement(doc: Document, name: string, text: string): HTMLElement {
  const element = doc.createElement(name);
  element.append(doc.createTextNode(text));
  return element;
}

/** The node of one inline piece. */
function inlineNode(doc: Document, piece: ArticleInline): Node {
  switch (piece.kind) {
    case "text":
      return doc.createTextNode(piece.text);
    case "strong":
      return textElement(doc, "strong", piece.text);
    case "emphasis":
      return textElement(doc, "em", piece.text);
    case "code":
      return textElement(doc, "code", piece.text);
    case "link":
      return linkElement(doc, piece.url);
    default: {
      const exhaustive: never = piece;
      throw new Error(`Unhandled article inline: ${String(exhaustive)}`);
    }
  }
}

/**
 * Appends the lines to parent with one LF text node between two lines, so
 * the CSS's pre-wrap shows each source line on its own line. Each node is
 * appended alone: one line may hold a hundred thousand pieces, too many to
 * spread into the arguments of one call.
 */
function appendLines(
  doc: Document,
  parent: HTMLElement,
  lines: readonly ArticleLine[],
): void {
  lines.forEach((line, index) => {
    if (index > 0) {
      parent.append(doc.createTextNode("\n"));
    }
    for (const piece of line) {
      parent.append(inlineNode(doc, piece));
    }
  });
}

/** A list of name (ul or ol) with one li per item and nothing between them. */
function listElement(
  doc: Document,
  name: "ul" | "ol",
  items: readonly ArticleItem[],
): HTMLElement {
  const list = doc.createElement(name);
  for (const item of items) {
    const entry = doc.createElement("li");
    appendLines(doc, entry, item);
    list.append(entry);
  }
  return list;
}

/**
 * The element of one block. The article's title heading is an h2 and a
 * section heading an h3, below the video section's h1 in the viewer.
 */
function blockElement(doc: Document, block: ArticleBlock): HTMLElement {
  switch (block.kind) {
    case "heading": {
      const heading = doc.createElement(block.level === "title" ? "h2" : "h3");
      appendLines(doc, heading, [block.line]);
      return heading;
    }
    case "paragraph": {
      const paragraph = doc.createElement("p");
      appendLines(doc, paragraph, block.lines);
      return paragraph;
    }
    case "bulletList":
      return listElement(doc, "ul", block.items);
    case "orderedList": {
      const list = listElement(doc, "ol", block.items);
      list.setAttribute("start", String(block.start));
      return list;
    }
    case "codeBlock": {
      const pre = doc.createElement("pre");
      pre.append(textElement(doc, "code", block.lines.join("\n")));
      return pre;
    }
    default: {
      const exhaustive: never = block;
      throw new Error(`Unhandled article block: ${String(exhaustive)}`);
    }
  }
}

/**
 * Builds the article element from the parsed tree alone; it never receives
 * the Markdown string. The element is a div.article-body with dir=ltr and
 * lang=ja, fixed values: the article is Japanese, and each line's direction
 * comes from its own text through the CSS's unicode-bidi: plaintext. No text
 * node goes between two blocks or two items, so the element's text is the
 * tree's lines and nothing else.
 */
export function buildArticleBody(
  doc: Document,
  blocks: readonly ArticleBlock[],
): HTMLElement {
  const body = doc.createElement("div");
  body.setAttribute("class", "article-body");
  body.setAttribute("dir", "ltr");
  body.setAttribute("lang", "ja");
  for (const block of blocks) {
    body.append(blockElement(doc, block));
  }
  return body;
}

/**
 * Draws the display Markdown into container, unless the same Markdown is
 * already drawn there. A poll that brings the same article again keeps the
 * reader's focus and selection inside it; a container emptied in between is
 * drawn again.
 */
export function drawArticleBody(
  container: HTMLElement,
  markdown: string,
): void {
  drawOnKeyChange(container, markdown, () => {
    container.replaceChildren(
      buildArticleBody(container.ownerDocument, parseArticleMarkdown(markdown)),
    );
  });
}

/**
 * The article body as text: markdown is set with textContent and never
 * parsed, so tags and links in it stay characters and the element gets no
 * child element and no attribute from it. The CSS keeps its line breaks and
 * spaces (pre-wrap) and its text direction (plaintext); dir=auto takes the
 * direction from the text.
 */
export function renderArticleBody(
  doc: Document,
  markdown: string,
): HTMLElement {
  const body = doc.createElement("div");
  body.className = "article-body";
  body.setAttribute("dir", "auto");
  body.textContent = markdown;
  return body;
}
