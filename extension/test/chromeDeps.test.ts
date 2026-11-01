import assert from "node:assert/strict";
import { describe, it } from "node:test";
import type { TestContext } from "node:test";
import vm from "node:vm";

import { JSDOM } from "jsdom";

import {
  createClipboardWriter,
  createFileSaver,
  createMetadataReader,
  createQueryIdSource,
  createSelectionReader,
  createSendDeps,
  createViewerChannelOpener,
  createViewerTabs,
  createWindowFocus,
  createWorkerSend,
  listenForMessages,
  objectUrlLifetimeMs,
  readUiLanguage,
  type I18nApi,
  type MessageEventsApi,
  type MessageListener,
  type NativeMessagingApi,
  type ObjectUrlApi,
  type ScriptingApi,
  type TabOperationsApi,
  type ViewerChannel,
  type ViewerTabs,
} from "../src/browser/chromeDeps.ts";
import { viewerChannelName } from "../src/core/extensionMessages.ts";
import type {
  AwaitStart,
  ViewerChannelMessage,
} from "../src/core/extensionMessages.ts";
import type { PageSelection } from "../src/core/collect.ts";
import { nativeHostName } from "../src/core/native.ts";

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

interface NativeCall {
  readonly application: string;
  readonly message: object;
}

interface RuntimeRecorder {
  readonly runtime: NativeMessagingApi;
  readonly calls: NativeCall[];
}

/**
 * A runtime whose sendNativeMessage records the call and then runs answer:
 * answer may call back (setting lastError for the callback's duration, as the
 * browser does) or throw instead of calling back.
 */
function recordingRuntime(
  answer: (respond: (response: unknown, lastError?: string) => void) => void,
): RuntimeRecorder {
  const calls: NativeCall[] = [];
  const runtime: { lastError?: { message?: string } | undefined } & Pick<
    NativeMessagingApi,
    "sendNativeMessage"
  > = {
    sendNativeMessage(application, message, callback) {
      calls.push({ application, message });
      answer((response, lastError) => {
        runtime.lastError =
          lastError === undefined ? undefined : { message: lastError };
        try {
          callback(response);
        } finally {
          runtime.lastError = undefined;
        }
      });
    },
  };
  return { runtime, calls };
}

describe("send dependencies", () => {
  const message = {
    videoUrl: "https://www.youtube.com/watch?v=abc",
    transcript: "x",
  };

  it("sends the message to the fixed host once and resolves its reply", async () => {
    const reply = { v: 1, kind: "error", error: "server_timeout" };
    const { runtime, calls } = recordingRuntime((respond) => respond(reply));
    const result = await createSendDeps(runtime).sendNative(message);
    assert.deepEqual(result, { kind: "message", value: reply });
    assert.deepEqual(calls, [{ application: nativeHostName, message }]);
  });

  it("resolves a lastError as failed with its text, whatever the reply", async () => {
    const { runtime } = recordingRuntime((respond) =>
      respond(undefined, "Native host has exited."),
    );
    assert.deepEqual(await createSendDeps(runtime).sendNative(message), {
      kind: "failed",
      lastError: "Native host has exited.",
    });
  });

  it("resolves a lastError without a message as failed", async () => {
    const runtime: NativeMessagingApi = {
      lastError: {},
      sendNativeMessage: (_application, _message, callback) =>
        callback(undefined),
    };
    assert.deepEqual(await createSendDeps(runtime).sendNative(message), {
      kind: "failed",
      lastError: undefined,
    });
  });

  it("resolves a synchronous throw as failed instead of waiting", async () => {
    const { runtime } = recordingRuntime(() => {
      throw new TypeError("Message exceeded maximum allowed size of 64MiB.");
    });
    assert.deepEqual(await createSendDeps(runtime).sendNative(message), {
      kind: "failed",
      lastError: "TypeError: Message exceeded maximum allowed size of 64MiB.",
    });
  });

  it("runs a scheduled callback after the delay unless it is cancelled", async () => {
    const { runtime } = recordingRuntime(() => {});
    const { schedule } = createSendDeps(runtime);
    const ran: string[] = [];
    schedule(50, () => ran.push("kept"));
    const cancel = schedule(50, () => ran.push("cancelled"));
    cancel();
    await new Promise((resolve) => setTimeout(resolve, 5));
    assert.deepEqual(ran, [], "ran before the delay");
    await new Promise((resolve) => setTimeout(resolve, 100));
    assert.deepEqual(ran, ["kept"]);
  });
});

