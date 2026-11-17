// The running viewer's flow in the extension-only path: it takes its input
// from the service worker, checks it in a Worker that runs the article
// generation module, reserves the run, reads the saved settings, calls
// generate, reports progress and the result to the service worker, and stops
// the Worker when its own record says another viewer took the run over. The
// page script wires the real Worker, messaging, storage functions and clock
// into RunnerDeps; the order of the calls, every deadline and every retry are
// decided here.
//
// The input (with the copied transcript) and the API key stay in this flow's
// locals: the input goes to the Worker and nowhere else, and the saved
// settings are turned into the generate request at once, unchecked, and
// dropped. Neither is passed to a hook, a message to the service worker or a
// log.

import type {
  onSessionChanged,
  readAnthropicSettings,
} from "./browser/chromeDeps.ts";
import type { BrowserCall } from "./browser/chromeDeps.ts";
import { jobItemName, readJobRecord } from "./core/jobRecord.ts";
import type { RecordFailure } from "./core/jobRecord.ts";
import {
  parseClaimInputReply,
  parseReserveReply,
  parseWriteReply,
} from "./core/jobMessages.ts";
import type {
  ReserveReply,
  RunResult,
  RunnerHello,
  StartPayload,
  SwRequest,
} from "./core/jobMessages.ts";
import {
  readGenerateResult,
  readPrepareResult,
  readProgressEvent,
} from "./core/wasmResult.ts";
import type { InternalDetail, Steps } from "./core/wasmResult.ts";
import type { WasmWorker } from "./wasmWorkerClient.ts";
import { workerLoadTimeoutMs } from "./wasmWorkerClient.ts";

/** How long the generate call may take before the run fails as run_timeout: 50 minutes. */
export const runTimeoutMs = 50 * 60 * 1000;

/** How long to wait before sending an unanswered result again, or reconnecting a Port that could not be opened. */
export const retryMs = 2_000;

/** Sends one request to the service worker and resolves its reply, untrusted. Never rejects. */
export type SendToSw = (request: SwRequest) => Promise<BrowserCall<unknown>>;

/**
 * Connects the runner's Port, whose first message is hello, and calls
 * onDisconnect when it is cut. Returns a function that closes it, or
 * undefined when it could not be connected.
 */
export type ConnectRunnerPort = (
  hello: RunnerHello,
  onDisconnect: () => void,
) => (() => void) | undefined;

export interface RunnerDeps {
  /** Starts a new Worker that loads the article generation module. */
  readonly startWorker: () => WasmWorker;
  /** Runs callback once after ms milliseconds; returns a function that cancels it. */
  readonly schedule: (ms: number, callback: () => void) => () => void;
  /** The time in ms, only to measure how much of the load deadline is left. */
  readonly now: () => number;
  readonly sendToSw: SendToSw;
  readonly connectPort: ConnectRunnerPort;
  readonly readSettings: typeof readAnthropicSettings;
  readonly onSessionChanged: typeof onSessionChanged;
  /** Turns on (true) or off (false) the confirmation before the page is reloaded or closed. */
  readonly confirmUnload: (on: boolean) => void;
}

/** The page's own identity: the start ID in its URL and the random ID of this page load. */
export interface RunnerIdentity {
  readonly startId: string;
  readonly instanceId: string;
}

export interface RunnerHooks {
  /** The reply to the reservation, before anything else happens. */
  onReserved(reply: ReserveReply): void;
  /** The steps of each progress event of the run. */
  onProgress(steps: Steps): void;
}

/** How a run ended, for the page to show. */
export type RunOutcome =
  /** The service worker no longer holds the input: it was claimed, or never sent. */
  | { readonly kind: "input_unavailable" }
  /** The input was rejected, or the Worker failed to check it; the service worker was told. */
  | {
      readonly kind: "prepare_rejected";
      readonly reason: "invalid_body" | "internal";
      readonly internalDetail?: InternalDetail;
    }
  /** The reservation could not be made or its reply could not be read. */
  | { readonly kind: "reserve_unavailable" }
  /** The reservation did not start a run: an existing job, the same video, or a refusal. */
  | {
      readonly kind: "not_run";
      readonly reply: Exclude<ReserveReply, { status: "new_run" }>;
    }
  /** This viewer ran the job to its end. */
  | {
      readonly kind: "finished";
      readonly jobId: string;
      readonly result: RunResult;
      /** Whether the record holds the result: it does for "stored" and "in_record". */
      readonly stored: "stored" | "not_stored" | "in_record";
    }
  /** The run was taken over, or its record removed, and the Worker was stopped. */
  | { readonly kind: "stopped"; readonly jobId: string };

const initialSteps: Steps = {
  draft: "pending",
  shorten: "pending",
  review: "pending",
};

