import assert from "node:assert/strict";
import { describe, it } from "node:test";

import { catalogs } from "../src/core/i18n/catalog.ts";
import type { Catalog } from "../src/core/i18n/catalog.ts";
import { locales } from "../src/core/i18n/locale.ts";
import type { Locale } from "../src/core/i18n/locale.ts";
import type { RejectionMessage } from "../src/core/messages.ts";
import type { ArticleKey } from "../src/core/articleKey.ts";
import { unexpectedChecks } from "../src/core/exchange.ts";
import type { ExchangeFailure, UnexpectedCheck } from "../src/core/exchange.ts";
import type { ExchangeMessages } from "../src/core/i18n/catalog.ts";
import type { RetryingStatus, StoppingStatus } from "../src/core/jobView.ts";
import { startRejectStatus, statusRejectStatus } from "../src/core/response.ts";
import type { RequestId, ServerRejectReason } from "../src/core/response.ts";
import {
  jobRejectMessage,
  startResultMessage,
  statusNoticeMessage,
} from "../src/core/sendMessages.ts";

// Tests may assert a branded type to build fixtures.
const requestId = "0123456789abcdef" as RequestId;

/**
 * The whole message as one string, for content checks. Matching on this text
 * is matching the displayed contract: the wording is what the user reads.
 */
function textOf(message: RejectionMessage): string {
  return [message.text, ...message.steps].join("\n");
}

const localeList = Object.keys(locales) as Locale[];
describe("job API messages", () => {
  const jobRejectReasons = [
    ...new Set([
      ...Object.keys(startRejectStatus),
      ...Object.keys(statusRejectStatus),
    ]),
  ] as ServerRejectReason[];
  const checks = Object.keys(unexpectedChecks) as UnexpectedCheck[];

  // One outcome of each exchange kind that is not a server response.
  const failures: { name: string; failure: ExchangeFailure }[] = [
    {
      name: "server-unavailable",
      failure: { kind: "not-sent", cause: "server-unavailable" },
    },
    {
      name: "message-too-large",
      failure: { kind: "not-sent", cause: "message-too-large" },
    },
    { name: "no-response", failure: { kind: "no-response" } },
    { name: "host-failed", failure: { kind: "host-failed" } },
    {
      name: "unexpected",
      failure: { kind: "unexpected", check: "member-set" },
    },
  ];

  // Each operation's message function over the exchange failures, and the
  // catalog section it must read.
  const operations: {
    name: string;
    message: (failure: ExchangeFailure, locale: Locale) => RejectionMessage;
    section: (catalog: Catalog) => ExchangeMessages;
  }[] = [
    {
      name: "start",
      message: (failure, locale) => startResultMessage(failure, locale),
      section: (catalog) => catalog.startExchange,
    },
    {
      name: "status",
      message: (failure, locale) =>
        statusNoticeMessage(failure as RetryingStatus | StoppingStatus, locale),
      section: (catalog) => catalog.statusExchange,
    },
  ];

  /** The catalog message an exchange failure must show. */
  function inSection(
    messages: ExchangeMessages,
    failure: ExchangeFailure,
  ): RejectionMessage {
    switch (failure.kind) {
      case "not-sent":
        return messages.notSent[failure.cause];
      case "no-response":
        return messages.noResponse;
      case "host-failed":
        return messages.hostFailed;
      case "unexpected":
        return messages.unexpected;
    }
  }

  for (const locale of localeList) {
    it(`gives every job API reason a distinct text in ${locale}`, () => {
      const texts = jobRejectReasons.map(
        (reason) => jobRejectMessage(reason, locale).text,
      );
      assert.equal(new Set(texts).size, jobRejectReasons.length);
    });

    it(`shows the start's own results in ${locale}`, () => {
      const catalog = catalogs[locale];
      assert.equal(
        startResultMessage(
          { kind: "accepted", key: "k" as ArticleKey, requestId },
          locale,
        ),
        catalog.startResult.accepted,
      );
      assert.equal(
        startResultMessage({ kind: "no-result" }, locale),
        catalog.startResult.noResult,
      );
    });

    for (const { name, message, section } of operations) {
      it(`reads the ${name} section for every exchange failure in ${locale}`, () => {
        const messages = section(catalogs[locale]);
        for (const { name: kind, failure } of failures) {
          assert.equal(
            message(failure, locale),
            inSection(messages, failure),
            kind,
          );
        }
      });

      it(`gives every ${name} exchange failure a distinct text in ${locale}`, () => {
        const texts = failures.map(
          ({ failure }) => message(failure, locale).text,
        );
        assert.equal(new Set(texts).size, failures.length);
      });

      it(`shows one ${name} message for every unexpected check in ${locale}`, () => {
        const expected = section(catalogs[locale]).unexpected;
        for (const check of checks) {
          assert.equal(
            message({ kind: "unexpected", check }, locale),
            expected,
            check,
          );
        }
      });

      it(`names the host install step for a ${name} host failure in ${locale}`, () => {
        const text = textOf(message({ kind: "host-failed" }, locale));
        assert.ok(text.includes("yt2column-nmhost install"), text);
      });

      it(`names the server start for a ${name} not sent to the server in ${locale}`, () => {
        const text = textOf(
          message({ kind: "not-sent", cause: "server-unavailable" }, locale),
        );
        assert.ok(text.includes("yt2column-server"), text);
      });
    }

    it(`shows the reason's message for every operation's rejection in ${locale}`, () => {
      const catalog = catalogs[locale];
      const reasons = <R extends ServerRejectReason>(set: Record<R, number>) =>
        Object.keys(set) as R[];
      for (const reason of reasons(startRejectStatus)) {
        assert.equal(
          startResultMessage({ kind: "rejected", reason, requestId }, locale),
          catalog.jobReject[reason],
          reason,
        );
      }
      for (const reason of reasons(statusRejectStatus)) {
        assert.equal(
          statusNoticeMessage({ kind: "rejected", reason, requestId }, locale),
          catalog.jobReject[reason],
          reason,
        );
      }
    });

    it(`words each operation's exchange failures differently in ${locale}`, () => {
      for (const { name, failure } of failures) {
        const texts = operations.map(({ message }) =>
          textOf(message(failure, locale)),
        );
        assert.equal(new Set(texts).size, operations.length, name);
      }
    });
  }
});