describe("readUiLanguage", () => {
  const cases: { name: string; i18n: I18nApi; want: string | undefined }[] = [
    {
      name: "returns the string the browser reported",
      i18n: { getUILanguage: () => "ja-JP" },
      want: "ja-JP",
    },
    {
      name: "returns undefined when the browser throws",
      i18n: {
        getUILanguage: () => {
          throw new Error("i18n unavailable");
        },
      },
      want: undefined,
    },
    {
      name: "returns undefined when the browser reports a non-string",
      i18n: { getUILanguage: () => 1 },
      want: undefined,
    },
  ];
  for (const { name, i18n, want } of cases) {
    it(name, () => {
      assert.equal(readUiLanguage(i18n), want);
    });
  }
});

const rejectWith = (text: string) => () => Promise.reject(new Error(text));
const throwWith = (text: string) => () => {
  throw new Error(text);
};

describe("runtime message listener", () => {
  type Callback = Parameters<MessageEventsApi["onMessage"]["addListener"]>[0];

  /** Registers listener through listenForMessages and returns the callback the browser got. */
  function registered(listener: MessageListener): Callback {
    const callbacks: Callback[] = [];
    listenForMessages(
      { onMessage: { addListener: (callback) => callbacks.push(callback) } },
      listener,
    );
    assert.equal(callbacks.length, 1);
    return callbacks[0]!;
  }

  it("passes the message, the sender and what the listener returned", () => {
    const seen: unknown[] = [];
    const callback = registered((message, sender) => {
      seen.push(message, sender);
      return true;
    });
    const sender = { id: "x", url: "u" };
    assert.equal(
      callback("m", sender, () => {}),
      true,
    );
    assert.deepEqual(seen, ["m", sender]);
  });

  it("leaves the message unanswered when the listener throws", () => {
    const callback = registered(() => {
      throw new Error("boom");
    });
    assert.equal(
      callback("m", {}, () => {}),
      false,
    );
  });

  it("drops a response the closed sender cannot receive", () => {
    let respond: ((response: unknown) => void) | undefined;
    const callback = registered((_message, _sender, sendResponse) => {
      respond = sendResponse;
      return true;
    });
    const responses: unknown[] = [];
    callback("m", {}, (response) => {
      responses.push(response);
      throw new Error("Attempting to use a disconnected port object");
    });
    assert.doesNotThrow(() => respond?.("r"));
    assert.deepEqual(responses, ["r"]);
  });
});

describe("worker send", () => {
  const message: AwaitStart = { type: "await-start", sentAt: 1 };

  it("resolves the worker's response", async () => {
    const sent: unknown[] = [];
    const send = createWorkerSend({
      sendMessage: async (value) => {
        sent.push(value);
        return "reply";
      },
    });
    assert.deepEqual(await send(message), { ok: true, value: "reply" });
    assert.deepEqual(sent, [message]);
  });

  for (const [name, sendMessage] of [
    ["a rejection", rejectWith("Could not establish connection.")],
    ["a throw", throwWith("Extension context invalidated.")],
  ] as const) {
    it(`resolves ${name} as a failure`, async () => {
      const result = await createWorkerSend({ sendMessage })(message);
      assert.equal(result.ok, false);
    });
  }
});

