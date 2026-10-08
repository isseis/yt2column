import assert from "node:assert/strict";
import { describe, it } from "node:test";
import { JSDOM } from "jsdom";

import { rejectionMessage } from "../src/core/messages.ts";
import type {
  CollectOutcome,
  PageSelection,
  SelectionReader,
} from "../src/core/collect.ts";
import { summarize } from "../src/core/summary.ts";
import type { OutcomeSummary } from "../src/core/types.ts";
import {
  launchContextFromTab,
  missingResultText,
  runMenuLaunch,
  runPopupLaunch,
  runResultWindow,
} from "../src/launch.ts";
import type {
  Logger,
  MenuLaunchDeps,
  PopupLaunchDeps,
  SummaryStore,
} from "../src/launch.ts";

const watchUrl = "https://www.youtube.com/watch?v=abc";
const malicious = "<img src=x onerror=alert(1)>";

interface Recorded {
  readonly label: string;
  readonly data: unknown;
}

interface RecordingLogger {
  readonly log: Logger;
  readonly infos: Recorded[];
  readonly errors: Recorded[];
}

/** A Logger that records every call instead of printing. */
function recordingLogger(): RecordingLogger {
  const infos: Recorded[] = [];
  const errors: Recorded[] = [];
  return {
    log: {
      info: (label, data) => infos.push({ label, data }),
      error: (label, data) => errors.push({ label, data }),
    },
    infos,
    errors,
  };
}

interface RecordingStore {
  readonly store: SummaryStore;
  readonly puts: { key: string; summary: OutcomeSummary }[];
  readonly takes: string[];
  readonly removes: string[];
  readonly entries: Map<string, unknown>;
}

/** An in-memory SummaryStore that records calls and can be made to fail. */
function recordingStore(
  options: { putFails?: boolean; removeFails?: boolean } = {},
): RecordingStore {
  const puts: { key: string; summary: OutcomeSummary }[] = [];
  const takes: string[] = [];
  const removes: string[] = [];
  const entries = new Map<string, unknown>();
  const store: SummaryStore = {
    async put(key, summary) {
      if (options.putFails === true) {
        throw new Error("put failed");
      }
      puts.push({ key, summary });
      entries.set(key, summary);
    },
    async take(key) {
      takes.push(key);
      const value = entries.get(key);
      entries.delete(key);
      return value;
    },
    async remove(key) {
      removes.push(key);
      if (options.removeFails === true) {
        throw new Error("remove failed");
      }
      entries.delete(key);
    },
  };
  return { store, puts, takes, removes, entries };
}

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

/** A SelectionReader whose read always rejects with the given cause. */
function failingReader(cause: unknown): SelectionReader {
  return {
    read: async () => {
      throw cause;
    },
  };
}

interface MenuHarness {
  readonly deps: MenuLaunchDeps;
  readonly recorder: RecordingLogger;
  readonly store: RecordingStore;
  readonly windows: string[];
  readonly badge: { signals: number; clears: number };
}

interface MenuOptions {
  readonly reader?: SelectionReader;
  readonly store?: RecordingStore;
  readonly newKey?: () => string;
  readonly openResultWindow?: (key: string) => Promise<void>;
  readonly signalDisplayFailure?: () => Promise<void>;
  readonly clearDisplayFailure?: () => Promise<void>;
}

/** Builds menu-path dependencies with recording fakes and visible counters. */
function menuHarness(options: MenuOptions = {}): MenuHarness {
  const recorder = recordingLogger();
  const store = options.store ?? recordingStore();
  const windows: string[] = [];
  const badge = { signals: 0, clears: 0 };
  const deps: MenuLaunchDeps = {
    reader: options.reader ?? readerReturning(page("selection")),
    store: store.store,
    newKey: options.newKey ?? (() => "key-1"),
    openResultWindow:
      options.openResultWindow ??
      (async (key) => {
        windows.push(key);
      }),
    signalDisplayFailure:
      options.signalDisplayFailure ??
      (async () => {
        badge.signals += 1;
      }),
    clearDisplayFailure:
      options.clearDisplayFailure ??
      (async () => {
        badge.clears += 1;
      }),
    log: recorder.log,
  };
  return { deps, recorder, store, windows, badge };
}

