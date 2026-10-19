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
import type { SettingsReader } from "./core/settings.ts";
import { summarize } from "./core/summary.ts";
import type { LaunchContext } from "./core/types.ts";
import { sendArticle } from "./send.ts";
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

const popupLogLabel = "yt2column popup launch";
const metadataLogLabel = "yt2column video metadata";
const activeTabLabel = "yt2column active tab";
const blockedLabel = "yt2column blocked";
const sendLabel = "yt2column send result";

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
      const sendResult = await sendArticle(prepared.request, deps.send);
      logSend(deps.log, deps.now, startedAt, prepared.request.port, sendResult);
      state = "sent";
      renderSendOutcome(status, toSendOutcome(sendResult));
    })();
  });
}
