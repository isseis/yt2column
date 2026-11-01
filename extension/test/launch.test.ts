import assert from "node:assert/strict";
import { describe, it } from "node:test";

import { readUiLanguage } from "../src/browser/chromeDeps.ts";
import type { I18nApi } from "../src/browser/chromeDeps.ts";
import type {
  CollectedInput,
  CollectOutcome,
  PageSelection,
  SelectionReader,
} from "../src/core/collect.ts";
import { catalogs } from "../src/core/i18n/catalog.ts";
import { locales } from "../src/core/i18n/locale.ts";
import type { Locale } from "../src/core/i18n/locale.ts";
import { acceptMetadata } from "../src/core/metadata.ts";
import type { MetadataResult } from "../src/core/metadata.ts";
import { nativeHostName } from "../src/core/native.ts";
import type { NativeReply } from "../src/core/native.ts";
import { maxBodyBytes, prepareSend } from "../src/core/prepare.ts";
import {
  buildCollectedView,
  metadataDisplayLimit,
} from "../src/core/popupView.ts";
import type { PopupView } from "../src/core/popupView.ts";
import type { SendOutcome } from "../src/core/articlesResponse.ts";
import type { RequestId } from "../src/core/response.ts";
import { sendOutcomeMessage } from "../src/core/sendMessages.ts";
import {
  launchContextFromTab,
  runPopupLaunch,
  runPopupSend,
  startPopup,
} from "../src/launch.ts";
import type {
  Logger,
  MetadataReader,
  PopupLaunchDeps,
  PopupSendDeps,
  PopupStartDeps,
} from "../src/launch.ts";
import { renderPopup } from "../src/ui/render.ts";
import type { PopupElements } from "../src/ui/render.ts";
import {
  expectAbsent,
  expectLocaleMessage,
  expectMessage,
  otherLocales,
  popupDocument,
  popupElements,
} from "./helpers/dom.ts";
import { recordingLogger } from "./helpers/logger.ts";
import type { RecordingLogger } from "./helpers/logger.ts";
import {
  deferredNative,
  recordingNative,
  responseReply,
  testSendDeps,
} from "./helpers/native.ts";

const watchUrl = "https://www.youtube.com/watch?v=abc";
const malicious = "<img src=x onerror=alert(1)>";
const launchLabel = "yt2column popup launch";
const uiLanguageLabel = "yt2column ui language";
const metadataLabel = "yt2column video metadata";
const blockedLabel = "yt2column blocked";
const sendLabel = "yt2column send result";
const requestId = "0123456789abcdef" as RequestId;
const acceptedBody = JSON.stringify({ status: "accepted", requestId });
const acceptedOutcome: SendOutcome = { kind: "accepted", requestId };
const localeList = Object.keys(locales) as Locale[];

