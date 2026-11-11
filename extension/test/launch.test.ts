import assert from "node:assert/strict";
import { describe, it } from "node:test";

import { readUiLanguage } from "../src/browser/chromeDeps.ts";
import type { BrowserCall, I18nApi } from "../src/browser/chromeDeps.ts";
import type { ArticleKey } from "../src/core/articleKey.ts";
import type {
  CollectedInput,
  PageSelection,
  SelectionReader,
} from "../src/core/collect.ts";
import type {
  SourceTab,
  StartRequest,
  ViewerChannelMessage,
} from "../src/core/extensionMessages.ts";
import {
  parseStartRequest,
  parseViewerChannelMessage,
} from "../src/core/extensionMessages.ts";
import { catalogs } from "../src/core/i18n/catalog.ts";
import { locales } from "../src/core/i18n/locale.ts";
import type { Locale } from "../src/core/i18n/locale.ts";
import { acceptMetadata } from "../src/core/metadata.ts";
import type { MetadataResult } from "../src/core/metadata.ts";
import type { NativeReply } from "../src/core/native.ts";
import {
  elapsedRedrawMs,
  pollIntervalMs,
  startResultDeadlineMs,
} from "../src/core/poll.ts";
import { maxBodyBytes, prepareSend } from "../src/core/prepare.ts";
import { countCharacters } from "../src/core/summary.ts";
import {
  buildCollectedView,
  metadataDisplayLimit,
} from "../src/core/popupView.ts";
import type {
  CollectedView,
  ExistingState,
  PopupJobState,
  PopupView,
} from "../src/core/popupView.ts";
import type { PollNotice } from "../src/core/jobView.ts";
import type { StartOutcome } from "../src/core/response.ts";
import { statusNoticeMessage } from "../src/core/sendMessages.ts";
import type { Logger } from "../src/core/types.ts";
import { parseViewerUrl } from "../src/core/viewerUrl.ts";
import {
  launchContextFromTab,
  runPopupLaunch,
  runPopupSend,
  startPopup,
} from "../src/launch.ts";
import type {
  MetadataReader,
  PopupLaunch,
  PopupLaunchDeps,
  PopupStartDeps,
} from "../src/launch.ts";
import { renderPopup } from "../src/ui/render.ts";
import type { PopupElements } from "../src/ui/render.ts";
import {
  fakeChannels,
  fakeFocus,
  fakeIdentity,
  fakeTabs,
  pendingWorker,
  type FakeChannels,
  type FakeFocus,
  type FakeTabs,
  type PendingWorker,
} from "./helpers/chrome.ts";
import { fakeClock, settle, type FakeClock } from "./helpers/clock.ts";
import {
  expectAbsent,
  otherLocales,
  popupDocument,
  popupElements,
} from "./helpers/dom.ts";
import {
  doneJob as doneDisplay,
  doneSteps,
  requestId,
  runningSteps,
} from "./helpers/job.ts";
import { recordingLogger } from "./helpers/logger.ts";
import type { RecordingLogger } from "./helpers/logger.ts";
import {
  pendingHost,
  responseReply,
  type PendingHost,
} from "./helpers/native.ts";

const watchUrl = "https://www.youtube.com/watch?v=abc";
const malicious = "<img src=x onerror=alert(1)>";
const launchLabel = "yt2column popup launch";
const uiLanguageLabel = "yt2column ui language";
const metadataLabel = "yt2column video metadata";
const blockedLabel = "yt2column blocked";
const activeTabLabel = "yt2column active tab";
const statusLabel = "yt2column status result";
const localeList = Object.keys(locales) as Locale[];
/** The tab fakeTab is: the selection's source, where the viewer opens next to. */
const source: SourceTab = { tabId: 7, windowId: 3, index: 4 };
const sentAt = 1_700_000_000_000;
const key = "key-1" as ArticleKey;
const queryId = "0".repeat(31) + "1";

/** The skeleton's ids, one per element startPopup needs. */
const skeletonIds = [
  "video",
  "checks",
  "rejection",
  "action",
  "primary",
  "secondary",
  "status",
  "details",
] as const;

/** The five containers, which every render shows or hides. */
const containerIds = [
  "video",
  "checks",
  "rejection",
  "action",
  "details",
] as const;

/** Builds a chrome.tabs.Tab with the fields the code reads. */
function fakeTab(overrides: Partial<chrome.tabs.Tab> = {}): chrome.tabs.Tab {
  return {
    id: source.tabId,
    windowId: source.windowId,
    index: source.index,
    url: watchUrl,
    title: "A video",
    ...overrides,
  } as chrome.tabs.Tab;
}

/** Builds a page selection; the document URL defaults to the watch URL. */
function page(text: string, documentUrl = watchUrl): PageSelection {
  return { text, documentUrl };
}

/** A SelectionReader that returns the given selection and cannot fail. */
function readerReturning(selection: PageSelection): SelectionReader {
  return { read: async () => selection };
}

/** The JSON text the injected metadata reader returns for these values. */
function metadataJson(
  values: { title?: string; channelName?: string; description?: string },
  descriptionCollapsed = false,
): string {
  return JSON.stringify({ ...values, descriptionCollapsed });
}

interface FakeMetadataReader {
  readonly reader: MetadataReader;
  readonly calls: { tabId: number; expectedUrl: string }[];
}

/**
 * A MetadataReader that records its calls and returns text, or rejects when
 * the expected URL is not pageUrl, as the injected reader does on a mismatch.
 */
function metadataReader(text: string, pageUrl = watchUrl): FakeMetadataReader {
  const calls: { tabId: number; expectedUrl: string }[] = [];
  return {
    reader: {
      read: async (tabId, expectedUrl) => {
        calls.push({ tabId, expectedUrl });
        if (expectedUrl !== pageUrl) {
          throw new Error("the injected function returned an unexpected value");
        }
        return text;
      },
    },
    calls,
  };
}

/** A MetadataReader whose every read rejects with error. */
function rejectingMetadataReader(error: unknown): MetadataReader {
  return {
    read: async () => {
      throw error;
    },
  };
}

/** A MetadataReader whose read waits until resolve is called with the text. */
function deferredMetadataReader(): {
  readonly reader: MetadataReader;
  resolve(text: string): void;
} {
  let resolveRead: ((text: string) => void) | undefined;
  return {
    reader: {
      read: () =>
        new Promise<string>((resolve) => {
          resolveRead = resolve;
        }),
    },
    resolve: (text) => {
      assert.ok(resolveRead !== undefined, "the read has not started");
      resolveRead(text);
    },
  };
}

/** The collected view of the watch URL's input with the given selection and metadata. */
function collectedView(
  selection = "selection",
  metadata: MetadataResult = acceptMetadata({ descriptionCollapsed: false }),
): PopupView {
  const input = {
    selection,
    url: watchUrl,
    title: "A video",
  } as CollectedInput;
  return buildCollectedView(
    input,
    metadata,
    prepareSend(input, metadata.values),
  );
}

interface PopupHarness {
  readonly deps: PopupLaunchDeps;
  readonly recorder: RecordingLogger;
}

/** Builds popup-launch dependencies with a recording logger and fake tab. */
function popupHarness(overrides: Partial<PopupLaunchDeps> = {}): PopupHarness {
  const recorder = recordingLogger();
  const deps: PopupLaunchDeps = {
    reader: readerReturning(page("selection")),
    metadataReader: metadataReader(metadataJson({})).reader,
    activeTab: async () => fakeTab(),
    log: recorder.log,
    ...overrides,
  };
  return { deps, recorder };
}

/** The outcome the launch path logged for the developer: its kind and reason only. */
function loggedOutcome(recorder: RecordingLogger): unknown {
  const entry = recorder.infos.find(
    (recorded) => recorded.label === launchLabel,
  );
  assert.ok(entry !== undefined, "expected the popup launch log");
  return (entry.data as { outcome: unknown }).outcome;
}

/** The collected launch of collectedView's input, from the source tab. */
function collectedLaunch(
  selection = "selection",
  metadata: MetadataResult = acceptMetadata({ descriptionCollapsed: false }),
): PopupLaunch {
  return { view: collectedView(selection, metadata), source };
}

/** What a popup run of the send path is made of, all fakes the test drives. */
interface Popup {
  readonly elements: PopupElements;
  readonly clock: FakeClock;
  readonly worker: PendingWorker;
  readonly host: PendingHost;
  readonly tabs: FakeTabs;
  readonly focus: FakeFocus;
  readonly channels: FakeChannels;
  readonly logger: RecordingLogger;
}

/**
 * Draws launch's view into a fresh copy of the shipped skeleton at sentAt and
 * runs the send path on it, in the order startPopup does.
 */
function shown(
  launch: PopupLaunch,
  locale: Locale = "en",
  log?: RecordingLogger,
  // Timers that never fire, as a throttled page's timer may run late.
  lateTimers = false,
): Popup {
  const elements = popupElements();
  const clock = fakeClock(sentAt);
  const worker = pendingWorker();
  const host = pendingHost();
  const tabs = fakeTabs();
  const focus = fakeFocus();
  const channels = fakeChannels();
  const logger = log ?? recordingLogger();
  renderPopup(elements, launch.view, locale);
  runPopupSend(elements, launch, {
    sendToWorker: worker.send,
    send: {
      sendNative: host.sendNative,
      schedule: lateTimers ? () => () => undefined : clock.schedule,
    },
    tabs,
    focusWindow: focus.focus,
    openChannel: channels.open,
    queryId: () => queryId,
    identity: fakeIdentity,
    now: clock.now,
    log: logger.log,
    locale,
  });
  return { elements, clock, worker, host, tabs, focus, channels, logger };
}

