import assert from "node:assert/strict";
import { describe, it } from "node:test";

import type {
  BrowserCall,
  TabPlace,
  ViewerChannel,
  WriteClipboard,
} from "../src/browser/chromeDeps.ts";
import type { ArticleKey } from "../src/core/articleKey.ts";
import { parseViewerChannelMessage } from "../src/core/extensionMessages.ts";
import type { ViewerChannelMessage } from "../src/core/extensionMessages.ts";
import { articleFileName } from "../src/core/fileName.ts";
import { catalogs } from "../src/core/i18n/catalog.ts";
import { formatDuration } from "../src/core/i18n/locale.ts";
import type { ViewerState } from "../src/core/jobView.ts";
import type { NativeReply } from "../src/core/native.ts";
import {
  elapsedRedrawMs,
  pollIntervalMs,
  startResultDeadlineMs,
} from "../src/core/poll.ts";
import { publishRejectStatus } from "../src/core/response.ts";
import type { PublishRejectReason } from "../src/core/response.ts";
import {
  publishResultMessage,
  statusNoticeMessage,
} from "../src/core/sendMessages.ts";
import { parseViewerUrl, viewerQuery } from "../src/core/viewerUrl.ts";
import type { ViewerParams } from "../src/core/viewerUrl.ts";
import { sendTimeoutMs } from "../src/send.ts";
import { renderViewer } from "../src/ui/renderViewer.ts";
import { startViewer } from "../src/viewerApp.ts";
import {
  fakeChannels,
  fakeClipboard,
  fakeSaves,
  fakeTabs,
  pendingWorker,
  type FakeChannels,
  type FakeClipboard,
  type FakeSaves,
  type PendingWorker,
} from "./helpers/chrome.ts";
import { fakeClock, settle, type FakeClock } from "./helpers/clock.ts";
import { viewerDocument, viewerElements } from "./helpers/dom.ts";
import { recordingLogger, type RecordingLogger } from "./helpers/logger.ts";
import { doneSteps, requestId, runningSteps } from "./helpers/job.ts";
import {
  errorReply,
  pendingHost,
  responseReply,
  type PendingHost,
} from "./helpers/native.ts";

const sentAt = 1_700_000_000_000;
/** When the page loads in most cases: a second after the send. */
const loadedAt = sentAt + 1_000;
const key = "key-1" as ArticleKey;
const watchUrl = "https://www.youtube.com/watch?v=abc";
const ownTab: TabPlace = { tabId: 5, windowId: 2 };

const seed = {
  heading: "Heading",
  channelName: "Channel",
  title: "Video title",
  url: watchUrl,
  sentAt,
};

const article = {
  markdown: "# Title\n\nBody\r\nmore",
  mentionsNeutralized: false,
  videoTitle: "Video title",
  channelName: "Channel",
  file: "---\ntitle: Title\n---\n\nBody\r\nmore\n",
  cost: { state: "known", usd: "0.1234" },
  durationMs: 65_000,
};

type Json = Record<string, unknown>;

const runningJob: Json = { state: "running", steps: runningSteps };
const failedJob: Json = {
  state: "failed",
  steps: doneSteps,
  failure: "llm_failed",
};
function doneJob(publish: Json): Json {
  return { state: "done", steps: doneSteps, article, publish };
}

/** The host's reply to a status query that found job. */
function statusReply(job: Json, webhookConfigured = true): NativeReply {
  return responseReply(
    200,
    JSON.stringify({ status: "ok", requestId, webhookConfigured, job }),
  );
}

const publishAccepted = responseReply(
  202,
  JSON.stringify({ status: "accepted", requestId }),
);

/** The host's reply rejecting a status query: polling stops. */
const statusRejected = responseReply(
  400,
  JSON.stringify({ status: "rejected", reason: "invalid_key", requestId }),
);

/** The running job of runningJob as the viewer shows it, with no notice. */
const runningState: ViewerState = {
  kind: "job",
  job: { kind: "running", steps: runningSteps },
  notice: { kind: "none" },
};

function publishRejected(reason: PublishRejectReason): NativeReply {
  return responseReply(
    publishRejectStatus[reason],
    JSON.stringify({ status: "rejected", reason, requestId }),
  );
}

interface Viewer {
  readonly doc: Document;
  readonly clock: FakeClock;
  readonly host: PendingHost;
  readonly worker: PendingWorker;
  readonly channels: FakeChannels;
  readonly clipboard: FakeClipboard;
  readonly saves: FakeSaves;
  readonly logger: RecordingLogger;
  /** Each query the page wrote into its URL, in order. */
  readonly replaced: string[];
  /** Makes the page hidden or visible; becoming visible fires its listeners. */
  setVisible(visible: boolean): void;
  /** The viewer's state section and status regions, found in doc. */
  readonly elements: ReturnType<typeof viewerElements>;
  /** An endpoint of the viewers' channel the test posts on, and what it received. */
  readonly peer: ViewerChannel;
  readonly received: ViewerChannelMessage[];
}

interface Options {
  readonly query?: string;
  readonly hash?: string;
  readonly now?: number;
  readonly uiLanguage?: string;
  /** The page's own tab; undefined outside a tab. */
  readonly tab?: TabPlace | undefined;
  /** Timers that never fire, as a throttled page's timer may run late. */
  readonly lateTimers?: boolean;
  /** Replaces the fake clipboard's write. */
  readonly writeClipboard?: WriteClipboard;
}

