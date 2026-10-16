import assert from "node:assert/strict";
import { describe, it } from "node:test";

import type {
  CollectedInput,
  CollectOutcome,
  PageSelection,
  SelectionReader,
} from "../src/core/collect.ts";
import type { RejectionMessage } from "../src/core/messages.ts";
import type { CollectedSummary, ResultView } from "../src/core/resultView.ts";
import type { RequestId, SendOutcome } from "../src/core/response.ts";
import {
  blockMessage,
  unreadableResultMessage,
} from "../src/core/sendMessages.ts";
import type { SettingsReader } from "../src/core/settings.ts";
import { summarize } from "../src/core/summary.ts";
import {
  launchContextFromTab,
  runMenuLaunch,
  runPopupLaunch,
  runPopupSend,
  runResultWindow,
} from "../src/launch.ts";
import type {
  MenuLaunchDeps,
  PopupLaunchDeps,
  PopupSendDeps,
  ResultWindowDeps,
} from "../src/launch.ts";
import type { FetchFn, FetchResponse, SendDeps } from "../src/send.ts";
import { container } from "./helpers/dom.ts";
import { recordingLogger } from "./helpers/logger.ts";
import type { RecordingLogger } from "./helpers/logger.ts";
import {
  identifiableToken,
  fixedSettings,
  failingSettings,
} from "./helpers/settings.ts";
import {
  recordingFetch,
  rejectingFetch,
  testSendDeps,
  textResponse,
} from "./helpers/fetch.ts";

const watchUrl = "https://www.youtube.com/watch?v=abc";
const malicious = "<img src=x onerror=alert(1)>";
const requestId = "0123456789abcdef" as RequestId;
const acceptedBody = JSON.stringify({ status: "accepted", requestId });
const acceptedOutcome: SendOutcome = { kind: "accepted", requestId };

/** Builds a chrome.tabs.Tab with the fields the code reads. */
function fakeTab(overrides: Partial<chrome.tabs.Tab> = {}): chrome.tabs.Tab {
  return {
    id: 7,
    url: watchUrl,
    title: "A video",
    ...overrides,
  } as chrome.tabs.Tab;
}

/** Builds context-menu click data with the fields the code reads. */
function menuInfo(
  overrides: Partial<chrome.contextMenus.OnClickData> = {},
): chrome.contextMenus.OnClickData {
  return {
    menuItemId: "yt2column",
    editable: false,
    frameId: 0,
    pageUrl: watchUrl,
    ...overrides,
  } as chrome.contextMenus.OnClickData;
}

/** Builds a page selection; the document URL defaults to the watch URL. */
function page(text: string, documentUrl = watchUrl): PageSelection {
  return { text, documentUrl };
}

/** A SelectionReader that returns the given selection and cannot fail. */
function readerReturning(selection: PageSelection): SelectionReader {
  return { read: async () => selection };
}

/** The collected display summary from a selection. */
function collectedSummary(selection = "selection"): CollectedSummary {
  const summary = summarize({
    kind: "collected",
    input: { selection, url: watchUrl, title: "A video" } as CollectedInput,
  });
  assert.ok(summary.kind === "collected");
  return summary;
}

/** Settings with a valid token and no stored port (the default 8765). */
function readySettings() {
  return fixedSettings({ token: identifiableToken, port: undefined });
}

/** A recording fetch that returns an accepted response. */
function acceptedFetch() {
  return recordingFetch(async () => textResponse(202, acceptedBody));
}

/** A fetch whose response resolves only when the test calls resolve. */
function deferredFetch(): {
  fetchFn: FetchFn;
  resolve(response: FetchResponse): void;
} {
  let settle: ((response: FetchResponse) => void) | undefined;
  const pending = new Promise<FetchResponse>((resolve) => {
    settle = resolve;
  });
  return {
    fetchFn: async () => pending,
    resolve: (response) => settle?.(response),
  };
}

/** Waits for the queued microtasks and one macrotask to settle. */
async function flush(): Promise<void> {
  await new Promise((resolve) => setTimeout(resolve, 0));
}

