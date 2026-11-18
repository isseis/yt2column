import type {
  BrowserCall,
  ExtensionIdentity,
  FocusWindow,
  OpenViewerChannel,
  TabPlace,
  ViewerTabs,
  onSessionChanged,
  readSessionItems,
} from "./browser/chromeDeps.ts";
import type { BuildKind } from "./buildKind.d.ts";
import { collect } from "./core/collect.ts";
import type { SelectionReader } from "./core/collect.ts";
import { chooseViewer, parseStartReply } from "./core/jobMessages.ts";
import type {
  SendMode,
  SourceTab,
  StartReply,
  SwRequest,
} from "./core/jobMessages.ts";
import { jobItemName, readJobRecord } from "./core/jobRecord.ts";
import type { JobRecord } from "./core/jobRecord.ts";
import { jobViewerQuery, startUrlTitle } from "./core/jobViewerUrl.ts";
import { elapsedRedrawMs, recordDisplay } from "./core/jobView.ts";
import {
  acceptMetadata,
  parsePageMetadata,
  readFailureResult,
} from "./core/metadata.ts";
import type { MetadataResult } from "./core/metadata.ts";
import { resolveLocale } from "./core/i18n/locale.ts";
import type { Locale } from "./core/i18n/locale.ts";
import {
  buildCollectedView,
  primaryAction,
  secondaryAction,
} from "./core/popupView.ts";
import type {
  ExistingState,
  JobPopupView,
  PopupJobState,
  PopupView,
} from "./core/popupView.ts";
import { prepareSend } from "./core/prepare.ts";
import type { StartInput } from "./core/prepare.ts";
import type { LaunchContext, Logger } from "./core/types.ts";
import { findJobViewers } from "./jobViewerQuery.ts";
import { applyPageLocale } from "./ui/pageLocale.ts";
import { renderPopup, renderPopupFrame } from "./ui/render.ts";
import type { PopupElements } from "./ui/render.ts";

/** The only browser capability the metadata read needs. Faked in unit tests. */
export interface MetadataReader {
  /** Reads the metadata of the tab's top frame as JSON text. Rejects on any failure. */
  read(tabId: number, expectedUrl: string): Promise<string>;
}

export interface PopupLaunchDeps {
  /**
   * Keeps chrome.storage.local from the functions injected into a page
   * (setLocalAccessLevel in production). It runs before anything is
   * injected, and its failure injects nothing.
   */
  readonly protectSettings: () => Promise<BrowserCall<undefined>>;
  readonly reader: SelectionReader;
  readonly metadataReader: MetadataReader;
  activeTab(): Promise<chrome.tabs.Tab | undefined>;
  readonly log: Logger;
}

/** What the launch path found: the view, and the tab a collected selection came from. */
export interface PopupLaunch {
  readonly view: PopupView;
  /** Present exactly when the view is collected: the start opens the viewer next to it. */
  readonly source?: SourceTab;
}

export interface PopupSendDeps {
  /** Sends a request to the service worker; the reply is untrusted. */
  readonly sendToSw: (request: SwRequest) => Promise<BrowserCall<unknown>>;
  readonly readSession: typeof readSessionItems;
  readonly onSessionChanged: typeof onSessionChanged;
  readonly openChannel: OpenViewerChannel;
  /** A new query ID for each query to the viewers. */
  readonly queryId: () => string;
  /** Runs callback once after ms milliseconds; returns a function that cancels it. */
  readonly schedule: (ms: number, callback: () => void) => () => void;
  readonly tabs: Pick<ViewerTabs, "create" | "activate" | "get">;
  readonly focusWindow: FocusWindow;
  /** chrome.runtime in production: the viewer page's URL. */
  readonly identity: Pick<ExtensionIdentity, "getURL">;
  /** Opens the options page. */
  readonly openOptions: () => Promise<BrowserCall<undefined>>;
  readonly now: () => number;
  readonly log: Logger;
  /** The display language; it selects displayed text only, never a log. */
  readonly locale: Locale;
}

/** What the popup entry needs; popup.ts builds it from the chrome APIs. */
export interface PopupStartDeps {
  /**
   * Reads the browser's UI language; readUiLanguage(chrome.i18n) in
   * production. An unreadable language is reported as undefined, never as a
   * throw, so the entry point needs no failure path.
   */
  uiLanguage(): string | undefined;
  /** The build's kind, from the module the build writes. */
  readonly buildKind: BuildKind;
  readonly launch: PopupLaunchDeps;
  readonly send: Omit<PopupSendDeps, "locale">;
}

