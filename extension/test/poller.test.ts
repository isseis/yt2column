import assert from "node:assert/strict";
import { describe, it } from "node:test";

import { pollIntervalMs } from "../src/core/poll.ts";
import type { PollStep } from "../src/core/poll.ts";
import type { JobStatus, StatusOutcome } from "../src/core/response.ts";
import { createPoller } from "../src/poller.ts";
import type { Poller } from "../src/poller.ts";
import { fakeClock, settle, type FakeClock } from "./helpers/clock.ts";
import { article, doneSteps, requestId, runningSteps } from "./helpers/job.ts";

function ok(job: JobStatus): StatusOutcome {
  return { kind: "ok", requestId, webhookConfigured: true, job };
}

const running = ok({ state: "running", steps: runningSteps });
const failed = ok({
  state: "failed",
  steps: doneSteps,
  failure: "llm_failed",
});
const notPosted = ok({
  state: "done",
  steps: doneSteps,
  article,
  publish: { state: "not_posted" },
});
const posting = ok({
  state: "done",
  steps: doneSteps,
  article,
  publish: { state: "posting" },
});
const unknown = ok({ state: "unknown" });

interface Harness {
  readonly clock: FakeClock;
  readonly poller: Poller;
  readonly steps: PollStep[];
  /** How many status exchanges the poller has started. */
  fetches(): number;
  /** Answers the exchange in flight with outcome and lets the reply run. */
  reply(outcome: StatusOutcome): Promise<void>;
}

/** A poller whose exchanges stay in flight until the test answers them. */
function harness(): Harness {
  const clock = fakeClock();
  const steps: PollStep[] = [];
  const waiting: ((outcome: StatusOutcome) => void)[] = [];
  let count = 0;
  const poller = createPoller(
    {
      fetchStatus: () => {
        count += 1;
        return new Promise((resolve) => waiting.push(resolve));
      },
      schedule: clock.schedule,
    },
    (step) => steps.push(step),
  );
  return {
    clock,
    poller,
    steps,
    fetches: () => count,
    reply: async (outcome) => {
      const answer = waiting.shift();
      assert.ok(answer, "no exchange is in flight");
      answer(outcome);
      await settle();
    },
  };
}

describe("createPoller", () => {
  it("fetches at once on start and again one interval after a reply to continue", async () => {
    const { clock, poller, fetches, reply } = harness();
    poller.start();
    assert.equal(fetches(), 1);
    await reply(running);
    clock.advance(pollIntervalMs - 1);
    assert.equal(fetches(), 1);
    clock.advance(1);
    assert.equal(fetches(), 2);
  });

  it("sends no next fetch before the reply of the one in flight", async () => {
    const { clock, poller, fetches, reply } = harness();
    poller.start();
    clock.advance(10 * pollIntervalMs);
    poller.fetchNow();
    assert.equal(fetches(), 1);
    assert.equal(clock.pending(), 0);
    await reply(running);
    assert.equal(clock.pending(), 1);
  });

  it("does nothing on fetchNow before start, while in flight, or after a stop decision", async () => {
    const { clock, poller, fetches, reply } = harness();
    poller.fetchNow();
    assert.equal(fetches(), 0);
    poller.start();
    poller.fetchNow();
    assert.equal(fetches(), 1);
    await reply(failed);
    poller.fetchNow();
    clock.advance(10 * pollIntervalMs);
    assert.equal(fetches(), 1);
  });

  it("cancels the timer whatever starts a fetch, so repeated triggers keep one chain", async () => {
    const { clock, poller, fetches, reply } = harness();
    poller.start();
    await reply(running);
    for (let n = 0; n < 3; n += 1) {
      poller.fetchNow();
      assert.equal(clock.pending(), 0);
      await reply(running);
    }
    assert.equal(fetches(), 4);
    poller.start();
    assert.equal(clock.pending(), 0);
    await reply(running);
    clock.advance(pollIntervalMs);
    assert.equal(fetches(), 6);
    assert.equal(clock.pending(), 0);
  });

  it("stops on a reply of the current epoch whose decision is stop", async () => {
    const { clock, poller, steps, fetches, reply } = harness();
    poller.start();
    await reply(notPosted);
    clock.advance(10 * pollIntervalMs);
    assert.equal(fetches(), 1);
    assert.equal(steps.at(-1)?.decision, "stop");
  });

  // A take-over or a publish moves to a new epoch while a fetch sent before
  // it is in flight; each row's reply would stop polling in the current
  // epoch (the test above).
  for (const { name, outcome } of [
    { name: "a failed job sent before a take-over", outcome: failed },
    { name: "a not posted article sent before a publish", outcome: notPosted },
  ]) {
    it(`does not stop on ${name}, and fetches the new epoch at once`, async () => {
      const { poller, steps, fetches, reply } = harness();
      poller.start();
      poller.bumpEpoch();
      assert.equal(fetches(), 1);
      await reply(outcome);
      assert.equal(fetches(), 2);
      assert.equal(steps.length, 1);
      assert.equal(steps[0]?.decision, "continue");
      assert.ok(
        steps[0]?.job,
        "a reply of an earlier epoch still updates the display",
      );
      await reply(outcome);
      assert.equal(steps[1]?.decision, "stop");
    });
  }

  it("starts polling on bumpEpoch after a stop decision", async () => {
    const { poller, fetches, reply } = harness();
    poller.start();
    await reply(failed);
    poller.bumpEpoch();
    assert.equal(fetches(), 2);
    await reply(running);
    assert.equal(fetches(), 2);
  });

  it("keeps the timer on bumpEpoch while waiting for the next fetch", async () => {
    const { clock, poller, fetches, reply } = harness();
    poller.start();
    await reply(running);
    poller.bumpEpoch();
    assert.equal(fetches(), 1);
    clock.advance(pollIntervalMs);
    assert.equal(fetches(), 2);
  });

  it("cancels the timer on stop and drops a reply that arrives after it", async () => {
    const { clock, poller, steps, fetches, reply } = harness();
    poller.start();
    await reply(running);
    poller.stop();
    assert.equal(clock.pending(), 0);
    clock.advance(10 * pollIntervalMs);
    assert.equal(fetches(), 1);

    poller.start();
    poller.stop();
    await reply(running);
    assert.equal(steps.length, 1);
    assert.equal(clock.pending(), 0);
    assert.equal(fetches(), 2);
  });

  it("gives afterStatus the last job shown since start, which start clears", async () => {
    const { poller, steps, reply } = harness();
    poller.start();
    await reply(posting);
    // A notice-only step keeps the job shown as the previous display.
    poller.fetchNow();
    await reply({ kind: "host-failed" });
    assert.equal(steps.at(-1)?.job, undefined);
    poller.fetchNow();
    await reply(unknown);
    assert.deepEqual(steps.at(-1)?.job, { kind: "lost", kept: article });

    poller.start();
    await reply(unknown);
    assert.deepEqual(steps.at(-1)?.job, { kind: "lost" });
  });
});