interface FakeStore {
  readonly store: ResultWindowDeps["store"];
  readonly puts: { key: string; view: ResultView }[];
  readonly removes: string[];
  seed(key: string, value: unknown): void;
  fire(key: string, value: unknown): void;
  watching(key: string): boolean;
}

/** An in-memory ResultViewStore that records calls and can be made to fail. */
function fakeStore(
  options: {
    putFails?: boolean;
    putFailsOnSent?: boolean;
    getFails?: boolean;
    removeFails?: boolean;
  } = {},
): FakeStore {
  const puts: { key: string; view: ResultView }[] = [];
  const removes: string[] = [];
  const entries = new Map<string, unknown>();
  const watchers = new Map<string, Set<(value: unknown) => void>>();
  return {
    store: {
      async put(key, view) {
        if (
          options.putFails === true ||
          (options.putFailsOnSent === true && view.kind === "sent")
        ) {
          throw new Error("put failed");
        }
        puts.push({ key, view });
        entries.set(key, view);
      },
      async get(key) {
        if (options.getFails === true) {
          throw new Error("get failed");
        }
        return entries.get(key);
      },
      async remove(key) {
        removes.push(key);
        if (options.removeFails === true) {
          throw new Error("remove failed");
        }
        entries.delete(key);
      },
      watch(key, listener) {
        const set = watchers.get(key) ?? new Set();
        set.add(listener);
        watchers.set(key, set);
        return () => {
          set.delete(listener);
        };
      },
    },
    puts,
    removes,
    seed: (key, value) => {
      entries.set(key, value);
    },
    fire: (key, value) => {
      for (const listener of watchers.get(key) ?? []) {
        listener(value);
      }
    },
    watching: (key) => (watchers.get(key)?.size ?? 0) > 0,
  };
}

interface FakeSchedule {
  schedule(ms: number, callback: () => void): () => void;
  fire(): void;
  delayMs(): number | undefined;
  pending(): boolean;
}

/** A schedule that records the delay and fires only when the test calls fire. */
function fakeSchedule(): FakeSchedule {
  let pending: (() => void) | undefined;
  let delay: number | undefined;
  return {
    schedule: (ms, callback) => {
      delay = ms;
      pending = callback;
      return () => {
        pending = undefined;
      };
    },
    fire: () => {
      const callback = pending;
      pending = undefined;
      callback?.();
    },
    delayMs: () => delay,
    pending: () => pending !== undefined,
  };
}

interface MenuHarness {
  readonly deps: MenuLaunchDeps;
  readonly recorder: RecordingLogger;
  readonly store: FakeStore;
  readonly windows: string[];
  readonly badge: { signals: number; clears: number };
  readonly fetch: ReturnType<typeof acceptedFetch>;
}

interface MenuOptions {
  readonly reader?: SelectionReader;
  readonly settings?: SettingsReader;
  readonly send?: SendDeps;
  readonly store?: FakeStore;
  readonly newKey?: () => string;
  readonly openResultWindow?: (key: string) => Promise<void>;
}

/** Builds menu-path dependencies with recording fakes and visible counters. */
function menuHarness(options: MenuOptions = {}): MenuHarness {
  const recorder = recordingLogger();
  const store = options.store ?? fakeStore();
  const windows: string[] = [];
  const badge = { signals: 0, clears: 0 };
  const fetch = acceptedFetch();
  const deps: MenuLaunchDeps = {
    reader: options.reader ?? readerReturning(page("selection")),
    settings: options.settings ?? readySettings().reader,
    send: options.send ?? testSendDeps(fetch.fetchFn),
    store: store.store,
    newKey: options.newKey ?? (() => "key-1"),
    now: () => 1000,
    openResultWindow:
      options.openResultWindow ??
      (async (key) => {
        windows.push(key);
      }),
    signalDisplayFailure: async () => {
      badge.signals += 1;
    },
    clearDisplayFailure: async () => {
      badge.clears += 1;
    },
    log: recorder.log,
  };
  return { deps, recorder, store, windows, badge, fetch };
}

interface PopupHarness {
  readonly deps: PopupLaunchDeps;
  readonly recorder: RecordingLogger;
}

