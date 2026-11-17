import assert from "node:assert/strict";
import { describe, it } from "node:test";

import type { BrowserCall } from "../src/browser/chromeDeps.ts";
import {
  createJobCoordinator,
  contactWaitMs,
  maxRecords,
  startExpiryMs,
} from "../src/jobCoordinator.ts";
import type { JobCoordinator, RunnerPort } from "../src/jobCoordinator.ts";
import { jobItemName, startItemName } from "../src/core/jobRecord.ts";
import type { JobRecord } from "../src/core/jobRecord.ts";
import { parseJobViewerUrl } from "../src/core/jobViewerUrl.ts";
import type { MessageSender } from "../src/browser/chromeDeps.ts";
import {
  fakeExtensionId,
  fakeIdentity,
  fakeLocal,
  fakeSession,
  fakeTabs,
  popupSender,
  viewerSender,
} from "./helpers/chrome.ts";
import { fakeClock, settle } from "./helpers/clock.ts";
import { recordingLogger } from "./helpers/logger.ts";

const watchUrl = "https://www.youtube.com/watch?v=dQw4w9WgXcQ";
const input = {
  videoUrl: watchUrl,
  transcript: "0:00\nSECRET TRANSCRIPT\n",
  title: "Video",
  channelName: "Channel",
};
const digestA = "aa".repeat(32);
const digestB = "bb".repeat(32);
const digestC = "cc".repeat(32);
const videoA = "dQw4w9WgXcQ";
const videoB = "abcdefghijk";
const instance = (n: number): string => n.toString(16).padStart(32, "0");
const steps = {
  draft: "pending",
  shorten: "pending",
  review: "pending",
} as const;
const doneSteps = {
  draft: "done",
  shorten: "skipped",
  review: "done",
} as const;
const article = {
  markdown: "# T\n\nBody.",
  mentionsNeutralized: false,
  videoTitle: "Video",
  channelName: "Channel",
  file: "---\n",
  cost: { state: "known", usd: "0.1234" },
  durationMs: 1,
} as const;
const NO_REPLY = Symbol("no reply");

interface FakePort extends RunnerPort {
  disconnected: boolean;
  deliver(message: unknown): void;
  drop(): void;
}

function fakePort(sender: MessageSender, name = "runner"): FakePort {
  const messages: ((m: unknown) => void)[] = [];
  const drops: (() => void)[] = [];
  const port: FakePort = {
    name,
    sender,
    disconnected: false,
    onMessage: (l) => void messages.push(l),
    onDisconnect: (l) => void drops.push(l),
    disconnect: () => {
      port.disconnected = true;
    },
    deliver: (m) => messages.forEach((l) => l(m)),
    drop: () => drops.forEach((l) => l()),
  };
  return port;
}

interface Options {
  /** Makes the n-th write (1-based) to the session fail. */
  failWrites?: number[];
}

function setup(options: Options = {}) {
  const clock = fakeClock(1_000);
  const session = fakeSession();
  const local = fakeLocal();
  const tabs = fakeTabs();
  const logger = recordingLogger();
  local.items.set("anthropic.apiKey", "sk-ant-secret");
  let ids = 0;
  let writes = 0;
  const coordinator: JobCoordinator = createJobCoordinator({
    identity: fakeIdentity,
    tabs,
    keys: local.keys,
    readSession: session.read,
    writeSession: async (items) => {
      writes += 1;
      if (options.failWrites?.includes(writes)) {
        return {
          ok: false,
          error: "write failed",
        } satisfies BrowserCall<undefined>;
      }
      return session.write(items);
    },
    removeSession: session.remove,
    newId: () => `e${(++ids).toString(16).padStart(31, "0")}`,
    schedule: clock.schedule,
    now: clock.now,
    log: logger.log,
  });
  /** Sends a message; resolves with the reply, or NO_REPLY when the listener left it unanswered. */
  const ask = (message: unknown, sender: MessageSender): Promise<unknown> =>
    new Promise((resolve) => {
      if (!coordinator.onMessage(message, sender, resolve)) {
        resolve(NO_REPLY);
      }
    });
  return {
    clock,
    session,
    local,
    tabs,
    logger,
    coordinator,
    ask,
    writes: () => writes,
  };
}
type Setup = ReturnType<typeof setup>;

/** A stored job record, with the members of its state. */
function seed(
  s: Setup,
  jobId: string,
  fields: Partial<Record<string, unknown>> & { state: JobRecord["state"] },
): JobRecord {
  const base = {
    version: 1,
    jobId,
    inputDigest: digestA,
    videoId: videoA,
    heading: { videoTitle: "Video", channelName: "Channel" },
    videoUrl: watchUrl,
    acceptedAt: 500,
    steps,
  };
  const byState: Record<JobRecord["state"], unknown> = {
    running: { runner: { tabId: 7, instanceId: instance(7) } },
    stopped: {
      endedAt: 600,
      runner: { tabId: 7, instanceId: instance(7) },
      stopReason: "tab_closed",
      steps: { ...steps, draft: "running" },
    },
    done: { endedAt: 600, article, steps: doneSteps },
    failed: {
      endedAt: 600,
      failure: "llm_failed",
      steps: { ...steps, draft: "failed" },
    },
  };
  const record = { ...base, ...(byState[fields.state] as object), ...fields };
  s.session.items.set(jobItemName(jobId), record);
  return record as unknown as JobRecord;
}

const jobId = (n: number): string => `1${n.toString(16).padStart(31, "0")}`;
const stored = (s: Setup, id: string): Record<string, unknown> | undefined =>
  s.session.items.get(jobItemName(id)) as Record<string, unknown> | undefined;

