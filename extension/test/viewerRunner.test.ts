import assert from "node:assert/strict";
import { describe, it } from "node:test";

import type { BrowserCall } from "../src/browser/chromeDeps.ts";
import { jobItemName } from "../src/core/jobRecord.ts";
import type { ReserveReply, SwRequest } from "../src/core/jobMessages.ts";
import type { Steps } from "../src/core/wasmResult.ts";
import { retryMs, runTimeoutMs, runViewer } from "../src/viewerRunner.ts";
import type {
  RunnerDeps,
  RunnerHooks,
  RunOutcome,
} from "../src/viewerRunner.ts";
import {
  startWasmWorker,
  workerLoadTimeoutMs,
} from "../src/wasmWorkerClient.ts";
import { fakeLocal, fakeSession } from "./helpers/chrome.ts";
import type { FakeLocal, FakeSession } from "./helpers/chrome.ts";
import { fakeClock, settle } from "./helpers/clock.ts";
import type { FakeClock } from "./helpers/clock.ts";
import { fakeWorkers } from "./helpers/worker.ts";
import type { FakeWorker, FakeWorkers } from "./helpers/worker.ts";

const startId = "0123456789abcdef0123456789abcdef";
const instanceId = "fedcba9876543210fedcba9876543210";
const otherInstance = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa";
const jobId = "11111111111111111111111111111111";
const secret = "sk-ant-secret-value";
const digest = "ab".repeat(32);
const input = {
  videoUrl: "https://www.youtube.com/watch?v=dQw4w9WgXcQ",
  transcript: "0:00\nHello\n",
  title: "Video",
  channelName: "Channel",
};
const preparedOk = JSON.stringify({
  status: "ok",
  inputDigest: digest,
  videoId: "dQw4w9WgXcQ",
});
const doneSteps: Steps = { draft: "done", shorten: "skipped", review: "done" };
const article = {
  markdown: "# Title\n\nBody.",
  mentionsNeutralized: false,
  videoTitle: "Video",
  channelName: "Channel",
  file: "---\n",
  cost: { state: "known", usd: "0.1234" },
  durationMs: 1000,
};
const generated = JSON.stringify({ status: "done", steps: doneSteps, article });
const progress = (steps: Steps): string => JSON.stringify({ steps });

type Replies = Partial<Record<SwRequest["type"], () => BrowserCall<unknown>>>;

interface Setup {
  clock: FakeClock;
  workers: FakeWorkers;
  local: FakeLocal;
  session: FakeSession;
  /** Every request to the service worker, in order. */
  requests: SwRequest[];
  /** Everything that happened, in order, for order checks. */
  log: string[];
  /** The replies of the service worker, by request type; changed per test. */
  replies: Replies;
  hooks: { reserved: ReserveReply[]; steps: Steps[] };
  ports: {
    hellos: unknown[];
    disconnect: (() => void)[];
    closed: number;
    fail: boolean;
  };
  unload: boolean[];
  start: () => Promise<RunOutcome>;
  /** The Worker of the run, once it exists. */
  worker: () => FakeWorker;
}

const ok = <T>(value: T): BrowserCall<T> => ({ ok: true, value });

