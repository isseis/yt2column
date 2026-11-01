// Sends one message to the native messaging host and classifies what came
// back. The request is sent exactly once and never retried: the server
// keeps working after a 202, so a resend could post the article twice. Each
// call starts the host for one message and one reply, so no connection stays
// open while the job runs. Whatever comes back, or does not, becomes a
// result value, never a thrown error.

import { classifyHostReply, classifyNativeReply } from "./core/native.ts";
import type { HostOutcome } from "./core/exchange.ts";
import type { HostMessage } from "./core/hostMessage.ts";
import type { NativeReply } from "./core/native.ts";
import type { PreparedRequest, StartInput } from "./core/prepare.ts";
import type { SendResult } from "./core/articlesResponse.ts";

export interface SendDeps {
  /**
   * Calls chrome.runtime.sendNativeMessage(nativeHostName, message) once.
   * Never rejects. StartInput is the popup's current /articles body; it is
   * dropped once the popup sends a HostMessage.
   */
  sendNative(message: HostMessage | StartInput): Promise<NativeReply>;
  /** Runs callback once after ms milliseconds; returns a function that cancels it. */
  schedule(ms: number, callback: () => void): () => void;
}

/** Upper bound for one exchange, from the call until the reply. */
export const sendTimeoutMs = 25_000;

/**
 * Calls deps.sendNative exactly once and classifies what came back, or
 * "timed-out" when sendTimeoutMs passes first; a reply that arrives after the
 * limit is ignored. Never throws and never retries: a classifier fault is
 * classificationError.
 */
async function sendOnce<T>(
  message: HostMessage | StartInput,
  deps: SendDeps,
  classify: (reply: NativeReply) => T,
  classificationError: T,
): Promise<T> {
  let cancel = (): void => {};
  const timedOut = new Promise<NativeReply>((resolve) => {
    cancel = deps.schedule(sendTimeoutMs, () => resolve({ kind: "timed-out" }));
  });
  // sendNative never rejects or throws by contract; a broken one is still a
  // host failure, not a send that hangs with the button disabled.
  const failed = (cause: unknown): NativeReply => ({
    kind: "failed",
    lastError: String(cause),
  });
  let sent: Promise<NativeReply>;
  try {
    sent = deps.sendNative(message).catch(failed);
  } catch (cause) {
    sent = Promise.resolve(failed(cause));
  }
  const reply = await Promise.race([sent, timedOut]);
  cancel();
  try {
    return classify(reply);
  } catch {
    return classificationError;
  }
}

/**
 * One exchange with the host: sends message once and returns the relayed
 * response or the failure, for the operation's check. The time limit is no
 * response. Never throws and never retries.
 */
export async function exchange(
  message: HostMessage,
  deps: SendDeps,
): Promise<HostOutcome> {
  return sendOnce(message, deps, classifyHostReply, {
    kind: "unexpected",
    check: "classification-error",
  });
}

/** The popup's /articles send, until it moves to exchange. */
export async function sendArticle(
  request: PreparedRequest,
  deps: SendDeps,
): Promise<SendResult> {
  return sendOnce(request.body.message, deps, classifyNativeReply, {
    kind: "unexpected",
    check: "classification-error",
  });
}
