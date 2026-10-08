import { rejectionMessage } from "../core/messages.ts";
import type { OutcomeSummary } from "../core/types.ts";

/** Appended to a value that was shortened for display. */
const ellipsis = "\u2026";

/** The value shown for a field, marked with an ellipsis when shortened. */
function displayValue(value: string, truncated: boolean): string {
  return truncated ? `${value}${ellipsis}` : value;
}

interface FieldStyle {
  readonly className: string;
  readonly dirAuto?: boolean;
}

/** One labelled field; the label and value are set with textContent only. */
function field(
  doc: Document,
  label: string,
  value: string,
  style: FieldStyle,
): HTMLElement {
  const row = doc.createElement("div");
  row.className = "summary-field";
  const labelElement = doc.createElement("span");
  labelElement.className = "summary-label";
  labelElement.textContent = label;
  const valueElement = doc.createElement("span");
  valueElement.className = `summary-value ${style.className}`;
  if (style.dirAuto === true) {
    valueElement.setAttribute("dir", "auto");
  }
  valueElement.textContent = value;
  row.append(labelElement, valueElement);
  return row;
}

/** The collected fields, in display order. */
function collectedNodes(
  doc: Document,
  summary: Extract<OutcomeSummary, { kind: "collected" }>,
): HTMLElement[] {
  return [
    field(doc, "Title", displayValue(summary.title, summary.titleTruncated), {
      className: "summary-title",
      dirAuto: true,
    }),
    field(doc, "URL", displayValue(summary.url, summary.urlTruncated), {
      className: "summary-url",
    }),
    field(doc, "Characters", String(summary.characterCount), {
      className: "summary-character-count",
    }),
    field(doc, "Lines", String(summary.lineCount), {
      className: "summary-line-count",
    }),
    field(
      doc,
      "Preview",
      displayValue(summary.preview, summary.previewTruncated),
      { className: "summary-preview", dirAuto: true },
    ),
  ];
}

/** The rejection message and, when present, its numbered steps. */
function rejectedNodes(
  doc: Document,
  summary: Extract<OutcomeSummary, { kind: "rejected" }>,
): HTMLElement[] {
  const message = rejectionMessage(summary.reason);
  const text = doc.createElement("p");
  text.className = "summary-message";
  text.textContent = message.text;
  const nodes: HTMLElement[] = [text];
  if (message.steps.length > 0) {
    const list = doc.createElement("ol");
    list.className = "summary-steps";
    for (const step of message.steps) {
      const item = doc.createElement("li");
      item.textContent = step;
      list.append(item);
    }
    nodes.push(list);
  }
  return nodes;
}

/**
 * Replaces the children of container with a text-only rendering of the
 * summary. Every string is set with textContent, so a value read from the
 * page is displayed as text and never parsed as HTML.
 */
export function renderSummary(
  container: HTMLElement,
  summary: OutcomeSummary,
): void {
  const doc = container.ownerDocument;
  const nodes =
    summary.kind === "collected"
      ? collectedNodes(doc, summary)
      : rejectedNodes(doc, summary);
  container.replaceChildren(...nodes);
}
