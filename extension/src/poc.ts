// The 0084 PoC page (PoC branch only, never merged into main). It loads the
// Go WebAssembly build of the pipeline, calls it only when a button is
// pressed, and shows the result. The API key and the workspace ID are read
// from their fields at the press and passed to the call; they are never
// written to chrome.storage, Web Storage, IndexedDB, a cookie, the URL, the
// console, or the record shown on the page. The 0084 calls send no message to
// the service worker. The 0090 additions at the end do: a background run
// passes its request (with the key, for generate) to the service worker's
// memory, and the access-level check writes a dummy value to storage.local.
/* eslint-disable no-restricted-syntax -- PoC: the 0090 checks use chrome.storage. */
import { parseArticleMarkdown } from "./core/articleMarkdown.ts";
import { buildArticleBody } from "./ui/articleBody.ts";
import { pingRunner } from "./pocPing.ts";

interface BuildInfo {
  goVersion: string;
  vcsRevision: string | null;
  vcsModified: boolean | null;
}

interface Yt2ColumnWasm {
  buildInfo(): string;
  generate(request: string): Promise<string>;
  longOutput(request: string): Promise<string>;
  probe(request: string): Promise<string>;
}

interface GoRuntime {
  importObject: WebAssembly.Imports;
  run(instance: WebAssembly.Instance): Promise<void>;
}

interface PocGlobal {
  Go?: new () => GoRuntime;
  yt2column?: Yt2ColumnWasm;
  yt2columnReady?: () => void;
}

interface Failure {
  kind: string;
  httpStatus: number | null;
  message: string;
}

interface StageReport {
  stage: string;
  status: string;
  elapsedMs: number | null;
  requestedModel: string | null;
  model: string | null;
  inputTokens: number | null;
  outputTokens: number | null;
  costUsd: string | null;
}

interface GenerateResult {
  ok: boolean;
  markdown?: string;
  failure?: Failure;
  stages?: StageReport[];
}

type CallName = "generate" | "longOutput" | "probe";

/**
 * A call that has not resolved after this long is shown as stalled. generate
 * runs up to three LLM calls of at most 15 minutes each, so its limit is past
 * 45 minutes; longOutput is one such call; a probe call that stalls inside a
 * response body is expected never to end.
 */
const stallAfterMs: Record<CallName, number> = {
  generate: 50 * 60 * 1000,
  longOutput: 20 * 60 * 1000,
  probe: 20 * 60 * 1000,
};

/** Failure kinds whose call's settings are not recorded (see runCall). */
const settingsWithheldKinds = new Set(["invalid_input", "invalid_config"]);

const pocGlobal = globalThis as unknown as PocGlobal;

/** Page state. Only one call runs at a time. */
const state = {
  /** "loading" until the runtime is ready; "failed" or "ended" stop all calls. */
  runtime: "loading" as "loading" | "ready" | "failed" | "ended",
  runtimeReason: "",
  build: null as BuildInfo | null,
  loadMs: null as number | null,
  running: null as CallName | null,
  /** Set when a call passed its stallAfterMs; cleared only by reloading the page. */
  stalled: false,
};

function element<T extends HTMLElement>(id: string): T {
  const found = document.getElementById(id);
  if (found === null) {
    throw new Error(`missing element #${id}`);
  }
  return found as T;
}

function fieldValue(id: string): string {
  return element<HTMLInputElement | HTMLTextAreaElement | HTMLSelectElement>(id)
    .value;
}

function setStatus(id: string, text: string): void {
  element(id).textContent = text;
}

/** Appends one record line. The record never holds a key or a body text. */
function appendRecord(entry: Record<string, unknown>): void {
  const record = element("record");
  record.textContent += `${JSON.stringify(entry)}\n`;
}

