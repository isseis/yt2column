// Interprets a server response to POST /articles, the path the popup uses
// until it moves to the job API. The response body is untrusted: the member
// set is checked with Object.keys, so the fixed names read below are own
// properties; no parsed method is called, no inherited member is read, and the
// raw body text is never carried out of this module.

import { hasExactKeys } from "./exchange.ts";
import type { NotSentCause } from "./exchange.ts";
import { parseRequestId } from "./response.ts";
import type { RequestId } from "./response.ts";

/**
 * The reason values the extension shows with a fixed message. The server also
 * answers not_found and method_not_allowed to a wrong path or method; the host
 * only ever sends POST /articles, so either one fails the status pairing and
 * is unexpected.
 */
export type ArticlesRejectReason =
  | "too_many_requests"
  | "body_too_large"
  | "invalid_body"
  | "invalid_video_url"
  | "invalid_transcript"
  | "internal_error";

/** The status code each reason is sent with; a Record, so a missing reason fails the type check. */
export const articlesRejectStatus: Record<ArticlesRejectReason, number> = {
  too_many_requests: 429,
  body_too_large: 413,
  invalid_body: 400,
  invalid_video_url: 400,
  invalid_transcript: 400,
  internal_error: 500,
};

/** True when value is one of the declared server reject reasons. */
export function isArticlesRejectReason(
  value: unknown,
): value is ArticlesRejectReason {
  return (
    typeof value === "string" &&
    Object.prototype.hasOwnProperty.call(articlesRejectStatus, value)
  );
}

/** Upper bound for the response body; a valid response is under 100 bytes. */
export const maxArticlesResponseBytes = 1_024;

/** The server response the host relayed, before interpretation. */
export type ArticlesReceivedResponse =
  | {
      readonly kind: "body";
      readonly status: number;
      readonly bytes: Uint8Array;
    }
  | { readonly kind: "body-over-limit"; readonly status: number };

/** Which check an unexpected reply failed; for the developer log only. */
export type ArticlesUnexpectedCheck =
  | "envelope-shape"
  | "envelope-version"
  | "host-protocol-error"
  | "body-over-limit"
  | "encoding"
  | "escape"
  | "json-shape"
  | "duplicate-member"
  | "member-set"
  | "request-id"
  | "status-pairing"
  | "classification-error";

/**
 * A send result as displayed. "not-sent" is known not to have reached the
 * server; "no-response", "host-failed" and "unexpected" may have reached it.
 */
export type SendOutcome =
  | { readonly kind: "accepted"; readonly requestId: RequestId }
  | {
      readonly kind: "rejected";
      readonly reason: ArticlesRejectReason;
      readonly requestId: RequestId;
    }
  | { readonly kind: "not-sent"; readonly cause: NotSentCause }
  | { readonly kind: "no-response" }
  // Any browser error: the host did not start, or exited without a reply.
  | { readonly kind: "host-failed" }
  | { readonly kind: "unexpected"; readonly check: ArticlesUnexpectedCheck };

/**
 * What sendArticle returns. "unexpected" also carries the server's status when
 * a response was relayed, and "host-failed" the browser's error text, for the
 * developer log only: the text is never compared, displayed or stored.
 */
export type SendResult =
  | Exclude<SendOutcome, { kind: "unexpected" | "host-failed" }>
  | {
      readonly kind: "unexpected";
      readonly check: ArticlesUnexpectedCheck;
      readonly status?: number;
    }
  | { readonly kind: "host-failed"; readonly lastError: string | undefined };

/** Drops the log-only members; the only way a SendResult becomes a displayed value. */
export function toSendOutcome(result: SendResult): SendOutcome {
  switch (result.kind) {
    case "unexpected":
      return { kind: "unexpected", check: result.check };
    case "host-failed":
      return { kind: "host-failed" };
    default:
      return result;
  }
}

/** The three results a server response can classify as. */
export type ArticlesResponseResult = Extract<
  SendResult,
  { kind: "accepted" | "rejected" | "unexpected" }
>;

/** An unexpected response at a status: the status and the failed check, no body text. */
function unexpected(
  status: number,
  check: ArticlesUnexpectedCheck,
): ArticlesResponseResult {
  return { kind: "unexpected", status, check };
}

/**
 * Interprets a received response in the fixed order. Never throws. The parsed
 * members are read as own properties only, so an attacker-chosen member name
 * cannot run code or read an inherited value.
 */
export function classifyArticlesResponse(
  received: ArticlesReceivedResponse,
): ArticlesResponseResult {
  const status = received.status;
  if (received.kind === "body-over-limit") {
    return unexpected(status, "body-over-limit");
  }
  let text: string;
  try {
    text = new TextDecoder("utf-8", {
      fatal: true,
      ignoreBOM: true,
    }).decode(received.bytes);
  } catch {
    return unexpected(status, "encoding");
  }
  // The accepted members and values are ASCII, so a backslash is never
  // legitimate; rejecting it also rules out the escape forms.
  if (text.includes("\\")) {
    return unexpected(status, "escape");
  }
  let parsed: unknown;
  try {
    parsed = JSON.parse(text);
  } catch {
    return unexpected(status, "json-shape");
  }
  if (typeof parsed !== "object" || parsed === null || Array.isArray(parsed)) {
    return unexpected(status, "json-shape");
  }
  const record = parsed as Record<string, unknown>;
  const keys = Object.keys(record);
  if (!Object.values(record).every((value) => typeof value === "string")) {
    return unexpected(status, "json-shape");
  }
  // Every quote is structural, so a duplicate member adds two quotes per
  // dropped name and value: the count then exceeds four per surviving member.
  const quotes = (text.match(/"/g) ?? []).length;
  if (quotes !== keys.length * 4) {
    return unexpected(status, "duplicate-member");
  }
  if (record.status === "accepted") {
    if (!hasExactKeys(record, ["status", "requestId"])) {
      return unexpected(status, "member-set");
    }
  } else if (record.status === "rejected") {
    if (!hasExactKeys(record, ["status", "reason", "requestId"])) {
      return unexpected(status, "member-set");
    }
  } else {
    return unexpected(status, "member-set");
  }
  const requestId = parseRequestId(record.requestId);
  if (requestId === undefined) {
    return unexpected(status, "request-id");
  }
  if (record.status === "accepted") {
    if (status !== 202) {
      return unexpected(status, "status-pairing");
    }
    return { kind: "accepted", requestId };
  }
  const reason = record.reason;
  if (
    !isArticlesRejectReason(reason) ||
    articlesRejectStatus[reason] !== status
  ) {
    return unexpected(status, "status-pairing");
  }
  return { kind: "rejected", reason, requestId };
}