/** The step states of a run that ended with a step still running: that step failed. */
function endRunningSteps(steps: Steps): Steps {
  const fail = (state: Steps["draft"]): Steps["draft"] =>
    state === "running" ? "failed" : state;
  return {
    draft: fail(steps.draft),
    shorten: fail(steps.shorten),
    review: fail(steps.review),
  };
}

/** The input as the module's prepare request. */
function prepareRequest(input: StartPayload): string {
  return JSON.stringify(input);
}

/**
 * The generate request from the saved settings, which are written into it as
 * they were read: the module checks their shape.
 */
function generateRequest(
  inputDigest: string,
  found: {
    readonly anthropicApiKey: unknown;
    readonly anthropicWorkspaceId?: unknown;
  },
): string {
  return JSON.stringify({ inputDigest, settings: found });
}

/** Runs the viewer's flow once. Never rejects. */
export async function runViewer(
  identity: RunnerIdentity,
  deps: RunnerDeps,
  hooks: RunnerHooks,
): Promise<RunOutcome> {
  const claimed = await deps.sendToSw({
    type: "claimInput",
    startId: identity.startId,
  });
  const claim = claimed.ok ? parseClaimInputReply(claimed.value) : undefined;
  if (claim?.status !== "ok") {
    return { kind: "input_unavailable" };
  }
  const input = claim.input;

  // Worker start-up and prepare share one 30 second deadline: the client's
  // own load deadline ends at ready, so the call carries what is left of it.
  const loadStarted = deps.now();
  const worker = deps.startWorker();
  const prepared = await worker.call("prepare", prepareRequest(input), {
    timeoutMs: Math.max(1, workerLoadTimeoutMs - (deps.now() - loadStarted)),
  });
  const checked = await checkInput(prepared, identity, deps);
  if (checked.kind === "rejected") {
    worker.terminate();
    return checked.outcome;
  }

  const reserved = await deps.sendToSw({
    type: "reserve",
    startId: identity.startId,
    inputDigest: checked.inputDigest,
    videoId: checked.videoId,
    instanceId: identity.instanceId,
    heading: {
      videoTitle: input.title ?? "",
      channelName: input.channelName ?? "",
    },
    videoUrl: input.videoUrl,
  });
  const reply = reserved.ok ? parseReserveReply(reserved.value) : undefined;
  if (reply === undefined) {
    worker.terminate();
    return { kind: "reserve_unavailable" };
  }
  hooks.onReserved(reply);
  if (reply.status !== "new_run") {
    worker.terminate();
    return { kind: "not_run", reply };
  }
  return runJob(
    reply.jobId,
    checked.inputDigest,
    worker,
    identity,
    deps,
    hooks,
  );
}

/**
 * The input's digest and video ID when the prepare call gave an ok result.
 * Otherwise tells the service worker why the input was not accepted and
 * returns the outcome.
 */
async function checkInput(
  prepared: Awaited<ReturnType<WasmWorker["call"]>>,
  identity: RunnerIdentity,
  deps: RunnerDeps,
): Promise<
  | {
      readonly kind: "ok";
      readonly inputDigest: string;
      readonly videoId: string;
    }
  | { readonly kind: "rejected"; readonly outcome: RunOutcome }
> {
  const reject = async (
    reason: "invalid_body" | "internal",
    internalDetail?: InternalDetail,
  ): Promise<{ readonly kind: "rejected"; readonly outcome: RunOutcome }> => {
    await deps.sendToSw({
      type: "prepareRejected",
      startId: identity.startId,
      reason,
      ...(internalDetail === undefined ? {} : { internalDetail }),
    });
    return {
      kind: "rejected",
      outcome: {
        kind: "prepare_rejected",
        reason,
        ...(internalDetail === undefined ? {} : { internalDetail }),
      },
    };
  };
  switch (prepared.kind) {
    case "timeout":
    case "stopped":
      return reject("internal", "worker_load");
    case "failed":
      return reject("internal", prepared.detail);
    case "reply": {
      const result = readPrepareResult(prepared.value);
      if (!result.ok) {
        return reject("internal", result.detail);
      }
      return result.value.status === "rejected"
        ? reject("invalid_body")
        : {
            kind: "ok",
            inputDigest: result.value.inputDigest,
            videoId: result.value.videoId,
          };
    }
  }
}

