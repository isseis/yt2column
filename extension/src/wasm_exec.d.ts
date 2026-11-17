// The type of the Go toolchain's wasm_exec.js, which the build copies next to
// the compiled scripts (scripts/build.ts). It is the toolchain's file, not
// this repository's: importing it for its side effect defines the Go class on
// the global object. Only src/wasmWorker.ts imports it.

declare global {
  /** The runtime of a Go program compiled to WebAssembly. */
  class Go {
    /** The imports the module is instantiated with. */
    readonly importObject: WebAssembly.Imports;
    /** Runs the program; settles when it exits. */
    run(instance: WebAssembly.Instance): Promise<void>;
  }
}

export {};
