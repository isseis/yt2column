import type { PageSelection, SelectionReader } from "../core/collect.ts";
import type { SettingsStore } from "../core/settings.ts";
import type { Logger, ResultViewStore } from "../launch.ts";
import type { FetchFn, SendDeps } from "../send.ts";

/** The one chrome.scripting call used; its result is validated at runtime. */
export interface ScriptingApi {
  executeScript(injection: {
    readonly target: { readonly tabId: number };
    readonly func: () => PageSelection;
  }): Promise<Array<{ readonly result?: unknown }>>;
}

/** The chrome.storage area surface used (session for result views, local for settings). */
export interface StorageArea {
  get(keys: string | string[]): Promise<Record<string, unknown>>;
  set(items: Record<string, unknown>): Promise<void>;
  remove(keys: string | string[]): Promise<void>;
}

/** The chrome.storage.onChanged surface used to follow a result view. */
export interface StorageChangeEvents {
  addListener(callback: StorageChangeListener): void;
  removeListener(callback: StorageChangeListener): void;
}

export type StorageChangeListener = (
  changes: Record<string, { readonly newValue?: unknown }>,
  areaName: string,
) => void;

export interface WindowsApi {
  create(data: {
    readonly url: string;
    readonly type: "popup";
  }): Promise<unknown>;
}

export interface ActionApi {
  setBadgeText(details: { readonly text: string }): Promise<void>;
}

export interface TabsApi {
  query(info: {
    readonly active: boolean;
    readonly currentWindow: boolean;
  }): Promise<chrome.tabs.Tab[]>;
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
      });
      const result = results[0]?.result;
      if (!isPageSelection(result)) {
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

const tokenKey = "sharedToken";
const portKey = "serverPort";

/**
 * The SettingsStore backed by the given storage area (chrome.storage.local in
 * production). The token and the port use separate keys, so saving or clearing
 * one never touches the other, and load reads both keys in one get.
 */
export function createSettingsStore(area: StorageArea): SettingsStore {
  return {
    async load() {
      const values = await area.get([tokenKey, portKey]);
      return { token: values[tokenKey], port: values[portKey] };
    },
    async saveToken(token) {
      await area.set({ [tokenKey]: token });
    },
    async clearToken() {
      await area.remove(tokenKey);
    },
    async savePort(port) {
      await area.set({ [portKey]: port });
    },
  };
}

/**
 * The ResultViewStore backed by the given storage area (chrome.storage.session
 * in production) and change events (chrome.storage.onChanged). watch reports
 * only a value newly stored under the key in the "session" area; a removal or
 * a change in another area or key is not reported.
 */
export function createResultViewStore(
  area: StorageArea,
  events: StorageChangeEvents,
): ResultViewStore {
  return {
    async put(key, view) {
      await area.set({ [key]: view });
    },
    async get(key) {
      const values = await area.get(key);
      return values[key];
    },
    async remove(key) {
      await area.remove(key);
    },
    watch(key, listener) {
      const callback: StorageChangeListener = (changes, areaName) => {
        if (areaName !== "session") {
          return;
        }
        const change = changes[key];
        if (
          change === undefined ||
          !Object.prototype.hasOwnProperty.call(change, "newValue")
        ) {
          return;
        }
        listener(change.newValue);
      };
      events.addListener(callback);
      return () => events.removeListener(callback);
    },
  };
}

/** Opens resultUrl with the summary key in the fragment, as a popup window. */
export function createResultWindowOpener(
  windows: WindowsApi,
  resultUrl: string,
): (key: string) => Promise<void> {
  return async (key) => {
    await windows.create({ url: `${resultUrl}#${key}`, type: "popup" });
  };
}

export interface Badge {
  signal(): Promise<void>;
  clear(): Promise<void>;
}

/** The toolbar badge used as the visible fallback for a display failure. */
export function createBadge(action: ActionApi): Badge {
  return {
    signal: () => action.setBadgeText({ text: "!" }),
    clear: () => action.setBadgeText({ text: "" }),
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
 * The SendDeps backed by the global fetch and AbortSignal.timeout. fetch is
 * called through this wrapper, so the global stays an unbound reference;
 * calling the global as another object's method would throw "Illegal
 * invocation" and be read as an unknown result.
 */
export function createSendDeps(): SendDeps {
  const fetchFn: FetchFn = (url, init) => fetch(url, init);
  return {
    fetch: fetchFn,
    timeoutSignal: (ms) => AbortSignal.timeout(ms),
  };
}

/** Runs callback once after ms milliseconds; the returned function cancels it. */
export function schedule(ms: number, callback: () => void): () => void {
  const id = setTimeout(callback, ms);
  return () => clearTimeout(id);
}
