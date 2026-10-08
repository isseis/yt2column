// The popup entry point. It builds the real dependencies in chromeDeps.ts and
// calls the popup path; the path logic lives in launch.ts.
import {
  createActiveTabQuery,
  createLogger,
  createSelectionReader,
} from "./browser/chromeDeps.ts";
import { runPopupLaunch } from "./launch.ts";

void runPopupLaunch(document.body, {
  reader: createSelectionReader(chrome.scripting),
  activeTab: createActiveTabQuery(chrome.tabs),
  log: createLogger(),
});