function queryOf(params: Partial<ViewerParams> = {}): string {
  return viewerQuery({ ...seed, ...params });
}

/** Starts a viewer on a fresh viewer.html and lets it run until it waits. */
async function open(options: Options = {}): Promise<Viewer> {
  const doc = viewerDocument();
  const clock = fakeClock(options.now ?? loadedAt);
  const tabs = fakeTabs();
  tabs.current = "tab" in options ? options.tab : ownTab;
  const channels = fakeChannels();
  const clipboard = fakeClipboard();
  const saves = fakeSaves();
  const logger = recordingLogger();
  const replaced: string[] = [];
  let visible = true;
  const visibleListeners: (() => void)[] = [];

  const host = pendingHost();
  const worker = pendingWorker();

  const opened = channels.open();
  assert.ok(opened.ok);
  const peer = opened.value;
  const received: ViewerChannelMessage[] = [];
  peer.listen((data) => {
    const message = parseViewerChannelMessage(data);
    assert.ok(message, "the viewer posted a message that fails the check");
    received.push(message);
  });

  void startViewer(doc, {
    uiLanguage: () => options.uiLanguage ?? "en-US",
    page: {
      search: `?${options.query ?? queryOf()}`,
      hash: options.hash ?? "",
      replaceQuery: (written) => void replaced.push(written),
      isVisible: () => visible,
      onVisible: (listener) => void visibleListeners.push(listener),
    },
    tabs,
    sendToWorker: worker.send,
    openChannel: channels.open,
    send: {
      sendNative: host.sendNative,
      schedule:
        options.lateTimers === true ? () => () => undefined : clock.schedule,
    },
    now: clock.now,
    writeClipboard: options.writeClipboard ?? clipboard.write,
    saveFile: saves.save,
    log: logger.log,
  });
  await settle();
  return {
    doc,
    clock,
    host,
    worker,
    channels,
    clipboard,
    saves,
    logger,
    replaced,
    setVisible: (next) => {
      visible = next;
      if (next) {
        for (const listener of visibleListeners) {
          listener();
        }
      }
    },
    elements: viewerElements(doc),
    peer,
    received,
  };
}

/** The state section's text drawn for state on a fresh skeleton, to compare with the page. */
function drawnState(
  state: ViewerState,
  elapsedMs = 0,
  existing = false,
): string {
  const elements = viewerElements();
  renderViewer(
    elements,
    { state, elapsedMs, publishing: false, existing },
    "en",
  );
  return elements.state.textContent ?? "";
}

/** The page's state section text. */
function stateText(viewer: Viewer): string {
  return viewer.elements.state.textContent ?? "";
}

/** A viewer opened on a key URL that has shown job. */
async function showing(job: Json, webhookConfigured = true): Promise<Viewer> {
  const viewer = await open({ query: queryOf({ key }) });
  await viewer.host.answer(statusReply(job, webhookConfigured));
  return viewer;
}

/** Posts message on the test's endpoint and lets the page answer. */
async function post(
  viewer: Viewer,
  message: ViewerChannelMessage,
): Promise<void> {
  assert.ok(viewer.peer.post(message));
  await settle();
}

const queryId = "0".repeat(31) + "1";

/** A take-over of the test's tab for queryId, from a send at newSentAt answered with answer. */
function takeOver(
  newSentAt = sentAt + 50_000,
  answer: "accepted" | "same-video" = "accepted",
): ViewerChannelMessage {
  return {
    type: "take-over",
    queryId,
    tabId: 5,
    sentAt: newSentAt,
    answer,
  };
}

