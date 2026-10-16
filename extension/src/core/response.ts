// Interprets a received response as a trusted value. The response body is
// untrusted: the member set is checked with Object.keys, so the fixed names
// read below are own properties; no parsed method is called, no inherited
// member is read, and the raw body text is never carried out of this module.

import { hasExactKeys } from "./summary.ts";

/** The reason values the server sends. */
export type ServerRejectReason =
  | "forbidden_host"
  | "not_found"
  | "method_not_allowed"
  | "unsupported_media_type"
  | "forbidden_origin"
  | "unauthorized"
  | "too_many_requests"
  | "body_too_large"
  | "invalid_body"
  | "invalid_video_url"
  | "invalid_transcript"
  | "internal_error";

/** The status code each reason is sent with; a Record, so a missing reason fails the type check. */
export const serverRejectStatus: Record<ServerRejectReason, number> = {
  forbidden_host: 403,
  not_found: 404,
  method_not_allowed: 405,
  unsupported_media_type: 415,
  forbidden_origin: 403,
  unauthorized: 401,
  too_many_requests: 429,
  body_too_large: 413,
  invalid_body: 400,
  invalid_video_url: 400,
  invalid_transcript: 400,
  internal_error: 500,
};

/** True when value is one of the declared server reject reasons. */
export function isServerRejectReason(
  value: unknown,
): value is ServerRejectReason {
  return (
    typeof value === "string" &&
    Object.prototype.hasOwnProperty.call(serverRejectStatus, value)
  );
}

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

/** Upper bound for the response body; a valid response is under 100 bytes. */
export const maxResponseBytes = 1_024;

/** What sendArticle received, before interpretation. */
export type ReceivedResponse =
  | {
      readonly kind: "body";
      readonly status: number;
      readonly bytes: Uint8Array;
    }
  | { readonly kind: "body-over-limit"; readonly status: number }
  | { readonly kind: "redirected" };

/** Which check an unexpected response failed; for the developer log only. */
export type UnexpectedCheck =
  | "body-over-limit"
  | "encoding"
  | "escape"
  | "json-shape"
  | "duplicate-member"
  | "member-set"
  | "request-id"
  | "status-pairing"
  | "classification-error";

/** Runtime set of the unexpected checks; a Record, so it stays in step with the union. */
export const unexpectedChecks: Record<UnexpectedCheck, true> = {
  "body-over-limit": true,
  encoding: true,
  escape: true,
  "json-shape": true,
  "duplicate-member": true,
  "member-set": true,
  "request-id": true,
  "status-pairing": true,
  "classification-error": true,
};

/** A send result without the cause of "unknown": what is displayed and stored. */
export type SendOutcome =
  | { readonly kind: "accepted"; readonly requestId: RequestId }
  | {
      readonly kind: "rejected";
      readonly reason: ServerRejectReason;
      readonly requestId: RequestId;
    }
  | { readonly kind: "unknown" }
  | {
      readonly kind: "unexpected";
      readonly status: number;
      readonly check: UnexpectedCheck;
    }
  | { readonly kind: "redirected" };

/** What sendArticle returns: "unknown" also carries the failure, for the developer log. */
export type SendResult =
  | Exclude<SendOutcome, { kind: "unknown" }>
  | { readonly kind: "unknown"; readonly cause: unknown };

/** Drops the cause of "unknown"; the only way a SendResult becomes a stored or displayed value. */
export function toSendOutcome(result: SendResult): SendOutcome {
  if (result.kind === "unknown") {
    return { kind: "unknown" };
  }
  return result;
}

/** An unexpected response at a status: the status and the failed check, no body text. */
function unexpected(
  status: number,
  check: UnexpectedCheck,
): Extract<SendOutcome, { kind: "unexpected" }> {
  return { kind: "unexpected", status, check };
}

/**
 * Interprets a received response in the fixed order. Never
 * returns "unknown" and never throws. The parsed members are read as own
 * properties only, so an attacker-chosen member name cannot run code or read
 * an inherited value.
 */
export function classifyResponse(
  received: ReceivedResponse,
): Exclude<SendOutcome, { kind: "unknown" }> {
  if (received.kind === "redirected") {
    return { kind: "redirected" };
  }
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
  if (!isServerRejectReason(reason) || serverRejectStatus[reason] !== status) {
    return unexpected(status, "status-pairing");
  }
  return { kind: "rejected", reason, requestId };
}
