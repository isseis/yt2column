import type { PageSelection, SelectionReader } from "../core/collect.ts";
import { viewerChannelName } from "../core/extensionMessages.ts";
import type {
  AwaitStart,
  StartRequest,
  ViewerChannelMessage,
} from "../core/extensionMessages.ts";
import { nativeHostName } from "../core/native.ts";
import type { Logger, MetadataReader } from "../launch.ts";
import type { SendDeps } from "../send.ts";

/** The chrome.scripting call used; the injected function's result is validated at runtime. */
export interface ScriptingApi {
  executeScript<Args extends unknown[], Result>(injection: {
    readonly target: { readonly tabId: number };
    readonly func: (...args: Args) => Result;
    readonly args: Args;
  }): Promise<Array<{ readonly result?: unknown }>>;
}

/** The chrome.runtime surface used: the callback form of sendNativeMessage and its lastError. */
export interface NativeMessagingApi {
  sendNativeMessage(
    application: string,
    message: object,
    callback: (response: unknown) => void,
  ): void;
  readonly lastError?: { readonly message?: string } | undefined;
}

export interface TabsApi {
  query(info: {
    readonly active: boolean;
    readonly currentWindow: boolean;
  }): Promise<chrome.tabs.Tab[]>;
}

/** The chrome.i18n surface used. The result is checked at run time. */
export interface I18nApi {
  getUILanguage(): unknown;
}

/**
 * Runs in the tab's top frame. It must not reference any name from this
 * module: chrome.scripting serializes the function's source and runs it in the
 * page, so an outer reference would throw there. It reads only the page-shared
 * DOM selection and location.
 */
function readSelection(): PageSelection {
  return {
    text: window.getSelection()?.toString() ?? "",
    documentUrl: location.href,
  };
}

/** True when value is an object whose text and documentUrl are both strings. */
function isPageSelection(value: unknown): value is PageSelection {
  if (typeof value !== "object" || value === null) {
    return false;
  }
  const record = value as { text?: unknown; documentUrl?: unknown };
  return (
    typeof record.text === "string" && typeof record.documentUrl === "string"
  );
}

/**
 * The SelectionReader backed by chrome.scripting. read() rejects when the
 * injection throws or returns a value that is not the expected shape.
 */
export function createSelectionReader(
  scripting: ScriptingApi,
): SelectionReader {
  return {
    async read(tabId: number): Promise<PageSelection> {
      const results = await scripting.executeScript({
        target: { tabId },
        func: readSelection,
        args: [],
      });
      const result = results[0]?.result;
      if (!isPageSelection(result)) {
        throw new Error("the injected function returned an unexpected value");
      }
      return result;
    },
  };
}

/**
 * Runs in the tab's top frame. It must not reference any name from this
 * module: chrome.scripting serializes the function's source and runs it in the
 * page, so an outer reference would throw there. It returns the watch metadata
 * as JSON text, or null when the page has already moved away from the URL the
 * selection was collected on. Members whose element is absent are left out.
 */
function readPageMetadata(expectedUrl: string): string | null {
  if (location.href !== expectedUrl) {
    return null;
  }
  const metadata: {
    title?: string;
    channelName?: string;
    description?: string;
    descriptionCollapsed: boolean;
  } = { descriptionCollapsed: false };
  const titleElement = document.querySelector(
    "ytd-watch-metadata #title h1 yt-formatted-string",
  );
  if (titleElement !== null) {
    metadata.title = titleElement.textContent ?? "";
  }
  const channelElement = document.querySelector("#owner #channel-name #text");
  if (channelElement !== null) {
    metadata.channelName = channelElement.textContent ?? "";
  }
  const expander = document.querySelector(
    "ytd-watch-metadata #description-inline-expander",
  );
  if (expander !== null) {
    // The collapsed state alone picks the element: reading #expanded's content
    // as a second signal would let a still-empty #expanded send a snippet while
    // the expand prompt stays hidden.
    const collapsed = !expander.hasAttribute("is-expanded");
    metadata.descriptionCollapsed = collapsed;
    const snippet = expander.querySelector(
      collapsed ? "#attributed-snippet-text" : "#expanded yt-attributed-string",
    );
    if (snippet !== null) {
      metadata.description = snippet.textContent ?? "";
    }
  }
  return JSON.stringify(metadata);
}

/**
 * The MetadataReader backed by chrome.scripting. read() rejects when the
 * injection throws or returns a value that is not a string (a URL mismatch
 * makes the injected function return null).
 */
