// Fakes of the browser wrappers in src/browser/chromeDeps.ts, in the shape of
// their interfaces. Each records what the code under test asked for and lets
// the test decide when and how the browser answers, so the order of tab
// creation, runtime messages and channel messages can be set case by case.

import {
  localItems,
  storedAnthropicSettings,
} from "../../src/browser/chromeDeps.ts";
import type {
  BrowserCall,
  ExtensionIdentity,
  FocusWindow,
  MessageEventsApi,
  MessageSender,
  NewTab,
  OpenViewerChannel,
  SaveFile,
  SendToWorker,
  TabPlace,
  ViewerChannel,
  ViewerTabs,
  WriteClipboard,
  localKeys,
  onSessionChanged,
  readAnthropicSettings,
  readSessionItems,
  removeAnthropicSettings,
  removeSessionItems,
  SessionChanges,
  SessionItemChange,
  setLocalAccessLevel,
  writeAnthropicSettings,
  writeSessionItems,
} from "../../src/browser/chromeDeps.ts";
import type {
  AwaitStart,
  StartRequest,
} from "../../src/core/extensionMessages.ts";
import { settle } from "./clock.ts";

export const fakeExtensionId = "abcdefghijklmnopabcdefghijklmnop";

/** The extension's identity, with the origin a page of this ID has. */
export const fakeIdentity: ExtensionIdentity = {
  id: fakeExtensionId,
  getURL: (path) => `chrome-extension://${fakeExtensionId}/${path}`,
};

/** The sender the toolbar popup has: no tab, the popup's URL. */
export const popupSender: MessageSender = {
  id: fakeExtensionId,
  url: fakeIdentity.getURL("popup.html"),
};

/** The sender a viewer page in tabId has. */
export function viewerSender(tabId: number, query = "h=x"): MessageSender {
  return {
    id: fakeExtensionId,
    url: fakeIdentity.getURL(`viewer.html?${query}`),
    tab: { id: tabId },
  };
}

const ok = <T>(value: T): BrowserCall<T> => ({ ok: true, value });
const failed = <T>(error: string): BrowserCall<T> => ({ ok: false, error });

export interface FakeRuntime {
  /** The onMessage event: listenForMessages registers on it, as on chrome.runtime. */
  readonly events: MessageEventsApi;
  /**
   * Sends message from sender, as runtime.sendMessage does, except that the
   * listener runs synchronously instead of as a task; the code under test
   * awaits the reply either way, so only the order of later tasks differs, and
   * every test settles before it checks. It fails at once
   * when no listener is registered or the listener returns false without
   * responding, and resolves the response otherwise. A closed sender is a
   * page that closed after sending: the response never reaches it, and its
   * promise resolves as a failure once the listener has returned. As in the
   * browser, a sendResponse to a closed sender throws.
   */
  send(
    sender: MessageSender,
    message: unknown,
    options?: { readonly closed?: boolean },
  ): Promise<BrowserCall<unknown>>;
  /** The SendToWorker of a page whose sender is sender. */
  senderFor(sender: MessageSender): SendToWorker;
  /** What the listener returned for each message, in order. */
  readonly returned: boolean[];
}

type RawListener = Parameters<MessageEventsApi["onMessage"]["addListener"]>[0];

export function fakeRuntime(): FakeRuntime {
  let registered: RawListener | undefined;
  const returned: boolean[] = [];
  const send: FakeRuntime["send"] = (sender, message, options = {}) => {
    const listener = registered;
    if (listener === undefined) {
      return Promise.resolve(failed("no receiving end"));
    }
    return new Promise((resolve) => {
      let responded = false;
      const answer = listener(message, sender, (response) => {
        if (options.closed === true) {
          throw new Error("Attempting to use a disconnected port object");
        }
        if (responded) {
          return;
        }
        responded = true;
        resolve(ok(response));
      });
      returned.push(answer);
      if (options.closed === true) {
        resolve(failed("the sender closed"));
      } else if (!answer && !responded) {
        resolve(failed("the message port closed before a response"));
      }
    });
  };
  return {
    events: {
      onMessage: {
        addListener: (listener) => {
          registered = listener;
        },
      },
    },
    send,
    senderFor: (sender) => (message) => send(sender, message),
    returned,
  };
}

