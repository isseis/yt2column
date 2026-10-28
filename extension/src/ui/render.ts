import { catalogs } from "../core/i18n/catalog.ts";
import type { Message } from "../core/i18n/catalog.ts";
import { formatCount } from "../core/i18n/locale.ts";
import type { Locale } from "../core/i18n/locale.ts";
import { expandPromptMessage, rejectionMessage } from "../core/messages.ts";
import type { MetadataField, MetadataResult } from "../core/metadata.ts";
import { metadataDisplayLimit } from "../core/popupView.ts";
import { truncate } from "../core/summary.ts";
import type { OutcomeSummary, SummaryField } from "../core/types.ts";

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

// A record keyed by every SummaryField, so adding a field fails the type check
// here until it has a style; the key order is the display order.
const summaryStyles: Record<SummaryField, FieldStyle> = {
  title: { className: "summary-title", dirAuto: true },
  url: { className: "summary-url" },
  characters: { className: "summary-character-count" },
  lines: { className: "summary-line-count" },
  preview: { className: "summary-preview", dirAuto: true },
};

/** The collected fields, in display order; counts use the locale's number format. */
function collectedNodes(
  doc: Document,
  summary: Extract<OutcomeSummary, { kind: "collected" }>,
  locale: Locale,
): HTMLElement[] {
  const values: Record<SummaryField, string> = {
    title: displayValue(summary.title, summary.titleTruncated),
    url: displayValue(summary.url, summary.urlTruncated),
    characters: formatCount(summary.characterCount, locale),
    lines: formatCount(summary.lineCount, locale),
    preview: displayValue(summary.preview, summary.previewTruncated),
  };
  const labels = catalogs[locale].summaryLabels;
  return (Object.keys(summaryStyles) as SummaryField[]).map((name) =>
    field(doc, labels[name], values[name], summaryStyles[name]),
  );
}

/** The message text and, when present, its numbered steps; textContent only. */
export function messageNodes(doc: Document, message: Message): HTMLElement[] {
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

/** The rejection message and, when present, its numbered steps. */
function rejectedNodes(
  doc: Document,
  summary: Extract<OutcomeSummary, { kind: "rejected" }>,
  locale: Locale,
): HTMLElement[] {
  return messageNodes(doc, rejectionMessage(summary.reason, locale));
}

/**
 * Replaces the children of container with a text-only rendering of the
 * summary. Every string is set with textContent, so a value read from the
 * page is displayed as text and never parsed as HTML.
 */
export function renderSummary(
  container: HTMLElement,
  summary: OutcomeSummary,
  locale: Locale,
): void {
  const doc = container.ownerDocument;
  const nodes =
    summary.kind === "collected"
      ? collectedNodes(doc, summary, locale)
      : rejectedNodes(doc, summary, locale);
  container.replaceChildren(...nodes);
}

/** One metadata row's value-element class; its label is in the catalog. */
interface MetadataRow {
  readonly className: string;
}

// A record keyed by every MetadataField, so adding a field fails the type check
// here until it has a row; the key order is the display order.
const metadataRows: Record<MetadataField, MetadataRow> = {
  title: { className: "metadata-title" },
  channelName: { className: "metadata-channel" },
  description: { className: "metadata-description" },
};

/**
 * Appends the video metadata fields -- and the expand prompt when the
 * description is collapsed -- below whatever the container already holds.
 * renderSummary replaces the container's children, so this is called after it.
 * Values are set with textContent only and are truncated for display alone;
 * the values sent to the server are never truncated.
 */
export function renderMetadata(
  container: HTMLElement,
  result: MetadataResult,
  locale: Locale,
): void {
  const doc = container.ownerDocument;
  const labels = catalogs[locale].metadataLabels;
  const nodes: HTMLElement[] = [];
  for (const name of Object.keys(metadataRows) as MetadataField[]) {
    const value = result.values[name];
    if (value === undefined) {
      continue;
    }
    const row = metadataRows[name];
    const shortened = truncate(value, metadataDisplayLimit);
    nodes.push(
      field(
        doc,
        labels[name],
        displayValue(shortened.value, shortened.truncated),
        {
          className: row.className,
          dirAuto: true,
        },
      ),
    );
  }
  if (result.showExpandPrompt) {
    nodes.push(...messageNodes(doc, expandPromptMessage(locale)));
  }
  container.append(...nodes);
}
