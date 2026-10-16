import assert from "node:assert/strict";
import { describe, it } from "node:test";
import vm from "node:vm";

import {
  createResultViewStore,
  createSelectionReader,
  createSettingsStore,
  createSummaryStore,
  type ScriptingApi,
  type StorageArea,
  type StorageChangeEvents,
  type StorageChangeListener,
} from "../src/browser/chromeDeps.ts";
import type { PageSelection } from "../src/core/collect.ts";
import type { ResultView } from "../src/core/resultView.ts";
import type { ServerPort, SharedToken } from "../src/core/settings.ts";

type Injection = Parameters<ScriptingApi["executeScript"]>[0];

interface ScriptingRecorder {
  readonly scripting: ScriptingApi;
  readonly injections: Injection[];
}

/** A ScriptingApi whose executeScript records injections and delegates to respond. */
function recordingScripting(
  respond: (injection: Injection) => Promise<Array<{ result?: unknown }>>,
): ScriptingRecorder {
  const injections: Injection[] = [];
  return {
    scripting: {
      executeScript: async (injection) => {
        injections.push(injection);
        return respond(injection);
      },
    },
    injections,
  };
}

describe("selection reader", () => {
  it("resolves the selection the injected function returned", async () => {
    const selection: PageSelection = { text: "hello", documentUrl: "u" };
    const { scripting, injections } = recordingScripting(async () => [
      { result: selection },
    ]);
    const reader = createSelectionReader(scripting);
    assert.deepEqual(await reader.read(3), selection);
    assert.equal(injections[0]?.target.tabId, 3);
  });

  const malformed: { name: string; results: { result?: unknown }[] }[] = [
    { name: "an empty result list", results: [] },
    { name: "a null result", results: [{ result: null }] },
    { name: "an undefined result", results: [{ result: undefined }] },
    { name: "a string result", results: [{ result: "text" }] },
    { name: "a missing documentUrl", results: [{ result: { text: "x" } }] },
    { name: "a missing text", results: [{ result: { documentUrl: "u" } }] },
    {
      name: "a non-string text",
      results: [{ result: { text: 1, documentUrl: "u" } }],
    },
    {
      name: "a non-string documentUrl",
      results: [{ result: { text: "x", documentUrl: 1 } }],
    },
  ];
  for (const { name, results } of malformed) {
    it(`rejects ${name}`, async () => {
      const { scripting } = recordingScripting(async () => results);
      await assert.rejects(createSelectionReader(scripting).read(1));
    });
  }

  it("rejects when the injection throws", async () => {
    const { scripting } = recordingScripting(async () => {
      throw new Error("injection denied");
    });
    await assert.rejects(createSelectionReader(scripting).read(1));
  });

  it("reads only window.getSelection and location", async () => {
    const { scripting, injections } = recordingScripting(async () => [
      { result: { text: "x", documentUrl: "u" } },
    ]);
    await createSelectionReader(scripting).read(1);
    const injection = injections[0];
    assert.ok(injection !== undefined);
    const context = {
      window: { getSelection: () => ({ toString: () => "hello\nworld" }) },
      location: { href: "https://www.youtube.com/watch?v=abc" },
    };
    const result = vm.runInNewContext(
      `(${injection.func.toString()})()`,
      context,
    ) as PageSelection;
    assert.equal(result.text, "hello\nworld");
    assert.equal(result.documentUrl, "https://www.youtube.com/watch?v=abc");
  });
});

/** The keys of a StorageArea call as a list, whether one key or many. */
function keyList(keys: string | string[]): string[] {
  return typeof keys === "string" ? [keys] : keys;
}

/** An in-memory StorageArea backed by entries. */
function memoryStorage(entries: Map<string, unknown>): StorageArea {
  return {
    get: async (keys) => {
      const values: Record<string, unknown> = {};
      for (const key of keyList(keys)) {
        if (entries.has(key)) {
          values[key] = entries.get(key);
        }
      }
      return values;
    },
    set: async (items) => {
      for (const [key, value] of Object.entries(items)) {
        entries.set(key, value);
      }
    },
    remove: async (keys) => {
      for (const key of keyList(keys)) {
        entries.delete(key);
      }
    },
  };
}

describe("summary store", () => {
  it("returns the stored value from take and removes it", async () => {
    const entries = new Map<string, unknown>([["key-1", "summary"]]);
    const store = createSummaryStore(memoryStorage(entries));
    assert.equal(await store.take("key-1"), "summary");
    assert.equal(entries.has("key-1"), false);
  });
});