/** The service worker side of a page's runtime messages, answered by the test. */
export interface PendingWorker {
  readonly send: SendToWorker;
  /** Every message sent, in order. */
  readonly sent: (StartRequest | AwaitStart)[];
  /** Answers the oldest open message and lets the page run. */
  answer(reply: BrowserCall<unknown>): Promise<void>;
}

export function pendingWorker(): PendingWorker {
  const sent: (StartRequest | AwaitStart)[] = [];
  const open: ((reply: BrowserCall<unknown>) => void)[] = [];
  return {
    sent,
    send: (message) => {
      sent.push(message);
      return new Promise((resolve) => open.push(resolve));
    },
    answer: async (reply) => {
      const resolve = open.shift();
      if (resolve === undefined) {
        throw new Error("no worker message is open");
      }
      resolve(reply);
      await settle();
    },
  };
}

export type TabOperation =
  | { readonly op: "create"; readonly tab: NewTab }
  | { readonly op: "activate" | "remove" | "get"; readonly tabId: number }
  | { readonly op: "getCurrent" };

export interface FakeTabs extends ViewerTabs {
  /** Every call, in order. */
  readonly calls: TabOperation[];
  /** The open tabs and their windows. create adds one, remove deletes it. */
  readonly open: Map<number, number>;
  /** Makes each later create wait until releaseCreates. */
  holdCreates(): void;
  /** Completes every create held so far, and stops holding. */
  releaseCreates(): void;
  /** Makes every later call of op fail. */
  fail(op: TabOperation["op"]): void;
  /** What getCurrent returns: the page's own tab, or undefined outside a tab. */
  current: TabPlace | undefined;
}

/** Tabs whose IDs start at firstId and open in windowId unless a window is given. */
export function fakeTabs(firstId = 100, windowId = 1): FakeTabs {
  const calls: TabOperation[] = [];
  const open = new Map<number, number>();
  const failing = new Set<TabOperation["op"]>();
  let nextId = firstId;
  let holding = false;
  const held: (() => void)[] = [];
  const known = (tabId: number): BrowserCall<TabPlace> => {
    const inWindow = open.get(tabId);
    return inWindow === undefined
      ? failed(`No tab with id: ${tabId}.`)
      : ok({ tabId, windowId: inWindow });
  };
  const tabs: FakeTabs = {
    calls,
    open,
    current: undefined,
    holdCreates: () => {
      holding = true;
    },
    releaseCreates: () => {
      holding = false;
      for (const release of held.splice(0)) {
        release();
      }
    },
    fail: (op) => {
      failing.add(op);
    },
    create: async (tab) => {
      calls.push({ op: "create", tab });
      if (holding) {
        await new Promise<void>((resolve) => held.push(resolve));
      }
      if (failing.has("create")) {
        return failed("Tabs cannot be created.");
      }
      const tabId = nextId++;
      open.set(tabId, tab.windowId ?? windowId);
      return known(tabId);
    },
    activate: async (tabId) => {
      calls.push({ op: "activate", tabId });
      if (failing.has("activate")) {
        return failed("Tabs cannot be edited.");
      }
      const tab = known(tabId);
      return tab.ok ? ok(undefined) : failed(tab.error);
    },
    remove: async (tabId) => {
      calls.push({ op: "remove", tabId });
      if (failing.has("remove")) {
        return failed("Tabs cannot be edited.");
      }
      const tab = known(tabId);
      if (!tab.ok) {
        return failed(tab.error);
      }
      open.delete(tabId);
      return ok(undefined);
    },
    get: async (tabId) => {
      calls.push({ op: "get", tabId });
      return failing.has("get") ? failed("get failed") : known(tabId);
    },
    getCurrent: async () => {
      calls.push({ op: "getCurrent" });
      return failing.has("getCurrent")
        ? failed("getCurrent failed")
        : ok(tabs.current);
    },
  };
  return tabs;
}