interface PopupHarness {
  readonly deps: PopupLaunchDeps;
  readonly recorder: RecordingLogger;
}

/** Builds popup-path dependencies with a recording logger and fake tab. */
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

/** A fresh container element in its own document. */
function container(): HTMLElement {
  const dom = new JSDOM("<!DOCTYPE html><body><div id='root'></div></body>");
  const element = dom.window.document.getElementById("root");
  assert.ok(element !== null);
  return element;
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
    assert.deepEqual(menu.store.puts[0]?.summary, summarize(popupOutcome));
  });

  it("rejects the same way through the menu and popup paths", async () => {
    const reader = readerReturning(page("text"));
    const tab = fakeTab({ url: "https://example.com/" });
    const popup = popupHarness({ reader, activeTab: async () => tab });
    const popupOutcome = await runPopupLaunch(container(), popup.deps);
    const menu = menuHarness({ reader });
    await runMenuLaunch(menuInfo(), tab, menu.deps);
    assert.deepEqual(popupOutcome, {
      kind: "rejected",
      reason: "not-watch-page",
    });
    assert.deepEqual(loggedOutcome(menu.recorder), popupOutcome);
  });

  it("treats a missing tab as collection-failed in both paths", async () => {
    const reader = readerReturning(page("text"));
    const popup = popupHarness({ reader, activeTab: async () => undefined });
    const popupOutcome = await runPopupLaunch(container(), popup.deps);
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
    const outcome = await runPopupLaunch(container(), popup.deps);
    assert.equal(outcome.kind, "rejected");
    assert.equal(outcome.reason, "collection-failed");
    assert.equal(popup.recorder.errors.length, 1);
  });

  it("ignores info.selectionText and reads the selection itself", async () => {
    const reader = readerReturning(page("actual selection"));
    const menu = menuHarness({ reader });
    await runMenuLaunch(
      menuInfo({ selectionText: "wrong" }),
      fakeTab(),
      menu.deps,
    );
    const outcome = loggedOutcome(menu.recorder);
    assert.ok(outcome.kind === "collected");
    assert.equal(outcome.input.selection, "actual selection");
  });

  it("keeps untrusted text out of the fixed log label", async () => {
    const reader = readerReturning(page(`${malicious}\n`));
    const popup = popupHarness({ reader });
    await runPopupLaunch(container(), popup.deps);
    const label = popup.recorder.infos[0]?.label;
    assert.equal(label, "yt2column popup launch");
    assert.ok(label !== undefined);
    assert.ok(!label.includes(malicious));
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
    const store = recordingStore({ putFails: true });
    const menu = menuHarness({ store });
    await runMenuLaunch(menuInfo(), fakeTab(), menu.deps);
    assert.deepEqual(menu.windows, []);
    assert.deepEqual(store.removes, []);
    assert.equal(menu.badge.signals, 1);
    assert.equal(menu.recorder.errors.length, 1);
  });

  it("removes the stored summary and signals the badge when the window fails", async () => {
    const menu = menuHarness({
      openResultWindow: async () => {
        throw new Error("window failed");
      },
    });
    await runMenuLaunch(menuInfo(), fakeTab(), menu.deps);
    assert.deepEqual(menu.store.removes, ["key-1"]);
    assert.equal(menu.badge.signals, 1);
  });

  it("does not reject when the window and the removal both fail", async () => {
    const store = recordingStore({ removeFails: true });
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

  it("never rejects on a collection failure in either path", async () => {
    await runMenuLaunch(menuInfo(), undefined, menuHarness().deps);
    await runPopupLaunch(
      container(),
      popupHarness({ reader: failingReader(new Error("boom")) }).deps,
    );
  });
});

