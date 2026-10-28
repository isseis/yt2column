import { collect } from "./core/collect.ts";
import type { CollectOutcome, SelectionReader } from "./core/collect.ts";
import {
  acceptMetadata,
  emptyMetadataResult,
  parsePageMetadata,
  readFailureResult,
} from "./core/metadata.ts";
import type { MetadataResult } from "./core/metadata.ts";
import { catalogs } from "./core/i18n/catalog.ts";
import { resolveLocale } from "./core/i18n/locale.ts";
import type { Locale } from "./core/i18n/locale.ts";
import { nativeHostName } from "./core/native.ts";
import { prepareSend } from "./core/prepare.ts";
import { toSendOutcome } from "./core/response.ts";
import type { SendResult } from "./core/response.ts";
import { summarize } from "./core/summary.ts";
import type { LaunchContext } from "./core/types.ts";
import { sendArticle } from "./send.ts";
import type { SendDeps } from "./send.ts";
import { applyPageLocale } from "./ui/pageLocale.ts";
import { messageNodes, renderMetadata, renderSummary } from "./ui/render.ts";
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
  /** The display language; it selects displayed text only, never a log. */
  readonly locale: Locale;
}

/** The popup launch's outcome and the metadata the send will carry. */
export interface PopupLaunchResult {
  readonly outcome: CollectOutcome;
  readonly metadata: MetadataResult;
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
  readonly launch: Omit<PopupLaunchDeps, "locale">;
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
  renderSummary(container, summarize(outcome), deps.locale);
  if (outcome.kind !== "collected" || launch === undefined) {
    return { outcome, metadata: emptyMetadataResult() };
  }
  const metadata = await readMetadata(
    launch.tabId,
    outcome.input.url,
    deps.metadataReader,
    deps.log,
  );
  renderMetadata(container, metadata, deps.locale);
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
  const prepared = prepareSend(outcome.input, result.metadata.values);
  const catalog = catalogs[deps.locale];
  const doc = container.ownerDocument;
  if (prepared.kind === "blocked") {
    // A blocked launch is normal, not a defect.
    deps.log.info(blockedLabel, { reason: prepared.reason });
    container.replaceChildren(
      ...messageNodes(doc, catalog.preflight[prepared.reason]),
    );
    return;
  }
  const status = doc.createElement("div");
  status.className = "send-message";
  const button = doc.createElement("button");
  button.type = "button";
  button.className = "send-button";
  button.textContent = catalog.sendButton;
  container.replaceChildren(button, status);

  let state: "unused" | "sending" | "sent" = "unused";
  button.addEventListener("click", () => {
    void (async () => {
      if (state !== "unused") {
        return;
      }
      state = "sending";
      button.disabled = true;
      status.replaceChildren(...messageNodes(doc, catalog.sending));
      const startedAt = deps.now();
      const sendResult = await sendArticle(prepared.request, deps.send);
      logSend(deps.log, deps.now, startedAt, sendResult);
      state = "sent";
      renderSendOutcome(status, toSendOutcome(sendResult), deps.locale);
    })();
  });
}

/**
 * Popup entry. Reads the browser's UI language and logs it as read, decides
 * the locale from it, and sets the page language before the first render so
 * the Japanese typography rules are in place from the first frame. Then runs
 * the launch path into the summary region and the send path into the send
 * region. The UI-language log keeps its fixed label in every locale, and the
 * value it records is never displayed or sent. Never throws.
 */
export async function startPopup(
  doc: Document,
  deps: PopupStartDeps,
): Promise<void> {
  const uiLanguage = deps.uiLanguage();
  deps.launch.log.info(uiLanguageLabel, uiLanguage);
  const locale = resolveLocale(uiLanguage);
  applyPageLocale(doc, locale);
  const summaryRegion = doc.getElementById("summary");
  const sendRegion = doc.getElementById("send");
  if (summaryRegion === null || sendRegion === null) {
    return;
  }
  const launched = await runPopupLaunch(summaryRegion, {
    ...deps.launch,
    locale,
  });
  await runPopupSend(sendRegion, launched, { ...deps.send, locale });
}
