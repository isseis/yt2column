import assert from "node:assert/strict";
import { describe, it } from "node:test";

import {
  deleteSettings,
  loadBuildInfo,
  saveCheckDeadlineMs,
  saveSettings,
} from "../src/optionsFlow.ts";
import type { OptionsFlowDeps, OptionsForm } from "../src/optionsFlow.ts";
import {
  startWasmWorker,
  workerLoadTimeoutMs,
} from "../src/wasmWorkerClient.ts";
import { fakeLocal } from "./helpers/chrome.ts";
import type { FakeLocal } from "./helpers/chrome.ts";
import { fakeClock, settle } from "./helpers/clock.ts";
import type { FakeClock } from "./helpers/clock.ts";
import { fakeWorkers } from "./helpers/worker.ts";
import type { FakeWorkers } from "./helpers/worker.ts";

interface FakeForm extends OptionsForm {
  values: { apiKey: string; workspaceId: string };
  /** The busy flag after each setBusy call, in order. */
  readonly busy: boolean[];
  /** What the busy flag was at each call of the local functions, in order. */
  readonly busyAtCalls: [string, boolean][];
}

interface Setup {
  deps: OptionsFlowDeps;
  form: FakeForm;
  local: FakeLocal;
  workers: FakeWorkers;
  clock: FakeClock;
}

function setup(values = { apiKey: "sk-ant-key", workspaceId: "" }): Setup {
  const workers = fakeWorkers();
  const clock = fakeClock();
  const local = fakeLocal();
  const busy: boolean[] = [];
  const busyAtCalls: [string, boolean][] = [];
  const form: FakeForm = {
    values,
    busy,
    busyAtCalls,
    read: () => ({ ...form.values }),
    setBusy: (value) => {
      busy.push(value);
    },
  };
  const current = (): boolean => busy.at(-1) ?? false;
  const deps: OptionsFlowDeps = {
    form,
    startWorker: () => startWasmWorker(workers.create, clock.schedule),
    schedule: clock.schedule,
    write: (apiKey, workspaceId) => {
      busyAtCalls.push(["write", current()]);
      return local.write(apiKey, workspaceId);
    },
    remove: () => {
      busyAtCalls.push(["remove", current()]);
      return local.remove();
    },
    keys: local.keys,
  };
  return { deps, form, local, workers, clock };
}

/** Lets the save reach its Worker call and answers it. */
async function answer(s: Setup, value: string): Promise<void> {
  await settle();
  const worker = s.workers.created[0];
  assert.ok(worker !== undefined);
  worker.send({ kind: "ready" });
  await settle();
  worker.send({ callId: 1, kind: "reply", value });
}

