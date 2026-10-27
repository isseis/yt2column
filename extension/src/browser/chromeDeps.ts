import type { PageSelection, SelectionReader } from "../core/collect.ts";
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
