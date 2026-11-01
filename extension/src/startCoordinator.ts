// The service worker's part of a generation start. The popup asks for a
// start; the worker checks who asked, opens the viewer tab in the background
// next to the YouTube tab, sends the start to the host exactly once, and
// hands the result to the viewer and the popup. When the key is accepted and
// another viewer already shows it, that viewer takes the send over and the
// new tab is closed, but only after the new tab confirmed it is still waiting
// for this send and the other viewer confirmed the take-over; whatever cannot
// be confirmed keeps the new tab. The worker keeps each send in memory only,
// for at most startResultDeadlineMs from its send time, and never polls.

import type {
  ExtensionIdentity,
  MessageListener,
  MessageSender,
  OpenViewerChannel,
  TabPlace,
  BrowserCall,
  ViewerTabs,
} from "./browser/chromeDeps.ts";
import {
  chooseViewer,
  parseAwaitStart,
  parseStartRequest,
} from "./core/extensionMessages.ts";
import type {
  AwaitStartReply,
  PresentAnswer,
  StartReply,
  StartRequest,
} from "./core/extensionMessages.ts";
import { startMessage } from "./core/hostMessage.ts";
import { startResultDeadlineMs } from "./core/poll.ts";
import type { PreparedRequest } from "./core/prepare.ts";
import {
  classifyStart,
  startLogView,
  toStartOutcome,
} from "./core/response.ts";
import type { StartOutcome } from "./core/response.ts";
import { viewerQuery } from "./core/viewerUrl.ts";
import type { Logger } from "./launch.ts";
import { exchange } from "./send.ts";
import type { SendDeps } from "./send.ts";
import { askViewers, findViewers } from "./viewerQuery.ts";
import type { ViewerQueryDeps } from "./viewerQuery.ts";

export interface StartCoordinatorDeps {
  /** chrome.runtime in production: the extension's ID and its page URLs. */
  readonly identity: ExtensionIdentity;
  readonly tabs: Pick<ViewerTabs, "create" | "remove">;
  readonly openChannel: OpenViewerChannel;
  readonly queryId: () => string;
  /** The host exchange; its schedule also times the queries and the deadline. */
  readonly send: SendDeps;
  readonly now: () => number;
  readonly log: Logger;
}

const startRequestLabel = "yt2column start request";
const startResultLabel = "yt2column start result";
const viewerTabLabel = "yt2column viewer tab";
const awaitStartLabel = "yt2column await start";

/** One send the worker holds until the deadline. */
interface StartRecord {
  readonly sentAt: number;
  /** True until tabs.create has completed. */
  creating: boolean;
  /** The viewer tab; undefined while it is being created and when it could not be. */
  tabId: number | undefined;
  /** The result for the viewer tab, once it is decided that the tab keeps this send. */
  result: StartOutcome | undefined;
  /** The viewer tab's waits that arrived before the result. */
  readonly waits: ((reply: AwaitStartReply) => void)[];
}

/** A wait that matched no record while a tab was still being created. */
interface HeldWait {
  readonly tabId: number;
  readonly sentAt: number;
  readonly respond: (reply: AwaitStartReply) => void;
}

/** Why a start request was ignored; for the developer log only. */
type SenderCheck = "sender-id" | "sender-tab" | "sender-url";

/**
 * The check of who sent a start request: the extension itself, not from a
 * tab, and exactly the toolbar popup's URL, query and fragment included.
 * Injected scripts and extension pages opened in a tab have a sender.tab.
 */
function startSenderCheck(
  sender: MessageSender,
  identity: ExtensionIdentity,
): SenderCheck | undefined {
  if (sender.id !== identity.id) {
    return "sender-id";
  }
  if (sender.tab !== undefined) {
    return "sender-tab";
  }
  if (sender.url !== identity.getURL("popup.html")) {
    return "sender-url";
  }
  return undefined;
}

/** The sender's tab when it is one of the extension's viewer pages; undefined otherwise. */
function viewerSenderTab(
  sender: MessageSender,
  identity: ExtensionIdentity,
): number | undefined {
  const tabId = sender.tab?.id;
  if (
    sender.id !== identity.id ||
    typeof tabId !== "number" ||
    sender.url === undefined
  ) {
    return undefined;
  }
  const query = sender.url.indexOf("?");
  const page = query === -1 ? sender.url : sender.url.slice(0, query);
  return page === identity.getURL("viewer.html") ? tabId : undefined;
}