function showRuntime(): void {
  const list = element("runtime");
  list.replaceChildren();
  const rows: [string, string][] = [
    ["State", state.runtime],
    ["Go version", state.build?.goVersion ?? "-"],
    ["vcs.revision", state.build?.vcsRevision ?? "(not stamped)"],
    [
      "vcs.modified",
      state.build?.vcsModified === null || state.build === null
        ? "(not stamped)"
        : String(state.build.vcsModified),
    ],
    [
      "Load time (ms, from navigation start)",
      state.loadMs === null ? "-" : state.loadMs.toFixed(1),
    ],
  ];
  for (const [name, value] of rows) {
    const term = document.createElement("dt");
    term.textContent = name;
    const detail = document.createElement("dd");
    detail.textContent = value;
    list.append(term, detail);
  }
  setStatus("runtime-status", state.runtimeReason);
}

/**
 * Returns why no call may start now, or null when one may. A build whose
 * working tree was modified, or whose state was not stamped, cannot be tied
 * to a commit, so measuring it is refused.
 */
function blockedReason(name: CallName): string | null {
  switch (state.runtime) {
    case "loading":
      return "The WebAssembly module is not ready yet.";
    case "failed":
      return `The WebAssembly module failed to load: ${state.runtimeReason}`;
    case "ended":
      return "The WebAssembly runtime has exited. Reload the page.";
    case "ready":
      break;
  }
  if (state.build?.vcsModified !== false) {
    return "The build is not from a clean, committed tree; rebuild with make poc-build.";
  }
  if (state.stalled) {
    return "A call was treated as stalled. Reload the page before starting another.";
  }
  if (state.running !== null) {
    return `A call (${state.running}) is running.`;
  }
  if (name !== "probe") {
    if (fieldValue("api-key") === "" || fieldValue("workspace-id") === "") {
      return "Enter the API key and the workspace ID.";
    }
  }
  return null;
}

interface VisibilitySummary {
  hiddenMs: number;
  hiddenTransitions: number;
}

/**
 * Tracks how long the tab was hidden while one call ran. peek reads the
 * values so far and keeps tracking; stop reads them and removes the listener.
 */
function visibilityTracker(): {
  peek: () => VisibilitySummary;
  stop: () => VisibilitySummary;
} {
  let hiddenSince =
    document.visibilityState === "hidden" ? performance.now() : null;
  let hiddenMs = 0;
  let hiddenTransitions = 0;
  const listener = (): void => {
    if (document.visibilityState === "hidden") {
      hiddenSince = performance.now();
      hiddenTransitions += 1;
    } else if (hiddenSince !== null) {
      hiddenMs += performance.now() - hiddenSince;
      hiddenSince = null;
    }
  };
  document.addEventListener("visibilitychange", listener);
  const peek = (): VisibilitySummary => {
    const open = hiddenSince === null ? 0 : performance.now() - hiddenSince;
    return { hiddenMs: Math.round(hiddenMs + open), hiddenTransitions };
  };
  return {
    peek,
    stop: () => {
      document.removeEventListener("visibilitychange", listener);
      return peek();
    },
  };
}

/**
 * Runs one call: checks the button rules, marks the page busy, and gives the
 * parsed result to show. A call that passes its stallAfterMs is marked
 * stalled, which blocks every later call until the page is reloaded, because
 * a send that stalls inside a response body is expected not to end. A result
 * that still arrives after that is shown and recorded in full, marked late.
 */
