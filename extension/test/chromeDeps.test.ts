import assert from "node:assert/strict";
import { describe, it } from "node:test";
import vm from "node:vm";

import { JSDOM } from "jsdom";

import {
  createMetadataReader,
  createSelectionReader,
  createSettingsStore,
  type ScriptingApi,
  type StorageArea,
} from "../src/browser/chromeDeps.ts";
import type { PageSelection } from "../src/core/collect.ts";
import type { ServerPort, SharedToken } from "../src/core/settings.ts";

type Injection = {
  readonly target: { readonly tabId: number };
  readonly func: (...args: never[]) => unknown;
  readonly args: readonly unknown[];
};

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
      async executeScript<Args extends unknown[], Result>(injection: {
        readonly target: { readonly tabId: number };
        readonly func: (...args: Args) => Result;
        readonly args: Args;
      }): Promise<Array<{ readonly result?: unknown }>> {
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

const watchUrl = "https://www.youtube.com/watch?v=abc";

async function metadataInjection(): Promise<Injection> {
  const { scripting, injections } = recordingScripting(async () => [
    { result: "{}" },
  ]);
  await createMetadataReader(scripting).read(1, watchUrl);
  const injection = injections[0];
  assert.ok(injection !== undefined);
  return injection;
}

describe("metadata reader", () => {
  it("resolves the JSON text the injected function returned", async () => {
    const { scripting, injections } = recordingScripting(async () => [
      { result: '{"descriptionCollapsed":false}' },
    ]);
    const reader = createMetadataReader(scripting);
    assert.equal(
      await reader.read(3, watchUrl),
      '{"descriptionCollapsed":false}',
    );
    assert.equal(injections[0]?.target.tabId, 3);
    assert.deepEqual(injections[0]?.args, [watchUrl]);
  });

  const malformed: { name: string; results: { result?: unknown }[] }[] = [
    { name: "an empty result list", results: [] },
    { name: "a null result", results: [{ result: null }] },
    { name: "an undefined result", results: [{ result: undefined }] },
    { name: "a number result", results: [{ result: 1 }] },
    { name: "an object result", results: [{ result: {} }] },
  ];
  for (const { name, results } of malformed) {
    it(`rejects ${name}`, async () => {
      const { scripting } = recordingScripting(async () => results);
      await assert.rejects(createMetadataReader(scripting).read(1, watchUrl));
    });
  }

  it("rejects when the injection throws", async () => {
    const { scripting } = recordingScripting(async () => {
      throw new Error("injection denied");
    });
    await assert.rejects(createMetadataReader(scripting).read(1, watchUrl));
  });
});

describe("page metadata injection", () => {
  /** A description expander with the given attribute, snippet and full text. */
  function expander(attributes: string, snippet: string, full: string): string {
    return (
      `<ytd-text-inline-expander id="description-inline-expander"${attributes}>` +
      `<div id="attributed-snippet-text">${snippet}</div>` +
      `<div id="expanded"><yt-attributed-string>${full}</yt-attributed-string></div>` +
      `</ytd-text-inline-expander>`
    );
  }

  /** The watch page body around the given expander. */
  function watchBody(expanderHtml: string): string {
    return (
      `<ytd-watch-metadata>` +
      `<div id="title"><h1><yt-formatted-string>Video title</yt-formatted-string></h1></div>` +
      `<div id="owner"><div id="channel-name"><span id="text">Channel name</span></div></div>` +
      `${expanderHtml}</ytd-watch-metadata>`
    );
  }

  /** A jsdom document wrapping body at the watch URL. */
  function watchDom(body: string): JSDOM {
    return new JSDOM(`<!DOCTYPE html><body>${body}</body>`, { url: watchUrl });
  }

  /** Runs the recorded injection in dom, returning its JSON text or null. */
  function run(
    injection: Injection,
    dom: JSDOM,
    expectedUrl = watchUrl,
  ): string | null {
    const context = {
      document: dom.window.document,
      location: dom.window.location,
    };
    return vm.runInNewContext(
      `(${injection.func.toString()})(${JSON.stringify(expectedUrl)})`,
      context,
    ) as string | null;
  }

  /** The parsed JSON the injection returned for body. */
  async function read(
    body: string,
    expectedUrl = watchUrl,
  ): Promise<Record<string, unknown>> {
    const injection = await metadataInjection();
    const text = run(injection, watchDom(body), expectedUrl);
    assert.ok(text !== null);
    return JSON.parse(text) as Record<string, unknown>;
  }

  it("reads the three values and the collapsed state", async () => {
    const collapsed = await read(
      watchBody(expander("", "Snippet", "Full description")),
    );
    assert.equal(collapsed.title, "Video title");
    assert.equal(collapsed.channelName, "Channel name");
    assert.equal(collapsed.description, "Snippet");
    assert.equal(collapsed.descriptionCollapsed, true);

    const expanded = await read(
      watchBody(expander(" is-expanded", "Snippet", "Full description")),
    );
    assert.equal(expanded.title, "Video title");
    assert.equal(expanded.channelName, "Channel name");
    assert.equal(expanded.description, "Full description");
    assert.equal(expanded.descriptionCollapsed, false);
  });

  it("returns null when the page URL differs from the expected URL", async () => {
    const injection = await metadataInjection();
    const dom = watchDom(watchBody(expander("", "Snippet", "Full")));
    assert.equal(
      run(injection, dom, "https://www.youtube.com/watch?v=other"),
      null,
    );
  });

  it("omits a member whose element is absent", async () => {
    const parsed = await read("<div></div>");
    assert.deepEqual(Object.keys(parsed), ["descriptionCollapsed"]);
    assert.equal(parsed.descriptionCollapsed, false);

    // The expander is present but the element its state selects is absent: the
    // description is omitted while the collapsed flag still comes from
    // is-expanded.
    const absent = [
      {
        name: "collapsed",
        attributes: "",
        children:
          '<div id="expanded"><yt-attributed-string>Full</yt-attributed-string></div>',
        collapsed: true,
      },
      {
        name: "expanded",
        attributes: " is-expanded",
        children: '<div id="attributed-snippet-text">Snippet</div>',
        collapsed: false,
      },
    ];
    for (const { name, attributes, children, collapsed } of absent) {
      const body =
        `<ytd-watch-metadata>` +
        `<ytd-text-inline-expander id="description-inline-expander"${attributes}>${children}` +
        `</ytd-text-inline-expander></ytd-watch-metadata>`;
      const omitted = await read(body);
      assert.equal(omitted.description, undefined, name);
      assert.equal(omitted.descriptionCollapsed, collapsed, name);
    }
  });

  it("chooses the full-text element when is-expanded is set", async () => {
    // is-expanded is present while #expanded is still empty: the expected
    // element is chosen even though it has no content (the snippet beside it
    // is not).
    const open = await read(watchBody(expander(" is-expanded", "Snippet", "")));
    assert.equal(open.description, "");
    assert.equal(open.descriptionCollapsed, false);
  });

  it("does not operate the page", async () => {
    const injection = await metadataInjection();
    const dom = watchDom(watchBody(expander("", "Snippet", "Full")));
    const before = dom.serialize();
    run(injection, dom, watchUrl);
    // The serialized DOM catches mutations. The source scan below is the
    // denylist of page-operating calls the plan enumerates (AC-19), not a proof
    // that no other operation exists.
    assert.equal(dom.serialize(), before);
    const source = injection.func.toString();
    for (const call of [
      ".click(",
      "dispatchEvent",
      ".focus(",
      ".blur(",
      ".submit(",
      "requestSubmit(",
      "scrollIntoView",
      ".value =",
    ]) {
      assert.ok(!source.includes(call), call);
    }
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
