// What the popup and the viewers display in the extension-only path. Every
// value comes from a job record read back from chrome.storage.session, from a
// start's reply, or from the running viewer's own result, and every display
// choice is a switch over these unions. The failures that end before the LLM
// API is called (api_key_missing, settings_invalid) are kept apart from the
// others: they are shown as a missing setting and never with a cost.

import type { JobRecord, RecordFailure } from "./jobRecord.ts";
import type { StartRejection } from "./jobMessages.ts";
import type { ArticleResult, StepState, Steps } from "./wasmResult.ts";

/**
 * A step's state as displayed: a record's state, or interrupted for a step
 * the record still holds as running after the job ended (the run stopped with
 * its page, or failed outside the module), which no longer runs.
 */
export type ShownStepState = StepState | "interrupted";

/** The state a step is shown in; a running step of an ended job is interrupted. */
export function shownStepState(
  state: StepState,
  jobEnded: boolean,
): ShownStepState {
  return jobEnded && state === "running" ? "interrupted" : state;
}

/** How often a page draws a running job's elapsed time again while it is visible. */
export const elapsedRedrawMs = 1_000;

/** The failures shown as a missing setting: the LLM API was never called. */
export type SetupFailure = Extract<
  RecordFailure,
  "api_key_missing" | "settings_invalid"
>;

/** The failures shown as a failed article, with an unknown cost. */
export type ArticleFailure = Exclude<
  RecordFailure,
  SetupFailure | "api_key_rejected"
>;

/** The job as displayed. */
export type JobDisplay =
  | { readonly kind: "running"; readonly steps: Steps }
  | {
      readonly kind: "done";
      readonly steps: Steps;
      readonly article: ArticleResult;
    }
  // The running viewer holds the article, but the record could not hold it.
  | {
      readonly kind: "unstored";
      readonly steps: Steps;
      readonly article: ArticleResult;
    }
  | { readonly kind: "setup"; readonly failure: SetupFailure }
  | { readonly kind: "key-rejected"; readonly steps: Steps }
  | {
      readonly kind: "failed";
      readonly steps: Steps;
      readonly failure: ArticleFailure;
    }
  | { readonly kind: "stopped"; readonly steps: Steps }
  // No record holds the job: the browser closed, the extension was reloaded
  // or updated, or the record was dropped to make room.
  | { readonly kind: "lost" };

/** The display of a job's record; lost when there is none. */
export function recordDisplay(record: JobRecord | undefined): JobDisplay {
  if (record === undefined) {
    return { kind: "lost" };
  }
  switch (record.state) {
    case "running":
      return { kind: "running", steps: record.steps };
    case "done":
      return { kind: "done", steps: record.steps, article: record.article };
    case "stopped":
      return { kind: "stopped", steps: record.steps };
    case "failed":
      return failedDisplay(record.steps, record.failure);
  }
}

/** The display of a run that failed with failure after steps. */
export function failedDisplay(
  steps: Steps,
  failure: RecordFailure,
): JobDisplay {
  switch (failure) {
    case "api_key_missing":
    case "settings_invalid":
      return { kind: "setup", failure };
    case "api_key_rejected":
      return { kind: "key-rejected", steps };
    default:
      return { kind: "failed", steps, failure };
  }
}

/** The article a job shows: a done job's, or the one the running viewer holds. */
export function jobArticle(
  job: JobDisplay | undefined,
): ArticleResult | undefined {
  switch (job?.kind) {
    case "done":
    case "unstored":
      return job.article;
    case "running":
    case "setup":
    case "key-rejected":
    case "failed":
    case "stopped":
    case "lost":
    case undefined:
      return undefined;
  }
}

/** Whether a job's cost is shown as unknown: it called the LLM API but made no article. */
export function costUnknown(job: JobDisplay): boolean {
  switch (job.kind) {
    case "key-rejected":
    case "failed":
    case "stopped":
      return true;
    case "running":
    case "done":
    case "unstored":
    case "setup":
    case "lost":
      return false;
  }
}

/**
 * A start that gave no job to show. rejected: the service worker or the
 * viewer refused it (api_key_missing is shown as a missing setting). unknown:
 * the reservation's result never arrived. input-unavailable: the viewer
 * could not take the input from the service worker.
 */
export type StartFailure =
  | { readonly kind: "rejected"; readonly reason: StartRejection }
  | { readonly kind: "unknown" }
  | { readonly kind: "input-unavailable" };

/** Whether a refused start offers the operation that opens the options page. */
export function startOffersOptions(failure: StartFailure): boolean {
  return failure.kind === "rejected" && failure.reason === "api_key_missing";
}

/** Whether a job offers the operation that opens the options page. */
export function jobOffersOptions(job: JobDisplay): boolean {
  return job.kind === "setup" || job.kind === "key-rejected";
}

/** What the viewer's main area shows. */
export type ViewerState =
  // The URL failed parseJobViewerUrl.
  | { readonly kind: "invalid-link" }
  // A start not reserved yet: taking the input, checking it, reserving.
  | { readonly kind: "preparing" }
  | { readonly kind: "start-failed"; readonly failure: StartFailure }
  // A job ID whose record has not been read yet.
  | { readonly kind: "loading" }
  | { readonly kind: "job"; readonly job: JobDisplay };
