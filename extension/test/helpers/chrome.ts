// Fakes of the browser wrappers in src/browser/chromeDeps.ts, in the shape of
// their interfaces. Each records what the code under test asked for and lets
// the test decide when and how the browser answers, so the order of tab
// creation, runtime messages and channel messages can be set case by case.

import type {
  BrowserCall,
  ExtensionIdentity,
  FocusWindow,
  MessageListener,
  MessageSender,
  NewTab,
  OpenViewerChannel,
  SaveFile,
  SendToWorker,
  TabPlace,
  ViewerChannel,
  ViewerTabs,
  WriteClipboard,
} from "../../src/browser/chromeDeps.ts";

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
  /** Registers listener, as listenForMessages does for chrome.runtime.onMessage. */
  listen(listener: MessageListener): void;
  /**
   * Sends message from sender, as runtime.sendMessage does. It fails at once
   * when no listener is registered or the listener returns false without
   * responding, and resolves the response otherwise. A closed sender is a
   * page that closed after sending: the response never reaches it, and its
   * promise resolves as a failure once the listener has returned.
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

export function fakeRuntime(): FakeRuntime {
  let registered: MessageListener | undefined;
  const returned: boolean[] = [];
  const send: FakeRuntime["send"] = (sender, message, options = {}) => {
    const listener = registered;
    if (listener === undefined) {
      return Promise.resolve(failed("no receiving end"));
    }
    return new Promise((resolve) => {
      let responded = false;
      const answer = listener(message, sender, (response) => {
        if (responded) {
          return;
        }
        responded = true;
        if (options.closed !== true) {
          resolve(ok(response));
        }
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
    listen: (listener) => {
      registered = listener;
    },
    send,
    senderFor: (sender) => (message) => send(sender, message),
    returned,
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
 * delivered as a task, a copy to every other endpoint open at delivery time,
 * never to the endpoint that posted it, and an endpoint posts nothing once
 * closed: the BroadcastChannel rules the code depends on.
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