/** Builds popup-launch dependencies with a recording logger and fake tab. */
function popupHarness(overrides: Partial<PopupLaunchDeps> = {}): PopupHarness {
  const recorder = recordingLogger();
  const deps: PopupLaunchDeps = {
    reader: readerReturning(page("selection")),
    activeTab: async () => fakeTab(),
    log: recorder.log,
    ...overrides,
  };
  return { deps, recorder };
}

/** The outcome the path logged for the developer. */
function loggedOutcome(recorder: RecordingLogger): CollectOutcome {
  const entry = recorder.infos[0];
  assert.ok(entry !== undefined, "expected an info log");
  return (entry.data as { outcome: CollectOutcome }).outcome;
}

/** Asserts the container text shows the message and each of its steps. */
function expectMessage(root: HTMLElement, message: RejectionMessage): void {
  const text = root.textContent ?? "";
  assert.ok(text.includes(message.text), message.text);
  for (const step of message.steps) {
    assert.ok(text.includes(step), step);
  }
}

/**
 * Dispatches a click on the popup send button and waits for the handler to
 * settle. dispatching the event (rather than calling click()) exercises the
 * handler even when the button is disabled, so the state guard is tested and
 * not merely the disabled attribute.
 */
async function clickSend(container: HTMLElement): Promise<void> {
  const button = container.querySelector(".send-button");
  assert.ok(button !== null);
  const view = container.ownerDocument.defaultView;
  assert.ok(view !== null);
  button.dispatchEvent(new view.Event("click"));
  await flush();
}

describe("launchContextFromTab", () => {
  it("returns undefined without a tab", () => {
    assert.equal(launchContextFromTab(undefined), undefined);
  });

  it("returns undefined without a tab id", () => {
    const tab = fakeTab();
    delete tab.id;
    assert.equal(launchContextFromTab(tab), undefined);
  });

  it("copies the id, url and title verbatim", () => {
    const tab = fakeTab({ id: 9, url: watchUrl, title: "  A title  " });
    assert.deepEqual(launchContextFromTab(tab), {
      tabId: 9,
      url: watchUrl,
      title: "  A title  ",
    });
  });
});

