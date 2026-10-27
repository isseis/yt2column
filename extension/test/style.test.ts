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
 * True when one media query can apply to the popup's screen. jsdom does not
 * evaluate media queries, so this reads the medium the query names: a
 * print-only or negated-screen query cannot reach the screen, and every other
 * query can.
 */
function queryAppliesToScreen(query: string): boolean {
  let text = query.trim().toLowerCase();
  if (text.startsWith("only ")) {
    text = text.slice("only ".length).trim();
  }
  return !(
    text === "print" ||
    text.startsWith("print ") ||
    text === "not screen" ||
    text.startsWith("not screen ")
  );
}

/** True when any query of a media query list can apply to the popup's screen. */
function appliesToScreen(mediaText: string): boolean {
  return mediaText.split(",").some(queryAppliesToScreen);
}

/**
 * The style rules of static/style.css, parsed by jsdom's CSSOM. `all` includes
 * the rules of conditional and layered blocks, so no rule can escape the
 * selector checks; `onScreen` drops the print-only blocks, so the value checks
 * read what the popup displays. A layered block applies, so its rules stay.
 */
function parseStyleSheet(): {
  all: CSSStyleRule[];
  onScreen: CSSStyleRule[];
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
  const onScreen: CSSStyleRule[] = [];
  const collect = (list: CSSRuleList, screen: boolean): void => {
    for (const rule of list) {
      if (rule instanceof dom.window.CSSStyleRule) {
        all.push(rule);
        if (screen) {
          onScreen.push(rule);
        }
      } else if (rule instanceof dom.window.CSSMediaRule) {
        collect(rule.cssRules, screen && appliesToScreen(rule.media.mediaText));
      } else if (isGroupingRule(rule)) {
        collect(rule.cssRules, screen);
      }
    }
  };
  collect(sheet.cssRules, true);
  return { all, onScreen };
}

/**
 * The declaration of property that wins for the selector: among the rules
 * whose selector text is exactly selector, the last one marked !important if
 * any, otherwise the last one in document order; "" when none declares it.
 * Specificity is not weighed: the guarded declarations use this selector
 * alone.
 */
function effectiveValue(
  rules: readonly CSSStyleRule[],
  selector: string,
  property: string,
): string {
  let normal = "";
  let important = "";
  for (const rule of rules) {
    if (rule.selectorText !== selector) {
      continue;
    }
    const declared = rule.style.getPropertyValue(property);
    if (declared === "") {
      continue;
    }
    if (rule.style.getPropertyPriority(property) === "important") {
      important = declared;
    } else {
      normal = declared;
    }
  }
  return important === "" ? normal : important;
}

/** The font families a font-family value lists, in order, without quotes. */
function fontFamilies(value: string): string[] {
  return value.split(",").map((family) => family.trim().replace(/^"|"$/g, ""));
}

describe("Japanese typography", () => {
  const { all, onScreen } = parseStyleSheet();

  it("puts the Japanese font candidates first on a Japanese page", () => {
    assert.deepEqual(
      fontFamilies(effectiveValue(onScreen, ":root:lang(ja)", "font-family")),
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
      effectiveValue(onScreen, "body:lang(ja)", "line-height"),
      "1.7",
    );
  });

  it("applies the Japanese rules to the root and body only", () => {
    // The set, not the order: a second block for the same selector is fine,
    // while any other selector -- including one inside a conditional block --
    // is not.
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
      fontFamilies(effectiveValue(onScreen, ":root", "font-family")),
      ["system-ui", "sans-serif"],
    );
  });

  it("sets a line height only on the Japanese body", () => {
    const selectors = new Set(
      onScreen
        .filter((rule) => rule.style.getPropertyValue("line-height") !== "")
        .map((rule) => rule.selectorText),
    );
    assert.deepEqual([...selectors], ["body:lang(ja)"]);
  });
});