/** The skeleton's ids, one per element startPopup needs. */
const skeletonIds = [
  "video",
  "checks",
  "rejection",
  "action",
  "primary",
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
    id: 7,
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

/** A collected outcome for the watch URL with the given selection. */
function collectedOutcome(selection = "selection"): CollectOutcome {
  return {
    kind: "collected",
    input: { selection, url: watchUrl, title: "A video" } as CollectedInput,
  };
}

/** A recording sendNative whose reply wraps an accepted response. */
function acceptedNative() {
  return recordingNative(async () => responseReply(202, acceptedBody));
}

/** A recording sendNative whose every call fails with lastError. */
function failingNative(lastError: string) {
  return recordingNative(async (): Promise<NativeReply> => ({
    kind: "failed",
    lastError,
  }));
}

/** Waits for the queued microtasks and one macrotask to settle. */
async function flush(): Promise<void> {
  await new Promise((resolve) => setTimeout(resolve, 0));
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

/** Send dependencies over sendNative, logging to log, in locale. */
function sendDeps(
  sendNative: Parameters<typeof testSendDeps>[0],
  locale: Locale = "en",
  log: Logger = recordingLogger().log,
): PopupSendDeps {
  return { send: testSendDeps(sendNative), now: () => 1000, log, locale };
}

/**
 * Draws view into a fresh copy of the shipped skeleton and runs the send path
 * on it, in the order startPopup does.
 */
function shown(view: PopupView, deps: PopupSendDeps): PopupElements {
  const elements = popupElements();
  renderPopup(elements, view, deps.locale);
  runPopupSend(elements, view, deps);
  return elements;
}

/** The outcome the launch path logged for the developer. */
function loggedOutcome(recorder: RecordingLogger): CollectOutcome {
  const entry = recorder.infos.find(
    (recorded) => recorded.label === launchLabel,
  );
  assert.ok(entry !== undefined, "expected the popup launch log");
  return (entry.data as { outcome: CollectOutcome }).outcome;
}

/**
 * Dispatches a click on the primary button and waits for the handler to
 * settle. Dispatching the event (rather than calling click()) exercises the
 * handler even when the button is disabled, so the state guard is tested and
 * not merely the disabled attribute.
 */
async function clickSend(elements: PopupElements): Promise<void> {
  const view = elements.primary.ownerDocument.defaultView;
  assert.ok(view !== null);
  elements.primary.dispatchEvent(new view.Event("click"));
  await flush();
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
    const view = await runPopupLaunch(popup.deps);

    assert.ok(view.kind === "collected");
    const logged = loggedOutcome(popup.recorder);
    assert.ok(logged.kind === "collected");
    assert.equal(logged.input.selection, selection);
    assert.equal(logged.input.title, title);
    // The view is built from the same input: the tab title stands in for the
    // missing video title, and the counts are the selection's.
    assert.equal(view.video.heading.value, title);
    assert.equal(view.selection.lineCount, 4);
  });

  it("sends the video title, not the tab title, in the body", async () => {
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

    const popupNative = acceptedNative();
    const view = await runPopupLaunch(
      popupHarness({
        reader,
        metadataReader: metadata,
        activeTab: async () => tab,
      }).deps,
    );
    const elements = shown(view, sendDeps(popupNative.sendNative));
    await clickSend(elements);

    assert.deepEqual(popupNative.messages, [
      {
        videoUrl: watchUrl,
        transcript: selection,
        ...values,
      },
    ]);
    assert.equal(
      elements.status.querySelector(".send-request-id")?.textContent,
      requestId,
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
      const view = await runPopupLaunch(
        popupHarness({
          reader,
          metadataReader: popupMetadata.reader,
          activeTab: async () => tab,
        }).deps,
      );
      assert.deepEqual(popupMetadata.calls, []);
      assert.deepEqual(view, { kind: "rejected", reason: name });
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
      const view = await runPopupLaunch(popup.deps);
      assert.equal(view.kind, "collected");
      const popupNative = acceptedNative();
      const elements = shown(view, sendDeps(popupNative.sendNative));
      // Without a video title the primary button still sends.
      assert.equal(elements.primary.disabled, false);
      await clickSend(elements);
      assert.deepEqual(popupNative.messages, [
        { videoUrl: watchUrl, transcript: "selection" },
      ]);

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
    const view = await runPopupLaunch(popup.deps);
    assert.deepEqual(view, { kind: "rejected", reason: "collection-failed" });
    const logged = loggedOutcome(popup.recorder);
    assert.equal(logged.kind, "rejected");
    assert.equal(logged.reason, "collection-failed");
  });

  it("treats a failing activeTab as collection-failed without rejecting", async () => {
    const popup = popupHarness({
      activeTab: async () => {
        throw new Error("query failed");
      },
    });
    const view = await runPopupLaunch(popup.deps);
    assert.deepEqual(view, { kind: "rejected", reason: "collection-failed" });
    assert.equal(popup.recorder.errors.length, 1);
  });

  it("keeps untrusted text out of the fixed log labels", async () => {
    const reader = readerReturning(page(`${malicious}\n`));
    const popup = popupHarness({ reader });
    await runPopupLaunch(popup.deps);
    assert.equal(popup.recorder.infos[0]?.label, launchLabel);
  });
});