describe("launch paths", () => {
  it("collects the same input through the menu and popup paths", async () => {
    const selection = " 00:00\ncaption line\n\n";
    const title = "  A title  ";
    const reader = readerReturning(page(selection));
    const tab = fakeTab({ id: 7, url: watchUrl, title });

    const popup = popupHarness({ reader, activeTab: async () => tab });
    const popupOutcome = await runPopupLaunch(container(), popup.deps);

    const menu = menuHarness({ reader });
    await runMenuLaunch(menuInfo(), tab, menu.deps);

    assert.ok(popupOutcome.kind === "collected");
    assert.equal(popupOutcome.input.selection, selection);
    assert.equal(popupOutcome.input.title, title);
    assert.deepEqual(loggedOutcome(menu.recorder), popupOutcome);
    assert.deepEqual(menu.store.puts[0]?.view, {
      kind: "sending",
      summary: summarize(popupOutcome),
    });
  });

  it("sends the same body and classification through both paths", async () => {
    const selection = " 00:00\ncaption line\n\n";
    const title = "  A title  ";
    const reader = readerReturning(page(selection));
    const tab = fakeTab({ id: 7, url: watchUrl, title });

    const menuFetch = acceptedFetch();
    const menu = menuHarness({ reader, send: testSendDeps(menuFetch.fetchFn) });
    await runMenuLaunch(menuInfo(), tab, menu.deps);
    const menuBody = menuFetch.calls[0]?.init.body;
    assert.equal(typeof menuBody, "string");
    assert.deepEqual(JSON.parse(menuBody as string), {
      videoUrl: watchUrl,
      transcript: selection,
    });

    const popupFetch = acceptedFetch();
    const outcome = await runPopupLaunch(
      container(),
      popupHarness({ reader, activeTab: async () => tab }).deps,
    );
    const popupRoot = container();
    await runPopupSend(popupRoot, outcome, {
      settings: readySettings().reader,
      send: testSendDeps(popupFetch.fetchFn),
      now: () => 1000,
      log: recordingLogger().log,
    });
    await clickSend(popupRoot);

    assert.equal(popupFetch.calls[0]?.init.body, menuBody);
    const menuSent = menu.store.puts[1]?.view;
    assert.ok(menuSent?.kind === "sent");
    assert.deepEqual(menuSent.outcome, acceptedOutcome);
    assert.equal(
      popupRoot.querySelector(".send-request-id")?.textContent,
      requestId,
    );
  });

  it("does not read settings or send for a rejected collection", async () => {
    const settings = failingSettings(new Error("must not load"));
    const menu = menuHarness({
      reader: readerReturning(page("text")),
      settings: settings.reader,
    });
    const tab = fakeTab({ url: "https://example.com/" });
    await runMenuLaunch(menuInfo(), tab, menu.deps);
    assert.equal(settings.loadCalls(), 0);
    assert.equal(menu.fetch.calls.length, 0);
    assert.deepEqual(menu.store.puts[0]?.view, {
      kind: "blocked",
      reason: "not-watch-page",
    });
  });

  it("treats a missing tab as collection-failed in both paths", async () => {
    const reader = readerReturning(page("text"));
    const popup = popupHarness({ reader, activeTab: async () => undefined });
    const popupOutcome = await runPopupLaunch(container(), popup.deps);
    assert.equal(popupOutcome.kind, "rejected");
    assert.equal(popupOutcome.reason, "collection-failed");

    const menu = menuHarness({ reader });
    await runMenuLaunch(menuInfo(), undefined, menu.deps);
    assert.equal(loggedOutcome(menu.recorder).kind, "rejected");
  });

  it("treats a failing activeTab as collection-failed without rejecting", async () => {
    const popup = popupHarness({
      activeTab: async () => {
        throw new Error("query failed");
      },
    });
    const outcome = await runPopupLaunch(container(), popup.deps);
    assert.equal(outcome.kind, "rejected");
    assert.equal(outcome.reason, "collection-failed");
    assert.equal(popup.recorder.errors.length, 1);
  });

  it("ignores info.selectionText and reads the selection itself", async () => {
    const menu = menuHarness({
      reader: readerReturning(page("actual selection")),
    });
    await runMenuLaunch(
      menuInfo({ selectionText: "wrong" }),
      fakeTab(),
      menu.deps,
    );
    const outcome = loggedOutcome(menu.recorder);
    assert.ok(outcome.kind === "collected");
    assert.equal(outcome.input.selection, "actual selection");
  });

  it("keeps untrusted text out of the fixed log labels", async () => {
    const menu = menuHarness({
      reader: readerReturning(page(`${malicious}\n`)),
    });
    await runMenuLaunch(menuInfo(), fakeTab(), menu.deps);
    assert.equal(menu.recorder.infos[0]?.label, "yt2column menu launch");
  });

  it("clears the badge at the start of the menu path", async () => {
    const menu = menuHarness();
    await runMenuLaunch(menuInfo(), fakeTab(), menu.deps);
    assert.equal(menu.badge.clears, 1);
  });

  it("signals the badge and does not throw when the key cannot be created", async () => {
    const menu = menuHarness({
      newKey: () => {
        throw new Error("no key");
      },
    });
    await runMenuLaunch(menuInfo(), fakeTab(), menu.deps);
    assert.deepEqual(menu.windows, []);
    assert.equal(menu.badge.signals, 1);
    assert.equal(menu.recorder.errors.length, 1);
  });

  it("does not open a window and signals the badge when put fails", async () => {
    const store = fakeStore({ putFails: true });
    const menu = menuHarness({ store });
    await runMenuLaunch(menuInfo(), fakeTab(), menu.deps);
    assert.deepEqual(menu.windows, []);
    assert.deepEqual(store.removes, []);
    assert.equal(menu.badge.signals, 1);
  });

  it("does not send when the result window cannot be opened", async () => {
    const menu = menuHarness({
      openResultWindow: async () => {
        throw new Error("window failed");
      },
    });
    await runMenuLaunch(menuInfo(), fakeTab(), menu.deps);
    assert.deepEqual(menu.store.removes, ["key-1"]);
    assert.equal(menu.badge.signals, 1);
    assert.equal(menu.fetch.calls.length, 0);
  });

  it("does not reject when the window and the removal both fail", async () => {
    const store = fakeStore({ removeFails: true });
    const menu = menuHarness({
      store,
      openResultWindow: async () => {
        throw new Error("window failed");
      },
    });
    await runMenuLaunch(menuInfo(), fakeTab(), menu.deps);
    assert.deepEqual(store.removes, ["key-1"]);
    assert.equal(menu.badge.signals, 1);
    assert.equal(menu.recorder.errors.length, 2);
  });
});

