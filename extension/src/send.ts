// Sends one PreparedRequest to the native messaging host and classifies what
// came back. The request is sent exactly once and never retried: the server
// keeps working after a 202, so a resend could post the article twice. Each
// call starts the host for one message and one reply, so no connection stays
// open while the job runs. Anything that is not a complete, accepted reply
// becomes a displayed outcome, never a thrown error.

import { classifyNativeReply } from "./core/native.ts";
import type { NativeReply } from "./core/native.ts";
import type { PreparedRequest } from "./core/prepare.ts";
import type { SendResult } from "./core/response.ts";

export interface SendDeps {
  /** Calls chrome.runtime.sendNativeMessage(nativeHostName, message) once. Never rejects. */
  sendNative(message: Readonly<Record<string, string>>): Promise<NativeReply>;
  /** Runs callback once after ms milliseconds; returns a function that cancels it. */
  schedule(ms: number, callback: () => void): () => void;
}

/** Upper bound for one exchange, from the call until the reply. */
export const sendTimeoutMs = 25_000;

/**
 * Calls deps.sendNative exactly once and classifies what came back, or
 * "timed-out" when sendTimeoutMs passes first; a reply that arrives after the
 * limit is ignored. Never throws and never retries.
 */
export async function sendArticle(
  request: PreparedRequest,
  deps: SendDeps,
): Promise<SendResult> {
  let cancel = (): void => {};
  const timedOut = new Promise<NativeReply>((resolve) => {
    cancel = deps.schedule(sendTimeoutMs, () => resolve({ kind: "timed-out" }));
  });
  const sent = deps
    .sendNative(request.body.message)
    .catch((cause: unknown): NativeReply => ({
      kind: "failed",
      lastError: String(cause),
    }));
  const reply = await Promise.race([sent, timedOut]);
  cancel();
  try {
    return classifyNativeReply(reply);
  } catch {
    return { kind: "unexpected", check: "classification-error" };
  }
}
