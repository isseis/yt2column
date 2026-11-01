// The polling rule: after each status exchange, what the display becomes and
// whether to fetch again. Polling repeats only what can change by itself (a
// running job, a posting article) or an exchange that may succeed on a later
// try; a result that repeating cannot change stops it. A failed exchange
// never replaces the state shown: it adds a notice beside it.

import type { JobDisplay, PollNotice } from "./jobView.ts";
import type { Article, StatusOutcome } from "./response.ts";

/** The fixed polling interval: each status query starts one host process. */
export const pollIntervalMs = 2_000;

/** How long after the send time a viewer or the popup waits for the start result. */
export const startResultDeadlineMs = 35_000;

/** How long a query for the viewers of a key collects answers. */
export const viewerQueryWindowMs = 1_000;

export interface PollStep {
  readonly decision: "continue" | "stop";
  /** The new display; absent when the state shown is kept. */
  readonly job?: JobDisplay;
  readonly notice: PollNotice;
}

/**
 * The article a lost job keeps: the one a done job showed just before, or the
 * one a lost job already kept. A reply of an earlier epoch can show lost
 * before the new epoch's fetch returns unknown again, and the article shown
 * must stay saveable and copyable.
 */
function shownArticle(previous: JobDisplay | undefined): Article | undefined {
  switch (previous?.kind) {
    case "done":
      return previous.article;
    case "lost":
      return previous.kept;
    case "running":
    case "failed":
    case undefined:
      return undefined;
  }
}

const noNotice: PollNotice = { kind: "none" };

/** A status the server returned: the job is shown, and only work in progress is polled again. */
function afterJob(
  previous: JobDisplay | undefined,
  outcome: Extract<StatusOutcome, { kind: "ok" }>,
): PollStep {
  const { job } = outcome;
  switch (job.state) {
    case "running":
      return {
        decision: "continue",
        job: { kind: "running", steps: job.steps },
        notice: noNotice,
      };
    case "done":
      return {
        // A posting article is polled until it is no longer posting, whoever
        // started the publish.
        decision: job.publish.state === "posting" ? "continue" : "stop",
        job: {
          kind: "done",
          steps: job.steps,
          article: job.article,
          publish: job.publish,
          webhookConfigured: outcome.webhookConfigured,
        },
        notice: noNotice,
      };
    case "failed":
      return {
        decision: "stop",
        job: { kind: "failed", steps: job.steps, failure: job.failure },
        notice: noNotice,
      };
    case "unknown": {
      const kept = shownArticle(previous);
      return {
        decision: "stop",
        job: kept === undefined ? { kind: "lost" } : { kind: "lost", kept },
        notice: noNotice,
      };
    }
  }
}

/** A send known not to have reached the server: retried only while the server may start. */
function afterNotSent(
  outcome: Extract<StatusOutcome, { kind: "not-sent" }>,
): PollStep {
  switch (outcome.cause) {
    case "server-unavailable":
      return {
        decision: "continue",
        notice: {
          kind: "retrying",
          outcome: { kind: "not-sent", cause: "server-unavailable" },
        },
      };
    // A key alone is never too large; repeating cannot change it.
    case "message-too-large":
      return {
        decision: "stop",
        notice: {
          kind: "stopped",
          outcome: { kind: "not-sent", cause: "message-too-large" },
        },
      };
  }
}

/** The display update and the decision after one status exchange. Pure. */
export function afterStatus(
  previous: JobDisplay | undefined,
  outcome: StatusOutcome,
): PollStep {
  switch (outcome.kind) {
    case "ok":
      return afterJob(previous, outcome);
    case "not-sent":
      return afterNotSent(outcome);
    case "no-response":
    case "host-failed":
      return { decision: "continue", notice: { kind: "retrying", outcome } };
    case "rejected":
    case "unexpected":
      return { decision: "stop", notice: { kind: "stopped", outcome } };
  }
}
