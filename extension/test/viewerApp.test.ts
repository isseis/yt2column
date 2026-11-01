import assert from "node:assert/strict";
import { describe, it } from "node:test";

import type {
  BrowserCall,
  TabPlace,
  ViewerChannel,
} from "../src/browser/chromeDeps.ts";
import type { ArticleKey } from "../src/core/articleKey.ts";
import { parseViewerChannelMessage } from "../src/core/extensionMessages.ts";
import type {
  AwaitStart,
  StartRequest,
  ViewerChannelMessage,
} from "../src/core/extensionMessages.ts";
import { articleFileName } from "../src/core/fileName.ts";
import type { HostMessage } from "../src/core/hostMessage.ts";
import { catalogs } from "../src/core/i18n/catalog.ts";
import type { ViewerState } from "../src/core/jobView.ts";
import type { NativeReply } from "../src/core/native.ts";
import { pollIntervalMs, startResultDeadlineMs } from "../src/core/poll.ts";
import { publishRejectStatus } from "../src/core/response.ts";
import type { PublishRejectReason, RequestId } from "../src/core/response.ts";
import {
  publishResultMessage,
  startResultMessage,
  statusNoticeMessage,
} from "../src/core/sendMessages.ts";
import { parseViewerUrl, viewerQuery } from "../src/core/viewerUrl.ts";
import type { ViewerParams } from "../src/core/viewerUrl.ts";
import { sendTimeoutMs } from "../src/send.ts";
import { renderViewer } from "../src/ui/renderViewer.ts";
import { elapsedRedrawMs, startViewer } from "../src/viewerApp.ts";
import {
  fakeChannels,
  fakeClipboard,
  fakeSaves,
  fakeTabs,
  type FakeChannels,
  type FakeClipboard,
  type FakeSaves,
} from "./helpers/chrome.ts";
import { fakeClock, settle, type FakeClock } from "./helpers/clock.ts";
import { viewerDocument, viewerElements } from "./helpers/dom.ts";
import { recordingLogger, type RecordingLogger } from "./helpers/logger.ts";
import { errorReply, responseReply } from "./helpers/native.ts";

const sentAt = 1_700_000_000_000;
/** When the page loads in most cases: a second after the send. */
const loadedAt = sentAt + 1_000;
const key = "key-1" as ArticleKey;
const requestId = "0123456789abcdef" as RequestId;
const watchUrl = "https://www.youtube.com/watch?v=abc";
const ownTab: TabPlace = { tabId: 5, windowId: 2 };

const seed = {
  heading: "Heading",
  channelName: "Channel",
  title: "Video title",
  url: watchUrl,
  sentAt,
};

