// Interprets the server's response to each job API operation: a generation
// start, a status query and a publish. The body is untrusted: it is parsed by
// parseStrictJson, and accepted only when its member set matches the
// documented shape exactly at every level, every code is in its closed set and
// the status code fits. No member is declared extensible, and a response that
// fails any check is unexpected as a whole: no part of its article, codes or
// numbers is used.

import { parseArticleKey } from "./articleKey.ts";
import type { ArticleKey } from "./articleKey.ts";
import type {
  ExchangeFailure,
  ExchangeFailureLog,
  HostOutcome,
  UnexpectedCheck,
} from "./exchange.ts";
import { parseStrictJson } from "./strictJson.ts";
import type { JsonValue } from "./strictJson.ts";

declare const requestIdBrand: unique symbol;

/** Sixteen lowercase hexadecimal characters. Only parseRequestId creates it. */
export type RequestId = string & { readonly [requestIdBrand]: true };

const requestIdPattern = /^[0-9a-f]{16}$/;

/** Returns the value unchanged when it has the accepted form; otherwise undefined. */
export function parseRequestId(value: unknown): RequestId | undefined {
  if (typeof value !== "string" || !requestIdPattern.test(value)) {
    return undefined;
  }
  return value as RequestId;
}

// The closed sets of the job API's codes. Each is declared with a
// Record<..., true>, so a value added to the type without its entry fails the
// type check, and a test compares the sets with the development guide.

export type JobState = "unknown" | "running" | "done" | "failed";

export const jobStates: Record<JobState, true> = {
  unknown: true,
  running: true,
  done: true,
  failed: true,
};

export type StepName = "draft" | "shorten" | "review";

export const stepNames: Record<StepName, true> = {
  draft: true,
  shorten: true,
  review: true,
};

export type StepState = "pending" | "running" | "done" | "skipped" | "failed";

export const stepStates: Record<StepState, true> = {
  pending: true,
  running: true,
  done: true,
  skipped: true,
  failed: true,
};

export type Steps = Readonly<Record<StepName, StepState>>;

export type FailureCode =
  | "internal"
  | "transcript_too_long"
  | "llm_timeout"
  | "llm_empty"
  | "llm_truncated"
  | "llm_failed"
  | "article_invalid"
  | "article_too_large";

export const failureCodes: Record<FailureCode, true> = {
  internal: true,
  transcript_too_long: true,
  llm_timeout: true,
  llm_empty: true,
  llm_truncated: true,
  llm_failed: true,
  article_invalid: true,
  article_too_large: true,
};

export type PublishState =
  "not_posted" | "posting" | "posted" | "failed" | "unpostable";

export const publishStates: Record<PublishState, true> = {
  not_posted: true,
  posting: true,
  posted: true,
  failed: true,
  unpostable: true,
};

type FailedPublishReason = "webhook" | "internal";
type UnpostableReason = "mention" | "other";
export type PublishReason = FailedPublishReason | UnpostableReason;

const failedPublishReasons: Record<FailedPublishReason, true> = {
  webhook: true,
  internal: true,
};

const unpostableReasons: Record<UnpostableReason, true> = {
  mention: true,
  other: true,
};

export const publishReasons: Record<PublishReason, true> = {
  ...failedPublishReasons,
  ...unpostableReasons,
};

export type PublishStatus =
  | { readonly state: "not_posted" | "posting" | "posted" }
  | { readonly state: "failed"; readonly reason: FailedPublishReason }
  | { readonly state: "unpostable"; readonly reason: UnpostableReason };

declare const usdBrand: unique symbol;

/** One to six digits, ".", four digits: the server's cost text. Only response.ts creates it. */
export type UsdAmount = `${number}` & { readonly [usdBrand]: true };

// Six integer digits cover any cost below one million dollars; a longer run
// of digits is no article's cost and is never handed to the formatter.
const usdPattern = /^[0-9]{1,6}\.[0-9]{4}$/;

