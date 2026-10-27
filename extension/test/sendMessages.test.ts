import assert from "node:assert/strict";
import { describe, it } from "node:test";

import { catalogs } from "../src/core/i18n/catalog.ts";
import type { Catalog } from "../src/core/i18n/catalog.ts";
import { locales } from "../src/core/i18n/locale.ts";
import type { Locale } from "../src/core/i18n/locale.ts";
import { rejectionMessage } from "../src/core/messages.ts";
import type { RejectionMessage } from "../src/core/messages.ts";
import { preflightReasons } from "../src/core/prepare.ts";
import type { BlockReason } from "../src/core/prepare.ts";
import { serverRejectStatus } from "../src/core/response.ts";
import type {
  NotSentCause,
  RequestId,
  SendOutcome,
  ServerRejectReason,
  UnexpectedCheck,
} from "../src/core/response.ts";
import {
  blockMessage,
  sendOutcomeMessage,
  serverRejectMessage,
} from "../src/core/sendMessages.ts";
import { rejectionReasons } from "../src/core/types.ts";
import type { RejectionReason } from "../src/core/types.ts";

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
const rejectionReasonsList = Object.keys(rejectionReasons) as RejectionReason[];
const preflightReasonsList = Object.keys(
  preflightReasons,
) as (keyof typeof preflightReasons)[];
const serverRejectReasons = Object.keys(
  serverRejectStatus,
) as ServerRejectReason[];

// The properties whose wording differs by locale, as per-locale word tables.
// Each pattern must match the text of every message for a result that is not
// known: it asks to check Slack before sending again, in one sentence in ja.
const unknownResultGuidance: Record<Locale, readonly RegExp[]> = {
  en: [/check slack/iu, /before sending again/iu],
  ja: [/送り直す前に[^。\n]*Slack[^。\n]*確かめて/u],
};
// No message mentions an options page, a token or a port. The ja words are
// whole terms: a bare "ポート" would also match words such as "サポート".
const forbiddenWords: Record<Locale, RegExp> = {
  en: /option|token|\bport\b/iu,
  ja: /オプション|トークン|ポート番号/u,
};

describe("blockMessage", () => {
  for (const locale of localeList) {
    it(`delegates a collection reason to rejectionMessage in ${locale}`, () => {
      for (const reason of rejectionReasonsList) {
        assert.deepEqual(
          blockMessage(reason, locale),
          rejectionMessage(reason, locale),
        );
      }
    });

    it(`returns the ${locale} catalog's message for every block reason`, () => {
      for (const reason of rejectionReasonsList) {
        assert.equal(
          blockMessage(reason, locale),
          catalogs[locale].rejection[reason],
          reason,
        );
      }
      for (const reason of preflightReasonsList) {
        assert.equal(
          blockMessage(reason, locale),
          catalogs[locale].preflight[reason],
          reason,
        );
      }
    });

    it(`gives every block reason a distinct non-empty text in ${locale}`, () => {
      const blockReasons: BlockReason[] = [
        ...rejectionReasonsList,
        ...preflightReasonsList,
      ];
      const texts = blockReasons.map(
        (reason) => blockMessage(reason, locale).text,
      );
      for (const [index, text] of texts.entries()) {
        assert.notEqual(text, "", blockReasons[index]);
      }
      assert.equal(new Set(texts).size, blockReasons.length);
    });

    it(`names the limit for body-too-large in ${locale}`, () => {
      assert.ok(
        textOf(blockMessage("body-too-large", locale)).includes("4 MiB"),
      );
    });
  }
});

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
  const unexpectedChecks: Record<UnexpectedCheck, true> = {
    "extension-timeout": true,
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
        { kind: "unexpected", check: "extension-timeout" },
        locale,
      );
      for (const check of Object.keys(unexpectedChecks) as UnexpectedCheck[]) {
        assert.deepEqual(
          sendOutcomeMessage({ kind: "unexpected", check }, locale),
          message,
          check,
        );
      }
    });

    it(`never mentions an options page, a token or a port in ${locale}`, () => {
      const messages = [
        ...outcomes.map(({ outcome }) => sendOutcomeMessage(outcome, locale)),
        ...serverRejectReasons.map((reason) =>
          serverRejectMessage(reason, locale),
        ),
        ...[...rejectionReasonsList, ...preflightReasonsList].map((reason) =>
          blockMessage(reason, locale),
        ),
      ];
      for (const message of messages) {
        const text = textOf(message);
        assert.doesNotMatch(text, forbiddenWords[locale], text);
      }
    });
  }
});
