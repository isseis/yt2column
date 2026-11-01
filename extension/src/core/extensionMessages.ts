// The messages the extension's pages pass to each other, and their checks.
// The popup asks the service worker for a generation start, a viewer waits
// for that start's result, and the viewers answer queries for a key on a
// BroadcastChannel. A message is untrusted until its parse function accepts
// it: each checks the exact member set and every value's type and range, and
// rejects anything else instead of repairing it.

import { parseArticleKey } from "./articleKey.ts";
import type { ArticleKey } from "./articleKey.ts";
import { notSentCauses, unexpectedChecks } from "./exchange.ts";
import type { ExchangeFailure } from "./exchange.ts";
import { prepareStartInput } from "./prepare.ts";
import type { StartInput } from "./prepare.ts";
import { parseRequestId, startRejectStatus } from "./response.ts";
import type { RequestId, StartOutcome } from "./response.ts";
import { viewerSeedError } from "./viewerUrl.ts";
import type { ViewerSeed } from "./viewerUrl.ts";

/** Where the YouTube tab is, so the viewer opens next to it. */
export interface SourceTab {
  readonly tabId: number;
  readonly windowId: number;
  readonly index: number;
}

/** popup → service worker, chrome.runtime.sendMessage. */
export interface StartRequest {
  readonly type: "start";
  readonly input: StartInput;
  readonly viewer: ViewerSeed;
  readonly source: SourceTab;
}

/** service worker → popup, the sendResponse of a StartRequest. */
export interface StartReply {
  readonly outcome: StartOutcome;
  /** The tab that shows this send: the one the worker opened, or the one that took over. */
  readonly viewerTabId?: number;
}

/** viewer → service worker. The tab is the sender's; sentAt correlates the send. */
export interface AwaitStart {
  readonly type: "await-start";
  readonly sentAt: number;
}

/** service worker → viewer, the sendResponse of an AwaitStart. */
export type AwaitStartReply =
  | { readonly kind: "result"; readonly outcome: StartOutcome }
  // The worker holds no send for this tab and sentAt.
  | { readonly kind: "none" };

/** BroadcastChannel "yt2column-viewers". */
export type ViewerChannelMessage =
  | {
      readonly type: "find";
      readonly queryId: string;
      readonly key: ArticleKey;
    }
  | {
      readonly type: "present";
      readonly queryId: string;
      readonly tabId: number;
      readonly windowId: number;
      readonly sentAt: number;
    }
  | {
      readonly type: "waiting";
      readonly queryId: string;
      readonly tabId: number;
      readonly sentAt: number;
    }
  | {
      readonly type: "take-over";
      readonly queryId: string;
      readonly tabId: number;
      readonly sentAt: number;
    }
  | {
      readonly type: "took-over";
      readonly queryId: string;
      readonly tabId: number;
    };

export type PresentAnswer = Extract<ViewerChannelMessage, { type: "present" }>;

/** The viewer to use among those that answered present: the oldest sentAt, then the smallest tabId. */
export function chooseViewer(
  answers: readonly PresentAnswer[],
): PresentAnswer | undefined {
  let chosen: PresentAnswer | undefined;
  for (const answer of answers) {
    if (
      chosen === undefined ||
      answer.sentAt < chosen.sentAt ||
      (answer.sentAt === chosen.sentAt && answer.tabId < chosen.tabId)
    ) {
      chosen = answer;
    }
  }
  return chosen;
}

/** Thrown inside the checks only; each parse function turns it into undefined. */
class Rejected {}

type Members = Readonly<Record<string, unknown>>;

/** The value as a plain record; its members are checked by the caller. */
function asRecord(value: unknown): Members {
  if (typeof value !== "object" || value === null || Array.isArray(value)) {
    throw new Rejected();
  }
  return value as Members;
}

/** The value as a record with every required member and no others but the optional ones. */
function members(
  value: unknown,
  required: readonly string[],
  optional: readonly string[] = [],
): Members {
  const record = asRecord(value);
  const allowed = new Set([...required, ...optional]);
  if (
    !required.every((key) => Object.hasOwn(record, key)) ||
    !Object.keys(record).every((key) => allowed.has(key))
  ) {
    throw new Rejected();
  }
  return record;
}

function text(value: unknown): string {
  if (typeof value !== "string") {
    throw new Rejected();
  }
  return value;
}

