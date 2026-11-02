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
  SelectionCheck,
  SelectionStatus,
  VideoView,
} from "../core/popupView.ts";
import { startResultMessage } from "../core/sendMessages.ts";
import type { DisplayText } from "../core/summary.ts";
import type { CheckName, CheckState } from "../core/types.ts";
import { checkIcon } from "./icons.ts";
import { messageNodes } from "./message.ts";
import {
  loadingIndicator,
  noticeSummary,
  pollNotice,
  renderJobState,
  startFailure,
  stepsSummary,
  writeStatusSummary,
} from "./renderJob.ts";

export { messageNodes };

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
      container.replaceChildren(loadingIndicator(doc, catalog.popup.sending));
      return;
    case "start-failed":
      container.replaceChildren(startFailure(doc, state.failure, locale));
      return;
    case "loading":
      // A stopped poll waits for nothing, so its notice stands alone, as in
      // the viewer. The close-popup guidance still applies.
      container.replaceChildren(
        ...(state.notice.kind === "stopped"
          ? []
          : [loadingIndicator(doc, catalog.viewer.loading)]),
        ...pollNotice(doc, state.notice, locale),
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

/** The job's part of the popup's state summary. */
function popupJobSummary(
  job: Extract<PopupJobState, { kind: "job" }>["job"],
  locale: Locale,
): string[] {
  const catalog = catalogs[locale];
  switch (job.kind) {
    case "running":
      return [catalog.viewer.running, stepsSummary(job.steps, locale)];
    case "done":
      return [catalog.popup.articleReady.text];
    case "failed":
      return [catalog.failure[job.failure].text];
    case "lost":
      return [catalog.viewer.lost.text];
  }
}

/** The state summary's sentences: loading started or ended, the steps, completion or failure. */
function popupSummary(state: PopupJobState, locale: Locale): string[] {
  const catalog = catalogs[locale];
  switch (state.kind) {
    case "sending":
      return [catalog.popup.sending.text];
    case "start-failed":
      return [startResultMessage(state.failure, locale).text];
    case "loading":
      return [
        ...(state.notice.kind === "stopped"
          ? []
          : [catalog.viewer.loading.text]),
        ...noticeSummary(state.notice, locale),
      ];
    case "job":
      return [
        ...popupJobSummary(state.job, locale),
        ...noticeSummary(state.notice, locale),
      ];
  }
}

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
  const names = catalogs[locale].popup.primary;
  const action = primaryAction(state);
  primary.textContent =
    action === "open-viewer"
      ? names.openViewer
      : action === "retry"
        ? names.retry
        : names.generate;
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
  renderVideoSection(doc, elements.video, view.video);
  renderDetailsSection(doc, elements.details, view.details, locale);
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
