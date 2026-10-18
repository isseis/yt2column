import assert from "node:assert/strict";
import { describe, it } from "node:test";

import { expandPromptMessage } from "../src/core/messages.ts";
import type { MetadataResult } from "../src/core/metadata.ts";
import { metadataDisplayLimit, renderMetadata } from "../src/ui/render.ts";
import { container } from "./helpers/dom.ts";

const malicious = "<img src=x onerror=alert(1)>";

/** The displayed value of the field labelled label, if any. */
function valueOf(root: HTMLElement, label: string): string | undefined {
  for (const field of root.querySelectorAll(".summary-field")) {
    if (field.querySelector(".summary-label")?.textContent === label) {
      return field.querySelector(".summary-value")?.textContent ?? undefined;
    }
  }
  return undefined;
}

/** A metadata result with the given values and prompt flag. */
function result(over: Partial<MetadataResult> = {}): MetadataResult {
  return { values: {}, showExpandPrompt: false, omissions: [], ...over };
}

describe("renderMetadata", () => {
  it("shows each present value with its label", () => {
    const root = container();
    renderMetadata(
      root,
      result({
        values: {
          title: "A title",
          channelName: "A channel",
          description: "A description",
        },
      }),
    );
    assert.equal(valueOf(root, "Video title"), "A title");
    assert.equal(valueOf(root, "Channel"), "A channel");
    assert.equal(valueOf(root, "Description"), "A description");
  });

  it("shows only the values that are present", () => {
    const root = container();
    renderMetadata(root, result({ values: { channelName: "A channel" } }));
    assert.equal(valueOf(root, "Channel"), "A channel");
    assert.equal(valueOf(root, "Video title"), undefined);
    assert.equal(valueOf(root, "Description"), undefined);
  });

  it("displays markup in a value as text, not elements", () => {
    const root = container();
    renderMetadata(
      root,
      result({ values: { title: malicious, description: malicious } }),
    );
    assert.equal(root.querySelector("img"), null);
    assert.ok(root.textContent?.includes(malicious));
  });

  it("marks each metadata value for automatic direction", () => {
    const root = container();
    renderMetadata(
      root,
      result({ values: { title: "t", channelName: "c", description: "d" } }),
    );
    for (const selector of [
      ".metadata-title",
      ".metadata-channel",
      ".metadata-description",
    ]) {
      assert.equal(root.querySelector(selector)?.getAttribute("dir"), "auto");
    }
  });

  it("keeps a value exactly at the limit unchanged", () => {
    const root = container();
    const exact = "a".repeat(metadataDisplayLimit);
    renderMetadata(root, result({ values: { title: exact } }));
    assert.equal(valueOf(root, "Video title"), exact);
  });

  it("truncates a value one over the limit with an ellipsis", () => {
    const root = container();
    const over = "a".repeat(metadataDisplayLimit + 1);
    renderMetadata(root, result({ values: { title: over } }));
    assert.equal(
      valueOf(root, "Video title"),
      `${"a".repeat(metadataDisplayLimit)}\u2026`,
    );
  });

  it("does not split a surrogate pair at the boundary", () => {
    const root = container();
    const pair = "\u{1F600}";
    const value = `${"a".repeat(metadataDisplayLimit - 1)}${pair}b`;
    renderMetadata(root, result({ values: { description: value } }));
    const shown = valueOf(root, "Description");
    assert.equal(shown, `${"a".repeat(metadataDisplayLimit - 1)}${pair}\u2026`);
    // A UTF-16 slice would leave a lone surrogate behind.
    assert.equal(/\p{Cs}/u.test(shown ?? ""), false);
  });

  it("appends without replacing the existing children", () => {
    const root = container();
    const kept = root.ownerDocument.createElement("p");
    kept.textContent = "kept";
    root.append(kept);
    renderMetadata(root, result({ values: { title: "A title" } }));
    assert.equal(root.querySelector("p")?.textContent, "kept");
    assert.equal(valueOf(root, "Video title"), "A title");
  });

  it("shows the expand prompt when the description is collapsed", () => {
    const root = container();
    renderMetadata(
      root,
      result({
        values: { description: "beginning" },
        showExpandPrompt: true,
      }),
    );
    assert.ok(root.textContent?.includes(expandPromptMessage.text));
    const items = root.querySelectorAll("ol.summary-steps > li");
    assert.equal(items.length, expandPromptMessage.steps.length);
  });

  it("shows no expand prompt otherwise", () => {
    const root = container();
    renderMetadata(
      root,
      result({ values: { description: "beginning" }, showExpandPrompt: false }),
    );
    assert.ok(!(root.textContent ?? "").includes(expandPromptMessage.text));
    assert.equal(root.querySelector("ol.summary-steps"), null);
  });

  it("does not put page text in the expand prompt", () => {
    const root = container();
    const description = "A page-only description";
    renderMetadata(
      root,
      result({ values: { description }, showExpandPrompt: true }),
    );
    const promptText = Array.from(
      root.querySelectorAll(".summary-message, .summary-steps"),
    )
      .map((node) => node.textContent)
      .join("");
    assert.ok(!promptText.includes(description));
    assert.equal(valueOf(root, "Description"), description);
  });
});
