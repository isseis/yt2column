// The extension service worker. It registers the one context-menu item and
// wires the menu path to runMenuLaunch. The path logic lives in launch.ts;
// this file only builds the real dependencies in browser/chromeDeps.ts and
// calls the path.
import {
  createBadge,
  createLogger,
  createResultWindowOpener,
  createSelectionReader,
  createSummaryStore,
} from "./browser/chromeDeps.ts";
import type { MenuLaunchDeps } from "./launch.ts";
import { runMenuLaunch } from "./launch.ts";

const menuItemId = "yt2column-collect";
const menuItemTitle = "Use selection with yt2column";
// Deliberately looser than the accepted watch URLs: the menu only offers the
// item, and collect() decides whether the page can be used.
const menuDocumentPattern = "https://www.youtube.com/watch*";
const registrationLabel = "yt2column menu registration";

/** Builds the menu path dependencies on the real chrome APIs. */
function menuDeps(): MenuLaunchDeps {
  const badge = createBadge(chrome.action);
  return {
    reader: createSelectionReader(chrome.scripting),
    store: createSummaryStore(chrome.storage.session),
    newKey: () => crypto.randomUUID(),
    openResultWindow: createResultWindowOpener(
      chrome.windows,
      chrome.runtime.getURL("result.html"),
    ),
    signalDisplayFailure: badge.signal,
    clearDisplayFailure: badge.clear,
    log: createLogger(),
  };
}

/**
 * Rebuilds the single context-menu item. removeAll is awaited so a reload
 * does not leave duplicates; a failed create is recorded, not thrown.
 */
async function registerMenuItem(): Promise<void> {
  await chrome.contextMenus.removeAll();
  chrome.contextMenus.create(
    {
      id: menuItemId,
      title: menuItemTitle,
      contexts: ["selection"],
      documentUrlPatterns: [menuDocumentPattern],
    },
    () => {
      if (chrome.runtime.lastError !== undefined) {
        console.error(registrationLabel, chrome.runtime.lastError);
      }
    },
  );
}

chrome.runtime.onInstalled.addListener(() => {
  void registerMenuItem();
});

// Registered at the module top level so each service-worker startup adds it.
chrome.contextMenus.onClicked.addListener((info, tab) => {
  void runMenuLaunch(info, tab, menuDeps());
});