/** Runs a reserved job: reads the settings, generates, reports, and delivers the result. */
async function runJob(
  jobId: string,
  inputDigest: string,
  worker: WasmWorker,
  identity: RunnerIdentity,
  deps: RunnerDeps,
  hooks: RunnerHooks,
): Promise<RunOutcome> {
  let ending: "in_record" | "stopped" | undefined;
  let wake: () => void = () => {};
  const settle = (how: "in_record" | "stopped"): void => {
    if (ending === undefined) {
      ending = how;
      wake();
    }
  };
  let generating = true;
  let resultHeld = false;

  // The record is watched from the reservation on: a removed record, or one
  // another viewer now runs, stops the Worker. A stopped record alone does
  // not: a result delivered later turns it back into done or failed.
  const stopWatching = deps.onSessionChanged((changes) => {
    const change = changes.get(jobItemName(jobId));
    if (change === undefined) {
      return;
    }
    if (change.removed) {
      stopRun();
      return;
    }
    const record = readJobRecord(jobId, change.newValue);
    if (record === undefined) {
      return;
    }
    if (record.state === "done" || record.state === "failed") {
      if (resultHeld) {
        settle("in_record");
      } else {
        stopRun();
      }
    } else if (record.runner.instanceId !== identity.instanceId) {
      stopRun();
    }
  });
  const stopRun = (): void => {
    if (generating) {
      worker.terminate();
    }
    settle("stopped");
  };

  // The Port is kept for the whole run and reconnected when it is cut.
  let closePort: (() => void) | undefined;
  let portOpen = true;
  const hello: RunnerHello = {
    type: "hello",
    jobId,
    instanceId: identity.instanceId,
  };
  let cancelReconnect: () => void = () => {};
  const connect = (): void => {
    if (!portOpen) {
      return;
    }
    closePort = deps.connectPort(hello, connect);
    if (closePort === undefined) {
      cancelReconnect = deps.schedule(retryMs, connect);
    }
  };
  connect();

  deps.confirmUnload(true);
  let lastSteps = initialSteps;
  let result: RunResult | undefined;
  let queue: Promise<void> = Promise.resolve();
  try {
    result = await generate();
  } finally {
    generating = false;
    deps.confirmUnload(false);
  }

  async function generate(): Promise<RunResult | undefined> {
    const settings = await deps.readSettings();
    if (!settings.ok) {
      return failed("internal", "settings_read");
    }
    if (settings.value.kind === "no_api_key") {
      return failed("api_key_missing");
    }
    const request = generateRequest(inputDigest, settings.value.settings);
    const called = await worker.call("generate", request, {
      timeoutMs: runTimeoutMs,
      onProgress: (event) => {
        const progress = readProgressEvent(event);
        // An unreadable event is not a result; the module's final result
        // decides the run, so it is skipped.
        if (progress.ok) {
          lastSteps = progress.value.steps;
          hooks.onProgress(lastSteps);
          queue = queue.then(() => sendProgress(progress.value.steps));
        }
      },
    });
    switch (called.kind) {
      case "stopped":
        return undefined;
      case "timeout":
        return failed("run_timeout");
      case "failed":
        return failed("internal", called.detail);
      case "reply": {
        const read = readGenerateResult(called.value);
        if (!read.ok) {
          return failed("internal", read.detail);
        }
        const value = read.value;
        return value.status === "done"
          ? { state: "done", steps: value.steps, article: value.article }
          : {
              state: "failed",
              steps: value.steps,
              failure: value.failure,
              ...(value.internalDetail === undefined
                ? {}
                : { internalDetail: value.internalDetail }),
            };
      }
    }
  }

  function failed(
    failure: RecordFailure,
    internalDetail?: InternalDetail,
  ): RunResult {
    return {
      state: "failed",
      steps: endRunningSteps(lastSteps),
      failure,
      ...(internalDetail === undefined ? {} : { internalDetail }),
    };
  }

  async function sendProgress(steps: Steps): Promise<void> {
    const sent = await deps.sendToSw({
      type: "progress",
      jobId,
      instanceId: identity.instanceId,
      steps,
    });
    const reply = sent.ok ? parseWriteReply(sent.value) : undefined;
    if (reply?.status === "not_runner") {
      stopRun();
    }
  }

  try {
    await queue;
    if (result === undefined || ending === "stopped") {
      return { kind: "stopped", jobId };
    }
    worker.terminate();
    resultHeld = true;
    const delivered = await deliver(result);
    return delivered === "stopped"
      ? { kind: "stopped", jobId }
      : { kind: "finished", jobId, result, stored: delivered };
  } finally {
    stopWatching();
    portOpen = false;
    cancelReconnect();
    closePort?.();
  }

  /** Sends the result until the service worker answers or the record holds it. */
  async function deliver(
    held: RunResult,
  ): Promise<"stored" | "not_stored" | "in_record" | "stopped"> {
    for (;;) {
      if (ending !== undefined) {
        return ending;
      }
      const sent = await deps.sendToSw({
        type: "result",
        jobId,
        instanceId: identity.instanceId,
        result: held,
      });
      const reply = sent.ok ? parseWriteReply(sent.value) : undefined;
      switch (reply?.status) {
        case "stored":
          return "stored";
        case "not_stored":
          return "not_stored";
        case "not_runner":
          return "stopped";
        case undefined:
          break;
      }
      if (ending !== undefined) {
        return ending;
      }
      await new Promise<void>((resolve) => {
        const cancel = deps.schedule(retryMs, resolve);
        wake = () => {
          cancel();
          resolve();
        };
      });
    }
  }
}
