// The popup entry point. It builds the real dependencies in chromeDeps.ts and
// hands them to startPopup, which decides the language, sets the page language
// before the first render, finds the skeleton in popup.html, and draws the
// popup once the launch path has protected the settings and collected, read
// and checked everything; the path logic lives in launch.ts. The send path
// then arms the primary button only for a sendable selection: a press asks
// the service worker for the start, and the popup follows the job's record.
import {
  createActiveTabQuery,
  createLogger,
  createMetadataReader,
  createOptionsOpener,
  createQueryIdSource,
  createSelectionReader,
  createSwSend,
  createViewerChannelOpener,
  createViewerTabs,
  createWindowFocus,
  onSessionChanged,
  readSessionItems,
  readUiLanguage,
  schedule,
  setLocalAccessLevel,
} from "./browser/chromeDeps.ts";
import { buildKind } from "./buildKind.js";
import { startPopup } from "./launch.ts";

const log = createLogger();
void startPopup(document, {
  uiLanguage: () => readUiLanguage(chrome.i18n),
  buildKind,
  launch: {
    protectSettings: setLocalAccessLevel,
    reader: createSelectionReader(chrome.scripting),
    metadataReader: createMetadataReader(chrome.scripting),
    activeTab: createActiveTabQuery(chrome.tabs),
    log,
  },
  send: {
    sendToSw: createSwSend(chrome.runtime),
    readSession: readSessionItems,
    onSessionChanged,
    openChannel: createViewerChannelOpener(),
    queryId: createQueryIdSource(),
    schedule,
    tabs: createViewerTabs(chrome.tabs),
    focusWindow: createWindowFocus(),
    identity: chrome.runtime,
    openOptions: createOptionsOpener(chrome.runtime),
    now: () => Date.now(),
    log,
  },
});