export type Cost =
  | { readonly state: "known"; readonly usd: UsdAmount }
  | { readonly state: "unknown" };

export interface Article {
  /** The display Markdown, shown and copied unchanged. */
  readonly markdown: string;
  /** The file content, saved unchanged. */
  readonly file: string;
  readonly cost: Cost;
  readonly durationMs: number;
}

export type JobStatus =
  | { readonly state: "unknown" }
  | { readonly state: "running"; readonly steps: Steps }
  | {
      readonly state: "done";
      readonly steps: Steps;
      readonly article: Article;
      readonly publish: PublishStatus;
    }
  | {
      readonly state: "failed";
      readonly steps: Steps;
      readonly failure: FailureCode;
    };

// The reasons each operation's path can return, with the status code each is
// sent with. The host relays a status query and a publish without a body, so
// invalid_body is not in their sets; not_found and method_not_allowed answer a
// wrong path or method, which the extension never sends. Both are unexpected.

export type StartRejectReason =
  | "too_many_requests"
  | "body_too_large"
  | "invalid_body"
  | "invalid_video_url"
  | "invalid_transcript"
  | "internal_error";

export const startRejectStatus: Record<StartRejectReason, number> = {
  too_many_requests: 429,
  body_too_large: 413,
  invalid_body: 400,
  invalid_video_url: 400,
  invalid_transcript: 400,
  internal_error: 500,
};

export type StatusRejectReason = "invalid_key" | "internal_error";

export const statusRejectStatus: Record<StatusRejectReason, number> = {
  invalid_key: 400,
  internal_error: 500,
};

export type PublishRejectReason =
  | "invalid_key"
  | "webhook_not_configured"
  | "article_not_ready"
  | "article_not_found"
  | "article_not_postable"
  | "internal_error";

export const publishRejectStatus: Record<PublishRejectReason, number> = {
  invalid_key: 400,
  webhook_not_configured: 503,
  article_not_ready: 409,
  article_not_found: 404,
  article_not_postable: 409,
  internal_error: 500,
};

/** Every reason the extension shows with a fixed message; the catalog is keyed by it. */
export type ServerRejectReason =
  StartRejectReason | StatusRejectReason | PublishRejectReason;

/** A generation start's result as displayed. */
export type StartOutcome =
  | {
      readonly kind: "accepted";
      readonly key: ArticleKey;
      readonly requestId: RequestId;
    }
  | {
      readonly kind: "rejected";
      readonly reason: StartRejectReason;
      readonly requestId: RequestId;
    }
  | ExchangeFailure;

/** A status query's result as displayed. */
export type StatusOutcome =
  | {
      readonly kind: "ok";
      readonly requestId: RequestId;
      readonly webhookConfigured: boolean;
      readonly job: JobStatus;
    }
  | {
      readonly kind: "rejected";
      readonly reason: StatusRejectReason;
      readonly requestId: RequestId;
    }
  | ExchangeFailure;

/** A publish's result as displayed. */
export type PublishOutcome =
  | { readonly kind: "accepted"; readonly requestId: RequestId }
  | {
      readonly kind: "rejected";
      readonly reason: PublishRejectReason;
      readonly requestId: RequestId;
    }
  | ExchangeFailure;

// What the classify functions return: an exchange failure keeps its log-only
// members (the browser's error text, an unexpected reply's status).
export type StartOutcomeLog =
  Exclude<StartOutcome, ExchangeFailure> | ExchangeFailureLog;
export type StatusOutcomeLog =
  Exclude<StatusOutcome, ExchangeFailure> | ExchangeFailureLog;
export type PublishOutcomeLog =
  Exclude<PublishOutcome, ExchangeFailure> | ExchangeFailureLog;

/** Thrown inside the checks only; classify turns it into an unexpected result. */
class Rejected {
  readonly check: UnexpectedCheck;

  constructor(check: UnexpectedCheck) {
    this.check = check;
  }
}