/** A non-negative safe integer: a tab, window or index, or a send time. */
function count(value: unknown): number {
  if (typeof value !== "number" || !Number.isSafeInteger(value) || value < 0) {
    throw new Rejected();
  }
  return value;
}

function literal<T extends string>(value: unknown, expected: T): T {
  if (value !== expected) {
    throw new Rejected();
  }
  return expected;
}

function oneOf<T extends string>(value: unknown, set: Record<T, unknown>): T {
  if (typeof value !== "string" || !Object.hasOwn(set, value)) {
    throw new Rejected();
  }
  return value as T;
}

function articleKey(value: unknown): ArticleKey {
  const key = parseArticleKey(value);
  if (key === undefined) {
    throw new Rejected();
  }
  return key;
}

const queryIdPattern = /^[0-9a-f]{32}$/;

/** A query ID: 128 random bits as 32 lowercase hexadecimal characters. */
function queryId(value: unknown): string {
  const id = text(value);
  if (!queryIdPattern.test(id)) {
    throw new Rejected();
  }
  return id;
}

/**
 * Whether text holds no unpaired surrogate. Written by hand because
 * String.prototype.isWellFormed needs Chrome 111 and the minimum is 110.
 */
function isWellFormed(value: string): boolean {
  for (let index = 0; index < value.length; index += 1) {
    const unit = value.charCodeAt(index);
    if (unit >= 0xd800 && unit <= 0xdbff) {
      const next = value.charCodeAt(index + 1);
      if (!(next >= 0xdc00 && next <= 0xdfff)) {
        return false;
      }
      index += 1;
    } else if (unit >= 0xdc00 && unit <= 0xdfff) {
      return false;
    }
  }
  return true;
}

/** A well-formed string: a viewer value that becomes part of a URL. */
function wellFormed(value: unknown): string {
  const checked = text(value);
  if (!isWellFormed(checked)) {
    throw new Rejected();
  }
  return checked;
}

/** Runs check, turning a rejection or any other exception into undefined. */
function attempt<T>(check: () => T): T | undefined {
  try {
    return check();
  } catch {
    return undefined;
  }
}

const inputMetadataKeys = ["title", "channelName", "description"] as const;

/** The input in the request body's shape: two strings, and the metadata only when non-empty. */
function startInput(value: unknown): StartInput {
  const record = members(value, ["videoUrl", "transcript"], inputMetadataKeys);
  const input: { -readonly [K in keyof StartInput]: StartInput[K] } = {
    videoUrl: text(record.videoUrl),
    transcript: text(record.transcript),
  };
  for (const key of inputMetadataKeys) {
    if (Object.hasOwn(record, key)) {
      const metadata = text(record[key]);
      if (metadata === "") {
        throw new Rejected();
      }
      input[key] = metadata;
    }
  }
  return input;
}

/** A seed whose strings are well-formed and whose values pass the viewer URL's rules at now. */
function viewerSeed(value: unknown, now: number): ViewerSeed {
  const record = members(
    value,
    ["heading", "url", "sentAt"],
    ["channelName", "title"],
  );
  const seed: { -readonly [K in keyof ViewerSeed]: ViewerSeed[K] } = {
    heading: wellFormed(record.heading),
    url: wellFormed(record.url),
    sentAt: count(record.sentAt),
  };
  if (Object.hasOwn(record, "channelName")) {
    seed.channelName = wellFormed(record.channelName);
  }
  if (Object.hasOwn(record, "title")) {
    seed.title = wellFormed(record.title);
  }
  if (viewerSeedError(seed, now) !== undefined) {
    throw new Rejected();
  }
  return seed;
}

/**
 * Checks a start request received at now: the exact member set; an input in
 * the request body's shape that prepareStartInput measures within 4 MiB; a
 * viewer seed of well-formed strings that passes the viewer URL's rules with
 * its send time no later than now; and three non-negative integers for the
 * source tab. Returns undefined for anything else, including any exception
 * raised while checking.
 */
export function parseStartRequest(
  value: unknown,
  now: number,
): StartRequest | undefined {
  return attempt(() => {
    const record = members(value, ["type", "input", "viewer", "source"]);
    literal(record.type, "start");
    const input = startInput(record.input);
    if (prepareStartInput(input).kind !== "ready") {
      throw new Rejected();
    }
    const viewer = viewerSeed(record.viewer, now);
    const source = members(record.source, ["tabId", "windowId", "index"]);
    return {
      type: "start",
      input,
      viewer,
      source: {
        tabId: count(source.tabId),
        windowId: count(source.windowId),
        index: count(source.index),
      },
    };
  });
}

