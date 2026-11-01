import { collect } from "./core/collect.ts";
import type { SelectionReader } from "./core/collect.ts";
import {
  acceptMetadata,
  parsePageMetadata,
  readFailureResult,
} from "./core/metadata.ts";
import type { MetadataResult } from "./core/metadata.ts";
import { catalogs } from "./core/i18n/catalog.ts";
import { resolveLocale } from "./core/i18n/locale.ts";
import type { Locale } from "./core/i18n/locale.ts";
import { nativeHostName } from "./core/native.ts";
import { buildCollectedView } from "./core/popupView.ts";
import type { PopupView } from "./core/popupView.ts";
import { prepareSend } from "./core/prepare.ts";
import { toSendOutcome } from "./core/articlesResponse.ts";
import type { SendResult } from "./core/articlesResponse.ts";
import type { LaunchContext } from "./core/types.ts";
import { sendArticle } from "./send.ts";
import type { SendDeps } from "./send.ts";
import { applyPageLocale } from "./ui/pageLocale.ts";
import { messageNodes, renderPopup } from "./ui/render.ts";
import type { ActionControls, PopupElements } from "./ui/render.ts";
import { renderSendOutcome } from "./ui/renderSend.ts";

/** Developer-facing log. The first argument is always a fixed label. */
export interface Logger {
  info(label: string, data: unknown): void;
  error(label: string, data: unknown): void;
}

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

export interface PopupSendDeps {
  readonly send: SendDeps;
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
const sendLabel = "yt2column send result";
const uiLanguageLabel = "yt2column ui language";

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
 * Logs one send attempt: the start time, the duration, the host name and the
 * result. The result keeps what is dropped before display: the server's
 * status of an unexpected reply and the browser's error text of a host
 * failure, never a body's text. A result that is a failure goes to error;
 * accepted and rejected go to info.
 */
function logSend(
  log: Logger,
  now: () => number,
  startedAt: number,
  result: SendResult,
): void {
  const data = {
    at: new Date(startedAt).toISOString(),
    durationMs: now() - startedAt,
    host: nativeHostName,
    result,
  };
  if (result.kind === "accepted" || result.kind === "rejected") {
    log.info(sendLabel, data);
  } else {
    log.error(sendLabel, data);
  }
}

/**
 * Popup path. Collects the active tab's selection; after a successful
 * collection it reads the video metadata once and checks the body size, then
 * builds what the popup shows. A rejected collection becomes a rejected view
 * without reading the metadata. It touches no DOM, so nothing is shown until
 * the caller renders the returned view. Never throws.
 */
export async function runPopupLaunch(
  deps: PopupLaunchDeps,
): Promise<PopupView> {
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
  if (outcome.kind === "rejected") {
    return { kind: "rejected", reason: outcome.reason };
  }
  if (launch === undefined) {
    // collect never collects without a launch context; this only narrows it.
    return { kind: "rejected", reason: "collection-failed" };
  }
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
  return buildCollectedView(outcome.input, metadata, prepared);
}

/**
 * Popup send path. Contract: the primary button becomes pressable only when
 * the view is collected and its selection is sendable; for any other view it
 * does nothing, so the button stays disabled as the skeleton and renderPopup
 * left it. A press sends the view's request at most once per popup, disables
 * the button at once, and shows the sending notice and then the result in the
 * status region. It touches only the button and the status region, so the
 * other sections never change, and it never moves the focus. Never throws.
 */
export function runPopupSend(
  controls: ActionControls,
  view: PopupView,
  deps: PopupSendDeps,
): void {
  if (view.kind !== "collected" || view.selection.status.kind !== "sendable") {
    return;
  }
  const { request } = view.selection.status;
  const { primary, status } = controls;
  const doc = primary.ownerDocument;
  const catalog = catalogs[deps.locale];

  let state: "unused" | "sending" | "sent" = "unused";
  primary.addEventListener("click", () => {
    void (async () => {
      if (state !== "unused") {
        return;
      }
      state = "sending";
      primary.disabled = true;
      status.replaceChildren(...messageNodes(doc, catalog.sending));
      const startedAt = deps.now();
      const sendResult = await sendArticle(request, deps.send);
      logSend(deps.log, deps.now, startedAt, sendResult);
      state = "sent";
      renderSendOutcome(status, toSendOutcome(sendResult), deps.locale);
    })();
  });
  primary.disabled = false;
}

/**
 * The skeleton's elements, found in doc by id. Contract: undefined unless every
 * element is present and of its kind -- the primary button a button and the
 * details a details element, checked against the document's own constructors
 * -- so a partial skeleton is never drawn into.
 */
export function findPopupElements(doc: Document): PopupElements | undefined {
  const win = doc.defaultView;
  const video = doc.getElementById("video");
  const checks = doc.getElementById("checks");
  const rejection = doc.getElementById("rejection");
  const action = doc.getElementById("action");
  const primary = doc.getElementById("primary");
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
    !(details instanceof win.HTMLDetailsElement)
  ) {
    return undefined;
  }
  return { video, checks, rejection, action, primary, status, details };
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
  const view = await runPopupLaunch(deps.launch);
  renderPopup(elements, view, locale);
  runPopupSend(elements, view, { ...deps.send, locale });
}