type JsonObject = Extract<JsonValue, { kind: "object" }>;
type Members = JsonObject["members"];

/** True when value is an own key of set; a Record<..., true> or <..., number> holds only own keys. */
function isIn<K extends string>(
  set: Readonly<Record<K, unknown>>,
  value: string,
): value is K {
  return Object.prototype.hasOwnProperty.call(set, value);
}

/** The members of value, which must be an object with exactly names. */
function exactMembers(value: JsonValue, names: readonly string[]): Members {
  if (value.kind !== "object") {
    throw new Rejected("value-shape");
  }
  const members = value.members;
  if (
    members.size !== names.length ||
    !names.every((name) => members.has(name))
  ) {
    throw new Rejected("member-set");
  }
  return members;
}

/** The member a caller has already required by exactMembers. */
function member(members: Members, name: string): JsonValue {
  const value = members.get(name);
  if (value === undefined) {
    throw new Rejected("member-set");
  }
  return value;
}

/** The string value of a member. */
function stringMember(members: Members, name: string): string {
  const value = member(members, name);
  if (value.kind !== "string") {
    throw new Rejected("value-shape");
  }
  return value.value;
}

/** The value of a member, which must be a string in set. */
function codeMember<K extends string>(
  members: Members,
  name: string,
  set: Readonly<Record<K, true>>,
): K {
  const value = stringMember(members, name);
  if (!isIn(set, value)) {
    throw new Rejected("value-shape");
  }
  return value;
}

/** A non-empty string member: the article's text is never empty. */
function textMember(members: Members, name: string): string {
  const value = stringMember(members, name);
  if (value === "") {
    throw new Rejected("value-shape");
  }
  return value;
}

/** The string member name of an object whose member set depends on it, read before that set is checked. */
function discriminant(value: JsonValue, name: string): string {
  if (value.kind !== "object") {
    throw new Rejected("value-shape");
  }
  const found = value.members.get(name);
  if (found === undefined || found.kind !== "string") {
    throw new Rejected("member-set");
  }
  return found.value;
}

function requestIdMember(members: Members): RequestId {
  const requestId = parseRequestId(stringMember(members, "requestId"));
  if (requestId === undefined) {
    throw new Rejected("request-id");
  }
  return requestId;
}

/** The HTTP status must be the one the response's form is sent with. */
function requirePairing(status: number, expected: number): void {
  if (status !== expected) {
    throw new Rejected("status-pairing");
  }
}

/** A rejection whose reason is in reasons and whose status is the reason's. */
function rejection<R extends string>(
  value: JsonValue,
  status: number,
  reasons: Readonly<Record<R, number>>,
): {
  readonly kind: "rejected";
  readonly reason: R;
  readonly requestId: RequestId;
} {
  const members = exactMembers(value, ["status", "reason", "requestId"]);
  const reason = stringMember(members, "reason");
  const requestId = requestIdMember(members);
  if (!isIn(reasons, reason) || reasons[reason] !== status) {
    throw new Rejected("status-pairing");
  }
  return { kind: "rejected", reason, requestId };
}

function steps(value: JsonValue): Steps {
  const members = exactMembers(value, Object.keys(stepNames));
  return {
    draft: codeMember(members, "draft", stepStates),
    shorten: codeMember(members, "shorten", stepStates),
    review: codeMember(members, "review", stepStates),
  };
}

/** A done job's steps: every step finished or was not needed. */
function finishedSteps(value: JsonValue): Steps {
  const result = steps(value);
  for (const state of Object.values(result)) {
    if (state !== "done" && state !== "skipped") {
      throw new Rejected("value-shape");
    }
  }
  return result;
}

function cost(value: JsonValue): Cost {
  const state = discriminant(value, "state");
  switch (state) {
    case "known": {
      const usd = stringMember(exactMembers(value, ["state", "usd"]), "usd");
      if (!usdPattern.test(usd)) {
        throw new Rejected("value-shape");
      }
      return { state: "known", usd: usd as UsdAmount };
    }
    case "unknown":
      exactMembers(value, ["state"]);
      return { state: "unknown" };
    default:
      throw new Rejected("value-shape");
  }
}