/**
 * Dispatches a click on button, by default the primary button, and lets the
 * page run. Dispatching the event (rather than calling click()) exercises the
 * handler even when the button is disabled or hidden, so the state guard is
 * tested and not merely the attribute.
 */
async function press(
  popup: Popup,
  button: HTMLButtonElement = popup.elements.primary,
): Promise<void> {
  const win = button.ownerDocument.defaultView;
  assert.ok(win !== null);
  button.dispatchEvent(new win.Event("click"));
  await settle();
}

/**
 * Clicks button, by default the primary button, as a keyboard press (Enter or
 * Space) does, a click with no pointer clicks counted, or as a mouse click,
 * and lets the page run.
 */
async function pressWith(
  popup: Popup,
  device: "keyboard" | "mouse",
  button: HTMLButtonElement = popup.elements.primary,
): Promise<void> {
  const win = button.ownerDocument.defaultView;
  assert.ok(win !== null);
  button.dispatchEvent(
    new win.MouseEvent("click", { detail: device === "keyboard" ? 0 : 1 }),
  );
  await settle();
}

/**
 * Leaves the focus nowhere, as the browser does with a focused button it
 * disables or hides; jsdom keeps the focus on it, and does not blur it either.
 */
function dropFocus(doc: Document): void {
  const holder = doc.createElement("input");
  doc.body.append(holder);
  holder.focus();
  holder.remove();
  assert.equal(doc.activeElement, doc.body);
}

/** The start requests the popup sent to the worker. */
function startRequests(popup: Popup): StartRequest[] {
  return popup.worker.sent.filter(
    (message): message is StartRequest => message.type === "start",
  );
}

/** The worker's reply carrying outcome, and the viewer tab when given. */
function startReply(
  outcome: StartOutcome,
  viewerTabId?: number,
): BrowserCall<unknown> {
  return {
    ok: true,
    value: viewerTabId === undefined ? { outcome } : { outcome, viewerTabId },
  };
}

const accepted: StartOutcome = { kind: "accepted", key, requestId };

type Json = Record<string, unknown>;

const article = {
  markdown: "# Title\n\nBody",
  mentionsNeutralized: false,
  videoTitle: "",
  channelName: "",
  file: "---\ntitle: Title\n---\n\nBody\n",
  cost: { state: "known", usd: "0.1234" },
  durationMs: 65_000,
};
const runningJob: Json = { state: "running", steps: runningSteps };
const doneJob: Json = {
  state: "done",
  steps: doneSteps,
  article,
};
const failedJob: Json = {
  state: "failed",
  steps: doneSteps,
  failure: "llm_failed",
};

/** The host's reply to a status query that found job. */
function statusReply(job: Json): NativeReply {
  return responseReply(200, JSON.stringify({ status: "ok", requestId, job }));
}

/** A popup pressed once whose start was accepted with the viewer in tab viewerTabId. */
async function acceptedPopup(
  viewerTabId?: number,
  launch = collectedLaunch(),
  locale: Locale = "en",
): Promise<Popup> {
  const popup = shown(launch, locale);
  await press(popup);
  await popup.worker.answer(startReply(accepted, viewerTabId));
  return popup;
}

/** The checks section text renderPopup draws for state in a fresh skeleton. */
function drawnChecks(
  view: CollectedView,
  state: PopupJobState,
  locale: Locale = "en",
  elapsedMs = 0,
): string {
  const elements = popupElements();
  renderPopup(
    elements,
    { kind: "job", video: view.video, details: view.details, state },
    locale,
    elapsedMs,
  );
  return elements.checks.textContent ?? "";
}

/** The popup's checks section text. */
function checksText(popup: Popup): string {
  return popup.elements.checks.textContent ?? "";
}

/** The collected view of a launch, asserting it is one. */
function collected(launch: PopupLaunch): CollectedView {
  assert.ok(launch.view.kind === "collected");
  return launch.view;
}

describe("launchContextFromTab", () => {
  it("returns undefined without a tab", () => {
    assert.equal(launchContextFromTab(undefined), undefined);
  });

  it("returns undefined without a tab id", () => {
    const tab = fakeTab();
    delete tab.id;
    assert.equal(launchContextFromTab(tab), undefined);
  });

  it("copies the id, url and title verbatim", () => {
    const tab = fakeTab({ id: 9, url: watchUrl, title: "  A title  " });
    assert.deepEqual(launchContextFromTab(tab), {
      tabId: 9,
      url: watchUrl,
      title: "  A title  ",
    });
  });
});

describe("launch paths", () => {
  it("collects the selection and the tab title through the popup path", async () => {
    const selection = " 00:00\ncaption line\n\n";
    const title = "  A title  ";
    const reader = readerReturning(page(selection));
    const tab = fakeTab({ id: 7, url: watchUrl, title });

    const popup = popupHarness({ reader, activeTab: async () => tab });
    const launch = await runPopupLaunch(popup.deps);

    const view = collected(launch);
    // The tab's place is kept for the send, which opens the viewer beside it.
    assert.deepEqual(launch.source, source);
    assert.deepEqual(loggedOutcome(popup.recorder), { kind: "collected" });
    // The view is built from the same input: the tab title stands in for the
    // missing video title, and the counts are the selection's.
    assert.equal(view.video.heading.value, title);
    assert.equal(view.selection.lineCount, 4);
  });

  it("sends the video title, not the tab title, in the start input", async () => {
    const selection = " 00:00\ncaption line\n\n";
    // The tab title and the video title differ, so the body shows which one
    // was sent: the body title must be the video title read from the page.
    const tabTitle = "TAB-TITLE-7f3a";
    const values = {
      title: "VIDEO-TITLE-91c2",
      channelName: "CHANNEL-5d0e",
      description: " first line\nsecond line ",
    };
    const reader = readerReturning(page(selection));
    const metadata = metadataReader(metadataJson(values)).reader;
    const tab = fakeTab({ id: 7, url: watchUrl, title: tabTitle });

    const launch = await runPopupLaunch(
      popupHarness({
        reader,
        metadataReader: metadata,
        activeTab: async () => tab,
      }).deps,
    );
    const popup = shown(launch);
    await press(popup);

    assert.deepEqual(
      startRequests(popup).map((request) => request.input),
      [{ videoUrl: watchUrl, transcript: selection, ...values }],
    );
  });

  it("reads the metadata once, from the collected tab and URL", async () => {
    const tab = fakeTab({ id: 9, url: watchUrl });
    const popupMetadata = metadataReader(metadataJson({}));
    await runPopupLaunch(
      popupHarness({
        metadataReader: popupMetadata.reader,
        activeTab: async () => tab,
      }).deps,
    );
    assert.deepEqual(popupMetadata.calls, [
      { tabId: 9, expectedUrl: watchUrl },
    ]);
  });

  const rejectedLaunches: {
    name: string;
    tab: chrome.tabs.Tab | undefined;
    selection: string;
  }[] = [
    {
      name: "not-watch-page",
      tab: fakeTab({ url: "https://example.com/" }),
      selection: "text",
    },
    { name: "collection-failed", tab: undefined, selection: "text" },
    { name: "empty-selection", tab: fakeTab(), selection: " \n" },
    { name: "empty-title", tab: fakeTab({ title: " " }), selection: "text" },
  ];
  for (const { name, tab, selection } of rejectedLaunches) {
    it(`does not read the metadata for a ${name} collection`, async () => {
      const reader = readerReturning(page(selection));
      const popupMetadata = metadataReader(metadataJson({ title: "T" }));
      const launch = await runPopupLaunch(
        popupHarness({
          reader,
          metadataReader: popupMetadata.reader,
          activeTab: async () => tab,
        }).deps,
      );
      assert.deepEqual(popupMetadata.calls, []);
      assert.deepEqual(launch, { view: { kind: "rejected", reason: name } });
    });
  }

  const readFailures: {
    name: string;
    reader: MetadataReader;
    check?: string;
  }[] = [
    {
      name: "the injection fails",
      reader: rejectingMetadataReader(new Error("injection failed")),
    },
    {
      // The URL comparison itself is tested in chromeDeps.test.ts; this row
      // checks that the reader's rejection on a mismatch omits every field.
      name: "the page URL is not the collected URL",
      reader: metadataReader(
        metadataJson({ title: "T" }),
        "https://www.youtube.com/watch?v=other",
      ).reader,
    },
    {
      name: "the read has the wrong shape",
      reader: metadataReader("[]").reader,
      check: "top-level",
    },
  ];
  for (const { name, reader, check } of readFailures) {
    it(`sends without metadata and logs an error when ${name}`, async () => {
      const allFailed = ["title", "channelName", "description"].map(
        (field) => ({
          field,
          reason: "read-failure",
        }),
      );
      const popup = popupHarness({ metadataReader: reader });
      const launch = await runPopupLaunch(popup.deps);
      collected(launch);
      const sent = shown(launch);
      // Without a video title the primary button still sends.
      assert.equal(sent.elements.primary.disabled, false);
      await press(sent);
      assert.deepEqual(
        startRequests(sent).map((request) => request.input),
        [{ videoUrl: watchUrl, transcript: "selection" }],
      );

      const expected =
        check === undefined
          ? { omissions: allFailed }
          : { check, omissions: allFailed };
      assert.deepEqual(
        popup.recorder.errors.filter((entry) => entry.label === metadataLabel),
        [{ label: metadataLabel, data: expected }],
      );
    });
  }

  it("treats a missing tab as collection-failed", async () => {
    const reader = readerReturning(page("text"));
    const popup = popupHarness({ reader, activeTab: async () => undefined });
    const launch = await runPopupLaunch(popup.deps);
    assert.deepEqual(launch, {
      view: { kind: "rejected", reason: "collection-failed" },
    });
    assert.deepEqual(loggedOutcome(popup.recorder), {
      kind: "rejected",
      reason: "collection-failed",
    });
  });

  it("treats a failing activeTab as collection-failed without rejecting", async () => {
    const popup = popupHarness({
      activeTab: async () => {
        throw new Error("query failed");
      },
    });
    const launch = await runPopupLaunch(popup.deps);
    assert.deepEqual(launch, {
      view: { kind: "rejected", reason: "collection-failed" },
    });
    assert.equal(popup.recorder.errors.length, 1);
  });

  const placeless: { name: string; overrides: Partial<chrome.tabs.Tab> }[] = [
    { name: "a negative window", overrides: { windowId: -1 } },
    { name: "a negative index", overrides: { index: -1 } },
    { name: "a fractional index", overrides: { index: 0.5 } },
  ];
  for (const { name, overrides } of placeless) {
    it(`treats a tab with ${name} as collection-failed`, async () => {
      // Precondition: the same tab with its place is collected.
      const placed = await runPopupLaunch(popupHarness().deps);
      assert.equal(placed.view.kind, "collected");
      const harness = popupHarness({
        activeTab: async () => fakeTab(overrides),
      });
      const launch = await runPopupLaunch(harness.deps);
      assert.deepEqual(launch, {
        view: { kind: "rejected", reason: "collection-failed" },
      });
      assert.deepEqual(harness.recorder.errors, [
        { label: activeTabLabel, data: { check: "tab-place" } },
      ]);
      assert.deepEqual(loggedOutcome(harness.recorder), {
        kind: "rejected",
        reason: "collection-failed",
      });
    });
  }

  it("keeps untrusted text out of the fixed log labels", async () => {
    const reader = readerReturning(page(`${malicious}\n`));
    const popup = popupHarness({ reader });
    await runPopupLaunch(popup.deps);
    assert.equal(popup.recorder.infos[0]?.label, launchLabel);
  });
});

