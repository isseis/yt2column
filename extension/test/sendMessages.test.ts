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
      assert.deepEqual(blockMessage(reason), rejectionMessage(reason));
    }
  });

  it("gives every block reason a non-empty text", () => {
    const blockReasons: BlockReason[] = [
      ...rejectionReasonsList,
      ...preflightReasonsList,
    ];
    for (const reason of blockReasons) {
      assert.notEqual(blockMessage(reason).text, "", reason);
    }
  });

  it("gives every block reason a distinct text", () => {
    const blockReasons: BlockReason[] = [
      ...rejectionReasonsList,
      ...preflightReasonsList,
    ];
    const texts = blockReasons.map((reason) => blockMessage(reason).text);
    assert.equal(new Set(texts).size, blockReasons.length);
  });

  it("names the next action for the preflight reasons", () => {
    assert.ok(textOf(blockMessage("settings-unreadable")).includes("options"));
    assert.ok(textOf(blockMessage("port-invalid")).includes("8765"));
    assert.ok(textOf(blockMessage("body-too-large")).includes("4 MiB"));
  });
});

describe("serverRejectMessage", () => {
  const reasons = Object.keys(serverRejectStatus) as ServerRejectReason[];

  it("gives every reason a non-empty text", () => {
    for (const reason of reasons) {
      assert.notEqual(serverRejectMessage(reason).text, "", reason);
    }
  });

  it("gives every reason a distinct text", () => {
    const texts = reasons.map((reason) => serverRejectMessage(reason).text);
    assert.equal(new Set(texts).size, reasons.length);
  });
});

describe("sendOutcomeMessage", () => {
  it("delegates a rejection to serverRejectMessage", () => {
    const outcome: SendOutcome = {
      kind: "rejected",
      reason: "unauthorized",
      requestId,
    };
    assert.deepEqual(
      sendOutcomeMessage(outcome),
      serverRejectMessage("unauthorized"),
    );
  });

  const slackCases: { name: string; outcome: SendOutcome }[] = [
    { name: "accepted", outcome: { kind: "accepted", requestId } },
    { name: "unknown", outcome: { kind: "unknown" } },
    {
      name: "unexpected",
      outcome: { kind: "unexpected", status: 500, check: "json-shape" },
    },
    { name: "redirected", outcome: { kind: "redirected" } },
    {
      name: "too_many_requests",
      outcome: { kind: "rejected", reason: "too_many_requests", requestId },
    },
  ];
  for (const { name, outcome } of slackCases) {
    it(`tells the user to check Slack for ${name}`, () => {
      assert.ok(textOf(sendOutcomeMessage(outcome)).includes("Slack"), name);
    });
  }
});
