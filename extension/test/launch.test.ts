import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import path from "node:path";
import { describe, it } from "node:test";

import { JSDOM } from "jsdom";

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
import { expandPromptMessage, rejectionMessage } from "../src/core/messages.ts";
import { emptyMetadataResult } from "../src/core/metadata.ts";
import type { MetadataResult } from "../src/core/metadata.ts";
import { nativeHostName } from "../src/core/native.ts";
import type { NativeReply } from "../src/core/native.ts";
import { maxBodyBytes } from "../src/core/prepare.ts";
import { metadataDisplayLimit } from "../src/core/popupView.ts";
import type { RequestId, SendOutcome } from "../src/core/response.ts";
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
  PopupLaunchResult,
  PopupSendDeps,
  PopupStartDeps,
} from "../src/launch.ts";
import {
  container,
  expectAbsent,
  expectLocaleMessage,
  expectMessage,
  otherLocales,
  query,
  valueOf,
} from "./helpers/dom.ts";
import { recordingLogger } from "./helpers/logger.ts";
import type { RecordingLogger } from "./helpers/logger.ts";
import {
  deferredNative,
  recordingNative,
  responseReply,
  testSendDeps,
} from "./helpers/native.ts";
import { extensionDir } from "./helpers/paths.ts";

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

/** A collected outcome for the watch URL with the given selection. */
function collectedOutcome(selection = "selection"): CollectOutcome {
  return {
    kind: "collected",
    input: { selection, url: watchUrl, title: "A video" } as CollectedInput,
  };
}

/** A popup launch result; the metadata defaults to the empty result. */
function launched(
  outcome: CollectOutcome,
  metadata: MetadataResult = emptyMetadataResult(),
): PopupLaunchResult {
  return { outcome, metadata };
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
    locale: "en",
    ...overrides,
  };
  return { deps, recorder };
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
 * Dispatches a click on the popup send button and waits for the handler to
 * settle. dispatching the event (rather than calling click()) exercises the
 * handler even when the button is disabled, so the state guard is tested and
 * not merely the disabled attribute.
 */
async function clickSend(container: HTMLElement): Promise<void> {
  const button = container.querySelector(".send-button");
  assert.ok(button !== null);
  const view = container.ownerDocument.defaultView;
  assert.ok(view !== null);
  button.dispatchEvent(new view.Event("click"));
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
    const { outcome: popupOutcome } = await runPopupLaunch(
      container(),
      popup.deps,
    );

    assert.ok(popupOutcome.kind === "collected");
    assert.equal(popupOutcome.input.selection, selection);
    assert.equal(popupOutcome.input.title, title);
    assert.deepEqual(loggedOutcome(popup.recorder), popupOutcome);
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
    const outcome = await runPopupLaunch(
      container(),
      popupHarness({
        reader,
        metadataReader: metadata,
        activeTab: async () => tab,
      }).deps,
    );
    const popupRoot = container();
    await runPopupSend(popupRoot, outcome, {
      send: testSendDeps(popupNative.sendNative),
      now: () => 1000,
      log: recordingLogger().log,
      locale: "en",
    });
    await clickSend(popupRoot);

    assert.deepEqual(popupNative.messages, [
      {
        videoUrl: watchUrl,
        transcript: selection,
        ...values,
      },
    ]);
    assert.equal(
      popupRoot.querySelector(".send-request-id")?.textContent,
      requestId,
    );
  });

  it("reads the metadata once, from the collected tab and URL", async () => {
    const tab = fakeTab({ id: 9, url: watchUrl });
    const popupMetadata = metadataReader(metadataJson({}));
    await runPopupLaunch(
      container(),
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
      const result = await runPopupLaunch(
        container(),
        popupHarness({
          reader,
          metadataReader: popupMetadata.reader,
          activeTab: async () => tab,
        }).deps,
      );
      assert.deepEqual(popupMetadata.calls, []);
      assert.equal(result.outcome.kind, "rejected");
      assert.equal(result.outcome.reason, name);
      assert.deepEqual(result.metadata, emptyMetadataResult());
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
      const result = await runPopupLaunch(container(), popup.deps);
      assert.equal(result.outcome.kind, "collected");
      assert.deepEqual(result.metadata.values, {});
      const popupNative = acceptedNative();
      const popupRoot = container();
      await runPopupSend(popupRoot, result, {
        send: testSendDeps(popupNative.sendNative),
        now: () => 1000,
        log: recordingLogger().log,
        locale: "en",
      });
      await clickSend(popupRoot);
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
    const { outcome: popupOutcome } = await runPopupLaunch(
      container(),
      popup.deps,
    );
    assert.equal(popupOutcome.kind, "rejected");
    assert.equal(popupOutcome.reason, "collection-failed");
    assert.deepEqual(loggedOutcome(popup.recorder), popupOutcome);
  });

  it("treats a failing activeTab as collection-failed without rejecting", async () => {
    const popup = popupHarness({
      activeTab: async () => {
        throw new Error("query failed");
      },
    });
    const { outcome } = await runPopupLaunch(container(), popup.deps);
    assert.equal(outcome.kind, "rejected");
    assert.equal(outcome.reason, "collection-failed");
    assert.equal(popup.recorder.errors.length, 1);
  });

  it("keeps untrusted text out of the fixed log labels", async () => {
    const reader = readerReturning(page(`${malicious}\n`));
    const popup = popupHarness({ reader });
    await runPopupLaunch(container(), popup.deps);
    assert.equal(popup.recorder.infos[0]?.label, launchLabel);
  });
});

