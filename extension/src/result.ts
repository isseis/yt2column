// The result-window entry point. It builds the real dependencies in
// chromeDeps.ts and calls the result path; the path logic lives in launch.ts.
import {
  createLogger,
  createResultViewStore,
  schedule,
} from "./browser/chromeDeps.ts";
import { runResultWindow } from "./launch.ts";

void runResultWindow(document.body, location.hash, {
  store: createResultViewStore(
    chrome.storage.session,
    chrome.storage.onChanged,
  ),
  schedule,
  log: createLogger(),
});