/** The Port of the runner of a seeded record, connected. */
async function connect(
  s: Setup,
  id: string,
  tabId = 7,
  instanceId = instance(7),
): Promise<FakePort> {
  const port = fakePort(viewerSender(tabId));
  s.coordinator.onConnect(port);
  port.deliver({ type: "hello", jobId: id, instanceId });
  await settle();
  return port;
}

/** Starts a run from the popup and returns what the viewer needs. */
async function startFlow(
  s: Setup,
  mode: "inquiry" | "plain" = "inquiry",
  payload: {
    videoUrl: string;
    transcript: string;
    title?: string;
    channelName?: string;
  } = input,
) {
  const popup = s.ask({ type: "start", mode, input: payload }, popupSender);
  await settle();
  const create = [...s.tabs.calls].reverse().find((c) => c.op === "create");
  assert.ok(create?.op === "create");
  const url = new URL(create.tab.url);
  const startId = url.searchParams.get("r");
  assert.ok(startId !== null);
  const tabId = Math.max(...s.tabs.open.keys());
  const viewer = viewerSender(tabId);
  const reserve = (digest: string, video: string, instanceId: string) =>
    s.ask(
      {
        type: "reserve",
        startId,
        inputDigest: digest,
        videoId: video,
        instanceId,
        heading: { videoTitle: "Video", channelName: "Channel" },
        videoUrl: watchUrl,
      },
      viewer,
    );
  return { popup, startId, tabId, viewer, reserve };
}

describe("a generation start", () => {
  it("opens the viewer in the background, keeps the input in memory only, and answers the popup after the reservation", async () => {
    const s = setup();
    const f = await startFlow(s);
    const create = s.tabs.calls.find((c) => c.op === "create");
    assert.ok(create?.op === "create");
    assert.equal(create.tab.active, false);
    const parsed = parseJobViewerUrl(new URL(create.tab.url).search, "");
    assert.ok(parsed.ok);
    assert.deepEqual(parsed.params.target, {
      kind: "start",
      startId: f.startId,
    });
    assert.equal(parsed.params.title, "Video");
    // The start context is in the session area; the transcript is not.
    const context = s.session.items.get(startItemName(f.startId)) as Record<
      string,
      unknown
    >;
    assert.equal(context.tabId, f.tabId);
    assert.equal(context.mode, "inquiry");
    assert.ok(
      !JSON.stringify([...s.session.items]).includes("SECRET TRANSCRIPT"),
    );
    // The popup is still waiting.
    let answered = false;
    void f.popup.then(() => {
      answered = true;
    });
    await settle();
    assert.equal(answered, false);
    const reply = await f.reserve(digestA, videoA, instance(1));
    assert.equal((reply as { status: string }).status, "new_run");
    const jobIdReply = (reply as { jobId: string }).jobId;
    assert.deepEqual(await f.popup, { status: "accepted", jobId: jobIdReply });
    assert.ok(!s.session.items.has(startItemName(f.startId)));
    assert.ok(
      !JSON.stringify([...s.session.items]).includes("SECRET TRANSCRIPT"),
    );
  });

  it("refuses without a key, and asks only for the names of the stored items", async () => {
    const s = setup();
    s.local.items.clear();
    assert.deepEqual(
      await s.ask({ type: "start", mode: "plain", input }, popupSender),
      { status: "rejected", reason: "api_key_missing" },
    );
    assert.equal(s.tabs.calls.length, 0);
    s.local.items.set("anthropic.apiKey", "x");
    await startFlow(s);
    assert.deepEqual(new Set(s.local.calls), new Set(["keys"]));
  });

  it("refuses as internal, and opens nothing, when the item names cannot be read", async () => {
    const s = setup();
    s.local.fail("keys");
    assert.deepEqual(
      await s.ask({ type: "start", mode: "plain", input }, popupSender),
      { status: "rejected", reason: "internal" },
    );
    assert.equal(s.tabs.calls.length, 0);
  });

  it("accepts the start only from the popup, and only in the exact request shape", async () => {
    const s = setup();
    const message = { type: "start", mode: "plain", input };
    const senders: MessageSender[] = [
      { ...popupSender, tab: { id: 3 } },
      viewerSender(4),
      { id: "other-extension", url: popupSender.url },
      { id: fakeExtensionId, url: `${popupSender.url}?x=1` },
      { id: fakeExtensionId },
      {},
    ];
    for (const sender of senders) {
      assert.equal(
        await s.ask(message, sender),
        NO_REPLY,
        JSON.stringify(sender),
      );
    }
    assert.equal(await s.ask({ ...message, extra: 1 }, popupSender), NO_REPLY);
    assert.equal(s.tabs.calls.length, 0);
    assert.equal(s.session.calls.length, 0);
  });

  it("refuses an input over 4 MiB without opening a tab", async () => {
    const s = setup();
    const big = { ...input, transcript: "a".repeat(4 * 1024 * 1024) };
    assert.equal(
      await s.ask({ type: "start", mode: "plain", input: big }, popupSender),
      NO_REPLY,
    );
    assert.equal(s.tabs.calls.length, 0);
  });

  it("refuses a URL that is not an accepted watch URL as invalid_body, without a tab", async () => {
    const s = setup();
    const reply = await s.ask(
      {
        type: "start",
        mode: "plain",
        input: { ...input, videoUrl: "https://example.com/" },
      },
      popupSender,
    );
    assert.deepEqual(reply, { status: "rejected", reason: "invalid_body" });
    assert.equal(s.tabs.calls.length, 0);
  });

  it("refuses as internal when the tab cannot be opened, or the context cannot be written, and leaves no tab", async () => {
    const noTab = setup();
    noTab.tabs.fail("create");
    assert.deepEqual(
      await noTab.ask({ type: "start", mode: "plain", input }, popupSender),
      { status: "rejected", reason: "internal" },
    );

    const noContext = setup({ failWrites: [1] });
    assert.deepEqual(
      await noContext.ask({ type: "start", mode: "plain", input }, popupSender),
      { status: "rejected", reason: "internal" },
    );
    assert.equal(noContext.tabs.open.size, 0);
    assert.equal(noContext.session.items.size, 0);
  });

  it("keeps the page title the URL carries to 64 code points, taken from the video title, else from the heading", async () => {
    const long = setup();
    await startFlow(long, "plain", {
      ...input,
      title: "t".repeat(100),
    });
    const created = long.tabs.calls.find((c) => c.op === "create");
    assert.ok(created?.op === "create");
    const parsed = parseJobViewerUrl(new URL(created.tab.url).search, "");
    assert.ok(parsed.ok);
    assert.equal(parsed.params.title, `${"t".repeat(63)}…`);

    const untitled = setup();
    const g = await startFlow(untitled, "plain", {
      videoUrl: watchUrl,
      transcript: "x",
    });
    const second = untitled.tabs.calls.find((c) => c.op === "create");
    assert.ok(second?.op === "create");
    const parsedTwo = parseJobViewerUrl(new URL(second.tab.url).search, "");
    assert.ok(parsedTwo.ok);
    assert.equal(parsedTwo.params.title, watchUrl);
    const context = untitled.session.items.get(
      startItemName(g.startId),
    ) as Record<string, unknown>;
    assert.equal(context.heading, watchUrl);
    assert.equal(context.videoTitle, "");
  });

  it("answers unknown, and forgets the input and the context, when no reservation comes in 60 seconds", async () => {
    const s = setup();
    const f = await startFlow(s);
    s.clock.advance(startExpiryMs - 1);
    await settle();
    assert.ok(s.session.items.has(startItemName(f.startId)));
    s.clock.advance(1);
    assert.deepEqual(await f.popup, { status: "unknown" });
    assert.ok(!s.session.items.has(startItemName(f.startId)));
    assert.deepEqual(
      await s.ask({ type: "claimInput", startId: f.startId }, f.viewer),
      undefined,
    );
  });

  it("answers the popup with the reason when the viewer rejects the input", async () => {
    const s = setup();
    const f = await startFlow(s);
    const reply = await s.ask(
      { type: "prepareRejected", startId: f.startId, reason: "invalid_body" },
      f.viewer,
    );
    assert.deepEqual(reply, { status: "stored" });
    assert.deepEqual(await f.popup, {
      status: "rejected",
      reason: "invalid_body",
    });
    assert.ok(!s.session.items.has(startItemName(f.startId)));
    assert.equal(
      [...s.session.items.keys()].filter((k) => k.startsWith("job:")).length,
      0,
    );
  });
});

