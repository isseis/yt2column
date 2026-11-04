// Draws a job's state and writes its state summary: the parts the popup's
// checks section and status region and the viewer's state section share. Every
// string is set with textContent, so a value that reached this layer as a
// string is never parsed as HTML; the check that made a reply unexpected is
// never drawn.

import { catalogs } from "../core/i18n/catalog.ts";
import type { Message } from "../core/i18n/catalog.ts";
import { formatDuration, formatUsd } from "../core/i18n/locale.ts";
import type { Locale } from "../core/i18n/locale.ts";
import type {
  JobDisplay,
  PollNotice,
  StartFailure,
  ViewerState,
} from "../core/jobView.ts";
import type { PopupJobState } from "../core/popupView.ts";
import { stepNames } from "../core/response.ts";
import type { Article, RequestId, StepName, Steps } from "../core/response.ts";
import {
  startResultMessage,
  statusNoticeMessage,
} from "../core/sendMessages.ts";
import { loadingIcon, stepIcon } from "./icons.ts";
import { messageNodes } from "./message.ts";

/**
 * Whether a running job's elapsed time is drawn. The popup and a viewer draw
 * the time since the send; a viewer of an existing article of the same video
 * does not, since that article began before this send.
 */
export type ElapsedTime =
  { readonly kind: "shown"; readonly ms: number } | { readonly kind: "hidden" };

// The steps in the order the server runs them, which is the order of the
// record every step name is checked against.
const stepOrder = Object.keys(stepNames) as StepName[];

/**
 * The generation steps as a list, one item per step in the order the server
 * runs them. Each item holds, in the order assistive technology reads them,
 * the symbol (aria-hidden), the step's name and the state's word (visually
 * hidden), like a check row of the popup.
 */
function stepChecklist(
  doc: Document,
  steps: Steps,
  locale: Locale,
): HTMLOListElement {
  const catalog = catalogs[locale];
  const list = doc.createElement("ol");
  list.className = "step-list";
  for (const name of stepOrder) {
    const state = steps[name];
    const item = doc.createElement("li");
    item.className = "step-row";
    const stepName = doc.createElement("span");
    stepName.className = "step-name";
    stepName.textContent = catalog.stepNames[name];
    const stateWord = doc.createElement("span");
    stateWord.className = "step-state visually-hidden";
    stateWord.textContent = catalog.stepStates[state];
    item.append(stepIcon(doc, state), stepName, stateWord);
    list.append(item);
  }
  return list;
}

/** A list of names and their values, such as the elapsed time or the cost. */
function factList(
  doc: Document,
  className: string,
  facts: readonly (readonly [string, string])[],
): HTMLDListElement {
  const list = doc.createElement("dl");
  list.className = `job-facts ${className}`;
  for (const [name, value] of facts) {
    const term = doc.createElement("dt");
    term.textContent = name;
    const definition = doc.createElement("dd");
    definition.textContent = value;
    list.append(term, definition);
  }
  return list;
}

/**
 * The elapsed time since the send. It carries no aria-live and the caller
 * keeps it outside every status region, so redrawing it each second is not
 * read out.
 */
function elapsedTime(
  doc: Document,
  elapsedMs: number,
  locale: Locale,
): HTMLDListElement {
  return factList(doc, "job-elapsed", [
    [catalogs[locale].viewer.elapsed, formatDuration(elapsedMs, locale)],
  ]);
}

/** The finished article's time taken and its estimated cost. */
function articleFacts(
  doc: Document,
  article: Article,
  locale: Locale,
): HTMLDListElement {
  const viewer = catalogs[locale].viewer;
  const cost =
    article.cost.state === "known"
      ? formatUsd(article.cost.usd, locale)
      : viewer.costUnknown;
  return factList(doc, "job-result", [
    [viewer.duration, formatDuration(article.durationMs, locale)],
    [viewer.cost, cost],
  ]);
}

/**
 * The loading mark and the sentence that says which wait it is, then the
 * message's numbered steps. The steps are siblings of the mark-and-sentence
 * row, not flex items beside it, so a step wraps under the row.
 */
