import assert from "node:assert/strict";
import { describe, it } from "node:test";

import { catalogs } from "../src/core/i18n/catalog.ts";
import { locales } from "../src/core/i18n/locale.ts";
import type { Locale } from "../src/core/i18n/locale.ts";
import { expandPromptMessage } from "../src/core/messages.ts";
import type { MetadataResult } from "../src/core/metadata.ts";
import { metadataDisplayLimit, renderMetadata } from "../src/ui/render.ts";
import {
  container,
  expectAbsent,
  expectLocaleMessage,
  otherLocales,
  valueElement,
  valueOf,
} from "./helpers/dom.ts";

const malicious = "<img src=x onerror=alert(1)>";
const localeList = Object.keys(locales) as Locale[];

/** A metadata result with the given values and prompt flag. */
function result(over: Partial<MetadataResult> = {}): MetadataResult {
  return { values: {}, showExpandPrompt: false, omissions: [], ...over };
}

const allValues = {
  title: "A title",
  channelName: "A channel",
  description: "A description",
};

describe("renderMetadata", () => {
  // The English labels stay literal strings: reading them from the catalog
  // would let a changed English label pass unnoticed.
  it("shows each present value with its label, in field order", () => {
    const root = container();
    renderMetadata(root, result({ values: allValues }), "en");
    assert.equal(valueOf(root, "Video title"), "A title");
    assert.equal(valueOf(root, "Channel"), "A channel");
    assert.equal(valueOf(root, "Description"), "A description");
    assert.deepEqual(
      Array.from(
        root.querySelectorAll(".summary-label"),
        (node) => node.textContent,
      ),
      ["Video title", "Channel", "Description"],
    );
  });

  it("gives each value its class and automatic direction", () => {
    const root = container();
    renderMetadata(
      root,
      result({ values: { title: "t", channelName: "c", description: "d" } }),
      "en",
    );
    const cases = [
      { label: "Video title", className: "metadata-title" },
      { label: "Channel", className: "metadata-channel" },
      { label: "Description", className: "metadata-description" },
    ];
    for (const { label, className } of cases) {
      const value = valueElement(root, label);
      assert.ok(value?.classList.contains(className), label);
      assert.equal(value?.getAttribute("dir"), "auto", label);
    }
  });

  for (const locale of localeList) {
    const labels = catalogs[locale].metadataLabels;

    it(`shows each present value under its ${locale} label, in field order`, () => {
      const root = container();
      renderMetadata(root, result({ values: allValues }), locale);
      assert.equal(valueOf(root, labels.title), "A title");
      assert.equal(valueOf(root, labels.channelName), "A channel");
      assert.equal(valueOf(root, labels.description), "A description");
      assert.deepEqual(
        Array.from(
          root.querySelectorAll(".summary-label"),
          (node) => node.textContent,
        ),
        [labels.title, labels.channelName, labels.description],
      );
      const shown = Object.values(labels);
      for (const other of otherLocales(locale)) {
        expectAbsent(
          root,
          Object.values(catalogs[other].metadataLabels),
          shown,
        );
      }
    });

    it(`shows only the values that are present in ${locale}`, () => {
      const root = container();
      renderMetadata(
        root,
        result({ values: { channelName: "A channel" } }),
        locale,
      );
      assert.equal(valueOf(root, labels.channelName), "A channel");
      assert.equal(valueOf(root, labels.title), undefined);
      assert.equal(valueOf(root, labels.description), undefined);
    });

    it(`displays markup in a value as text, not elements, in ${locale}`, () => {
      const root = container();
      renderMetadata(
        root,
        result({ values: { title: malicious, description: malicious } }),
        locale,
      );
      assert.equal(root.querySelector("img"), null);
      assert.ok(root.textContent?.includes(malicious));
    });

    it(`shows the ${locale} expand prompt only when showExpandPrompt is set`, () => {
      const cases = [
        { name: "collapsed", showExpandPrompt: true, present: true },
        { name: "not collapsed", showExpandPrompt: false, present: false },
      ];
      for (const { name, showExpandPrompt, present } of cases) {
        const root = container();
        renderMetadata(
          root,
          result({ values: { description: "beginning" }, showExpandPrompt }),
          locale,
        );
        const message = root.querySelector(".summary-message");
        assert.equal(message !== null, present, name);
        if (!present) {
          assert.equal(root.querySelector("ol.summary-steps"), null, name);
          continue;
        }
        const prompt = expandPromptMessage(locale);
        assert.equal(message?.textContent, prompt.text, name);
        assert.deepEqual(
          Array.from(
            root.querySelectorAll("ol.summary-steps > li"),
            (item) => item.textContent,
          ),
          prompt.steps,
          name,
        );
        expectLocaleMessage(root, locale, expandPromptMessage);
      }
    });

    it(`does not put page text in the expand prompt in ${locale}`, () => {
      const root = container();
      const description = "A page-only description";
      renderMetadata(
        root,
        result({ values: { description }, showExpandPrompt: true }),
        locale,
      );
      assert.notEqual(root.querySelector(".summary-message"), null);
      const promptText = Array.from(
        root.querySelectorAll(".summary-message, .summary-steps"),
      )
        .map((node) => node.textContent)
        .join("");
      assert.ok(!promptText.includes(description));
      assert.equal(valueOf(root, labels.description), description);
    });
  }

  it("truncates only a value over the display limit", () => {
    const pair = "\u{1F600}";
    const cases = [
      {
        name: "exactly at the limit",
        value: "a".repeat(metadataDisplayLimit),
        shown: "a".repeat(metadataDisplayLimit),
      },
      {
        name: "one over the limit",
        value: "a".repeat(metadataDisplayLimit + 1),
        shown: `${"a".repeat(metadataDisplayLimit)}…`,
      },
      {
        name: "a surrogate pair at the boundary",
        value: `${"a".repeat(metadataDisplayLimit - 1)}${pair}b`,
        shown: `${"a".repeat(metadataDisplayLimit - 1)}${pair}…`,
      },
    ];
    for (const { name, value, shown } of cases) {
      const root = container();
      renderMetadata(root, result({ values: { description: value } }), "en");
      assert.equal(valueOf(root, "Description"), shown, name);
    }
  });

  it("appends after the existing children", () => {
    const root = container();
    const kept = root.ownerDocument.createElement("p");
    kept.textContent = "kept";
    root.append(kept);
    renderMetadata(root, result({ values: { title: "A title" } }), "en");
    assert.equal(root.firstElementChild, kept);
    assert.equal(valueOf(root, "Video title"), "A title");
  });
});