describe("popup job send", () => {
  it("makes the primary button pressable for a sendable selection and sends nothing until it is pressed", async () => {
    const popup = shown(collectedLaunch());
    assert.equal(popup.elements.primary.disabled, false);
    await settle();
    assert.deepEqual(popup.worker.sent, []);
    assert.deepEqual(popup.host.sent, []);
    assert.deepEqual(popup.tabs.calls, []);
    assert.deepEqual(popup.channels.posted, []);
    assert.equal(popup.clock.pending(), 0);
  });

  it("asks the worker for one inquiry start and never sends a start to the host", async () => {
    const launch = collectedLaunch();
    const popup = shown(launch);
    await press(popup);
    await press(popup);
    assert.deepEqual(popup.worker.sent, [
      {
        type: "start",
        mode: "inquiry",
        input: { videoUrl: watchUrl, transcript: "selection" },
        viewer: { heading: "A video", url: watchUrl, sentAt },
        source,
      },
    ]);
    // Nothing reaches the host until the key is accepted, and then only the
    // status queries do.
    assert.deepEqual(popup.host.sent, []);
    await popup.worker.answer(startReply(accepted));
    await popup.host.answer(statusReply(runningJob));
    popup.clock.advance(pollIntervalMs);
    await settle();
    assert.deepEqual(popup.host.sent, [
      { v: 1, op: "status", key },
      { v: 1, op: "status", key },
    ]);
    assert.equal(startRequests(popup).length, 1);
  });

  it("sends the full video title while the viewer seed holds the shown heading and the cut title", async () => {
    const title = "t".repeat(1_001);
    const launch = await runPopupLaunch(
      popupHarness({
        metadataReader: metadataReader(metadataJson({ title })).reader,
      }).deps,
    );
    const popup = shown(launch);
    // Precondition: the heading is cut for display, so the checks below tell
    // the sent value apart from the displayed one.
    assert.equal(
      popup.elements.video.querySelector("h1")?.textContent,
      `${"t".repeat(metadataDisplayLimit)}…`,
    );
    await press(popup);
    const [request] = startRequests(popup);
    assert.ok(request !== undefined);
    assert.equal(request.input.title, title);
    assert.equal(
      request.viewer.heading,
      `${"t".repeat(metadataDisplayLimit)}…`,
    );
    assert.equal(request.viewer.title, "t".repeat(1_000));
  });

  it("replaces an unpaired surrogate in the seed's heading so the worker accepts the request", async () => {
    const launch = await runPopupLaunch(
      popupHarness({ activeTab: async () => fakeTab({ title: "A\uD800b" }) })
        .deps,
    );
    // Precondition: the heading shown is the tab title, surrogate and all.
    assert.equal(collected(launch).video.heading.value, "A\uD800b");
    const popup = shown(launch);
    await press(popup);
    const [request] = startRequests(popup);
    assert.ok(request !== undefined);
    assert.equal(request.viewer.heading, "A\uFFFDb");
    assert.equal(parseStartRequest(request, sentAt).ok, true);
  });

  it("builds a seed the worker accepts from the longest values the popup shows", async () => {
    // Astral characters, so code points and UTF-16 units differ.
    const astral = "\u{1F600}";
    const cases: {
      name: string;
      tabTitle: string;
      values: { title?: string; channelName?: string };
      lengths: { heading: number; channelName?: number; title?: number };
    }[] = [
      // No video title: the heading is the tab title, cut to 1,000 plus the ellipsis.
      {
        name: "tab title",
        tabTitle: astral.repeat(1_500),
        values: {},
        lengths: { heading: 1_001 },
      },
      // A video title: the heading is cut to 200, the seed's title to 1,000.
      {
        name: "video title",
        tabTitle: "A video",
        values: {
          title: astral.repeat(1_500),
          channelName: astral.repeat(300),
        },
        lengths: { heading: 201, channelName: 201, title: 1_000 },
      },
    ];
    for (const { name, tabTitle, values, lengths } of cases) {
      const launch = await runPopupLaunch(
        popupHarness({
          activeTab: async () => fakeTab({ title: tabTitle }),
          metadataReader: metadataReader(metadataJson(values)).reader,
        }).deps,
      );
      const popup = shown(launch);
      await press(popup);
      const [request] = startRequests(popup);
      assert.ok(request !== undefined);
      const { viewer } = request;
      assert.deepEqual(
        {
          heading: countCharacters(viewer.heading),
          ...(viewer.channelName === undefined
            ? {}
            : { channelName: countCharacters(viewer.channelName) }),
          ...(viewer.title === undefined
            ? {}
            : { title: countCharacters(viewer.title) }),
        },
        lengths,
        name,
      );
      assert.equal(parseStartRequest(request, sentAt).ok, true, name);
    }
  });

  it("sends the shown description while the expand prompt is up", async () => {
    const launch = await runPopupLaunch(
      popupHarness({
        metadataReader: metadataReader(
          metadataJson({ description: "first lines" }, true),
        ).reader,
      }).deps,
    );
    // Precondition: the launch reached the collapsed state; its display is
    // render.test.ts's.
    assert.equal(collected(launch).description.status.kind, "collapsed");
    const popup = shown(launch);
    assert.equal(popup.elements.primary.disabled, false);
    await press(popup);
    assert.equal(startRequests(popup)[0]?.input.description, "first lines");
  });

  const popupBlocks: {
    // Tells apart rows that share the reason.
    when: string;
    launch: Partial<PopupLaunchDeps>;
  }[] = [
    {
      // The size count itself is prepare.test.ts's contract.
      when: "the selection takes the body over the limit",
      launch: { reader: readerReturning(page("a".repeat(maxBodyBytes))) },
    },
    {
      // The selection alone fits, so only the metadata can block the send.
      when: "the metadata takes the body over the limit",
      launch: {
        metadataReader: metadataReader(
          metadataJson({ description: "d".repeat(maxBodyBytes) }),
        ).reader,
      },
    },
  ];
  for (const { when, launch } of popupBlocks) {
    it(`keeps the button disabled and sends nothing when ${when}`, async () => {
      const harness = popupHarness(launch);
      const launched = await runPopupLaunch(harness.deps);
      // Precondition: the size check blocked the selection; its display is
      // render.test.ts's.
      assert.deepEqual(collected(launched).selection.status, {
        kind: "blocked",
        reason: "body-too-large",
      });
      const popup = shown(launched);
      assert.equal(popup.elements.primary.disabled, true);
      await press(popup);
      assert.deepEqual(popup.worker.sent, []);
      assert.equal(popup.elements.status.childNodes.length, 0);
      // A blocked launch is normal: logged at info, never at error.
      assert.deepEqual(
        harness.recorder.infos.filter((entry) => entry.label === blockedLabel),
        [{ label: blockedLabel, data: { reason: "body-too-large" } }],
      );
      assert.equal(
        harness.recorder.errors.some((entry) => entry.label === blockedLabel),
        false,
      );
    });
  }

  it("keeps the button disabled and sends nothing for a rejected collection", async () => {
    const popup = shown({
      view: { kind: "rejected", reason: "empty-selection" },
    });
    assert.equal(popup.elements.primary.disabled, true);
    await press(popup);
    assert.deepEqual(popup.worker.sent, []);
    assert.equal(popup.elements.status.childNodes.length, 0);
  });
});

