// What a viewer (and the popup's job states) displays, and which row of the
// publish button's table applies. The values are built only from checked
// results, and every display choice is a switch over these unions.

import type { NotSentCause } from "./exchange.ts";
import type {
  Article,
  FailureCode,
  PublishOutcome,
  PublishRejectReason,
  PublishStatus,
  RequestId,
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
  // A key, waiting for the first status.
  | { readonly kind: "loading" }
  | {
      readonly kind: "job";
      readonly job: JobDisplay;
      readonly notice: PollNotice;
    };

/** A start that gave no key; no-result is a wait that ended without a result. */
export type StartFailure =
  Exclude<StartOutcome, { kind: "accepted" }> | { readonly kind: "no-result" };

/** The job as displayed; built from a StatusOutcome by afterStatus alone. */
export type JobDisplay =
  | { readonly kind: "running"; readonly steps: Steps }
  | {
      readonly kind: "done";
      readonly steps: Steps;
      readonly article: Article;
      readonly publish: PublishStatus;
      readonly webhookConfigured: boolean;
    }
  | {
      readonly kind: "failed";
      readonly steps: Steps;
      readonly failure: FailureCode;
    }
  // The server holds no job for the key; kept when a done article was shown.
  | { readonly kind: "lost"; readonly kept?: Article };

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

/** Which row of the publish button's table applies, top row first: an upper row wins over the rows below it. */
export type PublishRow =
  | { readonly row: "lost" }
  | { readonly row: "posted" }
  | { readonly row: "posting" }
  | { readonly row: "unpostable"; readonly reason: "mention" | "other" }
  | { readonly row: "webhook-not-configured" }
  | { readonly row: "failed"; readonly reason: "webhook" | "internal" }
  | { readonly row: "not-posted" };

export interface PublishControl {
  readonly enabled: boolean;
  readonly row: PublishRow;
}

/** The first row of the publish table that applies; rows are checked top to bottom, so an upper row wins. */
function publishRow(
  job: Extract<JobDisplay, { kind: "done" | "lost" }>,
): PublishRow {
  if (job.kind === "lost") {
    return { row: "lost" };
  }
  const { publish } = job;
  if (publish.state === "posted") {
    return { row: "posted" };
  }
  if (publish.state === "posting") {
    return { row: "posting" };
  }
  if (publish.state === "unpostable") {
    return { row: "unpostable", reason: publish.reason };
  }
  if (!job.webhookConfigured) {
    return { row: "webhook-not-configured" };
  }
  if (publish.state === "failed") {
    return { row: "failed", reason: publish.reason };
  }
  return { row: "not-posted" };
}

/**
 * The publish button for a done or lost job. Only the failed and not-posted
 * rows can be pressed, and not while publishing: a publish operation is in
 * flight or waits for its first status.
 */
export function publishControl(
  job: Extract<JobDisplay, { kind: "done" | "lost" }>,
  publishing: boolean,
): PublishControl {
  const row = publishRow(job);
  switch (row.row) {
    case "failed":
    case "not-posted":
      return { enabled: !publishing, row };
    case "lost":
    case "posted":
    case "posting":
    case "unpostable":
    case "webhook-not-configured":
      return { enabled: false, row };
  }
}

/** The result of the last publish operation, shown beside the button until the next press. */
export type PublishOpNotice =
  | { readonly kind: "none" }
  | {
      readonly kind: "rejected";
      readonly reason: PublishRejectReason;
      readonly requestId: RequestId;
    }
  | { readonly kind: "not-sent"; readonly cause: NotSentCause }
  // No response, the host failed, or an unexpected reply: the result is not
  // known. The outcome is kept because each kind has its own message.
  | {
      readonly kind: "unknown";
      readonly outcome: Extract<
        PublishOutcome,
        { kind: "no-response" | "host-failed" | "unexpected" }
      >;
    };
