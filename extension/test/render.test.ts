import assert from "node:assert/strict";
import { describe, it } from "node:test";

import { rejectionMessage } from "../src/core/messages.ts";
import type { OutcomeSummary } from "../src/core/types.ts";
import { renderSummary } from "../src/ui/render.ts";
import { container, valueOf } from "./helpers/dom.ts";

const malicious = "<img src=x onerror=alert(1)>";

const collected: Extract<OutcomeSummary, { kind: "collected" }> = {
  kind: "collected",
  title: "A video",
  titleTruncated: false,
  url: "https://www.youtube.com/watch?v=abc",
  urlTruncated: false,
  characterCount: 12,
  lineCount: 2,
  preview: "00:00 hello\nworld",
  previewTruncated: false,
};

describe("renderSummary", () => {
  it("shows the title, URL, counts and preview with labels", () => {
    const root = container();
    renderSummary(root, collected, "en");
    assert.equal(valueOf(root, "Title"), collected.title);
    assert.equal(valueOf(root, "URL"), collected.url);
    assert.equal(valueOf(root, "Characters"), "12");
    assert.equal(valueOf(root, "Lines"), "2");
    assert.equal(valueOf(root, "Preview"), collected.preview);
  });

  it("marks the title and preview for automatic direction", () => {
    const root = container();
    renderSummary(root, collected, "en");
    assert.equal(
      root.querySelector(".summary-title")?.getAttribute("dir"),
      "auto",
    );
    assert.equal(
      root.querySelector(".summary-preview")?.getAttribute("dir"),
      "auto",
    );
  });

  it("marks a shortened title or preview with an ellipsis", () => {
    const root = container();
    renderSummary(
      root,
      {
        ...collected,
        titleTruncated: true,
        previewTruncated: true,
      },
      "en",
    );
    assert.equal(valueOf(root, "Title"), `${collected.title}\u2026`);
    assert.equal(valueOf(root, "Preview"), `${collected.preview}\u2026`);
  });

  it("displays markup in the title and preview as text, not elements", () => {
    const root = container();
    renderSummary(
      root,
      {
        ...collected,
        title: malicious,
        preview: malicious,
      },
      "en",
    );
    assert.equal(root.querySelector("img"), null);
    assert.ok(root.textContent?.includes(malicious));
  });

  it("shows the rejection message and its steps as a numbered list", () => {
    const root = container();
    renderSummary(root, { kind: "rejected", reason: "not-watch-page" }, "en");
    const message = rejectionMessage("not-watch-page", "en");
    assert.ok(root.textContent?.includes(message.text));
    const items = root.querySelectorAll("ol.summary-steps > li");
    assert.equal(items.length, message.steps.length);
  });

  it("shows no list when the rejection has no steps", () => {
    const root = container();
    renderSummary(root, { kind: "rejected", reason: "empty-title" }, "en");
    assert.ok(
      root.textContent?.includes(rejectionMessage("empty-title", "en").text),
    );
    assert.equal(root.querySelector("ol.summary-steps"), null);
  });

  it("replaces the previous content on each call", () => {
    const root = container();
    renderSummary(root, collected, "en");
    renderSummary(root, { kind: "rejected", reason: "empty-selection" }, "en");
    assert.equal(root.querySelectorAll(".summary-field").length, 0);
  });
});