describe("popup job states", () => {
  for (const locale of localeList) {
    it(`shows sending in place of the checks in ${locale}, keeping the video section and the details`, async () => {
      const launch = await runPopupLaunch(
        popupHarness({
          metadataReader: metadataReader(
            metadataJson({ title: "A video title", channelName: "A channel" }),
          ).reader,
        }).deps,
      );
      const popup = shown(launch, locale);
      const sections = [popup.elements.video, popup.elements.details];
      const before = sections.map((section) => section.cloneNode(true));
      await press(popup);
      assert.equal(
        checksText(popup),
        drawnChecks(collected(launch), { kind: "sending" }, locale),
      );
      assert.equal(popup.elements.primary.disabled, true);
      assert.ok(popup.elements.status.childNodes.length > 0);
      sections.forEach((section, index) => {
        assert.ok(section.isEqualNode(before[index] ?? null), section.id);
      });
      await popup.worker.answer(startReply(accepted));
      await popup.host.answer(statusReply(runningJob));
      sections.forEach((section, index) => {
        assert.ok(section.isEqualNode(before[index] ?? null), section.id);
      });
    });
  }

  const focusCases: {
    name: string;
    /** What happens to the focus while the button is disabled. */
    meanwhile: (popup: Popup) => void;
    /** Where the focus is expected once the button can be pressed again. */
    expected: (popup: Popup) => Element;
  }[] = [
    {
      name: "gives the focus back to the button once it can be pressed again",
      meanwhile: (popup) => dropFocus(popup.elements.primary.ownerDocument),
      expected: (popup) => popup.elements.primary,
    },
    {
      name: "leaves the focus where the user moved it while the button was disabled",
      meanwhile: (popup) => {
        const summary = popup.elements.details.querySelector("summary");
        assert.ok(summary !== null);
        summary.focus();
      },
      expected: (popup) => {
        const summary = popup.elements.details.querySelector("summary");
        assert.ok(summary !== null);
        return summary;
      },
    },
  ];
  for (const { name, meanwhile, expected } of focusCases) {
    it(name, async () => {
      const popup = shown(collectedLaunch());
      const doc = popup.elements.primary.ownerDocument;
      popup.elements.primary.focus();
      await pressWith(popup, "keyboard");
      // Precondition: the press disabled the button.
      assert.equal(popup.elements.primary.disabled, true);
      meanwhile(popup);
      await popup.worker.answer(startReply(accepted, 100));
      assert.equal(popup.elements.primary.disabled, false);
      assert.equal(doc.activeElement, expected(popup));
    });
  }

  it("gives the focus back to the button after a retry", async () => {
    const popup = await acceptedPopup(100);
    await popup.host.answer(statusReply(failedJob));
    const doc = popup.elements.primary.ownerDocument;
    popup.elements.primary.focus();
    await pressWith(popup, "keyboard");
    assert.equal(popup.elements.primary.disabled, true);
    dropFocus(doc);
    await popup.worker.answer(startReply(accepted, 101));
    assert.equal(doc.activeElement, popup.elements.primary);
  });

  it("moves no focus to the button after a mouse press", async () => {
    const popup = shown(collectedLaunch());
    const doc = popup.elements.primary.ownerDocument;
    // A mouse press may focus the button (Windows, Linux) or not (macOS).
    popup.elements.primary.focus();
    await pressWith(popup, "mouse");
    assert.equal(popup.elements.primary.disabled, true);
    dropFocus(doc);
    await popup.worker.answer(startReply(accepted, 100));
    assert.equal(popup.elements.primary.disabled, false);
    assert.equal(doc.activeElement, doc.body);
  });

  it("shows loading once the key is accepted, polls it at once and offers to open the viewer", async () => {
    const launch = collectedLaunch();
    const popup = await acceptedPopup(100, launch);
    assert.deepEqual(popup.host.sent, [{ v: 1, op: "status", key }]);
    assert.equal(
      checksText(popup),
      drawnChecks(collected(launch), {
        kind: "loading",
        notice: { kind: "none" },
      }),
    );
    assert.equal(popup.elements.primary.disabled, false);
    assert.equal(
      popup.elements.primary.textContent,
      catalogs.en.popup.primary.openViewer,
    );
  });

  it("shows the running steps with the time elapsed since the send and keeps polling", async () => {
    const launch = collectedLaunch();
    const popup = await acceptedPopup(100, launch);
    await popup.host.answer(statusReply(runningJob));
    popup.clock.advance(3 * elapsedRedrawMs);
    await settle();
    assert.equal(
      checksText(popup),
      drawnChecks(
        collected(launch),
        {
          kind: "job",
          job: { kind: "running", steps: runningSteps },
          notice: { kind: "none" },
        },
        "en",
        3 * elapsedRedrawMs,
      ),
    );
    assert.equal(popup.host.count("status"), 2);
    assert.equal(
      popup.elements.primary.textContent,
      catalogs.en.popup.primary.openViewer,
    );
  });

  it("shows a done article and stops polling", async () => {
    const launch = collectedLaunch();
    const popup = await acceptedPopup(100, launch);
    await popup.host.answer(statusReply(doneJob));
    popup.clock.advance(3 * pollIntervalMs);
    await settle();
    assert.equal(popup.host.count("status"), 1);
    assert.equal(
      checksText(popup),
      drawnChecks(collected(launch), {
        kind: "job",
        job: doneDisplay,
        notice: { kind: "none" },
      }),
    );
    assert.equal(popup.elements.primary.disabled, false);
    assert.equal(
      popup.elements.primary.textContent,
      catalogs.en.popup.primary.openViewer,
    );
  });

  it("keeps the shown job and polls again after a failed status exchange", async () => {
    const launch = collectedLaunch();
    const popup = await acceptedPopup(100, launch);
    await popup.host.answer(statusReply(runningJob));
    popup.clock.advance(pollIntervalMs);
    await settle();
    await popup.host.answer({ kind: "failed", lastError: "host gone" });
    assert.equal(
      checksText(popup),
      drawnChecks(
        collected(launch),
        {
          kind: "job",
          job: { kind: "running", steps: runningSteps },
          notice: { kind: "retrying", outcome: { kind: "host-failed" } },
        },
        "en",
        pollIntervalMs,
      ),
    );
    popup.clock.advance(pollIntervalMs);
    await settle();
    assert.equal(popup.host.count("status"), 3);
  });

  const startFailures: Exclude<
    StartOutcome,
    { kind: "accepted" } | { kind: "same-video" }
  >[] = [
    { kind: "rejected", reason: "too_many_requests", requestId },
    { kind: "not-sent", cause: "server-unavailable" },
    { kind: "no-response" },
    { kind: "host-failed" },
    { kind: "unexpected", check: "member-set" },
  ];
  for (const outcome of startFailures) {
    it(`shows a ${outcome.kind} start with the button disabled and sends nothing more`, async () => {
      const launch = collectedLaunch();
      const popup = shown(launch);
      await press(popup);
      await popup.worker.answer(startReply(outcome, 100));
      assert.equal(
        checksText(popup),
        drawnChecks(collected(launch), {
          kind: "start-failed",
          failure: outcome,
        }),
      );
      assert.equal(popup.elements.primary.disabled, true);
      await press(popup);
      popup.clock.advance(startResultDeadlineMs);
      await settle();
      assert.equal(popup.worker.sent.length, 1);
      assert.deepEqual(popup.host.sent, []);
      assert.deepEqual(popup.tabs.calls, []);
    });
  }

  const noResults: {
    name: string;
    run: (popup: Popup) => Promise<void>;
  }[] = [
    {
      name: "the worker leaves the request unanswered",
      run: (popup) => popup.worker.answer({ ok: false, error: "no receiver" }),
    },
    {
      name: "the reply fails its check",
      run: (popup) =>
        popup.worker.answer({
          ok: true,
          value: { outcome: accepted, extra: 1 },
        }),
    },
    {
      name: "no reply comes before the deadline",
      run: async (popup) => {
        popup.clock.advance(startResultDeadlineMs - 1);
        await settle();
        // Precondition: still sending just before the deadline.
        assert.equal(popup.elements.primary.disabled, true);
        assert.equal(
          checksText(popup),
          drawnChecks(collected(collectedLaunch()), { kind: "sending" }),
        );
        popup.clock.advance(1);
        await settle();
        // A reply after the deadline is ignored.
        await popup.worker.answer(startReply(accepted, 100));
      },
    },
  ];
  for (const { name, run } of noResults) {
    it(`shows that the result is unknown and polls nothing when ${name}`, async () => {
      const launch = collectedLaunch();
      const popup = shown(launch);
      await press(popup);
      await run(popup);
      assert.equal(
        checksText(popup),
        drawnChecks(collected(launch), {
          kind: "start-failed",
          failure: { kind: "no-result" },
        }),
      );
      assert.equal(popup.elements.primary.disabled, true);
      popup.clock.advance(pollIntervalMs);
      await settle();
      assert.deepEqual(popup.host.sent, []);
    });
  }
});

describe("popup start deadline", () => {
  it("ignores a reply that comes after the deadline when the timer runs late", async () => {
    const launch = collectedLaunch();
    const popup = shown(launch, "en", undefined, true);
    await press(popup);
    popup.clock.advance(startResultDeadlineMs);
    await popup.worker.answer(startReply(accepted, 100));
    assert.equal(
      checksText(popup),
      drawnChecks(collected(launch), {
        kind: "start-failed",
        failure: { kind: "no-result" },
      }),
    );
    assert.deepEqual(popup.host.sent, []);
  });
});

/**
 * Opens an endpoint of the popup's channel that answers each find for key
 * with present, from each of viewers, as the viewers showing it do.
 */
function viewersShowing(
  popup: Popup,
  viewers: readonly { tabId: number; windowId: number; sentAt: number }[],
): void {
  const opened = popup.channels.open();
  assert.ok(opened.ok);
  const peer = opened.value;
  peer.listen((data) => {
    const message: ViewerChannelMessage | undefined =
      parseViewerChannelMessage(data);
    if (message?.type !== "find" || message.key !== key) {
      return;
    }
    for (const viewer of viewers) {
      peer.post({ type: "present", queryId: message.queryId, ...viewer });
    }
  });
}