async function runCall(
  name: CallName,
  request: () => string,
  settings: Record<string, unknown>,
  show: (result: unknown) => Record<string, unknown>,
): Promise<void> {
  const reason = blockedReason(name);
  if (reason !== null) {
    setStatus("call-status", reason);
    return;
  }
  const api = pocGlobal.yt2column;
  if (api === undefined) {
    setStatus("call-status", "The WebAssembly module is not registered.");
    return;
  }
  state.running = name;
  setStatus("call-status", `Running ${name}...`);
  element("article").hidden = true;
  element("stages").hidden = true;
  const startedAt = new Date().toISOString();
  const start = performance.now();
  const tracking = visibilityTracker();
  const limitMs = stallAfterMs[name];
  const stallTimer = setTimeout(() => {
    state.stalled = true;
    state.running = null;
    setStatus(
      "call-status",
      `${name} did not finish within ${limitMs / 60000} minutes; it is treated as stalled. Reload the page before starting another call.`,
    );
    appendRecord({
      call: name,
      startedAt,
      build: state.build,
      settings,
      stalledAfterMs: limitMs,
      visibility: tracking.peek(),
    });
  }, limitMs);
  let resultText: string;
  try {
    resultText = await api[name](request());
  } catch {
    // The Go side never rejects; reaching here means the runtime is gone.
    resultText = JSON.stringify({
      failure: { kind: "panic", httpStatus: null, message: "the call threw" },
      stages: [],
    });
  }
  clearTimeout(stallTimer);
  const pageElapsedMs = Math.round(performance.now() - start);
  const late = state.stalled;
  const visibility = tracking.stop();
  if (!late) {
    state.running = null;
  }
  const parsed = JSON.parse(resultText) as unknown;
  // Set before show, which appends the failure or the result to it.
  setStatus(
    "call-status",
    late
      ? `${name} finished late, in ${pageElapsedMs} ms, after it was treated as stalled. Reload the page before starting another call.`
      : `${name} finished in ${pageElapsedMs} ms.`,
  );
  const summary = show(parsed);
  // A rejected input can be a key pasted into the wrong field, so the
  // settings of a call rejected by validation are not recorded.
  const failureKind = (parsed as { failure?: { kind?: string } | null }).failure
    ?.kind;
  const withheld =
    failureKind !== undefined && settingsWithheldKinds.has(failureKind);
  appendRecord({
    call: name,
    startedAt,
    userAgent: navigator.userAgent,
    build: state.build,
    loadMs: state.loadMs,
    settings: withheld ? `(withheld: ${failureKind})` : settings,
    pageElapsedMs,
    late,
    visibility,
    ...summary,
  });
}

function showFailure(failure: Failure): void {
  const status = failure.httpStatus === null ? "" : ` (${failure.httpStatus})`;
  element("call-status").textContent +=
    ` Failed: ${failure.kind}${status}: ${failure.message}`;
}

function showStages(stages: StageReport[] | undefined): void {
  if (stages === undefined) {
    return;
  }
  const table = element<HTMLTableElement>("stages");
  const body = table.tBodies[0];
  if (body === undefined) {
    return;
  }
  body.replaceChildren();
  for (const stage of stages) {
    const row = document.createElement("tr");
    for (const value of [
      stage.stage,
      stage.status,
      stage.elapsedMs,
      stage.requestedModel,
      stage.model,
      stage.inputTokens,
      stage.outputTokens,
      stage.costUsd,
    ]) {
      const cell = document.createElement("td");
      cell.textContent = value === null ? "unknown" : String(value);
      row.append(cell);
    }
    body.append(row);
  }
  table.hidden = false;
}

function apiCallMembers(): {
  apiKey: string;
  workspaceId: string;
  browserAccessHeader: boolean;
} {
  return {
    apiKey: fieldValue("api-key"),
    workspaceId: fieldValue("workspace-id"),
    browserAccessHeader: element<HTMLInputElement>("browser-access").checked,
  };
}

function generateSettings(): {
  generation: { model: string; effort: string };
  review: { model: string; effort: string };
  browserAccessHeader: boolean;
} {
  return {
    generation: {
      model: fieldValue("generation-model"),
      effort: fieldValue("generation-effort"),
    },
    review: {
      model: fieldValue("review-model"),
      effort: fieldValue("review-effort"),
    },
    browserAccessHeader: element<HTMLInputElement>("browser-access").checked,
  };
}

/** The generate request. It holds the API key: never record or log it. */
function generateRequest(
  settings: ReturnType<typeof generateSettings>,
): string {
  const request: Record<string, unknown> = {
    ...apiCallMembers(),
    videoUrl: fieldValue("video-url"),
    transcript: fieldValue("transcript"),
    generation: settings.generation,
    review: settings.review,
  };
  // Absent optional members, as the extension sends them to the server.
  for (const [key, id] of [
    ["title", "title"],
    ["channelName", "channel-name"],
    ["description", "description"],
  ] as const) {
    const value = fieldValue(id);
    if (value !== "") {
      request[key] = value;
    }
  }
  return JSON.stringify(request);
}

