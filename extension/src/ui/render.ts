import { catalogs } from "../core/i18n/catalog.ts";
import type { Message } from "../core/i18n/catalog.ts";
import type { Locale } from "../core/i18n/locale.ts";
import { expandPromptMessage, rejectionMessage } from "../core/messages.ts";
import { descriptionStates, selectionStates } from "../core/popupView.ts";
import type {
  CollectedView,
  DescriptionStatus,
  DetailsView,
  PopupView,
  SelectionCheck,
  SelectionStatus,
  VideoView,
} from "../core/popupView.ts";
import type { DisplayText } from "../core/summary.ts";
import type { CheckName, CheckState } from "../core/types.ts";
import { checkIcon } from "./icons.ts";

/** Appended to a value that was shortened for display. */
const ellipsis = "\u2026";

/** The value shown for a field, marked with an ellipsis when shortened. */
function displayValue(value: string, truncated: boolean): string {
  return truncated ? `${value}${ellipsis}` : value;
}

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

/** The primary button and the status region; all the send path touches. */
export interface ActionControls {
  readonly primary: HTMLButtonElement;
  readonly status: HTMLElement;
}

/** The skeleton's elements; startPopup finds them in popup.html by id. */
export interface PopupElements extends ActionControls {
  readonly video: HTMLElement;
  readonly checks: HTMLElement;
  readonly rejection: HTMLElement;
  readonly action: HTMLElement;
  readonly details: HTMLDetailsElement;
}

/** A paragraph with its class and text set; the caller adds any dir attribute. */
function valueParagraph(
  doc: Document,
  className: string,
  text: string,
): HTMLParagraphElement {
  const element = doc.createElement("p");
  element.className = className;
  element.textContent = text;
  return element;
}

/** The video section: the heading, then the channel name only when it is sent. */
function renderVideoSection(
  doc: Document,
  container: HTMLElement,
  view: VideoView,
): void {
  const heading = doc.createElement("h1");
  heading.className = "video-heading";
  heading.setAttribute("dir", "auto");
  heading.textContent = displayValue(
    view.heading.value,
    view.heading.truncated,
  );
  if (view.channelName === undefined) {
    container.replaceChildren(heading);
    return;
  }
  const channelName = valueParagraph(
    doc,
    "channel-name",
    displayValue(view.channelName.value, view.channelName.truncated),
  );
  channelName.setAttribute("dir", "auto");
  container.replaceChildren(heading, channelName);
}

/**
 * One check row. The parts come in the order assistive technology reads them:
 * the symbol (aria-hidden), the row's name, the state's word (visually hidden),
 * then the content. Only a warning row carries the warning class, so only that
 * row's background differs.
 */
function checkRow(
  doc: Document,
  name: CheckName,
  state: CheckState,
  locale: Locale,
  content: readonly HTMLElement[],
): HTMLLIElement {
  const catalog = catalogs[locale];
  const row = doc.createElement("li");
  row.className = "check-row";
  if (state === "warning") {
    row.classList.add("check-row-warning");
  }
  const rowName = doc.createElement("span");
  rowName.className = "check-name";
  rowName.textContent = catalog.checkNames[name];
  const stateWord = doc.createElement("span");
  stateWord.className = "check-state visually-hidden";
  stateWord.textContent = catalog.checkStates[state];
  const body = doc.createElement("div");
  body.className = "check-content";
  body.append(...content);
  row.append(checkIcon(doc, state), rowName, stateWord, body);
  return row;
}

/**
 * The selection row's content: the counts on one line, the head preview and,
 * unless the selection fits in it, the tail preview; when the body is over the
 * limit, the reason and its steps. The ellipsis marks the side that was cut.
 */
function selectionContent(
  doc: Document,
  check: SelectionCheck,
  status: SelectionStatus,
  locale: Locale,
): HTMLElement[] {
  const catalog = catalogs[locale];
  const head = valueParagraph(
    doc,
    "preview-head",
    displayValue(check.head.value, check.head.truncated),
  );
  head.setAttribute("dir", "auto");
  const nodes: HTMLElement[] = [
    valueParagraph(
      doc,
      "check-size",
      catalog.selectionSize(check.characterCount, check.lineCount),
    ),
    head,
  ];
  if (check.tail !== undefined) {
    const tail = doc.createElement("div");
    tail.className = "preview-tail";
    const value = doc.createElement("span");
    value.className = "preview-tail-value";
    value.setAttribute("dir", "auto");
    value.textContent = check.tail.truncated
      ? `${ellipsis}${check.tail.value}`
      : check.tail.value;
    tail.append(value);
    nodes.push(tail);
  }
  if (status.kind === "blocked") {
    nodes.push(...messageNodes(doc, catalog.preflight[status.reason]));
  }
  return nodes;
}