describe("claiming the input", () => {
  it("hands the input over once, and only to the viewer of the start's tab", async () => {
    const s = setup();
    const f = await startFlow(s);
    const stranger = viewerSender(f.tabId + 50);
    assert.equal(
      await s.ask({ type: "claimInput", startId: f.startId }, stranger),
      undefined,
    );
    assert.equal(
      await s.ask({ type: "claimInput", startId: f.startId }, popupSender),
      NO_REPLY,
    );
    assert.equal(
      await s.ask(
        { type: "claimInput", startId: f.startId },
        { ...f.viewer, id: "x" },
      ),
      NO_REPLY,
    );
    assert.deepEqual(
      await s.ask({ type: "claimInput", startId: f.startId }, f.viewer),
      { status: "ok", input },
    );
    assert.deepEqual(
      await s.ask({ type: "claimInput", startId: f.startId }, f.viewer),
      { status: "gone" },
    );
  });

  it("answers gone when the worker restarted and the input was lost, though the context remains", async () => {
    const s = setup();
    const f = await startFlow(s);
    const fresh = setup();
    for (const [name, value] of s.session.items) {
      fresh.session.items.set(name, value);
    }
    fresh.tabs.open.set(f.tabId, 1);
    assert.deepEqual(
      await fresh.ask({ type: "claimInput", startId: f.startId }, f.viewer),
      { status: "gone" },
    );
  });
});