/** An exchange failure as displayed: its kind and, where it has one, its cause or check. */
function exchangeFailure(record: Members): ExchangeFailure {
  switch (record.kind) {
    case "not-sent":
      members(record, ["kind", "cause"]);
      return { kind: "not-sent", cause: oneOf(record.cause, notSentCauses) };
    case "no-response":
      members(record, ["kind"]);
      return { kind: "no-response" };
    case "host-failed":
      members(record, ["kind"]);
      return { kind: "host-failed" };
    case "unexpected":
      members(record, ["kind", "check"]);
      return {
        kind: "unexpected",
        check: oneOf(record.check, unexpectedChecks),
      };
    default:
      throw new Rejected();
  }
}

function requestId(value: unknown): RequestId {
  const id = parseRequestId(value);
  if (id === undefined) {
    throw new Rejected();
  }
  return id;
}

/** A generation start's displayed result, as another page passed it. */
function startOutcome(value: unknown): StartOutcome {
  const record = asRecord(value);
  switch (record.kind) {
    case "accepted":
      members(record, ["kind", "key", "requestId"]);
      return {
        kind: "accepted",
        key: articleKey(record.key),
        requestId: requestId(record.requestId),
      };
    case "rejected":
      members(record, ["kind", "reason", "requestId"]);
      return {
        kind: "rejected",
        reason: oneOf(record.reason, startRejectStatus),
        requestId: requestId(record.requestId),
      };
    default:
      return exchangeFailure(record);
  }
}

/** Checks the service worker's reply to a start request. */
export function parseStartReply(value: unknown): StartReply | undefined {
  return attempt(() => {
    const record = members(value, ["outcome"], ["viewerTabId"]);
    const outcome = startOutcome(record.outcome);
    if (!Object.hasOwn(record, "viewerTabId")) {
      return { outcome };
    }
    return { outcome, viewerTabId: count(record.viewerTabId) };
  });
}

/** Checks a viewer's wait for its start result. */
export function parseAwaitStart(value: unknown): AwaitStart | undefined {
  return attempt(() => {
    const record = members(value, ["type", "sentAt"]);
    return {
      type: literal(record.type, "await-start"),
      sentAt: count(record.sentAt),
    };
  });
}

/** Checks the service worker's reply to a viewer's wait. */
export function parseAwaitStartReply(
  value: unknown,
): AwaitStartReply | undefined {
  return attempt((): AwaitStartReply => {
    const record = asRecord(value);
    switch (record.kind) {
      case "result":
        members(record, ["kind", "outcome"]);
        return { kind: "result", outcome: startOutcome(record.outcome) };
      case "none":
        members(record, ["kind"]);
        return { kind: "none" };
      default:
        throw new Rejected();
    }
  });
}

/** Checks a message on the viewers' channel; the channel carries no sender, only same-origin pages can post. */
export function parseViewerChannelMessage(
  value: unknown,
): ViewerChannelMessage | undefined {
  return attempt((): ViewerChannelMessage => {
    const record = asRecord(value);
    switch (record.type) {
      case "find":
        members(record, ["type", "queryId", "key"]);
        return {
          type: "find",
          queryId: queryId(record.queryId),
          key: articleKey(record.key),
        };
      case "present":
        members(record, ["type", "queryId", "tabId", "windowId", "sentAt"]);
        return {
          type: "present",
          queryId: queryId(record.queryId),
          tabId: count(record.tabId),
          windowId: count(record.windowId),
          sentAt: count(record.sentAt),
        };
      case "waiting":
      case "take-over":
        members(record, ["type", "queryId", "tabId", "sentAt"]);
        return {
          type: record.type,
          queryId: queryId(record.queryId),
          tabId: count(record.tabId),
          sentAt: count(record.sentAt),
        };
      case "took-over":
        members(record, ["type", "queryId", "tabId"]);
        return {
          type: "took-over",
          queryId: queryId(record.queryId),
          tabId: count(record.tabId),
        };
      default:
        throw new Rejected();
    }
  });
}