/** The finds the popup posted. */
function finds(popup: Popup): unknown[] {
  return popup.channels.posted.filter(
    (message) => (message as { type?: unknown }).type === "find",
  );
}

/** Presses open-viewer on a running job and lets the query window pass. */
async function openViewer(popup: Popup): Promise<void> {
  await press(popup);
  popup.clock.advance(1_000);
  await settle();
}

describe("open viewer", () => {
  it("opens a new viewer when the viewer that answered is gone before it is brought forward", async () => {
    const popup = await acceptedPopup();
    await popup.host.answer(statusReply(runningJob));
    // Tab 200 answers but is no longer open when the popup looks it up.
    viewersShowing(popup, [{ tabId: 200, windowId: 8, sentAt: 1 }]);
    await openViewer(popup);
    assert.deepEqual(
      popup.tabs.calls.map((call) => call.op),
      ["get", "create"],
    );
    assert.deepEqual(popup.focus.focused, [1]);
  });

  it("keeps the button usable when the new viewer tab cannot be opened", async () => {
    const popup = await acceptedPopup();
    await popup.host.answer(statusReply(runningJob));
    popup.tabs.fail("create");
    await openViewer(popup);
    assert.deepEqual(popup.focus.focused, []);
    // A second press tries again: the first one ended.
    await openViewer(popup);
    assert.equal(
      popup.tabs.calls.filter((call) => call.op === "create").length,
      2,
    );
  });

  it("brings nothing forward once a retry replaced the send it opens", async () => {
    const popup = await acceptedPopup();
    await popup.host.answer(statusReply(runningJob));
    // The open-viewer waits for the viewers' answers, from 1.5 s ...
    popup.clock.advance(1_500);
    await press(popup);
    assert.equal(finds(popup).length, 1);
    // ... while the next status, at 2 s, fails the job and retry is pressed.
    popup.clock.advance(500);
    await settle();
    await popup.host.answer(statusReply(failedJob));
    await press(popup);
    assert.equal(startRequests(popup).length, 2);
    popup.clock.advance(1_000);
    await settle();
    assert.deepEqual(popup.tabs.calls, []);
    assert.deepEqual(popup.focus.focused, []);
  });

  it("brings the tab that shows the send forward, even when another viewer answers for the key", async () => {
    const popup = await acceptedPopup(100);
    await popup.host.answer(statusReply(runningJob));
    popup.tabs.open.set(100, 9);
    popup.tabs.open.set(200, 8);
    viewersShowing(popup, [{ tabId: 200, windowId: 8, sentAt: 1 }]);
    await openViewer(popup);
    assert.deepEqual(popup.tabs.calls, [
      { op: "get", tabId: 100 },
      { op: "activate", tabId: 100 },
    ]);
    assert.deepEqual(popup.focus.focused, [9]);
    assert.deepEqual(finds(popup), []);
    assert.equal(popup.worker.sent.length, 1);
    assert.equal(popup.host.count("start"), 0);
  });

  it("brings forward the oldest viewer that answers for the key once the tab is gone", async () => {
    const popup = await acceptedPopup(100);
    await popup.host.answer(statusReply(doneJob));
    popup.tabs.open.set(200, 8);
    popup.tabs.open.set(201, 6);
    viewersShowing(popup, [
      { tabId: 201, windowId: 6, sentAt: 2 },
      { tabId: 200, windowId: 8, sentAt: 1 },
    ]);
    await openViewer(popup);
    assert.deepEqual(finds(popup), [{ type: "find", queryId, key }]);
    assert.deepEqual(popup.tabs.calls, [
      { op: "get", tabId: 100 },
      { op: "get", tabId: 200 },
      { op: "activate", tabId: 200 },
    ]);
    assert.deepEqual(popup.focus.focused, [8]);
    assert.equal(popup.worker.sent.length, 1);
    assert.equal(popup.host.count("start"), 0);
  });

  it("opens a new viewer of the key in front when no viewer answers, and uses it on the next press", async () => {
    const launch = collectedLaunch();
    // The worker could not open a tab, so its reply names none.
    const popup = await acceptedPopup(undefined, launch);
    await openViewer(popup);
    assert.equal(finds(popup).length, 1);
    const created = popup.tabs.calls.find((call) => call.op === "create");
    assert.ok(created?.op === "create");
    assert.equal(created.tab.active, true);
    const prefix = fakeIdentity.getURL("viewer.html");
    assert.ok(created.tab.url.startsWith(`${prefix}?`));
    const parsed = parseViewerUrl(
      created.tab.url.slice(prefix.length),
      "",
      sentAt,
    );
    assert.ok(parsed.ok);
    assert.deepEqual(parsed.params, {
      heading: "A video",
      url: watchUrl,
      sentAt,
      key,
    });
    // fakeTabs opens a tab without a window in window 1.
    assert.deepEqual(popup.focus.focused, [1]);
    assert.equal(popup.worker.sent.length, 1);

    await openViewer(popup);
    assert.deepEqual(popup.tabs.calls.slice(-2), [
      { op: "get", tabId: 100 },
      { op: "activate", tabId: 100 },
    ]);
    assert.equal(finds(popup).length, 1);
  });

  it("opens one viewer however often the button is pressed while it opens", async () => {
    const popup = await acceptedPopup();
    await press(popup);
    await press(popup);
    popup.clock.advance(1_000);
    await settle();
    assert.equal(finds(popup).length, 1);
    assert.equal(
      popup.tabs.calls.filter((call) => call.op === "create").length,
      1,
    );
  });
});

describe("retry", () => {
  const ends: { name: string; job: Json }[] = [
    { name: "failed", job: failedJob },
    { name: "lost", job: { state: "unknown" } },
  ];
  for (const { name, job } of ends) {
    it(`sends the same input once more from a new send time for a ${name} job`, async () => {
      const launch = collectedLaunch();
      const popup = await acceptedPopup(100, launch);
      await popup.host.answer(statusReply(job));
      assert.equal(
        popup.elements.primary.textContent,
        catalogs.en.popup.primary.retry,
      );
      assert.equal(popup.elements.primary.disabled, false);
      popup.clock.advance(5_000);
      await press(popup);
      await press(popup);
      const [first, second, ...rest] = startRequests(popup);
      assert.ok(first !== undefined && second !== undefined);
      assert.deepEqual(rest, []);
      assert.deepEqual(second.input, first.input);
      // An inquiry's retry is an inquiry again.
      assert.equal(second.mode, "inquiry");
      assert.deepEqual(second.viewer, {
        ...first.viewer,
        sentAt: sentAt + 5_000,
      });
      assert.equal(
        checksText(popup),
        drawnChecks(collected(launch), { kind: "sending" }),
      );
      await popup.worker.answer(startReply(accepted, 101));
      assert.equal(popup.host.count("status"), 2);
      assert.equal(popup.host.count("start"), 0);
    });
  }
});

/** The existing article's key: never the key an accepted send gets. */
const existingKey = "key-existing" as ArticleKey;
const sameVideo: StartOutcome = {
  kind: "same-video",
  key: existingKey,
  requestId,
};

/** A popup pressed once whose inquiry got a same-video answer with the viewer in tab viewerTabId. */
async function choicePopup(
  viewerTabId?: number,
  launch = collectedLaunch(),
  locale: Locale = "en",
): Promise<Popup> {
  const popup = shown(launch, locale);
  await press(popup);
  await popup.worker.answer(startReply(sameVideo, viewerTabId));
  return popup;
}

/** The sentences the popup's status region holds. */
function statusSentences(popup: Popup): (string | null)[] {
  return Array.from(
    popup.elements.status.children,
    (child) => child.textContent,
  );
}

