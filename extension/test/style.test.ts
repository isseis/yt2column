import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import path from "node:path";
import { describe, it } from "node:test";

import { JSDOM } from "jsdom";

import { extensionDir } from "./helpers/paths.ts";

/**
 * The style rules of static/style.css, parsed by jsdom's CSSOM, including the
 * rules of a media query. Other at-rules are not a place this guard reads: it
 * checks the declarations the page uses in every state.
 */
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
  const rules: CSSStyleRule[] = [];
  const collect = (list: CSSRuleList): void => {
    for (const rule of list) {
      if (rule instanceof dom.window.CSSStyleRule) {
        rules.push(rule);
      } else if (rule instanceof dom.window.CSSMediaRule) {
        collect(rule.cssRules);
      }
    }
  };
  collect(sheet.cssRules);
  return rules;
}

/** The value the cascade gives property for selector, or "" when unset. */
function declaredValue(
  rules: CSSStyleRule[],
  selector: string,
  property: string,
): string {
  let value = "";
  for (const rule of rules) {
    if (rule.selectorText !== selector) {
      continue;
    }
    const declared = rule.style.getPropertyValue(property);
    if (declared !== "") {
      value = declared;
    }
  }
  return value;
}

/** The font families a font-family value lists, in order, without quotes. */
function fontFamilies(value: string): string[] {
  return value.split(",").map((family) => family.trim().replace(/^"|"$/g, ""));
}

describe("Japanese typography", () => {
  const rules = styleRules();

  it("puts the Japanese font candidates first on a Japanese page", () => {
    assert.deepEqual(
      fontFamilies(declaredValue(rules, ":root:lang(ja)", "font-family")),
      [
        "Hiragino Sans",
        "Noto Sans JP",
        "Yu Gothic UI",
        "system-ui",
        "sans-serif",
      ],
    );
  });

  it("gives a Japanese page an explicit line height", () => {
    assert.equal(declaredValue(rules, "body:lang(ja)", "line-height"), "1.7");
  });

  it("applies the Japanese rules to the root and body only", () => {
    const selectors = rules
      .map((rule) => rule.selectorText)
      .filter((selector) => selector.includes(":lang(ja)"));
    assert.deepEqual(selectors, [":root:lang(ja)", "body:lang(ja)"]);
  });

  it("keeps the English font candidates", () => {
    assert.deepEqual(
      fontFamilies(declaredValue(rules, ":root", "font-family")),
      ["system-ui", "sans-serif"],
    );
  });

  it("sets a line height only on the Japanese body", () => {
    const withLineHeight = rules
      .filter((rule) => rule.style.getPropertyValue("line-height") !== "")
      .map((rule) => rule.selectorText);
    assert.deepEqual(withLineHeight, ["body:lang(ja)"]);
  });
});
