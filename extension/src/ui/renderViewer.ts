// Draws the viewer page into viewer.html's skeleton: the header from the
// viewer URL's values, the state, the article and its operations, and the two
// status regions. Every string is set with textContent, so a value read from
// the URL or the server stays text.

import { isAcceptedWatchUrl } from "../core/acceptedUrl.ts";
import { catalogs } from "../core/i18n/catalog.ts";
import type { Message } from "../core/i18n/catalog.ts";
import type { Locale } from "../core/i18n/locale.ts";
import { publishControl } from "../core/jobView.ts";
import type {
  JobDisplay,
  PollNotice,
  PublishOpNotice,
  PublishRow,
  ViewerState,
} from "../core/jobView.ts";
import {
  publishResultMessage,
  startResultMessage,
  statusNoticeMessage,
} from "../core/sendMessages.ts";
import type { ViewerParams } from "../core/viewerUrl.ts";
import { renderArticleBody } from "./articleBody.ts";
import { messageNodes } from "./render.ts";
import {
  loadingIndicator,
  renderJobState,
  requestIdNode,
  startFailure,
  stepsSummary,
} from "./renderJob.ts";

/** The skeleton's elements; the viewer finds them in viewer.html by id. */
export interface ViewerElements {
  readonly heading: HTMLElement;
  readonly channel: HTMLElement;
  readonly videoLink: HTMLAnchorElement;
  readonly state: HTMLElement;
  /** The state summary's status region, outside the state section. */
  readonly stateStatus: HTMLElement;
  readonly article: HTMLElement;
  readonly actions: HTMLElement;
  readonly publish: HTMLButtonElement;
  readonly publishRow: HTMLElement;
  readonly save: HTMLButtonElement;
  readonly copy: HTMLButtonElement;
  /** The operation result's status region, among the operations. */
  readonly actionStatus: HTMLElement;
}

/** The values the header shows, all from the viewer URL. */
export type ViewerHeader = Pick<
  ViewerParams,
  "heading" | "channelName" | "url"
>;

/** What one draw of the viewer shows. */
export interface ViewerView {
  readonly state: ViewerState;
  /** The time since the send; drawn only while the job runs. */
  readonly elapsedMs: number;
  /** A publish operation is in flight or waits for its first status. */
  readonly publishing: boolean;
}

/** The result of the last operation, shown in the operation result's region. */
export type ActionNotice =
  | PublishOpNotice
  | { readonly kind: "saved" }
  | { readonly kind: "copied" }
  | { readonly kind: "copy-failed" };

/**
 * Draws the heading, the channel name and the link to the video. Without the
 * URL's values (a link that failed the URL check) all three stay hidden. The
 * link is shown only for a URL that passes isAcceptedWatchUrl: its href is
 * that URL's own text, never built from the article, and it opens a new tab
 * that gets neither the opener nor the referrer.
 */
export function renderViewerHeader(
  elements: ViewerElements,
  header: ViewerHeader | undefined,
  locale: Locale,
): void {
  const { heading, channel, videoLink } = elements;
  heading.hidden = header === undefined;
  heading.textContent = header?.heading ?? "";
  channel.hidden = header?.channelName === undefined;
  channel.textContent = header?.channelName ?? "";
  if (header === undefined || !isAcceptedWatchUrl(header.url)) {
    videoLink.hidden = true;
    videoLink.removeAttribute("href");
    videoLink.textContent = "";
    return;
  }
  videoLink.hidden = false;
  videoLink.href = header.url;
  videoLink.target = "_blank";
  videoLink.rel = "noopener noreferrer";
  videoLink.textContent = catalogs[locale].viewer.videoLink;
}

/** A done or lost job: the jobs whose article and publish button can show. */
type PublishableJob = Extract<JobDisplay, { kind: "done" | "lost" }>;

/** The view's job when it is done or lost; undefined for every other state. */
function publishableJob(state: ViewerState): PublishableJob | undefined {
  if (state.kind !== "job") {
    return undefined;
  }
  switch (state.job.kind) {
    case "done":
    case "lost":
      return state.job;
    case "running":
    case "failed":
      return undefined;
  }
}

/** The messages beside the publish button for the row that applies. */
function publishRowMessages(row: PublishRow, locale: Locale): Message[] {
  const catalog = catalogs[locale];
  switch (row.row) {
    case "lost":
      // The state section shows the steps to send again; the row says only
      // why the button cannot be pressed.
      return [{ text: catalog.viewer.lost.text, steps: [] }];
    case "posted":
      return [catalog.publishStates.posted];
    case "posting":
      return [catalog.publishStates.posting];
    case "unpostable":
      return [
        catalog.publishStates.unpostable,
        catalog.publishReasons[row.reason],
      ];
    case "webhook-not-configured":
      return [catalog.viewer.webhookNotConfigured];
    case "failed":
      return [catalog.publishStates.failed, catalog.publishReasons[row.reason]];
    case "not-posted":
      return [catalog.publishStates.not_posted];
  }
}