describe("the reservation", () => {
  it("adds a running record for a new input, with the viewer as its runner and no input in it", async () => {
    const s = setup();
    const f = await startFlow(s);
    const reply = (await f.reserve(digestA, videoA, instance(1))) as {
      status: string;
      jobId: string;
    };
    assert.equal(reply.status, "new_run");
    assert.deepEqual(stored(s, reply.jobId), {
      version: 1,
      jobId: reply.jobId,
      inputDigest: digestA,
      videoId: videoA,
      heading: { videoTitle: "Video", channelName: "Channel" },
      videoUrl: watchUrl,
      acceptedAt: 1_000,
      steps,
      state: "running",
      runner: { tabId: f.tabId, instanceId: instance(1) },
    });
  });

  it("answers only the viewer of the start's tab", async () => {
    const s = setup();
    const f = await startFlow(s);
    const message = {
      type: "reserve",
      startId: f.startId,
      inputDigest: digestA,
      videoId: videoA,
      instanceId: instance(1),
      heading: { videoTitle: "", channelName: "" },
      videoUrl: watchUrl,
    };
    assert.equal(await s.ask(message, viewerSender(f.tabId + 1)), undefined);
    assert.equal(await s.ask(message, popupSender), NO_REPLY);
    assert.equal(
      [...s.session.items.keys()].filter((k) => k.startsWith("job:")).length,
      0,
    );
  });

  it("shows the job of the same input, running or done, without adding a record", async () => {
    for (const state of ["running", "done"] as const) {
      const s = setup();
      seed(s, jobId(1), { state });
      await connect(s, jobId(1));
      const f = await startFlow(s, "plain");
      assert.deepEqual(await f.reserve(digestA, videoA, instance(1)), {
        status: "existing",
        jobId: jobId(1),
      });
      assert.deepEqual(await f.popup, { status: "accepted", jobId: jobId(1) });
      assert.equal(
        [...s.session.items.keys()].filter((k) => k.startsWith("job:")).length,
        1,
      );
    }
  });

  it("answers the same video, newest first, only for an inquiry, and only for a running or done record", async () => {
    const s = setup();
    seed(s, jobId(1), { state: "done", inputDigest: digestB, acceptedAt: 500 });
    seed(s, jobId(2), { state: "done", inputDigest: digestC, acceptedAt: 900 });
    seed(s, jobId(3), {
      state: "failed",
      inputDigest: "dd".repeat(32),
      acceptedAt: 950,
    });
    const f = await startFlow(s, "inquiry");
    assert.deepEqual(await f.reserve(digestA, videoA, instance(1)), {
      status: "same_video",
      jobId: jobId(2),
    });
    assert.deepEqual(await f.popup, { status: "same_video", jobId: jobId(2) });
    assert.equal(stored(s, jobId(2))?.state, "done");

    const plain = setup();
    seed(plain, jobId(1), { state: "done", inputDigest: digestB });
    const g = await startFlow(plain, "plain");
    const reply = (await g.reserve(digestA, videoA, instance(1))) as {
      status: string;
    };
    assert.equal(reply.status, "new_run");

    const otherVideo = setup();
    seed(otherVideo, jobId(1), {
      state: "done",
      inputDigest: digestB,
      videoId: videoB,
    });
    const h = await startFlow(otherVideo, "inquiry");
    assert.equal(
      ((await h.reserve(digestA, videoA, instance(1))) as { status: string })
        .status,
      "new_run",
    );
  });

  it("runs a failed or stopped record of the same input again under its own job ID", async () => {
    for (const state of ["failed", "stopped"] as const) {
      const s = setup();
      seed(s, jobId(1), { state });
      const f = await startFlow(s, "plain");
      assert.deepEqual(await f.reserve(digestA, videoA, instance(9)), {
        status: "new_run",
        jobId: jobId(1),
      });
      assert.deepEqual(stored(s, jobId(1)), {
        version: 1,
        jobId: jobId(1),
        inputDigest: digestA,
        videoId: videoA,
        heading: { videoTitle: "Video", channelName: "Channel" },
        videoUrl: watchUrl,
        acceptedAt: 1_000,
        steps,
        state: "running",
        runner: { tabId: f.tabId, instanceId: instance(9) },
      });
    }
  });

  it("allows two runs at a time and refuses the third without adding a record", async () => {
    const s = setup();
    seed(s, jobId(1), {
      state: "running",
      inputDigest: digestB,
      videoId: videoB,
      runner: { tabId: 7, instanceId: instance(7) },
    });
    seed(s, jobId(2), {
      state: "running",
      inputDigest: digestC,
      videoId: "xxxxxxxxxxx",
      runner: { tabId: 8, instanceId: instance(8) },
    });
    await connect(s, jobId(1), 7, instance(7));
    await connect(s, jobId(2), 8, instance(8));
    const f = await startFlow(s, "plain");
    assert.deepEqual(await f.reserve(digestA, videoA, instance(1)), {
      status: "rejected",
      reason: "too_many_requests",
    });
    assert.deepEqual(await f.popup, {
      status: "rejected",
      reason: "too_many_requests",
    });
    assert.equal(
      [...s.session.items.keys()].filter((k) => k.startsWith("job:")).length,
      2,
    );

    const one = setup();
    seed(one, jobId(1), {
      state: "running",
      inputDigest: digestB,
      videoId: videoB,
    });
    await connect(one, jobId(1));
    const g = await startFlow(one, "plain");
    assert.equal(
      ((await g.reserve(digestA, videoA, instance(1))) as { status: string })
        .status,
      "new_run",
    );
  });

  it("adds one record, not two, for two reservations of the same input that arrive together", async () => {
    const s = setup();
    const first = await startFlow(s, "plain");
    const second = await startFlow(s, "plain");
    // The first runner connects its Port as soon as it has its reply, as the
    // viewer does; the second reservation waits for that contact.
    const firstReply = first
      .reserve(digestA, videoA, instance(1))
      .then(async (reply) => {
        const { jobId: id } = reply as { jobId: string };
        await connect(s, id, first.tabId, instance(1));
        return reply;
      });
    const [a, b] = await Promise.all([
      firstReply,
      second.reserve(digestA, videoA, instance(2)),
    ]);
    const statuses = [
      (a as { status: string }).status,
      (b as { status: string }).status,
    ].sort();
    assert.deepEqual(statuses, ["existing", "new_run"]);
    assert.equal(
      [...s.session.items.keys()].filter((k) => k.startsWith("job:")).length,
      1,
    );
  });

  it("refuses with internal when the record cannot be written", async () => {
    const s = setup({ failWrites: [2] });
    const f = await startFlow(s);
    assert.deepEqual(await f.reserve(digestA, videoA, instance(1)), {
      status: "rejected",
      reason: "internal",
    });
    assert.deepEqual(await f.popup, { status: "rejected", reason: "internal" });
    assert.equal(
      [...s.session.items.keys()].filter((k) => k.startsWith("job:")).length,
      0,
    );
  });
});

