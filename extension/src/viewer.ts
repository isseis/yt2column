// The viewer entry point. It builds the real dependencies in chromeDeps.ts
// and from the page's own document, location and history, and hands them to
// startViewer; the path logic lives in viewerApp.ts, and a viewer that runs a
// job runs it through viewerRunner.ts. The page URL is read once, as loaded:
// startViewer rewrites it only through replaceQuery.
import {
  createClipboardWriter,
  createFileSaver,
  createLogger,
  createOptionsOpener,
  createQueryIdSource,
  createRunnerPortConnector,
  createSwSend,
  createUnloadConfirmation,
  createViewerChannelOpener,
  createViewerTabs,
  onSessionChanged,
  readAnthropicSettings,
  readSessionItems,
  readUiLanguage,
  schedule,
} from "./browser/chromeDeps.ts";
import { buildKind } from "./buildKind.js";
import { startViewer } from "./viewerApp.ts";
import { createModuleWorker, startWasmWorker } from "./wasmWorkerClient.ts";

void startViewer(document, {
  uiLanguage: () => readUiLanguage(chrome.i18n),
  buildKind,
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
  sendToSw: createSwSend(chrome.runtime),
  readSession: readSessionItems,
  onSessionChanged,
  openChannel: createViewerChannelOpener(),
  newId: createQueryIdSource(),
  runner: {
    startWorker: () => startWasmWorker(createModuleWorker, schedule),
    connectPort: createRunnerPortConnector(chrome.runtime),
    readSettings: readAnthropicSettings,
    confirmUnload: createUnloadConfirmation(window),
  },
  schedule,
  now: () => Date.now(),
  openOptions: createOptionsOpener(chrome.runtime),
  writeClipboard: createClipboardWriter(navigator.clipboard),
  saveFile: createFileSaver(document, URL, schedule),
  log: createLogger(),
});
