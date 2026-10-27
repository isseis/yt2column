import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import path from "node:path";
import { describe, it } from "node:test";

import { JSDOM } from "jsdom";

import { extensionDir } from "./helpers/paths.ts";

/** A rule that groups other rules, such as @media, @supports or @layer. */
interface GroupingRule extends CSSRule {
  readonly cssRules: CSSRuleList;
}

/** True when the rule groups other rules rather than declaring styles. */
function isGroupingRule(rule: CSSRule): rule is GroupingRule {
  return "cssRules" in rule;
}

/**
 * The style rules of static/style.css, parsed by jsdom's CSSOM. The top-level
 * rules are the ones that apply unconditionally; all of them include the rules
 * of conditional and layered blocks, so a rule there cannot escape the checks.
 */
function parseStyleSheet(): {
  topLevel: CSSStyleRule[];
  all: CSSStyleRule[];
} {
  const css = readFileSync(
    path.join(extensionDir, "static", "style.css"),
    "utf8",
  );
  const dom = new JSDOM(
    `<!DOCTYPE html><html><head><style>${css}</style></head><body></body></html>`,
  );
  const sheet = dom.window.document.styleSheets[0];
  assert.ok(sheet !== undefined);
  const all: CSSStyleRule[] = [];
  const collect = (list: CSSRuleList): void => {
    for (const rule of list) {
      if (rule instanceof dom.window.CSSStyleRule) {
        all.push(rule);
      } else if (isGroupingRule(rule)) {
        collect(rule.cssRules);
      }
    }
  };
  collect(sheet.cssRules);
  const topLevel = [...sheet.cssRules].filter(
    (rule): rule is CSSStyleRule => rule instanceof dom.window.CSSStyleRule,
  );
  return { topLevel, all };
}

/**
 * The last declaration of property among the rules whose selector text is
 * exactly selector, in document order; "" when none declares it. This is the
 * value the sheet gives an element matched by that selector at the top level.
 */
function lastDeclaredValue(
  rules: readonly CSSStyleRule[],
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
  const { topLevel, all } = parseStyleSheet();

  it("puts the Japanese font candidates first on a Japanese page", () => {
    assert.deepEqual(
      fontFamilies(
        lastDeclaredValue(topLevel, ":root:lang(ja)", "font-family"),
      ),
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
    assert.equal(
      lastDeclaredValue(topLevel, "body:lang(ja)", "line-height"),
      "1.7",
    );
  });

  it("applies the Japanese rules to the root and body only", () => {
    // The set, not the order: a second block for the same selector is fine,
    // while any other selector -- including one inside an at-rule -- is not.
    const selectors = new Set(
      all
        .map((rule) => rule.selectorText)
        .filter((selector) => selector.includes(":lang(ja)")),
    );
    assert.deepEqual([...selectors].sort(), [
      ":root:lang(ja)",
      "body:lang(ja)",
    ]);
  });

  it("keeps the English font candidates", () => {
    assert.deepEqual(
      fontFamilies(lastDeclaredValue(topLevel, ":root", "font-family")),
      ["system-ui", "sans-serif"],
    );
  });

  it("sets a line height only on the Japanese body", () => {
    const selectors = new Set(
      all
        .filter((rule) => rule.style.getPropertyValue("line-height") !== "")
        .map((rule) => rule.selectorText),
    );
    assert.deepEqual([...selectors], ["body:lang(ja)"]);
  });
});