// A decimal integer: no sign, fraction or exponent, and no leading zero.
const durationPattern = /^(0|[1-9][0-9]*)$/;

/** A JSON number written as a decimal integer no greater than 2^53-1. */
function duration(value: JsonValue): number {
  if (value.kind !== "number" || !durationPattern.test(value.text)) {
    throw new Rejected("value-shape");
  }
  // The text is checked first, so Number reads only digits; a longer run than
  // a safe integer converts to a larger value and is refused here.
  const ms = Number(value.text);
  if (ms > Number.MAX_SAFE_INTEGER) {
    throw new Rejected("value-shape");
  }
  return ms;
}

function article(value: JsonValue): Article {
  const members = exactMembers(value, [
    "markdown",
    "file",
    "cost",
    "durationMs",
  ]);
  return {
    markdown: textMember(members, "markdown"),
    file: textMember(members, "file"),
    cost: cost(member(members, "cost")),
    durationMs: duration(member(members, "durationMs")),
  };
}

function publishStatus(value: JsonValue): PublishStatus {
  const state = discriminant(value, "state");
  if (!isIn(publishStates, state)) {
    throw new Rejected("value-shape");
  }
  switch (state) {
    case "not_posted":
    case "posting":
    case "posted":
      exactMembers(value, ["state"]);
      return { state };
    case "failed":
      return {
        state,
        reason: codeMember(
          exactMembers(value, ["state", "reason"]),
          "reason",
          failedPublishReasons,
        ),
      };
    case "unpostable":
      return {
        state,
        reason: codeMember(
          exactMembers(value, ["state", "reason"]),
          "reason",
          unpostableReasons,
        ),
      };
    default: {
      const exhaustive: never = state;
      throw new Error(`Unhandled publish state: ${String(exhaustive)}`);
    }
  }
}

function job(value: JsonValue): JobStatus {
  const state = discriminant(value, "state");
  if (!isIn(jobStates, state)) {
    throw new Rejected("value-shape");
  }
  switch (state) {
    case "unknown":
      exactMembers(value, ["state"]);
      return { state };
    case "running": {
      const members = exactMembers(value, ["state", "steps"]);
      return { state, steps: steps(member(members, "steps")) };
    }
    case "done": {
      const members = exactMembers(value, [
        "state",
        "steps",
        "article",
        "publish",
      ]);
      return {
        state,
        steps: finishedSteps(member(members, "steps")),
        article: article(member(members, "article")),
        publish: publishStatus(member(members, "publish")),
      };
    }
    case "failed": {
      const members = exactMembers(value, ["state", "steps", "failure"]);
      return {
        state,
        steps: steps(member(members, "steps")),
        failure: codeMember(members, "failure", failureCodes),
      };
    }
    default: {
      const exhaustive: never = state;
      throw new Error(`Unhandled job state: ${String(exhaustive)}`);
    }
  }
}

/**
 * Runs check over the parsed body of a relayed response. An exchange failure
 * passes through unchanged; a body that is not strict JSON, or that check
 * refuses, is unexpected with the response's status. Never throws.
 */
function classify<T>(
  received: HostOutcome,
  check: (value: JsonValue, status: number) => T,
): T | ExchangeFailureLog {
  if (received.kind !== "response") {
    return received;
  }
  const status = received.status;
  const parsed = parseStrictJson(received.bytes);
  if (!parsed.ok) {
    return { kind: "unexpected", check: parsed.error, status };
  }
  try {
    return check(parsed.value, status);
  } catch (cause) {
    const failed =
      cause instanceof Rejected ? cause.check : "classification-error";
    return { kind: "unexpected", check: failed, status };
  }
}

