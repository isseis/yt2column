import assert from "node:assert/strict";
import { describe, it } from "node:test";

import { catalogs } from "../src/core/i18n/catalog.ts";
import { locales } from "../src/core/i18n/locale.ts";
import type { Locale } from "../src/core/i18n/locale.ts";
import { rejectionMessage } from "../src/core/messages.ts";
import type { OutcomeSummary, SummaryField } from "../src/core/types.ts";
import { checkIcon } from "../src/ui/icons.ts";
import { renderSummary } from "../src/ui/render.ts";
import {
  container,
  expectAbsent,
  expectLocaleMessage,
  otherLocales,
  valueOf,
} from "./helpers/dom.ts";

const malicious = "<img src=x onerror=alert(1)>";
const localeList = Object.keys(locales) as Locale[];

/** The tag names of the symbol's shapes, in document order. */
function shapeNames(icon: SVGSVGElement): string[] {
  return Array.from(icon.children, (child) => child.tagName.toLowerCase());
}

describe("checkIcon", () => {
  const doc = container().ownerDocument;

  it("draws a check in a circle for ok and an exclamation mark in a triangle for warning", () => {
    // The two states must be told apart by shape, not by colour alone.
    assert.deepEqual(shapeNames(checkIcon(doc, "ok")), ["circle", "polyline"]);
    assert.deepEqual(shapeNames(checkIcon(doc, "warning")), [
      "polygon",
      "rect",
      "circle",
    ]);
  });

  for (const state of ["ok", "warning"] as const) {
    it(`hides the ${state} symbol from assistive technology`, () => {
      assert.equal(checkIcon(doc, state).getAttribute("aria-hidden"), "true");
    });

    it(`draws the ${state} symbol without a text character`, () => {
      const icon = checkIcon(doc, state);
      assert.equal(icon.textContent, "");
      assert.equal(icon.querySelector("text"), null);
    });
  }
});

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
  // The English labels stay literal strings: reading them from the catalog
  // would let a changed English label pass unnoticed.
  it("shows the title, URL, counts and preview with labels", () => {
    const root = container();
    renderSummary(root, collected, "en");
    assert.equal(valueOf(root, "Title"), collected.title);
    assert.equal(valueOf(root, "URL"), collected.url);
    assert.equal(valueOf(root, "Characters"), "12");
    assert.equal(valueOf(root, "Lines"), "2");
    assert.equal(valueOf(root, "Preview"), collected.preview);
    assert.deepEqual(
      Array.from(
        root.querySelectorAll(".summary-label"),
        (node) => node.textContent,
      ),
      ["Title", "URL", "Characters", "Lines", "Preview"],
    );
  });

  for (const locale of localeList) {
    const labels = catalogs[locale].summaryLabels;

    it(`shows the values unchanged under the ${locale} labels`, () => {
      const root = container();
      renderSummary(root, collected, locale);
      assert.equal(valueOf(root, labels.title), collected.title);
      assert.equal(valueOf(root, labels.url), collected.url);
      assert.equal(valueOf(root, labels.characters), "12");
      assert.equal(valueOf(root, labels.lines), "2");
      assert.equal(valueOf(root, labels.preview), collected.preview);
      // Only the labels whose wording differs between locales can be shown
      // absent; the absence check asserts that they differ first.
      const shown = Object.values(labels);
      for (const other of otherLocales(locale)) {
        const otherLabels = catalogs[other].summaryLabels;
        const differing = (Object.keys(labels) as SummaryField[])
          .filter((name) => otherLabels[name] !== labels[name])
          .map((name) => otherLabels[name]);
        assert.ok(differing.length > 0, other);
        expectAbsent(root, differing, shown);
      }
    });

    // en and ja group digits the same way, so this shows the grouping in each
    // locale but cannot tell which locale reached formatCount.
    it(`groups the digits of the counts in ${locale}`, () => {
      const root = container();
      renderSummary(
        root,
        { ...collected, characterCount: 12345, lineCount: 1234 },
        locale,
      );
      assert.equal(valueOf(root, labels.characters), "12,345");
      assert.equal(valueOf(root, labels.lines), "1,234");
    });

    it(`marks the title and preview for automatic direction in ${locale}`, () => {
      const root = container();
      renderSummary(root, collected, locale);
      assert.equal(
        root.querySelector(".summary-title")?.getAttribute("dir"),
        "auto",
      );
      assert.equal(
        root.querySelector(".summary-preview")?.getAttribute("dir"),
        "auto",
      );
    });

    it(`marks a shortened title or preview with an ellipsis in ${locale}`, () => {
      const root = container();
      renderSummary(
        root,
        { ...collected, titleTruncated: true, previewTruncated: true },
        locale,
      );
      assert.equal(valueOf(root, labels.title), `${collected.title}\u2026`);
      assert.equal(valueOf(root, labels.preview), `${collected.preview}\u2026`);
    });

    it(`displays markup in the title and preview as text, not elements, in ${locale}`, () => {
      const root = container();
      renderSummary(
        root,
        { ...collected, title: malicious, preview: malicious },
        locale,
      );
      assert.equal(root.querySelector("img"), null);
      assert.ok(root.textContent?.includes(malicious));
    });

    it(`shows the ${locale} rejection message and its steps as a numbered list`, () => {
      const root = container();
      renderSummary(
        root,
        { kind: "rejected", reason: "not-watch-page" },
        locale,
      );
      expectLocaleMessage(root, locale, (l) =>
        rejectionMessage("not-watch-page", l),
      );
      const items = root.querySelectorAll("ol.summary-steps > li");
      assert.equal(
        items.length,
        rejectionMessage("not-watch-page", locale).steps.length,
      );
    });

    it(`shows no list when the rejection has no steps in ${locale}`, () => {
      const root = container();
      renderSummary(root, { kind: "rejected", reason: "empty-title" }, locale);
      expectLocaleMessage(root, locale, (l) =>
        rejectionMessage("empty-title", l),
      );
      assert.equal(root.querySelector("ol.summary-steps"), null);
    });

    it(`replaces the previous content on each call in ${locale}`, () => {
      const root = container();
      renderSummary(root, collected, locale);
      renderSummary(
        root,
        { kind: "rejected", reason: "empty-selection" },
        locale,
      );
      assert.equal(root.querySelectorAll(".summary-field").length, 0);
    });
  }
});