export function createMetadataReader(scripting: ScriptingApi): MetadataReader {
  return {
    async read(tabId: number, expectedUrl: string): Promise<string> {
      const results = await scripting.executeScript({
        target: { tabId },
        func: readPageMetadata,
        args: [expectedUrl],
      });
      const result = results[0]?.result;
      if (typeof result !== "string") {
        throw new Error("the injected function returned an unexpected value");
      }
      return result;
    },
  };
}

/** The console-backed developer log; the label stays the first argument. */
export function createLogger(): Logger {
  return {
    info: (label, data) => console.info(label, data),
    error: (label, data) => console.error(label, data),
  };
}

/** Queries the active tab of the current window, for the popup path. */
export function createActiveTabQuery(
  tabs: TabsApi,
): () => Promise<chrome.tabs.Tab | undefined> {
  return async () => {
    const [tab] = await tabs.query({ active: true, currentWindow: true });
    return tab;
  };
}

/**
 * The browser's UI language, or undefined when it cannot be read. A throw or a
 * value that is not a string both resolve as undefined; the caller decides the
 * display language from the result and never has to handle a failure.
 */
export function readUiLanguage(i18n: I18nApi): string | undefined {
  try {
    const language = i18n.getUILanguage();
    return typeof language === "string" ? language : undefined;
  } catch {
    return undefined;
  }
}

/**
 * The SendDeps backed by runtime (chrome.runtime in production) and the
 * global timer. sendNative sends to the fixed host name only and never
 * rejects: a lastError, whatever its text, and a synchronous throw (the
 * browser throws instead of calling back for a message over its limit) both
 * resolve as "failed", so the promise always settles.
 */
export function createSendDeps(runtime: NativeMessagingApi): SendDeps {
  return {
    sendNative: (message) =>
      new Promise((resolve) => {
        try {
          runtime.sendNativeMessage(nativeHostName, message, (response) => {
            // Reading lastError in the callback also marks it as checked.
            const lastError = runtime.lastError;
            if (lastError !== undefined) {
              resolve({ kind: "failed", lastError: lastError.message });
              return;
            }
            resolve({ kind: "message", value: response });
          });
        } catch (error) {
          resolve({ kind: "failed", lastError: String(error) });
        }
      }),
    schedule: (ms, callback) => {
      const id = setTimeout(callback, ms);
      return () => clearTimeout(id);
    },
  };
}

/**
 * What a browser call became. error is the browser's text. It may name a URL,
 * so it is never shown, never passed to another page, and logged only where
 * the logging rules allow the browser's error text (a host failure).
 */
export type BrowserCall<T> =
  | { readonly ok: true; readonly value: T }
  | { readonly ok: false; readonly error: string };

/** Runs call once, turning a rejection or a synchronous throw into a failure. */
async function browserCall<T>(call: () => Promise<T>): Promise<BrowserCall<T>> {
  try {
    return { ok: true, value: await call() };
  } catch (error) {
    return { ok: false, error: String(error) };
  }
}

/** The extension's own identity: chrome.runtime in production. */
export interface ExtensionIdentity {
  readonly id: string;
  getURL(path: string): string;
}

/** The sender of a runtime message, as the browser describes it. */
export interface MessageSender {
  readonly id?: string | undefined;
  readonly url?: string | undefined;
  readonly tab?: { readonly id?: number | undefined } | undefined;
}

/**
 * Handles one runtime message. It returns true only when it will call
 * sendResponse later; false leaves the message unanswered, which makes the
 * sender's sendMessage fail at once.
 */
export type MessageListener = (
  message: unknown,
  sender: MessageSender,
  sendResponse: (response: unknown) => void,
) => boolean;

/** The chrome.runtime.onMessage surface used. */
export interface MessageEventsApi {
  readonly onMessage: {
    addListener(
      callback: (
        message: unknown,
        sender: MessageSender,
        sendResponse: (response?: unknown) => void,
      ) => boolean,
    ): void;
  };
}

/**
 * Registers listener for runtime messages. The service worker calls it
 * synchronously at its top level: the event that woke a stopped worker
 * reaches only the listeners its first run registered. A listener that throws
 * leaves the message unanswered, and a response the sender can no longer
 * receive (the popup closed) is dropped.
 */
export function listenForMessages(
  runtime: MessageEventsApi,
  listener: MessageListener,
): void {
  runtime.onMessage.addListener((message, sender, sendResponse) => {
    const respond = (response: unknown): void => {
      try {
        sendResponse(response);
      } catch {
        // The sender is gone; nothing is waiting for the response.
      }
    };
    try {
      return listener(message, sender, respond);
    } catch {
      return false;
    }
  });
}