function generate(): Promise<void> {
  const settings = generateSettings();
  return runCall(
    "generate",
    () => generateRequest(settings),
    { ...settings, transcriptChars: fieldValue("transcript").length },
    (result) => {
      const generated = result as GenerateResult;
      showStages(generated.stages);
      if (generated.ok && generated.markdown !== undefined) {
        const container = element("article");
        container.replaceChildren(
          buildArticleBody(document, parseArticleMarkdown(generated.markdown)),
        );
        container.hidden = false;
        return {
          ok: true,
          articleChars: generated.markdown.length,
          stages: generated.stages,
        };
      }
      if (generated.failure !== undefined) {
        showFailure(generated.failure);
      }
      return {
        ok: false,
        failure: generated.failure,
        stages: generated.stages,
      };
    },
  );
}

function longOutput(): Promise<void> {
  const settings = {
    setting: {
      model: fieldValue("long-model"),
      effort: fieldValue("long-effort"),
    },
    variant: fieldValue("long-variant"),
    browserAccessHeader: element<HTMLInputElement>("browser-access").checked,
  };
  return runCall(
    "longOutput",
    () =>
      JSON.stringify({
        ...apiCallMembers(),
        setting: settings.setting,
        variant: settings.variant,
      }),
    settings,
    (result) => {
      const measured = result as { failure: Failure | null };
      if (measured.failure !== null) {
        showFailure(measured.failure);
      }
      element("call-status").textContent += ` ${JSON.stringify(result)}`;
      return { result };
    },
  );
}

function probe(): Promise<void> {
  const settings = {
    baseUrl: fieldValue("probe-base"),
    foreignUrl: fieldValue("probe-foreign"),
    probeCase: fieldValue("probe-case"),
  };
  return runCall(
    "probe",
    () => JSON.stringify(settings),
    // The URL fields are free text, so only the case is recorded; the probe
    // server's log records where each request arrived.
    { probeCase: settings.probeCase },
    (result) => {
      const probed = result as { failure: Failure | null };
      if (probed.failure !== null) {
        showFailure(probed.failure);
      }
      element("call-status").textContent += ` ${JSON.stringify(result)}`;
      return { result };
    },
  );
}

/** Loads wasm_exec.js as a classic script, which defines globalThis.Go. */
function loadGlue(): Promise<void> {
  return new Promise((resolve, reject) => {
    const script = document.createElement("script");
    script.src = "poc/wasm_exec.js";
    script.addEventListener("load", () => resolve());
    script.addEventListener("error", () =>
      reject(new Error("poc/wasm_exec.js could not be loaded")),
    );
    document.head.append(script);
  });
}

function failLoad(reason: string): void {
  state.runtime = "failed";
  state.runtimeReason = reason;
  showRuntime();
}

async function loadRuntime(): Promise<void> {
  showRuntime();
  try {
    await loadGlue();
  } catch (error) {
    failLoad(error instanceof Error ? error.message : String(error));
    return;
  }
  const GoClass = pocGlobal.Go;
  if (GoClass === undefined) {
    failLoad("wasm_exec.js did not define Go");
    return;
  }
  const go = new GoClass();
  pocGlobal.yt2columnReady = () => {
    // performance.now() counts from the navigation start (timeOrigin).
    state.loadMs = performance.now();
    const api = pocGlobal.yt2column;
    if (api === undefined) {
      failLoad("the module did not register yt2column");
      return;
    }
    const info = JSON.parse(api.buildInfo()) as
      BuildInfo | { failure: Failure };
    if ("failure" in info) {
      failLoad(`buildInfo failed: ${info.failure.message}`);
      return;
    }
    state.build = info;
    state.runtime = "ready";
    state.runtimeReason =
      state.build.vcsModified === false
        ? "Ready."
        : "Ready, but the build is not from a clean, committed tree; measurement buttons are disabled.";
    if (state.build.vcsModified !== false) {
      for (const id of ["generate", "long-output", "probe"]) {
        element<HTMLButtonElement>(id).disabled = true;
      }
    }
    showRuntime();
  };
  let instance: WebAssembly.Instance;
  try {
    const source = await WebAssembly.instantiateStreaming(
      // PoC exception: this reads the module packaged with the extension
      // (same origin); the PoC's API sends happen inside Go, through fetch.
      // eslint-disable-next-line no-restricted-globals
      fetch("poc/yt2column.wasm"),
      go.importObject,
    );
    instance = source.instance;
  } catch (error) {
    failLoad(error instanceof Error ? error.message : String(error));
    return;
  }
  void go.run(instance).then(() => {
    state.runtime = "ended";
    state.running = null;
    state.runtimeReason = "The WebAssembly runtime has exited.";
    showRuntime();
  });
}