export function loadingIndicator(
  doc: Document,
  message: Message,
): [HTMLDivElement, ...HTMLElement[]] {
  const row = doc.createElement("div");
  row.className = "loading";
  const [text, ...steps] = messageNodes(doc, message);
  row.append(loadingIcon(doc), text);
  return [row, ...steps];
}

/**
 * The steps and their states as one list for the state summary, such as
 * "Draft Done, Shorten In progress, and Review Not started"; the locale's
 * list separators keep one step's pair apart from the next.
 */
export function stepsSummary(steps: Steps, locale: Locale): string {
  const catalog = catalogs[locale];
  return new Intl.ListFormat(locale, { type: "conjunction" }).format(
    stepOrder.map(
      (name) => `${catalog.stepNames[name]} ${catalog.stepStates[steps[name]]}`,
    ),
  );
}

/**
 * The job's part of a state summary. The done sentences differ between the
 * popup and the viewer, so the caller supplies them; every other kind reads
 * the same catalog entries in both pages.
 */
export function jobStateSummary(
  job: JobDisplay,
  done: readonly string[],
  locale: Locale,
): string[] {
  const catalog = catalogs[locale];
  switch (job.kind) {
    case "running":
      return [catalog.viewer.running, stepsSummary(job.steps, locale)];
    case "done":
      return [...done];
    case "failed":
      return [catalog.failure[job.failure].text];
    case "lost":
      return [catalog.viewer.lost.text];
  }
}

/** The summary while a key waits for its first status, plus a failed exchange's notice. */
export function waitingSummary(notice: PollNotice, locale: Locale): string[] {
  return [
    ...(notice.kind === "stopped"
      ? []
      : [catalogs[locale].viewer.loading.text]),
    ...noticeSummary(notice, locale),
  ];
}

/** The summary of a start that gave no key. */
export function startFailureSummary(
  failure: StartFailure,
  locale: Locale,
): string[] {
  return [startResultMessage(failure, locale).text];
}

/** The server's request id as text, one click away from a full copy. */
export function requestIdNode(
  doc: Document,
  requestId: RequestId,
): HTMLParagraphElement {
  const id = doc.createElement("p");
  id.className = "send-request-id";
  id.textContent = requestId;
  return id;
}

/** A message followed, when the server gave one, by its request id as text. */
function messageWithRequestId(
  doc: Document,
  className: string,
  message: Message,
  requestId: RequestId | undefined,
): HTMLDivElement {
  const block = doc.createElement("div");
  block.className = className;
  block.append(...messageNodes(doc, message));
  if (requestId !== undefined) {
    block.append(requestIdNode(doc, requestId));
  }
  return block;
}

/** A start that gave no key: its reason and steps, and a rejection's request id. */
export function startFailure(
  doc: Document,
  failure: StartFailure,
  locale: Locale,
): HTMLDivElement {
  return messageWithRequestId(
    doc,
    "failure",
    startResultMessage(failure, locale),
    failure.kind === "rejected" ? failure.requestId : undefined,
  );
}

/** The notice of a failed status exchange, or nothing when there is none. */
export function pollNotice(
  doc: Document,
  notice: PollNotice,
  locale: Locale,
): HTMLDivElement[] {
  if (notice.kind === "none") {
    return [];
  }
  const { outcome } = notice;
  return [
    messageWithRequestId(
      doc,
      "job-notice",
      statusNoticeMessage(outcome, locale),
      outcome.kind === "rejected" ? outcome.requestId : undefined,
    ),
  ];
}

/**
 * The nodes while a key waits for its first status: the loading mark, then a
 * failed exchange's notice. A stopped poll waits for nothing, so its notice
 * stands alone.
 */
export function waitingNodes(
  doc: Document,
  notice: PollNotice,
  locale: Locale,
): HTMLElement[] {
  return [
    ...(notice.kind === "stopped"
      ? []
      : loadingIndicator(doc, catalogs[locale].viewer.loading)),
    ...pollNotice(doc, notice, locale),
  ];
}

