import assert from "node:assert/strict";
import { describe, it } from "node:test";

import { catalogs } from "../src/core/i18n/catalog.ts";
import { locales } from "../src/core/i18n/locale.ts";
import type { Locale } from "../src/core/i18n/locale.ts";
import { optionsView } from "../src/core/optionsView.ts";
import type {
  OptionsOutcome,
  OptionsResult,
  OptionsTone,
} from "../src/core/optionsView.ts";

const localeList = Object.keys(locales) as Locale[];

// One row per outcome, as the design's table of results gives it: whether
// the fields are emptied (only once the settings were written), and the
// message's look (failure when nothing was saved, warning when the save or
// delete was done but is uncertain or unchecked). A Record over the type, so
// an outcome added without its row fails the type check.
const rows: Readonly<
  Record<OptionsOutcome, { clearFields: boolean; tone: OptionsTone }>
> = {
  saved: { clearFields: true, tone: "plain" },
  saved_unverified: { clearFields: true, tone: "warning" },
  field_rejected: { clearFields: false, tone: "failure" },
  key_rejected: { clearFields: false, tone: "failure" },
  check_unavailable: { clearFields: false, tone: "failure" },
  write_uncertain: { clearFields: false, tone: "warning" },
  deleted: { clearFields: false, tone: "plain" },
  delete_uncertain: { clearFields: false, tone: "warning" },
};

/** The result of outcome, with both fields rejected for field_rejected. */
function resultOf(outcome: OptionsOutcome): OptionsResult {
  return outcome === "field_rejected"
    ? { outcome, fields: ["anthropicApiKey", "anthropicWorkspaceId"] }
    : { outcome };
}

describe("optionsView", () => {
  for (const [outcome, { clearFields, tone }] of Object.entries(rows) as [
    OptionsOutcome,
    { clearFields: boolean; tone: OptionsTone },
  ][]) {
    for (const locale of localeList) {
      it(`shows ${outcome} with its own message in ${locale}`, () => {
        const view = optionsView(resultOf(outcome), locale);
        assert.equal(view.message, catalogs[locale].options[outcome]);
        assert.equal(view.clearFields, clearFields);
        assert.equal(view.tone, tone);
        assert.deepEqual(
          view.rejectedFields,
          outcome === "field_rejected"
            ? ["anthropicApiKey", "anthropicWorkspaceId"]
            : [],
        );
      });
    }
  }

  it("marks only the fields the check did not accept", () => {
    assert.deepEqual(
      optionsView(
        { outcome: "field_rejected", fields: ["anthropicWorkspaceId"] },
        "en",
      ).rejectedFields,
      ["anthropicWorkspaceId"],
    );
  });

  it("gives every outcome a distinct message in each locale", () => {
    for (const locale of localeList) {
      const texts = Object.values(catalogs[locale].options).map(
        (message) => message.text,
      );
      assert.equal(new Set(texts).size, texts.length, locale);
    }
  });
});