/** Interprets a generation start's reply. Never throws. */
export function classifyStart(received: HostOutcome): StartOutcomeLog {
  return classify(received, (value, status): StartOutcomeLog => {
    switch (discriminant(value, "status")) {
      case "accepted": {
        const members = exactMembers(value, ["status", "key", "requestId"]);
        const key = parseArticleKey(stringMember(members, "key"));
        if (key === undefined) {
          throw new Rejected("value-shape");
        }
        const requestId = requestIdMember(members);
        requirePairing(status, 202);
        return { kind: "accepted", key, requestId };
      }
      case "rejected":
        return rejection(value, status, startRejectStatus);
      default:
        throw new Rejected("member-set");
    }
  });
}

/** Interprets a status query's reply. Never throws. */
export function classifyStatus(received: HostOutcome): StatusOutcomeLog {
  return classify(received, (value, status): StatusOutcomeLog => {
    switch (discriminant(value, "status")) {
      case "ok": {
        const members = exactMembers(value, [
          "status",
          "requestId",
          "webhookConfigured",
          "job",
        ]);
        const requestId = requestIdMember(members);
        const configured = member(members, "webhookConfigured");
        if (configured.kind !== "boolean") {
          throw new Rejected("value-shape");
        }
        const result = job(member(members, "job"));
        requirePairing(status, 200);
        return {
          kind: "ok",
          requestId,
          webhookConfigured: configured.value,
          job: result,
        };
      }
      case "rejected":
        return rejection(value, status, statusRejectStatus);
      default:
        throw new Rejected("member-set");
    }
  });
}

/** Interprets a publish's reply. Never throws. */
export function classifyPublish(received: HostOutcome): PublishOutcomeLog {
  return classify(received, (value, status): PublishOutcomeLog => {
    switch (discriminant(value, "status")) {
      case "accepted": {
        const members = exactMembers(value, ["status", "requestId"]);
        const requestId = requestIdMember(members);
        requirePairing(status, 202);
        return { kind: "accepted", requestId };
      }
      case "rejected":
        return rejection(value, status, publishRejectStatus);
      default:
        throw new Rejected("member-set");
    }
  });
}

/** Drops the log-only members of an exchange failure. */
function toExchangeFailure(log: ExchangeFailureLog): ExchangeFailure {
  switch (log.kind) {
    case "unexpected":
      return { kind: "unexpected", check: log.check };
    case "host-failed":
      return { kind: "host-failed" };
    case "not-sent":
      return { kind: "not-sent", cause: log.cause };
    case "no-response":
      return { kind: "no-response" };
    default: {
      const exhaustive: never = log;
      throw new Error(`Unhandled exchange failure: ${String(exhaustive)}`);
    }
  }
}

/** Drops the log-only members; the only way a StartOutcomeLog becomes a displayed value. */
export function toStartOutcome(log: StartOutcomeLog): StartOutcome {
  switch (log.kind) {
    case "accepted":
    case "rejected":
      return log;
    default:
      return toExchangeFailure(log);
  }
}

/** Drops the log-only members; the only way a StatusOutcomeLog becomes a displayed value. */
export function toStatusOutcome(log: StatusOutcomeLog): StatusOutcome {
  switch (log.kind) {
    case "ok":
    case "rejected":
      return log;
    default:
      return toExchangeFailure(log);
  }
}

/** Drops the log-only members; the only way a PublishOutcomeLog becomes a displayed value. */
export function toPublishOutcome(log: PublishOutcomeLog): PublishOutcome {
  switch (log.kind) {
    case "accepted":
    case "rejected":
      return log;
    default:
      return toExchangeFailure(log);
  }
}

// The values a developer log may hold for each result: kinds, checks, codes,
// status codes, request ids and the browser's error text. A view never holds
// a key, an article or any text from the response, so a log that takes only
// views cannot write them. Each view is built member by member, never spread,
// so a member added to a result later stays out until it is named here.

