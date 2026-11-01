import assert from "node:assert/strict";
import { describe, it } from "node:test";

import {
  chooseViewer,
  parseAwaitStart,
  parseAwaitStartReply,
  parseStartReply,
  parseStartRequest,
  parseViewerChannelMessage,
} from "../src/core/extensionMessages.ts";
import type { PresentAnswer } from "../src/core/extensionMessages.ts";
import { maxBodyBytes } from "../src/core/prepare.ts";

const now = 1_700_000_000_000;
const watchUrl = "https://www.youtube.com/watch?v=abc";
const queryId = "0123456789abcdef0123456789abcdef";
const requestId = "0123456789abcdef";

/** A request every check accepts; each rejected case changes one value of it. */
function validRequest(): Record<string, unknown> {
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
      heading: "Title \u{1F600}",
      channelName: "Channel",
      title: "Title",
      url: watchUrl,
      sentAt: now,
    },
    source: { tabId: 3, windowId: 1, index: 0 },
  };
}

type Patch = (request: Record<string, Record<string, unknown>>) => void;

/** validRequest with one change applied to its members. */
function changed(patch: Patch): unknown {
  const request = structuredClone(validRequest()) as Record<
    string,
    Record<string, unknown>
  >;
  patch(request);
  return request;
}

/** A request whose input JSON text is exactly bytes long. */
function requestOfBodySize(bytes: number): unknown {
  const empty = JSON.stringify({ videoUrl: watchUrl, transcript: "" });
  return changed((request) => {
    request.input = {
      videoUrl: watchUrl,
      transcript: "a".repeat(bytes - empty.length),
    };
  });
}

describe("parseStartRequest", () => {
  it("accepts the valid request unchanged", () => {
    assert.deepEqual(parseStartRequest(validRequest(), now), validRequest());
  });

  it("accepts a request without the optional members", () => {
    const request = changed((r) => {
      r.input = { videoUrl: watchUrl, transcript: "t" };
      r.viewer = { heading: "h", url: watchUrl, sentAt: 0 };
    });
    assert.deepEqual(parseStartRequest(request, now), request);
  });

  it("accepts an input of exactly 4 MiB", () => {
    assert.notEqual(
      parseStartRequest(requestOfBodySize(maxBodyBytes), now),
      undefined,
    );
  });

  const rejected: { name: string; value: unknown }[] = [
    { name: "null", value: null },
    { name: "an array", value: [validRequest()] },
    { name: "a string", value: "start" },
    {
      name: "another type",
      value: changed((r) => void (r.type = "Start" as never)),
    },
    { name: "an extra member", value: changed((r) => void (r.extra = {})) },
    { name: "a missing source", value: changed((r) => void delete r.source) },
    { name: "an input over 4 MiB", value: requestOfBodySize(maxBodyBytes + 1) },
    {
      name: "an input without its transcript",
      value: changed((r) => void delete r.input!.transcript),
    },
    {
      name: "an input with an extra member",
      value: changed((r) => void (r.input!.tabTitle = "x")),
    },
    {
      name: "an input whose URL is a number",
      value: changed((r) => void (r.input!.videoUrl = 1)),
    },
    {
      name: "an input with an empty title",
      value: changed((r) => void (r.input!.title = "")),
    },
    {
      name: "an input whose description is null",
      value: changed((r) => void (r.input!.description = null)),
    },
    {
      name: "a viewer without its heading",
      value: changed((r) => void delete r.viewer!.heading),
    },
    {
      name: "a viewer carrying the transcript",
      value: changed((r) => void (r.viewer!.transcript = "t")),
    },
    {
      name: "a viewer with an empty heading",
      value: changed((r) => void (r.viewer!.heading = "")),
    },
    {
      name: "a viewer heading over its limit",
      value: changed((r) => void (r.viewer!.heading = "a".repeat(1_002))),
    },
    {
      name: "a viewer URL that is not accepted",
      value: changed((r) => void (r.viewer!.url = "https://example.com/")),
    },
    {
      name: "a send time in the future",
      value: changed((r) => void (r.viewer!.sentAt = now + 1)),
    },
    {
      name: "a fractional send time",
      value: changed((r) => void (r.viewer!.sentAt = 1.5)),
    },
    {
      name: "a send time as text",
      value: changed((r) => void (r.viewer!.sentAt = String(now))),
    },
    {
      name: "an unpaired high surrogate in the heading",
      value: changed((r) => void (r.viewer!.heading = "a\uD83D")),
    },
    {
      name: "an unpaired low surrogate in the channel name",
      value: changed((r) => void (r.viewer!.channelName = "\uDE00a")),
    },
    {
      name: "a reversed surrogate pair in the title",
      value: changed((r) => void (r.viewer!.title = "\uDE00\uD83D")),
    },
    {
      name: "a source without its index",
      value: changed((r) => void delete r.source!.index),
    },
    {
      name: "a negative source tab",
      value: changed((r) => void (r.source!.tabId = -1)),
    },
    {
      name: "a fractional source window",
      value: changed((r) => void (r.source!.windowId = 0.5)),
    },
  ];
  for (const { name, value } of rejected) {
    it(`rejects ${name}`, () => {
      assert.equal(parseStartRequest(value, now), undefined);
    });
  }

  it("rejects, rather than throws, when reading a member throws", () => {
    const request = validRequest();
    Object.defineProperty(request, "input", {
      enumerable: true,
      get() {
        throw new Error("boom");
      },
    });
    assert.equal(parseStartRequest(request, now), undefined);
  });
});