/** The chrome.runtime.sendMessage surface used. */
export interface MessageSendApi {
  sendMessage(message: unknown): Promise<unknown>;
}

/** Sends one message to the service worker and resolves its response. Never rejects. */
export type SendToWorker = (
  message: StartRequest | AwaitStart,
) => Promise<BrowserCall<unknown>>;

/**
 * The SendToWorker backed by runtime (chrome.runtime in production). The
 * response is untrusted until the caller's parse function accepts it; a
 * message the worker left unanswered is a failure.
 */
export function createWorkerSend(runtime: MessageSendApi): SendToWorker {
  return (message) => browserCall(() => runtime.sendMessage(message));
}

/** Where a tab is. */
export interface TabPlace {
  readonly tabId: number;
  readonly windowId: number;
}

/** A tab to open. Without a place, the browser opens it in the current window. */
export interface NewTab {
  readonly url: string;
  readonly active: boolean;
  readonly windowId?: number;
  readonly index?: number;
  readonly openerTabId?: number;
}

/** The chrome.tabs surface the viewer tabs use; none of it needs the tabs permission. */
export interface TabOperationsApi {
  create(properties: chrome.tabs.CreateProperties): Promise<chrome.tabs.Tab>;
  update(
    tabId: number,
    properties: chrome.tabs.UpdateProperties,
  ): Promise<chrome.tabs.Tab | undefined>;
  remove(tabId: number): Promise<void>;
  get(tabId: number): Promise<chrome.tabs.Tab>;
  getCurrent(): Promise<chrome.tabs.Tab | undefined>;
}

/** The tab operations the extension's pages use. Every call resolves; none rejects. */
export interface ViewerTabs {
  create(tab: NewTab): Promise<BrowserCall<TabPlace>>;
  /** Selects the tab in its window. */
  activate(tabId: number): Promise<BrowserCall<undefined>>;
  remove(tabId: number): Promise<BrowserCall<undefined>>;
  /** Where the tab is now; a failure when there is no such tab. */
  get(tabId: number): Promise<BrowserCall<TabPlace>>;
  /** The calling page's own tab; undefined when the page is not in a tab. */
  getCurrent(): Promise<BrowserCall<TabPlace | undefined>>;
}

/** The place of a tab the browser returned; a tab without an ID is a failure. */
function tabPlace(tab: chrome.tabs.Tab): TabPlace {
  if (tab.id === undefined) {
    throw new Error("the tab has no id");
  }
  return { tabId: tab.id, windowId: tab.windowId };
}

/** The ViewerTabs backed by tabs (chrome.tabs in production). */
export function createViewerTabs(tabs: TabOperationsApi): ViewerTabs {
  return {
    create: (tab) =>
      browserCall(async () => {
        const properties: chrome.tabs.CreateProperties = {
          url: tab.url,
          active: tab.active,
        };
        if (tab.windowId !== undefined) {
          properties.windowId = tab.windowId;
        }
        if (tab.index !== undefined) {
          properties.index = tab.index;
        }
        if (tab.openerTabId !== undefined) {
          properties.openerTabId = tab.openerTabId;
        }
        return tabPlace(await tabs.create(properties));
      }),
    activate: (tabId) =>
      browserCall(async () => {
        await tabs.update(tabId, { active: true });
        return undefined;
      }),
    remove: (tabId) =>
      browserCall(async () => {
        await tabs.remove(tabId);
        return undefined;
      }),
    get: (tabId) => browserCall(async () => tabPlace(await tabs.get(tabId))),
    getCurrent: () =>
      browserCall(async () => {
        const tab = await tabs.getCurrent();
        return tab === undefined ? undefined : tabPlace(tab);
      }),
  };
}

/** The window update the focus uses: chrome.windows in production. */
export interface WindowFocusApi {
  update(
    windowId: number,
    info: { readonly focused: boolean },
  ): Promise<unknown>;
}

/** Brings a window to the front. Never rejects. */
export type FocusWindow = (windowId: number) => Promise<BrowserCall<undefined>>;

/**
 * The FocusWindow backed by api, chrome.windows by default: this module is the
 * one place that may reference chrome.windows, so an entry point leaves the
 * argument out.
 */
export function createWindowFocus(
  api: WindowFocusApi = chrome.windows,
): FocusWindow {
  return (windowId) =>
    browserCall(async () => {
      await api.update(windowId, { focused: true });
      return undefined;
    });
}