describe("the limits on the records", () => {
  const jobKeys = (s: Setup): string[] =>
    [...s.session.items.keys()].filter((k) => k.startsWith("job:"));

  it("drops, at 50 records, the failed or stopped one that ended first, before any done one", async () => {
    const s = setup();
    for (let n = 1; n < maxRecords; n += 1) {
      seed(s, jobId(n), {
        state: "done",
        inputDigest: n.toString(16).padStart(64, "0"),
        videoId: videoB,
        endedAt: 100 + n,
      });
    }
    seed(s, jobId(60), {
      state: "failed",
      inputDigest: "ee".repeat(32),
      videoId: videoB,
      endedAt: 900,
    });
    seed(s, jobId(61), {
      state: "stopped",
      inputDigest: "ff".repeat(32),
      videoId: videoB,
      endedAt: 800,
    });
    // 51 seeded: the limit holds the new record too, so two are dropped.
    const f = await startFlow(s, "plain");
    assert.equal(
      ((await f.reserve(digestA, videoA, instance(1))) as { status: string })
        .status,
      "new_run",
    );
    assert.ok(!jobKeys(s).includes(jobItemName(jobId(61))));
    assert.ok(!jobKeys(s).includes(jobItemName(jobId(60))));
    assert.ok(jobKeys(s).includes(jobItemName(jobId(1))));
    assert.equal(jobKeys(s).length, maxRecords);
  });

  it("drops the done record that ended first when only done records are left", async () => {
    const s = setup();
    for (let n = 1; n <= maxRecords; n += 1) {
      seed(s, jobId(n), {
        state: "done",
        inputDigest: n.toString(16).padStart(64, "0"),
        videoId: videoB,
        endedAt: 1_000 - n,
      });
    }
    const f = await startFlow(s, "plain");
    assert.equal(
      ((await f.reserve(digestA, videoA, instance(1))) as { status: string })
        .status,
      "new_run",
    );
    assert.ok(!jobKeys(s).includes(jobItemName(jobId(maxRecords))));
    assert.ok(jobKeys(s).includes(jobItemName(jobId(1))));
  });

  it("drops records to keep the total at 8 MiB", async () => {
    const s = setup();
    const big = { ...article, markdown: "x".repeat(200_000) };
    for (let n = 1; n <= 45; n += 1) {
      seed(s, jobId(n), {
        state: "done",
        article: big,
        inputDigest: n.toString(16).padStart(64, "0"),
        videoId: videoB,
        endedAt: 100 + n,
      });
    }
    // 45 records of about 200 KB: over 8 MiB already, so the oldest go first.
    const f = await startFlow(s, "plain");
    assert.equal(
      ((await f.reserve(digestA, videoA, instance(1))) as { status: string })
        .status,
      "new_run",
    );
    assert.ok(!jobKeys(s).includes(jobItemName(jobId(1))));
    let total = 0;
    for (const key of jobKeys(s)) {
      total += new TextEncoder().encode(
        JSON.stringify(s.session.items.get(key)),
      ).length;
    }
    assert.ok(total <= 8 * 1024 * 1024);
  });
});

describe("a running record whose runner is not connected", () => {
  it("stops it as no_contact at once when its tab is gone, then runs the input again", async () => {
    const s = setup();
    seed(s, jobId(1), { state: "running" });
    const f = await startFlow(s, "plain");
    // Another input, so the stale record is left stopped and can be read.
    const reply = (await f.reserve(digestB, videoB, instance(1))) as {
      status: string;
    };
    assert.equal(reply.status, "new_run");
    assert.equal(stored(s, jobId(1))?.state, "stopped");
    assert.equal(stored(s, jobId(1))?.stopReason, "no_contact");
  });

  it("waits 10 seconds for the runner to reconnect, then stops it, outside the order of other requests", async () => {
    const s = setup();
    seed(s, jobId(1), { state: "running" });
    s.tabs.open.set(7, 1);
    const f = await startFlow(s, "plain");
    const reserving = f.reserve(digestB, videoB, instance(1));
    await settle();
    assert.equal(stored(s, jobId(1))?.state, "running");
    // Another request is answered meanwhile.
    const other = await s.ask(
      { type: "claimInput", startId: f.startId },
      f.viewer,
    );
    assert.deepEqual(other, { status: "ok", input });
    s.clock.advance(contactWaitMs - 1);
    await settle();
    assert.equal(stored(s, jobId(1))?.state, "running");
    s.clock.advance(1);
    assert.equal(((await reserving) as { status: string }).status, "new_run");
    assert.equal(stored(s, jobId(1))?.state, "stopped");
    assert.equal(stored(s, jobId(1))?.stopReason, "no_contact");
  });

  it("keeps it running when the runner reconnects in time, and shows it as the existing job", async () => {
    const s = setup();
    seed(s, jobId(1), { state: "running" });
    s.tabs.open.set(7, 1);
    const f = await startFlow(s, "plain");
    const reserving = f.reserve(digestA, videoA, instance(1));
    await settle();
    await connect(s, jobId(1));
    assert.deepEqual(await reserving, { status: "existing", jobId: jobId(1) });
    assert.equal(stored(s, jobId(1))?.state, "running");
  });

  it("does not stop a record that was reserved again while it waited", async () => {
    const s = setup();
    seed(s, jobId(1), { state: "running" });
    s.tabs.open.set(7, 1);
    const f = await startFlow(s, "plain");
    const reserving = f.reserve(digestB, videoB, instance(1));
    await settle();
    // The same job is taken over by another viewer, which connects, before the wait ends.
    seed(s, jobId(1), {
      state: "running",
      runner: { tabId: 9, instanceId: instance(9) },
    });
    s.tabs.open.set(9, 1);
    await connect(s, jobId(1), 9, instance(9));
    s.clock.advance(contactWaitMs);
    assert.equal(((await reserving) as { status: string }).status, "new_run");
    assert.equal(stored(s, jobId(1))?.state, "running");
    assert.deepEqual(
      (stored(s, jobId(1))?.runner as { tabId: number }).tabId,
      9,
    );
  });
});