describe("menu send", () => {
  it("sends once and stores the sending then the sent view", async () => {
    const menu = menuHarness();
    await runMenuLaunch(menuInfo(), fakeTab(), menu.deps);
    assert.deepEqual(
      menu.store.puts.map((put) => put.view.kind),
      ["sending", "sent"],
    );
    assert.equal(menu.fetch.calls.length, 1);
    const sent = menu.store.puts[1]?.view;
    assert.ok(sent?.kind === "sent");
    assert.deepEqual(sent.outcome, acceptedOutcome);
  });

  it("stores an unknown result without its cause", async () => {
    const cause = new TypeError("Failed to fetch");
    const fetch = rejectingFetch(cause);
    const menu = menuHarness({ send: testSendDeps(fetch.fetchFn) });
    await runMenuLaunch(menuInfo(), fakeTab(), menu.deps);
    const sent = menu.store.puts[1]?.view;
    assert.ok(sent?.kind === "sent");
    assert.deepEqual(sent.outcome, { kind: "unknown" });
    assert.equal(menu.recorder.errors.length, 1);
  });

  const blocks: { reason: string; settings: SettingsReader; error: boolean }[] =
    [
      {
        reason: "settings-unreadable",
        settings: failingSettings(new Error("storage failed")).reader,
        error: true,
      },
      {
        reason: "token-missing",
        settings: fixedSettings({ token: undefined, port: undefined }).reader,
        error: false,
      },
      {
        reason: "port-invalid",
        settings: fixedSettings({ token: identifiableToken, port: "80" })
          .reader,
        error: false,
      },
      {
        reason: "body-too-large",
        settings: readySettings().reader,
        error: false,
      },
    ];
  for (const { reason, settings, error } of blocks) {
    it(`blocks ${reason} without sending`, async () => {
      const reader =
        reason === "body-too-large"
          ? readerReturning(page("a".repeat(4 * 1024 * 1024 + 1)))
          : readerReturning(page("selection"));
      const menu = menuHarness({ reader, settings });
      await runMenuLaunch(menuInfo(), fakeTab(), menu.deps);
      assert.deepEqual(menu.store.puts[0]?.view, { kind: "blocked", reason });
      assert.equal(menu.fetch.calls.length, 0);
      assert.deepEqual(menu.windows, ["key-1"]);
      if (error) {
        assert.equal(menu.recorder.errors.length, 1);
      } else {
        assert.equal(menu.recorder.infos.length, 2);
      }
    });
  }

  it("signals the badge when the blocked view cannot be stored", async () => {
    const store = fakeStore({ putFails: true });
    const menu = menuHarness({
      store,
      settings: fixedSettings({ token: undefined, port: undefined }).reader,
    });
    await runMenuLaunch(menuInfo(), fakeTab(), menu.deps);
    assert.deepEqual(menu.windows, []);
    assert.equal(menu.badge.signals, 1);
    assert.equal(menu.recorder.errors.length, 1);
  });

  it("logs and signals the badge when the sent view cannot be stored", async () => {
    const store = fakeStore({ putFailsOnSent: true });
    const menu = menuHarness({ store });
    await runMenuLaunch(menuInfo(), fakeTab(), menu.deps);
    assert.equal(menu.fetch.calls.length, 1);
    assert.equal(menu.badge.signals, 1);
    assert.equal(menu.recorder.errors.length, 1);
  });
});

