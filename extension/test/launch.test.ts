import assert from "node:assert/strict";
import { describe, it } from "node:test";

import type {
  CollectedInput,
  CollectOutcome,
  PageSelection,
  SelectionReader,
} from "../src/core/collect.ts";
import { expandPromptMessage } from "../src/core/messages.ts";
import type { RejectionMessage } from "../src/core/messages.ts";
import { emptyMetadataResult } from "../src/core/metadata.ts";
import type { MetadataResult } from "../src/core/metadata.ts";
import { maxBodyBytes } from "../src/core/prepare.ts";
import type { PreflightReason } from "../src/core/prepare.ts";
import type { CollectedSummary, ResultView } from "../src/core/resultView.ts";
import type { RequestId, SendOutcome } from "../src/core/response.ts";
import {
  blockMessage,
  sendOutcomeMessage,
  unreadableResultMessage,
} from "../src/core/sendMessages.ts";
import type { SettingsReader } from "../src/core/settings.ts";
import { summarize } from "../src/core/summary.ts";
import { metadataDisplayLimit } from "../src/ui/render.ts";
import { sendingMessage } from "../src/ui/renderSend.ts";
import {
  launchContextFromTab,
  resultWaitMs,
  runMenuLaunch,
  runPopupLaunch,
  runPopupSend,
  runResultWindow,
} from "../src/launch.ts";
import type {
  MenuLaunchDeps,
  MetadataReader,
  PopupLaunchDeps,
  PopupLaunchResult,
  PopupSendDeps,
  ResultWindowDeps,
} from "../src/launch.ts";
import type { FetchFn, FetchResponse, SendDeps } from "../src/send.ts";
import { container, query } from "./helpers/dom.ts";
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
const metadataLabel = "yt2column video metadata";
const blockedLabel = "yt2column blocked";
const sendLabel = "yt2column send result";
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

/** The JSON text the injected metadata reader returns for these values. */
function metadataJson(
  values: { title?: string; channelName?: string; description?: string },
  descriptionCollapsed = false,
): string {
  return JSON.stringify({ ...values, descriptionCollapsed });
}

interface FakeMetadataReader {
  readonly reader: MetadataReader;
  readonly calls: { tabId: number; expectedUrl: string }[];
}

/**
 * A MetadataReader that records its calls and returns text, or rejects when
 * the expected URL is not pageUrl, as the injected reader does on a mismatch.
 */
function metadataReader(text: string, pageUrl = watchUrl): FakeMetadataReader {
  const calls: { tabId: number; expectedUrl: string }[] = [];
  return {
    reader: {
      read: async (tabId, expectedUrl) => {
        calls.push({ tabId, expectedUrl });
        if (expectedUrl !== pageUrl) {
          throw new Error("the injected function returned an unexpected value");
        }
        return text;
      },
    },
    calls,
  };
}

/** A MetadataReader whose every read rejects with error. */
function rejectingMetadataReader(error: unknown): MetadataReader {
  return {
    read: async () => {
      throw error;
    },
  };
}

/** A collected outcome for the watch URL with the given selection. */
function collectedOutcome(selection = "selection"): CollectOutcome {
  return {
    kind: "collected",
    input: { selection, url: watchUrl, title: "A video" } as CollectedInput,
  };
}

