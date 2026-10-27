import assert from "node:assert/strict";
import { describe, it } from "node:test";

import { rejectionMessage } from "../src/core/messages.ts";
import type { RejectionMessage } from "../src/core/messages.ts";
import { preflightReasons } from "../src/core/prepare.ts";
import type { BlockReason } from "../src/core/prepare.ts";
import { serverRejectStatus } from "../src/core/response.ts";
import type {
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

const rejectionReasonsList = Object.keys(rejectionReasons) as RejectionReason[];
const preflightReasonsList = Object.keys(
  preflightReasons,
) as (keyof typeof preflightReasons)[];

describe("blockMessage", () => {
  it("delegates a collection reason to rejectionMessage", () => {
    for (const reason of rejectionReasonsList) {
      assert.deepEqual(
        blockMessage(reason, "en"),
        rejectionMessage(reason, "en"),
      );
    }
  });

  it("gives every block reason a non-empty text", () => {
    const blockReasons: BlockReason[] = [
      ...rejectionReasonsList,
      ...preflightReasonsList,
    ];
    for (const reason of blockReasons) {
      assert.notEqual(blockMessage(reason, "en").text, "", reason);
    }
  });

  it("gives every block reason a distinct text", () => {
    const blockReasons: BlockReason[] = [
      ...rejectionReasonsList,
      ...preflightReasonsList,
    ];
    const texts = blockReasons.map((reason) => blockMessage(reason, "en").text);
    assert.equal(new Set(texts).size, blockReasons.length);
  });

  it("names the limit for body-too-large", () => {
    assert.ok(textOf(blockMessage("body-too-large", "en")).includes("4 MiB"));
  });
});

describe("serverRejectMessage", () => {
  const reasons = Object.keys(serverRejectStatus) as ServerRejectReason[];

  it("gives every reason a non-empty text", () => {
    for (const reason of reasons) {
      assert.notEqual(serverRejectMessage(reason, "en").text, "", reason);
    }
  });

  it("gives every reason a distinct text", () => {
    const texts = reasons.map(
      (reason) => serverRejectMessage(reason, "en").text,
    );
    assert.equal(new Set(texts).size, reasons.length);
  });
});

describe("sendOutcomeMessage", () => {
  it("delegates a rejection to serverRejectMessage", () => {
    const outcome: SendOutcome = {
      kind: "rejected",
      reason: "invalid_body",
      requestId,
    };
    assert.deepEqual(
      sendOutcomeMessage(outcome, "en"),
      serverRejectMessage("invalid_body", "en"),
    );
  });

  // One outcome per displayed kind and not-sent cause.
  const outcomes: { name: string; outcome: SendOutcome }[] = [
    { name: "accepted", outcome: { kind: "accepted", requestId } },
    {
      name: "rejected",
      outcome: { kind: "rejected", reason: "invalid_body", requestId },
    },
    {
      name: "server-unavailable",
      outcome: { kind: "not-sent", cause: "server-unavailable" },
    },
    {
      name: "message-too-large",
      outcome: { kind: "not-sent", cause: "message-too-large" },
    },
    { name: "no-response", outcome: { kind: "no-response" } },
    { name: "host-failed", outcome: { kind: "host-failed" } },
    {
      name: "unexpected",
      outcome: { kind: "unexpected", check: "envelope-shape" },
    },
  ];

  it("gives every outcome a distinct text", () => {
    const texts = outcomes.map(
      ({ outcome }) => sendOutcomeMessage(outcome, "en").text,
    );
    assert.equal(new Set(texts).size, outcomes.length);
  });

  const notSent = ["server-unavailable", "message-too-large"];
  for (const name of notSent) {
    it(`does not ask to check Slack when nothing was sent (${name})`, () => {
      const entry = outcomes.find((row) => row.name === name);
      assert.ok(entry !== undefined);
      assert.equal(
        textOf(sendOutcomeMessage(entry.outcome, "en")).includes("Slack"),
        false,
      );
    });
  }

  const unknown = ["no-response", "host-failed", "unexpected"];
  for (const name of unknown) {
    it(`asks to check Slack before sending again when the result is unknown (${name})`, () => {
      const entry = outcomes.find((row) => row.name === name);
      assert.ok(entry !== undefined);
      const text = textOf(
        sendOutcomeMessage(entry.outcome, "en"),
      ).toLowerCase();
      assert.ok(text.includes("check slack"), name);
      assert.ok(text.includes("before sending again"), name);
    });
  }

  it("tells the user to check Slack after an accepted send and too_many_requests", () => {
    assert.ok(
      textOf(
        sendOutcomeMessage({ kind: "accepted", requestId }, "en"),
      ).includes("Slack"),
    );
    assert.ok(
      textOf(serverRejectMessage("too_many_requests", "en")).includes("Slack"),
    );
  });

  it("names the host install step for host-failed", () => {
    const text = textOf(sendOutcomeMessage({ kind: "host-failed" }, "en"));
    assert.ok(text.includes("yt2column-nmhost install"));
  });

  it("names the server start for server-unavailable", () => {
    const text = textOf(
      sendOutcomeMessage(
        { kind: "not-sent", cause: "server-unavailable" },
        "en",
      ),
    );
    assert.ok(text.includes("yt2column-server"));
  });

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

  it("shows one message for every unexpected check", () => {
    const message = sendOutcomeMessage(
      {
        kind: "unexpected",
        check: "extension-timeout",
      },
      "en",
    );
    for (const check of Object.keys(unexpectedChecks) as UnexpectedCheck[]) {
      assert.deepEqual(
        sendOutcomeMessage({ kind: "unexpected", check }, "en"),
        message,
        check,
      );
    }
  });

  it("never mentions an options page, a token or a port", () => {
    const messages = [
      ...outcomes.map(({ outcome }) => sendOutcomeMessage(outcome, "en")),
      ...(Object.keys(serverRejectStatus) as ServerRejectReason[]).map(
        (reason) => serverRejectMessage(reason, "en"),
      ),
      ...[...rejectionReasonsList, ...preflightReasonsList].map((reason) =>
        blockMessage(reason, "en"),
      ),
    ];
    for (const message of messages) {
      const text = textOf(message);
      assert.doesNotMatch(text, /option|token|\bport\b/iu, text);
    }
  });
});
