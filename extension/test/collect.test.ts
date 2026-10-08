import assert from "node:assert/strict";
import { describe, it } from "node:test";

import {
  collect,
  type CollectOutcome,
  type PageSelection,
  type SelectionReader,
} from "../src/core/collect.ts";
import type { LaunchContext, RejectionReason } from "../src/core/types.ts";

const watchUrl = "https://www.youtube.com/watch?v=abc";
const context: LaunchContext = { tabId: 7, url: watchUrl, title: "A video" };

function page(text: string, documentUrl = watchUrl): PageSelection {
  return { text, documentUrl };
}

function readerReturning(selection: PageSelection): SelectionReader {
  return { read: async () => selection };
}

/** A reader that records the tab ids it is asked to read. */
function readingReader(selection: PageSelection): {
  reader: SelectionReader;
  calls: number[];
} {
  const calls: number[] = [];
  return {
    reader: {
      read: async (tabId) => {
        calls.push(tabId);
        return selection;
      },
    },
    calls,
  };
}

function assertRejected(
  outcome: CollectOutcome,
  reason: RejectionReason,
): void {
  assert.ok(outcome.kind === "rejected", "expected a rejection");
  assert.equal(outcome.reason, reason);
}

describe("collect", () => {
  it("rejects an undefined launch as collection-failed without reading", async () => {
    const { reader, calls } = readingReader(page("x"));
    const outcome = await collect(undefined, reader);
    assertRejected(outcome, "collection-failed");
    assert.deepEqual(calls, []);
    assert.ok("cause" in outcome);
    assert.ok(outcome.cause instanceof Error);
  });

  it("rejects a missing URL as not-watch-page without reading", async () => {
    const { reader, calls } = readingReader(page("x"));
    const outcome = await collect(
      { tabId: 7, url: undefined, title: "A video" },
      reader,
    );
    assertRejected(outcome, "not-watch-page");
    assert.deepEqual(calls, []);
  });

  it("rejects a non-watch URL as not-watch-page without reading", async () => {
    const { reader, calls } = readingReader(page("x"));
    const outcome = await collect(
      { tabId: 7, url: "https://example.com/", title: "A video" },
      reader,
    );
    assertRejected(outcome, "not-watch-page");
    assert.deepEqual(calls, []);
  });

  it("rejects a failed read as collection-failed and keeps the cause", async () => {
    const cause = new Error("injection denied");
    const reader: SelectionReader = {
      read: async () => {
        throw cause;
      },
    };
    const outcome = await collect(context, reader);
    assertRejected(outcome, "collection-failed");
    assert.ok("cause" in outcome);
    assert.equal(outcome.cause, cause);
  });

  it("rejects a document URL that does not match the launched URL", async () => {
    const reader = readerReturning(
      page("text", "https://www.youtube.com/watch?v=other"),
    );
    const outcome = await collect(context, reader);
    assertRejected(outcome, "collection-failed");
    assert.ok("cause" in outcome && outcome.cause instanceof Error);
    assert.ok(outcome.cause.message.includes(watchUrl));
    assert.ok(outcome.cause.message.includes("v=other"));
  });

  const blankSelections = ["", " ", "\n", "\t", "\u3000", " \n\u3000 \r"];
  for (const selection of blankSelections) {
    it(`rejects a selection of ${JSON.stringify(selection)} as empty-selection`, async () => {
      const outcome = await collect(context, readerReturning(page(selection)));
      assertRejected(outcome, "empty-selection");
    });
  }

  it("rejects U+0085-only selection (White_Space) as empty-selection", async () => {
    const outcome = await collect(context, readerReturning(page("\u0085")));
    assertRejected(outcome, "empty-selection");
  });

  it("accepts a U+FEFF-only selection, which is not White_Space", async () => {
    const outcome = await collect(context, readerReturning(page("\uFEFF")));
    assert.ok(outcome.kind === "collected");
    assert.equal(outcome.input.selection, "\uFEFF");
  });

  it("rejects an empty title as empty-title", async () => {
    const outcome = await collect(
      { tabId: 7, url: watchUrl, title: "" },
      readerReturning(page("text")),
    );
    assertRejected(outcome, "empty-title");
  });

  it("rejects a whitespace-only title as empty-title", async () => {
    const outcome = await collect(
      { tabId: 7, url: watchUrl, title: " \n\u3000" },
      readerReturning(page("text")),
    );
    assertRejected(outcome, "empty-title");
  });

  it("rejects a U+3000-only title as empty-title", async () => {
    const outcome = await collect(
      { tabId: 7, url: watchUrl, title: "\u3000" },
      readerReturning(page("text")),
    );
    assertRejected(outcome, "empty-title");
  });

  it("rejects an undefined title as empty-title", async () => {
    const outcome = await collect(
      { tabId: 7, url: watchUrl, title: undefined },
      readerReturning(page("text")),
    );
    assertRejected(outcome, "empty-title");
  });

  it("checks the selection before the title", async () => {
    const outcome = await collect(
      { tabId: 7, url: watchUrl, title: "" },
      readerReturning(page(" \n")),
    );
    assertRejected(outcome, "empty-selection");
  });

  it("checks the URL before reading the selection", async () => {
    const { reader, calls } = readingReader(page(""));
    const outcome = await collect(
      { tabId: 7, url: "https://example.com/", title: "" },
      reader,
    );
    assertRejected(outcome, "not-watch-page");
    assert.deepEqual(calls, []);
  });

  it("reports a read failure before inspecting the selection", async () => {
    const reader: SelectionReader = {
      read: async () => {
        throw new Error("boom");
      },
    };
    const outcome = await collect(
      { tabId: 7, url: watchUrl, title: "" },
      reader,
    );
    assertRejected(outcome, "collection-failed");
  });

  it("reads the launched tab and copies the values without trimming", async () => {
    const selection = " 00:00\ncaption line\n\n";
    const title = "  A title  ";
    const { reader, calls } = readingReader(page(selection));
    const outcome = await collect({ tabId: 7, url: watchUrl, title }, reader);
    assert.deepEqual(calls, [7]);
    assert.ok(outcome.kind === "collected");
    assert.deepEqual(outcome.input, { selection, url: watchUrl, title });
  });
});
