// Renders a send outcome and a block reason as text-only elements. Every
// string is set with textContent, so a value that reached this layer as a
// string is never parsed as HTML; the response body's text never reaches here.

import type { RejectionMessage } from "../core/messages.ts";
import type { BlockReason } from "../core/prepare.ts";
import type { SendOutcome } from "../core/response.ts";
import { blockMessage, sendOutcomeMessage } from "../core/sendMessages.ts";
import { messageNodes } from "./render.ts";

/** The "sending" notice the popup shows while a send is in flight. */
export const sendingMessage: RejectionMessage = {
  text: "Sending the article to the server...",
  steps: [
    "Do not close this window until the result appears.",
    "If you close it, check Slack before sending again.",
  ],
};

/** The message, plus the request id or the status code when the outcome has one. */
function outcomeNodes(doc: Document, outcome: SendOutcome): HTMLElement[] {
  const nodes = messageNodes(doc, sendOutcomeMessage(outcome));
  if (outcome.kind === "accepted" || outcome.kind === "rejected") {
    const id = doc.createElement("p");
    id.className = "send-request-id";
    id.textContent = outcome.requestId;
    nodes.push(id);
  }
  if (outcome.kind === "unexpected") {
    const status = doc.createElement("p");
    status.className = "send-status";
    status.textContent = String(outcome.status);
    nodes.push(status);
  }
  return nodes;
}

/** Replaces the children of container with a text-only rendering of a send outcome. */
export function renderSendOutcome(
  container: HTMLElement,
  outcome: SendOutcome,
): void {
  const doc = container.ownerDocument;
  container.replaceChildren(...outcomeNodes(doc, outcome));
}

/** Replaces the children of container with the text-only message for a block reason. */
export function renderBlockReason(
  container: HTMLElement,
  reason: BlockReason,
): void {
  const doc = container.ownerDocument;
  container.replaceChildren(...messageNodes(doc, blockMessage(reason)));
}