describe("parseStartReply", () => {
  const accepted: { name: string; value: unknown }[] = [
    {
      name: "an accepted outcome with its viewer tab",
      value: {
        outcome: { kind: "accepted", key: "k", requestId },
        viewerTabId: 4,
      },
    },
    {
      name: "a rejection without a viewer tab",
      value: {
        outcome: { kind: "rejected", reason: "invalid_body", requestId },
      },
    },
    {
      name: "a send not sent",
      value: { outcome: { kind: "not-sent", cause: "server-unavailable" } },
    },
    { name: "no response", value: { outcome: { kind: "no-response" } } },
    { name: "a host failure", value: { outcome: { kind: "host-failed" } } },
    {
      name: "an unexpected reply",
      value: { outcome: { kind: "unexpected", check: "member-set" } },
    },
  ];
  for (const { name, value } of accepted) {
    it(`accepts ${name} unchanged`, () => {
      assert.deepEqual(parseStartReply(value), value);
    });
  }

  const rejected: { name: string; value: unknown }[] = [
    { name: "a missing outcome", value: { viewerTabId: 4 } },
    {
      name: "an extra member",
      value: { outcome: { kind: "no-response" }, tabId: 4 },
    },
    {
      name: "a negative viewer tab",
      value: { outcome: { kind: "no-response" }, viewerTabId: -1 },
    },
    { name: "an unknown kind", value: { outcome: { kind: "ok" } } },
    {
      name: "an accepted outcome without its key",
      value: { outcome: { kind: "accepted", requestId } },
    },
    {
      name: "an accepted outcome with an empty key",
      value: { outcome: { kind: "accepted", key: "", requestId } },
    },
    {
      name: "a malformed request ID",
      value: { outcome: { kind: "accepted", key: "k", requestId: "XYZ" } },
    },
    {
      name: "a reason outside the start's set",
      value: {
        outcome: { kind: "rejected", reason: "invalid_key", requestId },
      },
    },
    {
      name: "an unknown not-sent cause",
      value: { outcome: { kind: "not-sent", cause: "offline" } },
    },
    {
      name: "an unknown check",
      value: { outcome: { kind: "unexpected", check: "escape" } },
    },
    {
      name: "a host failure carrying the browser's error text",
      value: { outcome: { kind: "host-failed", lastError: "text" } },
    },
    {
      name: "an unexpected reply carrying its status",
      value: { outcome: { kind: "unexpected", check: "syntax", status: 500 } },
    },
  ];
  for (const { name, value } of rejected) {
    it(`rejects ${name}`, () => {
      assert.equal(parseStartReply(value), undefined);
    });
  }
});

describe("parseAwaitStart", () => {
  it("accepts a wait unchanged", () => {
    const value = { type: "await-start", sentAt: now };
    assert.deepEqual(parseAwaitStart(value), value);
  });

  const rejected: { name: string; value: unknown }[] = [
    { name: "another type", value: { type: "start", sentAt: now } },
    {
      name: "a tab of its own",
      value: { type: "await-start", sentAt: now, tabId: 1 },
    },
    { name: "a missing send time", value: { type: "await-start" } },
    {
      name: "a negative send time",
      value: { type: "await-start", sentAt: -1 },
    },
    {
      name: "a send time as text",
      value: { type: "await-start", sentAt: "1" },
    },
  ];
  for (const { name, value } of rejected) {
    it(`rejects ${name}`, () => {
      assert.equal(parseAwaitStart(value), undefined);
    });
  }
});