describe("popup send", () => {
  for (const locale of localeList) {
    const deps = (
      sendNative: Parameters<typeof testSendDeps>[0],
    ): { deps: PopupSendDeps; recorder: RecordingLogger } => {
      const recorder = recordingLogger();
      return { deps: sendDeps(sendNative, locale, recorder.log), recorder };
    };

    it(`makes the primary button pressable for a sendable selection and does not send until it is pressed in ${locale}`, async () => {
      const native = acceptedNative();
      const elements = shown(collectedView(), deps(native.sendNative).deps);
      assert.equal(elements.primary.disabled, false);
      await flush();
      assert.equal(native.messages.length, 0);
    });

    it(`sends once, disables the button and shows the ${locale} result in the status region`, async () => {
      const native = acceptedNative();
      const { deps: popupDeps, recorder } = deps(native.sendNative);
      const elements = shown(collectedView(), popupDeps);
      await clickSend(elements);
      assert.equal(native.messages.length, 1);
      assert.equal(elements.primary.disabled, true);
      expectLocaleMessage(elements.status, locale, (l) =>
        sendOutcomeMessage(acceptedOutcome, l),
      );
      assert.equal(
        elements.status.querySelector(".send-request-id")?.textContent,
        requestId,
      );
      // An accepted send is normal: it is logged once at info and never at error.
      assert.equal(
        recorder.infos.filter((entry) => entry.label === sendLabel).length,
        1,
      );
      assert.equal(
        recorder.errors.some((entry) => entry.label === sendLabel),
        false,
      );
    });

    it(`sends only once when the button is clicked repeatedly in ${locale}`, async () => {
      const native = acceptedNative();
      const elements = shown(collectedView(), deps(native.sendNative).deps);
      await clickSend(elements);
      await clickSend(elements);
      assert.equal(native.messages.length, 1);
    });

    it(`shows the ${locale} sending notice in the status region and a disabled button while sending`, async () => {
      const deferred = deferredNative();
      const elements = shown(collectedView(), deps(deferred.sendNative).deps);
      elements.primary.click();
      await flush();
      assert.equal(elements.primary.disabled, true);
      expectLocaleMessage(elements.status, locale, (l) => catalogs[l].sending);
      if (locale === "en") {
        // The close guidance and the Slack warning are what the notice is
        // for, so assert the whole English sentences independently of the
        // catalog it renders.
        const text = elements.status.textContent ?? "";
        for (const sentence of [
          "Do not close this window until the result appears.",
          "If you close it, check Slack before sending again.",
        ]) {
          assert.ok(text.includes(sentence), sentence);
        }
      }
      deferred.resolve(responseReply(202, acceptedBody));
      await flush();
      assert.equal(
        elements.status.querySelector(".send-request-id")?.textContent,
        requestId,
      );
    });

    it(`keeps the video, checks and details unchanged by a ${locale} send`, async () => {
      const deferred = deferredNative();
      const view = collectedView(
        "selection",
        acceptMetadata({
          title: "A video title",
          channelName: "A channel",
          description: "A description",
          descriptionCollapsed: false,
        }),
      );
      const elements = shown(view, deps(deferred.sendNative).deps);
      const sections = [elements.video, elements.checks, elements.details];
      const before = sections.map((section) => section.cloneNode(true));
      elements.primary.click();
      await flush();
      // Precondition: the send has started, so the status region changed.
      assert.ok(elements.status.childNodes.length > 0);
      sections.forEach((section, index) => {
        assert.ok(section.isEqualNode(before[index] ?? null), section.id);
      });
      deferred.resolve(responseReply(202, acceptedBody));
      await flush();
      // Precondition: the result is shown.
      assert.ok(elements.status.querySelector(".send-request-id") !== null);
      sections.forEach((section, index) => {
        assert.ok(section.isEqualNode(before[index] ?? null), section.id);
      });
    });

    it(`keeps the button pressable under the ${locale} expand prompt and sends the shown description`, async () => {
      const native = acceptedNative();
      const view = await runPopupLaunch(
        popupHarness({
          metadataReader: metadataReader(
            metadataJson({ description: "first lines" }, true),
          ).reader,
        }).deps,
      );
      // Precondition: the launch reached the collapsed state; its display is
      // render.test.ts's.
      assert.ok(view.kind === "collected");
      assert.equal(view.description.status.kind, "collapsed");
      const elements = shown(view, deps(native.sendNative).deps);
      assert.equal(elements.primary.disabled, false);
      await clickSend(elements);
      assert.equal(native.messages.length, 1);
      const sent = native.messages[0];
      assert.ok(sent !== undefined && !("op" in sent));
      assert.equal(sent.description, "first lines");
    });

    it(`keeps the button pressable when no description is sent in ${locale}`, async () => {
      const native = acceptedNative();
      const view = await runPopupLaunch(popupHarness().deps);
      // Precondition: no description is sent; its display is render.test.ts's.
      assert.ok(view.kind === "collected");
      assert.equal(view.description.status.kind, "not-sent");
      const elements = shown(view, deps(native.sendNative).deps);
      assert.equal(elements.primary.disabled, false);
      await clickSend(elements);
      assert.deepEqual(native.messages, [
        { videoUrl: watchUrl, transcript: "selection" },
      ]);
    });

    it(`logs a host failure at error and shows the ${locale} host-failed result`, async () => {
      const native = failingNative(
        "Specified native messaging host not found.",
      );
      const { deps: popupDeps, recorder } = deps(native.sendNative);
      const elements = shown(collectedView(), popupDeps);
      await clickSend(elements);
      assert.equal(native.messages.length, 1);
      expectLocaleMessage(elements.status, locale, (l) =>
        sendOutcomeMessage({ kind: "host-failed" }, l),
      );
      assert.deepEqual(
        recorder.errors.map((entry) => entry.label),
        [sendLabel],
      );
      assert.equal(
        recorder.infos.some((entry) => entry.label === sendLabel),
        false,
      );
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
      it(`shows the ${locale} body-too-large reason and a disabled button without sending when ${when}`, async () => {
        const native = acceptedNative();
        const popup = popupHarness(launch);
        const view = await runPopupLaunch(popup.deps);
        // Precondition: the size check blocked the selection; its display is
        // render.test.ts's.
        assert.ok(view.kind === "collected");
        assert.deepEqual(view.selection.status, {
          kind: "blocked",
          reason: "body-too-large",
        });
        const elements = shown(view, deps(native.sendNative).deps);
        assert.equal(elements.primary.disabled, true);
        await clickSend(elements);
        assert.equal(native.messages.length, 0);
        assert.equal(elements.status.childNodes.length, 0);
        // A blocked launch is normal: logged at info, never at error.
        assert.deepEqual(
          popup.recorder.infos.filter((entry) => entry.label === blockedLabel),
          [{ label: blockedLabel, data: { reason: "body-too-large" } }],
        );
        assert.equal(
          popup.recorder.errors.some((entry) => entry.label === blockedLabel),
          false,
        );
      });
    }

    it(`shows the ${locale} primary button disabled without sending for a rejected collection`, async () => {
      const native = acceptedNative();
      const elements = shown(
        { kind: "rejected", reason: "empty-selection" },
        deps(native.sendNative).deps,
      );
      assert.equal(elements.primary.disabled, true);
      await clickSend(elements);
      assert.equal(native.messages.length, 0);
      assert.equal(elements.status.childNodes.length, 0);
    });
  }

  it("sends the full video title while the popup shows it truncated", async () => {
    const title = "t".repeat(metadataDisplayLimit + 1);
    const native = acceptedNative();
    const view = await runPopupLaunch(
      popupHarness({
        metadataReader: metadataReader(metadataJson({ title })).reader,
      }).deps,
    );
    const elements = shown(view, sendDeps(native.sendNative));
    // Precondition: the heading is cut for display, so the body check below
    // tells the sent value apart from the displayed one.
    assert.equal(
      elements.video.querySelector("h1")?.textContent,
      `${"t".repeat(metadataDisplayLimit)}\u2026`,
    );
    await clickSend(elements);
    const sent = native.messages[0];
    assert.ok(sent !== undefined && !("op" in sent));
    assert.equal(sent.title, title);
  });
});

describe("logs by locale", () => {
  /** Every log entry of one launch and one send, with only the locale set. */
  async function logsIn(locale: Locale): Promise<RecordingLogger> {
    const recorder = recordingLogger();
    const values = { title: "t", channelName: "c", description: "d" };
    const view = await runPopupLaunch(
      popupHarness({
        metadataReader: metadataReader(metadataJson(values, true)).reader,
        log: recorder.log,
      }).deps,
    );
    await clickSend(
      shown(view, sendDeps(acceptedNative().sendNative, locale, recorder.log)),
    );
    return recorder;
  }

  it("records the same labels and values in every locale", async () => {
    const english = await logsIn("en");
    // Precondition: the launch, metadata and send logs were all written.
    assert.deepEqual(
      english.infos.map((entry) => entry.label),
      ["yt2column popup launch", metadataLabel, sendLabel],
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
    const view = await runPopupLaunch(popup.deps);
    // The send carries the metadata too: one sent and one blocked run.
    const send = recordingLogger();
    await clickSend(
      shown(view, sendDeps(acceptedNative().sendNative, "en", send.log)),
    );
    // The same metadata with a selection over the body limit blocks the send.
    const blocked = popupHarness({
      metadataReader: reader,
      reader: readerReturning(page("a".repeat(maxBodyBytes))),
    });
    await runPopupLaunch(blocked.deps);
    // Precondition: both the send and the blocked logs were written.
    assert.ok(send.infos.some((entry) => entry.label === sendLabel));
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

describe("send log", () => {
  /** Sends once from a collected popup with sendNative; returns the log and the status region. */
  async function sendOnce(
    native: ReturnType<typeof recordingNative>,
  ): Promise<{ recorder: RecordingLogger; status: HTMLElement }> {
    const recorder = recordingLogger();
    const elements = shown(
      collectedView(),
      sendDeps(native.sendNative, "en", recorder.log),
    );
    await clickSend(elements);
    return { recorder, status: elements.status };
  }

  /** The data of the one send log entry, at info or at error. */
  function sendLog(recorder: RecordingLogger): Record<string, unknown> {
    const entries = [...recorder.infos, ...recorder.errors].filter(
      (entry) => entry.label === sendLabel,
    );
    assert.equal(entries.length, 1);
    return entries[0]?.data as Record<string, unknown>;
  }

  it("records the host name, and no port or token", async () => {
    const { recorder } = await sendOnce(acceptedNative());
    const data = sendLog(recorder);
    assert.deepEqual(Object.keys(data).sort(), [
      "at",
      "durationMs",
      "host",
      "result",
    ]);
    assert.equal(data["host"], nativeHostName);
  });

  it("logs the browser's error text but does not display it", async () => {
    const lastError = "LASTERROR-TEXT-4c1d";
    const { recorder, status } = await sendOnce(failingNative(lastError));
    assert.deepEqual(sendLog(recorder)["result"], {
      kind: "host-failed",
      lastError,
    });
    assert.equal(
      status.ownerDocument.body.textContent?.includes(lastError),
      false,
    );
  });

  it("logs an unexpected reply's status but does not display it", async () => {
    // The server's answer to a wrong path: a reason the extension does not show.
    const body = JSON.stringify({
      status: "rejected",
      reason: "not_found",
      requestId,
    });
    const { recorder, status } = await sendOnce(
      recordingNative(async () => responseReply(404, body)),
    );
    assert.deepEqual(sendLog(recorder)["result"], {
      kind: "unexpected",
      check: "status-pairing",
      status: 404,
    });
    expectMessage(
      status,
      sendOutcomeMessage({ kind: "unexpected", check: "status-pairing" }, "en"),
    );
    assert.equal(status.ownerDocument.body.textContent?.includes("404"), false);
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
        send: testSendDeps(acceptedNative().sendNative),
        now: () => 1000,
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
    await flush();
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
    assert.equal(primary.textContent, catalogs.ja.primaryButton);
    expectAbsent(
      doc.body,
      [catalogs.en.primaryButton, ...Object.values(catalogs.en.checkNames)],
      [catalogs.ja.primaryButton, ...Object.values(catalogs.ja.checkNames)],
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
      catalogs.en.primaryButton,
    );
    assert.deepEqual(loggedOutcome(unreadable.recorder), collectedOutcome());
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
