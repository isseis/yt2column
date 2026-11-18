// The options page entry point. It builds the real dependencies in
// chromeDeps.ts and the Worker client, and hands them to startOptions; the
// page's path lives in optionsApp.ts and its flows in optionsFlow.ts.
import {
  localKeys,
  readUiLanguage,
  removeAnthropicSettings,
  schedule,
  writeAnthropicSettings,
} from "./browser/chromeDeps.ts";
import { startOptions } from "./optionsApp.ts";
import { createModuleWorker, startWasmWorker } from "./wasmWorkerClient.ts";

void startOptions(document, {
  uiLanguage: () => readUiLanguage(chrome.i18n),
  flow: {
    startWorker: () => startWasmWorker(createModuleWorker, schedule),
    schedule,
    write: writeAnthropicSettings,
    remove: removeAnthropicSettings,
    keys: localKeys,
  },
  onVisible: (listener) =>
    document.addEventListener("visibilitychange", () => {
      if (document.visibilityState === "visible") {
        listener();
      }
    }),
});
