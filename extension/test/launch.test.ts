import assert from "node:assert/strict";
import { describe, it } from "node:test";

import { readUiLanguage } from "../src/browser/chromeDeps.ts";
import type { BrowserCall, I18nApi } from "../src/browser/chromeDeps.ts";
import type {
  CollectedInput,
  PageSelection,
  SelectionReader,
} from "../src/core/collect.ts";
import { catalogs } from "../src/core/i18n/catalog.ts";
import { locales } from "../src/core/i18n/locale.ts";
import type { Locale } from "../src/core/i18n/locale.ts";
import {
  parseJobChannelMessage,
  parseSwRequest,
} from "../src/core/jobMessages.ts";
import type { SourceTab, SwRequest } from "../src/core/jobMessages.ts";
import { jobItemName } from "../src/core/jobRecord.ts";
import type { JobRecord } from "../src/core/jobRecord.ts";
import { parseJobViewerUrl } from "../src/core/jobViewerUrl.ts";
import { elapsedRedrawMs } from "../src/core/jobView.ts";
import { acceptMetadata } from "../src/core/metadata.ts";
import type { MetadataResult } from "../src/core/metadata.ts";
import { maxBodyBytes, prepareSend } from "../src/core/prepare.ts";
import {
  buildCollectedView,
  metadataDisplayLimit,
} from "../src/core/popupView.ts";
import type {
  CollectedView,
  PopupJobState,
  PopupView,
} from "../src/core/popupView.ts";
import { startFailureMessage } from "../src/core/sendMessages.ts";
import type { Logger } from "../src/core/types.ts";
import {
  launchContextFromTab,
  runPopupLaunch,
  runPopupSend,
  startPopup,
  startReplyWaitMs,
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
  fakeSession,
  fakeTabs,
  pendingSw,
  type FakeChannels,
  type FakeFocus,
  type FakeSession,
  type FakeTabs,
  type PendingSw,
} from "./helpers/chrome.ts";
import { fakeClock, settle, type FakeClock } from "./helpers/clock.ts";
import {
  expectAbsent,
  otherLocales,
  popupDocument,
  popupElements,
} from "./helpers/dom.ts";
import { recordingLogger } from "./helpers/logger.ts";
import type { RecordingLogger } from "./helpers/logger.ts";
import {
  doneRecord,
  failedRecord,
  jobId,
  runningRecord,
  stoppedRecord,
} from "./helpers/record.ts";

const watchUrl = "https://www.youtube.com/watch?v=abc";
const malicious = "<img src=x onerror=alert(1)>";
const launchLabel = "yt2column popup launch";
const uiLanguageLabel = "yt2column ui language";
const metadataLabel = "yt2column video metadata";
const blockedLabel = "yt2column blocked";
const activeTabLabel = "yt2column active tab";
const protectLabel = "yt2column settings protection";
const recordLabel = "yt2column job record";
const localeList = Object.keys(locales) as Locale[];
/** The tab fakeTab is: the selection's source, where the viewer opens next to. */

/** The tab fakeTab is: the selection's source, where the viewer opens next to. */
const source: SourceTab = { tabId: 7, windowId: 3, index: 4 };
const sentAt = 1_700_000_000_000;
const queryId = "0".repeat(31) + "1";
const otherJob = "fedcba9876543210fedcba9876543210";

