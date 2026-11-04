import type {
  ExtensionIdentity,
  FocusWindow,
  OpenViewerChannel,
  SendToWorker,
  TabPlace,
  ViewerTabs,
} from "./browser/chromeDeps.ts";
import type { ArticleKey } from "./core/articleKey.ts";
import { collect } from "./core/collect.ts";
import type { SelectionReader } from "./core/collect.ts";
import { chooseViewer, parseStartReply } from "./core/extensionMessages.ts";
import type {
  SendMode,
  SourceTab,
  StartReply,
  StartRequest,
} from "./core/extensionMessages.ts";
import {
  acceptMetadata,
  parsePageMetadata,
  readFailureResult,
} from "./core/metadata.ts";
import type { MetadataResult } from "./core/metadata.ts";
import { resolveLocale } from "./core/i18n/locale.ts";
import type { Locale } from "./core/i18n/locale.ts";
import { elapsedRedrawMs, startResultDeadlineMs } from "./core/poll.ts";
import type { PollStep } from "./core/poll.ts";
import {
  buildCollectedView,
  primaryAction,
  secondaryAction,
  viewerSeed,
} from "./core/popupView.ts";
import type {
  JobPopupView,
  PopupJobState,
  PopupView,
} from "./core/popupView.ts";
import { prepareSend } from "./core/prepare.ts";
import type { LaunchContext, Logger } from "./core/types.ts";
import { viewerQuery } from "./core/viewerUrl.ts";
import type { ViewerSeed } from "./core/viewerUrl.ts";
import { createStatusPoller } from "./poller.ts";
import type { Poller } from "./poller.ts";
import type { SendDeps } from "./send.ts";
import { applyPageLocale } from "./ui/pageLocale.ts";
import { renderPopup } from "./ui/render.ts";
import type { PopupElements } from "./ui/render.ts";
import { findViewers } from "./viewerQuery.ts";

/** The only browser capability the metadata read needs. Faked in unit tests. */
export interface MetadataReader {
  /** Reads the metadata of the tab's top frame as JSON text. Rejects on any failure. */
  read(tabId: number, expectedUrl: string): Promise<string>;
}

export interface PopupLaunchDeps {
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
  /** Sends the start request to the service worker. */
  readonly sendToWorker: SendToWorker;
  /** The status exchanges, and the timer of every wait. */
  readonly send: SendDeps;
  readonly tabs: Pick<ViewerTabs, "create" | "activate" | "get">;
  readonly focusWindow: FocusWindow;
  readonly openChannel: OpenViewerChannel;
  /** A new query ID for each query to the viewers. */
  readonly queryId: () => string;
  /** chrome.runtime in production: the viewer page's URL. */
  readonly identity: Pick<ExtensionIdentity, "getURL">;
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
  readonly launch: PopupLaunchDeps;
  readonly send: Omit<PopupSendDeps, "locale">;
}

const popupLogLabel = "yt2column popup launch";
const metadataLogLabel = "yt2column video metadata";
const activeTabLabel = "yt2column active tab";
const blockedLabel = "yt2column blocked";
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
 * Popup path. Collects the active tab's selection; after a successful
 * collection it reads the video metadata once and checks the body size, then
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

/**
 * What a new viewer tab's URL holds when openViewer has to create one: the
 * current send's seed for open-viewer, or, for open-existing, the seed's
 * values with the press time as the send time and the existing-article mark.
 */
type NewViewerTab =
  | { readonly kind: "send" }
  | { readonly kind: "existing"; readonly pressedAt: number };