/**
 * How long the popup waits for the start's result. The service worker
 * answers unknown itself 60 seconds after it took the start; the popup waits
 * a little longer, so that answer arrives first, and shows the same unknown
 * result when even that is lost.
 */
export const startReplyWaitMs = 65_000;

const popupLogLabel = "yt2column popup launch";
const metadataLogLabel = "yt2column video metadata";
const activeTabLabel = "yt2column active tab";
const blockedLabel = "yt2column blocked";
const protectLabel = "yt2column settings protection";
const recordLabel = "yt2column job record";
const uiLanguageLabel = "yt2column ui language";

/** Where the tab is; undefined unless its window and index are non-negative integers. */
function sourceTab(tab: chrome.tabs.Tab, tabId: number): SourceTab | undefined {
  const { windowId, index } = tab;
  if (
    !Number.isSafeInteger(windowId) ||
    windowId < 0 ||
    !Number.isSafeInteger(index) ||
    index < 0
  ) {
    return undefined;
  }
  return { tabId, windowId, index };
}

/** Copies id, url and title verbatim; undefined when there is no tab or no tab id. */
export function launchContextFromTab(
  tab: chrome.tabs.Tab | undefined,
): LaunchContext | undefined {
  if (tab === undefined || tab.id === undefined) {
    return undefined;
  }
  return { tabId: tab.id, url: tab.url, title: tab.title };
}

/**
 * Reads the video metadata of the tab the selection was collected from, and
 * accepts what passes the checks. Called only after a successful collection;
 * a failed read omits every field but never fails the launch. The log holds
 * field names, omission reasons and the failed check, never a read value or
 * the rejection error, which may carry page text. Never throws.
 */
async function readMetadata(
  tabId: number,
  expectedUrl: string,
  reader: MetadataReader,
  log: Logger,
): Promise<MetadataResult> {
  let text: string;
  try {
    text = await reader.read(tabId, expectedUrl);
  } catch {
    const failed = readFailureResult();
    log.error(metadataLogLabel, { omissions: failed.omissions });
    return failed;
  }
  const read = parsePageMetadata(text);
  if (read.kind === "rejected") {
    const failed = readFailureResult(read.check);
    log.error(metadataLogLabel, {
      check: read.check,
      omissions: failed.omissions,
    });
    return failed;
  }
  const result = acceptMetadata(read.page);
  const accepted = Object.keys(result.values);
  const collapsed = read.page.descriptionCollapsed;
  if (result.omissions.length > 0) {
    log.info(metadataLogLabel, {
      accepted,
      collapsed,
      omissions: result.omissions,
    });
  } else {
    log.info(metadataLogLabel, { accepted, collapsed });
  }
  return result;
}

/**
 * Popup path. First keeps the stored settings from the page; when that fails
 * it injects nothing and returns a rejected view. Then collects the active
 * tab's selection; after a successful collection it reads the video metadata
 * once and checks the body size, then
 * builds what the popup shows, with the tab's place for the send. A rejected
 * collection becomes a rejected view without reading the metadata, and so
 * does a tab whose place cannot be read. It touches no DOM, so nothing is
 * shown until the caller renders the returned view. Never throws.
 */
export async function runPopupLaunch(
  deps: PopupLaunchDeps,
): Promise<PopupLaunch> {
  let tab: chrome.tabs.Tab | undefined;
  try {
    tab = await deps.activeTab();
  } catch (error) {
    deps.log.error(activeTabLabel, error);
    tab = undefined;
  }
  const launch = launchContextFromTab(tab);
  // The settings are kept from the page before anything is injected into it.
  const protectedSettings = await deps.protectSettings();
  if (!protectedSettings.ok) {
    deps.log.error(protectLabel, { set: false });
    return { view: { kind: "rejected", reason: "settings-unprotected" } };
  }
  const outcome = await collect(launch, deps.reader);
  // The log holds the outcome's kind and reason, never the collected input:
  // the selection, the URL and the tab title are values the log must not hold.
  if (outcome.kind === "rejected") {
    deps.log.info(popupLogLabel, {
      outcome: { kind: "rejected", reason: outcome.reason },
    });
    return { view: { kind: "rejected", reason: outcome.reason } };
  }
  // collect never collects without a launch context, which needs a tab; this
  // only narrows them.
  const source =
    tab === undefined || launch === undefined
      ? undefined
      : sourceTab(tab, launch.tabId);
  if (launch === undefined || source === undefined) {
    // The start needs the tab's place to open the viewer beside it.
    deps.log.error(activeTabLabel, { check: "tab-place" });
    deps.log.info(popupLogLabel, {
      outcome: { kind: "rejected", reason: "collection-failed" },
    });
    return { view: { kind: "rejected", reason: "collection-failed" } };
  }
  deps.log.info(popupLogLabel, { outcome: { kind: "collected" } });
  const metadata = await readMetadata(
    launch.tabId,
    outcome.input.url,
    deps.metadataReader,
    deps.log,
  );
  const prepared = prepareSend(outcome.input, metadata.values);
  if (prepared.kind === "blocked") {
    // A blocked launch is normal, not a defect.
    deps.log.info(blockedLabel, { reason: prepared.reason });
  }
  return {
    view: buildCollectedView(outcome.input, metadata, prepared),
    source,
  };
}