describe("progress and results", () => {
  it("writes the progress of the runner and answers after the write", async () => {
    const s = setup();
    seed(s, jobId(1), { state: "running" });
    const message = {
      type: "progress",
      jobId: jobId(1),
      instanceId: instance(7),
      steps: { draft: "running", shorten: "pending", review: "pending" },
    };
    assert.deepEqual(await s.ask(message, viewerSender(7)), {
      status: "stored",
    });
    assert.deepEqual(stored(s, jobId(1))?.steps, message.steps);
    assert.equal(stored(s, jobId(1))?.state, "running");
  });

  it("answers not_runner, and changes nothing, for a sender that is not the runner", async () => {
    const s = setup();
    const before = seed(s, jobId(1), { state: "running" });
    const message = {
      type: "progress",
      jobId: jobId(1),
      instanceId: instance(7),
      steps: { draft: "running", shorten: "pending", review: "pending" },
    };
    for (const [m, sender] of [
      [message, viewerSender(8)],
      [{ ...message, instanceId: instance(8) }, viewerSender(7)],
      [{ ...message, jobId: jobId(2) }, viewerSender(7)],
    ] as const) {
      assert.deepEqual(await s.ask(m, sender), { status: "not_runner" });
    }
    assert.equal(await s.ask(message, popupSender), NO_REPLY);
    assert.equal(
      await s.ask(message, {
        id: fakeExtensionId,
        url: "https://example.com/",
        tab: { id: 7 },
      }),
      NO_REPLY,
    );
    assert.deepEqual(
      stored(s, jobId(1)),
      before as unknown as Record<string, unknown>,
    );
    assert.equal(s.session.calls.filter((c) => c === "write").length, 0);
  });

  it("records a done result without the runner, and a failure with its detail", async () => {
    const s = setup();
    seed(s, jobId(1), { state: "running" });
    const done = {
      type: "result",
      jobId: jobId(1),
      instanceId: instance(7),
      result: { state: "done", steps: doneSteps, article },
    };
    assert.deepEqual(await s.ask(done, viewerSender(7)), { status: "stored" });
    assert.deepEqual(stored(s, jobId(1)), {
      version: 1,
      jobId: jobId(1),
      inputDigest: digestA,
      videoId: videoA,
      heading: { videoTitle: "Video", channelName: "Channel" },
      videoUrl: watchUrl,
      acceptedAt: 500,
      steps: doneSteps,
      state: "done",
      endedAt: 1_000,
      article,
    });

    const failed = setup();
    seed(failed, jobId(1), { state: "running" });
    const result = {
      state: "failed",
      steps: { draft: "failed", shorten: "pending", review: "pending" },
      failure: "internal",
      internalDetail: "worker_error",
    };
    assert.deepEqual(
      await failed.ask(
        { type: "result", jobId: jobId(1), instanceId: instance(7), result },
        viewerSender(7),
      ),
      { status: "stored" },
    );
    const record = stored(failed, jobId(1));
    assert.equal(record?.state, "failed");
    assert.equal(record?.failure, "internal");
    assert.equal(record?.internalDetail, "worker_error");
    assert.ok(!("runner" in (record ?? {})));
  });

  it("takes the result of a stopped record from the same runner, and returns it to done", async () => {
    const s = setup();
    seed(s, jobId(1), { state: "stopped" });
    const done = {
      type: "result",
      jobId: jobId(1),
      instanceId: instance(7),
      result: { state: "done", steps: doneSteps, article },
    };
    assert.deepEqual(await s.ask(done, viewerSender(7)), { status: "stored" });
    assert.equal(stored(s, jobId(1))?.state, "done");
    assert.ok(!("stopReason" in (stored(s, jobId(1)) ?? {})));
  });

  it("does not take the result of a stopped record when the input was reserved again", async () => {
    const s = setup();
    seed(s, jobId(1), {
      state: "stopped",
      runner: { tabId: 9, instanceId: instance(9) },
    });
    const done = {
      type: "result",
      jobId: jobId(1),
      instanceId: instance(7),
      result: { state: "done", steps: doneSteps, article },
    };
    assert.deepEqual(await s.ask(done, viewerSender(7)), {
      status: "not_runner",
    });
    assert.equal(stored(s, jobId(1))?.state, "stopped");
  });

  it("takes progress of a stopped record without leaving the stopped state", async () => {
    const s = setup();
    seed(s, jobId(1), { state: "stopped" });
    const message = {
      type: "progress",
      jobId: jobId(1),
      instanceId: instance(7),
      steps: { draft: "done", shorten: "running", review: "pending" },
    };
    assert.deepEqual(await s.ask(message, viewerSender(7)), {
      status: "stored",
    });
    assert.equal(stored(s, jobId(1))?.state, "stopped");
    assert.deepEqual(stored(s, jobId(1))?.steps, message.steps);
  });

  it("answers stored to a result delivered again, and not_runner to a different one, once the record is done", async () => {
    const s = setup();
    seed(s, jobId(1), { state: "running" });
    const done = {
      type: "result",
      jobId: jobId(1),
      instanceId: instance(7),
      result: { state: "done", steps: doneSteps, article },
    };
    await s.ask(done, viewerSender(7));
    s.clock.advance(5_000);
    assert.deepEqual(await s.ask(done, viewerSender(7)), { status: "stored" });
    assert.equal(stored(s, jobId(1))?.endedAt, 1_000);
    const other = {
      ...done,
      result: { ...done.result, article: { ...article, markdown: "other" } },
    };
    assert.deepEqual(await s.ask(other, viewerSender(7)), {
      status: "not_runner",
    });
    assert.deepEqual(
      await s.ask(
        {
          type: "progress",
          jobId: jobId(1),
          instanceId: instance(7),
          steps: doneSteps,
        },
        viewerSender(7),
      ),
      { status: "not_runner" },
    );
  });

  it("drops a record by the rule and writes again once, when a write fails", async () => {
    const s = setup({ failWrites: [1] });
    seed(s, jobId(1), { state: "running" });
    seed(s, jobId(2), { state: "failed", inputDigest: digestB, endedAt: 100 });
    const done = {
      type: "result",
      jobId: jobId(1),
      instanceId: instance(7),
      result: { state: "done", steps: doneSteps, article },
    };
    assert.deepEqual(await s.ask(done, viewerSender(7)), { status: "stored" });
    assert.equal(stored(s, jobId(1))?.state, "done");
    assert.equal(stored(s, jobId(2)), undefined);
  });

  it("answers not_stored and tries once to mark the record failed with store_failed, when the write fails twice", async () => {
    const s = setup({ failWrites: [1, 2] });
    seed(s, jobId(1), { state: "running" });
    seed(s, jobId(2), { state: "failed", inputDigest: digestB, endedAt: 100 });
    const done = {
      type: "result",
      jobId: jobId(1),
      instanceId: instance(7),
      result: { state: "done", steps: doneSteps, article },
    };
    assert.deepEqual(await s.ask(done, viewerSender(7)), {
      status: "not_stored",
    });
    assert.equal(stored(s, jobId(1))?.state, "failed");
    assert.equal(stored(s, jobId(1))?.internalDetail, "store_failed");
    assert.equal(s.writes(), 3);
  });

  it("answers not_stored without writing for a record over the read-back limit", async () => {
    const s = setup();
    seed(s, jobId(1), { state: "running" });
    const huge = { ...article, markdown: "x".repeat(600 * 1024) };
    const done = {
      type: "result",
      jobId: jobId(1),
      instanceId: instance(7),
      result: { state: "done", steps: doneSteps, article: huge },
    };
    assert.deepEqual(await s.ask(done, viewerSender(7)), {
      status: "not_stored",
    });
    assert.equal(stored(s, jobId(1))?.state, "running");
  });
});

