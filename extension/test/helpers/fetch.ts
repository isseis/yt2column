import type { FetchFn, FetchResponse, SendDeps } from "../../src/send.ts";

export interface FetchCall {
  readonly url: string;
  readonly init: RequestInit;
}

export interface FetchRecorder {
  readonly fetchFn: FetchFn;
  readonly calls: FetchCall[];
}

/** A FetchFn that records each call and answers with respond(call). */
export function recordingFetch(
  respond: (call: FetchCall) => Promise<FetchResponse>,
): FetchRecorder {
  const calls: FetchCall[] = [];
  return {
    fetchFn: async (url, init) => {
      const call = { url, init };
      calls.push(call);
      return respond(call);
    },
    calls,
  };
}

/** A FetchFn that rejects with cause on every call. */
export function rejectingFetch(cause: unknown): FetchRecorder {
  return recordingFetch(async () => {
    throw cause;
  });
}

/** A body stream that yields the chunks, then closes. */
export function streamOf(chunks: Uint8Array[]): ReadableStream<Uint8Array> {
  return new ReadableStream<Uint8Array>({
    start(controller) {
      for (const chunk of chunks) {
        controller.enqueue(chunk);
      }
      controller.close();
    },
  });
}

/** A response whose body is the UTF-8 encoding of text. */
export function textResponse(status: number, text: string): FetchResponse {
  return {
    type: "basic",
    status,
    body: streamOf([new TextEncoder().encode(text)]),
  };
}

/** An opaqueredirect response whose body throws if it is ever read. */
export function redirectResponse(): FetchResponse {
  const body = new ReadableStream<Uint8Array>({
    pull(controller) {
      controller.error(new Error("the redirect body must not be read"));
    },
  });
  return { type: "opaqueredirect", status: 0, body };
}

/** A body stream that yields the chunks, then fails with cause. */
export function failingStream(
  chunks: Uint8Array[],
  cause: unknown,
): ReadableStream<Uint8Array> {
  let index = 0;
  return new ReadableStream<Uint8Array>({
    pull(controller) {
      const chunk = chunks[index];
      if (chunk !== undefined) {
        controller.enqueue(chunk);
        index += 1;
        return;
      }
      controller.error(cause);
    },
  });
}

export interface TrackedStream {
  readonly body: ReadableStream<Uint8Array>;
  cancelled(): boolean;
}

/** An endless body stream that records whether it is cancelled. */
export function endlessStream(chunkSize: number): TrackedStream {
  let cancelled = false;
  const body = new ReadableStream<Uint8Array>({
    pull(controller) {
      controller.enqueue(new Uint8Array(chunkSize));
    },
    cancel() {
      cancelled = true;
    },
  });
  return { body, cancelled: () => cancelled };
}

export interface RejectingCancelStream {
  readonly body: ReadableStream<Uint8Array>;
  cancelCalls(): number;
}

/** An endless stream whose cancel() rejects, to test best-effort cancelling. */
export function endlessStreamWithRejectingCancel(
  chunkSize: number,
): RejectingCancelStream {
  let cancelCalls = 0;
  const body = new ReadableStream<Uint8Array>({
    pull(controller) {
      controller.enqueue(new Uint8Array(chunkSize));
    },
    cancel() {
      cancelCalls += 1;
      return Promise.reject(new Error("cancel failed"));
    },
  });
  return { body, cancelCalls: () => cancelCalls };
}

/**
 * SendDeps using fetchFn. The signal never aborts unless timeoutSignal is
 * given; tests pass one to record the requested timeout and the signal.
 */
export function testSendDeps(
  fetchFn: FetchFn,
  timeoutSignal?: (ms: number) => AbortSignal,
): SendDeps {
  return {
    fetch: fetchFn,
    timeoutSignal: timeoutSignal ?? (() => new AbortController().signal),
  };
}