describe("startViewer: display and URL", () => {
  for (const [name, options] of [
    ["an unknown name", { query: `${queryOf()}&x=1` }],
    ["a fragment", { hash: "#x" }],
    [
      "a send time after the load",
      { query: queryOf({ sentAt: loadedAt + 1 }) },
    ],
    ["an empty key", { query: `${queryOf()}&k=` }],
    ["an e without a key", { query: `${queryOf()}&e=1` }],
  ] as const) {
    it(`shows an invalid link and sends nothing for ${name}`, async () => {
      const viewer = await open(options);
      assert.equal(stateText(viewer), drawnState({ kind: "invalid-link" }));
      assert.equal(viewer.elements.heading.hidden, true);
      assert.equal(viewer.elements.article.hidden, true);
      assert.deepEqual(viewer.host.sent, []);
      assert.deepEqual(viewer.worker.sent, []);
      assert.equal(viewer.doc.title, "yt2column");
      assert.deepEqual(
        viewer.logger.errors.map(({ label }) => label),
        ["yt2column viewer url"],
      );
      viewer.clock.advance(startResultDeadlineMs);
      await settle();
      assert.deepEqual(viewer.host.sent, []);
    });
  }

  it("polls a URL that holds a key at once and shows its status", async () => {
    const viewer = await open({ query: queryOf({ key }) });
    assert.equal(
      stateText(viewer),
      drawnState({ kind: "loading", notice: { kind: "none" } }),
    );
    assert.deepEqual(viewer.host.sent, [{ v: 1, op: "status", key }]);
    assert.deepEqual(viewer.worker.sent, []);
    await viewer.host.answer(statusReply(runningJob));
    assert.ok(viewer.elements.state.querySelector(".step-list") !== null);
  });

  for (const [kind, existing] of [
    ["accepted", false],
    ["same-video", true],
  ] as const) {
    it(`waits for the start result, adds the key${existing ? " and e=1" : ""} to the URL on ${kind} and polls it`, async () => {
      const viewer = await open();
      assert.equal(stateText(viewer), drawnState({ kind: "sending" }));
      assert.deepEqual(viewer.worker.sent, [{ type: "await-start", sentAt }]);
      await viewer.worker.answer({
        ok: true,
        value: { kind: "result", outcome: { kind, key, requestId } },
      });
      assert.equal(viewer.replaced.length, 1);
      const written = parseViewerUrl(`?${viewer.replaced[0]}`, "", loadedAt);
      assert.ok(written.ok);
      assert.deepEqual(
        written.params,
        existing ? { ...seed, key, existing: true } : { ...seed, key },
      );
      assert.deepEqual(viewer.host.sent, [{ v: 1, op: "status", key }]);
      // Only an existing article's page shows the notice.
      assert.equal(viewer.elements.existing.hidden, !existing);
    });
  }

  for (const [name, finish] of [
    [
      "the worker holds no send",
      (viewer: Viewer) =>
        viewer.worker.answer({ ok: true, value: { kind: "none" } }),
    ],
    [
      "the message fails",
      (viewer: Viewer) =>
        viewer.worker.answer({ ok: false, error: "no receiving end" }),
    ],
    [
      "the reply fails its check",
      (viewer: Viewer) =>
        viewer.worker.answer({ ok: true, value: { kind: "result" } }),
    ],
    [
      "the deadline passes first, and the late result",
      async (viewer: Viewer) => {
        viewer.clock.advance(startResultDeadlineMs - (loadedAt - sentAt));
        await viewer.worker.answer({
          ok: true,
          value: {
            kind: "result",
            outcome: { kind: "accepted", key, requestId },
          },
        });
      },
    ],
  ] as const) {
    it(`shows no result and sends no status query when ${name}`, async () => {
      const viewer = await open();
      await finish(viewer);
      assert.equal(
        stateText(viewer),
        drawnState({ kind: "start-failed", failure: { kind: "no-result" } }),
      );
      assert.deepEqual(viewer.host.sent, []);
      assert.deepEqual(viewer.replaced, []);
    });
  }

  it("shows no result for a result that arrives after the deadline before its late timer", async () => {
    const viewer = await open({ lateTimers: true });
    viewer.clock.advance(startResultDeadlineMs - (loadedAt - sentAt));
    await viewer.worker.answer({
      ok: true,
      value: { kind: "result", outcome: { kind: "accepted", key, requestId } },
    });
    assert.equal(
      stateText(viewer),
      drawnState({ kind: "start-failed", failure: { kind: "no-result" } }),
    );
    assert.deepEqual(viewer.host.sent, []);
    assert.deepEqual(viewer.replaced, []);
  });

  it("does not wait at all when the page loads after the deadline", async () => {
    const viewer = await open({ now: sentAt + startResultDeadlineMs });
    assert.deepEqual(viewer.worker.sent, []);
    assert.equal(
      stateText(viewer),
      drawnState({ kind: "start-failed", failure: { kind: "no-result" } }),
    );
  });

  it("waits only until the deadline counted from the URL's send time", async () => {
    const viewer = await open({ now: sentAt + 30_000 });
    viewer.clock.advance(startResultDeadlineMs - 30_000 - 1);
    assert.equal(stateText(viewer), drawnState({ kind: "sending" }));
    viewer.clock.advance(1);
    assert.equal(
      stateText(viewer),
      drawnState({ kind: "start-failed", failure: { kind: "no-result" } }),
    );
  });

  for (const outcome of [
    { kind: "rejected", reason: "invalid_transcript", requestId },
    { kind: "not-sent", cause: "server-unavailable" },
    { kind: "no-response" },
  ] as const) {
    it(`shows a start that was ${outcome.kind} with its reason and polls nothing`, async () => {
      const viewer = await open();
      await viewer.worker.answer({
        ok: true,
        value: { kind: "result", outcome },
      });
      assert.equal(
        stateText(viewer),
        drawnState({ kind: "start-failed", failure: outcome }),
      );
      viewer.clock.advance(startResultDeadlineMs);
      await settle();
      assert.deepEqual(viewer.host.sent, []);
    });
  }

  it("counts the elapsed time from the URL's send time after a reload, and redraws it each second while visible", async () => {
    const reloadedAt = sentAt + 65_000;
    const viewer = await open({ query: queryOf({ key }), now: reloadedAt });
    await viewer.host.answer(statusReply(runningJob));
    assert.equal(stateText(viewer), drawnState(runningState, 65_000));
    viewer.clock.advance(elapsedRedrawMs);
    assert.equal(stateText(viewer), drawnState(runningState, 66_000));

    viewer.setVisible(false);
    viewer.clock.advance(elapsedRedrawMs);
    viewer.clock.advance(elapsedRedrawMs);
    assert.equal(stateText(viewer), drawnState(runningState, 67_000));
    // Becoming visible draws the time at once, before the next second.
    viewer.setVisible(true);
    assert.equal(stateText(viewer), drawnState(runningState, 68_000));
  });

  it("shows an existing article after a reload of an e=1 URL: no elapsed time or its redraw while running, the time taken when done", async () => {
    const viewer = await open({
      query: queryOf({ key, existing: true }),
      now: sentAt + 65_000,
    });
    await viewer.host.answer(statusReply(runningJob));
    assert.equal(stateText(viewer), drawnState(runningState, 0, true));
    assert.equal(viewer.elements.existing.hidden, false);
    assert.deepEqual(viewer.replaced, []);
    // Only the next poll is armed: no per-second redraw beside it.
    assert.equal(viewer.clock.pending(), 1);

    viewer.clock.advance(pollIntervalMs);
    await viewer.host.answer(statusReply(doneJob({ state: "not_posted" })));
    assert.equal(
      viewer.elements.state.querySelector(".job-result dd")?.textContent,
      formatDuration(article.durationMs, "en"),
    );
    assert.equal(viewer.elements.existing.hidden, false);
  });

  for (const [name, reply, notice] of [
    [
      "a retryable failure, beside the loading mark",
      errorReply("server_unavailable"),
      {
        kind: "retrying",
        outcome: { kind: "not-sent", cause: "server-unavailable" },
      },
    ],
    [
      "a rejection, alone since polling stopped",
      statusRejected,
      {
        kind: "stopped",
        outcome: { kind: "rejected", reason: "invalid_key", requestId },
      },
    ],
  ] as const) {
    it(`shows a failed first status: ${name}`, async () => {
      const viewer = await open({ query: queryOf({ key }) });
      await viewer.host.answer(reply);
      assert.equal(stateText(viewer), drawnState({ kind: "loading", notice }));
      // A failed exchange is logged as an error; a server's rejection is its answer.
      const level =
        notice.kind === "retrying" ? viewer.logger.errors : viewer.logger.infos;
      assert.equal(level.at(-1)?.label, "yt2column status result");
      assert.equal(
        (level.at(-1)?.data as { decision: string }).decision,
        notice.kind === "retrying" ? "continue" : "stop",
      );
    });
  }

  it("titles the page with the URL's heading and sets its language", async () => {
    const viewer = await open({ uiLanguage: "ja" });
    assert.equal(viewer.doc.title, "Heading - yt2column");
    assert.equal(viewer.doc.documentElement.lang, "ja");
    assert.equal(viewer.elements.heading.textContent, "Heading");
    assert.equal(viewer.elements.channel.textContent, "Channel");
  });

  it("keeps the state shown, without the loading mark, during later status queries", async () => {
    const viewer = await showing(runningJob);
    viewer.clock.advance(pollIntervalMs);
    assert.equal(viewer.host.count("status"), 2);
    assert.equal(viewer.elements.state.querySelector(".loading"), null);
    assert.ok(viewer.elements.state.querySelector(".step-list") !== null);
  });

  // renderViewer writes the summary only when it changes; this checks that
  // the viewer's redraw on every poll and every second keeps that property.
  it("writes the state summary only when the state changes", async () => {
    const viewer = await showing(runningJob);
    const summary = viewer.elements.stateStatus.firstChild;
    assert.ok(summary !== null);
    viewer.clock.advance(pollIntervalMs);
    await viewer.host.answer(statusReply(runningJob));
    viewer.clock.advance(elapsedRedrawMs);
    assert.equal(viewer.elements.stateStatus.firstChild, summary);
    viewer.clock.advance(pollIntervalMs);
    await viewer.host.answer(statusReply(doneJob({ state: "not_posted" })));
    assert.notEqual(viewer.elements.stateStatus.firstChild, summary);
  });

  it("fetches once at once when the page becomes visible", async () => {
    const viewer = await showing(runningJob);
    viewer.setVisible(false);
    viewer.setVisible(true);
    assert.equal(viewer.host.count("status"), 2);
    await viewer.host.answer(statusReply(runningJob));
    const last = viewer.logger.infos.at(-1);
    assert.equal(last?.label, "yt2column status result");
    assert.equal((last?.data as { visible: boolean }).visible, true);
  });
});

