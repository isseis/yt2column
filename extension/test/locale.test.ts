import assert from "node:assert/strict";
import { describe, it } from "node:test";

import { JSDOM } from "jsdom";

import {
  formatCount,
  formatDuration,
  formatUsd,
  locales,
  resolveLocale,
} from "../src/core/i18n/locale.ts";
import type { UsdAmount } from "../src/core/response.ts";
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

describe("formatDuration", () => {
  const cases: { ms: number; want: Record<Locale, string> }[] = [
    { ms: 0, want: { en: "0 sec", ja: "0 秒" } },
    // Whole seconds: the fraction is dropped, not rounded up.
    { ms: 59_999, want: { en: "59 sec", ja: "59 秒" } },
    { ms: 60_000, want: { en: "1 min 0 sec", ja: "1 分 0 秒" } },
    { ms: 65_000, want: { en: "1 min 5 sec", ja: "1 分 5 秒" } },
    // Hours are written as minutes.
    { ms: 3_725_000, want: { en: "62 min 5 sec", ja: "62 分 5 秒" } },
    // A clock set back shows zero rather than a negative time.
    { ms: -1_000, want: { en: "0 sec", ja: "0 秒" } },
  ];
  for (const { ms, want } of cases) {
    for (const locale of Object.keys(locales) as Locale[]) {
      it(`formats ${ms} ms in ${locale}`, () => {
        assert.equal(formatDuration(ms, locale), want[locale]);
      });
    }
  }
});

describe("formatUsd", () => {
  const cases: { usd: string; want: string }[] = [
    { usd: "0.1234", want: "$0.1234" },
    { usd: "0.0000", want: "$0.0000" },
    // Six integer digits, the most the response check accepts, with grouping.
    { usd: "123456.1234", want: "$123,456.1234" },
  ];
  for (const { usd, want } of cases) {
    for (const locale of Object.keys(locales) as Locale[]) {
      it(`formats ${usd} in ${locale} with the server's digits`, () => {
        assert.equal(formatUsd(usd as UsdAmount, locale), want);
      });
    }
  }
});