/**
 * The description row's message. The description's own text never reaches the
 * view, so only its state and, when it is full, its length are drawn.
 */
function descriptionMessage(
  status: DescriptionStatus,
  locale: Locale,
): Message {
  const catalog = catalogs[locale];
  switch (status.kind) {
    case "full":
      return {
        text: catalog.descriptionFull(status.characterCount),
        steps: [],
      };
    case "collapsed":
      return expandPromptMessage(locale);
    case "not-sent":
      return catalog.descriptionNotSent;
  }
}

/** The checks section: the selection row and the description row, in that order. */
function renderChecksSection(
  doc: Document,
  container: HTMLElement,
  view: CollectedView,
  locale: Locale,
): void {
  const list = doc.createElement("ul");
  list.className = "check-list";
  list.append(
    checkRow(
      doc,
      "selection",
      selectionStates[view.selection.status.kind],
      locale,
      selectionContent(doc, view.selection, view.selection.status, locale),
    ),
    checkRow(
      doc,
      "description",
      descriptionStates[view.description.status.kind],
      locale,
      messageNodes(doc, descriptionMessage(view.description.status, locale)),
    ),
  );
  container.replaceChildren(list);
}

/**
 * The rejection card. Its name is the reason's sentence: the first node of the
 * message gets the id the skeleton's aria-labelledby points at.
 */
function renderRejectionSection(
  doc: Document,
  container: HTMLElement,
  view: Extract<PopupView, { kind: "rejected" }>,
  locale: Locale,
): void {
  const [text, ...rest] = messageNodes(
    doc,
    rejectionMessage(view.reason, locale),
  );
  text.id = "rejection-text";
  container.replaceChildren(text, ...rest);
}

/** One name and its value in the details; the value is text only. */
function detailRow(
  doc: Document,
  name: string,
  value: DisplayText,
): HTMLElement[] {
  const term = doc.createElement("dt");
  term.className = "detail-name";
  term.textContent = name;
  const definition = doc.createElement("dd");
  definition.className = "detail-value";
  definition.setAttribute("dir", "auto");
  definition.textContent = displayValue(value.value, value.truncated);
  return [term, definition];
}

/** The details: the toggle's name, then the URL and the tab title. */
function renderDetailsSection(
  doc: Document,
  container: HTMLElement,
  view: DetailsView,
  locale: Locale,
): void {
  const names = catalogs[locale].details;
  const summary = doc.createElement("summary");
  summary.className = "details-summary";
  summary.textContent = names.summary;
  const list = doc.createElement("dl");
  list.className = "details-list";
  list.append(
    ...detailRow(doc, names.url, view.url),
    ...detailRow(doc, names.tabTitle, view.tabTitle),
  );
  container.replaceChildren(summary, list);
}

/**
 * Draws the popup view into the skeleton. It replaces each container's
 * children, so drawing twice adds nothing, and shows the view's sections while
 * hiding the others; a rejected view also disables the primary button. Every string is set with textContent, so a value read
 * from the page stays text. It never opens the details and never removes the
 * primary button's disabled attribute: the send path does that, and only for a
 * sendable selection.
 */
export function renderPopup(
  elements: PopupElements,
  view: PopupView,
  locale: Locale,
): void {
  const doc = elements.video.ownerDocument;
  const collected = view.kind === "collected";
  elements.video.hidden = !collected;
  elements.checks.hidden = !collected;
  elements.details.hidden = !collected;
  elements.rejection.hidden = collected;
  elements.action.hidden = false;
  elements.primary.textContent = catalogs[locale].primaryButton;
  if (!collected) {
    elements.primary.disabled = true;
  }
  if (view.kind === "collected") {
    elements.rejection.replaceChildren();
    renderVideoSection(doc, elements.video, view.video);
    renderChecksSection(doc, elements.checks, view, locale);
    renderDetailsSection(doc, elements.details, view.details, locale);
    return;
  }
  elements.video.replaceChildren();
  elements.checks.replaceChildren();
  elements.details.replaceChildren();
  renderRejectionSection(doc, elements.rejection, view, locale);
}
