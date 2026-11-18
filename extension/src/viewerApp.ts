// The viewer page's path in the extension-only path. The page URL names what
// the viewer shows and nothing else: a start not reserved yet (r) or a job
// (k), the collected video URL and the page title. It is checked once at
// load. A viewer of a start runs the job itself (viewerRunner.ts) and, once
// the run is reserved, writes the job ID into its URL; a viewer of a job only
// shows the job's record from chrome.storage.session and follows its change
// events. No URL value is ever a generation input, and no load of the page
// starts a run: only a start the service worker hands over does.
// A URL marked e=1 shows an existing article of the same video: the mark is
// added on a same-video reservation, and once added it is never removed.
// The viewer answers the other pages' queries for its job, and a viewer that
// is handed a job another viewer already shows closes its own tab. Every
// result is a value drawn into viewer.html; the log gets only fixed labels and
// kinds.

import type {
  BrowserCall,
  OpenViewerChannel,
  SaveFile,
  TabPlace,
  ViewerTabs,
  WriteClipboard,
  onSessionChanged,
  readSessionItems,
} from "./browser/chromeDeps.ts";
import type { BuildKind } from "./buildKind.d.ts";
import { watchVideoId } from "./core/acceptedUrl.ts";
import { articleFileName } from "./core/fileName.ts";
import { resolveLocale } from "./core/i18n/locale.ts";
import type { Locale } from "./core/i18n/locale.ts";
import { parseJobChannelMessage } from "./core/jobMessages.ts";
import type { RunResult, SwRequest } from "./core/jobMessages.ts";
import {
  jobItemName,
  readJobRecord,
  readStartContext,
  startItemName,
} from "./core/jobRecord.ts";
import type { JobRecord, StartContext } from "./core/jobRecord.ts";
import { jobViewerQuery, parseJobViewerUrl } from "./core/jobViewerUrl.ts";
import type { JobViewerParams } from "./core/jobViewerUrl.ts";
import {
  elapsedRedrawMs,
  failedDisplay,
  jobArticle,
  recordDisplay,
} from "./core/jobView.ts";
import type { JobDisplay, ViewerState } from "./core/jobView.ts";
import type { Logger } from "./core/types.ts";
import { viewerTitle } from "./core/viewerTitle.ts";
import type { TitleSource } from "./core/viewerTitle.ts";
import { findJobViewers } from "./jobViewerQuery.ts";
import { applyPageLocale } from "./ui/pageLocale.ts";
import {
  renderActionNotice,
  renderViewer,
  renderViewerHeader,
} from "./ui/renderViewer.ts";
import type {
  ActionNotice,
  ViewerElements,
  ViewerHeader,
} from "./ui/renderViewer.ts";
import { runViewer } from "./viewerRunner.ts";
import type { RunOutcome, RunnerDeps } from "./viewerRunner.ts";

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
  /** The build's kind, from the module the build writes. */
  readonly buildKind: BuildKind;
  readonly page: ViewerPage;
  readonly tabs: Pick<ViewerTabs, "getCurrent" | "remove">;
  readonly sendToSw: (request: SwRequest) => Promise<BrowserCall<unknown>>;
  readonly readSession: typeof readSessionItems;
  readonly onSessionChanged: typeof onSessionChanged;
  readonly openChannel: OpenViewerChannel;
  /** A new ID: 128 random bits as 32 lowercase hexadecimal characters. */
  readonly newId: () => string;
  /** What the running viewer needs besides the messaging, the storage events and the timer. */
  readonly runner: Pick<
    RunnerDeps,
    "startWorker" | "connectPort" | "readSettings" | "confirmUnload"
  >;
  /** Runs callback once after ms milliseconds; returns a function that cancels it. */
  readonly schedule: (ms: number, callback: () => void) => () => void;
  readonly now: () => number;
  /** Opens the options page. */
  readonly openOptions: () => Promise<BrowserCall<undefined>>;
  readonly writeClipboard: WriteClipboard;
  readonly saveFile: SaveFile;
  readonly log: Logger;
}

