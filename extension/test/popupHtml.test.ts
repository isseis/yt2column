import assert from "node:assert/strict";
import { describe, it } from "node:test";

import type { CollectedInput } from "../src/core/collect.ts";
import { acceptMetadata } from "../src/core/metadata.ts";
import { buildCollectedView } from "../src/core/popupView.ts";
import { prepareSend } from "../src/core/prepare.ts";
import { renderPopup } from "../src/ui/render.ts";
import { popupDocument, popupElements } from "./helpers/dom.ts";

/** Every element a user can press or focus, in document order. */
const interactiveSelector =
  "a[href], button, input, select, textarea, summary, [tabindex], [contenteditable]";

/** An element's tag name and id, such as "section#video". */
function describeElement(element: Element): string {
  return `${element.tagName.toLowerCase()}#${element.id}`;
}

describe("popup.html skeleton", () => {
  it("holds the five containers in display order, each hidden", () => {
    const doc = popupDocument();
    const containers = Array.from(doc.body.children).filter(
      (child) => child.tagName.toLowerCase() !== "script",
    );
    assert.deepEqual(containers.map(describeElement), [
      "section#video",
      "section#checks",
      "section#rejection",
      "section#action",
      "details#details",
    ]);
    for (const element of containers) {
      assert.equal(element.hasAttribute("hidden"), true, element.id);
    }
  });

  it("starts in English until startPopup sets the display language", () => {
    assert.equal(popupDocument().documentElement.lang, "en");
  });

  it("names the rejection card by the reason's sentence", () => {
    const doc = popupDocument();
    assert.equal(
      doc.getElementById("rejection")?.getAttribute("aria-labelledby"),
      "rejection-text",
    );
  });

  it("puts a disabled primary button and then the status region in the action section", () => {
    const action = popupDocument().getElementById("action");
    assert.ok(action !== null);
    assert.deepEqual(Array.from(action.children, describeElement), [
      "button#primary",
      "div#status",
    ]);
    const [primary, status] = Array.from(action.children);
    assert.equal(primary?.getAttribute("type"), "button");
    assert.equal(primary?.hasAttribute("disabled"), true);
    assert.equal(status?.getAttribute("role"), "status");
  });

  it("has the status region as its only live region", () => {
    const doc = popupDocument();
    assert.deepEqual(
      Array.from(
        doc.querySelectorAll("[role=status], [role=alert], [aria-live]"),
        describeElement,
      ),
      ["div#status"],
    );
  });

  it("leaves only the primary button and the details toggle to press, in display order, without tabindex", () => {
    // The toggle is drawn by renderPopup, so the shipped skeleton is checked
    // once a collected view is drawn into it.
    const elements = popupElements();
    const input = {
      selection: "selection",
      url: "https://www.youtube.com/watch?v=abc",
      title: "A tab title",
    } as CollectedInput;
    const metadata = acceptMetadata({ descriptionCollapsed: false });
    renderPopup(
      elements,
      buildCollectedView(input, metadata, prepareSend(input, metadata.values)),
      "en",
    );
    const doc = elements.primary.ownerDocument;
    const interactive = Array.from(doc.querySelectorAll(interactiveSelector));
    assert.deepEqual(
      interactive.map((element) => element.tagName.toLowerCase()),
      ["button", "summary"],
    );
    assert.equal(interactive[0], elements.primary);
    assert.equal(interactive[1]?.parentElement, elements.details);
    assert.equal(doc.querySelector("[tabindex]"), null);
  });
});