describe("the ways a run is found to have stopped", () => {
  it("stops the running records of a closed tab as tab_closed, and no others", async () => {
    const s = setup();
    seed(s, jobId(1), { state: "running" });
    seed(s, jobId(2), {
      state: "running",
      inputDigest: digestB,
      runner: { tabId: 8, instanceId: instance(8) },
    });
    seed(s, jobId(3), { state: "done", inputDigest: digestC });
    s.coordinator.onTabRemoved(7);
    await settle();
    assert.equal(stored(s, jobId(1))?.state, "stopped");
    assert.equal(stored(s, jobId(1))?.stopReason, "tab_closed");
    assert.equal(typeof stored(s, jobId(1))?.endedAt, "number");
    assert.equal(stored(s, jobId(2))?.state, "running");
    assert.equal(stored(s, jobId(3))?.state, "done");
  });

  it("stops the record as page_reloaded when the page of the same tab reports a reload", async () => {
    const s = setup();
    seed(s, jobId(1), { state: "running" });
    const message = {
      type: "runnerReloaded",
      jobId: jobId(1),
      instanceId: instance(8),
    };
    assert.deepEqual(await s.ask(message, viewerSender(8)), {
      status: "not_runner",
    });
    assert.deepEqual(
      await s.ask({ ...message, instanceId: instance(7) }, viewerSender(7)),
      { status: "not_runner" },
    );
    assert.equal(stored(s, jobId(1))?.state, "running");
    assert.deepEqual(await s.ask(message, viewerSender(7)), {
      status: "stored",
    });
    assert.equal(stored(s, jobId(1))?.stopReason, "page_reloaded");
  });

  it("stops the record as page_closed when the runner's Port is cut, and not when the runner has reconnected", async () => {
    const s = setup();
    seed(s, jobId(1), { state: "running" });
    const first = await connect(s, jobId(1));
    const second = await connect(s, jobId(1));
    first.drop();
    await settle();
    assert.equal(stored(s, jobId(1))?.state, "running");
    second.drop();
    await settle();
    assert.equal(stored(s, jobId(1))?.state, "stopped");
    assert.equal(stored(s, jobId(1))?.stopReason, "page_closed");
  });

  it("accepts only a runner's Port whose first message names the record's runner", async () => {
    const s = setup();
    seed(s, jobId(1), { state: "running" });
    const bad: [FakePort, unknown][] = [
      [
        fakePort(viewerSender(7), "other"),
        { type: "hello", jobId: jobId(1), instanceId: instance(7) },
      ],
      [
        fakePort(popupSender),
        { type: "hello", jobId: jobId(1), instanceId: instance(7) },
      ],
      [
        fakePort(viewerSender(8)),
        { type: "hello", jobId: jobId(1), instanceId: instance(7) },
      ],
      [
        fakePort(viewerSender(7)),
        { type: "hello", jobId: jobId(1), instanceId: instance(8) },
      ],
      [
        fakePort(viewerSender(7)),
        { type: "hello", jobId: jobId(2), instanceId: instance(7) },
      ],
      [
        fakePort(viewerSender(7)),
        { type: "hello", jobId: jobId(1), instanceId: instance(7), extra: 1 },
      ],
    ];
    for (const [port, hello] of bad) {
      s.coordinator.onConnect(port);
      port.deliver(hello);
      await settle();
      assert.ok(port.disconnected, JSON.stringify(hello));
      port.drop();
    }
    await settle();
    assert.equal(stored(s, jobId(1))?.state, "running");
    const good = fakePort(viewerSender(7));
    s.coordinator.onConnect(good);
    good.deliver({ type: "hello", jobId: jobId(1), instanceId: instance(7) });
    await settle();
    assert.equal(good.disconnected, false);
  });

  it("lets the runner of a stopped record reconnect", async () => {
    const s = setup();
    seed(s, jobId(1), { state: "stopped" });
    const port = await connect(s, jobId(1));
    assert.equal(port.disconnected, false);
  });
});

