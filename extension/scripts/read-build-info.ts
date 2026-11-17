// Loads an article generation module the way a page does, with the Go
// toolchain's wasm_exec.js, and prints what its buildInfo function returns:
//
//   node scripts/read-build-info.ts <module.wasm> <wasm_exec.js>
//
// The build's output check (scripts/check-dist.ts, through scripts/build.ts)
// runs it in a process of its own, so the globals wasm_exec.js defines and the
// Go program, which keeps waiting for calls, never enter the build's process.
// It exits 1 when the module registers no buildInfo (a development module
// whose scenario does not parse registers nothing).
import { readFileSync } from "node:fs";
import vm from "node:vm";

/** The global namespace the module registers its functions under. */
const namespaceName = "yt2column";

/** The part of wasm_exec.js's Go class this script uses. */
interface GoRuntime {
  readonly importObject: WebAssembly.Imports;
  run(instance: WebAssembly.Instance): Promise<void>;
}

const [wasmPath, wasmExecPath] = process.argv.slice(2);
if (wasmPath === undefined || wasmExecPath === undefined) {
  console.error("usage: read-build-info.ts <module.wasm> <wasm_exec.js>");
  process.exit(2);
}

// The Go program writes its standard output and error through console.log;
// keep standard output for the one result.
console.log = (...args: unknown[]) => {
  console.error(...args);
};

// wasm_exec.js is a classic script that defines globalThis.Go; it is the
// toolchain's file, the same one the extension loads.
vm.runInThisContext(readFileSync(wasmExecPath, "utf8"), {
  filename: wasmExecPath,
});
const globals = globalThis as unknown as Record<string, unknown>;
const GoClass = globals["Go"] as new () => GoRuntime;
const go = new GoClass();
const { instance } = await WebAssembly.instantiate(
  readFileSync(wasmPath),
  go.importObject,
);
// run returns when the program exits; a module that registered its functions
// keeps waiting for calls, so its Promise never settles, and it has
// registered them by the time run hands control back.
void go.run(instance);
const namespace = globals[namespaceName] as Record<string, unknown> | undefined;
const buildInfo = namespace?.["buildInfo"];
if (typeof buildInfo !== "function") {
  console.error(`the module registered no ${namespaceName}.buildInfo`);
  process.exit(1);
}
process.stdout.write(String((buildInfo as () => unknown)()));
process.exit(0);