const viewerUrlLabel = "yt2column viewer url";
const runLabel = "yt2column viewer run";
const recordLabel = "yt2column job record";

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

/** The display of a run's own result that the record does not hold. */
function heldDisplay(result: RunResult): JobDisplay {
  return result.state === "done"
    ? { kind: "unstored", steps: result.steps, article: result.article }
    : failedDisplay(result.steps, result.failure);
}

/** The state a run's outcome shows when no record decides it; undefined when the record does. */
function outcomeState(outcome: RunOutcome): ViewerState | undefined {
  switch (outcome.kind) {
    case "input_unavailable":
      return {
        kind: "start-failed",
        failure: { kind: "input-unavailable" },
      };
    case "prepare_rejected":
      return {
        kind: "start-failed",
        failure: { kind: "rejected", reason: outcome.reason },
      };
    case "reserve_unavailable":
      return { kind: "start-failed", failure: { kind: "unknown" } };
    case "not_run":
      return outcome.reply.status === "rejected"
        ? {
            kind: "start-failed",
            failure: { kind: "rejected", reason: outcome.reply.reason },
          }
        : undefined;
    case "finished":
      return outcome.stored === "not_stored"
        ? { kind: "job", job: heldDisplay(outcome.result) }
        : undefined;
    case "stopped":
      return undefined;
  }
}

/**
 * Viewer entry. Decides the locale and sets the page language, finds the
 * skeleton (with any element missing it does nothing), learns its own tab,
 * and checks the URL: a link that fails the check shows that it is invalid,
 * with a fixed page title, and sends nothing. A start URL runs the job; a
 * job URL shows its record, and, when the record names this tab as its
 * running page, tells the service worker that the page was reloaded. The
 * page title follows the values the header shows. Never throws.
 */
