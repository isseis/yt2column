// The options page's flows: saving the API key and workspace ID, deleting
// them, showing whether an API key is set, and reading the build's
// information. The page script wires the real Worker, storage functions and
// form into OptionsFlowDeps; everything else, including every deadline and
// the order of the calls, is decided here.
//
// A save copies the two fields once, when the button is pressed, checks that
// copy in a fresh Worker, and writes the same copy with one set, only when
// the check says ok or unverified. The fields and both buttons stay disabled
// from the copy until the write has finished, so a value is never written
// that was not checked. The overall deadline covers the Worker's start-up
// and the check and stops when the check has a result: it never reaches the
// write. The key is only read from the form, handed to the Worker and the
// write, and never kept, logged or shown.

import type {
  localKeys,
  removeAnthropicSettings,
  writeAnthropicSettings,
} from "./browser/chromeDeps.ts";
import type { OptionsResult } from "./core/optionsView.ts";
import { readBuildInfo, readSettingsCheckResult } from "./core/wasmResult.ts";
import type { BuildInfo } from "./core/wasmResult.ts";
import type { WasmWorker } from "./wasmWorkerClient.ts";
import { workerLoadTimeoutMs } from "./wasmWorkerClient.ts";

/** The whole deadline of a save's check, from the press to the check's result. */
export const saveCheckDeadlineMs = 90_000;

/** The two fields of the options page, and the control of their editing. */
export interface OptionsForm {
  /** The fields' values now; a save reads them once. */
  read(): { readonly apiKey: string; readonly workspaceId: string };
  /** Disables (true) or enables (false) both fields and both buttons. */
  setBusy(busy: boolean): void;
}

export interface OptionsFlowDeps {
  readonly form: OptionsForm;
  /** Starts a new Worker that loads the article generation module. */
  readonly startWorker: () => WasmWorker;
  /** Runs callback once after ms milliseconds; returns a function that cancels it. */
  readonly schedule: (ms: number, callback: () => void) => () => void;
  readonly write: typeof writeAnthropicSettings;
  readonly remove: typeof removeAnthropicSettings;
  readonly keys: typeof localKeys;
}

/** What a flow shows: its outcome, and whether an API key is set after it (undefined when that could not be read). */
export interface OptionsFlowResult {
  readonly result: OptionsResult;
  readonly apiKeySet: boolean | undefined;
}

/** Whether an API key is set, from the names of the stored items; undefined when they could not be read. */
export async function readApiKeySet(
  deps: Pick<OptionsFlowDeps, "keys">,
): Promise<boolean | undefined> {
  const keys = await deps.keys();
  return keys.ok ? keys.value.apiKeySet : undefined;
}

/**
 * Runs one call in a fresh Worker with a deadline for the whole of it (the
 * Worker's start-up included), then stops the Worker whatever happened.
 */
async function callOnce(
  deps: Pick<OptionsFlowDeps, "startWorker" | "schedule">,
  fn: "checkSettings" | "buildInfo",
  request: string,
  deadlineMs: number,
): Promise<string | undefined> {
  const worker = deps.startWorker();
  const cancel = deps.schedule(deadlineMs, () => worker.terminate());
  try {
    const result = await worker.call(fn, request);
    return result.kind === "reply" ? result.value : undefined;
  } finally {
    cancel();
    worker.terminate();
  }
}

/**
 * Saves the two fields as they are when the button is pressed. Writes only
 * after the check said ok or unverified, with one write of the same values.
 * Never rejects.
 */
export async function saveSettings(
  deps: OptionsFlowDeps,
): Promise<OptionsFlowResult> {
  const { apiKey, workspaceId } = deps.form.read();
  deps.form.setBusy(true);
  try {
    const result = await checkAndWrite(deps, apiKey, workspaceId);
    return { result, apiKeySet: await readApiKeySet(deps) };
  } finally {
    deps.form.setBusy(false);
  }
}

async function checkAndWrite(
  deps: OptionsFlowDeps,
  apiKey: string,
  workspaceId: string,
): Promise<OptionsResult> {
  const text = await callOnce(
    deps,
    "checkSettings",
    JSON.stringify({
      anthropicApiKey: apiKey,
      anthropicWorkspaceId: workspaceId,
    }),
    saveCheckDeadlineMs,
  );
  const checked =
    text === undefined ? undefined : readSettingsCheckResult(text);
  if (checked === undefined || !checked.ok) {
    return { outcome: "check_unavailable" };
  }
  switch (checked.value.status) {
    case "rejected":
      return { outcome: "field_rejected", fields: checked.value.fields };
    case "key_rejected":
      return { outcome: "key_rejected" };
    case "ok":
    case "unverified": {
      const written = await deps.write(apiKey, workspaceId);
      if (!written.ok) {
        return { outcome: "write_uncertain" };
      }
      return {
        outcome: checked.value.status === "ok" ? "saved" : "saved_unverified",
      };
    }
  }
}

/** Deletes the stored settings with one remove. Never rejects. */
export async function deleteSettings(
  deps: OptionsFlowDeps,
): Promise<OptionsFlowResult> {
  deps.form.setBusy(true);
  try {
    const removed = await deps.remove();
    return {
      result: { outcome: removed.ok ? "deleted" : "delete_uncertain" },
      apiKeySet: await readApiKeySet(deps),
    };
  } finally {
    deps.form.setBusy(false);
  }
}

/**
 * Reads the build's information in a Worker of its own, which is stopped
 * afterwards, within the load deadline. Undefined when the Worker did not
 * load in time or its answer is not a valid result.
 */
export async function loadBuildInfo(
  deps: Pick<OptionsFlowDeps, "startWorker" | "schedule">,
): Promise<BuildInfo | undefined> {
  const text = await callOnce(deps, "buildInfo", "", workerLoadTimeoutMs);
  const info = text === undefined ? undefined : readBuildInfo(text);
  return info?.ok === true ? info.value : undefined;
}
