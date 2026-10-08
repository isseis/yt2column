import assert from "node:assert/strict";
import { describe, it } from "node:test";
import vm from "node:vm";

import {
  createSelectionReader,
  createSummaryStore,
  type ScriptingApi,
  type StorageArea,
} from "../src/browser/chromeDeps.ts";
import type { PageSelection } from "../src/core/collect.ts";

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

/** An in-memory StorageArea backed by entries. */
function memoryStorage(entries: Map<string, unknown>): StorageArea {
  return {
    get: async (key) => (entries.has(key) ? { [key]: entries.get(key) } : {}),
    set: async (items) => {
      for (const [key, value] of Object.entries(items)) {
        entries.set(key, value);
      }
    },
    remove: async (key) => {
      entries.delete(key);
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
