// Interprets what one sendNativeMessage call delivered. The host's reply is
// untrusted: its member set is checked with Object.keys before any member is
// read, the server's response it wraps goes through classifyResponse, and no
// string from the reply or the browser's error is carried into a displayed
// value. A browser error is never told apart by its text: the text is a
// browser-internal constant, not an API contract.

import {
  classifyResponse,
  hasExactKeys,
  maxResponseBytes,
} from "./response.ts";
import type { SendResult } from "./response.ts";

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

/** True when value is one of the host's fixed error values. */
function isHostErrorValue(value: unknown): value is HostErrorValue {
  return (
    typeof value === "string" &&
    Object.prototype.hasOwnProperty.call(hostErrorValues, value)
  );
}

/** The send result each host error stands for. */
function hostErrorResult(error: HostErrorValue): SendResult {
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
 * shape and version, then its kind. A wrapped server response is checked by
 * classifyResponse. Never throws.
 */
export function classifyNativeReply(reply: NativeReply): SendResult {
  if (reply.kind === "timed-out") {
    return { kind: "unexpected", check: "extension-timeout" };
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
  if (record.kind === "error") {
    if (
      !hasExactKeys(record, ["v", "kind", "error"]) ||
      !isHostErrorValue(record.error)
    ) {
      return { kind: "unexpected", check: "envelope-shape" };
    }
    return hostErrorResult(record.error);
  }
  if (record.kind === "response") {
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
      return classifyResponse({ kind: "body-over-limit", status });
    }
    return classifyResponse({ kind: "body", status, bytes });
  }
  return { kind: "unexpected", check: "envelope-shape" };
}
