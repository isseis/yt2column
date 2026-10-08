// The result-window entry point. It builds the real dependencies in
// chromeDeps.ts and calls the result path; the path logic lives in launch.ts.
import { createLogger, createSummaryStore } from "./browser/chromeDeps.ts";
import { runResultWindow } from "./launch.ts";

void runResultWindow(document.body, location.hash, {
  store: createSummaryStore(chrome.storage.session),
  log: createLogger(),
});
