import assert from "node:assert/strict";
import { describe, it } from "node:test";

import { JSDOM } from "jsdom";

import {
  formatCount,
  locales,
  resolveLocale,
} from "../src/core/i18n/locale.ts";
import type { Locale } from "../src/core/i18n/locale.ts";
import { applyPageLocale } from "../src/ui/pageLocale.ts";

describe("resolveLocale", () => {
  const cases: { uiLanguage: string | undefined; want: Locale }[] = [
    { uiLanguage: "ja", want: "ja" },
    { uiLanguage: "ja-JP", want: "ja" },
    { uiLanguage: "JA", want: "ja" },
    { uiLanguage: "en", want: "en" },
    { uiLanguage: "en-US", want: "en" },
    { uiLanguage: "fr", want: "en" },
    { uiLanguage: "zh-CN", want: "en" },
    // A longer or different primary subtag that starts like "ja" is not Japanese.
    { uiLanguage: "jam", want: "en" },
    { uiLanguage: "jv", want: "en" },
    // Only "-" separates the subtag.
    { uiLanguage: "ja_JP", want: "en" },
    { uiLanguage: "", want: "en" },
    { uiLanguage: undefined, want: "en" },
  ];
  for (const { uiLanguage, want } of cases) {
    it(`resolves ${JSON.stringify(uiLanguage)} to ${want}`, () => {
      assert.equal(resolveLocale(uiLanguage), want);
    });
  }
});

describe("formatCount", () => {
  it("groups the digits of a count in both locales", () => {
    // Precondition: the runtime has ICU data for both locales, so a mismatch
    // below is a formatting failure, not missing locale data.
    assert.deepEqual(Intl.NumberFormat.supportedLocalesOf(["en", "ja"]), [
      "en",
      "ja",
    ]);
    assert.equal(formatCount(12345, "en"), "12,345");
    assert.equal(formatCount(12345, "ja"), "12,345");
  });
});

describe("applyPageLocale", () => {
  for (const locale of Object.keys(locales) as Locale[]) {
    it(`sets the html element's lang attribute to ${locale}`, () => {
      const document = new JSDOM("<!DOCTYPE html><html><body></body></html>")
        .window.document;
      applyPageLocale(document, locale);
      assert.equal(document.documentElement.lang, locale);
    });
  }
});
