import assert from "node:assert/strict";
import { describe, it } from "node:test";

import type { BrowserCall, TabPlace } from "../src/browser/chromeDeps.ts";
import { articleFileName } from "../src/core/fileName.ts";
import { catalogs } from "../src/core/i18n/catalog.ts";
import { formatDuration } from "../src/core/i18n/locale.ts";
import { parseJobChannelMessage } from "../src/core/jobMessages.ts";
import type { SwRequest } from "../src/core/jobMessages.ts";
import { jobItemName, startItemName } from "../src/core/jobRecord.ts";
import type { JobRecord } from "../src/core/jobRecord.ts";
import { jobViewerQuery, parseJobViewerUrl } from "../src/core/jobViewerUrl.ts";
import type { JobViewerParams } from "../src/core/jobViewerUrl.ts";
import { elapsedRedrawMs } from "../src/core/jobView.ts";
import type { ViewerState } from "../src/core/jobView.ts";
import { renderViewer } from "../src/ui/renderViewer.ts";
import { startViewer } from "../src/viewerApp.ts";
import { startWasmWorker } from "../src/wasmWorkerClient.ts";
import {
  fakeChannels,
  fakeClipboard,
  fakeLocal,
  fakeSaves,
  fakeSession,
  fakeTabs,
  type FakeChannels,
  type FakeClipboard,
  type FakeLocal,
  type FakeSaves,
  type FakeSession,
  type FakeTabs,
} from "./helpers/chrome.ts";
import { fakeClock, settle, type FakeClock } from "./helpers/clock.ts";
import { viewerDocument, viewerElements } from "./helpers/dom.ts";
import { recordingLogger, type RecordingLogger } from "./helpers/logger.ts";
import {
  article,
  doneRecord,
  failedRecord,
  jobId,
  runningRecord,
  startContext,
  startId,
  watchUrl,
} from "./helpers/record.ts";
import { fakeWorkers, type FakeWorkers } from "./helpers/worker.ts";

const ownTab: TabPlace = { tabId: 7, windowId: 2 };
const otherJob = "fedcba9876543210fedcba9876543210";
const newIds = ["a".repeat(32), "b".repeat(32), "c".repeat(32)];
const secret = "sk-ant-secret-value";
const digest = "ab".repeat(32);
const input = {
  videoUrl: watchUrl,
  transcript: "0:00\nHello\n",
  title: "A video title",
  channelName: "A channel",
};
const preparedOk = JSON.stringify({
  status: "ok",
  inputDigest: digest,
  videoId: "dQw4w9WgXcQ",
});

const ok = <T>(value: T): BrowserCall<T> => ({ ok: true, value });
/** A video title as the module neutralizes it: U+200B after the '@'. */
const neutralTitle = "Neutral @\u200btitle";
const queryA = "1".repeat(32);
const queryB = "2".repeat(32);

type Replies = Partial<Record<SwRequest["type"], () => BrowserCall<unknown>>>;

interface Viewer {
  readonly doc: Document;
  readonly clock: FakeClock;
  readonly session: FakeSession;
  readonly local: FakeLocal;
  readonly tabs: FakeTabs;
  readonly channels: FakeChannels;
  readonly workers: FakeWorkers;
  readonly clipboard: FakeClipboard;
  readonly saves: FakeSaves;
  readonly logger: RecordingLogger;
  /** Every request to the service worker, in order. */
  readonly requests: SwRequest[];
  /** The service worker's replies by request type; a test may change them. */
  readonly replies: Replies;
  /** Each query the page wrote into its URL, in order. */
  readonly replaced: string[];
  readonly options: { opened: number };
  /** Makes the page hidden or visible; becoming visible fires its listeners. */
  setVisible(visible: boolean): void;
  readonly elements: ReturnType<typeof viewerElements>;
  /** The end of startViewer, which waits for the run of a start URL. */
  readonly done: Promise<void>;
}