/** The state section's content for one state. */
function drawState(
  container: HTMLElement,
  view: ViewerView,
  locale: Locale,
): void {
  const doc = container.ownerDocument;
  const viewer = catalogs[locale].viewer;
  const { state } = view;
  switch (state.kind) {
    case "invalid-link": {
      const block = doc.createElement("div");
      block.className = "failure";
      block.append(...messageNodes(doc, viewer.invalidLink));
      container.replaceChildren(block);
      return;
    }
    case "sending":
      container.replaceChildren(loadingIndicator(doc, viewer.sending));
      return;
    case "loading":
      container.replaceChildren(loadingIndicator(doc, viewer.loading));
      return;
    case "start-failed":
      container.replaceChildren(startFailure(doc, state.failure, locale));
      return;
    case "job":
      renderJobState(
        container,
        state.job,
        state.notice,
        view.elapsedMs,
        locale,
      );
      return;
  }
}

/** The job's part of the summary: what changed for assistive technology. */
function jobSummary(job: JobDisplay, locale: Locale): string[] {
  const catalog = catalogs[locale];
  switch (job.kind) {
    case "running":
      return [catalog.viewer.running, stepsSummary(job.steps, locale)];
    case "done":
      // The same sentence the popup shows when the article is ready.
      return [catalog.popup.articleReady.text];
    case "failed":
      return [catalog.failure[job.failure].text];
    case "lost":
      return [catalog.viewer.lost.text];
  }
}

/** The notice's part of the summary: that the status could not be read. */
function noticeSummary(notice: PollNotice, locale: Locale): string[] {
  return notice.kind === "none"
    ? []
    : [statusNoticeMessage(notice.outcome, locale).text];
}

/**
 * The state summary: loading started or ended, a step changed, the job
 * finished or failed, the status could not be read. It holds no elapsed time,
 * so the same state gives the same summary whatever the clock says.
 */
function stateSummary(state: ViewerState, locale: Locale): string {
  const viewer = catalogs[locale].viewer;
  switch (state.kind) {
    case "invalid-link":
      return viewer.invalidLink.text;
    case "sending":
      return viewer.sending.text;
    case "loading":
      return viewer.loading.text;
    case "start-failed":
      return startResultMessage(state.failure, locale).text;
    case "job":
      return [
        ...jobSummary(state.job, locale),
        ...noticeSummary(state.notice, locale),
      ].join(" ");
  }
}

/**
 * Draws the state, the article and the operations. The state summary's
 * region is written only when its text changes, so a poll that repeats the
 * same result, or the elapsed time's redraw, is not read out again. The
 * article is replaced only when its text changes, so a redraw keeps the
 * reader's selection. The operations are shown with an article: save and copy
 * can always be pressed, and the publish button only when publishControl's
 * row allows it. Focus is never moved.
 */
export function renderViewer(
  elements: ViewerElements,
  view: ViewerView,
  locale: Locale,
): void {
  const doc = elements.state.ownerDocument;
  const catalog = catalogs[locale];
  drawState(elements.state, view, locale);
  const summary = stateSummary(view.state, locale);
  if (elements.stateStatus.textContent !== summary) {
    elements.stateStatus.textContent = summary;
  }

  // The article is a done job's, or the one a lost job kept.
  const job = publishableJob(view.state);
  const article = job?.kind === "done" ? job.article : job?.kept;
  if (job === undefined || article === undefined) {
    elements.article.hidden = true;
    elements.article.replaceChildren();
    elements.actions.hidden = true;
    elements.publish.disabled = true;
    elements.save.disabled = true;
    elements.copy.disabled = true;
    return;
  }
  elements.article.hidden = false;
  if (elements.article.textContent !== article.markdown) {
    elements.article.replaceChildren(renderArticleBody(doc, article.markdown));
  }

  const control = publishControl(job, view.publishing);
  elements.actions.hidden = false;
  elements.publish.textContent = catalog.viewer.publish;
  elements.publish.disabled = !control.enabled;
  elements.publishRow.replaceChildren(
    ...publishRowMessages(control.row, locale).flatMap((message) =>
      messageNodes(doc, message),
    ),
  );
  elements.save.textContent = catalog.viewer.save;
  elements.save.disabled = false;
  elements.copy.textContent = catalog.viewer.copy;
  elements.copy.disabled = false;
}

/** The message for an operation's result; none for no result. */
function actionMessage(
  notice: ActionNotice,
  locale: Locale,
): Message | undefined {
  const viewer = catalogs[locale].viewer;
  switch (notice.kind) {
    case "none":
      return undefined;
    case "rejected":
    case "not-sent":
      return publishResultMessage(notice, locale);
    case "unknown":
      return publishResultMessage(notice.outcome, locale);
    case "saved":
      return { text: viewer.saved, steps: [] };
    case "copied":
      return { text: viewer.copied, steps: [] };
    case "copy-failed":
      return { text: viewer.copyFailed, steps: [] };
  }
}

/**
 * Writes the last operation's result into its status region beside the
 * operations, with a rejection's request id; focus stays where it is.
 */
export function renderActionNotice(
  elements: ViewerElements,
  notice: ActionNotice,
  locale: Locale,
): void {
  const doc = elements.actionStatus.ownerDocument;
  const message = actionMessage(notice, locale);
  if (message === undefined) {
    elements.actionStatus.replaceChildren();
    return;
  }
  const nodes: HTMLElement[] = messageNodes(doc, message);
  if (notice.kind === "rejected") {
    nodes.push(requestIdNode(doc, notice.requestId));
  }
  elements.actionStatus.replaceChildren(...nodes);
}
