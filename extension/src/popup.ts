// The popup entry point. It builds the real dependencies in chromeDeps.ts and
// calls the popup and send paths; the path logic lives in launch.ts. The
// summary goes into one region and the send controls into the other.
import {
  createActiveTabQuery,
  createLogger,
  createMetadataReader,
  createSelectionReader,
  createSendDeps,
} from "./browser/chromeDeps.ts";
import { runPopupLaunch, runPopupSend } from "./launch.ts";

const summaryRegion = document.getElementById("summary");
const sendRegion = document.getElementById("send");
if (summaryRegion !== null && sendRegion !== null) {
  const log = createLogger();
  void (async () => {
    const launched = await runPopupLaunch(summaryRegion, {
      reader: createSelectionReader(chrome.scripting),
      metadataReader: createMetadataReader(chrome.scripting),
      activeTab: createActiveTabQuery(chrome.tabs),
      log,
      // English until the popup entry resolves the browser's UI language.
      locale: "en",
    });
    await runPopupSend(sendRegion, launched, {
      send: createSendDeps(chrome.runtime),
      now: () => Date.now(),
      log,
      locale: "en",
    });
  })();
}