interface Options {
  readonly query: string;
  readonly hash?: string;
  readonly uiLanguage?: string;
  readonly buildKind?: "release" | "dev";
  /** The page's own tab; undefined outside a tab. */
  readonly tab?: TabPlace | undefined;
  /** Items in chrome.storage.session before the page loads. */
  readonly items?: readonly (readonly [string, unknown])[];
  readonly replies?: Replies;
}

/** The query of a viewer URL. */
function queryOf(params: Partial<JobViewerParams> = {}): string {
  return jobViewerQuery({
    target: { kind: "job", jobId },
    videoUrl: watchUrl,
    title: "A URL title",
    ...params,
  });
}

const startQuery = queryOf({ target: { kind: "start", startId } });

/** Starts a viewer on a fresh viewer.html and lets it run until it waits. */
async function open(options: Options): Promise<Viewer> {
  const doc = viewerDocument();
  const clock = fakeClock(10_000);
  const session = fakeSession();
  for (const [name, value] of options.items ?? []) {
    session.items.set(name, value);
  }
  const local = fakeLocal();
  local.items.set("anthropic.apiKey", secret);
  local.items.set("anthropic.workspaceId", "");
  const tabs = fakeTabs();
  tabs.current = "tab" in options ? options.tab : ownTab;
  if (tabs.current !== undefined) {
    tabs.open.set(tabs.current.tabId, tabs.current.windowId);
  }
  const channels = fakeChannels();
  const workers = fakeWorkers();
  const clipboard = fakeClipboard();
  const saves = fakeSaves();
  const logger = recordingLogger();
  const requests: SwRequest[] = [];
  const replies: Replies = {
    claimInput: () => ok({ status: "ok", input }),
    reserve: () => ok({ status: "new_run", jobId }),
    prepareRejected: () => ok({ status: "stored" }),
    progress: () => ok({ status: "stored" }),
    result: () => ok({ status: "stored" }),
    runnerReloaded: () => ok({ status: "stored" }),
    ...options.replies,
  };
  const replaced: string[] = [];
  const options$ = { opened: 0 };
  let visible = true;
  const visibleListeners: (() => void)[] = [];
  const ids = [...newIds];
  const done = startViewer(doc, {
    uiLanguage: () => options.uiLanguage ?? "en-US",
    buildKind: options.buildKind ?? "release",
    page: {
      search: `?${options.query}`,
      hash: options.hash ?? "",
      replaceQuery: (query) => replaced.push(query),
      isVisible: () => visible,
      onVisible: (listener) => visibleListeners.push(listener),
    },
    tabs,
    sendToSw: async (request) => {
      requests.push(structuredClone(request));
      const reply = replies[request.type];
      return reply === undefined ? { ok: false, error: "no reply" } : reply();
    },
    readSession: session.read,
    onSessionChanged: session.onChanged,
    openChannel: channels.open,
    newId: () => ids.shift() ?? "f".repeat(32),
    runner: {
      startWorker: () => startWasmWorker(workers.create, clock.schedule),
      connectPort: () => () => {},
      readSettings: local.read,
      confirmUnload: () => {},
    },
    schedule: clock.schedule,
    now: clock.now,
    openOptions: async () => {
      options$.opened += 1;
      return ok(undefined);
    },
    writeClipboard: clipboard.write,
    saveFile: saves.save,
    log: logger.log,
  });
  await settle();
  return {
    doc,
    clock,
    session,
    local,
    tabs,
    channels,
    workers,
    clipboard,
    saves,
    logger,
    requests,
    replies,
    replaced,
    options: options$,
    setVisible: (next) => {
      visible = next;
      if (next) {
        for (const listener of visibleListeners) {
          listener();
        }
      }
    },
    elements: viewerElements(doc),
    done,
  };
}