export interface FakeChannels {
  /** Opens one endpoint, as the browser's channel opener does. */
  readonly open: OpenViewerChannel;
  /** The data of every message posted on any endpoint, in order. */
  readonly posted: unknown[];
  /** Makes every later open fail. */
  failOpen(): void;
}

interface Endpoint {
  closed: boolean;
  readonly listeners: Set<(data: unknown) => void>;
}

/**
 * The viewers' channel between the endpoints opened in one test. A message is
 * delivered as a task, a copy to every other endpoint open when it was posted
 * and still open at delivery, never to the endpoint that posted it, and an
 * endpoint posts nothing once closed: the BroadcastChannel rules the code
 * depends on.
 */
export function fakeChannels(): FakeChannels {
  const endpoints = new Set<Endpoint>();
  const posted: unknown[] = [];
  let failing = false;
  return {
    posted,
    failOpen: () => {
      failing = true;
    },
    open: () => {
      if (failing) {
        return failed("channel unavailable");
      }
      const endpoint: Endpoint = { closed: false, listeners: new Set() };
      endpoints.add(endpoint);
      const channel: ViewerChannel = {
        post: (message) => {
          if (endpoint.closed) {
            return false;
          }
          posted.push(message);
          for (const other of endpoints) {
            if (other === endpoint) {
              continue;
            }
            setImmediate(() => {
              if (other.closed) {
                return;
              }
              for (const listener of other.listeners) {
                listener(structuredClone(message));
              }
            });
          }
          return true;
        },
        listen: (listener) => {
          endpoint.listeners.add(listener);
          return () => endpoint.listeners.delete(listener);
        },
        close: () => {
          endpoint.closed = true;
          endpoints.delete(endpoint);
        },
      };
      return ok(channel);
    },
  };
}

export interface FakeFocus {
  readonly focus: FocusWindow;
  /** The windows brought to the front, in order. */
  readonly focused: number[];
}

export function fakeFocus(): FakeFocus {
  const focused: number[] = [];
  return {
    focused,
    focus: async (windowId) => {
      focused.push(windowId);
      return ok(undefined);
    },
  };
}

export interface FakeClipboard {
  readonly write: WriteClipboard;
  readonly written: string[];
  /** Makes every later write fail. */
  fail(): void;
}

export function fakeClipboard(): FakeClipboard {
  const written: string[] = [];
  let failing = false;
  return {
    written,
    fail: () => {
      failing = true;
    },
    write: async (text) => {
      if (failing) {
        return failed("Document is not focused.");
      }
      written.push(text);
      return ok(undefined);
    },
  };
}

export interface FakeSaves {
  readonly save: SaveFile;
  /** Each file saved, in order. */
  readonly saved: { readonly fileName: string; readonly text: string }[];
}

export function fakeSaves(): FakeSaves {
  const saved: FakeSaves["saved"] = [];
  return {
    saved,
    save: (fileName, text) => {
      saved.push({ fileName, text });
      return ok(undefined);
    },
  };
}

/** The fixed chrome.storage.local functions a fake answers. */
export type LocalOperation =
  "read" | "write" | "remove" | "keys" | "setAccessLevel";

/** The fixed chrome.storage.local functions of browser/chromeDeps.ts, over items the test holds. */
export interface FakeLocal {
  readonly read: typeof readAnthropicSettings;
  readonly write: typeof writeAnthropicSettings;
  readonly remove: typeof removeAnthropicSettings;
  readonly keys: typeof localKeys;
  readonly setAccessLevel: typeof setLocalAccessLevel;
  /**
   * The stored items by name. A test may put any value in, as the developer
   * tools can; write and remove change only the two settings items.
   */
  readonly items: Map<string, unknown>;
  /** Every call, in order. */
  readonly calls: LocalOperation[];
  /** Makes every later call of op fail; a failed write or remove changes nothing. */
  fail(op: LocalOperation): void;
}

