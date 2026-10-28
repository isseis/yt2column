// Renders a send outcome as text-only elements. Every string is set with
// textContent, so a value that reached this layer as a string is never parsed
// as HTML; the response body's text never reaches here.

import type { Locale } from "../core/i18n/locale.ts";
import type { SendOutcome } from "../core/response.ts";
import { sendOutcomeMessage } from "../core/sendMessages.ts";
import { messageNodes } from "./render.ts";

/** The message, plus the request id when the outcome has one. */
function outcomeNodes(
  doc: Document,
  outcome: SendOutcome,
  locale: Locale,
): HTMLElement[] {
  const nodes = messageNodes(doc, sendOutcomeMessage(outcome, locale));
  if (outcome.kind === "accepted" || outcome.kind === "rejected") {
    const id = doc.createElement("p");
    id.className = "send-request-id";
    id.textContent = outcome.requestId;
    nodes.push(id);
  }
  return nodes;
}

/** Replaces the children of container with a text-only rendering of a send outcome. */
export function renderSendOutcome(
  container: HTMLElement,
  outcome: SendOutcome,
  locale: Locale,
): void {
  const doc = container.ownerDocument;
  container.replaceChildren(...outcomeNodes(doc, outcome, locale));
}