/** Puts record into the session area as the worker writes it, which announces it. */
async function store(viewer: Viewer, record: JobRecord): Promise<void> {
  await viewer.session.write({ [jobItemName(record.jobId)]: record });
  await settle();
}

/** The state section's text renderViewer draws for state in a fresh skeleton. */
function drawnState(
  state: ViewerState,
  elapsedMs = 0,
  existing = false,
): string {
  const elements = viewerElements();
  renderViewer(
    elements,
    { state, elapsedMs, existing, buildKind: "release" },
    "en",
    { openOptions: () => {} },
  );
  return elements.state.textContent ?? "";
}

function stateText(viewer: Viewer): string {
  return viewer.elements.state.textContent ?? "";
}

const types = (viewer: Viewer): string[] =>
  viewer.requests.map((request) => request.type);

/** The running record of this page's own run, as the worker writes it on the reservation. */
function ownRunning(acceptedAt = 10_000): JobRecord {
  const record = runningRecord(ownTab.tabId, acceptedAt);
  assert.ok(record.state === "running");
  return {
    ...record,
    runner: { tabId: ownTab.tabId, instanceId: newIds[0] ?? "" },
  };
}

/** Lets the run's Worker load and answers prepare. */
async function prepared(viewer: Viewer, reply = preparedOk): Promise<void> {
  const worker = viewer.workers.created[0];
  assert.ok(worker !== undefined, "no Worker was started");
  worker.send({ kind: "ready" });
  worker.send({ callId: 1, kind: "reply", value: reply });
  await settle();
}

describe("startViewer: links", () => {
  for (const [name, query] of [
    ["an empty query", ""],
    ["an unknown name", `${queryOf()}&x=1`],
    ["a start and a job together", `${queryOf()}&r=${startId}`],
    ["a title too long", queryOf({ title: "t".repeat(65) })],
  ] as const) {
    it(`shows an invalid link with its fixed title and reads and sends nothing for ${name}`, async () => {
      const viewer = await open({ query });
      assert.equal(stateText(viewer), drawnState({ kind: "invalid-link" }));
      assert.equal(viewer.doc.title, "Invalid link - yt2column");
      assert.deepEqual(viewer.requests, []);
      assert.deepEqual(viewer.session.calls, []);
      assert.equal(viewer.workers.created.length, 0);
      assert.equal(viewer.elements.heading.hidden, true);
      assert.equal(
        viewer.logger.errors.map((entry) => entry.label).join(),
        "yt2column viewer url",
      );
    });
  }

  it("rejects a link with a fragment", async () => {
    const viewer = await open({ query: queryOf(), hash: "#x" });
    assert.equal(stateText(viewer), drawnState({ kind: "invalid-link" }));
  });

  it("sets the page language and a development build's name in the title", async () => {
    const viewer = await open({
      query: "",
      uiLanguage: "ja-JP",
      buildKind: "dev",
    });
    assert.equal(viewer.doc.documentElement.lang, "ja");
    assert.equal(
      viewer.doc.title,
      `${catalogs.ja.viewerTitle.invalidLink} - ${catalogs.ja.viewerTitle.name.dev}`,
    );
  });
});

