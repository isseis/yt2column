// The viewer page's path. The page URL is the viewer's only memory: it is
// checked once at load, and from it the viewer either waits for its start
// result from the service worker or, holding a key, polls the job's status.
// The viewer answers the other pages' queries for its key, takes a send over
// only from a query it answered, and sends a publish only when the button is
// pressed on a row that allows it, never again by itself. Every result is a
// value drawn into viewer.html; the log gets only what the log views allow.

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
import { publishMessage, statusMessage } from "./core/hostMessage.ts";
import { resolveLocale } from "./core/i18n/locale.ts";
import { jobArticle, publishControl } from "./core/jobView.ts";
import type { ViewerState } from "./core/jobView.ts";
import type { PollStep } from "./core/poll.ts";
import { startResultDeadlineMs } from "./core/poll.ts";
import {
  classifyPublish,
  classifyStatus,
  publishLogView,
  statusLogView,
  toPublishOutcome,
  toStatusOutcome,
} from "./core/response.ts";
import type {
  Article,
  PublishLogView,
  StartOutcome,
  StatusLogView,
  StatusOutcome,
} from "./core/response.ts";
import { parseViewerUrl, viewerQuery } from "./core/viewerUrl.ts";
import type { ViewerParams } from "./core/viewerUrl.ts";
import type { Logger } from "./launch.ts";
import { createPoller } from "./poller.ts";
import type { Poller } from "./poller.ts";
import { exchange } from "./send.ts";
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
const statusResultLabel = "yt2column status result";
const publishResultLabel = "yt2column publish result";
const takeOverLabel = "yt2column take over";

/** How often the elapsed time of a running job is drawn again while the page is visible. */
export const elapsedRedrawMs = 1_000;

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
    state: element("state"),
    stateStatus: element("state-status"),
    article: element("article"),
    actions: element("actions"),
    publish: button("publish"),
    publishRow: element("publish-row"),
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

/** The status result as logged, waiting for the poller's decision. */
interface StatusLog {
  readonly view: StatusLogView;
  /** The viewer's epoch when the fetch was sent. */
  readonly epoch: number;
  /** Whether the page becoming visible sent the fetch. */
  readonly visible: boolean;
}

