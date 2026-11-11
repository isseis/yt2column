// The viewer page's path. The page URL is the viewer's only memory: it is
// checked once at load, and from it the viewer either waits for its start
// result from the service worker or, holding a key, polls the job's status.
// A URL marked e=1 shows an existing article of the same video: the mark is
// added on a same-video answer, and once added it is never removed.
// The viewer answers the other pages' queries for its key and takes a send
// over only from a query it answered. Every result is a value drawn into
// viewer.html; the log gets only what the log views allow.

import type {
  OpenViewerChannel,
  SaveFile,
  SendToWorker,
  TabPlace,
  ViewerChannel,
  ViewerTabs,
  WriteClipboard,
} from "./browser/chromeDeps.ts";
import { watchVideoId } from "./core/acceptedUrl.ts";
import type { ArticleKey } from "./core/articleKey.ts";
import {
  parseAwaitStartReply,
  parseViewerChannelMessage,
} from "./core/extensionMessages.ts";
import type { ViewerChannelMessage } from "./core/extensionMessages.ts";
import { articleFileName } from "./core/fileName.ts";
import { resolveLocale } from "./core/i18n/locale.ts";
import { jobArticle } from "./core/jobView.ts";
import type { ViewerState } from "./core/jobView.ts";
import type { PollStep } from "./core/poll.ts";
import { elapsedRedrawMs, startResultDeadlineMs } from "./core/poll.ts";
import type { Article, StartOutcome } from "./core/response.ts";
import {
  parseViewerUrl,
  showsExisting,
  viewerQuery,
} from "./core/viewerUrl.ts";
import type { ViewerParams } from "./core/viewerUrl.ts";
import type { Logger } from "./core/types.ts";
import { createStatusPoller } from "./poller.ts";
import type { Poller } from "./poller.ts";
import type { SendDeps } from "./send.ts";
import { applyPageLocale } from "./ui/pageLocale.ts";
import {
  renderActionNotice,
  renderViewer,
  renderViewerHeader,
} from "./ui/renderViewer.ts";
import type { ActionNotice, ViewerElements } from "./ui/renderViewer.ts";

/** The page's own surface: its URL, its history entry and its visibility. */
export interface ViewerPage {
  /** location.search and location.hash as the page was loaded. */
  readonly search: string;
  readonly hash: string;
  /** Replaces the page URL's query without adding a history entry: history.replaceState. */
  replaceQuery(query: string): void;
  /** Whether the page is visible now: document.visibilityState. */
  isVisible(): boolean;
  /** Calls listener each time the page becomes visible: visibilitychange. */
  onVisible(listener: () => void): void;
}

/** What the viewer entry needs; viewer.ts builds it from the browser APIs. */
export interface ViewerStartDeps {
  /** The browser's UI language, undefined when it cannot be read. */
  uiLanguage(): string | undefined;
  readonly page: ViewerPage;
  readonly tabs: Pick<ViewerTabs, "getCurrent">;
  readonly sendToWorker: SendToWorker;
  readonly openChannel: OpenViewerChannel;
  /** The host exchange and the timer of every wait. */
  readonly send: SendDeps;
  readonly now: () => number;
  readonly writeClipboard: WriteClipboard;
  readonly saveFile: SaveFile;
  readonly log: Logger;
}

const awaitStartLabel = "yt2column await start";
const viewerUrlLabel = "yt2column viewer url";
const takeOverLabel = "yt2column take over";

const pageTitleSuffix = " - yt2column";

/**
 * The skeleton's elements, found in doc by id. Undefined unless every element
 * is present and of its kind, so a partial skeleton is never drawn into.
 */
export function findViewerElements(doc: Document): ViewerElements | undefined {
  const win = doc.defaultView;
  if (win === null) {
    return undefined;
  }
  const element = (id: string): HTMLElement | undefined =>
    doc.getElementById(id) ?? undefined;
  const button = (id: string): HTMLButtonElement | undefined => {
    const found = doc.getElementById(id);
    return found instanceof win.HTMLButtonElement ? found : undefined;
  };
  const videoLink = doc.getElementById("video-link");
  const elements = {
    heading: element("heading"),
    channel: element("channel"),
    videoLink:
      videoLink instanceof win.HTMLAnchorElement ? videoLink : undefined,
    existing: element("existing"),
    state: element("state"),
    stateStatus: element("state-status"),
    article: element("article"),
    actions: element("actions"),
    save: button("save"),
    copy: button("copy"),
    actionStatus: element("action-status"),
  };
  for (const value of Object.values(elements)) {
    if (value === undefined) {
      return undefined;
    }
  }
  return elements as ViewerElements;
}

/** The article a state shows: its job's, when it shows a job. */
function shownArticle(state: ViewerState): Article | undefined {
  return state.kind === "job" ? jobArticle(state.job) : undefined;
}

/**
 * Whether a take-over restarts the send: a failed or lost job, or a stopped
 * poll, shows nothing that is still going, so the viewer shows the new send
 * from its own send time. A running or done job, or a first status still
 * awaited, goes on as it is.
 */