describe("popup send", () => {
  const deps = (
    overrides: Partial<PopupSendDeps> = {},
  ): { deps: PopupSendDeps; recorder: RecordingLogger } => {
    const recorder = recordingLogger();
    return {
      deps: {
        settings: readySettings().reader,
        send: testSendDeps(acceptedFetch().fetchFn),
        now: () => 1000,
        log: recorder.log,
        ...overrides,
      },
      recorder,
    };
  };

  const collected = (): CollectOutcome => ({
    kind: "collected",
    input: {
      selection: "selection",
      url: watchUrl,
      title: "A video",
    } as CollectedInput,
  });

  it("shows a send button and does not send until it is clicked", async () => {
    const fetch = acceptedFetch();
    const { deps: popupDeps } = deps({ send: testSendDeps(fetch.fetchFn) });
    const root = container();
    await runPopupSend(root, collected(), popupDeps);
    assert.ok(root.querySelector(".send-button") !== null);
    assert.equal(fetch.calls.length, 0);
  });

  it("sends once, disables the button and shows the result", async () => {
    const fetch = acceptedFetch();
    const { deps: popupDeps } = deps({ send: testSendDeps(fetch.fetchFn) });
    const root = container();
    await runPopupSend(root, collected(), popupDeps);
    await clickSend(root);
    assert.equal(fetch.calls.length, 1);
    const button = root.querySelector(".send-button");
    assert.ok(button instanceof Object && "disabled" in button);
    assert.equal((button as HTMLButtonElement).disabled, true);
    assert.equal(
      root.querySelector(".send-request-id")?.textContent,
      requestId,
    );
  });

  it("sends only once when the button is clicked repeatedly", async () => {
    const fetch = acceptedFetch();
    const { deps: popupDeps } = deps({ send: testSendDeps(fetch.fetchFn) });
    const root = container();
    await runPopupSend(root, collected(), popupDeps);
    await clickSend(root);
    await clickSend(root);
    assert.equal(fetch.calls.length, 1);
  });

  it("shows the sending notice and a disabled button while sending", async () => {
    const deferred = deferredFetch();
    const { deps: popupDeps } = deps({ send: testSendDeps(deferred.fetchFn) });
    const root = container();
    await runPopupSend(root, collected(), popupDeps);
    const button = root.querySelector(".send-button");
    assert.ok(button instanceof Object && "click" in button);
    (button as HTMLButtonElement).click();
    await flush();
    assert.equal((button as HTMLButtonElement).disabled, true);
    const text = root.textContent ?? "";
    assert.ok(text.includes("close"), text);
    assert.ok(text.includes("Slack"), text);
    deferred.resolve(textResponse(202, acceptedBody));
    await flush();
    assert.equal(
      root.querySelector(".send-request-id")?.textContent,
      requestId,
    );
  });

  it("shows a block reason and no button for a preflight block", async () => {
    const { deps: popupDeps } = deps({
      settings: fixedSettings({ token: undefined, port: undefined }).reader,
    });
    const root = container();
    await runPopupSend(root, collected(), popupDeps);
    assert.equal(root.querySelector(".send-button"), null);
    expectMessage(root, blockMessage("token-missing"));
  });

  it("renders nothing and does not read settings for a rejected collection", async () => {
    const settings = failingSettings(new Error("must not load"));
    const { deps: popupDeps } = deps({ settings: settings.reader });
    const root = container();
    root.append(root.ownerDocument.createElement("p"));
    await runPopupSend(
      root,
      { kind: "rejected", reason: "empty-selection" },
      popupDeps,
    );
    assert.equal(root.childNodes.length, 0);
    assert.equal(settings.loadCalls(), 0);
  });

  const popupBlocks: {
    reason: "settings-unreadable" | "port-invalid";
    settings: SettingsReader;
  }[] = [
    {
      reason: "settings-unreadable",
      settings: failingSettings(new Error("storage failed")).reader,
    },
    {
      reason: "port-invalid",
      settings: fixedSettings({ token: identifiableToken, port: "80" }).reader,
    },
  ];
  for (const { reason, settings } of popupBlocks) {
    it(`shows the ${reason} reason and no button without sending`, async () => {
      const fetch = acceptedFetch();
      const { deps: popupDeps } = deps({
        settings,
        send: testSendDeps(fetch.fetchFn),
      });
      const root = container();
      await runPopupSend(root, collected(), popupDeps);
      assert.equal(root.querySelector(".send-button"), null);
      expectMessage(root, blockMessage(reason));
      assert.equal(fetch.calls.length, 0);
    });
  }
});

