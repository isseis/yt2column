import assert from "node:assert/strict";
import { existsSync } from "node:fs";
import path from "node:path";
import { describe, it } from "node:test";

import {
  startWasmWorker,
  wasmWorkerScript,
  workerLoadTimeoutMs,
} from "../src/wasmWorkerClient.ts";
import type { WasmWorker, WorkerCallResult } from "../src/wasmWorkerClient.ts";
import { fakeClock, settle } from "./helpers/clock.ts";
import type { FakeClock } from "./helpers/clock.ts";
import { extensionDir } from "./helpers/paths.ts";
import { fakeWorkers } from "./helpers/worker.ts";
import type { FakeWorker } from "./helpers/worker.ts";

/** A started client, its fake Worker and clock. */
function started(): {
  client: WasmWorker;
  worker: FakeWorker;
  clock: FakeClock;
} {
  const workers = fakeWorkers();
  const clock = fakeClock();
  const client = startWasmWorker(workers.create, clock.schedule);
  const [worker] = workers.created;
  assert.ok(worker !== undefined);
  return { client, worker, clock };
}

/** The result of a call once it has settled; undefined while it waits. */
function track(promise: Promise<WorkerCallResult>): {
  result: () => WorkerCallResult | undefined;
} {
  let result: WorkerCallResult | undefined;
  void promise.then((value) => {
    result = value;
  });
  return { result: () => result };
}

/** Makes the first call and answers it, so the Worker counts as loaded. */
async function loaded(): Promise<{
  client: WasmWorker;
  worker: FakeWorker;
  clock: FakeClock;
}> {
  const parts = started();
  const first = parts.client.call("buildInfo", "");
  parts.worker.send({ callId: 1, kind: "reply", value: "{}" });
  assert.deepEqual(await first, { kind: "reply", value: "{}" });
  return parts;
}