/** The value a message declares as its type; undefined when it declares none. */
function declaredType(message: unknown): unknown {
  if (typeof message !== "object" || message === null) {
    return undefined;
  }
  return Object.hasOwn(message, "type")
    ? (message as { readonly type: unknown }).type
    : undefined;
}

/**
 * The service worker's runtime message listener. It answers two messages,
 * chosen by their declared type: a start request from the toolbar popup and
 * a viewer's wait for its start result. It returns true only for a message it
 * accepted and will answer; anything else, from any sender, gets false, no
 * host message and no tab.
 */
export function createStartCoordinator(
  deps: StartCoordinatorDeps,
): MessageListener {
  const records = new Set<StartRecord>();
  let held: HeldWait[] = [];
  const queryDeps: ViewerQueryDeps = {
    openChannel: deps.openChannel,
    queryId: deps.queryId,
    schedule: deps.send.schedule,
  };

  /** Answers a wait from its record, holds it while a tab is being created, or answers none. */
  const answerWait = (wait: HeldWait): "hit" | "held" | "none" => {
    for (const record of records) {
      if (record.tabId === wait.tabId && record.sentAt === wait.sentAt) {
        if (record.result === undefined) {
          record.waits.push(wait.respond);
        } else {
          wait.respond({ kind: "result", outcome: record.result });
        }
        return "hit";
      }
    }
    if ([...records].some((record) => record.creating)) {
      held.push(wait);
      return "held";
    }
    wait.respond({ kind: "none" });
    return "none";
  };

  /** Matches the held waits again once a tab creation has completed. */
  const answerHeld = (): void => {
    const waits = held;
    held = [];
    for (const wait of waits) {
      const answer = answerWait(wait);
      if (answer !== "held") {
        deps.log.info(awaitStartLabel, { record: answer });
      }
    }
  };

  /** Ends a record: its pending waits are answered none. */
  const discard = (record: StartRecord): void => {
    if (!records.delete(record)) {
      return;
    }
    for (const respond of record.waits.splice(0)) {
      respond({ kind: "none" });
    }
    answerHeld();
  };

  /** Gives the record's tab its result: the waits so far now, later waits from the record. */
  const deliver = (record: StartRecord, outcome: StartOutcome): void => {
    if (!records.has(record)) {
      return;
    }
    record.result = outcome;
    for (const respond of record.waits.splice(0)) {
      respond({ kind: "result", outcome });
    }
  };

  /** Asks viewer to take the send over; true only when it confirmed within the window. */
  const takeOver = async (
    queryId: string,
    viewer: PresentAnswer,
    sentAt: number,
  ): Promise<boolean> => {
    let tookOver = false;
    await askViewers(
      { type: "take-over", queryId, tabId: viewer.tabId, sentAt },
      queryDeps,
      (answer) => {
        tookOver =
          answer.type === "took-over" &&
          answer.queryId === queryId &&
          answer.tabId === viewer.tabId;
        return tookOver;
      },
    );
    return tookOver;
  };

  /**
   * Decides which tab shows the send once its result is known, and returns
   * that tab for the popup: the new tab, unless an accepted key is already
   * shown by another viewer, the new tab answered that it waits for this
   * send, that viewer took the send over, and the new tab was closed. The
   * new tab gets the result unless it was closed; a tab that could not be
   * created leaves the result to the popup alone.
   */
  const placeResult = async (
    record: StartRecord,
    created: BrowserCall<TabPlace>,
    outcome: StartOutcome,
  ): Promise<number | undefined> => {
    if (!created.ok) {
      deps.log.error(viewerTabLabel, { created: false });
      deliver(record, outcome);
      return undefined;
    }
    const newTab = created.value.tabId;
    if (outcome.kind !== "accepted") {
      deps.log.info(viewerTabLabel, { created: true });
      deliver(record, outcome);
      return newTab;
    }
    const answers = await findViewers(outcome.key, queryDeps);
    const waiting = answers.waiting.some(
      (answer) => answer.tabId === newTab && answer.sentAt === record.sentAt,
    );
    const viewer = chooseViewer(
      answers.present.filter((answer) => answer.tabId !== newTab),
    );
    let tookOver = false;
    let remove: "not-attempted" | "ok" | "failed" = "not-attempted";
    if (viewer !== undefined && waiting) {
      tookOver = await takeOver(answers.queryId, viewer, record.sentAt);
      if (tookOver) {
        remove = (await deps.tabs.remove(newTab)).ok ? "ok" : "failed";
      }
    }
    const tabLog = {
      created: true,
      present: answers.present.length,
      waiting: answers.waiting.length,
      tookOver,
      remove,
    };
    if (remove === "failed") {
      deps.log.error(viewerTabLabel, tabLog);
    } else {
      deps.log.info(viewerTabLabel, tabLog);
    }
    if (viewer !== undefined && remove === "ok") {
      discard(record);
      return viewer.tabId;
    }
    deliver(record, outcome);
    return newTab;
  };

  /**
   * The one handler of a start request that passed every check: it opens the
   * viewer tab, sends the start to the host exactly once and never again,
   * places the result and answers the popup. The record ends at the deadline
   * whatever happens.
   */
  const runAcceptedStart = async (
    request: StartRequest,
    prepared: PreparedRequest,
    respond: (reply: StartReply) => void,
  ): Promise<void> => {
    const record: StartRecord = {
      sentAt: request.viewer.sentAt,
      creating: true,
      tabId: undefined,
      result: undefined,
      waits: [],
    };
    records.add(record);
    deps.send.schedule(
      Math.max(0, record.sentAt + startResultDeadlineMs - deps.now()),
      () => discard(record),
    );
    const creation = deps.tabs
      .create({
        url: `${deps.identity.getURL("viewer.html")}?${viewerQuery(request.viewer)}`,
        active: false,
        windowId: request.source.windowId,
        index: request.source.index + 1,
        openerTabId: request.source.tabId,
      })
      .then((created) => {
        record.creating = false;
        record.tabId = created.ok ? created.value.tabId : undefined;
        answerHeld();
        return created;
      });
    const startedAt = deps.now();
    const result = classifyStart(
      await exchange(startMessage(prepared), deps.send),
    );
    const durationMs = deps.now() - startedAt;
    const resultLog = { result: startLogView(result), durationMs };
    if (result.kind === "accepted" || result.kind === "rejected") {
      deps.log.info(startResultLabel, resultLog);
    } else {
      deps.log.error(startResultLabel, resultLog);
    }
    const outcome = toStartOutcome(result);
    const viewerTabId = await placeResult(record, await creation, outcome);
    respond(viewerTabId === undefined ? { outcome } : { outcome, viewerTabId });
  };

  const onStartRequest = (
    message: unknown,
    sender: MessageSender,
    sendResponse: (response: unknown) => void,
  ): boolean => {
    const senderCheck = startSenderCheck(sender, deps.identity);
    if (senderCheck !== undefined) {
      deps.log.error(startRequestLabel, {
        accepted: false,
        check: senderCheck,
      });
      return false;
    }
    const parsed = parseStartRequest(message, deps.now());
    if (!parsed.ok) {
      deps.log.error(startRequestLabel, {
        accepted: false,
        check: parsed.check,
      });
      return false;
    }
    deps.log.info(startRequestLabel, { accepted: true });
    // Every step it awaits resolves by contract, and the request's values
    // passed the checks the URL encoding and the tab need, so it does not
    // reject.
    void runAcceptedStart(parsed.request, parsed.prepared, sendResponse);
    return true;
  };

  const onAwaitStart = (
    message: unknown,
    sender: MessageSender,
    sendResponse: (response: unknown) => void,
  ): boolean => {
    const tabId = viewerSenderTab(sender, deps.identity);
    const wait = parseAwaitStart(message);
    if (tabId === undefined || wait === undefined) {
      return false;
    }
    deps.log.info(awaitStartLabel, {
      record: answerWait({ tabId, sentAt: wait.sentAt, respond: sendResponse }),
    });
    return true;
  };

  return (message, sender, sendResponse) => {
    switch (declaredType(message)) {
      case "start":
        return onStartRequest(message, sender, sendResponse);
      case "await-start":
        return onAwaitStart(message, sender, sendResponse);
      default:
        return false;
    }
  };
}
