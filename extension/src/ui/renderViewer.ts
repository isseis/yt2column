// Draws the viewer page into viewer.html's skeleton: the header from the
// viewer URL's values, the existing-article notice, the state, the article
// and its operations, and the two status regions. Every string is set with textContent, so a value read from
// the URL or the server stays text.

import { isAcceptedWatchUrl } from "../core/acceptedUrl.ts";
import { catalogs } from "../core/i18n/catalog.ts";
import type { Message } from "../core/i18n/catalog.ts";
import type { Locale } from "../core/i18n/locale.ts";
import { jobArticle } from "../core/jobView.ts";
import type { JobDisplay, ViewerState } from "../core/jobView.ts";
import type { Article } from "../core/response.ts";
import type { ViewerParams } from "../core/viewerUrl.ts";
import { drawArticleBody } from "./articleBody.ts";
import { messageNodes } from "./message.ts";
import {
  drawStateOnChange,
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
import type { ElapsedTime } from "./renderJob.ts";

/** The skeleton's elements; the viewer finds them in viewer.html by id. */
export interface ViewerElements {
  readonly heading: HTMLElement;
  readonly channel: HTMLElement;
  readonly videoLink: HTMLAnchorElement;
  /** The notice of an existing article of the same video, under the header. */
  readonly existing: HTMLElement;
  readonly state: HTMLElement;
  /** The state summary's status region, outside the state section. */
  readonly stateStatus: HTMLElement;
  readonly article: HTMLElement;
  readonly actions: HTMLElement;
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
  /**
   * The time since the send; drawn only while the job runs, and never when
   * existing.
   */
  readonly elapsedMs: number;
  /**
   * The viewer shows an existing article of the same video (its URL holds
   * e=1): the notice is shown, and a running job's elapsed time is not.
   */
  readonly existing: boolean;
}

/** The result of the last operation, shown in the operation result's region. */
export type ActionNotice =
  | { readonly kind: "none" }
  | { readonly kind: "saved" }
  | { readonly kind: "copied"; readonly mentionsNeutralized: boolean }
  | { readonly kind: "copy-failed" };

/**
 * Draws the heading, the channel name and the link to the video. Without the
 * URL's values (a link that failed the URL check) all three stay hidden. The
 * link is shown only for a URL that passes isAcceptedWatchUrl: its href is
 * that URL's own text, never built from the article, and it opens a new tab
 * that gets neither the opener nor the referrer.
 *
 * Once the article is done (article is the shown one), the heading and the
 * channel name are the server's neutralized values, shown as they are, so a
 * selection of the whole page pastes no mention: an empty video title keeps
 * the URL's heading (the start sent no title), and an empty channel name hides
 * the line. Before that they are the URL's values.
 */
export function renderViewerHeader(
  elements: ViewerElements,
  header: ViewerHeader | undefined,
  locale: Locale,
  article: Article | undefined,
): void {
  const { heading, channel, videoLink } = elements;
  const shown =
    header === undefined ? undefined : headerValues(header, article);
  heading.hidden = shown === undefined;
  heading.textContent = shown?.heading ?? "";
  channel.hidden = shown?.channelName === undefined;
  channel.textContent = shown?.channelName ?? "";
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

/**
 * The heading and the channel name to show: the done article's neutralized
 * values, with the URL's heading for an empty video title and no channel
 * line for an empty channel name, or the URL's values before the article.
 */
function headerValues(
  header: ViewerHeader,
  article: Article | undefined,
): { readonly heading: string; readonly channelName: string | undefined } {
  if (article === undefined) {
    return { heading: header.heading, channelName: header.channelName };
  }
  return {
    heading: article.videoTitle === "" ? header.heading : article.videoTitle,
    channelName: article.channelName === "" ? undefined : article.channelName,
  };
}

/** Whether the view's running job shows its elapsed time. */
function viewElapsed(view: ViewerView): ElapsedTime {
  return view.existing
    ? { kind: "hidden" }
    : { kind: "shown", ms: view.elapsedMs };
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
      container.replaceChildren(...loadingIndicator(doc, viewer.sending));
      return;
    case "loading":
      container.replaceChildren(...waitingNodes(doc, state.notice, locale));
      return;
    case "start-failed":
      container.replaceChildren(startFailure(doc, state.failure, locale));
      return;
    case "job":
      renderJobState(
        container,
        state.job,
        state.notice,
        viewElapsed(view),
        locale,
      );
      return;
  }
}

/** The job's part of the summary: what changed for assistive technology. */
function jobSummary(job: JobDisplay, locale: Locale): string[] {
  // A done job has the same sentence the popup shows when the article is ready.
  return jobStateSummary(
    job,
    [catalogs[locale].popup.articleReady.text],
    locale,
  );
}

/**
 * The state summary's sentences: loading started or ended, a step changed,
 * the job finished or failed, the status could not be read, after the existing-article notice's sentence when it is shown.
 * They hold no elapsed time, so the same state gives the same summary
 * whatever the clock says.
 */
function stateSummary(view: ViewerView, locale: Locale): string[] {
  const existing = view.existing ? [catalogs[locale].viewer.existing.text] : [];
  return [...existing, ...stateOwnSummary(view.state, locale)];
}

/** The state's own part of the state summary. */
function stateOwnSummary(state: ViewerState, locale: Locale): string[] {
  const viewer = catalogs[locale].viewer;
  switch (state.kind) {
    case "invalid-link":
      return [viewer.invalidLink.text];
    case "sending":
      return [viewer.sending.text];
    case "loading":
      return waitingSummary(state.notice, locale);
    case "start-failed":
      return startFailureSummary(state.failure, locale);
    case "job":
      return [
        ...jobSummary(state.job, locale),
        ...noticeSummary(state.notice, locale),
      ];
  }
}

/**
 * Shows the existing-article notice, or hides it. Its nodes are replaced only
 * when its text changes, so a redraw keeps the reader's selection.
 */
function drawExisting(
  region: HTMLElement,
  existing: boolean,
  locale: Locale,
): void {
  region.hidden = !existing;
  if (!existing) {
    region.replaceChildren();
    return;
  }
  const nodes = messageNodes(
    region.ownerDocument,
    catalogs[locale].viewer.existing,
  );
  const text = nodes.map((node) => node.textContent).join("");
  if (region.textContent !== text) {
    region.replaceChildren(...nodes);
  }
}

/**
 * Draws the existing-article notice, the state, the operations and the
 * article, in that order. The state summary's region is written only when
 * its text changes, so a poll that repeats the same result, or the elapsed
 * time's redraw, is not read out again. The operations are shown with an
 * article: save and copy can always be pressed. They are set before the article is
 * drawn, so saving and copying work even if drawing the article fails. The
 * article is drawn from its Markdown again only when the Markdown changed or
 * the article was hidden in between, so a redraw keeps the reader's focus
 * and selection inside it. Focus is never moved.
 */
export function renderViewer(
  elements: ViewerElements,
  view: ViewerView,
  locale: Locale,
): void {
  const catalog = catalogs[locale];
  drawExisting(elements.existing, view.existing, locale);
  drawStateOnChange(
    elements.state,
    view.state,
    viewElapsed(view),
    locale,
    () => {
      drawState(elements.state, view, locale);
    },
  );
  writeStatusSummary(elements.stateStatus, stateSummary(view, locale));

  // The article is a done job's, or the one a lost job kept.
  const article =
    view.state.kind === "job" ? jobArticle(view.state.job) : undefined;
  if (article === undefined) {
    elements.article.hidden = true;
    elements.article.replaceChildren();
    elements.actions.hidden = true;
    elements.save.disabled = true;
    elements.copy.disabled = true;
    return;
  }
  elements.actions.hidden = false;
  elements.save.textContent = catalog.viewer.save;
  elements.save.disabled = false;
  elements.copy.textContent = catalog.viewer.copy;
  elements.copy.disabled = false;

  // After the operations, so a fault in drawing the article still leaves
  // saving and copying it.
  elements.article.hidden = false;
  drawArticleBody(elements.article, article.markdown);
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
    case "saved":
      return { text: viewer.saved, steps: [] };
    case "copied":
      return {
        text: notice.mentionsNeutralized
          ? viewer.copiedNeutralized
          : viewer.copied,
        steps: [],
      };
    case "copy-failed":
      return { text: viewer.copyFailed, steps: [] };
  }
}

/**
 * Writes the last operation's result into its status region beside the
 * operations; focus stays where it is.
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
  elements.actionStatus.replaceChildren(...messageNodes(doc, message));
}
