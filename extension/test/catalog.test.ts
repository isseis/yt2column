import assert from "node:assert/strict";
import { describe, it } from "node:test";

import { catalogs } from "../src/core/i18n/catalog.ts";
import type { Catalog, Message } from "../src/core/i18n/catalog.ts";
import { locales } from "../src/core/i18n/locale.ts";
import type { Locale } from "../src/core/i18n/locale.ts";

const localeList = Object.keys(locales) as Locale[];

interface Entries {
  /** Every Message in the catalog, by its path such as "rejection.empty-title". */
  readonly messages: Map<string, Message>;
  /** Every label string, by its path such as "details.summary". */
  readonly labels: Map<string, string>;
  /** Every function item, by its path, wrapped to take the row's numbers. */
  readonly counts: Map<string, (count: number, second: number) => string>;
}

function isMessage(value: unknown): value is Message {
  return (
    typeof value === "object" &&
    value !== null &&
    "text" in value &&
    "steps" in value
  );
}

/** Walks the whole catalog, so an item added to Catalog is checked without listing it here. */
function entries(catalog: Catalog): Entries {
  const messages = new Map<string, Message>();
  const labels = new Map<string, string>();
  const counts = new Map<string, (count: number, second: number) => string>();
  const walk = (value: unknown, path: string): void => {
    if (typeof value === "string") {
      labels.set(path, value);
    } else if (isMessage(value)) {
      messages.set(path, value);
    } else if (typeof value === "function") {
      // A function item embeds numbers into its text; its declared parameter
      // count says whether it takes the second count as well.
      const call = value as (...args: number[]) => unknown;
      counts.set(path, (count, second) =>
        String(call.length > 1 ? call(count, second) : call(count)),
      );
    } else {
      assert.ok(typeof value === "object" && value !== null, path);
      for (const [key, child] of Object.entries(value)) {
        walk(child, path === "" ? key : `${path}.${key}`);
      }
    }
  };
  walk(catalog, "");
  return { messages, labels, counts };
}

describe("catalogs", () => {
  for (const locale of localeList) {
    it(`gives every message, label and count a non-empty text in ${locale}`, () => {
      const { messages, labels, counts } = entries(catalogs[locale]);
      // Precondition: the walk reached the messages, the labels and the counts.
      assert.ok(messages.size > 0);
      assert.ok(labels.size > 0);
      assert.ok(counts.size > 0);
      for (const [path, message] of messages) {
        assert.notEqual(message.text.trim(), "", path);
        for (const step of message.steps) {
          assert.notEqual(step.trim(), "", path);
        }
      }
      for (const [path, label] of labels) {
        assert.notEqual(label.trim(), "", path);
      }
      for (const [path, count] of counts) {
        // One and more than one, as the singular form is chosen by the number.
        for (const [first, second] of [
          [1, 1],
          [12345, 420],
        ]) {
          assert.notEqual(count(first ?? 0, second ?? 0).trim(), "", path);
        }
      }
    });
  }

  it("uses the English singular form for a count of one", () => {
    assert.equal(catalogs.en.selectionSize(1, 1), "1 character \u00b7 1 line");
    assert.equal(
      catalogs.en.selectionSize(2, 2),
      "2 characters \u00b7 2 lines",
    );
    assert.equal(
      catalogs.en.descriptionFull(1),
      "Full text \u00b7 1 character",
    );
    assert.equal(
      catalogs.en.descriptionFull(2),
      "Full text \u00b7 2 characters",
    );
  });

  // The YouTube button's name on YouTube, per the extension's language.
  const youtubeButton: Record<Locale, string> = {
    en: '"...more"',
    ja: "もっと見る",
  };
  for (const locale of localeList) {
    it(`names the YouTube button in the ${locale} expand prompt steps`, () => {
      const steps = catalogs[locale].expandPrompt.steps;
      assert.ok(
        steps.some((step) => step.includes(youtubeButton[locale])),
        youtubeButton[locale],
      );
    });
  }

  it("gives a Japanese message steps only where the English one has them", () => {
    const en = entries(catalogs.en).messages;
    const ja = entries(catalogs.ja).messages;
    // Precondition: the messages pair up by path.
    assert.deepEqual([...ja.keys()].sort(), [...en.keys()].sort());
    for (const [path, message] of ja) {
      if (message.steps.length > 0) {
        assert.ok((en.get(path)?.steps.length ?? 0) > 0, path);
      }
    }
  });

  it("names the 4 MiB limit for body-too-large in every locale", () => {
    for (const locale of localeList) {
      const message = catalogs[locale].preflight["body-too-large"];
      assert.ok(
        [message.text, ...message.steps].join("\n").includes("4 MiB"),
        locale,
      );
    }
  });
});