describe("saveSettings", () => {
  it("checks the copy taken at the press and writes that copy once, after ok", async () => {
    const s = setup({
      apiKey: "sk-ant-key",
      workspaceId: "wrkspc_01AbCdEfGhJkLmNpQrStUvWx",
    });
    const saving = saveSettings(s.deps);
    await answer(s, '{"status":"ok"}');
    // Typing after the press changes nothing that is checked or written.
    s.form.values = { apiKey: "other", workspaceId: "" };
    assert.deepEqual(await saving, {
      result: { outcome: "saved" },
      apiKeySet: true,
    });
    assert.deepEqual(s.workers.created[0]?.posted, [
      {
        callId: 1,
        fn: "checkSettings",
        request: JSON.stringify({
          anthropicApiKey: "sk-ant-key",
          anthropicWorkspaceId: "wrkspc_01AbCdEfGhJkLmNpQrStUvWx",
        }),
      },
    ]);
    assert.deepEqual(s.local.items.get("anthropic.apiKey"), "sk-ant-key");
    assert.equal(
      s.local.items.get("anthropic.workspaceId"),
      "wrkspc_01AbCdEfGhJkLmNpQrStUvWx",
    );
    assert.deepEqual(s.local.calls, ["write", "keys"]);
    assert.equal(s.workers.created[0]?.terminations, 1);
  });

  it("keeps the form disabled from the press until the write has finished", async () => {
    const s = setup();
    const saving = saveSettings(s.deps);
    assert.deepEqual(s.form.busy, [true]);
    await answer(s, '{"status":"ok"}');
    await saving;
    assert.deepEqual(s.form.busy, [true, false]);
    assert.deepEqual(s.form.busyAtCalls, [["write", true]]);
  });

  it("writes an empty workspace ID as an empty string, and reports unverified", async () => {
    const s = setup();
    const saving = saveSettings(s.deps);
    await answer(s, '{"status":"unverified"}');
    assert.deepEqual((await saving).result, { outcome: "saved_unverified" });
    assert.equal(s.local.items.get("anthropic.workspaceId"), "");
  });

  it("writes nothing when the check rejects a field or the key", async () => {
    const rejected = setup();
    const first = saveSettings(rejected.deps);
    await answer(
      rejected,
      '{"status":"rejected","fields":["anthropicApiKey"]}',
    );
    assert.deepEqual((await first).result, {
      outcome: "field_rejected",
      fields: ["anthropicApiKey"],
    });
    assert.deepEqual(rejected.local.calls, ["keys"]);

    const keyRejected = setup();
    const second = saveSettings(keyRejected.deps);
    await answer(keyRejected, '{"status":"key_rejected"}');
    assert.deepEqual((await second).result, { outcome: "key_rejected" });
    assert.deepEqual(keyRejected.local.calls, ["keys"]);
  });

  it("reports the check as unavailable, and writes nothing, for an unreadable result or a Worker that fails", async () => {
    const garbled = setup();
    const first = saveSettings(garbled.deps);
    await answer(garbled, '{"status":"ok","extra":1}');
    assert.deepEqual((await first).result, { outcome: "check_unavailable" });
    assert.ok(!garbled.local.calls.includes("write"));

    const crashed = setup();
    const second = saveSettings(crashed.deps);
    await settle();
    crashed.workers.created[0]?.fail();
    assert.deepEqual((await second).result, { outcome: "check_unavailable" });
    assert.ok(!crashed.local.calls.includes("write"));
    assert.equal(crashed.form.busy.at(-1), false);
  });

  it("stops at the 90 second deadline, with no write, when the check has no result", async () => {
    const s = setup();
    const saving = saveSettings(s.deps);
    await settle();
    s.workers.created[0]?.send({ kind: "ready" });
    await settle();
    s.clock.advance(saveCheckDeadlineMs - 1);
    await settle();
    assert.equal(s.workers.created[0]?.terminations, 0);
    s.clock.advance(1);
    assert.deepEqual((await saving).result, { outcome: "check_unavailable" });
    assert.equal(s.workers.created[0]?.terminations, 1);
    assert.ok(!s.local.calls.includes("write"));
  });

  it("counts the Worker's load in the deadline: a Worker that never loads fails at its own 30 seconds", async () => {
    const s = setup();
    const saving = saveSettings(s.deps);
    await settle();
    s.clock.advance(workerLoadTimeoutMs);
    assert.deepEqual((await saving).result, { outcome: "check_unavailable" });
    assert.ok(!s.local.calls.includes("write"));
  });

  it("does not let the deadline reach the write", async () => {
    const s = setup();
    let release: () => void = () => {};
    const gate = new Promise<void>((resolve) => {
      release = resolve;
    });
    const deps: OptionsFlowDeps = {
      ...s.deps,
      write: async (apiKey, workspaceId) => {
        await gate;
        return s.local.write(apiKey, workspaceId);
      },
    };
    const saving = saveSettings(deps);
    await answer(s, '{"status":"ok"}');
    await settle();
    assert.equal(s.clock.pending(), 0);
    s.clock.advance(saveCheckDeadlineMs * 2);
    release();
    assert.deepEqual((await saving).result, { outcome: "saved" });
  });

  it("reports an uncertain write and still re-reads whether a key is set", async () => {
    const s = setup();
    s.local.fail("write");
    const saving = saveSettings(s.deps);
    await answer(s, '{"status":"ok"}');
    assert.deepEqual(await saving, {
      result: { outcome: "write_uncertain" },
      apiKeySet: false,
    });
    assert.deepEqual(s.local.calls, ["write", "keys"]);
  });

  it("reports that it cannot tell whether a key is set when the names cannot be read", async () => {
    const s = setup();
    s.local.fail("keys");
    const saving = saveSettings(s.deps);
    await answer(s, '{"status":"ok"}');
    assert.equal((await saving).apiKeySet, undefined);
  });
});

describe("deleteSettings", () => {
  it("removes with one call and re-reads whether a key is set", async () => {
    const s = setup();
    s.local.items.set("anthropic.apiKey", "sk-ant-key");
    assert.deepEqual(await deleteSettings(s.deps), {
      result: { outcome: "deleted" },
      apiKeySet: false,
    });
    assert.deepEqual(s.local.calls, ["remove", "keys"]);
    assert.deepEqual(s.form.busy, [true, false]);
    assert.deepEqual(s.form.busyAtCalls, [["remove", true]]);
  });

  it("reports an uncertain delete, and re-reads, when the remove fails", async () => {
    const s = setup();
    s.local.items.set("anthropic.apiKey", "sk-ant-key");
    s.local.fail("remove");
    assert.deepEqual(await deleteSettings(s.deps), {
      result: { outcome: "delete_uncertain" },
      apiKeySet: true,
    });
  });
});

describe("loadBuildInfo", () => {
  const info =
    '{"goVersion":"go1.26.5","vcsRevision":"","vcsModified":false,"mainPath":"github.com/isseis/yt2column/cmd/yt2column-wasm","llm":{"kind":"claude"}}';

  it("reads the build information in a Worker of its own and stops it", async () => {
    const s = setup();
    const loading = loadBuildInfo(s.deps);
    await answer(s, info);
    assert.equal(
      (await loading)?.mainPath,
      "github.com/isseis/yt2column/cmd/yt2column-wasm",
    );
    assert.equal(s.workers.created[0]?.terminations, 1);
    assert.deepEqual(s.workers.created[0]?.posted, [
      { callId: 1, fn: "buildInfo", request: "" },
    ]);
  });

  it("is undefined for an invalid result, and for a Worker that has not answered within the load deadline", async () => {
    const invalid = setup();
    const first = loadBuildInfo(invalid.deps);
    await answer(invalid, '{"goVersion":1}');
    assert.equal(await first, undefined);

    const slow = setup();
    const second = loadBuildInfo(slow.deps);
    await settle();
    slow.workers.created[0]?.send({ kind: "ready" });
    await settle();
    slow.clock.advance(workerLoadTimeoutMs);
    assert.equal(await second, undefined);
    assert.equal(slow.workers.created[0]?.terminations, 1);
  });
});
