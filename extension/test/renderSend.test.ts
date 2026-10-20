import assert from "node:assert/strict";
import { describe, it } from "node:test";

import type { RejectionMessage } from "../src/core/messages.ts";
import type { BlockReason } from "../src/core/prepare.ts";
import type { RequestId, SendOutcome } from "../src/core/response.ts";
import { blockMessage, sendOutcomeMessage } from "../src/core/sendMessages.ts";
import { renderBlockReason, renderSendOutcome } from "../src/ui/renderSend.ts";
import { container, query } from "./helpers/dom.ts";

const requestId = "0123456789abcdef" as RequestId;

/** Asserts the container's text shows the message and each of its steps. */
function expectMessage(root: HTMLElement, message: RejectionMessage): void {
  const text = root.textContent ?? "";
  assert.ok(text.includes(message.text), message.text);
  for (const step of message.steps) {
    assert.ok(text.includes(step), step);
  }
}

describe("renderSendOutcome", () => {
  const accepted: SendOutcome = { kind: "accepted", requestId };
  const rejected: SendOutcome = {
    kind: "rejected",
    reason: "invalid_transcript",
    requestId,
  };

  const withRequestId = [accepted, rejected];
  for (const outcome of withRequestId) {
    it(`shows the ${outcome.kind} message and its request id`, () => {
      const root = container();
      renderSendOutcome(root, outcome);
      expectMessage(root, sendOutcomeMessage(outcome));
      assert.equal(query(root, ".send-request-id").textContent, requestId);
    });
  }

  const withoutRequestId: SendOutcome[] = [
    { kind: "not-sent", cause: "server-unavailable" },
    { kind: "no-response" },
    { kind: "host-failed" },
  ];
  for (const outcome of withoutRequestId) {
    it(`shows the ${outcome.kind} message and no request id`, () => {
      const root = container();
      renderSendOutcome(root, outcome);
      expectMessage(root, sendOutcomeMessage(outcome));
      assert.equal(root.querySelector(".send-request-id"), null);
    });
  }

  it("shows the unexpected message but not the check", () => {
    const root = container();
    const outcome: SendOutcome = { kind: "unexpected", check: "json-shape" };
    renderSendOutcome(root, outcome);
    expectMessage(root, sendOutcomeMessage(outcome));
    assert.equal((root.textContent ?? "").includes("json-shape"), false);
  });

  it("replaces the previous content on each call", () => {
    const root = container();
    renderSendOutcome(root, accepted);
    renderSendOutcome(root, { kind: "no-response" });
    assert.equal(root.querySelectorAll(".send-request-id").length, 0);
    expectMessage(root, sendOutcomeMessage({ kind: "no-response" }));
  });
});

describe("renderBlockReason", () => {
  // The rendering does not vary by reason, so two reasons suffice here; the
  // popup's blocked display is covered per reason in launch.test.ts, and the
  // message content is sendMessages.test.ts's contract.
  const blockReasons: BlockReason[] = ["empty-selection", "body-too-large"];
  for (const reason of blockReasons) {
    it(`shows the ${reason} block message`, () => {
      const root = container();
      renderBlockReason(root, reason);
      expectMessage(root, blockMessage(reason));
    });
  }
});