describe("display", () => {
  it("renders the popup summary as text, not elements", async () => {
    const reader = readerReturning(page(malicious));
    const root = container();
    await runPopupLaunch(root, popupHarness({ reader }).deps);
    assert.equal(root.querySelector("img"), null);
    assert.ok(root.textContent?.includes(malicious));
  });

  it("shows the reason and steps in the popup for a non-watch page", async () => {
    const reader = readerReturning(page(malicious));
    const root = container();
    await runPopupLaunch(
      root,
      popupHarness({
        reader,
        activeTab: async () => fakeTab({ url: "https://example.com/" }),
      }).deps,
    );
    const message = rejectionMessage("not-watch-page");
    assert.equal(root.querySelector("img"), null);
    assert.ok(root.textContent?.includes(message.text));
    assert.equal(
      root.querySelectorAll("ol.summary-steps > li").length,
      message.steps.length,
    );
  });

  it("shows the collected summary in the popup for a video page", async () => {
    const root = container();
    await runPopupLaunch(root, popupHarness().deps);
    assert.equal(root.querySelectorAll(".summary-field").length, 5);
  });

  it("reads and removes the stored summary in the result window", async () => {
    const store = recordingStore();
    const summary = summarize({
      kind: "rejected",
      reason: "empty-selection",
    });
    await store.store.put("key-1", summary);
    const root = container();
    await runResultWindow(root, "#key-1", {
      store: store.store,
      log: recordingLogger().log,
    });
    assert.deepEqual(store.takes, ["key-1"]);
    assert.equal(store.entries.has("key-1"), false);
    assert.ok(
      root.textContent?.includes(rejectionMessage("empty-selection").text),
    );
  });

  it("renders the result window summary as text, not elements", async () => {
    const store = recordingStore();
    await store.store.put("key-1", {
      kind: "collected",
      title: malicious,
      titleTruncated: false,
      url: watchUrl,
      urlTruncated: false,
      characterCount: malicious.length,
      lineCount: 1,
      preview: malicious,
      previewTruncated: false,
    });
    const root = container();
    await runResultWindow(root, "#key-1", {
      store: store.store,
      log: recordingLogger().log,
    });
    assert.equal(root.querySelector("img"), null);
    assert.ok(root.textContent?.includes(malicious));
  });

  it("shows the fixed notice when the hash is missing", async () => {
    const store = recordingStore();
    for (const hash of ["", "#"]) {
      const root = container();
      await runResultWindow(root, hash, {
        store: store.store,
        log: recordingLogger().log,
      });
      assert.equal(root.textContent, missingResultText);
    }
    assert.deepEqual(store.takes, []);
  });

  it("shows the fixed notice when no summary is stored", async () => {
    const store = recordingStore();
    const root = container();
    await runResultWindow(root, "#missing", {
      store: store.store,
      log: recordingLogger().log,
    });
    assert.equal(root.textContent, missingResultText);
  });

  it("shows the fixed notice when reading the summary fails", async () => {
    const recorder = recordingLogger();
    const store: SummaryStore = {
      put: async () => undefined,
      take: async () => {
        throw new Error("storage failed");
      },
      remove: async () => undefined,
    };
    const root = container();
    await runResultWindow(root, "#key-1", { store, log: recorder.log });
    assert.equal(root.textContent, missingResultText);
    assert.equal(recorder.errors.length, 1);
  });

  it("logs and shows the fixed notice when the stored shape is wrong", async () => {
    const recorder = recordingLogger();
    const store: SummaryStore = {
      put: async () => undefined,
      take: async () => ({ kind: "bogus" }),
      remove: async () => undefined,
    };
    const root = container();
    await runResultWindow(root, "#key-1", { store, log: recorder.log });
    assert.equal(root.textContent, missingResultText);
    assert.equal(recorder.errors.length, 1);
  });
});
