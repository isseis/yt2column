import assert from "node:assert/strict";
import { describe, it } from "node:test";

import {
  maxRecordBytes,
  readJobRecord,
  readStartContext,
} from "../src/core/jobRecord.ts";
import type { JobRecord, StartContext } from "../src/core/jobRecord.ts";

// The record holds neither the copied transcript nor the description: a type
// that gained either member fails this file's type check.
type NoInput<T> =
  Extract<keyof T, "transcript" | "description"> extends never ? true : false;
const recordHoldsNoInput: NoInput<JobRecord> = true;
const contextHoldsNoInput: NoInput<StartContext> = true;
void [recordHoldsNoInput, contextHoldsNoInput];

const jobId = "0123456789abcdef0123456789abcdef";
const instanceId = "fedcba9876543210fedcba9876543210";

const base = {
  version: 1,
  jobId,
  inputDigest: "ab".repeat(32),
  videoId: "dQw4w9WgXcQ",
  heading: { videoTitle: "Title", channelName: "Channel" },
  videoUrl: "https://www.youtube.com/watch?v=dQw4w9WgXcQ",
  acceptedAt: 1_000,
};
const doneSteps = { draft: "done", shorten: "skipped", review: "done" };
const article = {
  markdown: "# Title\n\nBody.",
  mentionsNeutralized: false,
  videoTitle: "Title",
  channelName: "Channel",
  file: "---\n",
  cost: { state: "known", usd: "0.1234" },
  durationMs: 1_500,
};

const records: Record<string, Record<string, unknown>> = {
  running: {
    ...base,
    state: "running",
    steps: { draft: "running", shorten: "pending", review: "pending" },
    runner: { tabId: 7, instanceId },
  },
  done: {
    ...base,
    state: "done",
    steps: doneSteps,
    endedAt: 2_000,
    article,
  },
  failed: {
    ...base,
    state: "failed",
    steps: { draft: "failed", shorten: "pending", review: "pending" },
    endedAt: 2_000,
    failure: "run_timeout",
  },
  failedInternal: {
    ...base,
    state: "failed",
    steps: { draft: "pending", shorten: "pending", review: "pending" },
    endedAt: 2_000,
    failure: "internal",
    internalDetail: "store_failed",
  },
  stopped: {
    ...base,
    state: "stopped",
    steps: { draft: "running", shorten: "pending", review: "pending" },
    endedAt: 2_000,
    runner: { tabId: 7, instanceId },
    stopReason: "page_reloaded",
  },
};

