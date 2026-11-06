// The check of a built article element against the list of elements and
// attributes the builder may make. Every value is checked here on its own
// terms, without the parse, so an article tree that reached the builder by a
// type assertion cannot pass with a value the parse would never have made.

import assert from "node:assert/strict";

/** The fixed attributes of the article element. */
const bodyAttributes: Readonly<Record<string, string>> = {
  class: "article-body",
  dir: "ltr",
  lang: "ja",
};

/** The fixed attributes of a link, beside its href. */
const linkAttributes: Readonly<Record<string, string>> = {
  target: "_blank",
  rel: "noopener noreferrer",
  dir: "ltr",
};

/** The elements the builder makes inside the article element, with no attribute unless named below. */
const plainElements = new Set([
  "h2",
  "h3",
  "p",
  "ul",
  "li",
  "pre",
  "code",
  "strong",
  "em",
]);

/** The attributes of element, by name. */
function attributes(element: Element): Record<string, string> {
  return Object.fromEntries(
    Array.from(element.attributes, (attribute) => [
      attribute.name,
      attribute.value,
    ]),
  );
}

/** Checks a link's href: an https URL that serializes to itself, with no user name or password, shown as its text. */
function assertLink(element: Element, where: string): void {
  const { href, ...fixed } = attributes(element);
  assert.deepEqual(fixed, linkAttributes, where);
  assert.ok(href !== undefined, `${where}: a link without href`);
  assert.ok(href.startsWith("https://"), `${where}: ${href}`);
  const url = new URL(href);
  assert.equal(url.href, href, where);
  assert.equal(url.username, "", where);
  assert.equal(url.password, "", where);
  assert.equal(element.textContent, href, where);
}

/**
 * Asserts that body is an article element the builder may make: a div with
 * the fixed attributes, and below it only text nodes and the listed elements,
 * each with only its listed attributes and values. where names the input in
 * a failure.
 */
export function assertArticleDom(body: Element, where: string): void {
  assert.equal(body.tagName.toLowerCase(), "div", where);
  assert.deepEqual(attributes(body), bodyAttributes, where);
  const pending: Node[] = [];
  for (const child of Array.from(body.childNodes)) {
    pending.push(child);
  }
  for (let node = pending.pop(); node !== undefined; node = pending.pop()) {
    if (node.nodeType === node.TEXT_NODE) {
      continue;
    }
    assert.equal(
      node.nodeType,
      node.ELEMENT_NODE,
      `${where}: a node of type ${node.nodeType}`,
    );
    const element = node as Element;
    const name = element.tagName.toLowerCase();
    if (name === "a") {
      assertLink(element, where);
    } else if (name === "ol") {
      const { start, ...rest } = attributes(element);
      assert.deepEqual(rest, {}, `${where}: ol`);
      assert.match(start ?? "", /^[0-9]{1,9}$/, `${where}: ol start`);
    } else {
      assert.ok(plainElements.has(name), `${where}: element ${name}`);
      assert.deepEqual(attributes(element), {}, `${where}: ${name}`);
    }
    // One at a time: a paragraph may hold a hundred thousand children.
    for (const child of Array.from(element.childNodes)) {
      pending.push(child);
    }
  }
}
