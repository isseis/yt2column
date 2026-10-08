import assert from "node:assert/strict";
import { describe, it } from "node:test";

import type { CollectedInput, CollectOutcome } from "../src/core/collect.ts";
import { parseSummary, summarize } from "../src/core/summary.ts";
import type { OutcomeSummary } from "../src/core/types.ts";

const watchUrl = "https://www.youtube.com/watch?v=abc";

function collected(
  selection: string,
  title = "a title",
  url = watchUrl,
): CollectOutcome {
  const input = { selection, url, title } as CollectedInput;
  return { kind: "collected", input };
}

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

describe("parseSummary", () => {
  it("accepts summarize output for a collected input", () => {
    const summary = summarize(collected("hello\nworld", "a title"));
    assert.deepEqual(parseSummary(summary), summary);
  });

  it("accepts summarize output for a rejection", () => {
    const summary = summarize(collected("x"));
    const rejected = summarize({
      kind: "rejected",
      reason: "empty-title",
    });
    assert.deepEqual(parseSummary(summary), summary);
    assert.deepEqual(parseSummary(rejected), rejected);
  });

  it("rejects a non-object value", () => {
    for (const value of [undefined, null, 42, "x", true]) {
      assert.equal(parseSummary(value), undefined);
    }
  });

  it("rejects an unknown kind", () => {
    assert.equal(parseSummary({ kind: "other" }), undefined);
  });

  it("rejects a missing field", () => {
    const summary = summarize(collected("x"));
    assert.ok(summary.kind === "collected");
    const { preview, ...rest } = summary;
    assert.equal(preview, "x");
    assert.equal(parseSummary(rest), undefined);
  });

  it("rejects an extra field", () => {
    const summary = summarize(collected("x"));
    assert.equal(parseSummary({ ...summary, extra: 1 }), undefined);
  });

  it("rejects a field of the wrong type", () => {
    const summary = summarize(collected("x"));
    assert.ok(summary.kind === "collected");
    assert.equal(parseSummary({ ...summary, characterCount: "5" }), undefined);
  });

  it("rejects an unknown reason", () => {
    assert.equal(
      parseSummary({ kind: "rejected", reason: "bogus" }),
      undefined,
    );
  });

  it("rejects a rejection with an extra field", () => {
    assert.equal(
      parseSummary({ kind: "rejected", reason: "empty-title", cause: "x" }),
      undefined,
    );
  });
});