/** The skeleton's ids, one per element startPopup needs. */
const skeletonIds = [
  "build-mark",
  "video",
  "checks",
  "rejection",
  "action",
  "setup",
  "primary",
  "secondary",
  "status",
  "details",
  "options",
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
    protectSettings: async () => ({ ok: true, value: undefined }),
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
  readonly sw: PendingSw;
  readonly session: FakeSession;
  readonly tabs: FakeTabs;
  readonly focus: FakeFocus;
  readonly channels: FakeChannels;
  readonly logger: RecordingLogger;
  /** How many times the options page was opened. */
  readonly options: { opened: number };
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
  // Wraps the session area's read, as a test that orders it against an event does.
  wrapRead: (read: FakeSession["read"]) => FakeSession["read"] = (read) => read,
): Popup {
  const elements = popupElements();
  const clock = fakeClock(sentAt);
  const sw = pendingSw();
  const session = fakeSession();
  const tabs = fakeTabs();
  const focus = fakeFocus();
  const channels = fakeChannels();
  const logger = log ?? recordingLogger();
  const options = { opened: 0 };
  renderPopup(elements, launch.view, locale);
  runPopupSend(elements, launch, {
    sendToSw: sw.send,
    readSession: wrapRead(session.read),
    onSessionChanged: session.onChanged,
    openChannel: channels.open,
    queryId: () => queryId,
    schedule: lateTimers ? () => () => undefined : clock.schedule,
    tabs,
    focusWindow: focus.focus,
    identity: fakeIdentity,
    openOptions: async () => {
      options.opened += 1;
      return { ok: true, value: undefined };
    },
    now: clock.now,
    log: logger.log,
    locale,
  });
  return {
    elements,
    clock,
    sw,
    session,
    tabs,
    focus,
    channels,
    logger,
    options,
  };
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
function startRequests(popup: Popup): Extract<SwRequest, { type: "start" }>[] {
  return popup.sw.sent.filter(
    (request): request is Extract<SwRequest, { type: "start" }> =>
      request.type === "start",
  );
}

const ok = <T>(value: T): BrowserCall<T> => ({ ok: true, value });
const accepted = ok({ status: "accepted", jobId });
const sameVideo = ok({ status: "same_video", jobId: otherJob });

/** Puts record into the session area as the worker writes it, which announces it. */
async function store(popup: Popup, record: JobRecord): Promise<void> {
  await popup.session.write({ [jobItemName(record.jobId)]: record });
  await settle();
}

/** A popup pressed once whose start was accepted, with its record running. */
async function acceptedPopup(
  launch = collectedLaunch(),
  locale: Locale = "en",
): Promise<Popup> {
  const popup = shown(launch, locale);
  await press(popup);
  popup.session.items.set(jobItemName(jobId), runningRecord());
  await popup.sw.answer(accepted);
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

/** The status region's sentences. */
function statusSentences(popup: Popup): (string | null)[] {
  return Array.from(
    popup.elements.status.children,
    (child) => child.textContent,
  );
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

describe("settings protection", () => {
  it("keeps the settings from the page before it injects anything", async () => {
    const order: string[] = [];
    const harness = popupHarness({
      protectSettings: async () => {
        order.push("protect");
        return { ok: true, value: undefined };
      },
      reader: {
        read: async () => {
          order.push("read");
          return page("selection");
        },
      },
      metadataReader: {
        read: async () => {
          order.push("metadata");
          return metadataJson({});
        },
      },
    });
    await runPopupLaunch(harness.deps);
    assert.deepEqual(order, ["protect", "read", "metadata"]);
  });

  it("injects nothing and shows a refused launch when the settings cannot be protected", async () => {
    const injected: string[] = [];
    const harness = popupHarness({
      protectSettings: async () => ({ ok: false, error: "ERROR-TEXT-77d1" }),
      reader: {
        read: async () => {
          injected.push("read");
          return page("selection");
        },
      },
      metadataReader: {
        read: async () => {
          injected.push("metadata");
          return metadataJson({});
        },
      },
    });
    const launch = await runPopupLaunch(harness.deps);
    assert.deepEqual(injected, []);
    assert.deepEqual(launch, {
      view: { kind: "rejected", reason: "settings-unprotected" },
    });
    assert.deepEqual(harness.recorder.errors, [
      { label: protectLabel, data: { set: false } },
    ]);
    // The rejected view can send nothing.
    const popup = shown(launch);
    assert.equal(popup.elements.primary.disabled, true);
    await press(popup);
    assert.deepEqual(popup.sw.sent, []);
    assert.equal(
      JSON.stringify(harness.recorder.errors).includes("ERROR-TEXT-77d1"),
      false,
    );
  });
});

describe("popup job send", () => {
  it("makes the primary button pressable for a sendable selection and sends nothing until it is pressed", async () => {
    const popup = shown(collectedLaunch());
    assert.equal(popup.elements.primary.disabled, false);
    await settle();
    assert.deepEqual(popup.sw.sent, []);
    assert.deepEqual(popup.session.calls, []);
    assert.deepEqual(popup.tabs.calls, []);
    assert.deepEqual(popup.channels.posted, []);
    assert.equal(popup.clock.pending(), 0);
  });

  it("asks the worker for one inquiry start that the worker's check accepts, and sends nothing else", async () => {
    const popup = shown(collectedLaunch());
    await press(popup);
    await press(popup);
    assert.deepEqual(popup.sw.sent, [
      {
        type: "start",
        mode: "inquiry",
        input: { videoUrl: watchUrl, transcript: "selection" },
        source,
      },
    ]);
    const [request] = popup.sw.sent;
    assert.equal(parseSwRequest(request).ok, true);
  });

  it("sends the full video title, not the heading cut for display", async () => {
    const title = "t".repeat(1_001);
    const launch = await runPopupLaunch(
      popupHarness({
        metadataReader: metadataReader(metadataJson({ title })).reader,
      }).deps,
    );
    const popup = shown(launch);
    // Precondition: the heading is cut for display, so the check below tells
    // the sent value apart from the displayed one.
    assert.equal(
      popup.elements.video.querySelector("h1")?.textContent,
      `${"t".repeat(metadataDisplayLimit)}…`,
    );
    await press(popup);
    assert.equal(startRequests(popup)[0]?.input.title, title);
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
      assert.deepEqual(popup.sw.sent, []);
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
    assert.deepEqual(popup.sw.sent, []);
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
      popup.session.items.set(jobItemName(jobId), runningRecord());
      await popup.sw.answer(accepted);
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
      await popup.sw.answer(accepted);
      assert.equal(popup.elements.primary.disabled, false);
      assert.equal(doc.activeElement, expected(popup));
    });
  }

  it("gives the focus back to the button after a retry", async () => {
    const popup = await acceptedPopup();
    await store(popup, failedRecord("llm_failed"));
    const doc = popup.elements.primary.ownerDocument;
    popup.elements.primary.focus();
    await pressWith(popup, "keyboard");
    assert.equal(popup.elements.primary.disabled, true);
    dropFocus(doc);
    await popup.sw.answer(accepted);
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
    await popup.sw.answer(accepted);
    assert.equal(popup.elements.primary.disabled, false);
    assert.equal(doc.activeElement, doc.body);
  });

  it("shows loading once the job is accepted, then the record it reads, and offers to open the viewer", async () => {
    const launch = collectedLaunch();
    const popup = shown(launch);
    await press(popup);
    // The read is held until the loading state is checked.
    popup.session.fail("read");
    await popup.sw.answer(accepted);
    assert.equal(
      checksText(popup),
      drawnChecks(collected(launch), { kind: "loading" }),
    );
    assert.equal(popup.elements.primary.disabled, false);
    assert.equal(
      popup.elements.primary.textContent,
      catalogs.en.popup.primary.openViewer,
    );
    // A failed read keeps the state and is logged, without a value.
    assert.deepEqual(popup.logger.errors, [
      { label: recordLabel, data: { read: false } },
    ]);
    // The record's change event still reaches the popup.
    await store(popup, runningRecord());
    assert.equal(
      checksText(popup),
      drawnChecks(
        collected(launch),
        { kind: "job", job: { kind: "running", steps: runningRecord().steps } },
        "en",
      ),
    );
  });

  it("shows the running steps with the time elapsed since the press, redrawn each second", async () => {
    const launch = collectedLaunch();
    const popup = await acceptedPopup(launch);
    const running: PopupJobState = {
      kind: "job",
      job: { kind: "running", steps: runningRecord().steps },
    };
    assert.equal(
      checksText(popup),
      drawnChecks(collected(launch), running, "en", 0),
    );
    popup.clock.advance(elapsedRedrawMs * 65);
    assert.equal(
      checksText(popup),
      drawnChecks(collected(launch), running, "en", 65_000),
    );
  });

  const ended: {
    name: string;
    record: () => JobRecord | undefined;
    state: PopupJobState;
  }[] = [
    {
      name: "a done article",
      record: () => doneRecord(),
      state: {
        kind: "job",
        job: {
          kind: "done",
          steps: doneRecord().steps,
          article: (doneRecord() as Extract<JobRecord, { state: "done" }>)
            .article,
        },
      },
    },
    {
      name: "a failed article",
      record: () => failedRecord("llm_failed"),
      state: {
        kind: "job",
        job: {
          kind: "failed",
          steps: failedRecord("llm_failed").steps,
          failure: "llm_failed",
        },
      },
    },
    {
      name: "a missing API key after the reservation",
      record: () => failedRecord("api_key_missing"),
      state: {
        kind: "job",
        job: { kind: "setup", failure: "api_key_missing" },
      },
    },
    {
      name: "a rejected key",
      record: () => failedRecord("api_key_rejected"),
      state: {
        kind: "job",
        job: {
          kind: "key-rejected",
          steps: failedRecord("api_key_rejected").steps,
        },
      },
    },
    {
      name: "a stopped run",
      record: () => stoppedRecord(),
      state: {
        kind: "job",
        job: { kind: "stopped", steps: stoppedRecord().steps },
      },
    },
    {
      name: "a removed record",
      record: () => undefined,
      state: { kind: "job", job: { kind: "lost" } },
    },
  ];
  for (const { name, record, state } of ended) {
    it(`shows ${name} from the record's change event and stops the elapsed time's redraw`, async () => {
      const launch = collectedLaunch();
      const popup = await acceptedPopup(launch);
      const next = record();
      if (next === undefined) {
        await popup.session.remove([jobItemName(jobId)]);
        await settle();
      } else {
        await store(popup, next);
      }
      assert.equal(checksText(popup), drawnChecks(collected(launch), state));
      assert.equal(popup.clock.pending(), 0);
    });
  }

  it("follows only its own job's record", async () => {
    const popup = await acceptedPopup();
    const before = checksText(popup);
    await store(popup, { ...doneRecord(), jobId: otherJob });
    assert.equal(checksText(popup), before);
  });

  it("shows a record that is not one of the shapes as lost, never repaired", async () => {
    const launch = collectedLaunch();
    const popup = await acceptedPopup(launch);
    await popup.session.write({
      [jobItemName(jobId)]: { ...runningRecord(), state: "paused" },
    });
    await settle();
    assert.equal(
      checksText(popup),
      drawnChecks(collected(launch), { kind: "job", job: { kind: "lost" } }),
    );
  });

  it("keeps a change event that came before the first read's result", async () => {
    const launch = collectedLaunch();
    // The read takes the record as it is, and answers only when released.
    let release = (): void => {};
    const popup = shown(launch, "en", undefined, false, (read) => (names) => {
      const answer = read(names);
      return new Promise((resolve) => {
        release = () => void answer.then(resolve);
      });
    });
    await press(popup);
    popup.session.items.set(jobItemName(jobId), runningRecord());
    await popup.sw.answer(accepted);
    // The change event arrives before the read's older record.
    await store(popup, doneRecord());
    release();
    await settle();
    assert.equal(
      checksText(popup),
      drawnChecks(collected(launch), {
        kind: "job",
        job: {
          kind: "done",
          steps: doneRecord().steps,
          article: (doneRecord() as Extract<JobRecord, { state: "done" }>)
            .article,
        },
      }),
    );
  });

  const rejections = ["invalid_body", "too_many_requests", "internal"] as const;
  for (const reason of rejections) {
    it(`shows a ${reason} refusal with the button disabled and sends nothing more`, async () => {
      const launch = collectedLaunch();
      const popup = shown(launch);
      await press(popup);
      await popup.sw.answer(ok({ status: "rejected", reason }));
      const state: PopupJobState = {
        kind: "start-failed",
        failure: { kind: "rejected", reason },
      };
      assert.equal(checksText(popup), drawnChecks(collected(launch), state));
      assert.equal(popup.elements.primary.disabled, true);
      assert.equal(popup.elements.setup.hidden, true);
      await press(popup);
      assert.equal(startRequests(popup).length, 1);
      assert.deepEqual(popup.session.calls, []);
    });
  }

  it("shows a missing API key with the setup operation, which opens the options page", async () => {
    const launch = collectedLaunch();
    const popup = shown(launch);
    await press(popup);
    await popup.sw.answer(
      ok({ status: "rejected", reason: "api_key_missing" }),
    );
    const state: PopupJobState = {
      kind: "start-failed",
      failure: { kind: "rejected", reason: "api_key_missing" },
    };
    assert.equal(checksText(popup), drawnChecks(collected(launch), state));
    assert.deepEqual(statusSentences(popup), [
      catalogs.en.setup.api_key_missing.text,
    ]);
    assert.equal(popup.elements.setup.hidden, false);
    assert.equal(popup.elements.primary.disabled, true);
    await press(popup, popup.elements.setup);
    assert.equal(popup.options.opened, 1);
    // Opening the options page sends nothing and opens no viewer.
    assert.equal(startRequests(popup).length, 1);
    assert.deepEqual(popup.tabs.calls, []);
  });

  const unknowns: {
    name: string;
    reply: (popup: Popup) => Promise<void>;
  }[] = [
    {
      name: "the worker answers unknown",
      reply: (popup) => popup.sw.answer(ok({ status: "unknown" })),
    },
    {
      name: "the message fails",
      reply: (popup) => popup.sw.answer({ ok: false, error: "closed" }),
    },
    {
      name: "the reply is not a start reply",
      reply: (popup) =>
        popup.sw.answer(ok({ status: "accepted", jobId: "not-an-id" })),
    },
    {
      name: "no reply comes before the deadline",
      reply: async (popup) => {
        popup.clock.advance(startReplyWaitMs);
        await settle();
      },
    },
  ];
  for (const { name, reply } of unknowns) {
    it(`shows that the result is unknown and reads no record when ${name}`, async () => {
      const launch = collectedLaunch();
      const popup = shown(launch);
      await press(popup);
      await reply(popup);
      assert.equal(
        checksText(popup),
        drawnChecks(collected(launch), {
          kind: "start-failed",
          failure: { kind: "unknown" },
        }),
      );
      assert.deepEqual(statusSentences(popup), [
        startFailureMessage({ kind: "unknown" }, "en").text,
      ]);
      assert.deepEqual(popup.session.calls, []);
      assert.equal(popup.elements.primary.disabled, true);
    });
  }

  it("ignores a reply that comes after the deadline", async () => {
    const launch = collectedLaunch();
    const popup = shown(launch);
    await press(popup);
    popup.clock.advance(startReplyWaitMs);
    await settle();
    await popup.sw.answer(accepted);
    assert.deepEqual(popup.session.calls, []);
    assert.equal(
      checksText(popup),
      drawnChecks(collected(launch), {
        kind: "start-failed",
        failure: { kind: "unknown" },
      }),
    );
  });
});

/** Answers the next find query on the channel with the viewers in tabIds. */
function viewersShowing(
  popup: Popup,
  shownJob: string,
  places: readonly { tabId: number; windowId: number }[],
): void {
  const opened = popup.channels.open();
  assert.ok(opened.ok);
  const channel = opened.value;
  channel.listen((data) => {
    const message = parseJobChannelMessage(data);
    if (message?.type === "find" && message.jobId === shownJob) {
      for (const place of places) {
        channel.post({ type: "present", queryId: message.queryId, ...place });
      }
    }
  });
}

/** Presses the primary button and lets the query window end. */
async function openViewerPress(
  popup: Popup,
  button: HTMLButtonElement = popup.elements.primary,
): Promise<void> {
  await press(popup, button);
  popup.clock.advance(1_000);
  await settle();
}

/** The job viewer URL of a created tab, parsed. */
function createdViewer(popup: Popup): ReturnType<typeof parseJobViewerUrl> {
  const created = popup.tabs.calls.find((call) => call.op === "create");
  assert.ok(created !== undefined && created.op === "create");
  const prefix = fakeIdentity.getURL("viewer.html");
  assert.ok(created.tab.url.startsWith(`${prefix}?`), created.tab.url);
  assert.equal(created.tab.active, true);
  return parseJobViewerUrl(created.tab.url.slice(prefix.length), "");
}

describe("open viewer", () => {
  it("brings forward the viewer with the smallest tab ID among those that show the job", async () => {
    const popup = await acceptedPopup();
    popup.tabs.open.set(30, 5);
    popup.tabs.open.set(20, 6);
    viewersShowing(popup, jobId, [
      { tabId: 30, windowId: 5 },
      { tabId: 20, windowId: 6 },
    ]);
    await openViewerPress(popup);
    assert.deepEqual(
      popup.tabs.calls.filter((call) => call.op !== "get"),
      [{ op: "activate", tabId: 20 }],
    );
    assert.deepEqual(popup.focus.focused, [6]);
    assert.equal(startRequests(popup).length, 1);
  });

  it("opens a new viewer of the job in front when no viewer answers", async () => {
    const popup = await acceptedPopup();
    await openViewerPress(popup);
    assert.deepEqual(createdViewer(popup), {
      ok: true,
      params: {
        target: { kind: "job", jobId },
        videoUrl: watchUrl,
        title: watchUrl,
      },
    });
    assert.deepEqual(popup.focus.focused, [1]);
  });

  it("opens a new viewer when the viewer that answered is gone before it is brought forward", async () => {
    const popup = await acceptedPopup();
    viewersShowing(popup, jobId, [{ tabId: 30, windowId: 5 }]);
    await openViewerPress(popup);
    assert.equal(createdViewer(popup).ok, true);
  });

  it("opens one viewer however often the button is pressed while it opens", async () => {
    const popup = await acceptedPopup();
    await press(popup);
    await press(popup);
    popup.clock.advance(1_000);
    await settle();
    assert.equal(
      popup.tabs.calls.filter((call) => call.op === "create").length,
      1,
    );
  });

  it("keeps the button usable when the new viewer tab cannot be opened", async () => {
    const popup = await acceptedPopup();
    popup.tabs.fail("create");
    await openViewerPress(popup);
    popup.tabs.calls.length = 0;
    await openViewerPress(popup);
    assert.equal(
      popup.tabs.calls.filter((call) => call.op === "create").length,
      1,
    );
  });

  it("brings nothing forward once a retry replaced the send it opens", async () => {
    const popup = await acceptedPopup();
    await store(popup, runningRecord());
    await press(popup);
    // The record fails while the query runs; the retry starts a new send.
    await store(popup, failedRecord("llm_failed"));
    await press(popup);
    popup.clock.advance(1_000);
    await settle();
    assert.deepEqual(popup.tabs.calls, []);
    assert.deepEqual(popup.focus.focused, []);
  });
});

describe("retry", () => {
  for (const [name, record] of [
    ["failed", () => failedRecord("llm_failed")],
    ["stopped", () => stoppedRecord()],
    ["key-rejected", () => failedRecord("api_key_rejected")],
  ] as const) {
    it(`sends the same input once more in the same mode for a ${name} job, and follows the new reply`, async () => {
      const popup = await acceptedPopup();
      await store(popup, record());
      await press(popup);
      const [first, second] = startRequests(popup);
      assert.ok(first !== undefined && second !== undefined);
      assert.deepEqual(second, first);
      assert.equal(popup.elements.primary.disabled, true);
      await popup.sw.answer(accepted);
      await store(popup, runningRecord());
      assert.equal(
        popup.elements.primary.textContent,
        catalogs.en.popup.primary.openViewer,
      );
    });
  }

  it("offers the setup operation and a retry for a job that needed a setting", async () => {
    const popup = await acceptedPopup();
    await store(popup, failedRecord("settings_invalid"));
    assert.equal(popup.elements.setup.hidden, false);
    assert.equal(
      popup.elements.primary.textContent,
      catalogs.en.popup.primary.retry,
    );
    await press(popup, popup.elements.setup);
    assert.equal(popup.options.opened, 1);
  });

  it("stops following the replaced job's record once the retry is sent", async () => {
    const popup = await acceptedPopup();
    await store(popup, failedRecord("llm_failed"));
    await press(popup);
    const sending = checksText(popup);
    await store(popup, doneRecord());
    assert.equal(checksText(popup), sending);
  });
});

describe("popup choice", () => {
  /** The other job's record in state. */
  function other(record: JobRecord): JobRecord {
    return { ...record, jobId: otherJob };
  }

  /**
   * A popup pressed once whose start was answered with the same video's job,
   * whose record is running, as a same-video answer's record is.
   */
  async function choicePopup(locale: Locale = "en"): Promise<Popup> {
    const popup = shown(collectedLaunch(), locale);
    await press(popup);
    popup.session.items.set(jobItemName(otherJob), other(runningRecord()));
    await popup.sw.answer(sameVideo);
    return popup;
  }

  for (const locale of localeList) {
    it(`shows the choice in place of the checks in ${locale} and follows the existing article`, async () => {
      const launch = collectedLaunch();
      const popup = shown(launch, locale);
      await press(popup);
      popup.session.items.set(jobItemName(otherJob), other(runningRecord()));
      await popup.sw.answer(sameVideo);
      assert.equal(
        checksText(popup),
        drawnChecks(
          collected(launch),
          { kind: "choice", existing: "running" },
          locale,
        ),
      );
      await store(popup, other(doneRecord()));
      assert.equal(
        checksText(popup),
        drawnChecks(
          collected(launch),
          { kind: "choice", existing: "done" },
          locale,
        ),
      );
      assert.equal(popup.elements.secondary.hidden, false);
    });
  }

  it("brings the viewer of the existing article forward for open-existing, and sends nothing", async () => {
    const popup = await choicePopup();
    popup.tabs.open.set(40, 2);
    viewersShowing(popup, otherJob, [{ tabId: 40, windowId: 2 }]);
    await openViewerPress(popup);
    assert.deepEqual(
      popup.tabs.calls.filter((call) => call.op !== "get"),
      [{ op: "activate", tabId: 40 }],
    );
    assert.equal(startRequests(popup).length, 1);
  });

  it("opens a new viewer marked as an existing article when no viewer answers", async () => {
    const popup = await choicePopup();
    await openViewerPress(popup);
    assert.deepEqual(createdViewer(popup), {
      ok: true,
      params: {
        target: { kind: "job", jobId: otherJob, existing: true },
        videoUrl: watchUrl,
        title: watchUrl,
      },
    });
  });

  it("sends one plain start of the same input from the secondary button and shows the job states", async () => {
    const popup = await choicePopup();
    await press(popup, popup.elements.secondary);
    const [inquiry, plain] = startRequests(popup);
    assert.ok(inquiry !== undefined && plain !== undefined);
    assert.deepEqual(plain, { ...inquiry, mode: "plain" });
    popup.session.items.set(jobItemName(jobId), runningRecord());
    await popup.sw.answer(accepted);
    assert.equal(popup.elements.secondary.hidden, true);
    assert.equal(
      popup.elements.primary.textContent,
      catalogs.en.popup.primary.openViewer,
    );
  });

  for (const [name, change] of [
    [
      "failed",
      (popup: Popup) => store(popup, other(failedRecord("llm_failed"))),
    ],
    ["stopped", (popup: Popup) => store(popup, other(stoppedRecord()))],
    [
      "removed",
      async (popup: Popup) => {
        popup.session.items.set(jobItemName(otherJob), other(runningRecord()));
        await popup.session.remove([jobItemName(otherJob)]);
        await settle();
      },
    ],
  ] as const) {
    it(`ends the choice when the existing article is ${name}, and sends one inquiry on the next press`, async () => {
      const popup = await choicePopup();
      await change(popup);
      assert.equal(
        popup.elements.primary.textContent,
        catalogs.en.popup.primary.generate,
      );
      assert.equal(popup.elements.secondary.hidden, true);
      await store(popup, other(doneRecord()));
      // Ended: the existing article's later record changes nothing.
      assert.equal(popup.elements.secondary.hidden, true);
      await press(popup);
      assert.deepEqual(
        startRequests(popup).map((request) => request.mode),
        ["inquiry", "inquiry"],
      );
    });
  }

  it("retries a plain send in plain mode", async () => {
    const popup = await choicePopup();
    await press(popup, popup.elements.secondary);
    await popup.sw.answer(accepted);
    await store(popup, failedRecord("llm_failed"));
    await press(popup);
    assert.deepEqual(
      startRequests(popup).map((request) => request.mode),
      ["inquiry", "plain", "plain"],
    );
  });
});

describe("popup choice focus", () => {
  /** A popup showing the choice of a running existing article. */
  async function choiceShown(): Promise<Popup> {
    const popup = shown(collectedLaunch());
    await press(popup);
    popup.session.items.set(jobItemName(otherJob), {
      ...runningRecord(),
      jobId: otherJob,
    });
    await popup.sw.answer(sameVideo);
    return popup;
  }

  it("gives the focus back to the primary button once the choice makes it pressable", async () => {
    const popup = shown(collectedLaunch());
    const doc = popup.elements.primary.ownerDocument;
    popup.elements.primary.focus();
    await pressWith(popup, "keyboard");
    assert.equal(popup.elements.primary.disabled, true);
    dropFocus(doc);
    popup.session.items.set(jobItemName(otherJob), {
      ...runningRecord(),
      jobId: otherJob,
    });
    await popup.sw.answer(sameVideo);
    assert.equal(popup.elements.primary.disabled, false);
    assert.equal(doc.activeElement, popup.elements.primary);
  });

  it("moves the focus from the secondary button to the primary button when the choice ends", async () => {
    const popup = await choiceShown();
    const doc = popup.elements.primary.ownerDocument;
    popup.elements.secondary.focus();
    await store(popup, { ...failedRecord("llm_failed"), jobId: otherJob });
    assert.equal(popup.elements.secondary.hidden, true);
    assert.equal(doc.activeElement, popup.elements.primary);
  });

  it("leaves the focus on the secondary button while the choice stays", async () => {
    const popup = await choiceShown();
    const doc = popup.elements.primary.ownerDocument;
    popup.elements.secondary.focus();
    await store(popup, { ...doneRecord(), jobId: otherJob });
    assert.equal(popup.elements.secondary.hidden, false);
    assert.equal(doc.activeElement, popup.elements.secondary);
  });

  it("gives the focus to the primary button after a keyboard press of the secondary button", async () => {
    const popup = await choiceShown();
    const doc = popup.elements.primary.ownerDocument;
    popup.elements.secondary.focus();
    await pressWith(popup, "keyboard", popup.elements.secondary);
    dropFocus(doc);
    await popup.sw.answer(accepted);
    assert.equal(doc.activeElement, popup.elements.primary);
  });
});

describe("popup send log", () => {
  it("logs none of the selection, the tab title, the job ID, the article or the video's values", async () => {
    const markers = {
      selection: "SELECTION-MARKER-27bc",
      tabTitle: "TAB-TITLE-MARKER-5f20",
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
    popup.session.fail("read");
    await popup.sw.answer(accepted);
    const done = doneRecord() as Extract<JobRecord, { state: "done" }>;
    await store(popup, {
      ...done,
      article: { ...done.article, markdown: markers.markdown },
    });
    // Precondition: the read failure was logged.
    assert.ok(
      harness.recorder.errors.some((entry) => entry.label === recordLabel),
    );
    const serialized = JSON.stringify([
      harness.recorder.infos,
      harness.recorder.errors,
    ]);
    for (const value of [...Object.values(markers), watchUrl, jobId]) {
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
    popup.session.fail("read");
    await popup.sw.answer(accepted);
    return recorder;
  }

  it("records the same labels and values in every locale", async () => {
    const english = await logsIn("en");
    // Precondition: the launch, metadata and send logs were all written.
    assert.deepEqual(
      english.infos.map((entry) => entry.label),
      [launchLabel, metadataLabel],
    );
    assert.deepEqual(
      english.errors.map((entry) => entry.label),
      [recordLabel],
    );
    for (const locale of otherLocales("en")) {
      const other = await logsIn(locale);
      assert.deepEqual(other.infos, english.infos, locale);
      assert.deepEqual(other.errors, english.errors, locale);
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
    sent.session.fail("read");
    await sent.sw.answer(accepted);
    // The same metadata with a selection over the body limit blocks the send.
    const blocked = popupHarness({
      metadataReader: reader,
      reader: readerReturning(page("a".repeat(maxBodyBytes))),
    });
    await runPopupLaunch(blocked.deps);
    // Precondition: both the send and the blocked logs were written.
    assert.ok(send.errors.some((entry) => entry.label === recordLabel));
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
      buildKind?: PopupStartDeps["buildKind"];
      openOptions?: PopupStartDeps["send"]["openOptions"];
    } = {},
  ): PopupStartDeps {
    const session = fakeSession();
    return {
      uiLanguage: () => readUiLanguage(i18n),
      buildKind: overrides.buildKind ?? "release",
      launch: {
        protectSettings: async () => ({ ok: true, value: undefined }),
        reader: readerReturning(page("selection")),
        metadataReader: metadataReader(metadataJson({})).reader,
        activeTab: async () => fakeTab(),
        log,
        ...overrides.launch,
      },
      send: {
        sendToSw: pendingSw().send,
        readSession: session.read,
        onSessionChanged: session.onChanged,
        openChannel: fakeChannels().open,
        queryId: () => queryId,
        schedule: fakeClock().schedule,
        tabs: fakeTabs(),
        focusWindow: fakeFocus().focus,
        identity: fakeIdentity,
        openOptions:
          overrides.openOptions ??
          (async () => ({ ok: true, value: undefined })),
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
      name: "#setup is not a button",
      break: (doc: Document) => replaceWithDiv(doc, "setup"),
    },
    {
      name: "#options is not a button",
      break: (doc: Document) => replaceWithDiv(doc, "options"),
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

  it("shows the options operation, which opens the options page, and no build mark in a release build", async () => {
    const recorder = recordingLogger();
    const doc = popupDocument();
    let opened = 0;
    await startPopup(
      doc,
      startDeps({ getUILanguage: () => "en-US" }, recorder.log, {
        openOptions: async () => {
          opened += 1;
          return { ok: true, value: undefined };
        },
      }),
    );
    const elements = popupElements(doc);
    assert.equal(elements.options.hidden, false);
    assert.equal(elements.options.textContent, catalogs.en.openOptions.link);
    assert.equal(elements.buildMark.hidden, true);
    elements.options.click();
    await settle();
    assert.equal(opened, 1);
  });

  it("shows the options operation even when the launch is refused", async () => {
    const recorder = recordingLogger();
    const doc = popupDocument();
    await startPopup(
      doc,
      startDeps({ getUILanguage: () => "en-US" }, recorder.log, {
        launch: {
          activeTab: async () => fakeTab({ url: "https://example.com/" }),
        },
      }),
    );
    assert.equal(popupElements(doc).rejection.hidden, false);
    assert.equal(popupElements(doc).options.hidden, false);
  });

  it("shows the build mark in a development build", async () => {
    const recorder = recordingLogger();
    const doc = popupDocument();
    await startPopup(
      doc,
      startDeps({ getUILanguage: () => "ja-JP" }, recorder.log, {
        buildKind: "dev",
      }),
    );
    const mark = popupElements(doc).buildMark;
    assert.equal(mark.hidden, false);
    assert.equal(mark.textContent, catalogs.ja.buildMark);
  });
});