/**
 * Popup send path. Contract: the primary button becomes pressable only when
 * the view is collected and its selection is sendable; for any other view it
 * does nothing, so the button stays disabled as the skeleton and renderPopup
 * left it. Nothing is sent before the first press. Each press that sends asks
 * the service worker for one generation start and never sends one again by
 * itself; the popup never sends a start to the host. The first press asks
 * with an inquiry. From then on the popup shows the job states in place of
 * the checks, keeping the video section and the details, and the buttons do
 * what primaryAction and secondaryAction say: nothing while there is no key,
 * open the viewer for a key, retry, which asks for one new start of the same
 * input in the mode of the send it replaces, open the existing article of a
 * same-video answer, which sends nothing, generate from the current
 * selection, a plain start, or, once the existing article is gone, a new
 * inquiry. A key is polled until polling stops or the popup closes. It moves
 * the focus back to the primary button a keyboard press disabled, once the
 * button can be pressed again, and to the primary button from a secondary
 * button about to be hidden; it never throws.
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
  const input = view.selection.status.request.body.message;
  const { primary, secondary } = elements;
  const doc = primary.ownerDocument;

  let pressed = false;
  // Set when a keyboard press disables the button: the browser then drops
  // its focus, and it goes back to the button once it can be pressed. A mouse
  // press moves no focus, as the popup gives no element the focus when it
  // opens.
  let returnFocus = false;
  let state: PopupJobState = { kind: "sending" };
  let seed: ViewerSeed = viewerSeed(view.video, input, 0);
  let key: ArticleKey | undefined;
  /** The mode of the current send; a retry sends in it again. */
  let mode: SendMode = "inquiry";
  /** The tab that shows this send, as the worker or the last open-viewer found it. */
  let viewerTabId: number | undefined;
  let poller: Poller | undefined;
  // Counts the sends, so an open-viewer that outlives its send changes nothing.
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
    renderPopup(elements, jobView, deps.locale, deps.now() - seed.sentAt);
  };

  /** Draws the elapsed time again each second while a running job is shown. */
  const armRedraw = (): void => {
    cancelRedraw();
    cancelRedraw = () => {};
    if (state.kind === "job" && state.job.kind === "running") {
      cancelRedraw = deps.send.schedule(elapsedRedrawMs, () => {
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

  /**
   * A step of the existing article's poll while the choice is shown: running
   * and done keep the choice with the new state, a failed or lost article
   * ends it and stops polling, and a failed exchange adds its notice.
   */
  const onChoiceStep = (
    choice: Extract<PopupJobState, { kind: "choice" }>,
    step: PollStep,
  ): void => {
    switch (step.job?.kind) {
      case undefined:
        show({ ...choice, notice: step.notice });
        return;
      case "running":
      case "done":
        show({ kind: "choice", existing: step.job.kind, notice: step.notice });
        return;
      case "failed":
      case "lost":
        // afterStatus already stops at a failed or lost job; dropping the
        // poller here keeps the ended choice from depending on that.
        poller?.stop();
        poller = undefined;
        show({ kind: "choice-ended" });
        return;
      default: {
        const exhaustive: never = step.job;
        throw new Error(`Unhandled job: ${String(exhaustive)}`);
      }
    }
  };

  const onStep = (step: PollStep): void => {
    if (state.kind === "choice") {
      onChoiceStep(state, step);
    } else if (step.job !== undefined) {
      show({ kind: "job", job: step.job, notice: step.notice });
    } else if (state.kind === "job") {
      show({ ...state, notice: step.notice });
    } else {
      show({ kind: "loading", notice: step.notice });
    }
  };

  /**
   * Waits for the worker's reply until startResultDeadlineMs after the send
   * time; undefined at the deadline, and for a reply that is late, failed, or
   * fails its check.
   */
  const askWorker = (
    request: StartRequest,
  ): Promise<StartReply | undefined> => {
    const deadline = request.viewer.sentAt + startResultDeadlineMs;
    return new Promise((resolve) => {
      let settled = false;
      const cancel = deps.send.schedule(
        Math.max(0, deadline - deps.now()),
        () => {
          if (!settled) {
            settled = true;
            resolve(undefined);
          }
        },
      );
      void deps.sendToWorker(request).then((reply) => {
        if (settled) {
          return;
        }
        settled = true;
        cancel();
        // A timer that fired late can let a reply in after the deadline.
        resolve(
          reply.ok && deps.now() < deadline
            ? parseStartReply(reply.value)
            : undefined,
        );
      });
    });
  };

  /**
   * Sends one start of the input in sendMode, from the sending state. A
   * same-video answer shows the choice and polls the existing article's key;
   * an accepted one shows the job states and polls its key.
   */
  const start = async (sendMode: SendMode): Promise<void> => {
    poller?.stop();
    poller = undefined;
    key = undefined;
    viewerTabId = undefined;
    mode = sendMode;
    sends += 1;
    // An open-viewer of the replaced send gives up at its next step; it must
    // not keep the new send's open-viewer from starting meanwhile.
    opening = false;
    seed = viewerSeed(view.video, input, deps.now());
    show({ kind: "sending" });
    // Every action that sends is offered only once the previous reply is in,
    // so no earlier send's reply can still arrive here.
    const reply = await askWorker({
      type: "start",
      mode: sendMode,
      input,
      viewer: seed,
      source,
    });
    if (reply === undefined) {
      show({ kind: "start-failed", failure: { kind: "no-result" } });
      return;
    }
    const { outcome } = reply;
    switch (outcome.kind) {
      case "accepted":
      case "same-video":
        break;
      case "rejected":
      case "not-sent":
      case "no-response":
      case "host-failed":
      case "unexpected":
        show({ kind: "start-failed", failure: outcome });
        return;
    }
    key = outcome.key;
    viewerTabId = reply.viewerTabId;
    poller = createStatusPoller(key, deps, onStep);
    // The worker checks a same-video answer only for an inquiry, so it never
    // answers a plain send with one.
    show(
      outcome.kind === "same-video"
        ? { kind: "choice", existing: "loading", notice: { kind: "none" } }
        : { kind: "loading", notice: { kind: "none" } },
    );
    poller.start();
  };

  /** Selects the tab and brings its window to the front, which closes the popup. */
  const bringForward = async (place: TabPlace): Promise<void> => {
    await deps.tabs.activate(place.tabId);
    await deps.focusWindow(place.windowId);
  };

  /**
   * Brings forward the first that applies: the tab that shows this send when
   * it is still there; else a viewer that answers it shows the key; else a
   * new viewer tab of the key. It never sends a start. A retry pressed while
   * it looks ends it: the viewer it would bring forward shows the send the
   * retry replaced, and bringing a window forward would close the popup.
   */
  const openViewer = async (
    viewerKey: ArticleKey,
    newTab: NewViewerTab,
  ): Promise<void> => {
    const send = sends;
    const current = (): boolean => send === sends;
    if (viewerTabId !== undefined) {
      const known = await deps.tabs.get(viewerTabId);
      if (!current()) {
        return;
      }
      if (known.ok) {
        await bringForward(known.value);
        return;
      }
    }
    const answers = await findViewers(viewerKey, {
      openChannel: deps.openChannel,
      queryId: deps.queryId,
      schedule: deps.send.schedule,
    });
    if (!current()) {
      return;
    }
    const chosen = chooseViewer(answers.present);
    if (chosen !== undefined) {
      const place = await deps.tabs.get(chosen.tabId);
      if (!current()) {
        return;
      }
      if (place.ok) {
        viewerTabId = chosen.tabId;
        await bringForward(place.value);
        return;
      }
    }
    const query =
      newTab.kind === "send"
        ? viewerQuery({ ...seed, key: viewerKey })
        : viewerQuery({
            ...seed,
            sentAt: newTab.pressedAt,
            key: viewerKey,
            existing: true,
          });
    const created = await deps.tabs.create({
      url: `${deps.identity.getURL("viewer.html")}?${query}`,
      active: true,
    });
    if (!created.ok || !current()) {
      return;
    }
    viewerTabId = created.value.tabId;
    await deps.focusWindow(created.value.windowId);
  };

  /** Opens a viewer of the current key once at a time; a press while one opens does nothing. */
  const openOnce = (newTab: NewViewerTab): void => {
    if (opening || key === undefined) {
      return;
    }
    opening = true;
    const send = sends;
    void openViewer(key, newTab).then(() => {
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
        openOnce({ kind: "send" });
        return;
      case "open-existing":
        openOnce({ kind: "existing", pressedAt: deps.now() });
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
 * element is present and of its kind -- both buttons button elements and
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
  const status = doc.getElementById("status");
  const details = doc.getElementById("details");
  if (
    win === null ||
    video === null ||
    checks === null ||
    rejection === null ||
    action === null ||
    status === null ||
    !(primary instanceof win.HTMLButtonElement) ||
    !(secondary instanceof win.HTMLButtonElement) ||
    !(details instanceof win.HTMLDetailsElement)
  ) {
    return undefined;
  }
  return {
    video,
    checks,
    rejection,
    action,
    primary,
    secondary,
    status,
    details,
  };
}

/**
 * Popup entry. Reads the browser's UI language and logs it as read, decides
 * the locale from it, and sets the page language before the first render so
 * the Japanese typography rules are in place from the first frame. Then finds
 * the skeleton in popup.html -- with any element missing it runs no path and
 * draws nothing -- runs the launch path, draws the view it returns in one
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
  const launch = await runPopupLaunch(deps.launch);
  renderPopup(elements, launch.view, locale);
  runPopupSend(elements, launch, { ...deps.send, locale });
}
