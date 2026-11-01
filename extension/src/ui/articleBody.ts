// The one place the article's body is drawn. The rest of the viewer -- the
// state, polling, saving and copying -- uses the article's values unchanged
// and never reaches into this element, so the body can later be drawn as
// rendered Markdown by replacing this function and its CSS rules alone.

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
