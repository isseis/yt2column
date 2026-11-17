// The calls between a page and the Worker that runs the article generation
// module, and the Worker's side of them. A page sends one call at a time and
// matches the answer by callId; the Worker answers each call exactly once,
// with a reply, an error (the function's Promise rejected), or an exit (the
// Go program ended, or a function could not be called), and may send progress
// before the answer. src/wasmWorker.ts wires serveWorkerCalls to the Worker's
// globals; this module touches none, so its rules are tested here.

/** The article generation module's file in the build output; scripts/build-variant.ts writes it there. */
export const wasmModuleFile = "yt2column.wasm";

/** The global the module registers its four functions under (internal/extjob/jsmodule). */
export const wasmNamespace = "yt2column";

/** The module's functions a page calls. */
export type WorkerFunction =
  "checkSettings" | "prepare" | "generate" | "buildInfo";

const workerFunctions: Readonly<Record<WorkerFunction, true>> = {
  checkSettings: true,
  prepare: true,
  generate: true,
  buildInfo: true,
};

/** A call a page posts to the Worker. buildInfo takes no request; its request is "". */
export interface WorkerCall {
  readonly callId: number;
  readonly fn: WorkerFunction;
  readonly request: string;
}

/** What the Worker posts to the page about a call. */
export type WorkerMessage =
  | { readonly callId: number; readonly kind: "reply"; readonly value: string }
  | {
      readonly callId: number;
      readonly kind: "progress";
      readonly event: string;
    }
  | { readonly callId: number; readonly kind: "error" }
  | { readonly callId: number; readonly kind: "exit" };

/** The members of value when it is a plain object with exactly names; undefined otherwise. */
function exactRecord(
  value: unknown,
  names: readonly string[],
): Readonly<Record<string, unknown>> | undefined {
  if (typeof value !== "object" || value === null || Array.isArray(value)) {
    return undefined;
  }
  const keys = Object.keys(value);
  return keys.length === names.length &&
    names.every((name) => Object.hasOwn(value, name))
    ? (value as Readonly<Record<string, unknown>>)
    : undefined;
}

/** True when value is a call ID: a positive safe integer. */
function isCallId(value: unknown): value is number {
  return Number.isSafeInteger(value) && (value as number) > 0;
}

/** The call in data, or undefined when its shape is not exactly a WorkerCall's. */
export function parseWorkerCall(data: unknown): WorkerCall | undefined {
  const record = exactRecord(data, ["callId", "fn", "request"]);
  if (record === undefined) {
    return undefined;
  }
  const { callId, fn, request } = record;
  if (
    !isCallId(callId) ||
    typeof fn !== "string" ||
    !Object.hasOwn(workerFunctions, fn) ||
    typeof request !== "string"
  ) {
    return undefined;
  }
  return { callId, fn: fn as WorkerFunction, request };
}

/** The message in data, or undefined when its shape is not exactly one WorkerMessage's. */
export function parseWorkerMessage(data: unknown): WorkerMessage | undefined {
  if (typeof data !== "object" || data === null) {
    return undefined;
  }
  const kind = (data as { readonly kind?: unknown }).kind;
  switch (kind) {
    case "reply": {
      const record = exactRecord(data, ["callId", "kind", "value"]);
      return record !== undefined &&
        isCallId(record.callId) &&
        typeof record.value === "string"
        ? { callId: record.callId, kind, value: record.value }
        : undefined;
    }
    case "progress": {
      const record = exactRecord(data, ["callId", "kind", "event"]);
      return record !== undefined &&
        isCallId(record.callId) &&
        typeof record.event === "string"
        ? { callId: record.callId, kind, event: record.event }
        : undefined;
    }
    case "error":
    case "exit": {
      const record = exactRecord(data, ["callId", "kind"]);
      return record !== undefined && isCallId(record.callId)
        ? { callId: record.callId, kind }
        : undefined;
    }
    default:
      return undefined;
  }
}

/** The module once it runs. */
export interface LoadedModule {
  /** The module's namespace object, read at each call: the module sets it while it starts. */
  readonly namespace: () => unknown;
  /** Settles when the Go program ends, whatever the reason. */
  readonly exited: Promise<unknown>;
}

/** What the Worker's side needs from the Worker's globals. */
export interface WorkerHost {
  /** Loads and starts the module; rejects when it cannot. */
  load(): Promise<LoadedModule>;
  /** Posts a message to the page. */
  post(message: WorkerMessage): void;
  /** Raises error as the Worker's error event, which the page sees. */
  reportError(error: unknown): void;
}

/**
 * Starts loading the module through host and returns the handler of the
 * page's messages. Contract: every well-formed call is answered exactly once,
 * after the module has loaded: with the function's string result (reply),
 * with error when its Promise rejects or resolves to a non-string, and with
 * exit when the Go program has ended or ends before the answer, when the
 * function is not registered, when calling it throws, and when it returns a
 * value of the wrong kind (a non-Promise from checkSettings or generate, a
 * non-string from prepare or buildInfo: what a panic inside the module's
 * registered function leaves). Progress is posted only while the call is
 * unanswered. A failed load posts nothing and is reported once as the
 * Worker's error event. A malformed call, and a call whose ID is still
 * unanswered, is ignored: it has no ID to answer.
 */
export function serveWorkerCalls(host: WorkerHost): (data: unknown) => void {
  const unanswered = new Set<number>();
  let exited = false;
  const answer = (message: WorkerMessage): void => {
    if (unanswered.delete(message.callId)) {
      host.post(message);
    }
  };
  const exitAll = (): void => {
    exited = true;
    for (const callId of [...unanswered]) {
      answer({ callId, kind: "exit" });
    }
  };
  const loading = host.load().then((loaded) => {
    loaded.exited.then(exitAll, exitAll);
    return loaded;
  });
  loading.catch((error: unknown) => host.reportError(error));

  const run = (call: WorkerCall, loaded: LoadedModule): void => {
    const { callId } = call;
    const exit = (): void => answer({ callId, kind: "exit" });
    if (exited) {
      exit();
      return;
    }
    const namespace = loaded.namespace();
    const fn =
      typeof namespace === "object" && namespace !== null
        ? (namespace as Readonly<Record<string, unknown>>)[call.fn]
        : undefined;
    if (typeof fn !== "function") {
      exit();
      return;
    }
    let returned: unknown;
    try {
      switch (call.fn) {
        case "generate":
          returned = fn(call.request, (event: unknown) => {
            if (typeof event === "string" && unanswered.has(callId)) {
              host.post({ callId, kind: "progress", event });
            }
          });
          break;
        case "buildInfo":
          returned = fn();
          break;
        case "checkSettings":
        case "prepare":
          returned = fn(call.request);
          break;
      }
    } catch {
      exit();
      return;
    }
    if (call.fn === "prepare" || call.fn === "buildInfo") {
      if (typeof returned === "string") {
        answer({ callId, kind: "reply", value: returned });
      } else {
        exit();
      }
      return;
    }
    if (!(returned instanceof Promise)) {
      exit();
      return;
    }
    returned.then(
      (value: unknown) =>
        answer(
          typeof value === "string"
            ? { callId, kind: "reply", value }
            : { callId, kind: "error" },
        ),
      () => answer({ callId, kind: "error" }),
    );
  };

  return (data) => {
    const call = parseWorkerCall(data);
    if (call === undefined || unanswered.has(call.callId)) {
      return;
    }
    unanswered.add(call.callId);
    loading.then(
      (loaded) => run(call, loaded),
      () => {
        // The failed load was reported once, as the Worker's error event.
      },
    );
  };
}