describe("popup send", () => {
  const collected = (): PopupLaunchResult => launched(collectedOutcome());

  for (const locale of localeList) {
    const catalog = catalogs[locale];
    const deps = (
      overrides: Partial<PopupSendDeps> = {},
    ): { deps: PopupSendDeps; recorder: RecordingLogger } => {
      const recorder = recordingLogger();
      return {
        deps: {
          send: testSendDeps(acceptedNative().sendNative),
          now: () => 1000,
          log: recorder.log,
          locale,
          ...overrides,
        },
        recorder,
      };
    };

    it(`shows the ${locale} send button and does not send until it is clicked`, async () => {
      const native = acceptedNative();
      const { deps: popupDeps } = deps({
        send: testSendDeps(native.sendNative),
      });
      const root = container();
      await runPopupSend(root, collected(), popupDeps);
      const button = query(root, ".send-button");
      assert.equal(button.textContent, catalog.sendButton);
      if (locale === "en") {
        // The English label stays a literal: reading it from the catalog
        // would let a changed English label pass unnoticed.
        assert.equal(button.textContent, "Send to yt2column");
      }
      for (const other of otherLocales(locale)) {
        expectAbsent(root, [catalogs[other].sendButton], [catalog.sendButton]);
      }
      assert.equal(native.messages.length, 0);
    });

    it(`sends once, disables the button and shows the ${locale} result`, async () => {
      const native = acceptedNative();
      const { deps: popupDeps, recorder } = deps({
        send: testSendDeps(native.sendNative),
      });
      const root = container();
      await runPopupSend(root, collected(), popupDeps);
      await clickSend(root);
      assert.equal(native.messages.length, 1);
      assert.equal(
        query<HTMLButtonElement>(root, ".send-button").disabled,
        true,
      );
      expectLocaleMessage(root, locale, (l) =>
        sendOutcomeMessage(acceptedOutcome, l),
      );
      assert.equal(
        root.querySelector(".send-request-id")?.textContent,
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
      const { deps: popupDeps } = deps({
        send: testSendDeps(native.sendNative),
      });
      const root = container();
      await runPopupSend(root, collected(), popupDeps);
      await clickSend(root);
      await clickSend(root);
      assert.equal(native.messages.length, 1);
    });

    it(`shows the ${locale} sending notice and a disabled button while sending`, async () => {
      const deferred = deferredNative();
      const { deps: popupDeps } = deps({
        send: testSendDeps(deferred.sendNative),
      });
      const root = container();
      await runPopupSend(root, collected(), popupDeps);
      const button = query<HTMLButtonElement>(root, ".send-button");
      button.click();
      await flush();
      assert.equal(button.disabled, true);
      expectLocaleMessage(root, locale, (l) => catalogs[l].sending);
      if (locale === "en") {
        // The close guidance and the Slack warning are what the notice is
        // for, so assert the whole English sentences independently of the
        // catalog it renders.
        const text = root.textContent ?? "";
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
        root.querySelector(".send-request-id")?.textContent,
        requestId,
      );
    });

    it(`keeps the button under the ${locale} expand prompt and sends the shown description`, async () => {
      const native = acceptedNative();
      const { deps: popupDeps } = deps({
        send: testSendDeps(native.sendNative),
      });
      const summaryRoot = container();
      const sendRoot = container();
      const result = await runPopupLaunch(
        summaryRoot,
        popupHarness({
          metadataReader: metadataReader(
            metadataJson({ description: "first lines" }, true),
          ).reader,
          locale,
        }).deps,
      );
      await runPopupSend(sendRoot, result, popupDeps);
      expectLocaleMessage(summaryRoot, locale, expandPromptMessage);
      assert.ok(sendRoot.querySelector(".send-button") !== null);
      await clickSend(sendRoot);
      assert.equal(native.messages.length, 1);
      assert.equal(native.messages[0]?.["description"], "first lines");
    });

    it(`logs a host failure at error and shows the ${locale} host-failed result`, async () => {
      const native = failingNative(
        "Specified native messaging host not found.",
      );
      const { deps: popupDeps, recorder } = deps({
        send: testSendDeps(native.sendNative),
      });
      const root = container();
      await runPopupSend(root, collected(), popupDeps);
      await clickSend(root);
      assert.equal(native.messages.length, 1);
      expectLocaleMessage(root, locale, (l) =>
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
      result: PopupLaunchResult;
    }[] = [
      {
        // The size count itself is prepare.test.ts's contract.
        when: "the selection takes the body over the limit",
        result: launched(collectedOutcome("a".repeat(maxBodyBytes))),
      },
      {
        // The selection alone fits, so only the metadata can block the send.
        when: "the metadata takes the body over the limit",
        result: launched(collectedOutcome(), {
          values: { description: "d".repeat(maxBodyBytes) },
          showExpandPrompt: false,
          omissions: [],
        }),
      },
    ];
    for (const { when, result } of popupBlocks) {
      it(`shows the ${locale} body-too-large reason and no button without sending when ${when}`, async () => {
        const native = acceptedNative();
        const { deps: popupDeps, recorder } = deps({
          send: testSendDeps(native.sendNative),
        });
        const root = container();
        await runPopupSend(root, result, popupDeps);
        assert.equal(root.querySelector(".send-button"), null);
        expectLocaleMessage(
          root,
          locale,
          (l) => catalogs[l].preflight["body-too-large"],
        );
        assert.equal(native.messages.length, 0);
        // A blocked launch is normal: logged at info, never at error.
        assert.deepEqual(
          recorder.infos.filter((entry) => entry.label === blockedLabel),
          [{ label: blockedLabel, data: { reason: "body-too-large" } }],
        );
        assert.equal(
          recorder.errors.some((entry) => entry.label === blockedLabel),
          false,
        );
      });
    }
  }

  it("sends the full description while the popup shows it truncated", async () => {
    const description = "x".repeat(metadataDisplayLimit + 50);
    const native = recordingNative(async () =>
      responseReply(202, acceptedBody),
    );
    const summaryRoot = container();
    const result = await runPopupLaunch(
      summaryRoot,
      popupHarness({
        metadataReader: metadataReader(metadataJson({ description })).reader,
      }).deps,
    );
    // Precondition: the popup shows a truncated value, so the body check
    // below tells the sent value apart from the displayed one.
    assert.equal(summaryRoot.textContent?.includes(description), false);
    const root = container();
    await runPopupSend(root, result, {
      send: testSendDeps(native.sendNative),
      now: () => 1000,
      log: recordingLogger().log,
      locale: "en",
    });
    await clickSend(root);
    assert.equal(native.messages[0]?.["description"], description);
  });

  it("renders nothing and does not send for a rejected collection", async () => {
    const native = acceptedNative();
    const root = container();
    root.append(root.ownerDocument.createElement("p"));
    await runPopupSend(
      root,
      launched({ kind: "rejected", reason: "empty-selection" }),
      {
        send: testSendDeps(native.sendNative),
        now: () => 1000,
        log: recordingLogger().log,
        locale: "en",
      },
    );
    assert.equal(root.childNodes.length, 0);
    assert.equal(native.messages.length, 0);
  });
});

describe("logs by locale", () => {
  /** Every log entry of one launch and one send, with only the locale set. */
  async function logsIn(locale: Locale): Promise<RecordingLogger> {
    const recorder = recordingLogger();
    const values = { title: "t", channelName: "c", description: "d" };
    const result = await runPopupLaunch(
      container(),
      popupHarness({
        metadataReader: metadataReader(metadataJson(values, true)).reader,
        log: recorder.log,
        locale,
      }).deps,
    );
    const root = container();
    await runPopupSend(root, result, {
      send: testSendDeps(acceptedNative().sendNative),
      now: () => 1000,
      log: recorder.log,
      locale,
    });
    await clickSend(root);
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
    const result = await runPopupLaunch(container(), popup.deps);
    // The send controls hold the metadata too: one sent and one blocked run.
    const send = recordingLogger();
    const sendRoot = container();
    await runPopupSend(sendRoot, result, {
      send: testSendDeps(acceptedNative().sendNative),
      now: () => 1000,
      log: send.log,
      locale: "en",
    });
    await clickSend(sendRoot);
    // The same metadata with a selection over the body limit blocks the send.
    const blocked = recordingLogger();
    await runPopupSend(
      container(),
      { ...result, outcome: collectedOutcome("a".repeat(maxBodyBytes)) },
      {
        send: testSendDeps(acceptedNative().sendNative),
        now: () => 1000,
        log: blocked.log,
        locale: "en",
      },
    );
    // Precondition: both send-control logs were written.
    assert.ok(send.infos.some((entry) => entry.label === sendLabel));
    assert.ok(blocked.infos.some((entry) => entry.label === blockedLabel));
    return { popup: popup.recorder, send, blocked };
  }

  const infoCases: {
    name: string;
    text: string;
    expected: Record<string, unknown>;
  }[] = [
    {
      name: "each omitted field and its reason",
      text: metadataJson(
        { title: "kept", channelName: " \u3000", description: "a\uD800b" },
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
  /** Sends once from a collected popup with sendNative; returns the log and the display. */
  async function sendOnce(
    native: ReturnType<typeof recordingNative>,
  ): Promise<{ recorder: RecordingLogger; root: HTMLElement }> {
    const recorder = recordingLogger();
    const root = container();
    await runPopupSend(root, launched(collectedOutcome()), {
      send: testSendDeps(native.sendNative),
      now: () => 1000,
      log: recorder.log,
      locale: "en",
    });
    await clickSend(root);
    return { recorder, root };
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
    const { recorder, root } = await sendOnce(failingNative(lastError));
    assert.deepEqual(sendLog(recorder)["result"], {
      kind: "host-failed",
      lastError,
    });
    assert.equal(root.textContent?.includes(lastError), false);
  });

  it("logs an unexpected reply's status but does not display it", async () => {
    // The server's answer to a wrong path: a reason the extension does not show.
    const body = JSON.stringify({
      status: "rejected",
      reason: "not_found",
      requestId,
    });
    const { recorder, root } = await sendOnce(
      recordingNative(async () => responseReply(404, body)),
    );
    assert.deepEqual(sendLog(recorder)["result"], {
      kind: "unexpected",
      check: "status-pairing",
      status: 404,
    });
    expectMessage(
      root,
      sendOutcomeMessage({ kind: "unexpected", check: "status-pairing" }, "en"),
    );
    assert.equal(root.textContent?.includes("404"), false);
  });
});

describe("display", () => {
  for (const locale of localeList) {
    it(`renders the launch summary in ${locale}`, async () => {
      const labels = catalogs[locale].summaryLabels;
      const collectedRoot = container();
      await runPopupLaunch(collectedRoot, popupHarness({ locale }).deps);
      assert.equal(valueOf(collectedRoot, labels.title), "A video");
      const rejectedRoot = container();
      await runPopupLaunch(
        rejectedRoot,
        popupHarness({
          activeTab: async () => fakeTab({ url: "https://example.com/" }),
          locale,
        }).deps,
      );
      expectLocaleMessage(rejectedRoot, locale, (l) =>
        rejectionMessage("not-watch-page", l),
      );
    });
  }

  it("renders the popup summary as text, not elements", async () => {
    const root = container();
    await runPopupLaunch(
      root,
      popupHarness({ reader: readerReturning(page(malicious)) }).deps,
    );
    assert.equal(root.querySelector("img"), null);
    assert.ok(root.textContent?.includes(malicious));
  });
});

describe("startPopup", () => {
  /** The document static/popup.html defines, so its region ids are checked. */
  function popupDocument(): Document {
    const html = readFileSync(
      path.join(extensionDir, "static", "popup.html"),
      "utf8",
    );
    return new JSDOM(html).window.document;
  }

  /** A document with the popup's two regions and the given initial lang. */
  function popupMarkup(lang: string): Document {
    return new JSDOM(
      `<!DOCTYPE html><html lang='${lang}'><body><div id='summary'></div><div id='send'></div></body></html>`,
    ).window.document;
  }

  /** Start dependencies over fake services, with one recording log. */
  function startDeps(
    i18n: I18nApi,
    log: Logger,
    overrides: {
      launch?: Partial<Omit<PopupLaunchDeps, "locale">>;
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

  it("displays Japanese after reading a Japanese UI language", async () => {
    const recorder = recordingLogger();
    const doc = popupDocument();
    await startPopup(
      doc,
      startDeps({ getUILanguage: () => "ja-JP" }, recorder.log),
    );
    // The shipped document says lang="en", so only the entry point can make it ja.
    assert.equal(doc.documentElement.lang, "ja");
    const summary = query<HTMLElement>(doc, "#summary");
    assert.equal(valueOf(summary, catalogs.ja.summaryLabels.title), "A video");
    expectAbsent(
      summary,
      [catalogs.en.summaryLabels.title],
      [catalogs.ja.summaryLabels.title],
    );
    const send = query<HTMLElement>(doc, "#send");
    assert.equal(
      query(send, ".send-button").textContent,
      catalogs.ja.sendButton,
    );
    expectAbsent(send, [catalogs.en.sendButton], [catalogs.ja.sendButton]);
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
      valueOf(
        query<HTMLElement>(unreadable.doc, "#summary"),
        catalogs.en.summaryLabels.title,
      ),
      "A video",
    );
    assert.deepEqual(loggedOutcome(unreadable.recorder), collectedOutcome());
    const fallbackButton = query<HTMLButtonElement>(
      unreadable.doc,
      ".send-button",
    );
    assert.equal(fallbackButton.textContent, catalogs.en.sendButton);
    // Both runs behave alike; the en-US run is the reference the plan names.
    assert.equal(
      query(unreadable.doc, "#summary").textContent,
      query(english.doc, "#summary").textContent,
    );
    assert.deepEqual(
      loggedOutcome(unreadable.recorder),
      loggedOutcome(english.recorder),
    );
    assert.equal(
      fallbackButton.textContent,
      query<HTMLButtonElement>(english.doc, ".send-button").textContent,
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

  it("sets the page language and runs no path when a region is missing", async () => {
    const recorder = recordingLogger();
    const doc = new JSDOM(
      "<!DOCTYPE html><html lang='en'><body><div id='summary'></div></body></html>",
    ).window.document;
    // Resolving rather than rejecting also rules out a null region reaching
    // the launch path.
    await startPopup(
      doc,
      startDeps({ getUILanguage: () => "ja-JP" }, recorder.log),
    );
    assert.equal(doc.documentElement.lang, "ja");
    assert.deepEqual(
      recorder.infos.map((entry) => entry.label),
      [uiLanguageLabel],
    );
  });

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