describe("startViewer: operations", () => {
  for (const [name, reply] of [
    ["not sent", errorReply("server_unavailable")],
    ["host failed", { kind: "failed", lastError: "host exited" }],
    ["rejected", statusRejected],
  ] as const) {
    it(`sends neither a start nor a publish on a failed status (${name})`, async () => {
      // A take-over of a done job restarts its stopped poller, so the status
      // fails while the not posted row can be pressed: only the viewer's own
      // restraint keeps it from publishing.
      const viewer = await showing(doneJob({ state: "not_posted" }));
      await post(viewer, { type: "find", queryId, key });
      await post(viewer, takeOver());
      await viewer.host.answer(reply);
      assert.equal(viewer.elements.publish.disabled, false);
      viewer.clock.advance(10 * pollIntervalMs);
      await settle();
      assert.equal(viewer.host.count("publish"), 0);
      assert.deepEqual(viewer.worker.sent, []);
    });
  }

  it("shows a failed status beside the job and keeps polling a retryable one", async () => {
    const viewer = await showing(runningJob);
    viewer.clock.advance(pollIntervalMs);
    await viewer.host.answer(errorReply("server_unavailable"));
    assert.ok(viewer.elements.state.querySelector(".step-list") !== null);
    assert.ok(
      stateText(viewer).includes(
        statusNoticeMessage(
          { kind: "not-sent", cause: "server-unavailable" },
          "en",
        ).text,
      ),
    );
    viewer.clock.advance(pollIntervalMs);
    assert.equal(viewer.host.count("status"), 3);
  });

  it("publishes once on a press, keeps the button off until the new epoch's status, and polls while posting", async () => {
    const viewer = await showing(doneJob({ state: "not_posted" }));
    const { publish } = viewer.elements;
    assert.equal(publish.disabled, false);
    publish.click();
    publish.click();
    await settle();
    assert.deepEqual(viewer.host.sent.at(-1), { v: 1, op: "publish", key });
    assert.equal(viewer.host.count("publish"), 1);
    assert.equal(publish.disabled, true);

    await viewer.host.answer(publishAccepted);
    assert.equal(viewer.host.count("status"), 2, "the poller restarts at once");
    assert.equal(publish.disabled, true);
    await viewer.host.answer(statusReply(doneJob({ state: "posting" })));
    assert.equal(publish.disabled, true);
    viewer.clock.advance(pollIntervalMs);
    await viewer.host.answer(statusReply(doneJob({ state: "posted" })));
    viewer.clock.advance(10 * pollIntervalMs);
    assert.equal(viewer.host.count("status"), 3);
    assert.equal(viewer.host.count("publish"), 1);
    assert.equal(publish.disabled, true);
  });

  it("keeps the button off when a status sent before the publish returns", async () => {
    const viewer = await showing(doneJob({ state: "not_posted" }));
    // A take-over of a done job restarts the stopped poller in a new epoch,
    // so a status query is in flight when the button is pressed.
    await post(viewer, { type: "find", queryId, key });
    await post(viewer, takeOver());
    assert.equal(viewer.host.count("status"), 2);
    viewer.elements.publish.click();
    await settle();
    await viewer.host.answer(publishAccepted, "publish");
    await viewer.host.answer(statusReply(doneJob({ state: "not_posted" })));
    assert.equal(viewer.host.count("status"), 3);
    assert.equal(viewer.elements.publish.disabled, true);
    await viewer.host.answer(statusReply(doneJob({ state: "posting" })));
    assert.equal(viewer.elements.publish.disabled, true);
  });

  it("polls a posting article it did not publish until it is no longer posting", async () => {
    const viewer = await showing(doneJob({ state: "posting" }));
    viewer.clock.advance(pollIntervalMs);
    await viewer.host.answer(statusReply(doneJob({ state: "posted" })));
    viewer.clock.advance(10 * pollIntervalMs);
    assert.equal(viewer.host.count("status"), 2);
    assert.equal(viewer.host.count("publish"), 0);
  });

  it("enables the button again after a take-over that goes on came between the publish and its first status", async () => {
    const viewer = await showing(doneJob({ state: "not_posted" }));
    viewer.elements.publish.click();
    await settle();
    await viewer.host.answer(publishAccepted);
    await post(viewer, { type: "find", queryId, key });
    await post(viewer, takeOver());
    await viewer.host.answer(statusReply(doneJob({ state: "posting" })));
    const stale = viewer.logger.infos.at(-1);
    assert.equal(stale?.label, "yt2column status result");
    assert.deepEqual(
      [
        (stale?.data as { earlierEpoch: boolean }).earlierEpoch,
        (stale?.data as { decision: string }).decision,
      ],
      [true, "continue"],
    );
    await viewer.host.answer(
      statusReply(doneJob({ state: "failed", reason: "webhook" })),
    );
    assert.equal(viewer.elements.publish.disabled, false);
  });

  it("changes nothing shown for a publish answer that arrives after the send restarted", async () => {
    // A done job whose polling stopped restarts on a take-over, and its
    // not posted row can still be pressed.
    const viewer = await showing(doneJob({ state: "not_posted" }));
    await post(viewer, { type: "find", queryId, key });
    await post(viewer, takeOver());
    await viewer.host.answer(statusRejected);
    viewer.elements.publish.click();
    await settle();
    assert.equal(viewer.host.count("publish"), 1);
    viewer.elements.copy.click();
    await settle();
    assert.equal(
      viewer.elements.actionStatus.textContent,
      catalogs.en.viewer.copied,
    );
    await post(viewer, { type: "find", queryId, key });
    await post(viewer, takeOver(sentAt + 60_000));
    const statuses = viewer.host.count("status");
    await viewer.host.answer(publishRejected("article_not_found"), "publish");
    assert.equal(
      stateText(viewer),
      drawnState({ kind: "loading", notice: { kind: "none" } }),
    );
    assert.equal(viewer.elements.actionStatus.textContent, "");
    await viewer.host.answer(statusReply(runningJob));
    assert.ok(viewer.elements.state.querySelector(".step-list") !== null);
    viewer.clock.advance(pollIntervalMs);
    assert.equal(viewer.host.count("status"), statuses + 1);
  });

  it("enables the button again after the new epoch's first status shows a failed post", async () => {
    const viewer = await showing(doneJob({ state: "not_posted" }));
    viewer.elements.publish.click();
    await settle();
    await viewer.host.answer(publishAccepted);
    await viewer.host.answer(
      statusReply(doneJob({ state: "failed", reason: "webhook" })),
    );
    assert.equal(viewer.elements.publish.disabled, false);
  });

  for (const [name, reply, notice, polls] of [
    [
      "rejected",
      publishRejected("article_not_ready"),
      { kind: "rejected", reason: "article_not_ready", requestId },
      false,
    ],
    [
      "not sent",
      errorReply("server_unavailable"),
      { kind: "not-sent", cause: "server-unavailable" },
      false,
    ],
    [
      "unknown",
      { kind: "failed", lastError: "host exited" },
      { kind: "host-failed" },
      true,
    ],
  ] as const) {
    it(`does not send a ${name} publish again, and shows its result`, async () => {
      const viewer = await showing(doneJob({ state: "not_posted" }));
      viewer.elements.publish.click();
      await settle();
      await viewer.host.answer(reply);
      assert.ok(
        (viewer.elements.actionStatus.textContent ?? "").includes(
          publishResultMessage(notice, "en").text,
        ),
      );
      assert.equal(viewer.host.count("status"), polls ? 2 : 1);
      if (polls) {
        await viewer.host.answer(statusReply(doneJob({ state: "not_posted" })));
      }
      viewer.clock.advance(sendTimeoutMs + 10 * pollIntervalMs);
      await settle();
      assert.equal(viewer.host.count("publish"), 1);
    });
  }

  it("shows the article as lost, still saveable, when the server no longer holds it", async () => {
    const viewer = await showing(doneJob({ state: "not_posted" }));
    viewer.elements.publish.click();
    await settle();
    await viewer.host.answer(publishRejected("article_not_found"));
    assert.equal(viewer.elements.publish.disabled, true);
    assert.ok(stateText(viewer).includes(catalogs.en.viewer.lost.text));
    // The kept article still gives the heading its values.
    assert.equal(viewer.elements.heading.textContent, article.videoTitle);
    assert.equal(viewer.elements.channel.textContent, article.channelName);
    viewer.elements.save.click();
    assert.deepEqual(
      viewer.saves.saved.map(({ text }) => text),
      [article.file],
    );
  });

  for (const [name, publish, webhookConfigured] of [
    ["posted", { state: "posted" }, true],
    ["posting", { state: "posting" }, true],
    ["unpostable", { state: "unpostable", reason: "mention" }, true],
    ["webhook not configured", { state: "not_posted" }, false],
  ] as const) {
    it(`sends no publish for a press on the ${name} row, even if the button were enabled`, async () => {
      const viewer = await showing(doneJob(publish), webhookConfigured);
      assert.equal(viewer.elements.publish.disabled, true);
      viewer.elements.publish.disabled = false;
      viewer.elements.publish.click();
      await settle();
      assert.equal(viewer.host.count("publish"), 0);
    });
  }

  it("saves the article's file text unchanged under the title's file name", async () => {
    const viewer = await showing(doneJob({ state: "not_posted" }));
    viewer.elements.save.click();
    assert.deepEqual(viewer.saves.saved, [
      { fileName: articleFileName("Video title", "abc"), text: article.file },
    ]);
    assert.equal(
      viewer.elements.actionStatus.textContent,
      catalogs.en.viewer.saved,
    );
  });

  it("copies the display Markdown and says so, without moving focus", async () => {
    const viewer = await showing(doneJob({ state: "not_posted" }));
    const focused = viewer.doc.activeElement;
    viewer.elements.copy.click();
    await settle();
    assert.deepEqual(viewer.clipboard.written, [article.markdown]);
    assert.equal(
      viewer.elements.actionStatus.textContent,
      catalogs.en.viewer.copied,
    );
    assert.equal(viewer.doc.activeElement, focused);
  });

  it("changes nothing shown for a copy that finishes after the send restarted", async () => {
    let finishCopy: ((result: BrowserCall<undefined>) => void) | undefined;
    const viewer = await open({
      query: queryOf({ key }),
      writeClipboard: () =>
        new Promise((resolve) => {
          finishCopy = resolve;
        }),
    });
    await viewer.host.answer(statusReply(doneJob({ state: "not_posted" })));
    // A done job whose polling stopped restarts on a take-over.
    await post(viewer, { type: "find", queryId, key });
    await post(viewer, takeOver());
    await viewer.host.answer(statusRejected);
    viewer.elements.copy.click();
    await settle();
    assert.ok(finishCopy, "the copy did not start");
    await post(viewer, { type: "find", queryId, key });
    await post(viewer, takeOver(sentAt + 60_000));
    finishCopy({ ok: true, value: undefined });
    await settle();
    assert.equal(viewer.elements.actionStatus.textContent, "");
  });

  it("says when the copy fails", async () => {
    const viewer = await showing(doneJob({ state: "not_posted" }));
    viewer.clipboard.fail();
    viewer.elements.copy.click();
    await settle();
    assert.equal(
      viewer.elements.actionStatus.textContent,
      catalogs.en.viewer.copyFailed,
    );
  });

  for (const locale of ["en", "ja"] as const) {
    it(`says a copy neutralized mentions, with no count, without moving focus in ${locale}`, async () => {
      const viewer = await open({
        query: queryOf({ key }),
        uiLanguage: locale,
      });
      await viewer.host.answer(
        statusReply({
          ...doneJob({ state: "not_posted" }),
          article: { ...article, mentionsNeutralized: true },
        }),
      );
      const focused = viewer.doc.activeElement;
      viewer.elements.copy.click();
      await settle();
      assert.deepEqual(viewer.clipboard.written, [article.markdown]);
      assert.equal(
        viewer.elements.actionStatus.textContent,
        catalogs[locale].viewer.copiedNeutralized,
      );
      assert.equal(viewer.doc.activeElement, focused);
    });
  }

  it("does not say a failed copy neutralized mentions", async () => {
    const viewer = await showing({
      ...doneJob({ state: "not_posted" }),
      article: { ...article, mentionsNeutralized: true },
    });
    viewer.clipboard.fail();
    viewer.elements.copy.click();
    await settle();
    assert.equal(
      viewer.elements.actionStatus.textContent,
      catalogs.en.viewer.copyFailed,
    );
  });

  it("leaves the operation result alone when the reader copies a selection", async () => {
    const viewer = await showing({
      ...doneJob({ state: "not_posted" }),
      article: { ...article, mentionsNeutralized: true },
    });
    viewer.elements.save.click();
    const status = viewer.elements.actionStatus;
    const before = { text: status.textContent, first: status.firstChild };
    for (const target of [viewer.doc, viewer.elements.article]) {
      target.dispatchEvent(
        new viewer.doc.defaultView!.Event("copy", { bubbles: true }),
      );
    }
    await settle();
    assert.equal(status.textContent, catalogs.en.viewer.saved);
    assert.equal(status.textContent, before.text);
    assert.equal(status.firstChild, before.first);
    assert.deepEqual(viewer.clipboard.written, []);
  });

  it("neither shows nor copies a done article without mentionsNeutralized", async () => {
    const older: Json = { ...article };
    delete older.mentionsNeutralized;
    const viewer = await showing({
      ...doneJob({ state: "not_posted" }),
      article: older,
    });
    assert.equal(viewer.elements.article.hidden, true);
    assert.equal(viewer.elements.article.textContent, "");
    viewer.elements.copy.click();
    await settle();
    assert.deepEqual(viewer.clipboard.written, []);
  });

  it("keeps the heading's text through the redraw each second of a running job", async () => {
    const viewer = await showing(runningJob);
    const node = viewer.elements.heading.firstChild;
    assert.ok(node !== null);
    viewer.clock.advance(3_000);
    await settle();
    assert.equal(viewer.elements.heading.firstChild, node);
  });

  it("shows the done article's video title and channel name in the heading", async () => {
    const viewer = await open({ query: queryOf({ key }) });
    assert.equal(viewer.elements.heading.textContent, "Heading");
    await viewer.host.answer(
      statusReply({
        ...doneJob({ state: "not_posted" }),
        article: {
          ...article,
          videoTitle: "Live @\u200bhere",
          channelName: "@\u200bhandle",
        },
      }),
    );
    assert.equal(viewer.elements.heading.textContent, "Live @\u200bhere");
    assert.equal(viewer.elements.channel.textContent, "@\u200bhandle");
    // The tab title keeps the URL's heading.
    assert.equal(viewer.doc.title, "Heading - yt2column");
  });
});