describe("startViewer: a job URL", () => {
  it("shows the record's running job with the time since the reservation, redrawn each second while visible", async () => {
    const viewer = await open({
      query: queryOf(),
      items: [[jobItemName(jobId), runningRecord(99, 4_000)]],
    });
    const running: ViewerState = {
      kind: "job",
      job: { kind: "running", steps: runningRecord().steps },
    };
    assert.equal(stateText(viewer), drawnState(running, 6_000));
    viewer.clock.advance(elapsedRedrawMs);
    assert.equal(stateText(viewer), drawnState(running, 7_000));
    viewer.setVisible(false);
    viewer.clock.advance(elapsedRedrawMs);
    assert.equal(viewer.clock.pending(), 0);
    viewer.setVisible(true);
    assert.equal(viewer.clock.pending(), 1);
  });

  it("titles the page and heads it with the record's values", async () => {
    const viewer = await open({
      query: queryOf(),
      items: [[jobItemName(jobId), runningRecord(99)]],
    });
    assert.equal(viewer.doc.title, "A video title - yt2column");
    assert.equal(viewer.elements.heading.textContent, "A video title");
    assert.equal(viewer.elements.channel.textContent, "A channel");
    assert.equal(viewer.elements.videoLink.getAttribute("href"), watchUrl);
  });

  it("uses the URL's title for the heading and the page title when the record collected no video title", async () => {
    const record = runningRecord(99);
    const viewer = await open({
      query: queryOf(),
      items: [
        [
          jobItemName(jobId),
          { ...record, heading: { videoTitle: "", channelName: "" } },
        ],
      ],
    });
    assert.equal(viewer.elements.heading.textContent, "A URL title");
    assert.equal(viewer.elements.channel.hidden, true);
    assert.equal(viewer.doc.title, "A URL title - yt2column");
  });

  it("follows the record's change events to the done article, with saving and copying", async () => {
    const viewer = await open({
      query: queryOf(),
      items: [[jobItemName(jobId), runningRecord(99)]],
    });
    const done = doneRecord() as Extract<JobRecord, { state: "done" }>;
    await store(viewer, {
      ...done,
      article: { ...article, videoTitle: neutralTitle },
    });
    assert.equal(viewer.elements.article.hidden, false);
    assert.equal(viewer.elements.heading.textContent, neutralTitle);
    assert.equal(viewer.doc.title, `${neutralTitle} - yt2column`);
    assert.equal(viewer.clock.pending(), 0);
    viewer.elements.save.click();
    viewer.elements.copy.click();
    await settle();
    assert.deepEqual(viewer.saves.saved, [
      {
        fileName: articleFileName("A video title", "dQw4w9WgXcQ"),
        text: article.file,
      },
    ]);
    assert.deepEqual(viewer.clipboard.written, [article.markdown]);
    assert.equal(
      viewer.elements.actionStatus.textContent,
      catalogs.en.viewer.copied,
    );
  });

  it("shows a lost article with the URL's title and link when no record holds the job", async () => {
    const viewer = await open({ query: queryOf() });
    assert.equal(
      stateText(viewer),
      drawnState({ kind: "job", job: { kind: "lost" } }),
    );
    assert.equal(viewer.doc.title, "A URL title - yt2column");
    assert.equal(viewer.elements.heading.textContent, "A URL title");
    assert.equal(viewer.elements.videoLink.getAttribute("href"), watchUrl);
    assert.equal(viewer.elements.actions.hidden, true);
  });

  it("shows a record removed later as lost", async () => {
    const viewer = await open({
      query: queryOf(),
      items: [[jobItemName(jobId), doneRecord()]],
    });
    await viewer.session.remove([jobItemName(jobId)]);
    await settle();
    assert.equal(
      stateText(viewer),
      drawnState({ kind: "job", job: { kind: "lost" } }),
    );
    assert.equal(viewer.elements.article.hidden, true);
  });

  it("follows only its own job's record", async () => {
    const viewer = await open({
      query: queryOf(),
      items: [[jobItemName(jobId), runningRecord(99)]],
    });
    const before = stateText(viewer);
    await store(viewer, { ...doneRecord(), jobId: otherJob });
    assert.equal(stateText(viewer), before);
  });

  it("starts no run: it takes no input and starts no Worker", async () => {
    for (const record of [undefined, failedRecord("llm_failed")]) {
      const viewer = await open({
        query: queryOf(),
        items: record === undefined ? [] : [[jobItemName(jobId), record]],
      });
      assert.deepEqual(types(viewer), []);
      assert.equal(viewer.workers.created.length, 0);
      assert.equal(viewer.local.calls.length, 0);
    }
  });

  it("tells the worker that the running page was reloaded when its record names this tab", async () => {
    const viewer = await open({
      query: queryOf(),
      items: [[jobItemName(jobId), runningRecord(ownTab.tabId)]],
    });
    assert.deepEqual(viewer.requests, [
      { type: "runnerReloaded", jobId, instanceId: newIds[0] },
    ]);
  });

  it("tells the worker nothing when the running page is in another tab", async () => {
    const viewer = await open({
      query: queryOf(),
      items: [[jobItemName(jobId), runningRecord(99)]],
    });
    assert.deepEqual(viewer.requests, []);
  });

  it("shows an existing article without the elapsed time, with its notice", async () => {
    const viewer = await open({
      query: queryOf({ target: { kind: "job", jobId, existing: true } }),
      items: [[jobItemName(jobId), runningRecord(99)]],
    });
    assert.equal(viewer.elements.existing.hidden, false);
    assert.equal(viewer.elements.state.querySelector(".job-elapsed"), null);
    assert.equal(viewer.clock.pending(), 0);
  });

  for (const [name, record] of [
    ["a missing API key", failedRecord("api_key_missing")],
    ["a rejected key", failedRecord("api_key_rejected")],
  ] as const) {
    it(`offers the setup operation for ${name}, which opens the options page`, async () => {
      const viewer = await open({
        query: queryOf(),
        items: [[jobItemName(jobId), record]],
      });
      const button = viewer.elements.state.querySelector("button");
      assert.ok(button !== null);
      button.click();
      await settle();
      assert.equal(viewer.options.opened, 1);
    });
  }

  it("answers present to a query for its job, and nothing to a query for another", async () => {
    const viewer = await open({
      query: queryOf(),
      items: [[jobItemName(jobId), runningRecord(99)]],
    });
    const opened = viewer.channels.open();
    assert.ok(opened.ok);
    const received: unknown[] = [];
    opened.value.listen((data) => received.push(data));
    opened.value.post({ type: "find", queryId: queryA, jobId: otherJob });
    opened.value.post({ type: "find", queryId: queryB, jobId });
    await settle();
    assert.deepEqual(received.map(parseJobChannelMessage), [
      {
        type: "present",
        queryId: queryB,
        tabId: ownTab.tabId,
        windowId: ownTab.windowId,
      },
    ]);
  });

  it("answers nothing when it does not know its own tab", async () => {
    const viewer = await open({
      query: queryOf(),
      tab: undefined,
      items: [[jobItemName(jobId), runningRecord(99)]],
    });
    const opened = viewer.channels.open();
    assert.ok(opened.ok);
    const received: unknown[] = [];
    opened.value.listen((data) => received.push(data));
    opened.value.post({ type: "find", queryId: queryB, jobId });
    await settle();
    assert.deepEqual(received, []);
  });
});