describe("parseAwaitStartReply", () => {
  const accepted: { name: string; value: unknown }[] = [
    {
      name: "a result",
      value: { kind: "result", outcome: { kind: "no-response" } },
    },
    { name: "none", value: { kind: "none" } },
  ];
  for (const { name, value } of accepted) {
    it(`accepts ${name} unchanged`, () => {
      assert.deepEqual(parseAwaitStartReply(value), value);
    });
  }

  const rejected: { name: string; value: unknown }[] = [
    { name: "a result without its outcome", value: { kind: "result" } },
    {
      name: "a result with an invalid outcome",
      value: { kind: "result", outcome: { kind: "ok" } },
    },
    {
      name: "none with an outcome",
      value: { kind: "none", outcome: { kind: "no-response" } },
    },
    { name: "an unknown kind", value: { kind: "later" } },
  ];
  for (const { name, value } of rejected) {
    it(`rejects ${name}`, () => {
      assert.equal(parseAwaitStartReply(value), undefined);
    });
  }
});

describe("parseViewerChannelMessage", () => {
  const accepted: { name: string; value: unknown }[] = [
    { name: "a find", value: { type: "find", queryId, key: "k" } },
    {
      name: "a present",
      value: { type: "present", queryId, tabId: 1, windowId: 2, sentAt: now },
    },
    {
      name: "a waiting",
      value: { type: "waiting", queryId, tabId: 1, sentAt: now },
    },
    {
      name: "a take-over",
      value: { type: "take-over", queryId, tabId: 1, sentAt: now },
    },
    { name: "a took-over", value: { type: "took-over", queryId, tabId: 1 } },
  ];
  for (const { name, value } of accepted) {
    it(`accepts ${name} unchanged`, () => {
      assert.deepEqual(parseViewerChannelMessage(value), value);
    });
  }

  const rejected: { name: string; value: unknown }[] = [
    { name: "an unknown type", value: { type: "found", queryId } },
    {
      name: "a query ID one character short",
      value: { type: "find", queryId: queryId.slice(1), key: "k" },
    },
    {
      name: "an uppercase query ID",
      value: { type: "find", queryId: queryId.toUpperCase(), key: "k" },
    },
    {
      name: "a find with an empty key",
      value: { type: "find", queryId, key: "" },
    },
    {
      name: "a find with a key over its limit",
      value: { type: "find", queryId, key: "k".repeat(257) },
    },
    {
      name: "a present without its window",
      value: { type: "present", queryId, tabId: 1, sentAt: now },
    },
    {
      name: "a waiting with a window",
      value: { type: "waiting", queryId, tabId: 1, windowId: 2, sentAt: now },
    },
    {
      name: "a take-over with a fractional tab",
      value: { type: "take-over", queryId, tabId: 1.5, sentAt: now },
    },
    {
      name: "a took-over with a send time",
      value: { type: "took-over", queryId, tabId: 1, sentAt: now },
    },
    {
      name: "a find with an extra member",
      value: { type: "find", queryId, key: "k", tabId: 1 },
    },
  ];
  for (const { name, value } of rejected) {
    it(`rejects ${name}`, () => {
      assert.equal(parseViewerChannelMessage(value), undefined);
    });
  }
});

describe("chooseViewer", () => {
  function present(tabId: number, sentAt: number): PresentAnswer {
    return { type: "present", queryId, tabId, windowId: 1, sentAt };
  }

  it("returns undefined without answers", () => {
    assert.equal(chooseViewer([]), undefined);
  });

  it("chooses the oldest send time whatever the order", () => {
    const oldest = present(9, 100);
    assert.equal(
      chooseViewer([present(1, 200), oldest, present(2, 300)]),
      oldest,
    );
    assert.equal(chooseViewer([oldest, present(1, 200)]), oldest);
  });

  it("chooses the smallest tab among equal send times", () => {
    const smallest = present(2, 100);
    assert.equal(
      chooseViewer([present(5, 100), smallest, present(3, 100)]),
      smallest,
    );
  });
});