describe("popup choice", () => {
  for (const locale of localeList) {
    it(`shows the choice in place of the checks in ${locale}, keeping the video section and the details, and follows the existing article`, async () => {
      const launch = await runPopupLaunch(
        popupHarness({
          metadataReader: metadataReader(
            metadataJson({ title: "A video title", channelName: "A channel" }),
          ).reader,
        }).deps,
      );
      const popup = shown(launch, locale);
      const sections = [popup.elements.video, popup.elements.details];
      const before = sections.map((section) => section.cloneNode(true));
      const doc = popup.elements.primary.ownerDocument;
      await press(popup);
      await popup.worker.answer(startReply(sameVideo, 100));
      const catalog = catalogs[locale];
      const steps: [ExistingState, Json | undefined][] = [
        ["loading", undefined],
        ["running", runningJob],
        ["done", doneJob],
      ];
      for (const [existing, job] of steps) {
        if (job !== undefined) {
          popup.clock.advance(pollIntervalMs);
          await settle();
          await popup.host.answer(statusReply(job));
        }
        assert.equal(
          checksText(popup),
          drawnChecks(
            collected(launch),
            { kind: "choice", existing, notice: { kind: "none" } },
            locale,
          ),
          existing,
        );
        assert.ok(checksText(popup).includes(catalog.popup.choice.text));
        assert.ok(
          checksText(popup).includes(catalog.popup.existingStates[existing]),
        );
        // The summary tells the choice and the article's state without
        // moving the focus.
        assert.deepEqual(
          statusSentences(popup),
          [catalog.popup.choice.text, catalog.popup.existingStates[existing]],
          existing,
        );
        assert.equal(doc.activeElement, doc.body, existing);
        assert.equal(
          popup.elements.primary.textContent,
          catalog.popup.primary.openExisting,
        );
        assert.equal(popup.elements.primary.disabled, false);
        assert.equal(popup.elements.secondary.hidden, false);
        assert.equal(
          popup.elements.secondary.textContent,
          catalog.popup.secondary,
        );
        sections.forEach((section, index) => {
          assert.ok(section.isEqualNode(before[index] ?? null), section.id);
        });
      }
    });
  }

  it("keeps polling the existing article and sends no other start while no button is pressed", async () => {
    const popup = await choicePopup(100);
    for (let i = 0; i < 3; i += 1) {
      await popup.host.answer(statusReply(runningJob));
      popup.clock.advance(pollIntervalMs);
      await settle();
    }
    // The positive mark: the existing article's key is still polled.
    assert.equal(popup.host.count("status"), 4);
    assert.deepEqual(popup.host.sent.at(-1), {
      v: 1,
      op: "status",
      key: existingKey,
    });
    assert.equal(popup.worker.sent.length, 1);
    assert.equal(popup.host.count("start"), 0);
  });

  it("brings the tab that shows the send forward for open-existing, sends nothing and keeps the choice", async () => {
    const popup = await choicePopup(100);
    await popup.host.answer(statusReply(doneJob));
    popup.tabs.open.set(100, 9);
    await openViewer(popup);
    assert.deepEqual(popup.tabs.calls, [
      { op: "get", tabId: 100 },
      { op: "activate", tabId: 100 },
    ]);
    assert.deepEqual(popup.focus.focused, [9]);
    assert.deepEqual(finds(popup), []);
    assert.equal(popup.worker.sent.length, 1);
    assert.equal(popup.host.count("start"), 0);
    assert.equal(
      popup.elements.primary.textContent,
      catalogs.en.popup.primary.openExisting,
    );
    assert.equal(popup.elements.secondary.hidden, false);
  });

  it("opens a new existing-article viewer stamped with the press time when no viewer answers", async () => {
    const popup = await choicePopup();
    await popup.host.answer(statusReply(runningJob));
    // Pressed later than the send, so the press time differs from it.
    popup.clock.advance(7_000);
    const pressedAt = popup.clock.now();
    await openViewer(popup);
    assert.equal(finds(popup).length, 1);
    const created = popup.tabs.calls.find((call) => call.op === "create");
    assert.ok(created?.op === "create");
    assert.equal(created.tab.active, true);
    const prefix = fakeIdentity.getURL("viewer.html");
    assert.ok(created.tab.url.startsWith(`${prefix}?`));
    const parsed = parseViewerUrl(
      created.tab.url.slice(prefix.length),
      "",
      pressedAt,
    );
    assert.ok(parsed.ok);
    assert.deepEqual(parsed.params, {
      heading: "A video",
      url: watchUrl,
      sentAt: pressedAt,
      key: existingKey,
      existing: true,
    });
    assert.deepEqual(popup.focus.focused, [1]);
    assert.equal(popup.worker.sent.length, 1);
    assert.equal(popup.host.count("start"), 0);
    assert.equal(
      popup.elements.primary.textContent,
      catalogs.en.popup.primary.openExisting,
    );
    assert.equal(popup.elements.secondary.hidden, false);
  });

  it("sends one plain start of the same input from the secondary button and shows the job states", async () => {
    const launch = collectedLaunch();
    const popup = await choicePopup(100, launch);
    // A running existing article keeps its poll going until the press stops it.
    await popup.host.answer(statusReply(runningJob));
    popup.tabs.open.set(100, 9);
    popup.clock.advance(5_000);
    await settle();
    const existingPolls = (): number =>
      popup.host.sent.filter(
        (message) => (message as { key?: unknown }).key === existingKey,
      ).length;
    // Precondition: the existing article was polled again before the press.
    assert.equal(existingPolls(), 2);
    await press(popup, popup.elements.secondary);
    await press(popup, popup.elements.secondary);
    const [first, second, ...rest] = startRequests(popup);
    assert.ok(first !== undefined && second !== undefined);
    assert.deepEqual(rest, []);
    assert.equal(first.mode, "inquiry");
    assert.equal(second.mode, "plain");
    assert.deepEqual(second.input, first.input);
    assert.deepEqual(second.viewer, {
      ...first.viewer,
      sentAt: sentAt + 5_000,
    });
    assert.equal(
      checksText(popup),
      drawnChecks(collected(launch), { kind: "sending" }),
    );
    assert.equal(popup.elements.secondary.hidden, true);
    await popup.worker.answer(startReply(accepted, 101));
    assert.equal(
      checksText(popup),
      drawnChecks(collected(launch), {
        kind: "loading",
        notice: { kind: "none" },
      }),
    );
    // The plain send's key is polled, and the existing article's no longer:
    // the host answers in order, the existing article's open poll first.
    assert.deepEqual(popup.host.sent.at(-1), { v: 1, op: "status", key });
    await popup.host.answer(statusReply(runningJob));
    await popup.host.answer(statusReply(runningJob));
    popup.clock.advance(3 * pollIntervalMs);
    await settle();
    assert.equal(existingPolls(), 2);
    // The existing article's tab is left alone.
    assert.deepEqual(popup.tabs.calls, []);
    assert.equal(popup.host.count("start"), 0);
  });

  it("lets the plain send's open-viewer act while the existing article's open-viewer it replaced still waits", async () => {
    const popup = await choicePopup();
    await popup.host.answer(statusReply(runningJob));
    // Open-existing looks for the existing article's viewers for 1 s ...
    await press(popup);
    assert.equal(finds(popup).length, 1);
    // ... while the plain send replaces it and is accepted.
    await press(popup, popup.elements.secondary);
    await popup.worker.answer(startReply(accepted));
    await press(popup);
    assert.equal(finds(popup).length, 2);
    popup.clock.advance(1_000);
    await settle();
    // Only the plain send's viewer opens: the replaced open gave up.
    const created = popup.tabs.calls.filter((call) => call.op === "create");
    assert.equal(created.length, 1);
    const prefix = fakeIdentity.getURL("viewer.html");
    const [only] = created;
    assert.ok(only?.op === "create");
    const parsed = parseViewerUrl(
      only.tab.url.slice(prefix.length),
      "",
      sentAt,
    );
    assert.ok(parsed.ok);
    assert.equal("key" in parsed.params ? parsed.params.key : undefined, key);
  });

  const ends: { name: string; job: Json }[] = [
    { name: "failed", job: failedJob },
    { name: "lost", job: { state: "unknown" } },
  ];
  for (const { name, job } of ends) {
    it(`ends the choice when the existing article is ${name}, stops polling, and sends one inquiry on the next press`, async () => {
      const launch = collectedLaunch();
      const popup = await choicePopup(100, launch);
      await popup.host.answer(statusReply(job));
      assert.equal(
        checksText(popup),
        drawnChecks(collected(launch), { kind: "choice-ended" }),
      );
      assert.deepEqual(statusSentences(popup), [
        catalogs.en.popup.choiceEnded.text,
      ]);
      assert.equal(
        popup.elements.primary.textContent,
        catalogs.en.popup.primary.generate,
      );
      assert.equal(popup.elements.primary.disabled, false);
      assert.equal(popup.elements.secondary.hidden, true);
      popup.clock.advance(3 * pollIntervalMs);
      await settle();
      assert.equal(popup.host.count("status"), 1);
      await press(popup);
      await press(popup);
      const requests = startRequests(popup);
      assert.equal(requests.length, 2);
      assert.equal(requests[1]?.mode, "inquiry");
      assert.deepEqual(requests[1]?.input, requests[0]?.input);
    });
  }

  it("keeps the choice and both buttons beside a notice and polls again after an exchange that may succeed later", async () => {
    const launch = collectedLaunch();
    const popup = await choicePopup(100, launch);
    await popup.host.answer(statusReply(runningJob));
    popup.clock.advance(pollIntervalMs);
    await settle();
    await popup.host.answer({ kind: "failed", lastError: "host gone" });
    const notice: PollNotice = {
      kind: "retrying",
      outcome: { kind: "host-failed" },
    };
    assert.equal(
      checksText(popup),
      drawnChecks(collected(launch), {
        kind: "choice",
        existing: "running",
        notice,
      }),
    );
    assert.deepEqual(statusSentences(popup), [
      catalogs.en.popup.choice.text,
      catalogs.en.popup.existingStates.running,
      statusNoticeMessage(notice.outcome, "en").text,
    ]);
    assert.equal(popup.elements.primary.disabled, false);
    assert.equal(popup.elements.secondary.hidden, false);
    popup.clock.advance(pollIntervalMs);
    await settle();
    assert.equal(popup.host.count("status"), 3);
  });

  it("keeps the choice and both buttons beside a notice and stops polling after an exchange that cannot succeed", async () => {
    const launch = collectedLaunch();
    const popup = await choicePopup(100, launch);
    await popup.host.answer(responseReply(200, "{"));
    const notice: PollNotice = {
      kind: "stopped",
      outcome: { kind: "unexpected", check: "syntax" },
    };
    assert.equal(
      checksText(popup),
      drawnChecks(collected(launch), {
        kind: "choice",
        existing: "loading",
        notice,
      }),
    );
    assert.equal(popup.elements.primary.disabled, false);
    assert.equal(popup.elements.secondary.hidden, false);
    popup.clock.advance(3 * pollIntervalMs);
    await settle();
    assert.equal(popup.host.count("status"), 1);
  });
});

