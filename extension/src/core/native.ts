// Interprets what one sendNativeMessage call delivered. The host's reply is
// untrusted: its version is read as an own property, its kind picks the
// member set it must have exactly (checked with Object.keys), the server's
// response it wraps is returned as bytes for each operation's check, and no
// string from the reply or the browser's error is carried into a displayed
// value. A browser error is never told apart by its text: the text is a
// browser-internal constant, not an API contract.

import {
  classifyArticlesResponse,
  maxArticlesResponseBytes,
} from "./articlesResponse.ts";
import type {
  ArticlesUnexpectedCheck,
  SendResult,
} from "./articlesResponse.ts";
import { hasExactKeys, maxResponseBytes } from "./exchange.ts";
import type {
  ExchangeFailureLog,
  HostOutcome,
  UnexpectedCheck,
} from "./exchange.ts";

/** The host name the extension sends to; the same as the host's manifest "name". */
export const nativeHostName = "io.github.isseis.yt2column";

// The host reply's version and wire values. A Go guard reads these
// declarations and compares them with internal/nmhost, so keep each one in
// this form.
export const hostReplyVersion = 1;

/** The two shapes of a host reply. */
export type HostReplyKind = "response" | "error";

export const hostReplyKinds: Record<HostReplyKind, true> = {
  response: true,
  error: true,
};

/** The host's fixed errors. */
export type HostErrorValue =
  | "server_unavailable"
  | "server_timeout"
  | "protocol_error"
  | "message_too_large";

export const hostErrorValues: Record<HostErrorValue, true> = {
  server_unavailable: true,
  server_timeout: true,
  protocol_error: true,
  message_too_large: true,
};

/** What one sendNativeMessage call delivered, before interpretation. */
export type NativeReply =
  | { readonly kind: "message"; readonly value: unknown }
  | { readonly kind: "failed"; readonly lastError: string | undefined }
  | { readonly kind: "timed-out" };

/** True when value is one of the two reply kinds. */
function isHostReplyKind(value: unknown): value is HostReplyKind {
  return (
    typeof value === "string" &&
    Object.prototype.hasOwnProperty.call(hostReplyKinds, value)
  );
}

/** True when value is one of the host's fixed error values. */
function isHostErrorValue(value: unknown): value is HostErrorValue {
  return (
    typeof value === "string" &&
    Object.prototype.hasOwnProperty.call(hostErrorValues, value)
  );
}

/** The exchange result each host error stands for. */
function hostErrorResult(error: HostErrorValue): ExchangeFailureLog {
  switch (error) {
    case "server_unavailable":
      return { kind: "not-sent", cause: "server-unavailable" };
    case "message_too_large":
      return { kind: "not-sent", cause: "message-too-large" };
    case "server_timeout":
      return { kind: "no-response" };
    case "protocol_error":
      return { kind: "unexpected", check: "host-protocol-error" };
    default: {
      const exhaustive: never = error;
      throw new Error(`Unhandled host error: ${String(exhaustive)}`);
    }
  }
}

/**
 * Decodes standard base64 with padding, or returns undefined. Only the
 * canonical form is accepted: the decoded bytes must encode back to the same
 * text, so atob's leniency (whitespace, missing padding, non-zero trailing
 * bits) is rejected rather than repaired.
 */
function decodeBase64(text: string): Uint8Array | undefined {
  let binary: string;
  try {
    binary = atob(text);
  } catch {
    return undefined;
  }
  if (btoa(binary) !== text) {
    return undefined;
  }
  const bytes = new Uint8Array(binary.length);
  for (let i = 0; i < binary.length; i += 1) {
    bytes[i] = binary.charCodeAt(i);
  }
  return bytes;
}

/**
 * Interprets a reply in a fixed order: the browser's outcome, the reply's
 * shape and version, then its kind. A wrapped server response is returned as
 * its status and decoded bytes, at most maxResponseBytes, for the operation's
 * check to interpret. The extension's own time limit is no response: the
 * request may have reached the server, and asking again may succeed. Never
 * throws.
 */
export function classifyHostReply(reply: NativeReply): HostOutcome {
  if (reply.kind === "timed-out") {
    return { kind: "no-response" };
  }
  if (reply.kind === "failed") {
    return { kind: "host-failed", lastError: reply.lastError };
  }
  const value = reply.value;
  if (typeof value !== "object" || value === null || Array.isArray(value)) {
    return { kind: "unexpected", check: "envelope-shape" };
  }
  const record = value as Record<string, unknown>;
  if (
    !Object.prototype.hasOwnProperty.call(record, "v") ||
    record.v !== hostReplyVersion
  ) {
    return { kind: "unexpected", check: "envelope-version" };
  }
  const kind = record.kind;
  if (!isHostReplyKind(kind)) {
    return { kind: "unexpected", check: "envelope-shape" };
  }
  switch (kind) {
    case "error":
      if (
        !hasExactKeys(record, ["v", "kind", "error"]) ||
        !isHostErrorValue(record.error)
      ) {
        return { kind: "unexpected", check: "envelope-shape" };
      }
      return hostErrorResult(record.error);
    case "response": {
      const status = record.status;
      const body = record.body;
      if (
        !hasExactKeys(record, ["v", "kind", "status", "body"]) ||
        typeof status !== "number" ||
        !Number.isInteger(status) ||
        status < 100 ||
        status > 599 ||
        typeof body !== "string"
      ) {
        return { kind: "unexpected", check: "envelope-shape" };
      }
      const bytes = decodeBase64(body);
      if (bytes === undefined) {
        return { kind: "unexpected", check: "envelope-shape" };
      }
      if (bytes.byteLength > maxResponseBytes) {
        return { kind: "unexpected", check: "body-over-limit", status };
      }
      return { kind: "response", status, bytes };
    }
    default: {
      const exhaustive: never = kind;
      throw new Error(`Unhandled reply kind: ${String(exhaustive)}`);
    }
  }
}

/**
 * The /articles check of an outside check classifyHostReply reports. Only the
 * four outside checks reach here; the rest belong to the job API's body
 * checks and stand for a classifier fault.
 */
function articlesCheck(check: UnexpectedCheck): ArticlesUnexpectedCheck {
  switch (check) {
    case "envelope-shape":
    case "envelope-version":
    case "host-protocol-error":
    case "body-over-limit":
      return check;
    default:
      return "classification-error";
  }
}

/**
 * Interprets a reply for the popup's /articles path, until it moves to the job
 * API: classifyHostReply's result, with the wrapped body bounded by the
 * /articles limit and checked by classifyArticlesResponse. Never throws.
 */
export function classifyNativeReply(reply: NativeReply): SendResult {
  const outcome = classifyHostReply(reply);
  switch (outcome.kind) {
    case "response": {
      const { status, bytes } = outcome;
      if (bytes.byteLength > maxArticlesResponseBytes) {
        return classifyArticlesResponse({ kind: "body-over-limit", status });
      }
      return classifyArticlesResponse({ kind: "body", status, bytes });
    }
    case "unexpected": {
      const check = articlesCheck(outcome.check);
      return outcome.status === undefined
        ? { kind: "unexpected", check }
        : { kind: "unexpected", check, status: outcome.status };
    }
    default:
      return outcome;
  }
}