describe("startViewer: a start URL", () => {
  it("shows the start context's values while it prepares, and runs the job it was handed", async () => {
    const viewer = await open({
      query: startQuery,
      items: [[startItemName(startId), startContext(ownTab.tabId, 9_000)]],
    });
    assert.equal(stateText(viewer), drawnState({ kind: "preparing" }));
    assert.equal(viewer.doc.title, "A video title - yt2column");
    assert.equal(viewer.elements.heading.textContent, "A video title");
    assert.deepEqual(types(viewer), ["claimInput"]);
    await prepared(viewer);
    assert.deepEqual(types(viewer), ["claimInput", "reserve"]);
    // The reservation turns the URL into the job's, keeping u and t.
    assert.deepEqual(
      viewer.replaced.map((query) => parseJobViewerUrl(`?${query}`, "")),
      [
        {
          ok: true,
          params: {
            target: { kind: "job", jobId },
            videoUrl: watchUrl,
            title: "A URL title",
          },
        },
      ],
    );
    // The worker writes the record; the viewer follows it.
    await store(viewer, ownRunning());
    assert.equal(
      stateText(viewer),
      drawnState(
        { kind: "job", job: { kind: "running", steps: runningRecord().steps } },
        0,
      ),
    );
  });

  it("shows the done article its record holds at the end of the run", async () => {
    const viewer = await open({
      query: startQuery,
      items: [[startItemName(startId), startContext(ownTab.tabId)]],
    });
    await prepared(viewer);
    await store(viewer, ownRunning());
    const worker = viewer.workers.created[0];
    assert.ok(worker !== undefined);
    worker.send({
      callId: 2,
      kind: "reply",
      value: JSON.stringify({
        status: "done",
        steps: doneRecord().steps,
        article,
      }),
    });
    await settle();
    await store(viewer, doneRecord());
    await viewer.done;
    assert.ok(types(viewer).includes("result"));
    assert.equal(viewer.elements.article.hidden, false);
    assert.equal(viewer.elements.save.disabled, false);
  });

  it("keeps showing the article it holds when the worker could not store it, whatever the record says", async () => {
    const viewer = await open({
      query: startQuery,
      items: [[startItemName(startId), startContext(ownTab.tabId)]],
      replies: { result: () => ok({ status: "not_stored" }) },
    });
    await prepared(viewer);
    await store(viewer, ownRunning());
    const worker = viewer.workers.created[0];
    assert.ok(worker !== undefined);
    worker.send({
      callId: 2,
      kind: "reply",
      value: JSON.stringify({
        status: "done",
        steps: doneRecord().steps,
        article,
      }),
    });
    await viewer.done;
    await settle();
    const unstored: ViewerState = {
      kind: "job",
      job: { kind: "unstored", steps: doneRecord().steps, article },
    };
    assert.equal(stateText(viewer), drawnState(unstored));
    // The worker's mark of the failed store does not replace the article.
    await store(viewer, {
      ...failedRecord("internal"),
      internalDetail: "store_failed",
    } as JobRecord);
    assert.equal(stateText(viewer), drawnState(unstored));
    assert.equal(viewer.elements.save.disabled, false);
  });

  for (const [status, existing] of [
    ["existing", false],
    ["same_video", true],
  ] as const) {
    it(`shows the ${status} job it was handed${existing ? ", marked as an existing article," : ""} when no other viewer shows it`, async () => {
      const viewer = await open({
        query: startQuery,
        items: [
          [startItemName(startId), startContext(ownTab.tabId)],
          [jobItemName(otherJob), { ...doneRecord(), jobId: otherJob }],
        ],
        replies: { reserve: () => ok({ status, jobId: otherJob }) },
      });
      await prepared(viewer);
      viewer.clock.advance(1_000);
      await viewer.done;
      await settle();
      const [query] = viewer.replaced;
      assert.ok(query !== undefined);
      assert.deepEqual(parseJobViewerUrl(`?${query}`, ""), {
        ok: true,
        params: {
          target: existing
            ? { kind: "job", jobId: otherJob, existing: true }
            : { kind: "job", jobId: otherJob },
          videoUrl: watchUrl,
          title: "A URL title",
        },
      });
      assert.equal(viewer.elements.article.hidden, false);
      assert.equal(viewer.elements.existing.hidden, !existing);
      assert.equal(viewer.workers.created[0]?.terminations, 1);
      assert.equal(
        viewer.tabs.calls.some((call) => call.op === "remove"),
        false,
      );
    });
  }

  it("closes its own tab when another viewer already shows the job it was handed", async () => {
    const viewer = await open({
      query: startQuery,
      items: [[startItemName(startId), startContext(ownTab.tabId)]],
      replies: { reserve: () => ok({ status: "existing", jobId: otherJob }) },
    });
    const peer = viewer.channels.open();
    assert.ok(peer.ok);
    peer.value.listen((data) => {
      const message = parseJobChannelMessage(data);
      if (message?.type === "find" && message.jobId === otherJob) {
        peer.value.post({
          type: "present",
          queryId: message.queryId,
          tabId: 40,
          windowId: 3,
        });
      }
    });
    await prepared(viewer);
    viewer.clock.advance(1_000);
    await settle();
    assert.deepEqual(
      viewer.tabs.calls.filter((call) => call.op === "remove"),
      [{ op: "remove", tabId: ownTab.tabId }],
    );
  });

  const ends: {
    name: string;
    replies?: Replies;
    prepare?: string;
    state: ViewerState;
  }[] = [
    {
      name: "the input was already taken",
      replies: { claimInput: () => ok({ status: "gone" }) },
      state: { kind: "start-failed", failure: { kind: "input-unavailable" } },
    },
    {
      name: "the input is not accepted",
      prepare: JSON.stringify({ status: "rejected", reason: "invalid_body" }),
      state: {
        kind: "start-failed",
        failure: { kind: "rejected", reason: "invalid_body" },
      },
    },
    {
      name: "the reservation is refused",
      replies: {
        reserve: () => ok({ status: "rejected", reason: "too_many_requests" }),
      },
      state: {
        kind: "start-failed",
        failure: { kind: "rejected", reason: "too_many_requests" },
      },
    },
    {
      name: "the reservation's reply cannot be read",
      replies: { reserve: () => ok({ status: "new_run" }) },
      state: { kind: "start-failed", failure: { kind: "unknown" } },
    },
  ];
  for (const { name, replies, prepare, state } of ends) {
    it(`shows the start's end, keeping the start context's title, when ${name}`, async () => {
      const viewer = await open({
        query: startQuery,
        items: [[startItemName(startId), startContext(ownTab.tabId)]],
        ...(replies === undefined ? {} : { replies }),
      });
      if (viewer.workers.created.length > 0) {
        await prepared(viewer, prepare);
      }
      await viewer.done;
      assert.equal(stateText(viewer), drawnState(state));
      assert.equal(viewer.doc.title, "A video title - yt2column");
      assert.deepEqual(viewer.replaced, []);
    });
  }

  it("uses the URL's title once the start context is gone, and starts no Worker without the input", async () => {
    const viewer = await open({
      query: startQuery,
      replies: { claimInput: () => ok({ status: "gone" }) },
    });
    await viewer.done;
    assert.equal(viewer.doc.title, "A URL title - yt2column");
    assert.equal(viewer.workers.created.length, 0);
    assert.equal(
      stateText(viewer),
      drawnState({
        kind: "start-failed",
        failure: { kind: "input-unavailable" },
      }),
    );
  });

  it("logs the run's outcome kind and none of the input, the key, the job or the URL's values", async () => {
    const viewer = await open({
      query: startQuery,
      items: [[startItemName(startId), startContext(ownTab.tabId)]],
      replies: {
        reserve: () => ok({ status: "rejected", reason: "internal" }),
      },
    });
    await prepared(viewer);
    await viewer.done;
    assert.deepEqual(viewer.logger.infos, [
      { label: "yt2column viewer run", data: { outcome: "not_run" } },
    ]);
    const serialized = JSON.stringify([
      viewer.logger.infos,
      viewer.logger.errors,
    ]);
    for (const value of [
      secret,
      input.transcript,
      input.title,
      watchUrl,
      startId,
      digest,
    ]) {
      assert.equal(serialized.includes(value), false, value);
    }
  });

  it("counts the elapsed time from the reservation once the record is read", async () => {
    const viewer = await open({
      query: startQuery,
      items: [[startItemName(startId), startContext(ownTab.tabId, 9_000)]],
    });
    await prepared(viewer);
    await store(viewer, ownRunning(9_500));
    assert.equal(
      viewer.elements.state.querySelector(".job-elapsed dd")?.textContent,
      formatDuration(500, "en"),
    );
  });
});