describe("popup choice focus", () => {
  it("gives the focus back to the primary button once the choice makes it pressable", async () => {
    const popup = shown(collectedLaunch());
    const doc = popup.elements.primary.ownerDocument;
    popup.elements.primary.focus();
    await pressWith(popup, "keyboard");
    assert.equal(popup.elements.primary.disabled, true);
    dropFocus(doc);
    await popup.worker.answer(startReply(sameVideo, 100));
    assert.equal(doc.activeElement, popup.elements.primary);
  });

  it("moves the focus from the secondary button to the primary button when the choice ends", async () => {
    const popup = await choicePopup(100);
    const doc = popup.elements.primary.ownerDocument;
    popup.elements.secondary.focus();
    assert.equal(doc.activeElement, popup.elements.secondary);
    await popup.host.answer(statusReply(failedJob));
    assert.equal(popup.elements.secondary.hidden, true);
    assert.equal(doc.activeElement, popup.elements.primary);
  });

  it("leaves the focus on the secondary button while the choice stays", async () => {
    const popup = await choicePopup(100);
    const doc = popup.elements.primary.ownerDocument;
    popup.elements.secondary.focus();
    await popup.host.answer(statusReply(doneJob));
    assert.equal(doc.activeElement, popup.elements.secondary);
  });

  it("gives the focus to the primary button after a keyboard press of the secondary button", async () => {
    const popup = await choicePopup(100);
    const doc = popup.elements.primary.ownerDocument;
    await popup.host.answer(statusReply(runningJob));
    popup.elements.secondary.focus();
    await pressWith(popup, "keyboard", popup.elements.secondary);
    assert.equal(popup.elements.secondary.hidden, true);
    dropFocus(doc);
    await popup.worker.answer(startReply(accepted, 101));
    assert.equal(doc.activeElement, popup.elements.primary);
  });
});

describe("retry modes", () => {
  it("retries a plain send in plain mode, again after its own retry fails", async () => {
    const popup = await choicePopup(100);
    // The host answers in order: first the existing article's poll, so the
    // next status answered is the plain send's.
    await popup.host.answer(statusReply(runningJob));
    await press(popup, popup.elements.secondary);
    await popup.worker.answer(startReply(accepted, 101));
    await popup.host.answer(statusReply(failedJob));
    await press(popup);
    await popup.worker.answer(startReply(accepted, 102));
    await popup.host.answer(statusReply({ state: "unknown" }));
    await press(popup);
    assert.deepEqual(
      startRequests(popup).map((request) => request.mode),
      ["inquiry", "plain", "plain", "plain"],
    );
  });
});

describe("popup status log", () => {
  /** A popup whose one status exchange got reply; its log and page. */
  async function statusOnce(reply: NativeReply): Promise<Popup> {
    const popup = await acceptedPopup(100);
    await popup.host.answer(reply);
    return popup;
  }

  /** The data of the one status log entry, at info or at error. */
  function statusLog(popup: Popup): Record<string, unknown> {
    const entries = [...popup.logger.infos, ...popup.logger.errors].filter(
      (entry) => entry.label === statusLabel,
    );
    assert.equal(entries.length, 1);
    return entries[0]?.data as Record<string, unknown>;
  }

  it("logs a status result with its decision and where it came from, at info", async () => {
    const popup = await statusOnce(statusReply(doneJob));
    assert.deepEqual(
      popup.logger.infos.filter((entry) => entry.label === statusLabel),
      [
        {
          label: statusLabel,
          data: {
            result: {
              kind: "ok",
              requestId,
              job: {
                state: "done",
                steps: doneSteps,
                costKnown: true,
              },
            },
            decision: "stop",
            earlierEpoch: false,
            visible: false,
          },
        },
      ],
    );
  });

  it("logs the browser's error text of a failed status exchange at error but does not display it", async () => {
    const lastError = "LASTERROR-TEXT-4c1d";
    const popup = await statusOnce({ kind: "failed", lastError });
    assert.deepEqual(statusLog(popup)["result"], {
      kind: "host-failed",
      lastError,
    });
    assert.ok(popup.logger.errors.some((entry) => entry.label === statusLabel));
    assert.equal(
      popup.elements.checks.ownerDocument.body.textContent?.includes(lastError),
      false,
    );
  });

  it("logs an unexpected status reply's code but does not display it", async () => {
    // The server's answer to a wrong path: a reason the extension does not show.
    const popup = await statusOnce(
      responseReply(
        404,
        JSON.stringify({ status: "rejected", reason: "not_found", requestId }),
      ),
    );
    assert.deepEqual(statusLog(popup)["result"], {
      kind: "unexpected",
      // not_found is no reason of a status query, so its value fails.
      check: "value-shape",
      status: 404,
    });
    assert.ok(popup.logger.errors.some((entry) => entry.label === statusLabel));
    assert.equal(
      popup.elements.checks.ownerDocument.body.textContent?.includes("404"),
      false,
    );
  });

  it("logs none of the selection, the tab title, the key, the article or the video's values", async () => {
    const markers = {
      selection: "SELECTION-MARKER-27bc",
      tabTitle: "TAB-TITLE-MARKER-5f20",
      key: "KEY-MARKER-61f0",
      markdown: "ARTICLE-MARKER-0b7e",
      title: "TITLE-MARKER-3c55",
      channelName: "CHANNEL-MARKER-9e12",
      description: "DESCRIPTION-MARKER-a4d8",
    };
    // One recorder for the whole popup session, the launch included.
    const harness = popupHarness({
      reader: readerReturning(page(markers.selection)),
      activeTab: async () => fakeTab({ title: markers.tabTitle }),
      metadataReader: metadataReader(
        metadataJson({
          title: markers.title,
          channelName: markers.channelName,
          description: markers.description,
        }),
      ).reader,
    });
    const launch = await runPopupLaunch(harness.deps);
    const popup = shown(launch, "en", harness.recorder);
    await press(popup);
    await popup.worker.answer(
      startReply(
        { kind: "accepted", key: markers.key as ArticleKey, requestId },
        100,
      ),
    );
    await popup.host.answer(
      statusReply({
        ...doneJob,
        article: { ...article, markdown: markers.markdown },
      }),
    );
    // Precondition: the start and the status went through, and were logged.
    assert.deepEqual(popup.host.sent, [
      { v: 1, op: "status", key: markers.key },
    ]);
    assert.equal(statusLog(popup)["decision"], "stop");
    const serialized = JSON.stringify([
      harness.recorder.infos,
      harness.recorder.errors,
    ]);
    for (const value of [...Object.values(markers), watchUrl]) {
      assert.equal(serialized.includes(value), false, value);
    }
  });
});

describe("logs by locale", () => {
  /** Every log entry of one launch and one send, with only the locale set. */
  async function logsIn(locale: Locale): Promise<RecordingLogger> {
    const recorder = recordingLogger();
    const values = { title: "t", channelName: "c", description: "d" };
    const launch = await runPopupLaunch(
      popupHarness({
        metadataReader: metadataReader(metadataJson(values, true)).reader,
        log: recorder.log,
      }).deps,
    );
    const popup = shown(launch, locale, recorder);
    await press(popup);
    await popup.worker.answer(startReply(accepted, 100));
    await popup.host.answer(statusReply(runningJob));
    return recorder;
  }

  it("records the same labels and values in every locale", async () => {
    const english = await logsIn("en");
    // Precondition: the launch, metadata and send logs were all written.
    assert.deepEqual(
      english.infos.map((entry) => entry.label),
      ["yt2column popup launch", metadataLabel, statusLabel],
    );
    for (const locale of otherLocales("en")) {
      const other = await logsIn(locale);
      assert.deepEqual(other.infos, english.infos, locale);
    }
  });
});

describe("video metadata log", () => {
  /** The popup's launch, send and blocked log entries for one read result. */
  async function metadataLogs(reader: MetadataReader): Promise<{
    popup: RecordingLogger;
    send: RecordingLogger;
    blocked: RecordingLogger;
  }> {
    const popup = popupHarness({ metadataReader: reader });
    const launch = await runPopupLaunch(popup.deps);
    // The send carries the metadata too: one sent and one blocked run.
    const send = recordingLogger();
    const sent = shown(launch, "en", send);
    await press(sent);
    await sent.worker.answer(startReply(accepted, 100));
    await sent.host.answer(statusReply(doneJob));
    // The same metadata with a selection over the body limit blocks the send.
    const blocked = popupHarness({
      metadataReader: reader,
      reader: readerReturning(page("a".repeat(maxBodyBytes))),
    });
    await runPopupLaunch(blocked.deps);
    // Precondition: both the send and the blocked logs were written.
    assert.ok(send.infos.some((entry) => entry.label === statusLabel));
    assert.ok(
      blocked.recorder.infos.some((entry) => entry.label === blockedLabel),
    );
    return { popup: popup.recorder, send, blocked: blocked.recorder };
  }

  const infoCases: {
    name: string;
    text: string;
    expected: Record<string, unknown>;
  }[] = [
    {
      name: "each omitted field and its reason",
      text: metadataJson(
        { title: "kept", channelName: " 　", description: "a\uD800b" },
        true,
      ),
      expected: {
        accepted: ["title"],
        collapsed: true,
        omissions: [
          { field: "channelName", reason: "blank" },
          { field: "description", reason: "unpaired-surrogate" },
        ],
      },
    },
    {
      name: "only the accepted fields when nothing is omitted",
      text: metadataJson({ title: "t", channelName: "c", description: "d" }),
      expected: {
        accepted: ["title", "channelName", "description"],
        collapsed: false,
      },
    },
  ];
  for (const { name, text, expected } of infoCases) {
    it(`records ${name} at info`, async () => {
      const { popup } = await metadataLogs(metadataReader(text).reader);
      assert.deepEqual(
        popup.infos.filter((entry) => entry.label === metadataLabel),
        [{ label: metadataLabel, data: expected }],
      );
      assert.equal(popup.errors.length, 0);
    });
  }

  it("keeps the read values and the read error out of the logs", async () => {
    // Every branch of the read gets identifiable page text: accepted values,
    // omitted values, a rejected shape, and a rejection error.
    const values = {
      title: "VALUE-TITLE-2b8e",
      channelName: "VALUE-CHANNEL-c41f",
      description: "VALUE-DESCRIPTION-77a0",
    };
    const omittedSurrogate = "OMITTED-SURROGATE-5e19";
    const rejectedShape = "REJECTED-SHAPE-a3d4";
    const readError = "ERROR-PAGE-TEXT-0d6b";
    const runs = await Promise.all([
      metadataLogs(metadataReader(metadataJson(values, true)).reader),
      metadataLogs(
        metadataReader(
          metadataJson({ description: `${omittedSurrogate}\uD800` }),
        ).reader,
      ),
      metadataLogs(
        metadataReader(
          JSON.stringify({
            title: rejectedShape,
            descriptionCollapsed: false,
            unknown: 1,
          }),
        ).reader,
      ),
      metadataLogs(rejectingMetadataReader(new Error(readError))),
    ]);
    const serialized = JSON.stringify(
      runs.flatMap((run) =>
        [run.popup, run.send, run.blocked].map((recorder) => [
          recorder.infos,
          recorder.errors,
        ]),
      ),
      (_key, value: unknown) =>
        value instanceof Error ? `${value.name}: ${value.message}` : value,
    );
    for (const value of [
      ...Object.values(values),
      omittedSurrogate,
      rejectedShape,
      readError,
    ]) {
      assert.equal(serialized.includes(value), false, value);
    }
  });
});

