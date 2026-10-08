import { collect } from "./core/collect.ts";
import type { CollectOutcome, SelectionReader } from "./core/collect.ts";
import { parseSummary, summarize } from "./core/summary.ts";
import type { LaunchContext, OutcomeSummary } from "./core/types.ts";
import { renderSummary } from "./ui/render.ts";

/** Developer-facing log. The first argument is always a fixed label. */
export interface Logger {
  info(label: string, data: unknown): void;
  error(label: string, data: unknown): void;
}

/** Hands an OutcomeSummary from the service worker to the result window. */
export interface SummaryStore {
  put(key: string, summary: OutcomeSummary): Promise<void>;
  /** Returns the stored value and removes it; undefined when absent. */
  take(key: string): Promise<unknown>;
  /**
   * Discards the stored value for key; used when the result window cannot be
   * opened. Rejects when the removal fails.
   */
  remove(key: string): Promise<void>;
}

export interface MenuLaunchDeps {
  readonly reader: SelectionReader;
  readonly store: SummaryStore;
  readonly newKey: () => string;
  openResultWindow(key: string): Promise<void>;
  /** Visible fallback when the result window cannot be shown (toolbar badge). */
  signalDisplayFailure(): Promise<void>;
  clearDisplayFailure(): Promise<void>;
  readonly log: Logger;
}

export interface PopupLaunchDeps {
  readonly reader: SelectionReader;
  activeTab(): Promise<chrome.tabs.Tab | undefined>;
  readonly log: Logger;
}

export interface ResultWindowDeps {
  readonly store: SummaryStore;
  readonly log: Logger;
}

const menuLogLabel = "yt2column menu launch";
const popupLogLabel = "yt2column popup launch";
const resultLogLabel = "yt2column result window";
const clearBadgeLabel = "yt2column clear badge";
const putLabel = "yt2column store summary";
const openWindowLabel = "yt2column open result window";
const removeLabel = "yt2column remove summary";
const badgeLabel = "yt2column signal display failure";
const activeTabLabel = "yt2column active tab";

/** The message shown when the result window finds no usable summary. */
export const missingResultText =
  "This result is no longer available. Launch yt2column again.";

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
 * Context-menu path. Collects the selection, stores the display summary, and
 * opens the result window. The stored summary is written before the window is
 * opened, and a window failure removes that key. Every failure is logged; the
 * function never throws, and it never reads info.selectionText.
 */
export async function runMenuLaunch(
  info: chrome.contextMenus.OnClickData,
  tab: chrome.tabs.Tab | undefined,
  deps: MenuLaunchDeps,
): Promise<void> {
  await attempt(deps.clearDisplayFailure, deps.log, clearBadgeLabel);
  const outcome = await collect(launchContextFromTab(tab), deps.reader);
  deps.log.info(menuLogLabel, {
    pageUrl: info.pageUrl,
    frameId: info.frameId,
    outcome,
  });
  const key = deps.newKey();
  try {
    await deps.store.put(key, summarize(outcome));
  } catch (error) {
    deps.log.error(putLabel, error);
    await attempt(deps.signalDisplayFailure, deps.log, badgeLabel);
    return;
  }
  try {
    await deps.openResultWindow(key);
  } catch (error) {
    deps.log.error(openWindowLabel, error);
    await attempt(() => deps.store.remove(key), deps.log, removeLabel);
    await attempt(deps.signalDisplayFailure, deps.log, badgeLabel);
  }
}

/**
 * Popup path. Collects the active tab's selection and renders the summary
 * into container, returning the outcome for #111's send button. Never throws.
 */
export async function runPopupLaunch(
  container: HTMLElement,
  deps: PopupLaunchDeps,
): Promise<CollectOutcome> {
  let tab: chrome.tabs.Tab | undefined;
  try {
    tab = await deps.activeTab();
  } catch (error) {
    deps.log.error(activeTabLabel, error);
    tab = undefined;
  }
  const outcome = await collect(launchContextFromTab(tab), deps.reader);
  deps.log.info(popupLogLabel, { outcome });
  renderSummary(container, summarize(outcome));
  return outcome;
}

/** Replaces the children of container with a single fixed notice. */
function renderNotice(container: HTMLElement, text: string): void {
  const notice = container.ownerDocument.createElement("p");
  notice.className = "summary-message";
  notice.textContent = text;
  container.replaceChildren(notice);
}

/**
 * Result window. Takes location.hash as is, renders the summary stored under
 * its key after taking it, or a fixed notice when there is none. Never throws.
 */
export async function runResultWindow(
  container: HTMLElement,
  hash: string,
  deps: ResultWindowDeps,
): Promise<void> {
  const key = hash.startsWith("#") ? hash.slice(1) : hash;
  if (key === "") {
    renderNotice(container, missingResultText);
    return;
  }
  let stored: unknown;
  try {
    stored = await deps.store.take(key);
  } catch (error) {
    deps.log.error(resultLogLabel, error);
    renderNotice(container, missingResultText);
    return;
  }
  const summary = parseSummary(stored);
  if (summary === undefined) {
    if (stored !== undefined) {
      deps.log.error(
        resultLogLabel,
        "the stored summary has an unexpected shape",
      );
    }
    renderNotice(container, missingResultText);
    return;
  }
  renderSummary(container, summary);
}