export async function startViewer(
  doc: Document,
  deps: ViewerStartDeps,
): Promise<void> {
  const locale: Locale = resolveLocale(deps.uiLanguage());
  applyPageLocale(doc, locale);
  const found = findViewerElements(doc);
  if (found === undefined) {
    return;
  }
  const elements: ViewerElements = found;
  const controls = {
    openOptions: (): void => {
      void deps.openOptions();
    },
  };
  const current = await deps.tabs.getCurrent();
  const tab: TabPlace | undefined = current.ok ? current.value : undefined;

  const parsed = parseJobViewerUrl(deps.page.search, deps.page.hash);
  if (!parsed.ok) {
    deps.log.error(viewerUrlLabel, { error: parsed.error });
    doc.title = viewerTitle({ kind: "invalid-link" }, deps.buildKind, locale);
    renderViewerHeader(elements, undefined, locale, undefined);
    renderViewer(
      elements,
      {
        state: { kind: "invalid-link" },
        elapsedMs: 0,
        existing: false,
        buildKind: deps.buildKind,
      },
      locale,
      controls,
    );
    return;
  }
  let params: JobViewerParams = parsed.params;
  /** The page shows an existing article of the same video: its URL holds e=1. */
  const existing = (): boolean =>
    params.target.kind === "job" && params.target.existing === true;

  // What the header and the page title show: the URL's title until a start
  // context or a record says more.
  let header: ViewerHeader = {
    heading: params.title,
    videoUrl: params.videoUrl,
  };
  let titleSource: TitleSource = { kind: "url", urlTitle: params.title };
  /** The video title the record collected; the saved file's name. */
  let fileTitle: string | undefined;
  /** Where the elapsed time counts from: the start context's or the record's time. */
  let since: number | undefined;
  let state: ViewerState =
    params.target.kind === "start"
      ? { kind: "preparing" }
      : { kind: "loading" };
  /** Set once the viewer's own run ended with a result its record does not hold. */
  let held: Extract<ViewerState, { kind: "job" }> | undefined;
  let actionNotice: ActionNotice = { kind: "none" };
  let cancelRedraw = (): void => {};
  /** The job this viewer shows, once it knows one. */
  let jobId: string | undefined =
    params.target.kind === "job" ? params.target.jobId : undefined;

  let headerArticle: ReturnType<typeof jobArticle>;
  let headerDrawn = false;
  const shownArticle = (): ReturnType<typeof jobArticle> =>
    state.kind === "job" ? jobArticle(state.job) : undefined;

  const draw = (): void => {
    // The heading shows the module's neutralized values once the article is
    // done. It is drawn again only when the header or the shown article
    // changes, so the redraw each second keeps a selection in the heading.
    const article = shownArticle();
    if (!headerDrawn || article !== headerArticle) {
      headerDrawn = true;
      headerArticle = article;
      renderViewerHeader(elements, header, locale, article);
    }
    const titled: TitleSource =
      article !== undefined && titleSource.kind === "record"
        ? { ...titleSource, videoTitle: article.videoTitle }
        : titleSource;
    doc.title = viewerTitle(titled, deps.buildKind, locale);
    renderViewer(
      elements,
      {
        state,
        elapsedMs: since === undefined ? 0 : deps.now() - since,
        existing: existing(),
        buildKind: deps.buildKind,
      },
      locale,
      controls,
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
      cancelRedraw = deps.schedule(elapsedRedrawMs, () => {
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

  const setHeader = (next: ViewerHeader): void => {
    header = next;
    headerDrawn = false;
  };

  /** Shows the record of the job, or that it is lost; a held result stays until the record holds one. */
  const onRecord = (record: JobRecord | undefined): void => {
    if (record === undefined) {
      setHeader({ heading: params.title, videoUrl: params.videoUrl });
      titleSource = { kind: "url", urlTitle: params.title };
    } else {
      setHeader({
        heading:
          record.heading.videoTitle.trim() === ""
            ? params.title
            : record.heading.videoTitle,
        channelName: record.heading.channelName,
        videoUrl: record.videoUrl,
      });
      titleSource = {
        kind: "record",
        videoTitle: record.heading.videoTitle,
        heading: params.title,
      };
      fileTitle = record.heading.videoTitle;
      since = record.acceptedAt;
    }
    // The page's own result that its record could not hold stays shown,
    // whatever the record says afterwards.
    show(held ?? { kind: "job", job: recordDisplay(record) });
  };

  let stopWatching = (): void => {};
  /**
   * Follows the record of watched: its change events first, then one read,
   * so no change can come between them unseen. A read that a change event
   * overtook is dropped; first receives the first record seen.
   */
  const watch = (
    watched: string,
    first: (record: JobRecord | undefined) => void = () => {},
  ): void => {
    stopWatching();
    const name = jobItemName(watched);
    let changed = false;
    stopWatching = deps.onSessionChanged((changes) => {
      const change = changes.get(name);
      if (change === undefined) {
        return;
      }
      changed = true;
      onRecord(
        change.removed ? undefined : readJobRecord(watched, change.newValue),
      );
    });
    void deps.readSession([name]).then((read) => {
      if (changed) {
        return;
      }
      if (!read.ok) {
        deps.log.error(recordLabel, { read: false });
        return;
      }
      const record = readJobRecord(watched, read.value[name]);
      onRecord(record);
      first(record);
    });
  };

  /** Writes params into the page URL, replacing its history entry. */
  const writeUrl = (): void => {
    deps.page.replaceQuery(jobViewerQuery(params));
  };

  const showActionNotice = (notice: ActionNotice): void => {
    actionNotice = notice;
    renderActionNotice(elements, actionNotice, locale);
  };

  const saveArticle = (): void => {
    const article = shownArticle();
    const videoId = watchVideoId(params.videoUrl);
    if (article === undefined || videoId === undefined) {
      return;
    }
    const saved = deps.saveFile(
      articleFileName(fileTitle, videoId),
      article.file,
    );
    showActionNotice(saved.ok ? { kind: "saved" } : { kind: "none" });
  };

  const copyArticle = async (): Promise<void> => {
    const article = shownArticle();
    if (article === undefined) {
      return;
    }
    const copied = await deps.writeClipboard(article.markdown);
    showActionNotice(
      copied.ok
        ? { kind: "copied", mentionsNeutralized: article.mentionsNeutralized }
        : { kind: "copy-failed" },
    );
  };

  elements.save.addEventListener("click", saveArticle);
  elements.copy.addEventListener("click", () => void copyArticle());
  deps.page.onVisible(() => {
    draw();
    armRedraw();
  });

  // The viewer answers a query for the job it shows, from the moment it knows one.
  const opened = deps.openChannel();
  if (opened.ok && tab !== undefined) {
    const channel = opened.value;
    const place = tab;
    channel.listen((data) => {
      const message = parseJobChannelMessage(data);
      if (
        message?.type === "find" &&
        jobId !== undefined &&
        message.jobId === jobId
      ) {
        channel.post({
          type: "present",
          queryId: message.queryId,
          tabId: place.tabId,
          windowId: place.windowId,
        });
      }
    });
  }

  draw();

  if (params.target.kind === "job") {
    const shownJob = params.target.jobId;
    watch(shownJob, (record) => {
      // A load of the running page's tab with a job URL is that page
      // reloaded: the run it held has stopped.
      if (
        record?.state === "running" &&
        tab !== undefined &&
        record.runner.tabId === tab.tabId
      ) {
        void deps.sendToSw({
          type: "runnerReloaded",
          jobId: shownJob,
          instanceId: deps.newId(),
        });
      }
    });
    return;
  }

  const { startId } = params.target;
  const contextName = startItemName(startId);
  const context = await deps
    .readSession([contextName])
    .then((read): StartContext | undefined =>
      read.ok ? readStartContext(startId, read.value[contextName]) : undefined,
    );
  if (context !== undefined) {
    setHeader({
      heading: context.heading,
      channelName: context.channelName,
      videoUrl: context.videoUrl,
    });
    titleSource = {
      kind: "start",
      videoTitle: context.videoTitle,
      heading: context.heading,
    };
    fileTitle = context.videoTitle;
    since = context.createdAt;
    draw();
  }

  const outcome = await runViewer(
    { startId, instanceId: deps.newId() },
    {
      ...deps.runner,
      sendToSw: deps.sendToSw,
      onSessionChanged: deps.onSessionChanged,
      schedule: deps.schedule,
    },
    {
      onReserved: (reply) => {
        switch (reply.status) {
          case "rejected":
            return;
          case "new_run":
            jobId = reply.jobId;
            params = {
              ...params,
              target: { kind: "job", jobId: reply.jobId },
            };
            writeUrl();
            watch(reply.jobId);
            return;
          case "existing":
          case "same_video": {
            const handed = reply.jobId;
            params = {
              ...params,
              target:
                reply.status === "same_video"
                  ? { kind: "job", jobId: handed, existing: true }
                  : { kind: "job", jobId: handed },
            };
            writeUrl();
            void handOver(handed);
            return;
          }
        }
      },
      onProgress: () => {},
    },
  );
  deps.log.info(runLabel, { outcome: outcome.kind });
  const ended = outcomeState(outcome);
  if (ended === undefined) {
    return;
  }
  if (ended.kind === "job") {
    held = ended;
  }
  show(ended);

  /**
   * A job another viewer may already show: when one answers, this tab
   * closes, leaving that viewer; otherwise this viewer shows the job.
   */
  async function handOver(handed: string): Promise<void> {
    const answers = await findJobViewers(handed, {
      openChannel: deps.openChannel,
      queryId: deps.newId,
      schedule: deps.schedule,
    });
    const other = answers.find((answer) => answer.tabId !== tab?.tabId);
    if (other !== undefined && tab !== undefined) {
      const closed = await deps.tabs.remove(tab.tabId);
      if (closed.ok) {
        return;
      }
    }
    jobId = handed;
    watch(handed);
  }
}