/** A popup launch result; the metadata defaults to the empty result. */
function launched(
  outcome: CollectOutcome,
  metadata: MetadataResult = emptyMetadataResult(),
): PopupLaunchResult {
  return { outcome, metadata };
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
  readonly metadataReader?: MetadataReader;
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
    metadataReader:
      options.metadataReader ?? metadataReader(metadataJson({})).reader,
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
    metadataReader: metadataReader(metadataJson({})).reader,
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
    const { outcome: popupOutcome } = await runPopupLaunch(
      container(),
      popup.deps,
    );

    const menu = menuHarness({ reader });
    await runMenuLaunch(menuInfo(), tab, menu.deps);

    assert.ok(popupOutcome.kind === "collected");
    assert.equal(popupOutcome.input.selection, selection);
    assert.equal(popupOutcome.input.title, title);
    assert.deepEqual(loggedOutcome(popup.recorder), popupOutcome);
    assert.deepEqual(loggedOutcome(menu.recorder), popupOutcome);
    assert.deepEqual(menu.store.puts[0]?.view, {
      kind: "sending",
      summary: summarize(popupOutcome),
    });
  });

  it("sends the same body and classification through both paths", async () => {
    const selection = " 00:00\ncaption line\n\n";
    // The tab title and the video title differ, so the body shows which one
    // was sent: the body title must be the video title read from the page.
    const tabTitle = "TAB-TITLE-7f3a";
    const values = {
      title: "VIDEO-TITLE-91c2",
      channelName: "CHANNEL-5d0e",
      description: " first line\nsecond line ",
    };
    const reader = readerReturning(page(selection));
    const metadata = metadataReader(metadataJson(values)).reader;
    const tab = fakeTab({ id: 7, url: watchUrl, title: tabTitle });

    const menuFetch = acceptedFetch();
    const menu = menuHarness({
      reader,
      metadataReader: metadata,
      send: testSendDeps(menuFetch.fetchFn),
    });
    await runMenuLaunch(menuInfo(), tab, menu.deps);
    const menuBody = menuFetch.calls[0]?.init.body;
    assert.equal(typeof menuBody, "string");
    assert.deepEqual(JSON.parse(menuBody as string), {
      videoUrl: watchUrl,
      transcript: selection,
      ...values,
    });

    const popupFetch = acceptedFetch();
    const outcome = await runPopupLaunch(
      container(),
      popupHarness({
        reader,
        metadataReader: metadata,
        activeTab: async () => tab,
      }).deps,
    );
    const popupRoot = container();
    await runPopupSend(popupRoot, outcome, {
      settings: readySettings().reader,
      send: testSendDeps(popupFetch.fetchFn),
      now: () => 1000,
      log: recordingLogger().log,
    });
    await clickSend(popupRoot);

    const popupBody = popupFetch.calls[0]?.init.body;
    assert.equal(typeof popupBody, "string");
    assert.deepEqual(JSON.parse(popupBody as string), {
      videoUrl: watchUrl,
      transcript: selection,
      ...values,
    });
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

  it("reads the metadata once, from the collected tab and URL", async () => {
    const tab = fakeTab({ id: 9, url: watchUrl });
    const menuMetadata = metadataReader(metadataJson({}));
    const menu = menuHarness({ metadataReader: menuMetadata.reader });
    await runMenuLaunch(menuInfo(), tab, menu.deps);
    const popupMetadata = metadataReader(metadataJson({}));
    await runPopupLaunch(
      container(),
      popupHarness({
        metadataReader: popupMetadata.reader,
        activeTab: async () => tab,
      }).deps,
    );
    const expected = [{ tabId: 9, expectedUrl: watchUrl }];
    assert.deepEqual(menuMetadata.calls, expected);
    assert.deepEqual(popupMetadata.calls, expected);
  });

  const rejectedLaunches: {
    name: string;
    tab: chrome.tabs.Tab | undefined;
    selection: string;
  }[] = [
    {
      name: "not-watch-page",
      tab: fakeTab({ url: "https://example.com/" }),
      selection: "text",
    },
    { name: "collection-failed", tab: undefined, selection: "text" },
    { name: "empty-selection", tab: fakeTab(), selection: " \n" },
    { name: "empty-title", tab: fakeTab({ title: " " }), selection: "text" },
  ];
  for (const { name, tab, selection } of rejectedLaunches) {
    it(`does not read the metadata for a ${name} collection`, async () => {
      const reader = readerReturning(page(selection));
      const menuMetadata = metadataReader(metadataJson({ title: "T" }));
      const menu = menuHarness({ reader, metadataReader: menuMetadata.reader });
      await runMenuLaunch(menuInfo(), tab, menu.deps);
      const popupMetadata = metadataReader(metadataJson({ title: "T" }));
      const result = await runPopupLaunch(
        container(),
        popupHarness({
          reader,
          metadataReader: popupMetadata.reader,
          activeTab: async () => tab,
        }).deps,
      );
      assert.deepEqual(menuMetadata.calls, []);
      assert.deepEqual(popupMetadata.calls, []);
      assert.equal(result.outcome.kind, "rejected");
      assert.equal(result.outcome.reason, name);
      assert.deepEqual(result.metadata, emptyMetadataResult());
      assert.deepEqual(menu.store.puts[0]?.view, {
        kind: "blocked",
        reason: name,
      });
    });
  }

  const readFailures: {
    name: string;
    reader: MetadataReader;
    check?: string;
  }[] = [
    {
      name: "the injection fails",
      reader: rejectingMetadataReader(new Error("injection failed")),
    },
    {
      // The URL comparison itself is tested in chromeDeps.test.ts; this row
      // checks that the reader's rejection on a mismatch omits every field.
      name: "the page URL is not the collected URL",
      reader: metadataReader(
        metadataJson({ title: "T" }),
        "https://www.youtube.com/watch?v=other",
      ).reader,
    },
    {
      name: "the read has the wrong shape",
      reader: metadataReader("[]").reader,
      check: "top-level",
    },
  ];
  for (const { name, reader, check } of readFailures) {
    it(`sends without metadata and logs an error when ${name}`, async () => {
      const allFailed = ["title", "channelName", "description"].map(
        (field) => ({
          field,
          reason: "read-failure",
        }),
      );
      const menu = menuHarness({ metadataReader: reader });
      await runMenuLaunch(menuInfo(), fakeTab(), menu.deps);
      assert.deepEqual(
        menu.store.puts.map((put) => put.view.kind),
        ["sending", "sent"],
      );
      assert.deepEqual(JSON.parse(menu.fetch.calls[0]?.init.body as string), {
        videoUrl: watchUrl,
        transcript: "selection",
      });

      const popup = popupHarness({ metadataReader: reader });
      const result = await runPopupLaunch(container(), popup.deps);
      assert.equal(result.outcome.kind, "collected");
      assert.deepEqual(result.metadata.values, {});
      const popupFetch = acceptedFetch();
      const popupRoot = container();
      await runPopupSend(popupRoot, result, {
        settings: readySettings().reader,
        send: testSendDeps(popupFetch.fetchFn),
        now: () => 1000,
        log: recordingLogger().log,
      });
      await clickSend(popupRoot);
      assert.deepEqual(JSON.parse(popupFetch.calls[0]?.init.body as string), {
        videoUrl: watchUrl,
        transcript: "selection",
      });

      const expected =
        check === undefined
          ? { omissions: allFailed }
          : { check, omissions: allFailed };
      for (const recorder of [menu.recorder, popup.recorder]) {
        assert.deepEqual(
          recorder.errors.filter((entry) => entry.label === metadataLabel),
          [{ label: metadataLabel, data: expected }],
        );
      }
    });
  }

  it("treats a missing tab as collection-failed in both paths", async () => {
    const reader = readerReturning(page("text"));
    const popup = popupHarness({ reader, activeTab: async () => undefined });
    const { outcome: popupOutcome } = await runPopupLaunch(
      container(),
      popup.deps,
    );
    assert.equal(popupOutcome.kind, "rejected");
    assert.equal(popupOutcome.reason, "collection-failed");

    const menu = menuHarness({ reader });
    await runMenuLaunch(menuInfo(), undefined, menu.deps);
    const menuOutcome = loggedOutcome(menu.recorder);
    assert.equal(menuOutcome.kind, "rejected");
    assert.equal(menuOutcome.reason, "collection-failed");
  });

  it("treats a failing activeTab as collection-failed without rejecting", async () => {
    const popup = popupHarness({
      activeTab: async () => {
        throw new Error("query failed");
      },
    });
    const { outcome } = await runPopupLaunch(container(), popup.deps);
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
    const reader = readerReturning(page(`${malicious}\n`));
    const menu = menuHarness({ reader });
    await runMenuLaunch(menuInfo(), fakeTab(), menu.deps);
    assert.equal(menu.recorder.infos[0]?.label, "yt2column menu launch");
    const popup = popupHarness({ reader });
    await runPopupLaunch(container(), popup.deps);
    assert.equal(popup.recorder.infos[0]?.label, "yt2column popup launch");
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
      const entries = error ? menu.recorder.errors : menu.recorder.infos;
      const entry = entries.find(
        (candidate) => candidate.label === "yt2column blocked",
      );
      assert.ok(entry !== undefined);
      assert.equal((entry.data as { reason: string }).reason, reason);
    });
  }

  it("sends a collapsed description without a confirmation step", async () => {
    const menu = menuHarness({
      metadataReader: metadataReader(
        metadataJson({ description: "first lines" }, true),
      ).reader,
    });
    await runMenuLaunch(menuInfo(), fakeTab(), menu.deps);
    assert.equal(menu.fetch.calls.length, 1);
    const body = JSON.parse(menu.fetch.calls[0]?.init.body as string) as {
      description?: string;
    };
    assert.equal(body.description, "first lines");
    // The views hold only the summary and the outcome: no expand prompt.
    assert.deepEqual(
      menu.store.puts.map((put) => put.view),
      [
        { kind: "sending", summary: collectedSummary() },
        { kind: "sent", summary: collectedSummary(), outcome: acceptedOutcome },
      ],
    );
  });

  it("blocks body-too-large when the metadata takes the body over the limit", async () => {
    const menu = menuHarness({
      metadataReader: metadataReader(
        metadataJson({ description: "d".repeat(maxBodyBytes) }),
      ).reader,
    });
    await runMenuLaunch(menuInfo(), fakeTab(), menu.deps);
    assert.deepEqual(menu.store.puts[0]?.view, {
      kind: "blocked",
      reason: "body-too-large",
    });
    assert.deepEqual(menu.windows, ["key-1"]);
    assert.equal(menu.fetch.calls.length, 0);
  });

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

  const collected = (): PopupLaunchResult => launched(collectedOutcome());

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
    assert.equal(query<HTMLButtonElement>(root, ".send-button").disabled, true);
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
    const button = query<HTMLButtonElement>(root, ".send-button");
    button.click();
    await flush();
    assert.equal(button.disabled, true);
    expectMessage(root, sendingMessage);
    // The close guidance and the Slack warning are what the notice is for, so
    // assert the whole sentences independently of the constant it renders.
    const text = root.textContent ?? "";
    for (const sentence of [
      "Do not close this window until the result appears.",
      "If you close it, check Slack before sending again.",
    ]) {
      assert.ok(text.includes(sentence), sentence);
    }
    deferred.resolve(textResponse(202, acceptedBody));
    await flush();
    assert.equal(
      root.querySelector(".send-request-id")?.textContent,
      requestId,
    );
  });

  it("keeps the button under the expand prompt and sends the shown description", async () => {
    const fetch = acceptedFetch();
    const { deps: popupDeps } = deps({ send: testSendDeps(fetch.fetchFn) });
    const summaryRoot = container();
    const sendRoot = container();
    const result = await runPopupLaunch(
      summaryRoot,
      popupHarness({
        metadataReader: metadataReader(
          metadataJson({ description: "first lines" }, true),
        ).reader,
      }).deps,
    );
    await runPopupSend(sendRoot, result, popupDeps);
    expectMessage(summaryRoot, expandPromptMessage);
    assert.ok(sendRoot.querySelector(".send-button") !== null);
    await clickSend(sendRoot);
    assert.equal(fetch.calls.length, 1);
    const body = JSON.parse(fetch.calls[0]?.init.body as string) as {
      description?: string;
    };
    assert.equal(body.description, "first lines");
  });

  it("sends the full description while the popup shows it truncated", async () => {
    const description = "x".repeat(metadataDisplayLimit + 50);
    const fetch = acceptedFetch();
    const { deps: popupDeps } = deps({ send: testSendDeps(fetch.fetchFn) });
    const summaryRoot = container();
    const result = await runPopupLaunch(
      summaryRoot,
      popupHarness({
        metadataReader: metadataReader(metadataJson({ description })).reader,
      }).deps,
    );
    // Precondition: the popup shows a truncated value, so the body check
    // below tells the sent value apart from the displayed one.
    assert.equal(summaryRoot.textContent?.includes(description), false);
    const root = container();
    await runPopupSend(root, result, popupDeps);
    await clickSend(root);
    const body = JSON.parse(fetch.calls[0]?.init.body as string) as {
      description?: string;
    };
    assert.equal(body.description, description);
  });

  it("renders nothing and does not read settings for a rejected collection", async () => {
    const settings = failingSettings(new Error("must not load"));
    const { deps: popupDeps } = deps({ settings: settings.reader });
    const root = container();
    root.append(root.ownerDocument.createElement("p"));
    await runPopupSend(
      root,
      launched({ kind: "rejected", reason: "empty-selection" }),
      popupDeps,
    );
    assert.equal(root.childNodes.length, 0);
    assert.equal(settings.loadCalls(), 0);
  });

  it("logs a failed send at error and shows the unknown result", async () => {
    const fetch = rejectingFetch(new TypeError("Failed to fetch"));
    const { deps: popupDeps, recorder } = deps({
      send: testSendDeps(fetch.fetchFn),
    });
    const root = container();
    await runPopupSend(root, collected(), popupDeps);
    await clickSend(root);
    assert.equal(fetch.calls.length, 1);
    expectMessage(root, sendOutcomeMessage({ kind: "unknown" }));
    assert.deepEqual(
      recorder.errors.map((entry) => entry.label),
      [sendLabel],
    );
    assert.equal(
      recorder.infos.some((entry) => entry.label === sendLabel),
      false,
    );
  });

  const popupBlocks: {
    reason: PreflightReason;
    // Tells apart rows that share a reason.
    when?: string;
    settings: SettingsReader;
    result?: PopupLaunchResult;
    // A settings read failure is a defect and is logged at error.
    error: boolean;
  }[] = [
    {
      reason: "token-missing",
      settings: fixedSettings({ token: undefined, port: undefined }).reader,
      error: false,
    },
    {
      reason: "settings-unreadable",
      settings: failingSettings(new Error("storage failed")).reader,
      error: true,
    },
    {
      reason: "port-invalid",
      settings: fixedSettings({ token: identifiableToken, port: "80" }).reader,
      error: false,
    },
    {
      reason: "body-too-large",
      when: "the selection is over the limit",
      settings: readySettings().reader,
      result: launched(collectedOutcome("a".repeat(maxBodyBytes))),
      error: false,
    },
    {
      // The selection alone fits, so only the metadata can block the send.
      reason: "body-too-large",
      when: "the metadata takes the body over the limit",
      settings: readySettings().reader,
      result: launched(collectedOutcome(), {
        values: { description: "d".repeat(maxBodyBytes) },
        showExpandPrompt: false,
        omissions: [],
      }),
      error: false,
    },
  ];
  for (const { reason, when, settings, result, error } of popupBlocks) {
    const suffix = when === undefined ? "" : ` when ${when}`;
    it(`shows the ${reason} reason and no button without sending${suffix}`, async () => {
      const fetch = acceptedFetch();
      const { deps: popupDeps, recorder } = deps({
        settings,
        send: testSendDeps(fetch.fetchFn),
      });
      const root = container();
      await runPopupSend(root, result ?? collected(), popupDeps);
      assert.equal(root.querySelector(".send-button"), null);
      expectMessage(root, blockMessage(reason));
      assert.equal(fetch.calls.length, 0);
      const entries = error ? recorder.errors : recorder.infos;
      assert.deepEqual(
        entries
          .filter((entry) => entry.label === blockedLabel)
          .map((entry) => (entry.data as { reason: unknown }).reason),
        [reason],
      );
    });
  }
});

