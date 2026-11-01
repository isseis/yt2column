import assert from "node:assert/strict";
import { describe, it } from "node:test";

import { catalogs } from "../src/core/i18n/catalog.ts";
import type { Catalog, Message } from "../src/core/i18n/catalog.ts";
import { locales } from "../src/core/i18n/locale.ts";
import type { Locale } from "../src/core/i18n/locale.ts";
import { maxBodyBytes } from "../src/core/prepare.ts";
import { failureCodes, startRejectStatus } from "../src/core/response.ts";
import type { ServerRejectReason } from "../src/core/response.ts";

const localeList = Object.keys(locales) as Locale[];

/** The whole message as one string, for content checks of the displayed wording. */
function textOf(message: Message): string {
  return [message.text, ...message.steps].join("\n");
}

// No text mentions an options page, a token or a port. The ja words are
// whole terms: the bare katakana for "port" would also match words such as
// "support" and "report".
// Slack, in either script: the start's messages must name it in neither locale.
const slackWord = /slack|\u30b9\u30e9\u30c3\u30af/iu;

const forbiddenWords: Record<Locale, RegExp> = {
  en: /option|token|\bport\b/iu,
  ja: /オプション|トークン|ポート番号/u,
};

interface Entries {
  /** Every Message in the catalog, by its path such as "rejection.empty-title". */
  readonly messages: Map<string, Message>;
  /** Every label string, by its path such as "details.summary". */
  readonly labels: Map<string, string>;
  /** Every function item, by its path, wrapped to take the row's numbers. */
  readonly counts: Map<string, (count: number, second: number) => unknown>;
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
  const counts = new Map<string, (count: number, second: number) => unknown>();
  const walk = (value: unknown, path: string): void => {
    if (typeof value === "string") {
      labels.set(path, value);
    } else if (isMessage(value)) {
      messages.set(path, value);
    } else if (typeof value === "function") {
      // A function item embeds numbers into its text; both counts are passed
      // and a function that takes one simply ignores the second.
      const call = value as (...args: number[]) => unknown;
      counts.set(path, (count, second) => call(count, second));
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
        const pairs: readonly (readonly [number, number])[] = [
          [1, 1],
          [12345, 420],
        ];
        for (const [first, second] of pairs) {
          const text = count(first, second);
          assert.ok(typeof text === "string", path);
          assert.notEqual(text.trim(), "", path);
        }
      }
    });
  }

  it("formats the counts exactly as the wording table writes them", () => {
    const cases: {
      name: string;
      actual: (catalog: Catalog) => string;
      expected: Record<Locale, string>;
    }[] = [
      {
        name: "a long selection",
        actual: (catalog) => catalog.selectionSize(12345, 420),
        expected: {
          en: "12,345 characters \u00b7 420 lines",
          ja: "12,345 文字・420 行",
        },
      },
      {
        name: "a long description",
        actual: (catalog) => catalog.descriptionFull(1203),
        expected: {
          en: "Full text \u00b7 1,203 characters",
          ja: "全文・1,203 文字",
        },
      },
      {
        name: "one character and one line",
        actual: (catalog) => catalog.selectionSize(1, 1),
        expected: { en: "1 character \u00b7 1 line", ja: "1 文字・1 行" },
      },
      {
        name: "one description character",
        actual: (catalog) => catalog.descriptionFull(1),
        expected: { en: "Full text \u00b7 1 character", ja: "全文・1 文字" },
      },
    ];
    for (const { name, actual, expected } of cases) {
      for (const locale of localeList) {
        assert.equal(
          actual(catalogs[locale]),
          expected[locale],
          `${name}: ${locale}`,
        );
      }
    }
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

  it("states the body limit that prepareSend enforces in every locale", () => {
    // The wording is the user-visible contract, and the limit it names is the
    // code's own, so a changed byte limit without new wording fails here.
    const limit = `${maxBodyBytes / 1024 / 1024} MiB`;
    for (const locale of localeList) {
      const message = catalogs[locale].preflight["body-too-large"];
      assert.ok(
        [message.text, ...message.steps].join("\n").includes(limit),
        locale,
      );
    }
  });

  for (const locale of localeList) {
    it(`never mentions an options page, a token or a port in ${locale}`, () => {
      const { messages, labels } = entries(catalogs[locale]);
      // Precondition: the walk reached the job API's sections too.
      assert.ok(messages.has("startExchange.noResponse"));
      assert.ok(labels.has("viewer.publish"));
      for (const [path, message] of messages) {
        assert.doesNotMatch(textOf(message), forbiddenWords[locale], path);
      }
      for (const [path, label] of labels) {
        assert.doesNotMatch(label, forbiddenWords[locale], path);
      }
    });

    it(`gives every failure code a distinct message in ${locale}`, () => {
      const codes = Object.keys(failureCodes) as (keyof typeof failureCodes)[];
      const texts = codes.map((code) => catalogs[locale].failure[code].text);
      assert.equal(new Set(texts).size, codes.length);
    });

    it(`never mentions Slack while sending or for a start's result in ${locale}`, () => {
      const catalog = catalogs[locale];
      const startReasons = Object.keys(
        startRejectStatus,
      ) as ServerRejectReason[];
      const walked = entries(catalog).messages;
      const checked: [string, Message][] = [
        ["popup.sending", catalog.popup.sending],
        ["popup.viewerNotice", catalog.popup.viewerNotice],
        ["viewer.sending", catalog.viewer.sending],
        ...[...walked].filter(
          ([path]) =>
            path.startsWith("startExchange.") ||
            path.startsWith("startResult."),
        ),
        ...startReasons.map((reason): [string, Message] => [
          `jobReject.${reason}`,
          catalog.jobReject[reason],
        ]),
      ];
      // Precondition: every exchange message and both results were reached.
      assert.equal(
        checked.filter(([path]) => path.startsWith("start")).length,
        7,
      );
      for (const [path, message] of checked) {
        assert.doesNotMatch(textOf(message), slackWord, path);
      }
      // The scan reaches the wording: the publish messages may name Slack, and do.
      assert.ok(
        Object.values(catalog.publishStates).some((message) =>
          slackWord.test(textOf(message)),
        ),
      );
    });
  }
});