/** The existing article's state for the choice, or undefined when the choice ends. */
function existingState(
  record: JobRecord | undefined,
): ExistingState | undefined {
  switch (record?.state) {
    case "running":
      return "running";
    case "done":
      return "done";
    case "failed":
    case "stopped":
    case undefined:
      return undefined;
  }
}

/**
 * Popup send path. Contract: the primary button becomes pressable only when
 * the view is collected and its selection is sendable; for any other view it
 * does nothing, so the button stays disabled as the skeleton and renderPopup
 * left it. Nothing is sent before the first press. Each press that sends asks
 * the service worker for one generation start and never sends one again by
 * itself. The first press asks with an inquiry. From then on the popup shows
 * the job states in place of the checks, keeping the video section and the
 * details: the job's record is read from chrome.storage.session and followed
 * through its change events, never polled. The buttons do what primaryAction
 * and secondaryAction say: nothing while there is no job, open the viewer of
 * a job, retry, which asks for one new start of the same input in the mode of
 * the send it replaces, open the existing article of a same-video answer,
 * which sends nothing, generate from the current selection, a plain start,
 * or, once the existing article is gone, a new inquiry. The setup operation
 * opens the options page. It moves the focus back to the primary button a
 * keyboard press disabled, once the button can be pressed again, and to the
 * primary button from a secondary button about to be hidden; it never throws.
 */
