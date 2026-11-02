// Runs the status polling of one page, the viewer or the popup. A page sends
// at most one status query at a time and holds at most one timer for the
// next: whatever starts a fetch cancels that timer, and only a reply the
// page decides to continue after sets a new one, so repeated triggers never
// fork the chain. Whether to continue is afterStatus's decision; the poller
// only adds the epochs: a reply to a fetch sent before a take-over or a
// publish may update the display but never stops polling, and a fetch of the
// new epoch follows it at once.

import type { ArticleKey } from "./core/articleKey.ts";
import { statusMessage } from "./core/hostMessage.ts";
import { afterStatus, pollIntervalMs } from "./core/poll.ts";
import type { PollStep } from "./core/poll.ts";
import type { JobDisplay } from "./core/jobView.ts";
import {
  classifyStatus,
  isExchangeFailureView,
  statusLogView,
  toStatusOutcome,
} from "./core/response.ts";
import type { StatusLogView, StatusOutcome } from "./core/response.ts";
import type { Logger } from "./core/types.ts";
import { exchange } from "./send.ts";
import type { SendDeps } from "./send.ts";

export interface PollerDeps {
  /** Runs one status exchange for the page's key. Never rejects. */
  fetchStatus(): Promise<StatusOutcome>;
  /** Runs callback once after ms milliseconds; returns a function that cancels it. */
  schedule(ms: number, callback: () => void): () => void;
}

/** Where a step's reply came from, for the page's log and its own waits. */
export interface StepInfo {
  /** The reply answers a fetch sent before the latest start or bumpEpoch. */
  readonly earlierEpoch: boolean;
  /** The fetch was sent by fetchNow: the page became visible. */
  readonly visible: boolean;
}

export interface Poller {
  /** Starts or restarts polling in a new epoch; fetches at once. */
  start(): void;
  /** Fetches at once when not in flight and not stopped; used on visibilitychange. */
  fetchNow(): void;
  /** Starts a new epoch: while a fetch is in flight, its reply cannot stop polling and a new fetch follows at once; when stopped, starts polling. Used after a take-over and a publish. */
  bumpEpoch(): void;
  stop(): void;
}

/**
 * Polls with deps and hands each reply's step to onStep. Before start the
 * poller is stopped. The display afterStatus is given as previous is the last
 * job a step showed since start; start clears it, since a restart shows
 * loading. A reply that arrives after stop is dropped, and a step of an
 * earlier epoch is handed on with the decision the poller took, continue.
 * The epochs are the poller's alone: info says whether a reply is of an
 * earlier one, so a page never keeps its own count of them.
 */
export function createPoller(
  deps: PollerDeps,
  onStep: (step: PollStep, info: StepInfo) => void,
): Poller {
  let epoch = 0;
  let inFlight = false;
  let stopped = true;
  let previous: JobDisplay | undefined;
  let cancelTimer = (): void => {};

  const fetch = (visible = false): void => {
    cancelTimer();
    cancelTimer = () => {};
    inFlight = true;
    const sentEpoch = epoch;
    void deps.fetchStatus().then((outcome) => {
      inFlight = false;
      receive(sentEpoch, visible, outcome);
    });
  };

  const receive = (
    sentEpoch: number,
    visible: boolean,
    outcome: StatusOutcome,
  ): void => {
    if (stopped) {
      return;
    }
    const step = afterStatus(previous, outcome);
    if (step.job !== undefined) {
      previous = step.job;
    }
    // Decide before onStep, so a page that reacts to the step by calling the
    // poller finds the next fetch already in flight or scheduled.
    if (sentEpoch !== epoch) {
      fetch();
      onStep(
        { ...step, decision: "continue" },
        { earlierEpoch: true, visible },
      );
      return;
    }
    if (step.decision === "continue") {
      cancelTimer = deps.schedule(pollIntervalMs, () => fetch());
    } else {
      stopped = true;
    }
    onStep(step, { earlierEpoch: false, visible });
  };

  return {
    start: () => {
      epoch += 1;
      stopped = false;
      previous = undefined;
      if (!inFlight) {
        fetch();
      }
    },
    fetchNow: () => {
      if (!inFlight && !stopped) {
        fetch(true);
      }
    },
    bumpEpoch: () => {
      epoch += 1;
      if (stopped) {
        stopped = false;
        if (!inFlight) {
          fetch();
        }
      }
    },
    stop: () => {
      stopped = true;
      cancelTimer();
      cancelTimer = () => {};
    },
  };
}

const statusResultLabel = "yt2column status result";

/**
 * Polls the status of key over the host exchange of send, timed by its
 * schedule, for the popup and the viewer alike. Each reply is logged before
 * onStep sees it: its log view, which holds neither the key nor the article,
 * the decision, and where it came from; a failed exchange at error, a
 * server's answer at info.
 */
export function createStatusPoller(
  key: ArticleKey,
  deps: { readonly send: SendDeps; readonly log: Logger },
  onStep: (step: PollStep, info: StepInfo) => void,
): Poller {
  // The poller sends one fetch at a time, so the reply a step answers is
  // the one logged last.
  let logged: StatusLogView | undefined;
  return createPoller(
    {
      fetchStatus: async () => {
        const log = classifyStatus(
          await exchange(statusMessage(key), deps.send),
        );
        logged = statusLogView(log);
        return toStatusOutcome(log);
      },
      schedule: deps.send.schedule,
    },
    (step, info) => {
      const result = logged;
      logged = undefined;
      if (result !== undefined) {
        const entry = {
          result,
          decision: step.decision,
          earlierEpoch: info.earlierEpoch,
          visible: info.visible,
        };
        if (isExchangeFailureView(result)) {
          deps.log.error(statusResultLabel, entry);
        } else {
          deps.log.info(statusResultLabel, entry);
        }
      }
      onStep(step, info);
    },
  );
}