describe("viewer tabs", () => {
  const tab = (id: number | undefined, windowId = 4): chrome.tabs.Tab =>
    ({ id, windowId }) as chrome.tabs.Tab;

  /** TabOperationsApi whose every method rejects, except those overridden. */
  function tabsApi(overrides: Partial<TabOperationsApi>): TabOperationsApi {
    return {
      create: rejectWith("create"),
      update: rejectWith("update"),
      remove: rejectWith("remove"),
      get: rejectWith("get"),
      getCurrent: rejectWith("getCurrent"),
      ...overrides,
    };
  }

  it("creates the tab with the given place and resolves where it opened", async () => {
    const created: chrome.tabs.CreateProperties[] = [];
    const tabs = createViewerTabs(
      tabsApi({
        create: async (properties) => {
          created.push(properties);
          return tab(9);
        },
      }),
    );
    assert.deepEqual(
      await tabs.create({
        url: "chrome-extension://x/viewer.html?h=a",
        active: false,
        windowId: 4,
        index: 3,
        openerTabId: 2,
      }),
      { ok: true, value: { tabId: 9, windowId: 4 } },
    );
    assert.deepEqual(await tabs.create({ url: "u", active: true }), {
      ok: true,
      value: { tabId: 9, windowId: 4 },
    });
    assert.deepEqual(created, [
      {
        url: "chrome-extension://x/viewer.html?h=a",
        active: false,
        windowId: 4,
        index: 3,
        openerTabId: 2,
      },
      { url: "u", active: true },
    ]);
  });

  it("activates the tab by updating it to active", async () => {
    const updates: unknown[] = [];
    const tabs = createViewerTabs(
      tabsApi({
        update: async (tabId, properties) => {
          updates.push([tabId, properties]);
          return tab(tabId);
        },
      }),
    );
    assert.deepEqual(await tabs.activate(5), { ok: true, value: undefined });
    assert.deepEqual(updates, [[5, { active: true }]]);
  });

  it("removes, gets and finds the current tab", async () => {
    const removed: number[] = [];
    const tabs = createViewerTabs(
      tabsApi({
        remove: async (tabId) => void removed.push(tabId),
        get: async (tabId) => tab(tabId, 6),
        getCurrent: async () => tab(8, 2),
      }),
    );
    assert.deepEqual(await tabs.remove(5), { ok: true, value: undefined });
    assert.deepEqual(removed, [5]);
    assert.deepEqual(await tabs.get(5), {
      ok: true,
      value: { tabId: 5, windowId: 6 },
    });
    assert.deepEqual(await tabs.getCurrent(), {
      ok: true,
      value: { tabId: 8, windowId: 2 },
    });
  });

  it("resolves a page outside a tab as no current tab", async () => {
    const tabs = createViewerTabs(
      tabsApi({ getCurrent: async () => undefined }),
    );
    assert.deepEqual(await tabs.getCurrent(), { ok: true, value: undefined });
  });

  it("resolves a tab without an ID as a failure", async () => {
    const tabs = createViewerTabs(
      tabsApi({
        create: async () => tab(undefined),
        get: async () => tab(undefined),
        getCurrent: async () => tab(undefined),
      }),
    );
    assert.equal((await tabs.create({ url: "u", active: false })).ok, false);
    assert.equal((await tabs.get(1)).ok, false);
    assert.equal((await tabs.getCurrent()).ok, false);
  });

  const calls: {
    name: string;
    call: (tabs: ViewerTabs) => Promise<{ ok: boolean }>;
  }[] = [
    {
      name: "create",
      call: (tabs) => tabs.create({ url: "u", active: false }),
    },
    { name: "activate", call: (tabs) => tabs.activate(1) },
    { name: "remove", call: (tabs) => tabs.remove(1) },
    { name: "get", call: (tabs) => tabs.get(1) },
    { name: "getCurrent", call: (tabs) => tabs.getCurrent() },
  ];
  for (const { name, call } of calls) {
    it(`resolves a rejected ${name} as a failure`, async () => {
      assert.equal((await call(createViewerTabs(tabsApi({})))).ok, false);
    });
    it(`resolves a ${name} that throws as a failure`, async () => {
      const throwing = throwWith(name);
      const api = tabsApi({
        create: throwing,
        update: throwing,
        remove: throwing,
        get: throwing,
        getCurrent: throwing,
      });
      assert.equal((await call(createViewerTabs(api))).ok, false);
    });
  }
});

describe("window focus", () => {
  it("updates the window to focused", async () => {
    const updates: unknown[] = [];
    const focus = createWindowFocus({
      update: async (windowId, info) => void updates.push([windowId, info]),
    });
    assert.deepEqual(await focus(3), { ok: true, value: undefined });
    assert.deepEqual(updates, [[3, { focused: true }]]);
  });

  for (const [name, update] of [
    ["a rejection", rejectWith("No window with id: 3.")],
    ["a throw", throwWith("No window with id: 3.")],
  ] as const) {
    it(`resolves ${name} as a failure`, async () => {
      assert.equal((await createWindowFocus({ update })(3)).ok, false);
    });
  }
});

describe("viewer channel opener", () => {
  const message: ViewerChannelMessage = {
    type: "took-over",
    queryId: "0123456789abcdef0123456789abcdef",
    tabId: 1,
  };

  /** Opens a channel backed by Node's BroadcastChannel, closed when the test ends. */
  function opened(t: TestContext): ViewerChannel {
    const channel = createViewerChannelOpener()();
    assert.ok(channel.ok);
    t.after(() => channel.value.close());
    return channel.value;
  }

  it("delivers to another endpoint of the viewers' channel and not to the poster", async (t) => {
    const poster = opened(t);
    const receiver = opened(t);
    const toPoster: unknown[] = [];
    poster.listen((data) => toPoster.push(data));
    const received = new Promise((resolve) => receiver.listen(resolve));
    assert.equal(poster.post(message), true);
    assert.deepEqual(await received, message);
    assert.deepEqual(toPoster, []);
  });

  it("opens the channel named for the viewers", () => {
    const names: string[] = [];
    const open = createViewerChannelOpener((name) => {
      names.push(name);
      return new BroadcastChannel(name);
    });
    const channel = open();
    assert.ok(channel.ok);
    channel.value.close();
    assert.deepEqual(names, [viewerChannelName]);
  });

  it("stops calling a listener once stopped", async (t) => {
    const poster = opened(t);
    const receiver = opened(t);
    const received: unknown[] = [];
    receiver.listen((data) => received.push(data))();
    const second = new Promise((resolve) => receiver.listen(resolve));
    poster.post(message);
    await second;
    assert.deepEqual(received, []);
  });

  it("reports a post on a closed channel as not posted", () => {
    const channel = createViewerChannelOpener()();
    assert.ok(channel.ok);
    channel.value.close();
    assert.equal(channel.value.post(message), false);
  });

  it("resolves a channel that cannot be opened as a failure", () => {
    const open = createViewerChannelOpener(() => {
      throw new Error("BroadcastChannel is not defined");
    });
    assert.equal(open().ok, false);
  });
});

