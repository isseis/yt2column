// 0090 PoC (PoC branch only): the module-worker form. The imports run in
// order, so globalThis.Go exists before poc-worker-core.js runs.
import "./poc/wasm_exec.js";
import "./poc-worker-core.js";
