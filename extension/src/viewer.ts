// The viewer entry point. It builds the real dependencies in chromeDeps.ts
// and from the page's own document, location and history, and hands them to
// startViewer; the path logic lives in viewerApp.ts. The page URL is read
// once, as loaded: startViewer rewrites it only through replaceQuery.
import {
  createClipboardWriter,
  createFileSaver,
  createLogger,
  createSendDeps,
  createViewerChannelOpener,
  createViewerTabs,
  createWorkerSend,
  readUiLanguage,
} from "./browser/chromeDeps.ts";
import { startViewer } from "./viewerApp.ts";

const send = createSendDeps(chrome.runtime);
void startViewer(document, {
  uiLanguage: () => readUiLanguage(chrome.i18n),
  page: {
    search: location.search,
    hash: location.hash,
    replaceQuery: (query) => history.replaceState(null, "", `?${query}`),
    isVisible: () => document.visibilityState === "visible",
    onVisible: (listener) =>
      document.addEventListener("visibilitychange", () => {
        if (document.visibilityState === "visible") {
          listener();
        }
      }),
  },
  tabs: createViewerTabs(chrome.tabs),
  sendToWorker: createWorkerSend(chrome.runtime),
  openChannel: createViewerChannelOpener(),
  send,
  now: () => Date.now(),
  writeClipboard: createClipboardWriter(navigator.clipboard),
  saveFile: createFileSaver(document, URL, send.schedule),
  log: createLogger(),
});
