// 0090 PoC (PoC branch only, never merged into main): the body of the
// dedicated worker that runs the Go WebAssembly entry point. It is plain
// JavaScript with no import or export, so poc-worker-classic.js can load it
// with importScripts and poc-worker-module.js with a static import. Both load
// wasm_exec.js first, which defines globalThis.Go.
//
// Protocol with the runner page:
//   worker -> page: {type: "ready", buildInfo, loadMs} once the module runs,
//                   or {type: "loadFailed", message} when it cannot load;
//   page -> worker: {call, request} (call is "generate" or "probe");
//   worker -> page: {type: "result", result} with the entry point's JSON string,
//                   or {type: "exited"} when the Go runtime exits.
// The request may hold the API key; it is never posted back or logged.
/* global WebAssembly, fetch, performance */
/* eslint-disable no-restricted-globals -- PoC: loads the packaged module. */
"use strict";
(() => {
  const scope = globalThis;
  const post = (message) => scope.postMessage(message);
  const ready = new Promise((resolve, reject) => {
    scope.yt2columnReady = () => resolve();
    const go = new scope.Go();
    WebAssembly.instantiateStreaming(
      fetch("poc/yt2column.wasm"),
      go.importObject,
    )
      .then((source) => {
        void go.run(source.instance).then(() => post({ type: "exited" }));
      })
      .catch(reject);
  });
  ready.then(
    () => {
      post({
        type: "ready",
        buildInfo: scope.yt2column.buildInfo(),
        loadMs: performance.now(),
      });
    },
    (error) => {
      post({
        type: "loadFailed",
        message: error instanceof Error ? error.message : String(error),
      });
    },
  );
  let started = false;
  scope.onmessage = async (event) => {
    if (started) {
      return;
    }
    started = true;
    const { call, request } = event.data;
    await ready;
    const api = scope.yt2column;
    if (call !== "generate" && call !== "probe") {
      return;
    }
    const result = await api[call](request);
    post({ type: "result", result });
  };
})();
