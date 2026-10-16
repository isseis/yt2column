// The popup entry point. It builds the real dependencies in chromeDeps.ts and
// calls the popup and send paths; the path logic lives in launch.ts. The
// summary goes into one region and the send controls into the other.
import {
  createActiveTabQuery,
  createLogger,
  createSelectionReader,
  createSendDeps,
  createSettingsStore,
} from "./browser/chromeDeps.ts";
import { runPopupLaunch, runPopupSend } from "./launch.ts";

const summaryRegion = document.getElementById("summary");
const sendRegion = document.getElementById("send");
if (summaryRegion !== null && sendRegion !== null) {
  const log = createLogger();
  void (async () => {
    const outcome = await runPopupLaunch(summaryRegion, {
      reader: createSelectionReader(chrome.scripting),
      activeTab: createActiveTabQuery(chrome.tabs),
      log,
    });
    await runPopupSend(sendRegion, outcome, {
      settings: createSettingsStore(chrome.storage.local),
      send: createSendDeps(),
      now: () => Date.now(),
      log,
    });
  })();
}
