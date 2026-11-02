// The popup entry point. It builds the real dependencies in chromeDeps.ts and
// hands them to startPopup, which decides the language, sets the page language
// before the first render, finds the skeleton in popup.html, and draws the
// popup once the launch path has collected, read and checked everything; the
// path logic lives in launch.ts. The send path then arms the primary button
// only for a sendable selection: a press asks the service worker for the
// start, and the popup polls the key and opens the viewer.
import {
  createActiveTabQuery,
  createLogger,
  createMetadataReader,
  createQueryIdSource,
  createSelectionReader,
  createSendDeps,
  createViewerChannelOpener,
  createViewerTabs,
  createWindowFocus,
  createWorkerSend,
  readUiLanguage,
} from "./browser/chromeDeps.ts";
import { startPopup } from "./launch.ts";

const log = createLogger();
void startPopup(document, {
  uiLanguage: () => readUiLanguage(chrome.i18n),
  launch: {
    reader: createSelectionReader(chrome.scripting),
    metadataReader: createMetadataReader(chrome.scripting),
    activeTab: createActiveTabQuery(chrome.tabs),
    log,
  },
  send: {
    sendToWorker: createWorkerSend(chrome.runtime),
    send: createSendDeps(chrome.runtime),
    tabs: createViewerTabs(chrome.tabs),
    focusWindow: createWindowFocus(),
    openChannel: createViewerChannelOpener(),
    queryId: createQueryIdSource(),
    identity: chrome.runtime,
    now: () => Date.now(),
    log,
  },
});
