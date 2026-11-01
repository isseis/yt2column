import type { HostMessage } from "../../src/core/hostMessage.ts";
import type { NativeReply } from "../../src/core/native.ts";
import type { StartInput } from "../../src/core/prepare.ts";
import type { SendDeps } from "../../src/send.ts";

type Message = HostMessage | StartInput;

export interface NativeRecorder {
  readonly sendNative: SendDeps["sendNative"];
  readonly messages: Message[];
}

/** A sendNative that records each message and answers with respond(message). */
export function recordingNative(
  respond: (message: Message) => Promise<NativeReply>,
): NativeRecorder {
  const messages: Message[] = [];
  return {
    sendNative: async (message) => {
      messages.push(message);
      return respond(message);
    },
    messages,
  };
}

/** The host reply wrapping a server response of status with the given body bytes. */
export function bytesReply(status: number, bytes: Uint8Array): NativeReply {
  return {
    kind: "message",
    value: {
      v: 1,
      kind: "response",
      status,
      body: Buffer.from(bytes).toString("base64"),
    },
  };
}

/** The host reply wrapping a server response of status whose body is the UTF-8 of text. */
export function responseReply(status: number, text: string): NativeReply {
  return bytesReply(status, new TextEncoder().encode(text));
}

/** The host reply carrying one of the host's errors. */
export function errorReply(error: string): NativeReply {
  return { kind: "message", value: { v: 1, kind: "error", error } };
}

/** A sendNative whose reply resolves only when the test calls resolve. */
export function deferredNative(): NativeRecorder & {
  resolve(reply: NativeReply): void;
} {
  let settle: ((reply: NativeReply) => void) | undefined;
  const pending = new Promise<NativeReply>((resolve) => {
    settle = resolve;
  });
  return {
    ...recordingNative(async () => pending),
    resolve: (reply) => settle?.(reply),
  };
}

export interface ManualSchedule {
  readonly schedule: SendDeps["schedule"];
  /** The delay of each schedule call, in order. */
  readonly delays: number[];
  /** Runs every callback that was scheduled and not cancelled. */
  fire(): void;
  /** How many scheduled callbacks were cancelled. */
  cancelled(): number;
}

/** A schedule that never runs a callback by itself; the test fires it. */
export function manualSchedule(): ManualSchedule {
  const delays: number[] = [];
  const pending = new Set<() => void>();
  let cancelled = 0;
  return {
    schedule: (ms, callback) => {
      delays.push(ms);
      pending.add(callback);
      return () => {
        if (pending.delete(callback)) {
          cancelled += 1;
        }
      };
    },
    delays,
    fire: () => {
      for (const callback of [...pending]) {
        pending.delete(callback);
        callback();
      }
    },
    cancelled: () => cancelled,
  };
}

/** SendDeps using sendNative; the time limit never passes unless schedule is given. */
export function testSendDeps(
  sendNative: SendDeps["sendNative"],
  schedule: SendDeps["schedule"] = manualSchedule().schedule,
): SendDeps {
  return { sendNative, schedule };
}
