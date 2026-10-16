import assert from "node:assert/strict";
import { describe, it } from "node:test";

import type { RejectionMessage } from "../src/core/messages.ts";
import type { BlockReason } from "../src/core/prepare.ts";
import type { CollectedSummary, ResultView } from "../src/core/resultView.ts";
import type { RequestId, SendOutcome } from "../src/core/response.ts";
import { blockMessage, sendOutcomeMessage } from "../src/core/sendMessages.ts";
import {
  renderResultView,
  renderSendOutcome,
  sendingMessage,
} from "../src/ui/renderSend.ts";
import { container, query } from "./helpers/dom.ts";

const requestId = "0123456789abcdef" as RequestId;
const malicious = "<img src=x onerror=alert(1)>";

const collected: CollectedSummary = {
  kind: "collected",
  title: "A video",
  titleTruncated: false,
  url: "https://www.youtube.com/watch?v=abc",
  urlTruncated: false,
  characterCount: 2,
  lineCount: 1,
  preview: "hi",
  previewTruncated: false,
};

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
    reason: "unauthorized",
    requestId,
  };

  const withRequestId = [accepted, rejected];
  for (const outcome of withRequestId) {
    it(`shows the ${outcome.kind} message and its request id`, () => {
      const root = container();
      renderSendOutcome(root, outcome);
      expectMessage(root, sendOutcomeMessage(outcome));
      assert.equal(query(root, ".send-request-id").textContent, requestId);
      assert.equal(root.querySelector(".send-status"), null);
    });
  }

  it("shows the unknown message", () => {
    const root = container();
    renderSendOutcome(root, { kind: "unknown" });
    expectMessage(root, sendOutcomeMessage({ kind: "unknown" }));
  });

  it("shows the unexpected message and the status number but not the check", () => {
    const root = container();
    renderSendOutcome(root, {
      kind: "unexpected",
      status: 500,
      check: "json-shape",
    });
    expectMessage(
      root,
      sendOutcomeMessage({
        kind: "unexpected",
        status: 500,
        check: "json-shape",
      }),
    );
    assert.equal(query(root, ".send-status").textContent, "500");
    assert.equal((root.textContent ?? "").includes("json-shape"), false);
  });

  it("shows the redirected message and no status number", () => {
    const root = container();
    renderSendOutcome(root, { kind: "redirected" });
    expectMessage(root, sendOutcomeMessage({ kind: "redirected" }));
    assert.equal(root.querySelector(".send-status"), null);
  });

  it("replaces the previous content on each call", () => {
    const root = container();
    renderSendOutcome(root, accepted);
    renderSendOutcome(root, { kind: "unknown" });
    assert.equal(root.querySelectorAll(".send-request-id").length, 0);
    expectMessage(root, sendOutcomeMessage({ kind: "unknown" }));
  });
});

describe("renderResultView", () => {
  // One collection reason and one preflight reason: the blocked branch does
  // not vary by reason, and message content is sendMessages.test.ts's contract.
  const blockReasons: BlockReason[] = ["empty-selection", "token-missing"];
  for (const reason of blockReasons) {
    it(`shows the ${reason} block message`, () => {
      const root = container();
      const view: ResultView = { kind: "blocked", reason };
      renderResultView(root, view);
      expectMessage(root, blockMessage(reason));
    });
  }

  it("shows the sending notice with the close guidance and the summary", () => {
    const root = container();
    renderResultView(root, { kind: "sending", summary: collected });
    expectMessage(root, sendingMessage);
    // The close guidance and the Slack warning are the substance of AC-22, so
    // assert them independently of the constant the code renders.
    const text = root.textContent ?? "";
    assert.ok(text.includes("close"), text);
    assert.ok(text.includes("Slack"), text);
    assert.ok(text.includes(collected.title));
  });

  it("shows the sent outcome and its request id alongside the summary", () => {
    const root = container();
    renderResultView(root, {
      kind: "sent",
      summary: collected,
      outcome: { kind: "accepted", requestId },
    });
    assert.equal(query(root, ".send-request-id").textContent, requestId);
    assert.ok((root.textContent ?? "").includes(collected.title));
  });

  it("displays markup in the summary as text, not elements", () => {
    const root = container();
    renderResultView(root, {
      kind: "sent",
      summary: { ...collected, title: malicious, preview: malicious },
      outcome: { kind: "accepted", requestId },
    });
    assert.equal(root.querySelector("img"), null);
    assert.ok((root.textContent ?? "").includes(malicious));
  });
});
