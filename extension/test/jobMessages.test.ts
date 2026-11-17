import assert from "node:assert/strict";
import { describe, it } from "node:test";

import {
  chooseViewer,
  parseClaimInputReply,
  parseJobChannelMessage,
  parseReserveReply,
  parseRunnerHello,
  parseStartReply,
  parseSwRequest,
  parseWriteReply,
} from "../src/core/jobMessages.ts";
import type { PresentAnswer } from "../src/core/jobMessages.ts";
import { maxBodyBytes } from "../src/core/prepare.ts";

const jobId = "0123456789abcdef0123456789abcdef";
const instanceId = "fedcba9876543210fedcba9876543210";
const doneSteps = { draft: "done", shorten: "skipped", review: "done" };
const article = {
  markdown: "# Title\n\nBody.",
  mentionsNeutralized: false,
  videoTitle: "Title",
  channelName: "Channel",
  file: "---\n",
  cost: { state: "known", usd: "0.1234" },
  durationMs: 1000,
};
const input = {
  videoUrl: "https://www.youtube.com/watch?v=dQw4w9WgXcQ",
  transcript: "0:00\nHi\n",
};

const source = { tabId: 4, windowId: 2, index: 9 };

const requests: Record<string, unknown> = {
  start: { type: "start", mode: "inquiry", input, source },
  startWithMetadata: {
    type: "start",
    mode: "plain",
    input: { ...input, title: "T", channelName: "C", description: "D" },
    source,
  },
  claimInput: { type: "claimInput", startId: jobId },
  prepareRejected: {
    type: "prepareRejected",
    startId: jobId,
    reason: "invalid_body",
  },
  prepareInternal: {
    type: "prepareRejected",
    startId: jobId,
    reason: "internal",
    internalDetail: "worker_load",
  },
  reserve: {
    type: "reserve",
    startId: jobId,
    inputDigest: "ab".repeat(32),
    videoId: "dQw4w9WgXcQ",
    instanceId,
    heading: { videoTitle: "T", channelName: "C" },
    videoUrl: input.videoUrl,
  },
  progress: {
    type: "progress",
    jobId,
    instanceId,
    steps: { draft: "running", shorten: "pending", review: "pending" },
  },
  resultDone: {
    type: "result",
    jobId,
    instanceId,
    result: { state: "done", steps: doneSteps, article },
  },
  resultFailed: {
    type: "result",
    jobId,
    instanceId,
    result: { state: "failed", steps: doneSteps, failure: "run_timeout" },
  },
  resultInternal: {
    type: "result",
    jobId,
    instanceId,
    result: {
      state: "failed",
      steps: doneSteps,
      failure: "internal",
      internalDetail: "settings_read",
    },
  },
  runnerReloaded: { type: "runnerReloaded", jobId, instanceId },
};

const remove = Symbol("remove");

/** A copy of a request fixture with the member at path (dot-separated) set to value, or removed. */
function mutated(name: string, path: string, value: unknown): unknown {
  const copy = structuredClone(requests[name]);
  const keys = path.split(".");
  let target = copy as Record<string, unknown>;
  for (const key of keys.slice(0, -1)) {
    target = target[key] as Record<string, unknown>;
  }
  const last = keys.at(-1) as string;
  if (value === remove) {
    delete target[last];
  } else {
    target[last] = value;
  }
  return copy;
}

describe("parseSwRequest", () => {
  it("accepts every request in its exact shape, unchanged", () => {
    for (const [name, request] of Object.entries(requests)) {
      const parsed = parseSwRequest(request);
      assert.ok(parsed.ok, name);
      assert.deepEqual(parsed.request, request, name);
    }
  });

  it("rejects an unknown or missing member, a wrong type and a bad value by shape", () => {
    const bad: unknown[] = [
      null,
      "start",
      [],
      { type: "unknown" },
      mutated("start", "extra", 1),
      mutated("start", "mode", remove),
      mutated("start", "mode", "other"),
      mutated("start", "input.extra", "x"),
      mutated("start", "input.title", 5),
      mutated("start", "input.transcript", remove),
      mutated("start", "source", remove),
      mutated("start", "source.index", -1),
      mutated("start", "source.extra", 1),
      mutated("claimInput", "startId", "XYZ"),
      mutated("prepareRejected", "internalDetail", "worker_load"),
      mutated("prepareInternal", "internalDetail", "nope"),
      mutated("reserve", "inputDigest", "ab"),
      mutated("reserve", "videoId", "short"),
      mutated("reserve", "heading.extra", "x"),
      mutated("progress", "steps.draft", "bad"),
      mutated("progress", "jobId", remove),
      mutated("resultDone", "result.steps.review", "running"),
      mutated("resultDone", "result.article.cost.usd", "1"),
      mutated("resultDone", "result.failure", "internal"),
      mutated("resultFailed", "result.failure", "nope"),
      mutated("resultFailed", "result.internalDetail", "panic"),
      mutated("resultFailed", "result.article", article),
      mutated("resultInternal", "result.internalDetail", "nope"),
      mutated("runnerReloaded", "instanceId", "x"),
    ];
    for (const value of bad) {
      const parsed = parseSwRequest(value);
      assert.ok(
        !parsed.ok && parsed.check === "shape",
        JSON.stringify(value)?.slice(0, 80),
      );
    }
  });

  it("rejects a value that has no JSON form", () => {
    assert.ok(!parseSwRequest(undefined).ok);
    const cycle: Record<string, unknown> = { type: "claimInput" };
    cycle.self = cycle;
    assert.ok(!parseSwRequest(cycle).ok);
  });

  it("limits a start's input to 4 MiB of JSON, counted in bytes", () => {
    const wrapper = JSON.stringify({ ...input, transcript: "" }).length;
    const atLimit = "a".repeat(maxBodyBytes - wrapper);
    const at = {
      type: "start",
      mode: "plain",
      input: { ...input, transcript: atLimit },
      source,
    };
    assert.ok(parseSwRequest(at).ok);
    const over = { ...at, input: { ...at.input, transcript: `${atLimit}a` } };
    const parsed = parseSwRequest(over);
    assert.ok(!parsed.ok && parsed.check === "size");
    // Multi-byte text counts by its UTF-8 bytes, not its length.
    const multibyte = {
      ...at,
      input: { ...at.input, transcript: "あ".repeat(maxBodyBytes / 3) },
    };
    const second = parseSwRequest(multibyte);
    assert.ok(!second.ok && second.check === "size");
  });
});