describe("startViewer: queries, take-over and logs", () => {
  it("answers present for its key, nothing for another key, and waiting while it has none", async () => {
    const viewer = await showing(runningJob);
    await post(viewer, { type: "find", queryId, key });
    await post(viewer, {
      type: "find",
      queryId,
      key: "other" as ArticleKey,
    });
    assert.deepEqual(viewer.received, [
      { type: "present", queryId, tabId: 5, windowId: 2, sentAt },
    ]);

    const waiting = await open();
    await post(waiting, { type: "find", queryId, key });
    assert.deepEqual(waiting.received, [
      { type: "waiting", queryId, tabId: 5, sentAt },
    ]);
  });

  it("answers nothing when it does not know its own tab", async () => {
    const viewer = await open({ query: queryOf({ key }), tab: undefined });
    await post(viewer, { type: "find", queryId, key });
    assert.deepEqual(viewer.received, []);
  });

  for (const answer of ["accepted", "same-video"] as const) {
    it(`goes on as it is when ${answer === "accepted" ? "an" : "a"} ${answer} send takes over a running job, and a status sent before cannot stop it`, async () => {
      const viewer = await showing(runningJob);
      viewer.clock.advance(pollIntervalMs);
      await post(viewer, { type: "find", queryId, key });
      await post(viewer, takeOver(undefined, answer));
      assert.deepEqual(viewer.received.at(-1), {
        type: "took-over",
        queryId,
        tabId: 5,
      });
      await viewer.host.answer(statusReply(failedJob));
      assert.equal(
        viewer.host.count("status"),
        3,
        "a new epoch's query follows at once",
      );
    });
  }

  // Design's take-over table: what the viewer shows when the take-over
  // arrives decides whether it goes on or restarts. Each setup leaves no
  // status exchange open.
  for (const [name, setup, restarts] of [
    ["a running job", () => showing(runningJob), false],
    ["a done job", () => showing(doneJob({ state: "posted" })), false],
    [
      "a first status still awaited after a retryable failure",
      async () => {
        const viewer = await open({ query: queryOf({ key }) });
        await viewer.host.answer(errorReply("server_unavailable"));
        return viewer;
      },
      false,
    ],
    ["a failed job", () => showing(failedJob), true],
    ["a lost job", () => showing({ state: "unknown" }), true],
    [
      "a first status that stopped polling",
      async () => {
        const viewer = await open({ query: queryOf({ key }) });
        await viewer.host.answer(statusRejected);
        return viewer;
      },
      true,
    ],
    [
      "a running job whose polling stopped",
      async () => {
        const viewer = await showing(runningJob);
        viewer.clock.advance(pollIntervalMs);
        await viewer.host.answer(statusRejected);
        return viewer;
      },
      true,
    ],
    [
      "a done job whose polling stopped",
      async () => {
        const viewer = await showing(doneJob({ state: "posting" }));
        viewer.clock.advance(pollIntervalMs);
        await viewer.host.answer(statusRejected);
        return viewer;
      },
      true,
    ],
  ] as const) {
    // An accepted send leaves the URL's mark as it is; a same-video send
    // adds e=1 whether the send goes on or restarts.
    for (const answer of ["accepted", "same-video"] as const) {
      const existing = answer === "same-video";
      it(`${restarts ? "restarts from the new send time" : "goes on as it is"} when ${answer === "accepted" ? "an" : "a"} ${answer} send takes over ${name}`, async () => {
        const viewer = await setup();
        const statuses = viewer.host.count("status");
        const shown = stateText(viewer);
        const timers = viewer.clock.pending();
        const redrawArmed =
          viewer.elements.state.querySelector(".job-elapsed") !== null;
        await post(viewer, { type: "find", queryId, key });
        const newSentAt = viewer.clock.now() - 3_000;
        await post(viewer, takeOver(newSentAt, answer));
        assert.deepEqual(viewer.received.at(-1), {
          type: "took-over",
          queryId,
          tabId: 5,
        });
        assert.deepEqual(viewer.logger.infos.at(-1), {
          label: "yt2column take over",
          data: { accepted: true, restarted: restarts },
        });
        assert.equal(viewer.elements.existing.hidden, !existing);
        if (!restarts && !existing) {
          assert.deepEqual(viewer.replaced, []);
          assert.equal(stateText(viewer), shown);
          return;
        }
        const written = parseViewerUrl(
          `?${viewer.replaced.at(-1)}`,
          "",
          viewer.clock.now(),
        );
        assert.ok(written.ok);
        assert.deepEqual(written.params, {
          ...seed,
          key,
          sentAt: restarts ? newSentAt : sentAt,
          ...(existing ? { existing: true } : {}),
        });
        if (!restarts) {
          // The marked page drops the elapsed time and, for a running job,
          // its per-second redraw; the poll of a running job is unchanged.
          assert.equal(
            viewer.elements.state.querySelector(".job-elapsed"),
            null,
          );
          if (redrawArmed) {
            assert.equal(viewer.clock.pending(), timers - 1);
          }
          return;
        }
        assert.equal(
          stateText(viewer),
          drawnState({ kind: "loading", notice: { kind: "none" } }),
        );
        assert.equal(viewer.host.count("status"), statuses + 1);
        // The elapsed time now counts from the take-over's send time, unless
        // the page shows an existing article.
        await viewer.host.answer(statusReply(runningJob));
        assert.equal(
          stateText(viewer),
          drawnState(runningState, existing ? 0 : 3_000, existing),
        );
      });
    }
  }

  it("keeps e=1 when an accepted send takes over a page that shows an existing article", async () => {
    const viewer = await open({ query: queryOf({ key, existing: true }) });
    await viewer.host.answer(statusReply(failedJob));
    await post(viewer, { type: "find", queryId, key });
    const newSentAt = viewer.clock.now() - 3_000;
    await post(viewer, takeOver(newSentAt));
    const written = parseViewerUrl(`?${viewer.replaced.at(-1)}`, "", newSentAt);
    assert.ok(written.ok);
    assert.deepEqual(written.params, {
      ...seed,
      key,
      existing: true,
      sentAt: newSentAt,
    });
    assert.equal(viewer.elements.existing.hidden, false);
  });

  it("ignores a take-over meant for another tab, even for a query it answered", async () => {
    const viewer = await showing(failedJob);
    await post(viewer, { type: "find", queryId, key });
    await post(viewer, { ...takeOver(), tabId: 6 } as ViewerChannelMessage);
    assert.deepEqual(
      viewer.received.map((message) => message.type),
      ["present"],
    );
    assert.deepEqual(viewer.replaced, []);
    assert.equal(viewer.host.count("status"), 1);
  });

  it("ignores a take-over for a query it did not answer present to", async () => {
    const viewer = await showing(failedJob);
    await post(viewer, takeOver());
    await post(viewer, { type: "find", queryId: "f".repeat(32), key });
    await post(viewer, takeOver());
    assert.deepEqual(
      viewer.received.map((message) => message.type),
      ["present"],
    );
    assert.deepEqual(viewer.replaced, []);
    assert.equal(viewer.host.count("status"), 1);
  });

  it("logs none of the key, the article or the URL's values", async () => {
    const marker = "MARKER-7f3a";
    const viewer = await open({
      query: viewerQuery({
        heading: `${marker} heading`,
        channelName: `${marker} channel`,
        title: `${marker} title`,
        url: watchUrl,
        sentAt,
      }),
    });
    const markedKey = `${marker}-key` as ArticleKey;
    await viewer.worker.answer({
      ok: true,
      value: {
        kind: "result",
        outcome: { kind: "accepted", key: markedKey, requestId },
      },
    });
    await viewer.host.answer(
      statusReply({
        ...doneJob({ state: "not_posted" }),
        article: { ...article, markdown: marker, file: marker },
      }),
    );
    viewer.elements.publish.click();
    await settle();
    await viewer.host.answer(publishAccepted);
    await post(viewer, { type: "find", queryId, key: markedKey });
    await post(viewer, takeOver());
    await viewer.host.answer({ kind: "failed", lastError: "host exited" });
    const logged = JSON.stringify([
      ...viewer.logger.infos,
      ...viewer.logger.errors,
    ]);
    assert.ok(logged.includes("yt2column status result"));
    assert.ok(logged.includes("yt2column publish result"));
    assert.ok(logged.includes("yt2column take over"));
    assert.ok(!logged.includes(marker), logged);
    assert.ok(!logged.includes(watchUrl), logged);
  });
});
