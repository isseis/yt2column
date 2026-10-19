import assert from "node:assert/strict";
import { describe, it } from "node:test";

import type { CollectedInput, CollectOutcome } from "../src/core/collect.ts";
import { summarize } from "../src/core/summary.ts";
import { rejectionReasons } from "../src/core/types.ts";
import type { OutcomeSummary, RejectionReason } from "../src/core/types.ts";

const watchUrl = "https://www.youtube.com/watch?v=abc";

/** Builds a collected outcome from a selection and optional title and URL. */
function collected(
  selection: string,
  title = "a title",
  url = watchUrl,
): CollectOutcome {
  const input = { selection, url, title } as CollectedInput;
  return { kind: "collected", input };
}

/** Summarizes a collected selection and asserts the summary is collected. */
function collectedSummary(
  selection: string,
): Extract<OutcomeSummary, { kind: "collected" }> {
  const summary = summarize(collected(selection));
  assert.ok(summary.kind === "collected");
  return summary;
}

describe("summarize", () => {
  it("counts a surrogate pair as one character", () => {
    assert.equal(collectedSummary("ab\u{1F600}cd").characterCount, 5);
  });

  it("counts lines split on \\r\\n, \\r or \\n", () => {
    assert.equal(collectedSummary("a\r\nb\rc\nd").lineCount, 4);
  });

  it("counts leading and trailing blank lines", () => {
    assert.equal(collectedSummary("\na\n").lineCount, 3);
  });

  it("keeps a preview at exactly the limit intact", () => {
    const preview = "x".repeat(200);
    const summary = collectedSummary(preview);
    assert.equal(summary.preview, preview);
    assert.equal(summary.previewTruncated, false);
  });

  it("truncates a preview one over the limit", () => {
    const summary = collectedSummary("x".repeat(201));
    assert.equal(summary.preview, "x".repeat(200));
    assert.equal(summary.previewTruncated, true);
  });

  it("does not split a surrogate pair when truncating", () => {
    const summary = collectedSummary("\u{1F600}".repeat(201));
    assert.equal(summary.preview, "\u{1F600}".repeat(200));
    assert.equal(summary.previewTruncated, true);
    assert.equal(summary.characterCount, 201);
  });

  it("keeps a title and URL at exactly the limit intact", () => {
    const long = "t".repeat(1000);
    const summary = summarize(collected("body", long, long));
    assert.ok(summary.kind === "collected");
    assert.equal(summary.title, long);
    assert.equal(summary.titleTruncated, false);
    assert.equal(summary.url, long);
    assert.equal(summary.urlTruncated, false);
  });

  it("truncates a title and URL one over the limit", () => {
    const long = "t".repeat(1001);
    const summary = summarize(collected("body", long, long));
    assert.ok(summary.kind === "collected");
    assert.equal(summary.title, long.slice(0, 1000));
    assert.equal(summary.titleTruncated, true);
    assert.equal(summary.url, long.slice(0, 1000));
    assert.equal(summary.urlTruncated, true);
  });

  it("does not split a surrogate pair when truncating the title or URL", () => {
    const long = "\u{1F600}".repeat(1001);
    const summary = summarize(collected("body", long, long));
    assert.ok(summary.kind === "collected");
    assert.equal(summary.title, "\u{1F600}".repeat(1000));
    assert.equal(summary.titleTruncated, true);
    assert.equal(summary.url, "\u{1F600}".repeat(1000));
    assert.equal(summary.urlTruncated, true);
  });

  it("reduces a collected input to exactly the nine summary members", () => {
    assert.deepEqual(summarize(collected("hello\nworld", "a title")), {
      kind: "collected",
      title: "a title",
      titleTruncated: false,
      url: watchUrl,
      urlTruncated: false,
      characterCount: 11,
      lineCount: 2,
      preview: "hello\nworld",
      previewTruncated: false,
    });
  });

  it("drops the cause of every rejection, keeping only kind and reason", () => {
    const reasons = Object.keys(rejectionReasons) as RejectionReason[];
    for (const reason of reasons) {
      // Only collection-failed declares a cause; the cast gives every reason
      // one, so a summary that copies extra members fails for each of them.
      const outcome = {
        kind: "rejected",
        reason,
        cause: new Error("test"),
      } as CollectOutcome;
      assert.deepEqual(
        summarize(outcome),
        { kind: "rejected", reason },
        reason,
      );
    }
  });

  it("passes a rejection reason through unchanged", () => {
    const outcome: CollectOutcome = {
      kind: "rejected",
      reason: "empty-selection",
    };
    assert.deepEqual(summarize(outcome), {
      kind: "rejected",
      reason: "empty-selection",
    });
  });
});