function setup(): Setup {
  const clock = fakeClock();
  const workers = fakeWorkers();
  const local = fakeLocal();
  const session = fakeSession();
  local.items.set("anthropic.apiKey", secret);
  local.items.set("anthropic.workspaceId", "");
  const requests: SwRequest[] = [];
  const log: string[] = [];
  const replies: Replies = {
    claimInput: () => ok({ status: "ok", input }),
    reserve: () => ok({ status: "new_run", jobId }),
    prepareRejected: () => ok({ status: "stored" }),
    progress: () => ok({ status: "stored" }),
    result: () => ok({ status: "stored" }),
  };
  const hooks = { reserved: [] as ReserveReply[], steps: [] as Steps[] };
  const ports = {
    hellos: [] as unknown[],
    disconnect: [] as (() => void)[],
    closed: 0,
    fail: false,
  };
  const unload: boolean[] = [];
  const deps: RunnerDeps = {
    startWorker: () => startWasmWorker(workers.create, clock.schedule),
    schedule: clock.schedule,
    now: clock.now,
    sendToSw: async (request) => {
      requests.push(structuredClone(request));
      log.push(`sw:${request.type}`);
      const reply = replies[request.type];
      return reply === undefined ? { ok: false, error: "no reply" } : reply();
    },
    connectPort: (hello, onDisconnect) => {
      log.push("port");
      ports.hellos.push(structuredClone(hello));
      if (ports.fail) {
        return undefined;
      }
      ports.disconnect.push(onDisconnect);
      return () => {
        ports.closed += 1;
      };
    },
    readSettings: async () => {
      log.push("settings");
      return local.read();
    },
    onSessionChanged: session.onChanged,
    confirmUnload: (on) => {
      log.push(`unload:${on}`);
      unload.push(on);
    },
  };
  const runnerHooks: RunnerHooks = {
    onReserved: (reply) => hooks.reserved.push(reply),
    onProgress: (steps) => hooks.steps.push(steps),
  };
  return {
    clock,
    workers,
    local,
    session,
    requests,
    log,
    replies,
    hooks,
    ports,
    unload,
    start: () => runViewer({ startId, instanceId }, deps, runnerHooks),
    worker: () => {
      const worker = workers.created[0];
      assert.ok(worker !== undefined);
      return worker;
    },
  };
}

/** Starts the run and answers prepare, leaving it at the reservation or after. */
async function started(s: Setup): Promise<{ outcome: Promise<RunOutcome> }> {
  const running = s.start();
  await settle();
  s.worker().send({ kind: "ready" });
  s.worker().send({ callId: 1, kind: "reply", value: preparedOk });
  await settle();
  // Wrapped, so the async function does not wait for the whole run.
  return { outcome: running };
}

const types = (s: Setup): string[] => s.requests.map((request) => request.type);

/** A running record of the job, as the service worker would write it. */
function record(
  state: "running" | "stopped" | "done",
  who = instanceId,
): unknown {
  const base = {
    version: 1,
    jobId,
    inputDigest: digest,
    videoId: "dQw4w9WgXcQ",
    heading: { videoTitle: "Video", channelName: "Channel" },
    videoUrl: input.videoUrl,
    acceptedAt: 1,
  };
  switch (state) {
    case "running":
      return {
        ...base,
        state,
        steps: { draft: "running", shorten: "pending", review: "pending" },
        runner: { tabId: 5, instanceId: who },
      };
    case "stopped":
      return {
        ...base,
        state,
        steps: { draft: "running", shorten: "pending", review: "pending" },
        endedAt: 2,
        runner: { tabId: 5, instanceId: who },
        stopReason: "no_contact",
      };
    case "done":
      return { ...base, state, steps: doneSteps, endedAt: 2, article };
  }
}

