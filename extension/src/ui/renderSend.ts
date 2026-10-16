// Renders a send outcome and a result-view value as text-only elements. Every
// string is set with textContent, so a value that reached this layer as a
// string is never parsed as HTML; the response body's text never reaches here.

import type { RejectionMessage } from "../core/messages.ts";
import type { CollectedSummary, ResultView } from "../core/resultView.ts";
import type { SendOutcome } from "../core/response.ts";
import { blockMessage, sendOutcomeMessage } from "../core/sendMessages.ts";
import { messageNodes, renderSummary } from "./render.ts";

/** The "sending" notice; the popup and the result window both show it. */
export const sendingMessage: RejectionMessage = {
  text: "Sending the article to the server...",
  steps: [
    "Do not close this window until the result appears.",
    "If you close it, check Slack before sending again.",
  ],
};

/** A section holding the collected summary, rendered by renderSummary. */
function summaryNode(doc: Document, summary: CollectedSummary): HTMLElement {
  const section = doc.createElement("div");
  section.className = "send-summary";
  renderSummary(section, summary);
  return section;
}

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

/** Replaces the children of container with a text-only rendering of a result view. */
export function renderResultView(
  container: HTMLElement,
  view: ResultView,
): void {
  const doc = container.ownerDocument;
  switch (view.kind) {
    case "blocked":
      container.replaceChildren(
        ...messageNodes(doc, blockMessage(view.reason)),
      );
      return;
    case "sending":
      container.replaceChildren(
        ...messageNodes(doc, sendingMessage),
        summaryNode(doc, view.summary),
      );
      return;
    case "sent":
      container.replaceChildren(
        ...outcomeNodes(doc, view.outcome),
        summaryNode(doc, view.summary),
      );
      return;
  }
}