describe("result window", () => {
  const sentView = (outcome: SendOutcome): ResultView => ({
    kind: "sent",
    summary: collectedSummary(),
    outcome,
  });

  function resultDeps(
    store: FakeStore,
    overrides: Partial<ResultWindowDeps> = {},
  ): {
    deps: ResultWindowDeps;
    recorder: RecordingLogger;
    schedule: FakeSchedule;
  } {
    const recorder = recordingLogger();
    const schedule = fakeSchedule();
    return {
      deps: {
        store: store.store,
        schedule: schedule.schedule,
        log: recorder.log,
        ...overrides,
      },
      recorder,
      schedule,
    };
  }

  it("shows a sending view, then the sent result on a notification", async () => {
    const store = fakeStore();
    const view = sentView(acceptedOutcome);
    store.seed("key-1", { kind: "sending", summary: collectedSummary() });
    const { deps, schedule } = resultDeps(store);
    const root = container();
    const run = runResultWindow(root, "#key-1", deps);
    await flush();
    assert.ok((root.textContent ?? "").includes("Sending the article"));
    store.fire("key-1", view);
    await run;
    assert.equal(
      root.querySelector(".send-request-id")?.textContent,
      requestId,
    );
    assert.deepEqual(store.removes, ["key-1"]);
    assert.equal(store.watching("key-1"), false);
    assert.equal(schedule.delayMs(), 35_000);
    // The wait-limit reservation is cancelled on reaching Final.
    assert.equal(schedule.pending(), false);
  });

  it("keeps the sent result when the get resolves after the notification", async () => {
    const store = fakeStore();
    store.seed("key-1", { kind: "sending", summary: collectedSummary() });
    const { deps } = resultDeps(store);
    const root = container();
    const run = runResultWindow(root, "#key-1", deps);
    store.fire("key-1", sentView(acceptedOutcome));
    await run;
    assert.equal(
      root.querySelector(".send-request-id")?.textContent,
      requestId,
    );
  });

  it("ignores a get rejection that arrives after the final view", async () => {
    const store = fakeStore();
    let rejectGet: ((reason: unknown) => void) | undefined;
    const deferred: ResultWindowDeps["store"] = {
      ...store.store,
      get: () =>
        new Promise((_resolve, reject) => {
          rejectGet = reject;
        }),
    };
    const { deps, recorder, schedule } = resultDeps(store);
    const root = container();
    const run = runResultWindow(root, "#key-1", { ...deps, store: deferred });
    store.fire("key-1", sentView(acceptedOutcome));
    rejectGet?.(new Error("get failed"));
    await run;
    assert.equal(
      root.querySelector(".send-request-id")?.textContent,
      requestId,
    );
    assert.equal(recorder.errors.length, 0);
    assert.equal(schedule.pending(), false);
  });

  it("shows a blocked view and removes the key", async () => {
    const store = fakeStore();
    store.seed("key-1", { kind: "blocked", reason: "token-missing" });
    const { deps } = resultDeps(store);
    const root = container();
    await runResultWindow(root, "#key-1", deps);
    expectMessage(root, blockMessage("token-missing"));
    assert.deepEqual(store.removes, ["key-1"]);
  });

  it("logs a failure when the key cannot be removed at Final", async () => {
    const store = fakeStore({ removeFails: true });
    store.seed("key-1", { kind: "blocked", reason: "token-missing" });
    const { deps, recorder } = resultDeps(store);
    const root = container();
    await runResultWindow(root, "#key-1", deps);
    await flush();
    expectMessage(root, blockMessage("token-missing"));
    assert.equal(recorder.errors.length, 1);
  });

  it("shows the unknown display at the wait limit and logs it", async () => {
    const store = fakeStore();
    store.seed("key-1", { kind: "sending", summary: collectedSummary() });
    const { deps, recorder, schedule } = resultDeps(store);
    const root = container();
    const run = runResultWindow(root, "#key-1", deps);
    await flush();
    schedule.fire();
    await run;
    assert.ok((root.textContent ?? "").includes("Slack"));
    assert.deepEqual(store.removes, ["key-1"]);
    assert.equal(recorder.errors.length, 1);
  });

  it("shows the unreadable notice when get fails", async () => {
    const store = fakeStore({ getFails: true });
    const { deps, recorder } = resultDeps(store);
    const root = container();
    await runResultWindow(root, "#key-1", deps);
    expectMessage(root, unreadableResultMessage);
    assert.equal(recorder.errors.length, 1);
  });

  it("shows the unreadable notice when get returns no value", async () => {
    const store = fakeStore();
    const { deps, recorder } = resultDeps(store);
    const root = container();
    await runResultWindow(root, "#missing", deps);
    expectMessage(root, unreadableResultMessage);
    assert.equal(recorder.errors.length, 1);
  });

  it("shows the unreadable notice when the stored shape is wrong", async () => {
    const store = fakeStore();
    store.seed("key-1", { kind: "bogus" });
    const { deps, recorder } = resultDeps(store);
    const root = container();
    await runResultWindow(root, "#key-1", deps);
    expectMessage(root, unreadableResultMessage);
    assert.equal(recorder.errors.length, 1);
  });

  it("shows the unreadable notice without a key and does not touch the store", async () => {
    const store = fakeStore();
    const { deps } = resultDeps(store);
    const root = container();
    await runResultWindow(root, "", deps);
    expectMessage(root, unreadableResultMessage);
    assert.deepEqual(store.removes, []);
    assert.equal(store.watching(""), false);
  });

  it("ignores events after the final view", async () => {
    const store = fakeStore();
    const view = sentView(acceptedOutcome);
    store.seed("key-1", view);
    const { deps, schedule } = resultDeps(store);
    const root = container();
    await runResultWindow(root, "#key-1", deps);
    store.fire("key-1", { kind: "sending", summary: collectedSummary() });
    schedule.fire();
    assert.equal(
      root.querySelector(".send-request-id")?.textContent,
      requestId,
    );
    assert.deepEqual(store.removes, ["key-1"]);
  });
});