export function runPopupSend(
  elements: PopupElements,
  launch: PopupLaunch,
  deps: PopupSendDeps,
): void {
  const { view, source } = launch;
  if (
    view.kind !== "collected" ||
    view.selection.status.kind !== "sendable" ||
    source === undefined
  ) {
    return;
  }
  const input: StartInput = view.selection.status.request.body.message;
  const { primary, secondary, setup } = elements;
  const doc = primary.ownerDocument;

  let pressed = false;
  // Set when a keyboard press disables the button: the browser then drops
  // its focus, and it goes back to the button once it can be pressed. A mouse
  // press moves no focus, as the popup gives no element the focus when it
  // opens.
  let returnFocus = false;
  let state: PopupJobState = { kind: "sending" };
  let sentAt = 0;
  /** The job of the current send, once the worker named one. */
  let jobId: string | undefined;
  /** The mode of the current send; a retry sends in it again. */
  let mode: SendMode = "inquiry";
  let stopWatching = (): void => {};
  // Counts the sends, so a reply or an open-viewer that outlives its send
  // changes nothing.
  let sends = 0;
  let opening = false;
  let cancelRedraw = (): void => {};

  const draw = (): void => {
    const jobView: JobPopupView = {
      kind: "job",
      video: view.video,
      details: view.details,
      state,
    };
    renderPopup(elements, jobView, deps.locale, deps.now() - sentAt);
  };

  /** Draws the elapsed time again each second while a running job is shown. */
  const armRedraw = (): void => {
    cancelRedraw();
    cancelRedraw = () => {};
    if (state.kind === "job" && state.job.kind === "running") {
      cancelRedraw = deps.schedule(elapsedRedrawMs, () => {
        draw();
        armRedraw();
      });
    }
  };

  /**
   * Gives the focus back to the primary button once it can be pressed again
   * after a keyboard press disabled or hid the button pressed, when no element
   * has the focus then: an element the user focused in between keeps it.
   */
  const restoreFocus = (): void => {
    if (!returnFocus || primary.disabled) {
      return;
    }
    returnFocus = false;
    if (doc.activeElement === doc.body) {
      primary.focus();
    }
  };

  const show = (next: PopupJobState): void => {
    // A focused secondary button about to be hidden hands the focus to the
    // primary button first, while that one can still be pressed.
    if (
      doc.activeElement === secondary &&
      secondaryAction(next) === "none" &&
      primaryAction(next) !== "none" &&
      !primary.disabled
    ) {
      primary.focus();
    }
    state = next;
    draw();
    armRedraw();
    restoreFocus();
  };

  /** Shows what the record of the job says, in the job states or in the choice. */
  const onRecord = (record: JobRecord | undefined): void => {
    if (state.kind === "choice") {
      const existing = existingState(record);
      if (existing === undefined) {
        stopWatching();
        show({ kind: "choice-ended" });
      } else {
        show({ kind: "choice", existing });
      }
      return;
    }
    show({ kind: "job", job: recordDisplay(record) });
  };

  /**
   * Follows the record of watched: its change events first, then one read,
   * so no change can come between them unseen. A read that a change event
   * overtook is dropped; a read that fails keeps the state shown.
   */
  const watch = (watched: string): void => {
    const send = sends;
    const name = jobItemName(watched);
    let changed = false;
    stopWatching = deps.onSessionChanged((changes) => {
      const change = changes.get(name);
      if (change === undefined || send !== sends) {
        return;
      }
      changed = true;
      onRecord(
        change.removed ? undefined : readJobRecord(watched, change.newValue),
      );
    });
    void deps.readSession([name]).then((read) => {
      if (changed || send !== sends) {
        return;
      }
      if (!read.ok) {
        deps.log.error(recordLabel, { read: false });
        return;
      }
      onRecord(readJobRecord(watched, read.value[name]));
    });
  };

  /**
   * Waits for the worker's reply until startReplyWaitMs after the send;
   * undefined at the deadline, and for a reply that failed or fails its
   * check.
   */
  const askWorker = (request: SwRequest): Promise<StartReply | undefined> =>
    new Promise((resolve) => {
      let settled = false;
      const cancel = deps.schedule(startReplyWaitMs, () => {
        if (!settled) {
          settled = true;
          resolve(undefined);
        }
      });
      void deps.sendToSw(request).then((reply) => {
        if (settled) {
          return;
        }
        settled = true;
        cancel();
        resolve(reply.ok ? parseStartReply(reply.value) : undefined);
      });
    });

  /**
   * Sends one start of the input in sendMode, from the sending state. A
   * same-video answer shows the choice and follows the existing article's
   * record; an accepted one shows the job states and follows its record.
   */
  const start = async (sendMode: SendMode): Promise<void> => {
    stopWatching();
    stopWatching = () => {};
    jobId = undefined;
    mode = sendMode;
    sends += 1;
    const send = sends;
    // An open-viewer of the replaced send gives up at its next step; it must
    // not keep the new send's open-viewer from starting meanwhile.
    opening = false;
    sentAt = deps.now();
    show({ kind: "sending" });
    const reply = await askWorker({
      type: "start",
      mode: sendMode,
      input,
      source,
    });
    if (send !== sends) {
      return;
    }
    switch (reply?.status) {
      case undefined:
      case "unknown":
        show({ kind: "start-failed", failure: { kind: "unknown" } });
        return;
      case "rejected":
        show({
          kind: "start-failed",
          failure: { kind: "rejected", reason: reply.reason },
        });
        return;
      case "accepted":
        jobId = reply.jobId;
        show({ kind: "loading" });
        watch(reply.jobId);
        return;
      case "same_video":
        jobId = reply.jobId;
        show({ kind: "choice", existing: "loading" });
        watch(reply.jobId);
        return;
    }
  };

  /** Selects the tab and brings its window to the front, which closes the popup. */
  const bringForward = async (place: TabPlace): Promise<void> => {
    await deps.tabs.activate(place.tabId);
    await deps.focusWindow(place.windowId);
  };

  /**
   * Brings forward a viewer that answers it shows the job, or opens a new
   * viewer tab of the job; existing marks the new tab as showing an existing
   * article. It never sends a start. A retry pressed while it looks ends it:
   * the viewer it would bring forward shows the send the retry replaced, and
   * bringing a window forward would close the popup.
   */
  const openViewer = async (
    viewerJob: string,
    existing: boolean,
  ): Promise<void> => {
    const send = sends;
    const current = (): boolean => send === sends;
    const answers = await findJobViewers(viewerJob, {
      openChannel: deps.openChannel,
      queryId: deps.queryId,
      schedule: deps.schedule,
    });
    if (!current()) {
      return;
    }
    const chosen = chooseViewer(answers);
    if (chosen !== undefined) {
      const place = await deps.tabs.get(chosen.tabId);
      if (!current()) {
        return;
      }
      if (place.ok) {
        await bringForward(place.value);
        return;
      }
    }
    const title = startUrlTitle(input);
    if (title === undefined) {
      deps.log.error(recordLabel, { viewerUrl: "no-title" });
      return;
    }
    const query = jobViewerQuery({
      target: existing
        ? { kind: "job", jobId: viewerJob, existing: true }
        : { kind: "job", jobId: viewerJob },
      videoUrl: input.videoUrl,
      title,
    });
    const created = await deps.tabs.create({
      url: `${deps.identity.getURL("viewer.html")}?${query}`,
      active: true,
    });
    if (!created.ok || !current()) {
      return;
    }
    await deps.focusWindow(created.value.windowId);
  };

  /** Opens a viewer of the current job once at a time; a press while one opens does nothing. */
  const openOnce = (existing: boolean): void => {
    if (opening || jobId === undefined) {
      return;
    }
    opening = true;
    const send = sends;
    void openViewer(jobId, existing).then(() => {
      // A start since then already cleared the flag, and may have set it for
      // its own open-viewer.
      if (send === sends) {
        opening = false;
      }
    });
  };

  primary.addEventListener("click", (event) => {
    if (!pressed) {
      pressed = true;
      returnFocus = isKeyboardPress(event);
      void start("inquiry");
      return;
    }
    switch (primaryAction(state)) {
      case "none":
        return;
      case "retry":
        returnFocus = isKeyboardPress(event);
        void start(mode);
        return;
      case "start-inquiry":
        returnFocus = isKeyboardPress(event);
        void start("inquiry");
        return;
      case "open-viewer":
        openOnce(false);
        return;
      case "open-existing":
        openOnce(true);
        return;
    }
  });
  secondary.addEventListener("click", (event) => {
    switch (secondaryAction(state)) {
      case "none":
        return;
      case "start-plain":
        returnFocus = isKeyboardPress(event);
        void start("plain");
        return;
    }
  });
  setup.addEventListener("click", () => {
    void deps.openOptions();
  });
  primary.disabled = false;
}

