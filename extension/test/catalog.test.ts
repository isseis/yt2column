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
  /** Every label string, by its path such as "summaryLabels.url". */
  readonly labels: Map<string, string>;
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
  const walk = (value: unknown, path: string): void => {
    if (typeof value === "string") {
      labels.set(path, value);
    } else if (isMessage(value)) {
      messages.set(path, value);
    } else {
      assert.ok(typeof value === "object" && value !== null, path);
      for (const [key, child] of Object.entries(value)) {
        walk(child, path === "" ? key : `${path}.${key}`);
      }
    }
  };
  walk(catalog, "");
  return { messages, labels };
}

describe("catalogs", () => {
  for (const locale of localeList) {
    it(`gives every message and label a non-empty text in ${locale}`, () => {
      const { messages, labels } = entries(catalogs[locale]);
      // Precondition: the walk reached the messages and the labels.
      assert.ok(messages.size > 0);
      assert.ok(labels.size > 0);
      for (const [path, message] of messages) {
        assert.notEqual(message.text.trim(), "", path);
        for (const step of message.steps) {
          assert.notEqual(step.trim(), "", path);
        }
      }
      for (const [path, label] of labels) {
        assert.notEqual(label.trim(), "", path);
      }
    });
  }

  it("gives a Japanese message steps only where the English one has them", () => {
    const en = entries(catalogs.en).messages;
    const ja = entries(catalogs.ja).messages;
    assert.deepEqual([...ja.keys()], [...en.keys()]);
    for (const [path, message] of ja) {
      if (message.steps.length > 0) {
        assert.ok((en.get(path)?.steps.length ?? 0) > 0, path);
      }
    }
  });
});