describe("query IDs", () => {
  it("writes 16 random bytes as 32 lowercase hexadecimal characters", () => {
    const sizes: number[] = [];
    const next = createQueryIdSource({
      getRandomValues: (array) => {
        sizes.push(array.length);
        array.set(Array.from({ length: array.length }, (_, i) => i * 17));
        return array;
      },
    });
    assert.equal(next(), "00112233445566778899aabbccddeeff");
    assert.deepEqual(sizes, [16]);
  });

  it("draws a new ID each time from crypto by default", () => {
    const next = createQueryIdSource();
    const first = next();
    assert.match(first, /^[0-9a-f]{32}$/);
    assert.notEqual(next(), first);
  });
});

describe("clipboard writer", () => {
  it("writes the text as given", async () => {
    const written: string[] = [];
    const write = createClipboardWriter({
      writeText: async (text) => void written.push(text),
    });
    assert.deepEqual(await write("# Title\n"), { ok: true, value: undefined });
    assert.deepEqual(written, ["# Title\n"]);
  });

  for (const [name, writeText] of [
    ["a rejection", rejectWith("Document is not focused.")],
    ["a throw", throwWith("clipboard unavailable")],
  ] as const) {
    it(`resolves ${name} as a failure`, async () => {
      assert.equal((await createClipboardWriter({ writeText })("x")).ok, false);
    });
  }
});

describe("file saver", () => {
  /** A saver on a jsdom document that records each anchor clicked and each URL made or revoked. */
  function recordingSaver(urls: Partial<ObjectUrlApi> = {}) {
    const doc = new JSDOM("<!doctype html><body></body>").window.document;
    const clicks: { href: string; download: string; attached: boolean }[] = [];
    const created = doc.createElement.bind(doc);
    doc.createElement = ((name: string) => {
      const element = created(name);
      element.addEventListener("click", (event) => {
        event.preventDefault();
        const anchor = element as HTMLAnchorElement;
        clicks.push({
          href: anchor.getAttribute("href") ?? "",
          download: anchor.download,
          attached: anchor.isConnected,
        });
      });
      return element;
    }) as typeof doc.createElement;
    const blobs: Blob[] = [];
    const revoked: string[] = [];
    const scheduled: { ms: number; run: () => void }[] = [];
    const save = createFileSaver(
      doc,
      {
        createObjectURL: (blob) => {
          blobs.push(blob);
          return `blob:chrome-extension://x/${blobs.length}`;
        },
        revokeObjectURL: (url) => void revoked.push(url),
        ...urls,
      },
      (ms, run) => {
        scheduled.push({ ms, run });
        return () => {};
      },
    );
    return { save, clicks, blobs, revoked, scheduled };
  }

  it("clicks a detached download anchor for a Blob of the text's UTF-8", async () => {
    const { save, clicks, blobs } = recordingSaver();
    const text = "# Caf\u00e9\r\nbody \u{1F600}\n";
    assert.deepEqual(save("title.md", text), { ok: true, value: undefined });
    assert.deepEqual(clicks, [
      {
        href: "blob:chrome-extension://x/1",
        download: "title.md",
        attached: false,
      },
    ]);
    assert.equal(blobs[0]?.type, "text/markdown;charset=utf-8");
    assert.deepEqual(
      new Uint8Array(await blobs[0]!.arrayBuffer()),
      new TextEncoder().encode(text),
    );
  });

  it("revokes the object URL only after its lifetime", () => {
    const { save, revoked, scheduled } = recordingSaver();
    save("a.md", "x");
    assert.deepEqual(revoked, []);
    assert.deepEqual(
      scheduled.map(({ ms }) => ms),
      [objectUrlLifetimeMs],
    );
    scheduled[0]?.run();
    assert.deepEqual(revoked, ["blob:chrome-extension://x/1"]);
  });

  it("resolves a URL that cannot be made as a failure, clicking nothing", () => {
    const { save, clicks } = recordingSaver({
      createObjectURL: throwWith("out of memory"),
    });
    assert.equal(save("a.md", "x").ok, false);
    assert.deepEqual(clicks, []);
  });
});