describe("video metadata log", () => {
  /** The metadata log entries of both paths for one read result. */
  async function metadataLogs(
    reader: MetadataReader,
  ): Promise<{ menu: RecordingLogger; popup: RecordingLogger }> {
    const menu = menuHarness({ metadataReader: reader });
    await runMenuLaunch(menuInfo(), fakeTab(), menu.deps);
    const popup = popupHarness({ metadataReader: reader });
    await runPopupLaunch(container(), popup.deps);
    return { menu: menu.recorder, popup: popup.recorder };
  }

  const infoCases: {
    name: string;
    text: string;
    expected: Record<string, unknown>;
  }[] = [
    {
      name: "each omitted field and its reason",
      text: metadataJson(
        { title: "kept", channelName: " \u3000", description: "a\uD800b" },
        true,
      ),
      expected: {
        accepted: ["title"],
        collapsed: true,
        omissions: [
          { field: "channelName", reason: "blank" },
          { field: "description", reason: "unpaired-surrogate" },
        ],
      },
    },
    {
      name: "only the accepted fields when nothing is omitted",
      text: metadataJson({ title: "t", channelName: "c", description: "d" }),
      expected: {
        accepted: ["title", "channelName", "description"],
        collapsed: false,
      },
    },
  ];
  for (const { name, text, expected } of infoCases) {
    it(`records ${name} at info`, async () => {
      const recorders = await metadataLogs(metadataReader(text).reader);
      for (const recorder of [recorders.menu, recorders.popup]) {
        assert.deepEqual(
          recorder.infos.filter((entry) => entry.label === metadataLabel),
          [{ label: metadataLabel, data: expected }],
        );
        assert.equal(recorder.errors.length, 0);
      }
    });
  }

  it("keeps the read values and the read error out of the logs", async () => {
    // Every branch of the read gets identifiable page text: accepted values,
    // omitted values, a rejected shape, and a rejection error.
    const values = {
      title: "VALUE-TITLE-2b8e",
      channelName: "VALUE-CHANNEL-c41f",
      description: "VALUE-DESCRIPTION-77a0",
    };
    const omittedSurrogate = "OMITTED-SURROGATE-5e19";
    const rejectedShape = "REJECTED-SHAPE-a3d4";
    const readError = "ERROR-PAGE-TEXT-0d6b";
    const runs = await Promise.all([
      metadataLogs(metadataReader(metadataJson(values, true)).reader),
      metadataLogs(
        metadataReader(
          metadataJson({ description: `${omittedSurrogate}\uD800` }),
        ).reader,
      ),
      metadataLogs(
        metadataReader(
          JSON.stringify({
            title: rejectedShape,
            descriptionCollapsed: false,
            unknown: 1,
          }),
        ).reader,
      ),
      metadataLogs(rejectingMetadataReader(new Error(readError))),
    ]);
    const serialized = JSON.stringify(
      runs.flatMap((run) =>
        [run.menu, run.popup].map((recorder) => [
          recorder.infos,
          recorder.errors,
        ]),
      ),
      (_key, value: unknown) =>
        value instanceof Error ? `${value.name}: ${value.message}` : value,
    );
    for (const value of [
      ...Object.values(values),
      omittedSurrogate,
      rejectedShape,
      readError,
    ]) {
      assert.equal(serialized.includes(value), false, value);
    }
  });
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
    expectMessage(root, sendingMessage);
    store.fire("key-1", view);
    await run;
    assert.equal(
      root.querySelector(".send-request-id")?.textContent,
      requestId,
    );
    assert.deepEqual(store.removes, ["key-1"]);
    assert.equal(store.watching("key-1"), false);
    assert.equal(schedule.delayMs(), resultWaitMs);
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
    expectMessage(root, sendOutcomeMessage({ kind: "unknown" }));
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
    const send = recordingLogger();
    await runPopupSend(root, outcome, {
      settings: readySettings().reader,
      send: testSendDeps(acceptedFetch().fetchFn),
      now: () => 1000,
      log: send.log,
    });
    await clickSend(root);
    // The block happens after the token is loaded, so the token is in reach.
    const blocked = recordingLogger();
    const blockedRoot = container();
    await runPopupSend(blockedRoot, outcome, {
      settings: fixedSettings({ token: identifiableToken, port: "80" }).reader,
      send: testSendDeps(acceptedFetch().fetchFn),
      now: () => 1000,
      log: blocked.log,
    });
    // Precondition: each log under test was written.
    assert.ok(popup.recorder.infos.length > 0);
    assert.ok(send.infos.some((entry) => entry.label === sendLabel));
    assert.ok(blocked.infos.some((entry) => entry.label === blockedLabel));

    const serialized = JSON.stringify({
      menuInfos: menu.recorder.infos,
      menuErrors: menu.recorder.errors,
      popupInfos: popup.recorder.infos,
      popupErrors: popup.recorder.errors,
      sendInfos: send.infos,
      sendErrors: send.errors,
      blockedInfos: blocked.infos,
      blockedErrors: blocked.errors,
      storePuts: menu.store.puts,
      text: root.textContent,
      blockedText: blockedRoot.textContent,
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
});
