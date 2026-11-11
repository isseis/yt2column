// What a viewer (and the popup's job states) displays. The values are built
// only from checked results, and every display choice is a switch over these
// unions.

import type {
  Article,
  FailureCode,
  StartOutcome,
  StatusOutcome,
  Steps,
} from "./response.ts";

/** What the viewer's main area shows. */
export type ViewerState =
  // The URL failed parseViewerUrl.
  | { readonly kind: "invalid-link" }
  // No key yet, waiting for the start result.
  | { readonly kind: "sending" }
  // Rejected, not sent, or unknown.
  | { readonly kind: "start-failed"; readonly failure: StartFailure }
  // A key, waiting for the first status. A failed status exchange adds its
  // notice, as it does beside a job.
  | { readonly kind: "loading"; readonly notice: PollNotice }
  | {
      readonly kind: "job";
      readonly job: JobDisplay;
      readonly notice: PollNotice;
    };

/**
 * A start that gave no key; no-result is a wait that ended without a result.
 * A same-video answer gave a key, so it is not one.
 */
export type StartFailure =
  | Exclude<StartOutcome, { kind: "accepted" } | { kind: "same-video" }>
  | { readonly kind: "no-result" };

/** The job as displayed; built from a StatusOutcome by afterStatus alone. */
export type JobDisplay =
  | { readonly kind: "running"; readonly steps: Steps }
  | {
      readonly kind: "done";
      readonly steps: Steps;
      readonly article: Article;
    }
  | {
      readonly kind: "failed";
      readonly steps: Steps;
      readonly failure: FailureCode;
    }
  // The server holds no job for the key; kept when a done article was shown.
  | { readonly kind: "lost"; readonly kept?: Article };

/** The article a job shows: a done job's, or the one a lost job kept; none for any other job. */
export function jobArticle(job: JobDisplay | undefined): Article | undefined {
  switch (job?.kind) {
    case "done":
      return job.article;
    case "lost":
      return job.kept;
    case "running":
    case "failed":
    case undefined:
      return undefined;
  }
}

/** Status results that leave the state shown and keep polling; derived from StatusOutcome, never re-listed. */
export type RetryingStatus =
  | Extract<StatusOutcome, { kind: "no-response" | "host-failed" }>
  | { readonly kind: "not-sent"; readonly cause: "server-unavailable" };

/** Status results that leave the state shown and stop polling. */
export type StoppingStatus =
  | Extract<StatusOutcome, { kind: "rejected" | "unexpected" }>
  | { readonly kind: "not-sent"; readonly cause: "message-too-large" };

/** The notice beside the state when a status exchange failed; the state itself is kept. */
export type PollNotice =
  | { readonly kind: "none" }
  | { readonly kind: "retrying"; readonly outcome: RetryingStatus }
  // A rejection keeps its requestId.
  | { readonly kind: "stopped"; readonly outcome: StoppingStatus };
