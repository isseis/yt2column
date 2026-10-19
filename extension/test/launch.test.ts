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
import { maxBodyBytes } from "../src/core/prepare.ts";
import type { PreflightReason } from "../src/core/prepare.ts";
import type { RequestId, SendOutcome } from "../src/core/response.ts";
import { blockMessage, sendOutcomeMessage } from "../src/core/sendMessages.ts";
import type { SettingsReader } from "../src/core/settings.ts";
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
import type { FetchFn, FetchResponse } from "../src/send.ts";
import { container, query } from "./helpers/dom.ts";
import { recordingLogger } from "./helpers/logger.ts";
import type { RecordingLogger } from "./helpers/logger.ts";
import {
  identifiableToken,
  fixedSettings,
  failingSettings,
} from "./helpers/settings.ts";
import {
  recordingFetch,
  rejectingFetch,
  testSendDeps,
  textResponse,
} from "./helpers/fetch.ts";

const watchUrl = "https://www.youtube.com/watch?v=abc";
const malicious = "<img src=x onerror=alert(1)>";
const metadataLabel = "yt2column video metadata";
const popupLaunchLabel = "yt2column popup launch";
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

/** Settings with a valid token and no stored port (the default 8765). */
function readySettings() {
  return fixedSettings({ token: identifiableToken, port: undefined });
}

/** A recording fetch that returns an accepted response. */
function acceptedFetch() {
  return recordingFetch(async () => textResponse(202, acceptedBody));
}

