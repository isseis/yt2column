import { collect } from "./core/collect.ts";
import type { CollectOutcome, SelectionReader } from "./core/collect.ts";
import {
  acceptMetadata,
  emptyMetadataResult,
  parsePageMetadata,
  readFailureResult,
} from "./core/metadata.ts";
import type { MetadataResult } from "./core/metadata.ts";
import { prepareSend } from "./core/prepare.ts";
import type { PrepareOutcome } from "./core/prepare.ts";
import { toSendOutcome } from "./core/response.ts";
import type { SendResult } from "./core/response.ts";
import { parseResultView } from "./core/resultView.ts";
import type { ResultView } from "./core/resultView.ts";
import { unreadableResultMessage } from "./core/sendMessages.ts";
import type { SettingsReader } from "./core/settings.ts";
import { summarize } from "./core/summary.ts";
import type { LaunchContext } from "./core/types.ts";
import { sendArticle, sendTimeoutMs } from "./send.ts";
import type { SendDeps } from "./send.ts";
import { messageNodes, renderMetadata, renderSummary } from "./ui/render.ts";
import {
  renderResultView,
  renderSendOutcome,
  sendingMessage,
} from "./ui/renderSend.ts";

/** Developer-facing log. The first argument is always a fixed label. */
export interface Logger {
  info(label: string, data: unknown): void;
  error(label: string, data: unknown): void;
}

/** Hands a ResultView from the service worker to the result window. */
export interface ResultViewStore {
  put(key: string, view: ResultView): Promise<void>;
  /** Returns the stored value; undefined when absent. Does not remove it. */
  get(key: string): Promise<unknown>;
  remove(key: string): Promise<void>;
  /**
   * Calls listener with each value newly stored under key in storage.session.
   * Removals are not reported. Returns a function that stops the listener.
   */
  watch(key: string, listener: (value: unknown) => void): () => void;
}

/** The only browser capability the metadata read needs. Faked in unit tests. */
export interface MetadataReader {
  /** Reads the metadata of the tab's top frame as JSON text. Rejects on any failure. */
  read(tabId: number, expectedUrl: string): Promise<string>;
}

export interface MenuLaunchDeps {
  readonly reader: SelectionReader;
  readonly metadataReader: MetadataReader;
  readonly settings: SettingsReader;
  readonly send: SendDeps;
  readonly store: ResultViewStore;
  readonly newKey: () => string;
  readonly now: () => number;
  openResultWindow(key: string): Promise<void>;
  /** Visible fallback when the result window cannot be shown (toolbar badge). */
  signalDisplayFailure(): Promise<void>;
  clearDisplayFailure(): Promise<void>;
  readonly log: Logger;
}

export interface PopupLaunchDeps {
  readonly reader: SelectionReader;
  readonly metadataReader: MetadataReader;
  activeTab(): Promise<chrome.tabs.Tab | undefined>;
  readonly log: Logger;
}

/** The popup launch's outcome and the metadata the send will carry. */
export interface PopupLaunchResult {
  readonly outcome: CollectOutcome;
  readonly metadata: MetadataResult;
}

export interface PopupSendDeps {
  readonly settings: SettingsReader;
  readonly send: SendDeps;
  readonly now: () => number;
  readonly log: Logger;
}

export interface ResultWindowDeps {
  readonly store: ResultViewStore;
  /** Runs callback once after ms milliseconds; returns a function that cancels it. */
  schedule(ms: number, callback: () => void): () => void;
  readonly log: Logger;
}

const menuLogLabel = "yt2column menu launch";
const popupLogLabel = "yt2column popup launch";
const metadataLogLabel = "yt2column video metadata";
const resultLogLabel = "yt2column result window";
const clearBadgeLabel = "yt2column clear badge";
const putLabel = "yt2column store result view";
const openWindowLabel = "yt2column open result window";
const removeLabel = "yt2column remove result view";
const badgeLabel = "yt2column signal display failure";
const activeTabLabel = "yt2column active tab";
const keyLabel = "yt2column create result view key";
const blockedLabel = "yt2column blocked";
const sendLabel = "yt2column send result";

/** How long a result window waits for the result after it opens. */
export const resultWaitMs = sendTimeoutMs + 10_000;

/** Copies id, url and title verbatim; undefined when there is no tab or no tab id. */
export function launchContextFromTab(
  tab: chrome.tabs.Tab | undefined,
): LaunchContext | undefined {
  if (tab === undefined || tab.id === undefined) {
    return undefined;
  }
  return { tabId: tab.id, url: tab.url, title: tab.title };
}