describe("a run from the input to the result", () => {
  it("claims the input, prepares, reserves, reads the settings, generates and delivers the result", async () => {
    const s = setup();
    const { outcome: running } = await started(s);
    // generate is call 2; the key was read after the reservation.
    assert.deepEqual(s.log.slice(0, 5), [
      "sw:claimInput",
      "sw:reserve",
      "port",
      "unload:true",
      "settings",
    ]);
    assert.deepEqual(s.worker().posted, [
      { callId: 1, fn: "prepare", request: JSON.stringify(input) },
      {
        callId: 2,
        fn: "generate",
        request: JSON.stringify({
          inputDigest: digest,
          settings: { anthropicApiKey: secret, anthropicWorkspaceId: "" },
        }),
      },
    ]);
    assert.deepEqual(s.requests[1], {
      type: "reserve",
      startId,
      inputDigest: digest,
      videoId: "dQw4w9WgXcQ",
      instanceId,
      heading: { videoTitle: "Video", channelName: "Channel" },
      videoUrl: input.videoUrl,
    });
    assert.deepEqual(s.ports.hellos, [{ type: "hello", jobId, instanceId }]);
    s.worker().send({
      callId: 2,
      kind: "progress",
      event: progress({
        draft: "running",
        shorten: "pending",
        review: "pending",
      }),
    });
    s.worker().send({ callId: 2, kind: "reply", value: generated });
    const outcome = await running;
    assert.equal(outcome.kind, "finished");
    assert.equal(outcome.kind === "finished" && outcome.stored, "stored");
    assert.deepEqual(types(s), ["claimInput", "reserve", "progress", "result"]);
    // The confirmation is off before the result is sent, and the Worker and Port are gone.
    assert.ok(s.log.indexOf("unload:false") < s.log.indexOf("sw:result"));
    assert.deepEqual(s.unload, [true, false]);
    assert.equal(s.worker().terminations, 1);
    assert.equal(s.ports.closed, 1);
    assert.deepEqual(s.hooks.reserved, [{ status: "new_run", jobId }]);
    assert.equal(s.hooks.steps.length, 1);
    assert.equal(s.local.calls.length, 1);
  });

  it("never lets the API key reach a service worker message, a hook or the outcome", async () => {
    const s = setup();
    const { outcome: running } = await started(s);
    s.worker().send({
      callId: 2,
      kind: "progress",
      event: progress({
        draft: "running",
        shorten: "pending",
        review: "pending",
      }),
    });
    s.worker().send({ callId: 2, kind: "reply", value: generated });
    const outcome = await running;
    const seen = JSON.stringify([s.requests, s.hooks, outcome, s.ports.hellos]);
    assert.ok(!seen.includes(secret));
  });

  it("reports the failure the module returns, with its internal detail", async () => {
    const s = setup();
    const { outcome: running } = await started(s);
    s.worker().send({
      callId: 2,
      kind: "reply",
      value: JSON.stringify({
        status: "failed",
        steps: { draft: "failed", shorten: "pending", review: "pending" },
        failure: "api_key_rejected",
      }),
    });
    const outcome = await running;
    assert.deepEqual(outcome.kind === "finished" && outcome.result, {
      state: "failed",
      steps: { draft: "failed", shorten: "pending", review: "pending" },
      failure: "api_key_rejected",
    });
  });
});

describe("the saved settings", () => {
  it("does not call generate, and fails as api_key_missing, when the key item is absent", async () => {
    const s = setup();
    s.local.items.clear();
    const outcome = await (await started(s)).outcome;
    assert.equal(s.worker().posted.length, 1);
    assert.deepEqual(outcome.kind === "finished" && outcome.result, {
      state: "failed",
      steps: { draft: "pending", shorten: "pending", review: "pending" },
      failure: "api_key_missing",
    });
    assert.deepEqual(types(s), ["claimInput", "reserve", "result"]);
  });

  it("treats an empty string and null as present, and passes the value unchecked", async () => {
    for (const value of ["", null, 123]) {
      const s = setup();
      s.local.items.set("anthropic.apiKey", value);
      s.local.items.delete("anthropic.workspaceId");
      const { outcome: running } = await started(s);
      assert.equal(
        (s.worker().posted[1] as { request: string }).request,
        JSON.stringify({
          inputDigest: digest,
          settings: { anthropicApiKey: value },
        }),
      );
      s.worker().send({ callId: 2, kind: "reply", value: generated });
      await running;
    }
  });

  it("fails as internal settings_read when the read fails", async () => {
    const s = setup();
    s.local.fail("read");
    const outcome = await (await started(s)).outcome;
    assert.deepEqual(outcome.kind === "finished" && outcome.result, {
      state: "failed",
      steps: { draft: "pending", shorten: "pending", review: "pending" },
      failure: "internal",
      internalDetail: "settings_read",
    });
    assert.equal(s.worker().posted.length, 1);
  });
});

