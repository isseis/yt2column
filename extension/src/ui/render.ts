import { catalogs } from "../core/i18n/catalog.ts";
import type { Message } from "../core/i18n/catalog.ts";
import type { Locale } from "../core/i18n/locale.ts";
import { expandPromptMessage, rejectionMessage } from "../core/messages.ts";
import {
  descriptionStates,
  primaryAction,
  selectionStates,
} from "../core/popupView.ts";
import type {
  CollectedView,
  DescriptionStatus,
  DetailsView,
  PopupJobState,
  PopupView,
  PrimaryAction,
  SelectionCheck,
  SelectionStatus,
  VideoView,
} from "../core/popupView.ts";
import type { DisplayText } from "../core/summary.ts";
import type { CheckName, CheckState } from "../core/types.ts";
import { checkIcon } from "./icons.ts";
import { messageNodes } from "./message.ts";
import {
  jobStateSummary,
  loadingIndicator,
  noticeSummary,
  renderJobState,
  startFailure,
  startFailureSummary,
  waitingNodes,
  waitingSummary,
  writeStatusSummary,
} from "./renderJob.ts";

/** Appended to a value that was shortened for display. */
const ellipsis = "\u2026";

/** The value shown for a field, marked with an ellipsis when shortened. */
function displayValue(value: string, truncated: boolean): string {
  return truncated ? `${value}${ellipsis}` : value;
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

/** The last video section and details drawn into each container. */
const drawnVideo = new WeakMap<HTMLElement, VideoView>();
const drawnDetails = new WeakMap<
  HTMLElement,
  { readonly view: DetailsView; readonly locale: Locale }
>();

/** Whether two display values are the same text with the same truncation. */
function sameText(left: DisplayText, right: DisplayText): boolean {
  return left.value === right.value && left.truncated === right.truncated;
}

/** Whether two video sections would be drawn the same. */
function sameVideo(left: VideoView, right: VideoView): boolean {
  if (!sameText(left.heading, right.heading)) {
    return false;
  }
  if (left.channelName === undefined || right.channelName === undefined) {
    return left.channelName === right.channelName;
  }
  return sameText(left.channelName, right.channelName);
}

/** Whether two details would be drawn the same. */
function sameDetails(left: DetailsView, right: DetailsView): boolean {
  return (
    sameText(left.url, right.url) && sameText(left.tabTitle, right.tabTitle)
  );
}

/**
 * Draws the video section only when its values changed. A job redraws once a
 * second to show the elapsed time; replacing an unchanged section would move
 * the focus off the details toggle and drop the reader's selection.
 */
function drawVideoSection(
  doc: Document,
  container: HTMLElement,
  view: VideoView,
): void {
  const last = drawnVideo.get(container);
  if (last !== undefined && sameVideo(last, view)) {
    return;
  }
  renderVideoSection(doc, container, view);
  drawnVideo.set(container, view);
}

/** Draws the details only when their values changed, for the same reason. */
function drawDetailsSection(
  doc: Document,
  container: HTMLElement,
  view: DetailsView,
  locale: Locale,
): void {
  // The toggle's name is in the locale, so a changed locale draws again too.
  const last = drawnDetails.get(container);
  if (
    last !== undefined &&
    last.locale === locale &&
    sameDetails(last.view, view)
  ) {
    return;
  }
  renderDetailsSection(doc, container, view, locale);
  drawnDetails.set(container, { view, locale });
}

/** The checks section's content for a job state: the state, then its guidance. */
function drawPopupState(
  container: HTMLElement,
  state: PopupJobState,
  elapsedMs: number,
  locale: Locale,
): void {
  const doc = container.ownerDocument;
  const catalog = catalogs[locale];
  switch (state.kind) {
    case "sending":
      container.replaceChildren(
        ...loadingIndicator(doc, catalog.popup.sending),
      );
      return;
    case "start-failed":
      container.replaceChildren(startFailure(doc, state.failure, locale));
      return;
    case "loading":
      // The close-popup guidance applies even to a stopped poll.
      container.replaceChildren(
        ...waitingNodes(doc, state.notice, locale),
        ...messageNodes(doc, catalog.popup.viewerNotice),
      );
      return;
    case "job": {
      renderJobState(container, state.job, state.notice, elapsedMs, locale);
      const guidance =
        state.job.kind === "running"
          ? catalog.popup.viewerNotice
          : state.job.kind === "done"
            ? catalog.popup.articleReady
            : undefined;
      if (guidance !== undefined) {
        container.append(...messageNodes(doc, guidance));
      }
      return;
    }
  }
}

/** The state summary's sentences: loading started or ended, the steps, completion or failure. */
function popupSummary(state: PopupJobState, locale: Locale): string[] {
  switch (state.kind) {
    case "sending":
      return [catalogs[locale].popup.sending.text];
    case "start-failed":
      return startFailureSummary(state.failure, locale);
    case "loading":
      return waitingSummary(state.notice, locale);
    case "job":
      return [
        ...jobStateSummary(
          state.job,
          [catalogs[locale].popup.articleReady.text],
          locale,
        ),
        ...noticeSummary(state.notice, locale),
      ];
  }
}

/** The catalog's button name for each action; a new action fails to compile here. */
const primaryNames: Readonly<
  Record<PrimaryAction, "generate" | "openViewer" | "retry">
> = {
  none: "generate",
  "open-viewer": "openViewer",
  retry: "retry",
};

/**
 * The primary button for a job state: its name follows the action, and only
 * an available action makes it pressable. With no action the button keeps the
 * pre-send name and stays disabled, as the design leaves it while the start
 * has no result.
 */
function applyJobPrimary(
  primary: HTMLButtonElement,
  state: PopupJobState,
  locale: Locale,
): void {
  const action = primaryAction(state);
  const name = catalogs[locale].popup.primary[primaryNames[action]];
  // A running job redraws each second; rewriting the same name of a focused
  // button would only churn its text node.
  if (primary.textContent !== name) {
    primary.textContent = name;
  }
  primary.disabled = action === "none";
}

/**
 * Draws the popup view into the skeleton. It replaces each container's
 * children, so drawing twice adds nothing, and shows the view's sections while
 * hiding the others. Every string is set with textContent, so a value read
 * from the page stays text. It never opens the details. A collected or
 * rejected view leaves the primary button disabled, whatever the skeleton
 * says: only the send path enables it, and only for a sendable selection. A
 * job view instead follows primaryAction, and its state summary is written
 * only when it changes, so the elapsed time's redraw is not announced.
 */
export function renderPopup(
  elements: PopupElements,
  view: PopupView,
  locale: Locale,
  elapsedMs = 0,
): void {
  const doc = elements.video.ownerDocument;
  const catalog = catalogs[locale];
  elements.action.hidden = false;
  elements.rejection.hidden = view.kind !== "rejected";
  if (view.kind === "rejected") {
    elements.video.hidden = true;
    elements.checks.hidden = true;
    elements.details.hidden = true;
    elements.video.replaceChildren();
    elements.checks.replaceChildren();
    elements.details.replaceChildren();
    drawnVideo.delete(elements.video);
    drawnDetails.delete(elements.details);
    elements.status.replaceChildren();
    elements.primary.textContent = catalog.primaryButton;
    elements.primary.disabled = true;
    renderRejectionSection(doc, elements.rejection, view, locale);
    return;
  }
  elements.video.hidden = false;
  elements.checks.hidden = false;
  elements.details.hidden = false;
  elements.rejection.replaceChildren();
  drawVideoSection(doc, elements.video, view.video);
  drawDetailsSection(doc, elements.details, view.details, locale);
  if (view.kind === "collected") {
    elements.status.replaceChildren();
    elements.primary.textContent = catalog.primaryButton;
    elements.primary.disabled = true;
    renderChecksSection(doc, elements.checks, view, locale);
    return;
  }
  drawPopupState(elements.checks, view.state, elapsedMs, locale);
  writeStatusSummary(elements.status, popupSummary(view.state, locale));
  applyJobPrimary(elements.primary, view.state, locale);
}