/** Runs action, recording a failure with the fixed label instead of throwing. */
async function attempt(
  action: () => Promise<void>,
  log: Logger,
  label: string,
): Promise<void> {
  try {
    await action();
  } catch (error) {
    log.error(label, error);
  }
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

/** A launch that ends without a request, after the collection and settings checks. */
type BlockedPrepare = Extract<PrepareOutcome, { readonly kind: "blocked" }>;

/**
 * Logs a blocked launch. Everything but a settings read failure is normal and
 * goes to info; a settings read failure is a defect and goes to error.
 */
function logBlocked(log: Logger, prepared: BlockedPrepare): void {
  const data = { reason: prepared.reason, cause: prepared.cause };
  if (prepared.reason === "settings-unreadable") {
    log.error(blockedLabel, data);
  } else {
    log.info(blockedLabel, data);
  }
}

/**
 * Logs one send attempt: the start time, the duration, the port and the
 * result. A result that is a failure goes to error; accepted and rejected go
 * to info. The token is never part of the logged values.
 */
function logSend(
  log: Logger,
  now: () => number,
  startedAt: number,
  port: string,
  result: SendResult,
): void {
  const data = {
    at: new Date(startedAt).toISOString(),
    durationMs: now() - startedAt,
    port,
    result,
  };
  if (result.kind === "accepted" || result.kind === "rejected") {
    log.info(sendLabel, data);
  } else {
    log.error(sendLabel, data);
  }
}

/**
 * Context-menu path: collect, prepare, show the window, then send once. The
 * sending view is written before the window opens, and the send only starts
 * once that write and the window both succeed. Every failure is logged; the
 * function never throws, and it never reads info.selectionText.
 */
export async function runMenuLaunch(
  info: chrome.contextMenus.OnClickData,
  tab: chrome.tabs.Tab | undefined,
  deps: MenuLaunchDeps,
): Promise<void> {
  await attempt(deps.clearDisplayFailure, deps.log, clearBadgeLabel);
  const launch = launchContextFromTab(tab);
  const outcome = await collect(launch, deps.reader);
  deps.log.info(menuLogLabel, {
    pageUrl: info.pageUrl,
    frameId: info.frameId,
    outcome,
  });
  // The page is read only after a successful collection, and only once.
  const metadata =
    outcome.kind === "collected" && launch !== undefined
      ? await readMetadata(
          launch.tabId,
          outcome.input.url,
          deps.metadataReader,
          deps.log,
        )
      : emptyMetadataResult();
  let key: string;
  try {
    key = deps.newKey();
  } catch (error) {
    deps.log.error(keyLabel, error);
    await attempt(deps.signalDisplayFailure, deps.log, badgeLabel);
    return;
  }
  const prepared = await prepareSend(outcome, deps.settings, metadata.values);
  if (prepared.kind === "blocked") {
    logBlocked(deps.log, prepared);
    try {
      await deps.store.put(key, { kind: "blocked", reason: prepared.reason });
    } catch (error) {
      deps.log.error(putLabel, error);
      await attempt(deps.signalDisplayFailure, deps.log, badgeLabel);
      return;
    }
    await openResultWindow(key, deps);
    return;
  }
  const summary = summarize(outcome);
  if (summary.kind !== "collected") {
    // Unreachable: prepareSend returns ready only for a collected input.
    deps.log.error(menuLogLabel, "a ready request without a collected input");
    return;
  }
  try {
    await deps.store.put(key, { kind: "sending", summary });
  } catch (error) {
    deps.log.error(putLabel, error);
    await attempt(deps.signalDisplayFailure, deps.log, badgeLabel);
    return;
  }
  if (!(await openResultWindow(key, deps))) {
    return;
  }
  const startedAt = deps.now();
  const result = await sendArticle(prepared.request, deps.send);
  logSend(deps.log, deps.now, startedAt, prepared.request.port, result);
  try {
    await deps.store.put(key, {
      kind: "sent",
      summary,
      outcome: toSendOutcome(result),
    });
  } catch (error) {
    deps.log.error(putLabel, error);
    await attempt(deps.signalDisplayFailure, deps.log, badgeLabel);
  }
}

/** Opens the result window for key; on failure removes the key and badges. */
async function openResultWindow(
  key: string,
  deps: MenuLaunchDeps,
): Promise<boolean> {
  try {
    await deps.openResultWindow(key);
    return true;
  } catch (error) {
    deps.log.error(openWindowLabel, error);
    await attempt(() => deps.store.remove(key), deps.log, removeLabel);
    await attempt(deps.signalDisplayFailure, deps.log, badgeLabel);
    return false;
  }
}

/**
 * Popup path. Collects the active tab's selection and renders the summary
 * into container; after a successful collection it reads the video metadata
 * once and renders it below the summary. Returns the outcome and the metadata
 * for the send controls. Never throws.
 */
export async function runPopupLaunch(
  container: HTMLElement,
  deps: PopupLaunchDeps,
): Promise<PopupLaunchResult> {
  let tab: chrome.tabs.Tab | undefined;
  try {
    tab = await deps.activeTab();
  } catch (error) {
    deps.log.error(activeTabLabel, error);
    tab = undefined;
  }
  const launch = launchContextFromTab(tab);
  const outcome = await collect(launch, deps.reader);
  deps.log.info(popupLogLabel, { outcome });
  renderSummary(container, summarize(outcome));
  if (outcome.kind !== "collected" || launch === undefined) {
    return { outcome, metadata: emptyMetadataResult() };
  }
  const metadata = await readMetadata(
    launch.tabId,
    outcome.input.url,
    deps.metadataReader,
    deps.log,
  );
  renderMetadata(container, metadata);
  return { outcome, metadata };
}

/**
 * Popup send controls below the summary. Checks once when the popup opens,
 * shows a disabled button while sending, and sends at most once per popup.
 * The expand prompt does not change the button: a send carries the metadata
 * read at launch. Never throws.
 */
export async function runPopupSend(
  container: HTMLElement,
  result: PopupLaunchResult,
  deps: PopupSendDeps,
): Promise<void> {
  const { outcome } = result;
  if (outcome.kind === "rejected") {
    // The summary region already shows the rejection reason.
    container.replaceChildren();
    return;
  }
  const prepared = await prepareSend(
    outcome,
    deps.settings,
    result.metadata.values,
  );
  if (prepared.kind === "blocked") {
    logBlocked(deps.log, prepared);
    renderResultView(container, { kind: "blocked", reason: prepared.reason });
    return;
  }
  const doc = container.ownerDocument;
  const status = doc.createElement("div");
  status.className = "send-message";
  const button = doc.createElement("button");
  button.type = "button";
  button.className = "send-button";
  button.textContent = "Send to yt2column";
  container.replaceChildren(button, status);

  let state: "unused" | "sending" | "sent" = "unused";
  button.addEventListener("click", () => {
    void (async () => {
      if (state !== "unused") {
        return;
      }
      state = "sending";
      button.disabled = true;
      status.replaceChildren(...messageNodes(doc, sendingMessage));
      const startedAt = deps.now();
      const result = await sendArticle(prepared.request, deps.send);
      logSend(deps.log, deps.now, startedAt, prepared.request.port, result);
      state = "sent";
      renderSendOutcome(status, toSendOutcome(result));
    })();
  });
}

/**
 * Result window. Reads the stored view under location.hash's key, follows it
 * until a final value or the wait limit, and shows one of: the blocked reason,
 * the sending notice, the send result, or a fixed "unknown/unreadable" notice.
 * It moves through its states in one direction only. Never throws.
 */
export async function runResultWindow(
  container: HTMLElement,
  hash: string,
  deps: ResultWindowDeps,
): Promise<void> {
  const doc = container.ownerDocument;
  const key = hash.replace(/^#/, "");
  let state: "waiting" | "sending" | "final" = "waiting";
  let stopWatch: () => void = () => {};
  let cancelWait: () => void = () => {};

  /** Whether the window has already reached Final. */
  const reachedFinal = (): boolean => state === "final";

  /** Moves to Final: stops following, renders once, then removes the key. */
  const finish = (render: () => void): void => {
    if (state === "final") {
      return;
    }
    state = "final";
    stopWatch();
    cancelWait();
    render();
    if (key !== "") {
      void deps.store.remove(key).catch((error) => {
        deps.log.error(removeLabel, error);
      });
    }
  };

  const showUnreadable = (detail: unknown): void => {
    deps.log.error(resultLogLabel, detail);
    finish(() => {
      container.replaceChildren(...messageNodes(doc, unreadableResultMessage));
    });
  };

  const onValue = (value: unknown): void => {
    if (state === "final") {
      return;
    }
    const view = parseResultView(value);
    if (view === undefined) {
      showUnreadable("the stored result view has an unexpected shape");
      return;
    }
    switch (view.kind) {
      case "sending":
        if (state === "waiting") {
          state = "sending";
          renderResultView(container, view);
        }
        return;
      case "blocked":
      case "sent":
        finish(() => {
          renderResultView(container, view);
        });
        return;
      default: {
        const exhaustive: never = view;
        deps.log.error(resultLogLabel, exhaustive);
      }
    }
  };

  if (key === "") {
    showUnreadable("the result window has no key");
    return;
  }

  // Watch before get, so a value stored between the two is not missed.
  stopWatch = deps.store.watch(key, onValue);
  cancelWait = deps.schedule(resultWaitMs, () => {
    if (state === "final") {
      return;
    }
    deps.log.error(
      resultLogLabel,
      "the result did not arrive before the wait limit",
    );
    finish(() => {
      renderSendOutcome(container, { kind: "unknown" });
    });
  });

  let stored: unknown;
  try {
    stored = await deps.store.get(key);
  } catch (error) {
    // A final view may have arrived while get was pending; Final ignores it.
    if (!reachedFinal()) {
      showUnreadable(error);
    }
    return;
  }
  if (stored !== undefined) {
    onValue(stored);
  } else if (state === "waiting") {
    showUnreadable("the stored result view is missing");
  }
}
