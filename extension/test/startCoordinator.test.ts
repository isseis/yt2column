import assert from "node:assert/strict";
import { describe, it } from "node:test";

import type {
  BrowserCall,
  MessageSender,
  ViewerChannel,
} from "../src/browser/chromeDeps.ts";
import {
  parseViewerChannelMessage,
  type ViewerChannelMessage,
} from "../src/core/extensionMessages.ts";
import type { NativeReply } from "../src/core/native.ts";
import {
  startResultDeadlineMs,
  viewerQueryWindowMs,
} from "../src/core/poll.ts";
import { maxBodyBytes } from "../src/core/prepare.ts";
import { sendTimeoutMs } from "../src/send.ts";
import { listenForMessages } from "../src/browser/chromeDeps.ts";
import { createStartCoordinator } from "../src/startCoordinator.ts";
import {
  fakeChannels,
  fakeExtensionId,
  fakeIdentity,
  fakeRuntime,
  fakeTabs,
  popupSender,
  viewerSender,
  type FakeChannels,
  type FakeRuntime,
  type FakeTabs,
} from "./helpers/chrome.ts";
import { fakeClock, settle, type FakeClock } from "./helpers/clock.ts";
import { recordingLogger, type RecordingLogger } from "./helpers/logger.ts";
import {
  errorReply,
  recordingNative,
  responseReply,
  type NativeRecorder,
} from "./helpers/native.ts";

const sentAt = 1_700_000_000_000;
const watchUrl = "https://www.youtube.com/watch?v=abc";
const key = "key-1";
const requestId = "0123456789abcdef";
/** The ID the fake tabs give the first tab the worker opens. */
const newTab = 100;

/** A request the worker accepts; each rejected case changes one value of it. */
function startRequest(): Record<string, Record<string, unknown> | string> {
  return {
    type: "start",
    input: {
      videoUrl: watchUrl,
      transcript: "line one\nline two",
      title: "Title",
      channelName: "Channel",
      description: "Description",
    },
    viewer: {
      heading: "Title",
      channelName: "Channel",
      title: "Title",
      url: watchUrl,
      sentAt,
    },
    source: { tabId: 7, windowId: 3, index: 4 },
  };
}

/** startRequest with its viewer seed changed by patch. */
function withViewer(patch: Record<string, unknown>): unknown {
  const request = startRequest();
  return { ...request, viewer: { ...(request.viewer as object), ...patch } };
}

/** The ID of the worker's nth channel query; the fake source counts them. */
const nthQuery = (n: number): string => n.toString(16).padStart(32, "0");

const accepted = (acceptedKey = key): NativeReply =>
  responseReply(
    202,
    JSON.stringify({ status: "accepted", key: acceptedKey, requestId }),
  );

const rejected: NativeReply = responseReply(
  400,
  JSON.stringify({ status: "rejected", reason: "invalid_body", requestId }),
);

interface Setup {
  readonly runtime: FakeRuntime;
  readonly tabs: FakeTabs;
  readonly channels: FakeChannels;
  readonly clock: FakeClock;
  readonly native: NativeRecorder;
  readonly logger: RecordingLogger;
}

interface SetupOptions {
  /** What the host answers to each start; accepted by default. */
  readonly respond?: () => Promise<NativeReply>;
  /** The clock's time when the test starts; the send time by default. */
  readonly clockStart?: number;
}

/**
 * A coordinator on fakes, registered on runtime through listenForMessages as
 * background.ts registers it; the host answers with reply.
 */
function setup(
  reply: NativeReply = accepted(),
  { respond = async () => reply, clockStart = sentAt }: SetupOptions = {},
): Setup {
  const runtime = fakeRuntime();
  const tabs = fakeTabs(newTab);
  const channels = fakeChannels();
  const clock = fakeClock(clockStart);
  const native = recordingNative(respond);
  const logger = recordingLogger();
  let queries = 0;
  listenForMessages(
    runtime.events,
    createStartCoordinator({
      identity: fakeIdentity,
      tabs,
      openChannel: channels.open,
      queryId: () => nthQuery((queries += 1)),
      send: { sendNative: native.sendNative, schedule: clock.schedule },
      now: clock.now,
      log: logger.log,
    }),
  );
  return { runtime, tabs, channels, clock, native, logger };
}

