// Sends one PreparedRequest and classifies what came back. The request is
// sent exactly once and never retried: the server keeps working after a 202,
// so a resend could post the article twice. Anything that is not a complete,
// accepted response becomes "unknown" (for the developer log) or an
// "unexpected" outcome, never a thrown error.

import { classifyResponse, maxResponseBytes } from "./core/response.ts";
import type { ReceivedResponse, SendResult } from "./core/response.ts";
import type { PreparedRequest } from "./core/prepare.ts";

/** Upper bound for the whole exchange, from the call until the response body is read. */
export const sendTimeoutMs = 25_000;

/** The parts of a fetch Response that sendArticle reads. */
export interface FetchResponse {
  readonly type: string;
  readonly status: number;
  readonly body: ReadableStream<Uint8Array> | null;
}

export type FetchFn = (
  url: string,
  init: RequestInit,
) => Promise<FetchResponse>;

export interface SendDeps {
  readonly fetch: FetchFn;
  /** Returns a signal that aborts after ms milliseconds (AbortSignal.timeout in production). */
  timeoutSignal(ms: number): AbortSignal;
}

/** The result of reading a response body up to the limit. */
type ReadBodyResult =
  | { readonly kind: "body"; readonly bytes: Uint8Array }
  | { readonly kind: "over-limit" }
  | { readonly kind: "failed"; readonly cause: unknown };

/** Concatenates the read chunks into one byte array of the counted length. */
function concat(chunks: Uint8Array[], total: number): Uint8Array {
  const bytes = new Uint8Array(total);
  let offset = 0;
  for (const chunk of chunks) {
    bytes.set(chunk, offset);
    offset += chunk.byteLength;
  }
  return bytes;
}

/**
 * Reads the body until it ends or exceeds the limit. On the first chunk that
 * pushes the total over the limit it cancels the reader and stops: a process
 * squatting the port could otherwise stream an unbounded body.
 */
async function readBody(
  body: ReadableStream<Uint8Array> | null,
): Promise<ReadBodyResult> {
  if (body === null) {
    return { kind: "body", bytes: new Uint8Array(0) };
  }
  const reader = body.getReader();
  const chunks: Uint8Array[] = [];
  let total = 0;
  let overLimit = false;
  try {
    for (;;) {
      const { done, value } = await reader.read();
      if (done) {
        break;
      }
      total += value.byteLength;
      if (total > maxResponseBytes) {
        overLimit = true;
        break;
      }
      chunks.push(value);
    }
  } catch (cause) {
    return { kind: "failed", cause };
  }
  if (overLimit) {
    // Cancelling is best effort: an over-limit body is over-limit even if the
    // source's cancel rejects, and must not be reclassified as unknown.
    try {
      await reader.cancel();
    } catch {
      // ignore
    }
    return { kind: "over-limit" };
  }
  return { kind: "body", bytes: concat(chunks, total) };
}

/**
 * Calls deps.fetch exactly once and classifies what came back. Never throws
 * and never retries. Returns { kind: "unknown", cause } when no complete
 * response arrived.
 */
export async function sendArticle(
  request: PreparedRequest,
  deps: SendDeps,
): Promise<SendResult> {
  let response: FetchResponse;
  try {
    response = await deps.fetch(request.url, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        Authorization: `Bearer ${request.token}`,
      },
      body: request.body.json,
      redirect: "manual",
      credentials: "omit",
      cache: "no-store",
      signal: deps.timeoutSignal(sendTimeoutMs),
    });
  } catch (cause) {
    return { kind: "unknown", cause };
  }
  let received: ReceivedResponse;
  if (response.type === "opaqueredirect") {
    received = { kind: "redirected" };
  } else {
    const read = await readBody(response.body);
    if (read.kind === "failed") {
      return { kind: "unknown", cause: read.cause };
    }
    received =
      read.kind === "over-limit"
        ? { kind: "body-over-limit", status: response.status }
        : { kind: "body", status: response.status, bytes: read.bytes };
  }
  try {
    return classifyResponse(received);
  } catch {
    return {
      kind: "unexpected",
      status: response.status,
      check: "classification-error",
    };
  }
}