describe("the input check", () => {
  it("stops without a Worker when the service worker no longer holds the input", async () => {
    for (const reply of [
      () => ok({ status: "gone" }),
      () => ({ ok: false as const, error: "x" }),
      () => ok({ nope: 1 }),
    ]) {
      const s = setup();
      s.replies.claimInput = reply;
      assert.deepEqual(await s.start(), { kind: "input_unavailable" });
      assert.equal(s.workers.created.length, 0);
    }
  });

  it("tells the service worker an invalid body and stops the Worker", async () => {
    const s = setup();
    const running = s.start();
    await settle();
    s.worker().send({
      callId: 1,
      kind: "reply",
      value: JSON.stringify({ status: "rejected", reason: "invalid_body" }),
    });
    assert.deepEqual(await running, {
      kind: "prepare_rejected",
      reason: "invalid_body",
    });
    assert.deepEqual(s.requests[1], {
      type: "prepareRejected",
      startId,
      reason: "invalid_body",
    });
    assert.equal(s.worker().terminations, 1);
    assert.deepEqual(types(s), ["claimInput", "prepareRejected"]);
  });

  it("treats a Worker that does not answer within 30 seconds as worker_load, the load time included", async () => {
    const s = setup();
    const running = s.start();
    await settle();
    s.clock.advance(workerLoadTimeoutMs - 1);
    await settle();
    assert.deepEqual(types(s), ["claimInput"]);
    s.clock.advance(1);
    assert.deepEqual(await running, {
      kind: "prepare_rejected",
      reason: "internal",
      internalDetail: "worker_load",
    });
    assert.deepEqual(s.requests[1], {
      type: "prepareRejected",
      startId,
      reason: "internal",
      internalDetail: "worker_load",
    });
  });

  it("treats a Worker error, and a result of the wrong shape, as internal", async () => {
    const crashed = setup();
    const first = crashed.start();
    await settle();
    crashed.worker().fail();
    assert.deepEqual(await first, {
      kind: "prepare_rejected",
      reason: "internal",
      internalDetail: "worker_load",
    });

    const garbled = setup();
    const second = garbled.start();
    await settle();
    garbled.worker().send({ callId: 1, kind: "reply", value: "{}" });
    assert.deepEqual(await second, {
      kind: "prepare_rejected",
      reason: "internal",
      internalDetail: "result_shape",
    });
  });
});

describe("the reservation", () => {
  it("runs nothing, and stops the Worker, for an existing job, the same video or a refusal", async () => {
    const replies: ReserveReply[] = [
      { status: "existing", jobId },
      { status: "same_video", jobId },
      { status: "rejected", reason: "too_many_requests" },
    ];
    for (const reply of replies) {
      const s = setup();
      s.replies.reserve = () => ok(reply);
      const outcome = await (await started(s)).outcome;
      assert.deepEqual(outcome, { kind: "not_run", reply });
      assert.deepEqual(s.hooks.reserved, [reply]);
      assert.equal(s.worker().terminations, 1);
      assert.equal(s.worker().posted.length, 1);
      assert.deepEqual(s.unload, []);
      assert.deepEqual(s.ports.hellos, []);
      assert.equal(s.local.calls.length, 0);
    }
  });

  it("stops the Worker when the reservation fails or its reply is unreadable", async () => {
    for (const reply of [
      () => ({ ok: false as const, error: "x" }),
      () => ok({ status: "new_run" }),
    ]) {
      const s = setup();
      s.replies.reserve = reply;
      assert.deepEqual(await (await started(s)).outcome, {
        kind: "reserve_unavailable",
      });
      assert.equal(s.worker().terminations, 1);
    }
  });
});

