import type { Message } from "../core/i18n/catalog.ts";

/**
 * The message text and, when present, its numbered steps; textContent only.
 * The text is always the first node, so a caller that names its container by
 * the message's own text can take it from the first slot.
 */
export function messageNodes(
  doc: Document,
  message: Message,
): [HTMLElement, ...HTMLElement[]] {
  const text = doc.createElement("p");
  text.className = "message-text";
  text.textContent = message.text;
  if (message.steps.length === 0) {
    return [text];
  }
  const list = doc.createElement("ol");
  list.className = "message-steps";
  for (const step of message.steps) {
    const item = doc.createElement("li");
    item.textContent = step;
    list.append(item);
  }
  return [text, list];
}
