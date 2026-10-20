import assert from "node:assert/strict";
import { describe, it } from "node:test";

import type {
  CollectedInput,
  CollectOutcome,
  PageSelection,
  SelectionReader,
} from "../src/core/collect.ts";
import { expandPromptMessage } from "../src/core/messages.ts";
import type { RejectionMessage } from "../src/core/messages.ts";
import { emptyMetadataResult } from "../src/core/metadata.ts";
import type { MetadataResult } from "../src/core/metadata.ts";
import { nativeHostName } from "../src/core/native.ts";
import type { NativeReply } from "../src/core/native.ts";
import { maxBodyBytes } from "../src/core/prepare.ts";
import type { RequestId, SendOutcome } from "../src/core/response.ts";
import { blockMessage, sendOutcomeMessage } from "../src/core/sendMessages.ts";
import { metadataDisplayLimit } from "../src/ui/render.ts";
import { sendingMessage } from "../src/ui/renderSend.ts";
import {
  launchContextFromTab,
  runPopupLaunch,
  runPopupSend,
} from "../src/launch.ts";
import type {
  MetadataReader,
  PopupLaunchDeps,
  PopupLaunchResult,
  PopupSendDeps,
} from "../src/launch.ts";
import { container, query } from "./helpers/dom.ts";
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
const metadataLabel = "yt2column video metadata";
const blockedLabel = "yt2column blocked";
const sendLabel = "yt2column send result";
const requestId = "0123456789abcdef" as RequestId;
const acceptedBody = JSON.stringify({ status: "accepted", requestId });
const acceptedOutcome: SendOutcome = { kind: "accepted", requestId };

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
    ...overrides,
  };
  return { deps, recorder };
}

/** The outcome the path logged for the developer. */
function loggedOutcome(recorder: RecordingLogger): CollectOutcome {
  const entry = recorder.infos[0];
  assert.ok(entry !== undefined, "expected an info log");
  return (entry.data as { outcome: CollectOutcome }).outcome;
}

/** Asserts the container text shows the message and each of its steps. */
function expectMessage(root: HTMLElement, message: RejectionMessage): void {
  const text = root.textContent ?? "";
  assert.ok(text.includes(message.text), message.text);
  for (const step of message.steps) {
    assert.ok(text.includes(step), step);
  }
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
    assert.equal(popup.recorder.infos[0]?.label, "yt2column popup launch");
  });
});

describe("popup send", () => {
  const deps = (
    overrides: Partial<PopupSendDeps> = {},
  ): { deps: PopupSendDeps; recorder: RecordingLogger } => {
    const recorder = recordingLogger();
    return {
      deps: {
        send: testSendDeps(acceptedNative().sendNative),
        now: () => 1000,
        log: recorder.log,
        ...overrides,
      },
      recorder,
    };
  };

  const collected = (): PopupLaunchResult => launched(collectedOutcome());

  it("shows a send button and does not send until it is clicked", async () => {
    const native = acceptedNative();
    const { deps: popupDeps } = deps({ send: testSendDeps(native.sendNative) });
    const root = container();
    await runPopupSend(root, collected(), popupDeps);
    assert.ok(root.querySelector(".send-button") !== null);
    assert.equal(native.messages.length, 0);
  });

  it("sends once, disables the button and shows the result", async () => {
    const native = acceptedNative();
    const { deps: popupDeps, recorder } = deps({
      send: testSendDeps(native.sendNative),
    });
    const root = container();
    await runPopupSend(root, collected(), popupDeps);
    await clickSend(root);
    assert.equal(native.messages.length, 1);
    assert.equal(query<HTMLButtonElement>(root, ".send-button").disabled, true);
    expectMessage(root, sendOutcomeMessage(acceptedOutcome));
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

  it("sends only once when the button is clicked repeatedly", async () => {
    const native = acceptedNative();
    const { deps: popupDeps } = deps({ send: testSendDeps(native.sendNative) });
    const root = container();
    await runPopupSend(root, collected(), popupDeps);
    await clickSend(root);
    await clickSend(root);
    assert.equal(native.messages.length, 1);
  });

  it("shows the sending notice and a disabled button while sending", async () => {
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
    expectMessage(root, sendingMessage);
    // The close guidance and the Slack warning are what the notice is for, so
    // assert the whole sentences independently of the constant it renders.
    const text = root.textContent ?? "";
    for (const sentence of [
      "Do not close this window until the result appears.",
      "If you close it, check Slack before sending again.",
    ]) {
      assert.ok(text.includes(sentence), sentence);
    }
    deferred.resolve(responseReply(202, acceptedBody));
    await flush();
    assert.equal(
      root.querySelector(".send-request-id")?.textContent,
      requestId,
    );
  });

  it("keeps the button under the expand prompt and sends the shown description", async () => {
    const native = acceptedNative();
    const { deps: popupDeps } = deps({ send: testSendDeps(native.sendNative) });
    const summaryRoot = container();
    const sendRoot = container();
    const result = await runPopupLaunch(
      summaryRoot,
      popupHarness({
        metadataReader: metadataReader(
          metadataJson({ description: "first lines" }, true),
        ).reader,
      }).deps,
    );
    await runPopupSend(sendRoot, result, popupDeps);
    expectMessage(summaryRoot, expandPromptMessage);
    assert.ok(sendRoot.querySelector(".send-button") !== null);
    await clickSend(sendRoot);
    assert.equal(native.messages.length, 1);
    assert.equal(native.messages[0]?.["description"], "first lines");
  });

  it("sends the full description while the popup shows it truncated", async () => {
    const description = "x".repeat(metadataDisplayLimit + 50);
    const native = acceptedNative();
    const { deps: popupDeps } = deps({ send: testSendDeps(native.sendNative) });
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
    await runPopupSend(root, result, popupDeps);
    await clickSend(root);
    assert.equal(native.messages[0]?.["description"], description);
  });

  it("renders nothing and does not send for a rejected collection", async () => {
    const native = acceptedNative();
    const { deps: popupDeps } = deps({ send: testSendDeps(native.sendNative) });
    const root = container();
    root.append(root.ownerDocument.createElement("p"));
    await runPopupSend(
      root,
      launched({ kind: "rejected", reason: "empty-selection" }),
      popupDeps,
    );
    assert.equal(root.childNodes.length, 0);
    assert.equal(native.messages.length, 0);
  });

  it("logs a host failure at error and shows the host-failed result", async () => {
    const native = failingNative("Specified native messaging host not found.");
    const { deps: popupDeps, recorder } = deps({
      send: testSendDeps(native.sendNative),
    });
    const root = container();
    await runPopupSend(root, collected(), popupDeps);
    await clickSend(root);
    assert.equal(native.messages.length, 1);
    expectMessage(root, sendOutcomeMessage({ kind: "host-failed" }));
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
    it(`shows the body-too-large reason and no button without sending when ${when}`, async () => {
      const native = acceptedNative();
      const { deps: popupDeps, recorder } = deps({
        send: testSendDeps(native.sendNative),
      });
      const root = container();
      await runPopupSend(root, result, popupDeps);
      assert.equal(root.querySelector(".send-button"), null);
      expectMessage(root, blockMessage("body-too-large"));
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
      sendOutcomeMessage({ kind: "unexpected", check: "status-pairing" }),
    );
    assert.equal(root.textContent?.includes("404"), false);
  });
});

describe("display", () => {
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
