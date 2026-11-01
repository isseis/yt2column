// Checked job values for the tests of the display and polling rules. The
// branded values are asserted here, as test fixtures may; the source builds
// them only through their parse functions.

import type { JobDisplay } from "../../src/core/jobView.ts";
import type {
  Article,
  PublishStatus,
  RequestId,
  Steps,
  UsdAmount,
} from "../../src/core/response.ts";

export const requestId = "0123456789abcdef" as RequestId;

export const runningSteps: Steps = {
  draft: "done",
  shorten: "running",
  review: "pending",
};

export const doneSteps: Steps = {
  draft: "done",
  shorten: "skipped",
  review: "done",
};

export const article: Article = {
  markdown: "# Title\n\nBody",
  file: "---\ntitle: Title\n---\n\nBody\n",
  cost: { state: "known", usd: "0.1234" as UsdAmount },
  durationMs: 65_000,
};

/** A done job as displayed, with the given publish status and Webhook setting. */
export function doneJob(
  publish: PublishStatus,
  webhookConfigured = true,
): Extract<JobDisplay, { kind: "done" }> {
  return {
    kind: "done",
    steps: doneSteps,
    article,
    publish,
    webhookConfigured,
  };
}
