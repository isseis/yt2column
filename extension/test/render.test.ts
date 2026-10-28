import assert from "node:assert/strict";
import { describe, it } from "node:test";

import type { CollectedInput } from "../src/core/collect.ts";
import { catalogs } from "../src/core/i18n/catalog.ts";
import { locales } from "../src/core/i18n/locale.ts";
import type { Locale } from "../src/core/i18n/locale.ts";
import { acceptMetadata } from "../src/core/metadata.ts";
import { expandPromptMessage, rejectionMessage } from "../src/core/messages.ts";
import { buildCollectedView } from "../src/core/popupView.ts";
import type { CollectedView, PopupView } from "../src/core/popupView.ts";
import { maxBodyBytes, prepareSend } from "../src/core/prepare.ts";
import { rejectionReasons } from "../src/core/types.ts";
import type {
  OutcomeSummary,
  RejectionReason,
  SummaryField,
} from "../src/core/types.ts";
import { checkIcon } from "../src/ui/icons.ts";
import { renderPopup, renderSummary } from "../src/ui/render.ts";
import type { PopupElements } from "../src/ui/render.ts";
import {
  container,
  expectAbsent,
  expectLocaleMessage,
  expectMessage,
  otherLocales,
  popupElements,
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
      const items = root.querySelectorAll("ol.message-steps > li");
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
      assert.equal(root.querySelector("ol.message-steps"), null);
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

const watchUrl = "https://www.youtube.com/watch?v=abc";

/** A collected input fixture; tests may assert the branded type to build one. */
function collectedInput(
  selection = "selection",
  title = "A tab title",
  url = watchUrl,
): CollectedInput {
  return { selection, url, title } as CollectedInput;
}

/** The collected view for an input and the metadata it accepts. */
function collectedView(
  input: CollectedInput = collectedInput(),
  values: { title?: string; channelName?: string; description?: string } = {},
  descriptionCollapsed = false,
): CollectedView {
  const metadata = acceptMetadata({ ...values, descriptionCollapsed });
  return buildCollectedView(
    input,
    metadata,
    prepareSend(input, metadata.values),
  );
}

/** The rendered check rows, in display order. */
function checkRows(elements: PopupElements): HTMLLIElement[] {
  return Array.from(elements.checks.querySelectorAll("li.check-row"));
}

/** The document position flag meaning "the second node follows the first". */
const documentPositionFollowing = 4;

/** True when first comes before second in document order. */
function precedes(first: Node, second: Node): boolean {
  return (
    (first.compareDocumentPosition(second) & documentPositionFollowing) !== 0
  );
}

/** A check row's parts in document order: the symbol, then each child's class. */
function rowParts(row: Element): string[] {
  return Array.from(row.children, (child) =>
    child.tagName.toLowerCase() === "svg"
      ? "icon"
      : (child.classList.item(0) ?? ""),
  );
}

describe("renderPopup", () => {
  // The English row names and button name stay literal strings: reading them
  // from the catalog would let a changed English name pass unnoticed.
  it("shows the English row names and primary button name literally", () => {
    const elements = popupElements();
    renderPopup(elements, collectedView(), "en");
    assert.deepEqual(
      Array.from(
        elements.checks.querySelectorAll(".check-name"),
        (node) => node.textContent,
      ),
      ["Selected captions", "Description"],
    );
    assert.equal(elements.primary.textContent, "Generate article");
  });

  for (const locale of localeList) {
    const catalog = catalogs[locale];

    it(`shows the collected sections in display order and keeps the details closed in ${locale}`, () => {
      const elements = popupElements();
      renderPopup(elements, collectedView(), locale);
      assert.equal(elements.rejection.hidden, true);
      for (const section of [
        elements.video,
        elements.checks,
        elements.action,
        elements.details,
      ]) {
        assert.equal(section.hidden, false);
      }
      assert.ok(precedes(elements.video, elements.checks));
      assert.ok(precedes(elements.checks, elements.action));
      assert.ok(precedes(elements.action, elements.details));
      assert.equal(elements.details.open, false);
      assert.equal(
        elements.video.querySelector("h1"),
        elements.video.firstElementChild,
      );
    });

    it(`draws the video title and the channel name under it in ${locale}`, () => {
      const elements = popupElements();
      renderPopup(
        elements,
        collectedView(collectedInput(), {
          title: "Video",
          channelName: "Channel",
        }),
        locale,
      );
      const [heading, channelName] = Array.from(elements.video.children);
      assert.equal(heading?.tagName, "H1");
      assert.equal(heading?.textContent, "Video");
      assert.equal(channelName?.tagName, "P");
      assert.equal(channelName?.textContent, "Channel");
    });

    it(`uses the tab title for the heading when the video title is not sent in ${locale}`, () => {
      const elements = popupElements();
      renderPopup(
        elements,
        collectedView(collectedInput("selection", "A tab title")),
        locale,
      );
      assert.equal(
        elements.video.querySelector("h1")?.textContent,
        "A tab title",
      );
      assert.equal(elements.video.querySelector("p"), null);
    });

    it(`draws the primary button's name without enabling it in ${locale}`, () => {
      const elements = popupElements();
      renderPopup(elements, collectedView(), locale);
      assert.equal(elements.primary.textContent, catalog.primaryButton);
      assert.equal(elements.primary.disabled, true);
    });

    it(`keeps the URL and the tab title inside the details in ${locale}`, () => {
      const elements = popupElements();
      const view = collectedView(collectedInput("selection", "A tab title"), {
        title: "Video",
      });
      renderPopup(elements, view, locale);
      const doc = elements.video.ownerDocument;
      // The tab title is not the heading here, so finding it outside the
      // details cannot be the heading AC-03 allows.
      assert.equal(view.video.heading.value, "Video");
      const outside = Array.from(doc.body.children)
        .filter((section) => section.id !== "details")
        .map((section) => section.textContent)
        .join("");
      assert.ok(!outside.includes(view.details.url.value));
      assert.ok(!outside.includes(view.details.tabTitle.value));
      const inside = elements.details.textContent ?? "";
      assert.ok(inside.includes(view.details.url.value));
      assert.ok(inside.includes(view.details.tabTitle.value));
    });

    it(`names the details toggle and pairs each value with its own name in ${locale}`, () => {
      const elements = popupElements();
      const view = collectedView(collectedInput("selection", "A tab title"), {
        title: "Video",
      });
      renderPopup(elements, view, locale);
      assert.equal(
        elements.details.querySelector("summary")?.textContent,
        catalog.details.summary,
      );
      const rows = Array.from(
        elements.details.querySelectorAll(".details-list > *"),
        (node) => [node.tagName, node.textContent],
      );
      assert.deepEqual(rows, [
        ["DT", catalog.details.url],
        ["DD", view.details.url.value],
        ["DT", catalog.details.tabTitle],
        ["DD", view.details.tabTitle.value],
      ]);
    });

    it(`marks a truncated heading and truncated details with an ellipsis in ${locale}`, () => {
      const elements = popupElements();
      const view = collectedView(
        collectedInput("selection", "t".repeat(1001), "u".repeat(1001)),
        { title: "v".repeat(201) },
      );
      assert.equal(view.video.heading.truncated, true);
      renderPopup(elements, view, locale);
      assert.ok(
        (elements.video.querySelector("h1")?.textContent ?? "").endsWith(
          "\u2026",
        ),
      );
      const values = Array.from(
        elements.details.querySelectorAll(".detail-value"),
        (node) => node.textContent ?? "",
      );
      assert.equal(values.length, 2);
      for (const value of values) {
        assert.ok(value.endsWith("\u2026"), value);
      }
    });

    it(`orders each check row's parts and shows its state word in ${locale}`, () => {
      const elements = popupElements();
      renderPopup(
        elements,
        collectedView(collectedInput(), { description: "A description" }, true),
        locale,
      );
      const [selection, description] = checkRows(elements);
      assert.ok(selection !== undefined && description !== undefined);
      assert.deepEqual(rowParts(selection), [
        "icon",
        "check-name",
        "check-state",
        "check-content",
      ]);
      assert.deepEqual(rowParts(description), [
        "icon",
        "check-name",
        "check-state",
        "check-content",
      ]);
      assert.equal(
        selection.querySelector("svg")?.getAttribute("aria-hidden"),
        "true",
      );
      assert.equal(
        selection.querySelector(".check-name")?.textContent,
        catalog.checkNames.selection,
      );
      assert.equal(
        description.querySelector(".check-name")?.textContent,
        catalog.checkNames.description,
      );
      assert.equal(
        selection.querySelector(".check-state")?.textContent,
        catalog.checkStates.ok,
      );
      assert.equal(
        description.querySelector(".check-state")?.textContent,
        catalog.checkStates.warning,
      );
      // The state word is the row's state for assistive technology: hiding it
      // from the tree would leave only the aria-hidden symbol.
      assert.equal(
        selection.querySelector(".check-state")?.getAttribute("aria-hidden"),
        null,
      );
    });

    it(`marks only the warning row and its symbol with the warning class in ${locale}`, () => {
      const elements = popupElements();
      renderPopup(
        elements,
        collectedView(collectedInput(), { description: "A description" }, true),
        locale,
      );
      const [selection, description] = checkRows(elements);
      assert.ok(selection !== undefined && description !== undefined);
      assert.equal(selection.classList.contains("check-row-warning"), false);
      assert.equal(description.classList.contains("check-row-warning"), true);
      assert.equal(
        selection.querySelector("svg")?.classList.contains("check-icon-ok"),
        true,
      );
      assert.equal(
        description
          .querySelector("svg")
          ?.classList.contains("check-icon-warning"),
        true,
      );
    });

    it(`draws the description's own text nowhere in ${locale}`, () => {
      const description = "Page-only description text";
      const elements = popupElements();
      renderPopup(
        elements,
        collectedView(collectedInput(), { description }),
        locale,
      );
      assert.equal(
        elements.video.ownerDocument.body.textContent?.includes(description),
        false,
      );
    });

    it(`shows no other locale's wording in ${locale}`, () => {
      const elements = popupElements();
      renderPopup(
        elements,
        collectedView(
          collectedInput(),
          { title: "Video", channelName: "Channel", description: "A" },
          true,
        ),
        locale,
      );
      const own = [
        ...Object.values(catalog.checkNames),
        ...Object.values(catalog.checkStates),
        catalog.primaryButton,
        catalog.details.summary,
        catalog.details.url,
        catalog.details.tabTitle,
      ];
      for (const other of otherLocales(locale)) {
        const otherCatalog = catalogs[other];
        const differing = [
          ...Object.values(otherCatalog.checkNames),
          ...Object.values(otherCatalog.checkStates),
          otherCatalog.primaryButton,
          otherCatalog.details.summary,
          otherCatalog.details.url,
          otherCatalog.details.tabTitle,
        ].filter((wording) => !own.includes(wording));
        assert.ok(differing.length > 0, other);
        expectAbsent(elements.video.ownerDocument.body, differing, own);
      }
    });

    it(`displays markup in every value as text in ${locale}`, () => {
      const elements = popupElements();
      // Six values, each with the markup at both ends of the selection so the
      // head and the tail previews both carry it.
      const selection = `${malicious}${"a".repeat(100)}${malicious}`;
      renderPopup(
        elements,
        collectedView(collectedInput(selection, malicious, malicious), {
          title: malicious,
          channelName: malicious,
          description: malicious,
        }),
        locale,
      );
      const doc = elements.video.ownerDocument;
      assert.equal(doc.querySelector("img"), null);
      for (const selector of [
        ".video-heading",
        ".channel-name",
        ".preview-head",
        ".preview-tail-value",
        ".detail-value",
      ]) {
        const shown = Array.from(doc.querySelectorAll(selector));
        assert.ok(shown.length > 0, selector);
        for (const element of shown) {
          assert.ok(element.textContent?.includes(malicious), selector);
        }
      }
    });

    it(`marks every shown value for automatic direction in ${locale}`, () => {
      const selection = `${"h".repeat(120)}${"t".repeat(120)}`;
      const elements = popupElements();
      renderPopup(
        elements,
        collectedView(collectedInput(selection), {
          title: "Video",
          channelName: "Channel",
        }),
        locale,
      );
      const doc = elements.video.ownerDocument;
      for (const selector of [
        ".video-heading",
        ".channel-name",
        ".preview-head",
        ".preview-tail-value",
      ]) {
        assert.equal(
          doc.querySelector(selector)?.getAttribute("dir"),
          "auto",
          selector,
        );
      }
      const values = Array.from(doc.querySelectorAll(".detail-value"));
      assert.equal(values.length, 2);
      for (const value of values) {
        assert.equal(value.getAttribute("dir"), "auto");
      }
    });

    it(`shows the selection's counts and both previews cut with an ellipsis in ${locale}`, () => {
      const selection = `${"h".repeat(120)}\n${"t".repeat(120)}`;
      const elements = popupElements();
      renderPopup(elements, collectedView(collectedInput(selection)), locale);
      const [row] = checkRows(elements);
      assert.ok(row !== undefined);
      assert.equal(
        row.querySelector(".check-size")?.textContent,
        catalog.selectionSize(241, 2),
      );
      assert.equal(
        row.querySelector(".preview-head")?.textContent,
        `${"h".repeat(100)}\u2026`,
      );
      assert.equal(
        row.querySelector(".preview-tail-value")?.textContent,
        `\u2026${"t".repeat(100)}`,
      );
    });

    // Both locales group digits the same way, so this shows the grouping but
    // cannot tell which locale reached formatCount.
    it(`groups the digits of the counts in ${locale}`, () => {
      // 12,345 code points and 1,234 lines: 1,233 line breaks.
      const selection = `${"a".repeat(12345 - 1233)}${"\n".repeat(1233)}`;
      const elements = popupElements();
      renderPopup(elements, collectedView(collectedInput(selection)), locale);
      const text =
        checkRows(elements)[0]?.querySelector(".check-size")?.textContent;
      assert.ok(text?.includes("12,345"), text);
      assert.ok(text?.includes("1,234"), text);
    });

    it(`shows no tail preview when the selection fits in the head preview in ${locale}`, () => {
      const elements = popupElements();
      renderPopup(
        elements,
        collectedView(collectedInput("x".repeat(100))),
        locale,
      );
      const [row] = checkRows(elements);
      assert.ok(row !== undefined);
      assert.equal(row.querySelector(".preview-tail"), null);
      assert.equal(
        row.querySelector(".preview-head")?.textContent,
        "x".repeat(100),
      );
    });

    it(`shows the over-limit reason and its steps in the selection row in ${locale}`, () => {
      const view = collectedView(collectedInput("a".repeat(maxBodyBytes)));
      assert.ok(view.selection.status.kind === "blocked");
      const elements = popupElements();
      renderPopup(elements, view, locale);
      const [row] = checkRows(elements);
      assert.ok(row !== undefined);
      assert.equal(row.classList.contains("check-row-warning"), true);
      const reason = catalog.preflight["body-too-large"];
      expectMessage(row, reason);
      assert.deepEqual(
        Array.from(
          row.querySelectorAll("ol.message-steps > li"),
          (item) => item.textContent,
        ),
        [...reason.steps],
      );
    });

    it(`shows the full description's wording and count with the ok state in ${locale}`, () => {
      // "A description" is 13 code points; the count is the view's, not the
      // text's.
      const elements = popupElements();
      renderPopup(
        elements,
        collectedView(collectedInput(), { description: "A description" }),
        locale,
      );
      const description = checkRows(elements)[1];
      assert.ok(description !== undefined);
      assert.equal(
        description.querySelector(".message-text")?.textContent,
        catalog.descriptionFull(13),
      );
      assert.equal(description.classList.contains("check-row-warning"), false);
      assert.equal(
        description.querySelector(".check-state")?.textContent,
        catalog.checkStates.ok,
      );
    });

    it(`shows the collapsed description's prompt and steps in its row in ${locale}`, () => {
      const elements = popupElements();
      renderPopup(
        elements,
        collectedView(collectedInput(), { description: "A description" }, true),
        locale,
      );
      const description = checkRows(elements)[1];
      assert.ok(description !== undefined);
      expectLocaleMessage(description, locale, expandPromptMessage);
      assert.deepEqual(
        Array.from(
          description.querySelectorAll("ol.message-steps > li"),
          (item) => item.textContent,
        ),
        [...expandPromptMessage(locale).steps],
      );
      // The collapsed row shows no count: exactly the prompt and its steps.
      assert.deepEqual(
        Array.from(
          description.querySelectorAll(".message-text, .message-steps > li"),
          (node) => node.textContent,
        ),
        [
          expandPromptMessage(locale).text,
          ...expandPromptMessage(locale).steps,
        ],
      );
    });

    it(`shows the not-sent wording without steps with the warning state in ${locale}`, () => {
      const elements = popupElements();
      renderPopup(elements, collectedView(collectedInput()), locale);
      const description = checkRows(elements)[1];
      assert.ok(description !== undefined);
      assert.equal(
        description.querySelector(".message-text")?.textContent,
        catalog.descriptionNotSent.text,
      );
      assert.equal(description.querySelector("ol"), null);
      assert.equal(description.classList.contains("check-row-warning"), true);
      assert.equal(
        description.querySelector(".check-state")?.textContent,
        catalog.checkStates.warning,
      );
    });

    it(`shows the rejection card and no other section in ${locale}`, () => {
      for (const reason of Object.keys(rejectionReasons) as RejectionReason[]) {
        const elements = popupElements();
        renderPopup(elements, { kind: "rejected", reason }, locale);
        assert.equal(elements.rejection.hidden, false, reason);
        assert.equal(elements.action.hidden, false, reason);
        assert.equal(elements.video.hidden, true, reason);
        assert.equal(elements.checks.hidden, true, reason);
        assert.equal(elements.details.hidden, true, reason);
        assert.equal(elements.primary.disabled, true, reason);
        assert.equal(
          elements.primary.textContent,
          catalog.primaryButton,
          reason,
        );
        const message = rejectionMessage(reason, locale);
        expectMessage(elements.rejection, message);
        assert.deepEqual(
          Array.from(
            elements.rejection.querySelectorAll("ol.message-steps > li"),
            (item) => item.textContent,
          ),
          [...message.steps],
          reason,
        );
        const labelledBy = elements.rejection.getAttribute("aria-labelledby");
        assert.equal(
          elements.rejection.querySelector(`#${labelledBy}`)?.textContent,
          message.text,
          reason,
        );
      }
    });

    it(`replaces its content on each call in ${locale}`, () => {
      const elements = popupElements();
      const doc = elements.video.ownerDocument;
      const view = collectedView(collectedInput(), { description: "A" });
      renderPopup(elements, view, locale);
      const drawn = doc.querySelectorAll("*").length;
      renderPopup(elements, view, locale);
      assert.equal(doc.querySelectorAll("*").length, drawn);
      const rejection: PopupView = { kind: "rejected", reason: "empty-title" };
      renderPopup(elements, rejection, locale);
      assert.equal(elements.video.children.length, 0);
      assert.equal(elements.checks.children.length, 0);
      assert.equal(elements.details.children.length, 0);
    });
  }
});
