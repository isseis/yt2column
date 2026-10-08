import type { PageSelection, SelectionReader } from "../core/collect.ts";
import type { Logger, SummaryStore } from "../launch.ts";

/** The one chrome.scripting call used; its result is validated at runtime. */
export interface ScriptingApi {
  executeScript(injection: {
    readonly target: { readonly tabId: number };
    readonly func: () => PageSelection;
  }): Promise<Array<{ readonly result?: unknown }>>;
}

/** The chrome.storage.session surface used. */
export interface StorageArea {
  get(keys: string): Promise<Record<string, unknown>>;
  set(items: Record<string, unknown>): Promise<void>;
  remove(keys: string): Promise<void>;
}

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

/**
 * The SummaryStore backed by chrome.storage.session. take() reads and removes;
 * remove() rejects when the underlying removal fails.
 */
export function createSummaryStore(storage: StorageArea): SummaryStore {
  return {
    async put(key, summary) {
      await storage.set({ [key]: summary });
    },
    async take(key) {
      const values = await storage.get(key);
      await storage.remove(key);
      return values[key];
    },
    async remove(key) {
      await storage.remove(key);
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