/** What a fake viewer does when a query reaches it. */
interface ViewerBehavior {
  readonly tabId: number;
  readonly sentAt: number;
  /** The key it shows; a viewer without one is still waiting for its start result. */
  readonly key?: string;
  /** Whether it answers find at all; a discarded tab does not. */
  readonly answers?: boolean;
  /** Whether it confirms a take-over it was asked for. */
  readonly takesOver?: boolean;
  /** Values its confirmation carries instead of its own: a confirmation the worker must not accept. */
  readonly confirmAs?: { readonly tabId?: number; readonly queryId?: string };
}

/**
 * A viewer on the channel that answers as the viewer page will: present for
 * its key, waiting while it has no key, and took-over for a take-over of its
 * own query and tab. It records every take-over it receives.
 */
function fakeViewer(
  channels: FakeChannels,
  behavior: ViewerBehavior,
): { readonly takeOvers: ViewerChannelMessage[] } {
  const opened = channels.open();
  assert.ok(opened.ok);
  const channel: ViewerChannel = opened.value;
  const takeOvers: ViewerChannelMessage[] = [];
  const presentTo = new Set<string>();
  channel.listen((data) => {
    const message = parseViewerChannelMessage(data);
    if (message === undefined || behavior.answers === false) {
      return;
    }
    if (message.type === "find") {
      if (behavior.key === undefined) {
        channel.post({
          type: "waiting",
          queryId: message.queryId,
          tabId: behavior.tabId,
          sentAt: behavior.sentAt,
        });
      } else if (behavior.key === message.key) {
        presentTo.add(message.queryId);
        channel.post({
          type: "present",
          queryId: message.queryId,
          tabId: behavior.tabId,
          windowId: 9,
          sentAt: behavior.sentAt,
        });
      }
    } else if (
      message.type === "take-over" &&
      message.tabId === behavior.tabId &&
      presentTo.has(message.queryId)
    ) {
      takeOvers.push(message);
      if (behavior.takesOver !== false) {
        channel.post({
          type: "took-over",
          queryId: behavior.confirmAs?.queryId ?? message.queryId,
          tabId: behavior.confirmAs?.tabId ?? behavior.tabId,
        });
      }
    }
  });
  return { takeOvers };
}

/** Runs the worker through the find window and the take-over window. */
async function runQueries(clock: FakeClock): Promise<void> {
  await settle();
  clock.advance(viewerQueryWindowMs);
  await settle();
  clock.advance(viewerQueryWindowMs);
  await settle();
}

/**
 * Runs every timer the worker may hold to its end, so a send scheduled for
 * later would be seen, and checks that none is left.
 */
async function runOut(clock: FakeClock): Promise<void> {
  await settle();
  clock.advance(startResultDeadlineMs + sendTimeoutMs);
  await settle();
  assert.equal(clock.pending(), 0, "a timer is left running");
}

/** The new viewer's wait for its result, sent once the tab exists. */
function awaitStart(
  runtime: FakeRuntime,
  tabId = newTab,
  waitSentAt = sentAt,
  sender: MessageSender = viewerSender(tabId),
): Promise<BrowserCall<unknown>> {
  return runtime.send(sender, { type: "await-start", sentAt: waitSentAt });
}

const acceptedOutcome = { kind: "accepted", key, requestId };

