// The popup entry point. It builds the real dependencies in chromeDeps.ts and
// hands them to startPopup, which decides the language, sets the page language
// before the first render and runs the popup and send paths; the path logic
// lives in launch.ts. The summary goes into one region and the send controls
// into the other.
import {
  createActiveTabQuery,
  createLogger,
  createMetadataReader,
  createSelectionReader,
  createSendDeps,
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
    send: createSendDeps(chrome.runtime),
    now: () => Date.now(),
    log,
  },
});