/**
 * Whether a click came from the keyboard (Enter or Space on a button), which
 * the browser reports as a click with no pointer clicks counted.
 */
function isKeyboardPress(event: Event): boolean {
  return "detail" in event && event.detail === 0;
}

/**
 * The skeleton's elements, found in doc by id. Contract: undefined unless every
 * element is present and of its kind -- the four buttons button elements and
 * the details a details element, checked against the document's own constructors
 * -- so a partial skeleton is never drawn into.
 */
export function findPopupElements(doc: Document): PopupElements | undefined {
  const win = doc.defaultView;
  const video = doc.getElementById("video");
  const checks = doc.getElementById("checks");
  const rejection = doc.getElementById("rejection");
  const action = doc.getElementById("action");
  const primary = doc.getElementById("primary");
  const secondary = doc.getElementById("secondary");
  const setup = doc.getElementById("setup");
  const status = doc.getElementById("status");
  const details = doc.getElementById("details");
  const options = doc.getElementById("options");
  const buildMark = doc.getElementById("build-mark");
  if (
    win === null ||
    buildMark === null ||
    video === null ||
    checks === null ||
    rejection === null ||
    action === null ||
    status === null ||
    !(primary instanceof win.HTMLButtonElement) ||
    !(secondary instanceof win.HTMLButtonElement) ||
    !(setup instanceof win.HTMLButtonElement) ||
    !(options instanceof win.HTMLButtonElement) ||
    !(details instanceof win.HTMLDetailsElement)
  ) {
    return undefined;
  }
  return {
    buildMark,
    video,
    checks,
    rejection,
    action,
    primary,
    secondary,
    setup,
    status,
    details,
    options,
  };
}

/**
 * Popup entry. Reads the browser's UI language and logs it as read, decides
 * the locale from it, and sets the page language before the first render so
 * the Japanese typography rules are in place from the first frame. Then finds
 * the skeleton in popup.html -- with any element missing it runs no path and
 * draws nothing -- draws the options operation and the build mark, runs the
 * launch path, draws the view it returns in one
 * render, and runs the send path. Until that render every section stays
 * hidden, so nothing shown is later replaced. The UI-language log keeps its
 * fixed label in every locale, and the value it records is never displayed or
 * sent. Never throws.
 */
export async function startPopup(
  doc: Document,
  deps: PopupStartDeps,
): Promise<void> {
  const uiLanguage = deps.uiLanguage();
  deps.launch.log.info(uiLanguageLabel, uiLanguage);
  const locale = resolveLocale(uiLanguage);
  applyPageLocale(doc, locale);
  const elements = findPopupElements(doc);
  if (elements === undefined) {
    return;
  }
  renderPopupFrame(elements, locale, deps.buildKind);
  elements.options.addEventListener("click", () => {
    void deps.send.openOptions();
  });
  const launch = await runPopupLaunch(deps.launch);
  renderPopup(elements, launch.view, locale);
  runPopupSend(elements, launch, { ...deps.send, locale });
}
