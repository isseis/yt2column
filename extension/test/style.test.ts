import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import path from "node:path";
import { describe, it } from "node:test";

import { JSDOM } from "jsdom";

import { extensionDir } from "./helpers/paths.ts";

/** The style rules of static/style.css, parsed by jsdom's CSSOM. */
function styleRules(): CSSStyleRule[] {
  const css = readFileSync(
    path.join(extensionDir, "static", "style.css"),
    "utf8",
  );
  const dom = new JSDOM(
    `<!DOCTYPE html><html><head><style>${css}</style></head><body></body></html>`,
  );
  const sheet = dom.window.document.styleSheets[0];
  assert.ok(sheet !== undefined);
  return [...sheet.cssRules].map((rule) => {
    if (rule instanceof dom.window.CSSStyleRule) {
      return rule;
    }
    assert.fail(
      `style.css holds a rule that is not a style rule: ${rule.cssText}`,
    );
  });
}

/** The font families a rule lists, in order, without their quotes. */
function fontFamilies(rule: CSSStyleRule): string[] {
  return rule.style
    .getPropertyValue("font-family")
    .split(",")
    .map((family) => family.trim().replace(/^"|"$/g, ""));
}

/** The one rule whose selector text is selector. */
function ruleFor(rules: CSSStyleRule[], selector: string): CSSStyleRule {
  const matching = rules.filter((rule) => rule.selectorText === selector);
  assert.equal(matching.length, 1, selector);
  const [rule] = matching;
  assert.ok(rule !== undefined);
  return rule;
}

describe("Japanese typography", () => {
  const rules = styleRules();

  it("puts the Japanese font candidates first on a Japanese page", () => {
    assert.deepEqual(fontFamilies(ruleFor(rules, ":root:lang(ja)")), [
      "Hiragino Sans",
      "Noto Sans JP",
      "Yu Gothic UI",
      "system-ui",
      "sans-serif",
    ]);
  });

  it("gives a Japanese page an explicit line height", () => {
    assert.equal(
      ruleFor(rules, "body:lang(ja)").style.getPropertyValue("line-height"),
      "1.7",
    );
  });

  it("applies the Japanese rules to the root and body only", () => {
    const selectors = rules
      .map((rule) => rule.selectorText)
      .filter((selector) => selector.includes(":lang(ja)"));
    assert.deepEqual(selectors, [":root:lang(ja)", "body:lang(ja)"]);
  });

  it("keeps the English font candidates", () => {
    assert.deepEqual(fontFamilies(ruleFor(rules, ":root")), [
      "system-ui",
      "sans-serif",
    ]);
  });

  it("sets a line height only on the Japanese body", () => {
    const withLineHeight = rules
      .filter((rule) => rule.style.getPropertyValue("line-height") !== "")
      .map((rule) => rule.selectorText);
    assert.deepEqual(withLineHeight, ["body:lang(ja)"]);
  });
});