describe("the reply parsers", () => {
  it("accept each reply in its exact shape and reject any other", () => {
    assert.deepEqual(parseStartReply({ status: "accepted", jobId }), {
      status: "accepted",
      jobId,
    });
    assert.deepEqual(parseStartReply({ status: "same_video", jobId }), {
      status: "same_video",
      jobId,
    });
    assert.deepEqual(
      parseStartReply({ status: "rejected", reason: "api_key_missing" }),
      {
        status: "rejected",
        reason: "api_key_missing",
      },
    );
    assert.deepEqual(parseStartReply({ status: "unknown" }), {
      status: "unknown",
    });
    assert.equal(parseStartReply({ status: "accepted" }), undefined);
    assert.equal(
      parseStartReply({ status: "rejected", reason: "other" }),
      undefined,
    );
    assert.equal(parseStartReply({ status: "unknown", jobId }), undefined);

    assert.deepEqual(parseClaimInputReply({ status: "ok", input }), {
      status: "ok",
      input,
    });
    assert.deepEqual(parseClaimInputReply({ status: "gone" }), {
      status: "gone",
    });
    assert.equal(parseClaimInputReply({ status: "ok" }), undefined);

    for (const status of ["new_run", "existing", "same_video"] as const) {
      assert.deepEqual(parseReserveReply({ status, jobId }), { status, jobId });
    }
    assert.deepEqual(
      parseReserveReply({ status: "rejected", reason: "too_many_requests" }),
      {
        status: "rejected",
        reason: "too_many_requests",
      },
    );
    assert.equal(
      parseReserveReply({ status: "rejected", reason: "invalid_body" }),
      undefined,
    );

    for (const status of ["stored", "not_stored", "not_runner"] as const) {
      assert.deepEqual(parseWriteReply({ status }), { status });
    }
    assert.equal(parseWriteReply({ status: "stored", extra: 1 }), undefined);
    assert.equal(parseWriteReply("stored"), undefined);
  });
});

describe("parseRunnerHello and parseJobChannelMessage", () => {
  it("accept the exact shapes", () => {
    const hello = { type: "hello", jobId, instanceId };
    assert.deepEqual(parseRunnerHello(hello), hello);
    assert.equal(parseRunnerHello({ ...hello, type: "bye" }), undefined);
    assert.equal(parseRunnerHello({ ...hello, extra: 1 }), undefined);
    const find = { type: "find", queryId: instanceId, jobId };
    assert.deepEqual(parseJobChannelMessage(find), find);
    const present = {
      type: "present",
      queryId: instanceId,
      tabId: 3,
      windowId: 1,
    };
    assert.deepEqual(parseJobChannelMessage(present), present);
    assert.equal(parseJobChannelMessage({ ...present, tabId: -1 }), undefined);
    assert.equal(parseJobChannelMessage({ ...find, jobId: "x" }), undefined);
    assert.equal(parseJobChannelMessage({ type: "other" }), undefined);
  });
});

describe("chooseViewer", () => {
  it("chooses the smallest tab ID, and none without answers", () => {
    const answer = (tabId: number): PresentAnswer => ({
      type: "present",
      queryId: jobId,
      tabId,
      windowId: 1,
    });
    assert.equal(chooseViewer([]), undefined);
    assert.equal(chooseViewer([answer(9), answer(4), answer(6)])?.tabId, 4);
  });
});