/** One endpoint of the viewers' channel. A message reaches every other open endpoint, never the one that posted it. */
export interface ViewerChannel {
  /** Posts message; false when the channel cannot post it (it is closed). */
  post(message: ViewerChannelMessage): boolean;
  /** Calls listener with the data of each message another endpoint posts; returns a function that stops it. */
  listen(listener: (data: unknown) => void): () => void;
  close(): void;
}

/** Opens one endpoint of the viewers' channel. Never throws. */
export type OpenViewerChannel = () => BrowserCall<ViewerChannel>;

/** The BroadcastChannel surface used. */
export interface BroadcastChannelLike {
  postMessage(message: unknown): void;
  addEventListener(
    type: "message",
    listener: (event: { readonly data: unknown }) => void,
  ): void;
  removeEventListener(
    type: "message",
    listener: (event: { readonly data: unknown }) => void,
  ): void;
  close(): void;
}

/**
 * The OpenViewerChannel backed by construct (a new BroadcastChannel in
 * production). Only the extension's own pages share its origin, so only they
 * can post on it; what arrives is still checked by the receiver.
 */
export function createViewerChannelOpener(
  construct: (name: string) => BroadcastChannelLike = (name) =>
    new BroadcastChannel(name),
): OpenViewerChannel {
  return () => {
    let channel: BroadcastChannelLike;
    try {
      channel = construct(viewerChannelName);
    } catch (error) {
      return { ok: false, error: String(error) };
    }
    return {
      ok: true,
      value: {
        post: (message) => {
          try {
            channel.postMessage(message);
            return true;
          } catch {
            return false;
          }
        },
        listen: (listener) => {
          const handler = (event: { readonly data: unknown }): void =>
            listener(event.data);
          channel.addEventListener("message", handler);
          return () => channel.removeEventListener("message", handler);
        },
        close: () => channel.close(),
      },
    };
  };
}

/** The random source of the query IDs: crypto in production. */
export interface RandomApi {
  getRandomValues(array: Uint8Array<ArrayBuffer>): Uint8Array<ArrayBuffer>;
}

/** Bytes in a query ID: 128 bits. */
const queryIdBytes = 16;

/** A new query ID: 128 random bits as 32 lowercase hexadecimal characters. */
export function createQueryIdSource(random: RandomApi = crypto): () => string {
  return () =>
    Array.from(random.getRandomValues(new Uint8Array(queryIdBytes)), (byte) =>
      byte.toString(16).padStart(2, "0"),
    ).join("");
}

/** The clipboard surface used: navigator.clipboard in production. */
export interface ClipboardApi {
  writeText(text: string): Promise<void>;
}

/** Writes text to the clipboard. Never rejects. */
export type WriteClipboard = (text: string) => Promise<BrowserCall<undefined>>;

/** The WriteClipboard backed by clipboard. */
export function createClipboardWriter(clipboard: ClipboardApi): WriteClipboard {
  return (text) =>
    browserCall(async () => {
      await clipboard.writeText(text);
      return undefined;
    });
}

/** The object URL surface used: URL in production. */
export interface ObjectUrlApi {
  createObjectURL(blob: Blob): string;
  revokeObjectURL(url: string): void;
}

/**
 * How long a saved file's object URL lives after the click: revoked at once,
 * it could be gone before the browser starts the download.
 */
export const objectUrlLifetimeMs = 60_000;

/** Saves text as a file. Never throws. */
export type SaveFile = (
  fileName: string,
  text: string,
) => BrowserCall<undefined>;

/**
 * The SaveFile that clicks an <a download> for a Blob URL on doc, which the
 * browser saves without the downloads permission because the page and the
 * Blob URL share an origin. The Blob is the UTF-8 of text with no BOM and
 * line endings unchanged. The anchor is never added to the document.
 */
export function createFileSaver(
  doc: Document,
  urls: ObjectUrlApi,
  schedule: (ms: number, callback: () => void) => () => void,
): SaveFile {
  return (fileName, text) => {
    try {
      const url = urls.createObjectURL(
        new Blob([text], { type: "text/markdown;charset=utf-8" }),
      );
      schedule(objectUrlLifetimeMs, () => urls.revokeObjectURL(url));
      const anchor = doc.createElement("a");
      anchor.href = url;
      anchor.download = fileName;
      anchor.click();
      return { ok: true, value: undefined };
    } catch (error) {
      return { ok: false, error: String(error) };
    }
  };
}
