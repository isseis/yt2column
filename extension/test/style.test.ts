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

/** A style rule and the media query lists of the blocks that contain it. */
interface ParsedRule {
  readonly rule: CSSStyleRule;
  /** The media query text of each enclosing block, outermost first. */
  readonly media: readonly string[];
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
 * Each rule keeps the media query list of the blocks around it, so the light
 * and dark declarations can be read apart.
 */
function parseStyleSheet(): {
  all: ParsedRule[];
  onScreen: ParsedRule[];
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
  const all: ParsedRule[] = [];
  const onScreen: ParsedRule[] = [];
  const collect = (
    list: CSSRuleList,
    screen: boolean,
    media: readonly string[],
  ): void => {
    for (const rule of list) {
      if (rule instanceof dom.window.CSSStyleRule) {
        const parsed = { rule, media };
        all.push(parsed);
        if (screen) {
          onScreen.push(parsed);
        }
      } else if (rule instanceof dom.window.CSSMediaRule) {
        const text = rule.media.mediaText;
        collect(rule.cssRules, screen && appliesToScreen(text), [
          ...media,
          text,
        ]);
      } else if (isGroupingRule(rule)) {
        collect(rule.cssRules, screen, media);
      }
    }
  };
  collect(sheet.cssRules, true, []);
  return { all, onScreen };
}

/** The selectors a rule applies to, split from its selector list. */
function selectorsOf({ rule }: ParsedRule): string[] {
  return rule.selectorText.split(",").map((selector) => selector.trim());
}

/** The rules whose selector list includes selector. */
function rulesFor(
  rules: readonly ParsedRule[],
  selector: string,
): ParsedRule[] {
  return rules.filter((entry) => selectorsOf(entry).includes(selector));
}

/**
 * The declaration of property that wins for the selector: among the matching
 * rules, the last one marked !important if any, otherwise the last one in
 * document order; "" when none declares it. Specificity is not weighed: the
 * guarded declarations name their selector exactly.
 */
function effectiveValue(
  rules: readonly ParsedRule[],
  selector: string,
  property: string,
): string {
  let normal = "";
  let important = "";
  for (const { rule } of rulesFor(rules, selector)) {
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

/** The !important declaration of property for the selector, or "". */
function importantValue(
  rules: readonly ParsedRule[],
  selector: string,
  property: string,
): string {
  for (const { rule } of rulesFor(rules, selector)) {
    if (
      rule.style.getPropertyPriority(property) === "important" &&
      rule.style.getPropertyValue(property) !== ""
    ) {
      return rule.style.getPropertyValue(property);
    }
  }
  return "";
}

/** True when a rule for selector gives property a value that names the variable. */
function references(
  rules: readonly ParsedRule[],
  selector: string,
  property: string,
  variable: string,
): boolean {
  return rulesFor(rules, selector).some(({ rule }) =>
    rule.style.getPropertyValue(property).includes(`var(${variable})`),
  );
}

/** True when the rule sits in a prefers-color-scheme: dark block. */
function isDark({ media }: ParsedRule): boolean {
  return media.some(
    (query) =>
      query.toLowerCase().replace(/\s+/g, "") === "(prefers-color-scheme:dark)",
  );
}

/** The last value the :root rules give the colour variable, or "". */
function variable(rules: readonly ParsedRule[], name: string): string {
  let value = "";
  for (const entry of rulesFor(rules, ":root")) {
    const declared = entry.rule.style.getPropertyValue(name);
    if (declared !== "") {
      value = declared;
    }
  }
  return value;
}

/** One sRGB channel of a #rrggbb colour, from 0 to 1. */
function channel(colour: string, start: number): number {
  return parseInt(colour.slice(start, start + 2), 16) / 255;
}

/** The WCAG 2.2 linear form of one sRGB channel. */
function linear(value: number): number {
  return value <= 0.03928 ? value / 12.92 : ((value + 0.055) / 1.055) ** 2.4;
}

/** The relative luminance WCAG 2.2 defines for a #rrggbb colour. */
function luminance(colour: string): number {
  return (
    0.2126 * linear(channel(colour, 1)) +
    0.7152 * linear(channel(colour, 3)) +
    0.0722 * linear(channel(colour, 5))
  );
}

/** The contrast ratio WCAG 2.2 defines for two #rrggbb colours. */
function contrast(first: string, second: string): number {
  const lighter = Math.max(luminance(first), luminance(second));
  const darker = Math.min(luminance(first), luminance(second));
  return (lighter + 0.05) / (darker + 0.05);
}

/** True when a min-height value is at least 44 CSS pixels. */
function atLeast44(value: string): boolean {
  const pixels = /^(\d+(?:\.\d+)?)px$/.exec(value);
  return pixels !== null && Number(pixels[1]) >= 44;
}

/** The font families a font-family value lists, in order, without quotes. */
function fontFamilies(value: string): string[] {
  return value.split(",").map((family) => family.trim().replace(/^"|"$/g, ""));
}

/**
 * Every colour pair the design requires a contrast for, with the rules that
 * paint the foreground and the background from those variables. The ratio is
 * checked in both themes and the references in the rules, so a table entry
 * cannot pass while the stylesheet uses another colour.
 */
const contrastPairs = [
  {
    foreground: "--text",
    background: "--bg",
    minimum: 4.5,
    foregroundRules: ["body"],
    foregroundProperty: "color",
    backgroundRules: ["body"],
  },
  {
    foreground: "--muted",
    background: "--bg",
    minimum: 4.5,
    foregroundRules: [".channel-name"],
    foregroundProperty: "color",
    backgroundRules: ["body"],
  },
  {
    foreground: "--text",
    background: "--warn-bg",
    minimum: 4.5,
    foregroundRules: [".check-row"],
    foregroundProperty: "color",
    backgroundRules: [".check-row-warning"],
  },
  {
    foreground: "--muted",
    background: "--warn-bg",
    minimum: 4.5,
    foregroundRules: [".check-size"],
    foregroundProperty: "color",
    backgroundRules: [".check-row-warning"],
  },
  {
    foreground: "--warn-text",
    background: "--warn-bg",
    minimum: 4.5,
    foregroundRules: [
      ".check-row-warning .message-text",
      ".check-row-warning .message-steps",
    ],
    foregroundProperty: "color",
    backgroundRules: [".check-row-warning"],
  },
  {
    foreground: "--primary-text",
    background: "--primary-bg",
    minimum: 4.5,
    foregroundRules: ["#primary"],
    foregroundProperty: "color",
    backgroundRules: ["#primary"],
  },
  {
    foreground: "--ok",
    background: "--bg",
    minimum: 3,
    foregroundRules: [".check-icon-ok"],
    foregroundProperty: "color",
    backgroundRules: ["body"],
  },
  {
    foreground: "--warn",
    background: "--warn-bg",
    minimum: 3,
    foregroundRules: [".check-icon-warning"],
    foregroundProperty: "color",
    backgroundRules: [".check-row-warning"],
  },
  {
    foreground: "--focus",
    background: "--bg",
    minimum: 3,
    foregroundRules: [":focus-visible"],
    foregroundProperty: "outline",
    backgroundRules: ["body"],
  },
];

describe("colour variables", () => {
  const { onScreen } = parseStyleSheet();
  const light = onScreen.filter((entry) => !isDark(entry));
  const dark = onScreen.filter(isDark);

  const names = [
    "--bg",
    "--text",
    "--muted",
    "--border",
    "--warn-bg",
    "--warn-text",
    "--ok",
    "--warn",
    "--primary-bg",
    "--primary-text",
    "--disabled-bg",
    "--disabled-text",
    "--focus",
  ];

  for (const [theme, rules] of [
    ["light", light],
    ["dark", dark],
  ] as const) {
    it(`declares every colour variable in the ${theme} theme`, () => {
      for (const name of names) {
        assert.match(variable(rules, name), /^#[0-9a-f]{6}$/, name);
      }
    });

    it(`keeps every pair at its minimum contrast in the ${theme} theme`, () => {
      for (const pair of contrastPairs) {
        const ratio = contrast(
          variable(rules, pair.foreground),
          variable(rules, pair.background),
        );
        assert.ok(
          ratio >= pair.minimum,
          `${pair.foreground} on ${pair.background}: ${ratio.toFixed(2)}`,
        );
      }
    });

    it(`gives the disabled primary button its own colours in the ${theme} theme`, () => {
      const disabled = rulesFor(onScreen, "#primary:disabled");
      assert.ok(
        disabled.some(({ rule }) =>
          rule.style
            .getPropertyValue("background-color")
            .includes("var(--disabled-bg)"),
        ),
      );
      assert.ok(
        disabled.some(({ rule }) =>
          rule.style.getPropertyValue("color").includes("var(--disabled-text)"),
        ),
      );
    });
  }

  it("paints every pair with the pair's own variables", () => {
    for (const pair of contrastPairs) {
      for (const selector of pair.foregroundRules) {
        assert.ok(
          references(
            onScreen,
            selector,
            pair.foregroundProperty,
            pair.foreground,
          ),
          `${selector} ${pair.foregroundProperty} ${pair.foreground}`,
        );
      }
      for (const selector of pair.backgroundRules) {
        assert.ok(
          references(onScreen, selector, "background-color", pair.background),
          `${selector} background-color ${pair.background}`,
        );
      }
    }
  });

  it("gives only the warning row the warning background", () => {
    assert.equal(
      effectiveValue(onScreen, ".check-row", "background-color"),
      "",
    );
    assert.ok(
      references(
        onScreen,
        ".check-row-warning",
        "background-color",
        "--warn-bg",
      ),
    );
  });

  it("separates the checks and the rejection card with a border", () => {
    for (const selector of ["#checks", "#rejection"]) {
      assert.ok(
        effectiveValue(onScreen, selector, "border").includes("var(--border)"),
        selector,
      );
    }
  });

  it("takes every colour from a variable", () => {
    // A literal would pass a contrast check of the variable it shadows while
    // the page shows something else.
    const literal = /#[0-9a-f]{3,8}\b|rgba?\(|hsla?\(/i;
    const properties = [
      "color",
      "background-color",
      "border-color",
      "border-top-color",
      "border-bottom-color",
      "outline-color",
      "text-decoration-color",
      "fill",
      "stroke",
      "border",
      "border-top",
      "outline",
      "background",
    ];
    for (const entry of onScreen) {
      if (selectorsOf(entry).includes(":root")) {
        continue;
      }
      for (const property of properties) {
        const value = entry.rule.style.getPropertyValue(property);
        assert.ok(
          !literal.test(value),
          `${entry.rule.selectorText} ${property}: ${value}`,
        );
      }
    }
  });

  it("declares the dark variables after the light ones", () => {
    // The dark block only overrides: placed before the light one, the light
    // values would win in both themes.
    const roots = onScreen.filter((entry) =>
      selectorsOf(entry).includes(":root"),
    );
    const lightIndex = roots.findIndex((entry) => !isDark(entry));
    const darkIndex = roots.findIndex(isDark);
    assert.ok(lightIndex >= 0, "no light :root");
    assert.ok(darkIndex > lightIndex, "the dark :root comes first");
  });
});

describe("layout", () => {
  const { onScreen } = parseStyleSheet();

  it("hides a [hidden] element whatever display rule it has", () => {
    assert.equal(effectiveValue(onScreen, "[hidden]", "display"), "none");
    assert.equal(importantValue(onScreen, "[hidden]", "display"), "none");
  });

  it("fixes the body's width at 360 CSS pixels", () => {
    assert.equal(effectiveValue(onScreen, "body", "width"), "360px");
    assert.equal(effectiveValue(onScreen, "body", "min-width"), "");
  });

  it("fills the action section with the primary button and keeps it 44px tall", () => {
    assert.equal(effectiveValue(onScreen, "#primary", "width"), "100%");
    assert.ok(atLeast44(effectiveValue(onScreen, "#primary", "min-height")));
  });

  it("gives the details toggle the content's width and 44px of height", () => {
    assert.equal(
      effectiveValue(onScreen, "#details > summary", "width"),
      "100%",
    );
    assert.ok(
      atLeast44(effectiveValue(onScreen, "#details > summary", "min-height")),
    );
  });

  it("clamps the heading and the channel name to two lines", () => {
    for (const selector of [".video-heading", ".channel-name"]) {
      assert.equal(
        effectiveValue(onScreen, selector, "-webkit-line-clamp"),
        "2",
        selector,
      );
      assert.equal(
        effectiveValue(onScreen, selector, "display"),
        "-webkit-box",
        selector,
      );
      assert.equal(
        effectiveValue(onScreen, selector, "overflow"),
        "hidden",
        selector,
      );
      // A value with no break opportunity must still wrap, or the clamp never
      // engages and the value is clipped without an ellipsis.
      assert.equal(
        effectiveValue(onScreen, selector, "overflow-wrap"),
        "anywhere",
        selector,
      );
    }
  });

  it("keeps bidirectional control characters inside their own value", () => {
    for (const selector of [
      ".video-heading",
      ".channel-name",
      ".detail-value",
      ".preview-head",
    ]) {
      assert.equal(
        effectiveValue(onScreen, selector, "unicode-bidi"),
        "plaintext",
        selector,
      );
    }
    assert.equal(
      effectiveValue(onScreen, ".preview-tail-value", "unicode-bidi"),
      "isolate",
    );
  });

  it("keeps each preview on one line", () => {
    for (const selector of [".preview-head", ".preview-tail"]) {
      assert.equal(
        effectiveValue(onScreen, selector, "white-space"),
        "pre",
        selector,
      );
      assert.equal(
        effectiveValue(onScreen, selector, "overflow"),
        "hidden",
        selector,
      );
      assert.equal(
        effectiveValue(onScreen, selector, "text-overflow"),
        "ellipsis",
        selector,
      );
    }
  });

  it("shows the tail preview's end by running its container right to left", () => {
    assert.equal(effectiveValue(onScreen, ".preview-tail", "direction"), "rtl");
    const reversed = rulesFor(onScreen, ".preview-tail:has(> :dir(rtl))");
    assert.equal(reversed.length, 1);
    // Alone in its rule: a browser that cannot parse the selector drops only
    // this rule, not a group of them.
    assert.equal(
      reversed[0]?.rule.selectorText,
      ".preview-tail:has(> :dir(rtl))",
    );
    assert.equal(reversed[0]?.rule.style.getPropertyValue("direction"), "ltr");
  });

  it("hides the state word without removing it from the accessibility tree", () => {
    const selector = ".visually-hidden";
    assert.equal(effectiveValue(onScreen, selector, "position"), "absolute");
    assert.equal(effectiveValue(onScreen, selector, "width"), "1px");
    assert.equal(effectiveValue(onScreen, selector, "height"), "1px");
    assert.notEqual(effectiveValue(onScreen, selector, "clip"), "");
    assert.notEqual(effectiveValue(onScreen, selector, "display"), "none");
    assert.notEqual(effectiveValue(onScreen, selector, "visibility"), "hidden");
  });

  it("marks the focused element with an outline outside the page's background", () => {
    assert.ok(references(onScreen, ":focus-visible", "outline", "--focus"));
    assert.equal(
      effectiveValue(onScreen, ":focus-visible", "outline-offset"),
      "2px",
    );
  });
});

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
        .map((entry) => entry.rule.selectorText)
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
        .filter(
          (entry) => entry.rule.style.getPropertyValue("line-height") !== "",
        )
        .map((entry) => entry.rule.selectorText),
    );
    assert.deepEqual([...selectors], ["body:lang(ja)"]);
  });
});