function takeOverRestarts(state: ViewerState): boolean {
  switch (state.kind) {
    case "loading":
      return state.notice.kind === "stopped";
    case "job":
      return (
        state.job.kind === "failed" ||
        state.job.kind === "lost" ||
        state.notice.kind === "stopped"
      );
    // A viewer without a key never answers present, so it is never asked.
    case "invalid-link":
    case "sending":
    case "start-failed":
      return false;
  }
}

/**
 * Viewer entry. Decides the locale and sets the page language, finds the
 * skeleton (with any element missing it does nothing), learns its own tab,
 * and checks the URL: a link that fails the check shows that it is invalid
 * and sends nothing. A URL with a key polls at once; one without waits for
 * the start result until startResultDeadlineMs after its send time, ignores
 * a result that comes later, and on acceptance adds the key to the URL.
 * Never throws.
 */
export async function startViewer(
  doc: Document,
  deps: ViewerStartDeps,
): Promise<void> {
  const locale = resolveLocale(deps.uiLanguage());
  applyPageLocale(doc, locale);
  const found = findViewerElements(doc);
  if (found === undefined) {
    return;
  }
  const elements: ViewerElements = found;
  const loadedAt = deps.now();
  const current = await deps.tabs.getCurrent();
  const tab: TabPlace | undefined = current.ok ? current.value : undefined;

  const parsed = parseViewerUrl(deps.page.search, deps.page.hash, loadedAt);
  if (!parsed.ok) {
    deps.log.error(viewerUrlLabel, { error: parsed.error });
    renderViewerHeader(elements, undefined, locale, undefined);
    renderViewer(
      elements,
      {
        state: { kind: "invalid-link" },
        elapsedMs: 0,
        existing: false,
      },
      locale,
    );
    return;
  }
  let params: ViewerParams = parsed.params;
  /** The page shows an existing article of the same video: its URL holds e=1. */
  const existing = (): boolean => showsExisting(params);
  renderViewerHeader(elements, params, locale, undefined);
  doc.title = `${params.heading}${pageTitleSuffix}`;

  let state: ViewerState = { kind: "sending" };
  let actionNotice: ActionNotice = { kind: "none" };
  let poller: Poller | undefined;
  let key: ArticleKey | undefined;
  let cancelRedraw = (): void => {};
  // Counts the starts of polling: a take-over that restarts the send makes a
  // copy result that was in flight belong to the send it replaced.
  let restarts = 0;

  /** The article the heading was last drawn with. */
  let headerArticle: Article | undefined;
  const draw = (): void => {
    // The heading shows the server's neutralized values once the article
    // is done. It is drawn again only when the shown article changes, so
    // the redraw each second keeps a selection in the heading.
    const article = shownArticle(state);
    if (article !== headerArticle) {
      headerArticle = article;
      renderViewerHeader(elements, params, locale, article);
    }
    renderViewer(
      elements,
      {
        state,
        elapsedMs: deps.now() - params.sentAt,
        existing: existing(),
      },
      locale,
    );
  };

  /**
   * Draws the elapsed time again each second while a running job is shown
   * with its elapsed time and the page is visible.
   */
  const armRedraw = (): void => {
    cancelRedraw();
    cancelRedraw = () => {};
    if (
      !existing() &&
      state.kind === "job" &&
      state.job.kind === "running" &&
      deps.page.isVisible()
    ) {
      cancelRedraw = deps.send.schedule(elapsedRedrawMs, () => {
        draw();
        armRedraw();
      });
    }
  };

  const show = (next: ViewerState): void => {
    state = next;
    draw();
    armRedraw();
  };

  const showActionNotice = (notice: ActionNotice): void => {
    actionNotice = notice;
    renderActionNotice(elements, actionNotice, locale);
  };

  /** Writes params into the page URL, replacing its history entry. */
  const writeUrl = (): void => {
    deps.page.replaceQuery(viewerQuery(params));
  };

  const onStep = (step: PollStep): void => {
    if (step.job !== undefined) {
      show({ kind: "job", job: step.job, notice: step.notice });
    } else if (state.kind === "job") {
      show({ ...state, notice: step.notice });
    } else {
      show({ kind: "loading", notice: step.notice });
    }
  };

  /** Shows loading and polls for key from a new epoch. */
  const startPolling = (pollKey: ArticleKey): void => {
    key = pollKey;
    restarts += 1;
    poller ??= createStatusPoller(pollKey, deps, onStep);
    show({ kind: "loading", notice: { kind: "none" } });
    poller.start();
  };

  /** A wait that ended without a start result: the result is unknown, and nothing is polled. */
  const noResult = (received: "none" | "deadline" | "failed"): void => {
    deps.log.error(awaitStartLabel, { received });
    show({ kind: "start-failed", failure: { kind: "no-result" } });
  };

  const onStartResult = (outcome: StartOutcome): void => {
    deps.log.info(awaitStartLabel, {
      received: "result",
      outcome: outcome.kind,
    });
    switch (outcome.kind) {
      case "accepted":
        params = { ...params, key: outcome.key };
        break;
      // The key of an existing article of the same video: polled as an
      // accepted key, and the URL marks the page as showing it.
      case "same-video":
        params = { ...params, key: outcome.key, existing: true };
        break;
      default:
        show({ kind: "start-failed", failure: outcome });
        return;
    }
    writeUrl();
    startPolling(outcome.key);
  };

  /**
   * Asks the service worker for this tab's start result once, until the
   * deadline: a reply after it, or after the page gave up, is ignored.
   */
  const awaitStart = async (): Promise<void> => {
    const remaining = params.sentAt + startResultDeadlineMs - deps.now();
    if (remaining <= 0) {
      noResult("deadline");
      return;
    }
    show({ kind: "sending" });
    let settled = false;
    const cancel = deps.send.schedule(remaining, () => {
      if (!settled) {
        settled = true;
        noResult("deadline");
      }
    });
    const reply = await deps.sendToWorker({
      type: "await-start",
      sentAt: params.sentAt,
    });
    if (settled) {
      return;
    }
    settled = true;
    cancel();
    // A timer that fired late can let a reply in after the deadline.
    if (deps.now() >= params.sentAt + startResultDeadlineMs) {
      noResult("deadline");
      return;
    }
    if (!reply.ok) {
      noResult("failed");
      return;
    }
    const answer = parseAwaitStartReply(reply.value);
    if (answer === undefined) {
      noResult("failed");
    } else if (answer.kind === "none") {
      noResult("none");
    } else {
      onStartResult(answer.outcome);
    }
  };

  // The queries this viewer answered present to; only these may take it over.
  const presentQueries = new Set<string>();

  const onTakeOver = (
    channel: ViewerChannel,
    message: Extract<ViewerChannelMessage, { type: "take-over" }>,
  ): void => {
    if (tab === undefined || message.tabId !== tab.tabId) {
      return;
    }
    if (!presentQueries.delete(message.queryId) || poller === undefined) {
      deps.log.info(takeOverLabel, { accepted: false });
      return;
    }
    channel.post({
      type: "took-over",
      queryId: message.queryId,
      tabId: tab.tabId,
    });
    const restarted = takeOverRestarts(state);
    deps.log.info(takeOverLabel, { accepted: true, restarted });
    // A send answered with a same-video article marks the page, whether the
    // send restarts or goes on; an accepted send leaves the mark as it is.
    switch (message.answer) {
      case "accepted":
        break;
      case "same-video":
        // A viewer answers present only while it holds a key.
        if ("key" in params) {
          params = { ...params, existing: true };
        }
        break;
    }
    if (!restarted) {
      if (message.answer === "same-video") {
        writeUrl();
        draw();
        armRedraw();
      }
      poller?.bumpEpoch();
      return;
    }
    params = { ...params, sentAt: message.sentAt };
    writeUrl();
    showActionNotice({ kind: "none" });
    if (key !== undefined) {
      startPolling(key);
    }
  };

  const onChannelMessage = (channel: ViewerChannel, data: unknown): void => {
    const message = parseViewerChannelMessage(data);
    if (message === undefined || tab === undefined) {
      return;
    }
    switch (message.type) {
      case "find":
        if (key !== undefined && message.key === key) {
          presentQueries.add(message.queryId);
          channel.post({
            type: "present",
            queryId: message.queryId,
            tabId: tab.tabId,
            windowId: tab.windowId,
            sentAt: params.sentAt,
          });
        } else if (state.kind === "sending") {
          channel.post({
            type: "waiting",
            queryId: message.queryId,
            tabId: tab.tabId,
            sentAt: params.sentAt,
          });
        }
        return;
      case "take-over":
        onTakeOver(channel, message);
        return;
      case "present":
      case "waiting":
      case "took-over":
        return;
    }
  };

  const saveArticle = (): void => {
    const article = shownArticle(state);
    const videoId = watchVideoId(params.url);
    if (article === undefined || videoId === undefined) {
      return;
    }
    const saved = deps.saveFile(
      articleFileName(params.title, videoId),
      article.file,
    );
    showActionNotice(saved.ok ? { kind: "saved" } : { kind: "none" });
  };

  const copyArticle = async (): Promise<void> => {
    const article = shownArticle(state);
    if (article === undefined) {
      return;
    }
    const startedIn = restarts;
    const copied = await deps.writeClipboard(article.markdown);
    // The send was restarted while copying: the result is about the article
    // the new send replaces, so it changes nothing shown.
    if (restarts !== startedIn) {
      return;
    }
    showActionNotice(
      copied.ok
        ? { kind: "copied", mentionsNeutralized: article.mentionsNeutralized }
        : { kind: "copy-failed" },
    );
  };

  elements.save.addEventListener("click", saveArticle);
  elements.copy.addEventListener("click", () => void copyArticle());

  deps.page.onVisible(() => {
    poller?.fetchNow();
    draw();
    armRedraw();
  });

  const opened = deps.openChannel();
  if (opened.ok) {
    const channel = opened.value;
    channel.listen((data) => onChannelMessage(channel, data));
  }

  if ("key" in params) {
    startPolling(params.key);
  } else {
    await awaitStart();
  }
}