describe("wasm Worker client", () => {
  it("posts each call with a new callId and matches the reply by it", async () => {
    const { client, worker } = started();
    const first = client.call("prepare", '{"a":1}');
    assert.deepEqual(worker.posted, [
      { callId: 1, fn: "prepare", request: '{"a":1}' },
    ]);
    worker.send({ callId: 1, kind: "reply", value: "one" });
    assert.deepEqual(await first, { kind: "reply", value: "one" });

    const second = track(client.call("generate", "g"));
    assert.deepEqual(worker.posted[1], {
      callId: 2,
      fn: "generate",
      request: "g",
    });
    // A late answer to the first call is not the second call's.
    worker.send({ callId: 1, kind: "reply", value: "stale" });
    await settle();
    assert.equal(second.result(), undefined);
    worker.send({ callId: 2, kind: "reply", value: "two" });
    await settle();
    assert.deepEqual(second.result(), { kind: "reply", value: "two" });
    assert.equal(worker.terminations, 0);
  });

  it("passes the waiting call's progress on, and no other call's", async () => {
    const { client, worker } = await loaded();
    const events: string[] = [];
    const call = client.call("generate", "g", {
      onProgress: (event) => events.push(event),
    });
    worker.send({ callId: 1, kind: "progress", event: "old" });
    worker.send({ callId: 2, kind: "progress", event: "p1" });
    worker.send({ callId: 2, kind: "progress", event: "p2" });
    worker.send({ callId: 2, kind: "reply", value: "done" });
    worker.send({ callId: 2, kind: "progress", event: "late" });
    assert.deepEqual(await call, { kind: "reply", value: "done" });
    assert.deepEqual(events, ["p1", "p2"]);
  });

  it("fails as worker_load when the first answer does not come within the load deadline", async () => {
    const { client, worker, clock } = started();
    const call = track(client.call("prepare", "p"));
    clock.advance(workerLoadTimeoutMs - 1);
    await settle();
    assert.equal(call.result(), undefined);
    clock.advance(1);
    await settle();
    assert.deepEqual(call.result(), {
      kind: "failed",
      detail: "worker_load",
    });
    assert.equal(worker.terminations, 1);
  });

  it("stops the load deadline at the first answer, even a progress event", async () => {
    const { client, worker, clock } = started();
    const call = track(client.call("generate", "g"));
    worker.send({ callId: 1, kind: "progress", event: "e" });
    clock.advance(workerLoadTimeoutMs * 2);
    await settle();
    assert.equal(call.result(), undefined);
    assert.equal(worker.terminations, 0);
  });

  const failures: {
    name: string;
    act: (worker: FakeWorker, callId: number) => void;
    beforeLoad: string;
    afterLoad: string;
  }[] = [
    {
      name: "an error event",
      act: (worker) => worker.fail(),
      beforeLoad: "worker_load",
      afterLoad: "worker_error",
    },
    {
      name: "an exit",
      act: (worker, callId) => worker.send({ callId, kind: "exit" }),
      beforeLoad: "worker_load",
      afterLoad: "runtime_exit",
    },
    {
      name: "a rejected Promise",
      act: (worker, callId) => worker.send({ callId, kind: "error" }),
      beforeLoad: "panic",
      afterLoad: "panic",
    },
    {
      name: "a message of another shape",
      act: (worker, callId) =>
        worker.send({ callId, kind: "reply", value: "v", extra: 1 }),
      beforeLoad: "worker_load",
      afterLoad: "worker_error",
    },
  ];
  for (const { name, act, beforeLoad, afterLoad } of failures) {
    it(`fails as ${beforeLoad} on ${name} before the module has answered`, async () => {
      const { client, worker } = started();
      const call = client.call("prepare", "p");
      act(worker, 1);
      assert.deepEqual(await call, { kind: "failed", detail: beforeLoad });
      assert.equal(worker.terminations, 1);
    });

    it(`fails as ${afterLoad} on ${name} after the module has answered`, async () => {
      const { client, worker } = await loaded();
      const call = client.call("generate", "g");
      act(worker, 2);
      assert.deepEqual(await call, { kind: "failed", detail: afterLoad });
      assert.equal(worker.terminations, 1);
    });
  }

  it("uses only the first failure and ignores what the Worker sends after it", async () => {
    const { client, worker, clock } = await loaded();
    const events: string[] = [];
    const call = track(
      client.call("generate", "g", {
        onProgress: (event) => events.push(event),
        timeoutMs: 1_000,
      }),
    );
    worker.send({ callId: 2, kind: "exit" });
    // A Go timer left behind can raise an error event after the exit.
    worker.fail();
    worker.send({ callId: 2, kind: "reply", value: "late" });
    worker.send({ callId: 2, kind: "progress", event: "late" });
    clock.advance(workerLoadTimeoutMs + 1_000);
    await settle();
    assert.deepEqual(call.result(), {
      kind: "failed",
      detail: "runtime_exit",
    });
    assert.deepEqual(events, []);
    assert.equal(worker.terminations, 1);
    assert.equal(clock.pending(), 0);
  });

  it("ends the Worker as timeout when the call's own deadline passes", async () => {
    const { client, worker, clock } = await loaded();
    const call = track(client.call("generate", "g", { timeoutMs: 5_000 }));
    clock.advance(4_999);
    await settle();
    assert.equal(call.result(), undefined);
    clock.advance(1);
    await settle();
    assert.deepEqual(call.result(), { kind: "timeout" });
    assert.equal(worker.terminations, 1);
  });

  it("cancels the call's deadline when the answer comes", async () => {
    const { client, worker, clock } = await loaded();
    const call = client.call("generate", "g", { timeoutMs: 5_000 });
    worker.send({ callId: 2, kind: "reply", value: "v" });
    await call;
    assert.equal(clock.pending(), 0);
    assert.equal(worker.terminations, 0);
  });

  it("resolves a waiting call as stopped on terminate, and every later call", async () => {
    const { client, worker } = started();
    const call = client.call("prepare", "p");
    client.terminate();
    client.terminate();
    assert.deepEqual(await call, { kind: "stopped" });
    assert.deepEqual(await client.call("buildInfo", ""), { kind: "stopped" });
    assert.equal(worker.terminations, 1);
    assert.equal(worker.posted.length, 1);
  });

  it("answers a call made after a failure as stopped without posting it", async () => {
    const { client, worker } = started();
    const call = client.call("prepare", "p");
    worker.fail();
    await call;
    assert.deepEqual(await client.call("prepare", "p"), { kind: "stopped" });
    assert.equal(worker.posted.length, 1);
  });

  it("refuses a second call while one waits", () => {
    const { client } = started();
    void client.call("prepare", "p");
    assert.throws(() => client.call("prepare", "p"));
  });

  it("names a Worker script the build compiles from src/", () => {
    assert.ok(
      existsSync(
        path.join(
          extensionDir,
          "src",
          wasmWorkerScript.replace(/\.js$/, ".ts"),
        ),
      ),
    );
  });
});
