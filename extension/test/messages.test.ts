import assert from "node:assert/strict";
import { describe, it } from "node:test";

import { catalogs } from "../src/core/i18n/catalog.ts";
import { locales } from "../src/core/i18n/locale.ts";
import type { Locale } from "../src/core/i18n/locale.ts";
import { expandPromptMessage, rejectionMessage } from "../src/core/messages.ts";
import { rejectionReasons } from "../src/core/types.ts";
import type { RejectionReason } from "../src/core/types.ts";

const reasons = Object.keys(rejectionReasons) as RejectionReason[];
const localeList = Object.keys(locales) as Locale[];

describe("rejectionMessage", () => {
  for (const locale of localeList) {
    it(`gives every reason a distinct non-empty text in ${locale}`, () => {
      const texts = reasons.map(
        (reason) => rejectionMessage(reason, locale).text,
      );
      for (const text of texts) {
        assert.notEqual(text, "");
      }
      assert.equal(new Set(texts).size, reasons.length);
    });

    it(`gives steps only to not-watch-page and empty-selection in ${locale}`, () => {
      for (const reason of reasons) {
        const hasSteps = rejectionMessage(reason, locale).steps.length > 0;
        const wantsSteps =
          reason === "not-watch-page" || reason === "empty-selection";
        assert.equal(hasSteps, wantsSteps, reason);
      }
    });

    it(`returns the ${locale} catalog's rejection message for every reason`, () => {
      for (const reason of reasons) {
        assert.equal(
          rejectionMessage(reason, locale),
          catalogs[locale].rejection[reason],
          reason,
        );
      }
    });

    it(`gives the watch-page URL example as written in English in ${locale}`, () => {
      const message = rejectionMessage("not-watch-page", locale);
      assert.ok(
        [message.text, ...message.steps]
          .join("\n")
          .includes("https://www.youtube.com/watch?v=..."),
      );
    });
  }
});

describe("expandPromptMessage", () => {
  for (const locale of localeList) {
    it(`returns the ${locale} catalog's expand prompt`, () => {
      assert.equal(expandPromptMessage(locale), catalogs[locale].expandPrompt);
    });
  }
});