// ---- 0090 additions -------------------------------------------------------
// The background-runner runs, the storage.local access-level check, and the
// storage.session quota check. The run payload sent to the service worker
// holds the API key for a generate run; it stays in the service worker's
// memory until the runner claims it and is never stored. The run records in
// chrome.storage.session are written by the service worker from the runner's
// reports and never hold the key, the workspace ID, or any text.

/** Why a background run may not start now, or null. */
function backgroundBlockedReason(kind: "generate" | "probe"): string | null {
  if (state.runtime !== "ready" || state.build?.vcsModified !== false) {
    return "The page's WebAssembly module must be ready and built from a clean, committed tree.";
  }
  if (kind === "generate") {
    if (fieldValue("api-key") === "" || fieldValue("workspace-id") === "") {
      return "Enter the API key and the workspace ID.";
    }
  }
  return null;
}

/** The deadline field: empty is none; anything but a positive integer is refused. */
function deadlineSec(): number | null | "invalid" {
  const raw = fieldValue("deadline-sec");
  if (raw === "") {
    return null;
  }
  return /^[1-9][0-9]{0,4}$/.test(raw) ? Number(raw) : "invalid";
}

async function startBackground(kind: "generate" | "probe"): Promise<void> {
  const reason = backgroundBlockedReason(kind);
  const deadline = deadlineSec();
  if (reason !== null || deadline === "invalid") {
    setStatus(
      "bg-status",
      reason ?? "The deadline must be empty or a positive integer.",
    );
    return;
  }
  const workerForm =
    fieldValue("worker-form") === "module" ? "module" : "classic";
  let request: string;
  let settings: Record<string, unknown>;
  if (kind === "generate") {
    const generation = generateSettings();
    request = generateRequest(generation);
    settings = {
      ...generation,
      transcriptChars: fieldValue("transcript").length,
    };
  } else {
    const probeCase = fieldValue("probe-case");
    request = JSON.stringify({
      baseUrl: fieldValue("probe-base"),
      foreignUrl: fieldValue("probe-foreign"),
      probeCase,
    });
    settings = { probeCase };
  }
  settings = {
    ...settings,
    build: state.build,
    userAgent: navigator.userAgent,
  };
  const answer = (await chrome.runtime.sendMessage({
    poc: "pocStart",
    payload: { kind, request, workerForm, deadlineSec: deadline, settings },
  })) as { runId?: string; error?: string } | undefined;
  setStatus(
    "bg-status",
    answer?.runId !== undefined
      ? `Started run ${answer.runId} (${kind}, ${workerForm}).`
      : `Could not start: ${answer?.error ?? "no answer"}`,
  );
}

async function runRecords(): Promise<
  { runId?: string; requestedAt?: string }[]
> {
  const all = await chrome.storage.session.get(null);
  return Object.entries(all)
    .filter(([key]) => key.startsWith("pocRun:"))
    .map(([, value]) => value as { runId?: string; requestedAt?: string })
    .sort((a, b) => (a.requestedAt ?? "").localeCompare(b.requestedAt ?? ""));
}

/** Pings a runner from this (foreground) page and appends the result. */
async function pingFromPage(): Promise<void> {
  let runId = fieldValue("ping-run-id");
  if (runId === "") {
    runId = (await runRecords()).at(-1)?.runId ?? "";
  }
  if (runId === "") {
    element("ping-result").textContent += "no run to ping\n";
    return;
  }
  const result = await pingRunner(runId);
  element("ping-result").textContent +=
    `${JSON.stringify({ runId, ...result })}\n`;
}

async function showRuns(): Promise<void> {
  const runs = await runRecords();
  element("runs").textContent = runs
    .map((run) => JSON.stringify(run))
    .join("\n");
}

const accessDummyKey = "poc0090AccessDummy";
const accessDummyValue = "dummy-0090-not-a-secret";

