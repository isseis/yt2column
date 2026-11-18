// Job records, start contexts and article results for the tests of the
// extension-only path's display. The branded cost is asserted here, as test
// fixtures may; the source builds it only through its parse function.

import type { JobRecord, StartContext } from "../../src/core/jobRecord.ts";
import type { JobDisplay } from "../../src/core/jobView.ts";
import type {
  ArticleResult,
  Steps,
  UsdAmount,
} from "../../src/core/wasmResult.ts";

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

export const failedSteps: Steps = {
  draft: "done",
  shorten: "failed",
  review: "pending",
};

export const article: ArticleResult = {
  markdown: "# Title\n\nBody",
  mentionsNeutralized: false,
  videoTitle: "",
  channelName: "",
  file: "---\ntitle: Title\n---\n\nBody\n",
  cost: { state: "known", usd: "0.1234" as UsdAmount },
  durationMs: 65_000,
};

/** A done job as displayed. */
export const doneJob: Extract<JobDisplay, { kind: "done" }> = {
  kind: "done",
  steps: doneSteps,
  article,
};

export const jobId = "0123456789abcdef0123456789abcdef";
export const startId = "fedcba9876543210fedcba9876543210";
export const instanceId = "00112233445566778899aabbccddeeff";
export const watchUrl = "https://www.youtube.com/watch?v=dQw4w9WgXcQ";

/** The values every record of jobId shares. */
function base(acceptedAt: number) {
  return {
    version: 1,
    jobId,
    inputDigest: "a".repeat(64),
    videoId: "dQw4w9WgXcQ",
    heading: { videoTitle: "A video title", channelName: "A channel" },
    videoUrl: watchUrl,
    acceptedAt,
  } as const;
}

/** A running record whose runner is tabId. */
export function runningRecord(
  tabId = 7,
  acceptedAt = 1_000,
  steps: Steps = runningSteps,
): JobRecord {
  return {
    ...base(acceptedAt),
    steps,
    state: "running",
    runner: { tabId, instanceId },
  };
}

/** A done record. */
export function doneRecord(acceptedAt = 1_000): JobRecord {
  return {
    ...base(acceptedAt),
    steps: doneSteps,
    state: "done",
    endedAt: acceptedAt + 65_000,
    article,
  };
}

/** A failed record. */
export function failedRecord(
  failure: Extract<JobRecord, { state: "failed" }>["failure"],
  acceptedAt = 1_000,
): JobRecord {
  return {
    ...base(acceptedAt),
    steps: failedSteps,
    state: "failed",
    endedAt: acceptedAt + 5_000,
    failure,
  };
}

/** A stopped record. */
export function stoppedRecord(acceptedAt = 1_000): JobRecord {
  return {
    ...base(acceptedAt),
    steps: failedSteps,
    state: "stopped",
    endedAt: acceptedAt + 5_000,
    runner: { tabId: 7, instanceId },
    stopReason: "tab_closed",
  };
}

/** The start context of startId for the viewer in tabId. */
export function startContext(tabId = 7, createdAt = 500): StartContext {
  return {
    version: 1,
    startId,
    tabId,
    mode: "inquiry",
    createdAt,
    heading: "A video title",
    channelName: "A channel",
    videoTitle: "A video title",
    videoUrl: watchUrl,
  };
}