describe("startPopup", () => {
  /** A copy of the shipped popup.html with the given initial lang. */
  function popupMarkup(lang: string): Document {
    const doc = popupDocument();
    doc.documentElement.lang = lang;
    return doc;
  }

  /** Start dependencies over fake services, with one recording log. */
  function startDeps(
    i18n: I18nApi,
    log: Logger,
    overrides: {
      launch?: Partial<PopupLaunchDeps>;
    } = {},
  ): PopupStartDeps {
    return {
      uiLanguage: () => readUiLanguage(i18n),
      launch: {
        reader: readerReturning(page("selection")),
        metadataReader: metadataReader(metadataJson({})).reader,
        activeTab: async () => fakeTab(),
        log,
        ...overrides.launch,
      },
      send: {
        sendToWorker: pendingWorker().send,
        send: {
          sendNative: pendingHost().sendNative,
          schedule: fakeClock().schedule,
        },
        tabs: fakeTabs(),
        focusWindow: fakeFocus().focus,
        openChannel: fakeChannels().open,
        queryId: () => queryId,
        identity: fakeIdentity,
        now: () => sentAt,
        log,
      },
    };
  }

  /** The skeleton's containers that are shown. */
  function shownContainers(doc: Document): string[] {
    return containerIds.filter(
      (id) => doc.getElementById(id)?.hidden === false,
    );
  }

  it("shows nothing and keeps the primary button disabled until the metadata read ends", async () => {
    const recorder = recordingLogger();
    const doc = popupDocument();
    const deferred = deferredMetadataReader();
    const tabTitle = "TAB-TITLE-30be";
    const videoTitle = "VIDEO-TITLE-8d41";
    const started = startPopup(
      doc,
      startDeps({ getUILanguage: () => "en-US" }, recorder.log, {
        launch: {
          metadataReader: deferred.reader,
          activeTab: async () => fakeTab({ title: tabTitle }),
        },
      }),
    );
    await settle();
    // Precondition: the launch path is waiting on the read.
    assert.ok(recorder.infos.some((entry) => entry.label === launchLabel));
    assert.deepEqual(shownContainers(doc), []);
    assert.equal(popupElements(doc).primary.disabled, true);
    assert.equal(doc.body.textContent?.includes(tabTitle), false);

    deferred.resolve(metadataJson({ title: videoTitle }));
    await started;
    assert.deepEqual(shownContainers(doc), [
      "video",
      "checks",
      "action",
      "details",
    ]);
    assert.equal(popupElements(doc).primary.disabled, false);
    assert.equal(doc.querySelector("#video h1")?.textContent, videoTitle);
    // Opening the popup moves no focus, so Enter cannot send before a look.
    assert.equal(doc.activeElement, doc.body);
  });

  it("displays Japanese after reading a Japanese UI language", async () => {
    const recorder = recordingLogger();
    const doc = popupDocument();
    await startPopup(
      doc,
      startDeps({ getUILanguage: () => "ja-JP" }, recorder.log),
    );
    // The shipped document says lang="en", so only the entry point can make it ja.
    assert.equal(doc.documentElement.lang, "ja");
    const primary = popupElements(doc).primary;
    assert.equal(primary.textContent, catalogs.ja.popup.primary.generate);
    expectAbsent(
      doc.body,
      [
        catalogs.en.popup.primary.generate,
        ...Object.values(catalogs.en.checkNames),
      ],
      [
        catalogs.ja.popup.primary.generate,
        ...Object.values(catalogs.ja.checkNames),
      ],
    );
    assert.ok(doc.body.textContent?.includes(catalogs.ja.checkNames.selection));
  });

  it("falls back to English, as for en-US, when the UI language cannot be read", async () => {
    /**
     * One start over a fresh Japanese document: only the entry point can set
     * lang="en", so the attribute checks the fallback rather than the fixture.
     */
    async function run(i18n: I18nApi) {
      const recorder = recordingLogger();
      const doc = popupMarkup("ja");
      await startPopup(doc, startDeps(i18n, recorder.log));
      return { doc, recorder };
    }
    const unreadable = await run({
      getUILanguage: () => {
        throw new Error("i18n unavailable");
      },
    });
    const english = await run({ getUILanguage: () => "en-US" });
    for (const { doc } of [unreadable, english]) {
      assert.equal(doc.documentElement.lang, "en");
    }
    // The unreadable run shows English, not just the same as the en-US run.
    assert.equal(
      popupElements(unreadable.doc).primary.textContent,
      catalogs.en.popup.primary.generate,
    );
    assert.deepEqual(loggedOutcome(unreadable.recorder), { kind: "collected" });
    // Both runs behave alike; the en-US run is the reference the plan names.
    assert.equal(unreadable.doc.body.textContent, english.doc.body.textContent);
    assert.deepEqual(
      loggedOutcome(unreadable.recorder),
      loggedOutcome(english.recorder),
    );
  });

  it("sets the page language before the first render", async () => {
    const recorder = recordingLogger();
    const doc = popupMarkup("en");
    const seen: string[] = [];
    await startPopup(
      doc,
      startDeps({ getUILanguage: () => "ja-JP" }, recorder.log, {
        launch: {
          activeTab: async () => {
            seen.push(doc.documentElement.lang);
            return fakeTab();
          },
        },
      }),
    );
    assert.deepEqual(seen, ["ja"]);
  });

  /**
   * Replaces the element with id by a div with the same id and hidden state,
   * so only its kind is wrong.
   */
  function replaceWithDiv(doc: Document, id: string): void {
    const element = doc.getElementById(id);
    assert.ok(element !== null);
    const div = doc.createElement("div");
    div.id = id;
    div.hidden = element.hasAttribute("hidden");
    element.replaceWith(div);
  }

  const brokenSkeletons: { name: string; break: (doc: Document) => void }[] = [
    ...skeletonIds.map((id) => ({
      name: `#${id} is missing`,
      break: (doc: Document) => {
        const element = doc.getElementById(id);
        assert.ok(element !== null);
        // Unwrap rather than remove, so a container's children (the button
        // and the status region inside #action) stay and only id is missing.
        element.replaceWith(...element.childNodes);
        for (const other of skeletonIds) {
          assert.equal(doc.getElementById(other) === null, other === id, other);
        }
      },
    })),
    {
      name: "#primary is not a button",
      break: (doc: Document) => replaceWithDiv(doc, "primary"),
    },
    {
      name: "#secondary is not a button",
      break: (doc: Document) => replaceWithDiv(doc, "secondary"),
    },
    {
      name: "#details is not a details element",
      break: (doc: Document) => replaceWithDiv(doc, "details"),
    },
  ];
  for (const { name, break: breakSkeleton } of brokenSkeletons) {
    it(`sets the page language and runs no path or render when ${name}`, async () => {
      const recorder = recordingLogger();
      const doc = popupDocument();
      breakSkeleton(doc);
      // Resolving rather than rejecting also rules out a missing element
      // reaching the render.
      await startPopup(
        doc,
        startDeps({ getUILanguage: () => "ja-JP" }, recorder.log),
      );
      assert.equal(doc.documentElement.lang, "ja");
      assert.deepEqual(
        recorder.infos.map((entry) => entry.label),
        [uiLanguageLabel],
      );
      assert.deepEqual(shownContainers(doc), []);
      assert.equal(doc.body.textContent?.trim(), "");
    });
  }

  const loggedValues: { name: string; i18n: I18nApi; data: unknown }[] = [
    { name: "as read", i18n: { getUILanguage: () => "ja-JP" }, data: "ja-JP" },
    {
      name: "as undefined when unreadable",
      i18n: {
        getUILanguage: () => {
          throw new Error("i18n unavailable");
        },
      },
      data: undefined,
    },
  ];
  for (const { name, i18n, data } of loggedValues) {
    it(`logs the UI language once, ${name}`, async () => {
      const recorder = recordingLogger();
      await startPopup(popupMarkup("en"), startDeps(i18n, recorder.log));
      assert.deepEqual(
        recorder.infos.filter((entry) => entry.label === uiLanguageLabel),
        [{ label: uiLanguageLabel, data }],
      );
    });
  }
});