/** A generation start's result as logged: the key is left out. */
export type StartLogView =
  | { readonly kind: "accepted"; readonly requestId: RequestId }
  | {
      readonly kind: "rejected";
      readonly reason: StartRejectReason;
      readonly requestId: RequestId;
    }
  | ExchangeFailureLog;

/** A job as logged: its codes and whether its cost is known; the article is left out. */
export interface JobLogView {
  readonly state: JobState;
  readonly steps?: Steps;
  readonly publish?: PublishStatus;
  readonly failure?: FailureCode;
  readonly costKnown?: boolean;
}

/** A status query's result as logged. */
export type StatusLogView =
  | {
      readonly kind: "ok";
      readonly requestId: RequestId;
      readonly job: JobLogView;
    }
  | {
      readonly kind: "rejected";
      readonly reason: StatusRejectReason;
      readonly requestId: RequestId;
    }
  | ExchangeFailureLog;

/** A publish's result as logged. */
export type PublishLogView =
  | { readonly kind: "accepted"; readonly requestId: RequestId }
  | {
      readonly kind: "rejected";
      readonly reason: PublishRejectReason;
      readonly requestId: RequestId;
    }
  | ExchangeFailureLog;

/** A copy of an exchange failure with only the members it declares. */
function failureLogView(log: ExchangeFailureLog): ExchangeFailureLog {
  switch (log.kind) {
    case "not-sent":
      return { kind: "not-sent", cause: log.cause };
    case "no-response":
      return { kind: "no-response" };
    case "host-failed":
      return { kind: "host-failed", lastError: log.lastError };
    case "unexpected":
      return log.status === undefined
        ? { kind: "unexpected", check: log.check }
        : { kind: "unexpected", check: log.check, status: log.status };
    default: {
      const exhaustive: never = log;
      throw new Error(`Unhandled exchange failure: ${String(exhaustive)}`);
    }
  }
}

function stepsView(value: Steps): Steps {
  return { draft: value.draft, shorten: value.shorten, review: value.review };
}

function publishView(value: PublishStatus): PublishStatus {
  switch (value.state) {
    case "failed":
      return { state: value.state, reason: value.reason };
    case "unpostable":
      return { state: value.state, reason: value.reason };
    default:
      return { state: value.state };
  }
}

function jobLogView(value: JobStatus): JobLogView {
  switch (value.state) {
    case "unknown":
      return { state: value.state };
    case "running":
      return { state: value.state, steps: stepsView(value.steps) };
    case "done":
      return {
        state: value.state,
        steps: stepsView(value.steps),
        publish: publishView(value.publish),
        costKnown: value.article.cost.state === "known",
      };
    case "failed":
      return {
        state: value.state,
        steps: stepsView(value.steps),
        failure: value.failure,
      };
    default: {
      const exhaustive: never = value;
      throw new Error(`Unhandled job state: ${String(exhaustive)}`);
    }
  }
}

/** What a generation start's log may hold. */
export function startLogView(log: StartOutcomeLog): StartLogView {
  switch (log.kind) {
    case "accepted":
      return { kind: "accepted", requestId: log.requestId };
    case "rejected":
      return { kind: "rejected", reason: log.reason, requestId: log.requestId };
    default:
      return failureLogView(log);
  }
}

/** What a status query's log may hold. */
export function statusLogView(log: StatusOutcomeLog): StatusLogView {
  switch (log.kind) {
    case "ok":
      return { kind: "ok", requestId: log.requestId, job: jobLogView(log.job) };
    case "rejected":
      return { kind: "rejected", reason: log.reason, requestId: log.requestId };
    default:
      return failureLogView(log);
  }
}

/** What a publish's log may hold. */
export function publishLogView(log: PublishOutcomeLog): PublishLogView {
  switch (log.kind) {
    case "accepted":
      return { kind: "accepted", requestId: log.requestId };
    case "rejected":
      return { kind: "rejected", reason: log.reason, requestId: log.requestId };
    default:
      return failureLogView(log);
  }
}
