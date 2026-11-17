// A fake of the Worker that runs the article generation module, in the shape
// of wasmWorkerClient.ts's WorkerLike. The test plays the Worker: it reads
// what the page posted and decides what the Worker answers and when.

import type { WorkerLike } from "../../src/wasmWorkerClient.ts";

export interface FakeWorker extends WorkerLike {
  /** Every message the page posted, in order. */
  readonly posted: unknown[];
  /** How many times the page called terminate. */
  readonly terminations: number;
  /**
   * Delivers data to the page's message listeners. It delivers even after
   * terminate: a message the Worker posted before it was stopped can still
   * be on its way, and the page must ignore it.
   */
  send(data: unknown): void;
  /** Fires the page's error listeners, as an uncaught error in the Worker does. */
  fail(): void;
}

/** Workers created by the code under test, in order, through create. */
export interface FakeWorkers {
  readonly create: () => WorkerLike;
  readonly created: FakeWorker[];
}

export function fakeWorkers(): FakeWorkers {
  const created: FakeWorker[] = [];
  return {
    created,
    create: () => {
      const messageListeners: ((event: { readonly data: unknown }) => void)[] =
        [];
      const errorListeners: (() => void)[] = [];
      const posted: unknown[] = [];
      let terminations = 0;
      const worker: FakeWorker = {
        posted,
        get terminations() {
          return terminations;
        },
        postMessage: (message) => {
          posted.push(structuredClone(message));
        },
        terminate: () => {
          terminations += 1;
        },
        addEventListener: (
          type: "message" | "error",
          listener:
            ((event: { readonly data: unknown }) => void) | (() => void),
        ) => {
          if (type === "message") {
            messageListeners.push(
              listener as (event: { readonly data: unknown }) => void,
            );
          } else {
            errorListeners.push(listener as () => void);
          }
        },
        send: (data) => {
          for (const listener of messageListeners) {
            listener({ data: structuredClone(data) });
          }
        },
        fail: () => {
          for (const listener of errorListeners) {
            listener();
          }
        },
      };
      created.push(worker);
      return worker;
    },
  };
}
