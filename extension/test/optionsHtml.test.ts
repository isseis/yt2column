import assert from "node:assert/strict";
import { describe, it } from "node:test";

import { optionsDocument } from "./helpers/dom.ts";

/** Every element a user can press or focus, in document order. */
const interactiveSelector =
  "a[href], button, input, select, textarea, summary, [tabindex], [contenteditable]";

/** An element's tag name and id, such as "input#api-key". */
function describeElement(element: Element): string {
  return `${element.tagName.toLowerCase()}#${element.id}`;
}

describe("options.html skeleton", () => {
  it("loads the shared theme before the options page's layout", () => {
    assert.deepEqual(
      Array.from(
        optionsDocument().querySelectorAll("link[rel=stylesheet]"),
        (link) => link.getAttribute("href"),
      ),
      ["theme.css", "options.css"],
    );
  });

  it("starts in English with a page title until startOptions sets them", () => {
    const doc = optionsDocument();
    assert.equal(doc.documentElement.lang, "en");
    assert.notEqual(doc.title.trim(), "");
  });

  it("puts the two fields, then save and delete, in display order without tabindex", () => {
    const doc = optionsDocument();
    assert.deepEqual(
      Array.from(doc.querySelectorAll(interactiveSelector), describeElement),
      ["input#api-key", "input#workspace-id", "button#save", "button#delete"],
    );
    assert.equal(doc.querySelector("[tabindex]"), null);
    for (const id of ["save", "delete"]) {
      assert.equal(doc.getElementById(id)?.getAttribute("type"), "button", id);
    }
  });

  it("gives the buttons and the fields the classes that size them for style.test.ts", () => {
    // style.test.ts checks the 44px rules of .action-button and .field input;
    // these classes are what make the page's controls take them.
    const doc = optionsDocument();
    for (const id of ["save", "delete"]) {
      assert.ok(
        doc.getElementById(id)?.classList.contains("action-button"),
        id,
      );
    }
    for (const id of ["api-key", "workspace-id"]) {
      assert.ok(
        doc.getElementById(id)?.parentElement?.classList.contains("field"),
        id,
      );
    }
  });

  it("keeps the fields out of a form and off the browser's password and spelling help", () => {
    // A password field or a field in a form with a submit makes the browser
    // offer to save the key, which account sync would carry elsewhere.
    const doc = optionsDocument();
    assert.equal(doc.querySelector("form"), null);
    assert.equal(doc.querySelector("input[type=password]"), null);
    for (const id of ["api-key", "workspace-id"]) {
      const input = doc.getElementById(id);
      assert.equal(input?.getAttribute("type"), "text", id);
      assert.equal(input?.getAttribute("autocomplete"), "off", id);
      assert.equal(input?.getAttribute("spellcheck"), "false", id);
    }
  });

  it("names each field by its label and describes it by its hint", () => {
    const doc = optionsDocument();
    for (const id of ["api-key", "workspace-id"]) {
      const label = doc.querySelector(`label[for="${id}"]`);
      assert.equal(label?.id, `${id}-label`, id);
      const hint = doc.getElementById(
        doc.getElementById(id)?.getAttribute("aria-describedby") ?? "",
      );
      assert.equal(hint?.id, `${id}-hint`, id);
    }
  });

  it("has the status region as its only live region", () => {
    const doc = optionsDocument();
    assert.deepEqual(
      Array.from(
        doc.querySelectorAll("[role=status], [role=alert], [aria-live]"),
        describeElement,
      ),
      ["div#status"],
    );
  });
});