/** A failed exchange goes to the error log; a server's answer to info. */
function isExchangeFailure(view: StatusLogView | PublishLogView): boolean {
  switch (view.kind) {
    case "ok":
    case "accepted":
    case "rejected":
      return false;
    case "not-sent":
    case "no-response":
    case "host-failed":
    case "unexpected":
      return true;
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
    renderViewerHeader(elements, undefined, locale);
    renderViewer(
      elements,
      { state: { kind: "invalid-link" }, elapsedMs: 0, publishing: false },
      locale,
    );
    return;
  }
  let params: ViewerParams = parsed.params;
  renderViewerHeader(elements, params, locale);
  doc.title = `${params.heading}${pageTitleSuffix}`;

  let state: ViewerState = { kind: "sending" };
  let publishing = false;
  /** An accepted publish waits for the first status of a later epoch to end publishing. */
  let awaitingPublishStatus = false;
  let actionNotice: ActionNotice = { kind: "none" };
  // Counts the poller's epochs: it moves exactly when the viewer calls the
  // poller's start or bumpEpoch, so a status result sent in an earlier epoch
  // is told apart here as it is in the poller.
  let epoch = 0;
  let statusLog: StatusLog | undefined;
  let visibleFetch = false;
  let poller: Poller | undefined;
  let key: ArticleKey | undefined;
  let cancelRedraw = (): void => {};
  // Counts the starts of polling: a take-over that restarts the send makes a
  // publish answer that was in flight belong to the send it replaced.
  let restarts = 0;

  const draw = (): void => {
    renderViewer(
      elements,
      { state, elapsedMs: deps.now() - params.sentAt, publishing },
      locale,
    );
  };

  /** Draws the elapsed time again each second while a running job is shown and the page is visible. */
  const armRedraw = (): void => {
    cancelRedraw();
    cancelRedraw = () => {};
    if (
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

  const fetchStatus = async (statusKey: ArticleKey): Promise<StatusOutcome> => {
    const sentEpoch = epoch;
    const visible = visibleFetch;
    const log = classifyStatus(
      await exchange(statusMessage(statusKey), deps.send),
    );
    statusLog = { view: statusLogView(log), epoch: sentEpoch, visible };
    return toStatusOutcome(log);
  };

  const onStep = (step: PollStep): void => {
    const logged = statusLog;
    statusLog = undefined;
    const currentEpoch = logged?.epoch === epoch;
    if (logged !== undefined) {
      const entry = {
        result: logged.view,
        decision: step.decision,
        earlierEpoch: !currentEpoch,
        visible: logged.visible,
      };
      if (isExchangeFailure(logged.view)) {
        deps.log.error(statusResultLabel, entry);
      } else {
        deps.log.info(statusResultLabel, entry);
      }
    }
    // The first current reply after an accepted publish ends publishing. The
    // epoch only grows, and a take-over that goes on may move it past the
    // publish's own epoch, so the reply is not required to be of that epoch.
    if (currentEpoch && awaitingPublishStatus) {
      publishing = false;
      awaitingPublishStatus = false;
    }
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
    poller ??= createPoller(
      {
        fetchStatus: () => fetchStatus(pollKey),
        schedule: deps.send.schedule,
      },
      onStep,
    );
    show({ kind: "loading", notice: { kind: "none" } });
    epoch += 1;
    poller.start();
  };

  /** Moves the poller to a new epoch, starting it when it had stopped. */
  const bumpEpoch = (): void => {
    if (poller === undefined) {
      return;
    }
    epoch += 1;
    poller.bumpEpoch();
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
    if (outcome.kind !== "accepted") {
      show({ kind: "start-failed", failure: outcome });
      return;
    }
    params = { ...params, key: outcome.key };
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
    if (!restarted) {
      bumpEpoch();
      return;
    }
    params = { ...params, sentAt: message.sentAt };
    writeUrl();
    publishing = false;
    awaitingPublishStatus = false;
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

  /**
   * The one place a publish is sent: on a press of the button, only while
   * publishControl's row allows it, exactly once, and never again by itself.
   * An accepted publish keeps the button off until the first status of the
   * new epoch; an unknown result is settled by polling, not by a resend.
   */
  const publishArticle = async (): Promise<void> => {
    if (
      key === undefined ||
      state.kind !== "job" ||
      state.job.kind !== "done" ||
      !publishControl(state.job, publishing).enabled
    ) {
      return;
    }
    publishing = true;
    showActionNotice({ kind: "none" });
    draw();
    const pressedIn = restarts;
    const log = classifyPublish(await exchange(publishMessage(key), deps.send));
    const view = publishLogView(log);
    const replaced = restarts !== pressedIn;
    if (isExchangeFailure(view)) {
      deps.log.error(publishResultLabel, { result: view, replaced });
    } else {
      deps.log.info(publishResultLabel, { result: view, replaced });
    }
    // The send was restarted while the publish was in flight: its answer is
    // about the article the new send replaces, so it changes nothing shown.
    if (replaced) {
      return;
    }
    const outcome = toPublishOutcome(log);
    switch (outcome.kind) {
      case "accepted":
        bumpEpoch();
        awaitingPublishStatus = true;
        break;
      case "rejected": {
        publishing = false;
        showActionNotice(outcome);
        if (outcome.reason === "article_not_found") {
          // The server no longer holds the article: shown as lost, keeping
          // what was shown so it can still be saved and copied.
          poller?.stop();
          const kept = shownArticle(state);
          state = {
            kind: "job",
            job: kept === undefined ? { kind: "lost" } : { kind: "lost", kept },
            notice: { kind: "none" },
          };
        }
        break;
      }
      case "not-sent":
        publishing = false;
        showActionNotice(outcome);
        break;
      case "no-response":
      case "host-failed":
      case "unexpected":
        publishing = false;
        showActionNotice({ kind: "unknown", outcome });
        bumpEpoch();
        break;
    }
    draw();
    armRedraw();
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
    const copied = await deps.writeClipboard(article.markdown);
    showActionNotice(copied.ok ? { kind: "copied" } : { kind: "copy-failed" });
  };

  elements.publish.addEventListener("click", () => void publishArticle());
  elements.save.addEventListener("click", saveArticle);
  elements.copy.addEventListener("click", () => void copyArticle());

  deps.page.onVisible(() => {
    if (poller !== undefined) {
      visibleFetch = true;
      poller.fetchNow();
      visibleFetch = false;
    }
    draw();
    armRedraw();
  });

  const opened = deps.openChannel();
  if (opened.ok) {
    const channel = opened.value;
    channel.listen((data) => onChannelMessage(channel, data));
  }

  if (params.key !== undefined) {
    startPolling(params.key);
  } else {
    await awaitStart();
  }
}
