// 0090 PoC runner page (PoC branch only, never merged into main). The service
// worker opens it as an inactive tab with the run ID in the fragment. It
// claims the run's payload from the service worker, starts a dedicated
// worker that loads the Go WebAssembly entry point, sends it one call, and
// reports to the service worker, which keeps the run's record in
// chrome.storage.session for the control page.
//
// The payload's request may hold the API key and the workspace ID. They are
// kept only in this page's memory and the worker's, are posted to the worker
// once, and never appear in the record, on the page, or in a log. The record
// holds the visibility history, timings, step results, and failure kinds.

interface Failure {
  kind: string;
  httpStatus: number | null;
  message: string;
}

interface PocPayload {
  kind: "generate" | "probe";
  request: string;
  workerForm: "classic" | "module";
  deadlineSec: number | null;
}

interface BuildInfo {
  goVersion: string;
  vcsRevision: string | null;
  vcsModified: boolean | null;
}

type WorkerMessage =
  | { type: "ready"; buildInfo: string; loadMs: number }
  | { type: "loadFailed"; message: string }
  | { type: "result"; result: string }
  | { type: "exited" };

const runId = location.hash.slice(1);
const pageStart = performance.now();
/** Identifies this document; a reload gets a new one. */
const instanceId = crypto.randomUUID();
const visibilityAtLoad = document.visibilityState;
const visibilityEvents: { state: string; atMs: number }[] = [];
let everVisible = visibilityAtLoad === "visible";
let finished = false;

const record: Record<string, unknown> = {};

function element(id: string): HTMLElement {
  const found = document.getElementById(id);
  if (found === null) {
    throw new Error(`missing element #${id}`);
  }
  return found;
}

function visibility(): Record<string, unknown> {
  return {
    visibilityAtLoad,
    visibilityEvents: [...visibilityEvents],
    everVisible,
    visibilityAtReport: document.visibilityState,
  };
}

/** Merges update into the record, shows it, and sends it to the service worker. */
function report(update: Record<string, unknown>): void {
  Object.assign(record, update, visibility());
  element("runner-status").textContent = String(record["status"] ?? "");
  element("runner-record").textContent = JSON.stringify(record, null, 2);
  void chrome.runtime
    .sendMessage({ poc: "pocUpdate", runId, record: { ...record } })
    .catch(() => undefined);
}

document.addEventListener("visibilitychange", () => {
  if (document.visibilityState === "visible") {
    everVisible = true;
  }
  visibilityEvents.push({
    state: document.visibilityState,
    atMs: Math.round(performance.now() - pageStart),
  });
  if (!finished) {
    report({});
  }
});

// The liveness query the design uses: a ping for this run on the PoC channel
// gets a pong with this document's instance ID, answered from the main
// thread while the worker generates.
const pingChannel = new BroadcastChannel("poc0090-runner");
pingChannel.addEventListener("message", (event: MessageEvent<unknown>) => {
  const data = event.data as {
    type?: unknown;
    runId?: unknown;
    pingId?: unknown;
  };
  if (data.type === "ping" && data.runId === runId) {
    pingChannel.postMessage({
      type: "pong",
      runId,
      pingId: data.pingId,
      instanceId,
      status: record["status"] ?? null,
      visibilityState: document.visibilityState,
    });
  }
});

/** The part of a generate result the record keeps: no article text. */
function summarizeGenerate(parsed: {
  ok: boolean;
  markdown?: string;
  failure?: Failure;
  stages?: unknown[];
}): Record<string, unknown> {
  return {
    ok: parsed.ok,
    articleChars: parsed.markdown?.length ?? null,
    failure: parsed.failure ?? null,
    stages: parsed.stages ?? null,
    // A transport failure is the only kind that means fetch itself failed.
    workerFetchOk: parsed.failure?.kind !== "transport",
  };
}

function summarizeProbe(parsed: {
  failure: Failure | null;
}): Record<string, unknown> {
  return {
    probe: parsed,
    workerFetchOk: parsed.failure?.kind !== "transport",
  };
}

async function main(): Promise<void> {
  report({
    status: "claiming",
    runnerLoadedAtMs: Math.round(pageStart),
    instanceId,
  });
  const answer = (await chrome.runtime.sendMessage({
    poc: "pocClaim",
    runId,
  })) as { payload?: PocPayload; error?: string } | undefined;
  const payload = answer?.payload;
  if (payload === undefined) {
    report({ status: "claimFailed", error: answer?.error ?? "no answer" });
    return;
  }
  report({ status: "loading", workerForm: payload.workerForm });
  const worker =
    payload.workerForm === "module"
      ? new Worker("poc-worker-module.js", { type: "module" })
      : new Worker("poc-worker-classic.js");
  let callStart = 0;
  let deadlineTimer: ReturnType<typeof setTimeout> | undefined;
  const finish = (update: Record<string, unknown>): void => {
    if (finished) {
      return;
    }
    finished = true;
    clearTimeout(deadlineTimer);
    worker.terminate();
    report({
      ...update,
      callElapsedMs:
        callStart === 0 ? null : Math.round(performance.now() - callStart),
      terminatedAtMs: Math.round(performance.now() - pageStart),
    });
  };
  worker.addEventListener("error", (event) => {
    finish({ status: "workerError", error: event.message || "error event" });
  });
  worker.addEventListener("messageerror", () => {
    finish({ status: "workerError", error: "messageerror" });
  });
  worker.addEventListener("message", (event: MessageEvent<WorkerMessage>) => {
    const message = event.data;
    switch (message.type) {
      case "loadFailed":
        finish({ status: "loadFailed", error: message.message });
        return;
      case "exited":
        finish({ status: "runtimeExited" });
        return;
      case "ready": {
        const build = JSON.parse(message.buildInfo) as BuildInfo;
        report({ status: "ready", build, workerLoadMs: message.loadMs });
        if (build.vcsModified !== false) {
          finish({ status: "refused: build not from a clean tree" });
          return;
        }
        callStart = performance.now();
        worker.postMessage({ call: payload.kind, request: payload.request });
        report({
          status: "running",
          callStartedAtMs: Math.round(callStart - pageStart),
        });
        if (payload.deadlineSec !== null) {
          deadlineTimer = setTimeout(() => {
            finish({ status: "deadline: worker terminated" });
          }, payload.deadlineSec * 1000);
        }
        return;
      }
      case "result": {
        const parsed = JSON.parse(message.result) as unknown;
        const summary =
          payload.kind === "generate"
            ? summarizeGenerate(
                parsed as Parameters<typeof summarizeGenerate>[0],
              )
            : summarizeProbe(parsed as Parameters<typeof summarizeProbe>[0]);
        finish({ status: "finished", ...summary });
        return;
      }
    }
  });
}

void main();
