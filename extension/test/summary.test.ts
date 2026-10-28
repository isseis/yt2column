import assert from "node:assert/strict";
import { describe, it } from "node:test";

import type { CollectedInput, CollectOutcome } from "../src/core/collect.ts";
import {
  countCharacters,
  countLines,
  summarize,
  truncate,
  truncateTail,
} from "../src/core/summary.ts";
import { rejectionReasons } from "../src/core/types.ts";
import type { RejectionReason } from "../src/core/types.ts";

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

describe("truncate", () => {
  it("keeps text at exactly the limit intact", () => {
    assert.deepEqual(truncate("abcde", 5), {
      value: "abcde",
      truncated: false,
    });
  });

  it("cuts text one over the limit to its first code points", () => {
    assert.deepEqual(truncate("abcde", 4), { value: "abcd", truncated: true });
  });

  it("does not split a surrogate pair at the boundary", () => {
    assert.deepEqual(truncate(`a\u{1F600}b`, 2), {
      value: `a\u{1F600}`,
      truncated: true,
    });
  });
});

describe("truncateTail", () => {
  it("keeps text at exactly the limit intact", () => {
    assert.deepEqual(truncateTail("abcde", 5), {
      value: "abcde",
      truncated: false,
    });
  });

  it("cuts text one over the limit to its last code points", () => {
    assert.deepEqual(truncateTail("abcde", 4), {
      value: "bcde",
      truncated: true,
    });
  });

  it("does not split a surrogate pair at the boundary", () => {
    assert.deepEqual(truncateTail(`ab\u{1F600}`, 2), {
      value: `b\u{1F600}`,
      truncated: true,
    });
  });
});

describe("countCharacters", () => {
  it("counts a surrogate pair as one character", () => {
    assert.equal(countCharacters("ab\u{1F600}cd"), 5);
  });
});

describe("countLines", () => {
  it("counts lines split on \\r\\n, \\r or \\n", () => {
    assert.equal(countLines("a\r\nb\rc\nd"), 4);
  });

  it("counts leading and trailing blank lines", () => {
    assert.equal(countLines("\na\n"), 3);
  });
});

describe("summarize", () => {
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

  // The digest carries nothing beyond its declared members, so the full
  // selection or a rejection's cause cannot reach the display through a spread.
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