export function fakeLocal(): FakeLocal {
  const items = new Map<string, unknown>();
  const calls: LocalOperation[] = [];
  const failing = new Set<LocalOperation>();
  /** Records the call and answers it with value, or with a failure when op fails. */
  const answer = async <T>(
    op: LocalOperation,
    value: () => T,
  ): Promise<BrowserCall<T>> => {
    calls.push(op);
    return failing.has(op)
      ? failed(`${op} failed`)
      : ok(structuredClone(value()));
  };
  return {
    items,
    calls,
    fail: (op) => {
      failing.add(op);
    },
    read: () =>
      answer("read", () => storedAnthropicSettings(Object.fromEntries(items))),
    write: (apiKey, workspaceId) =>
      answer("write", () => {
        items.set("anthropic.apiKey", apiKey);
        items.set("anthropic.workspaceId", workspaceId);
        return undefined;
      }),
    remove: () =>
      answer("remove", () => {
        items.delete("anthropic.apiKey");
        items.delete("anthropic.workspaceId");
        return undefined;
      }),
    keys: () => answer("keys", () => localItems([...items.keys()])),
    setAccessLevel: () => answer("setAccessLevel", () => undefined),
  };
}

/** The chrome.storage.session functions a fake answers. */
export type SessionOperation = "read" | "write" | "remove";

/** The fixed chrome.storage.session functions of browser/chromeDeps.ts, over items the test holds. */
export interface FakeSession {
  readonly read: typeof readSessionItems;
  readonly write: typeof writeSessionItems;
  readonly remove: typeof removeSessionItems;
  readonly onChanged: typeof onSessionChanged;
  /** The stored items by name; a test may put any value in. */
  readonly items: Map<string, unknown>;
  /** Every call, in order. */
  readonly calls: SessionOperation[];
  /** Makes every later call of op fail; a failed write or remove changes nothing. */
  fail(op: SessionOperation): void;
}

/**
 * Session storage as the browser keeps it: values are stored and read as
 * copies, and each write or remove that changes items is announced to every
 * listener registered at that time, as a task, with copies of the new values.
 * A remove announces only the items that existed.
 */
export function fakeSession(): FakeSession {
  const items = new Map<string, unknown>();
  const calls: SessionOperation[] = [];
  const failing = new Set<SessionOperation>();
  const listeners = new Set<(changes: SessionChanges) => void>();
  const announce = (changes: Map<string, SessionItemChange>): void => {
    if (changes.size === 0) {
      return;
    }
    for (const listener of [...listeners]) {
      setImmediate(() => {
        if (listeners.has(listener)) {
          listener(structuredClone(changes));
        }
      });
    }
  };
  return {
    items,
    calls,
    fail: (op) => {
      failing.add(op);
    },
    read: async (names) => {
      calls.push("read");
      if (failing.has("read")) {
        return failed("read failed");
      }
      const selected = [...items].filter(
        ([name]) => names === "every" || names.includes(name),
      );
      return ok(structuredClone(Object.fromEntries(selected)));
    },
    write: async (written) => {
      calls.push("write");
      if (failing.has("write")) {
        return failed("write failed");
      }
      const changes = new Map<string, SessionItemChange>();
      for (const [name, value] of Object.entries(structuredClone(written))) {
        items.set(name, value);
        changes.set(name, { removed: false, newValue: value });
      }
      announce(changes);
      return ok(undefined);
    },
    remove: async (names) => {
      calls.push("remove");
      if (failing.has("remove")) {
        return failed("remove failed");
      }
      const changes = new Map<string, SessionItemChange>();
      for (const name of names) {
        if (items.delete(name)) {
          changes.set(name, { removed: true });
        }
      }
      announce(changes);
      return ok(undefined);
    },
    onChanged: (listener) => {
      listeners.add(listener);
      return () => {
        listeners.delete(listener);
      };
    },
  };
}