describe("start request sender checks", () => {
  it("accepts the toolbar popup's request: one start and one tab", async () => {
    const { runtime, tabs, native, clock } = setup();
    const reply = runtime.send(popupSender, startRequest());
    await runQueries(clock);
    assert.deepEqual(runtime.returned, [true]);
    assert.equal(native.messages.length, 1);
    assert.equal(tabs.calls.filter(({ op }) => op === "create").length, 1);
    assert.deepEqual(await reply, {
      ok: true,
      value: { outcome: acceptedOutcome, viewerTabId: newTab },
    });
  });

  const viewerUrl = fakeIdentity.getURL("viewer.html");
  const ignored: {
    name: string;
    sender: MessageSender;
    request?: unknown;
    check: string;
  }[] = [
    {
      name: "a script injected into a page",
      sender: { ...popupSender, tab: { id: 7 } },
      check: "sender-tab",
    },
    {
      name: "a viewer page",
      sender: { id: fakeExtensionId, url: `${viewerUrl}?h=a` },
      check: "sender-url",
    },
    {
      name: "another extension",
      sender: { ...popupSender, id: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb" },
      check: "sender-id",
    },
    {
      name: "the popup with a query",
      sender: { ...popupSender, url: `${popupSender.url}?x` },
      check: "sender-url",
    },
    {
      name: "the popup with a fragment",
      sender: { ...popupSender, url: `${popupSender.url}#x` },
      check: "sender-url",
    },
    {
      name: "a request of another shape",
      sender: popupSender,
      request: { ...startRequest(), extra: true },
      check: "shape",
    },
    {
      name: "an input over 4 MiB",
      sender: popupSender,
      request: {
        ...startRequest(),
        input: {
          videoUrl: watchUrl,
          transcript: "a".repeat(
            maxBodyBytes -
              JSON.stringify({ videoUrl: watchUrl, transcript: "" }).length +
              1,
          ),
        },
      },
      check: "size",
    },
    {
      name: "an unpaired surrogate in the heading",
      sender: popupSender,
      request: withViewer({ heading: "a\uD83D" }),
      check: "shape",
    },
    {
      name: "a send time in the future",
      sender: popupSender,
      request: withViewer({ sentAt: sentAt + 1 }),
      check: "shape",
    },
  ];
  for (const { name, sender, request, check } of ignored) {
    it(`ignores ${name}: no start, no tab, false`, async () => {
      const { runtime, tabs, native, logger } = setup();
      const reply = runtime.send(sender, request ?? startRequest());
      await settle();
      assert.deepEqual(runtime.returned, [false]);
      assert.deepEqual(native.messages, []);
      assert.deepEqual(tabs.calls, []);
      assert.deepEqual(logger.errors, [
        {
          label: "yt2column start request",
          data: { accepted: false, check },
        },
      ]);
      assert.equal((await reply).ok, false);
    });
  }

  it("ignores a start request of another declared type without checking it", async () => {
    const { runtime, native, tabs, logger } = setup();
    await runtime.send(popupSender, { ...startRequest(), type: "publish" });
    assert.deepEqual(runtime.returned, [false]);
    assert.deepEqual(native.messages, []);
    assert.deepEqual(tabs.calls, []);
    // The start check logs every request it rejects; this one never reached it.
    assert.deepEqual(logger.errors, []);
  });

  it("does not take a popup's wait for a viewer's", async () => {
    const { runtime, native, tabs } = setup();
    const reply = await runtime.send(popupSender, {
      type: "await-start",
      sentAt,
    });
    assert.equal(reply.ok, false);
    assert.deepEqual(native.messages, []);
    assert.deepEqual(tabs.calls, []);
  });
});

describe("viewer tab", () => {
  it("opens the viewer in the background next to the YouTube tab", async () => {
    const { runtime, tabs, clock } = setup(rejected);
    void runtime.send(popupSender, startRequest());
    await runQueries(clock);
    assert.deepEqual(tabs.calls[0], {
      op: "create",
      tab: {
        url: `${fakeIdentity.getURL("viewer.html")}?h=Title&c=Channel&t=Title&u=${encodeURIComponent(watchUrl)}&s=${sentAt}`,
        active: false,
        windowId: 3,
        index: 5,
        openerTabId: 7,
      },
    });
  });

  it("answers the popup alone when the tab cannot be opened", async () => {
    const { runtime, tabs, clock, native } = setup();
    tabs.fail("create");
    const popup = runtime.send(popupSender, startRequest());
    await runQueries(clock);
    assert.deepEqual(await popup, {
      ok: true,
      value: { outcome: acceptedOutcome },
    });
    assert.equal(native.messages.length, 1);
  });
});

describe("start message", () => {
  it("sends the host the start with the popup's input as given", async () => {
    const { runtime, native, clock } = setup();
    void runtime.send(popupSender, startRequest());
    await runQueries(clock);
    assert.deepEqual(native.messages, [
      {
        v: 1,
        op: "start",
        input: {
          videoUrl: "https://www.youtube.com/watch?v=abc",
          transcript: "line one\nline two",
          title: "Title",
          channelName: "Channel",
          description: "Description",
        },
      },
    ]);
  });
});

describe("no resend", () => {
  const failures: { name: string; reply: NativeReply; outcome: unknown }[] = [
    {
      name: "a rejection",
      reply: rejected,
      outcome: { kind: "rejected", reason: "invalid_body", requestId },
    },
    {
      name: "a server that is not running",
      reply: errorReply("server_unavailable"),
      outcome: { kind: "not-sent", cause: "server-unavailable" },
    },
    {
      name: "no response",
      reply: errorReply("server_timeout"),
      outcome: { kind: "no-response" },
    },
    {
      name: "a host failure",
      reply: { kind: "failed", lastError: "Native host has exited." },
      outcome: { kind: "host-failed" },
    },
    {
      name: "an unexpected reply",
      reply: responseReply(202, "{}"),
      outcome: { kind: "unexpected", check: "member-set" },
    },
  ];
  for (const { name, reply, outcome } of failures) {
    it(`gives ${name} to the viewer and the popup, and never sends again`, async () => {
      const { runtime, native, clock } = setup(reply);
      const popup = runtime.send(popupSender, startRequest());
      await settle();
      const viewer = awaitStart(runtime);
      await runQueries(clock);
      assert.deepEqual(await viewer, {
        ok: true,
        value: { kind: "result", outcome },
      });
      assert.deepEqual(await popup, {
        ok: true,
        value: { outcome, viewerTabId: newTab },
      });
      await runOut(clock);
      assert.equal(native.messages.length, 1);
    });
  }

  it("sends nothing again when the host does not answer within the limit", async () => {
    const { runtime, clock, native } = setup(accepted(), {
      respond: () => new Promise<NativeReply>(() => {}),
    });
    const popup = runtime.send(popupSender, startRequest());
    await settle();
    clock.advance(sendTimeoutMs);
    await settle();
    assert.deepEqual(await popup, {
      ok: true,
      value: { outcome: { kind: "no-response" }, viewerTabId: newTab },
    });
    await runOut(clock);
    assert.equal(native.messages.length, 1);
  });
});

describe("popup closed", () => {
  it("gives the result to the viewer when the popup closed after sending", async () => {
    const { runtime, clock, native } = setup();
    // The response to the closed popup throws, as in the browser, once the
    // result is known; the viewer must get the result all the same.
    void runtime.send(popupSender, startRequest(), { closed: true });
    await settle();
    const viewer = awaitStart(runtime);
    await runQueries(clock);
    assert.deepEqual(await viewer, {
      ok: true,
      value: { kind: "result", outcome: acceptedOutcome },
    });
    assert.equal(native.messages.length, 1);
  });
});

describe("early waits", () => {
  it("answers none to a held wait once its tab could not be created", async () => {
    const { runtime, tabs, clock } = setup();
    tabs.holdCreates();
    tabs.fail("create");
    void runtime.send(popupSender, startRequest());
    await settle();
    const viewer = awaitStart(runtime);
    await settle();
    tabs.releaseCreates();
    await runQueries(clock);
    assert.deepEqual(await viewer, { ok: true, value: { kind: "none" } });
  });

  it("answers none to a held wait that matches no record once the tab exists", async () => {
    const { runtime, tabs, clock } = setup();
    tabs.holdCreates();
    void runtime.send(popupSender, startRequest());
    await settle();
    const viewer = awaitStart(runtime, newTab, sentAt - 1);
    await settle();
    tabs.releaseCreates();
    await runQueries(clock);
    assert.deepEqual(await viewer, { ok: true, value: { kind: "none" } });
  });

  it("holds a wait that arrives before the tab exists, then answers it", async () => {
    const { runtime, tabs, clock } = setup();
    tabs.holdCreates();
    void runtime.send(popupSender, startRequest());
    await settle();
    const viewer = awaitStart(runtime);
    await settle();
    assert.deepEqual(runtime.returned, [true, true]);
    tabs.releaseCreates();
    await runQueries(clock);
    assert.deepEqual(await viewer, {
      ok: true,
      value: { kind: "result", outcome: acceptedOutcome },
    });
  });
});

describe("reuse", () => {
  it("lets the oldest viewer of the key take over and closes the new tab", async () => {
    const { runtime, tabs, channels, clock } = setup();
    const waiting = fakeViewer(channels, { tabId: newTab, sentAt });
    const older = fakeViewer(channels, { tabId: 20, sentAt: 5, key });
    const newer = fakeViewer(channels, { tabId: 10, sentAt: 6, key });
    const popup = runtime.send(popupSender, startRequest());
    await settle();
    const viewer = awaitStart(runtime);
    await runQueries(clock);
    assert.deepEqual(await popup, {
      ok: true,
      value: { outcome: acceptedOutcome, viewerTabId: 20 },
    });
    // The take-over carries the find's query ID and this send's time.
    assert.deepEqual(older.takeOvers, [
      { type: "take-over", queryId: nthQuery(1), tabId: 20, sentAt },
    ]);
    assert.deepEqual(newer.takeOvers, []);
    assert.deepEqual(waiting.takeOvers, []);
    assert.deepEqual(
      tabs.calls.map(({ op }) => op),
      ["create", "remove"],
      // The worker has no window focus at all; of the tab operations, only
      // the create and the remove happen.
      "the take-over activates no tab",
    );
    assert.equal(tabs.open.has(newTab), false);
    clock.advance(startResultDeadlineMs);
    await settle();
    assert.deepEqual(
      await viewer,
      { ok: true, value: { kind: "none" } },
      "a tab closed for the take-over never gets the result",
    );
  });

  const kept: {
    name: string;
    /** Sets up the viewers and returns the one showing the key (or another key). */
    viewers: (channels: FakeChannels) => {
      readonly takeOvers: ViewerChannelMessage[];
    };
    /** How many take-overs that viewer is asked for. */
    takeOvers: number;
    failRemove?: boolean;
  }[] = [
    {
      name: "the new tab does not answer that it waits",
      takeOvers: 0,
      viewers: (channels) => {
        return fakeViewer(channels, { tabId: 20, sentAt: 5, key });
      },
    },
    {
      name: "the new tab waits for another send",
      takeOvers: 0,
      viewers: (channels) => {
        fakeViewer(channels, { tabId: newTab, sentAt: sentAt - 1 });
        return fakeViewer(channels, { tabId: 20, sentAt: 5, key });
      },
    },
    {
      name: "the other viewer does not confirm the take-over",
      takeOvers: 1,
      viewers: (channels) => {
        fakeViewer(channels, { tabId: newTab, sentAt });
        return fakeViewer(channels, {
          tabId: 20,
          sentAt: 5,
          key,
          takesOver: false,
        });
      },
    },
    {
      name: "the take-over is confirmed for another tab",
      takeOvers: 1,
      viewers: (channels) => {
        fakeViewer(channels, { tabId: newTab, sentAt });
        return fakeViewer(channels, {
          tabId: 20,
          sentAt: 5,
          key,
          confirmAs: { tabId: 21 },
        });
      },
    },
    {
      name: "the take-over is confirmed for another query",
      takeOvers: 1,
      viewers: (channels) => {
        fakeViewer(channels, { tabId: newTab, sentAt });
        return fakeViewer(channels, {
          tabId: 20,
          sentAt: 5,
          key,
          confirmAs: { queryId: "f".repeat(32) },
        });
      },
    },
    {
      name: "no viewer shows the key",
      takeOvers: 0,
      viewers: (channels) => {
        fakeViewer(channels, { tabId: newTab, sentAt });
        return fakeViewer(channels, { tabId: 20, sentAt: 5, key: "key-2" });
      },
    },
    {
      name: "the viewer of the key does not answer (discarded)",
      takeOvers: 0,
      viewers: (channels) => {
        fakeViewer(channels, { tabId: newTab, sentAt });
        return fakeViewer(channels, {
          tabId: 20,
          sentAt: 5,
          key,
          answers: false,
        });
      },
    },
    {
      name: "the new tab cannot be closed",
      takeOvers: 1,
      viewers: (channels) => {
        fakeViewer(channels, { tabId: newTab, sentAt });
        return fakeViewer(channels, { tabId: 20, sentAt: 5, key });
      },
      failRemove: true,
    },
  ];
  for (const { name, viewers, takeOvers, failRemove } of kept) {
    it(`keeps the new tab and gives it the result when ${name}`, async () => {
      const { runtime, tabs, channels, clock } = setup();
      if (failRemove === true) {
        tabs.fail("remove");
      }
      const existing = viewers(channels);
      const popup = runtime.send(popupSender, startRequest());
      await settle();
      const viewer = awaitStart(runtime);
      await runQueries(clock);
      assert.deepEqual(await viewer, {
        ok: true,
        value: { kind: "result", outcome: acceptedOutcome },
      });
      assert.deepEqual(await popup, {
        ok: true,
        value: { outcome: acceptedOutcome, viewerTabId: newTab },
      });
      assert.equal(tabs.open.has(newTab), true);
      assert.equal(existing.takeOvers.length, takeOvers);
      assert.ok(!tabs.calls.some(({ op }) => op === "activate"));
    });
  }
});

describe("viewer tab log", () => {
  it("logs a removal that failed at error, with each step's outcome", async () => {
    const { runtime, tabs, channels, clock, logger } = setup();
    tabs.fail("remove");
    fakeViewer(channels, { tabId: newTab, sentAt });
    fakeViewer(channels, { tabId: 20, sentAt: 5, key });
    void runtime.send(popupSender, startRequest());
    await runQueries(clock);
    assert.deepEqual(
      logger.errors.filter(({ label }) => label === "yt2column viewer tab"),
      [
        {
          label: "yt2column viewer tab",
          data: {
            created: true,
            present: 1,
            waiting: 1,
            tookOver: true,
            remove: "failed",
          },
        },
      ],
    );
  });
});

describe("viewer waits", () => {
  /** A worker that has given its new tab an accepted result. */
  async function delivered(): Promise<Setup> {
    const context = setup();
    void context.runtime.send(popupSender, startRequest());
    await runQueries(context.clock);
    return context;
  }

  it("answers the result again to a reload of the same tab", async () => {
    const { runtime } = await delivered();
    assert.deepEqual(await awaitStart(runtime), {
      ok: true,
      value: { kind: "result", outcome: acceptedOutcome },
    });
  });

  it("answers none to a tab whose send time matches no record", async () => {
    const { runtime } = await delivered();
    assert.deepEqual(await awaitStart(runtime, newTab, sentAt - 1), {
      ok: true,
      value: { kind: "none" },
    });
  });

  it("never gives one tab's result to another tab", async () => {
    const { runtime } = await delivered();
    assert.deepEqual(await awaitStart(runtime, newTab + 1), {
      ok: true,
      value: { kind: "none" },
    });
  });

  it("drops the record at the deadline from the send time, not from arrival", async () => {
    // The request arrives 3 seconds after its send time.
    const { runtime, clock } = setup(accepted(), {
      clockStart: sentAt + 3_000,
    });
    void runtime.send(popupSender, startRequest());
    await runQueries(clock);
    clock.advance(sentAt + startResultDeadlineMs - clock.now() - 1);
    await settle();
    assert.deepEqual(await awaitStart(runtime), {
      ok: true,
      value: { kind: "result", outcome: acceptedOutcome },
    });
    clock.advance(1);
    await settle();
    assert.deepEqual(await awaitStart(runtime), {
      ok: true,
      value: { kind: "none" },
    });
  });

  it("drops at once a record whose deadline passed before it arrived", async () => {
    const { runtime, clock } = setup(rejected, {
      clockStart: sentAt + startResultDeadlineMs + 1,
    });
    const popup = runtime.send(popupSender, startRequest());
    await settle();
    await runQueries(clock);
    assert.deepEqual(await awaitStart(runtime), {
      ok: true,
      value: { kind: "none" },
    });
    assert.equal((await popup).ok, true);
  });

  const viewerUrl = fakeIdentity.getURL("viewer.html");
  const ignored: { name: string; sender: MessageSender }[] = [
    {
      name: "another extension",
      sender: {
        ...viewerSender(newTab),
        id: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
      },
    },
    {
      name: "a sender without a tab",
      sender: { id: fakeExtensionId, url: viewerUrl },
    },
    {
      name: "a tab without an ID",
      sender: { id: fakeExtensionId, url: viewerUrl, tab: {} },
    },
    {
      name: "a page that is not the viewer",
      sender: {
        ...viewerSender(newTab),
        url: fakeIdentity.getURL("popup.html?viewer.html"),
      },
    },
    {
      name: "a viewer URL with a fragment instead of a query",
      sender: { ...viewerSender(newTab), url: `${viewerUrl}#h=a` },
    },
  ];
  for (const { name, sender } of ignored) {
    it(`ignores a wait from ${name}`, async () => {
      const { runtime } = await delivered();
      const reply = await awaitStart(runtime, newTab, sentAt, sender);
      assert.equal(reply.ok, false);
      assert.deepEqual(runtime.returned.at(-1), false);
    });
  }

  it("ignores a wait of another shape", async () => {
    const { runtime } = await delivered();
    const reply = await runtime.send(viewerSender(newTab), {
      type: "await-start",
      sentAt: String(sentAt),
    });
    assert.equal(reply.ok, false);
  });
});

describe("start logs", () => {
  const marker = "MARKER";

  it("logs no input, metadata, URL or key, and keeps the browser error out of the replies", async () => {
    const { runtime, clock, logger } = setup({
      kind: "failed",
      lastError: "Native host has exited. BROWSER-ERROR",
    });
    const markedUrl = `https://www.youtube.com/watch?v=${marker}url`;
    const request = {
      type: "start",
      input: {
        videoUrl: markedUrl,
        transcript: `${marker} transcript`,
        title: `${marker} title`,
        channelName: `${marker} channel`,
        description: `${marker} description`,
      },
      viewer: {
        heading: `${marker} heading`,
        channelName: `${marker} channel`,
        title: `${marker} title`,
        url: markedUrl,
        sentAt,
      },
      source: { tabId: 7, windowId: 3, index: 4 },
    };
    const popup = runtime.send(popupSender, request);
    await settle();
    const viewer = awaitStart(runtime);
    await runQueries(clock);
    // The replies carry no error text: the "a host failure" row of no resend
    // compares them whole.
    await popup;
    await viewer;
    const logs = JSON.stringify([logger.infos, logger.errors]);
    assert.ok(!logs.includes(marker), logs);
    assert.ok(logs.includes("BROWSER-ERROR"), logs);
  });

  it("logs no key of an accepted start and no key of a reuse", async () => {
    const markedKey = `${marker}-key`;
    const { runtime, channels, clock, logger } = setup(accepted(markedKey));
    fakeViewer(channels, { tabId: newTab, sentAt });
    fakeViewer(channels, { tabId: 20, sentAt: 5, key: markedKey });
    const popup = runtime.send(popupSender, startRequest());
    await runQueries(clock);
    assert.deepEqual(await popup, {
      ok: true,
      value: {
        outcome: { kind: "accepted", key: markedKey, requestId },
        viewerTabId: 20,
      },
    });
    const logs = JSON.stringify([logger.infos, logger.errors]);
    assert.ok(!logs.includes(marker), logs);
    assert.deepEqual(
      logger.infos.filter(({ label }) => label === "yt2column viewer tab"),
      [
        {
          label: "yt2column viewer tab",
          data: {
            created: true,
            present: 1,
            waiting: 1,
            tookOver: true,
            remove: "ok",
          },
        },
      ],
    );
  });
});

describe("concurrent sends", () => {
  /** A setup whose host answers rejected to the first start and not-sent to the second. */
  function twoAnswers(): Setup {
    let starts = 0;
    return setup(undefined, {
      respond: async () =>
        (starts += 1) === 1 ? rejected : errorReply("server_unavailable"),
    });
  }

  const second = (): unknown => withViewer({ sentAt: sentAt + 1 });

  it("gives each viewer its own send's result and keeps one deadline from ending the other", async () => {
    const { runtime, clock } = twoAnswers();
    void runtime.send(popupSender, startRequest());
    await settle();
    clock.advance(1);
    void runtime.send(popupSender, second());
    await runQueries(clock);
    const own = async (tabId: number, waitSentAt: number) =>
      ((await awaitStart(runtime, tabId, waitSentAt)) as { value: unknown })
        .value;
    assert.deepEqual(await own(newTab, sentAt), {
      kind: "result",
      outcome: { kind: "rejected", reason: "invalid_body", requestId },
    });
    const notSent = {
      kind: "result",
      outcome: { kind: "not-sent", cause: "server-unavailable" },
    };
    assert.deepEqual(await own(newTab + 1, sentAt + 1), notSent);
    assert.deepEqual(await own(newTab, sentAt + 1), { kind: "none" });
    clock.advance(sentAt + startResultDeadlineMs - clock.now());
    await settle();
    assert.deepEqual(await own(newTab, sentAt), { kind: "none" });
    assert.deepEqual(await own(newTab + 1, sentAt + 1), notSent);
  });

  it("holds a wait for one send while another send's tab is still being created", async () => {
    const { runtime, tabs, clock } = twoAnswers();
    tabs.holdCreates();
    void runtime.send(popupSender, startRequest());
    await settle();
    clock.advance(1);
    void runtime.send(popupSender, second());
    await settle();
    const viewer = awaitStart(runtime, newTab + 1, sentAt + 1);
    await settle();
    tabs.releaseCreates();
    await runQueries(clock);
    assert.deepEqual(await viewer, {
      ok: true,
      value: {
        kind: "result",
        outcome: { kind: "not-sent", cause: "server-unavailable" },
      },
    });
  });

  it("lets one viewer of the key take over two sends of that key", async () => {
    const { runtime, tabs, channels, clock } = setup();
    fakeViewer(channels, { tabId: newTab, sentAt });
    fakeViewer(channels, { tabId: newTab + 1, sentAt: sentAt + 1 });
    const existing = fakeViewer(channels, { tabId: 20, sentAt: 5, key });
    const first = runtime.send(popupSender, startRequest());
    await settle();
    clock.advance(1);
    const later = runtime.send(popupSender, second());
    await runQueries(clock);
    clock.advance(viewerQueryWindowMs);
    await settle();
    for (const reply of [await first, await later]) {
      assert.deepEqual(reply, {
        ok: true,
        value: { outcome: acceptedOutcome, viewerTabId: 20 },
      });
    }
    assert.equal(existing.takeOvers.length, 2);
    assert.deepEqual(
      tabs.calls.filter(({ op }) => op === "remove"),
      [
        { op: "remove", tabId: newTab },
        { op: "remove", tabId: newTab + 1 },
      ],
    );
  });
});
