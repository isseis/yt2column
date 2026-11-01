import assert from "node:assert/strict";
import { describe, it } from "node:test";

import { catalogs } from "../src/core/i18n/catalog.ts";
import type { Catalog } from "../src/core/i18n/catalog.ts";
import { locales } from "../src/core/i18n/locale.ts";
import type { Locale } from "../src/core/i18n/locale.ts";
import type { RejectionMessage } from "../src/core/messages.ts";
import { articlesRejectStatus } from "../src/core/articlesResponse.ts";
import type {
  ArticlesRejectReason,
  ArticlesUnexpectedCheck,
  SendOutcome,
} from "../src/core/articlesResponse.ts";
import type { ArticleKey } from "../src/core/articleKey.ts";
import { unexpectedChecks } from "../src/core/exchange.ts";
import type {
  ExchangeFailure,
  NotSentCause,
  UnexpectedCheck,
} from "../src/core/exchange.ts";
import type { ExchangeMessages } from "../src/core/i18n/catalog.ts";
import type { RetryingStatus, StoppingStatus } from "../src/core/jobView.ts";
import {
  publishRejectStatus,
  startRejectStatus,
  statusRejectStatus,
} from "../src/core/response.ts";
import type { RequestId, ServerRejectReason } from "../src/core/response.ts";
import {
  jobRejectMessage,
  publishResultMessage,
  sendOutcomeMessage,
  serverRejectMessage,
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
const serverRejectReasons = Object.keys(
  articlesRejectStatus,
) as ArticlesRejectReason[];

// The properties whose wording differs by locale, as per-locale word tables.
// Each pattern must match the text of every message for a result that is not
// known: it asks to check Slack before sending again, in one sentence in ja.
const unknownResultGuidance: Record<Locale, readonly RegExp[]> = {
  en: [/check slack/iu, /before sending again/iu],
  ja: [/送り直す前に[^。\n]*Slack[^。\n]*確かめて/u],
};
describe("serverRejectMessage", () => {
  for (const locale of localeList) {
    it(`returns the ${locale} catalog's message for every reason`, () => {
      for (const reason of serverRejectReasons) {
        assert.equal(
          serverRejectMessage(reason, locale),
          catalogs[locale].serverReject[reason],
          reason,
        );
      }
    });

    it(`gives every reason a distinct non-empty text in ${locale}`, () => {
      const texts = serverRejectReasons.map(
        (reason) => serverRejectMessage(reason, locale).text,
      );
      for (const [index, text] of texts.entries()) {
        assert.notEqual(text, "", serverRejectReasons[index]);
      }
      assert.equal(new Set(texts).size, serverRejectReasons.length);
    });
  }
});

describe("sendOutcomeMessage", () => {
  // One outcome per displayed kind and not-sent cause, with the catalog
  // message it must show.
  const outcomes: {
    name: string;
    outcome: SendOutcome;
    inCatalog: (catalog: Catalog) => RejectionMessage;
  }[] = [
    {
      name: "accepted",
      outcome: { kind: "accepted", requestId },
      inCatalog: (catalog) => catalog.accepted,
    },
    {
      name: "rejected",
      outcome: { kind: "rejected", reason: "invalid_body", requestId },
      inCatalog: (catalog) => catalog.serverReject.invalid_body,
    },
    ...(["server-unavailable", "message-too-large"] as NotSentCause[]).map(
      (cause) => ({
        name: cause,
        outcome: { kind: "not-sent", cause } as SendOutcome,
        inCatalog: (catalog: Catalog) => catalog.notSent[cause],
      }),
    ),
    {
      name: "no-response",
      outcome: { kind: "no-response" },
      inCatalog: (catalog) => catalog.noResponse,
    },
    {
      name: "host-failed",
      outcome: { kind: "host-failed" },
      inCatalog: (catalog) => catalog.hostFailed,
    },
    {
      name: "unexpected",
      outcome: { kind: "unexpected", check: "envelope-shape" },
      inCatalog: (catalog) => catalog.unexpected,
    },
  ];

  /** The row named name. */
  function outcomeNamed(name: string): SendOutcome {
    const entry = outcomes.find((row) => row.name === name);
    assert.ok(entry !== undefined, name);
    return entry.outcome;
  }

  // Every check, as a Record so that a new check must be listed here.
  const unexpectedChecks: Record<ArticlesUnexpectedCheck, true> = {
    "envelope-shape": true,
    "envelope-version": true,
    "host-protocol-error": true,
    "body-over-limit": true,
    encoding: true,
    escape: true,
    "json-shape": true,
    "duplicate-member": true,
    "member-set": true,
    "request-id": true,
    "status-pairing": true,
    "classification-error": true,
  };

  for (const locale of localeList) {
    it(`returns the ${locale} catalog's message for every outcome`, () => {
      for (const { name, outcome, inCatalog } of outcomes) {
        assert.equal(
          sendOutcomeMessage(outcome, locale),
          inCatalog(catalogs[locale]),
          name,
        );
      }
    });

    it(`delegates a rejection to serverRejectMessage in ${locale}`, () => {
      assert.deepEqual(
        sendOutcomeMessage(outcomeNamed("rejected"), locale),
        serverRejectMessage("invalid_body", locale),
      );
    });

    it(`gives every outcome a distinct text in ${locale}`, () => {
      const texts = outcomes.map(
        ({ outcome }) => sendOutcomeMessage(outcome, locale).text,
      );
      assert.equal(new Set(texts).size, outcomes.length);
    });

    for (const name of ["server-unavailable", "message-too-large"]) {
      it(`does not ask to check Slack when nothing was sent (${name}, ${locale})`, () => {
        assert.equal(
          textOf(sendOutcomeMessage(outcomeNamed(name), locale)).includes(
            "Slack",
          ),
          false,
        );
      });
    }

    for (const name of ["no-response", "host-failed", "unexpected"]) {
      it(`asks to check Slack before sending again when the result is unknown (${name}, ${locale})`, () => {
        const text = textOf(sendOutcomeMessage(outcomeNamed(name), locale));
        for (const pattern of unknownResultGuidance[locale]) {
          assert.match(text, pattern, name);
        }
      });
    }

    it(`asks to check Slack before sending again in the sending notice in ${locale}`, () => {
      // A closed popup leaves the result unknown, as the outcomes above do.
      const text = textOf(catalogs[locale].sending);
      for (const pattern of unknownResultGuidance[locale]) {
        assert.match(text, pattern);
      }
    });

    it(`tells the user to check Slack after an accepted send and too_many_requests in ${locale}`, () => {
      assert.ok(
        textOf(sendOutcomeMessage(outcomeNamed("accepted"), locale)).includes(
          "Slack",
        ),
      );
      assert.ok(
        textOf(serverRejectMessage("too_many_requests", locale)).includes(
          "Slack",
        ),
      );
    });

    it(`names the host install step for host-failed in ${locale}`, () => {
      const text = textOf(
        sendOutcomeMessage(outcomeNamed("host-failed"), locale),
      );
      assert.ok(text.includes("yt2column-nmhost install"));
    });

    it(`names the server start for server-unavailable in ${locale}`, () => {
      const text = textOf(
        sendOutcomeMessage(outcomeNamed("server-unavailable"), locale),
      );
      assert.ok(text.includes("yt2column-server"));
    });

    it(`shows one message for every unexpected check in ${locale}`, () => {
      const message = sendOutcomeMessage(
        { kind: "unexpected", check: "envelope-shape" },
        locale,
      );
      for (const check of Object.keys(
        unexpectedChecks,
      ) as ArticlesUnexpectedCheck[]) {
        assert.deepEqual(
          sendOutcomeMessage({ kind: "unexpected", check }, locale),
          message,
          check,
        );
      }
    });
  }
});

describe("job API messages", () => {
  const jobRejectReasons = [
    ...new Set([
      ...Object.keys(startRejectStatus),
      ...Object.keys(statusRejectStatus),
      ...Object.keys(publishRejectStatus),
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
    {
      name: "publish",
      message: (failure, locale) => publishResultMessage(failure, locale),
      section: (catalog) => catalog.publishExchange,
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
    it(`gives every job API reason its own distinct message in ${locale}`, () => {
      const texts = jobRejectReasons.map((reason) => {
        const message = jobRejectMessage(reason, locale);
        assert.equal(message, catalogs[locale].jobReject[reason], reason);
        return message.text;
      });
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

    it(`delegates every operation's rejection to jobRejectMessage in ${locale}`, () => {
      assert.equal(
        startResultMessage(
          { kind: "rejected", reason: "invalid_body", requestId },
          locale,
        ),
        jobRejectMessage("invalid_body", locale),
      );
      assert.equal(
        statusNoticeMessage(
          { kind: "rejected", reason: "invalid_key", requestId },
          locale,
        ),
        jobRejectMessage("invalid_key", locale),
      );
      assert.equal(
        publishResultMessage(
          { kind: "rejected", reason: "article_not_found", requestId },
          locale,
        ),
        jobRejectMessage("article_not_found", locale),
      );
    });
  }
});
