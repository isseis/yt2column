// The options-page entry point. It builds the real dependencies in
// chromeDeps.ts and calls the options path; the logic lives in optionsPage.ts.
import { createLogger, createSettingsStore } from "./browser/chromeDeps.ts";
import { runOptionsPage } from "./optionsPage.ts";

void runOptionsPage(document.body, {
  store: createSettingsStore(chrome.storage.local),
  log: createLogger(),
});