describe("review fixes", () => {
  it("does not count a Port cut while its hello was being checked as connected", async () => {
    const s = setup();
    seed(s, jobId(1), { state: "running" });
    const port = fakePort(viewerSender(7));
    s.coordinator.onConnect(port);
    port.deliver({ type: "hello", jobId: jobId(1), instanceId: instance(7) });
    port.drop();
    await settle();
    assert.equal(stored(s, jobId(1))?.state, "stopped");
    assert.equal(stored(s, jobId(1))?.stopReason, "page_closed");
  });

  it("makes room before a result is written, so the total stays within 8 MiB", async () => {
    const s = setup();
    const big = { ...article, markdown: "x".repeat(250_000) };
    seed(s, jobId(1), { state: "running" });
    for (let n = 2; n <= 35; n += 1) {
      seed(s, jobId(n), {
        state: n === 2 ? "failed" : "done",
        ...(n === 2 ? {} : { article: big }),
        inputDigest: n.toString(16).padStart(64, "0"),
        videoId: videoB,
        endedAt: 100 + n,
      });
    }
    const result = {
      state: "done",
      steps: doneSteps,
      article: { ...article, markdown: "y".repeat(400_000) },
    };
    assert.deepEqual(
      await s.ask(
        { type: "result", jobId: jobId(1), instanceId: instance(7), result },
        viewerSender(7),
      ),
      { status: "stored" },
    );
    assert.equal(stored(s, jobId(2)), undefined);
    let total = 0;
    for (const [name, value] of s.session.items) {
      if (name.startsWith("job:")) {
        total += new TextEncoder().encode(JSON.stringify(value)).length;
      }
    }
    assert.ok(total <= 8 * 1024 * 1024, String(total));
  });

  it("takes the heading and video URL from the start context, and refuses a start whose context is over the limit", async () => {
    const s = setup();
    const f = await startFlow(s, "plain");
    const reply = (await s.ask(
      {
        type: "reserve",
        startId: f.startId,
        inputDigest: digestA,
        videoId: videoA,
        instanceId: instance(1),
        heading: { videoTitle: "from viewer", channelName: "x" },
        videoUrl: watchUrl,
      },
      f.viewer,
    )) as { jobId: string };
    assert.deepEqual(stored(s, reply.jobId)?.heading, {
      videoTitle: "Video",
      channelName: "Channel",
    });

    // A start context over the read-back limit is refused at the start.
    const huge = setup();
    const refused = await huge.ask(
      {
        type: "start",
        mode: "plain",
        input: { ...input, title: "t".repeat(600 * 1024) },
      },
      popupSender,
    );
    assert.deepEqual(refused, { status: "rejected", reason: "invalid_body" });
    assert.equal(huge.tabs.open.size, 0);
    assert.equal(huge.session.items.size, 0);
  });

  it("starts with a title that has a control character, using the video URL for the page title", async () => {
    for (const title of ["a\tb", "a\u0085b"]) {
      const s = setup();
      const f = await startFlow(s, "plain", { ...input, title });
      const created = s.tabs.calls.find((c) => c.op === "create");
      assert.ok(created?.op === "create");
      const parsed = parseJobViewerUrl(new URL(created.tab.url).search, "");
      assert.ok(parsed.ok);
      assert.equal(parsed.params.title, watchUrl);
      assert.ok(f.startId);
    }
  });

  it("does not wait for a runner that connected while its tab was looked up", async () => {
    const s = setup();
    seed(s, jobId(1), { state: "running" });
    s.tabs.open.set(7, 1);
    const f = await startFlow(s, "plain");
    const get = s.tabs.get;
    s.tabs.get = async (tabId) => {
      const answer = await get(tabId);
      await connect(s, jobId(1));
      return answer;
    };
    assert.deepEqual(await f.reserve(digestA, videoA, instance(1)), {
      status: "existing",
      jobId: jobId(1),
    });
  });
});
