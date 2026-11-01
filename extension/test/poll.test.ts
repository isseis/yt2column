import assert from "node:assert/strict";
import { describe, it } from "node:test";

import type { JobDisplay } from "../src/core/jobView.ts";
import { afterStatus } from "../src/core/poll.ts";
import type { PollStep } from "../src/core/poll.ts";
import type { JobStatus, StatusOutcome } from "../src/core/response.ts";
import {
  article,
  doneJob,
  doneSteps,
  requestId,
  runningSteps,
} from "./helpers/job.ts";

/** A status the server returned for the job. */
function ok(job: JobStatus, webhookConfigured = true): StatusOutcome {
  return { kind: "ok", requestId, webhookConfigured, job };
}

const running: JobDisplay = { kind: "running", steps: runningSteps };

describe("afterStatus", () => {
  const rows: {
    name: string;
    previous?: JobDisplay;
    outcome: StatusOutcome;
    step: PollStep;
  }[] = [
    {
      name: "a running job continues",
      outcome: ok({ state: "running", steps: runningSteps }),
      step: { decision: "continue", job: running, notice: { kind: "none" } },
    },
    {
      name: "a done article that is posting continues",
      previous: doneJob({ state: "not_posted" }),
      outcome: ok({
        state: "done",
        steps: doneSteps,
        article,
        publish: { state: "posting" },
      }),
      step: {
        decision: "continue",
        job: doneJob({ state: "posting" }),
        notice: { kind: "none" },
      },
    },
    {
      name: "a done article not posting stops, with its Webhook setting",
      previous: running,
      outcome: ok(
        {
          state: "done",
          steps: doneSteps,
          article,
          publish: { state: "posted" },
        },
        false,
      ),
      step: {
        decision: "stop",
        job: doneJob({ state: "posted" }, false),
        notice: { kind: "none" },
      },
    },
    ...(
      [
        { state: "not_posted" },
        { state: "failed", reason: "webhook" },
        { state: "unpostable", reason: "mention" },
      ] as const
    ).map((publish) => ({
      name: `a done article ${publish.state} stops`,
      outcome: ok({ state: "done", steps: doneSteps, article, publish }),
      step: {
        decision: "stop" as const,
        job: doneJob(publish),
        notice: { kind: "none" as const },
      },
    })),
    {
      name: "a failed job stops",
      previous: running,
      outcome: ok({
        state: "failed",
        steps: runningSteps,
        failure: "llm_timeout",
      }),
      step: {
        decision: "stop",
        job: { kind: "failed", steps: runningSteps, failure: "llm_timeout" },
        notice: { kind: "none" },
      },
    },
    {
      name: "an unknown job after running stops without an article",
      previous: running,
      outcome: ok({ state: "unknown" }),
      step: {
        decision: "stop",
        job: { kind: "lost" },
        notice: { kind: "none" },
      },
    },
    {
      name: "an unknown job on the first status stops without an article",
      outcome: ok({ state: "unknown" }),
      step: {
        decision: "stop",
        job: { kind: "lost" },
        notice: { kind: "none" },
      },
    },
    {
      name: "an unknown job after a done article keeps the article",
      previous: doneJob({ state: "posted" }),
      outcome: ok({ state: "unknown" }),
      step: {
        decision: "stop",
        job: { kind: "lost", kept: article },
        notice: { kind: "none" },
      },
    },
    {
      name: "an unknown job after a lost one keeps its kept article",
      previous: { kind: "lost", kept: article },
      outcome: ok({ state: "unknown" }),
      step: {
        decision: "stop",
        job: { kind: "lost", kept: article },
        notice: { kind: "none" },
      },
    },
    {
      name: "a server that is not running keeps the state and continues",
      previous: running,
      outcome: { kind: "not-sent", cause: "server-unavailable" },
      step: {
        decision: "continue",
        notice: {
          kind: "retrying",
          outcome: { kind: "not-sent", cause: "server-unavailable" },
        },
      },
    },
    {
      name: "no response keeps the state and continues",
      previous: running,
      outcome: { kind: "no-response" },
      step: {
        decision: "continue",
        notice: { kind: "retrying", outcome: { kind: "no-response" } },
      },
    },
    {
      name: "a host failure keeps the state and continues",
      previous: running,
      outcome: { kind: "host-failed" },
      step: {
        decision: "continue",
        notice: { kind: "retrying", outcome: { kind: "host-failed" } },
      },
    },
    {
      name: "a message too large keeps the state and stops",
      previous: running,
      outcome: { kind: "not-sent", cause: "message-too-large" },
      step: {
        decision: "stop",
        notice: {
          kind: "stopped",
          outcome: { kind: "not-sent", cause: "message-too-large" },
        },
      },
    },
    {
      name: "an invalid key keeps the state and stops, with its request ID",
      previous: running,
      outcome: { kind: "rejected", reason: "invalid_key", requestId },
      step: {
        decision: "stop",
        notice: {
          kind: "stopped",
          outcome: { kind: "rejected", reason: "invalid_key", requestId },
        },
      },
    },
    {
      name: "an internal error keeps the state and stops",
      previous: running,
      outcome: { kind: "rejected", reason: "internal_error", requestId },
      step: {
        decision: "stop",
        notice: {
          kind: "stopped",
          outcome: { kind: "rejected", reason: "internal_error", requestId },
        },
      },
    },
    {
      name: "an unexpected reply keeps the state and stops",
      previous: running,
      outcome: { kind: "unexpected", check: "member-set" },
      step: {
        decision: "stop",
        notice: {
          kind: "stopped",
          outcome: { kind: "unexpected", check: "member-set" },
        },
      },
    },
  ];
  for (const { name, previous, outcome, step } of rows) {
    it(name, () => {
      assert.deepEqual(afterStatus(previous, outcome), step);
    });
  }
});