interface AreaRecorder {
  readonly area: StorageArea;
  readonly gets: (string | string[])[];
  readonly sets: Record<string, unknown>[];
  readonly removes: (string | string[])[];
}

/** A memoryStorage that also records each call's keys. */
function recordingArea(entries: Map<string, unknown>): AreaRecorder {
  const base = memoryStorage(entries);
  const gets: (string | string[])[] = [];
  const sets: Record<string, unknown>[] = [];
  const removes: (string | string[])[] = [];
  return {
    area: {
      get: async (keys) => {
        gets.push(keys);
        return base.get(keys);
      },
      set: async (items) => {
        sets.push(items);
        return base.set(items);
      },
      remove: async (keys) => {
        removes.push(keys);
        return base.remove(keys);
      },
    },
    gets,
    sets,
    removes,
  };
}

describe("settings store", () => {
  it("loads both keys with one get", async () => {
    const entries = new Map<string, unknown>([
      ["sharedToken", "token"],
      ["serverPort", "8765"],
    ]);
    const { area, gets } = recordingArea(entries);
    const store = createSettingsStore(area);
    assert.deepEqual(await store.load(), { token: "token", port: "8765" });
    assert.deepEqual(gets, [["sharedToken", "serverPort"]]);
  });

  it("saves the token and the port under separate keys", async () => {
    const { area, sets } = recordingArea(new Map());
    const store = createSettingsStore(area);
    await store.saveToken("token" as SharedToken);
    await store.savePort("8765" as ServerPort);
    assert.deepEqual(sets, [{ sharedToken: "token" }, { serverPort: "8765" }]);
  });

  it("clears only the token", async () => {
    const entries = new Map<string, unknown>([
      ["sharedToken", "token"],
      ["serverPort", "8765"],
    ]);
    const { area, removes } = recordingArea(entries);
    const store = createSettingsStore(area);
    await store.clearToken();
    assert.deepEqual(removes, ["sharedToken"]);
    assert.deepEqual(await store.load(), { token: undefined, port: "8765" });
  });
});

interface EventRecorder {
  readonly events: StorageChangeEvents;
  fire(
    changes: Record<string, { newValue?: unknown; oldValue?: unknown }>,
    areaName: string,
  ): void;
  count(): number;
}

/** A StorageChangeEvents whose listeners can be fired by hand. */
function recordingEvents(): EventRecorder {
  const listeners = new Set<StorageChangeListener>();
  return {
    events: {
      addListener: (callback) => {
        listeners.add(callback);
      },
      removeListener: (callback) => {
        listeners.delete(callback);
      },
    },
    fire: (changes, areaName) => {
      for (const callback of [...listeners]) {
        callback(changes, areaName);
      }
    },
    count: () => listeners.size,
  };
}

const resultView: ResultView = { kind: "blocked", reason: "empty-title" };

describe("result view store", () => {
  it("puts, gets and removes a value under the key", async () => {
    const { area } = recordingArea(new Map());
    const store = createResultViewStore(area, recordingEvents().events);
    await store.put("key-1", resultView);
    assert.deepEqual(await store.get("key-1"), resultView);
    await store.remove("key-1");
    assert.equal(await store.get("key-1"), undefined);
  });

  it("watch reports a value newly stored under the key in session", () => {
    const events = recordingEvents();
    const store = createResultViewStore(
      recordingArea(new Map()).area,
      events.events,
    );
    const seen: unknown[] = [];
    store.watch("key-1", (value) => seen.push(value));
    events.fire({ "key-1": { newValue: resultView } }, "session");
    assert.deepEqual(seen, [resultView]);
  });

  it("watch ignores another area, another key, and a removal", () => {
    const events = recordingEvents();
    const store = createResultViewStore(
      recordingArea(new Map()).area,
      events.events,
    );
    const seen: unknown[] = [];
    store.watch("key-1", (value) => seen.push(value));
    events.fire({ "key-1": { newValue: resultView } }, "local");
    events.fire({ "key-2": { newValue: resultView } }, "session");
    events.fire({ "key-1": { oldValue: resultView } }, "session");
    assert.deepEqual(seen, []);
  });

  it("watch stops after the returned function is called", () => {
    const events = recordingEvents();
    const store = createResultViewStore(
      recordingArea(new Map()).area,
      events.events,
    );
    const seen: unknown[] = [];
    const stop = store.watch("key-1", (value) => seen.push(value));
    stop();
    events.fire({ "key-1": { newValue: resultView } }, "session");
    assert.deepEqual(seen, []);
    assert.equal(events.count(), 0);
  });
});