const runningSteps = { draft: "done", shorten: "running", review: "pending" };
const doneSteps = { draft: "done", shorten: "skipped", review: "done" };
const article = {
  markdown: "# Title\n\nBody\r\nmore",
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

function publishRejected(reason: PublishRejectReason): NativeReply {
  return responseReply(
    publishRejectStatus[reason],
    JSON.stringify({ status: "rejected", reason, requestId }),
  );
}

/** A host whose exchanges stay open until the test answers them, in order. */
interface FakeHost {
  readonly sent: HostMessage[];
  /** How many exchanges of op were sent. */
  count(op: HostMessage["op"]): number;
  /** Answers the oldest open exchange, of op when given, and lets the page run. */
  answer(reply: NativeReply, op?: HostMessage["op"]): Promise<void>;
}

/** The worker side of the viewer's runtime messages. */
interface FakeWorker {
  readonly sent: (StartRequest | AwaitStart)[];
  /** Answers the oldest open message and lets the page run. */
  answer(reply: BrowserCall<unknown>): Promise<void>;
}

interface Viewer {
  readonly doc: Document;
  readonly clock: FakeClock;
  readonly host: FakeHost;
  readonly worker: FakeWorker;
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

  const hostSent: HostMessage[] = [];
  const hostOpen: {
    readonly op: HostMessage["op"];
    readonly resolve: (reply: NativeReply) => void;
  }[] = [];
  const host: FakeHost = {
    sent: hostSent,
    count: (op) => hostSent.filter((message) => message.op === op).length,
    answer: async (reply, op) => {
      const index = hostOpen.findIndex(
        (exchange) => op === undefined || exchange.op === op,
      );
      const [open] = index < 0 ? [] : hostOpen.splice(index, 1);
      assert.ok(open, "no host exchange is open");
      open.resolve(reply);
      await settle();
    },
  };
  const workerSent: (StartRequest | AwaitStart)[] = [];
  const workerOpen: ((reply: BrowserCall<unknown>) => void)[] = [];
  const worker: FakeWorker = {
    sent: workerSent,
    answer: async (reply) => {
      const resolve = workerOpen.shift();
      assert.ok(resolve, "no worker message is open");
      resolve(reply);
      await settle();
    },
  };

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
    sendToWorker: (message) => {
      workerSent.push(message);
      return new Promise((resolve) => workerOpen.push(resolve));
    },
    openChannel: channels.open,
    send: {
      sendNative: (message) => {
        const hostMessage = message as HostMessage;
        hostSent.push(hostMessage);
        return new Promise((resolve) =>
          hostOpen.push({ op: hostMessage.op, resolve }),
        );
      },
      schedule: clock.schedule,
    },
    now: clock.now,
    writeClipboard: clipboard.write,
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
function drawnState(state: ViewerState, elapsedMs = 0): string {
  const elements = viewerElements();
  renderViewer(elements, { state, elapsedMs, publishing: false }, "en");
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

/** A take-over of the test's tab for queryId, from a send at newSentAt. */
function takeOver(newSentAt = sentAt + 50_000): ViewerChannelMessage {
  return { type: "take-over", queryId, tabId: 5, sentAt: newSentAt };
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

  it("waits for the start result, adds the key to the URL on acceptance and polls it", async () => {
    const viewer = await open();
    assert.equal(stateText(viewer), drawnState({ kind: "sending" }));
    assert.deepEqual(viewer.worker.sent, [{ type: "await-start", sentAt }]);
    await viewer.worker.answer({
      ok: true,
      value: { kind: "result", outcome: { kind: "accepted", key, requestId } },
    });
    assert.equal(viewer.replaced.length, 1);
    const written = parseViewerUrl(`?${viewer.replaced[0]}`, "", loadedAt);
    assert.ok(written.ok);
    assert.deepEqual(written.params, { ...seed, key });
    assert.deepEqual(viewer.host.sent, [{ v: 1, op: "status", key }]);
  });

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
      assert.ok(
        stateText(viewer).includes(startResultMessage(outcome, "en").text),
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
    const running: ViewerState = {
      kind: "job",
      job: {
        kind: "running",
        steps: { draft: "done", shorten: "running", review: "pending" },
      },
      notice: { kind: "none" },
    };
    assert.equal(stateText(viewer), drawnState(running, 65_000));
    viewer.clock.advance(elapsedRedrawMs);
    assert.equal(stateText(viewer), drawnState(running, 66_000));

    viewer.setVisible(false);
    viewer.clock.advance(elapsedRedrawMs);
    viewer.clock.advance(elapsedRedrawMs);
    assert.equal(stateText(viewer), drawnState(running, 67_000));
  });

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
    [
      "rejected",
      responseReply(
        400,
        JSON.stringify({
          status: "rejected",
          reason: "invalid_key",
          requestId,
        }),
      ),
    ],
  ] as const) {
    it(`sends neither a start nor a publish on a failed status (${name})`, async () => {
      // A posting article is polled, and its button cannot be pressed.
      const viewer = await showing(doneJob({ state: "posting" }));
      viewer.clock.advance(pollIntervalMs);
      await viewer.host.answer(reply);
      viewer.clock.advance(10 * pollIntervalMs);
      await settle();
      assert.deepEqual(
        new Set(viewer.host.sent.map((message) => message.op)),
        new Set(["status"]),
      );
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

  it("goes on as it is when it takes over a running job, and a status sent before cannot stop it", async () => {
    const viewer = await showing(runningJob);
    viewer.clock.advance(pollIntervalMs);
    await post(viewer, { type: "find", queryId, key });
    await post(viewer, takeOver());
    assert.deepEqual(viewer.received.at(-1), {
      type: "took-over",
      queryId,
      tabId: 5,
    });
    assert.deepEqual(viewer.replaced, []);
    await viewer.host.answer(statusReply(failedJob));
    assert.equal(
      viewer.host.count("status"),
      3,
      "a new epoch's query follows at once",
    );
  });

  for (const [name, job] of [
    ["failed", failedJob],
    ["lost", { state: "unknown" }],
  ] as const) {
    it(`restarts from the new send time when it takes over a ${name} job`, async () => {
      const viewer = await showing(job);
      assert.equal(viewer.host.count("status"), 1);
      await post(viewer, { type: "find", queryId, key });
      const newSentAt = sentAt + 50_000;
      await post(viewer, takeOver(newSentAt));
      assert.deepEqual(viewer.received.at(-1), {
        type: "took-over",
        queryId,
        tabId: 5,
      });
      const written = parseViewerUrl(
        `?${viewer.replaced.at(-1)}`,
        "",
        newSentAt,
      );
      assert.ok(written.ok);
      assert.deepEqual(written.params, { ...seed, key, sentAt: newSentAt });
      assert.equal(
        stateText(viewer),
        drawnState({ kind: "loading", notice: { kind: "none" } }),
      );
      assert.equal(viewer.host.count("status"), 2);
    });
  }

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