describe("readJobRecord", () => {
  for (const [name, record] of Object.entries(records)) {
    it(`reads a ${name} record as it was written`, () => {
      assert.deepEqual(readJobRecord(jobId, structuredClone(record)), record);
    });
  }

  it("reads a failure only the running viewer decides", () => {
    for (const failure of ["api_key_missing", "settings_invalid"]) {
      assert.ok(
        readJobRecord(jobId, { ...records["failed"], failure }) !== undefined,
      );
    }
  });

  /** A record of state changed by change. */
  const changed = (
    state: string,
    change: (record: Record<string, unknown>) => void,
  ): Record<string, unknown> => {
    const record = structuredClone(records[state] ?? {});
    change(record);
    return record;
  };

  const absent: { name: string; value: unknown }[] = [
    {
      name: "another version",
      value: changed("done", (r) => {
        r["version"] = 2;
      }),
    },
    {
      name: "an unknown member",
      value: changed("done", (r) => {
        r["note"] = "x";
      }),
    },
    {
      name: "the copied transcript",
      value: changed("running", (r) => {
        r["transcript"] = "0:00 Hello";
      }),
    },
    {
      name: "the description",
      value: changed("running", (r) => {
        r["description"] = "About";
      }),
    },
    {
      name: "a missing member",
      value: changed("done", (r) => {
        delete r["acceptedAt"];
      }),
    },
    {
      name: "a running record without a runner",
      value: changed("running", (r) => {
        delete r["runner"];
      }),
    },
    {
      name: "a done record with a runner",
      value: changed("done", (r) => {
        r["runner"] = { tabId: 7, instanceId };
      }),
    },
    {
      name: "a done record with a failure",
      value: changed("done", (r) => {
        r["failure"] = "internal";
      }),
    },
    {
      name: "a running record with an end time",
      value: changed("running", (r) => {
        r["endedAt"] = 2_000;
      }),
    },
    {
      name: "a failed record with an article",
      value: changed("failed", (r) => {
        r["article"] = article;
      }),
    },
    {
      name: "a stopped record without a reason",
      value: changed("stopped", (r) => {
        delete r["stopReason"];
      }),
    },
    {
      name: "a detail on a failure that is not internal",
      value: changed("failed", (r) => {
        r["internalDetail"] = "panic";
      }),
    },
    {
      name: "an unknown detail",
      value: changed("failedInternal", (r) => {
        r["internalDetail"] = "disk_full";
      }),
    },
    {
      name: "an unknown failure",
      value: changed("failed", (r) => {
        r["failure"] = "server_unavailable";
      }),
    },
    {
      name: "an unknown stop reason",
      value: changed("stopped", (r) => {
        r["stopReason"] = "crashed";
      }),
    },
    {
      name: "an unknown state",
      value: changed("running", (r) => {
        r["state"] = "queued";
      }),
    },
    {
      name: "a done record whose step did not finish",
      value: changed("done", (r) => {
        r["steps"] = { draft: "done", shorten: "running", review: "done" };
      }),
    },
    {
      name: "a job ID of 31 digits",
      value: changed("done", (r) => {
        r["jobId"] = jobId.slice(1);
      }),
    },
    {
      name: "an uppercase job ID",
      value: changed("done", (r) => {
        r["jobId"] = jobId.toUpperCase();
      }),
    },
    {
      name: "an input digest of 63 digits",
      value: changed("done", (r) => {
        r["inputDigest"] = "ab".repeat(32).slice(1);
      }),
    },
    {
      name: "an instance ID of 33 digits",
      value: changed("running", (r) => {
        r["runner"] = { tabId: 7, instanceId: `${instanceId}0` };
      }),
    },
    {
      name: "a negative tab ID",
      value: changed("running", (r) => {
        r["runner"] = { tabId: -1, instanceId };
      }),
    },
    {
      name: "a heading with a missing member",
      value: changed("done", (r) => {
        r["heading"] = { videoTitle: "Title" };
      }),
    },
    {
      name: "a fractional time",
      value: changed("done", (r) => {
        r["acceptedAt"] = 1.5;
      }),
    },
    {
      name: "an article with an empty Markdown",
      value: changed("done", (r) => {
        r["article"] = { ...article, markdown: "" };
      }),
    },
    { name: "undefined", value: undefined },
    { name: "a string", value: "job" },
    { name: "a BigInt", value: 1n },
  ];
  for (const { name, value } of absent) {
    it(`treats ${name} as absent`, () => {
      assert.equal(readJobRecord(jobId, value), undefined);
    });
  }

  it("treats another job's record as absent", () => {
    assert.equal(readJobRecord("f".repeat(32), records["done"]), undefined);
  });

  it("accepts a record of exactly the size limit and not one byte more", () => {
    // Pads the Markdown with three-byte characters so a character count
    // would differ from the byte count.
    const sized = (bytes: number): Record<string, unknown> => {
      const empty = { ...article, markdown: "#" };
      const record = { ...records["done"], article: empty };
      const baseBytes = new TextEncoder().encode(JSON.stringify(record)).length;
      let pad = "あ".repeat(Math.floor((bytes - baseBytes) / 3));
      pad += "a".repeat(bytes - baseBytes - 3 * pad.length);
      const result = { ...record, article: { ...empty, markdown: `#${pad}` } };
      assert.equal(
        new TextEncoder().encode(JSON.stringify(result)).length,
        bytes,
      );
      return result;
    };
    assert.ok(readJobRecord(jobId, sized(maxRecordBytes)) !== undefined);
    assert.equal(readJobRecord(jobId, sized(maxRecordBytes + 1)), undefined);
  });
});

describe("readStartContext", () => {
  const startId = "00112233445566778899aabbccddeeff";
  const context = {
    version: 1,
    startId,
    tabId: 9,
    mode: "inquiry",
    createdAt: 1_000,
    heading: "Title",
    channelName: "",
    videoTitle: "",
    videoUrl: "https://www.youtube.com/watch?v=dQw4w9WgXcQ",
  };

  it("reads a context as it was written", () => {
    assert.deepEqual(
      readStartContext(startId, structuredClone(context)),
      context,
    );
    assert.deepEqual(
      readStartContext(startId, { ...context, mode: "plain" })?.mode,
      "plain",
    );
  });

  const absent: { name: string; value: unknown }[] = [
    { name: "an unknown mode", value: { ...context, mode: "retry" } },
    { name: "the transcript", value: { ...context, transcript: "0:00 Hi" } },
    {
      name: "a missing channel name",
      value: Object.fromEntries(
        Object.entries(context).filter(([name]) => name !== "channelName"),
      ),
    },
    {
      name: "a start ID of 31 digits",
      value: { ...context, startId: startId.slice(1) },
    },
    { name: "another version", value: { ...context, version: 0 } },
  ];
  for (const { name, value } of absent) {
    it(`treats ${name} as absent`, () => {
      assert.equal(readStartContext(startId, value), undefined);
    });
  }

  it("treats another start's context as absent", () => {
    assert.equal(readStartContext("e".repeat(32), context), undefined);
  });
});
