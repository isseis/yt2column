// Sends one message to the native messaging host and classifies what came
// back. Each message is sent exactly once and never retried: a generation
// start or a publish is sent again only when the user asks for it. Each call
// starts the host for one message and one reply, so no connection stays open
// while the job runs. Whatever comes back, or does not, becomes a result
// value, never a thrown error.

import { classifyHostReply } from "./core/native.ts";
import type { HostOutcome } from "./core/exchange.ts";
import type { HostMessage } from "./core/hostMessage.ts";
import type { NativeReply } from "./core/native.ts";

export interface SendDeps {
  /**
   * Calls chrome.runtime.sendNativeMessage(nativeHostName, message) once.
   * Never rejects. Only a HostMessage is sent: the job API's operations.
   */
  sendNative(message: HostMessage): Promise<NativeReply>;
  /** Runs callback once after ms milliseconds; returns a function that cancels it. */
  schedule(ms: number, callback: () => void): () => void;
}

/** Upper bound for one exchange, from the call until the reply. */
export const sendTimeoutMs = 25_000;

/**
 * One exchange with the host: calls deps.sendNative exactly once and returns
 * the relayed response or the failure, for the operation's check. The time
 * limit, sendTimeoutMs, is no response, and a reply that arrives after it is
 * ignored. Never throws and never retries: a classifier fault is an
 * unexpected reply.
 */
export async function exchange(
  message: HostMessage,
  deps: SendDeps,
): Promise<HostOutcome> {
  let cancel = (): void => {};
  const timedOut = new Promise<NativeReply>((resolve) => {
    cancel = deps.schedule(sendTimeoutMs, () => resolve({ kind: "timed-out" }));
  });
  // sendNative never rejects or throws by contract; a broken one is still a
  // host failure, not an exchange that never ends.
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
    return classifyHostReply(reply);
  } catch {
    return { kind: "unexpected", check: "classification-error" };
  }
}