/** A fetch whose response resolves only when the test calls resolve. */
function deferredFetch(): {
  fetchFn: FetchFn;
  resolve(response: FetchResponse): void;
} {
  let settle: ((response: FetchResponse) => void) | undefined;
  const pending = new Promise<FetchResponse>((resolve) => {
    settle = resolve;
  });
  return {
    fetchFn: async () => pending,
    resolve: (response) => settle?.(response),
  };
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

    const popupFetch = acceptedFetch();
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
      settings: readySettings().reader,
      send: testSendDeps(popupFetch.fetchFn),
      now: () => 1000,
      log: recordingLogger().log,
    });
    await clickSend(popupRoot);

    const popupBody = popupFetch.calls[0]?.init.body;
    assert.equal(typeof popupBody, "string");
    assert.deepEqual(JSON.parse(popupBody as string), {
      videoUrl: watchUrl,
      transcript: selection,
      ...values,
    });
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
      const popupFetch = acceptedFetch();
      const popupRoot = container();
      await runPopupSend(popupRoot, result, {
        settings: readySettings().reader,
        send: testSendDeps(popupFetch.fetchFn),
        now: () => 1000,
        log: recordingLogger().log,
      });
      await clickSend(popupRoot);
      assert.deepEqual(JSON.parse(popupFetch.calls[0]?.init.body as string), {
        videoUrl: watchUrl,
        transcript: "selection",
      });

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
        settings: readySettings().reader,
        send: testSendDeps(acceptedFetch().fetchFn),
        now: () => 1000,
        log: recorder.log,
        ...overrides,
      },
      recorder,
    };
  };

  const collected = (): PopupLaunchResult => launched(collectedOutcome());

  it("shows a send button and does not send until it is clicked", async () => {
    const fetch = acceptedFetch();
    const { deps: popupDeps } = deps({ send: testSendDeps(fetch.fetchFn) });
    const root = container();
    await runPopupSend(root, collected(), popupDeps);
    assert.ok(root.querySelector(".send-button") !== null);
    assert.equal(fetch.calls.length, 0);
  });

  it("sends once, disables the button and shows the result", async () => {
    const fetch = acceptedFetch();
    const { deps: popupDeps, recorder } = deps({
      send: testSendDeps(fetch.fetchFn),
    });
    const root = container();
    await runPopupSend(root, collected(), popupDeps);
    await clickSend(root);
    assert.equal(fetch.calls.length, 1);
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
    const fetch = acceptedFetch();
    const { deps: popupDeps } = deps({ send: testSendDeps(fetch.fetchFn) });
    const root = container();
    await runPopupSend(root, collected(), popupDeps);
    await clickSend(root);
    await clickSend(root);
    assert.equal(fetch.calls.length, 1);
  });

  it("shows the sending notice and a disabled button while sending", async () => {
    const deferred = deferredFetch();
    const { deps: popupDeps } = deps({ send: testSendDeps(deferred.fetchFn) });
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
    deferred.resolve(textResponse(202, acceptedBody));
    await flush();
    assert.equal(
      root.querySelector(".send-request-id")?.textContent,
      requestId,
    );
  });

  it("keeps the button under the expand prompt and sends the shown description", async () => {
    const fetch = acceptedFetch();
    const { deps: popupDeps } = deps({ send: testSendDeps(fetch.fetchFn) });
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
    assert.equal(fetch.calls.length, 1);
    const body = JSON.parse(fetch.calls[0]?.init.body as string) as {
      description?: string;
    };
    assert.equal(body.description, "first lines");
  });

  it("sends the full description while the popup shows it truncated", async () => {
    const description = "x".repeat(metadataDisplayLimit + 50);
    const fetch = acceptedFetch();
    const { deps: popupDeps } = deps({ send: testSendDeps(fetch.fetchFn) });
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
    const body = JSON.parse(fetch.calls[0]?.init.body as string) as {
      description?: string;
    };
    assert.equal(body.description, description);
  });

  it("renders nothing and does not read settings for a rejected collection", async () => {
    const settings = failingSettings(new Error("must not load"));
    const fetch = acceptedFetch();
    const { deps: popupDeps } = deps({
      settings: settings.reader,
      send: testSendDeps(fetch.fetchFn),
    });
    const root = container();
    root.append(root.ownerDocument.createElement("p"));
    await runPopupSend(
      root,
      launched({ kind: "rejected", reason: "empty-selection" }),
      popupDeps,
    );
    assert.equal(root.childNodes.length, 0);
    assert.equal(settings.loadCalls(), 0);
    assert.equal(fetch.calls.length, 0);
  });

  it("logs a failed send at error and shows the unknown result", async () => {
    const fetch = rejectingFetch(new TypeError("Failed to fetch"));
    const { deps: popupDeps, recorder } = deps({
      send: testSendDeps(fetch.fetchFn),
    });
    const root = container();
    await runPopupSend(root, collected(), popupDeps);
    await clickSend(root);
    assert.equal(fetch.calls.length, 1);
    expectMessage(root, sendOutcomeMessage({ kind: "unknown" }));
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
    reason: PreflightReason;
    // Tells apart rows that share a reason.
    when?: string;
    settings: SettingsReader;
    result?: PopupLaunchResult;
    // A settings read failure is a defect and is logged at error.
    error: boolean;
  }[] = [
    {
      reason: "token-missing",
      settings: fixedSettings({ token: undefined, port: undefined }).reader,
      error: false,
    },
    {
      reason: "settings-unreadable",
      settings: failingSettings(new Error("storage failed")).reader,
      error: true,
    },
    {
      reason: "port-invalid",
      settings: fixedSettings({ token: identifiableToken, port: "80" }).reader,
      error: false,
    },
    {
      reason: "token-invalid",
      settings: fixedSettings({ token: "not-a-token", port: undefined }).reader,
      error: false,
    },
    {
      // The size count itself is prepare.test.ts's contract.
      reason: "body-too-large",
      when: "the selection takes the body over the limit",
      settings: readySettings().reader,
      result: launched(collectedOutcome("a".repeat(maxBodyBytes))),
      error: false,
    },
    {
      // The selection alone fits, so only the metadata can block the send.
      reason: "body-too-large",
      when: "the metadata takes the body over the limit",
      settings: readySettings().reader,
      result: launched(collectedOutcome(), {
        values: { description: "d".repeat(maxBodyBytes) },
        showExpandPrompt: false,
        omissions: [],
      }),
      error: false,
    },
  ];
  for (const { reason, when, settings, result, error } of popupBlocks) {
    const suffix = when === undefined ? "" : ` when ${when}`;
    it(`shows the ${reason} reason and no button without sending${suffix}`, async () => {
      const fetch = acceptedFetch();
      const { deps: popupDeps, recorder } = deps({
        settings,
        send: testSendDeps(fetch.fetchFn),
      });
      const root = container();
      await runPopupSend(root, result ?? collected(), popupDeps);
      assert.equal(root.querySelector(".send-button"), null);
      expectMessage(root, blockMessage(reason));
      assert.equal(fetch.calls.length, 0);
      const [entries, others] = error
        ? [recorder.errors, recorder.infos]
        : [recorder.infos, recorder.errors];
      assert.deepEqual(
        entries
          .filter((entry) => entry.label === blockedLabel)
          .map((entry) => (entry.data as { reason: unknown }).reason),
        [reason],
      );
      assert.equal(
        others.some((entry) => entry.label === blockedLabel),
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
      settings: readySettings().reader,
      send: testSendDeps(acceptedFetch().fetchFn),
      now: () => 1000,
      log: send.log,
    });
    await clickSend(sendRoot);
    const blocked = recordingLogger();
    await runPopupSend(container(), result, {
      settings: fixedSettings({ token: undefined, port: undefined }).reader,
      send: testSendDeps(acceptedFetch().fetchFn),
      now: () => 1000,
      log: blocked.log,
    });
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

describe("token never leaks", () => {
  it("keeps the token out of the logs and the display", async () => {
    const popup = popupHarness();
    const root = container();
    const outcome = await runPopupLaunch(root, popup.deps);
    const send = recordingLogger();
    await runPopupSend(root, outcome, {
      settings: readySettings().reader,
      send: testSendDeps(acceptedFetch().fetchFn),
      now: () => 1000,
      log: send.log,
    });
    await clickSend(root);
    // The block happens after the token is loaded, so the token is in reach.
    const blocked = recordingLogger();
    const blockedRoot = container();
    await runPopupSend(blockedRoot, outcome, {
      settings: fixedSettings({ token: identifiableToken, port: "80" }).reader,
      send: testSendDeps(acceptedFetch().fetchFn),
      now: () => 1000,
      log: blocked.log,
    });
    // A failed send with the token loaded takes the error branch of the log.
    const failed = recordingLogger();
    const failedRoot = container();
    await runPopupSend(failedRoot, outcome, {
      settings: readySettings().reader,
      send: testSendDeps(
        rejectingFetch(new TypeError("Failed to fetch")).fetchFn,
      ),
      now: () => 1000,
      log: failed.log,
    });
    await clickSend(failedRoot);
    // Precondition: each log under test was written.
    assert.ok(
      popup.recorder.infos.some((entry) => entry.label === popupLaunchLabel),
    );
    assert.ok(send.infos.some((entry) => entry.label === sendLabel));
    assert.ok(blocked.infos.some((entry) => entry.label === blockedLabel));
    assert.ok(failed.errors.some((entry) => entry.label === sendLabel));

    const serialized = JSON.stringify(
      {
        popupInfos: popup.recorder.infos,
        popupErrors: popup.recorder.errors,
        sendInfos: send.infos,
        sendErrors: send.errors,
        blockedInfos: blocked.infos,
        blockedErrors: blocked.errors,
        failedInfos: failed.infos,
        failedErrors: failed.errors,
        text: root.textContent,
        blockedText: blockedRoot.textContent,
        failedText: failedRoot.textContent,
      },
      // JSON.stringify turns an Error into {}; keep its name and message.
      (_key, value: unknown) =>
        value instanceof Error ? `${value.name}: ${value.message}` : value,
    );
    assert.equal(serialized.includes(identifiableToken), false);
    assert.equal(serialized.includes(identifiableToken.slice(0, 4)), false);
    assert.equal(serialized.includes(identifiableToken.slice(-4)), false);
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
