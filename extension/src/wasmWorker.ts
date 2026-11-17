// The Worker that runs the article generation module: it loads the module with
// the Go toolchain's wasm_exec.js and answers the page's calls through
// serveWorkerCalls, which holds every rule. This file only wires the Worker's
// globals to it, and is the one file the lint rules let fetch: it fetches the
// module from the extension's own files. The module's Go code makes the API
// calls.
import "./wasm_exec.js";

import {
  serveWorkerCalls,
  wasmModuleFile,
  wasmNamespace,
} from "./core/workerCalls.ts";

const handle = serveWorkerCalls({
  load: async () => {
    const go = new Go();
    const { instance } = await WebAssembly.instantiateStreaming(
      fetch(wasmModuleFile),
      go.importObject,
    );
    // run registers the module's functions before it hands control back;
    // its Promise settles only when the Go program ends.
    const exited = go.run(instance);
    return {
      namespace: () =>
        (globalThis as unknown as Readonly<Record<string, unknown>>)[
          wasmNamespace
        ],
      exited,
    };
  },
  post: (message) => postMessage(message),
  reportError: (error) => reportError(error),
});

addEventListener("message", (event) => handle(event.data));
