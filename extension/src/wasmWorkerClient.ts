// The page's side of the Worker that runs the article generation module: the
// running viewer and the options page call the module's functions through it.
// It sends one call at a time and matches the answer by callId
// (core/workerCalls.ts). Every way the Worker can fail ends the Worker: the
// first failure is the result of the waiting call, and of every later call,
// the Worker is terminated, and anything it sends afterwards is ignored, so a
// late error event from a Go timer left behind by an exit cannot turn into a
// second result.

import { parseWorkerMessage } from "./core/workerCalls.ts";
import type { WorkerFunction } from "./core/workerCalls.ts";

/** How long a new Worker has to load the module and say that it runs (ready). */
export const workerLoadTimeoutMs = 30_000;

/** The script the Worker runs, next to the pages in the build output. */
export const wasmWorkerScript = "wasmWorker.js";

/** The Worker surface used: a module Worker running wasmWorkerScript in production. */
export interface WorkerLike {
  postMessage(message: unknown): void;
  terminate(): void;
  addEventListener(
    type: "message",
    listener: (event: { readonly data: unknown }) => void,
  ): void;
  addEventListener(type: "error", listener: () => void): void;
}

/** Starts the Worker that runs the article generation module. */
export function createModuleWorker(): WorkerLike {
  return new Worker(wasmWorkerScript, { type: "module" });
}

/**
 * Why a call failed, as the internal failure's detail: the Worker did not load
 * the module and say ready in time, failed while loading, or ended before
 * ready (worker_load); it failed later (worker_error); the Go program ended while the
 * call waited (runtime_exit); the function's Promise rejected (panic).
 */
export type WorkerFailure =
  "worker_load" | "worker_error" | "runtime_exit" | "panic";

/**
 * A call's result. timeout: the call's own deadline passed. stopped:
 * terminate was called. Once the Worker has ended, every later call gets the
 * result it ended with, including a failure that came while no call waited.
 */
export type WorkerCallResult =
  | { readonly kind: "reply"; readonly value: string }
  | { readonly kind: "failed"; readonly detail: WorkerFailure }
  | { readonly kind: "timeout" }
  | { readonly kind: "stopped" };

export interface CallOptions {
  /** Receives each progress event of the call, while it waits. */
  readonly onProgress?: (event: string) => void;
  /** The call's own deadline; the Worker ends when it passes. */
  readonly timeoutMs?: number;
}

/** The module's functions, through one Worker. */
export interface WasmWorker {
  /**
   * Calls fn with request ("" for buildInfo) and resolves its result. Never
   * rejects. Throws when another call is waiting: a caller makes one call at a
   * time.
   */
  call(
    fn: WorkerFunction,
    request: string,
    options?: CallOptions,
  ): Promise<WorkerCallResult>;
  /** Ends the Worker; a waiting call resolves as stopped. */
  terminate(): void;
}

/** The call waiting for its answer. */
interface Waiting {
  readonly callId: number;
  readonly resolve: (result: WorkerCallResult) => void;
  readonly onProgress: ((event: string) => void) | undefined;
  readonly cancelTimeout: () => void;
}

/**
 * Starts a Worker with create and returns its calls. The load deadline starts
 * at once and ends when the Worker says the module runs; a Worker that cannot
 * be created ends at once as worker_load. Contract: each call resolves exactly
 * once; after the first failure, timeout or terminate, the Worker is
 * terminated once, every later message, error event and timer is ignored, and
 * every later call resolves with the result the Worker ended with.
 */
export function startWasmWorker(
  create: () => WorkerLike,
  schedule: (ms: number, callback: () => void) => () => void,
): WasmWorker {
  let worker: WorkerLike;
  try {
    worker = create();
  } catch {
    const failed: WorkerCallResult = { kind: "failed", detail: "worker_load" };
    return { call: () => Promise.resolve(failed), terminate: () => {} };
  }
  let ended: WorkerCallResult | undefined;
  let loaded = false;
  let nextCallId = 1;
  let waiting: Waiting | undefined;
  let cancelLoad: () => void = () => {};

  /** Ends the Worker and resolves the waiting call with result. */
  const end = (result: WorkerCallResult): void => {
    if (ended !== undefined) {
      return;
    }
    ended = result;
    cancelLoad();
    worker.terminate();
    const call = waiting;
    waiting = undefined;
    call?.cancelTimeout();
    call?.resolve(result);
  };
  const fail = (detail: WorkerFailure): void => end({ kind: "failed", detail });

  // ready cancels this timer, so it fires only before ready.
  cancelLoad = schedule(workerLoadTimeoutMs, () => fail("worker_load"));

  // After the end, end ignores every failure and no call waits, so nothing
  // the Worker still sends has an effect.
  worker.addEventListener("message", (event) => {
    const message = parseWorkerMessage(event.data);
    if (message === undefined) {
      // The Worker runs this repository's code; a message of another shape
      // means it is not the Worker this page expects.
      fail(loaded ? "worker_error" : "worker_load");
      return;
    }
    if (message.kind === "ready") {
      loaded = true;
      cancelLoad();
      return;
    }
    const call = waiting;
    if (call === undefined || message.callId !== call.callId) {
      return;
    }
    switch (message.kind) {
      case "progress":
        call.onProgress?.(message.event);
        return;
      case "reply":
        waiting = undefined;
        call.cancelTimeout();
        call.resolve({ kind: "reply", value: message.value });
        return;
      case "error":
        fail("panic");
        return;
      case "exit":
        fail(loaded ? "runtime_exit" : "worker_load");
        return;
    }
  });
  worker.addEventListener("error", () => {
    fail(loaded ? "worker_error" : "worker_load");
  });

  return {
    call: (fn, request, options = {}) => {
      if (ended !== undefined) {
        return Promise.resolve(ended);
      }
      if (waiting !== undefined) {
        throw new Error("a Worker call is already waiting");
      }
      const callId = nextCallId++;
      return new Promise((resolve) => {
        const { timeoutMs } = options;
        waiting = {
          callId,
          resolve,
          onProgress: options.onProgress,
          cancelTimeout:
            timeoutMs === undefined
              ? () => {}
              : schedule(timeoutMs, () => end({ kind: "timeout" })),
        };
        try {
          worker.postMessage({ callId, fn, request });
        } catch {
          fail(loaded ? "worker_error" : "worker_load");
        }
      });
    },
    terminate: () => end({ kind: "stopped" }),
  };
}