describe("token never leaks", () => {
  it("keeps the token out of the logs, the store and the display", async () => {
    const menu = menuHarness();
    await runMenuLaunch(menuInfo(), fakeTab(), menu.deps);
    const popup = popupHarness();
    const root = container();
    const outcome = await runPopupLaunch(root, popup.deps);
    const popupSend = {
      settings: readySettings().reader,
      send: testSendDeps(acceptedFetch().fetchFn),
      now: () => 1000,
      log: recordingLogger().log,
    };
    await runPopupSend(root, outcome, popupSend);
    await clickSend(root);

    const serialized = JSON.stringify({
      menuInfos: menu.recorder.infos,
      menuErrors: menu.recorder.errors,
      popupInfos: popup.recorder.infos,
      popupErrors: popup.recorder.errors,
      storePuts: menu.store.puts,
      text: root.textContent,
    });
    assert.equal(serialized.includes(identifiableToken), false);
    assert.equal(serialized.includes(identifiableToken.slice(0, 4)), false);
    assert.equal(serialized.includes(identifiableToken.slice(-4)), false);
  });
});

describe("display", () => {
  it("renders the popup summary as text, not elements", async () => {
    const root = container();
    await runPopupLaunch(
      root,
      popupHarness({ reader: readerReturning(page(malicious)) }).deps,
    );
    assert.equal(root.querySelector("img"), null);
    assert.ok(root.textContent?.includes(malicious));
  });

  it("shows the fixed notice in the result window for a missing key", async () => {
    const store = fakeStore();
    const root = container();
    await runResultWindow(root, "#", {
      store: store.store,
      schedule: fakeSchedule().schedule,
      log: recordingLogger().log,
    });
    expectMessage(root, unreadableResultMessage);
  });
});