describe("the deadline and the failures of the generate call", () => {
  it("fails the run as run_timeout after 50 minutes, and marks the running step failed", async () => {
    const s = setup();
    const { outcome: running } = await started(s);
    s.worker().send({
      callId: 2,
      kind: "progress",
      event: progress({ draft: "done", shorten: "running", review: "pending" }),
    });
    await settle();
    s.clock.advance(runTimeoutMs - 1);
    await settle();
    assert.equal(s.worker().terminations, 0);
    s.clock.advance(1);
    const outcome = await running;
    assert.deepEqual(outcome.kind === "finished" && outcome.result, {
      state: "failed",
      steps: { draft: "done", shorten: "failed", review: "pending" },
      failure: "run_timeout",
    });
    assert.equal(s.worker().terminations, 1);
  });

  it("fails as internal with the Worker's failure, an exit as runtime_exit, an unreadable result as result_shape", async () => {
    const cases: [(w: FakeWorker) => void, string][] = [
      [(w) => w.fail(), "worker_error"],
      [(w) => w.send({ callId: 2, kind: "exit" }), "runtime_exit"],
      [(w) => w.send({ callId: 2, kind: "error" }), "panic"],
      [(w) => w.send({ callId: 2, kind: "reply", value: "{" }), "result_shape"],
    ];
    for (const [act, detail] of cases) {
      const s = setup();
      const { outcome: running } = await started(s);
      act(s.worker());
      const outcome = await running;
      assert.deepEqual(
        outcome.kind === "finished" &&
          outcome.result?.state === "failed" && [
            outcome.result.failure,
            outcome.result.internalDetail,
          ],
        ["internal", detail],
      );
    }
  });

  it("skips an unreadable progress event and keeps the run going", async () => {
    const s = setup();
    const { outcome: running } = await started(s);
    s.worker().send({ callId: 2, kind: "progress", event: "{}" });
    s.worker().send({ callId: 2, kind: "reply", value: generated });
    const outcome = await running;
    assert.equal(outcome.kind, "finished");
    assert.deepEqual(types(s), ["claimInput", "reserve", "result"]);
  });
});

describe("delivering the result", () => {
  it("sends it again until the service worker answers", async () => {
    const s = setup();
    let answered = false;
    s.replies.result = () =>
      answered ? ok({ status: "stored" }) : { ok: false, error: "no SW" };
    const { outcome: running } = await started(s);
    s.worker().send({ callId: 2, kind: "reply", value: generated });
    await settle();
    assert.equal(types(s).filter((type) => type === "result").length, 1);
    s.clock.advance(retryMs);
    await settle();
    assert.equal(types(s).filter((type) => type === "result").length, 2);
    answered = true;
    s.clock.advance(retryMs);
    const outcome = await running;
    assert.equal(outcome.kind === "finished" && outcome.stored, "stored");
    assert.equal(types(s).filter((type) => type === "result").length, 3);
    assert.equal(s.clock.pending(), 0);
  });

  it("does not send it again after not_stored, and reports that the record does not hold it", async () => {
    const s = setup();
    s.replies.result = () => ok({ status: "not_stored" });
    const { outcome: running } = await started(s);
    s.worker().send({ callId: 2, kind: "reply", value: generated });
    const outcome = await running;
    assert.equal(outcome.kind === "finished" && outcome.stored, "not_stored");
    assert.equal(types(s).filter((type) => type === "result").length, 1);
  });

  it("stops sending when the record holds a result, though the service worker never answered", async () => {
    const s = setup();
    s.replies.result = () => ({ ok: false, error: "no SW" });
    const { outcome: running } = await started(s);
    s.worker().send({ callId: 2, kind: "reply", value: generated });
    await settle();
    await s.session.write({ [jobItemName(jobId)]: record("done") });
    await settle();
    const outcome = await running;
    assert.equal(outcome.kind === "finished" && outcome.stored, "in_record");
  });

  it("stops when the service worker says this page is not the runner", async () => {
    const s = setup();
    s.replies.result = () => ok({ status: "not_runner" });
    const { outcome: running } = await started(s);
    s.worker().send({ callId: 2, kind: "reply", value: generated });
    assert.deepEqual(await running, { kind: "stopped", jobId });
  });
});