/** The notice's part of a state summary: that the status could not be read. */
export function noticeSummary(notice: PollNotice, locale: Locale): string[] {
  return notice.kind === "none"
    ? []
    : [statusNoticeMessage(notice.outcome, locale).text];
}

/** Writes the summary, one paragraph per sentence, only when it changed. */
export function writeStatusSummary(
  region: HTMLElement,
  sentences: readonly string[],
): void {
  const shown = Array.from(region.children, (child) => child.textContent);
  if (
    shown.length === sentences.length &&
    shown.every((text, i) => text === sentences[i])
  ) {
    return;
  }
  const doc = region.ownerDocument;
  region.replaceChildren(
    ...sentences.map((sentence) => {
      const paragraph = doc.createElement("p");
      paragraph.textContent = sentence;
      return paragraph;
    }),
  );
}

/** The job's own nodes: the steps and, by kind, the elapsed time, the facts or the failure. */
function jobNodes(
  doc: Document,
  job: JobDisplay,
  elapsed: ElapsedTime,
  locale: Locale,
): HTMLElement[] {
  const catalog = catalogs[locale];
  switch (job.kind) {
    case "running": {
      const checklist = stepChecklist(doc, job.steps, locale);
      return elapsed.kind === "shown"
        ? [checklist, elapsedTime(doc, elapsed.ms, locale)]
        : [checklist];
    }
    case "done":
      return [
        stepChecklist(doc, job.steps, locale),
        articleFacts(doc, job.article, locale),
      ];
    case "failed":
      return [
        stepChecklist(doc, job.steps, locale),
        messageWithRequestId(
          doc,
          "failure",
          catalog.failure[job.failure],
          undefined,
        ),
      ];
    case "lost":
      return [
        messageWithRequestId(doc, "failure", catalog.viewer.lost, undefined),
      ];
  }
}

/**
 * Replaces the container's children with the job's state and the notice of a
 * failed status exchange, if any. A shown elapsed time is drawn only while the
 * job runs. Drawing again replaces everything, so nothing from an earlier
 * state stays.
 */
export function renderJobState(
  container: HTMLElement,
  job: JobDisplay,
  notice: PollNotice,
  elapsed: ElapsedTime,
  locale: Locale,
): void {
  const doc = container.ownerDocument;
  container.replaceChildren(
    ...jobNodes(doc, job, elapsed, locale),
    ...pollNotice(doc, notice, locale),
  );
}

/** The state last drawn into each state container, and the nodes it drew. */
const drawnStates = new WeakMap<
  HTMLElement,
  { readonly key: string; readonly nodes: readonly Node[] }
>();

/**
 * Draws a state into its container only when the state, the locale or
 * whether the elapsed time is shown changed, or when something else replaced
 * what was drawn. Otherwise only the elapsed time's value is written, so a
 * redraw each second while a job runs keeps a selection (such as a request id
 * being copied) and the focus inside the container. The key is the state's
 * JSON: the state is plain data, and values that serialize the same draw the
 * same.
 */
export function drawStateOnChange(
  container: HTMLElement,
  state: ViewerState | PopupJobState,
  elapsed: ElapsedTime,
  locale: Locale,
  draw: () => void,
): void {
  const key = JSON.stringify([locale, state, elapsed.kind]);
  const last = drawnStates.get(container);
  const current = Array.from(container.childNodes);
  if (
    last !== undefined &&
    last.key === key &&
    last.nodes.length === current.length &&
    last.nodes.every((node, i) => node === current[i])
  ) {
    const shown = container.querySelector(".job-elapsed dd");
    if (elapsed.kind === "shown") {
      const text = formatDuration(elapsed.ms, locale);
      if (shown !== null && shown.textContent !== text) {
        shown.textContent = text;
      }
    }
    return;
  }
  draw();
  drawnStates.set(container, { key, nodes: Array.from(container.childNodes) });
}