/** Reads chrome.storage.local from the probe server's tab, in the isolated world. */
async function readFromInjected(tabId: number): Promise<unknown> {
  try {
    const [injection] = await chrome.scripting.executeScript({
      target: { tabId },
      args: [accessDummyKey],
      func: async (key: string) => {
        const local = (
          globalThis as unknown as {
            chrome?: {
              storage?: {
                local?: { get(k: string): Promise<Record<string, unknown>> };
              };
            };
          }
        ).chrome?.storage?.local;
        if (local === undefined) {
          return { kind: "no-api" };
        }
        try {
          const got = await local.get(key);
          return { kind: "value", value: got[key] ?? null };
        } catch (error) {
          return { kind: "error", message: String(error) };
        }
      },
    });
    return injection?.result ?? { kind: "no-result" };
  } catch (error) {
    return { kind: "inject-failed", message: String(error) };
  }
}

async function accessCheck(): Promise<void> {
  const out: Record<string, unknown> = { userAgent: navigator.userAgent };
  const show = (): void => {
    element("access-result").textContent = JSON.stringify(out, null, 2);
  };
  const [tab] = await chrome.tabs.query({ url: "http://127.0.0.1/*" });
  if (tab?.id === undefined) {
    out["error"] = "Open http://127.0.0.1:8787/ in a tab first.";
    show();
    return;
  }
  const local = chrome.storage.local as unknown as {
    setAccessLevel?: (options: { accessLevel: string }) => Promise<void>;
  };
  await chrome.storage.local.set({ [accessDummyKey]: accessDummyValue });
  out["injectedBefore"] = await readFromInjected(tab.id);
  out["setAccessLevelType"] = typeof local.setAccessLevel;
  try {
    await local.setAccessLevel?.({ accessLevel: "TRUSTED_CONTEXTS" });
    out["setAccessLevel"] = "resolved";
  } catch (error) {
    out["setAccessLevel"] = `threw: ${String(error)}`;
  }
  out["injectedAfter"] = await readFromInjected(tab.id);
  // Restore the default and remove the dummy value.
  try {
    await local.setAccessLevel?.({
      accessLevel: "TRUSTED_AND_UNTRUSTED_CONTEXTS",
    });
    out["restore"] = "resolved";
  } catch (error) {
    out["restore"] = `threw: ${String(error)}`;
  }
  await chrome.storage.local.remove(accessDummyKey);
  show();
}

async function quotaCheck(): Promise<void> {
  const out: Record<string, unknown> = {
    userAgent: navigator.userAgent,
    quotaBytes: (chrome.storage.session as unknown as { QUOTA_BYTES?: number })
      .QUOTA_BYTES,
  };
  const chunk = "a".repeat(256 * 1024);
  const keys: string[] = [];
  let failure: string | null = null;
  for (let i = 0; i < 200; i += 1) {
    const key = `pocQuota:${i}`;
    try {
      await chrome.storage.session.set({ [key]: chunk });
      keys.push(key);
    } catch (error) {
      failure = String(error);
      break;
    }
  }
  out["recordsWritten"] = keys.length;
  out["bytesInUseAtStop"] = await chrome.storage.session.getBytesInUse(null);
  out["stopReason"] = failure ?? "reached 200 records without an error";
  await chrome.storage.session.remove(keys);
  out["bytesInUseAfterClear"] =
    await chrome.storage.session.getBytesInUse(null);
  element("quota-result").textContent = JSON.stringify(out, null, 2);
}

element("bg-generate").addEventListener(
  "click",
  () => void startBackground("generate"),
);
element("bg-probe").addEventListener(
  "click",
  () => void startBackground("probe"),
);
element("ping-runner").addEventListener("click", () => void pingFromPage());
element("access-check").addEventListener("click", () => void accessCheck());
element("quota-check").addEventListener("click", () => void quotaCheck());
chrome.storage.session.onChanged.addListener(() => void showRuns());
void showRuns();

element("generate").addEventListener("click", () => void generate());
element("long-output").addEventListener("click", () => void longOutput());
element("probe").addEventListener("click", () => void probe());
void loadRuntime();