describe("watching the record", () => {
  it("stops the Worker, and sends no result, when another page runs the job", async () => {
    const s = setup();
    const { outcome: running } = await started(s);
    await s.session.write({
      [jobItemName(jobId)]: record("running", otherInstance),
    });
    assert.deepEqual(await running, { kind: "stopped", jobId });
    assert.equal(s.worker().terminations, 1);
    assert.ok(!types(s).includes("result"));
    assert.deepEqual(s.unload, [true, false]);
  });

  it("stops the Worker when the record is removed, or a result appears in it that this page does not hold", async () => {
    const removed = setup();
    const { outcome: first } = await started(removed);
    await removed.session.write({ [jobItemName(jobId)]: record("running") });
    await removed.session.remove([jobItemName(jobId)]);
    assert.deepEqual(await first, { kind: "stopped", jobId });

    const done = setup();
    const { outcome: second } = await started(done);
    await done.session.write({ [jobItemName(jobId)]: record("done") });
    assert.deepEqual(await second, { kind: "stopped", jobId });
  });

  it("keeps running when its own record becomes stopped, and finishes with the result", async () => {
    const s = setup();
    const { outcome: running } = await started(s);
    await s.session.write({ [jobItemName(jobId)]: record("stopped") });
    await settle();
    assert.equal(s.worker().terminations, 0);
    s.worker().send({ callId: 2, kind: "reply", value: generated });
    const outcome = await running;
    assert.equal(outcome.kind, "finished");
  });

  it("ignores another job's record and a value that is not a record", async () => {
    const s = setup();
    const { outcome: running } = await started(s);
    await s.session.write({
      [jobItemName("22222222222222222222222222222222")]: record("done"),
      [jobItemName(jobId)]: { junk: true },
    });
    await settle();
    assert.equal(s.worker().terminations, 0);
    s.worker().send({ callId: 2, kind: "reply", value: generated });
    await running;
  });

  it("stops the Worker when a progress write says this page is not the runner", async () => {
    const s = setup();
    s.replies.progress = () => ok({ status: "not_runner" });
    const { outcome: running } = await started(s);
    s.worker().send({
      callId: 2,
      kind: "progress",
      event: progress({
        draft: "running",
        shorten: "pending",
        review: "pending",
      }),
    });
    assert.deepEqual(await running, { kind: "stopped", jobId });
    assert.equal(s.worker().terminations, 1);
  });

  it("stops watching when the run ends", async () => {
    const s = setup();
    const { outcome: running } = await started(s);
    s.worker().send({ callId: 2, kind: "reply", value: generated });
    await running;
    await s.session.write({
      [jobItemName(jobId)]: record("running", otherInstance),
    });
    await settle();
    assert.equal(s.worker().terminations, 1);
  });
});

describe("the runner's Port", () => {
  it("reconnects at once when it is cut, and retries a connection that cannot be made", async () => {
    const s = setup();
    const { outcome: running } = await started(s);
    assert.equal(s.ports.hellos.length, 1);
    s.ports.disconnect[0]?.();
    assert.equal(s.ports.hellos.length, 2);
    s.ports.fail = true;
    s.ports.disconnect[1]?.();
    assert.equal(s.ports.hellos.length, 3);
    s.clock.advance(retryMs);
    assert.equal(s.ports.hellos.length, 4);
    s.ports.fail = false;
    s.clock.advance(retryMs);
    assert.equal(s.ports.hellos.length, 5);
    s.worker().send({ callId: 2, kind: "reply", value: generated });
    await running;
    assert.equal(s.ports.closed, 1);
    assert.equal(s.clock.pending(), 0);
    // A cut after the run does not open a Port again.
    s.ports.disconnect[s.ports.disconnect.length - 1]?.();
    assert.equal(s.ports.hellos.length, 5);
  });
});
